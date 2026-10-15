import 'package:ai_camera_app/constants/DefaultTheme.dart';
import 'package:ai_camera_app/models/index.dart';
import 'package:ai_camera_app/pages/login/login_page.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:ai_camera_app/utils/SharedPreferenceUtil.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_dva/flutter_dva.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'test_dio_adapter.dart';

/// 渲染登录页。
Widget wrapLogin() => MaterialApp(
      theme: DefaultTheme.theme,
      home: const LoginPage(),
    );

/// 按手机尺寸视口渲染登录页。
///
/// 设计令牌以屏幕短边为基准等比缩放：flutter_test 默认的 800x600 画布
/// （短边 600 > 设计稿 414）会把所有令牌放大约 1.45 倍，内容因此超出画布、
/// 登录按钮落到可视区之外，`tester.tap` 会直接命中失败。
/// 真机是竖向手机屏（短边 ≈ 设计稿宽度），故在此按 414x896 逻辑像素渲染。
Future<void> pumpLogin(WidgetTester tester) async {
  tester.view.physicalSize = const Size(414, 896);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(wrapLogin());
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Dva dva;

  setUp(() async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    SharedPreferenceUtil.resetForTest();
    await SharedPreferenceUtil.init();
    ApiService.setToken(null);
    ApiService.resetDio();

    // 必须在 setUp 而非 testWidgets 内引导 store：
    // testWidgets 的测试体运行在 FakeAsync 区，`Dva.start()` 内部的
    // `Future.delayed(10ms)` 永远不会到期，会导致 await 死锁。
    dva = await bootDva(<Model<dynamic>>[AuthModel(), MenuModel()]);
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  group('LoginPage', () {
    testWidgets('渲染：用户名、密码、登录按钮与版本号', (WidgetTester tester) async {
      await pumpLogin(tester);

      expect(find.text('欢迎登录'), findsOneWidget);
      expect(find.text('AI Camera 智能监控平台'), findsOneWidget);
      expect(find.byKey(const Key('login-username')), findsOneWidget);
      expect(find.byKey(const Key('login-password')), findsOneWidget);
      expect(find.byKey(const Key('login-submit')), findsOneWidget);
      expect(find.text('登 录'), findsOneWidget);
      expect(find.textContaining('版本'), findsOneWidget);

      // 未登录时不展示错误提示。
      expect(find.byKey(const Key('login-error')), findsNothing);
    });

    testWidgets('空输入校验：逐个提示缺失字段，且不发起请求',
        (WidgetTester tester) async {
      final FixedAdapter adapter = FixedAdapter(
        wrapSuccess(<String, dynamic>{'token': 'tk-1'}),
      );
      ApiService.dio = buildTestDio(adapter);

      await pumpLogin(tester);

      // 通过 key 精确定位内联错误文本（避免与输入框 hint 文案冲突）。
      String? errorText() {
        final Finder f = find.byKey(const Key('login-error'));
        if (f.evaluate().isEmpty) return null;
        return tester.widget<Text>(f).data;
      }

      // 两个都空 → 提示用户名。
      await tester.tap(find.byKey(const Key('login-submit')));
      await tester.pump();
      expect(errorText(), '请输入用户名');

      // 只填用户名 → 提示密码。
      await tester.enterText(
        find.byKey(const Key('login-username')),
        'admin',
      );
      await tester.tap(find.byKey(const Key('login-submit')));
      await tester.pump();
      expect(errorText(), '请输入密码');

      // 本地校验不触发任何网络请求。
      expect(adapter.requests, isEmpty);
    });

    testWidgets('登录失败：内联展示后端返回的错误信息',
        (WidgetTester tester) async {
      final FixedAdapter adapter = FixedAdapter(
        wrapError(401, '用户名或密码错误'),
      );
      ApiService.dio = buildTestDio(adapter);

      await pumpLogin(tester);

      await tester.enterText(
        find.byKey(const Key('login-username')),
        'admin',
      );
      await tester.enterText(
        find.byKey(const Key('login-password')),
        'wrong-password',
      );
      await tester.tap(find.byKey(const Key('login-submit')));

      // 显式分帧等待异步 effect 完成（不使用 pumpAndSettle，
      // 避免 loading 态的无限动画导致其超时）。
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      await tester.pump(const Duration(milliseconds: 50));

      expect(find.byKey(const Key('login-error')), findsOneWidget);
      expect(find.text('用户名或密码错误'), findsOneWidget);
      // 失败后按钮恢复可点击，且不再有 token。
      expect(ApiService.token, isNull);
      expect(find.text('登 录'), findsOneWidget);
    });

    testWidgets('登录成功：状态写入 store，错误提示消失',
        (WidgetTester tester) async {
      final FixedAdapter adapter = FixedAdapter(
        wrapSuccess(<String, dynamic>{
          'token': 'tk-login-ok',
          'user_id': 9,
          'name': '王五',
          'dept_id': 1,
          'data_perm': 'all',
          'cross_dept_ids': '',
          'permissions': <String>['risk:view'],
        }),
      );
      ApiService.dio = buildTestDio(adapter);

      await pumpLogin(tester);

      await tester.enterText(
        find.byKey(const Key('login-username')),
        'admin',
      );
      await tester.enterText(
        find.byKey(const Key('login-password')),
        'admin123',
      );
      await tester.tap(find.byKey(const Key('login-submit')));

      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      await tester.pump(const Duration(milliseconds: 50));

      final AuthState state = readState<AuthState>(dva, 'auth');
      expect(state.isLoggedIn, isTrue);
      expect(state.token, 'tk-login-ok');
      expect(state.name, '王五');
      expect(state.error, isNull);
      expect(find.byKey(const Key('login-error')), findsNothing);

      // 请求体字段与后端契约一致（取登录那次请求，末次请求已是 /app/menus）。
      final RequestOptions loginReq = adapter.requests.firstWhere(
        (RequestOptions r) => r.path.endsWith('/app/auth/login'),
      );
      final Map<String, dynamic> body = loginReq.data as Map<String, dynamic>;
      expect(body['user_name'], 'admin');
      expect(body['password'], 'admin123');

      // 登录成功后应带上 token 拉取菜单。
      expect(adapter.countOf('/app/menus'), 1);
    });
  });
}
