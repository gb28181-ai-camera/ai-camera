import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../models/index.dart';
import '../pages/alarm/alarm_handle_page.dart';
import '../pages/alarm/alarm_list_page.dart';
import '../pages/device/device_detail_page.dart';
import '../pages/device/device_list_page.dart';
// ⚠️ 临时：播放器共存 Spike 页面（见 player_spike_page.dart 顶部说明），验证完连同
// 下方 `/spike/player` 路由一并删除。
import '../pages/device/player_spike_page.dart';
import '../pages/gb/gb_alarm_page.dart';
import '../pages/gb/gb_device_list_page.dart';
import '../pages/gb/gb_live_page.dart';
import '../pages/gb/gb_playback_page.dart';
import '../pages/home/home_page.dart';
import '../pages/inspect/inspect_task_detail_page.dart';
import '../pages/inspect/inspect_task_list_page.dart';
import '../pages/iot/add_device_page.dart';
import '../pages/iot/ble_provision_page.dart';
import '../pages/iot/iot_device_page.dart';
import '../pages/iot/scan_claim_page.dart';
import '../pages/iot/share_device_page.dart';
import '../pages/login/login_page.dart';
import '../pages/notification/notification_list_page.dart';
import '../pages/profile/profile_page.dart';
import '../pages/risk/risk_detail_page.dart';
import '../pages/risk/risk_handle_page.dart';
import '../pages/risk/risk_list_page.dart';
import '../pages/todos/todo_list_page.dart';
import '../pages/voice/VoiceChatPage.dart';
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

        // ── 语音聊天（v0.6.0）：半双工 PCM16 对话 ──
        GoRoute(
          path: '/voice',
          name: 'voiceChat',
          builder: (BuildContext context, GoRouterState state) =>
              const VoiceChatPage(),
        ),

        // ── 国标监控（GB/T 28181）：设备/通道浏览 → 实时播放 ──
        GoRoute(
          path: '/gb/devices',
          name: 'gbDevices',
          builder: (BuildContext context, GoRouterState state) =>
              const GbDeviceListPage(),
        ),
        GoRoute(
          path: '/gb/live',
          name: 'gbLive',
          builder: (BuildContext context, GoRouterState state) =>
              const GbLivePage(),
        ),
        // 录像回放：可带 ?channelId=&name= 直接进入；不带参数时页面内选择通道。
        GoRoute(
          path: '/gb/playback',
          name: 'gbPlayback',
          builder: (BuildContext context, GoRouterState state) =>
              const GbPlaybackPage(),
        ),
        // AI 监测事件（国标报警，只读）。
        GoRoute(
          path: '/gb/alarms',
          name: 'gbAlarms',
          builder: (BuildContext context, GoRouterState state) =>
              const GbAlarmPage(),
        ),

        // ── IOT 设备接入：添加（蓝牙配网 / 扫码）→ 我的设备 ──
        GoRoute(
          path: '/iot/add',
          name: 'iotAdd',
          builder: (BuildContext context, GoRouterState state) =>
              const AddDevicePage(),
        ),
        GoRoute(
          path: '/iot/ble',
          name: 'iotBle',
          builder: (BuildContext context, GoRouterState state) =>
              BleProvisionPage(
            // 从扫码页带入分享令牌时，配网 bind 指令会把它一并发给设备。
            token: state.uri.queryParameters['token'] ?? '',
          ),
        ),
        GoRoute(
          path: '/iot/scan',
          name: 'iotScan',
          builder: (BuildContext context, GoRouterState state) =>
              const ScanClaimPage(),
        ),
        GoRoute(
          path: '/iot/devices',
          name: 'iotDevices',
          builder: (BuildContext context, GoRouterState state) =>
              const IotDevicePage(),
        ),
        // 设备分享：仅适老化相机的 owner 可进入（后端同样以 owner 校验）。
        // deviceId 必填，name 只用于展示。
        GoRoute(
          path: '/iot/share',
          name: 'iotShare',
          builder: (BuildContext context, GoRouterState state) =>
              ShareDevicePage(
            deviceId: int.tryParse(
                  state.uri.queryParameters['deviceId'] ?? '',
                ) ??
                0,
            deviceName: state.uri.queryParameters['name'] ?? '',
          ),
        ),

        // ⚠️ 临时：播放器共存 Spike（验证完连同 import 一并删除）。
        GoRoute(
          path: '/spike/player',
          name: 'playerSpike',
          builder: (BuildContext context, GoRouterState state) =>
              const PlayerSpikePage(),
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
