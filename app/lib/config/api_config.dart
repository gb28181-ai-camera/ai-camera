import 'package:flutter/foundation.dart';

/// 后端接口地址配置。
///
/// 优先级：`--dart-define=API_BASE_URL=...` > 平台默认值。
class ApiConfig {
  ApiConfig._();

  /// 后端服务基地址。
  ///
  /// - 安卓模拟器内 `10.0.2.2` 指向宿主机的 `localhost`；
  /// - iOS 模拟器 / 桌面 / Web 直接用 `localhost`；
  /// - 真机调试必须改成宿主机的局域网 IP，例如：
  ///   `flutter run --dart-define=API_BASE_URL=http://192.168.1.100:8080`
  static String get baseUrl {
    const String override = String.fromEnvironment('API_BASE_URL');
    if (override.isNotEmpty) {
      return override;
    }
    if (!kIsWeb && defaultTargetPlatform == TargetPlatform.android) {
      return 'http://10.0.2.2:8080';
    }
    return 'http://localhost:8080';
  }

  /// 建立连接超时时间。
  static const Duration connectTimeout = Duration(seconds: 10);

  /// 接收响应超时时间。
  static const Duration receiveTimeout = Duration(seconds: 15);

  /// 发送请求超时时间。
  static const Duration sendTimeout = Duration(seconds: 15);

  /// 应用版本号（登录页底部展示）。
  static const String appVersion = 'v1.0.0';

  // ── 路由前缀 ──────────────────────────────────────────────
  /// 移动端接口前缀。
  static const String appPrefix = '/app';
}
