import 'package:flutter/material.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';

import '../../services/AppAvCallClient.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';

/// 设备 ↔ App 视频通话页。
///
/// 流程：
/// - 进入页面即申请 ticket 并连 `/app/av/ws`；
/// - 「呼叫」→ invite；设备 agree 后自动 joinRoom/offer/answer；
/// - 设备来电 → 页面显示「接听/拒绝」；
/// - 挂断发送 hangup + leaveRoom。
class AppCallPage extends StatefulWidget {
  /// 构造通话页。
  const AppCallPage({
    super.key,
    required this.deviceId,
    required this.deviceName,
  });

  /// 设备 ID。
  final int deviceId;

  /// 设备名称。
  final String deviceName;

  @override
  State<AppCallPage> createState() => _AppCallPageState();
}

class _AppCallPageState extends State<AppCallPage> {
  late final AppAvCallClient _client;
  bool _starting = true;

  @override
  void initState() {
    super.initState();
    _client = AppAvCallClient(
      deviceId: widget.deviceId,
      deviceName: widget.deviceName,
    );
    _start();
  }

  Future<void> _start() async {
    try {
      await _client.connect();
    } catch (err) {
      if (mounted) {
        context.showToast(err.toString());
      }
    } finally {
      if (mounted) setState(() => _starting = false);
    }
  }

  @override
  void dispose() {
    _client.hangUp();
    super.dispose();
  }

  String get _statusText {
    switch (_client.state.value) {
      case AppAvCallState.idle:
        return '未连接';
      case AppAvCallState.connecting:
        return '正在连接…';
      case AppAvCallState.ready:
        return '可发起通话';
      case AppAvCallState.ringing:
        return '设备来电';
      case AppAvCallState.calling:
        return '正在呼叫…';
      case AppAvCallState.inCall:
        return _client.remoteJoined.value ? '通话中' : '正在建立画面…';
      case AppAvCallState.ended:
        return '通话已结束';
      case AppAvCallState.error:
        return _client.lastError.value.isEmpty ? '连接失败' : _client.lastError.value;
    }
  }

  @override
  Widget build(BuildContext context) {
    final Widget remote = ValueListenableBuilder<bool>(
      valueListenable: _client.hasRemoteVideo,
      builder: (BuildContext context, bool hasVideo, _) {
        if (!hasVideo) {
          return Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: <Widget>[
                const CircularProgressIndicator(color: Colors.white70),
                SizedBox(height: AppSpacing.md),
                Text(
                  _starting ? '正在连接设备…' : '等待设备画面',
                  style: AppText.body.copyWith(color: Colors.white70),
                ),
              ],
            ),
          );
        }
        return RTCVideoView(
          _client.remoteRenderer,
          objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitContain,
        );
      },
    );

    return Scaffold(
      key: const Key('app-call-page'),
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Text(widget.deviceName),
      ),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            Expanded(child: remote),
            ValueListenableBuilder<AppAvCallState>(
              valueListenable: _client.state,
              builder: (BuildContext context, AppAvCallState state, _) {
                return Padding(
                  padding: EdgeInsets.all(AppSpacing.lg),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: <Widget>[
                      ValueListenableBuilder<String>(
                        valueListenable: _client.lastError,
                        builder: (BuildContext context, String err, _) {
                          return Text(
                            err.isNotEmpty ? err : _statusText,
                            textAlign: TextAlign.center,
                            style: AppText.body.copyWith(color: Colors.white70),
                          );
                        },
                      ),
                      SizedBox(height: AppSpacing.md),
                      _buildActions(state),
                    ],
                  ),
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildActions(AppAvCallState state) {
    switch (state) {
      case AppAvCallState.ready:
      case AppAvCallState.ended:
        return Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: <Widget>[
            _roundAction(
              key: const Key('app-call-invite'),
              icon: Icons.videocam,
              label: '呼叫',
              color: AppColors.success,
              onTap: () => _client.invite(),
            ),
          ],
        );
      case AppAvCallState.ringing:
        return Row(
          mainAxisAlignment: MainAxisAlignment.spaceEvenly,
          children: <Widget>[
            _roundAction(
              key: const Key('app-call-reject'),
              icon: Icons.call_end,
              label: '拒绝',
              color: AppColors.danger,
              onTap: () => _client.reject(),
            ),
            _roundAction(
              key: const Key('app-call-agree'),
              icon: Icons.call,
              label: '接听',
              color: AppColors.success,
              onTap: () => _client.agree(),
            ),
          ],
        );
      case AppAvCallState.calling:
      case AppAvCallState.inCall:
        return Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: <Widget>[
            _roundAction(
              key: const Key('app-call-hangup'),
              icon: Icons.call_end,
              label: '挂断',
              color: AppColors.danger,
              onTap: () => _client.hangUp(),
            ),
          ],
        );
      default:
        return const SizedBox(height: 56);
    }
  }

  Widget _roundAction({
    required Key key,
    required IconData icon,
    required String label,
    required Color color,
    required VoidCallback onTap,
  }) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: <Widget>[
        InkWell(
          key: key,
          onTap: onTap,
          customBorder: const CircleBorder(),
          child: Container(
            width: 64,
            height: 64,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            child: Icon(icon, color: Colors.white, size: 28),
          ),
        ),
        SizedBox(height: AppSpacing.xs),
        Text(label, style: AppText.hint.copyWith(color: Colors.white)),
      ],
    );
  }
}
