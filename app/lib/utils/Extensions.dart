import 'package:flutter/material.dart';
import 'package:intl/intl.dart';

import '../config/api_config.dart';

/// 通用扩展方法集合。
///
/// 包含时间解析/格式化、字符串与空值处理、BuildContext 快捷方式。
/// 后端返回的时间字段（Go `time.Time`）为 RFC3339，而提交时为
/// `yyyy-MM-dd HH:mm:ss`，因此 [parseServerTime] 同时兼容两种格式。

/// 后端时间格式（提交时使用）。
final DateFormat _fmtFull = DateFormat('yyyy-MM-dd HH:mm:ss');
final DateFormat _fmtMinute = DateFormat('yyyy-MM-dd HH:mm');
final DateFormat _fmtDate = DateFormat('yyyy-MM-dd');
final DateFormat _fmtTimeOnly = DateFormat('HH:mm');

/// 把后端返回的相对图片路径补全为可直接访问的绝对地址。
///
/// 后端图片字段为 `/uploads/2026/09/xxx.jpg` 形式，Flutter 的
/// [Image.network] 需要完整 URL；已是 `http(s)://` 的原样返回。
String absoluteImageUrl(String? raw) {
  if (raw == null || raw.trim().isEmpty) return '';
  final String s = raw.trim();
  if (s.startsWith('http://') || s.startsWith('https://')) return s;
  final String base = ApiConfig.baseUrl;
  return s.startsWith('/') ? '$base$s' : '$base/$s';
}

/// 兼容解析服务端时间字符串。
///
/// 支持 `2026-09-21T10:30:00+08:00`、`2026-09-21T10:30:00Z`、
/// `2026-09-21 10:30:00`、`2026-09-21` 四种形态；无法解析时返回 null。
DateTime? parseServerTime(String? raw) {
  if (raw == null) return null;
  final String s = raw.trim();
  if (s.isEmpty) return null;

  // 优先按 RFC3339 解析（后端 time.Time 的标准输出）。
  final DateTime? rfc = DateTime.tryParse(s);
  if (rfc != null) return rfc.toLocal();

  // 回退到 "yyyy-MM-dd HH:mm:ss" 与 "yyyy-MM-dd"。
  for (final DateFormat f in <DateFormat>[_fmtFull, _fmtMinute, _fmtDate]) {
    try {
      return f.parseStrict(s);
    } catch (_) {
      // 继续尝试下一种格式
    }
  }
  return null;
}

/// 字符串与时间的展示扩展。
extension StringDisplayExtension on String? {
  /// 空字符串 / null 时返回 [placeholder]。
  String orPlaceholder([String placeholder = '--']) {
    final String? s = this;
    if (s == null || s.trim().isEmpty) return placeholder;
    return s;
  }

  /// 解析为服务端时间。
  DateTime? get toServerTime => parseServerTime(this);

  /// 格式化为 `yyyy-MM-dd HH:mm`，无法解析时返回 `--`。
  String get formatMinute {
    final DateTime? dt = parseServerTime(this);
    return dt == null ? '--' : _fmtMinute.format(dt);
  }

  /// 格式化为 `yyyy-MM-dd`，无法解析时返回 `--`。
  String get formatDate {
    final DateTime? dt = parseServerTime(this);
    return dt == null ? '--' : _fmtDate.format(dt);
  }

  /// 相对时间描述：刚刚 / n 分钟前 / n 小时前 / n 天前 / 具体日期。
  String get fromNow {
    final DateTime? dt = parseServerTime(this);
    if (dt == null) return '--';
    final Duration diff = DateTime.now().difference(dt);
    if (diff.inSeconds < 60) return '刚刚';
    if (diff.inMinutes < 60) return '${diff.inMinutes} 分钟前';
    if (diff.inHours < 24) return '${diff.inHours} 小时前';
    if (diff.inDays < 7) return '${diff.inDays} 天前';
    return _fmtDate.format(dt);
  }

  /// 只取时分，用于时间轴。
  String get formatTimeOnly {
    final DateTime? dt = parseServerTime(this);
    return dt == null ? '--' : _fmtTimeOnly.format(dt);
  }
}

/// DateTime 展示扩展。
extension DateTimeDisplayExtension on DateTime? {
  /// 格式化为 `yyyy-MM-dd HH:mm:ss`。
  String get toFullString =>
      this == null ? '--' : _fmtFull.format(this!);

  /// 格式化为 `yyyy-MM-dd HH:mm`。
  String get toMinuteString =>
      this == null ? '--' : _fmtMinute.format(this!);

  /// 格式化为 `yyyy-MM-dd`。
  String get toDateString =>
      this == null ? '--' : _fmtDate.format(this!);

  /// 是否与今天同一天。
  bool get isToday {
    final DateTime? d = this;
    if (d == null) return false;
    final DateTime now = DateTime.now();
    return d.year == now.year && d.month == now.month && d.day == now.day;
  }
}

/// 数值扩展。
extension NumDisplayExtension on num? {
  /// 转百分比字符串（0~100 之间，越界自动裁剪）。
  String get toPercent {
    final num v = (this ?? 0).clamp(0, 100);
    return '${v.toStringAsFixed(0)}%';
  }

  /// 除零安全的比例，返回 0~1。
  double ratioTo(num total) {
    final num t = total;
    if (t <= 0) return 0;
    return ((this ?? 0) / t).clamp(0.0, 1.0).toDouble();
  }
}

/// bool 语义扩展（后端大量字段为 `*bool`，可能为 null）。
extension BoolExtension on bool? {
  /// null 视为 false。
  bool get orFalse => this ?? false;

  /// 已读 / 未读文案。
  String get readText => orFalse ? '已读' : '未读';
}

/// BuildContext 快捷方式。
extension ContextExtension on BuildContext {
  /// 屏幕宽度。
  double get screenWidth => MediaQuery.of(this).size.width;

  /// 屏幕高度。
  double get screenHeight => MediaQuery.of(this).size.height;

  /// 顶部安全区高度。
  double get topPadding => MediaQuery.of(this).padding.top;

  /// 显示一个浮层提示。
  void showToast(String message) {
    final ScaffoldMessengerState messenger = ScaffoldMessenger.of(this);
    messenger.hideCurrentSnackBar();
    messenger.showSnackBar(
      SnackBar(
        content: Text(message),
        duration: const Duration(seconds: 2),
        margin: const EdgeInsets.all(16),
      ),
    );
  }
}

/// 列表扩展。
extension ListExtension<T> on List<T>? {
  /// null 安全取值。
  List<T> get orEmpty => this ?? <T>[];

  /// 是否为空（null 也算空）。
  bool get isBlank => this == null || this!.isEmpty;
}
