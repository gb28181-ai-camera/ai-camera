import 'package:flutter/material.dart';

import '../theme/app_colors.dart';
import '../theme/app_spacing.dart';
import '../theme/app_text.dart';
import '../layout/Gaps.dart';

/// 首页数字统计卡（待处理风险 / 今日风险 等）。
class StatCard extends StatelessWidget {
  /// 构造统计卡。
  const StatCard({
    super.key,
    required this.title,
    required this.value,
    this.icon,
    this.color = AppColors.primary,
    this.onTap,
    this.compact = false,
  });

  /// 卡片标题。
  final String title;

  /// 数值（字符串以支持 `12/20` 之类）。
  final String value;

  /// 右上角图标。
  final IconData? icon;

  /// 强调色（数字与图标颜色）。
  final Color color;

  /// 点击回调，为 null 时不可点击。
  final VoidCallback? onTap;

  /// 紧凑模式（用于一行放多个）。
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final Widget content = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        Row(
          children: <Widget>[
            Expanded(
              child: Text(
                title,
                style: AppText.hint,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            if (icon != null) Icon(icon, size: 16.dp, color: color),
          ],
        ),
        SizedBox(height: compact ? AppSpacing.sm : AppSpacing.md),
        Text(
          value,
          style: AppText.statNumber.copyWith(
            color: color,
            fontSize: compact ? 22 : 26,
          ),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
      ],
    );

    return Material(
      color: AppColors.card,
      borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: Padding(
          padding: EdgeInsets.all(compact ? AppSpacing.md : AppSpacing.lg),
          child: content,
        ),
      ),
    );
  }
}

/// 带进度条的统计卡（设备在线情况）。
///
/// 进度条用纯 Widget 绘制（[LinearProgressIndicator]），
/// 本版本不引入图表库。
class ProgressStatCard extends StatelessWidget {
  /// 构造带进度的统计卡。
  const ProgressStatCard({
    super.key,
    required this.title,
    required this.total,
    required this.done,
    this.doneLabel = '在线',
    this.undoneLabel = '离线',
    this.color = AppColors.success,
    this.icon,
    this.onTap,
  });

  /// 卡片标题。
  final String title;

  /// 总数。
  final int total;

  /// 已完成数（如在线设备数）。
  final int done;

  /// 已完成项文案。
  final String doneLabel;

  /// 未完成项文案。
  final String undoneLabel;

  /// 进度条颜色。
  final Color color;

  /// 标题图标。
  final IconData? icon;

  /// 点击回调。
  final VoidCallback? onTap;

  /// 未完成数。
  int get undone => (total - done) < 0 ? 0 : total - done;

  /// 完成比例（0~1）。
  double get ratio => total <= 0 ? 0 : (done / total).clamp(0.0, 1.0);

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.card,
      borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Expanded(child: Text(title, style: AppText.section)),
                  if (icon != null)
                    Icon(icon, size: 18.dp, color: AppColors.textTertiary),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              Row(
                crossAxisAlignment: CrossAxisAlignment.baseline,
                textBaseline: TextBaseline.alphabetic,
                children: <Widget>[
                  Text(
                    '$done',
                    style: AppText.statNumber.copyWith(color: color),
                  ),
                  Text(
                    ' / $total',
                    style: AppText.caption,
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              ClipRRect(
                borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
                child: LinearProgressIndicator(
                  value: ratio,
                  minHeight: 8,
                  backgroundColor: AppColors.fill,
                  valueColor: AlwaysStoppedAnimation<Color>(color),
                ),
              ),
              SizedBox(height: AppSpacing.sm),
              Row(
                children: <Widget>[
                  Expanded(
                    child: Text(
                      '$doneLabel $done',
                      style: AppText.hint,
                    ),
                  ),
                  Text(
                    '$undoneLabel $undone',
                    style: AppText.hint,
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 状态标签（待处理 / 已处理 / online 等）。
class StatusTag extends StatelessWidget {
  /// 构造状态标签。
  const StatusTag({
    super.key,
    required this.text,
    required this.color,
    this.filled = false,
  });

  /// 文案。
  final String text;

  /// 主色。
  final Color color;

  /// 是否使用实心背景。
  final bool filled;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: 3.dp),
      decoration: BoxDecoration(
        color: filled ? color : color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
      ),
      child: Text(
        text,
        style: AppText.tag.copyWith(
          color: filled ? AppColors.textOnPrimary : color,
        ),
      ),
    );
  }
}
