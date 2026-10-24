// App 端 IOT 控制器：端点、请求体与响应解析。
import 'package:ai_camera_app/services/IotAppController.dart';
import 'package:ai_camera_app/services/MqttSignalingClient.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:flutter_test/flutter_test.dart';

import '../test_dio_adapter.dart';

/// 单台设备的模拟响应。
Map<String, dynamic> deviceJson({
  int id = 5,
  String name = '大门摄像头',
  String status = 'online',
  String role = 'owner',
}) {
  return <String, dynamic>{
    'id': id,
    'name': name,
    'device_code': 'CAM-0005',
    'product_id': 2,
    'product_name': '户外球机',
    'product_code': 'P-OD-01',
    'role': role,
    'status': status,
    'region_name': '东区',
    'activated_at': '2026-10-20T10:00:00+08:00',
  };
}

void main() {
  // ApiService 的错误分支会调用 AppNavigator.showToast，后者依赖 WidgetsBinding。
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    ApiService.setToken('tk-iot');
    ApiService.resetDio();
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  test('设备列表解析分页形态，并注入 Bearer token', () async {
    final PathRoutingAdapter adapter = PathRoutingAdapter(
      <String, Object>{
        '/app/iot/devices': wrapSuccess(<String, dynamic>{
          'items': <dynamic>[
            deviceJson(),
            deviceJson(id: 6, name: '后门摄像头', status: 'offline', role: 'shared'),
          ],
          'total': 2,
          'page': 1,
          'page_size': 20,
        }),
      },
    );
    ApiService.dio = buildTestDio(adapter);

    final RIotDeviceList resp = await IotAppController.devices();

    expect(resp.isSuccess, isTrue);
    expect(resp.total, 2);
    expect(resp.items, hasLength(2));
    expect(resp.items.first.name, '大门摄像头');
    expect(resp.items.first.productName, '户外球机');
    expect(resp.items.first.online, isTrue);
    expect(resp.items.first.roleLabel, '主账号');
    expect(resp.items.last.online, isFalse);
    expect(resp.items.last.roleLabel, '分享账号');
    expect(adapter.lastRequest?.headers['Authorization'], 'Bearer tk-iot');
    expect(adapter.lastRequest?.queryParameters['page'], 1);
  });

  test('设备列表兼容数组形态', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/iot/devices': wrapSuccess(<dynamic>[deviceJson()]),
      }),
    );

    final RIotDeviceList resp = await IotAppController.devices();
    expect(resp.items, hasLength(1));
    expect(resp.total, 1);
  });

  test('设备详情解析嵌套 device / account 包装', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/iot/devices/5': wrapSuccess(<String, dynamic>{
          'device': deviceJson(),
          'account': <String, dynamic>{
            'id': 9,
            'role': 'shared',
            'nickname': '门卫',
            'activated_at': '2026-10-20T10:00:00+08:00',
          },
          'product': <String, dynamic>{
            'id': 2,
            'code': 'P-OD-01',
            'name': '户外球机',
          },
          'service_groups': <dynamic>[],
        }),
      }),
    );

    final RIotDevice resp = await IotAppController.deviceDetail(5);
    expect(resp.device?.deviceCode, 'CAM-0005');
    expect(resp.device?.productName, '户外球机');
    expect(resp.device?.nickname, '门卫');
    // device 自身带 role=owner，优先于 account.role。
    expect(resp.device?.role, 'owner');
  });

  test('分享码认领：POST body 只带 token', () async {
    final PathRoutingAdapter adapter = PathRoutingAdapter(
      <String, Object>{
        '/app/iot/devices/claim': wrapSuccess(deviceJson()),
      },
    );
    ApiService.dio = buildTestDio(adapter);

    final RIotDevice resp = await IotAppController.claim('share-token-1');

    expect(resp.isSuccess, isTrue);
    expect(resp.device?.id, 5);
    expect(adapter.lastRequest?.method, 'POST');
    final dynamic body = adapter.lastRequest?.data;
    expect(body, isA<Map<String, dynamic>>());
    if (body is Map<String, dynamic>) {
      expect(body['token'], 'share-token-1');
    }
  });

  test('BLE 绑定：提交设备返回的编号与密钥', () async {
    final PathRoutingAdapter adapter = PathRoutingAdapter(
      <String, Object>{
        '/app/iot/devices/bind': wrapSuccess(deviceJson()),
      },
    );
    ApiService.dio = buildTestDio(adapter);

    final RIotDevice resp = await IotAppController.bind(
      deviceCode: 'CAM-0005',
      deviceSecret: 'secret-xyz',
      nickname: '大门',
    );

    expect(resp.isSuccess, isTrue);
    final dynamic body = adapter.lastRequest?.data;
    expect(body, isA<Map<String, dynamic>>());
    if (body is Map<String, dynamic>) {
      expect(body['device_code'], 'CAM-0005');
      expect(body['device_secret'], 'secret-xyz');
      expect(body['nickname'], '大门');
    }
  });

  test('设备二维码绑定：提交 product_code + device_code', () async {
    final PathRoutingAdapter adapter = PathRoutingAdapter(
      <String, Object>{
        '/app/iot/devices/bind-qr': wrapSuccess(deviceJson()),
      },
    );
    ApiService.dio = buildTestDio(adapter);

    final RIotDevice resp = await IotAppController.bindByQr(
      productCode: 'P-OD-01',
      deviceCode: 'CAM-0005',
    );

    expect(resp.isSuccess, isTrue);
    final dynamic body = adapter.lastRequest?.data;
    expect(body, isA<Map<String, dynamic>>());
    if (body is Map<String, dynamic>) {
      expect(body['product_code'], 'P-OD-01');
      expect(body['device_code'], 'CAM-0005');
      expect(body.containsKey('nickname'), isFalse);
    }
  });

  test('服务组解析：product / groups / mqtt / turn', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/iot/devices/5/services': wrapSuccess(<String, dynamic>{
          'product': <String, dynamic>{
            'code': 'P-OD-01',
            'name': '户外球机',
          },
          'groups': <dynamic>[
            <String, dynamic>{
              'code': 'grp-9',
              'name': '视频服务',
              'type': 'webrtc',
              'username': 'iot_u',
              'password': 'iot_p',
              'items': <dynamic>[
                <String, dynamic>{
                  'protocol': 'MQTT',
                  'host': '192.168.1.10',
                  'port': 1883,
                  'path': '',
                  'extra': '{"tls":true}',
                },
                <String, dynamic>{
                  'protocol': 'WebRTC',
                  'host': 'media.local',
                  'port': 443,
                  'path': '/whep',
                  'extra': <String, dynamic>{},
                },
              ],
            },
          ],
          'mqtt': <String, dynamic>{
            'url': 'mqtt://192.168.1.10:1883',
            'topic_prefix': 'iot/grp-9',
          },
          'turn': <String, dynamic>{
            'urls': <String>['turn:turn.local:3478'],
            'realm': 'ai-camera',
          },
        }),
      }),
    );

    final RIotServices resp = await IotAppController.services(5);
    final IotCredentials? credentials = resp.credentials;

    expect(resp.isSuccess, isTrue);
    expect(credentials, isNotNull);
    expect(credentials?.productName, '户外球机');
    expect(credentials?.groups, hasLength(1));
    final IotServiceGroup group = credentials!.groups.first;
    expect(group.code, 'grp-9');
    expect(group.username, 'iot_u');
    expect(group.items, hasLength(2));
    expect(group.mqttItem?.host, '192.168.1.10');
    expect(group.mqttItem?.extra['tls'], isTrue);
    expect(credentials.mqtt?.topicPrefix, 'iot/grp-9');
    expect(credentials.turn?.realm, 'ai-camera');

    final MqttSignalingConfig config = MqttSignalingConfig.fromCredentials(
      credentials,
      clientId: 'app_test',
    );
    expect(config.port, 1883);
    expect(config.host, '192.168.1.10');
    expect(config.subscribeTopic, 'iot/grp-9/app/app_test/signal');
    expect(config.publishTopic, 'iot/grp-9/signal/backend');
  });

  test('ICE 配置：data 为数组，expires_at 为 Unix 秒', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/rtc/ice-servers': wrapSuccess(<dynamic>[
          <String, dynamic>{
            'urls': <String>['turn:turn.local:3478?transport=udp'],
            'username': 'u',
            'credential': 'c',
            'ttl': 3600,
            'expires_at': 1790000000,
            'realm': 'ai-camera',
          },
        ]),
      }),
    );

    final RIceServers resp = await IotAppController.iceServers();
    final List<IceServerConfig> servers = resp.servers;

    expect(servers, hasLength(1));
    final IceServerConfig config = servers.first;
    expect(config.urls, <String>['turn:turn.local:3478?transport=udp']);
    expect(config.isEmpty, isFalse);
    expect(config.expiresAt, 1790000000);
    expect(config.toRtcIceServer(), <String, dynamic>{
      'urls': <String>['turn:turn.local:3478?transport=udp'],
      'username': 'u',
      'credential': 'c',
    });
  });

  test('ICE 配置：urls 支持单个字符串', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/rtc/ice-servers': wrapSuccess(<dynamic>[
          <String, dynamic>{
            'urls': 'stun:stun.local:3478',
            'ttl': 0,
          },
        ]),
      }),
    );

    final RIceServers resp = await IotAppController.iceServers();
    expect(resp.servers.first.urls, <String>['stun:stun.local:3478']);
  });

  test('ICE 为空数组时表示直连', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/rtc/ice-servers': wrapSuccess(<dynamic>[]),
      }),
    );

    final RIceServers resp = await IotAppController.iceServers();
    expect(resp.isSuccess, isTrue);
    expect(resp.servers, isEmpty);
  });

  test('业务错误透出后端中文提示', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/iot/devices/claim': wrapError(400, '分享码已失效'),
      }),
    );

    final RIotDevice resp = await IotAppController.claim('expired');
    expect(resp.isSuccess, isFalse);
    expect(resp.errorMessage, '分享码已失效');
    expect(resp.device, isNull);
  });

  test('绑定接口失败时错误文案可展示', () async {
    ApiService.dio = buildTestDio(
      PathRoutingAdapter(<String, Object>{
        '/app/iot/devices/bind-qr': wrapError(400, '设备已被其他账号绑定'),
      }),
    );

    final RIotDevice resp = await IotAppController.bindByQr(
      productCode: 'P',
      deviceCode: 'D',
    );
    expect(resp.isSuccess, isFalse);
    expect(resp.errorMessage, '设备已被其他账号绑定');
  });
}
