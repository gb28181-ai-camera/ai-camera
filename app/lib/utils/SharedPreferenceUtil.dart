import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// SharedPreferences 读写门面。
///
/// 必须在 `runApp` 之前调用 [init] 完成实例预热，
/// 否则冷启动时读不到 token 会直接落到登录页。
class SharedPreferenceUtil {
  SharedPreferenceUtil._();

  /// 单例实例，[init] 之后可用。
  static SharedPreferences? _prefs;

  // ── key 定义 ──────────────────────────────────────────────
  static const String keyToken = 'auth_token';
  static const String keyUserId = 'auth_user_id';
  static const String keyUserName = 'auth_user_name';
  static const String keyDisplayName = 'auth_display_name';
  static const String keyDataPerm = 'auth_data_perm';

  /// 预热 SharedPreferences 实例。
  ///
  /// 必须在 `runApp()` 之前 `await`，保证首帧就能同步读到登录态。
  static Future<void> init() async {
    _prefs ??= await SharedPreferences.getInstance();
  }

  /// 获取实例；未 [init] 时抛出 StateError，便于暴露调用顺序错误。
  static SharedPreferences get _instance {
    final SharedPreferences? p = _prefs;
    if (p == null) {
      throw StateError(
        'SharedPreferenceUtil.init() 必须在读取前调用（通常在 main() 中 await）。',
      );
    }
    return p;
  }

  /// 是否已完成预热。
  static bool get isReady => _prefs != null;

  /// 丢弃已缓存的实例，供单元测试在 `setMockInitialValues` 之后重新 [init]。
  ///
  /// 生产代码不应调用。
  @visibleForTesting
  static void resetForTest() => _prefs = null;

  // ── token ────────────────────────────────────────────────

  /// 读取登录 token，未登录返回 null。
  static String? getToken() {
    final String? t = _instance.getString(keyToken);
    return (t == null || t.isEmpty) ? null : t;
  }

  /// 写入登录 token。
  static Future<void> setToken(String token) =>
      _instance.setString(keyToken, token);

  // ── 用户信息 ──────────────────────────────────────────────

  /// 读取用户 ID。
  static int? getUserId() => _instance.getInt(keyUserId);

  /// 读取登录名。
  static String? getUserName() => _instance.getString(keyUserName);

  /// 读取显示姓名。
  static String? getDisplayName() => _instance.getString(keyDisplayName);

  /// 读取数据权限标识。
  static String? getDataPerm() => _instance.getString(keyDataPerm);

  /// 保存登录成功后的用户信息。
  static Future<void> saveUser({
    required int userId,
    required String userName,
    required String name,
    String? dataPerm,
  }) async {
    await _instance.setInt(keyUserId, userId);
    await _instance.setString(keyUserName, userName);
    await _instance.setString(keyDisplayName, name);
    if (dataPerm != null) {
      await _instance.setString(keyDataPerm, dataPerm);
    }
  }

  /// 更新显示姓名（个人资料修改后调用）。
  static Future<void> setDisplayName(String name) =>
      _instance.setString(keyDisplayName, name);

  // ── 清理 ─────────────────────────────────────────────────

  /// 清除登录态（token + 用户信息）。
  static Future<void> clearAuth() async {
    await _instance.remove(keyToken);
    await _instance.remove(keyUserId);
    await _instance.remove(keyUserName);
    await _instance.remove(keyDisplayName);
    await _instance.remove(keyDataPerm);
  }

  /// 清空全部本地数据（调试用）。
  static Future<void> clearAll() => _instance.clear();
}
