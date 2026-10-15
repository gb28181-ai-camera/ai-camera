import 'package:flutter/material.dart';

import '../constants/Screen.dart';
import 'app_colors.dart';

/// 全局设计令牌 —— 文字样式。
///
/// 命名规则：`AppText.<用途><字号>`，页面中禁止直接 new TextStyle。
///
/// **字号随设计稿等比缩放**：下面注释里的字号是**设计稿（414 宽）上的值**，
/// 实际取值经 [Screen.sizeByDesign] 换算，保证与间距、组件尺寸同比例变化，
/// 不会出现「间距缩了但字号没缩」导致的版式走样。
///
/// `height`（行高倍数）**不参与缩放** —— 它是相对字号的倍率，缩放会破坏行距语义。
///
/// 返回 getter 而非 `const`：缩放值依赖运行时屏幕尺寸，无法编译期确定。
/// 因此 `const Text('x', style: AppText.title)` 需去掉 `const`。
class AppText {
  AppText._();

  /// 按设计稿字号构造样式。`height` 为行高倍率，不缩放。
  static TextStyle _style({
    required double fontSize,
    required FontWeight weight,
    Color? color,
    double height = 1.4,
  }) =>
      TextStyle(
        fontSize: Screen.sizeByDesign(fontSize),
        fontWeight: weight,
        color: color,
        height: height,
      );

  /// 页面主标题（如详情页标题）。设计稿 20。
  static TextStyle get title =>
      _style(fontSize: 20, weight: FontWeight.w600, color: AppColors.textPrimary);

  /// 区块标题（卡片头部）。设计稿 16。
  static TextStyle get section =>
      _style(fontSize: 16, weight: FontWeight.w600, color: AppColors.textPrimary);

  /// 正文（默认阅读字号）。设计稿 14。
  static TextStyle get body => _style(
        fontSize: 14,
        weight: FontWeight.w400,
        color: AppColors.textPrimary,
        height: 1.5,
      );

  /// 正文加粗（列表条目标题）。设计稿 15。
  static TextStyle get bodyStrong =>
      _style(fontSize: 15, weight: FontWeight.w600, color: AppColors.textPrimary);

  /// 次要说明文字。设计稿 13。
  static TextStyle get caption => _style(
        fontSize: 13,
        weight: FontWeight.w400,
        color: AppColors.textSecondary,
      );

  /// 更弱的辅助文字（时间、位置）。设计稿 12。
  static TextStyle get hint => _style(
        fontSize: 12,
        weight: FontWeight.w400,
        color: AppColors.textTertiary,
      );

  /// 统计数字（首页大号数字）。设计稿 26。
  static TextStyle get statNumber => _style(
        fontSize: 26,
        weight: FontWeight.w700,
        color: AppColors.textPrimary,
        height: 1.2,
      );

  /// 标签文字（状态徽标）。设计稿 11。
  static TextStyle get tag =>
      _style(fontSize: 11, weight: FontWeight.w500, height: 1.2);

  /// 按钮文字。设计稿 16。
  static TextStyle get button =>
      _style(fontSize: 16, weight: FontWeight.w500, height: 1.2);

  /// 空态 / 错误提示。设计稿 14。
  static TextStyle get empty => _style(
        fontSize: 14,
        weight: FontWeight.w400,
        color: AppColors.textTertiary,
        height: 1.5,
      );

  /// 与 [body] 同字号但指定颜色的通用构造，避免页面直接 new TextStyle。
  static TextStyle colored(Color color, {double fontSize = 14, FontWeight weight = FontWeight.w400}) =>
      _style(fontSize: fontSize, weight: weight, color: color);
}
