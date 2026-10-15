import 'package:flutter/material.dart';

import '../theme/app_colors.dart';
import '../theme/app_spacing.dart';
import '../theme/app_text.dart';
import '../layout/Gaps.dart';

/// 空态 / 错误态占位。
///
/// 列表为空、加载失败、权限不足等场景统一使用本组件，保证文案与留白一致。
class EmptyState extends StatelessWidget {
  /// 构造空态。
  const EmptyState({
    super.key,
    this.icon = Icons.inbox_outlined,
    this.title = '暂无数据',
    this.description,
    this.actionLabel,
    this.onAction,
  });

  /// 加载失败态（带重试按钮）。
  factory EmptyState.error({
    Key? key,
    String? message,
    VoidCallback? onRetry,
  }) {
    return EmptyState(
      key: key,
      icon: Icons.cloud_off_outlined,
      title: '加载失败',
      description: message,
      actionLabel: onRetry == null ? null : '重新加载',
      onAction: onRetry,
    );
  }

  /// 图标。
  final IconData icon;

  /// 主文案。
  final String title;

  /// 补充说明。
  final String? description;

  /// 操作按钮文案。
  final String? actionLabel;

  /// 操作回调。
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.xxxl),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(icon, size: 56.dp, color: AppColors.textDisabled),
            SizedBox(height: AppSpacing.lg),
            Text(
              title,
              style: AppText.body,
              textAlign: TextAlign.center,
            ),
            if (description != null && description!.isNotEmpty) ...<Widget>[
              SizedBox(height: AppSpacing.sm),
              Text(
                description!,
                style: AppText.empty,
                textAlign: TextAlign.center,
              ),
            ],
            if (actionLabel != null && onAction != null) ...<Widget>[
              SizedBox(height: AppSpacing.xl),
              OutlinedButton(
                onPressed: onAction,
                style: OutlinedButton.styleFrom(
                  minimumSize: const Size(120, 40),
                ),
                child: Text(actionLabel!),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
