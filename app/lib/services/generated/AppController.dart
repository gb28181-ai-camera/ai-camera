/// 移动端业务接口（Swagger Tag: App）。
///
/// 覆盖契约 `docs/开发文档/后端开发/00-接口契约.md` §3.14 的全部 `/app/*` 端点。
/// 本文件为生成的 Controller 风格，**页面中禁止手写 URL**，必须调用这里的静态方法。
///
/// 所有方法均返回强类型响应（`RAppHome` / `RPageRiskRecord` / ...），
/// 调用方不得绕过类型直接访问 `data`。
library;

import 'package:dio/dio.dart' as dio;

import '../api_service.dart';
import 'definitions/LoginReq.dart';
import 'definitions/Response.dart';

/// 移动端业务控制器。
class AppController {
  AppController._();

  // ── 路径常量 ──────────────────────────────────────────────
  /// 菜单树。
  static String menusUsingGetPath = '/app/menus';
  /// 首页仪表盘。
  static String homeUsingGetPath = '/app/home';
  /// 待办汇总。
  static String todosUsingGetPath = '/app/todos';
  /// 风险列表。
  static String risksUsingGetPath = '/app/risks';
  /// 风险详情（拼接 ID）。
  static String riskDetailUsingGetPath = '/app/risks';
  /// 风险处理（拼接 ID）。
  static String handleRiskUsingPutPath = '/app/risks';
  /// 检测准确性确认（拼接 ID）。
  static String updateRiskAccurateUsingPutPath = '/app/risks';
  /// 告警列表。
  static String alarmsUsingGetPath = '/app/alarms';
  /// 告警处理（拼接 ID）。
  static String handleAlarmUsingPutPath = '/app/alarms';
  /// 我的巡检任务列表。
  static String inspectTasksUsingGetPath = '/app/inspect-tasks';
  /// 巡检任务详情（拼接 ID）。
  static String inspectTaskDetailUsingGetPath = '/app/inspect-tasks';
  /// 完成巡检任务（拼接 ID）。
  static String finishInspectTaskUsingPutPath = '/app/inspect-tasks';
  /// 设备列表。
  static String devicesUsingGetPath = '/app/devices';
  /// 设备详情（拼接 ID）。
  static String deviceDetailUsingGetPath = '/app/devices';
  /// 我的通知列表。
  static String notificationsUsingGetPath = '/app/notifications';
  /// 标记通知已读（拼接 ID）。
  static String markNotificationReadUsingPutPath = '/app/notifications';
  /// 个人资料。
  static String profileUsingGetPath = '/app/profile';
  /// 更新个人资料。
  static String updateProfileUsingPutPath = '/app/profile';
  /// 修改密码。
  static String changePasswordUsingPutPath = '/app/profile/password';

  // ══════════════════════════════════════════════════════════
  // 菜单 / 首页 / 待办
  // ══════════════════════════════════════════════════════════

  /// 获取移动端菜单树。
  ///
  /// 仅返回平台为 mobile 且启用、未隐藏的菜单，按当前用户三路角色授权过滤后组树。
  static Future<RAppMenuList> menusUsingGet([
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      menusUsingGetPath,
      headers: headers,
    );
    return RAppMenuList()..fromJson(response.data);
  }

  /// 获取移动端首页仪表盘数据。
  static Future<RAppHome> homeUsingGet([
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      homeUsingGetPath,
      headers: headers,
    );
    return RAppHome()..fromJson(response.data);
  }

  /// 获取移动端待办汇总（风险 / 告警 / 巡检 / 通知 四类）。
  static Future<RAppTodoList> todosUsingGet([
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      todosUsingGetPath,
      headers: headers,
    );
    return RAppTodoList()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 风险
  // ══════════════════════════════════════════════════════════

  /// 分页查询风险监控。
  ///
  /// 支持参数：`page`、`page_size`、`keyword`、`risk_type_id`、`region_id`、
  /// `device_id`、`device_code`、`status`、`detect_accurate`、`read`、
  /// `start_dt`、`end_dt`。
  static Future<RPageRiskRecord> risksUsingGet({
    Map<String, dynamic>? query,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      risksUsingGetPath,
      query: query,
      headers: headers,
    );
    return RPageRiskRecord()..fromJson(response.data);
  }

  /// 获取风险详情。
  static Future<RRiskRecord> riskDetailUsingGet(
    int id, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      '$riskDetailUsingGetPath/$id',
      headers: headers,
    );
    return RRiskRecord()..fromJson(response.data);
  }

  /// 处理风险（提交处理结果与处理图片）。
  static Future<REmptyResponse> handleRiskUsingPut(
    int id,
    HandleReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.put(
      '$handleRiskUsingPutPath/$id/handle',
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  /// 确认风险检测准确性（yes=确认风险，no=误报，直接置为已处理）。
  static Future<REmptyResponse> updateRiskAccurateUsingPut(
    int id,
    UpdateAccurateReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.put(
      '$updateRiskAccurateUsingPutPath/$id/accurate',
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 告警
  // ══════════════════════════════════════════════════════════

  /// 分页查询设备告警。
  ///
  /// 支持参数：`page`、`page_size`、`keyword`、`alarm_type_id`、`region_id`、
  /// `device_id`、`device_code`、`status`、`read`、`start_dt`、`end_dt`。
  static Future<RPageAlarmRecord> alarmsUsingGet({
    Map<String, dynamic>? query,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      alarmsUsingGetPath,
      query: query,
      headers: headers,
    );
    return RPageAlarmRecord()..fromJson(response.data);
  }

  /// 处理设备告警。
  static Future<REmptyResponse> handleAlarmUsingPut(
    int id,
    HandleReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.put(
      '$handleAlarmUsingPutPath/$id/handle',
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 巡检
  // ══════════════════════════════════════════════════════════

  /// 分页查询我的巡检任务（仅返回巡检人为当前登录用户的任务）。
  ///
  /// 支持参数：`page`、`page_size`、`keyword`、`region_id`、`status`、
  /// `start_dt`、`end_dt`。
  static Future<RPageInspectTask> inspectTasksUsingGet({
    Map<String, dynamic>? query,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      inspectTasksUsingGetPath,
      query: query,
      headers: headers,
    );
    return RPageInspectTask()..fromJson(response.data);
  }

  /// 获取巡检任务详情。
  static Future<RInspectTask> inspectTaskDetailUsingGet(
    int id, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      '$inspectTaskDetailUsingGetPath/$id',
      headers: headers,
    );
    return RInspectTask()..fromJson(response.data);
  }

  /// 完成巡检任务（提交巡检结果与结果图）。
  static Future<REmptyResponse> finishInspectTaskUsingPut(
    int id,
    FinishInspectTaskReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.put(
      '$finishInspectTaskUsingPutPath/$id/finish',
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 设备
  // ══════════════════════════════════════════════════════════

  /// 分页查询设备。
  ///
  /// 支持参数：`page`、`page_size`、`keyword`、`region_id`、`status`、
  /// `device_mode`、`work_mode`。
  static Future<RPageDevice> devicesUsingGet({
    Map<String, dynamic>? query,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      devicesUsingGetPath,
      query: query,
      headers: headers,
    );
    return RPageDevice()..fromJson(response.data);
  }

  /// 获取设备详情。
  static Future<RDevice> deviceDetailUsingGet(
    int id, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      '$deviceDetailUsingGetPath/$id',
      headers: headers,
    );
    return RDevice()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 通知
  // ══════════════════════════════════════════════════════════

  /// 分页查询我的通知。
  ///
  /// 支持参数：`page`、`page_size`、`category`（alarm/risk/inspect）、`read`。
  static Future<RPageNotification> notificationsUsingGet({
    Map<String, dynamic>? query,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      notificationsUsingGetPath,
      query: query,
      headers: headers,
    );
    return RPageNotification()..fromJson(response.data);
  }

  /// 标记通知已读（仅允许标记当前登录用户自己的通知）。
  static Future<REmptyResponse> markNotificationReadUsingPut(
    int id, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.put(
      '$markNotificationReadUsingPutPath/$id/read',
      <String, dynamic>{},
      null,
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 个人中心
  // ══════════════════════════════════════════════════════════

  /// 获取当前用户资料。
  static Future<RUser> profileUsingGet([
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.get(
      profileUsingGetPath,
      headers: headers,
    );
    return RUser()..fromJson(response.data);
  }

  /// 更新当前用户资料。
  static Future<REmptyResponse> updateProfileUsingPut(
    UpdateProfileReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.put(
      updateProfileUsingPutPath,
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  /// 修改当前用户密码。
  static Future<REmptyResponse> changePasswordUsingPut(
    ChangePasswordReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.put(
      changePasswordUsingPutPath,
      <String, dynamic>{},
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }
}
