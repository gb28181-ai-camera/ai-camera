import 'package:flutter/material.dart';

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

/// 通知列表页。
///
/// 支持分类筛选（全部 / 设备告警 / AI风险 / 巡检通知）与已读/未读标记；
/// 点击未读通知会调用 `PUT /app/notifications/{id}/read` 标记为已读。
class NotificationListPage extends StatefulWidget {
  /// 构造通知列表页。
  const NotificationListPage({super.key});

  @override
  State<NotificationListPage> createState() => _NotificationListPageState();
}

class _NotificationListPageState extends State<NotificationListPage> {
  /// 当前分类筛选，空串表示全部。
  String _category = '';

  /// 刷新令牌。
  int _tick = 0;

  /// 已在本页标记为已读的通知 ID（用于即时更新样式，无需整页刷新）。
  final Set<int> _locallyRead = <int>{};

  /// 可选筛选项。
  static const List<({String value, String label})> _filters =
      <({String value, String label})>[
    (value: '', label: '全部'),
    (value: 'alarm', label: '设备告警'),
    (value: 'risk', label: 'AI风险'),
    (value: 'inspect', label: '巡检通知'),
  ];

  /// 分页取数。
  Future<PageDataLike<NotificationItem>> _fetch(
    int page,
    int pageSize,
  ) async {
    final RPageNotification resp = await AppController.notificationsUsingGet(
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        if (_category.isNotEmpty) 'category': _category,
      },
    );
    return PageDataAdapter<NotificationItem>(
      total: resp.total,
      items: resp.items,
      hasMore: resp.page.hasMore,
    );
  }

  /// 切换分类筛选。
  void _onFilterChanged(String value) {
    if (_category == value) return;
    setState(() {
      _category = value;
      _tick++;
    });
  }

  /// 标记单条通知为已读。
  Future<void> _markRead(NotificationItem item) async {
    if (item.isRead || _locallyRead.contains(item.id)) return;

    setState(() => _locallyRead.add(item.id));
    final REmptyResponse resp =
        await AppController.markNotificationReadUsingPut(item.id);
    if (!mounted) return;
    if (!resp.ok) {
      // 失败则回滚本地已读标记。
      setState(() => _locallyRead.remove(item.id));
      context.showToast(resp.errorMessage);
    }
  }

  /// 该条是否已读（含本地即时标记）。
  bool _isRead(NotificationItem item) =>
      item.isRead || _locallyRead.contains(item.id);

  @override
  Widget build(BuildContext context) {
    return EntityListPage<NotificationItem>(
      title: '通知',
      reloadToken: '$_category#$_tick',
      fetcher: _fetch,
      emptyText: '暂无通知',
      emptyIcon: Icons.notifications_off_outlined,
      header: _buildFilterBar(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (
        BuildContext context,
        NotificationItem item,
        int index,
      ) =>
          _buildItem(item),
    );
  }

  /// 分类筛选条。
  Widget _buildFilterBar() {
    return Container(
      color: AppColors.card,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.sm,
      ),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: <Widget>[
            for (final ({String value, String label}) f in _filters)
              ...<Widget>[
              ChoiceChip(
                key: Key(
                  'notification-filter-${f.value.isEmpty ? 'all' : f.value}',
                ),
                label: Text(f.label),
                selected: _category == f.value,
                onSelected: (_) => _onFilterChanged(f.value),
              ),
              SizedBox(width: AppSpacing.sm),
            ],
          ],
        ),
      ),
    );
  }

  /// 单条通知。
  Widget _buildItem(NotificationItem item) {
    final bool read = _isRead(item);

    return EntityCard(
      onTap: () => _markRead(item),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          // 未读圆点
          Padding(
            padding: EdgeInsets.only(top: 6.dp, right: AppSpacing.sm),
            child: Container(
              width: 8.dp,
              height: 8.dp,
              decoration: BoxDecoration(
                color: read ? Colors.transparent : AppColors.danger,
                shape: BoxShape.circle,
              ),
            ),
          ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text(
                        item.title.orPlaceholder(item.categoryText),
                        style: read
                            ? AppText.body
                            : AppText.bodyStrong,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    SizedBox(width: AppSpacing.xs),
                    StatusTag(
                      text: item.categoryText,
                      color: _categoryColor(item.category),
                    ),
                  ],
                ),
                if (item.content.isNotEmpty) ...<Widget>[
                  SizedBox(height: AppSpacing.xs),
                  Text(
                    item.content,
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
                        item.createdDT.fromNow,
                        style: AppText.hint,
                      ),
                    ),
                    Text(
                      read ? '已读' : '未读',
                      style: AppText.hint.copyWith(
                        color: read
                            ? AppColors.textTertiary
                            : AppColors.danger,
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  static Color _categoryColor(String category) {
    switch (category) {
      case 'alarm':
        return AppColors.warning;
      case 'risk':
        return AppColors.danger;
      case 'inspect':
        return AppColors.primary;
      default:
        return AppColors.textSecondary;
    }
  }
}
