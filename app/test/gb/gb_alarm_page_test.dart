// AI 监测事件（国标报警）页测试：列表渲染、只读详情弹层、处理状态筛选。
import 'package:ai_camera_app/pages/gb/gb_alarm_page.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../test_dio_adapter.dart';

/// 构造一份国标报警列表响应。
Map<String, Object> alarmRoutes() => <String, Object>{
      '/app/gb/alarms': wrapSuccess(<String, dynamic>{
        'items': <dynamic>[
          <String, dynamic>{
            'id': 5,
            'device_id': 12,
            'channel_id': 21,
            'device_name': '东门球机',
            'channel_name': '通道一',
            'alarm_priority': '1',
            'alarm_type': '1',
            'alarm_time': '2026-10-21T10:00:00+08:00',
            'alarm_description': '视频信号丢失',
            'handled': false,
          },
          <String, dynamic>{
            'id': 6,
            'device_id': 12,
            'channel_id': 21,
            'device_name': '东门球机',
            'channel_name': '通道一',
            'alarm_priority': '3',
            'alarm_type': '2',
            'alarm_time': '2026-10-20T09:00:00+08:00',
            'alarm_description': '设备被移动',
            'handled': true,
          },
        ],
        'total': 2,
        'page': 1,
        'page_size': 20,
      }),
    };

void main() {
  setUp(() {
    ApiService.setToken('tk-gb-alarm');
    ApiService.resetDio();
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  testWidgets('列表展示类型/级别/设备通道/处理状态，点击打开只读详情', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final PathRoutingAdapter adapter = PathRoutingAdapter(alarmRoutes());
    ApiService.dio = buildTestDio(adapter);

    await tester.pumpWidget(const MaterialApp(home: GbAlarmPage()));
    await tester.pumpAndSettle();

    expect(find.text('AI 监测事件'), findsOneWidget);
    expect(find.text('视频报警'), findsOneWidget);
    expect(find.text('一级'), findsOneWidget);
    // 「未处理 / 已处理」同时是筛选 Chip 与列表状态标签。
    expect(find.text('未处理'), findsWidgets);
    expect(find.text('东门球机 · 通道一'), findsWidgets);
    expect(find.text('视频信号丢失'), findsWidgets);
    expect(find.text('设备报警'), findsOneWidget);
    expect(find.text('已处理'), findsWidgets);

    // 点击列表项 → 底部详情弹层（列表字段已足够，无需详情接口）。
    await tester.tap(find.byKey(const Key('gb-alarm-5')));
    await tester.pumpAndSettle();

    expect(find.text('报警详情'), findsOneWidget);
    expect(find.text('一级（特急）'), findsOneWidget);
    expect(find.text('报警描述'), findsOneWidget);
    // 只读：不应出现处理/抓图按钮。
    expect(find.text('处理'), findsNothing);
    expect(find.text('抓图'), findsNothing);
  });

  testWidgets('切换“未处理”筛选时携带 handled=false 重新加载', (
    WidgetTester tester,
  ) async {
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final PathRoutingAdapter adapter = PathRoutingAdapter(alarmRoutes());
    ApiService.dio = buildTestDio(adapter);

    await tester.pumpWidget(const MaterialApp(home: GbAlarmPage()));
    await tester.pumpAndSettle();

    // 首屏不带 handled 参数（全部）。
    expect(adapter.lastRequest!.queryParameters.containsKey('handled'), isFalse);

    await tester.tap(find.byKey(const Key('gb-alarm-filter-pending')));
    await tester.pumpAndSettle();

    final List<RequestOptions> alarmRequests = adapter.requests
        .where((RequestOptions r) => r.path.endsWith('/app/gb/alarms'))
        .toList();
    expect(alarmRequests, hasLength(2));
    expect(alarmRequests.last.queryParameters['handled'], isFalse);
  });
}
