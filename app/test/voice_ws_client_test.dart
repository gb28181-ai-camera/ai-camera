// 语音聊天：协议编解码、状态机、半双工门闸、重连退避、PCM 切帧。
//
// 全部使用本地假传输层 / 假音频实现，不联网、不依赖原生插件。
import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:ai_camera_app/services/api_service.dart';
import 'package:ai_camera_app/services/voice_audio.dart';
import 'package:ai_camera_app/services/voice_ws_client.dart';
import 'package:flutter_test/flutter_test.dart';

import 'test_dio_adapter.dart';

/// 假 WebSocket 传输层。
class FakeTransport implements VoiceWsTransport {
  /// 是否在 connect 时抛错。
  bool failConnect = false;

  /// 最近一次连接的地址。
  Uri? connectedUri;

  /// 已发送的文本帧。
  final List<String> textSent = <String>[];

  /// 已发送的二进制帧。
  final List<Uint8List> binarySent = <Uint8List>[];

  /// 是否已关闭。
  bool closed = false;

  final StreamController<Object> _messages = StreamController<Object>.broadcast(
    sync: true,
  );

  @override
  Stream<Object> get messages => _messages.stream;

  @override
  Future<void> connect(Uri uri) async {
    if (failConnect) {
      throw const VoiceWsException('模拟连接失败');
    }
    connectedUri = uri;
  }

  @override
  void sendText(String text) {
    textSent.add(text);
  }

  @override
  void sendBinary(Uint8List data) {
    binarySent.add(data);
  }

  @override
  Future<void> close() async {
    closed = true;
    if (!_messages.isClosed) await _messages.close();
  }

  /// 注入文本帧。
  void emitText(String text) {
    if (!_messages.isClosed) _messages.add(text);
  }

  /// 注入 JSON 帧。
  void emitJson(Map<String, dynamic> message) {
    emitText(jsonEncode(message));
  }

  /// 注入二进制帧。
  void emitBinary(Uint8List data) {
    if (!_messages.isClosed) _messages.add(data);
  }
}

/// 假麦克风。
class FakeMic implements VoiceMicSource {
  bool permission = true;
  int startCount = 0;
  int pauseCount = 0;
  int resumeCount = 0;
  int stopCount = 0;

  @override
  Future<bool> requestPermission() async => permission;

  @override
  Future<void> start(void Function(Uint8List chunk) onChunk) async {
    startCount++;
  }

  @override
  Future<void> pause() async {
    pauseCount++;
  }

  @override
  Future<void> resume() async {
    resumeCount++;
  }

  @override
  Future<void> stop() async {
    stopCount++;
  }

  @override
  Future<void> dispose() async {}
}

/// 假 TTS 播放器。
class FakeTts implements VoiceTtsSink {
  final List<Uint8List> fed = <Uint8List>[];
  int startCount = 0;
  int stopCount = 0;
  int sampleRate = 0;

  @override
  Future<void> start({int sampleRate = voiceDownstreamSampleRate}) async {
    startCount++;
    this.sampleRate = sampleRate;
  }

  @override
  Future<void> feed(Uint8List pcm) async {
    fed.add(pcm);
  }

  @override
  Future<void> stop() async {
    stopCount++;
  }

  @override
  Future<void> dispose() async {}
}

/// 构造测试配置：极短退避与尾巴，避免测试等待。
VoiceWsConfig testConfig({
  Duration reconnectBase = const Duration(milliseconds: 10),
  Duration reconnectMax = const Duration(milliseconds: 20),
  Duration ttsTail = const Duration(milliseconds: 20),
  Duration pingInterval = Duration.zero,
  int maxReconnectAttempts = 0,
}) {
  return VoiceWsConfig(
    baseUrl: 'http://api.local:8080',
    reconnectBase: reconnectBase,
    reconnectMax: reconnectMax,
    ttsTail: ttsTail,
    pingInterval: pingInterval,
    pingTimeout: const Duration(milliseconds: 200),
    helloTimeout: const Duration(seconds: 2),
    maxReconnectAttempts: maxReconnectAttempts,
    clientId: 'app-test',
  );
}

/// 轮询等待条件成立。
Future<void> waitFor(
  bool Function() predicate, {
  Duration timeout = const Duration(seconds: 2),
  String reason = '条件未满足',
}) async {
  final Stopwatch stopwatch = Stopwatch()..start();
  while (!predicate()) {
    if (stopwatch.elapsed > timeout) fail('等待超时：$reason');
    await Future<void>.delayed(const Duration(milliseconds: 5));
  }
}

/// 解析发送帧。
Map<String, dynamic> decodeFrame(String raw) {
  final dynamic value = jsonDecode(raw);
  return value is Map
      ? value.map(
          (dynamic key, dynamic item) =>
              MapEntry<String, dynamic>(key.toString(), item),
        )
      : <String, dynamic>{};
}

/// 服务端 hello。
Map<String, dynamic> serverHello() => <String, dynamic>{
  'type': 'hello',
  'session_id': 's1',
  'transport': 'websocket',
  'audio_params': <String, dynamic>{
    'format': 'pcm16',
    'sample_rate': 24000,
    'channels': 1,
    'frame_duration': 20,
  },
  'capabilities': <String, dynamic>{
    'mcp': true,
    'memory': true,
    'barge_in': false,
  },
};

/// 测试夹具：客户端 + 生成的传输层 + 回调记录。
class Harness {
  Harness({VoiceWsConfig? config, List<FakeTransport>? transports}) {
    final VoiceWsConfig cfg = config ?? testConfig();
    client = VoiceWsClient(
      config: cfg,
      ticketFetcher: () async {
        ticketCalls++;
        return const VoiceWsTicket(ticket: 'tk-1');
      },
      transportFactory: () {
        if (transports != null) {
          final FakeTransport transport = transports[created.length];
          created.add(transport);
          return transport;
        }
        final FakeTransport transport = FakeTransport();
        created.add(transport);
        return transport;
      },
    );
    client
      ..onStateChanged = states.add
      ..onStt = stt.add
      ..onLlm = llm.add
      ..onTts = tts.add
      ..onTool = tools.add
      ..onError = errors.add
      ..onWake = wakes.add
      ..onAudio = audio.add
      ..onMicGateChanged = micGates.add
      ..onReconnecting = (int attempt, Duration delay, String reason) {
        reconnects.add((attempt: attempt, delay: delay, reason: reason));
      }
      ..onServerHello = (VoiceServerHello hello) => hellos.add(hello);
  }

  late final VoiceWsClient client;
  final List<FakeTransport> created = <FakeTransport>[];
  final List<VoiceClientState> states = <VoiceClientState>[];
  final List<VoiceSttEvent> stt = <VoiceSttEvent>[];
  final List<VoiceLlmEvent> llm = <VoiceLlmEvent>[];
  final List<VoiceTtsEvent> tts = <VoiceTtsEvent>[];
  final List<VoiceToolEvent> tools = <VoiceToolEvent>[];
  final List<VoiceErrorEvent> errors = <VoiceErrorEvent>[];
  final List<VoiceWakeEvent> wakes = <VoiceWakeEvent>[];
  final List<Uint8List> audio = <Uint8List>[];
  final List<bool> micGates = <bool>[];
  final List<({int attempt, Duration delay, String reason})> reconnects =
      <({int attempt, Duration delay, String reason})>[];
  final List<VoiceServerHello> hellos = <VoiceServerHello>[];
  int ticketCalls = 0;

  FakeTransport get transport => created.last;
}

/// 建立连接并完成 hello。
Future<Harness> connectedHarness({VoiceWsConfig? config}) async {
  final Harness h = Harness(config: config);
  await h.client.connect();
  h.transport.emitJson(serverHello());
  await waitFor(() => h.client.isReady, reason: '服务端 hello');
  return h;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  group('buildVoiceWsUri', () {
    test('http → ws，并补 path 与 ticket', () {
      final Uri uri = buildVoiceWsUri(
        ticket: 'abc',
        baseUrl: 'http://10.0.2.2:8080',
      );
      expect(uri.scheme, 'ws');
      expect(uri.host, '10.0.2.2');
      expect(uri.port, 8080);
      expect(uri.path, '/app/voice/ws');
      expect(uri.queryParameters['ticket'], 'abc');
    });

    test('https → wss', () {
      final Uri uri = buildVoiceWsUri(
        ticket: 'abc',
        baseUrl: 'https://api.example.com',
      );
      expect(uri.scheme, 'wss');
      expect(uri.host, 'api.example.com');
      expect(uri.path, '/app/voice/ws');
    });

    test('显式 serverUri 保留路径并覆盖 ticket', () {
      final Uri uri = buildVoiceWsUri(
        ticket: 't2',
        baseUrl: 'http://ignored.local',
        serverUri: Uri.parse('ws://pipecat.local/ws/voice?x=1'),
      );
      expect(uri.toString(), 'ws://pipecat.local/ws/voice?x=1&ticket=t2');
    });

    test('基地址带路径前缀时保留前缀', () {
      final Uri uri = buildVoiceWsUri(
        ticket: 't3',
        baseUrl: 'http://gateway.local:9000/api/',
      );
      expect(uri.path, '/api/app/voice/ws');
    });
  });

  group('票据接口（mock HTTP）', () {
    tearDown(() {
      ApiService.resetDio();
      ApiService.setToken(null);
    });

    test('POST /app/voice/ws-ticket 并解析 ticket', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(<String, Object>{
        '/app/voice/ws-ticket': wrapSuccess(<String, dynamic>{
          'ticket': 'tk-http',
          'expires_in': 60,
        }),
      });
      ApiService.dio = buildTestDio(adapter);
      ApiService.setToken('jwt-test');

      final VoiceWsTicket ticket = await fetchVoiceTicketFromApi();

      expect(ticket.ticket, 'tk-http');
      expect(ticket.expiresIn, 60);
      expect(adapter.countOf('/app/voice/ws-ticket'), 1);
      expect(adapter.lastRequest!.headers['Authorization'], 'Bearer jwt-test');
    });

    test('业务错误转 VoiceWsException', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(<String, Object>{
        '/app/voice/ws-ticket': wrapError(5001, '票据服务不可用'),
      });
      ApiService.dio = buildTestDio(adapter);
      ApiService.setToken('jwt-test');

      await expectLater(
        fetchVoiceTicketFromApi(),
        throwsA(
          isA<VoiceWsException>().having(
            (VoiceWsException e) => e.message,
            'message',
            '票据服务不可用',
          ),
        ),
      );
    });
  });

  group('hello 与消息分发', () {
    test('首帧 hello 字段符合协议', () async {
      final Harness h = Harness();
      await h.client.connect();
      await h.client.connect(); // 幂等：不应重复连接

      expect(h.created, hasLength(1));
      expect(h.transport.connectedUri!.path, '/app/voice/ws');
      expect(h.transport.connectedUri!.queryParameters['ticket'], 'tk-1');
      expect(h.ticketCalls, 1);

      final Map<String, dynamic> hello = decodeFrame(
        h.transport.textSent.first,
      );
      expect(hello['type'], 'hello');
      expect(hello['version'], 1);
      expect(hello['transport'], 'websocket');
      expect((hello['features'] as Map)['mcp'], true);
      expect((hello['features'] as Map)['wake_word'], 'device');
      final Map<String, dynamic> audio = (hello['audio_params'] as Map)
          .cast<String, dynamic>();
      expect(audio['format'], 'pcm16');
      expect(audio['sample_rate'], 16000);
      expect(audio['channels'], 1);
      expect(audio['frame_duration'], 20);
      expect((hello['client'] as Map)['kind'], 'app');
      expect((hello['client'] as Map)['client_id'], 'app-test');

      expect(h.client.state, VoiceClientState.connecting);
    });

    test('服务端 hello → ready，并解析下行采样率与能力', () async {
      final Harness h = Harness();
      await h.client.connect();
      h.transport.emitJson(serverHello());

      expect(h.client.state, VoiceClientState.ready);
      expect(h.client.isReady, isTrue);
      expect(h.hellos, hasLength(1));
      expect(h.hellos.single.sessionId, 's1');
      expect(h.hellos.single.audioParams.sampleRate, 24000);
      expect(h.hellos.single.supportsMcp, isTrue);
      expect(h.states, contains(VoiceClientState.ready));

      await h.client.disconnect();
    });

    test('stt / llm / tool / error / wake 回调', () async {
      final Harness h = await connectedHarness();
      h.client.sendListenStart(mode: 'auto');

      h.transport.emitJson(<String, dynamic>{
        'type': 'stt',
        'text': '打开客厅灯',
        'is_final': false,
        'confidence': 0.91,
        'turn_id': 't1',
      });
      expect(h.stt.single.isFinal, isFalse);
      expect(h.stt.single.text, '打开客厅灯');
      expect(h.stt.single.confidence, closeTo(0.91, 0.001));
      expect(h.client.state, VoiceClientState.listening);

      h.transport.emitJson(<String, dynamic>{
        'type': 'stt',
        'text': '打开客厅灯',
        'is_final': true,
      });
      expect(h.stt, hasLength(2));
      expect(h.stt.last.isFinal, isTrue);
      expect(h.client.state, VoiceClientState.thinking);

      h.transport.emitJson(<String, dynamic>{
        'type': 'llm',
        'text': '好的，',
        'emotion': 'happy',
      });
      expect(h.llm.single.text, '好的，');
      expect(h.llm.single.emotion, 'happy');

      h.transport.emitJson(<String, dynamic>{
        'type': 'tool',
        'state': 'calling',
        'name': 'device.list',
        'display': '查询设备',
      });
      expect(h.tools.single.name, 'device.list');
      expect(h.tools.single.state, 'calling');

      h.transport.emitJson(<String, dynamic>{
        'type': 'error',
        'code': 'llm_overloaded',
        'message': '模型繁忙',
        'retryable': true,
      });
      expect(h.errors.single.code, 'llm_overloaded');
      expect(h.errors.single.retryable, isTrue);

      h.transport.emitJson(<String, dynamic>{
        'type': 'wake',
        'state': 'detected',
        'text': '小智小智',
        'confidence': 0.88,
      });
      expect(h.wakes.single.text, '小智小智');

      await h.client.disconnect();
    });

    test('二进制帧原样回调；非法 JSON 被忽略', () async {
      final Harness h = await connectedHarness();
      final Uint8List pcm = Uint8List.fromList(<int>[1, 2, 3, 4]);

      h.transport.emitText('not json');
      h.transport.emitBinary(pcm);

      expect(h.audio, hasLength(1));
      expect(h.audio.single, pcm);
      await h.client.disconnect();
    });
  });

  group('listen / abort 与半双工', () {
    test('listen start/stop 与上行音频门闸', () async {
      final Harness h = await connectedHarness();

      h.client.sendListenStart(mode: 'manual');
      expect(decodeFrame(h.transport.textSent.last), <String, dynamic>{
        'type': 'listen',
        'state': 'start',
        'mode': 'manual',
      });
      expect(h.client.state, VoiceClientState.listening);
      expect(h.client.micGateOpen, isTrue);
      expect(h.micGates.last, isTrue);

      final Uint8List frame = Uint8List(voiceFrameBytes);
      h.client.sendAudio(frame);
      expect(h.transport.binarySent, hasLength(1));
      expect(h.transport.binarySent.single.length, voiceFrameBytes);

      h.client.sendListenStop();
      expect(decodeFrame(h.transport.textSent.last), <String, dynamic>{
        'type': 'listen',
        'state': 'stop',
      });
      expect(h.client.state, VoiceClientState.thinking);
      expect(h.client.micGateOpen, isFalse);

      h.client.sendAudio(frame);
      expect(h.transport.binarySent, hasLength(1), reason: '门闸关闭时丢弃上行');

      await h.client.disconnect();
    });

    test('tts start 静音，tts stop 保留 300ms（测试用 20ms）尾巴后恢复', () async {
      final Harness h = await connectedHarness();
      h.client.sendListenStart(mode: 'auto');
      expect(h.client.micGateOpen, isTrue);

      h.transport.emitJson(<String, dynamic>{
        'type': 'tts',
        'state': 'start',
        'text': '你好',
        'turn_id': 't1',
      });
      expect(h.client.state, VoiceClientState.speaking);
      expect(h.client.micGateOpen, isFalse);
      expect(h.micGates.last, isFalse);

      h.client.sendAudio(Uint8List(voiceFrameBytes));
      expect(h.transport.binarySent, isEmpty);

      h.transport.emitJson(<String, dynamic>{
        'type': 'tts',
        'state': 'stop',
        'turn_id': 't1',
      });
      expect(h.client.micGateOpen, isFalse, reason: '尾巴期间仍静音');

      await waitFor(() => h.client.micGateOpen, reason: 'TTS 尾巴结束后恢复');
      expect(h.client.state, VoiceClientState.listening);
      expect(h.micGates.last, isTrue);

      await h.client.disconnect();
    });

    test('手动结束 listen 后，tts stop 不会重新打开门闸', () async {
      final Harness h = await connectedHarness();
      h.client.sendListenStart(mode: 'manual');
      h.client.sendListenStop();

      h.transport.emitJson(<String, dynamic>{
        'type': 'tts',
        'state': 'start',
        'text': '回复',
      });
      expect(h.client.micGateOpen, isFalse);
      h.transport.emitJson(<String, dynamic>{'type': 'tts', 'state': 'stop'});

      await Future<void>.delayed(const Duration(milliseconds: 50));
      expect(h.client.micGateOpen, isFalse);
      expect(h.client.state, VoiceClientState.ready);

      await h.client.disconnect();
    });

    test('abort 发送原因并复位状态', () async {
      final Harness h = await connectedHarness();
      h.client.sendListenStart(mode: 'auto');

      h.client.abort(reason: 'user');

      expect(decodeFrame(h.transport.textSent.last), <String, dynamic>{
        'type': 'abort',
        'reason': 'user',
      });
      expect(h.client.state, VoiceClientState.ready);
      expect(h.client.micGateOpen, isFalse);

      await h.client.disconnect();
    });

    test('mcp 与 detect 上行', () async {
      final Harness h = await connectedHarness();

      h.client.sendListenDetect(text: '小智小智');
      expect(decodeFrame(h.transport.textSent.last), <String, dynamic>{
        'type': 'listen',
        'state': 'detect',
        'mode': 'auto',
        'text': '小智小智',
      });

      h.client.sendMcp(<String, dynamic>{
        'jsonrpc': '2.0',
        'id': 1,
        'method': 'tools/list',
      });
      final Map<String, dynamic> mcp = decodeFrame(h.transport.textSent.last);
      expect(mcp['type'], 'mcp');
      expect((mcp['payload'] as Map)['method'], 'tools/list');

      await h.client.disconnect();
    });
  });

  group('重连与心跳', () {
    test('指数退避重连：失败两次后第三次成功', () async {
      final List<FakeTransport> transports = <FakeTransport>[
        FakeTransport()..failConnect = true,
        FakeTransport()..failConnect = true,
        FakeTransport(),
      ];
      final Harness h = Harness(
        config: testConfig(
          reconnectBase: const Duration(milliseconds: 10),
          reconnectMax: const Duration(milliseconds: 20),
        ),
        transports: transports,
      );

      await h.client.connect();
      await waitFor(
        () => h.created.length >= 3 && h.transport.connectedUri != null,
        reason: '第三次连接建立',
      );

      expect(h.ticketCalls, 3, reason: '每次重连都重新取 ticket');
      expect(h.reconnects, hasLength(2));
      expect(h.reconnects[0].delay, const Duration(milliseconds: 10));
      expect(h.reconnects[1].delay, const Duration(milliseconds: 20));
      expect(h.reconnects[0].attempt, 1);
      expect(h.reconnects[1].attempt, 2);
      expect(h.client.state, VoiceClientState.connecting);

      h.transport.emitJson(serverHello());
      await waitFor(() => h.client.isReady, reason: '重连后 hello');

      await h.client.disconnect();
    });

    test('达到重连上限进入 error 并上报错误', () async {
      final FakeTransport failing = FakeTransport()..failConnect = true;
      // 所有尝试都失败：工厂返回同一个失败传输层（close 后可复用）。
      final VoiceWsClient client = VoiceWsClient(
        config: testConfig(
          reconnectBase: const Duration(milliseconds: 5),
          reconnectMax: const Duration(milliseconds: 5),
          maxReconnectAttempts: 2,
        ),
        ticketFetcher: () async => const VoiceWsTicket(ticket: 'tk'),
        transportFactory: () => failing,
      );
      final List<VoiceClientState> states = <VoiceClientState>[];
      final List<VoiceErrorEvent> errors = <VoiceErrorEvent>[];
      client
        ..onStateChanged = states.add
        ..onError = errors.add;

      await client.connect();
      await waitFor(
        () => client.state == VoiceClientState.error,
        reason: '重连次数耗尽',
      );

      expect(states, contains(VoiceClientState.reconnecting));
      expect(errors.single.code, 'ws_reconnect_failed');
      expect(client.reconnectAttempt, 3);
      await client.disconnect();
    });

    test('主动断开后不再重连', () async {
      final Harness h = await connectedHarness();
      await h.client.disconnect();

      final int createdBefore = h.created.length;
      await Future<void>.delayed(const Duration(milliseconds: 50));

      expect(h.created, hasLength(createdBefore));
      expect(h.client.state, VoiceClientState.closed);
      expect(h.transport.closed, isTrue);
    });

    test('按间隔发送 ping', () async {
      final Harness h = await connectedHarness(
        config: testConfig(pingInterval: const Duration(milliseconds: 10)),
      );

      await waitFor(
        () => h.transport.textSent.any(
          (String raw) => decodeFrame(raw)['type'] == 'ping',
        ),
        reason: '心跳 ping',
      );

      await h.client.disconnect();
    });

    test('等待 hello 超时会触发重连', () async {
      const VoiceWsConfig config = VoiceWsConfig(
        baseUrl: 'http://api.local:8080',
        reconnectBase: Duration(milliseconds: 5),
        reconnectMax: Duration(milliseconds: 5),
        helloTimeout: Duration(milliseconds: 10),
        pingInterval: Duration.zero,
      );
      final Harness h = Harness(config: config);
      await h.client.connect();

      await waitFor(() => h.reconnects.isNotEmpty, reason: 'hello 超时后重连');
      await h.client.disconnect();
    });
  });

  group('PcmFrameSlicer 与音频会话', () {
    test('跨 chunk 切帧不丢字节', () {
      final PcmFrameSlicer slicer = PcmFrameSlicer();

      expect(slicer.add(Uint8List(100)), isEmpty);
      expect(slicer.pendingBytes, 100);

      // 100 + 1180 = 1280 = 2 × 640（16kHz/20ms）
      final List<Uint8List> frames = slicer.add(Uint8List(1180));
      expect(frames, hasLength(2));
      expect(
        frames.every((Uint8List f) => f.length == voiceFrameBytes),
        isTrue,
      );
      expect(slicer.pendingBytes, 0);

      // 700 = 1 × 640 + 60 残余
      final List<Uint8List> more = slicer.add(Uint8List(700));
      expect(more, hasLength(1));
      expect(slicer.pendingBytes, 60);

      slicer.reset();
      expect(slicer.pendingBytes, 0);
    });

    test('音频会话的暂停 / 恢复 / 播放顺序', () async {
      final FakeMic mic = FakeMic();
      final FakeTts tts = FakeTts();
      final VoiceAudioSession session = VoiceAudioSession(mic: mic, tts: tts);

      expect(await session.requestPermission(), isTrue);
      await session.startCapture((Uint8List _) {});
      expect(mic.startCount, 1);
      expect(session.isCapturing, isTrue);

      await session.pauseCapture();
      await session.pauseCapture();
      expect(mic.pauseCount, 1);
      expect(session.isCapturePaused, isTrue);

      await session.resumeCapture();
      expect(mic.resumeCount, 1);
      expect(session.isCapturePaused, isFalse);

      await session.play(Uint8List.fromList(<int>[1, 2]));
      expect(tts.startCount, 1, reason: '首次播放自动启动播放器');
      expect(tts.sampleRate, voiceDownstreamSampleRate);
      expect(tts.fed, hasLength(1));

      await session.dispose();
      expect(mic.stopCount, 1);
      expect(tts.stopCount, 1);
    });

    test('权限被拒时不启动采集', () async {
      final FakeMic mic = FakeMic()..permission = false;
      final VoiceAudioSession session = VoiceAudioSession(
        mic: mic,
        tts: FakeTts(),
      );

      expect(await session.requestPermission(), isFalse);
      expect(session.isCapturing, isFalse);
      await session.dispose();
    });
  });
}
