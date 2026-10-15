/// DVA 模型统一导出 + 全局状态访问门面。
///
/// 页面取值统一走 [AppStore]，禁止直接 `Provider().store`：
/// - `AppStore.auth` / `AppStore.menu` 在 store 未初始化时返回安全默认值；
/// - `AppStore.dispatch` 封装了 `DvaAction` 构造，调用处更简洁。
library;

import 'package:flutter_dva/flutter_dva.dart';

import 'auth_model.dart';
import 'menu_model.dart';

export 'auth_model.dart';
export 'menu_model.dart';

/// 全局状态访问门面。
class AppStore {
  AppStore._();

  /// 底层 Store；`Dva.start()` 之前为 null。
  static Store? get store => Provider().store;

  /// 是否已初始化。
  static bool get isReady => store != null;

  /// 当前登录态；未初始化时返回初始（未登录）状态。
  static AuthState get auth {
    final Store? s = store;
    if (s == null) return AuthState.initial();
    final AuthState? v = s.rootState['auth']?.getState() as AuthState?;
    return v ?? AuthState.initial();
  }

  /// 当前菜单 / 权限状态；未初始化时返回初始状态。
  static MenuState get menu {
    final Store? s = store;
    if (s == null) return MenuState.initial();
    final MenuState? v = s.rootState['menu']?.getState() as MenuState?;
    return v ?? MenuState.initial();
  }

  /// 是否已登录。
  static bool get isLoggedIn => auth.isLoggedIn;

  /// 是否拥有某权限标识。
  static bool hasPerm(String code) => menu.hasPerm(code);

  /// 派发 action，[type] 需带命名空间（如 `auth/login`）。
  static Future<dynamic> dispatch(String type, [dynamic payload]) async {
    final Store? s = store;
    if (s == null) return null;
    return s.dispatch(DvaAction(type, payload));
  }

  /// 登录。
  static Future<void> login(String userName, String password) => dispatch(
        'auth/login',
        <String, dynamic>{'user_name': userName, 'password': password},
      );

  /// 退出登录。
  static Future<void> logout() => dispatch('auth/logout');

  /// 重新加载菜单。
  static Future<void> reloadMenu() => dispatch('menu/load');
}
