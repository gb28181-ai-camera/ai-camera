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

/// AI 风险列表页。
///
/// 支持状态筛选（全部 / 待处理 / 已处理），上滑加载更多。
/// 条目展示缩略图、风险类型、设备、区域、时间与状态标签。
class RiskListPage extends StatefulWidget {
  /// 构造风险列表页。
  const RiskListPage({super.key});

  @override
  State<RiskListPage> createState() => _RiskListPageState();
}

class _RiskListPageState extends State<RiskListPage> {
  /// 当前筛选状态，空串表示全部。
  String _status = '';

  /// 刷新令牌：详情/处理页返回后自增，驱动列表重新加载。
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
    // 支持从首页带参进入（如 `/risks?status=pending`）。
    final String? fromQuery =
        GoRouterState.of(context).uri.queryParameters['status'];
    if (fromQuery != null && fromQuery.isNotEmpty) {
      _status = fromQuery;
    }
  }

  /// 分页取数。
  Future<PageDataLike<RiskRecord>> _fetch(int page, int pageSize) async {
    final RPageRiskRecord resp = await AppController.risksUsingGet(
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        if (_status.isNotEmpty) 'status': _status,
      },
    );
    return PageDataAdapter<RiskRecord>(
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

  /// 进入详情页，返回后刷新。
  Future<void> _openDetail(RiskRecord risk) async {
    await context.push<bool>('/risks/detail?id=${risk.id}');
    if (!mounted) return;
    setState(() => _tick++);
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<RiskRecord>(
      title: 'AI 风险',
      reloadToken: '$_status#$_tick',
      fetcher: _fetch,
      emptyText: _status.isEmpty ? '暂无风险记录' : '暂无该状态的风险',
      emptyIcon: Icons.verified_outlined,
      header: _buildFilterBar(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (
        BuildContext context,
        RiskRecord risk,
        int index,
      ) =>
          _buildItem(risk),
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
              key: Key('risk-filter-${f.value.isEmpty ? 'all' : f.value}'),
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

  /// 单条风险。
  Widget _buildItem(RiskRecord risk) {
    return EntityCard(
      onTap: () => _openDetail(risk),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          RemoteImage(
            url: risk.imageUrl,
            width: AppSpacing.thumbnailSize,
            height: AppSpacing.thumbnailSize,
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text(
                        risk.riskTypeName.orPlaceholder('未知风险类型'),
                        style: AppText.bodyStrong,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    SizedBox(width: AppSpacing.xs),
                    StatusTag(
                      text: risk.isHandled ? '已处理' : '待处理',
                      color: AppColors.statusColor(risk.status),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.xs),
                Row(
                  children: <Widget>[
                    Icon(
                      Icons.videocam_outlined,
                      size: 13.dp,
                      color: AppColors.textTertiary,
                    ),
                    SizedBox(width: 3.dp),
                    Expanded(
                      child: Text(
                        risk.deviceName.orPlaceholder(risk.deviceCode),
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
                      Icons.location_on_outlined,
                      size: 13.dp,
                      color: AppColors.textTertiary,
                    ),
                    SizedBox(width: 3.dp),
                    Expanded(
                      child: Text(
                        risk.location.orPlaceholder(
                          risk.regionName.orPlaceholder('未分配区域'),
                        ),
                        style: AppText.hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.xs),
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text(
                        risk.occurredDT.fromNow,
                        style: AppText.hint,
                      ),
                    ),
                    if (risk.detectScore > 0)
                      Text(
                        '置信度 ${risk.detectScoreText}',
                        style: AppText.hint.copyWith(
                          color: AppColors.levelColor(risk.riskLevel),
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
}
