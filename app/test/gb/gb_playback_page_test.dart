// 国标录像回放页测试：检索记录渲染、时间参数格式、设备超时提示。
//
// 页面通过构造参数注入通道（等价于路由 `?channelId=`），不触发播放内核；
// 只验证「检索 → 列表」这一段，播放/云台类真机能力不在单测覆盖范围。
import 'package:ai_camera_app/pages/gb/gb_playback_page.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../test_dio_adapter.dart';

/// 固定返回一组录像片段的路由适配器。
PathRoutingAdapter playbackAdapter({
  required bool timeout,
}) {
  return PathRoutingAdapter(<String, Object>{
    '/app/gb/playback/records': wrapSuccess(<String, dynamic>{
      'channel_id': 7,
      'total': 2,
      'timeout': timeout,
      'items': <dynamic>[
        <String, dynamic>{
          'channel_id': 7,
          'name': '上午录像',
          'start_time': '2026-01-01T08:00:00',
          'end_time': '2026-01-01T09:00:00',
          'time_len': 3600,
        },
        <String, dynamic>{
          'channel_id': 7,
          'name': '',
          'start_time': '2026-01-01T09:30:00',
          'end_time': '2026-01-01T09:35:30',
          'time_len': 330,
        },
      ],
    }),
  });
}

void main() {
  setUp(() {
    ApiService.setToken('tk-gb-playback');
    ApiService.resetDio();
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  testWidgets('按默认时间范围检索并渲染录像片段', (WidgetTester tester) async {
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final PathRoutingAdapter adapter = playbackAdapter(timeout: false);
    ApiService.dio = buildTestDio(adapter);

    await tester.pumpWidget(
      const MaterialApp(
        home: GbPlaybackPage(channelId: 7, channelName: '东门通道'),
      ),
    );
    await tester.pumpAndSettle();

    // 标题来自通道名称。
    expect(find.text('东门通道'), findsOneWidget);

    // 片段信息：名称、起止时间、时长。
    expect(find.text('上午录像'), findsOneWidget);
    expect(
      find.textContaining('2026-01-01 08:00 ~ 2026-01-01 09:00'),
      findsOneWidget,
    );
    expect(find.text('时长 1 小时 0 分'), findsOneWidget);
    expect(find.text('共 2 段'), findsOneWidget);
    // 名称为空的片段回退为「录像片段 N」。
    expect(find.text('录像片段 2'), findsOneWidget);

    // 请求参数：通道 ID + GB28181 固定时间格式（设备按 XML 原样解析）。
    final RequestOptions req = adapter.lastRequest!;
    expect(req.path, '/app/gb/playback/records');
    expect(req.queryParameters['channel_id'], 7);
    expect(
      req.queryParameters['start_time'],
      matches(RegExp(r'^\d{4}-\d{2}-\d{2}T00:00:00$')),
    );
    expect(
      req.queryParameters['end_time'],
      matches(RegExp(r'^\d{4}-\d{2}-\d{2}T23:59:59$')),
    );
  });

  testWidgets('设备 5 秒未响应时展示超时提示', (WidgetTester tester) async {
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final PathRoutingAdapter adapter = playbackAdapter(timeout: true);
    ApiService.dio = buildTestDio(adapter);

    await tester.pumpWidget(
      const MaterialApp(
        home: GbPlaybackPage(channelId: 7, channelName: '东门通道'),
      ),
    );
    await tester.pumpAndSettle();

    // 后端把「设备没回」作为业务结果返回，App 必须明确提示而不是当成空列表。
    expect(
      find.text('设备未在 5 秒内响应，请重试或缩小时间范围'),
      findsOneWidget,
    );
  });

  testWidgets('业务失败时展示后端错误文案', (WidgetTester tester) async {
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    ApiService.dio = buildTestDio(
      FixedAdapter(
        wrapError(400, '回放时间窗非法：结束时间必须晚于开始时间'),
        statusCode: 400,
      ),
    );

    await tester.pumpWidget(
      const MaterialApp(
        home: GbPlaybackPage(channelId: 7, channelName: '东门通道'),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.text('回放时间窗非法：结束时间必须晚于开始时间'),
      findsOneWidget,
    );
    expect(find.text('检索失败'), findsOneWidget);
  });
}
