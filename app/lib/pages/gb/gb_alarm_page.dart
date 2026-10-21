import 'package:flutter/material.dart';

import '../../layout/Gaps.dart';
import '../../services/generated/GbController.dart';
import '../../services/generated/definitions/AppGb.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/entity_list_page.dart';
import '../../widgets/stat_card.dart';

/// GB/T 28181 报警类型编码 → 中文文案。
///
/// 编码由设备在 MANSCDP `Alarm` 报文中原样上报，取值见
/// `docs/开发文档/页面功能和接口/13-国标视频页面与接口.md` §3.10
/// （与 Web 端 `GbAlarmList.tsx` 的 `ALARM_TYPE_LABELS` 保持一致）。
const Map<String, String> _alarmTypeLabels = <String, String>{
  '1': '视频报警',
  '2': '设备报警',
  '3': '交通报警',
  '4': '报警传输',
  '5': '其他报警',
};

/// 国标报警级别 → 展示文案 / 颜色。
///
/// ★ GB28181 的级别是 1–4，**1 最严重**（与「等级越高越严重」的直觉相反），
/// 因此这里不能用通用的 `high/medium/low` 字符串直接比较。
const Map<String, String> _priorityLabels = <String, String>{
  '1': '一级（特急）',
  '2': '二级（紧急）',
  '3': '三级（一般）',
  '4': '四级（其他）',
};

/// AI 监测事件（国标报警）只读列表页。
///
/// 数据来源 `GET /app/gb/alarms`：设备通过 MANSCDP `Alarm` 主动上报的报警，
/// 与「AI 风险识别」不是同一条链路。本期 App 端**只读**：
/// 查看列表与详情，不提供处理 / 抓图 / 删除（Web 端已有这些操作）。
///
/// 列表复用 [EntityListPage]，自带下拉刷新、上滑分页、空态与失败重试。
class GbAlarmPage extends StatefulWidget {
  /// 构造 AI 监测事件页。
  const GbAlarmPage({super.key});

  @override
  State<GbAlarmPage> createState() => _GbAlarmPageState();
}

class _GbAlarmPageState extends State<GbAlarmPage> {
  /// 处理状态筛选：null = 全部，false = 未处理，true = 已处理。
  bool? _handled;

  /// 刷新令牌：筛选变化时自增，驱动 [EntityListPage] 回到第一页重新加载。
  int _tick = 0;

  /// 可选筛选项。
  static const List<({bool? value, String label})> _filters =
      <({bool? value, String label})>[
    (value: null, label: '全部'),
    (value: false, label: '未处理'),
    (value: true, label: '已处理'),
  ];

  /// 分页取数。
  Future<PageDataLike<AppGbAlarm>> _fetch(int page, int pageSize) async {
    final RAppGbAlarmPage resp = await GbController.alarmsUsingGet(
      page: page,
      pageSize: pageSize,
      handled: _handled,
    );
    return PageDataAdapter<AppGbAlarm>(
      total: resp.total,
      items: resp.items,
      hasMore: resp.page.hasMore,
    );
  }

  /// 切换处理状态筛选。
  void _onFilterChanged(bool? value) {
    if (_handled == value) return;
    setState(() {
      _handled = value;
      _tick++;
    });
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<AppGbAlarm>(
      title: 'AI 监测事件',
      reloadToken: '$_handled#$_tick',
      fetcher: _fetch,
      emptyText: _handled == null ? '暂无监测事件' : '暂无该状态的事件',
      emptyIcon: Icons.notifications_off_outlined,
      header: _buildFilterBar(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (
        BuildContext context,
        AppGbAlarm alarm,
        int index,
      ) =>
          _buildItem(alarm),
    );
  }

  // ── 顶部筛选 ────────────────────────────────────────────────

  Widget _buildFilterBar() {
    return Container(
      color: AppColors.card,
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(
            '国标设备主动上报的报警事件；App 端只读，处理请前往 Web 端',
            style: AppText.hint,
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: <Widget>[
              for (final ({bool? value, String label}) f in _filters)
                ...<Widget>[
                  ChoiceChip(
                    key: Key(
                      'gb-alarm-filter-${_filterKey(f.value)}',
                    ),
                    label: Text(f.label),
                    selected: _handled == f.value,
                    onSelected: (_) => _onFilterChanged(f.value),
                  ),
                  SizedBox(width: AppSpacing.sm),
                ],
            ],
          ),
        ],
      ),
    );
  }

  /// 筛选值 → 稳定的 Key 片段。
  static String _filterKey(bool? value) {
    if (value == null) return 'all';
    return value ? 'handled' : 'pending';
  }

  // ── 列表项 ──────────────────────────────────────────────────

  Widget _buildItem(AppGbAlarm alarm) {
    final bool handled = alarm.handled ?? false;

    return EntityCard(
      key: Key('gb-alarm-${alarm.id}'),
      onTap: () => _showDetail(alarm),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  _alarmTypeText(alarm.alarmType),
                  style: AppText.bodyStrong,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              SizedBox(width: AppSpacing.xs),
              StatusTag(
                text: _priorityShortText(alarm.alarmPriority),
                color: _priorityColor(alarm.alarmPriority),
              ),
              SizedBox(width: AppSpacing.xs),
              StatusTag(
                text: handled ? '已处理' : '未处理',
                color: handled ? AppColors.success : AppColors.warning,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          _iconLine(
            Icons.videocam_outlined,
            <String>[
              alarm.deviceName,
              alarm.channelName,
            ].where((String s) => s.isNotEmpty).join(' · '),
          ),
          SizedBox(height: 2.dp),
          _iconLine(
            Icons.access_time,
            alarm.alarmTime.toServerTime.toFullString,
          ),
          if (alarm.alarmDescription.isNotEmpty) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Text(
              alarm.alarmDescription,
              style: AppText.caption,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          SizedBox(height: AppSpacing.sm),
          Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  alarm.alarmTime.fromNow,
                  style: AppText.hint,
                ),
              ),
              Text(
                '查看详情 >',
                style: AppText.caption.copyWith(color: AppColors.primary),
              ),
            ],
          ),
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
            text.isEmpty ? '--' : text,
            style: AppText.hint,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );
  }

  // ── 详情弹层 ────────────────────────────────────────────────

  /// 展示详情。
  ///
  /// App 端只读，详情所需字段列表接口已全部返回，无需再请求详情端点
  /// （`/app/gb` 也没有单独的报警详情接口）。
  void _showDetail(AppGbAlarm alarm) {
    final bool handled = alarm.handled ?? false;

    showModalBottomSheet<void>(
      context: context,
      backgroundColor: AppColors.card,
      isScrollControlled: true,
      builder: (BuildContext ctx) {
        return SafeArea(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.lg,
              AppSpacing.pagePadding,
              AppSpacing.lg,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text('报警详情', style: AppText.section),
                    ),
                    IconButton(
                      onPressed: () => Navigator.of(ctx).pop(),
                      icon: const Icon(Icons.close),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                InfoRow(
                  label: '报警类型',
                  value: _alarmTypeText(alarm.alarmType),
                ),
                InfoRow(
                  label: '报警级别',
                  value: _priorityFullText(alarm.alarmPriority),
                  valueColor: _priorityColor(alarm.alarmPriority),
                ),
                InfoRow(
                  label: '报警时间',
                  value: alarm.alarmTime.toServerTime.toFullString,
                ),
                InfoRow(
                  label: '设备',
                  value: alarm.deviceName.orPlaceholder('--'),
                ),
                InfoRow(
                  label: '通道',
                  value: alarm.channelName.orPlaceholder('--'),
                ),
                InfoRow(
                  label: '处理状态',
                  value: handled ? '已处理' : '未处理',
                  valueColor:
                      handled ? AppColors.success : AppColors.warning,
                ),
                InfoRow(
                  label: '报警描述',
                  value: alarm.alarmDescription.orPlaceholder('--'),
                ),
                if (alarm.snapPath.isNotEmpty) ...<Widget>[
                  SizedBox(height: AppSpacing.sm),
                  Text('报警抓图', style: AppText.caption),
                  SizedBox(height: AppSpacing.sm),
                  SizedBox(
                    width: double.infinity,
                    child: AspectRatio(
                      aspectRatio: 16 / 9,
                      child: RemoteImage(
                        url: alarm.snapPath,
                        fit: BoxFit.contain,
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }

  // ── 枚举文案 ────────────────────────────────────────────────

  static String _alarmTypeText(String type) =>
      _alarmTypeLabels[type] ?? type.orPlaceholder('未知类型');

  static String _priorityFullText(String priority) =>
      _priorityLabels[priority] ?? priority.orPlaceholder('--');

  /// 列表里的紧凑文案（去掉括号说明）。
  static String _priorityShortText(String priority) {
    switch (priority) {
      case '1':
        return '一级';
      case '2':
        return '二级';
      case '3':
        return '三级';
      case '4':
        return '四级';
      default:
        return priority.isEmpty ? '--' : priority;
    }
  }

  static Color _priorityColor(String priority) {
    switch (priority) {
      case '1':
        return AppColors.danger;
      case '2':
        return AppColors.warning;
      case '3':
        return AppColors.primary;
      default:
        return AppColors.textTertiary;
    }
  }
}
