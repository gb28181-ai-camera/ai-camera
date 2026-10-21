// 国标视频 Controller 测试。
//
// 不依赖真实后端：用 PathRoutingAdapter 按路径返回预设 JSON，并逐条断言
// 13 个端点的 HTTP 方法、路径、查询参数与请求体 —— 这些正是最容易与后端
// 契约（§10.12 / §15.1 / §15.2）漂移的地方。
import 'dart:convert';

import 'package:ai_camera_app/services/api_service.dart';
import 'package:ai_camera_app/services/generated/GbController.dart';
import 'package:ai_camera_app/services/generated/definitions/AppGb.dart';
import 'package:ai_camera_app/services/generated/definitions/Response.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import '../test_dio_adapter.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    ApiService.setToken('tk-gb');
    ApiService.resetDio();
  });

  tearDown(() {
    ApiService.setToken(null);
    ApiService.resetDio();
  });

  test('13 个端点的路径、方法、参数与强类型解析', () async {
    final PathRoutingAdapter adapter = PathRoutingAdapter(<String, Object>{
      '/app/gb/devices': wrapSuccess(<String, dynamic>{
        'items': <dynamic>[
          <String, dynamic>{'id': 1, 'name': '东门球机', 'status': 'online'},
        ],
        'total': 1,
        'page': 1,
        'page_size': 20,
      }),
      '/app/gb/channels': wrapSuccess(<String, dynamic>{
        'items': <dynamic>[
          <String, dynamic>{'id': 21, 'name': '通道一', 'status': 'ON'},
        ],
        'total': 1,
        'page': 1,
        'page_size': 100,
      }),
      '/app/gb/play/7/stop': wrapSuccess(null),
      '/app/gb/play/7/snapshot': wrapSuccess(<String, dynamic>{
        'url': 'http://localhost:8080/uploads/a.jpg',
        'file_name': 'a.jpg',
        'file_size': 1024,
      }),
      '/app/gb/play/7': wrapSuccess(<String, dynamic>{
        'app': 'rtp',
        'stream': '3B9ACA01',
        'http_base': 'http://10.0.0.5:8082',
        'webrtc': 'http://10.0.0.5:8082/whep',
        'flv': 'http://10.0.0.5:8082/rtp/3B9ACA01.live.flv',
        'hls': 'http://10.0.0.5:8082/rtp/3B9ACA01/hls.m3u8',
        'stream_id': 'call-1',
      }),
      '/app/gb/ptz/7': wrapSuccess(null),
      '/app/gb/playback/records': wrapSuccess(<String, dynamic>{
        'channel_id': 7,
        'total': 1,
        'timeout': false,
        'items': <dynamic>[
          <String, dynamic>{'channel_id': 7, 'time_len': 60},
        ],
      }),
      '/app/gb/playback/control': wrapSuccess(null),
      '/app/gb/playback/stop': wrapSuccess(null),
      '/app/gb/playback': wrapSuccess(<String, dynamic>{
        'stream_id': 'pb-1',
        'channel_id': 7,
      }),
      '/app/gb/talk/7/start': wrapSuccess(<String, dynamic>{
        'session_id': 'talk-1',
        'ssrc': '0342000002',
      }),
      '/app/gb/talk/7/stop': wrapSuccess(null),
      '/app/gb/alarms': wrapSuccess(<String, dynamic>{
        'items': <dynamic>[
          <String, dynamic>{'id': 5, 'handled': false},
        ],
        'total': 1,
        'page': 2,
        'page_size': 10,
      }),
    });
    ApiService.dio = buildTestDio(adapter);

    // 1. 设备分页
    final RAppGbDevicePage devices =
        await GbController.devicesUsingGet(status: 'online');
    expect(devices.isSuccess, isTrue);
    expect(devices.items.single.name, '东门球机');
    expect(devices.total, 1);

    // 2. 通道分页
    final RAppGbChannelPage channels = await GbController.channelsUsingGet(
      deviceId: 12,
      pageSize: 100,
    );
    expect(channels.items.single.isOnline, isTrue);
    expect(channels.total, 1);

    // 3. 实时点播
    final RAppGbPlay play = await GbController.playUsingPost(7);
    expect(play.isSuccess, isTrue);
    expect(play.value!.stream, '3B9ACA01');
    expect(play.value!.streamId, 'call-1');
    expect(play.value!.fallbackUrl, contains('.live.flv'));

    // 4. 停止点播
    final REmptyResponse stop = await GbController.stopPlayUsingPost(
      7,
      streamId: 'call-1',
    );
    expect(stop.ok, isTrue);

    // 5. 抓图
    final RAppGbSnapshot snapshot = await GbController.snapshotUsingPost(7);
    expect(snapshot.value!.fileName, 'a.jpg');

    // 6. 云台
    final REmptyResponse ptz = await GbController.ptzUsingPost(
      7,
      const GbPtzReq(direction: GbPtzDirection.up, speed: 6),
    );
    expect(ptz.ok, isTrue);

    // 7. 录像检索
    final RAppGbRecordQuery records = await GbController.recordsUsingGet(
      channelId: 7,
      startTime: '2026-01-01T00:00:00',
      endTime: '2026-01-01T23:59:59',
    );
    expect(records.value!.items.single.timeLen, 60);
    expect(records.value!.timeout, isFalse);

    // 8. 开始回放
    final RAppGbPlayback playback =
        await GbController.startPlaybackUsingPost(
      const GbPlaybackReq(
        channelId: 7,
        startTime: '2026-01-01T08:00:00',
        endTime: '2026-01-01T09:00:00',
      ),
    );
    expect(playback.value!.streamId, 'pb-1');

    // 9. 回放控制
    final REmptyResponse control =
        await GbController.controlPlaybackUsingPost(
      const GbPlaybackControlReq(
        streamId: 'pb-1',
        action: 'speed',
        scale: 2,
      ),
    );
    expect(control.ok, isTrue);

    // 10. 停止回放
    final REmptyResponse stopPlayback =
        await GbController.stopPlaybackUsingPost('pb-1');
    expect(stopPlayback.ok, isTrue);

    // 11. 开始对讲
    final RAppGbVoiceSession talk =
        await GbController.startTalkUsingPost(7);
    expect(talk.value!.sessionId, 'talk-1');

    // 12. 结束对讲
    final REmptyResponse stopTalk = await GbController.stopTalkUsingPost(
      7,
      sessionId: 'talk-1',
    );
    expect(stopTalk.ok, isTrue);

    // 13. 国标报警
    final RAppGbAlarmPage alarms = await GbController.alarmsUsingGet(
      page: 2,
      pageSize: 10,
      handled: false,
    );
    expect(alarms.items.single.handled, isFalse);
    expect(alarms.total, 1);

    // ── 请求侧断言 ────────────────────────────────────────────
    expect(adapter.requests, hasLength(13));

    RequestOptions requestOf(String suffix) =>
        adapter.requests.lastWhere((RequestOptions r) => r.path.endsWith(suffix));

    Map<String, dynamic> bodyOf(RequestOptions request) {
      final Object? data = request.data;
      // Dio 的适配器层可能拿到原始 Map，也可能拿到已序列化的 JSON 字符串，
      // 两种形态都归一化成 Map 再断言。
      if (data is Map) {
        return data.map(
          (dynamic key, dynamic value) =>
              MapEntry<String, dynamic>(key.toString(), value),
        );
      }
      if (data is String && data.isNotEmpty) {
        return Map<String, dynamic>.from(jsonDecode(data) as Map);
      }
      fail('请求体为空：${request.path}');
    }

    final RequestOptions devicesReq = requestOf('/app/gb/devices');
    expect(devicesReq.method, 'GET');
    expect(devicesReq.queryParameters['page'], 1);
    expect(devicesReq.queryParameters['page_size'], 20);
    expect(devicesReq.queryParameters['status'], 'online');
    expect(devicesReq.headers['Authorization'], 'Bearer tk-gb');

    final RequestOptions channelsReq = requestOf('/app/gb/channels');
    expect(channelsReq.method, 'GET');
    expect(channelsReq.queryParameters['device_id'], 12);
    expect(channelsReq.queryParameters['page_size'], 100);

    final RequestOptions playReq = requestOf('/app/gb/play/7');
    expect(playReq.method, 'POST');
    expect(bodyOf(playReq), <String, dynamic>{'protocol': 'webrtc'});

    final RequestOptions stopReq = requestOf('/app/gb/play/7/stop');
    expect(stopReq.method, 'POST');
    expect(bodyOf(stopReq), <String, dynamic>{'stream_id': 'call-1'});

    final RequestOptions snapshotReq = requestOf('/app/gb/play/7/snapshot');
    expect(snapshotReq.method, 'POST');

    final RequestOptions ptzReq = requestOf('/app/gb/ptz/7');
    expect(ptzReq.method, 'POST');
    expect(
      bodyOf(ptzReq),
      <String, dynamic>{'direction': 'up', 'speed': 6},
    );

    final RequestOptions recordsReq = requestOf('/app/gb/playback/records');
    expect(recordsReq.method, 'GET');
    expect(recordsReq.queryParameters['channel_id'], 7);
    expect(
      recordsReq.queryParameters['start_time'],
      '2026-01-01T00:00:00',
    );
    expect(
      recordsReq.queryParameters['end_time'],
      '2026-01-01T23:59:59',
    );

    final RequestOptions startPlaybackReq = requestOf('/app/gb/playback');
    expect(startPlaybackReq.method, 'POST');
    expect(bodyOf(startPlaybackReq), <String, dynamic>{
      'channel_id': 7,
      'start_time': '2026-01-01T08:00:00',
      'end_time': '2026-01-01T09:00:00',
    });

    final RequestOptions controlReq =
        requestOf('/app/gb/playback/control');
    expect(controlReq.method, 'POST');
    expect(bodyOf(controlReq), <String, dynamic>{
      'stream_id': 'pb-1',
      'action': 'speed',
      'scale': 2,
    });

    final RequestOptions stopPlaybackReq =
        requestOf('/app/gb/playback/stop');
    expect(bodyOf(stopPlaybackReq), <String, dynamic>{'stream_id': 'pb-1'});

    final RequestOptions startTalkReq = requestOf('/app/gb/talk/7/start');
    expect(startTalkReq.method, 'POST');
    expect(
      bodyOf(startTalkReq),
      <String, dynamic>{'agent_enabled': false},
    );

    final RequestOptions stopTalkReq = requestOf('/app/gb/talk/7/stop');
    expect(
      bodyOf(stopTalkReq),
      <String, dynamic>{'session_id': 'talk-1'},
    );

    final RequestOptions alarmsReq = requestOf('/app/gb/alarms');
    expect(alarmsReq.method, 'GET');
    expect(alarmsReq.queryParameters['page'], 2);
    expect(alarmsReq.queryParameters['page_size'], 10);
    expect(alarmsReq.queryParameters['handled'], isFalse);
  });

  test('业务错误透出后端中文 message', () async {
    ApiService.dio = buildTestDio(
      FixedAdapter(wrapError(400, '云台速度必须在 1-8 之间'), statusCode: 400),
    );

    final REmptyResponse resp = await GbController.ptzUsingPost(
      7,
      const GbPtzReq(direction: GbPtzDirection.up),
    );

    expect(resp.isSuccess, isFalse);
    expect(resp.errorMessage, '云台速度必须在 1-8 之间');
  });

  test('路径常量与契约 §10.12 一致', () {
    expect(GbController.devicesUsingGetPath, '/app/gb/devices');
    expect(GbController.channelsUsingGetPath, '/app/gb/channels');
    expect(GbController.playUsingPostPath, '/app/gb/play');
    expect(GbController.stopPlayUsingPostPath, '/app/gb/play');
    expect(GbController.snapshotUsingPostPath, '/app/gb/play');
    expect(GbController.ptzUsingPostPath, '/app/gb/ptz');
    expect(GbController.recordsUsingGetPath, '/app/gb/playback/records');
    expect(GbController.startPlaybackUsingPostPath, '/app/gb/playback');
    expect(
      GbController.controlPlaybackUsingPostPath,
      '/app/gb/playback/control',
    );
    expect(GbController.stopPlaybackUsingPostPath, '/app/gb/playback/stop');
    expect(GbController.startTalkUsingPostPath, '/app/gb/talk');
    expect(GbController.stopTalkUsingPostPath, '/app/gb/talk');
    expect(GbController.alarmsUsingGetPath, '/app/gb/alarms');
  });
}
