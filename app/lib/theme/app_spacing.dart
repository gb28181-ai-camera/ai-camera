import 'package:flutter/widgets.dart';

import '../constants/Screen.dart';

/// 全局设计令牌 —— 间距与圆角。
///
/// 所有页面必须使用此处的令牌，避免出现魔法数字。
///
/// **尺寸语义**：下面每个数值都是**设计稿（414 宽）上的尺寸**，
/// 取用时经 [Screen.sizeByDesign] 换算为当前设备的等比尺寸，
/// 因此在 360 宽的小屏与 480 宽的大屏上版式比例保持一致。
///
/// **为什么是 getter 而不是 `const`**：缩放值依赖运行时屏幕尺寸，无法在编译期确定。
/// 代价是不能出现在 `const` 表达式里（`const SizedBox(height: AppSpacing.md)` 会编译失败，
/// 需去掉 `const`）；收益是屏幕尺寸变化后能立即反映新值。
class AppSpacing {
  AppSpacing._();

  // ── 间距（设计稿值，4 的倍数）──────────────────────────────
  /// 设计稿 4。
  static double get xs => Screen.sizeByDesign(4);
  /// 设计稿 8。
  static double get sm => Screen.sizeByDesign(8);
  /// 设计稿 12。
  static double get md => Screen.sizeByDesign(12);
  /// 设计稿 16。
  static double get lg => Screen.sizeByDesign(16);
  /// 设计稿 20。
  static double get xl => Screen.sizeByDesign(20);
  /// 设计稿 24。
  static double get xxl => Screen.sizeByDesign(24);
  /// 设计稿 32。
  static double get xxxl => Screen.sizeByDesign(32);

  /// 页面左右统一留白（设计稿 16）。
  static double get pagePadding => Screen.sizeByDesign(16);

  // ── 圆角 ──────────────────────────────────────────────────
  /// 卡片圆角（设计稿 12）。
  static double get radiusCard => Screen.sizeByDesign(12);
  /// 按钮 / 输入框圆角（设计稿 8）。
  static double get radiusControl => Screen.sizeByDesign(8);
  /// 标签 / 徽标圆角（设计稿 4）。
  static double get radiusTag => Screen.sizeByDesign(4);
  /// 头像等正圆。该值不参与缩放，避免高倍缩放下失真。
  static double get radiusRound => 999.0;

  // ── 组件尺寸 ──────────────────────────────────────────────
  /// 主按钮高度（设计稿 48）。
  static double get buttonHeight => Screen.sizeByDesign(48);
  /// 输入框高度（设计稿 48）。
  static double get inputHeight => Screen.sizeByDesign(48);
  /// 列表缩略图边长（设计稿 72）。
  static double get thumbnailSize => Screen.sizeByDesign(72);
  /// 详情页大图高度（设计稿 240）。
  static double get detailImageHeight => Screen.sizeByDesign(240);
  /// 底部导航栏图标尺寸（设计稿 22）。
  static double get navIconSize => Screen.sizeByDesign(22);

  // ── 分割线 / 描边 ─────────────────────────────────────────
  /// 分割线粗细。**不缩放**：1 逻辑像素是物理约束，缩放会导致细线消失或变粗。
  static double get dividerThickness => 1.0;
  /// 卡片描边粗细。同样不缩放。
  static double get borderThickness => 1.0;
}

/// 圆角的缩放语法糖：`AppRadius.card` 等价于 `BorderRadius.circular(AppSpacing.radiusCard)`。
class AppRadius {
  AppRadius._();

  /// 卡片圆角。
  static BorderRadius get card => BorderRadius.circular(AppSpacing.radiusCard);
  /// 按钮 / 输入框圆角。
  static BorderRadius get control =>
      BorderRadius.circular(AppSpacing.radiusControl);
  /// 标签 / 徽标圆角。
  static BorderRadius get tag => BorderRadius.circular(AppSpacing.radiusTag);
  /// 正圆（头像、圆形按钮）。
  static BorderRadius get round =>
      BorderRadius.circular(AppSpacing.radiusRound);
}
