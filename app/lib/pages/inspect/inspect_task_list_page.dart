import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Definition.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/entity_list_page.dart';
import '../../widgets/stat_card.dart';
import '../../layout/Gaps.dart';

/// 我的巡检任务列表页。
///
/// 后端 `GET /app/inspect-tasks` 已按当前登录用户过滤，
/// 因此本页展示的即为「指派给我的」任务。完成后任务状态变为 finished，
/// 默认筛选「待巡检」时该任务会从列表消失。
class InspectTaskListPage extends StatefulWidget {
  /// 构造巡检任务列表页。
  const InspectTaskListPage({super.key});

  @override
  State<InspectTaskListPage> createState() => _InspectTaskListPageState();
}

class _InspectTaskListPageState extends State<InspectTaskListPage> {
  /// 当前筛选状态，默认只看待巡检。
  String _status = 'published';

  /// 刷新令牌：详情页完成后自增。
  int _tick = 0;

  bool _initialized = false;

  /// 可选筛选项。
  static const List<({String value, String label})> _filters =
      <({String value, String label})>[
    (value: 'published', label: '待巡检'),
    (value: 'finished', label: '已完成'),
    (value: '', label: '全部'),
  ];

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    final String? fromQuery =
        GoRouterState.of(context).uri.queryParameters['status'];
    if (fromQuery != null && fromQuery.isNotEmpty) {
      _status = fromQuery;
    }
  }

  /// 分页取数。
  Future<PageDataLike<InspectTask>> _fetch(int page, int pageSize) async {
    final RPageInspectTask resp = await AppController.inspectTasksUsingGet(
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        if (_status.isNotEmpty) 'status': _status,
      },
    );
    return PageDataAdapter<InspectTask>(
      total: resp.total,
      items: resp.items,
      hasMore: resp.page.hasMore,
    );
  }

  /// 切换筛选条件。
  void _onFilterChanged(String value) {
    if (_status == value) return;
    setState(() {
      _status = value;
      _tick++;
    });
  }

  /// 进入详情页，返回后刷新（完成巡检后任务应从当前列表消失）。
  Future<void> _openDetail(InspectTask task) async {
    final bool? finished =
        await context.push<bool>('/inspect-tasks/detail?id=${task.id}');
    if (!mounted) return;
    if (finished == true) {
      setState(() => _tick++);
    }
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<InspectTask>(
      title: '巡检任务',
      reloadToken: '$_status#$_tick',
      fetcher: _fetch,
      emptyText: _status == 'published' ? '暂无待巡检任务' : '暂无巡检任务',
      emptyIcon: Icons.fact_check_outlined,
      header: _buildFilterBar(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (
        BuildContext context,
        InspectTask task,
        int index,
      ) =>
          _buildItem(task),
    );
  }

  /// 状态筛选条。
  Widget _buildFilterBar() {
    return Container(
      color: AppColors.card,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.sm,
      ),
      child: Row(
        children: <Widget>[
          for (final ({String value, String label}) f in _filters) ...<Widget>[
            ChoiceChip(
              key: Key('inspect-filter-${f.value.isEmpty ? 'all' : f.value}'),
              label: Text(f.label),
              selected: _status == f.value,
              onSelected: (_) => _onFilterChanged(f.value),
            ),
            SizedBox(width: AppSpacing.sm),
          ],
        ],
      ),
    );
  }

  /// 单条任务。
  Widget _buildItem(InspectTask task) {
    final bool finished = task.isFinished;

    return EntityCard(
      onTap: () => _openDetail(task),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  task.name.orPlaceholder('巡检任务'),
                  style: AppText.bodyStrong,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              SizedBox(width: AppSpacing.xs),
              StatusTag(
                text: task.statusText,
                color: finished ? AppColors.success : AppColors.primary,
              ),
            ],
          ),
          if (task.content.isNotEmpty) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Text(
              task.content,
              style: AppText.caption,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          SizedBox(height: AppSpacing.sm),
          Row(
            children: <Widget>[
              Icon(
                Icons.location_on_outlined,
                size: 13.dp,
                color: AppColors.textTertiary,
              ),
              SizedBox(width: 3.dp),
              Expanded(
                child: Text(
                  task.location.orPlaceholder(
                    task.regionName.orPlaceholder('未指定位置'),
                  ),
                  style: AppText.hint,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
          SizedBox(height: 2.dp),
          Row(
            children: <Widget>[
              Icon(
                Icons.schedule_outlined,
                size: 13.dp,
                color: AppColors.textTertiary,
              ),
              SizedBox(width: 3.dp),
              Expanded(
                child: Text(
                  '巡检时间 ${task.planDT.orPlaceholder('--')}',
                  style: AppText.hint,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              if (!finished)
                Text(
                  '去巡检 >',
                  style: AppText.caption.copyWith(color: AppColors.primary),
                ),
            ],
          ),
        ],
      ),
    );
  }
}
