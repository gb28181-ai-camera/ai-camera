/// 移动端国标视频（GB/T 28181）数据结构定义。
///
/// 与后端 `internal/dto/response/app_gb.go` 一一对应（字段名严格使用后端 JSON tag）。
/// 移动端响应是**裁剪版**：不含 sip_password / password / sdp_ip 等管理面字段，
/// 因此这里不能直接复用 `AppController` 的 Web 设备/通道模型。
///
/// 页面中禁止直接读取 `Map<String, dynamic>`，一律通过本文件的强类型模型取值。
library;

import 'Definition.dart';
import 'Response.dart';

// ════════════════════════════════════════════════════════════════
// 设备 / 通道
// ════════════════════════════════════════════════════════════════

/// 国标设备（移动端裁剪版）。
class AppGbDevice {
  /// 构造设备。
  const AppGbDevice({
    required this.id,
    required this.name,
    required this.deviceCode,
    required this.gbDeviceId,
    required this.regionId,
    required this.regionName,
    required this.location,
    required this.status,
    required this.registerStatus,
    required this.channelCount,
    required this.active,
  });

  /// 从 JSON 构造。
  factory AppGbDevice.fromJson(Map<String, dynamic> json) {
    return AppGbDevice(
      id: defInt(json['id']),
      name: defStr(json['name']),
      deviceCode: defStr(json['device_code']),
      gbDeviceId: defStr(json['gb_device_id']),
      regionId: defIntOrNull(json['region_id']),
      regionName: defStr(json['region_name']),
      location: defStr(json['location']),
      status: defStr(json['status']),
      registerStatus: defStr(json['register_status']),
      channelCount: defInt(json['channel_count']),
      active: defBoolOrNull(json['active']),
    );
  }

  /// 主键。
  final int id;
  /// 设备名称。
  final String name;
  /// 平台内设备编号（非国标编码）。
  final String deviceCode;
  /// 国标设备编码（20 位）。
  final String gbDeviceId;
  /// 所属区域 ID。
  final int? regionId;
  /// 所属区域名称。
  final String regionName;
  /// 安装位置。
  final String location;
  /// 运行状态 online / offline。
  final String status;
  /// 注册状态 pending / activated / rejected。
  final String registerStatus;
  /// 通道数量。
  final int channelCount;
  /// 是否启用。
  final bool? active;

  /// 是否在线。
  bool get isOnline => status == 'online';

  /// 是否已注册（可点播）。
  bool get isActivated => registerStatus == 'activated';
}

/// 国标通道（移动端裁剪版）。
class AppGbChannel {
  /// 构造通道。
  const AppGbChannel({
    required this.id,
    required this.deviceId,
    required this.deviceName,
    required this.gbDeviceId,
    required this.gbChannelId,
    required this.name,
    required this.status,
    required this.channelType,
    required this.ptzType,
    required this.hasAudio,
    required this.enableBroadcast,
    required this.regionName,
    required this.groupName,
  });

  /// 从 JSON 构造。
  factory AppGbChannel.fromJson(Map<String, dynamic> json) {
    return AppGbChannel(
      id: defInt(json['id']),
      deviceId: defInt(json['device_id']),
      deviceName: defStr(json['device_name']),
      gbDeviceId: defStr(json['gb_device_id']),
      gbChannelId: defStr(json['gb_channel_id']),
      name: defStr(json['name']),
      status: defStr(json['status']),
      channelType: defInt(json['channel_type']),
      ptzType: defInt(json['ptz_type']),
      hasAudio: defBoolOrNull(json['has_audio']),
      enableBroadcast: defBoolOrNull(json['enable_broadcast']),
      regionName: defStr(json['region_name']),
      groupName: defStr(json['group_name']),
    );
  }

  /// 主键（播放/云台接口都用它，不是国标编码）。
  final int id;
  /// 所属设备 ID。
  final int deviceId;
  /// 所属设备名称（后端回填）。
  final String deviceName;
  /// 设备国标编码。
  final String gbDeviceId;
  /// 通道国标编码。
  final String gbChannelId;
  /// 通道名称。
  final String name;
  /// 通道状态 ON / OFF（大写，与设备侧一致）。
  final String status;
  /// 通道类型（设备侧上报）。
  final int channelType;
  /// 云台类型；0 通常表示不支持云台。
  final int ptzType;
  /// 是否带音频。
  final bool? hasAudio;
  /// 是否允许语音广播。
  final bool? enableBroadcast;
  /// 所属区域名称。
  final String regionName;
  /// 业务分组名称。
  final String groupName;

  /// 通道是否在线（设备侧用 ON/OFF，不是 online/offline）。
  bool get isOnline => status.toUpperCase() == 'ON';

  /// 是否可能支持云台控制。
  ///
  /// 这只是「有没有声明云台类型」的弱判断：部分设备 ptz_type=0 但实际支持，
  /// 也有声明了却不响应的。因此 UI 只用来做提示，不据此禁用面板。
  bool get maybeSupportsPtz => ptzType > 0;
}

// ════════════════════════════════════════════════════════════════
// 实时播放 / 抓图
// ════════════════════════════════════════════════════════════════

/// 实时点播结果（契约 §15.1 的移动端裁剪版）。
class AppGbPlayInfo {
  /// 构造播放信息。
  const AppGbPlayInfo({
    required this.app,
    required this.stream,
    required this.httpBase,
    required this.webrtc,
    required this.flv,
    required this.hls,
    required this.streamId,
  });

  /// 从 JSON 构造。
  factory AppGbPlayInfo.fromJson(Map<String, dynamic> json) {
    return AppGbPlayInfo(
      app: defStr(json['app']),
      stream: defStr(json['stream']),
      httpBase: defStr(json['http_base']),
      webrtc: defStr(json['webrtc']),
      flv: defStr(json['flv']),
      hls: defStr(json['hls']),
      streamId: defStr(json['stream_id']),
    );
  }

  /// 应用名（恒为 rtp）。
  final String app;
  /// 流 ID（SSRC 十六进制）。
  final String stream;
  /// ZLM HTTP 基址。
  final String httpBase;
  /// WHEP 信令地址（WebRTC）。
  final String webrtc;
  /// HTTP-FLV 地址。
  final String flv;
  /// HLS 地址。
  final String hls;
  /// 会话标识（SIP Call-ID），停止播放时回传。
  final String streamId;

  /// 是否有可用的 WebRTC 地址。
  bool get hasWebrtc => webrtc.isNotEmpty;

  /// 回退播放地址：优先 FLV，其次 HLS。
  String get fallbackUrl => flv.isNotEmpty ? flv : hls;
}

/// 抓图结果。
class AppGbSnapshot {
  /// 构造抓图结果。
  const AppGbSnapshot({
    required this.url,
    required this.fileName,
    required this.fileSize,
    required this.captureDt,
  });

  /// 从 JSON 构造。
  factory AppGbSnapshot.fromJson(Map<String, dynamic> json) {
    return AppGbSnapshot(
      url: defStr(json['url']),
      fileName: defStr(json['file_name']),
      fileSize: defInt(json['file_size']),
      captureDt: defStr(json['capture_dt']),
    );
  }

  /// 可直接访问的图片地址。
  final String url;
  /// 文件名。
  final String fileName;
  /// 文件大小（字节）。
  final int fileSize;
  /// 抓图时间。
  final String captureDt;

  /// 文件大小的人类可读形式。
  String get readableSize {
    if (fileSize <= 0) return '--';
    if (fileSize < 1024) return '$fileSize B';
    if (fileSize < 1024 * 1024) {
      return '${(fileSize / 1024).toStringAsFixed(1)} KB';
    }
    return '${(fileSize / 1024 / 1024).toStringAsFixed(1)} MB';
  }
}

// ════════════════════════════════════════════════════════════════
// 录像查询 / 回放
// ════════════════════════════════════════════════════════════════

/// 设备侧录像条目。
class AppGbRecord {
  /// 构造录像条目。
  const AppGbRecord({
    required this.channelId,
    required this.name,
    required this.startTime,
    required this.endTime,
    required this.timeLen,
  });

  /// 从 JSON 构造。
  factory AppGbRecord.fromJson(Map<String, dynamic> json) {
    return AppGbRecord(
      channelId: defInt(json['channel_id']),
      name: defStr(json['name']),
      startTime: defStr(json['start_time']),
      endTime: defStr(json['end_time']),
      timeLen: defInt(json['time_len']),
    );
  }

  /// 通道 ID。
  final int channelId;
  /// 录像名称。
  final String name;
  /// 开始时间（2006-01-02T15:04:05）。
  final String startTime;
  /// 结束时间。
  final String endTime;
  /// 时长（秒）。
  final int timeLen;

  /// 时长的人类可读形式。
  String get readableDuration {
    if (timeLen <= 0) return '--';
    final int hours = timeLen ~/ 3600;
    final int minutes = (timeLen % 3600) ~/ 60;
    final int seconds = timeLen % 60;
    if (hours > 0) return '$hours 小时 $minutes 分';
    if (minutes > 0) return '$minutes 分 $seconds 秒';
    return '$seconds 秒';
  }
}

/// 录像检索结果。
class AppGbRecordQueryResult {
  /// 构造检索结果。
  const AppGbRecordQueryResult({
    required this.channelId,
    required this.total,
    required this.timeout,
    required this.items,
  });

  /// 从 JSON 构造。
  factory AppGbRecordQueryResult.fromJson(Map<String, dynamic> json) {
    return AppGbRecordQueryResult(
      channelId: defInt(json['channel_id']),
      total: defInt(json['total']),
      timeout: defBool(json['timeout']),
      items: defMapList(json['items'])
          .map(AppGbRecord.fromJson)
          .toList(growable: false),
    );
  }

  /// 通道 ID。
  final int channelId;
  /// 条目总数。
  final int total;
  /// 设备是否在等待窗口内未回齐（items 可能为空）。
  final bool timeout;
  /// 录像条目。
  final List<AppGbRecord> items;
}

/// 回放会话。
class AppGbPlaybackInfo {
  /// 构造回放会话。
  const AppGbPlaybackInfo({
    required this.streamId,
    required this.app,
    required this.stream,
    required this.httpBase,
    required this.webrtc,
    required this.flv,
    required this.hls,
    required this.channelId,
    required this.startTime,
    required this.endTime,
  });

  /// 从 JSON 构造。
  factory AppGbPlaybackInfo.fromJson(Map<String, dynamic> json) {
    return AppGbPlaybackInfo(
      streamId: defStr(json['stream_id']),
      app: defStr(json['app']),
      stream: defStr(json['stream']),
      httpBase: defStr(json['http_base']),
      webrtc: defStr(json['webrtc']),
      flv: defStr(json['flv']),
      hls: defStr(json['hls']),
      channelId: defInt(json['channel_id']),
      startTime: defStr(json['start_time']),
      endTime: defStr(json['end_time']),
    );
  }

  /// 会话标识（停止回放时使用）。
  final String streamId;
  /// 应用名。
  final String app;
  /// 流 ID。
  final String stream;
  /// ZLM HTTP 基址。
  final String httpBase;
  /// WebRTC 地址。
  final String webrtc;
  /// HTTP-FLV 地址。
  final String flv;
  /// HLS 地址。
  final String hls;
  /// 通道 ID。
  final int channelId;
  /// 回放开始时间。
  final String startTime;
  /// 回放结束时间。
  final String endTime;
}

// ════════════════════════════════════════════════════════════════
// 语音对讲
// ════════════════════════════════════════════════════════════════

/// 语音对讲会话。
class AppGbVoiceSession {
  /// 构造语音会话。
  const AppGbVoiceSession({
    required this.sessionId,
    required this.mode,
    required this.agentEnabled,
    required this.ssrc,
    required this.audioCodec,
    required this.startDt,
  });

  /// 从 JSON 构造。
  factory AppGbVoiceSession.fromJson(Map<String, dynamic> json) {
    return AppGbVoiceSession(
      sessionId: defStr(json['session_id']),
      mode: defStr(json['mode']),
      agentEnabled: defBool(json['agent_enabled']),
      ssrc: defStr(json['ssrc']),
      audioCodec: defStr(json['audio_codec']),
      startDt: defStr(json['start_dt']),
    );
  }

  /// 会话标识，停止对讲时回传。
  final String sessionId;
  /// 半双工 / 全双工。
  final String mode;
  /// 是否接入了语音 Agent。
  final bool agentEnabled;
  /// 音频回推 SSRC。
  final String ssrc;
  /// 音频编码（G.711A）。
  final String audioCodec;
  /// 会话开始时间。
  final String startDt;
}

// ════════════════════════════════════════════════════════════════
// 国标报警
// ════════════════════════════════════════════════════════════════

/// 国标报警条目。
class AppGbAlarm {
  /// 构造报警。
  const AppGbAlarm({
    required this.id,
    required this.deviceId,
    required this.channelId,
    required this.deviceName,
    required this.channelName,
    required this.alarmPriority,
    required this.alarmType,
    required this.alarmTime,
    required this.alarmDescription,
    required this.snapPath,
    required this.handled,
  });

  /// 从 JSON 构造。
  factory AppGbAlarm.fromJson(Map<String, dynamic> json) {
    return AppGbAlarm(
      id: defInt(json['id']),
      deviceId: defIntOrNull(json['device_id']),
      channelId: defIntOrNull(json['channel_id']),
      deviceName: defStr(json['device_name']),
      channelName: defStr(json['channel_name']),
      alarmPriority: defStr(json['alarm_priority']),
      alarmType: defStr(json['alarm_type']),
      alarmTime: defStrOrNull(json['alarm_time']),
      alarmDescription: defStr(json['alarm_description']),
      snapPath: defStr(json['snap_path']),
      handled: defBoolOrNull(json['handled']),
    );
  }

  /// 主键。
  final int id;
  /// 设备 ID。
  final int? deviceId;
  /// 通道 ID。
  final int? channelId;
  /// 设备名称。
  final String deviceName;
  /// 通道名称。
  final String channelName;
  /// 报警级别 1/2/3/4。
  final String alarmPriority;
  /// 报警类型编码。
  final String alarmType;
  /// 报警时间（RFC3339）。
  final String? alarmTime;
  /// 报警描述。
  final String alarmDescription;
  /// 抓图路径。
  final String snapPath;
  /// 是否已处理。
  final bool? handled;
}

// ════════════════════════════════════════════════════════════════
// 请求体（对应后端 request.GbXxxReq）
// ════════════════════════════════════════════════════════════════

/// 契约 §15.2 冻结的九种云台方向。
class GbPtzDirection {
  GbPtzDirection._();

  /// 上。
  static const String up = 'up';
  /// 下。
  static const String down = 'down';
  /// 左。
  static const String left = 'left';
  /// 右。
  static const String right = 'right';
  /// 左上。
  static const String upLeft = 'up-left';
  /// 右上。
  static const String upRight = 'up-right';
  /// 左下。
  static const String downLeft = 'down-left';
  /// 右下。
  static const String downRight = 'down-right';
  /// 停止。
  static const String stop = 'stop';
}

/// 变焦动作（与方向互斥）。
class GbPtzAction {
  GbPtzAction._();

  /// 放大。
  static const String zoomIn = 'zoom-in';
  /// 缩小。
  static const String zoomOut = 'zoom-out';
  /// 停止变焦。
  static const String zoomStop = 'zoom-stop';
}

/// 实时点播请求（契约 §15.1）。
class GbPlayReq {
  /// 构造点播请求。
  const GbPlayReq({this.protocol = 'webrtc', this.streamId = ''});

  /// webrtc / flv / hls，缺省 webrtc。
  final String protocol;
  /// 可选：重用已有流。
  final String streamId;

  /// 序列化为后端 JSON。
  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      if (protocol.isNotEmpty) 'protocol': protocol,
      if (streamId.isNotEmpty) 'stream_id': streamId,
    };
  }
}

/// 云台控制请求（契约 §15.2）。
///
/// ★ direction 与 action **互斥**：后端两者都传或都不传都会返回 400。
/// 页面侧用 [GbPtzDirection] / [GbPtzAction] 常量，避免手写字符串拼错后静默失效。
class GbPtzReq {
  /// 构造云台请求。
  const GbPtzReq({this.direction = '', this.speed = 5, this.action = ''});

  /// 八方向 + 停止。
  final String direction;
  /// 速度 1–8（1 最慢、8 最快），缺省 5。
  final int speed;
  /// 变焦/光圈/聚焦动作，与 [direction] 互斥。
  final String action;

  /// 序列化为后端 JSON。
  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      if (direction.isNotEmpty) 'direction': direction,
      if (action.isNotEmpty) 'action': action,
      'speed': speed,
    };
  }
}

/// 录像回放请求。
class GbPlaybackReq {
  /// 构造回放请求。
  const GbPlaybackReq({
    required this.channelId,
    required this.startTime,
    required this.endTime,
  });

  /// 通道 ID。
  final int channelId;
  /// 开始时间，格式 `2006-01-02T15:04:05`。
  final String startTime;
  /// 结束时间，格式 `2006-01-02T15:04:05`。
  final String endTime;

  /// 序列化为后端 JSON。
  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'channel_id': channelId,
      'start_time': startTime,
      'end_time': endTime,
    };
  }
}

/// 回放控制请求（契约 §15.3）。
class GbPlaybackControlReq {
  /// 构造回放控制请求。
  const GbPlaybackControlReq({
    required this.streamId,
    required this.action,
    this.seekTime = '',
    this.scale = 0,
  });

  /// 回放会话标识。
  final String streamId;
  /// pause / resume / seek / speed。
  final String action;
  /// 仅 seek 时必填。
  final String seekTime;
  /// 仅 speed 时必填，1.0 为原速。
  final double scale;

  /// 序列化为后端 JSON。
  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'stream_id': streamId,
      'action': action,
      if (seekTime.isNotEmpty) 'seek_time': seekTime,
      if (scale > 0) 'scale': scale,
    };
  }
}

/// 语音会话启动请求。
class GbTalkStartReq {
  /// 构造对讲请求。
  const GbTalkStartReq({this.agentEnabled = false});

  /// 是否接入语音 Agent；控制面不可达时后端自动降级为纯对讲。
  final bool agentEnabled;

  /// 序列化为后端 JSON。
  Map<String, dynamic> toJson() {
    return <String, dynamic>{'agent_enabled': agentEnabled};
  }
}

// ════════════════════════════════════════════════════════════════
// 具体端点响应类型
// ════════════════════════════════════════════════════════════════

/// `GET /app/gb/devices` → `response.RAppPageGbDevice`。
class RAppGbDevicePage extends RPageResponse<AppGbDevice> {
  /// 构建设备分页响应。
  RAppGbDevicePage() : super(AppGbDevice.fromJson);
}

/// `GET /app/gb/channels` → `response.RAppPageGbChannel`。
class RAppGbChannelPage extends RPageResponse<AppGbChannel> {
  /// 构建通道分页响应。
  RAppGbChannelPage() : super(AppGbChannel.fromJson);
}

/// `POST /app/gb/play/{channelId}` → `response.RAppGbPlay`。
class RAppGbPlay extends RSingleResponse<AppGbPlayInfo> {
  /// 构建点播响应。
  RAppGbPlay() : super(AppGbPlayInfo.fromJson);
}

/// `POST /app/gb/play/{channelId}/snapshot` → `response.RAppGbSnapshot`。
class RAppGbSnapshot extends RSingleResponse<AppGbSnapshot> {
  /// 构建抓图响应。
  RAppGbSnapshot() : super(AppGbSnapshot.fromJson);
}

/// `GET /app/gb/playback/records` → `response.RAppGbRecordQuery`。
class RAppGbRecordQuery extends RSingleResponse<AppGbRecordQueryResult> {
  /// 构建录像检索响应。
  RAppGbRecordQuery() : super(AppGbRecordQueryResult.fromJson);
}

/// `POST /app/gb/playback` → `response.RAppGbPlayback`。
class RAppGbPlayback extends RSingleResponse<AppGbPlaybackInfo> {
  /// 构建回放会话响应。
  RAppGbPlayback() : super(AppGbPlaybackInfo.fromJson);
}

/// `POST /app/gb/talk/{channelId}/start` → `response.RAppGbVoiceSession`。
class RAppGbVoiceSession extends RSingleResponse<AppGbVoiceSession> {
  /// 构建语音会话响应。
  RAppGbVoiceSession() : super(AppGbVoiceSession.fromJson);
}

/// `GET /app/gb/alarms` → `response.RAppPageGbAlarm`。
class RAppGbAlarmPage extends RPageResponse<AppGbAlarm> {
  /// 构建报警分页响应。
  RAppGbAlarmPage() : super(AppGbAlarm.fromJson);
}
