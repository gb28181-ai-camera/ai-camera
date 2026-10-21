// 国标设备列表页测试：设备渲染 + 点击展开通道（懒加载）。
//
// 使用 MSW 等价的 Dio 适配器方案（PathRoutingAdapter），无需真实后端。
import 'package:ai_camera_app/pages/gb/gb_device_list_page.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../test_dio_adapter.dart';

void main() {
  testWidgets('设备列表渲染，点击设备后懒加载并展示通道', (WidgetTester tester) async {
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final PathRoutingAdapter adapter = PathRoutingAdapter(<String, Object>{
      '/app/gb/devices': wrapSuccess(<String, dynamic>{
        'items': <dynamic>[
          <String, dynamic>{
            'id': 12,
            'name': '东门球机',
            'device_code': 'DEV-001',
            'gb_device_id': '34020000001320000001',
            'region_name': '东门区域',
            'status': 'online',
            'register_status': 'activated',
            'channel_count': 4,
          },
        ],
        'total': 1,
        'page': 1,
        'page_size': 20,
      }),
      '/app/gb/channels': wrapSuccess(<String, dynamic>{
        'items': <dynamic>[
          <String, dynamic>{
            'id': 21,
            'device_id': 12,
            'device_name': '东门球机',
            'gb_channel_id': '34020000001310000001',
            'name': '通道一',
            'status': 'ON',
            'region_name': '东门区域',
          },
        ],
        'total': 1,
        'page': 1,
        'page_size': 100,
      }),
    });

    ApiService.setToken('tk-test');
    ApiService.resetDio();
    ApiService.dio = buildTestDio(adapter);
    addTearDown(() {
      ApiService.setToken(null);
      ApiService.resetDio();
    });

    await tester.pumpWidget(const MaterialApp(home: GbDeviceListPage()));
    await tester.pumpAndSettle();

    // 设备信息（名称 / 在线 / 通道数）。
    expect(find.text('东门球机'), findsOneWidget);
    expect(find.text('4 个通道'), findsOneWidget);
    expect(find.text('在线'), findsWidgets);

    // 通道尚未展开时不应请求通道接口。
    expect(adapter.countOf('/app/gb/channels'), 0);

    // 点击设备 → 展开并懒加载通道。
    await tester.tap(find.byKey(const Key('gb-device-12')));
    await tester.pumpAndSettle();

    expect(find.text('通道一'), findsOneWidget);
    expect(adapter.countOf('/app/gb/channels'), 1);
    expect(
      adapter.lastRequest!.queryParameters['device_id'],
      12,
    );
  });
}
