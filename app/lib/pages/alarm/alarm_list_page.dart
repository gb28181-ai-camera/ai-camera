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

/// 设备告警列表页。
///
/// 与风险列表结构一致，但**没有「检测准确」维度**（告警来自设备自身状态）。
/// 待处理告警点击后直接进入处理页。
class AlarmListPage extends StatefulWidget {
  /// 构造告警列表页。
  const AlarmListPage({super.key});

  @override
  State<AlarmListPage> createState() => _AlarmListPageState();
}

class _AlarmListPageState extends State<AlarmListPage> {
  /// 当前筛选状态，空串表示全部。
  String _status = '';

  /// 刷新令牌：处理页返回后自增，驱动列表重新加载。
  int _tick = 0;

  bool _initialized = false;

  /// 可选筛选项。
  static const List<({String value, String label})> _filters =
      <({String value, String label})>[
    (value: '', label: '全部'),
    (value: 'pending', label: '待处理'),
    (value: 'handled', label: '已处理'),
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
  Future<PageDataLike<AlarmRecord>> _fetch(int page, int pageSize) async {
    final RPageAlarmRecord resp = await AppController.alarmsUsingGet(
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        if (_status.isNotEmpty) 'status': _status,
      },
    );
    return PageDataAdapter<AlarmRecord>(
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

  /// 进入处理页，返回后刷新。
  Future<void> _openHandle(AlarmRecord alarm) async {
    final bool? handled = await context.push<bool>('/alarms/handle?id=${alarm.id}');
    if (!mounted) return;
    if (handled == true) {
      setState(() => _tick++);
    }
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<AlarmRecord>(
      title: '设备告警',
      reloadToken: '$_status#$_tick',
      fetcher: _fetch,
      emptyText: _status.isEmpty ? '暂无告警记录' : '暂无该状态的告警',
      emptyIcon: Icons.notifications_off_outlined,
      header: _buildFilterBar(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (
        BuildContext context,
        AlarmRecord alarm,
        int index,
      ) =>
          _buildItem(alarm),
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
              key: Key('alarm-filter-${f.value.isEmpty ? 'all' : f.value}'),
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

  /// 单条告警。
  Widget _buildItem(AlarmRecord alarm) {
    return EntityCard(
      onTap: alarm.isPending ? () => _openHandle(alarm) : null,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  alarm.alarmTypeName.orPlaceholder('设备告警'),
                  style: AppText.bodyStrong,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              SizedBox(width: AppSpacing.xs),
              StatusTag(
                text: _levelText(alarm.alarmLevel),
                color: AppColors.levelColor(alarm.alarmLevel),
              ),
              SizedBox(width: AppSpacing.xs),
              StatusTag(
                text: alarm.isHandled ? '已处理' : '待处理',
                color: AppColors.statusColor(alarm.status),
              ),
            ],
          ),
          if (alarm.content.isNotEmpty) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Text(
              alarm.content,
              style: AppText.caption,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          SizedBox(height: AppSpacing.sm),
          _iconLine(
            Icons.videocam_outlined,
            alarm.deviceName.orPlaceholder(alarm.deviceCode),
          ),
          SizedBox(height: 2.dp),
          _iconLine(
            Icons.location_on_outlined,
            alarm.location.orPlaceholder(
              alarm.regionName.orPlaceholder('未分配区域'),
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: <Widget>[
              Expanded(
                child: Text(alarm.occurredDT.fromNow, style: AppText.hint),
              ),
              if (alarm.isPending)
                Text(
                  '去处理 >',
                  style: AppText.caption.copyWith(color: AppColors.primary),
                ),
            ],
          ),
          // 已处理时展示处理结果摘要。
          if (alarm.isHandled && alarm.handleResult.isNotEmpty) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Container(
              width: double.infinity,
              padding: EdgeInsets.all(AppSpacing.sm),
              decoration: BoxDecoration(
                color: AppColors.fill,
                borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
              ),
              child: Text(
                '处理：${alarm.handleResult}',
                style: AppText.hint,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ],
      ),
    );
  }

  /// 图标 + 文本的说明行。
  Widget _iconLine(IconData icon, String text) {
    return Row(
      children: <Widget>[
        Icon(icon, size: 13.dp, color: AppColors.textTertiary),
        SizedBox(width: 3.dp),
        Expanded(
          child: Text(
            text,
            style: AppText.hint,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }

  static String _levelText(String level) {
    switch (level) {
      case 'high':
        return '高';
      case 'medium':
        return '中';
      case 'low':
        return '低';
      default:
        return '告警';
    }
  }
}
