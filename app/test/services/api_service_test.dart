import 'package:ai_camera_app/config/api_config.dart';
import 'package:ai_camera_app/services/api_service.dart';
import 'package:ai_camera_app/utils/SharedPreferenceUtil.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../test_dio_adapter.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() async {
    SharedPreferences.setMockInitialValues(<String, Object>{});
    SharedPreferenceUtil.resetForTest();
    await SharedPreferenceUtil.init();
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  group('ApiService 初始化', () {
    test('baseUrl 与超时参数真正传入 BaseOptions', () {
      final BaseOptions opt = ApiService.dio.options;

      expect(opt.baseUrl, ApiConfig.baseUrl);
      // 这是 boticz-mes 踩过的坑：声明了超时常量却没传给 BaseOptions。
      expect(opt.connectTimeout, ApiConfig.connectTimeout);
      expect(opt.receiveTimeout, ApiConfig.receiveTimeout);
      expect(opt.sendTimeout, ApiConfig.sendTimeout);
      expect(opt.connectTimeout, const Duration(seconds: 10));
      expect(opt.receiveTimeout, const Duration(seconds: 15));
    });
  });

  group('ApiService 请求头', () {
    test('注入内存缓存中的 Bearer token', () async {
      ApiService.setToken('tk-memory');
      final PathRoutingAdapter adapter = PathRoutingAdapter(
        <String, Object>{'/app/home': wrapSuccess(<String, dynamic>{})},
      );
      ApiService.dio = buildTestDio(adapter);

      final Response<dynamic> resp = await ApiService.get('/app/home');

      expect(resp.statusCode, 200);
      expect(adapter.requests, hasLength(1));
      expect(
        adapter.lastRequest!.headers['Authorization'],
        'Bearer tk-memory',
      );
    });

    test('内存缓存为空时回落到 SharedPreferences 取 token', () async {
      SharedPreferences.setMockInitialValues(<String, Object>{
        'auth_token': 'tk-stored',
      });
      SharedPreferenceUtil.resetForTest();
      await SharedPreferenceUtil.init();
      // 显式清空内存缓存，强制走磁盘兜底分支。
      ApiService.setToken(null);

      final FixedAdapter adapter = FixedAdapter(
        wrapSuccess(<String, dynamic>{}),
      );
      ApiService.dio = buildTestDio(adapter);

      await ApiService.get('/app/profile');

      expect(
        adapter.lastRequest!.headers['Authorization'],
        'Bearer tk-stored',
      );
      expect(ApiService.token, 'tk-stored');
    });

    test('未登录时不携带 Authorization 头', () async {
      final FixedAdapter adapter = FixedAdapter(
        wrapSuccess(<String, dynamic>{}),
      );
      ApiService.dio = buildTestDio(adapter);

      await ApiService.get('/app/home');

      expect(adapter.lastRequest!.headers.containsKey('Authorization'), isFalse);
      expect(ApiService.isLoggedIn, isFalse);
    });

    test('自定义 header 覆盖默认值，且查询参数剔除 null / 空串', () async {
      ApiService.setToken('tk-x');
      final FixedAdapter adapter = FixedAdapter(
        wrapSuccess(<String, dynamic>{}),
      );
      ApiService.dio = buildTestDio(adapter);

      await ApiService.get(
        '/app/risks',
        query: <String, dynamic>{
          'page': 1,
          'status': 'pending',
          'keyword': '',
          'region_id': null,
        },
        headers: <String, dynamic>{'X-Trace-Id': 'trace-1'},
      );

      final RequestOptions req = adapter.lastRequest!;
      expect(req.headers['X-Trace-Id'], 'trace-1');
      expect(req.queryParameters, <String, dynamic>{
        'page': 1,
        'status': 'pending',
      });
    });
  });

  group('ApiService 错误处理', () {
    test('业务 code=401：清空 token 且不抛异常', () async {
      ApiService.setToken('tk-expired');
      await SharedPreferenceUtil.setToken('tk-expired');

      final FixedAdapter adapter = FixedAdapter(
        wrapError(401, '登录已过期'),
      );
      ApiService.dio = buildTestDio(adapter);

      // 不抛异常，返回原始响应。
      final Response<dynamic> resp = await ApiService.get('/app/home');

      expect(resp.statusCode, 200);
      expect((resp.data as Map<String, dynamic>)['code'], 401);
      // 登录态已清理（内存 + 磁盘）。
      expect(ApiService.token, isNull);
      expect(SharedPreferenceUtil.getToken(), isNull);
    });

    test('HTTP 401：同样触发登录态清理', () async {
      ApiService.setToken('tk-http-expired');
      final FixedAdapter adapter = FixedAdapter(
        wrapError(401, 'unauthorized'),
        statusCode: 401,
      );
      ApiService.dio = buildTestDio(adapter);

      final Response<dynamic> resp = await ApiService.get('/app/home');

      expect(resp.statusCode, 401);
      expect(ApiService.token, isNull);
    });

    test('HTTP 500：返回响应而不抛出', () async {
      final FixedAdapter adapter = FixedAdapter(
        <String, dynamic>{'code': 500, 'message': '服务器内部错误'},
        statusCode: 500,
      );
      ApiService.dio = buildTestDio(adapter);

      final Response<dynamic> resp = await ApiService.get('/app/home');

      expect(resp.statusCode, 500);
    });

    test('网络异常（断网）：降级为合成响应，code = -1', () async {
      final ErrorAdapter adapter = ErrorAdapter(
        type: DioExceptionType.connectionError,
        message: '网络不可达',
      );
      ApiService.dio = buildTestDio(adapter);

      final Response<dynamic> resp = await ApiService.get('/app/home');

      expect(resp.statusCode, 0);
      final Map<String, dynamic> body = resp.data as Map<String, dynamic>;
      expect(body['code'], -1);
      expect(body['message'], contains('无法连接服务器'));
      // 请求确实发出过。
      expect(adapter.requests, hasLength(1));
    });

    test('超时异常：提示文案随异常类型变化', () async {
      final ErrorAdapter adapter = ErrorAdapter(
        type: DioExceptionType.receiveTimeout,
      );
      ApiService.dio = buildTestDio(adapter);

      final Response<dynamic> resp = await ApiService.post(
        '/app/auth/login',
        <String, dynamic>{},
        <String, dynamic>{'user_name': 'admin', 'password': 'x'},
      );

      final Map<String, dynamic> body = resp.data as Map<String, dynamic>;
      expect(body['message'], '响应超时，请稍后重试');
    });
  });

  group('ApiService 工具方法', () {
    test('clearToken 同时清理内存与 SharedPreferences', () async {
      ApiService.setToken('tk-1');
      await SharedPreferenceUtil.setToken('tk-1');
      expect(SharedPreferenceUtil.getToken(), 'tk-1');

      await ApiService.clearToken();

      expect(ApiService.token, isNull);
      expect(SharedPreferenceUtil.getToken(), isNull);
    });

    test('cleanQuery 剔除 null 与空串，保留 0 与 false', () {
      final Map<String, dynamic> cleaned = ApiService.cleanQuery(
        <String, dynamic>{
          'a': null,
          'b': '',
          'c': 0,
          'd': false,
          'e': 'x',
        },
      );
      expect(cleaned.containsKey('a'), isFalse);
      expect(cleaned.containsKey('b'), isFalse);
      expect(cleaned['c'], 0);
      expect(cleaned['d'], false);
      expect(cleaned['e'], 'x');
    });
  });
}
