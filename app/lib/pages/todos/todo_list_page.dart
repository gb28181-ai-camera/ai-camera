import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Definition.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../widgets/empty_state.dart';
import '../../widgets/loading_indicator.dart';
import '../../layout/Gaps.dart';

/// 待办页。
///
/// 四类待办分组展示：待处理风险 / 待处理告警 / 待巡检任务 / 未读通知。
/// 数据来源 `GET /app/todos`，点击分组跳转到对应列表页。
class TodoListPage extends StatefulWidget {
  /// 构造待办页。
  const TodoListPage({super.key});

  @override
  State<TodoListPage> createState() => _TodoListPageState();
}

class _TodoListPageState extends State<TodoListPage> {
  List<TodoItem> _items = <TodoItem>[];
  bool _loading = true;
  String? _error;

  /// 四类待办的兜底定义，保证后端未返回某一类时仍能看到入口。
  static const List<String> _allCategories = <String>[
    'risk',
    'alarm',
    'inspect',
    'notification',
  ];

  @override
  void initState() {
    super.initState();
    _load();
  }

  /// 加载待办汇总。
  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final RAppTodoList resp = await AppController.todosUsingGet();
      if (!mounted) return;
      setState(() {
        _items = resp.isSuccess ? resp.todos : <TodoItem>[];
        _error = resp.isSuccess ? null : resp.errorMessage;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '加载失败，请下拉重试';
      });
    }
  }

  /// 按类别取待办项，缺失时构造数量为 0 的占位。
  TodoItem _itemOf(String category) {
    for (final TodoItem it in _items) {
      if (it.category == category) return it;
    }
    return TodoItem(category: category, title: '', count: 0, refId: 0);
  }

  /// 总待办数。
  int get _totalCount =>
      _items.fold<int>(0, (int sum, TodoItem it) => sum + it.count);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('待办')),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _buildBody(),
      ),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const LoadingIndicator(text: '加载中...');
    }

    if (_error != null) {
      return ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: <Widget>[
          SizedBox(height: MediaQuery.of(context).size.height * 0.12),
          EmptyState.error(message: _error, onRetry: _load),
        ],
      );
    }

    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: EdgeInsets.only(bottom: AppSpacing.xxl),
      children: <Widget>[
        _buildSummary(),
        for (final String category in _allCategories)
          _buildGroup(_itemOf(category)),
        if (_totalCount == 0) ...<Widget>[
          SizedBox(height: AppSpacing.xxl),
          const EmptyState(
            icon: Icons.task_alt_outlined,
            title: '暂无待办',
            description: '所有风险与告警均已处理',
          ),
        ],
      ],
    );
  }

  // ── 汇总条 ────────────────────────────────────────────────

  Widget _buildSummary() {
    return Container(
      margin: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.primary,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Row(
        children: <Widget>[
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                '待办总数',
                style: AppText.caption.copyWith(
                  color: AppColors.textOnPrimary.withValues(alpha: 0.85),
                ),
              ),
              SizedBox(height: AppSpacing.xs),
              Text(
                '$_totalCount',
                style: AppText.statNumber.copyWith(
                  color: AppColors.textOnPrimary,
                  fontSize: 32.dp,
                ),
              ),
            ],
          ),
          const Spacer(),
          Icon(
            Icons.checklist_outlined,
            size: 44.dp,
            color: AppColors.textOnPrimary,
          ),
        ],
      ),
    );
  }

  // ── 分组 ──────────────────────────────────────────────────

  Widget _buildGroup(TodoItem item) {
    final Color color = _categoryColor(item.category);
    final IconData icon = _categoryIcon(item.category);

    return Padding(
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.xs,
      ),
      child: Material(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: InkWell(
          key: Key('todo-${item.category}'),
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
          onTap: () => context.push(item.route),
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.lg),
            child: Row(
              children: <Widget>[
                Container(
                  width: 40.dp,
                  height: 40.dp,
                  decoration: BoxDecoration(
                    color: color.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
                  ),
                  child: Icon(icon, size: 20.dp, color: color),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text(item.displayTitle, style: AppText.bodyStrong),
                      SizedBox(height: 2.dp),
                      Text(_categoryHint(item.category), style: AppText.hint),
                    ],
                  ),
                ),
                Text(
                  '${item.count}',
                  style: AppText.statNumber.copyWith(
                    fontSize: 20.dp,
                    color: item.count > 0 ? color : AppColors.textTertiary,
                  ),
                ),
                Icon(
                  Icons.chevron_right,
                  size: 20.dp,
                  color: AppColors.textTertiary,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _categoryHint(String category) {
    switch (category) {
      case 'risk':
        return 'AI 识别待人工确认与处理';
      case 'alarm':
        return '设备离线 / 电量 / 流量告警';
      case 'inspect':
        return '指派给我的巡检任务';
      case 'notification':
        return '尚未查看的系统通知';
      default:
        return '';
    }
  }

  static Color _categoryColor(String category) {
    switch (category) {
      case 'risk':
        return AppColors.danger;
      case 'alarm':
        return AppColors.warning;
      case 'inspect':
        return AppColors.primary;
      default:
        return AppColors.textSecondary;
    }
  }

  static IconData _categoryIcon(String category) {
    switch (category) {
      case 'risk':
        return Icons.warning_amber_outlined;
      case 'alarm':
        return Icons.notifications_active_outlined;
      case 'inspect':
        return Icons.fact_check_outlined;
      default:
        return Icons.notifications_none_outlined;
    }
  }
}
