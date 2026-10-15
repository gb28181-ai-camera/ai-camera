/// 测试用 Dio `HttpClientAdapter` 与 DVA 引导工具。
///
/// 这些适配器让单元测试完全不依赖真实后端：
/// - [PathRoutingAdapter]：按路径后缀匹配返回预设 JSON，并记录所有请求；
/// - [FixedAdapter]：所有请求返回同一份预设 JSON；
/// - [ErrorAdapter]：始终抛出网络异常，用于验证降级路径。
library;

import 'dart:convert';

import 'package:ai_camera_app/models/index.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_dva/flutter_dva.dart';

/// 构造带 JSON content-type 的响应体。
///
/// 必须显式声明 content-type，否则 Dio 不会把响应体解码成 Map。
ResponseBody _jsonBody(Object body, int statusCode) {
  return ResponseBody.fromString(
    body is String ? body : jsonEncode(body),
    statusCode,
    headers: <String, List<String>>{
      Headers.contentTypeHeader: <String>[Headers.jsonContentType],
    },
  );
}

/// 统一构造后端成功响应。
Map<String, dynamic> wrapSuccess(Object? data) => <String, dynamic>{
      'code': 200,
      'message': 'ok',
      'data': data,
    };

/// 统一构造后端错误响应。
Map<String, dynamic> wrapError(int code, String message) => <String, dynamic>{
      'code': code,
      'message': message,
      'data': null,
    };

/// 按路径后缀路由的适配器。
///
/// [routes] 的 key 可以是完整路径（`/app/home`）或后缀匹配
/// （`/risks/1/handle`），命中第一个匹配项即返回对应 JSON。
class PathRoutingAdapter implements HttpClientAdapter {
  /// 构造路由适配器。
  PathRoutingAdapter(this.routes, {this.statusCode = 200});

  /// 路径 → 响应体（Map / List / 已是 JSON 字符串皆可）。
  final Map<String, Object> routes;

  /// 命中时返回的 HTTP 状态码。
  int statusCode;

  /// 已收到的全部请求，按顺序记录，供断言 header / query。
  final List<RequestOptions> requests = <RequestOptions>[];

  /// 最近一次请求。
  RequestOptions? get lastRequest =>
      requests.isEmpty ? null : requests.last;

  /// 命中指定路径后缀的请求次数（用于断言某端点是否被调用）。
  int countOf(String pathSuffix) =>
      requests.where((RequestOptions r) => r.path.endsWith(pathSuffix)).length;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);

    for (final MapEntry<String, Object> entry in routes.entries) {
      if (options.path == entry.key || options.path.endsWith(entry.key)) {
        return _jsonBody(entry.value, statusCode);
      }
    }

    return _jsonBody(wrapError(404, '未匹配到测试路由：${options.path}'), 404);
  }

  @override
  void close({bool force = false}) {}
}

/// 固定响应适配器：所有请求都返回同一份 JSON。
class FixedAdapter implements HttpClientAdapter {
  /// 构造固定响应适配器。
  FixedAdapter(this.body, {this.statusCode = 200});

  /// 响应体。传入 `wrapError(...)` 可模拟业务错误。
  final Object body;

  /// HTTP 状态码。
  final int statusCode;

  /// 已收到的请求。
  final List<RequestOptions> requests = <RequestOptions>[];

  /// 最近一次请求。
  RequestOptions? get lastRequest =>
      requests.isEmpty ? null : requests.last;


  /// 命中指定路径后缀的请求次数。
  int countOf(String pathSuffix) =>
      requests.where((RequestOptions r) => r.path.endsWith(pathSuffix)).length;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return _jsonBody(body, statusCode);
  }

  @override
  void close({bool force = false}) {}
}

/// 错误适配器：始终抛出 [DioException]，模拟断网 / 超时。
class ErrorAdapter implements HttpClientAdapter {
  /// 构造错误适配器。
  ErrorAdapter({
    this.type = DioExceptionType.connectionError,
    this.message = '网络不可达',
  });

  /// 异常类型。
  final DioExceptionType type;

  /// 异常信息。
  final String message;

  /// 已收到的请求。
  final List<RequestOptions> requests = <RequestOptions>[];

  /// 最近一次请求。
  RequestOptions? get lastRequest =>
      requests.isEmpty ? null : requests.last;


  /// 命中指定路径后缀的请求次数。
  int countOf(String pathSuffix) =>
      requests.where((RequestOptions r) => r.path.endsWith(pathSuffix)).length;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    throw DioException(
      requestOptions: options,
      type: type,
      message: message,
    );
  }

  @override
  void close({bool force = false}) {}
}

/// 构造一个挂了测试适配器的 [Dio] 实例。
Dio buildTestDio(HttpClientAdapter adapter) {
  return Dio(
    BaseOptions(
      baseUrl: 'http://test.local',
      validateStatus: (int? status) => status != null && status < 500,
    ),
  )..httpClientAdapter = adapter;
}

/// 引导一个仅注册了 [models] 的 DVA store，用于模型层 / 页面层测试。
///
/// 挂钩方式与 `main.dart` 保持一致（onAction + onStateChange 双挂），
/// 因此监听 `authChangeNotifier` 的页面在测试中也能正常重建。
/// 不调用 `runApp`，不会渲染任何界面。
Future<Dva> bootDva(List<Model<dynamic>> models) async {
  final Dva dva = Dva(
    DvaOpts(
      models: models,
      onAction: (DvaAction action) => authChangeNotifier.value++,
      onStateChange: (StoreOfState prev, DvaAction action) =>
          authChangeNotifier.value++,
      // flutter_dva 默认不注册 onError，reducer/effect 中的异常会被静默吞掉。
      // 测试中显式打印，避免断言失败时无从排查。
      onError: (dynamic error, Map<String, dynamic> context) {
        debugPrint('[bootDva] 未捕获异常: $error');
        debugPrint('[bootDva] action: ${context['action']}');
      },
    ),
  );
  await dva.start(() => const SizedBox.shrink(), () async {});
  return dva;
}

/// 从 store 中读取某个命名空间的强类型状态。
T readState<T>(Dva dva, String namespace) =>
    dva.store!.getStateByKey<T>(namespace);

/// 派发一个 action 并等待其完成。
Future<void> dispatchAction(Dva dva, String type, [dynamic payload]) async {
  await dva.store!.dispatch(DvaAction(type, payload));
}
