import 'package:dio/dio.dart';

import '../config/api_config.dart';
import '../utils/AppNavigator.dart';
import '../utils/SharedPreferenceUtil.dart';

/// 全局 HTTP 门面。
///
/// 约定：
/// - 所有请求经由本类的静态方法发出，**不抛异常**，一律返回 `dio.Response`；
/// - 统一在 [getHeaders] 注入 `Authorization: Bearer <token>`；
/// - 统一在 [_handleResponse] 处理业务错误：401 清登录态并跳登录页，其余非 200 弹提示。
///
/// 生成代码（`lib/services/generated/*Controller.dart`）只调用本类，不直接持有 Dio。
class ApiService {
  ApiService._();

  /// 底层 Dio 实例。超时、baseUrl 均在 [BaseOptions] 中真正生效。
  ///
  /// 单元测试可直接赋值注入 mock adapter，如：
  /// `ApiService.dio = Dio(BaseOptions(baseUrl: 'http://test'))..httpClientAdapter = adapter;`
  static Dio dio = _createDio();

  /// 按 [ApiConfig] 构造 Dio 实例（超时参数在此真正传入 BaseOptions）。
  static Dio _createDio() {
    return Dio(
      BaseOptions(
        baseUrl: ApiConfig.baseUrl,
        connectTimeout: ApiConfig.connectTimeout,
        receiveTimeout: ApiConfig.receiveTimeout,
        sendTimeout: ApiConfig.sendTimeout,
        responseType: ResponseType.json,
        // 允许 4xx 进入业务处理，便于统一读取后端 401 的 JSON body
        validateStatus: (int? status) => status != null && status < 500,
        headers: <String, dynamic>{
          'Accept': 'application/json',
          'Content-Type': 'application/json',
        },
      ),
    );
  }

  /// 恢复为默认配置的 Dio 实例（测试 tearDown 用）。
  static void resetDio() {
    dio = _createDio();
  }

  // ── token 管理 ────────────────────────────────────────────

  /// 内存中的 token 缓存。热路径（每次请求）先读它，避免反复访问磁盘。
  static String? _tokenCache;

  /// 当前 token：内存缓存优先，未命中时回落到 SharedPreferences 并回填缓存。
  static String? get token {
    final String? cached = _tokenCache;
    if (cached != null && cached.isNotEmpty) return cached;
    if (!SharedPreferenceUtil.isReady) return null;
    final String? stored = SharedPreferenceUtil.getToken();
    _tokenCache = stored;
    return stored;
  }

  /// 设置内存 token（登录成功后调用；持久化由 [AuthModel] 负责）。
  static void setToken(String? value) => _tokenCache = value;

  /// 清除内存 token 与本地登录态。
  static Future<void> clearToken() async {
    _tokenCache = null;
    if (SharedPreferenceUtil.isReady) {
      await SharedPreferenceUtil.clearAuth();
    }
  }

  /// 是否已登录。
  static bool get isLoggedIn => (token ?? '').isNotEmpty;

  // ── 请求头 ────────────────────────────────────────────────

  /// 构造请求头，注入 Bearer token。
  ///
  /// [headers] 中的同名字段优先（便于个别接口覆盖 Content-Type）。
  static Map<String, dynamic> getHeaders([Map<String, dynamic>? headers]) {
    final Map<String, dynamic> result = <String, dynamic>{
      'Accept': 'application/json',
      'Content-Type': 'application/json',
    };
    final String? t = token;
    if (t != null && t.isNotEmpty) {
      result['Authorization'] = 'Bearer $t';
    }
    if (headers != null) {
      result.addAll(headers);
    }
    return result;
  }

  /// 构造 GET 查询参数（剔除 null / 空串，避免后端把空串当成筛选条件）。
  static Map<String, dynamic> cleanQuery(Map<String, dynamic>? query) {
    final Map<String, dynamic> result = <String, dynamic>{};
    query?.forEach((String k, dynamic v) {
      if (v == null) return;
      if (v is String && v.isEmpty) return;
      result[k] = v;
    });
    return result;
  }

  // ── HTTP 方法 ─────────────────────────────────────────────

  /// GET 请求。
  static Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? query,
    Map<String, dynamic>? headers,
  }) async {
    return _request(() => dio.get<dynamic>(
          path,
          queryParameters: cleanQuery(query),
          options: Options(headers: getHeaders(headers)),
        ));
  }

  /// POST 请求。
  static Future<Response<dynamic>> post(
    String path, [
    Map<String, dynamic>? query,
    dynamic body,
    Map<String, dynamic>? headers,
  ]) async {
    return _request(() => dio.post<dynamic>(
          path,
          queryParameters: cleanQuery(query),
          data: body,
          options: Options(headers: getHeaders(headers)),
        ));
  }

  /// PUT 请求。
  static Future<Response<dynamic>> put(
    String path, [
    Map<String, dynamic>? query,
    dynamic body,
    Map<String, dynamic>? headers,
  ]) async {
    return _request(() => dio.put<dynamic>(
          path,
          queryParameters: cleanQuery(query),
          data: body,
          options: Options(headers: getHeaders(headers)),
        ));
  }

  /// DELETE 请求。
  static Future<Response<dynamic>> delete(
    String path, [
    Map<String, dynamic>? query,
    dynamic body,
    Map<String, dynamic>? headers,
  ]) async {
    return _request(() => dio.delete<dynamic>(
          path,
          queryParameters: cleanQuery(query),
          data: body,
          options: Options(headers: getHeaders(headers)),
        ));
  }

  // ── 内部实现 ──────────────────────────────────────────────

  /// 正在执行 401 跳转，避免并发请求触发多次跳转 / 多次提示。
  static bool _redirecting = false;

  /// 执行请求并统一处理错误，保证不向外抛异常。
  static Future<Response<dynamic>> _request(
    Future<Response<dynamic>> Function() send,
  ) async {
    try {
      final Response<dynamic> response = await send();
      await _handleResponse(response);
      return response;
    } on DioException catch (e) {
      return _handleDioException(e);
    } catch (e) {
      _toast('请求失败：$e');
      return _syntheticResponse(null, -1, '请求失败：$e');
    }
  }

  /// 统一业务错误处理：401 跳登录，其余非 200 弹提示。
  static Future<void> _handleResponse(Response<dynamic> response) async {
    final int httpStatus = response.statusCode ?? 0;
    final dynamic body = response.data;
    final int code = _bodyCode(body);

    // HTTP 401，或业务 code=401，均视为登录态失效。
    if (httpStatus == 401 || code == 401) {
      await _handleUnauthorized();
      return;
    }

    if (httpStatus >= 400) {
      _toast(_bodyMessage(body) ?? '请求失败（HTTP $httpStatus）');
      return;
    }

    // 后端统一响应体中的业务错误码。
    if (code != 0 && code != 200) {
      _toast(_bodyMessage(body) ?? '请求失败（code=$code）');
    }
  }

  /// Dio 层异常（网络不可达、超时、连接失败等）统一降级为合成响应。
  static Future<Response<dynamic>> _handleDioException(DioException e) async {
    final Response<dynamic>? raw = e.response;
    if (raw != null) {
      await _handleResponse(raw);
      return raw;
    }

    final String message = _dioExceptionMessage(e);
    _toast(message);
    return _syntheticResponse(e.requestOptions, -1, message);
  }

  /// 登录态失效：清 token → 跳登录页。并发请求只处理一次。
  static Future<void> _handleUnauthorized() async {
    if (_redirecting) return;
    _redirecting = true;
    try {
      await clearToken();
      _toast('登录已过期，请重新登录');
      await AppNavigator.toLoginPage();
    } finally {
      // 仅抑制同一次失效期间的并发请求重复跳转；
      // 不使用延时复位，避免在 widget 测试中残留未完成的 Timer。
      _redirecting = false;
    }
  }

  /// 从响应体中读取业务状态码，非 JSON 时返回 0。
  static int _bodyCode(dynamic body) {
    if (body is Map) {
      final dynamic c = body['code'];
      if (c is int) return c;
      if (c is num) return c.toInt();
      if (c is String) return int.tryParse(c) ?? 0;
    }
    return 0;
  }

  /// 从响应体中读取 message。
  static String? _bodyMessage(dynamic body) {
    if (body is Map) {
      final dynamic m = body['message'];
      if (m is String && m.isNotEmpty) return m;
    }
    return null;
  }

  /// DioException → 中文提示。
  static String _dioExceptionMessage(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionTimeout:
        return '连接超时，请检查网络';
      case DioExceptionType.sendTimeout:
        return '请求发送超时';
      case DioExceptionType.receiveTimeout:
        return '响应超时，请稍后重试';
      case DioExceptionType.badCertificate:
        return '证书校验失败';
      case DioExceptionType.badResponse:
        return '服务异常（${e.response?.statusCode ?? '--'}）';
      case DioExceptionType.cancel:
        return '请求已取消';
      case DioExceptionType.connectionError:
        return '无法连接服务器，请检查网络';
      case DioExceptionType.transformTimeout:
        return '数据处理超时';
      case DioExceptionType.unknown:
        return '网络异常：${e.message ?? '未知错误'}';
    }
  }

  /// 合成一个响应对象，让上层可用统一逻辑处理失败（不抛异常）。
  static Response<dynamic> _syntheticResponse(
    RequestOptions? options,
    int code,
    String message,
  ) {
    return Response<dynamic>(
      requestOptions: options ??
          RequestOptions(
            path: '',
            baseUrl: ApiConfig.baseUrl,
          ),
      statusCode: code > 0 ? code : 0,
      data: <String, dynamic>{'code': code, 'message': message, 'data': null},
    );
  }

  /// 弹提示；无 UI 上下文（如单测）时静默忽略。
  static void _toast(String message) {
    AppNavigator.showToast(message);
  }
}
