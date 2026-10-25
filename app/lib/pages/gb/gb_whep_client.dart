import 'dart:async';
import 'dart:convert';

import 'package:dio/dio.dart' as dio;
import 'package:flutter/foundation.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';

/// WHEP（WebRTC-HTTP Egress Protocol）客户端。
///
/// 为什么自己实现而不是用 `flutter_webrtc` 自带示例：
/// ZLM 的 WHEP 端点只做一件事 —— 接收 `application/sdp` 的 offer，返回 answer SDP。
/// 没有额外的信令协议，用 Dio 发一个 POST 即可，无需引入新的信令库。
///
/// 与 Web 端 `LivePlayer.tsx` 保持同一套协商流程：
/// 1. `addTransceiver(video/audio, recvonly)`（ZLM 要求 offer 同时声明音视频收流能力）；
/// 2. `createOffer` → `setLocalDescription` → 等 ICE gathering 完成；
/// 3. POST offer SDP 到 WHEP 地址，`Content-Type: application/sdp`；
/// 4. 把返回的 answer `setRemoteDescription`；
/// 5. 等 `onTrack` 拿到视频轨道才算真正连上。
///
/// ★ H.265 限制：Android/iOS 的 `flutter_webrtc` 对 H.265 的支持取决于底层
/// libwebrtc 编译选项，且各机型差异很大。本类在超时或 `onTrack` 未到达时抛出异常，
/// 由调用方**自动回退 HTTP-FLV**（media_kit 走设备原生解码，H.265 支持取决于设备/系统）。
class GbWhepClient {
  /// 构造客户端；`connect` 成功后才持有资源。
  GbWhepClient();

  RTCPeerConnection? _pc;
  MediaStream? _stream;
  RTCVideoRenderer? _renderer;
  bool _closed = false;

  /// 是否已释放。
  bool get isClosed => _closed;

  /// 连接 WHEP 端点，并把远端视频渲染到 [renderer]。
  ///
  /// [connectTimeout] 是「拿到第一条视频轨道」的上限；超过即视为失败（H.265 不兼容
  /// 的典型表现就是 answer 成功但永远没有视频帧），调用方应回退其它播放器。
  Future<void> connect({
    required String url,
    required RTCVideoRenderer renderer,
    List<Map<String, dynamic>> iceServers = const <Map<String, dynamic>>[],
    Duration connectTimeout = const Duration(seconds: 10),
    Duration iceGatherTimeout = const Duration(seconds: 3),
  }) async {
    if (url.isEmpty) {
      throw GbWhepException('后端未返回 WebRTC 地址');
    }

    _renderer = renderer;
    final RTCPeerConnection pc = await createPeerConnection(
      <String, dynamic>{
        // TURN/STUN 由 /app/rtc/ice-servers 签发；为空时退化为直连，
        // 内网 ZLM 仍可工作，受限网络则依赖调用方传入的中继地址。
        'iceServers': iceServers,
        // flutter_webrtc 的 Unified Plan 实现，与 ZLM 的 WHEP 兼容。
        'sdpSemantics': 'unified-plan',
      },
    );
    _pc = pc;

    final Completer<MediaStream> videoReady = Completer<MediaStream>();
    // 先挂一个「吞掉错误」的监听：ICE 可能在下面的 await 之前就失败并 completeError，
    // 若此时 Future 还没有监听者，Dart 会把它当成未处理异常打到 Zone。
    // 后续 `await videoReady.future` 仍能收到同一个错误。
    unawaited(videoReady.future.then<void>((_) {}, onError: (Object _) {}));

    pc.onTrack = (RTCTrackEvent event) {
      if (event.track.kind != 'video' || videoReady.isCompleted) return;
      final List<MediaStream> streams = event.streams;
      if (streams.isNotEmpty) {
        videoReady.complete(streams.first);
        return;
      }
      // 少数平台（尤其部分 Android 实现）不把轨道挂在 stream 上，
      // 这里自建一个容器把轨道放进去，保证 renderer.srcObject 永远拿到有效流。
      unawaited(() async {
        try {
          final MediaStream stream = await createLocalMediaStream('gb-remote');
          await stream.addTrack(event.track);
          if (!videoReady.isCompleted) videoReady.complete(stream);
        } catch (e) {
          if (!videoReady.isCompleted) {
            videoReady.completeError(
              GbWhepException('WebRTC 远端轨道挂载失败：$e'),
            );
          }
        }
      }());
    };

    pc.onConnectionState = (RTCPeerConnectionState state) {
      if (state == RTCPeerConnectionState.RTCPeerConnectionStateFailed &&
          !videoReady.isCompleted) {
        videoReady.completeError(
          GbWhepException('WebRTC 连接失败（ICE/DTLS 未建立）'),
        );
      }
    };

    try {
      // ★ 必须同时声明 audio/video：ZLM 的 WHEP 实现按 offer 里的 m= 行建流，
      // 只声明 video 时部分版本会直接返回 400。
      await pc.addTransceiver(
        kind: RTCRtpMediaType.RTCRtpMediaTypeVideo,
        init: RTCRtpTransceiverInit(direction: TransceiverDirection.RecvOnly),
      );
      await pc.addTransceiver(
        kind: RTCRtpMediaType.RTCRtpMediaTypeAudio,
        init: RTCRtpTransceiverInit(direction: TransceiverDirection.RecvOnly),
      );

      final RTCSessionDescription offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      await _waitIceGathering(pc, iceGatherTimeout);

      final RTCSessionDescription? local = await pc.getLocalDescription();
      final String offerSdp = local?.sdp ?? offer.sdp ?? '';
      if (offerSdp.isEmpty) {
        throw GbWhepException('WebRTC 本地 SDP 生成失败');
      }

      final String answerSdp = await _exchangeSdp(url, offerSdp);
      if (_closed) {
        throw GbWhepException('WebRTC 连接已取消');
      }

      await pc.setRemoteDescription(
        RTCSessionDescription(answerSdp, 'answer'),
      );

      final MediaStream stream = await videoReady.future.timeout(
        connectTimeout,
        onTimeout: () => throw GbWhepException(
          'WebRTC 超时：${connectTimeout.inSeconds} 秒内未收到视频轨道'
          '（常见原因：设备/网关输出 H.265，App 端 WebRTC 不支持，已回退）',
        ),
      );

      if (_closed) {
        throw GbWhepException('WebRTC 连接已取消');
      }
      _stream = stream;
      renderer.srcObject = stream;
      debugPrint('[GB/WHEP] connected: url=$url stream=${stream.id}');
    } catch (e) {
      await close();
      rethrow;
    }
  }

  /// 等待 ICE gathering 完成。
  ///
  /// 内网场景只需要 host candidate。部分平台在「没有 STUN」时不会主动发出
  /// `Complete` 事件，因此这里给一个短超时：超时后直接用已收集到的 SDP 继续，
  /// 不会把本可播放的流误判为失败。
  Future<void> _waitIceGathering(
    RTCPeerConnection pc,
    Duration timeout,
  ) async {
    if (pc.iceGatheringState ==
        RTCIceGatheringState.RTCIceGatheringStateComplete) {
      return;
    }
    final Completer<void> complete = Completer<void>();
    final Function(RTCIceGatheringState)? previous = pc.onIceGatheringState;
    pc.onIceGatheringState = (RTCIceGatheringState state) {
      previous?.call(state);
      if (state == RTCIceGatheringState.RTCIceGatheringStateComplete &&
          !complete.isCompleted) {
        complete.complete();
      }
    };
    try {
      await complete.future.timeout(timeout);
    } on TimeoutException {
      debugPrint('[GB/WHEP] ICE gathering 超时，使用已收集的候选继续');
    } finally {
      pc.onIceGatheringState = previous;
    }
  }

  /// POST offer SDP，读取 answer SDP 文本。
  ///
  /// 这里**不能**走 `ApiService`：
  /// - WHEP 地址属于 ZLM 媒体节点（不是后端 `/web` 或 `/app` 接口），不需要 JWT；
  /// - 请求/响应都是纯文本 SDP，而 `ApiService` 的统一错误处理与 JSON 响应格式不适用。
  Future<String> _exchangeSdp(String url, String offerSdp) async {
    final dio.Dio client = dio.Dio(
      dio.BaseOptions(
        connectTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 15),
        sendTimeout: const Duration(seconds: 10),
        responseType: dio.ResponseType.plain,
        // 4xx/5xx 也交给本方法解析 ZLM 的中文错误，而不是在 Dio 层抛成通用异常。
        validateStatus: (int? status) => status != null,
        headers: <String, dynamic>{
          'Content-Type': 'application/sdp',
          'Accept': 'application/sdp',
        },
      ),
    );
    try {
      final dio.Response<String> response =
          await client.post<String>(url, data: offerSdp);
      final String body = (response.data ?? '').trim();
      final int status = response.statusCode ?? 0;

      if (status >= 400 || body.startsWith('{')) {
        throw GbWhepException(_extractErrorMessage(body, status));
      }
      if (body.isEmpty) {
        throw GbWhepException('WebRTC 信令返回空 SDP（HTTP $status）');
      }
      return body;
    } on dio.DioException catch (e) {
      throw GbWhepException(_dioMessage(e));
    } finally {
      client.close(force: true);
    }
  }

  /// 从 ZLM 的 JSON 错误体里取可读文案；非 JSON 时截断原文。
  String _extractErrorMessage(String body, int status) {
    if (body.isNotEmpty && body.startsWith('{')) {
      try {
        final dynamic decoded = jsonDecode(body);
        if (decoded is Map) {
          final dynamic msg = decoded['msg'] ?? decoded['message'];
          if (msg is String && msg.isNotEmpty) {
            return 'WebRTC 信令失败（HTTP $status）：$msg';
          }
        }
      } catch (_) {
        // 非合法 JSON，落入下面的原文截断分支。
      }
    }
    if (body.isEmpty) {
      return 'WebRTC 信令失败（HTTP $status）';
    }
    final String text = body.length > 160 ? '${body.substring(0, 160)}…' : body;
    return 'WebRTC 信令失败（HTTP $status）：$text';
  }

  /// Dio 异常 → 中文提示。
  String _dioMessage(dio.DioException e) {
    switch (e.type) {
      case dio.DioExceptionType.connectionTimeout:
        return 'WebRTC 信令连接超时';
      case dio.DioExceptionType.sendTimeout:
        return 'WebRTC 信令发送超时';
      case dio.DioExceptionType.receiveTimeout:
        return 'WebRTC 信令响应超时';
      case dio.DioExceptionType.connectionError:
        return '无法连接媒体服务器（WebRTC）';
      case dio.DioExceptionType.badResponse:
        return 'WebRTC 信令失败（HTTP ${e.response?.statusCode ?? '--'}）';
      case dio.DioExceptionType.cancel:
        return 'WebRTC 信令已取消';
      case dio.DioExceptionType.badCertificate:
        return 'WebRTC 证书校验失败';
      case dio.DioExceptionType.unknown:
        return 'WebRTC 信令异常：${e.message ?? '未知错误'}';
      case dio.DioExceptionType.transformTimeout:
        return 'WebRTC 信令数据处理超时';
    }
  }

  /// 释放 PeerConnection / 远端流，并解除 renderer 绑定。
  ///
  /// 必须调用，否则原生 PeerConnection 与硬解实例会泄漏；反复进出播放页后
  /// 表现为「后开的页面黑屏」（MediaCodec 实例耗尽）。
  Future<void> close() async {
    if (_closed) return;
    _closed = true;

    final RTCVideoRenderer? renderer = _renderer;
    _renderer = null;
    if (renderer != null) {
      try {
        renderer.srcObject = null;
      } catch (_) {
        // renderer 可能已被页面释放，忽略。
      }
    }

    final MediaStream? stream = _stream;
    _stream = null;
    if (stream != null) {
      for (final MediaStreamTrack track in stream.getTracks()) {
        try {
          await track.stop();
        } catch (_) {
          // 轨道可能已随 PeerConnection 关闭，忽略。
        }
      }
      try {
        await stream.dispose();
      } catch (_) {
        // 同上。
      }
    }

    final RTCPeerConnection? pc = _pc;
    _pc = null;
    if (pc != null) {
      pc.onTrack = null;
      pc.onConnectionState = null;
      pc.onIceGatheringState = null;
      try {
        await pc.close();
      } catch (_) {
        // 已关闭。
      }
      try {
        await pc.dispose();
      } catch (_) {
        // 部分平台 dispose 会重复释放，忽略。
      }
    }
  }
}

/// WHEP 连接失败（文案可直接展示给用户）。
class GbWhepException implements Exception {
  /// 构造异常。
  GbWhepException(this.message);

  /// 中文错误信息。
  final String message;

  @override
  String toString() => message;
}
