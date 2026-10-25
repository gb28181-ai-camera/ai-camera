/// 设备 ↔ App 实时通话客户端（WebRTC over WebSocket）。
///
/// 与 Web 预览不同，App 通话多一层「呼叫控制」：
///
///   App ──invite──▶ 设备        （或设备 ──invite──▶ App）
///   设备 ──agree / reject──▶ App
///   agree 后进入媒体协商：App 发 joinRoom → 设备发 offer → App answer
///
/// 信令由 Go 网关转 MQTT；媒体点对点（必要时 TURN）。
/// 设备不在线时后端会推送离线通知，App 收到通知后打开通话页即可。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:flutter/foundation.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import 'api_service.dart';

/// 通话状态。
enum AppAvCallState {
  /// 未连接。
  idle,

  /// 正在申请 ticket / 建连。
  connecting,

  /// 信令已就绪，可发起呼叫。
  ready,

  /// 来电振铃（等待接听/拒绝）。
  ringing,

  /// 呼叫中（已发 invite，等待对方同意）。
  calling,

  /// 通话中（媒体协商/已连接）。
  inCall,

  /// 已结束。
  ended,

  /// 出错。
  error,
}

/// App 通话客户端。
class AppAvCallClient {
  /// 构造客户端。
  AppAvCallClient({required this.deviceId, required this.deviceName});

  /// 设备 ID。
  final int deviceId;

  /// 设备名称（来电/通话页标题）。
  final String deviceName;

  /// 远端视频渲染器。
  final RTCVideoRenderer remoteRenderer = RTCVideoRenderer();

  /// 当前状态。
  final ValueNotifier<AppAvCallState> state =
      ValueNotifier<AppAvCallState>(AppAvCallState.idle);

  /// 最近错误（UI 可直接展示）。
  final ValueNotifier<String> lastError = ValueNotifier<String>('');

  /// 当前通话 ID。
  final ValueNotifier<String> callId = ValueNotifier<String>('');

  /// 对端角色（device/app）与远端是否已进入房间。
  final ValueNotifier<bool> remoteJoined = ValueNotifier<bool>(false);

  /// 是否已收到远端视频轨道（驱动页面从占位切换到 RTCVideoView）。
  final ValueNotifier<bool> hasRemoteVideo = ValueNotifier<bool>(false);

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _sub;
  RTCPeerConnection? _pc;
  final List<RTCIceCandidate> _pendingCandidates = <RTCIceCandidate>[];
  bool _remoteReady = false;
  bool _rendererReady = false;
  MediaStream? _localStream;

  /// 是否已连接信令。
  bool get connected => _channel != null;

  /// 建立信令连接（不自动呼叫）。
  Future<void> connect() async {
    if (_channel != null) return;
    state.value = AppAvCallState.connecting;
    lastError.value = '';
    try {
      if (!_rendererReady) {
        await remoteRenderer.initialize();
        _rendererReady = true;
      }
      final dynamic resp = await ApiService.post(
        '/app/av/devices/$deviceId/call-ticket',
        null,
        <String, dynamic>{},
      );
      final Map<String, dynamic> body =
          (resp.data as Map?)?.cast<String, dynamic>() ?? <String, dynamic>{};
      if (body['code'] != 200) {
        throw StateError((body['message'] ?? '申请通话 ticket 失败').toString());
      }
      final Map<String, dynamic> data =
          (body['data'] as Map?)?.cast<String, dynamic>() ?? <String, dynamic>{};
      final String wsUrl = (data['ws_url'] ?? '').toString();
      final String ticket = (data['ticket'] ?? '').toString();
      if (wsUrl.isEmpty || ticket.isEmpty) {
        throw StateError('ticket 响应缺少 ws_url/ticket');
      }
      final List<dynamic> iceRaw =
          (data['ice_servers'] as List<dynamic>?) ?? <dynamic>[];
      final List<Map<String, dynamic>> iceServers = iceRaw
          .whereType<Map>()
          .map((Map raw) => raw.cast<String, dynamic>())
          .where((Map<String, dynamic> s) =>
              (s['urls'] as List<dynamic>?)?.isNotEmpty ?? false)
          .map((Map<String, dynamic> s) => <String, dynamic>{
                'urls': List<String>.from(s['urls'] as List<dynamic>),
                if ((s['username'] ?? '').toString().isNotEmpty)
                  'username': s['username'].toString(),
                if ((s['credential'] ?? '').toString().isNotEmpty)
                  'credential': s['credential'].toString(),
              })
          .toList();

      await _createPeer(iceServers);
      final Uri uri = Uri.parse(wsUrl);
      final Uri withTicket = uri.replace(queryParameters: <String, String>{
        ...uri.queryParameters,
        'ticket': ticket,
      });
      final WebSocketChannel channel = WebSocketChannel.connect(withTicket);
      _channel = channel;
      _sub = channel.stream.listen(
        _onMessage,
        onError: (Object err) {
          lastError.value = '信令连接错误：$err';
          state.value = AppAvCallState.error;
        },
        onDone: () {
          _channel = null;
          if (state.value != AppAvCallState.ended) {
            state.value = AppAvCallState.ended;
          }
        },
      );
    } catch (err) {
      lastError.value = err.toString();
      state.value = AppAvCallState.error;
      await dispose();
      rethrow;
    }
  }

  /// 主动呼叫设备。
  Future<void> invite() async {
    final String id = _newCallId();
    callId.value = id;
    state.value = AppAvCallState.calling;
    _send(<String, dynamic>{
      'type': 'invite',
      'call_id': id,
      'media_type': 'video',
    });
  }

  /// 接听设备来电。
  Future<void> agree() async {
    final String id = callId.value;
    if (id.isEmpty) return;
    state.value = AppAvCallState.inCall;
    _send(<String, dynamic>{'type': 'agree', 'call_id': id});
    _send(<String, dynamic>{'type': 'joinRoom'});
  }

  /// 拒绝设备来电。
  void reject() {
    final String id = callId.value;
    if (id.isNotEmpty) {
      _send(<String, dynamic>{
        'type': 'reject',
        'call_id': id,
        'reason': 'user_reject',
      });
    }
    state.value = AppAvCallState.ended;
    unawaited(dispose());
  }

  /// 挂断。
  void hangUp() {
    final String id = callId.value;
    if (id.isNotEmpty) {
      _send(<String, dynamic>{
        'type': 'hangup',
        'call_id': id,
        'reason': 'user_hangup',
      });
    }
    _send(<String, dynamic>{'type': 'leaveRoom', 'reason': 'hangup'});
    state.value = AppAvCallState.ended;
    unawaited(dispose());
  }

  /// 释放资源。
  Future<void> dispose() async {
    _sub?.cancel();
    _sub = null;
    await _channel?.sink.close();
    _channel = null;
    _pendingCandidates.clear();
    _remoteReady = false;
    await _pc?.close();
    _pc = null;
    await _localStream?.dispose();
    _localStream = null;
    hasRemoteVideo.value = false;
    remoteRenderer.srcObject = null;
  }

  // ── 内部 ────────────────────────────────────────────────

  Future<void> _createPeer(List<Map<String, dynamic>> iceServers) async {
    await _pc?.close();
    _pc = await createPeerConnection(<String, dynamic>{
      'iceServers': iceServers,
      'sdpSemantics': 'unified-plan',
      'bundlePolicy': 'max-bundle',
    });
    _pc!.onTrack = (RTCTrackEvent event) {
      if (event.streams.isNotEmpty) {
        remoteRenderer.srcObject = event.streams.first;
        hasRemoteVideo.value = true;
      }
    };
    _pc!.onIceCandidate = (RTCIceCandidate candidate) {
      if (candidate.candidate == null || candidate.candidate!.isEmpty) return;
      _send(<String, dynamic>{
        'type': 'iceCandidate',
        'candidate': <String, dynamic>{
          'candidate': candidate.candidate,
          'sdpMid': candidate.sdpMid,
          'sdpMLineIndex': candidate.sdpMLineIndex,
        },
      });
    };
    _pc!.onConnectionState = (RTCPeerConnectionState value) {
      if (value == RTCPeerConnectionState.RTCPeerConnectionStateConnected) {
        state.value = AppAvCallState.inCall;
      }
      if (value == RTCPeerConnectionState.RTCPeerConnectionStateFailed) {
        lastError.value = 'WebRTC 连接失败，请检查网络/TURN';
      }
    };
  }

  Future<void> _onMessage(dynamic raw) async {
    final Map<String, dynamic> msg;
    try {
      msg = (jsonDecode(raw.toString()) as Map).cast<String, dynamic>();
    } catch (err) {
      lastError.value = '解析信令失败：$err';
      return;
    }
    final String type = (msg['type'] ?? '').toString();
    callId.value = (msg['call_id'] ?? callId.value).toString();
    switch (type) {
      case 'ready':
        state.value = AppAvCallState.ready;
        break;
      case 'invite':
        state.value = AppAvCallState.ringing;
        break;
      case 'agree':
        state.value = AppAvCallState.inCall;
        // 对方同意后进入媒体协商；设备是 offer 方。
        _send(<String, dynamic>{'type': 'joinRoom'});
        break;
      case 'reject':
        lastError.value = '对方拒绝了通话';
        state.value = AppAvCallState.ended;
        break;
      case 'hangup':
        state.value = AppAvCallState.ended;
        break;
      case 'joinRoom':
        if ((msg['role'] ?? '') == 'device') {
          remoteJoined.value = true;
          // 约定：收到设备 joinRoom 后再发一次，设备据此发 offer。
          _send(<String, dynamic>{'type': 'joinRoom'});
        }
        break;
      case 'offer':
        await _handleOffer((msg['sdp'] ?? '').toString());
        break;
      case 'answer':
        if (_pc != null &&
            _pc!.signalingState ==
                RTCSignalingState.RTCSignalingStateHaveLocalOffer) {
          await _pc!.setRemoteDescription(
            RTCSessionDescription((msg['sdp'] ?? '').toString(), 'answer'),
          );
          _remoteReady = true;
          await _flushCandidates();
        }
        break;
      case 'iceCandidate':
        final Map<String, dynamic>? c =
            (msg['candidate'] as Map?)?.cast<String, dynamic>();
        if (c == null || (c['candidate'] ?? '').toString().isEmpty) break;
        final RTCIceCandidate candidate = RTCIceCandidate(
          c['candidate'].toString(),
          (c['sdpMid'] ?? '').toString(),
          (c['sdpMLineIndex'] as num?)?.toInt() ?? 0,
        );
        if (_pc != null && _remoteReady) {
          await _pc!.addCandidate(candidate);
        } else {
          _pendingCandidates.add(candidate);
        }
        break;
      case 'leaveRoom':
        state.value = AppAvCallState.ended;
        break;
      case 'error':
        lastError.value = (msg['message'] ?? '信令错误').toString();
        break;
    }
  }

  Future<void> _handleOffer(String sdp) async {
    final RTCPeerConnection? pc = _pc;
    if (pc == null || sdp.isEmpty) return;
    if (pc.signalingState != RTCSignalingState.RTCSignalingStateStable) {
      try {
        await pc.setRemoteDescription(
          RTCSessionDescription('', 'rollback'),
        );
      } catch (_) {
        // 部分平台不支持 rollback；继续尝试直接接受新 offer。
      }
    }
    await pc.setRemoteDescription(RTCSessionDescription(sdp, 'offer'));
    _remoteReady = true;
    await _flushCandidates();
    // 若设备 offer 含 audio m-line，尝试把麦克风接上（设备需声明 sendrecv）。
    await _attachLocalAudio(pc);
    final RTCSessionDescription answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);
    _send(<String, dynamic>{
      'type': 'answer',
      'sdp': answer.sdp,
      'sdp_type': 'answer',
    });
  }

  Future<void> _attachLocalAudio(RTCPeerConnection pc) async {
    try {
      final List<RTCRtpTransceiver> transceivers =
          await pc.getTransceivers();
      final bool hasAudio = transceivers.any((RTCRtpTransceiver t) =>
          t.receiver.track?.kind == 'audio' ||
          t.sender.track?.kind == 'audio');
      if (!hasAudio) return;
      final MediaStream stream = await navigator.mediaDevices.getUserMedia(
        <String, dynamic>{'audio': true, 'video': false},
      );
      _localStream = stream;
      if (stream.getAudioTracks().isEmpty) return;
      final MediaStreamTrack mic = stream.getAudioTracks().first;
      for (final RTCRtpTransceiver t in transceivers) {
        if (t.receiver.track?.kind == 'audio' ||
            t.sender.track?.kind == 'audio') {
          await t.sender.replaceTrack(mic);
          break;
        }
      }
    } catch (_) {
      // 麦克风权限/设备不支持：仍可看画面，不阻断通话。
    }
  }

  Future<void> _flushCandidates() async {
    final RTCPeerConnection? pc = _pc;
    if (pc == null) return;
    final List<RTCIceCandidate> queued =
        List<RTCIceCandidate>.from(_pendingCandidates);
    _pendingCandidates.clear();
    for (final RTCIceCandidate c in queued) {
      try {
        await pc.addCandidate(c);
      } catch (_) {
        // 候选过期/重复可忽略。
      }
    }
  }

  void _send(Map<String, dynamic> msg) {
    final WebSocketChannel? channel = _channel;
    if (channel == null) return;
    try {
      channel.sink.add(jsonEncode(msg));
    } catch (err) {
      lastError.value = '发送信令失败：$err';
    }
  }

  String _newCallId() {
    final Random r = Random.secure();
    final String suffix =
        List<int>.generate(6, (_) => r.nextInt(256)).map((int v) {
      return v.toRadixString(16).padLeft(2, '0');
    }).join();
    return 'app-call-$suffix';
  }
}
