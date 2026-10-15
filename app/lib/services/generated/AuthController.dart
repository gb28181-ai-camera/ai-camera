/// 移动端认证接口（Swagger Tag: AppAuth）。
///
/// 本文件由「后端接口契约」生成风格手工维护，**禁止在页面中手写 URL 调用**，
/// 所有请求必须走本类的静态方法。
library;

import 'package:dio/dio.dart' as dio;

import '../api_service.dart';
import 'definitions/LoginReq.dart';
import 'definitions/Response.dart';

/// 认证控制器。
class AuthController {
  AuthController._();

  /// 移动端登录路径。
  static String loginUsingPostPath = '/app/auth/login';

  /// 用户登录。
  ///
  /// 与 `POST /web/auth/login` 共用登录逻辑，返回相同的 JWT 载荷
  /// （token / user_id / name / dept_id / data_perm / cross_dept_ids / permissions）。
  static Future<RLogin> loginUsingPost(
    LoginReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      loginUsingPostPath,
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return RLogin()..fromJson(response.data);
  }
}
