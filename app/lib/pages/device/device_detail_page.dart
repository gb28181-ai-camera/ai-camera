import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Definition.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/empty_state.dart';
import '../../widgets/entity_list_page.dart';
import '../../widgets/loading_indicator.dart';
import '../../widgets/stat_card.dart';
import '../../layout/Gaps.dart';

/// 设备详情页。
///
/// 路由：`/devices/detail?id=<id>`。
///
/// 展示运行状态、电量、电压、存储、固件版本、模型版本、设备模式、
/// 位置与最近在线时间。
class DeviceDetailPage extends StatefulWidget {
  /// 构造设备详情页。
  const DeviceDetailPage({super.key});

  @override
  State<DeviceDetailPage> createState() => _DeviceDetailPageState();
}

class _DeviceDetailPageState extends State<DeviceDetailPage> {
  int? _id;
  Device? _device;
  bool _loading = true;
  String? _error;
  bool _initialized = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;
    _id = int.tryParse(
      GoRouterState.of(context).uri.queryParameters['id'] ?? '',
    );
    if (_id == null) {
      _loading = false;
      _error = '缺少有效的设备 ID';
      return;
    }
    _load();
  }

  /// 加载设备详情。
  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final RDevice resp = await AppController.deviceDetailUsingGet(_id!);
      if (!mounted) return;
      setState(() {
        _device = resp.isSuccess ? resp.value : null;
        _error = resp.isSuccess ? null : resp.errorMessage;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '加载失败，请重试';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('设备详情')),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const LoadingIndicator(text: '加载中...');
    }

    final Device? device = _device;
    if (device == null) {
      return EmptyState.error(message: _error, onRetry: _load);
    }

    return ListView(
      padding: EdgeInsets.only(bottom: AppSpacing.xxl),
      children: <Widget>[
        _buildHeader(device),
        _buildRuntimeCard(device),
        _buildVersionCard(device),
        _buildLocationCard(device),
      ],
    );
  }

  /// 顶部状态区。
  Widget _buildHeader(Device device) {
    final Color color = AppColors.deviceStatusColor(device.status);

    return Container(
      width: double.infinity,
      color: AppColors.card,
      padding: EdgeInsets.all(AppSpacing.xl),
      child: Column(
        children: <Widget>[
          Container(
            width: 64.dp,
            height: 64.dp,
            decoration: BoxDecoration(
              color: color.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(AppSpacing.radiusCard * 1.5),
            ),
            child: Icon(Icons.videocam_outlined, size: 32.dp, color: color),
          ),
          SizedBox(height: AppSpacing.md),
          Text(
            device.name.orPlaceholder('未命名设备'),
            style: AppText.title,
            textAlign: TextAlign.center,
          ),
          SizedBox(height: AppSpacing.xs),
          Text(
            '编号 ${device.deviceCode.orPlaceholder('--')}',
            style: AppText.hint,
          ),
          SizedBox(height: AppSpacing.md),
          StatusTag(
            text: device.isOnline ? '在线' : '离线',
            color: color,
            filled: true,
          ),
        ],
      ),
    );
  }

  /// 运行指标。
  Widget _buildRuntimeCard(Device device) {
    return _card(
      title: '运行状态',
      children: <Widget>[
        // 电量条
        InfoRow(
          label: '电量',
          valueWidget: Row(
            children: <Widget>[
              Expanded(
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
                  child: LinearProgressIndicator(
                    value: (device.battery / 100).clamp(0.0, 1.0),
                    minHeight: 8,
                    backgroundColor: AppColors.fill,
                    valueColor: AlwaysStoppedAnimation<Color>(
                      device.battery >= 20
                          ? AppColors.success
                          : AppColors.danger,
                    ),
                  ),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Text('${device.battery}%', style: AppText.caption),
            ],
          ),
        ),
        InfoRow(label: '电压', value: device.voltage),
        InfoRow(
          label: '存储',
          valueWidget: Row(
            children: <Widget>[
              Expanded(
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
                  child: LinearProgressIndicator(
                    value: device.storageUsedRatio,
                    minHeight: 8,
                    backgroundColor: AppColors.fill,
                    valueColor: const AlwaysStoppedAnimation<Color>(
                      AppColors.warning,
                    ),
                  ),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Text(
                '${device.storageFree}/${device.storageTotal} MB',
                style: AppText.caption,
              ),
            ],
          ),
        ),
        InfoRow(
          label: '设备模式',
          value: device.deviceMode,
        ),
        InfoRow(label: '工作模式', value: device.workMode),
        InfoRow(
          label: '信息间隔',
          value: device.infoInterval > 0 ? '${device.infoInterval} 秒' : '',
        ),
        InfoRow(
          label: '实时监控',
          valueWidget: StatusTag(
            text: device.monitorEnabled ? '已开启' : '未开启',
            color: device.monitorEnabled
                ? AppColors.success
                : AppColors.textTertiary,
          ),
        ),
      ],
    );
  }

  /// 版本信息。
  Widget _buildVersionCard(Device device) {
    return _card(
      title: '版本信息',
      children: <Widget>[
        InfoRow(label: '固件版本', value: device.firmwareVer),
        InfoRow(label: '模型版本', value: device.modelVer),
        InfoRow(label: '系统版本', value: device.systemVer),
      ],
    );
  }

  /// 位置与在线时间。
  Widget _buildLocationCard(Device device) {
    return _card(
      title: '位置与在线',
      children: <Widget>[
        InfoRow(label: '所属区域', value: device.regionName),
        InfoRow(label: '设备位置', value: device.location),
        InfoRow(
          label: '最近在线',
          value: device.onlineDT.orPlaceholder('--'),
        ),
        InfoRow(
          label: '最近离线',
          value: device.offlineDT.orPlaceholder('--'),
        ),
        InfoRow(
          label: '最近心跳',
          value: device.lastBeatDT.orPlaceholder('--'),
        ),
        if (device.remark.isNotEmpty)
          InfoRow(label: '备注', value: device.remark),
      ],
    );
  }

  /// 统一卡片外观。
  Widget _card({required String title, required List<Widget> children}) {
    return Container(
      margin: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        0,
      ),
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text(title, style: AppText.section),
          SizedBox(height: AppSpacing.sm),
          ...children,
        ],
      ),
    );
  }
}
