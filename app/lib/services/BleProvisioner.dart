/// 蓝牙配网（BLE Provisioning）。
///
/// ## 协议（与设备固件约定）
///
/// 使用 Nordic UART Service（NUS）：
/// - Service `6e400001-b5a3-f393-e0a9-e50e24dcca9e`
/// - RX（App 写入）`6e400002-b5a3-f393-e0a9-e50e24dcca9e`
/// - TX（设备通知）`6e400003-b5a3-f393-e0a9-e50e24dcca9e`
///
/// 帧格式：UTF-8 JSON + `\n`（一行一帧）。一问一答：
/// 1. App → 设备 `{"cmd":"hello","product_code":"..."}`
/// 2. 设备 → App `{"status":"ok","device_code":"..."}`
/// 3. App → 设备 `{"cmd":"wifi","ssid":"...","password":"..."}`
/// 4. 设备 → App `{"status":"ok"}` 或 `{"status":"error","message":"..."}`
/// 5. App → 设备 `{"cmd":"bind","token":"<share_token 或空>"}`
/// 6. 设备 → App `{"status":"ok","device_code":"...","device_secret":"..."}`
///
/// 为什么最后一步由设备返回 `device_secret` 而不是 App 生成：
/// 服务端据此确认「App 与设备在同一物理现场」，避免仅凭设备编号就能把别人的
/// 摄像头绑定到自己账号下。BLE 链路的短距离特性就是这一步的信任来源。
///
/// ## 分层的理由
///
/// [BleProvisionSession] 只依赖 [BleLink]（写入 + 通知流），不直接依赖插件。
/// 这样帧编解码、超时与错误分支都能在 `flutter test` 中用假链路覆盖。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter_blue_plus/flutter_blue_plus.dart';
import 'package:permission_handler/permission_handler.dart';

import 'IotAppController.dart';

// ════════════════════════════════════════════════════════════════
// 协议常量
// ════════════════════════════════════════════════════════════════

/// Nordic UART Service 的 Service UUID。
const String bleNusServiceUuid = '6e400001-b5a3-f393-e0a9-e50e24dcca9e';

/// Nordic UART Service 的 RX（App 写入）UUID。
const String bleNusRxUuid = '6e400002-b5a3-f393-e0a9-e50e24dcca9e';

/// Nordic UART Service 的 TX（设备通知）UUID。
const String bleNusTxUuid = '6e400003-b5a3-f393-e0a9-e50e24dcca9e';

/// 一帧的最大缓存长度。
///
/// 设备异常时可能只发半帧且不发 `\n`，无上限会让缓冲区一直增长；
/// 正常帧（SDP 除外）都远小于 8KB。
const int _kMaxFrameBufferLength = 8192;

// ════════════════════════════════════════════════════════════════
// 帧编解码
// ════════════════════════════════════════════════════════════════

/// BLE 帧编解码。
class BleFrames {
  BleFrames._();

  /// 把 JSON 对象编码为一帧（UTF-8 + `\n`）。
  static List<int> encode(Map<String, dynamic> frame) {
    return utf8.encode('${jsonEncode(frame)}\n');
  }

  /// 解析单行；非 JSON 对象返回 null。
  static Map<String, dynamic>? decodeLine(String line) {
    final String text = line.trim();
    if (text.isEmpty) return null;
    try {
      final dynamic decoded = jsonDecode(text);
      if (decoded is Map) {
        return decoded.map(
          (dynamic key, dynamic value) =>
              MapEntry<String, dynamic>(key.toString(), value),
        );
      }
    } catch (_) {
      // 固件日志 / 调试文本：忽略。
    }
    return null;
  }
}

/// 流式帧解码器：处理 BLE 通知可能被拆包/粘包的情况。
///
/// 缓存的是**字节**而不是字符串：一个 UTF-8 汉字（3 字节）可能被拆到两次
/// BLE 通知里，若逐段 `utf8.decode` 会解出乱码。按 `0x0A` 切出完整行后再解码，
/// 才能保证中文 Wi-Fi 名称等字段原样还原。
class BleFrameDecoder {
  final List<int> _buffer = <int>[];

  /// 追加一段通知数据，返回本次能完整解析出的帧。
  List<Map<String, dynamic>> add(List<int> chunk) {
    if (chunk.isEmpty) return const <Map<String, dynamic>>[];
    _buffer.addAll(chunk);

    final List<Map<String, dynamic>> frames = <Map<String, dynamic>>[];
    int lineStart = 0;
    for (int i = 0; i < _buffer.length; i++) {
      if (_buffer[i] != 0x0A) continue; // '\n'
      final String line =
          utf8.decode(_buffer.sublist(lineStart, i), allowMalformed: true);
      lineStart = i + 1;
      final Map<String, dynamic>? frame = BleFrames.decodeLine(line);
      if (frame != null) frames.add(frame);
    }
    if (lineStart > 0) {
      _buffer.removeRange(0, lineStart);
    }

    // 防御：设备只发半帧时丢弃缓存，避免无限增长后卡死。
    if (_buffer.length > _kMaxFrameBufferLength) {
      _buffer.clear();
    }
    return frames;
  }

  /// 清空未完成的半帧缓存（重连时调用）。
  void reset() => _buffer.clear();
}

// ════════════════════════════════════════════════════════════════
// 扫描结果
// ════════════════════════════════════════════════════════════════

/// 扫描到的蓝牙设备。
class BleScanDevice {
  /// 构造扫描结果。
  const BleScanDevice({
    required this.remoteId,
    required this.name,
    required this.rssi,
  });

  /// 平台设备标识（连接时使用）。
  final String remoteId;

  /// 广播名称。
  final String name;

  /// 信号强度（dBm，越接近 0 越强）。
  final int rssi;

  /// 展示名；无名设备显示占位。
  String get displayName => name.isEmpty ? '未命名设备' : name;
}

// ════════════════════════════════════════════════════════════════
// 传输层
// ════════════════════════════════════════════════════════════════

/// 一条已建立的 BLE 连接（只暴露配网需要的能力）。
abstract class BleLink {
  /// TX 特征的通知流。
  Stream<List<int>> get notifications;

  /// 向 RX 特征写入一帧。
  Future<void> write(List<int> data);

  /// 断开连接。
  Future<void> disconnect();
}

/// 建立连接的工厂；测试可替换为假实现。
typedef BleLinkFactory = Future<BleLink> Function(String remoteId);

/// 基于 `flutter_blue_plus` 的真实连接。
class FlutterBluePlusLink implements BleLink {
  /// 构造连接。
  FlutterBluePlusLink({
    required this.device,
    required this.rx,
    required this.tx,
  });

  /// 蓝牙设备。
  final BluetoothDevice device;

  /// RX 特征（App 写入）。
  final BluetoothCharacteristic rx;

  /// TX 特征（设备通知）。
  final BluetoothCharacteristic tx;

  @override
  Stream<List<int>> get notifications => tx.onValueReceived;

  @override
  Future<void> write(List<int> data) {
    // 配网指令很短，用「带响应写」确保设备确实收到，避免丢帧后再等超时。
    return rx.write(data, withoutResponse: false);
  }

  @override
  Future<void> disconnect() async {
    try {
      await device.disconnect();
    } catch (_) {
      // 已断开。
    }
  }
}

// ════════════════════════════════════════════════════════════════
// 配网会话
// ════════════════════════════════════════════════════════════════

/// 一次配网会话（hello → wifi → bind）。
///
/// 一台设备对应一个实例；页面退出 / 失败必须 [close]。
class BleProvisionSession {
  /// 构造会话。
  BleProvisionSession(
    this._link, {
    this.responseTimeout = const Duration(seconds: 15),
  });

  final BleLink _link;

  /// 单步等待设备响应的超时。
  final Duration responseTimeout;

  final BleFrameDecoder _decoder = BleFrameDecoder();

  StreamSubscription<List<int>>? _subscription;
  Completer<Map<String, dynamic>>? _pending;
  bool _closed = false;

  /// 开始监听设备通知（连接成功后立即调用）。
  Future<void> start() async {
    _subscription = _link.notifications.listen((List<int> chunk) {
      final List<Map<String, dynamic>> frames = _decoder.add(chunk);
      for (final Map<String, dynamic> frame in frames) {
        final Completer<Map<String, dynamic>>? pending = _pending;
        if (pending != null && !pending.isCompleted) {
          _pending = null;
          pending.complete(frame);
        }
        // 没有等待中的请求时（设备主动上报）直接忽略。
      }
    });
  }

  /// 握手：发送 hello，返回设备编号。
  Future<String> hello({required String productCode}) async {
    final Map<String, dynamic> response = await _request(<String, dynamic>{
      'cmd': 'hello',
      'product_code': productCode,
    });
    _ensureOk(response, '设备握手');
    final String deviceCode = _stringOf(response['device_code']);
    if (deviceCode.isEmpty) {
      throw BleProvisionException('设备握手响应缺少 device_code');
    }
    return deviceCode;
  }

  /// 下发 Wi-Fi 凭据。
  Future<void> sendWifi({
    required String ssid,
    required String password,
  }) async {
    if (ssid.trim().isEmpty) {
      throw BleProvisionException('Wi-Fi 名称不能为空');
    }
    final Map<String, dynamic> response = await _request(<String, dynamic>{
      'cmd': 'wifi',
      'ssid': ssid,
      'password': password,
    });
    _ensureOk(response, 'Wi-Fi 配置');
  }

  /// 请求设备完成绑定，返回设备生成的编号与密钥。
  ///
  /// [token] 为分享令牌，可为空（设备主用户现场配网时不需要）。
  Future<BleBindResult> bind({String token = ''}) async {
    final Map<String, dynamic> response = await _request(<String, dynamic>{
      'cmd': 'bind',
      'token': token,
    });
    _ensureOk(response, '设备绑定');
    final String deviceCode = _stringOf(response['device_code']);
    final String deviceSecret = _stringOf(response['device_secret']);
    if (deviceCode.isEmpty || deviceSecret.isEmpty) {
      throw BleProvisionException('设备绑定响应缺少 device_code 或 device_secret');
    }
    return BleBindResult(deviceCode: deviceCode, deviceSecret: deviceSecret);
  }

  /// 发送一帧并等待下一帧响应。
  Future<Map<String, dynamic>> _request(Map<String, dynamic> frame) async {
    if (_closed) {
      throw BleProvisionException('蓝牙连接已关闭');
    }
    final Completer<Map<String, dynamic>> completer =
        Completer<Map<String, dynamic>>();
    _pending = completer;
    // 超时后设备仍可能回包；先吞掉结果，避免未处理异常。
    unawaited(completer.future.then<void>((_) {}, onError: (Object _) {}));

    try {
      await _link.write(BleFrames.encode(frame));
      return await completer.future.timeout(
        responseTimeout,
        onTimeout: () => throw BleProvisionException(
          '设备在 ${responseTimeout.inSeconds} 秒内没有响应，请靠近设备后重试',
        ),
      );
    } finally {
      if (identical(_pending, completer)) _pending = null;
    }
  }

  /// 校验 `status` 字段；error 时抛出设备给出的原因。
  void _ensureOk(Map<String, dynamic> response, String action) {
    final String status = _stringOf(response['status']);
    if (status == 'ok') return;
    final String message = _stringOf(response['message']);
    throw BleProvisionException(
      message.isEmpty ? '$action失败（设备未说明原因）' : '$action失败：$message',
    );
  }

  /// 关闭会话并断开蓝牙。
  Future<void> close() async {
    if (_closed) return;
    _closed = true;

    final StreamSubscription<List<int>>? subscription = _subscription;
    _subscription = null;
    await subscription?.cancel();

    final Completer<Map<String, dynamic>>? pending = _pending;
    _pending = null;
    if (pending != null && !pending.isCompleted) {
      pending.completeError(BleProvisionException('蓝牙连接已关闭'));
    }
    _decoder.reset();
    await _link.disconnect();
  }
}

// ════════════════════════════════════════════════════════════════
// 配网入口
// ════════════════════════════════════════════════════════════════

/// 蓝牙配网门面：权限、扫描、连接。
class BleProvisioner {
  /// 构造门面。
  BleProvisioner({
    BleLinkFactory? linkFactory,
    this.responseTimeout = const Duration(seconds: 15),
  }) : _linkFactory = linkFactory;

  /// 单步响应超时。
  final Duration responseTimeout;

  final BleLinkFactory? _linkFactory;

  final StreamController<List<BleScanDevice>> _scanController =
      StreamController<List<BleScanDevice>>.broadcast();

  final Map<String, BleScanDevice> _found = <String, BleScanDevice>{};

  StreamSubscription<List<ScanResult>>? _scanSubscription;

  /// 扫描结果流（按 RSSI 从强到弱排序，已按设备去重）。
  Stream<List<BleScanDevice>> get scanResults => _scanController.stream;

  /// 扫描状态流。
  ///
  /// 页面用它把「正在搜索」切换到「没有找到设备」：`startScan(timeout:)`
  /// 只是启动扫描，超时结束不会有回调通知页面。
  Stream<bool> get scanning => FlutterBluePlus.isScanning;

  /// 申请蓝牙权限。
  ///
  /// Android 12+ 需要 `BLUETOOTH_SCAN` / `BLUETOOTH_CONNECT`；
  /// Android 11 及以下扫描需要定位权限（系统限制，无法绕过）。
  /// 返回 null 表示已授权；否则返回可直接展示的中文错误。
  static Future<String?> ensurePermissions() async {
    if (kIsWeb) return '当前平台不支持蓝牙配网';
    // iOS 的蓝牙权限没有对应的 permission_handler 权限项，扫描时由系统弹窗。
    if (!Platform.isAndroid) return null;

    final PermissionStatus scan = await Permission.bluetoothScan.request();
    final PermissionStatus connect =
        await Permission.bluetoothConnect.request();
    if (scan.isGranted && connect.isGranted) return null;

    final PermissionStatus location =
        await Permission.locationWhenInUse.request();
    if (location.isGranted) return null;

    if (scan.isPermanentlyDenied ||
        connect.isPermanentlyDenied ||
        location.isPermanentlyDenied) {
      return '蓝牙权限已被永久拒绝，请到系统设置中开启「附近设备」权限后重试';
    }
    return '需要蓝牙权限才能搜索设备，请允许后重试';
  }

  /// 申请相机权限（扫码用）。
  ///
  /// 返回 null 表示已授权；否则返回中文错误。永久拒绝时提示去设置。
  static Future<String?> ensureCameraPermission() async {
    if (kIsWeb) return null;
    final PermissionStatus status = await Permission.camera.request();
    if (status.isGranted || status.isLimited) return null;
    if (status.isPermanentlyDenied) {
      return '相机权限已被永久拒绝，请到系统设置中开启相机权限后重试';
    }
    return '需要相机权限才能扫描二维码，请允许后重试';
  }

  /// 打开系统设置页（权限被永久拒绝时使用）。
  static Future<bool> openSettings() => openAppSettings();

  /// 开始扫描。
  Future<void> startScan({Duration timeout = const Duration(seconds: 15)}) async {
    await stopScan();
    _found.clear();
    _scanSubscription = FlutterBluePlus.onScanResults.listen(
      (List<ScanResult> results) {
        for (final ScanResult result in results) {
          final String name = result.advertisementData.advName.isNotEmpty
              ? result.advertisementData.advName
              : result.device.platformName;
          final bool advertisesNus = result.advertisementData.serviceUuids
              .any((Guid uuid) => _uuidEquals(uuid, bleNusServiceUuid));
          // 同名空设备过多会淹没列表：只保留有名字或广播了 NUS 的设备。
          if (name.isEmpty && !advertisesNus) continue;
          _found[result.device.remoteId.str] = BleScanDevice(
            remoteId: result.device.remoteId.str,
            name: name,
            rssi: result.rssi,
          );
        }
        final List<BleScanDevice> list = _found.values.toList(growable: false)
          ..sort((BleScanDevice a, BleScanDevice b) => b.rssi.compareTo(a.rssi));
        if (!_scanController.isClosed) _scanController.add(list);
      },
      onError: (Object error) {
        if (!_scanController.isClosed) {
          _scanController.addError(BleProvisionException('蓝牙扫描失败：$error'));
        }
      },
    );
    await FlutterBluePlus.startScan(timeout: timeout);
  }

  /// 停止扫描。
  Future<void> stopScan() async {
    final StreamSubscription<List<ScanResult>>? subscription = _scanSubscription;
    _scanSubscription = null;
    await subscription?.cancel();
    if (FlutterBluePlus.isScanningNow) {
      await FlutterBluePlus.stopScan();
    }
  }

  /// 连接设备并返回配网会话（内部已完成服务发现与通知订阅）。
  Future<BleProvisionSession> connect(String remoteId) async {
    final BleLinkFactory factory = _linkFactory ?? _connectReal;
    final BleLink link = await factory(remoteId);
    final BleProvisionSession session = BleProvisionSession(
      link,
      responseTimeout: responseTimeout,
    );
    try {
      await session.start();
      return session;
    } catch (e) {
      await session.close();
      throw BleProvisionException('蓝牙连接初始化失败：$e');
    }
  }

  /// 真实连接：连接 → 发现服务 → 找 NUS 特征 → 订阅 TX。
  Future<BleLink> _connectReal(String remoteId) async {
    try {
      final BluetoothDevice device = BluetoothDevice.fromId(remoteId);
      await device.connect(timeout: const Duration(seconds: 15));

      final List<BluetoothService> services = await device.discoverServices();
      final BluetoothService service = services.firstWhere(
        (BluetoothService item) => _uuidEquals(item.uuid, bleNusServiceUuid),
        orElse: () => throw BleProvisionException('设备未提供 Nordic UART 服务，固件可能不匹配'),
      );
      final BluetoothCharacteristic rx = service.characteristics.firstWhere(
        (BluetoothCharacteristic item) => _uuidEquals(item.uuid, bleNusRxUuid),
        orElse: () => throw BleProvisionException('设备缺少 RX 特征（6e400002）'),
      );
      final BluetoothCharacteristic tx = service.characteristics.firstWhere(
        (BluetoothCharacteristic item) => _uuidEquals(item.uuid, bleNusTxUuid),
        orElse: () => throw BleProvisionException('设备缺少 TX 特征（6e400003）'),
      );
      await tx.setNotifyValue(true);
      return FlutterBluePlusLink(device: device, rx: rx, tx: tx);
    } on BleProvisionException {
      rethrow;
    } catch (e) {
      throw BleProvisionException('蓝牙连接失败：$e');
    }
  }

  /// 释放扫描资源。
  Future<void> dispose() async {
    await stopScan();
    if (!_scanController.isClosed) {
      await _scanController.close();
    }
  }
}

/// 蓝牙配网异常（文案可直接展示给用户）。
class BleProvisionException implements Exception {
  /// 构造异常。
  BleProvisionException(this.message);

  /// 中文错误信息。
  final String message;

  @override
  String toString() => message;
}

/// 把动态值转成字符串（null → 空串）。
String _stringOf(dynamic value) => value?.toString() ?? '';

/// 比较 UUID。
///
/// `Guid.toString()` 对 16 位短 UUID 只返回 4 位十六进制，而常量是完整的
/// 128 位写法；统一去掉 `-` 并转小写后再比，避免蓝牙设备上报短 UUID 时匹配失败。
bool _uuidEquals(Guid uuid, String expected) {
  final String actual = uuid.str128.toLowerCase().replaceAll('-', '');
  final String target = expected.toLowerCase().replaceAll('-', '');
  return actual == target;
}
