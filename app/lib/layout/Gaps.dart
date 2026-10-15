import 'package:flutter/widgets.dart';

import '../constants/Screen.dart';

/// 间距设计令牌（已按设计稿等比缩放）。
///
/// 命名规则 `gap_dpN` 中的 `N` 是**设计稿上的尺寸**，取用时已是当前设备的等比尺寸。
/// 页面代码优先使用 `theme/app_spacing.dart` 的语义化令牌（`AppSpacing.lg` 等），
/// 仅当语义令牌不覆盖时才直接取 [Dimens]。
///
/// 与 boticz-mes `frontend_app/lib/layout/Gaps.dart` 保持一致：
/// 参考实现把值算在 `static double` 上（类加载时一次性求值），
/// 本实现改为 getter，屏幕尺寸变化后能立即反映新值（代价是一次除法）。
class Dimens {
  Dimens._();

  // ── 间距 ──────────────────────────────────────────────────
  static double get gap_dp2 => Screen.sizeByDesign(2);
  static double get gap_dp4 => Screen.sizeByDesign(4);
  static double get gap_dp5 => Screen.sizeByDesign(5);
  static double get gap_dp6 => Screen.sizeByDesign(6);
  static double get gap_dp7 => Screen.sizeByDesign(7);
  static double get gap_dp8 => Screen.sizeByDesign(8);
  static double get gap_dp9 => Screen.sizeByDesign(9);
  static double get gap_dp10 => Screen.sizeByDesign(10);
  static double get gap_dp12 => Screen.sizeByDesign(12);
  static double get gap_dp13 => Screen.sizeByDesign(13);
  static double get gap_dp14 => Screen.sizeByDesign(14);
  static double get gap_dp15 => Screen.sizeByDesign(15);
  static double get gap_dp16 => Screen.sizeByDesign(16);
  static double get gap_dp18 => Screen.sizeByDesign(18);
  static double get gap_dp20 => Screen.sizeByDesign(20);
  static double get gap_dp22 => Screen.sizeByDesign(22);
  static double get gap_dp24 => Screen.sizeByDesign(24);
  static double get gap_dp28 => Screen.sizeByDesign(28);
  static double get gap_dp30 => Screen.sizeByDesign(30);
  static double get gap_dp32 => Screen.sizeByDesign(32);
  static double get gap_dp36 => Screen.sizeByDesign(36);
  static double get gap_dp40 => Screen.sizeByDesign(40);
  static double get gap_dp48 => Screen.sizeByDesign(48);
  static double get gap_dp56 => Screen.sizeByDesign(56);
  static double get gap_dp60 => Screen.sizeByDesign(60);
  static double get gap_dp72 => Screen.sizeByDesign(72);
  static double get gap_dp100 => Screen.sizeByDesign(100);

  // ── 字号 ──────────────────────────────────────────────────
  static double get font_sp10 => Screen.sizeByDesign(10);
  static double get font_sp11 => Screen.sizeByDesign(11);
  static double get font_sp12 => Screen.sizeByDesign(12);
  static double get font_sp13 => Screen.sizeByDesign(13);
  static double get font_sp14 => Screen.sizeByDesign(14);
  static double get font_sp15 => Screen.sizeByDesign(15);
  static double get font_sp16 => Screen.sizeByDesign(16);
  static double get font_sp17 => Screen.sizeByDesign(17);
  static double get font_sp18 => Screen.sizeByDesign(18);
  static double get font_sp20 => Screen.sizeByDesign(20);
  static double get font_sp22 => Screen.sizeByDesign(22);
  static double get font_sp24 => Screen.sizeByDesign(24);
  static double get font_sp28 => Screen.sizeByDesign(28);
}

/// 常用间距组件。
///
/// 注意：这些 widget 依赖运行时屏幕尺寸，**不是 `const`**，
/// 因此不能出现在 `const` 表达式里（`const Column(children: [Gaps.vGap8])` 会编译失败）。
class Gaps {
  Gaps._();

  /// 宽 4 的横向间距。
  static Widget get hGap4 => SizedBox(width: Dimens.gap_dp4);
  /// 宽 8 的横向间距。
  static Widget get hGap8 => SizedBox(width: Dimens.gap_dp8);
  /// 宽 12 的横向间距。
  static Widget get hGap12 => SizedBox(width: Dimens.gap_dp12);
  /// 宽 16 的横向间距。
  static Widget get hGap16 => SizedBox(width: Dimens.gap_dp16);
  /// 宽 24 的横向间距。
  static Widget get hGap24 => SizedBox(width: Dimens.gap_dp24);

  /// 高 4 的纵向间距。
  static Widget get vGap4 => SizedBox(height: Dimens.gap_dp4);
  /// 高 8 的纵向间距。
  static Widget get vGap8 => SizedBox(height: Dimens.gap_dp8);
  /// 高 12 的纵向间距。
  static Widget get vGap12 => SizedBox(height: Dimens.gap_dp12);
  /// 高 16 的纵向间距。
  static Widget get vGap16 => SizedBox(height: Dimens.gap_dp16);
  /// 高 20 的纵向间距。
  static Widget get vGap20 => SizedBox(height: Dimens.gap_dp20);
  /// 高 24 的纵向间距。
  static Widget get vGap24 => SizedBox(height: Dimens.gap_dp24);
  /// 高 32 的纵向间距。
  static Widget get vGap32 => SizedBox(height: Dimens.gap_dp32);

  /// 撑满剩余空间。
  static Widget get expand => const Expanded(child: SizedBox.shrink());
}

/// 数值 → 缩放尺寸的语法糖。
///
/// `16.dp` 返回的是**按设计稿等比缩放后的 double**（不是 `SizedBox`），
/// 用于 `SizedBox(height: 16.dp)`、`EdgeInsets.all(12.dp)`、`BorderRadius.circular(8.dp)` 等。
extension DoubleEx on double {
  /// 按设计稿缩放的尺寸。
  double get dp => Screen.sizeByDesign(this);

  /// 缩放后的圆角（`Radius`）。
  Radius get radius => Radius.circular(dp);

  /// 缩放后的圆角（`BorderRadius`），四角相同。
  BorderRadius get allBorderRadius => BorderRadius.all(radius);

  /// 缩放后的正方形占位。
  SizedBox get space => SizedBox(width: dp, height: dp);

  /// 缩放后的纵向占位。
  SizedBox get dpBox => SizedBox(height: dp);

  /// 缩放后的横向占位。
  SizedBox get dpBoxW => SizedBox(width: dp);
}

/// 整数的同款语法糖（`16.dp` 与 `12.dp` 都自然可用）。
extension IntEx on int {
  /// 按设计稿缩放的尺寸。
  double get dp => Screen.sizeByDesign(toDouble());

  /// 缩放后的圆角（`Radius`）。
  Radius get radius => Radius.circular(dp);

  /// 缩放后的圆角（`BorderRadius`）。
  BorderRadius get allBorderRadius => BorderRadius.all(radius);

  /// 缩放后的正方形占位。
  SizedBox get space => SizedBox(width: dp, height: dp);

  /// 缩放后的纵向占位。
  SizedBox get dpBox => SizedBox(height: dp);

  /// 缩放后的横向占位。
  SizedBox get dpBoxW => SizedBox(width: dp);
}

/// `EdgeInsets` 按设计稿值构造的辅助函数。
///
/// 主要解决「四个方向传的都是设计稿值」时的可读性：
/// ```dart
/// padding: insetsAll(16),                 // 等价于 EdgeInsets.all(16.dp)
/// padding: insetsSymmetric(h: 16, v: 8),  // 等价于 EdgeInsets.symmetric(...)
/// padding: insetsOnly(top: 8, bottom: 12),
/// ```
EdgeInsets insetsAll(double v) => EdgeInsets.all(v.dp);

EdgeInsets insetsSymmetric({double h = 0, double v = 0}) =>
    EdgeInsets.symmetric(horizontal: h.dp, vertical: v.dp);

EdgeInsets insetsOnly({
  double left = 0,
  double top = 0,
  double right = 0,
  double bottom = 0,
}) =>
    EdgeInsets.only(
      left: left.dp,
      top: top.dp,
      right: right.dp,
      bottom: bottom.dp,
    );
