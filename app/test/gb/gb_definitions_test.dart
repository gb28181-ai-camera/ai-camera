// 国标视频模型解析测试。
//
// 重点：后端字段是 snake_case，且大量使用 *int64 / *bool（可为 null）。
// 这里锁定「字段不漏、null 不崩、分页/单条包装读取正确」，避免页面层再出现
// Map 直接取值的绕过类型写法。
import 'package:ai_camera_app/services/generated/definitions/AppGb.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('AppGbDevice', () {
    test('解析完整字段与空关联字段', () {
      final AppGbDevice device = AppGbDevice.fromJson(<String, dynamic>{
        'id': 12,
        'name': '东门球机',
        'device_code': 'DEV-001',
        'gb_device_id': '34020000001320000001',
        'region_id': 3,
        'region_name': '东门区域',
        'location': '东门岗亭',
        'status': 'online',
        'register_status': 'activated',
        'channel_count': 4,
        'active': true,
      });

      expect(device.id, 12);
      expect(device.name, '东门球机');
      expect(device.deviceCode, 'DEV-001');
      expect(device.gbDeviceId, '34020000001320000001');
      expect(device.regionId, 3);
      expect(device.regionName, '东门区域');
      expect(device.location, '东门岗亭');
      expect(device.isOnline, isTrue);
      expect(device.isActivated, isTrue);
      expect(device.channelCount, 4);
      expect(device.active, isTrue);

      final AppGbDevice empty = AppGbDevice.fromJson(<String, dynamic>{
        'id': '7',
        'status': 'offline',
        'register_status': 'pending',
      });
      expect(empty.id, 7);
      expect(empty.regionId, isNull);
      expect(empty.active, isNull);
      expect(empty.isOnline, isFalse);
      expect(empty.isActivated, isFalse);
    });
  });

  group('AppGbChannel', () {
    test('ON/OFF 状态判断与可空布尔', () {
      final AppGbChannel channel = AppGbChannel.fromJson(<String, dynamic>{
        'id': 21,
        'device_id': 12,
        'device_name': '东门球机',
        'gb_device_id': '34020000001320000001',
        'gb_channel_id': '34020000001310000001',
        'name': '通道一',
        'status': 'ON',
        'channel_type': 132,
        'ptz_type': 1,
        'has_audio': true,
        'enable_broadcast': null,
        'region_name': '东门区域',
        'group_name': '主干道',
      });

      expect(channel.id, 21);
      expect(channel.deviceId, 12);
      expect(channel.deviceName, '东门球机');
      expect(channel.status, 'ON');
      expect(channel.isOnline, isTrue);
      expect(channel.maybeSupportsPtz, isTrue);
      expect(channel.hasAudio, isTrue);
      expect(channel.enableBroadcast, isNull);
      expect(channel.regionName, '东门区域');
      expect(channel.groupName, '主干道');

      final AppGbChannel offline = AppGbChannel.fromJson(<String, dynamic>{
        'id': 22,
        'status': 'OFF',
        'ptz_type': 0,
      });
      expect(offline.isOnline, isFalse);
      expect(offline.maybeSupportsPtz, isFalse);
    });
  });

  group('实时播放 / 抓图', () {
    test('三种地址与 stream_id 全部保留', () {
      final AppGbPlayInfo info = AppGbPlayInfo.fromJson(<String, dynamic>{
        'app': 'rtp',
        'stream': '3B9ACA01',
        'http_base': 'http://10.0.0.5:8082',
        'webrtc': 'http://10.0.0.5:8082/index/api/whep?app=rtp&stream=3B9ACA01',
        'flv': 'http://10.0.0.5:8082/rtp/3B9ACA01.live.flv',
        'hls': 'http://10.0.0.5:8082/rtp/3B9ACA01/hls.m3u8',
        'stream_id': 'call-1',
      });

      expect(info.app, 'rtp');
      expect(info.stream, '3B9ACA01');
      expect(info.httpBase, 'http://10.0.0.5:8082');
      expect(info.hasWebrtc, isTrue);
      expect(info.fallbackUrl, info.flv);
      expect(info.streamId, 'call-1');

      final AppGbPlayInfo noFlv = AppGbPlayInfo.fromJson(<String, dynamic>{
        'hls': 'http://x/hls.m3u8',
      });
      expect(noFlv.hasWebrtc, isFalse);
      expect(noFlv.fallbackUrl, 'http://x/hls.m3u8');
    });

    test('抓图结果与可读文件大小', () {
      final AppGbSnapshot snapshot = AppGbSnapshot.fromJson(<String, dynamic>{
        'url': 'http://localhost:8080/uploads/202610/2f1c.jpg',
        'file_name': '2f1c.jpg',
        'file_size': 48213,
        'capture_dt': '2026-10-21 10:00:00',
      });

      expect(snapshot.url, contains('/uploads/'));
      expect(snapshot.fileName, '2f1c.jpg');
      expect(snapshot.readableSize, '47.1 KB');
      expect(snapshot.captureDt, '2026-10-21 10:00:00');
    });
  });

  group('录像 / 回放 / 对讲 / 报警', () {
    test('录像检索条目与 timeout', () {
      final AppGbRecordQueryResult result =
          AppGbRecordQueryResult.fromJson(<String, dynamic>{
        'channel_id': 21,
        'total': 1,
        'timeout': true,
        'items': <dynamic>[
          <String, dynamic>{
            'channel_id': 21,
            'name': '录像一',
            'start_time': '2026-01-01T08:00:00',
            'end_time': '2026-01-01T09:00:00',
            'time_len': 3600,
          },
        ],
      });

      expect(result.channelId, 21);
      expect(result.total, 1);
      expect(result.timeout, isTrue);
      expect(result.items, hasLength(1));
      expect(result.items.first.readableDuration, '1 小时 0 分');
    });

    test('回放会话字段', () {
      final AppGbPlaybackInfo info =
          AppGbPlaybackInfo.fromJson(<String, dynamic>{
        'stream_id': 'pb-1',
        'app': 'rtp',
        'stream': '0200000001',
        'http_base': 'http://10.0.0.5:8082',
        'webrtc': 'http://w',
        'flv': 'http://f',
        'hls': 'http://h',
        'channel_id': 21,
        'start_time': '2026-01-01 08:00:00',
        'end_time': '2026-01-01 09:00:00',
      });

      expect(info.streamId, 'pb-1');
      expect(info.channelId, 21);
      expect(info.webrtc, 'http://w');
      expect(info.startTime, '2026-01-01 08:00:00');
    });

    test('对讲会话字段', () {
      final AppGbVoiceSession session =
          AppGbVoiceSession.fromJson(<String, dynamic>{
        'session_id': 'talk-1',
        'mode': 'half_duplex',
        'agent_enabled': false,
        'ssrc': '0342000002',
        'audio_codec': 'G.711A',
        'start_dt': '2026-10-21 10:00:00',
      });

      expect(session.sessionId, 'talk-1');
      expect(session.mode, 'half_duplex');
      expect(session.agentEnabled, isFalse);
      expect(session.ssrc, '0342000002');
      expect(session.audioCodec, 'G.711A');
    });

    test('报警条目允许设备/通道为空', () {
      final AppGbAlarm alarm = AppGbAlarm.fromJson(<String, dynamic>{
        'id': 5,
        'device_id': null,
        'channel_id': 21,
        'device_name': '东门球机',
        'channel_name': '通道一',
        'alarm_priority': '1',
        'alarm_type': '12',
        'alarm_time': '2026-10-21T10:00:00+08:00',
        'alarm_description': '区域入侵',
        'snap_path': '/uploads/a.jpg',
        'handled': false,
      });

      expect(alarm.id, 5);
      expect(alarm.deviceId, isNull);
      expect(alarm.channelId, 21);
      expect(alarm.alarmPriority, '1');
      expect(alarm.alarmTime, '2026-10-21T10:00:00+08:00');
      expect(alarm.handled, isFalse);
    });
  });

  group('请求体序列化', () {
    test('direction 与 action 互斥地按需输出', () {
      expect(
        const GbPtzReq(direction: GbPtzDirection.up, speed: 6).toJson(),
        <String, dynamic>{'direction': 'up', 'speed': 6},
      );
      expect(
        const GbPtzReq(action: GbPtzAction.zoomIn).toJson(),
        <String, dynamic>{'action': 'zoom-in', 'speed': 5},
      );
      expect(
        const GbPlayReq().toJson(),
        <String, dynamic>{'protocol': 'webrtc'},
      );
      expect(
        const GbPlaybackControlReq(
          streamId: 's1',
          action: 'speed',
          scale: 2,
        ).toJson(),
        <String, dynamic>{'stream_id': 's1', 'action': 'speed', 'scale': 2},
      );
    });
  });

  group('分页响应包装', () {
    test('RAppGbDevicePage / RAppGbChannelPage 读取 items 与 total', () {
      final RAppGbDevicePage devices = RAppGbDevicePage()
        ..fromJson(<String, dynamic>{
          'code': 200,
          'message': 'ok',
          'data': <String, dynamic>{
            'items': <dynamic>[
              <String, dynamic>{'id': 1, 'name': 'A'},
            ],
            'total': 1,
            'page': 1,
            'page_size': 20,
          },
        });
      expect(devices.isSuccess, isTrue);
      expect(devices.total, 1);
      expect(devices.items.single.name, 'A');

      final RAppGbChannelPage channels = RAppGbChannelPage()
        ..fromJson(<String, dynamic>{
          'code': 200,
          'message': 'ok',
          'data': <String, dynamic>{
            'items': <dynamic>[
              <String, dynamic>{'id': 2, 'status': 'ON'},
            ],
            'total': 1,
            'page': 1,
            'page_size': 100,
          },
        });
      expect(channels.items.single.isOnline, isTrue);
      expect(channels.page.hasMore, isFalse);
    });
  });
}
