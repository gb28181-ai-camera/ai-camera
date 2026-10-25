/// 语音聊天 WebSocket 客户端（App 侧）。
///
/// 协议见 `docs/敏捷开发/v0.6.0 智能语音聊天/00-设计与协议.md`：
///
/// 1. `POST /app/voice/ws-ticket` 取一次性 ticket（复用登录 JWT）；
/// 2. `ws://<api-host>/app/voice/ws?ticket=...`（https 时为 `wss`）；
/// 3. 首帧 `hello`（PCM16 / 16k 上行，features.mcp=true，wake_word=device）；
/// 4. 上行 `listen` / `abort` / `mcp` / `ping` + 二进制 PCM16；
/// 5. 下行 `hello` / `stt` / `llm` / `tts` / `tool` / `error` / `wake` + 二进制 PCM16。
///
/// 设计要点：
/// - **传输层可替换**（[VoiceWsTransport]），单元测试用假传输层验证协议，
///   不依赖真机、也不联网；
/// - **重连采用指数退避**（默认 0.5s 起，封顶 30s），每次重连都重新取 ticket；
/// - **半双工由客户端兜底**：`tts/start` 关闭麦克风门闸，`tts/stop` 后保留
///   300ms 尾巴再打开（服务端也会丢弃这段时间的上行，客户端先做一层保护）；
/// - 不缓存、不落盘任何音频内容。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:web_socket_channel/io.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../config/api_config.dart';
import 'api_service.dart';

// ════════════════════════════════════════════════════════════════
// 协议常量
// ════════════════════════════════════════════════════════════════

/// App 侧上行采样率（PCM16 单声道）。
const int voiceUpstreamSampleRate = 16000;

/// 服务端下发的 TTS 采样率（服务端 hello 可覆盖）。
const int voiceDownstreamSampleRate = 24000;

/// 音频帧时长（毫秒）。
const int voiceFrameDurationMs = 20;

/// 每帧 PCM16 字节数。
///
/// 按项目约定切帧（20ms/320 字节）；`hello.audio_params.frame_duration`
/// 仍按协议固定上报 20ms，服务端按自身实现消费字节流。
/// PCM16 / 16kHz / 单声道 20ms 的字节数：16000 × 0.02 × 2 = 640。
/// （8kHz 才是 320；设备链路与 App 上行统一 16kHz，别写错。）
const int voiceFrameBytes = 640;

/// 票据接口路径。
const String voiceWsTicketPath = '/app/voice/ws-ticket';

/// WebSocket 接口路径。
const String voiceWsPath = '/app/voice/ws';

// ════════════════════════════════════════════════════════════════
// 异常与数据模型
// ════════════════════════════════════════════════════════════════

/// 语音链路异常（票据获取失败、协议字段缺失等）。
class VoiceWsException implements Exception {
  /// 构造异常。
  const VoiceWsException(this.message);

  /// 可读错误信息。
  final String message;

  @override
  String toString() => 'VoiceWsException: $message';
}

/// WS 一次性票据。
class VoiceWsTicket {
  /// 构造票据。
  const VoiceWsTicket({
    required this.ticket,
    this.expiresIn = 60,
    this.wsUrl = '',
  });

  /// 解析接口响应体。
  ///
  /// 兼容三种形态：
  /// - `{code, message, data: {ticket, expires_in, ws_url?}}`
  /// - `{code, message, data: "ticket"}`
  /// - 顶层直接 `{ticket: "..."}`
  factory VoiceWsTicket.fromResponse(dynamic body) {
    final Map<String, dynamic> root = _asMap(body);
    final dynamic data = root['data'];

    String ticket = '';
    int expiresIn = 60;
    String wsUrl = '';

    if (data is String) {
      ticket = data.trim();
    } else if (data is Map) {
      final Map<String, dynamic> payload = _asMap(data);
      ticket = _asString(payload['ticket']).trim();
      expiresIn = _asInt(payload['expires_in'], expiresIn);
      wsUrl = _asString(payload['ws_url']).trim();
    }

    if (ticket.isEmpty) {
      ticket = _asString(root['ticket']).trim();
    }
    if (expiresIn <= 0) expiresIn = 60;

    if (ticket.isEmpty) {
      final String message = _asString(root['message']);
      throw VoiceWsException(message.isNotEmpty ? message : '语音服务未返回连接票据');
    }

    return VoiceWsTicket(ticket: ticket, expiresIn: expiresIn, wsUrl: wsUrl);
  }

  /// 一次性票据。
  final String ticket;

  /// 票据有效期（秒）。
  final int expiresIn;

  /// 服务端显式下发的 WS 地址（可选）。
  final String wsUrl;
}

/// 音频参数。
class VoiceAudioParams {
  /// 构造音频参数。
  const VoiceAudioParams({
    this.format = 'pcm16',
    this.sampleRate = voiceDownstreamSampleRate,
    this.channels = 1,
    this.frameDuration = voiceFrameDurationMs,
  });

  /// 从 JSON 解析，缺省值按服务端下行约定。
  factory VoiceAudioParams.fromJson(Map<String, dynamic> json) {
    return VoiceAudioParams(
      format: _asString(json['format']).isNotEmpty
          ? _asString(json['format'])
          : 'pcm16',
      sampleRate: _asInt(json['sample_rate'], voiceDownstreamSampleRate),
      channels: _asInt(json['channels'], 1),
      frameDuration: _asInt(json['frame_duration'], voiceFrameDurationMs),
    );
  }

  /// 编码格式：`pcm16` / `opus`。
  final String format;

  /// 采样率。
  final int sampleRate;

  /// 声道数。
  final int channels;

  /// 帧时长（毫秒）。
  final int frameDuration;

  /// 是否为 App 端支持的 PCM16 单声道。
  bool get isPcm16Mono => format == 'pcm16' && channels == 1;

  /// 序列化为 JSON（客户端 hello 使用）。
  Map<String, dynamic> toJson() => <String, dynamic>{
    'format': format,
    'sample_rate': sampleRate,
    'channels': channels,
    'frame_duration': frameDuration,
  };
}

/// 服务端 hello。
class VoiceServerHello {
  /// 构造 hello。
  const VoiceServerHello({
    required this.sessionId,
    required this.transport,
    required this.audioParams,
    this.capabilities = const <String, dynamic>{},
  });

  /// 从 JSON 解析。
  factory VoiceServerHello.fromJson(Map<String, dynamic> json) {
    return VoiceServerHello(
      sessionId: _asString(json['session_id']),
      transport: _asString(json['transport']),
      audioParams: VoiceAudioParams.fromJson(_asMap(json['audio_params'])),
      capabilities: _asMap(json['capabilities']),
    );
  }

  /// 会话 ID。
  final String sessionId;

  /// 传输类型。
  final String transport;

  /// 服务端音频参数（下行采样率通常为 24000）。
  final VoiceAudioParams audioParams;

  /// 服务端能力（mcp / memory / barge_in）。
  final Map<String, dynamic> capabilities;

  /// 是否支持 MCP。
  bool get supportsMcp => _asBool(capabilities['mcp'], false);
}

/// 服务端 STT 事件。
class VoiceSttEvent {
  /// 构造事件。
  const VoiceSttEvent({
    required this.text,
    required this.isFinal,
    this.confidence = 0,
    this.turnId = '',
  });

  /// 从 JSON 解析。
  factory VoiceSttEvent.fromJson(Map<String, dynamic> json) {
    return VoiceSttEvent(
      text: _asString(json['text']),
      isFinal: _asBool(json['is_final'], false),
      confidence: _asDouble(json['confidence'], 0),
      turnId: _asString(json['turn_id']),
    );
  }

  /// 识别文本。
  final String text;

  /// 是否为最终结果。
  final bool isFinal;

  /// 置信度。
  final double confidence;

  /// 轮次 ID。
  final String turnId;
}

/// 服务端 LLM 事件。
class VoiceLlmEvent {
  /// 构造事件。
  const VoiceLlmEvent({required this.text, this.emotion = ''});

  /// 从 JSON 解析。
  factory VoiceLlmEvent.fromJson(Map<String, dynamic> json) {
    return VoiceLlmEvent(
      text: _asString(json['text']),
      emotion: _asString(json['emotion']),
    );
  }

  /// 增量文本。
  final String text;

  /// 情绪标签（可选）。
  final String emotion;
}

/// 服务端 TTS 事件。
class VoiceTtsEvent {
  /// 构造事件。
  const VoiceTtsEvent({required this.state, this.text = '', this.turnId = ''});

  /// 从 JSON 解析。
  factory VoiceTtsEvent.fromJson(Map<String, dynamic> json) {
    return VoiceTtsEvent(
      state: _asString(json['state']),
      text: _asString(json['text']),
      turnId: _asString(json['turn_id']),
    );
  }

  /// 状态：`start` / `sentence_start` / `stop`。
  final String state;

  /// 当前句子文本。
  final String text;

  /// 轮次 ID。
  final String turnId;
}

/// 工具调用事件（服务端 MCP 工具执行状态）。
class VoiceToolEvent {
  /// 构造事件。
  const VoiceToolEvent({
    required this.state,
    required this.name,
    this.display = '',
  });

  /// 从 JSON 解析。
  factory VoiceToolEvent.fromJson(Map<String, dynamic> json) {
    return VoiceToolEvent(
      state: _asString(json['state']),
      name: _asString(json['name']),
      display: _asString(json['display']),
    );
  }

  /// 状态：`calling` / `result` / `error`。
  final String state;

  /// 工具名（如 `device.list`）。
  final String name;

  /// 展示文案。
  final String display;
}

/// 服务端错误事件。
class VoiceErrorEvent {
  /// 构造事件。
  const VoiceErrorEvent({
    required this.code,
    required this.message,
    this.retryable = false,
  });

  /// 从 JSON 解析。
  factory VoiceErrorEvent.fromJson(Map<String, dynamic> json) {
    return VoiceErrorEvent(
      code: _asString(json['code']),
      message: _asString(json['message']),
      retryable: _asBool(json['retryable'], false),
    );
  }

  /// 错误码。
  final String code;

  /// 错误信息。
  final String message;

  /// 是否可重试。
  final bool retryable;
}

/// 唤醒事件（设备端唤醒回执）。
class VoiceWakeEvent {
  /// 构造事件。
  const VoiceWakeEvent({
    required this.state,
    this.text = '',
    this.confidence = 0,
  });

  /// 从 JSON 解析。
  factory VoiceWakeEvent.fromJson(Map<String, dynamic> json) {
    return VoiceWakeEvent(
      state: _asString(json['state']),
      text: _asString(json['text']),
      confidence: _asDouble(json['confidence'], 0),
    );
  }

  /// 状态（`detected`）。
  final String state;

  /// 唤醒词。
  final String text;

  /// 置信度。
  final double confidence;
}

// ════════════════════════════════════════════════════════════════
// 连接状态
// ════════════════════════════════════════════════════════════════

/// 语音链路状态（UI 直接映射）。
enum VoiceClientState {
  /// 未连接。
  idle,

  /// 正在取票据 / 建连 / 等 hello。
  connecting,

  /// 已握手，等待用户说话。
  ready,

  /// 正在上行音频（聆听）。
  listening,

  /// 已停止上行，等待 STT/LLM。
  thinking,

  /// 正在播放 TTS。
  speaking,

  /// 断线后指数退避重连中。
  reconnecting,

  /// 重连次数耗尽（仅当配置了上限）。
  error,

  /// 主动关闭。
  closed,
}

// ════════════════════════════════════════════════════════════════
// 传输层（可替换，便于单测）
// ════════════════════════════════════════════════════════════════

/// WebSocket 传输层抽象。
///
/// [messages] 依次给出 `String`（文本帧）或 `Uint8List`（二进制帧）。
abstract class VoiceWsTransport {
  /// 建立连接。
  Future<void> connect(Uri uri);

  /// 发送文本帧。
  void sendText(String text);

  /// 发送二进制帧。
  void sendBinary(Uint8List data);

  /// 下行消息流。
  Stream<Object> get messages;

  /// 关闭连接。
  Future<void> close();
}

/// 基于 `web_socket_channel` 的默认传输层。
class IOVoiceWsTransport implements VoiceWsTransport {
  /// 构造传输层。
  IOVoiceWsTransport({this.connectTimeout = const Duration(seconds: 10)});

  /// 建连超时。
  final Duration connectTimeout;

  final StreamController<Object> _messages = StreamController<Object>.broadcast(
    sync: true,
  );

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _subscription;

  @override
  Stream<Object> get messages => _messages.stream;

  @override
  Future<void> connect(Uri uri) async {
    final WebSocketChannel channel = IOWebSocketChannel.connect(
      uri,
      connectTimeout: connectTimeout,
    );
    await channel.ready;
    _channel = channel;
    _subscription = channel.stream.listen(
      (dynamic message) {
        if (message is String) {
          _messages.add(message);
        } else if (message is Uint8List) {
          _messages.add(message);
        } else if (message is List<int>) {
          _messages.add(Uint8List.fromList(message));
        }
      },
      onError: (Object error, StackTrace stackTrace) {
        if (!_messages.isClosed) _messages.addError(error, stackTrace);
      },
      onDone: () {
        if (!_messages.isClosed) unawaited(_messages.close());
      },
    );
  }

  @override
  void sendText(String text) => _channel?.sink.add(text);

  @override
  void sendBinary(Uint8List data) => _channel?.sink.add(data);

  @override
  Future<void> close() async {
    final StreamSubscription<dynamic>? subscription = _subscription;
    _subscription = null;
    await subscription?.cancel();
    final WebSocketChannel? channel = _channel;
    _channel = null;
    try {
      await channel?.sink.close();
    } catch (_) {
      // 关闭失败不向上抛：调用方正在做清理。
    }
    if (!_messages.isClosed) await _messages.close();
  }
}

// ════════════════════════════════════════════════════════════════
// 配置与工具函数
// ════════════════════════════════════════════════════════════════

/// 票据获取函数类型。
typedef VoiceTicketFetcher = Future<VoiceWsTicket> Function();

/// 传输层工厂类型。
typedef VoiceTransportFactory = VoiceWsTransport Function();

/// 语音 WS 客户端配置。
class VoiceWsConfig {
  /// 构造配置。
  const VoiceWsConfig({
    this.baseUrl,
    this.serverUri,
    this.clientKind = 'app',
    this.clientId,
    this.deviceId,
    this.pingInterval = const Duration(seconds: 15),
    this.pingTimeout = const Duration(seconds: 10),
    this.reconnectBase = const Duration(milliseconds: 500),
    this.reconnectMax = const Duration(seconds: 30),
    this.ttsTail = const Duration(milliseconds: 300),
    this.helloTimeout = const Duration(seconds: 10),
    this.maxReconnectAttempts = 0,
  });

  /// HTTP 基地址；为空时使用 [ApiConfig.baseUrl]。
  final String? baseUrl;

  /// 显式 WS 地址（优先级最高，测试或专网部署用）。
  final Uri? serverUri;

  /// 客户端类型（`app`）。
  final String clientKind;

  /// 客户端 ID；为空时自动生成。
  final String? clientId;

  /// 绑定设备 ID（可选）。
  final String? deviceId;

  /// 心跳间隔。
  final Duration pingInterval;

  /// 心跳超时（超时视为断线）。
  final Duration pingTimeout;

  /// 重连退避基数。
  final Duration reconnectBase;

  /// 重连退避上限（默认 30s）。
  final Duration reconnectMax;

  /// TTS 结束后保留的尾巴时长。
  final Duration ttsTail;

  /// 等待服务端 hello 的超时。
  final Duration helloTimeout;

  /// 重连次数上限；`<= 0` 表示无限重试。
  final int maxReconnectAttempts;

  /// 生效的 HTTP 基地址。
  String get effectiveBaseUrl => baseUrl ?? ApiConfig.baseUrl;
}

/// 默认票据获取：`POST /app/voice/ws-ticket`（自动带 Bearer token）。
Future<VoiceWsTicket> fetchVoiceTicketFromApi() async {
  final dynamic response = await ApiService.post(voiceWsTicketPath, null, null);
  final dynamic body = response.data;
  final int httpStatus = response.statusCode ?? 0;
  final Map<String, dynamic> root = _asMap(body);
  final int code = _asInt(root['code'], httpStatus);

  if (httpStatus < 200 || httpStatus >= 300 || (code != 0 && code != 200)) {
    final String message = _asString(root['message']);
    throw VoiceWsException(message.isNotEmpty ? message : '获取语音连接票据失败');
  }
  return VoiceWsTicket.fromResponse(body);
}

/// 由 ticket 与 HTTP 基地址构造 WS 地址。
///
/// - `http://host:8080` → `ws://host:8080/app/voice/ws?ticket=...`
/// - `https://host` → `wss://host/app/voice/ws?ticket=...`
/// - [serverUri] 非空时直接使用它，但会补齐 ticket 并规范 scheme。
Uri buildVoiceWsUri({
  required String ticket,
  required String baseUrl,
  Uri? serverUri,
}) {
  final Uri base;
  if (serverUri != null) {
    base = serverUri;
  } else {
    final String trimmed = baseUrl.trim();
    if (trimmed.isEmpty) {
      throw const VoiceWsException('缺少语音服务地址');
    }
    base = Uri.parse(trimmed);
  }

  final String scheme;
  switch (base.scheme.toLowerCase()) {
    case 'https':
    case 'wss':
      scheme = 'wss';
      break;
    case 'http':
    case 'ws':
      scheme = 'ws';
      break;
    default:
      scheme = base.scheme.isEmpty ? 'ws' : base.scheme;
  }

  String path = base.path;
  while (path.endsWith('/')) {
    path = path.substring(0, path.length - 1);
  }
  // 显式 serverUri / 服务端下发的 ws_url 视为完整地址，不再补默认 path。
  if (serverUri == null && !path.endsWith(voiceWsPath)) {
    path = '$path$voiceWsPath';
  }

  final Uri target = base.replace(
    scheme: scheme,
    path: path,
    queryParameters: <String, String>{
      ...base.queryParameters,
      'ticket': ticket,
    },
  );
  return target;
}

// ════════════════════════════════════════════════════════════════
// 客户端
// ════════════════════════════════════════════════════════════════

/// 语音聊天 WebSocket 客户端。
///
/// 使用方式：
/// ```dart
/// final client = VoiceWsClient();
/// client
///   ..onStateChanged = setState
///   ..onAudio = (pcm) => audio.play(pcm)
///   ..onMicGateChanged = (open) => open ? audio.resume() : audio.pause();
/// await client.connect();
/// ```
class VoiceWsClient {
  /// 构造客户端。
  VoiceWsClient({
    VoiceWsConfig config = const VoiceWsConfig(),
    VoiceTicketFetcher? ticketFetcher,
    VoiceTransportFactory? transportFactory,
  }) : _config = config,
       _ticketFetcher = ticketFetcher ?? fetchVoiceTicketFromApi,
       _transportFactory = transportFactory ?? (() => IOVoiceWsTransport()) {
    _clientId = _config.clientId ?? _newClientId();
  }

  // ── 回调 ──────────────────────────────────────────────────
  /// 状态变化。
  void Function(VoiceClientState state)? onStateChanged;

  /// 服务端 hello。
  void Function(VoiceServerHello hello)? onServerHello;

  /// STT。
  void Function(VoiceSttEvent event)? onStt;

  /// LLM。
  void Function(VoiceLlmEvent event)? onLlm;

  /// TTS。
  void Function(VoiceTtsEvent event)? onTts;

  /// MCP 工具执行状态。
  void Function(VoiceToolEvent event)? onTool;

  /// 服务端错误。
  void Function(VoiceErrorEvent event)? onError;

  /// 唤醒回执。
  void Function(VoiceWakeEvent event)? onWake;

  /// 服务端 MCP JSON-RPC。
  void Function(Map<String, dynamic> payload)? onMcp;

  /// 下行 PCM16 音频。
  void Function(Uint8List pcm)? onAudio;

  /// 麦克风门闸（`true` 可上行，`false` 半双工静音）。
  void Function(bool open)? onMicGateChanged;

  /// 即将重连（attempt 从 1 开始）。
  void Function(int attempt, Duration delay, String reason)? onReconnecting;

  final VoiceWsConfig _config;
  final VoiceTicketFetcher _ticketFetcher;
  final VoiceTransportFactory _transportFactory;

  late final String _clientId;

  VoiceClientState _state = VoiceClientState.idle;
  VoiceServerHello? _serverHello;
  VoiceWsTransport? _transport;
  StreamSubscription<Object>? _messageSub;

  bool _shouldRun = false;
  bool _manualClose = false;
  bool _channelConnected = false;
  bool _helloReceived = false;
  bool _listening = false;
  bool _speaking = false;
  bool _tailHold = false;
  bool _paused = false;
  bool _micGateOpen = false;

  int _generation = 0;
  int _reconnectAttempt = 0;

  Timer? _pingTimer;
  Timer? _pongTimer;
  Timer? _helloTimer;
  Timer? _reconnectTimer;
  Timer? _tailTimer;

  // ── 只读状态 ──────────────────────────────────────────────

  /// 当前状态。
  VoiceClientState get state => _state;

  /// 服务端 hello（握手完成后非空）。
  VoiceServerHello? get serverHello => _serverHello;

  /// 是否已完成握手。
  bool get isReady => _channelConnected && _helloReceived;

  /// 麦克风门闸是否打开。
  bool get micGateOpen => _micGateOpen;

  /// 当前重连尝试次数。
  int get reconnectAttempt => _reconnectAttempt;

  /// 客户端 ID。
  String get clientId => _clientId;

  /// 最近一次连接的 URI（测试与排障用）。
  Uri? get lastUri => _lastUri;
  Uri? _lastUri;

  // ── 连接生命周期 ──────────────────────────────────────────

  /// 建立连接。已连接或正在连接时为空操作。
  Future<void> connect() async {
    if (_state == VoiceClientState.connecting) return;
    if (isReady) return;

    _manualClose = false;
    _shouldRun = true;
    _reconnectAttempt = 0;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    await _openOnce();
  }

  /// 立即重连（重置退避）。
  Future<void> reconnectNow() async {
    _manualClose = false;
    _shouldRun = true;
    _reconnectAttempt = 0;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    await _openOnce();
  }

  /// 主动关闭；不再自动重连。
  Future<void> disconnect() async {
    _manualClose = true;
    _shouldRun = false;
    _reconnectAttempt = 0;
    _reconnectTimer?.cancel();
    _reconnectTimer = null;
    _generation++;
    await _cleanupConnection();
    _setState(VoiceClientState.closed);
  }

  /// 释放资源（等价于 [disconnect]）。
  Future<void> dispose() => disconnect();

  /// 页面切后台时暂停上行（不影响连接）。
  void setPaused(bool value) {
    if (_paused == value) return;
    _paused = value;
    _updateMicGate();
  }

  Future<void> _openOnce() async {
    if (!_shouldRun || _manualClose) return;

    final int generation = ++_generation;
    await _cleanupConnection();
    if (!_shouldRun || generation != _generation) return;

    _setState(VoiceClientState.connecting);

    VoiceWsTicket ticket;
    try {
      ticket = await _ticketFetcher();
    } catch (error) {
      if (generation != _generation) return;
      _scheduleReconnect(_errorMessage(error));
      return;
    }
    if (!_shouldRun || generation != _generation) return;

    final Uri uri;
    try {
      uri = buildVoiceWsUri(
        ticket: ticket.ticket,
        baseUrl: _config.effectiveBaseUrl,
        serverUri:
            _config.serverUri ??
            (ticket.wsUrl.isNotEmpty ? Uri.tryParse(ticket.wsUrl) : null),
      );
    } catch (error) {
      if (generation != _generation) return;
      _scheduleReconnect(_errorMessage(error));
      return;
    }

    _lastUri = uri;
    final VoiceWsTransport transport = _transportFactory();
    _transport = transport;
    _messageSub = transport.messages.listen(
      (Object message) {
        if (generation != _generation) return;
        if (message is String) {
          _handleText(message);
        } else if (message is Uint8List) {
          _handleBinary(message);
        } else if (message is List<int>) {
          _handleBinary(Uint8List.fromList(message));
        }
      },
      onError: (Object error, StackTrace _) {
        if (generation != _generation) return;
        _handleConnectionLost(_errorMessage(error));
      },
      onDone: () {
        if (generation != _generation) return;
        _handleConnectionLost('连接已断开');
      },
    );

    try {
      await transport.connect(uri);
    } catch (error) {
      if (generation != _generation) return;
      await _cleanupConnection();
      if (generation != _generation) return;
      _scheduleReconnect(_errorMessage(error));
      return;
    }
    if (!_shouldRun || generation != _generation) return;

    _channelConnected = true;
    _helloReceived = false;
    _serverHello = null;
    _sendHello();
    _startPing();
    _helloTimer?.cancel();
    _helloTimer = Timer(_config.helloTimeout, () {
      if (generation != _generation) return;
      if (!_helloReceived) {
        _handleConnectionLost('等待服务端握手超时');
      }
    });
    _updateMicGate();
  }

  void _handleConnectionLost(String reason) {
    if (!_shouldRun || _manualClose) return;
    if (_reconnectTimer != null && _reconnectTimer!.isActive) return;
    _generation++;
    unawaited(_cleanupConnection());
    _scheduleReconnect(reason);
  }

  void _scheduleReconnect(String reason) {
    if (!_shouldRun || _manualClose) return;

    _reconnectAttempt++;
    final int maxAttempts = _config.maxReconnectAttempts;
    if (maxAttempts > 0 && _reconnectAttempt > maxAttempts) {
      _setState(VoiceClientState.error);
      onError?.call(
        VoiceErrorEvent(
          code: 'ws_reconnect_failed',
          message: '语音连接失败：$reason',
          retryable: true,
        ),
      );
      return;
    }

    final int exponent = min(_reconnectAttempt - 1, 16);
    final int raw = _config.reconnectBase.inMilliseconds * (1 << exponent);
    final Duration delay = Duration(
      milliseconds: min(raw, _config.reconnectMax.inMilliseconds),
    );

    _setState(VoiceClientState.reconnecting);
    onReconnecting?.call(_reconnectAttempt, delay, reason);

    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(delay, () {
      unawaited(_openOnce());
    });
  }

  Future<void> _cleanupConnection() async {
    _stopPing();
    _helloTimer?.cancel();
    _helloTimer = null;
    _tailTimer?.cancel();
    _tailTimer = null;

    final StreamSubscription<Object>? sub = _messageSub;
    _messageSub = null;
    await sub?.cancel();

    final VoiceWsTransport? transport = _transport;
    _transport = null;
    if (transport != null) {
      try {
        await transport.close();
      } catch (_) {
        // 清理失败忽略。
      }
    }

    _channelConnected = false;
    _helloReceived = false;
    _serverHello = null;
    _listening = false;
    _speaking = false;
    _tailHold = false;
    _updateMicGate();
  }

  // ── 上行消息 ──────────────────────────────────────────────

  /// 开始上行（`listen/start`）。
  void sendListenStart({String mode = 'manual'}) {
    if (!isReady) return;
    _listening = true;
    _sendJson(<String, dynamic>{
      'type': 'listen',
      'state': 'start',
      'mode': mode,
    });
    if (!_speaking && !_tailHold) {
      _setState(VoiceClientState.listening);
    }
    _updateMicGate();
  }

  /// 停止上行（`listen/stop`）。
  void sendListenStop() {
    if (!isReady) return;
    _listening = false;
    _sendJson(<String, dynamic>{'type': 'listen', 'state': 'stop'});
    if (!_speaking) {
      _setState(VoiceClientState.thinking);
    }
    _updateMicGate();
  }

  /// 设备端唤醒命中（`listen/detect`）。
  void sendListenDetect({String? text, String mode = 'auto'}) {
    if (!isReady) return;
    _listening = true;
    _sendJson(<String, dynamic>{
      'type': 'listen',
      'state': 'detect',
      'mode': mode,
      if (text != null && text.isNotEmpty) 'text': text,
    });
    if (!_speaking) {
      _setState(VoiceClientState.listening);
    }
    _updateMicGate();
  }

  /// 打断当前 LLM/TTS（`abort`）。
  void abort({String reason = 'user'}) {
    if (!isReady) return;
    _sendJson(<String, dynamic>{'type': 'abort', 'reason': reason});
    _listening = false;
    _speaking = false;
    _tailHold = false;
    _tailTimer?.cancel();
    _tailTimer = null;
    _setState(VoiceClientState.ready);
    _updateMicGate();
  }

  /// 发送 MCP JSON-RPC。
  void sendMcp(Map<String, dynamic> payload) {
    if (!isReady) return;
    _sendJson(<String, dynamic>{'type': 'mcp', 'payload': payload});
  }

  /// 发送一帧 PCM16 上行音频；门闸关闭时直接丢弃（半双工兜底）。
  void sendAudio(Uint8List pcm) {
    if (pcm.isEmpty || !_micGateOpen) return;
    _transport?.sendBinary(pcm);
  }

  void _sendHello() {
    _sendJson(<String, dynamic>{
      'type': 'hello',
      'version': 1,
      'transport': 'websocket',
      'features': <String, dynamic>{
        'mcp': true,
        'aec': false,
        'wake_word': 'device',
      },
      'audio_params': <String, dynamic>{
        'format': 'pcm16',
        'sample_rate': voiceUpstreamSampleRate,
        'channels': 1,
        'frame_duration': voiceFrameDurationMs,
      },
      'client': <String, dynamic>{
        'kind': _config.clientKind,
        'client_id': _clientId,
        if (_config.deviceId != null && _config.deviceId!.isNotEmpty)
          'device_id': _config.deviceId,
      },
    });
  }

  void _sendJson(Map<String, dynamic> message) {
    final VoiceWsTransport? transport = _transport;
    if (transport == null || !_channelConnected) return;
    transport.sendText(jsonEncode(message));
  }

  // ── 下行消息 ──────────────────────────────────────────────

  void _handleText(String raw) {
    dynamic decoded;
    try {
      decoded = jsonDecode(raw);
    } catch (_) {
      return;
    }
    final Map<String, dynamic> message = _asMap(decoded);
    final String type = _asString(message['type']);

    switch (type) {
      case 'hello':
        _handleHello(message);
        break;
      case 'stt':
        final VoiceSttEvent event = VoiceSttEvent.fromJson(message);
        if (!_speaking) {
          if (event.isFinal) {
            _setState(VoiceClientState.thinking);
          } else if (_listening) {
            _setState(VoiceClientState.listening);
          }
        }
        onStt?.call(event);
        break;
      case 'llm':
        if (!_speaking) _setState(VoiceClientState.thinking);
        onLlm?.call(VoiceLlmEvent.fromJson(message));
        break;
      case 'tts':
        final VoiceTtsEvent event = VoiceTtsEvent.fromJson(message);
        if (event.state == 'start') {
          _beginTts();
        } else if (event.state == 'stop') {
          _endTts();
        }
        onTts?.call(event);
        break;
      case 'tool':
        onTool?.call(VoiceToolEvent.fromJson(message));
        break;
      case 'error':
        onError?.call(VoiceErrorEvent.fromJson(message));
        break;
      case 'wake':
        onWake?.call(VoiceWakeEvent.fromJson(message));
        break;
      case 'mcp':
        onMcp?.call(_asMap(message['payload']));
        break;
      case 'pong':
        _pongTimer?.cancel();
        break;
      case 'ping':
        _sendJson(<String, dynamic>{
          'type': 'pong',
          'ts': _asInt(message['ts'], 0),
        });
        break;
      default:
        break;
    }
  }

  void _handleHello(Map<String, dynamic> message) {
    _helloTimer?.cancel();
    _helloTimer = null;
    _channelConnected = true;
    _helloReceived = true;
    _reconnectAttempt = 0;
    _serverHello = VoiceServerHello.fromJson(message);
    _setState(_listening ? VoiceClientState.listening : VoiceClientState.ready);
    _updateMicGate();
    onServerHello?.call(_serverHello!);
  }

  void _handleBinary(Uint8List pcm) {
    if (pcm.isEmpty) return;
    onAudio?.call(pcm);
  }

  // ── TTS / 半双工 ──────────────────────────────────────────

  void _beginTts() {
    _tailTimer?.cancel();
    _tailTimer = null;
    _tailHold = false;
    _speaking = true;
    _setState(VoiceClientState.speaking);
    _updateMicGate();
  }

  void _endTts() {
    _speaking = false;
    _tailHold = true;
    _setState(_listening ? VoiceClientState.listening : VoiceClientState.ready);
    _updateMicGate();

    _tailTimer?.cancel();
    _tailTimer = Timer(_config.ttsTail, () {
      _tailHold = false;
      _updateMicGate();
    });
  }

  void _updateMicGate() {
    final bool open =
        _channelConnected &&
        _helloReceived &&
        _listening &&
        !_speaking &&
        !_tailHold &&
        !_paused;
    if (open == _micGateOpen) return;
    _micGateOpen = open;
    onMicGateChanged?.call(open);
  }

  // ── 心跳 ──────────────────────────────────────────────────

  void _startPing() {
    _stopPing();
    if (_config.pingInterval <= Duration.zero) return;
    _pingTimer = Timer.periodic(_config.pingInterval, (Timer _) {
      if (!_channelConnected) return;
      _sendJson(<String, dynamic>{
        'type': 'ping',
        'ts': DateTime.now().millisecondsSinceEpoch,
      });
      _pongTimer?.cancel();
      _pongTimer = Timer(_config.pingTimeout, () {
        _handleConnectionLost('心跳超时');
      });
    });
  }

  void _stopPing() {
    _pingTimer?.cancel();
    _pingTimer = null;
    _pongTimer?.cancel();
    _pongTimer = null;
  }

  void _setState(VoiceClientState next) {
    if (_state == next) return;
    _state = next;
    onStateChanged?.call(next);
  }

  static String _newClientId() {
    final Random random = Random.secure();
    final StringBuffer buffer = StringBuffer('app-');
    for (int i = 0; i < 8; i++) {
      buffer.write(random.nextInt(256).toRadixString(16).padLeft(2, '0'));
    }
    buffer
      ..write('-')
      ..write(DateTime.now().millisecondsSinceEpoch);
    return buffer.toString();
  }
}

// ════════════════════════════════════════════════════════════════
// 内部解析工具
// ════════════════════════════════════════════════════════════════

Map<String, dynamic> _asMap(dynamic value) {
  if (value is Map) {
    return value.map(
      (dynamic key, dynamic item) =>
          MapEntry<String, dynamic>(key.toString(), item),
    );
  }
  return <String, dynamic>{};
}

String _asString(dynamic value) {
  if (value is String) return value;
  if (value == null) return '';
  return value.toString();
}

int _asInt(dynamic value, int fallback) {
  if (value is int) return value;
  if (value is num) return value.toInt();
  if (value is String) return int.tryParse(value) ?? fallback;
  return fallback;
}

double _asDouble(dynamic value, double fallback) {
  if (value is double) return value;
  if (value is num) return value.toDouble();
  if (value is String) return double.tryParse(value) ?? fallback;
  return fallback;
}

bool _asBool(dynamic value, bool fallback) {
  if (value is bool) return value;
  if (value is num) return value != 0;
  if (value is String) {
    final String lower = value.toLowerCase();
    if (lower == 'true' || lower == '1') return true;
    if (lower == 'false' || lower == '0') return false;
  }
  return fallback;
}

String _errorMessage(Object error) {
  if (error is VoiceWsException) return error.message;
  return error.toString();
}
