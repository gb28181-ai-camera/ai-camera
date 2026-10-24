// MQTT 信令：topic 构造、offer 消息、request_id 匹配、错误与超时分支。
import 'dart:async';
import 'dart:convert';

import 'package:ai_camera_app/services/IotAppController.dart';
import 'package:ai_camera_app/services/MqttSignalingClient.dart';
import 'package:flutter_test/flutter_test.dart';

/// 假传输层：记录发布/订阅，并可手动注入下行消息。
class FakeTransport implements MqttSignalingTransport {
  final StreamController<String> _controller =
      StreamController<String>.broadcast();

  final List<String> subscribed = <String>[];
  final List<({String topic, String payload})> published =
      <({String topic, String payload})>[];

  /// 发布时的同步回调（用于模拟后端立即回 answer）。
  void Function(String payload)? onPublish;

  bool connected = false;
  bool connectFails = false;
  bool disconnected = false;

  @override
  Stream<String> get messages => _controller.stream;

  @override
  Future<void> connect() async {
    if (connectFails) {
      throw MqttSignalingException('broker 不可达');
    }
    connected = true;
  }

  @override
  Future<void> subscribe(String topic) async {
    subscribed.add(topic);
  }

  @override
  Future<void> publish(String topic, String payload) async {
    published.add((topic: topic, payload: payload));
    onPublish?.call(payload);
  }

  @override
  Future<void> disconnect() async {
    disconnected = true;
    await _controller.close();
  }

  /// 注入一条下行消息。
  void emit(String payload) => _controller.add(payload);
}

/// 测试用连接配置。
MqttSignalingConfig config({String groupCode = 'grp1'}) {
  return MqttSignalingConfig(
    groupCode: groupCode,
    host: 'broker.local',
    port: 1883,
    clientId: 'app_test',
    username: 'iot_user',
    password: 'iot_pass',
  );
}

/// 解析 JSON 对象（测试辅助）。
Map<String, dynamic> decode(String raw) {
  final dynamic value = jsonDecode(raw);
  if (value is Map) {
    return value.map(
      (dynamic key, dynamic value) =>
          MapEntry<String, dynamic>(key.toString(), value),
    );
  }
  return <String, dynamic>{};
}

void main() {
  group('MqttSignalingConfig', () {
    test('topic 与协议约定一致', () {
      final MqttSignalingConfig cfg = config();
      expect(cfg.subscribeTopic, 'iot/grp1/app/app_test/signal');
      expect(cfg.publishTopic, 'iot/grp1/signal/backend');
    });

    test('从服务组取 MQTT 服务项，端口 0 回落到 1883', () {
      const IotServiceGroup group = IotServiceGroup(
        id: 1,
        code: 'grp9',
        name: '视频组',
        type: 'webrtc',
        username: 'u9',
        password: 'p9',
        items: <IotServiceItem>[
          IotServiceItem(
            protocol: 'WebRTC',
            host: 'media.local',
            port: 443,
            path: '',
            extra: <String, dynamic>{},
          ),
          IotServiceItem(
            protocol: 'MQTT',
            host: '192.168.1.10',
            port: 0,
            path: '',
            extra: <String, dynamic>{},
          ),
        ],
      );

      final MqttSignalingConfig cfg = MqttSignalingConfig.fromServiceGroup(
        group,
        clientId: 'app_x',
      );
      expect(cfg.groupCode, 'grp9');
      expect(cfg.host, '192.168.1.10');
      expect(cfg.port, 1883);
      expect(cfg.username, 'u9');
      expect(cfg.password, 'p9');
      expect(cfg.subscribeTopic, 'iot/grp9/app/app_x/signal');
    });

    test('服务组没有 MQTT 服务项时抛出可读异常', () {
      const IotServiceGroup group = IotServiceGroup(
        id: 2,
        code: 'grp2',
        name: '仅 WebRTC',
        type: 'webrtc',
        username: 'u',
        password: 'p',
        items: <IotServiceItem>[
          IotServiceItem(
            protocol: 'WebRTC',
            host: 'media.local',
            port: 443,
            path: '',
            extra: <String, dynamic>{},
          ),
        ],
      );

      expect(
        () => MqttSignalingConfig.fromServiceGroup(group, clientId: 'c'),
        throwsA(
          isA<MqttSignalingException>().having(
            (MqttSignalingException e) => e.message,
            'message',
            contains('未配置 MQTT'),
          ),
        ),
      );
    });

    test('从 /services 响应构造：优先使用 mqtt.url 与 topic_prefix', () {
      const IotCredentials credentials = IotCredentials(
        productName: '户外球机',
        productCode: 'P-OD-01',
        groups: <IotServiceGroup>[
          IotServiceGroup(
            id: 1,
            code: 'grp-9',
            name: '视频服务',
            type: 'webrtc',
            username: 'iot_u',
            password: 'iot_p',
            items: <IotServiceItem>[
              IotServiceItem(
                protocol: 'MQTT',
                host: '192.168.1.10',
                port: 1883,
                path: '',
                extra: <String, dynamic>{},
              ),
            ],
          ),
        ],
        mqtt: IotMqttInfo(
          url: 'mqtt://broker.iot:1884',
          topicPrefix: 'iot/grp-9',
        ),
        turn: null,
      );

      final MqttSignalingConfig cfg = MqttSignalingConfig.fromCredentials(
        credentials,
        clientId: 'app_x',
      );

      expect(cfg.host, 'broker.iot');
      expect(cfg.port, 1884);
      expect(cfg.username, 'iot_u');
      expect(cfg.password, 'iot_p');
      expect(cfg.subscribeTopic, 'iot/grp-9/app/app_x/signal');
      expect(cfg.publishTopic, 'iot/grp-9/signal/backend');
    });

    test('mqtts 地址使用 TLS 与 8883 默认端口', () {
      const IotCredentials credentials = IotCredentials(
        productName: '',
        productCode: '',
        groups: <IotServiceGroup>[
          IotServiceGroup(
            id: 1,
            code: 'grp-1',
            name: '',
            type: 'webrtc',
            username: 'u',
            password: 'p',
            items: <IotServiceItem>[],
          ),
        ],
        mqtt: IotMqttInfo(url: 'mqtts://broker.iot', topicPrefix: ''),
        turn: null,
      );

      final MqttSignalingConfig cfg = MqttSignalingConfig.fromCredentials(
        credentials,
        clientId: 'c',
      );
      expect(cfg.useTls, isTrue);
      expect(cfg.port, 8883);
      // 后端没给 topic_prefix 时按 group_code 推导。
      expect(cfg.subscribeTopic, 'iot/grp-1/app/c/signal');
    });

    test('后端未下发 mqtt 时回退到服务项', () {
      const IotCredentials credentials = IotCredentials(
        productName: '',
        productCode: '',
        groups: <IotServiceGroup>[
          IotServiceGroup(
            id: 1,
            code: 'grp-2',
            name: '视频组',
            type: 'webrtc',
            username: 'u2',
            password: 'p2',
            items: <IotServiceItem>[
              IotServiceItem(
                protocol: 'MQTT',
                host: '10.0.0.2',
                port: 0,
                path: '',
                extra: <String, dynamic>{},
              ),
            ],
          ),
        ],
        mqtt: null,
        turn: null,
      );

      final MqttSignalingConfig cfg = MqttSignalingConfig.fromCredentials(
        credentials,
        clientId: 'c',
      );
      expect(cfg.host, '10.0.0.2');
      expect(cfg.port, 1883);
      expect(cfg.groupCode, 'grp-2');
    });

    test('没有服务组或没有 MQTT 服务项时拒绝连接', () {
      const IotCredentials noGroups = IotCredentials(
        productName: '',
        productCode: '',
        groups: <IotServiceGroup>[],
        mqtt: null,
        turn: null,
      );
      expect(
        () => MqttSignalingConfig.fromCredentials(noGroups, clientId: 'c'),
        throwsA(isA<MqttSignalingException>()),
      );

      const IotCredentials noMqtt = IotCredentials(
        productName: '',
        productCode: '',
        groups: <IotServiceGroup>[
          IotServiceGroup(
            id: 1,
            code: 'grp-3',
            name: '仅国标',
            type: 'gb28181',
            username: 'u',
            password: 'p',
            items: <IotServiceItem>[
              IotServiceItem(
                protocol: 'SIP',
                host: 'sip.local',
                port: 5060,
                path: '',
                extra: <String, dynamic>{},
              ),
            ],
          ),
        ],
        mqtt: null,
        turn: null,
      );
      expect(
        () => MqttSignalingConfig.fromCredentials(noMqtt, clientId: 'c'),
        throwsA(isA<MqttSignalingException>()),
      );
    });

    test('服务组缺少编码时拒绝连接', () {
      const IotServiceGroup group = IotServiceGroup(
        id: 3,
        code: '',
        name: '空编码',
        type: 'webrtc',
        username: '',
        password: '',
        items: <IotServiceItem>[
          IotServiceItem(
            protocol: 'MQTT',
            host: 'broker',
            port: 1883,
            path: '',
            extra: <String, dynamic>{},
          ),
        ],
      );

      expect(
        () => MqttSignalingConfig.fromServiceGroup(group, clientId: 'c'),
        throwsA(isA<MqttSignalingException>()),
      );
    });
  });

  group('offer 消息构造', () {
    test('字段名与后端约定一致', () {
      final Map<String, dynamic> message =
          MqttSignalingClient.buildOfferMessage(
        requestId: 'rid-1',
        clientId: 'app_test',
        channelId: 123,
        stream: '0BEBC201',
        sdp: 'v=0 offer',
      );

      expect(message, <String, dynamic>{
        'type': 'offer',
        'request_id': 'rid-1',
        'client_id': 'app_test',
        'channel_id': 123,
        'stream': '0BEBC201',
        'sdp': 'v=0 offer',
      });
    });

    test('request_id / client_id 唯一且只含安全字符', () {
      final Set<String> ids = <String>{};
      for (int i = 0; i < 50; i++) {
        ids.add(MqttSignalingClient.newRequestId());
      }
      expect(ids, hasLength(50));
      expect(
        RegExp(r'^[0-9a-f]{32}$').hasMatch(ids.first),
        isTrue,
      );

      final String clientId = MqttSignalingClient.newClientId();
      expect(clientId.startsWith('app_'), isTrue);
      expect(RegExp(r'^app_[0-9a-f]{16}$').hasMatch(clientId), isTrue);
    });
  });

  group('MqttSignalingClient', () {
    test('connect 后订阅本方 topic', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
      );

      await client.connect();

      expect(transport.connected, isTrue);
      expect(transport.subscribed, <String>['iot/grp1/app/app_test/signal']);
      expect(client.isConnected, isTrue);
      await client.close();
    });

    test('offer 发布到 backend topic，answer 按 request_id 匹配', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
      );
      transport.onPublish = (String payload) {
        final Map<String, dynamic> offer = decode(payload);
        expect(offer['type'], 'offer');
        expect(offer['channel_id'], 12);
        expect(offer['stream'], '0BEBC201');
        transport.emit(
          jsonEncode(<String, dynamic>{
            'type': 'answer',
            'request_id': offer['request_id'],
            'sdp': 'v=0 answer',
          }),
        );
      };

      await client.connect();
      final String answer = await client.requestAnswer(
        offerSdp: 'v=0 offer',
        channelId: 12,
        stream: '0BEBC201',
      );

      expect(answer, 'v=0 answer');
      expect(transport.published, hasLength(1));
      expect(
        transport.published.single.topic,
        'iot/grp1/signal/backend',
      );
      expect(client.pendingCount, 0);
      await client.close();
    });

    test('request_id 不匹配的 answer 被忽略', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
      );
      transport.onPublish = (String payload) {
        final Map<String, dynamic> offer = decode(payload);
        // 先来一条别的通道/客户端的迟到消息。
        transport.emit(
          jsonEncode(<String, dynamic>{
            'type': 'answer',
            'request_id': 'other-request',
            'sdp': 'v=0 wrong',
          }),
        );
        transport.emit(
          jsonEncode(<String, dynamic>{
            'type': 'answer',
            'request_id': offer['request_id'],
            'sdp': 'v=0 right',
          }),
        );
      };

      await client.connect();
      final String answer = await client.requestAnswer(
        offerSdp: 'v=0 offer',
        channelId: 12,
        stream: 's',
      );

      expect(answer, 'v=0 right');
      await client.close();
    });

    test('坏消息（非 JSON / 缺 request_id）不影响后续匹配', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
      );
      transport.onPublish = (String payload) {
        final Map<String, dynamic> offer = decode(payload);
        transport.emit('not-json');
        transport.emit('{"type":"answer","sdp":"v=0 no-id"}');
        transport.emit(
          jsonEncode(<String, dynamic>{
            'type': 'answer',
            'request_id': offer['request_id'],
            'sdp': 'v=0 ok',
          }),
        );
      };

      await client.connect();
      final String answer = await client.requestAnswer(
        offerSdp: 'v=0 offer',
        channelId: 1,
        stream: 's',
      );
      expect(answer, 'v=0 ok');
      await client.close();
    });

    test('后端 error 消息转成中文异常', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
      );
      transport.onPublish = (String payload) {
        final Map<String, dynamic> offer = decode(payload);
        transport.emit(
          jsonEncode(<String, dynamic>{
            'type': 'error',
            'request_id': offer['request_id'],
            'code': 'CHANNEL_OFFLINE',
            'message': '通道离线',
          }),
        );
      };

      await client.connect();
      await expectLater(
        client.requestAnswer(
          offerSdp: 'v=0 offer',
          channelId: 1,
          stream: 's',
        ),
        throwsA(
          isA<MqttSignalingException>().having(
            (MqttSignalingException e) => e.message,
            'message',
            allOf(contains('CHANNEL_OFFLINE'), contains('通道离线')),
          ),
        ),
      );
      await client.close();
    });

    test('answer 的 sdp 为空时视为失败', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
      );
      transport.onPublish = (String payload) {
        final Map<String, dynamic> offer = decode(payload);
        transport.emit(
          jsonEncode(<String, dynamic>{
            'type': 'answer',
            'request_id': offer['request_id'],
            'sdp': '',
          }),
        );
      };

      await client.connect();
      await expectLater(
        client.requestAnswer(
          offerSdp: 'v=0 offer',
          channelId: 1,
          stream: 's',
        ),
        throwsA(isA<MqttSignalingException>()),
      );
      await client.close();
    });

    test('超时分支：10 秒内没有 answer 时抛出可读异常', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
        answerTimeout: const Duration(milliseconds: 60),
      );

      await client.connect();
      await expectLater(
        client.requestAnswer(
          offerSdp: 'v=0 offer',
          channelId: 1,
          stream: 's',
        ),
        throwsA(
          isA<MqttSignalingException>().having(
            (MqttSignalingException e) => e.message,
            'message',
            contains('MQTT 信令超时'),
          ),
        ),
      );
      expect(client.pendingCount, 0);
      await client.close();
    });

    test('连接失败向上抛出，调用方可回退 WHEP', () async {
      final FakeTransport transport = FakeTransport()..connectFails = true;
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
      );

      await expectLater(
        client.connect(),
        throwsA(isA<MqttSignalingException>()),
      );
      expect(client.isConnected, isFalse);
    });

    test('close 会以异常结束未完成的请求（页面不会永久转圈）', () async {
      final FakeTransport transport = FakeTransport();
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: transport,
        answerTimeout: const Duration(seconds: 5),
      );
      await client.connect();

      final Future<String> pending = client.requestAnswer(
        offerSdp: 'v=0 offer',
        channelId: 1,
        stream: 's',
      );
      // 先挂上断言再关闭：close 会立即让 pending 以异常结束，
      // 晚于这个时机挂监听会被 Dart 当作未处理异步错误。
      final Future<void> expectation = expectLater(
        pending,
        throwsA(
          isA<MqttSignalingException>().having(
            (MqttSignalingException e) => e.message,
            'message',
            contains('已关闭'),
          ),
        ),
      );
      await client.close();
      await expectation;
    });

    test('未连接时请求直接被拒绝', () async {
      final MqttSignalingClient client = MqttSignalingClient(
        config: config(),
        transport: FakeTransport(),
      );

      await expectLater(
        client.requestAnswer(
          offerSdp: 'v=0 offer',
          channelId: 1,
          stream: 's',
        ),
        throwsA(isA<MqttSignalingException>()),
      );
    });
  });
}
