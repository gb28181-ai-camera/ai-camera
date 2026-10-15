import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'SharedPreferenceUtil.dart';

/// 全局导航门面。
///
/// 供**非 Widget 上下文**（`ApiService` 拦截 401、`AuthModel` 退出登录）使用；
/// Widget 内部请优先使用 `context.go/push`。
class AppNavigator {
  AppNavigator._();

  /// 绑定的 GoRouter 实例，由 `app_router.dart` 在创建时注入。
  static GoRouter? router;

  /// 根 Navigator 的 key，用于在无 context 时弹出对话框 / 提示。
  static final GlobalKey<NavigatorState> navigatorKey =
      GlobalKey<NavigatorState>(debugLabel: 'appNavigatorKey');

  /// 当前是否已绑定路由。
  static bool get isReady => router != null;

  /// 是否有可用的 Navigator 上下文。
  static BuildContext? get context => navigatorKey.currentContext;

  /// 跳转到指定路径。
  static void go(String location, {Object? extra}) {
    final GoRouter? r = router;
    if (r == null) return;
    r.go(location, extra: extra);
  }

  /// 压栈到指定路径。
  static void push(String location, {Object? extra}) {
    final GoRouter? r = router;
    if (r == null) return;
    r.push(location, extra: extra);
  }

  /// 返回上一页；无上一页时回到首页。
  static void back() {
    final GoRouter? r = router;
    if (r == null) return;
    if (r.canPop()) {
      r.pop();
    } else {
      r.go('/');
    }
  }

  /// 登录态失效处理：清 token → 跳转登录页。
  ///
  /// `ApiService` 收到 401 时调用。清理由 AuthModel 的 `logout` effect 完成，
  /// 因此这里只负责导航；`SharedPreferenceUtil.clearAuth()` 会先行执行，
  /// 保证即使后续导航失败也不会带着脏 token 重试。
  static Future<void> toLoginPage() async {
    if (SharedPreferenceUtil.isReady) {
      await SharedPreferenceUtil.clearAuth();
    }
    final GoRouter? r = router;
    if (r == null) return;
    // 已在登录页则不重复跳转，避免 redirect 循环。
    final String current = r.state.matchedLocation;
    if (current == '/login') return;
    r.go('/login');
  }

  /// 弹出根级提示（无 context 场景）。
  static void showToast(String message) {
    final BuildContext? ctx = navigatorKey.currentContext;
    if (ctx == null) return;
    ScaffoldMessenger.of(ctx)
      ..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(
          content: Text(message),
          duration: const Duration(seconds: 2),
          margin: const EdgeInsets.all(16),
        ),
      );
  }
}
