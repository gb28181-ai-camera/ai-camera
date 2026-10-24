// 产品类型分流、分享按钮权限与分享令牌解析。
import 'package:ai_camera_app/constants/DefaultTheme.dart';
import 'package:ai_camera_app/pages/iot/add_device_page.dart';
import 'package:ai_camera_app/pages/iot/iot_device_page.dart';
import 'package:ai_camera_app/pages/iot/share_device_page.dart';
import 'package:ai_camera_app/services/IotAppController.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../test_dio_adapter.dart';

/// 构造一台设备（产品类型 / 角色可调）。
IotDevice buildDevice({
  int id = 7,
  bool allowShare = true,
  String role = 'owner',
  String name = '客厅看护相机',
}) {
  return IotDevice.fromJson(<String, dynamic>{
    'id': id,
    'name': name,
    'device_code': 'ELDER-0007',
    'status': 'online',
    'role': role,
    'product': <String, dynamic>{
      'id': 3,
      'code': 'P-ELDER-01',
      'name': '适老化智能相机',
      'allow_share': allowShare,
    },
  });
}

/// 渲染单个设备卡片。
Future<void> pumpDeviceCard(
  WidgetTester tester,
  IotDevice device, {
  VoidCallback? onShare,
}) async {
  tester.view.physicalSize = const Size(414, 896);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);

  await tester.pumpWidget(
    MaterialApp(
      theme: DefaultTheme.theme,
      home: Scaffold(
        body: ListView(
          children: <Widget>[
            IotDeviceCard(
              device: device,
              onTap: () {},
              onShare: onShare,
            ),
          ],
        ),
      ),
    ),
  );
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    ApiService.setToken('tk-share');
    ApiService.resetDio();
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  group('产品类型 → 添加入口分流', () {
    test('security 直达扫码页，elderly 走蓝牙配网', () {
      expect(IotDeviceType.security.addRoute, '/iot/scan');
      expect(IotDeviceType.elderly.addRoute, '/iot/ble');
    });

    test('unknown 兜底走扫码页（兼容未下发 device_type 的后端）', () {
      expect(IotDeviceType.unknown.addRoute, '/iot/scan');
      expect(IotDeviceType.tryParse(''), IotDeviceType.unknown);
      expect(IotDeviceType.tryParse('SECURITY'), IotDeviceType.security);
      expect(IotDeviceType.tryParse('elderly'), IotDeviceType.elderly);
    });

    testWidgets('安防相机卡片跳转 /iot/scan', (WidgetTester tester) async {
      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      expect(find.text('安防相机'), findsOneWidget);
      expect(find.text('适老化智能相机'), findsOneWidget);

      await tester.tap(find.byKey(const Key('iot-add-security')));
      await tester.pumpAndSettle();

      expect(find.text('scan-page'), findsOneWidget);
    });

    testWidgets('适老化相机卡片跳转 /iot/ble', (WidgetTester tester) async {
      await tester.pumpWidget(_routerApp());
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const Key('iot-add-elderly')));
      await tester.pumpAndSettle();

      expect(find.text('ble-page'), findsOneWidget);
    });
  });

  group('分享权限由产品配置决定', () {
    test('owner 且产品允许分享才可分享', () {
      expect(buildDevice(role: 'owner', allowShare: true).canShare, isTrue);
      expect(buildDevice(role: 'shared', allowShare: true).canShare, isFalse);
      expect(buildDevice(role: 'owner', allowShare: false).canShare, isFalse);
    });

    testWidgets('owner + 允许分享 显示分享按钮', (WidgetTester tester) async {
      final IotDevice device = buildDevice();
      int shared = 0;
      await pumpDeviceCard(tester, device, onShare: () => shared++);

      expect(find.byKey(const Key('iot-device-share-7')), findsOneWidget);
      expect(find.byKey(const Key('iot-device-role-7')), findsOneWidget);
      expect(find.text('主账号'), findsOneWidget);

      await tester.tap(find.byKey(const Key('iot-device-share-7')));
      expect(shared, 1);
    });

    testWidgets('shared 不显示分享按钮', (WidgetTester tester) async {
      await pumpDeviceCard(
        tester,
        buildDevice(role: 'shared'),
        onShare: null,
      );

      expect(find.byKey(const Key('iot-device-share-7')), findsNothing);
      expect(find.text('分享账号'), findsOneWidget);
    });

    testWidgets('产品未开启分享时即使 owner 也不显示分享按钮',
        (WidgetTester tester) async {
      await pumpDeviceCard(
        tester,
        buildDevice(allowShare: false),
        onShare: null,
      );

      expect(find.byKey(const Key('iot-device-share-7')), findsNothing);
    });
  });

  group('分享令牌', () {
    test('解析 device_id / account_id / token / qr_content', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{
          '/app/iot/devices/7/share-token': wrapSuccess(<String, dynamic>{
            'device_id': 7,
            'account_id': 31,
            'token': 'share-abc-123',
            'qr_content': 'https://boticz.cn/deviceShareToken/share-abc-123',
          }),
        },
      );
      ApiService.dio = buildTestDio(adapter);

      final RIotDeviceShareToken resp =
          await IotAppController.createDeviceShareToken(7);

      expect(resp.isSuccess, isTrue);
      final IotDeviceShareToken? share = resp.share;
      expect(share, isNotNull);
      expect(share!.deviceId, 7);
      expect(share.accountId, 31);
      expect(share.token, 'share-abc-123');
      expect(
        share.qrContent,
        'https://boticz.cn/deviceShareToken/share-abc-123',
      );
      expect(adapter.lastRequest?.method, 'POST');
      expect(adapter.lastRequest?.headers['Authorization'], 'Bearer tk-share');
    });

    test('qr_content 可被扫码解析器识别为分享码（家人扫码后走 claim）', () async {
      final IotDeviceShareToken share = IotDeviceShareToken.fromJson(
        <String, dynamic>{
          'device_id': 7,
          'account_id': 31,
          'token': 'share-abc-123',
          'qr_content': 'https://boticz.cn/deviceShareToken/share-abc-123',
        },
      );

      final IotQrPayload? payload = IotQrPayload.tryParse(share.shareText);
      expect(payload, isA<DeviceShareCode>());
      expect(payload?.action, IotQrAction.claim);
      if (payload is DeviceShareCode) {
        expect(payload.token, 'share-abc-123');
      }
    });

    test('后端未给 qr_content 时按约定拼接分享链接', () {
      final IotDeviceShareToken share = IotDeviceShareToken.fromJson(
        <String, dynamic>{
          'device_id': 7,
          'account_id': 31,
          'token': 'share-abc-123',
        },
      );

      expect(
        share.shareText,
        'https://boticz.cn/deviceShareToken/share-abc-123',
      );
    });

    testWidgets('分享视图自动生成、展示分享码，并支持重新生成',
        (WidgetTester tester) async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{
          '/app/iot/devices/7/share-token': wrapSuccess(<String, dynamic>{
            'device_id': 7,
            'account_id': 31,
            'token': 'share-abc-123',
            'qr_content': 'https://boticz.cn/deviceShareToken/share-abc-123',
          }),
        },
      );
      ApiService.dio = buildTestDio(adapter);

      tester.view.physicalSize = const Size(414, 896);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);

      await tester.pumpWidget(
        MaterialApp(
          theme: DefaultTheme.theme,
          home: const Scaffold(
            body: DeviceShareView(deviceId: 7, deviceName: '客厅看护相机'),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('让家人用 App 扫此码加入，一次性有效'), findsOneWidget);
      final SelectableText token = tester.widget<SelectableText>(
        find.byKey(const Key('share-device-token')),
      );
      expect(token.data, 'https://boticz.cn/deviceShareToken/share-abc-123');
      expect(adapter.countOf('/app/iot/devices/7/share-token'), 1);

      await tester.tap(find.byKey(const Key('share-device-regenerate')));
      await tester.pumpAndSettle();
      expect(adapter.countOf('/app/iot/devices/7/share-token'), 2);
    });

    test('非 owner（403）时错误文案可展示', () async {
      ApiService.dio = buildTestDio(
        PathRoutingAdapter(<String, Object>{
          '/app/iot/devices/7/share-token':
              wrapError(403, '仅设备主账号可分享'),
        }),
      );

      final RIotDeviceShareToken resp =
          await IotAppController.createDeviceShareToken(7);

      expect(resp.isSuccess, isFalse);
      expect(resp.share, isNull);
      expect(resp.errorMessage, '仅设备主账号可分享');
    });
  });
}

/// 包含添加页与两个目标页的最小路由环境。
Widget _routerApp() {
  final GoRouter router = GoRouter(
    initialLocation: '/iot/add',
    routes: <RouteBase>[
      GoRoute(
        path: '/iot/add',
        builder: (BuildContext context, GoRouterState state) =>
            const AddDevicePage(),
      ),
      GoRoute(
        path: '/iot/scan',
        builder: (BuildContext context, GoRouterState state) =>
            const Scaffold(body: Text('scan-page')),
      ),
      GoRoute(
        path: '/iot/ble',
        builder: (BuildContext context, GoRouterState state) =>
            const Scaffold(body: Text('ble-page')),
      ),
    ],
  );
  return MaterialApp.router(
    theme: DefaultTheme.theme,
    routerConfig: router,
  );
}
