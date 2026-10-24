// 扫码内容解析与分流：分享码 / 设备二维码 / 非法内容。
import 'dart:convert';

import 'package:ai_camera_app/services/IotAppController.dart';
import 'package:flutter_test/flutter_test.dart';

/// 构造二维码 JSON 文本。
String qr(Map<String, dynamic> payload) => jsonEncode(payload);

void main() {
  group('分享码 device_share', () {
    test('合法分享码解析出 token，并分流到 claim', () {
      final IotQrPayload? payload = IotQrPayload.tryParse(
        qr(<String, dynamic>{
          'type': 'device_share',
          'token': 'share-token-123',
        }),
      );

      expect(payload, isA<DeviceShareCode>());
      expect(payload!.action, IotQrAction.claim);
      if (payload is DeviceShareCode) {
        expect(payload.token, 'share-token-123');
      }
    });

    test('token 前后空白会被裁剪', () {
      final IotQrPayload? payload = IotQrPayload.tryParse(
        qr(<String, dynamic>{
          'type': 'device_share',
          'token': '  abc  ',
        }),
      );
      expect(payload, isA<DeviceShareCode>());
      if (payload is DeviceShareCode) {
        expect(payload.token, 'abc');
      }
    });

    test('缺少 token 视为非法', () {
      expect(
        IotQrPayload.tryParse(qr(<String, dynamic>{'type': 'device_share'})),
        isNull,
      );
    });

    test('token 为空串视为非法', () {
      expect(
        IotQrPayload.tryParse(
          qr(<String, dynamic>{'type': 'device_share', 'token': '   '}),
        ),
        isNull,
      );
    });
  });

  group('设备二维码 device', () {
    test('合法设备码解析出产品编码与设备编码，并分流到 bindQr', () {
      final IotQrPayload? payload = IotQrPayload.tryParse(
        qr(<String, dynamic>{
          'type': 'device',
          'product_code': 'P001',
          'device_code': 'CAM-0001',
        }),
      );

      expect(payload, isA<DeviceQrCode>());
      expect(payload!.action, IotQrAction.bindQr);
      if (payload is DeviceQrCode) {
        expect(payload.productCode, 'P001');
        expect(payload.deviceCode, 'CAM-0001');
      }
    });

    test('缺少 product_code 视为非法', () {
      expect(
        IotQrPayload.tryParse(
          qr(<String, dynamic>{'type': 'device', 'device_code': 'CAM-1'}),
        ),
        isNull,
      );
    });

    test('缺少 device_code 视为非法', () {
      expect(
        IotQrPayload.tryParse(
          qr(<String, dynamic>{'type': 'device', 'product_code': 'P001'}),
        ),
        isNull,
      );
    });
  });

  group('链接格式（参考 neme-ai-app）', () {
    test('https://boticz.cn/qrDevice/{product}/{device} → 设备码', () {
      final IotQrPayload? payload = IotQrPayload.tryParse(
        'https://boticz.cn/qrDevice/P-OD-01/CAM-0005',
      );

      expect(payload, isA<DeviceQrCode>());
      expect(payload!.action, IotQrAction.bindQr);
      if (payload is DeviceQrCode) {
        expect(payload.productCode, 'P-OD-01');
        expect(payload.deviceCode, 'CAM-0005');
      }
    });

    test('http://boticz.cn/deviceShareToken/{token} → 分享码', () {
      final IotQrPayload? payload = IotQrPayload.tryParse(
        'http://boticz.cn/deviceShareToken/share-abc',
      );

      expect(payload, isA<DeviceShareCode>());
      expect(payload!.action, IotQrAction.claim);
      if (payload is DeviceShareCode) {
        expect(payload.token, 'share-abc');
      }
    });

    test('无 scheme 的链接同样可解析', () {
      final IotQrPayload? device = IotQrPayload.tryParse(
        'boticz.cn/qrDevice/P1/CAM1',
      );
      final IotQrPayload? share = IotQrPayload.tryParse(
        'boticz.cn/deviceShareToken/token-1',
      );

      expect(device, isA<DeviceQrCode>());
      expect(share, isA<DeviceShareCode>());
    });

    test('前后空白与查询串不影响解析', () {
      final IotQrPayload? payload = IotQrPayload.tryParse(
        '  https://boticz.cn/qrDevice/P-1/CAM-1?from=poster#top  ',
      );
      expect(payload, isA<DeviceQrCode>());
      if (payload is DeviceQrCode) {
        expect(payload.productCode, 'P-1');
        expect(payload.deviceCode, 'CAM-1');
      }
    });

    test('百分号编码的片段会解码', () {
      final IotQrPayload? payload = IotQrPayload.tryParse(
        'https://boticz.cn/qrDevice/P%2001/CAM%2D01',
      );
      expect(payload, isA<DeviceQrCode>());
      if (payload is DeviceQrCode) {
        expect(payload.productCode, 'P 01');
        expect(payload.deviceCode, 'CAM-01');
      }
    });
  });

  group('非法二维码', () {
    test('普通网址返回 null', () {
      expect(IotQrPayload.tryParse('https://example.com/abc'), isNull);
    });

    test('非本平台域名的同名路径返回 null', () {
      expect(
        IotQrPayload.tryParse('https://example.com/qrDevice/P/D'),
        isNull,
      );
      expect(
        IotQrPayload.tryParse('https://evil-boticz.cn/qrDevice/P/D'),
        isNull,
      );
    });

    test('链接缺字段 / 路径不认识返回 null', () {
      expect(
        IotQrPayload.tryParse('https://boticz.cn/qrDevice/P-1'),
        isNull,
      );
      expect(
        IotQrPayload.tryParse('https://boticz.cn/qrDevice/P-1/D-1/extra'),
        isNull,
      );
      expect(
        IotQrPayload.tryParse('https://boticz.cn/deviceShareToken'),
        isNull,
      );
      expect(
        IotQrPayload.tryParse('https://boticz.cn/deviceShareToken/  '),
        isNull,
      );
      expect(
        IotQrPayload.tryParse('https://boticz.cn/unknown/P/D'),
        isNull,
      );
    });

    test('空文本返回 null', () {
      expect(IotQrPayload.tryParse('   '), isNull);
    });

    test('未知 type 返回 null', () {
      expect(
        IotQrPayload.tryParse(
          qr(<String, dynamic>{'type': 'unknown', 'token': 'x'}),
        ),
        isNull,
      );
    });

    test('非对象 JSON 返回 null', () {
      expect(IotQrPayload.tryParse('[1,2,3]'), isNull);
    });

    test('截断的 JSON 返回 null', () {
      expect(IotQrPayload.tryParse('{"type":"device_share"'), isNull);
    });
  });

  test('同一入口按 type 分流到不同动作', () {
    final List<IotQrPayload> payloads = <IotQrPayload>[
      IotQrPayload.tryParse(
        qr(<String, dynamic>{'type': 'device_share', 'token': 't'}),
      )!,
      IotQrPayload.tryParse(
        qr(<String, dynamic>{
          'type': 'device',
          'product_code': 'P',
          'device_code': 'D',
        }),
      )!,
    ];

    expect(
      payloads.map((IotQrPayload e) => e.action).toList(),
      <IotQrAction>[IotQrAction.claim, IotQrAction.bindQr],
    );
    expect(payloads[0], isA<DeviceShareCode>());
    expect(payloads[1], isA<DeviceQrCode>());
  });

  test('DeviceShareCode.tryParse 不匹配设备二维码', () {
    expect(
      DeviceShareCode.tryParse(
        qr(<String, dynamic>{
          'type': 'device',
          'product_code': 'P',
          'device_code': 'D',
        }),
      ),
      isNull,
    );
    expect(DeviceQrCode.tryParse('{"type":"device_share","token":"t"}'), isNull);
  });
}
