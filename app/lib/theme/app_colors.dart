import 'package:flutter/material.dart';

/// 全局设计令牌 —— 颜色。
///
/// 移动端仅有一套浅色主题，所有颜色集中在此，禁止在页面中硬编码色值。
class AppColors {
  AppColors._();

  // ── 品牌主色 ──────────────────────────────────────────────
  /// 主色（品牌蓝），用于主按钮、选中态、强调文字。
  static const Color primary = Color(0xFF2B7FFF);
  static const Color primaryLight = Color(0xFFE8F1FF);
  static const Color primaryDark = Color(0xFF1A5FD0);

  // ── 背景与容器 ────────────────────────────────────────────
  /// 页面背景色。
  static const Color background = Color(0xFFF5F7FA);
  /// 卡片 / 弹层背景色。
  static const Color card = Color(0xFFFFFFFF);
  /// 分割线。
  static const Color divider = Color(0xFFEDEFF2);
  /// 输入框 / 次级容器的填充色。
  static const Color fill = Color(0xFFF5F7FA);

  // ── 文字 ──────────────────────────────────────────────────
  static const Color textPrimary = Color(0xFF1D2129);
  static const Color textSecondary = Color(0xFF4E5969);
  static const Color textTertiary = Color(0xFF86909C);
  static const Color textDisabled = Color(0xFFC9CDD4);
  static const Color textOnPrimary = Color(0xFFFFFFFF);

  // ── 语义色 ────────────────────────────────────────────────
  static const Color success = Color(0xFF00B42A);
  static const Color successLight = Color(0xFFE8FFEA);
  static const Color warning = Color(0xFFFF7D00);
  static const Color warningLight = Color(0xFFFFF7E8);
  static const Color danger = Color(0xFFF53F3F);
  static const Color dangerLight = Color(0xFFFFECE8);
  static const Color info = Color(0xFF2B7FFF);

  /// 离线 / 禁用态的灰。
  static const Color offline = Color(0xFFC9CDD4);

  // ── 风险 / 告警等级 ────────────────────────────────────────
  /// 风险等级 high / medium / low 对应的配色。
  static const Map<String, Color> levelColors = <String, Color>{
    'high': danger,
    'medium': warning,
    'low': primary,
  };

  /// 取风险等级颜色，未知等级回退为灰色。
  static Color levelColor(String? level) =>
      levelColors[level] ?? textTertiary;

  /// 处理状态 pending / handled 对应的配色。
  static Color statusColor(String? status) =>
      status == 'handled' ? success : warning;

  /// 设备运行状态 online / offline 对应的配色。
  static Color deviceStatusColor(String? status) =>
      status == 'online' ? success : offline;
}
