/// MQTT 信令客户端（WebRTC over MQTT）。
///
/// 背景：国标设备的实时画面原本通过 ZLM 的 WHEP（HTTP POST SDP）建立 WebRTC。
/// 当设备/服务组只暴露 MQTT 时（典型场景：设备在 NAT 后，平台取流走 MQTT 转发），
/// App 需要自己把 SDP offer 发到后端、再从 MQTT 收回 answer。
///
/// 协议（与后端约定）：
/// - 订阅 `iot/<group_code>/app/<client_id>/signal`
/// - 发布 `iot/<group_code>/signal/backend`
/// - 消息体：`{"type":"offer|answer|error","request_id":"...","client_id":"...",
///   "channel_id":123,"stream":"...","sdp":"..."}`
///
/// **本文件只负责信令**（消息编解码、request_id 匹配、超时），WebRTC 协商放在
/// [MqttWebRtcClient] 中。这样信令可以用假传输层做单元测试，不依赖真机与插件。
///
/// **为什么 MQTT 失败要回退 HTTP WHEP**：MQTT 只是「信令的另一种搬运方式」，
/// 媒体面仍是 WebRTC。失败原因通常与媒体无关（broker 不可达、服务组凭据过期、
/// 后端未部署 MQTT 转发）。此时设备仍可能在同一媒体节点上提供 WHEP 地址，
/// 回退 WHEP 可以避免用户因为一条信令链路故障而完全看不到画面。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:mqtt_client/mqtt_client.dart';
import 'package:mqtt_client/mqtt_server_client.dart';

import 'IotAppController.dart';

// ════════════════════════════════════════════════════════════════
// 配置
// ════════════════════════════════════════════════════════════════

/// MQTT 信令连接配置。
class MqttSignalingConfig {
  /// 构造配置。
  const MqttSignalingConfig({
    required this.groupCode,
    required this.host,
    required this.port,
    required this.clientId,
    this.username = '',
    this.password = '',
    this.useTls = false,
    this.topicPrefix = '',
  });

  /// 从设备的服务组推导配置。
  ///
  /// 服务组里必须存在 MQTT 服务项；缺失时抛出 [MqttSignalingException]，
  /// 由调用方回退 WHEP（不要静默使用错误端口）。
  factory MqttSignalingConfig.fromServiceGroup(
    IotServiceGroup group, {
    required String clientId,
  }) {
    final String code = group.code.trim();
    if (code.isEmpty) {
      throw MqttSignalingException('服务组缺少编码（group_code），无法构造 MQTT topic');
    }
    final IotServiceItem? item = group.mqttItem;
    if (item == null) {
      throw MqttSignalingException('服务组「${group.name.isNotEmpty ? group.name : code}」未配置 MQTT 服务项');
    }
    if (item.host.trim().isEmpty) {
      throw MqttSignalingException('MQTT 服务项缺少主机地址');
    }
    final dynamic tlsRaw = item.extra['tls'];
    return MqttSignalingConfig(
      groupCode: code,
      host: item.host.trim(),
      // 后端约定 `port=0` 时按协议取默认端口（MQTT 1883）。
      port: item.port > 0 ? item.port : 1883,
      clientId: clientId,
      username: group.username,
      password: group.password,
      useTls: tlsRaw is bool ? tlsRaw : false,
    );
  }

  /// 从 `/app/iot/devices/:id/services` 的响应构造。
  ///
  /// 优先使用后端下发的 `mqtt.url` / `mqtt.topic_prefix`：
  /// topic 前缀是 MQTT ACL 的隔离边界，客户端自己拼容易与服务端不一致。
  /// 后端未下发时回退到服务组里的 MQTT 服务项。
  factory MqttSignalingConfig.fromCredentials(
    IotCredentials credentials, {
    required String clientId,
  }) {
    final IotServiceGroup? group = credentials.mqttGroup;
    if (group == null) {
      throw MqttSignalingException('设备所属产品未授权任何服务组');
    }
    final IotServiceItem? item = group.mqttItem;
    final IotMqttInfo? mqtt = credentials.mqtt;
    if (mqtt == null && item == null) {
      throw MqttSignalingException('服务组「${group.name.isNotEmpty ? group.name : group.code}」未配置 MQTT 服务项');
    }

    String host = '';
    int port = 1883;
    bool useTls = false;
    if (mqtt != null && mqtt.url.trim().isNotEmpty) {
      final ({String host, int port, bool tls}) broker =
          _parseBrokerUrl(mqtt.url);
      host = broker.host;
      port = broker.port;
      useTls = broker.tls;
    }
    if (host.isEmpty) {
      if (item == null || item.host.trim().isEmpty) {
        throw MqttSignalingException('MQTT 服务项缺少主机地址');
      }
      host = item.host.trim();
      port = item.port > 0 ? item.port : 1883;
      final dynamic tlsRaw = item.extra['tls'];
      useTls = tlsRaw is bool ? tlsRaw : false;
    }

    final String mqttPrefix = mqtt?.topicPrefix ?? '';
    return MqttSignalingConfig(
      groupCode: group.code,
      topicPrefix: mqttPrefix.isNotEmpty ? mqttPrefix : 'iot/${group.code}',
      host: host,
      port: port,
      clientId: clientId,
      username: group.username,
      password: group.password,
      useTls: useTls,
    );
  }

  /// 解析 `mqtt://host:1883` / `mqtts://host` / 裸 `host:port`。
  static ({String host, int port, bool tls}) _parseBrokerUrl(String raw) {
    final String url = raw.trim();
    final Uri? uri = Uri.tryParse(url);
    if (uri != null && uri.host.isNotEmpty) {
      final String scheme = uri.scheme.toLowerCase();
      final bool tls =
          scheme == 'mqtts' || scheme == 'ssl' || scheme == 'tls';
      return (
        host: uri.host,
        port: uri.hasPort ? uri.port : (tls ? 8883 : 1883),
        tls: tls,
      );
    }
    // 容错：不带 scheme 的 host:port。
    final String text = url.contains('://') ? url.split('://').last : url;
    final List<String> parts = text.split(':');
    return (
      host: parts.first,
      port: parts.length > 1 ? int.tryParse(parts[1]) ?? 1883 : 1883,
      tls: false,
    );
  }

  /// 服务组编码（topic 中的 `<group_code>`）。
  final String groupCode;

  /// Broker 主机（不含 scheme）。
  final String host;

  /// Broker 端口。
  final int port;

  /// 客户端标识（由 App 生成，同时作为订阅 topic 的一段）。
  final String clientId;

  /// 服务组用户名。
  final String username;

  /// 服务组密码。
  final String password;

  /// 是否使用 TLS（`mqtts://`）。
  final bool useTls;

  /// 主题前缀。
  ///
  /// 后端 `/app/iot/devices/:id/services` 会下发 `mqtt.topic_prefix`
  /// （约定为 `iot/<group_code>`）；为空时按 groupCode 推导。
  final String topicPrefix;

  /// 实际使用的主题前缀。
  String get effectiveTopicPrefix =>
      topicPrefix.isNotEmpty ? topicPrefix : 'iot/$groupCode';

  /// App 订阅的 topic。
  String get subscribeTopic => '$effectiveTopicPrefix/app/$clientId/signal';

  /// App 发布的 topic。
  String get publishTopic => '$effectiveTopicPrefix/signal/backend';
}

// ════════════════════════════════════════════════════════════════
// 传输层抽象
// ════════════════════════════════════════════════════════════════

/// MQTT 传输层接口。
///
/// 抽出来是为了单元测试：测试用假实现直接推消息，无需启动 broker。
abstract class MqttSignalingTransport {
  /// 建立连接。
  Future<void> connect();

  /// 订阅 topic。
  Future<void> subscribe(String topic);

  /// 发布文本消息。
  Future<void> publish(String topic, String payload);

  /// 收到的文本消息流。
  Stream<String> get messages;

  /// 断开连接并释放资源。
  Future<void> disconnect();
}

/// 基于 `mqtt_client` 的真实传输层。
class MqttClientTransport implements MqttSignalingTransport {
  /// 构造传输层。
  MqttClientTransport(this.config);

  /// 连接配置。
  final MqttSignalingConfig config;

  final StreamController<String> _messages =
      StreamController<String>.broadcast();

  MqttServerClient? _client;
  StreamSubscription<List<MqttReceivedMessage<MqttMessage>>>? _updates;

  @override
  Stream<String> get messages => _messages.stream;

  @override
  Future<void> connect() async {
    if (_client != null) return;

    final MqttServerClient client = MqttServerClient.withPort(
      config.host,
      config.clientId,
      config.port,
    )
      ..logging(on: false)
      // 断线后不自动重连：一次播放会话对应一条连接，重连由页面「重试」触发，
      // 避免半死连接与旧 PeerConnection 混用。
      ..autoReconnect = false
      ..secure = config.useTls
      ..keepAlivePeriod = 30
      ..connectTimeoutPeriod = 10000;
    _client = client;

    try {
      await client.connect(
        config.username.isEmpty ? null : config.username,
        config.password.isEmpty ? null : config.password,
      );
    } catch (e) {
      await disconnect();
      throw MqttSignalingException('无法连接 MQTT Broker：$e');
    }

    if (client.connectionStatus?.state != MqttConnectionState.connected) {
      final String code = client.connectionStatus?.returnCode?.toString() ?? '未知错误';
      await disconnect();
      throw MqttSignalingException('MQTT 登录失败：$code（请检查服务组凭据）');
    }

    final Stream<List<MqttReceivedMessage<MqttMessage>>>? updates =
        client.updates;
    if (updates == null) {
      await disconnect();
      throw MqttSignalingException('MQTT 客户端未就绪（订阅流为空）');
    }
    _updates = updates.listen(
      (List<MqttReceivedMessage<MqttMessage>> batch) {
        for (final MqttReceivedMessage<MqttMessage> received in batch) {
          final MqttMessage payload = received.payload;
          if (payload is MqttPublishMessage) {
            _messages.add(
              utf8.decode(payload.payload.message, allowMalformed: true),
            );
          }
        }
      },
    );
  }

  @override
  Future<void> subscribe(String topic) async {
    final MqttServerClient? client = _client;
    if (client == null) {
      throw MqttSignalingException('MQTT 未连接，无法订阅 $topic');
    }
    final Subscription? subscription = client.subscribe(
      topic,
      MqttQos.atLeastOnce,
    );
    if (subscription == null) {
      throw MqttSignalingException('MQTT 订阅失败：$topic');
    }
  }

  @override
  Future<void> publish(String topic, String payload) async {
    final MqttServerClient? client = _client;
    if (client == null) {
      throw MqttSignalingException('MQTT 未连接，无法发布到 $topic');
    }
    final MqttClientPayloadBuilder builder = MqttClientPayloadBuilder()
      ..addString(payload);
    final payloadBuffer = builder.payload;
    if (payloadBuffer == null) {
      throw MqttSignalingException('MQTT 消息编码失败');
    }
    client.publishMessage(topic, MqttQos.atLeastOnce, payloadBuffer);
  }

  @override
  Future<void> disconnect() async {
    final StreamSubscription<List<MqttReceivedMessage<MqttMessage>>>? updates =
        _updates;
    _updates = null;
    await updates?.cancel();

    final MqttServerClient? client = _client;
    _client = null;
    if (client != null) {
      try {
        client.disconnect();
      } catch (_) {
        // 已断开。
      }
    }
    if (!_messages.isClosed) {
      await _messages.close();
    }
  }
}

// ════════════════════════════════════════════════════════════════
// 信令客户端
// ════════════════════════════════════════════════════════════════

/// MQTT 信令客户端。
///
/// 一个实例对应一次播放会话；切换通道 / 离开页面必须 [close]。
class MqttSignalingClient {
  /// 构造客户端。
  ///
  /// [transport] 为空时使用真实 `mqtt_client`；测试可注入假实现。
  MqttSignalingClient({
    required this.config,
    MqttSignalingTransport? transport,
    this.answerTimeout = const Duration(seconds: 10),
    String Function()? requestIdFactory,
  })  : _transport = transport ?? MqttClientTransport(config),
        _requestIdFactory = requestIdFactory ?? newRequestId;

  /// 连接配置。
  final MqttSignalingConfig config;

  /// 等待 answer 的超时；超时即判定信令失败，调用方回退 WHEP。
  final Duration answerTimeout;

  final MqttSignalingTransport _transport;
  final String Function() _requestIdFactory;

  /// request_id → 等待中的 answer。
  final Map<String, Completer<String>> _pending = <String, Completer<String>>{};

  StreamSubscription<String>? _subscription;
  bool _connected = false;

  /// 是否已连接并完成订阅。
  bool get isConnected => _connected;

  /// 当前等待 answer 的请求数（测试与诊断用）。
  int get pendingCount => _pending.length;

  /// 连接 Broker 并订阅本方 topic。
  Future<void> connect() async {
    if (_connected) return;
    await _transport.connect();
    _subscription = _transport.messages.listen(_onPayload);
    try {
      await _transport.subscribe(config.subscribeTopic);
    } catch (e) {
      await close();
      rethrow;
    }
    _connected = true;
  }

  /// 发送 SDP offer 并等待同一 `request_id` 的 answer。
  ///
  /// 返回 answer SDP；超时 / 后端返回 error 时抛出 [MqttSignalingException]。
  Future<String> requestAnswer({
    required String offerSdp,
    required int channelId,
    required String stream,
  }) async {
    if (!_connected) {
      throw MqttSignalingException('MQTT 信令未连接');
    }
    if (offerSdp.isEmpty) {
      throw MqttSignalingException('本地 SDP offer 为空');
    }

    final String requestId = _requestIdFactory();
    final Completer<String> completer = Completer<String>();
    _pending[requestId] = completer;
    // 超时后 answer 仍可能到达；先挂一个「吞掉结果」的监听，
    // 避免 Dart 把它当作未处理异常打到 Zone。
    unawaited(completer.future.then<void>((_) {}, onError: (Object _) {}));

    try {
      final Map<String, dynamic> message = buildOfferMessage(
        requestId: requestId,
        clientId: config.clientId,
        channelId: channelId,
        stream: stream,
        sdp: offerSdp,
      );
      await _transport.publish(config.publishTopic, jsonEncode(message));
      return await completer.future.timeout(
        answerTimeout,
        onTimeout: () => throw MqttSignalingException(
          'MQTT 信令超时：${answerTimeout.inSeconds} 秒内未收到 answer',
        ),
      );
    } finally {
      _pending.remove(requestId);
    }
  }

  /// 构造 offer 消息体（静态方法便于单测逐字段断言）。
  static Map<String, dynamic> buildOfferMessage({
    required String requestId,
    required String clientId,
    required int channelId,
    required String stream,
    required String sdp,
  }) {
    return <String, dynamic>{
      'type': 'offer',
      'request_id': requestId,
      'client_id': clientId,
      'channel_id': channelId,
      'stream': stream,
      'sdp': sdp,
    };
  }

  /// 生成新的 request_id（16 字节随机数的十六进制串）。
  static String newRequestId() => _randomHex(16);

  /// 生成客户端标识。只允许字母数字与下划线，避免 MQTT topic 含非法字符。
  static String newClientId() => 'app_${_randomHex(8)}';

  /// 消费一条 MQTT 文本消息。
  void _onPayload(String payload) {
    final Map<String, dynamic> message = _decode(payload);
    if (message.isEmpty) return;

    final String type = (message['type'] ?? '').toString();
    final String requestId = (message['request_id'] ?? '').toString();
    if (requestId.isEmpty) return;

    final Completer<String>? completer = _pending[requestId];
    // request_id 不匹配的消息（其它通道 / 其它客户端 / 迟到消息）一律忽略，
    // 否则并发播放两个通道时会把 A 的 answer 交给 B。
    if (completer == null || completer.isCompleted) return;

    if (type == 'answer') {
      final String sdp = (message['sdp'] ?? '').toString();
      if (sdp.isEmpty) {
        completer.completeError(
          MqttSignalingException('MQTT 返回的 answer 为空'),
        );
        return;
      }
      completer.complete(sdp);
      return;
    }

    if (type == 'error') {
      final String code = (message['code'] ?? '').toString();
      final String text = (message['message'] ?? '').toString();
      final String detail = <String>[
        if (code.isNotEmpty) code,
        if (text.isNotEmpty) text,
      ].join('：');
      completer.completeError(
        MqttSignalingException(
          detail.isEmpty ? 'MQTT 信令返回未知错误' : 'MQTT 信令失败：$detail',
        ),
      );
    }
  }

  /// 解析 JSON 对象；非对象 / 非法 JSON 返回空 Map。
  Map<String, dynamic> _decode(String payload) {
    try {
      final dynamic decoded = jsonDecode(payload);
      if (decoded is Map) {
        return decoded.map(
          (dynamic key, dynamic value) =>
              MapEntry<String, dynamic>(key.toString(), value),
        );
      }
    } catch (_) {
      // 非法 JSON：忽略，避免一条坏消息打断整条会话。
    }
    return <String, dynamic>{};
  }

  /// 断开连接；未完成的请求以异常结束，避免页面一直转圈。
  Future<void> close() async {
    _connected = false;
    final StreamSubscription<String>? subscription = _subscription;
    _subscription = null;
    await subscription?.cancel();

    for (final Completer<String> completer in _pending.values) {
      if (!completer.isCompleted) {
        completer.completeError(MqttSignalingException('MQTT 信令已关闭'));
      }
    }
    _pending.clear();
    await _transport.disconnect();
  }

  /// 生成 n 字节随机数的十六进制表示。
  static String _randomHex(int bytes) {
    final Random random = Random.secure();
    final StringBuffer buffer = StringBuffer();
    for (int i = 0; i < bytes; i++) {
      buffer.write(random.nextInt(256).toRadixString(16).padLeft(2, '0'));
    }
    return buffer.toString();
  }
}

// ════════════════════════════════════════════════════════════════
// WebRTC 协商
// ════════════════════════════════════════════════════════════════

/// 通过 MQTT 信令建立 WebRTC 收流，并渲染到 [RTCVideoRenderer]。
///
/// 流程与 WHEP 客户端一致，只是 SDP 交换从 HTTP POST 换成 MQTT：
/// 1. 连接 MQTT 并订阅本方 topic；
/// 2. `addTransceiver(video/audio, recvonly)`；
/// 3. `createOffer` → `setLocalDescription` → 等 ICE gathering；
/// 4. 把本地 SDP 作为 offer 发布，等待同一 request_id 的 answer；
/// 5. `setRemoteDescription(answer)`，等 `onTrack` 收到视频轨才算成功。
class MqttWebRtcClient {
  /// 构造客户端。
  MqttWebRtcClient({
    this.answerTimeout = const Duration(seconds: 10),
    this.connectTimeout = const Duration(seconds: 10),
    MqttSignalingTransport Function(MqttSignalingConfig config)?
        transportFactory,
  }) : _transportFactory = transportFactory;

  /// 等待 answer 的超时。
  final Duration answerTimeout;

  /// 等待首条视频轨的超时。
  final Duration connectTimeout;

  final MqttSignalingTransport Function(MqttSignalingConfig config)?
      _transportFactory;

  RTCPeerConnection? _pc;
  MediaStream? _stream;
  RTCVideoRenderer? _renderer;
  MqttSignalingClient? _signaling;
  bool _closed = false;

  /// 是否已释放。
  bool get isClosed => _closed;

  /// 建立连接并渲染远端视频。
  Future<void> connect({
    required MqttSignalingConfig config,
    required RTCVideoRenderer renderer,
    required int channelId,
    required String stream,
    required List<IceServerConfig> iceServers,
  }) async {
    _renderer = renderer;
    _closed = false;

    final RTCPeerConnection pc = await createPeerConnection(
      <String, dynamic>{
        // ICE 配置来自 `/app/rtc/ice-servers`；为空表示直连（局域网场景）。
        'iceServers': iceServers
            .where((IceServerConfig item) => !item.isEmpty)
            .map((IceServerConfig item) => item.toRtcIceServer())
            .toList(growable: false),
        'sdpSemantics': 'unified-plan',
      },
    );
    _pc = pc;

    final Completer<MediaStream> videoReady = Completer<MediaStream>();
    unawaited(videoReady.future.then<void>((_) {}, onError: (Object _) {}));

    pc.onTrack = (RTCTrackEvent event) {
      if (event.track.kind != 'video' || videoReady.isCompleted) return;
      final List<MediaStream> streams = event.streams;
      if (streams.isNotEmpty) {
        videoReady.complete(streams.first);
        return;
      }
      unawaited(() async {
        try {
          final MediaStream local = await createLocalMediaStream('iot-remote');
          await local.addTrack(event.track);
          if (!videoReady.isCompleted) videoReady.complete(local);
        } catch (e) {
          if (!videoReady.isCompleted) {
            videoReady.completeError(
              MqttSignalingException('WebRTC 远端轨道挂载失败：$e'),
            );
          }
        }
      }());
    };

    pc.onConnectionState = (RTCPeerConnectionState state) {
      if (state == RTCPeerConnectionState.RTCPeerConnectionStateFailed &&
          !videoReady.isCompleted) {
        videoReady.completeError(
          MqttSignalingException('WebRTC 连接失败（ICE/DTLS 未建立）'),
        );
      }
    };

    final MqttSignalingClient signaling = MqttSignalingClient(
      config: config,
      transport: _transportFactory?.call(config),
      answerTimeout: answerTimeout,
    );
    _signaling = signaling;

    try {
      await signaling.connect();
      if (_closed) throw MqttSignalingException('MQTT 信令连接已取消');

      await pc.addTransceiver(
        kind: RTCRtpMediaType.RTCRtpMediaTypeVideo,
        init: RTCRtpTransceiverInit(direction: TransceiverDirection.RecvOnly),
      );
      await pc.addTransceiver(
        kind: RTCRtpMediaType.RTCRtpMediaTypeAudio,
        init: RTCRtpTransceiverInit(direction: TransceiverDirection.RecvOnly),
      );

      final RTCSessionDescription offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      await _waitIceGathering(pc, const Duration(seconds: 3));

      final RTCSessionDescription? local = await pc.getLocalDescription();
      final String offerSdp = local?.sdp ?? offer.sdp ?? '';
      if (offerSdp.isEmpty) {
        throw MqttSignalingException('WebRTC 本地 SDP 生成失败');
      }

      final String answerSdp = await signaling.requestAnswer(
        offerSdp: offerSdp,
        channelId: channelId,
        stream: stream,
      );
      if (_closed) throw MqttSignalingException('MQTT 信令连接已取消');

      await pc.setRemoteDescription(
        RTCSessionDescription(answerSdp, 'answer'),
      );

      final MediaStream remote = await videoReady.future.timeout(
        connectTimeout,
        onTimeout: () => throw MqttSignalingException(
          'WebRTC 超时：${connectTimeout.inSeconds} 秒内未收到视频轨道'
          '（常见原因：设备输出 H.265，App 端 WebRTC 不支持）',
        ),
      );
      if (_closed) throw MqttSignalingException('MQTT 信令连接已取消');

      _stream = remote;
      renderer.srcObject = remote;
      debugPrint(
        '[IOT/MQTT] connected: channel=$channelId stream=$stream '
        'topic=${config.subscribeTopic}',
      );
    } catch (e) {
      await close();
      rethrow;
    }
  }

  /// 等 ICE gathering 完成；超时后用已收集的候选继续。
  Future<void> _waitIceGathering(
    RTCPeerConnection pc,
    Duration timeout,
  ) async {
    if (pc.iceGatheringState ==
        RTCIceGatheringState.RTCIceGatheringStateComplete) {
      return;
    }
    final Completer<void> complete = Completer<void>();
    final Function(RTCIceGatheringState)? previous = pc.onIceGatheringState;
    pc.onIceGatheringState = (RTCIceGatheringState state) {
      previous?.call(state);
      if (state == RTCIceGatheringState.RTCIceGatheringStateComplete &&
          !complete.isCompleted) {
        complete.complete();
      }
    };
    try {
      await complete.future.timeout(timeout);
    } on TimeoutException {
      debugPrint('[IOT/MQTT] ICE gathering 超时，使用已收集的候选继续');
    } finally {
      pc.onIceGatheringState = previous;
    }
  }

  /// 释放 PeerConnection / 远端流 / MQTT 连接。
  Future<void> close() async {
    if (_closed) return;
    _closed = true;

    final MqttSignalingClient? signaling = _signaling;
    _signaling = null;
    if (signaling != null) {
      try {
        await signaling.close();
      } catch (_) {
        // 已断开。
      }
    }

    final RTCVideoRenderer? renderer = _renderer;
    _renderer = null;
    if (renderer != null) {
      try {
        renderer.srcObject = null;
      } catch (_) {
        // renderer 可能已被页面释放。
      }
    }

    final MediaStream? stream = _stream;
    _stream = null;
    if (stream != null) {
      for (final MediaStreamTrack track in stream.getTracks()) {
        try {
          await track.stop();
        } catch (_) {
          // 轨道可能已随 PeerConnection 关闭。
        }
      }
      try {
        await stream.dispose();
      } catch (_) {
        // 同上。
      }
    }

    final RTCPeerConnection? pc = _pc;
    _pc = null;
    if (pc != null) {
      pc.onTrack = null;
      pc.onConnectionState = null;
      pc.onIceGatheringState = null;
      try {
        await pc.close();
      } catch (_) {
        // 已关闭。
      }
      try {
        await pc.dispose();
      } catch (_) {
        // 部分平台 dispose 会重复释放。
      }
    }
  }
}

/// MQTT 信令异常（文案可直接展示给用户）。
class MqttSignalingException implements Exception {
  /// 构造异常。
  MqttSignalingException(this.message);

  /// 中文错误信息。
  final String message;

  @override
  String toString() => message;
}
