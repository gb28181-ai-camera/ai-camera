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

/// 设备列表页。
///
/// 展示设备名称 / 编号 / 区域 / 运行状态 / 电量，点击进入详情。
class DeviceListPage extends StatefulWidget {
  /// 构造设备列表页。
  const DeviceListPage({super.key});

  @override
  State<DeviceListPage> createState() => _DeviceListPageState();
}

class _DeviceListPageState extends State<DeviceListPage> {
  /// 当前筛选状态，空串表示全部。
  String _status = '';

  /// 刷新令牌。
  int _tick = 0;

  bool _initialized = false;

  /// 可选筛选项。
  static const List<({String value, String label})> _filters =
      <({String value, String label})>[
    (value: '', label: '全部'),
    (value: 'online', label: '在线'),
    (value: 'offline', label: '离线'),
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
  Future<PageDataLike<Device>> _fetch(int page, int pageSize) async {
    final RPageDevice resp = await AppController.devicesUsingGet(
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        if (_status.isNotEmpty) 'status': _status,
      },
    );
    return PageDataAdapter<Device>(
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

  /// 进入设备详情。
  Future<void> _openDetail(Device device) async {
    await context.push<bool>('/devices/detail?id=${device.id}');
    if (!mounted) return;
    setState(() => _tick++);
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<Device>(
      title: '设备',
      reloadToken: '$_status#$_tick',
      fetcher: _fetch,
      emptyText: '暂无设备',
      emptyIcon: Icons.videocam_off_outlined,
      header: _buildFilterBar(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (BuildContext context, Device device, int index) =>
          _buildItem(device),
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
              key: Key('device-filter-${f.value.isEmpty ? 'all' : f.value}'),
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

  /// 单台设备。
  Widget _buildItem(Device device) {
    final bool online = device.isOnline;

    return EntityCard(
      onTap: () => _openDetail(device),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Container(
            width: 44.dp,
            height: 44.dp,
            decoration: BoxDecoration(
              color: AppColors.deviceStatusColor(device.status)
                  .withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
            ),
            child: Icon(
              Icons.videocam_outlined,
              size: 22.dp,
              color: AppColors.deviceStatusColor(device.status),
            ),
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
                        device.name.orPlaceholder('未命名设备'),
                        style: AppText.bodyStrong,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    SizedBox(width: AppSpacing.xs),
                    StatusTag(
                      text: online ? '在线' : '离线',
                      color: AppColors.deviceStatusColor(device.status),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.xs),
                Text(
                  '编号 ${device.deviceCode.orPlaceholder('--')}',
                  style: AppText.hint,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
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
                        device.regionName.orPlaceholder('未分配区域'),
                        style: AppText.hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                // 电量进度条（纯 Widget，不引入图表库）。
                Row(
                  children: <Widget>[
                    Icon(
                      _batteryIcon(device.battery),
                      size: 14.dp,
                      color: _batteryColor(device.battery),
                    ),
                    SizedBox(width: AppSpacing.xs),
                    Expanded(
                      child: ClipRRect(
                        borderRadius:
                            BorderRadius.circular(AppSpacing.radiusTag),
                        child: LinearProgressIndicator(
                          value: (device.battery / 100).clamp(0.0, 1.0),
                          minHeight: 6,
                          backgroundColor: AppColors.fill,
                          valueColor: AlwaysStoppedAnimation<Color>(
                            _batteryColor(device.battery),
                          ),
                        ),
                      ),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Text(
                      '${device.battery}%',
                      style: AppText.hint,
                    ),
                  ],
                ),
              ],
            ),
          ),
          Icon(
            Icons.chevron_right,
            size: 20.dp,
            color: AppColors.textTertiary,
          ),
        ],
      ),
    );
  }

  static IconData _batteryIcon(int battery) {
    if (battery >= 80) return Icons.battery_full;
    if (battery >= 50) return Icons.battery_5_bar;
    if (battery >= 20) return Icons.battery_3_bar;
    return Icons.battery_alert_outlined;
  }

  static Color _batteryColor(int battery) {
    if (battery >= 50) return AppColors.success;
    if (battery >= 20) return AppColors.warning;
    return AppColors.danger;
  }
}
