import 'package:flutter/material.dart';

/// 后端菜单 `icon` 字段 → Flutter [IconData] 的映射。
///
/// 后端存的是字符串（如 `dashboard`、`camera`），移动端不引入动态图标字体，
/// 因此在此维护一份静态映射表；未知图标回退为 [Icons.circle_outlined]。
class MenuIcon {
  MenuIcon._();

  /// 默认兜底图标。
  static const IconData fallback = Icons.circle_outlined;

  /// 图标名 → IconData 映射表。
  static const Map<String, IconData> _icons = <String, IconData>{
    // 导航与看板
    'dashboard': Icons.dashboard_outlined,
    'home': Icons.home_outlined,
    'monitor': Icons.monitor_outlined,
    'screen': Icons.desktop_windows_outlined,
    'chart': Icons.bar_chart_outlined,
    'stats': Icons.insert_chart_outlined,
    'analysis': Icons.analytics_outlined,

    // 风险与告警
    'risk': Icons.warning_amber_outlined,
    'warning': Icons.warning_amber_outlined,
    'alarm': Icons.notifications_active_outlined,
    'bell': Icons.notifications_none_outlined,
    'alert': Icons.error_outline,

    // 设备与资源
    'camera': Icons.videocam_outlined,
    'device': Icons.videocam_outlined,
    'video': Icons.videocam_outlined,
    'region': Icons.location_on_outlined,
    'location': Icons.location_on_outlined,
    'map': Icons.map_outlined,
    'card': Icons.credit_card_outlined,
    'sim': Icons.sim_card_outlined,
    'voice': Icons.volume_up_outlined,
    'mode': Icons.tune_outlined,

    // 巡检
    'inspect': Icons.fact_check_outlined,
    'task': Icons.assignment_outlined,
    'todo': Icons.checklist_outlined,
    'record': Icons.history_edu_outlined,

    // 系统
    'system': Icons.settings_outlined,
    'setting': Icons.settings_outlined,
    'config': Icons.settings_outlined,
    'user': Icons.person_outline,
    'profile': Icons.person_outline,
    'role': Icons.admin_panel_settings_outlined,
    'menu': Icons.menu_open_outlined,
    'dept': Icons.account_tree_outlined,
    'post': Icons.badge_outlined,
    'log': Icons.receipt_long_outlined,
    'notification': Icons.notifications_none_outlined,

    // 升级
    'upgrade': Icons.system_update_alt_outlined,
    'firmware': Icons.memory_outlined,
    'model': Icons.psychology_outlined,

    // 基础信息
    'basic': Icons.category_outlined,
    'type': Icons.label_outline,
  };

  /// 按图标名取 [IconData]，命中不到时返回 [fallback]。
  static IconData resolve(String? name) {
    if (name == null || name.isEmpty) return fallback;
    return _icons[name.trim().toLowerCase()] ?? fallback;
  }

  /// 该图标名是否已登记。
  static bool contains(String? name) =>
      name != null && _icons.containsKey(name.trim().toLowerCase());

  /// 已登记的图标名清单（用于测试与调试）。
  static List<String> get supportedNames => _icons.keys.toList(growable: false);
}
