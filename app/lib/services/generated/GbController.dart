/// 移动端国标视频（GB/T 28181）接口。
///
/// 覆盖契约 `docs/开发文档/后端开发/00-接口契约.md` §10.12 的全部 13 个 `/app/gb/*` 端点。
/// 风格与 `AppController.dart` 保持一致：`static const` 路径常量 + `static Future<T>` 方法
/// + 统一经 `ApiService` 发请求。**页面中禁止手写 URL**，必须调用这里的静态方法。
///
/// 错误处理复用 `ApiService`：请求不抛异常，失败时返回 `code != 200` 的响应，
/// 由调用方读取 `errorMessage`（后端返回的中文提示 / 网络异常的中文映射）。
library;

import 'package:dio/dio.dart' as dio;

import '../api_service.dart';
import 'definitions/AppGb.dart';
import 'definitions/Response.dart';

/// 移动端国标视频控制器。
class GbController {
  GbController._();

  // ── 路径常量（与 router/app.go 的 aAuth.Group("/gb") 严格对应） ──
  /// 国标设备分页。
  static const String devicesUsingGetPath = '/app/gb/devices';
  /// 国标通道分页。
  static const String channelsUsingGetPath = '/app/gb/channels';
  /// 实时点播（拼接通道 ID）。
  static const String playUsingPostPath = '/app/gb/play';
  /// 停止实时点播（拼接通道 ID）。
  static const String stopPlayUsingPostPath = '/app/gb/play';
  /// 抓图（拼接通道 ID）。
  static const String snapshotUsingPostPath = '/app/gb/play';
  /// 云台控制（拼接通道 ID）。
  static const String ptzUsingPostPath = '/app/gb/ptz';
  /// 录像检索。
  static const String recordsUsingGetPath = '/app/gb/playback/records';
  /// 开始回放。
  static const String startPlaybackUsingPostPath = '/app/gb/playback';
  /// 回放控制。
  static const String controlPlaybackUsingPostPath = '/app/gb/playback/control';
  /// 停止回放。
  static const String stopPlaybackUsingPostPath = '/app/gb/playback/stop';
  /// 开始语音对讲（拼接通道 ID）。
  static const String startTalkUsingPostPath = '/app/gb/talk';
  /// 结束语音对讲（拼接通道 ID）。
  static const String stopTalkUsingPostPath = '/app/gb/talk';
  /// 国标报警分页。
  static const String alarmsUsingGetPath = '/app/gb/alarms';

  // ══════════════════════════════════════════════════════════
  // 设备 / 通道
  // ══════════════════════════════════════════════════════════

  /// 分页查询国标设备。
  ///
  /// 只返回已配置国标接入的设备；不含 SIP 密码等接入配置。
  static Future<RAppGbDevicePage> devicesUsingGet({
    int page = 1,
    int pageSize = 20,
    String? keyword,
    String? registerStatus,
    String? status,
    int? regionId,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      devicesUsingGetPath,
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        'keyword': keyword,
        'register_status': registerStatus,
        'status': status,
        'region_id': regionId,
      },
      headers: headers,
    );
    return RAppGbDevicePage()..fromJson(response.data);
  }

  /// 分页查询国标通道。
  ///
  /// [deviceId] 用于「设备列表内联展开通道」场景；频道多时调大 [pageSize]。
  static Future<RAppGbChannelPage> channelsUsingGet({
    int page = 1,
    int pageSize = 20,
    String? keyword,
    int? deviceId,
    int? regionId,
    int? groupId,
    String? status,
    String? gbDeviceId,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      channelsUsingGetPath,
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        'keyword': keyword,
        'device_id': deviceId,
        'region_id': regionId,
        'group_id': groupId,
        'status': status,
        'gb_device_id': gbDeviceId,
      },
      headers: headers,
    );
    return RAppGbChannelPage()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 实时播放 / 抓图
  // ══════════════════════════════════════════════════════════

  /// 发起实时点播。
  ///
  /// 后端返回 WebRTC / HTTP-FLV / HLS 三种地址；App 按 `player_spike_page.dart`
  /// 的实测结论优先 WebRTC，失败再回退 HTTP-FLV。
  static Future<RAppGbPlay> playUsingPost(
    int channelId, {
    GbPlayReq? data,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.post(
      '$playUsingPostPath/$channelId',
      null,
      (data ?? const GbPlayReq()).toJson(),
      headers,
    );
    return RAppGbPlay()..fromJson(response.data);
  }

  /// 停止实时点播。
  ///
  /// [streamId] 可省略：省略时后端按通道停止进行中的会话。
  static Future<REmptyResponse> stopPlayUsingPost(
    int channelId, {
    String? streamId,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.post(
      '$stopPlayUsingPostPath/$channelId/stop',
      null,
      (streamId == null || streamId.isEmpty)
          ? null
          : <String, dynamic>{'stream_id': streamId},
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  /// 通道抓图（未开播时后端会自动先点播）。
  static Future<RAppGbSnapshot> snapshotUsingPost(
    int channelId, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      '$snapshotUsingPostPath/$channelId/snapshot',
      null,
      null,
      headers,
    );
    return RAppGbSnapshot()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 云台
  // ══════════════════════════════════════════════════════════

  /// 云台控制（八方向/停止 + 变焦等动作）。
  ///
  /// 注意 GB28181 的语义是「开始运动」「停止运动」两个独立指令：
  /// UI 必须按住下发、松开立即下发 `stop`，否则镜头会一直转。
  static Future<REmptyResponse> ptzUsingPost(
    int channelId,
    GbPtzReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      '$ptzUsingPostPath/$channelId',
      null,
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 录像查询 / 回放
  // ══════════════════════════════════════════════════════════

  /// 查询设备侧录像。
  ///
  /// 时间格式固定为 `2006-01-02T15:04:05`（GB28181 XML 原样解析）。
  /// 设备在等待窗口内没回齐时返回 `timeout=true`，而不是 HTTP 错误。
  static Future<RAppGbRecordQuery> recordsUsingGet({
    required int channelId,
    required String startTime,
    required String endTime,
    String? type,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      recordsUsingGetPath,
      query: <String, dynamic>{
        'channel_id': channelId,
        'start_time': startTime,
        'end_time': endTime,
        'type': type,
      },
      headers: headers,
    );
    return RAppGbRecordQuery()..fromJson(response.data);
  }

  /// 开始录像回放。
  static Future<RAppGbPlayback> startPlaybackUsingPost(
    GbPlaybackReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      startPlaybackUsingPostPath,
      null,
      data.toJson(),
      headers,
    );
    return RAppGbPlayback()..fromJson(response.data);
  }

  /// 回放控制（pause / resume / seek / speed）。
  static Future<REmptyResponse> controlPlaybackUsingPost(
    GbPlaybackControlReq data, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      controlPlaybackUsingPostPath,
      null,
      data.toJson(),
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  /// 停止录像回放。
  static Future<REmptyResponse> stopPlaybackUsingPost(
    String streamId, [
    Map<String, dynamic>? headers,
  ]) async {
    final dio.Response<dynamic> response = await ApiService.post(
      stopPlaybackUsingPostPath,
      null,
      <String, dynamic>{'stream_id': streamId},
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 语音对讲
  // ══════════════════════════════════════════════════════════

  /// 开始语音对讲。
  ///
  /// 返回的 `ssrc` 是 App 侧回推音频的目标 SSRC；当前 App 只建立/结束会话，
  /// 音频采集与 RTP 回推尚未接入（需真机验证，见 README）。
  static Future<RAppGbVoiceSession> startTalkUsingPost(
    int channelId, {
    GbTalkStartReq? data,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.post(
      '$startTalkUsingPostPath/$channelId/start',
      null,
      (data ?? const GbTalkStartReq()).toJson(),
      headers,
    );
    return RAppGbVoiceSession()..fromJson(response.data);
  }

  /// 结束语音对讲；[sessionId] 可省略（后端按通道找进行中的会话）。
  static Future<REmptyResponse> stopTalkUsingPost(
    int channelId, {
    String? sessionId,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.post(
      '$stopTalkUsingPostPath/$channelId/stop',
      null,
      (sessionId == null || sessionId.isEmpty)
          ? null
          : <String, dynamic>{'session_id': sessionId},
      headers,
    );
    return REmptyResponse()..fromJson(response.data);
  }

  // ══════════════════════════════════════════════════════════
  // 国标报警
  // ══════════════════════════════════════════════════════════

  /// 分页查询国标报警。
  static Future<RAppGbAlarmPage> alarmsUsingGet({
    int page = 1,
    int pageSize = 20,
    String? keyword,
    int? deviceId,
    int? channelId,
    String? alarmType,
    String? alarmPriority,
    bool? handled,
    String? startDt,
    String? endDt,
    Map<String, dynamic>? headers,
  }) async {
    final dio.Response<dynamic> response = await ApiService.get(
      alarmsUsingGetPath,
      query: <String, dynamic>{
        'page': page,
        'page_size': pageSize,
        'keyword': keyword,
        'device_id': deviceId,
        'channel_id': channelId,
        'alarm_type': alarmType,
        'alarm_priority': alarmPriority,
        'handled': handled,
        'start_dt': startDt,
        'end_dt': endDt,
      },
      headers: headers,
    );
    return RAppGbAlarmPage()..fromJson(response.data);
  }
}
