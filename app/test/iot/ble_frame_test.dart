// 蓝牙配网帧编解码：UTF-8 + `\n`，处理拆包与粘包。
import 'dart:convert';

import 'package:ai_camera_app/services/BleProvisioner.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('BleFrames', () {
    test('编码为 UTF-8 JSON 并以换行结尾', () {
      final List<int> bytes = BleFrames.encode(<String, dynamic>{
        'cmd': 'hello',
        'product_code': 'P001',
      });

      final String text = utf8.decode(bytes);
      expect(text.endsWith('\n'), isTrue);
      expect(jsonDecode(text.trim()), <String, dynamic>{
        'cmd': 'hello',
        'product_code': 'P001',
      });
    });

    test('中文内容按 UTF-8 编码后可原样解码', () {
      final List<int> bytes = BleFrames.encode(<String, dynamic>{
        'cmd': 'wifi',
        'ssid': '办公室-5G',
        'password': '密码123',
      });
      final Map<String, dynamic>? frame =
          BleFrames.decodeLine(utf8.decode(bytes));
      expect(frame?['ssid'], '办公室-5G');
      expect(frame?['password'], '密码123');
    });

    test('decodeLine 忽略空行与非法 JSON', () {
      expect(BleFrames.decodeLine(''), isNull);
      expect(BleFrames.decodeLine('   '), isNull);
      expect(BleFrames.decodeLine('not-json'), isNull);
      expect(BleFrames.decodeLine('[1,2]'), isNull);
    });
  });

  group('BleFrameDecoder', () {
    test('一次收到完整帧可直接解析', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      final List<Map<String, dynamic>> frames = decoder.add(
        BleFrames.encode(<String, dynamic>{
          'status': 'ok',
          'device_code': 'CAM-1',
        }),
      );

      expect(frames, hasLength(1));
      expect(frames.first['status'], 'ok');
      expect(frames.first['device_code'], 'CAM-1');
    });

    test('拆包：分两次通知到达，只在换行后产出帧', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      final List<int> bytes = BleFrames.encode(<String, dynamic>{
        'status': 'ok',
        'device_code': 'CAM-2',
      });
      final int split = bytes.length ~/ 2;

      expect(decoder.add(bytes.sublist(0, split)), isEmpty);
      final List<Map<String, dynamic>> frames =
          decoder.add(bytes.sublist(split));
      expect(frames, hasLength(1));
      expect(frames.first['device_code'], 'CAM-2');
    });

    test('拆包时多字节 UTF-8 字符不会被解坏', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      final List<int> bytes = BleFrames.encode(<String, dynamic>{
        'status': 'ok',
        'message': '连接成功',
      });
      // 在第一个汉字中间切开（汉字 3 字节）。
      final int split = bytes.indexOf(utf8.encode('连').first) + 1;

      expect(decoder.add(bytes.sublist(0, split)), isEmpty);
      final List<Map<String, dynamic>> frames =
          decoder.add(bytes.sublist(split));
      expect(frames, hasLength(1));
      expect(frames.first['message'], '连接成功');
    });

    test('粘包：一次通知包含两帧', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      final List<int> bytes = <int>[
        ...BleFrames.encode(<String, dynamic>{'status': 'ok'}),
        ...BleFrames.encode(<String, dynamic>{
          'status': 'ok',
          'device_code': 'CAM-3',
          'device_secret': 'secret-3',
        }),
      ];

      final List<Map<String, dynamic>> frames = decoder.add(bytes);
      expect(frames, hasLength(2));
      expect(frames[0]['status'], 'ok');
      expect(frames[1]['device_secret'], 'secret-3');
    });

    test('非法行被跳过，合法帧照常返回', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      final List<int> bytes = utf8.encode(
        'firmware log line\n'
        '{"status":"ok"}\n',
      );

      final List<Map<String, dynamic>> frames = decoder.add(bytes);
      expect(frames, hasLength(1));
      expect(frames.first['status'], 'ok');
    });

    test('未以换行结尾的半帧不会被提前消费', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      expect(decoder.add(utf8.encode('{"status":"ok"')), isEmpty);
      final List<Map<String, dynamic>> frames =
          decoder.add(utf8.encode('}\n'));
      expect(frames, hasLength(1));
      expect(frames.first['status'], 'ok');
    });

    test('reset 清空半帧缓存', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      expect(decoder.add(utf8.encode('{"status":')), isEmpty);
      decoder.reset();
      final List<Map<String, dynamic>> frames =
          decoder.add(utf8.encode('{"status":"ok"}\n'));
      expect(frames, hasLength(1));
      expect(frames.first['status'], 'ok');
    });

    test('超长无换行数据被丢弃，不会无限增长', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      final List<int> huge = List<int>.filled(9000, 65); // 'A' * 9000
      expect(decoder.add(huge), isEmpty);
      // 丢弃后可以从头开始正常解析。
      final List<Map<String, dynamic>> frames =
          decoder.add(utf8.encode('{"status":"ok"}\n'));
      expect(frames, hasLength(1));
    });

    test('空 chunk 不产生帧', () {
      final BleFrameDecoder decoder = BleFrameDecoder();
      expect(decoder.add(const <int>[]), isEmpty);
    });
  });
}
