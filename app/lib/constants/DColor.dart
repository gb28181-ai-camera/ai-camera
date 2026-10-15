import 'package:flutter/material.dart';

import '../theme/app_colors.dart';

/// 颜色常量门面（沿用 boticz-mes 的 DColor 命名习惯）。
///
/// 真实取值统一定义在 [AppColors]，本类只做别名转发，
/// 保证老代码 `DColor.primary` 与新代码 `AppColors.primary` 取到同一个值。
class DColor {
  DColor._();

  /// 品牌主色。
  static const Color primary = AppColors.primary;
  /// 页面背景色。
  static const Color background = AppColors.background;
  /// 卡片背景色。
  static const Color card = AppColors.card;
  /// 分割线。
  static const Color divider = AppColors.divider;

  /// 主文字。
  static const Color textPrimary = AppColors.textPrimary;
  /// 次要文字。
  static const Color textSecondary = AppColors.textSecondary;
  /// 辅助文字。
  static const Color textTertiary = AppColors.textTertiary;
  /// 主色上的文字。
  static const Color textOnPrimary = AppColors.textOnPrimary;

  /// 成功 / 已处理。
  static const Color success = AppColors.success;
  /// 警告 / 待处理。
  static const Color warning = AppColors.warning;
  /// 危险 / 高风险。
  static const Color danger = AppColors.danger;

  /// 常用字号下的次级灰字样式。
  static const TextStyle subText = TextStyle(
    fontSize: 12,
    color: AppColors.textTertiary,
  );
}
