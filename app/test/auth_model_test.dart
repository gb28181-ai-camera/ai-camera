import 'package:ai_camera_app/models/index.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:ai_camera_app/utils/SharedPreferenceUtil.dart';
import 'package:dio/dio.dart';
import 'package:flutter_dva/flutter_dva.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'test_dio_adapter.dart';

/// 后端登录成功的标准响应。
Map<String, dynamic> loginOkBody() => wrapSuccess(<String, dynamic>{
      'token': 'tk-abc-123',
      'user_id': 7,
      'name': '张三',
      'dept_id': 2,
      'data_perm': 'dept',
      'cross_dept_ids': '',
      'permissions': <String>['risk:handle', 'alarm:handle'],
    });

/// 菜单树（登录后 auth effect 会顺带拉取）。
Map<String, dynamic> menusBody() => wrapSuccess(<dynamic>[
      <String, dynamic>{
        'id': 1,
        'name': '风险',
        'parent_id': null,
        'level': 1,
        'path': '/risks',
        'icon': 'risk',
        'type': 'menu',
        'platform': 'mobile',
        'permission_code': 'risk:view',
        'hidden': false,
        'children': <dynamic>[],
      },
    ]);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late Dva dva;

  setUp(() async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    SharedPreferenceUtil.resetForTest();
    await SharedPreferenceUtil.init();
    ApiService.setToken(null);
    // 与生产一致：auth 与 menu 同时注册，登录后会顺带拉取菜单。
    dva = await bootDva(<Model<dynamic>>[AuthModel(), MenuModel()]);
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  group('AuthModel', () {
    test('登录成功：token 写入状态、内存与 SharedPreferences', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{
          '/app/auth/login': loginOkBody(),
          '/app/menus': menusBody(),
        },
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'auth/login', <String, dynamic>{
        'user_name': 'admin',
        'password': 'admin123',
      });

      final AuthState state = readState<AuthState>(dva, 'auth');

      expect(state.isLoggedIn, isTrue);
      expect(state.token, 'tk-abc-123');
      expect(state.userId, 7);
      expect(state.name, '张三');
      expect(state.dataPerm, 'dept');
      expect(state.loading, isFalse);
      expect(state.error, isNull);
      // 权限随登录载荷写入 user。
      expect(state.hasPermission('risk:handle'), isTrue);

      // 持久化：内存缓存与 SharedPreferences 均可见。
      expect(ApiService.token, 'tk-abc-123');
      expect(SharedPreferenceUtil.getToken(), 'tk-abc-123');
      expect(SharedPreferenceUtil.getUserId(), 7);
      expect(SharedPreferenceUtil.getDisplayName(), '张三');

      // 登录请求体使用 user_name 字段（而非 username）。
      final RequestOptions loginReq = adapter.requests.firstWhere(
        (RequestOptions r) => r.path.endsWith('/app/auth/login'),
      );
      final Map<String, dynamic> body =
          loginReq.data as Map<String, dynamic>;
      expect(body['user_name'], 'admin');
      expect(body['password'], 'admin123');

      // 登录成功后应带上 token 拉取移动端菜单。
      expect(adapter.countOf('/app/auth/login'), 1);
      expect(adapter.countOf('/app/menus'), 1);
      final MenuState menu = readState<MenuState>(dva, 'menu');
      expect(menu.menus, hasLength(1));
      expect(menu.hasPerm('risk:view'), isTrue);
    });

    test('登录失败：保留错误信息且不写入任何 token', () async {
      final FixedAdapter adapter = FixedAdapter(
        wrapError(401, '用户名或密码错误'),
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'auth/login', <String, dynamic>{
        'user_name': 'admin',
        'password': 'wrong',
      });

      final AuthState state = readState<AuthState>(dva, 'auth');

      expect(state.isLoggedIn, isFalse);
      expect(state.token, isNull);
      expect(state.loading, isFalse);
      expect(state.error, '用户名或密码错误');
      expect(SharedPreferenceUtil.getToken(), isNull);
      expect(ApiService.token, isNull);
    });

    test('空用户名或密码：本地拦截，不发起请求', () async {
      final FixedAdapter adapter = FixedAdapter(loginOkBody());
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'auth/login', <String, dynamic>{
        'user_name': '',
        'password': '',
      });

      final AuthState state = readState<AuthState>(dva, 'auth');
      expect(state.error, '请输入用户名和密码');
      expect(state.isLoggedIn, isFalse);
      expect(adapter.requests, isEmpty);
    });

    test('restoreToken：冷启动从本地恢复登录态', () async {
      SharedPreferences.setMockInitialValues(<String, Object>{
        'auth_token': 'tk-restored',
        'auth_user_id': 3,
        'auth_display_name': '李四',
        'auth_data_perm': 'all',
      });
      SharedPreferenceUtil.resetForTest();
      await SharedPreferenceUtil.init();

      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{'/app/menus': menusBody()},
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'auth/restoreToken');

      final AuthState state = readState<AuthState>(dva, 'auth');
      expect(state.isLoggedIn, isTrue);
      expect(state.token, 'tk-restored');
      expect(state.userId, 3);
      expect(state.name, '李四');
      expect(state.dataPerm, 'all');
      // 恢复后应带上 token 去拉菜单。
      expect(ApiService.token, 'tk-restored');
      expect(adapter.countOf('/app/menus'), 1);
      expect(
        adapter.lastRequest!.headers['Authorization'],
        'Bearer tk-restored',
      );
    });

    test('restoreToken：本地无 token 时保持未登录', () async {
      await dispatchAction(dva, 'auth/restoreToken');

      final AuthState state = readState<AuthState>(dva, 'auth');
      expect(state.isLoggedIn, isFalse);
      expect(state.token, isNull);
    });

    test('logout：清空状态与本地持久化的 token', () async {
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{
          '/app/auth/login': loginOkBody(),
          '/app/menus': menusBody(),
        },
      );
      ApiService.dio = buildTestDio(adapter);

      await dispatchAction(dva, 'auth/login', <String, dynamic>{
        'user_name': 'admin',
        'password': 'admin123',
      });
      expect(SharedPreferenceUtil.getToken(), 'tk-abc-123');

      await dispatchAction(dva, 'auth/logout');

      final AuthState state = readState<AuthState>(dva, 'auth');
      expect(state.isLoggedIn, isFalse);
      expect(state.token, isNull);
      expect(state.user, isNull);
      expect(SharedPreferenceUtil.getToken(), isNull);
      expect(ApiService.token, isNull);
    });
  });
}
