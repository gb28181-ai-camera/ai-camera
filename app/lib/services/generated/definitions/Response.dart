/// 后端统一响应包装 `{code, message, data}`。
///
/// 命名与后端 `internal/dto/response` 的类型一一对应：
/// `RXxxList` → [RListResponse]，`RPageXxx` → [RPageResponse]，`RXxx` → [RSingleResponse]。
library;

import 'Definition.dart';

/// 通用响应基类（对应 `response.BaseResponse`）。
///
/// 只承载 `code` / `message` / 原始 `data`；业务代码应使用下面的强类型子类，
/// 不要直接读取 [data]。
class Response {
  /// 构造响应。
  Response();

  /// 业务状态码，200 表示成功。
  int code = 0;

  /// 提示信息。
  String? message;

  /// 原始响应载荷。
  dynamic data;

  /// 从 JSON 填充。
  Response fromJson(dynamic json) {
    final Map<String, dynamic> m = defMap(json);
    code = defInt(m['code']);
    message = defStrOrNull(m['message']);
    data = m['data'];
    return this;
  }

  /// 是否业务成功。
  bool get isSuccess => code == 200;

  /// 是否为未授权（token 失效）。
  bool get isUnauthorized => code == 401;

  /// 错误文案（成功时为空）。
  String get errorMessage => message.orEmptyOr('请求失败');

  /// 空响应（本地兜底用）。
  static Response empty() => Response()
    ..code = 200
    ..message = null
    ..data = null;
}

/// 小工具：null / 空串时取兜底文案。
extension _FallbackString on String? {
  /// 空串或 null 时返回 [fallback]。
  String orEmptyOr(String fallback) {
    final String? s = this;
    return (s == null || s.isEmpty) ? fallback : s;
  }
}

/// 单条详情响应：`data` 为对象。
class RSingleResponse<T extends Object> extends Response {
  /// 构造单条响应。
  RSingleResponse(this._parse);

  final T Function(Map<String, dynamic>) _parse;

  T? _value;

  /// 解析后的对象；`data` 为 null 时返回 null。
  T? get value {
    if (_value != null) return _value;
    final dynamic d = data;
    if (d == null) return null;
    _value = _parse(defMap(d));
    return _value;
  }
}

/// 简单列表响应：`data` 为数组（对应 `RXxxList`）。
class RListResponse<T extends Object> extends Response {
  /// 构造列表响应。
  RListResponse(this._parse);

  final T Function(Map<String, dynamic>) _parse;

  List<T>? _items;

  /// 解析后的列表；`data` 非数组时返回空列表。
  List<T> get items {
    if (_items != null) return _items!;
    final dynamic d = data;
    if (d is! List) {
      _items = <T>[];
      return _items!;
    }
    _items = defMapList(d).map(_parse).toList(growable: false);
    return _items!;
  }

  /// 是否为空列表。
  bool get isEmpty => items.isEmpty;
}

/// 分页响应：`data` 为 `{total, page, page_size, items}`（对应 `RPageXxx`）。
class RPageResponse<T extends Object> extends Response {
  /// 构造分页响应。
  RPageResponse(this._parse);

  final T Function(Map<String, dynamic>) _parse;

  PageData<T>? _page;

  /// 解析后的分页载荷；`data` 为 null 时返回空页。
  PageData<T> get page {
    if (_page != null) return _page!;
    final dynamic d = data;
    if (d == null) {
      _page = PageData.empty<T>(_parse);
      return _page!;
    }
    _page = PageData<T>.fromJson(d, _parse);
    return _page!;
  }

  /// 当前页数据。
  List<T> get items => page.items;

  /// 总条数。
  int get total => page.total;
}

/// 无载荷响应（对应 `response.BaseResponse`，写操作使用）。
class REmptyResponse extends Response {
  /// 是否操作成功。
  bool get ok => isSuccess;
}

// ════════════════════════════════════════════════════════════════
// 具体端点响应类型
// ════════════════════════════════════════════════════════════════

/// `POST /app/auth/login` → `response.RLogin`。
class RLogin extends RSingleResponse<LoginData> {
  /// 构造登录响应。
  RLogin() : super(LoginData.fromJson);

  /// 登录载荷。
  LoginData? get loginData => value;
}

/// `GET /app/menus` → `response.RAppMenuList`。
class RAppMenuList extends RListResponse<MenuNode> {
  /// 构造菜单响应。
  RAppMenuList() : super(MenuNode.fromJson);

  /// 菜单树根节点。
  List<MenuNode> get menus => items;
}

/// `GET /app/home` → `response.RAppHome`。
class RAppHome extends RSingleResponse<AppHome> {
  /// 构造首页响应。
  RAppHome() : super(AppHome.fromJson);

  /// 首页数据，解析失败时返回空数据。
  AppHome get home => value ?? AppHome.empty;
}

/// `GET /app/todos` → `response.RAppTodoList`。
class RAppTodoList extends RListResponse<TodoItem> {
  /// 构造待办响应。
  RAppTodoList() : super(TodoItem.fromJson);

  /// 待办项列表。
  List<TodoItem> get todos => items;
}

/// `GET /app/risks` → `response.RPageRiskRecord`。
class RPageRiskRecord extends RPageResponse<RiskRecord> {
  /// 构造风险分页响应。
  RPageRiskRecord() : super(RiskRecord.fromJson);
}

/// `GET /app/risks/{id}` → `response.RRiskRecord`。
class RRiskRecord extends RSingleResponse<RiskRecord> {
  /// 构造风险详情响应。
  RRiskRecord() : super(RiskRecord.fromJson);
}

/// `GET /app/alarms` → `response.RPageAlarmRecord`。
class RPageAlarmRecord extends RPageResponse<AlarmRecord> {
  /// 构造告警分页响应。
  RPageAlarmRecord() : super(AlarmRecord.fromJson);
}

/// `GET /app/inspect-tasks` → `response.RPageInspectTask`。
class RPageInspectTask extends RPageResponse<InspectTask> {
  /// 构造巡检任务分页响应。
  RPageInspectTask() : super(InspectTask.fromJson);
}

/// `GET /app/inspect-tasks/{id}` → `response.RInspectTask`。
class RInspectTask extends RSingleResponse<InspectTask> {
  /// 构造巡检任务详情响应。
  RInspectTask() : super(InspectTask.fromJson);
}

/// `GET /app/devices` → `response.RPageDevice`。
class RPageDevice extends RPageResponse<Device> {
  /// 构造设备分页响应。
  RPageDevice() : super(Device.fromJson);
}

/// `GET /app/devices/{id}` → `response.RDevice`。
class RDevice extends RSingleResponse<Device> {
  /// 构造设备详情响应。
  RDevice() : super(Device.fromJson);
}

/// `GET /app/notifications` → `response.RPageNotification`。
class RPageNotification extends RPageResponse<NotificationItem> {
  /// 构造通知分页响应。
  RPageNotification() : super(NotificationItem.fromJson);
}

/// `GET /app/profile` → `response.RUser`。
class RUser extends RSingleResponse<UserProfile> {
  /// 构造用户资料响应。
  RUser() : super(UserProfile.fromJson);
}
