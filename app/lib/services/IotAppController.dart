/// App 端 IOT 设备接入接口（添加设备 / 已绑定设备 / 服务组 / ICE）。
///
/// 约定与 `GbController.dart` 一致：
/// - `static const` 路径常量 + `static Future<T>` 方法，统一经 [ApiService] 发请求；
/// - 页面中禁止手写 URL，也不得绕过类型直接读 `Map`；
/// - 请求不抛异常，失败时返回 `code != 200` 的响应，调用方读 `errorMessage`。
///
/// 为什么类型定义放在本文件而不是 `generated/definitions`：
/// App 端没有 Dart 生成器（后端只产出 Web 端 Swagger），上述 `/app/iot/*` 与
/// `/app/rtc/*` 是手写契约。字段名必须与后端 JSON tag 严格一致（snake_case），
/// 后端改动时同步修改此处，禁止在页面里做 `as` 强转。
library;

import 'dart:convert';

import 'package:dio/dio.dart' as dio;

import 'api_service.dart';
import 'generated/definitions/Definition.dart';
import 'generated/definitions/Response.dart';

// ════════════════════════════════════════════════════════════════
// 数据模型
// ════════════════════════════════════════════════════════════════

/// 产品类型（决定「添加设备」的入口与是否支持家庭分享）。
///
/// 当前只有两类产品，差异不止文案：
/// - `security`（安防相机）：出厂即联网，机身印有设备二维码，**无需配网**；
///   绑定只做「扫码认领」，因此入口直达扫码页；
/// - `elderly`（适老化智能相机）：需要先由家属通过蓝牙把 Wi-Fi 凭据写入设备，
///   设备联网后才能完成平台绑定；绑定后 owner 可以生成分享码邀请家人。
///
/// `unknown` 用于兼容尚未下发 `device_type` 的后端版本：此时按「安防相机」
/// 处理（直达扫码页），因为扫码页同时支持机身码与分享码，是更通用的兜底路径。
enum IotDeviceType {
  /// 安防相机。
  security('security', '安防'),

  /// 适老化智能相机。
  elderly('elderly', '适老化'),

  /// 未知类型（后端未下发）。
  unknown('', '未知');

  const IotDeviceType(this.wireValue, this.label);

  /// 后端字段值。
  final String wireValue;

  /// 中文短标签（列表 Tag 用）。
  final String label;

  /// 解析后端字段；空值或未知值返回 [unknown]。
  static IotDeviceType tryParse(String raw) {
    final String value = raw.trim().toLowerCase();
    for (final IotDeviceType type in IotDeviceType.values) {
      if (type.wireValue.isNotEmpty && type.wireValue == value) return type;
    }
    return IotDeviceType.unknown;
  }

  /// 「添加设备」页中该类型对应的入口路由。
  ///
  /// 安防相机 → 扫码绑定；适老化相机 → 蓝牙配网（配网成功后才会绑定）。
  String get addRoute {
    switch (this) {
      case IotDeviceType.security:
      case IotDeviceType.unknown:
        return '/iot/scan';
      case IotDeviceType.elderly:
        return '/iot/ble';
    }
  }
}

/// 已绑定设备（`ac_device` 的 App 端裁剪版）。
///
/// 字段以「用户视角」为主：产品名、在线状态、角色。`device_secret` 属于
/// 设备侧凭据，App 端接口不会返回，模型里也不保留。
class IotDevice {
  /// 构造设备。
  const IotDevice({
    required this.id,
    required this.name,
    required this.deviceCode,
    required this.nickname,
    required this.productId,
    required this.productName,
    required this.productCode,
    required this.allowShare,
    required this.role,
    required this.online,
    required this.status,
    required this.regionName,
    required this.activatedAt,
  });

  /// 从 JSON 构造。
  ///
  /// 后端不同端点给的是不同包装：
  /// - 列表：`{id, name, device_code, status, product:{...}, role, nickname}`；
  /// - 认领/绑定：`{device:{...}, account:{...}, role, created}`；
  /// - 详情：`{device:{...}, product:{...}, account:{...}, service_groups:[]}`。
  ///
  /// 统一在这里剥掉外层 `device` 包装，避免每个页面各写一套解析。
  factory IotDevice.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> nested = defMap(json['device']);
    final Map<String, dynamic> source = nested.isNotEmpty ? nested : json;
    final Map<String, dynamic> account = defMap(json['account']);
    // 列表项把产品放在条目自身的 `product` 里；详情响应则把产品放在顶层
    // `product`（`device` 内没有）。两处都取，避免 product / device_type 丢失。
    final Map<String, dynamic> product = defMap(source['product']).isNotEmpty
        ? defMap(source['product'])
        : defMap(json['product']);
    final String status = defStr(source['status']);
    final String nickname = _firstNonEmpty(<String>[
      defStr(source['nickname']),
      defStr(account['nickname']),
    ]);
    final String name = _firstNonEmpty(<String>[
      defStr(source['name']),
      nickname,
    ]);
    return IotDevice(
      id: defInt(source['id']),
      name: name,
      deviceCode: defStr(source['device_code']),
      nickname: nickname,
      productId: defIntOrNull(source['product_id'] ?? product['id']),
      productName: _firstNonEmpty(<String>[
        defStr(source['product_name']),
        defStr(product['name']),
        defStr(defMap(json['product'])['name']),
      ]),
      productCode: _firstNonEmpty(<String>[
        defStr(source['product_code']),
        defStr(product['code']),
        defStr(defMap(json['product'])['code']),
      ]),
      // 是否允许分享由产品配置决定（product.allow_share）；
      // 缺省 false，页面据此隐藏「分享」入口。
      allowShare: defBool(product['allow_share'] ?? source['allow_share'] ?? json['allow_share']),
      role: _firstNonEmpty(<String>[
        defStr(source['role']),
        defStr(json['role']),
        defStr(account['role']),
      ]),
      online: status.toLowerCase() == 'online',
      status: status,
      regionName: _firstNonEmpty(<String>[
        defStr(source['region_name']),
        defStr(defMap(source['region'])['name']),
      ]),
      activatedAt: _firstNonEmpty(<String>[
        defStr(source['activated_at']),
        defStr(account['activated_at']),
      ]),
    );
  }

  /// 主键（`ac_device.id`，播放/服务组接口都用它）。
  final int id;

  /// 设备名称。
  final String name;

  /// 平台内设备编号。
  final String deviceCode;

  /// 用户备注名（绑定时可填）。
  final String nickname;

  /// 所属产品 ID。
  final int? productId;

  /// 所属产品名称。
  final String productName;

  /// 产品编码。
  final String productCode;

  /// 产品是否允许分享（`product.allow_share`）。
  final bool allowShare;

  /// 当前用户在该设备上的角色：owner / shared。
  final String role;

  /// 是否在线（后端未给 `online` 时按 status 推导）。
  final bool online;

  /// 原始运行状态 online / offline。
  final String status;

  /// 所属区域名称。
  final String regionName;

  /// 激活时间。
  final String activatedAt;

  /// 角色中文名。
  String get roleLabel {
    switch (role) {
      case 'owner':
        return '主账号';
      case 'shared':
        return '分享账号';
      default:
        return role.isEmpty ? '--' : role;
    }
  }

  /// 是否为主账号（设备的实际持有人）。
  bool get isOwner => role == 'owner';

  /// 是否为分享账号（通过分享码加入的家人）。
  bool get isShared => role == 'shared';

  /// 是否可以生成分享码。
  ///
  /// **只有 owner 能分享**，原因有两层：
  /// 1. 服务端 `POST /app/iot/devices/:id/share-token` 以 owner 为边界，
  ///    分享账号调用会返回 403 —— 前端必须提前隐藏按钮，否则用户会撞到报错；
  /// 2. 分享意味着「代表设备主人邀请他人进入家庭」，若 shared 也能再分享，
  ///    owner 将无法收回已经扩散出去的访问权。因此分享权只属于设备主人。
  ///
  /// 是否分享由**产品配置**决定：只有 owner 且 product.allow_share=true 才能分享。
  bool get canShare => isOwner && allowShare;
}

/// 服务组内的单个服务（含该产品在该服务上的凭据）。
///
/// 凭据是「产品 × 服务」维度：同一服务组下不同服务的用户名/密码互不相同。
class IotServiceItem {
  /// 构造服务。
  const IotServiceItem({
    this.code = '',
    this.name = '',
    required this.protocol,
    required this.host,
    required this.port,
    required this.path,
    required this.extra,
    this.username = '',
    this.password = '',
  });

  /// 从 JSON 构造。
  factory IotServiceItem.fromJson(Map<String, dynamic> json) {
    return IotServiceItem(
      code: defStr(json['code']),
      name: defStr(json['name']),
      protocol: defStr(json['protocol']),
      host: defStr(json['host']),
      port: defInt(json['port']),
      path: defStr(json['path']),
      extra: _parseExtra(json['extra']),
      username: defStr(json['username']),
      password: defStr(json['password']),
    );
  }

  /// 服务编码（组内唯一）。
  final String code;

  /// 服务名称。
  final String name;

  /// 协议（MQTT / WebRTC / RTSP / SIP / WebSocket / TURN）。
  final String protocol;

  /// 主机名或 IP。
  final String host;

  /// 端口；后端允许为 0，由使用方按协议填默认端口。
  final int port;

  /// 路径（可选，部分协议用来携带额外标识）。
  final String path;

  /// 扩展参数（后端存 JSON 字符串，这里统一解析成 Map）。
  final Map<String, dynamic> extra;

  /// 该产品在该服务上的登录用户名。
  final String username;

  /// 该产品在该服务上的登录密码。
  final String password;

  /// 是否为 MQTT 服务。
  bool get isMqtt => protocol.toLowerCase().contains('mqtt');
}

/// IOT 服务组（含服务与产品级凭据）。
class IotServiceGroup {
  /// 构造服务组。
  const IotServiceGroup({
    required this.id,
    required this.code,
    required this.name,
    required this.type,
    this.services = const <IotServiceItem>[],
    // ── 兼容旧响应/旧调用方：组级凭据与 items 字段 ──
    List<IotServiceItem> items = const <IotServiceItem>[],
    String username = '',
    String password = '',
  })  : _legacyItems = items,
        _legacyUsername = username,
        _legacyPassword = password;

  /// 从 JSON 构造。
  factory IotServiceGroup.fromJson(Map<String, dynamic> json) {
    return IotServiceGroup(
      id: defInt(json['id']),
      code: defStr(json['code']),
      name: defStr(json['name']),
      type: defStr(json['type']),
      // 新契约字段名为 services；兼容旧响应的 items。
      services: defMapList(json['services'] ?? json['items'])
          .map(IotServiceItem.fromJson)
          .toList(growable: false),
    );
  }

  /// 主键。
  final int id;

  /// 组编码（MQTT topic 中的 `<group_code>`）。
  final String code;

  /// 组名称。
  final String name;

  /// 组类型 gb28181 / webrtc / aichat。
  final String type;

  /// 服务列表。
  final List<IotServiceItem> services;

  final List<IotServiceItem> _legacyItems;
  final String _legacyUsername;
  final String _legacyPassword;

  /// 服务列表（兼容旧字段 items）。
  List<IotServiceItem> get items =>
      services.isNotEmpty ? services : _legacyItems;

  /// 组级用户名（仅兼容旧响应；新契约凭据在服务上）。
  String get username => _legacyUsername;

  /// 组级密码（仅兼容旧响应；新契约凭据在服务上）。
  String get password => _legacyPassword;

  /// 第一个 MQTT 服务；没有则返回 null。
  IotServiceItem? get mqttItem {
    for (final IotServiceItem item in items) {
      if (item.isMqtt) return item;
    }
    return null;
  }
}

/// MQTT 连接信息（后端按产品下发）。
///
/// `url` 形如 `mqtt://host:1883`；`topicPrefix` 约定为 `iot/<group_code>`，
/// 同一产品下不同服务组使用不同前缀，MQTT ACL 依赖它做组间隔离。
class IotMqttInfo {
  /// 构造 MQTT 信息。
  const IotMqttInfo({required this.url, required this.topicPrefix});

  /// 从 JSON 构造。
  factory IotMqttInfo.fromJson(Map<String, dynamic> json) {
    return IotMqttInfo(
      url: defStr(json['url']),
      topicPrefix: defStr(json['topic_prefix']),
    );
  }

  /// Broker 地址。
  final String url;

  /// 主题前缀（`iot/<group_code>`）。
  final String topicPrefix;
}

/// TURN/STUN 接入点（不含短期凭据）。
class IotTurnInfo {
  /// 构造 TURN 信息。
  const IotTurnInfo({required this.urls, required this.realm});

  /// 从 JSON 构造。
  factory IotTurnInfo.fromJson(Map<String, dynamic> json) {
    return IotTurnInfo(
      urls: _parseUrls(json['urls']),
      realm: defStr(json['realm']),
    );
  }

  /// TURN/STUN 地址。
  final List<String> urls;

  /// TURN realm。
  final String realm;
}

/// `/app/iot/devices/:id/services` 返回的鉴权信息。
///
/// 结构是 `{product, groups:[{...,services:[{username,password,...}]}], mqtt, turn}`；
/// 每个服务携带该产品在该服务上的明文凭据（App 端登录 MQTT 必需）。
class IotCredentials {
  /// 构造鉴权信息。
  const IotCredentials({
    required this.productName,
    required this.productCode,
    required this.groups,
    required this.mqtt,
    required this.turn,
  });

  /// 从 JSON 构造。
  factory IotCredentials.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> product = defMap(json['product']);
    final dynamic mqttRaw = json['mqtt'];
    final dynamic turnRaw = json['turn'];
    return IotCredentials(
      productName: defStr(product['name']),
      productCode: defStr(product['code']),
      groups: defMapList(json['groups'])
          .map(IotServiceGroup.fromJson)
          .toList(growable: false),
      mqtt: mqttRaw is Map ? IotMqttInfo.fromJson(defMap(mqttRaw)) : null,
      turn: turnRaw is Map ? IotTurnInfo.fromJson(defMap(turnRaw)) : null,
    );
  }

  /// 产品名称。
  final String productName;

  /// 产品编码。
  final String productCode;

  /// 服务组（含服务与产品级凭据）。
  final List<IotServiceGroup> groups;

  /// MQTT 连接信息；未配置时为 null。
  final IotMqttInfo? mqtt;

  /// TURN 接入点；未配置时为 null。
  final IotTurnInfo? turn;

  /// 含 MQTT 服务的服务组；没有则返回第一个服务组。
  IotServiceGroup? get mqttGroup {
    for (final IotServiceGroup group in groups) {
      if (group.mqttItem != null) return group;
    }
    return groups.isEmpty ? null : groups.first;
  }
}

/// `/app/rtc/ice-servers` 返回的 ICE 配置。
///
/// `urls` 后端可能返回字符串或字符串数组，两种都接受。
class IceServerConfig {
  /// 构造 ICE 配置。
  const IceServerConfig({
    required this.urls,
    required this.username,
    required this.credential,
    required this.ttl,
    required this.expiresAt,
    required this.realm,
  });

  /// 从 JSON 构造。
  factory IceServerConfig.fromJson(Map<String, dynamic> json) {
    return IceServerConfig(
      urls: _parseUrls(json['urls']),
      username: defStr(json['username']),
      credential: defStr(json['credential']),
      ttl: defInt(json['ttl']),
      expiresAt: defInt(json['expires_at']),
      realm: defStr(json['realm']),
    );
  }

  /// STUN/TURN 地址列表。
  final List<String> urls;

  /// TURN 用户名（STUN 为空）。
  final String username;

  /// TURN 凭据（短期凭据，过期后需重新获取）。
  final String credential;

  /// 凭据有效期（秒）。
  final int ttl;

  /// 凭据过期时间（Unix 秒）。
  final int expiresAt;

  /// TURN realm。
  final String realm;

  /// 是否为空（空表示直连，不配置任何 STUN/TURN）。
  bool get isEmpty => urls.isEmpty;

  /// 转换为 `flutter_webrtc` 的 `iceServers` 元素。
  Map<String, dynamic> toRtcIceServer() {
    return <String, dynamic>{
      'urls': urls,
      if (username.isNotEmpty) 'username': username,
      if (credential.isNotEmpty) 'credential': credential,
    };
  }
}

/// 扫码后应调用的服务端动作。
enum IotQrAction {
  /// 分享码 → `POST /app/iot/devices/claim`。
  claim,

  /// 设备二维码 → `POST /app/iot/devices/bind-qr`。
  bindQr,
}

/// 扫码得到的内容。
///
/// 兼容两代二维码格式（URL 优先，JSON 兜底）：
///
/// 1. 链接格式（参考 `neme-ai-app`）：
///    - `boticz.cn/qrDevice/{product_code}/{device_code}` → 设备二维码；
///    - `boticz.cn/deviceShareToken/{token}` → 分享码；
///    - `http://` / `https://` 前缀可有可无。
/// 2. JSON 格式（本项目早期约定）：
///    - `{"type":"device","product_code":"...","device_code":"..."}`；
///    - `{"type":"device_share","token":"..."}`。
sealed class IotQrPayload {
  const IotQrPayload();

  /// 该二维码对应的服务端动作。
  IotQrAction get action;

  /// 解析二维码原文；不是本系统二维码时返回 null。
  ///
  /// 解析只做「结构校验」，不代表令牌/设备可用 —— 是否可用由服务端裁决，
  /// 失败原因（令牌过期、设备已被绑定）必须原样展示给用户。
  static IotQrPayload? tryParse(String raw) {
    final String text = raw.trim();
    if (text.isEmpty) return null;
    // 先按链接解析，失败再尝试 JSON：同一字符串只可能是其中一种格式。
    final IotQrPayload? fromUrl = _tryParseUrl(text);
    if (fromUrl != null) return fromUrl;
    return _tryParseJson(text);
  }

  /// 解析 `boticz.cn/qrDevice/...` / `boticz.cn/deviceShareToken/...`。
  static IotQrPayload? _tryParseUrl(String text) {
    final String lower = text.toLowerCase();
    // 没有 scheme 时 Uri 会把首段当成 path；补一个 https 再解析。
    final String withScheme = lower.startsWith('http://') ||
            lower.startsWith('https://')
        ? text
        : 'https://$text';
    final Uri? uri = Uri.tryParse(withScheme);
    if (uri == null || uri.host.isEmpty) return null;

    final String host = uri.host.toLowerCase();
    // 只接受本平台域名（含 www 子域），避免把任意站点的同名路径当设备码。
    if (host != 'boticz.cn' && !host.endsWith('.boticz.cn')) return null;

    final List<String> segments = uri.pathSegments;
    if (segments.length < 2) return null;
    final String kind = segments[0].toLowerCase();

    if (kind == 'qrdevice') {
      if (segments.length != 3) return null;
      final String productCode = segments[1].trim();
      final String deviceCode = segments[2].trim();
      if (productCode.isEmpty || deviceCode.isEmpty) return null;
      return DeviceQrCode(
        productCode: productCode,
        deviceCode: deviceCode,
      );
    }
    if (kind == 'devicesharetoken') {
      if (segments.length != 2) return null;
      final String token = segments[1].trim();
      if (token.isEmpty) return null;
      return DeviceShareCode(token);
    }
    return null;
  }

  /// 解析早期约定的 JSON 格式。
  static IotQrPayload? _tryParseJson(String text) {
    try {
      final dynamic decoded = jsonDecode(text);
      if (decoded is! Map) return null;
      final Map<String, dynamic> map = defMap(decoded);
      final String type = defStr(map['type']).trim();

      if (type == 'device_share') {
        final String token = defStr(map['token']).trim();
        if (token.isEmpty) return null;
        return DeviceShareCode(token);
      }
      if (type == 'device') {
        final String productCode = defStr(map['product_code']).trim();
        final String deviceCode = defStr(map['device_code']).trim();
        if (productCode.isEmpty || deviceCode.isEmpty) return null;
        return DeviceQrCode(
          productCode: productCode,
          deviceCode: deviceCode,
        );
      }
    } catch (_) {
      // 普通网址 / 文本二维码：不是本系统二维码。
    }
    return null;
  }
}

/// 分享码：`{"type":"device_share","token":"..."}`。
///
/// **令牌是一次性的**：后端认领后会立即作废，防止二维码被拍照后反复绑定；
/// 因此解析成功只代表「格式正确」，是否可用仍以服务端返回为准。
class DeviceShareCode extends IotQrPayload {
  /// 构造分享码。
  const DeviceShareCode(this.token);

  /// 分享令牌。
  final String token;

  @override
  IotQrAction get action => IotQrAction.claim;

  /// 解析分享码；非分享码返回 null。
  static DeviceShareCode? tryParse(String raw) {
    final IotQrPayload? payload = IotQrPayload.tryParse(raw);
    return payload is DeviceShareCode ? payload : null;
  }
}

/// 设备二维码：`{"type":"device","product_code":"...","device_code":"..."}`。
///
/// 与分享码不同，设备二维码**不含秘密**，它是印在机身/包装上的固定资产标识；
/// 绑定仍由服务端校验设备状态（未激活、未被他人绑定），因此不需要一次性令牌。
class DeviceQrCode extends IotQrPayload {
  /// 构造设备二维码。
  const DeviceQrCode({
    required this.productCode,
    required this.deviceCode,
  });

  /// 产品编码。
  final String productCode;

  /// 设备编码。
  final String deviceCode;

  @override
  IotQrAction get action => IotQrAction.bindQr;

  /// 解析设备二维码；非设备码返回 null。
  static DeviceQrCode? tryParse(String raw) {
    final IotQrPayload? payload = IotQrPayload.tryParse(raw);
    return payload is DeviceQrCode ? payload : null;
  }
}

/// 设备绑定结果（BLE 配网成功后由设备返回）。
class BleBindResult {
  /// 构建设备绑定结果。
  const BleBindResult({
    required this.deviceCode,
    required this.deviceSecret,
  });

  /// 设备编号。
  final String deviceCode;

  /// 设备接入密钥（只在配网时由设备直接给出，服务端之后只返回脱敏值）。
  final String deviceSecret;
}

/// 设备分享令牌（`POST /app/iot/devices/:id/share-token` 的返回）。
///
/// 服务端会同时创建一个「尚未绑定用户」的 shared 占位账号，令牌即该账号的
/// 认领凭证；家人用 App 扫码后走 `/app/iot/devices/claim` 完成绑定。
class IotDeviceShareToken {
  /// 构造分享令牌。
  const IotDeviceShareToken({
    required this.deviceId,
    required this.accountId,
    required this.token,
    required this.qrContent,
  });

  /// 从 JSON 构造。
  factory IotDeviceShareToken.fromJson(Map<String, dynamic> json) {
    return IotDeviceShareToken(
      deviceId: defInt(json['device_id']),
      accountId: defInt(json['account_id']),
      token: defStr(json['token']),
      qrContent: defStr(json['qr_content']),
    );
  }

  /// 设备 ID。
  final int deviceId;

  /// 待认领的共享账号 ID。
  final int accountId;

  /// 一次性分享令牌。
  final String token;

  /// 二维码内容：`https://boticz.cn/deviceShareToken/<token>`。
  final String qrContent;

  /// 用于展示 / 复制的文本；后端未给 `qr_content` 时按约定拼接。
  String get shareText =>
      qrContent.isNotEmpty ? qrContent : 'https://boticz.cn/deviceShareToken/$token';
}

// ════════════════════════════════════════════════════════════════
// 响应类型
// ════════════════════════════════════════════════════════════════

/// 单台设备响应。
class RIotDevice extends RSingleResponse<IotDevice> {
  /// 构造响应。
  RIotDevice() : super(IotDevice.fromJson);

  /// 设备详情。
  IotDevice? get device => value;
}

/// 设备列表响应。
///
/// 契约写的是「分页/列表」：后端可能返回数组，也可能返回
/// `{total, page, page_size, items}`。两种形态都解析，避免列表页因
/// 后端实现差异直接空白。
class RIotDeviceList extends Response {
  /// 已解析的列表。
  List<IotDevice>? _items;

  /// 设备列表。
  List<IotDevice> get items {
    final List<IotDevice>? cached = _items;
    if (cached != null) return cached;
    final dynamic payload = data;
    List<IotDevice> parsed;
    if (payload is List) {
      parsed = defMapList(payload)
          .map(IotDevice.fromJson)
          .toList(growable: false);
    } else {
      parsed = defMapList(defMap(payload)['items'])
          .map(IotDevice.fromJson)
          .toList(growable: false);
    }
    _items = parsed;
    return parsed;
  }

  /// 总条数（数组形态时等于当前条数）。
  int get total {
    final dynamic payload = data;
    if (payload is Map) {
      final Map<String, dynamic> map = defMap(payload);
      final dynamic raw = map['total'];
      if (raw != null) return defInt(raw);
    }
    return items.length;
  }
}

/// 分享令牌响应（`data` 为 `{device_id, account_id, token, qr_content}`）。
class RIotDeviceShareToken extends RSingleResponse<IotDeviceShareToken> {
  /// 构造响应。
  RIotDeviceShareToken() : super(IotDeviceShareToken.fromJson);

  /// 分享令牌。
  IotDeviceShareToken? get share => value;
}

/// 设备服务组响应（`data` 是 `{product, groups, mqtt, turn}` 对象）。
class RIotServices extends RSingleResponse<IotCredentials> {
  /// 构造响应。
  RIotServices() : super(IotCredentials.fromJson);

  /// 服务组与凭据。
  IotCredentials? get credentials => value;
}

/// ICE 配置响应（`data` 是数组，未配置 TURN 时为空数组）。
class RIceServers extends Response {
  List<IceServerConfig>? _servers;

  /// ICE 服务器列表。
  List<IceServerConfig> get servers {
    final List<IceServerConfig>? cached = _servers;
    if (cached != null) return cached;
    final List<IceServerConfig> parsed = defMapList(data)
        .map(IceServerConfig.fromJson)
        .toList(growable: false);
    _servers = parsed;
    return parsed;
  }
}

// ════════════════════════════════════════════════════════════════
// 控制器
// ════════════════════════════════════════════════════════════════

/// App 端 IOT 控制器。
class IotAppController {
  IotAppController._();

  // ── 路径常量（与后端 `/app/iot`、`/app/rtc` 路由严格对应）──
  /// 分享码认领。
  static const String claimPath = '/app/iot/devices/claim';
  /// 设备绑定。
  static const String bindPath = '/app/iot/devices/bind';
  /// 设备二维码绑定。
  static const String bindQrPath = '/app/iot/devices/bind-qr';
  /// 当前用户的设备列表。
  static const String devicesPath = '/app/iot/devices';
  /// ICE 配置。
  static const String iceServersPath = '/app/rtc/ice-servers';

  /// 设备分享令牌路径。
  static String deviceShareTokenPath(int id) => '$devicesPath/$id/share-token';

  // ══════════════════════════════════════════════════════════
  // 设备
  // ══════════════════════════════════════════════════════════

  /// 分页查询当前用户已绑定的设备。
  static Future<RIotDeviceList> devices({
    int page = 1,
    int pageSize = 20,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      devicesPath,
      query: <String, dynamic>{'page': page, 'page_size': pageSize},
      headers: headers,
    );
    return RIotDeviceList()..fromJson(response.data);
  }

  /// 查询设备详情。
  static Future<RIotDevice> deviceDetail(
    int id, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      '$devicesPath/$id',
      headers: headers,
    );
    return RIotDevice()..fromJson(response.data);
  }

  /// 查询设备可用的服务组（含服务与产品级凭据）。
  ///
  /// 每个服务上的 `password` 是**产品级明文凭据** —— 设备已通过 JWT 绑定到当前用户，
  /// 产品服务凭据是设备接入所必需的；这与 OTA 设备端接口的口径一致。
  static Future<RIotServices> services(
    int id, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      '$devicesPath/$id/services',
      headers: headers,
    );
    return RIotServices()..fromJson(response.data);
  }

  /// 用分享令牌认领设备。
  static Future<RIotDevice> claim(
    String token, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      claimPath,
      null,
      <String, dynamic>{'token': token},
      headers,
    );
    return RIotDevice()..fromJson(response.data);
  }

  /// 绑定设备（BLE 配网成功后调用）。
  ///
  /// [deviceCode] / [deviceSecret] 必须来自设备在配网会话中返回的值，
  /// 不能由 App 侧生成 —— 服务端据此确认「App 与设备确实在同一物理现场」。
  static Future<RIotDevice> bind({
    required String deviceCode,
    required String deviceSecret,
    String? nickname,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.post(
      bindPath,
      null,
      <String, dynamic>{
        'device_code': deviceCode,
        'device_secret': deviceSecret,
        if (nickname != null && nickname.trim().isNotEmpty)
          'nickname': nickname.trim(),
      },
      headers,
    );
    return RIotDevice()..fromJson(response.data);
  }

  /// 用设备二维码绑定设备（产品编码 + 设备编码）。
  ///
  /// 与 [bind] 的区别：二维码里没有 `device_secret`，安全性由服务端校验
  /// （设备必须已激活且未被他人绑定）保证，适合「设备已联网后再添加」的场景。
  static Future<RIotDevice> bindByQr({
    required String productCode,
    required String deviceCode,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.post(
      bindQrPath,
      null,
      <String, dynamic>{
        'product_code': productCode,
        'device_code': deviceCode,
      },
      headers,
    );
    return RIotDevice()..fromJson(response.data);
  }

  /// 为设备生成一次性分享令牌（仅 owner，适老化相机）。
  ///
  /// 每次调用都会创建一个新的 shared 占位账号，**旧令牌不会被自动作废**：
  /// 已生成但未被认领的码仍可用，需要由 Web 端
  /// `DELETE /web/iot/devices/:id/share-token` 统一撤销。
  /// 因此页面上的「重新生成」只用于换一个码，不做「旧码失效」承诺。
  static Future<RIotDeviceShareToken> createDeviceShareToken(
    int id, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      deviceShareTokenPath(id),
      null,
      null,
      headers,
    );
    return RIotDeviceShareToken()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // RTC
  // ══════════════════════════════════════════════════════════

  /// 查询 ICE 配置（STUN/TURN 短期凭据）。
  static Future<RIceServers> iceServers([
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      iceServersPath,
      headers: headers,
    );
    return RIceServers()..fromJson(response.data);
  }
}

// ════════════════════════════════════════════════════════════════
// 内部工具
// ════════════════════════════════════════════════════════════════

/// 取第一个非空字符串。
String _firstNonEmpty(List<String> values) {
  for (final String value in values) {
    if (value.isNotEmpty) return value;
  }
  return '';
}

/// 解析 `extra`：后端可能给 Map，也可能给 JSON 字符串。
Map<String, dynamic> _parseExtra(dynamic raw) {
  if (raw is Map) return defMap(raw);
  if (raw is String && raw.trim().isNotEmpty) {
    try {
      return defMap(jsonDecode(raw));
    } catch (_) {
      return <String, dynamic>{};
    }
  }
  return <String, dynamic>{};
}

/// 解析 `urls`：兼容字符串与字符串数组。
List<String> _parseUrls(dynamic raw) {
  if (raw is String) {
    final String value = raw.trim();
    return value.isEmpty ? const <String>[] : <String>[value];
  }
  if (raw is List) {
    return raw
        .map((dynamic e) => e?.toString().trim() ?? '')
        .where((String s) => s.isNotEmpty)
        .toList(growable: false);
  }
  return const <String>[];
}
