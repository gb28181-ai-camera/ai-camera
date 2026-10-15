import 'package:flutter/material.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

/// ⚠️⚠️ 临时 Spike 页面（验证完可整文件删除）⚠️⚠️
///
/// 目的：验证 v0.2.0 设计文档 §9.3 指出的**共存风险** —— 实时播放用的
/// `flutter_webrtc` 与回放用的 `media_kit` 能否在同一页面内并存。
///
/// 设计文档原文记录的两条已知风险：
/// 1. `media_kit_video` 在**非平台线程**发消息会触发 Flutter engine 报错
///    （`Platform channel messages must be sent on the platform thread…`），
///    出现该报错的工程正是与 `flutter_webrtc` 组合；
/// 2. 两者在 Android 上**都争用 MediaCodec 硬解实例**（移动端并发硬解常见仅 2–4 个），
///    同页一个 WebRTC 流 + 一个 FLV/MP4 流就可能失败。
///
/// 验证方法（重点点「两者同时启动」）：
/// - 单独启动 WebRTC（本地摄像头，真实走硬编）—— 确认基线可用；
/// - 单独启动 media_kit（网络流，真实走硬解）—— 确认基线可用；
/// - **同时启动两者** —— 观察是否出现上述 engine 报错 / 黑屏 / 解码失败；
/// - 反复「全部停止 → 全部启动」若干轮 —— 观察是否泄漏（MediaCodec 实例耗尽后
///   表现为后启动的一侧黑屏或抛错）。
///
/// 路由注册在 `app_router.dart` 的 `/spike/player`（该条亦为临时，删除时一并移除），
/// 也可用独立入口 `lib/spike_main.dart` 直接启动（免登录、免后端）。
class PlayerSpikePage extends StatefulWidget {
  /// 构造 Spike 页面。
  const PlayerSpikePage({super.key, this.autoStart = false});

  /// 首帧后自动「同时启动」两种内核。供 `lib/spike_main.dart` 无人值守跑测使用。
  final bool autoStart;

  @override
  State<PlayerSpikePage> createState() => _PlayerSpikePageState();
}

/// 占位媒体源（回放侧）。任意可播放的 mp4 / m3u8 / flv 均可。
///
/// 可用 `--dart-define=SPIKE_MEDIA_URL=<url>` 覆盖，指向本项目真实的
/// HTTP-FLV / HLS 回放地址，以贴近生产场景复现。
///
/// 注：默认值特意不选 `commondatastorage.googleapis.com` 那批常见测试片源 ——
/// 国内网络下该域名被解析到不可达地址（已实测 host 与模拟器均连不上），
/// media_kit 会表现为「启动成功但画面全黑」，容易被误判为共存冲突。
const String _kSampleMediaUrl = String.fromEnvironment(
  'SPIKE_MEDIA_URL',
  defaultValue: 'https://media.w3.org/2010/05/bunny/movie.mp4',
);

/// Spike 页面状态：分别持有 WebRTC 与 media_kit 两套渲染资源。
class _PlayerSpikePageState extends State<PlayerSpikePage> {
  // ── media_kit（回放侧） ──
  Player? _player;
  VideoController? _videoController;

  // ── flutter_webrtc（实时侧） ──
  RTCVideoRenderer? _renderer;
  MediaStream? _stream;

  bool _mediaKitBusy = false;
  bool _webRtcBusy = false;
  String? _mediaKitError;
  String? _webRtcError;

  /// media_kit 当前是否在运行。
  bool get _mediaKitOn => _player != null;

  /// WebRTC 当前是否在运行。
  bool get _webRtcOn => _renderer != null;

  @override
  void initState() {
    super.initState();
    if (widget.autoStart) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        _startBoth();
      });
    }
  }

  @override
  void dispose() {
    // 兜底释放，正常路径走「全部停止」。
    _player?.dispose();
    _releaseWebRtc();
    super.dispose();
  }

  // ────────────────────────── media_kit ──────────────────────────

  /// 启动 media_kit 播放器与视频视图。
  Future<void> _startMediaKit() async {
    if (_mediaKitBusy || _mediaKitOn) return;
    setState(() {
      _mediaKitBusy = true;
      _mediaKitError = null;
    });
    try {
      // Player 依赖原生 libmpv（由 media_kit_libs_video 及其各平台分包提供）。
      // ensureInitialized 幂等，重复调用安全。
      MediaKit.ensureInitialized();
      final Player player = Player();
      final VideoController controller = VideoController(player);
      await player.open(Media(_kSampleMediaUrl));
      if (!mounted) {
        await player.dispose();
        return;
      }
      setState(() {
        _player = player;
        _videoController = controller;
      });
      debugPrint('[SPIKE] media_kit started: $_kSampleMediaUrl');
    } catch (e) {
      debugPrint('[SPIKE] media_kit FAILED: $e');
      setState(() => _mediaKitError = 'media_kit 启动失败: $e');
    } finally {
      if (mounted) {
        setState(() => _mediaKitBusy = false);
      }
    }
  }

  /// 停止并释放 media_kit。
  ///
  /// 先置空让 `Video` 组件从树上摘除，等本帧结束再 dispose，避免组件在
  /// 已释放的 controller 上取值。
  Future<void> _stopMediaKit() async {
    final Player? player = _player;
    if (player == null) return;
    setState(() {
      _player = null;
      _videoController = null;
    });
    await WidgetsBinding.instance.endOfFrame;
    // media_kit_video 2.x 的 VideoController 无独立 dispose，随 Player 释放。
    await player.dispose();
  }

  // ────────────────────────── flutter_webrtc ──────────────────────────

  /// 启动 WebRTC 渲染器并接入本地摄像头画面。
  ///
  /// 用本地摄像头（而非空白 renderer）是为了真实触发硬编码/硬解码路径，
  /// 从而真实复现与 media_kit 的 MediaCodec 争用。
  Future<void> _startWebRtc() async {
    if (_webRtcBusy || _webRtcOn) return;
    setState(() {
      _webRtcBusy = true;
      _webRtcError = null;
    });
    final RTCVideoRenderer renderer = RTCVideoRenderer();
    try {
      await renderer.initialize();
      // getUserMedia 会在 Android/iOS 上触发系统权限弹窗（RECORD_AUDIO/CAMERA）。
      final MediaStream stream = await navigator.mediaDevices.getUserMedia(
        <String, dynamic>{'audio': false, 'video': true},
      );
      renderer.srcObject = stream;
      if (!mounted) {
        await _releaseStream(stream);
        await renderer.dispose();
        return;
      }
      setState(() {
        _renderer = renderer;
        _stream = stream;
      });
      debugPrint('[SPIKE] webrtc started: ${stream.id}');
    } catch (e) {
      debugPrint('[SPIKE] webrtc FAILED: $e');
      setState(() => _webRtcError = 'WebRTC 启动失败: $e');
      await renderer.dispose();
    } finally {
      if (mounted) {
        setState(() => _webRtcBusy = false);
      }
    }
  }

  /// 停止并释放 WebRTC 资源。
  Future<void> _stopWebRtc() async {
    setState(() {
      _renderer = null;
      _stream = null;
    });
    await _releaseWebRtc();
  }

  /// 释放 WebRTC 渲染器与媒体流（不触发 setState，供 dispose 复用）。
  Future<void> _releaseWebRtc() async {
    final RTCVideoRenderer? renderer = _renderer;
    final MediaStream? stream = _stream;
    _renderer = null;
    _stream = null;
    if (renderer != null) {
      renderer.srcObject = null;
    }
    if (stream != null) {
      await _releaseStream(stream);
    }
    if (renderer != null) {
      await renderer.dispose();
    }
  }

  /// 停掉流内所有轨道后销毁流。
  Future<void> _releaseStream(MediaStream stream) async {
    for (final MediaStreamTrack track in stream.getTracks()) {
      await track.stop();
    }
    await stream.dispose();
  }

  // ────────────────────────── 组合操作 ──────────────────────────

  /// 同时启动两种内核（关键验证场景）。
  Future<void> _startBoth() async {
    await _startWebRtc();
    await _startMediaKit();
  }

  /// 全部停止。
  Future<void> _stopAll() async {
    await _stopWebRtc();
    await _stopMediaKit();
  }

  // ────────────────────────── UI ──────────────────────────

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('播放器共存 Spike'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(12),
        children: <Widget>[
          const _IntroCard(),
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: <Widget>[
              FilledButton.icon(
                onPressed: _webRtcBusy
                    ? null
                    : (_webRtcOn ? _stopWebRtc : _startWebRtc),
                icon: Icon(_webRtcOn ? Icons.stop : Icons.play_arrow),
                label: Text(_webRtcOn ? '停止 WebRTC' : '启动 WebRTC'),
              ),
              FilledButton.icon(
                onPressed: _mediaKitBusy
                    ? null
                    : (_mediaKitOn ? _stopMediaKit : _startMediaKit),
                icon: Icon(_mediaKitOn ? Icons.stop : Icons.play_arrow),
                label: Text(_mediaKitOn ? '停止 media_kit' : '启动 media_kit'),
              ),
              FilledButton.tonal(
                onPressed: (_webRtcBusy || _mediaKitBusy) ? null : _startBoth,
                child: const Text('两者同时启动'),
              ),
              OutlinedButton(
                onPressed: (_webRtcBusy || _mediaKitBusy) ? null : _stopAll,
                child: const Text('全部停止'),
              ),
            ],
          ),
          const SizedBox(height: 12),
          _ViewSlot(
            title: 'WebRTC 实时视图（flutter_webrtc）',
            running: _webRtcOn,
            error: _webRtcError,
            busy: _webRtcBusy,
            child: _renderer == null
                ? null
                : RTCVideoView(
                    _renderer!,
                    mirror: true,
                    objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitCover,
                  ),
          ),
          const SizedBox(height: 12),
          _ViewSlot(
            title: '回放视图（media_kit / libmpv）',
            running: _mediaKitOn,
            error: _mediaKitError,
            busy: _mediaKitBusy,
            child: _videoController == null
                ? null
                : Video(
                    controller: _videoController!,
                    controls: NoVideoControls,
                    fit: BoxFit.cover,
                  ),
          ),
        ],
      ),
    );
  }
}

/// 顶部说明卡片。
class _IntroCard extends StatelessWidget {
  /// 构造说明卡片。
  const _IntroCard();

  @override
  Widget build(BuildContext context) {
    return Card(
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text(
              'Spike：两种播放内核同页共存',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            const SizedBox(height: 6),
            Text(
              '占位媒体源：$_kSampleMediaUrl\n'
              '（可用 --dart-define=SPIKE_MEDIA_URL=<url> 覆盖）\n\n'
              '请重点测试「两者同时启动」：观察是否出现 '
              'Platform channel ... platform thread 报错，或硬解实例争用导致的黑屏。',
              style: Theme.of(context).textTheme.bodySmall,
            ),
          ],
        ),
      ),
    );
  }
}

/// 单个渲染槽位：标题 + 16:9 画面区 + 状态/错误提示。
class _ViewSlot extends StatelessWidget {
  /// 构造渲染槽位。
  const _ViewSlot({
    required this.title,
    required this.running,
    required this.busy,
    required this.child,
    this.error,
  });

  /// 标题。
  final String title;

  /// 是否正在运行。
  final bool running;

  /// 是否正在启动中。
  final bool busy;

  /// 画面内容；为 null 时展示占位。
  final Widget? child;

  /// 错误信息。
  final String? error;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        Row(
          children: <Widget>[
            Expanded(
              child: Text(
                title,
                style: Theme.of(context).textTheme.titleSmall,
              ),
            ),
            _StatusChip(running: running, busy: busy),
          ],
        ),
        const SizedBox(height: 6),
        AspectRatio(
          aspectRatio: 16 / 9,
          child: Container(
            color: Colors.black,
            alignment: Alignment.center,
            child: child ??
                Text(
                  busy ? '启动中…' : '未启动',
                  style: const TextStyle(color: Colors.white54),
                ),
          ),
        ),
        if (error != null) ...<Widget>[
          const SizedBox(height: 4),
          Text(
            error!,
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
      ],
    );
  }
}

/// 运行状态小标签。
class _StatusChip extends StatelessWidget {
  /// 构造状态标签。
  const _StatusChip({required this.running, required this.busy});

  /// 是否正在运行。
  final bool running;

  /// 是否正在启动中。
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final String text = busy ? '启动中' : (running ? '运行中' : '已停止');
    final Color color = busy
        ? Colors.orange
        : (running ? Colors.green : Colors.grey);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(10),
      ),
      child: Text(
        text,
        style: TextStyle(color: color, fontSize: 12),
      ),
    );
  }
}
