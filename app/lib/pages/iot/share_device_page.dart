import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../layout/Gaps.dart';
import '../../services/IotAppController.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';

/// 设备分享页。
///
/// 只有适老化智能相机的 owner 才会进入这里：后端
/// `POST /app/iot/devices/:id/share-token` 会创建「尚未绑定用户」的 shared
/// 占位账号并返回一次性令牌；家人用 App 扫分享码后走 `/app/iot/devices/claim`
/// 完成绑定。
///
/// **二维码渲染说明**：当前 `pubspec.yaml` 没有 `qr_flutter`，
/// 按约定不新增依赖，因此这里展示 `qr_content` 文本并提供复制。
/// 后端若判断 `qr_content` 是链接，后续接入二维码库时只需替换展示区，
/// 生成 / 复制逻辑无需改动。
class ShareDevicePage extends StatelessWidget {
  /// 构造分享页。
  const ShareDevicePage({
    super.key,
    required this.deviceId,
    this.deviceName = '',
  });

  /// 设备 ID（`ac_device.id`）。
  final int deviceId;

  /// 设备名称（仅用于展示，可为空）。
  final String deviceName;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('分享设备')),
      backgroundColor: AppColors.background,
      body: SingleChildScrollView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        child: DeviceShareView(
          deviceId: deviceId,
          deviceName: deviceName,
        ),
      ),
    );
  }
}

/// 以底部弹层的形式打开分享视图（蓝牙配网成功页使用）。
///
/// 弹层与整页复用同一个 [DeviceShareView]，避免「配网页一套、列表页另一套」
/// 的生成/复制逻辑分叉。
Future<void> showDeviceShareSheet(
  BuildContext context, {
  required int deviceId,
  String deviceName = '',
}) {
  return showModalBottomSheet<void>(
    context: context,
    isScrollControlled: true,
    backgroundColor: AppColors.card,
    builder: (BuildContext sheetContext) {
      return SafeArea(
        child: Padding(
          padding: EdgeInsets.only(
            bottom: MediaQuery.of(sheetContext).viewInsets.bottom,
          ),
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.lg,
              AppSpacing.lg,
              AppSpacing.lg,
              AppSpacing.lg,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Row(
                  children: <Widget>[
                    Expanded(
                      child: Text('生成分享二维码', style: AppText.section),
                    ),
                    IconButton(
                      tooltip: '关闭',
                      onPressed: () => Navigator.of(sheetContext).pop(),
                      icon: const Icon(Icons.close, size: 20),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                DeviceShareView(
                  deviceId: deviceId,
                  deviceName: deviceName,
                ),
              ],
            ),
          ),
        ),
      );
    },
  );
}

/// 分享码生成与展示（页面 / 弹层共用）。
class DeviceShareView extends StatefulWidget {
  /// 构造分享视图。
  const DeviceShareView({
    super.key,
    required this.deviceId,
    this.deviceName = '',
  });

  /// 设备 ID。
  final int deviceId;

  /// 设备名称。
  final String deviceName;

  @override
  State<DeviceShareView> createState() => _DeviceShareViewState();
}

class _DeviceShareViewState extends State<DeviceShareView> {
  /// 初始即为 true：首帧就展示「生成中」，避免请求返回前闪一段空白。
  bool _loading = true;
  String? _error;
  IotDeviceShareToken? _share;

  @override
  void initState() {
    super.initState();
    // 用户点击「分享」即代表一次明确的分享意图，直接生成，减少一次点击；
    // 重复生成走下方「重新生成」按钮。
    //
    // 放在首帧后执行：initState 仍处于父级 build 期间，直接 setState 会触发
    // “setState() called during build” 断言；首帧后请求也保证界面先出现 loading。
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) unawaited(_generate());
    });
  }

  /// 调用后端生成分享令牌。
  Future<void> _generate() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    final RIotDeviceShareToken resp =
        await IotAppController.createDeviceShareToken(widget.deviceId);
    if (!mounted) return;
    if (!resp.isSuccess || resp.share == null) {
      setState(() {
        _loading = false;
        _error = resp.errorMessage;
      });
      return;
    }
    setState(() {
      _loading = false;
      _share = resp.share;
    });
  }

  /// 复制分享码（`qr_content` 链接）。
  Future<void> _copy() async {
    final IotDeviceShareToken? share = _share;
    if (share == null) return;
    await Clipboard.setData(ClipboardData(text: share.shareText));
    if (!mounted) return;
    context.showToast('分享码已复制');
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        if (widget.deviceName.isNotEmpty) ...<Widget>[
          Text(
            widget.deviceName,
            style: AppText.bodyStrong,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          SizedBox(height: AppSpacing.xs),
        ],
        Text(
          '让家人用 App 扫此码加入，一次性有效',
          style: AppText.hint,
        ),
        SizedBox(height: AppSpacing.md),
        _buildContent(),
      ],
    );
  }

  Widget _buildContent() {
    if (_loading) {
      return Container(
        key: const Key('share-device-loading'),
        padding: EdgeInsets.all(AppSpacing.xl),
        decoration: BoxDecoration(
          color: AppColors.fill,
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        ),
        child: Row(
          children: <Widget>[
            SizedBox(
              width: 18.dp,
              height: 18.dp,
              child: const CircularProgressIndicator(strokeWidth: 2),
            ),
            SizedBox(width: AppSpacing.md),
            Text('正在生成分享码…', style: AppText.body),
          ],
        ),
      );
    }

    if (_error != null) {
      return Container(
        padding: EdgeInsets.all(AppSpacing.lg),
        decoration: BoxDecoration(
          color: AppColors.dangerLight,
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text(
              _error!,
              key: const Key('share-device-error'),
              style: AppText.body.copyWith(color: AppColors.danger),
            ),
            SizedBox(height: AppSpacing.md),
            OutlinedButton(
              key: const Key('share-device-retry'),
              onPressed: () => unawaited(_generate()),
              child: const Text('重试'),
            ),
          ],
        ),
      );
    }

    final IotDeviceShareToken? share = _share;
    if (share == null) return const SizedBox.shrink();

    return Container(
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.fill,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          // 没有二维码图形库时，二维码内容本身就是唯一可传递的凭证。
          // 用 SelectableText 便于长按复制，同时保留显式复制按钮。
          SelectableText(
            share.shareText,
            key: const Key('share-device-token'),
            style: AppText.body.copyWith(fontFamily: 'monospace'),
          ),
          SizedBox(height: AppSpacing.md),
          Row(
            children: <Widget>[
              // 主题里的 OutlinedButton 使用 `Size.fromHeight`（宽度 infinity），
              // 作为 Row 的非弹性子节点会拿到无界宽度并触发布局断言；
              // 两枚按钮都放进 Expanded 以获得确定宽度。
              Expanded(
                child: FilledButton.icon(
                  key: const Key('share-device-copy'),
                  onPressed: () => unawaited(_copy()),
                  icon: const Icon(Icons.copy, size: 18),
                  label: const Text('复制分享码'),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: OutlinedButton(
                  key: const Key('share-device-regenerate'),
                  onPressed: () => unawaited(_generate()),
                  child: const Text('重新生成'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
