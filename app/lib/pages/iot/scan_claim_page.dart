import 'dart:async';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:mobile_scanner/mobile_scanner.dart';

import '../../layout/Gaps.dart';
import '../../services/BleProvisioner.dart';
import '../../services/IotAppController.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/empty_state.dart';
import '../../widgets/loading_indicator.dart';

/// 扫码添加设备页。
///
/// 按二维码 `type` 分流（见 [IotQrPayload]）：
/// - `device_share` → `POST /app/iot/devices/claim`（一次性令牌）；
/// - `device` → `POST /app/iot/devices/bind-qr`（产品编码 + 设备编码）。
///
/// 识别成功后立即停止相机，避免同一码被连续触发多次请求
/// （分享令牌是一次性的，重复提交第二次必然失败，会把成功提示盖成报错）。
class ScanClaimPage extends StatefulWidget {
  /// 构造扫码页。
  const ScanClaimPage({super.key});

  @override
  State<ScanClaimPage> createState() => _ScanClaimPageState();
}

class _ScanClaimPageState extends State<ScanClaimPage>
    with WidgetsBindingObserver {
  final MobileScannerController _controller = MobileScannerController(
    // 同一二维码在取景框内会持续回调；这里用 noDuplicates 减少重复事件，
    // 页面自身另有 `_handling` 兜底。
    detectionSpeed: DetectionSpeed.noDuplicates,
    formats: const <BarcodeFormat>[BarcodeFormat.qrCode],
  );

  bool _checkingPermission = true;
  bool _handling = false;
  String? _permissionError;
  String? _error;
  IotDevice? _boundDevice;

  /// 上次提示「不是本系统二维码」的时间，用于节流。
  DateTime _lastInvalidAt = DateTime.fromMillisecondsSinceEpoch(0);

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    unawaited(_init());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    unawaited(_controller.dispose());
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // 从系统设置返回后重新检查权限，用户开完权限即可直接扫码。
    if (state == AppLifecycleState.resumed && _permissionError != null) {
      unawaited(_init());
      return;
    }
    // 传入外部 controller 时，生命周期由页面负责（见 mobile_scanner 文档）。
    if (!_controller.value.hasCameraPermission) return;
    switch (state) {
      case AppLifecycleState.resumed:
        unawaited(_controller.start());
      case AppLifecycleState.inactive:
      case AppLifecycleState.paused:
      case AppLifecycleState.hidden:
      case AppLifecycleState.detached:
        unawaited(_controller.stop());
    }
  }

  /// 申请相机权限。
  Future<void> _init() async {
    final String? error = await BleProvisioner.ensureCameraPermission();
    if (!mounted) return;
    setState(() {
      _checkingPermission = false;
      _permissionError = error;
    });
  }

  /// 处理一次扫描结果。
  Future<void> _onDetect(BarcodeCapture capture) async {
    if (_handling || _boundDevice != null) return;
    String? raw;
    for (final Barcode barcode in capture.barcodes) {
      final String? value = barcode.rawValue;
      if (value != null && value.trim().isNotEmpty) {
        raw = value;
        break;
      }
    }
    if (raw == null) return;

    final IotQrPayload? payload = IotQrPayload.tryParse(raw);
    if (payload == null) {
      _notifyInvalid();
      return;
    }

    setState(() {
      _handling = true;
      _error = null;
    });
    await _controller.stop();

    try {
      final RIotDevice resp;
      if (payload is DeviceShareCode) {
        resp = await IotAppController.claim(payload.token);
      } else if (payload is DeviceQrCode) {
        resp = await IotAppController.bindByQr(
          productCode: payload.productCode,
          deviceCode: payload.deviceCode,
        );
      } else {
        throw const _ScanException('暂不支持该二维码类型');
      }

      if (!resp.isSuccess || resp.device == null) {
        throw _ScanException(resp.errorMessage);
      }
      if (!mounted) return;
      setState(() {
        _handling = false;
        _boundDevice = resp.device;
      });
    } catch (e) {
      if (!mounted) return;
      final String message =
          e is _ScanException ? e.message : '识别失败：$e';
      setState(() {
        _handling = false;
        _error = message;
      });
      context.showToast(message);
      // 失败后允许继续扫描：用户可能拿错了码，或需要重新对准。
      unawaited(_controller.start());
    }
  }

  /// 非本系统二维码提示（2 秒节流，避免取景框里抖动时连续弹窗）。
  void _notifyInvalid() {
    final DateTime now = DateTime.now();
    if (now.difference(_lastInvalidAt) < const Duration(seconds: 2)) return;
    _lastInvalidAt = now;
    if (!mounted) return;
    context.showToast('不是 AI Camera 设备二维码');
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('扫一扫添加')),
      backgroundColor: Colors.black,
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_checkingPermission) {
      return const LoadingIndicator(text: '正在检查相机权限…');
    }
    if (_permissionError != null) {
      return Container(
        color: AppColors.background,
        child: EmptyState(
          icon: Icons.no_photography_outlined,
          title: '无法使用相机',
          description: _permissionError,
          actionLabel: '打开系统设置',
          onAction: () {
            unawaited(BleProvisioner.openSettings());
          },
        ),
      );
    }
    if (_boundDevice != null) return _buildSuccess();

    return Stack(
      fit: StackFit.expand,
      children: <Widget>[
        MobileScanner(
          controller: _controller,
          onDetect: (BarcodeCapture capture) => unawaited(_onDetect(capture)),
          errorBuilder: (
            BuildContext context,
            MobileScannerException error,
          ) {
            return Center(
              child: Padding(
                padding: EdgeInsets.all(AppSpacing.lg),
                child: Text(
                  '相机启动失败：${error.errorDetails?.message ?? error.errorCode.name}',
                  style: AppText.body.copyWith(color: Colors.white70),
                  textAlign: TextAlign.center,
                ),
              ),
            );
          },
        ),
        _buildScanFrame(),
        Positioned(
          left: AppSpacing.pagePadding,
          right: AppSpacing.pagePadding,
          bottom: AppSpacing.xxxl,
          child: Column(
            children: <Widget>[
              if (_handling)
                Padding(
                  padding: EdgeInsets.only(bottom: AppSpacing.md),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: <Widget>[
                      SizedBox(
                        width: 16.dp,
                        height: 16.dp,
                        child: const CircularProgressIndicator(
                          strokeWidth: 2,
                          valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
                        ),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Text(
                        '正在添加设备…',
                        style: AppText.hint.copyWith(color: Colors.white),
                      ),
                    ],
                  ),
                ),
              if (_error != null)
                Padding(
                  padding: EdgeInsets.only(bottom: AppSpacing.md),
                  child: Text(
                    _error!,
                    style: AppText.caption.copyWith(color: AppColors.danger),
                    textAlign: TextAlign.center,
                  ),
                ),
              Text(
                '安防相机请扫机身二维码；家人分享请扫分享码\n'
                '分享码仅可使用一次，机身码在设备未绑定时可使用',
                style: AppText.hint.copyWith(color: Colors.white70),
                textAlign: TextAlign.center,
              ),
            ],
          ),
        ),
      ],
    );
  }

  /// 取景框：四角描边。
  Widget _buildScanFrame() {
    return Center(
      child: Container(
        width: 240.dp,
        height: 240.dp,
        decoration: BoxDecoration(
          border: Border.all(color: AppColors.primary, width: 2),
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        ),
      ),
    );
  }

  Widget _buildSuccess() {
    final IotDevice device = _boundDevice!;
    return Container(
      color: AppColors.background,
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      child: ListView(
        children: <Widget>[
          SizedBox(height: AppSpacing.xxxl),
          const Icon(
            Icons.check_circle_outline,
            size: 64,
            color: AppColors.success,
          ),
          SizedBox(height: AppSpacing.lg),
          Text('设备已添加', style: AppText.title, textAlign: TextAlign.center),
          SizedBox(height: AppSpacing.sm),
          Text(
            '${device.name}（${device.deviceCode}）',
            style: AppText.hint,
            textAlign: TextAlign.center,
          ),
          SizedBox(height: AppSpacing.xxxl),
          SizedBox(
            height: AppSpacing.buttonHeight,
            child: FilledButton(
              key: const Key('scan-success-devices'),
              onPressed: () => context.pushReplacement('/iot/devices'),
              child: const Text('查看我的设备'),
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          SizedBox(
            height: AppSpacing.buttonHeight,
            child: OutlinedButton(
              key: const Key('scan-success-continue'),
              onPressed: () {
                setState(() {
                  _boundDevice = null;
                  _error = null;
                });
                unawaited(_controller.start());
              },
              child: const Text('继续扫描'),
            ),
          ),
        ],
      ),
    );
  }
}

/// 扫码业务异常（只在本文件内部使用）。
class _ScanException implements Exception {
  const _ScanException(this.message);

  final String message;

  @override
  String toString() => message;
}
