import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../layout/Gaps.dart';
import '../../services/generated/GbController.dart';
import '../../services/generated/definitions/AppGb.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/entity_list_page.dart';
import '../../widgets/stat_card.dart';

/// 国标设备 / 通道浏览页。
///
/// 交互：设备列表 → 点击设备展开通道 → 点击通道进入实时播放页。
/// 设备列表复用 [EntityListPage]（下拉刷新、上滑分页、空态/错误态一致），
/// 通道列表按设备懒加载（首次展开才请求），避免一次性拉全量。
class GbDeviceListPage extends StatefulWidget {
  /// 构建设备列表页。
  const GbDeviceListPage({super.key});

  @override
  State<GbDeviceListPage> createState() => _GbDeviceListPageState();
}

class _GbDeviceListPageState extends State<GbDeviceListPage> {
  /// 设备运行状态筛选，空串表示全部。
  String _status = '';

  /// 刷新令牌：筛选变化 / 从播放页返回时递增。
  int _tick = 0;

  /// 已展开的设备 ID。
  final Set<int> _expanded = <int>{};

  /// 设备 ID → 通道列表缓存。
  final Map<int, List<AppGbChannel>> _channels = <int, List<AppGbChannel>>{};

  /// 设备 ID → 通道总数（用于提示「仅显示前 N 条」）。
  final Map<int, int> _channelTotals = <int, int>{};

  /// 正在请求通道的设备 ID。
  final Set<int> _channelLoading = <int>{};

  /// 设备 ID → 通道加载错误。
  final Map<int, String> _channelErrors = <int, String>{};

  /// 可选筛选项。
  static const List<({String value, String label})> _filters =
      <({String value, String label})>[
    (value: '', label: '全部'),
    (value: 'online', label: '在线'),
    (value: 'offline', label: '离线'),
  ];

  /// 分页取数。
  Future<PageDataLike<AppGbDevice>> _fetch(int page, int pageSize) async {
    final RAppGbDevicePage resp = await GbController.devicesUsingGet(
      page: page,
      pageSize: pageSize,
      status: _status.isEmpty ? null : _status,
    );
    return PageDataAdapter<AppGbDevice>(
      total: resp.total,
      items: resp.items,
      hasMore: resp.page.hasMore,
    );
  }

  /// 切换设备展开/收起；首次展开时加载通道。
  void _toggleDevice(AppGbDevice device) {
    if (_expanded.contains(device.id)) {
      setState(() => _expanded.remove(device.id));
      return;
    }
    setState(() => _expanded.add(device.id));
    if (!_channels.containsKey(device.id) &&
        !_channelLoading.contains(device.id)) {
      _loadChannels(device.id);
    }
  }

  /// 加载某设备的通道列表。
  ///
  /// 单设备通道通常不多，这里一次取 100 条；超过时在底部提示「仅显示前 100 条」。
  /// 真遇到超大设备（>100 通道）时，可再补分页加载。
  Future<void> _loadChannels(int deviceId, {bool force = false}) async {
    if (_channelLoading.contains(deviceId)) return;
    setState(() {
      _channelLoading.add(deviceId);
      _channelErrors.remove(deviceId);
      if (force) _channels.remove(deviceId);
    });
    try {
      final RAppGbChannelPage resp = await GbController.channelsUsingGet(
        deviceId: deviceId,
        pageSize: 100,
      );
      if (!mounted) return;
      setState(() {
        _channelLoading.remove(deviceId);
        if (resp.isSuccess) {
          _channels[deviceId] = resp.items;
          _channelTotals[deviceId] = resp.total;
        } else {
          _channelErrors[deviceId] = resp.errorMessage;
        }
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _channelLoading.remove(deviceId);
        _channelErrors[deviceId] = '通道加载失败，请重试';
      });
    }
  }

  /// 进入实时播放页。
  ///
  /// 返回后刷新设备列表：播放会触发设备侧建流，在线状态/流状态可能变化。
  Future<void> _openLive(AppGbChannel channel) async {
    final String name = Uri.encodeComponent(
      channel.name.isEmpty ? channel.gbChannelId : channel.name,
    );
    await context.push<void>('/gb/live?channelId=${channel.id}&name=$name');
    if (!mounted) return;
    setState(() => _tick++);
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<AppGbDevice>(
      title: '国标监控',
      reloadToken: '$_status#$_tick',
      fetcher: _fetch,
      emptyText: '暂无国标设备',
      emptyIcon: Icons.videocam_off_outlined,
      header: _buildHeader(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (
        BuildContext context,
        AppGbDevice device,
        int index,
      ) =>
          _buildDeviceCard(device),
    );
  }

  // ── 顶部筛选 ────────────────────────────────────────────────

  Widget _buildHeader() {
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
            '选择设备展开通道，点击通道查看实时画面',
            style: AppText.hint,
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: <Widget>[
              for (final ({String value, String label}) f in _filters)
                ...<Widget>[
                  ChoiceChip(
                    key: Key(
                      'gb-filter-${f.value.isEmpty ? 'all' : f.value}',
                    ),
                    label: Text(f.label),
                    selected: _status == f.value,
                    onSelected: (_) {
                      if (_status == f.value) return;
                      setState(() {
                        _status = f.value;
                        _tick++;
                      });
                    },
                  ),
                  SizedBox(width: AppSpacing.sm),
                ],
            ],
          ),
        ],
      ),
    );
  }

  // ── 设备卡片 ────────────────────────────────────────────────

  Widget _buildDeviceCard(AppGbDevice device) {
    final bool expanded = _expanded.contains(device.id);
    final Color statusColor = AppColors.deviceStatusColor(device.status);

    return EntityCard(
      key: Key('gb-device-${device.id}'),
      onTap: () => _toggleDevice(device),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Container(
                width: 44.dp,
                height: 44.dp,
                decoration: BoxDecoration(
                  color: statusColor.withValues(alpha: 0.12),
                  borderRadius:
                      BorderRadius.circular(AppSpacing.radiusControl),
                ),
                child: Icon(
                  Icons.videocam_outlined,
                  size: 22.dp,
                  color: statusColor,
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
                          text: device.isOnline ? '在线' : '离线',
                          color: statusColor,
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
                        Text(
                          '${device.channelCount} 个通道',
                          style: AppText.hint,
                        ),
                      ],
                    ),
                  ],
                ),
              ),
              Icon(
                expanded ? Icons.expand_less : Icons.expand_more,
                size: 20.dp,
                color: AppColors.textTertiary,
              ),
            ],
          ),
          if (expanded) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Divider(
              height: AppSpacing.md,
              thickness: AppSpacing.dividerThickness,
              color: AppColors.divider,
            ),
            ..._buildChannelSection(device),
          ],
        ],
      ),
    );
  }

  // ── 设备下方内联的通道列表 ───────────────────────────────────

  List<Widget> _buildChannelSection(AppGbDevice device) {
    if (_channelLoading.contains(device.id)) {
      return <Widget>[
        Padding(
          padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: <Widget>[
              SizedBox(
                width: 16.dp,
                height: 16.dp,
                child: const CircularProgressIndicator(strokeWidth: 2),
              ),
              SizedBox(width: AppSpacing.sm),
              Text('通道加载中…', style: AppText.hint),
            ],
          ),
        ),
      ];
    }

    final String? error = _channelErrors[device.id];
    if (error != null) {
      return <Widget>[
        Padding(
          padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
          child: Row(
            children: <Widget>[
              Expanded(
                child: Text(
                  error,
                  style: AppText.hint.copyWith(color: AppColors.danger),
                ),
              ),
              TextButton(
                key: Key('gb-channel-retry-${device.id}'),
                onPressed: () => _loadChannels(device.id, force: true),
                child: const Text('重试'),
              ),
            ],
          ),
        ),
      ];
    }

    final List<AppGbChannel> channels =
        _channels[device.id] ?? const <AppGbChannel>[];
    if (channels.isEmpty) {
      return <Widget>[
        Padding(
          padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
          child: Text('暂无通道', style: AppText.hint),
        ),
      ];
    }

    final int total = _channelTotals[device.id] ?? channels.length;
    return <Widget>[
      for (final AppGbChannel channel in channels) ...<Widget>[
        _buildChannelTile(channel),
        SizedBox(height: AppSpacing.xs),
      ],
      if (total > channels.length)
        Padding(
          padding: EdgeInsets.only(top: AppSpacing.xs),
          child: Text(
            '共 $total 个通道，仅显示前 ${channels.length} 个',
            style: AppText.hint,
          ),
        ),
    ];
  }

  /// 单个通道条目。
  Widget _buildChannelTile(AppGbChannel channel) {
    final Color color = AppColors.deviceStatusColor(
      channel.isOnline ? 'online' : 'offline',
    );

    return Material(
      key: Key('gb-channel-${channel.id}'),
      color: AppColors.fill,
      borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
      child: InkWell(
        borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
        onTap: () => _openLive(channel),
        child: Padding(
          padding: EdgeInsets.symmetric(
            horizontal: AppSpacing.md,
            vertical: AppSpacing.sm,
          ),
          child: Row(
            children: <Widget>[
              Icon(Icons.videocam_outlined, size: 18.dp, color: color),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      channel.name.orPlaceholder(channel.gbChannelId),
                      style: AppText.body.copyWith(
                        color: AppColors.textPrimary,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (channel.regionName.isNotEmpty ||
                        channel.groupName.isNotEmpty) ...<Widget>[
                      SizedBox(height: 2.dp),
                      Text(
                        <String>[
                          channel.regionName,
                          channel.groupName,
                        ].where((String s) => s.isNotEmpty).join(' · '),
                        style: AppText.hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ],
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              StatusTag(
                text: channel.isOnline ? '在线' : '离线',
                color: color,
              ),
              SizedBox(width: AppSpacing.xs),
              Icon(
                Icons.play_circle_outline,
                size: 20.dp,
                color: AppColors.primary,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
