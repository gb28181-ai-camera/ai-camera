import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../models/index.dart';
import '../pages/alarm/alarm_handle_page.dart';
import '../pages/alarm/alarm_list_page.dart';
import '../pages/device/device_detail_page.dart';
import '../pages/device/device_list_page.dart';
import '../pages/home/home_page.dart';
import '../pages/inspect/inspect_task_detail_page.dart';
import '../pages/inspect/inspect_task_list_page.dart';
import '../pages/login/login_page.dart';
import '../pages/notification/notification_list_page.dart';
import '../pages/profile/profile_page.dart';
import '../pages/risk/risk_detail_page.dart';
import '../pages/risk/risk_handle_page.dart';
import '../pages/risk/risk_list_page.dart';
import '../pages/todos/todo_list_page.dart';
import '../utils/AppNavigator.dart';

/// 应用路由表。
///
/// 结构：
/// - `/login` 独立于 Shell 之外；
/// - `ShellRoute` 包裹 5 个底部 Tab（首页 / 待办 / 风险 / 设备 / 我的）；
/// - 其余页面（详情、处理、告警、通知等）为独立 `GoRoute`，
///   详情页通过 `?id=` 查询参数传参，用 `int.tryParse` 解析。
///
/// 登录态由 [authChangeNotifier] 驱动 `refreshListenable`，
/// 配合 [redirect] 实现未登录强制跳登录、已登录访问 `/login` 自动回首页。
class AppRouter {
  AppRouter._();

  /// 底部 Tab 的路由路径（顺序即导航栏顺序）。
  static const List<String> tabPaths = <String>[
    '/',
    '/todos',
    '/risks',
    '/devices',
    '/profile',
  ];

  /// 底部 Tab 文案。
  static const List<String> tabLabels = <String>[
    '首页',
    '待办',
    '风险',
    '设备',
    '我的',
  ];

  /// 底部 Tab 图标。
  static const List<IconData> tabIcons = <IconData>[
    Icons.dashboard_outlined,
    Icons.checklist_outlined,
    Icons.warning_amber_outlined,
    Icons.videocam_outlined,
    Icons.person_outline,
  ];

  /// 底部 Tab 选中态图标。
  static const List<IconData> tabActiveIcons = <IconData>[
    Icons.dashboard,
    Icons.checklist,
    Icons.warning_amber,
    Icons.videocam,
    Icons.person,
  ];

  static GoRouter? _router;

  /// 全局唯一的路由实例（懒加载）。
  static GoRouter get router => _router ??= _create();

  /// 供测试重置单例。
  static void reset() => _router = null;

  /// 构建路由表。
  static GoRouter _create() {
    final GoRouter r = GoRouter(
      initialLocation: '/',
      navigatorKey: AppNavigator.navigatorKey,
      refreshListenable: authChangeNotifier,
      debugLogDiagnostics: false,
      redirect: _redirect,
      routes: <RouteBase>[
        // ── 登录（无底部导航） ──
        GoRoute(
          path: '/login',
          name: 'login',
          builder: (BuildContext context, GoRouterState state) =>
              const LoginPage(),
        ),

        // ── 底部 5 Tab ──
        ShellRoute(
          builder: (
            BuildContext context,
            GoRouterState state,
            Widget child,
          ) =>
              MainShell(state: state, child: child),
          routes: <RouteBase>[
            GoRoute(
              path: '/',
              name: 'home',
              builder: (BuildContext context, GoRouterState state) =>
                  const HomePage(),
            ),
            GoRoute(
              path: '/todos',
              name: 'todos',
              builder: (BuildContext context, GoRouterState state) =>
                  const TodoListPage(),
            ),
            GoRoute(
              path: '/risks',
              name: 'risks',
              builder: (BuildContext context, GoRouterState state) =>
                  const RiskListPage(),
            ),
            GoRoute(
              path: '/devices',
              name: 'devices',
              builder: (BuildContext context, GoRouterState state) =>
                  const DeviceListPage(),
            ),
            GoRoute(
              path: '/profile',
              name: 'profile',
              builder: (BuildContext context, GoRouterState state) =>
                  const ProfilePage(),
            ),
          ],
        ),

        // ── 独立页面（详情 / 处理 / 次级列表） ──
        GoRoute(
          path: '/risks/detail',
          name: 'riskDetail',
          builder: (BuildContext context, GoRouterState state) =>
              const RiskDetailPage(),
        ),
        GoRoute(
          path: '/risks/handle',
          name: 'riskHandle',
          builder: (BuildContext context, GoRouterState state) =>
              const RiskHandlePage(),
        ),
        GoRoute(
          path: '/alarms',
          name: 'alarms',
          builder: (BuildContext context, GoRouterState state) =>
              const AlarmListPage(),
        ),
        GoRoute(
          path: '/alarms/handle',
          name: 'alarmHandle',
          builder: (BuildContext context, GoRouterState state) =>
              const AlarmHandlePage(),
        ),
        GoRoute(
          path: '/inspect-tasks',
          name: 'inspectTasks',
          builder: (BuildContext context, GoRouterState state) =>
              const InspectTaskListPage(),
        ),
        GoRoute(
          path: '/inspect-tasks/detail',
          name: 'inspectTaskDetail',
          builder: (BuildContext context, GoRouterState state) =>
              const InspectTaskDetailPage(),
        ),
        GoRoute(
          path: '/devices/detail',
          name: 'deviceDetail',
          builder: (BuildContext context, GoRouterState state) =>
              const DeviceDetailPage(),
        ),
        GoRoute(
          path: '/notifications',
          name: 'notifications',
          builder: (BuildContext context, GoRouterState state) =>
              const NotificationListPage(),
        ),
      ],
      errorBuilder: (BuildContext context, GoRouterState state) => Scaffold(
        appBar: AppBar(title: const Text('页面不存在')),
        body: Center(
          child: Text('未找到路由：${state.uri}'),
        ),
      ),
    );

    // 供非 Widget 上下文（401 处理）使用。
    AppNavigator.router = r;
    return r;
  }

  /// 登录态重定向。
  static String? _redirect(BuildContext context, GoRouterState state) {
    final bool loggedIn = AppStore.isLoggedIn;
    final bool atLogin = state.matchedLocation == '/login';

    // 未登录且不在登录页 → 强制登录。
    if (!loggedIn && !atLogin) return '/login';
    // 已登录仍访问登录页 → 回首页。
    if (loggedIn && atLogin) return '/';
    return null;
  }

  /// 解析 `?id=` 查询参数为 int，非法返回 null。
  static int? parseId(GoRouterState state) =>
      int.tryParse(state.uri.queryParameters['id'] ?? '');
}

/// 底部导航外壳。
class MainShell extends StatelessWidget {
  /// 构造外壳。
  const MainShell({super.key, required this.state, required this.child});

  /// 当前路由状态（用于计算选中项）。
  final GoRouterState state;

  /// 子页面。
  final Widget child;

  /// 当前选中的 Tab 下标。
  int get _index {
    final String location = state.matchedLocation;
    // 从后往前匹配，保证 `/devices/detail` 之类前缀不误判。
    for (int i = AppRouter.tabPaths.length - 1; i >= 0; i--) {
      final String path = AppRouter.tabPaths[i];
      if (path == '/') {
        if (location == '/') return i;
        continue;
      }
      if (location == path) return i;
    }
    return 0;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: child,
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _index,
        onTap: (int i) => context.go(AppRouter.tabPaths[i]),
        items: <BottomNavigationBarItem>[
          for (int i = 0; i < AppRouter.tabPaths.length; i++)
            BottomNavigationBarItem(
              icon: Icon(
                AppRouter.tabIcons[i],
                size: 22,
              ),
              activeIcon: Icon(
                AppRouter.tabActiveIcons[i],
                size: 22,
              ),
              label: AppRouter.tabLabels[i],
            ),
        ],
      ),
    );
  }
}
