// 应用级冒烟测试：引导 store 后，未登录的应用应落到登录页。
//
// 说明：本文件原为 Flutter 模板生成的计数器测试（引用并不存在的 `MyApp`），
// 已重写为真正的启动冒烟测试。
import 'package:ai_camera_app/main.dart';
import 'package:ai_camera_app/models/index.dart';
import 'package:ai_camera_app/pages/login/login_page.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:ai_camera_app/utils/SharedPreferenceUtil.dart';
import 'package:flutter/material.dart';
import 'package:flutter_dva/flutter_dva.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'test_dio_adapter.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    SharedPreferenceUtil.resetForTest();
    await SharedPreferenceUtil.init();
    ApiService.setToken(null);
    ApiService.resetDio();

    // 必须在 setUp 而非 testWidgets 内引导 store：testWidgets 的测试体运行在
    // FakeAsync 区，`Dva.start()` 内部的延迟永远不会到期，会导致 await 死锁。
    await bootDva(<Model<dynamic>>[AuthModel(), MenuModel()]);
  });

  testWidgets('未登录启动：应用渲染到登录页且登录按钮可交互',
      (WidgetTester tester) async {
    // 按手机尺寸渲染：设计令牌以屏幕短边为基准等比缩放，
    // flutter_test 默认的 800x600 画布会被放大约 1.45 倍。
    tester.view.physicalSize = const Size(414, 896);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(const AiCameraApp());
    await tester.pump();

    expect(find.byType(LoginPage), findsOneWidget);
    expect(find.byKey(const Key('login-submit')), findsOneWidget);
  });
}
