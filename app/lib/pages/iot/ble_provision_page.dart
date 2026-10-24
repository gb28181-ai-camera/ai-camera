import 'dart:async';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../layout/Gaps.dart';
import '../../services/BleProvisioner.dart';
import '../../services/IotAppController.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/empty_state.dart';
import 'share_device_page.dart';

/// 页面阶段。
enum _BleStage {
  /// 扫描设备。
  scan,

  /// 已连接，等待输入 Wi-Fi。
  wifi,

  /// 正在配网（分步进度）。
  running,

  /// 配网与绑定成功。
  success,
}

/// 蓝牙配网页。
///
/// 流程：申请权限 → 扫描 → 连接 → hello 握手 → 输入 Wi-Fi → wifi 指令 →
/// bind 指令 → `POST /app/iot/devices/bind`。
///
/// 每一步都以设备的响应为准：设备不回包就停在当前步骤并提示重试，
/// 不做「乐观成功」（否则用户会以为已配好，实际设备仍离线）。
class BleProvisionPage extends StatefulWidget {
  /// 构造配网页。
  const BleProvisionPage({super.key, this.token = ''});

  /// 分享令牌（从扫码入口带入）；为空表示主用户现场配网。
  final String token;

  @override
  State<BleProvisionPage> createState() => _BleProvisionPageState();
}

class _BleProvisionPageState extends State<BleProvisionPage>
    with WidgetsBindingObserver {
  final BleProvisioner _provisioner = BleProvisioner();
  StreamSubscription<List<BleScanDevice>>? _scanSubscription;
  StreamSubscription<bool>? _scanStateSubscription;

  final TextEditingController _productCode = TextEditingController();
  final TextEditingController _nickname = TextEditingController();
  final TextEditingController _ssid = TextEditingController();
  final TextEditingController _password = TextEditingController();

  _BleStage _stage = _BleStage.scan;
  List<BleScanDevice> _devices = <BleScanDevice>[];
  BleScanDevice? _selected;
  BleProvisionSession? _session;
  String? _deviceCode;
  IotDevice? _boundDevice;

  bool _scanning = false;
  bool _busy = false;
  String? _permissionError;
  String? _error;

  /// 已完成的步骤下标（0 连接 / 1 握手 / 2 Wi-Fi / 3 绑定）。
  final Set<int> _completedSteps = <int>{};

  /// 当前步骤下标；等于 4 表示全部完成。
  int _activeStep = 0;

  static const List<String> _stepLabels = <String>[
    '连接设备',
    '设备握手',
    '配置 Wi-Fi',
    '绑定账号',
  ];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _scanSubscription = _provisioner.scanResults.listen(
      (List<BleScanDevice> devices) {
        if (!mounted) return;
        setState(() => _devices = devices);
      },
      onError: (Object error) {
        if (!mounted) return;
        setState(() {
          _scanning = false;
          _error = _messageOf(error);
        });
      },
    );
    // 扫描超时由插件内部结束，必须监听状态才能把「正在搜索」切回空态。
    _scanStateSubscription = _provisioner.scanning.listen((bool scanning) {
      if (!mounted || _scanning == scanning) return;
      setState(() => _scanning = scanning);
    });
    unawaited(_init());
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // 用户去系统设置开完权限返回时，自动重新检查并发起扫描。
    if (state == AppLifecycleState.resumed && _permissionError != null) {
      setState(() => _permissionError = null);
      unawaited(_init());
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    unawaited(_scanSubscription?.cancel());
    unawaited(_scanStateSubscription?.cancel());
    final BleProvisionSession? session = _session;
    _session = null;
    unawaited(session?.close());
    unawaited(_provisioner.dispose());
    _productCode.dispose();
    _nickname.dispose();
    _ssid.dispose();
    _password.dispose();
    super.dispose();
  }

  /// 申请权限后开始扫描。
  Future<void> _init() async {
    final String? permissionError = await BleProvisioner.ensurePermissions();
    if (!mounted) return;
    if (permissionError != null) {
      setState(() => _permissionError = permissionError);
      return;
    }
    await _startScan();
  }

  /// 开始（或重新开始）扫描。
  Future<void> _startScan() async {
    if (_busy) return;
    setState(() {
      _scanning = true;
      _error = null;
      _permissionError = null;
      _devices = <BleScanDevice>[];
      _stage = _BleStage.scan;
    });
    try {
      await _provisioner.startScan();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _scanning = false;
        _error = '蓝牙扫描失败：$e';
      });
    }
  }

  /// 连接设备并完成 hello 握手。
  Future<void> _connect(BleScanDevice device) async {
    if (_busy) return;
    setState(() {
      _busy = true;
      _error = null;
      _selected = device;
      _activeStep = 0;
      _completedSteps.clear();
    });
    await _provisioner.stopScan();
    if (mounted) setState(() => _scanning = false);

    BleProvisionSession? session;
    try {
      session = await _provisioner.connect(device.remoteId);
      _session = session;
      if (mounted) setState(() => _completedSteps.add(0));

      final String deviceCode = await session.hello(
        productCode: _productCode.text.trim(),
      );
      _deviceCode = deviceCode;
      if (!mounted) return;
      setState(() {
        _busy = false;
        _stage = _BleStage.wifi;
        _completedSteps.add(1);
        _activeStep = 2;
      });
    } catch (e) {
      await session?.close();
      _session = null;
      if (!mounted) return;
      setState(() {
        _busy = false;
        _stage = _BleStage.scan;
        _selected = null;
        _error = _messageOf(e);
      });
    }
  }

  /// 下发 Wi-Fi → 设备 bind → 服务端 bind。
  Future<void> _provision() async {
    final BleProvisionSession? session = _session;
    if (session == null || _busy) return;
    final String ssid = _ssid.text.trim();
    if (ssid.isEmpty) {
      context.showToast('请输入 Wi-Fi 名称');
      return;
    }

    setState(() {
      _busy = true;
      _error = null;
      _stage = _BleStage.running;
      _activeStep = 2;
    });

    try {
      await session.sendWifi(ssid: ssid, password: _password.text);
      if (mounted) setState(() => _completedSteps.add(2));

      if (mounted) setState(() => _activeStep = 3);
      final BleBindResult result = await session.bind(token: widget.token);
      if (mounted) setState(() => _completedSteps.add(3));

      // 设备侧完成绑定后，还要在平台侧建立「当前用户 ↔ 设备」关系。
      // 两步都成功才算配网完成：只做 BLE 绑定会让平台列表看不到设备。
      final RIotDevice resp = await IotAppController.bind(
        deviceCode: result.deviceCode,
        deviceSecret: result.deviceSecret,
        nickname: _nickname.text,
      );
      if (!resp.isSuccess || resp.device == null) {
        throw BleProvisionException(resp.errorMessage);
      }

      await session.close();
      _session = null;
      if (!mounted) return;
      setState(() {
        _busy = false;
        _activeStep = 4;
        _stage = _BleStage.success;
        _boundDevice = resp.device;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _stage = _BleStage.wifi;
        _error = _messageOf(e);
      });
    }
  }

  /// 放弃当前连接，回到扫描列表。
  Future<void> _reset() async {
    final BleProvisionSession? session = _session;
    _session = null;
    await session?.close();
    if (!mounted) return;
    setState(() {
      _selected = null;
      _deviceCode = null;
      _error = null;
      _completedSteps.clear();
      _activeStep = 0;
    });
    await _startScan();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('蓝牙配网')),
      backgroundColor: AppColors.background,
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_permissionError != null) {
      return EmptyState(
        icon: Icons.bluetooth_disabled,
        title: '无法使用蓝牙',
        description: _permissionError,
        actionLabel: '打开系统设置',
        onAction: () => unawaited(BleProvisioner.openSettings()),
      );
    }
    if (_stage == _BleStage.success) return _buildSuccess();
    if (_stage == _BleStage.wifi || _stage == _BleStage.running) {
      return _buildWifiForm();
    }
    return _buildScanList();
  }

  // ── 扫描 ────────────────────────────────────────────────────

  Widget _buildScanList() {
    return Column(
      children: <Widget>[
        Container(
          color: AppColors.card,
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            AppSpacing.md,
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                '请让设备进入配网模式（指示灯闪烁），并保持手机靠近设备',
                style: AppText.hint,
              ),
              SizedBox(height: AppSpacing.sm),
              TextField(
                key: const Key('ble-product-code'),
                controller: _productCode,
                decoration: InputDecoration(
                  labelText: '产品编码（选填）',
                  hintText: '设备固件用于校验产品型号',
                  isDense: true,
                  filled: true,
                  fillColor: AppColors.fill,
                  border: OutlineInputBorder(
                    borderRadius:
                        BorderRadius.circular(AppSpacing.radiusControl),
                    borderSide: BorderSide.none,
                  ),
                ),
              ),
              if (_error != null) ...<Widget>[
                SizedBox(height: AppSpacing.sm),
                Text(
                  _error!,
                  style: AppText.hint.copyWith(color: AppColors.danger),
                ),
              ],
            ],
          ),
        ),
        Expanded(
          child: _devices.isEmpty
              ? EmptyState(
                  icon: Icons.bluetooth_searching,
                  title: _scanning ? '正在搜索附近的设备…' : '没有找到设备',
                  description: _scanning
                      ? '通常需要 5–15 秒'
                      : '请确认设备已通电并进入配网模式，然后重新扫描',
                  actionLabel: _scanning ? null : '重新扫描',
                  onAction: _scanning ? null : _startScan,
                )
              : ListView.builder(
                  padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
                  itemCount: _devices.length,
                  itemBuilder: (BuildContext context, int index) =>
                      _buildDeviceTile(_devices[index]),
                ),
        ),
        if (_devices.isNotEmpty)
          SafeArea(
            top: false,
            child: Padding(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              child: OutlinedButton(
                key: const Key('ble-rescan'),
                onPressed: _busy ? null : _startScan,
                child: const Text('重新扫描'),
              ),
            ),
          ),
      ],
    );
  }

  Widget _buildDeviceTile(BleScanDevice device) {
    final bool selected = _selected?.remoteId == device.remoteId;
    return Padding(
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.xs,
      ),
      child: Material(
        key: Key('ble-device-${device.remoteId}'),
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: InkWell(
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
          onTap: _busy ? null : () => unawaited(_connect(device)),
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.md),
            child: Row(
              children: <Widget>[
                Icon(
                  Icons.bluetooth,
                  size: 20.dp,
                  color: selected ? AppColors.primary : AppColors.textTertiary,
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text(
                        device.displayName,
                        style: AppText.bodyStrong,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      SizedBox(height: 2.dp),
                      Text(
                        '信号 ${device.rssi} dBm · ${device.remoteId}',
                        style: AppText.hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                if (_busy && selected)
                  SizedBox(
                    width: 18.dp,
                    height: 18.dp,
                    child: const CircularProgressIndicator(strokeWidth: 2),
                  )
                else
                  Text('连接', style: AppText.caption.copyWith(
                    color: AppColors.primary,
                  )),
              ],
            ),
          ),
        ),
      ),
    );
  }

  // ── Wi-Fi 表单 / 进度 ───────────────────────────────────────

  Widget _buildWifiForm() {
    final bool running = _stage == _BleStage.running;
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: <Widget>[
        _buildProgressCard(),
        SizedBox(height: AppSpacing.md),
        Container(
          padding: EdgeInsets.all(AppSpacing.lg),
          decoration: BoxDecoration(
            color: AppColors.card,
            borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Text(
                '设备 ${_selected?.displayName ?? ''}',
                style: AppText.bodyStrong,
              ),
              SizedBox(height: 2.dp),
              Text(
                '设备编号 ${_deviceCode.orPlaceholder()}',
                style: AppText.hint,
              ),
              SizedBox(height: AppSpacing.lg),
              TextField(
                key: const Key('ble-ssid'),
                controller: _ssid,
                enabled: !running,
                decoration: InputDecoration(
                  labelText: 'Wi-Fi 名称（SSID）',
                  hintText: '仅支持 2.4GHz 网络',
                  filled: true,
                  fillColor: AppColors.fill,
                  border: OutlineInputBorder(
                    borderRadius:
                        BorderRadius.circular(AppSpacing.radiusControl),
                    borderSide: BorderSide.none,
                  ),
                ),
              ),
              SizedBox(height: AppSpacing.md),
              TextField(
                key: const Key('ble-password'),
                controller: _password,
                enabled: !running,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: 'Wi-Fi 密码',
                  filled: true,
                  fillColor: AppColors.fill,
                  border: OutlineInputBorder(
                    borderRadius:
                        BorderRadius.circular(AppSpacing.radiusControl),
                    borderSide: BorderSide.none,
                  ),
                ),
              ),
              SizedBox(height: AppSpacing.md),
              TextField(
                key: const Key('ble-nickname'),
                controller: _nickname,
                enabled: !running,
                decoration: InputDecoration(
                  labelText: '设备备注名（选填）',
                  filled: true,
                  fillColor: AppColors.fill,
                  border: OutlineInputBorder(
                    borderRadius:
                        BorderRadius.circular(AppSpacing.radiusControl),
                    borderSide: BorderSide.none,
                  ),
                ),
              ),
              if (_error != null) ...<Widget>[
                SizedBox(height: AppSpacing.md),
                Text(
                  _error!,
                  style: AppText.hint.copyWith(color: AppColors.danger),
                ),
              ],
              SizedBox(height: AppSpacing.lg),
              SizedBox(
                width: double.infinity,
                height: AppSpacing.buttonHeight,
                child: FilledButton(
                  key: const Key('ble-start-provision'),
                  onPressed: running ? null : () => unawaited(_provision()),
                  child: running
                      ? Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: <Widget>[
                            SizedBox(
                              width: 16.dp,
                              height: 16.dp,
                              child: const CircularProgressIndicator(
                                strokeWidth: 2,
                                valueColor:
                                    AlwaysStoppedAnimation<Color>(Colors.white),
                              ),
                            ),
                            SizedBox(width: AppSpacing.sm),
                            const Text('配网中…'),
                          ],
                        )
                      : const Text('开始配网'),
                ),
              ),
              SizedBox(height: AppSpacing.sm),
              Center(
                child: TextButton(
                  key: const Key('ble-reset'),
                  onPressed: running ? null : () => unawaited(_reset()),
                  child: const Text('重新选择设备'),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  /// 四步进度卡片。
  Widget _buildProgressCard() {
    return Container(
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text('配网进度', style: AppText.section),
          SizedBox(height: AppSpacing.md),
          for (int i = 0; i < _stepLabels.length; i++) ...<Widget>[
            Row(
              children: <Widget>[
                _StepIcon(
                  done: _completedSteps.contains(i),
                  active: _activeStep == i && _busy,
                ),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: Text(
                    _stepLabels[i],
                    style: _completedSteps.contains(i)
                        ? AppText.body.copyWith(color: AppColors.success)
                        : AppText.body,
                  ),
                ),
              ],
            ),
            if (i < _stepLabels.length - 1)
              Padding(
                padding: EdgeInsets.only(left: 9.dp),
                child: Container(
                  width: AppSpacing.dividerThickness,
                  height: AppSpacing.md,
                  color: AppColors.divider,
                ),
              ),
          ],
        ],
      ),
    );
  }

  // ── 成功 ────────────────────────────────────────────────────

  Widget _buildSuccess() {
    final IotDevice? device = _boundDevice;
    // 只有「适老化相机 + 主账号」才引导分享：
    // - 安防相机不提供家庭分享（产品定位是个人/场所设备）；
    // - 分享账号不能再分享（否则邀请链无法由 owner 收回）。
    final bool canShare = device?.canShare ?? false;
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: <Widget>[
        SizedBox(height: AppSpacing.xxxl),
        Icon(
          Icons.check_circle_outline,
          size: 64.dp,
          color: AppColors.success,
        ),
        SizedBox(height: AppSpacing.lg),
        Text(
          '设备已添加',
          style: AppText.title,
          textAlign: TextAlign.center,
        ),
        SizedBox(height: AppSpacing.sm),
        Text(
          '${device?.name ?? ''}（${device?.deviceCode ?? _deviceCode ?? ''}）',
          style: AppText.hint,
          textAlign: TextAlign.center,
        ),
        if (canShare) ...<Widget>[
          SizedBox(height: AppSpacing.md),
          Text(
            '把设备分享给家人，家人用 App 扫码即可加入家庭',
            style: AppText.hint,
            textAlign: TextAlign.center,
          ),
          SizedBox(height: AppSpacing.xxxl),
          SizedBox(
            height: AppSpacing.buttonHeight,
            child: FilledButton.icon(
              key: const Key('ble-success-share'),
              onPressed: () => unawaited(showDeviceShareSheet(
                context,
                deviceId: device!.id,
                deviceName: device.name,
              )),
              icon: const Icon(Icons.ios_share, size: 18),
              label: const Text('生成分享二维码'),
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          SizedBox(
            height: AppSpacing.buttonHeight,
            child: OutlinedButton(
              key: const Key('ble-success-devices'),
              onPressed: () => context.pushReplacement('/iot/devices'),
              child: const Text('查看我的设备'),
            ),
          ),
          TextButton(
            key: const Key('ble-success-later'),
            onPressed: () => context.pushReplacement('/iot/devices'),
            child: const Text('稍后再说'),
          ),
        ] else ...<Widget>[
          SizedBox(height: AppSpacing.xxxl),
          SizedBox(
            height: AppSpacing.buttonHeight,
            child: FilledButton(
              key: const Key('ble-success-devices'),
              onPressed: () => context.pushReplacement('/iot/devices'),
              child: const Text('查看我的设备'),
            ),
          ),
          SizedBox(height: AppSpacing.sm),
          SizedBox(
            height: AppSpacing.buttonHeight,
            child: OutlinedButton(
              key: const Key('ble-success-done'),
              onPressed: () => context.pop(),
              child: const Text('完成'),
            ),
          ),
        ],
      ],
    );
  }
}

/// 步骤图标：完成打勾、进行中转圈、未开始灰点。
class _StepIcon extends StatelessWidget {
  const _StepIcon({required this.done, required this.active});

  final bool done;
  final bool active;

  @override
  Widget build(BuildContext context) {
    if (done) {
      return Icon(Icons.check_circle, size: 18.dp, color: AppColors.success);
    }
    if (active) {
      return SizedBox(
        width: 18.dp,
        height: 18.dp,
        child: const CircularProgressIndicator(strokeWidth: 2),
      );
    }
    return Icon(
      Icons.radio_button_unchecked,
      size: 18.dp,
      color: AppColors.textDisabled,
    );
  }
}

/// 异常 → 可展示文案。
String _messageOf(Object error) {
  if (error is BleProvisionException) return error.message;
  return error.toString();
}
