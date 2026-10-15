import 'package:flutter/material.dart';

import '../theme/app_colors.dart';
import '../theme/app_spacing.dart';
import '../theme/app_text.dart';
import '../layout/Gaps.dart';

/// 居中加载指示器（整页使用）。
class LoadingIndicator extends StatelessWidget {
  /// 构造加载指示器。
  const LoadingIndicator({super.key, this.text, this.size = 28.0});

  /// 加载文案，为空时只显示转圈。
  final String? text;

  /// 转圈尺寸。
  final double size;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          SizedBox(
            width: size,
            height: size,
            child: const CircularProgressIndicator(strokeWidth: 2.4),
          ),
          if (text != null && text!.isNotEmpty) ...<Widget>[
            SizedBox(height: AppSpacing.md),
            Text(text!, style: AppText.hint),
          ],
        ],
      ),
    );
  }
}

/// 按钮内嵌的小转圈（登录按钮 loading 态使用）。
class InlineSpinner extends StatelessWidget {
  /// 构造内嵌转圈。
  const InlineSpinner({super.key, this.size = 16.0, this.color});

  /// 尺寸。
  final double size;

  /// 颜色，默认取主色上的文字色。
  final Color? color;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: size,
      height: size,
      child: CircularProgressIndicator(
        strokeWidth: 2,
        valueColor: AlwaysStoppedAnimation<Color>(
          color ?? AppColors.textOnPrimary,
        ),
      ),
    );
  }
}

/// 列表底部状态条：加载中 / 没有更多 / 加载失败重试。
class ListFooter extends StatelessWidget {
  /// 构造列表底部状态条。
  const ListFooter({
    super.key,
    required this.loading,
    required this.hasMore,
    this.error,
    this.onRetry,
    this.emptyCount = 0,
  });

  /// 是否正在加载下一页。
  final bool loading;

  /// 是否还有下一页。
  final bool hasMore;

  /// 加载下一页失败的错误信息。
  final String? error;

  /// 重试回调。
  final VoidCallback? onRetry;

  /// 当前已加载条数（为 0 时不展示「没有更多」）。
  final int emptyCount;

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        child: Center(
          child: SizedBox(
            width: 20.dp,
            height: 20.dp,
            child: const CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
      );
    }

    if (error != null && error!.isNotEmpty) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        child: Center(
          child: TextButton(
            onPressed: onRetry,
            child: Text('$error，点击重试'),
          ),
        ),
      );
    }

    if (!hasMore && emptyCount > 0) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        child: Center(child: Text('没有更多了', style: AppText.hint)),
      );
    }

    return SizedBox(height: AppSpacing.lg);
  }
}
