// 云台面板交互测试。
//
// 锁定 GB28181 最关键的一条交互语义：**按住下发方向、松开发 stop**。
// 若退化成点击式单发，真实设备上的镜头会一直转 —— 这是最容易回归、
// 又无法靠静态检查发现的缺陷。
import 'package:ai_camera_app/pages/gb/gb_ptz_panel.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<List<String>> pumpPanel(
    WidgetTester tester, {
    bool enabled = true,
  }) async {
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    final List<String> commands = <String>[];
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: GbPtzPanel(
              enabled: enabled,
              onCommand: (String direction, int speed) {
                commands.add(direction);
              },
              onAction: (String action, int speed) {
                commands.add(action);
              },
            ),
          ),
        ),
      ),
    );
    return commands;
  }

  testWidgets('按住方向键下发方向，松开立即下发 stop', (WidgetTester tester) async {
    final List<String> commands = await pumpPanel(tester);

    final TestGesture gesture =
        await tester.startGesture(tester.getCenter(find.byKey(const Key('gb-ptz-up'))));
    await tester.pump();
    expect(commands, <String>['up']);

    await gesture.up();
    await tester.pumpAndSettle();
    expect(commands, <String>['up', 'stop']);
  });

  testWidgets('按住变焦键下发动作，松开下发 zoom-stop', (WidgetTester tester) async {
    final List<String> commands = await pumpPanel(tester);

    final TestGesture gesture = await tester
        .startGesture(tester.getCenter(find.byKey(const Key('gb-ptz-zoom-in'))));
    await tester.pump();
    expect(commands, <String>['zoom-in']);

    await gesture.up();
    await tester.pumpAndSettle();
    expect(commands, <String>['zoom-in', 'zoom-stop']);
  });

  testWidgets('中央停止按钮单发一次 stop', (WidgetTester tester) async {
    final List<String> commands = await pumpPanel(tester);

    await tester.tap(find.byKey(const Key('gb-ptz-stop')));
    await tester.pumpAndSettle();

    expect(commands, <String>['stop']);
  });

  testWidgets('禁用时不下发指令', (WidgetTester tester) async {
    final List<String> commands = await pumpPanel(tester, enabled: false);

    final TestGesture gesture =
        await tester.startGesture(tester.getCenter(find.byKey(const Key('gb-ptz-left'))));
    await tester.pump();
    await gesture.up();
    await tester.pumpAndSettle();

    expect(commands, isEmpty);
  });
}
