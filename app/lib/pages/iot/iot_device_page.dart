import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../layout/Gaps.dart';
import '../../services/IotAppController.dart';
import '../../services/generated/GbController.dart';
import '../../services/generated/definitions/AppGb.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/entity_list_page.dart';
import '../../widgets/stat_card.dart';

/// 我的设备（当前用户已绑定的 IOT 设备）。
///
/// 点击设备进入实时播放：IOT 设备复用 `ac_device`，视频通道仍由国标通道提供，
/// 因此先按 `device_id` 查通道，再带 `deviceId` + `mode=mqtt` 进入播放页 ——
/// 播放页据此拉取该设备的 MQTT 服务组做信令，失败时自动回退 WHEP。
class IotDevicePage extends StatefulWidget {
  /// 构造我的设备页。
  const IotDevicePage({super.key});

  @override
  State<IotDevicePage> createState() => _IotDevicePageState();
}

class _IotDevicePageState extends State<IotDevicePage> {
  /// 刷新令牌：从播放页 / 添加页返回时递增。
  int _tick = 0;

  /// 正在查询通道的设备 ID（防止重复点击）。
  int? _openingId;

  /// 分页取数。
  Future<PageDataLike<IotDevice>> _fetch(int page, int pageSize) async {
    final RIotDeviceList resp = await IotAppController.devices(
      page: page,
      pageSize: pageSize,
    );
    return PageDataAdapter<IotDevice>(
      total: resp.total,
      items: resp.items,
      hasMore: resp.items.length < resp.total,
    );
  }

  /// 进入播放：先解析该设备的通道。
  Future<void> _openPlay(IotDevice device) async {
    if (_openingId != null) return;
    setState(() => _openingId = device.id);
    try {
      final RAppGbChannelPage resp = await GbController.channelsUsingGet(
        deviceId: device.id,
        pageSize: 50,
      );
      if (!mounted) return;
      if (!resp.isSuccess) {
        context.showToast(resp.errorMessage);
        return;
      }
      final List<AppGbChannel> channels = resp.items;
      if (channels.isEmpty) {
        context.showToast('该设备暂无可用视频通道');
        return;
      }

      AppGbChannel? channel;
      if (channels.length == 1) {
        channel = channels.first;
      } else {
        channel = await _chooseChannel(device, channels);
      }
      if (channel == null || !mounted) return;

      final String name = Uri.encodeComponent(
        channel.name.isEmpty ? channel.gbChannelId : channel.name,
      );
      await context.push<void>(
        '/gb/live?channelId=${channel.id}&name=$name'
        '&deviceId=${device.id}&mode=mqtt',
      );
      if (mounted) setState(() => _tick++);
    } finally {
      if (mounted) setState(() => _openingId = null);
    }
  }

  /// 多通道时让用户选择。
  Future<AppGbChannel?> _chooseChannel(
    IotDevice device,
    List<AppGbChannel> channels,
  ) {
    return showModalBottomSheet<AppGbChannel>(
      context: context,
      backgroundColor: AppColors.card,
      isScrollControlled: true,
      builder: (BuildContext ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.lg,
                AppSpacing.lg,
                AppSpacing.sm,
              ),
              child: Text(
                '${device.name} · 选择通道',
                style: AppText.section,
              ),
            ),
            ConstrainedBox(
              constraints: BoxConstraints(
                maxHeight: MediaQuery.of(ctx).size.height * 0.6,
              ),
              child: ListView.builder(
                shrinkWrap: true,
                itemCount: channels.length,
                itemBuilder: (BuildContext context, int index) {
                  final AppGbChannel channel = channels[index];
                  return ListTile(
                    key: Key('iot-channel-${channel.id}'),
                    leading: Icon(
                      Icons.videocam_outlined,
                      color: channel.isOnline
                          ? AppColors.success
                          : AppColors.offline,
                    ),
                    title: Text(
                      channel.name.isEmpty ? channel.gbChannelId : channel.name,
                      style: AppText.bodyStrong,
                    ),
                    subtitle: Text(
                      <String>[
                        channel.regionName,
                        channel.groupName,
                      ].where((String s) => s.isNotEmpty).join(' · '),
                      style: AppText.hint,
                    ),
                    trailing: Text(
                      channel.isOnline ? '在线' : '离线',
                      style: AppText.hint.copyWith(
                        color: channel.isOnline
                            ? AppColors.success
                            : AppColors.textTertiary,
                      ),
                    ),
                    onTap: () => Navigator.of(ctx).pop(channel),
                  );
                },
              ),
            ),
            SizedBox(height: AppSpacing.sm),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return EntityListPage<IotDevice>(
      title: '我的设备',
      reloadToken: '$_tick',
      fetcher: _fetch,
      emptyText: '还没有绑定设备',
      emptyIcon: Icons.videocam_off_outlined,
      header: _buildHeader(),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(bottom: AppSpacing.xxl, top: AppSpacing.xs),
      actions: <Widget>[
        IconButton(
          key: const Key('iot-devices-add'),
          tooltip: '添加设备',
          onPressed: () async {
            await context.push<void>('/iot/add');
            if (mounted) setState(() => _tick++);
          },
          icon: const Icon(Icons.add),
        ),
      ],
      itemBuilder: (BuildContext context, IotDevice device, int index) =>
          IotDeviceCard(
        key: Key('iot-device-${device.id}'),
        device: device,
        opening: _openingId == device.id,
        onTap: () => _openPlay(device),
        // 只有 owner 能分享（后端以 owner 为边界，shared 调用会 403），
        // 且当前只有适老化产品需要分享给家人；其余情况不渲染按钮。
        onShare: device.canShare ? () => _openShare(device) : null,
      ),
    );
  }

  /// 打开分享页（生成一次性分享码）。
  ///
  /// 用 query 参数传设备 ID / 名称，与项目内其他详情页保持一致。
  Future<void> _openShare(IotDevice device) async {
    final String name = Uri.encodeComponent(device.name);
    await context.push<void>('/iot/share?deviceId=${device.id}&name=$name');
  }

  Widget _buildHeader() {
    return Container(
      color: AppColors.card,
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.md,
      ),
      child: Row(
        children: <Widget>[
          Expanded(
            child: Text(
              '点击设备查看实时画面（MQTT 信令，失败自动回退）',
              style: AppText.hint,
            ),
          ),
          TextButton.icon(
            key: const Key('iot-devices-add-inline'),
            onPressed: () async {
              await context.push<void>('/iot/add');
              if (mounted) setState(() => _tick++);
            },
            icon: const Icon(Icons.add, size: 18),
            label: const Text('添加'),
          ),
        ],
      ),
    );
  }

}

/// 我的设备列表项。
///
/// 独立成公开 Widget 是为了让「分享按钮显隐」这类权限规则可以被组件测试覆盖：
/// 规则很容易在重构中退化（比如把 `canShare` 写成只看产品类型），
/// 而完整列表页依赖网络与路由，不适合做细粒度断言。
class IotDeviceCard extends StatelessWidget {
  /// 构建设备卡片。
  const IotDeviceCard({
    super.key,
    required this.device,
    required this.onTap,
    this.onShare,
    this.opening = false,
  });

  /// 设备数据。
  final IotDevice device;

  /// 点击卡片（进入实时播放）。
  final VoidCallback onTap;

  /// 分享回调；为 null 时不渲染分享按钮。
  final VoidCallback? onShare;

  /// 是否正在解析播放通道。
  final bool opening;

  @override
  Widget build(BuildContext context) {
    final Color statusColor =
        device.online ? AppColors.success : AppColors.offline;

    return EntityCard(
      onTap: onTap,
      child: Row(
        children: <Widget>[
          Container(
            width: 44.dp,
            height: 44.dp,
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
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
                      text: device.online ? '在线' : '离线',
                      color: statusColor,
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.xs),
                Wrap(
                  spacing: AppSpacing.xs,
                  runSpacing: AppSpacing.xs,
                  children: <Widget>[
                    StatusTag(
                      key: Key('iot-device-role-${device.id}'),
                      text: device.roleLabel,
                      color: device.isOwner
                          ? AppColors.success
                          : AppColors.textSecondary,
                    ),
                  ],
                ),
                SizedBox(height: 2.dp),
                Text(
                  '${device.productName.orPlaceholder('未绑定产品')} · '
                  '编号 ${device.deviceCode.orPlaceholder('--')}',
                  style: AppText.hint,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          if (opening)
            SizedBox(
              width: 18.dp,
              height: 18.dp,
              child: const CircularProgressIndicator(strokeWidth: 2),
            )
          else if (onShare != null)
            Column(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                TextButton(
                  key: Key('iot-device-share-${device.id}'),
                  style: TextButton.styleFrom(
                    padding: EdgeInsets.symmetric(
                      horizontal: AppSpacing.sm,
                      vertical: 0,
                    ),
                    minimumSize: Size(0, 30.dp),
                    tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  ),
                  onPressed: onShare,
                  child: const Text('分享'),
                ),
                Icon(
                  Icons.play_circle_outline,
                  size: 22.dp,
                  color: AppColors.primary,
                ),
              ],
            )
          else
            Icon(
              Icons.play_circle_outline,
              size: 22.dp,
              color: AppColors.primary,
            ),
        ],
      ),
    );
  }
}
