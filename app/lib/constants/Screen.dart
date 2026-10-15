import 'dart:math' as math;

import 'package:flutter/widgets.dart';

/// 布局模式。屏幕短边超过 [Screen.maxDesignWidth] 时视为 PC/平板，不再放大。
enum ScreenMode { mobile, pc }

/// 设计稿等比缩放基准。
///
/// **为什么需要它**：UI 稿按固定宽度（[designWidth]，本项目为 414）出图，
/// 但真机逻辑宽度差异很大（360 ~ 480+）。若各处直接写字面量像素，
/// 小屏会显得拥挤、大屏会显得松散 —— 即所谓「设计偏差」。
/// 本类把「设计稿上的尺寸」换算成「当前设备上的尺寸」，全项目统一走
/// [sizeByDesign]，从而在任意屏宽下保持与设计稿一致的比例关系。
///
/// 用法（推荐经由设计令牌，不要直接调用本类）：
/// ```dart
/// AppSpacing.lg          // 设计稿 16 → 当前设备上的等比尺寸
/// AppText.title          // 字号同样等比缩放
/// 12.dp                  // 临时尺寸的语法糖（返回缩放后的 double）
/// ```
///
/// 与 boticz-mes `frontend_app/lib/constants/Screen.dart` 的差异：
/// 参考实现从全局 navigator key 取 `BuildContext`，且在 context 为空时回退到设计稿尺寸；
/// 本实现直接读 `platformDispatcher` 的视图尺寸，**不依赖 navigator**，
/// 并在屏幕尺寸变化（旋转 / 桌面端缩放窗口）时自动失效缓存。
class Screen {
  Screen._();

  /// 设计稿宽度（逻辑像素）。UI 稿按此宽度出图。
  static double designWidth = 414;

  /// 超过此短边宽度即认为是 PC/平板布局。
  static double maxDesignWidth = 960;

  static const double _fallbackWidth = 414;
  static const double _fallbackHeight = 896;

  static Size? _cachedSize;
  static bool _observerBound = false;

  // ── 屏幕尺寸 ────────────────────────────────────────────────

  /// 当前视图的逻辑尺寸（已按 devicePixelRatio 换算）。
  ///
  /// 仅缓存**成功读到**的尺寸：绑定尚未初始化时的回退值不会被固化，
  /// 否则后续旋转屏幕或缩放窗口都拿不到真实尺寸（回退值会一直生效）。
  static Size get _logicalSize {
    final cached = _cachedSize;
    if (cached != null) return cached;

    final size = _readSize();
    if (size != null) {
      _cachedSize = size;
      return size;
    }
    return const Size(_fallbackWidth, _fallbackHeight);
  }

  /// 读取首个视图的逻辑尺寸；读不到（绑定未初始化 / 视图为空 / 尺寸非法）返回 null，
  /// 由调用方回退到设计稿尺寸，保证 [sizeByDesign] 永远有可用基准（不抛异常、不返回 NaN）。
  static Size? _readSize() {
    try {
      final views = WidgetsBinding.instance.platformDispatcher.views;
      if (views.isEmpty) return null;

      final view = views.first;
      final dpr = view.devicePixelRatio;
      if (dpr <= 0) return null;

      final size = view.physicalSize / dpr;
      if (size.width <= 0 || size.height <= 0) return null;

      _bindMetricsObserver();
      return size;
    } catch (_) {
      // 绑定尚未初始化（例如在 runApp 之前访问设计令牌）
      return null;
    }
  }

  /// 监听屏幕尺寸变化，变化时清空缓存（比参考实现的「一次性计算」更准确）。
  static void _bindMetricsObserver() {
    if (_observerBound) return;
    _observerBound = true;
    try {
      WidgetsBinding.instance.addObserver(_ScreenMetricsObserver());
    } catch (_) {
      _observerBound = false;
    }
  }

  /// 主动清空尺寸缓存。屏幕尺寸变化时由观察者自动调用。
  static void invalidate() => _cachedSize = null;

  /// 屏幕逻辑宽度。
  static double get width => _logicalSize.width;

  /// 屏幕逻辑高度。
  static double get height => _logicalSize.height;

  /// 屏幕短边（横竖屏取较小者），等比缩放的基准边。
  static double get shortSide => width > height ? height : width;

  /// 设备像素比。
  static double get devicePixelRatio {
    try {
      final views = WidgetsBinding.instance.platformDispatcher.views;
      if (views.isEmpty) return 1.0;
      final dpr = views.first.devicePixelRatio;
      return dpr > 0 ? dpr : 1.0;
    } catch (_) {
      return 1.0;
    }
  }

  /// 布局模式：短边超过 [maxDesignWidth] 视为 pc。
  static ScreenMode get mode =>
      shortSide > maxDesignWidth ? ScreenMode.pc : ScreenMode.mobile;

  // ── 安全区 ──────────────────────────────────────────────────

  /// 顶部安全区高度（状态栏 / 刘海）。`ViewPadding.top` 本身已是逻辑像素。
  static double get topSafeHeight {
    try {
      return WidgetsBinding.instance.platformDispatcher.views.first.padding.top;
    } catch (_) {
      return 0;
    }
  }

  /// 底部安全区高度（手势条）。
  static double get bottomSafeHeight {
    try {
      return WidgetsBinding.instance.platformDispatcher.views.first.padding.bottom;
    } catch (_) {
      return 0;
    }
  }

  // ── 核心换算 ────────────────────────────────────────────────

  /// 把设计稿尺寸换算为当前设备的尺寸。
  ///
  /// 例（设计稿基准 414）：`sizeByDesign(16)`
  /// - 414 宽设备 → 16（与设计稿一致）
  /// - 360 宽设备 → `16 / (414/360)` ≈ 13.9（等比缩小，版式比例不变）
  /// - 800 宽设备 → 16（**不放大**，见下）
  ///
  /// **只缩不放**：缩放系数被钳制为不小于 1，即屏幕比设计稿宽时直接采用设计稿原值。
  /// 若不加钳制，800x600 的桌面/平板窗口会把所有尺寸放大到 1.93 倍、
  /// 960 短边放大到 2.32 倍，界面会「虚胖」——这是参考实现的缺陷，本项目不沿用。
  /// 大屏应通过 [mode] 切换布局（更多列/更宽内容区），而不是等比放大。
  ///
  /// [designScreenWidth] 可用于「以某个特定屏宽为基准」的场景（默认取 [designWidth]）。
  static double sizeByDesign(double designSize, [double? designScreenWidth]) {
    final double short = shortSide;
    if (short <= 0) return designSize;

    final double base = designScreenWidth ?? designWidth;
    if (base <= 0) return designSize;

    // radio = 设计稿宽 / 实际短边；>1 表示小屏需缩小，<1 表示大屏——钳制为 1 不放大
    final double radio = math.max(base / short, 1.0);
    return designSize / radio;
  }

  /// 屏幕物理 PPI，用于需要按物理尺寸呈现的场景（一般无需使用）。
  static double get ppi {
    try {
      final dpr = devicePixelRatio;
      final physicalWidthPx = width * dpr;
      final physicalHeightPx = height * dpr;
      final diagonalPhysicalPx =
          math.sqrt(math.pow(physicalWidthPx, 2) + math.pow(physicalHeightPx, 2));
      final diagonalInch = diagonalPhysicalPx / (160 * dpr);
      if (diagonalInch <= 0) return 0;
      return double.parse((diagonalPhysicalPx / diagonalInch).toStringAsFixed(1));
    } catch (_) {
      return 0;
    }
  }
}

/// 屏幕尺寸变化的观察者，仅用于失效 [Screen] 的尺寸缓存。
class _ScreenMetricsObserver extends WidgetsBindingObserver {
  @override
  void didChangeMetrics() => Screen.invalidate();
}
