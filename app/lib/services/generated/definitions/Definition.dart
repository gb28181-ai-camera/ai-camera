/// 移动端接口数据结构定义（对应后端 `internal/dto/response` 与 `internal/model`）。
///
/// 本文件由「后端接口契约」（`docs/开发文档/后端开发/00-接口契约.md`）手工映射而来，
/// 字段名与后端 JSON tag 严格一一对应（snake_case）。
/// 后端结构变更时必须同步修改此处，**禁止**在页面中直接用 `as any` 式的动态访问。
library;

// ════════════════════════════════════════════════════════════════
// JSON 解析辅助函数
//
// 后端字段存在 *int64 / *bool（可为 null）与时间字符串混用的情况，
// 统一经此处转换，保证类型安全且不抛异常。
// ════════════════════════════════════════════════════════════════

/// 解析为 int，失败返回 0。
int defInt(dynamic v) {
  if (v == null) return 0;
  if (v is int) return v;
  if (v is num) return v.toInt();
  if (v is bool) return v ? 1 : 0;
  return int.tryParse(v.toString()) ?? 0;
}

/// 解析为可空 int，null / 空串返回 null。
int? defIntOrNull(dynamic v) {
  if (v == null) return null;
  if (v is int) return v;
  if (v is num) return v.toInt();
  final String s = v.toString().trim();
  if (s.isEmpty) return null;
  return int.tryParse(s);
}

/// 解析为 double，失败返回 0。
double defDouble(dynamic v) {
  if (v == null) return 0;
  if (v is double) return v;
  if (v is num) return v.toDouble();
  return double.tryParse(v.toString()) ?? 0;
}

/// 解析为 String，null 返回空串。
String defStr(dynamic v) => v?.toString() ?? '';

/// 解析为可空 String，null / 空串返回 null。
String? defStrOrNull(dynamic v) {
  if (v == null) return null;
  final String s = v.toString();
  return s.isEmpty ? null : s;
}

/// 解析为 bool，null 时取 [fallback]。
bool defBool(dynamic v, {bool fallback = false}) {
  if (v == null) return fallback;
  if (v is bool) return v;
  if (v is num) return v != 0;
  final String s = v.toString().toLowerCase();
  if (s == 'true' || s == '1') return true;
  if (s == 'false' || s == '0') return false;
  return fallback;
}

/// 解析为可空 bool，保留「未设置」语义（后端 `*bool`）。
bool? defBoolOrNull(dynamic v) {
  if (v == null) return null;
  if (v is bool) return v;
  if (v is num) return v != 0;
  final String s = v.toString().toLowerCase();
  if (s == 'true' || s == '1') return true;
  if (s == 'false' || s == '0') return false;
  return null;
}

/// 解析为动态 Map，失败返回空 Map。
Map<String, dynamic> defMap(dynamic v) {
  if (v is Map<String, dynamic>) return v;
  if (v is Map) return v.map((k, val) => MapEntry(k.toString(), val));
  return <String, dynamic>{};
}

/// 解析为 Map 列表。
List<Map<String, dynamic>> defMapList(dynamic v) {
  if (v is List) {
    return v.map(defMap).toList(growable: false);
  }
  return const <Map<String, dynamic>>[];
}

/// 解析为 String 列表。
List<String> defStrList(dynamic v) {
  if (v is List) {
    return v.map((e) => e.toString()).toList(growable: false);
  }
  return const <String>[];
}

// ════════════════════════════════════════════════════════════════
// 通用分页载荷
// ════════════════════════════════════════════════════════════════

/// 分页载荷：`{total, page, page_size, items}`。
class PageData<T> {
  /// 构造分页载荷。
  const PageData({
    required this.total,
    required this.page,
    required this.pageSize,
    required this.items,
  });

  /// 从 JSON 构造，[parse] 负责把单条 item 转成 [T]。
  factory PageData.fromJson(dynamic json, T Function(Map<String, dynamic>) parse) {
    final Map<String, dynamic> m = defMap(json);
    return PageData<T>(
      total: defInt(m['total']),
      page: defInt(m['page']),
      pageSize: defInt(m['page_size']),
      items: defMapList(m['items']).map(parse).toList(growable: false),
    );
  }

  /// 空载荷。
  static PageData<T> empty<T>(T Function(Map<String, dynamic>) parse) =>
      PageData<T>(total: 0, page: 1, pageSize: 20, items: <T>[]);

  /// 总条数。
  final int total;
  /// 当前页码。
  final int page;
  /// 每页条数。
  final int pageSize;
  /// 当前页数据。
  final List<T> items;

  /// 是否还有下一页（用于上滑加载更多）。
  bool get hasMore => page * pageSize < total;

  /// 是否为空。
  bool get isEmpty => items.isEmpty;
}

// ════════════════════════════════════════════════════════════════
// 关联对象（列表展示用名称字段）
// ════════════════════════════════════════════════════════════════

/// 区域。
class RegionRef {
  /// 构造区域。
  const RegionRef({required this.id, required this.name});

  /// 从 JSON 构造。
  factory RegionRef.fromJson(Map<String, dynamic> json) => RegionRef(
        id: defInt(json['id']),
        name: defStr(json['name']),
      );

  /// 区域 ID。
  final int id;
  /// 区域名称。
  final String name;
}

/// 风险类型。
class RiskTypeRef {
  /// 构造风险类型。
  const RiskTypeRef({
    required this.id,
    required this.name,
    required this.code,
    required this.level,
  });

  /// 从 JSON 构造。
  factory RiskTypeRef.fromJson(Map<String, dynamic> json) => RiskTypeRef(
        id: defInt(json['id']),
        name: defStr(json['name']),
        code: defStr(json['code']),
        level: defStr(json['level']),
      );

  /// 风险类型 ID。
  final int id;
  /// 风险类型名称。
  final String name;
  /// 风险编码。
  final String code;
  /// 风险等级 high/medium/low。
  final String level;
}

/// 告警类型。
class AlarmTypeRef {
  /// 构造告警类型。
  const AlarmTypeRef({
    required this.id,
    required this.name,
    required this.code,
    required this.level,
  });

  /// 从 JSON 构造。
  factory AlarmTypeRef.fromJson(Map<String, dynamic> json) => AlarmTypeRef(
        id: defInt(json['id']),
        name: defStr(json['name']),
        code: defStr(json['code']),
        level: defStr(json['level']),
      );

  /// 告警类型 ID。
  final int id;
  /// 告警类型名称。
  final String name;
  /// 告警编码 offline/battery/traffic。
  final String code;
  /// 告警等级 high/medium/low。
  final String level;
}

/// 部门。
class DeptRef {
  /// 构造部门。
  const DeptRef({required this.id, required this.name});

  /// 从 JSON 构造。
  factory DeptRef.fromJson(Map<String, dynamic> json) => DeptRef(
        id: defInt(json['id']),
        name: defStr(json['name']),
      );

  /// 部门 ID。
  final int id;
  /// 部门名称。
  final String name;
}

/// 岗位。
class PostRef {
  /// 构造岗位。
  const PostRef({required this.id, required this.name});

  /// 从 JSON 构造。
  factory PostRef.fromJson(Map<String, dynamic> json) => PostRef(
        id: defInt(json['id']),
        name: defStr(json['name']),
      );

  /// 岗位 ID。
  final int id;
  /// 岗位名称。
  final String name;
}

// ════════════════════════════════════════════════════════════════
// 设备
// ════════════════════════════════════════════════════════════════

/// 设备（含运行时状态快照与关联区域）。
class Device {
  /// 构建设备。
  const Device({
    required this.id,
    required this.name,
    required this.deviceCode,
    required this.regionId,
    required this.regionName,
    required this.location,
    required this.status,
    required this.battery,
    required this.voltage,
    required this.storageTotal,
    required this.storageFree,
    required this.firmwareVer,
    required this.modelVer,
    required this.systemVer,
    required this.workMode,
    required this.deviceMode,
    required this.infoInterval,
    required this.streamUrl,
    required this.onlineDT,
    required this.offlineDT,
    required this.lastBeatDT,
    required this.monitorEnabled,
    required this.remark,
  });

  /// 从 JSON 构造。
  factory Device.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> region = defMap(json['region']);
    return Device(
      id: defInt(json['id']),
      name: defStr(json['name']),
      deviceCode: defStr(json['device_code']),
      regionId: defIntOrNull(json['region_id']),
      regionName: defStr(region['name']),
      location: defStr(json['location']),
      status: defStr(json['status']),
      battery: defInt(json['battery']),
      voltage: defStr(json['voltage']),
      storageTotal: defInt(json['storage_total']),
      storageFree: defInt(json['storage_free']),
      firmwareVer: defStr(json['firmware_ver']),
      modelVer: defStr(json['model_ver']),
      systemVer: defStr(json['system_ver']),
      workMode: defStr(json['work_mode']),
      deviceMode: defStr(json['device_mode']),
      infoInterval: defInt(json['info_interval']),
      streamUrl: defStr(json['stream_url']),
      onlineDT: defStrOrNull(json['online_dt']),
      offlineDT: defStrOrNull(json['offline_dt']),
      lastBeatDT: defStrOrNull(json['last_beat_dt']),
      monitorEnabled: defBool(json['monitor_enabled']),
      remark: defStr(json['remark']),
    );
  }

  /// 设备 ID。
  final int id;
  /// 设备名称。
  final String name;
  /// 设备编号。
  final String deviceCode;
  /// 所属区域 ID。
  final int? regionId;
  /// 所属区域名称（由后端 `region.name` 提供）。
  final String regionName;
  /// 设备位置。
  final String location;
  /// 运行状态 online/offline。
  final String status;
  /// 电量百分比。
  final int battery;
  /// 电压。
  final String voltage;
  /// 存储总容量（MB）。
  final int storageTotal;
  /// 剩余存储容量（MB）。
  final int storageFree;
  /// 当前固件版本。
  final String firmwareVer;
  /// 当前 AI 模型版本。
  final String modelVer;
  /// 系统版本。
  final String systemVer;
  /// 设备工作模式。
  final String workMode;
  /// 设备模式（充电/全开/省电）。
  final String deviceMode;
  /// 信息上传间隔（秒）。
  final int infoInterval;
  /// 实时视频流地址。
  final String streamUrl;
  /// 最近上线时间。
  final String? onlineDT;
  /// 最近离线时间。
  final String? offlineDT;
  /// 最近心跳时间。
  final String? lastBeatDT;
  /// 是否加入实时监控。
  final bool monitorEnabled;
  /// 备注。
  final String remark;

  /// 是否在线。
  bool get isOnline => status == 'online';

  /// 存储使用率（0~1）。
  double get storageUsedRatio {
    if (storageTotal <= 0) return 0;
    final int used = storageTotal - storageFree;
    return (used / storageTotal).clamp(0.0, 1.0);
  }
}

// ════════════════════════════════════════════════════════════════
// AI 风险
// ════════════════════════════════════════════════════════════════

/// AI 风险记录。
class RiskRecord {
  /// 构造风险记录。
  const RiskRecord({
    required this.id,
    required this.riskTypeId,
    required this.riskTypeName,
    required this.riskLevel,
    required this.deviceId,
    required this.deviceCode,
    required this.deviceName,
    required this.regionName,
    required this.location,
    required this.imageUrl,
    required this.videoUrl,
    required this.detectScore,
    required this.occurredDT,
    required this.detectAccurate,
    required this.accurateBy,
    required this.accurateDT,
    required this.read,
    required this.status,
    required this.handleResult,
    required this.handleImage,
    required this.handler,
    required this.handledDT,
  });

  /// 从 JSON 构造。
  factory RiskRecord.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> type = defMap(json['risk_type']);
    final Map<String, dynamic> device = defMap(json['device']);
    final Map<String, dynamic> region = defMap(json['region']);
    return RiskRecord(
      id: defInt(json['id']),
      riskTypeId: defInt(json['risk_type_id']),
      riskTypeName: defStr(type['name']),
      riskLevel: defStr(json['risk_level']),
      deviceId: defInt(json['device_id']),
      deviceCode: defStr(json['device_code']),
      deviceName: defStr(device['name']),
      regionName: defStr(region['name']),
      location: defStr(json['location']),
      imageUrl: defStr(json['image_url']),
      videoUrl: defStr(json['video_url']),
      detectScore: defDouble(json['detect_score']),
      occurredDT: defStrOrNull(json['occurred_dt']),
      detectAccurate: defStr(json['detect_accurate']),
      accurateBy: defStr(json['accurate_by']),
      accurateDT: defStrOrNull(json['accurate_dt']),
      read: defBoolOrNull(json['read']),
      status: defStr(json['status']),
      handleResult: defStr(json['handle_result']),
      handleImage: defStr(json['handle_image']),
      handler: defStr(json['handler']),
      handledDT: defStrOrNull(json['handled_dt']),
    );
  }

  /// 风险记录 ID。
  final int id;
  /// 风险类型 ID。
  final int riskTypeId;
  /// 风险类型名称。
  final String riskTypeName;
  /// 风险等级 high/medium/low。
  final String riskLevel;
  /// 设备 ID。
  final int deviceId;
  /// 设备编号。
  final String deviceCode;
  /// 设备名称。
  final String deviceName;
  /// 区域名称。
  final String regionName;
  /// 设备位置。
  final String location;
  /// 风险图片地址。
  final String imageUrl;
  /// 风险录像地址。
  final String videoUrl;
  /// AI 置信度（0~1）。
  final double detectScore;
  /// 发生时间。
  final String? occurredDT;
  /// 检测准确 yes/no，空串表示尚未确认。
  final String detectAccurate;
  /// 确认人。
  final String accurateBy;
  /// 确认时间。
  final String? accurateDT;
  /// 是否已读。
  final bool? read;
  /// 处理状态 pending/handled。
  final String status;
  /// 处理结果。
  final String handleResult;
  /// 处理图片。
  final String handleImage;
  /// 处理人。
  final String handler;
  /// 处理时间。
  final String? handledDT;

  /// 是否已处理。
  bool get isHandled => status == 'handled';

  /// 是否待处理。
  bool get isPending => status == 'pending';

  /// 是否已确认检测准确性。
  bool get isAccurateConfirmed => detectAccurate == 'yes' || detectAccurate == 'no';

  /// 置信度百分比文本。
  String get detectScoreText => '${(detectScore * 100).toStringAsFixed(1)}%';
}

// ════════════════════════════════════════════════════════════════
// 设备告警
// ════════════════════════════════════════════════════════════════

/// 设备告警记录。
class AlarmRecord {
  /// 构造告警记录。
  const AlarmRecord({
    required this.id,
    required this.alarmTypeId,
    required this.alarmTypeName,
    required this.alarmLevel,
    required this.deviceId,
    required this.deviceCode,
    required this.deviceName,
    required this.regionName,
    required this.location,
    required this.content,
    required this.occurredDT,
    required this.read,
    required this.status,
    required this.handleResult,
    required this.handleImage,
    required this.handler,
    required this.handledDT,
  });

  /// 从 JSON 构造。
  factory AlarmRecord.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> type = defMap(json['alarm_type']);
    final Map<String, dynamic> device = defMap(json['device']);
    final Map<String, dynamic> region = defMap(json['region']);
    return AlarmRecord(
      id: defInt(json['id']),
      alarmTypeId: defInt(json['alarm_type_id']),
      alarmTypeName: defStr(type['name']),
      alarmLevel: defStr(json['alarm_level']),
      deviceId: defInt(json['device_id']),
      deviceCode: defStr(json['device_code']),
      deviceName: defStr(device['name']),
      regionName: defStr(region['name']),
      location: defStr(json['location']),
      content: defStr(json['content']),
      occurredDT: defStrOrNull(json['occurred_dt']),
      read: defBoolOrNull(json['read']),
      status: defStr(json['status']),
      handleResult: defStr(json['handle_result']),
      handleImage: defStr(json['handle_image']),
      handler: defStr(json['handler']),
      handledDT: defStrOrNull(json['handled_dt']),
    );
  }

  /// 告警记录 ID。
  final int id;
  /// 告警类型 ID。
  final int alarmTypeId;
  /// 告警类型名称。
  final String alarmTypeName;
  /// 告警等级 high/medium/low。
  final String alarmLevel;
  /// 设备 ID。
  final int deviceId;
  /// 设备编号。
  final String deviceCode;
  /// 设备名称。
  final String deviceName;
  /// 区域名称。
  final String regionName;
  /// 设备位置。
  final String location;
  /// 告警内容。
  final String content;
  /// 告警时间。
  final String? occurredDT;
  /// 是否已读。
  final bool? read;
  /// 处理状态 pending/handled。
  final String status;
  /// 处理结果。
  final String handleResult;
  /// 处理图片。
  final String handleImage;
  /// 处理人。
  final String handler;
  /// 处理时间。
  final String? handledDT;

  /// 是否已处理。
  bool get isHandled => status == 'handled';

  /// 是否待处理。
  bool get isPending => status == 'pending';
}

// ════════════════════════════════════════════════════════════════
// 巡检
// ════════════════════════════════════════════════════════════════

/// 巡检任务。
class InspectTask {
  /// 构造巡检任务。
  const InspectTask({
    required this.id,
    required this.name,
    required this.regionId,
    required this.regionName,
    required this.location,
    required this.inspectorId,
    required this.content,
    required this.planDT,
    required this.status,
    required this.read,
    required this.remark,
  });

  /// 从 JSON 构造。
  factory InspectTask.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> region = defMap(json['region']);
    return InspectTask(
      id: defInt(json['id']),
      name: defStr(json['name']),
      regionId: defIntOrNull(json['region_id']),
      regionName: defStr(region['name']),
      location: defStr(json['location']),
      inspectorId: defInt(json['inspector_id']),
      content: defStr(json['content']),
      planDT: defStrOrNull(json['plan_dt']),
      status: defStr(json['status']),
      read: defBoolOrNull(json['read']),
      remark: defStr(json['remark']),
    );
  }

  /// 任务 ID。
  final int id;
  /// 任务名称。
  final String name;
  /// 所属区域 ID。
  final int? regionId;
  /// 所属区域名称。
  final String regionName;
  /// 巡检位置。
  final String location;
  /// 巡检人用户 ID。
  final int inspectorId;
  /// 巡检内容。
  final String content;
  /// 巡检时间。
  final String? planDT;
  /// 状态 pending/published/finished/canceled。
  final String status;
  /// 是否已读。
  final bool? read;
  /// 备注。
  final String remark;

  /// 是否已完成。
  bool get isFinished => status == 'finished';

  /// 是否待巡检（已发布未完成）。
  bool get isPublished => status == 'published';

  /// 状态中文文案。
  String get statusText {
    switch (status) {
      case 'pending':
        return '待发布';
      case 'published':
        return '待巡检';
      case 'finished':
        return '已完成';
      case 'canceled':
        return '已取消';
      default:
        return status.isEmpty ? '--' : status;
    }
  }
}

// ════════════════════════════════════════════════════════════════
// 通知 / 待办 / 首页
// ════════════════════════════════════════════════════════════════

/// 系统通知。
class NotificationItem {
  /// 构造通知。
  const NotificationItem({
    required this.id,
    required this.userId,
    required this.category,
    required this.title,
    required this.content,
    required this.refType,
    required this.refId,
    required this.read,
    required this.readDT,
    required this.createdDT,
  });

  /// 从 JSON 构造。
  factory NotificationItem.fromJson(Map<String, dynamic> json) => NotificationItem(
        id: defInt(json['id']),
        userId: defInt(json['user_id']),
        category: defStr(json['category']),
        title: defStr(json['title']),
        content: defStr(json['content']),
        refType: defStr(json['ref_type']),
        refId: defInt(json['ref_id']),
        read: defBoolOrNull(json['read']),
        readDT: defStrOrNull(json['read_dt']),
        createdDT: defStrOrNull(json['created_dt']),
      );

  /// 通知 ID。
  final int id;
  /// 接收人用户 ID。
  final int userId;
  /// 类别 alarm/risk/inspect。
  final String category;
  /// 标题。
  final String title;
  /// 内容。
  final String content;
  /// 关联业务类型。
  final String refType;
  /// 关联业务 ID。
  final int refId;
  /// 是否已读。
  final bool? read;
  /// 已读时间。
  final String? readDT;
  /// 创建时间。
  final String? createdDT;

  /// 是否已读。
  bool get isRead => defBool(read);

  /// 类别中文文案。
  String get categoryText {
    switch (category) {
      case 'alarm':
        return '设备告警';
      case 'risk':
        return 'AI风险';
      case 'inspect':
        return '巡检通知';
      default:
        return category.isEmpty ? '通知' : category;
    }
  }
}

/// 设备在线情况统计。
class AppDeviceStat {
  /// 构造设备统计。
  const AppDeviceStat({
    required this.total,
    required this.online,
    required this.offline,
  });

  /// 从 JSON 构造。
  factory AppDeviceStat.fromJson(Map<String, dynamic> json) => AppDeviceStat(
        total: defInt(json['total']),
        online: defInt(json['online']),
        offline: defInt(json['offline']),
      );

  /// 设备总数。
  final int total;
  /// 在线设备数。
  final int online;
  /// 离线设备数。
  final int offline;

  /// 在线率（0~1）。
  double get onlineRatio => total <= 0 ? 0 : (online / total).clamp(0.0, 1.0);
}

/// 首页仪表盘数据。
class AppHome {
  /// 构造首页数据。
  const AppHome({
    required this.pendingRiskCount,
    required this.pendingAlarmCount,
    required this.pendingInspectCount,
    required this.todayRiskCount,
    required this.todayAlarmCount,
    required this.deviceStat,
    required this.recentRisks,
    required this.unreadCount,
  });

  /// 从 JSON 构造。
  factory AppHome.fromJson(Map<String, dynamic> json) => AppHome(
        pendingRiskCount: defInt(json['pending_risk_count']),
        pendingAlarmCount: defInt(json['pending_alarm_count']),
        pendingInspectCount: defInt(json['pending_inspect_count']),
        todayRiskCount: defInt(json['today_risk_count']),
        todayAlarmCount: defInt(json['today_alarm_count']),
        deviceStat: AppDeviceStat.fromJson(defMap(json['device_stat'])),
        recentRisks: defMapList(json['recent_risks'])
            .map(RiskRecord.fromJson)
            .toList(growable: false),
        unreadCount: defInt(json['unread_count']),
      );

  /// 空首页数据（加载失败时的兜底）。
  static AppHome get empty => const AppHome(
        pendingRiskCount: 0,
        pendingAlarmCount: 0,
        pendingInspectCount: 0,
        todayRiskCount: 0,
        todayAlarmCount: 0,
        deviceStat: AppDeviceStat(total: 0, online: 0, offline: 0),
        recentRisks: <RiskRecord>[],
        unreadCount: 0,
      );

  /// 待处理风险数。
  final int pendingRiskCount;
  /// 待处理告警数。
  final int pendingAlarmCount;
  /// 我的待巡检任务数。
  final int pendingInspectCount;
  /// 今日新增风险数。
  final int todayRiskCount;
  /// 今日新增告警数。
  final int todayAlarmCount;
  /// 设备在线统计。
  final AppDeviceStat deviceStat;
  /// 最新 5 条风险。
  final List<RiskRecord> recentRisks;
  /// 未读通知数。
  final int unreadCount;
}

/// 待办项。
class TodoItem {
  /// 构造待办项。
  const TodoItem({
    required this.category,
    required this.title,
    required this.count,
    required this.refId,
  });

  /// 从 JSON 构造。
  factory TodoItem.fromJson(Map<String, dynamic> json) => TodoItem(
        category: defStr(json['category']),
        title: defStr(json['title']),
        count: defInt(json['count']),
        refId: defInt(json['ref_id']),
      );

  /// 类别 risk/alarm/inspect/notification。
  final String category;
  /// 标题。
  final String title;
  /// 数量。
  final int count;
  /// 关联业务 ID（列表类为 0）。
  final int refId;

  /// 类别对应的路由。
  String get route {
    switch (category) {
      case 'risk':
        return '/risks';
      case 'alarm':
        return '/alarms';
      case 'inspect':
        return '/inspect-tasks';
      case 'notification':
        return '/notifications';
      default:
        return '/todos';
    }
  }

  /// 无数据时的兜底标题。
  String get displayTitle => title.isEmpty ? categoryText : title;

  /// 类别中文文案。
  String get categoryText {
    switch (category) {
      case 'risk':
        return '待处理风险';
      case 'alarm':
        return '待处理告警';
      case 'inspect':
        return '待巡检任务';
      case 'notification':
        return '未读通知';
      default:
        return '待办';
    }
  }
}

// ════════════════════════════════════════════════════════════════
// 用户 / 菜单
// ════════════════════════════════════════════════════════════════

/// 用户资料。
class UserProfile {
  /// 构造用户资料。
  const UserProfile({
    required this.id,
    required this.name,
    required this.userName,
    required this.email,
    required this.phone,
    required this.avatar,
    required this.deptId,
    required this.deptName,
    required this.postId,
    required this.postName,
    required this.isAdmin,
    required this.active,
  });

  /// 从 JSON 构造。
  factory UserProfile.fromJson(Map<String, dynamic> json) {
    final Map<String, dynamic> dept = defMap(json['dept']);
    final Map<String, dynamic> post = defMap(json['post']);
    return UserProfile(
      id: defInt(json['id']),
      name: defStr(json['name']),
      userName: defStr(json['user_name']),
      email: defStr(json['email']),
      phone: defStr(json['phone']),
      avatar: defStr(json['avatar']),
      deptId: defIntOrNull(json['dept_id']),
      deptName: defStr(dept['name']),
      postId: defIntOrNull(json['post_id']),
      postName: defStr(post['name']),
      isAdmin: defBool(json['is_admin']),
      active: defBool(json['active'], fallback: true),
    );
  }

  /// 用户 ID。
  final int id;
  /// 姓名。
  final String name;
  /// 登录名。
  final String userName;
  /// 邮箱。
  final String email;
  /// 手机号。
  final String phone;
  /// 头像地址。
  final String avatar;
  /// 部门 ID。
  final int? deptId;
  /// 部门名称。
  final String deptName;
  /// 岗位 ID。
  final int? postId;
  /// 岗位名称。
  final String postName;
  /// 是否超级管理员。
  final bool isAdmin;
  /// 是否启用。
  final bool active;

  /// 显示名（姓名优先，回退登录名）。
  String get displayName => name.isNotEmpty ? name : userName;
}

/// 移动端菜单节点（对应 `model.Menu`，children 为嵌套子菜单）。
class MenuNode {
  /// 构造菜单节点。
  const MenuNode({
    required this.id,
    required this.name,
    required this.parentId,
    required this.level,
    required this.path,
    required this.icon,
    required this.component,
    required this.type,
    required this.platform,
    required this.permissionCode,
    required this.hidden,
    required this.children,
  });

  /// 从 JSON 构造（递归解析 children）。
  factory MenuNode.fromJson(Map<String, dynamic> json) => MenuNode(
        id: defInt(json['id']),
        name: defStr(json['name']),
        parentId: defIntOrNull(json['parent_id']),
        level: defInt(json['level']),
        path: defStr(json['path']),
        icon: defStr(json['icon']),
        component: defStr(json['component']),
        type: defStr(json['type']),
        platform: defStr(json['platform']),
        permissionCode: defStr(json['permission_code']),
        hidden: defBool(json['hidden']),
        children: defMapList(json['children'])
            .map(MenuNode.fromJson)
            .toList(growable: false),
      );

  /// 菜单 ID。
  final int id;
  /// 菜单名称。
  final String name;
  /// 父菜单 ID。
  final int? parentId;
  /// 层级。
  final int level;
  /// 路由路径。
  final String path;
  /// 图标名。
  final String icon;
  /// 前端组件。
  final String component;
  /// 类型 directory/menu/button。
  final String type;
  /// 平台 pc/mobile。
  final String platform;
  /// 权限标识（按钮级）。
  final String permissionCode;
  /// 是否隐藏。
  final bool hidden;
  /// 子菜单。
  final List<MenuNode> children;

  /// 是否为按钮权限节点。
  bool get isButton => type == 'button';

  /// 是否为目录。
  bool get isDirectory => type == 'directory';

  /// 是否有子节点。
  bool get hasChildren => children.isNotEmpty;

  /// 深度优先展开为扁平的权限标识列表。
  List<String> get permissionCodes {
    final List<String> out = <String>[];
    if (permissionCode.isNotEmpty) out.add(permissionCode);
    for (final MenuNode c in children) {
      out.addAll(c.permissionCodes);
    }
    return out;
  }
}

/// 登录响应载荷（对应 `response.LoginData`）。
class LoginData {
  /// 构造登录载荷。
  const LoginData({
    required this.token,
    required this.userId,
    required this.name,
    required this.deptId,
    required this.dataPerm,
    required this.crossDeptIds,
    required this.permissions,
  });

  /// 从 JSON 构造。
  factory LoginData.fromJson(Map<String, dynamic> json) => LoginData(
        token: defStr(json['token']),
        userId: defInt(json['user_id']),
        name: defStr(json['name']),
        deptId: defIntOrNull(json['dept_id']),
        dataPerm: defStr(json['data_perm']),
        crossDeptIds: defStr(json['cross_dept_ids']),
        permissions: defStrList(json['permissions']),
      );

  /// JWT。
  final String token;
  /// 用户 ID。
  final int userId;
  /// 姓名。
  final String name;
  /// 部门 ID。
  final int? deptId;
  /// 数据权限标识 all/cross_dept/dept/personal。
  final String dataPerm;
  /// 跨部门数据权限-部门 ID 列表（逗号分隔）。
  final String crossDeptIds;
  /// 权限标识列表。
  final List<String> permissions;
}
