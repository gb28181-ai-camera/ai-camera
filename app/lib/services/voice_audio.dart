/// 语音聊天的音频链路（PCM16）。
///
/// 上行：麦克风采集 → PCM16 / 16kHz / 单声道 → 按 [voiceFrameBytes] 切帧
/// （20ms/640 字节）→ WebSocket 二进制帧。
///
/// 下行：WebSocket 二进制帧 → PCM16 / 24kHz / 单声道 → 边收边播。
///
/// 依赖选择：
/// - `record`：流式采集 PCM16（`AudioEncoder.pcm16bits`），BSD-3-Clause；
/// - `flutter_sound`：`startPlayerFromStream` + `feedUint8FromStream` 边收边播，
///   MPL-2.0。两者都提供官方 Android/iOS 实现。
///
/// 抽象出 [VoiceMicSource] / [VoiceTtsSink] 是为了让单元测试不依赖原生插件。
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:flutter_sound/flutter_sound.dart';
import 'package:record/record.dart';

import 'voice_ws_client.dart';

// ════════════════════════════════════════════════════════════════
// PCM 切帧
// ════════════════════════════════════════════════════════════════

/// 把任意长度的 PCM16 字节流切成固定大小帧。
///
/// 跨 chunk 的残余字节会保留到下一次 [add]，不会丢弃。
class PcmFrameSlicer {
  /// 构造切帧器。
  PcmFrameSlicer({this.frameBytes = voiceFrameBytes});

  /// 每帧字节数（默认 320）。
  final int frameBytes;

  Uint8List _pending = Uint8List(0);

  /// 尚未凑满一帧的字节数。
  int get pendingBytes => _pending.length;

  /// 追加一段 PCM16 字节，返回本次凑满的完整帧。
  List<Uint8List> add(Uint8List chunk) {
    if (chunk.isEmpty) return const <Uint8List>[];
    if (frameBytes <= 0) return <Uint8List>[chunk];

    final Uint8List merged = Uint8List(_pending.length + chunk.length)
      ..setRange(0, _pending.length, _pending)
      ..setRange(_pending.length, _pending.length + chunk.length, chunk);

    final List<Uint8List> frames = <Uint8List>[];
    int offset = 0;
    while (merged.length - offset >= frameBytes) {
      frames.add(Uint8List.sublistView(merged, offset, offset + frameBytes));
      offset += frameBytes;
    }
    _pending = offset == merged.length
        ? Uint8List(0)
        : Uint8List.fromList(merged.sublist(offset));
    return frames;
  }

  /// 清空残余字节。
  void reset() {
    _pending = Uint8List(0);
  }
}

// ════════════════════════════════════════════════════════════════
// 抽象接口（便于单测）
// ════════════════════════════════════════════════════════════════

/// 麦克风音源。
abstract class VoiceMicSource {
  /// 申请（或检查）麦克风权限。
  Future<bool> requestPermission();

  /// 开始采集；每个回调是一段 PCM16 字节（长度不保证等于一帧）。
  Future<void> start(void Function(Uint8List chunk) onChunk);

  /// 暂停采集（半双工静音）。
  Future<void> pause();

  /// 恢复采集。
  Future<void> resume();

  /// 停止并释放采集会话。
  Future<void> stop();

  /// 彻底释放资源。
  Future<void> dispose();
}

/// TTS 播放器。
abstract class VoiceTtsSink {
  /// 开始流式播放。
  Future<void> start({int sampleRate});

  /// 喂入 PCM16 数据。
  Future<void> feed(Uint8List pcm);

  /// 停止播放。
  Future<void> stop();

  /// 释放资源。
  Future<void> dispose();
}

// ════════════════════════════════════════════════════════════════
// record 实现：麦克风 → PCM16 流
// ════════════════════════════════════════════════════════════════

/// 基于 `record` 的麦克风音源。
class RecordVoiceMicSource implements VoiceMicSource {
  /// 构造音源。
  RecordVoiceMicSource({AudioRecorder? recorder}) : _recorder = recorder;

  AudioRecorder? _recorder;
  StreamSubscription<Uint8List>? _subscription;
  final PcmFrameSlicer _slicer = PcmFrameSlicer();
  bool _started = false;
  bool _paused = false;

  AudioRecorder get _ensureRecorder => _recorder ??= AudioRecorder();

  @override
  Future<bool> requestPermission() async {
    return _ensureRecorder.hasPermission();
  }

  @override
  Future<void> start(void Function(Uint8List chunk) onChunk) async {
    final AudioRecorder recorder = _ensureRecorder;
    await _cancelSubscription();
    _slicer.reset();
    _paused = false;

    // 不指定 streamBufferSize：交给平台按 AudioRecord 最小缓冲计算，
    // 过小的值会导致部分机型初始化失败；切帧由 [PcmFrameSlicer] 负责。
    final Stream<Uint8List> stream = await recorder.startStream(
      const RecordConfig(
        encoder: AudioEncoder.pcm16bits,
        sampleRate: voiceUpstreamSampleRate,
        numChannels: 1,
        // 语音对话不使用自动增益：音量突变会让服务端 VAD 误判。
        autoGain: false,
        echoCancel: true,
        noiseSuppress: true,
      ),
    );
    _started = true;
    _subscription = stream.listen(
      (Uint8List chunk) {
        if (_paused) return;
        for (final Uint8List frame in _slicer.add(chunk)) {
          onChunk(frame);
        }
      },
      onError: (Object _) {
        // 采集错误交由上层状态机处理（连接仍可继续）。
      },
    );
  }

  @override
  Future<void> pause() async {
    if (!_started || _paused) return;
    _paused = true;
    await _recorder?.pause();
  }

  @override
  Future<void> resume() async {
    if (!_started || !_paused) return;
    _paused = false;
    _slicer.reset();
    await _recorder?.resume();
  }

  @override
  Future<void> stop() async {
    if (!_started) return;
    _started = false;
    _paused = false;
    await _cancelSubscription();
    _slicer.reset();
    try {
      await _recorder?.stop();
    } catch (_) {
      // 已停止 / 未开始，忽略。
    }
  }

  Future<void> _cancelSubscription() async {
    final StreamSubscription<Uint8List>? sub = _subscription;
    _subscription = null;
    await sub?.cancel();
  }

  @override
  Future<void> dispose() async {
    await stop();
    final AudioRecorder? recorder = _recorder;
    _recorder = null;
    try {
      await recorder?.dispose();
    } catch (_) {
      // 释放失败忽略。
    }
  }
}

// ════════════════════════════════════════════════════════════════
// flutter_sound 实现：PCM16 流式播放
// ════════════════════════════════════════════════════════════════

/// 基于 `flutter_sound` 的 TTS 播放器。
class FlutterSoundTtsSink implements VoiceTtsSink {
  /// 构造播放器。
  FlutterSoundTtsSink({FlutterSoundPlayer? player}) : _player = player;

  FlutterSoundPlayer? _player;
  bool _started = false;

  FlutterSoundPlayer get _ensurePlayer => _player ??= FlutterSoundPlayer();

  @override
  Future<void> start({int sampleRate = voiceDownstreamSampleRate}) async {
    final FlutterSoundPlayer player = _ensurePlayer;
    if (!player.isOpen()) {
      await player.openPlayer();
    }
    if (_started) {
      await player.stopPlayer();
    }
    await player.startPlayerFromStream(
      codec: Codec.pcm16,
      interleaved: true,
      numChannels: 1,
      sampleRate: sampleRate,
      // 约 0.17s @24k：足够吸收网络抖动，又不至于明显滞后。
      bufferSize: 4096,
    );
    _started = true;
  }

  @override
  Future<void> feed(Uint8List pcm) async {
    if (!_started || pcm.isEmpty) return;
    await _ensurePlayer.feedUint8FromStream(pcm);
  }

  @override
  Future<void> stop() async {
    if (!_started) return;
    _started = false;
    try {
      await _player?.stopPlayer();
    } catch (_) {
      // 已停止，忽略。
    }
  }

  @override
  Future<void> dispose() async {
    await stop();
    final FlutterSoundPlayer? player = _player;
    _player = null;
    try {
      await player?.closePlayer();
    } catch (_) {
      // 释放失败忽略。
    }
  }
}

// ════════════════════════════════════════════════════════════════
// 音频会话
// ════════════════════════════════════════════════════════════════

/// 把麦克风与 TTS 播放器组合成一个会话。
///
/// 页面只依赖本类，不直接接触插件：
/// ```dart
/// final audio = VoiceAudioSession();
/// if (!await audio.requestPermission()) { ... }
/// await audio.startCapture(client.sendAudio);
/// await audio.startPlayback();
/// ```
class VoiceAudioSession {
  /// 构造音频会话。
  VoiceAudioSession({VoiceMicSource? mic, VoiceTtsSink? tts})
    : _mic = mic ?? RecordVoiceMicSource(),
      _tts = tts ?? FlutterSoundTtsSink();

  final VoiceMicSource _mic;
  final VoiceTtsSink _tts;

  bool _captureStarted = false;
  bool _capturePaused = false;
  bool _playbackStarted = false;
  int _playbackSampleRate = voiceDownstreamSampleRate;
  Future<void>? _startingPlayback;

  /// 是否已开始采集。
  bool get isCapturing => _captureStarted;

  /// 采集是否处于暂停（半双工）状态。
  bool get isCapturePaused => _capturePaused;

  /// 是否已开始播放。
  bool get isPlaying => _playbackStarted;

  /// 当前播放采样率。
  int get playbackSampleRate => _playbackSampleRate;

  /// 申请麦克风权限。
  Future<bool> requestPermission() => _mic.requestPermission();

  /// 开始采集；重复调用会重启采集会话。
  Future<void> startCapture(void Function(Uint8List frame) onFrame) async {
    await _mic.start(onFrame);
    _captureStarted = true;
    _capturePaused = false;
  }

  /// 暂停采集（`tts/start` 时调用）。
  Future<void> pauseCapture() async {
    if (!_captureStarted || _capturePaused) return;
    _capturePaused = true;
    await _mic.pause();
  }

  /// 恢复采集（`tts/stop` + 300ms 尾巴后调用）。
  Future<void> resumeCapture() async {
    if (!_captureStarted || !_capturePaused) return;
    _capturePaused = false;
    await _mic.resume();
  }

  /// 停止采集。
  Future<void> stopCapture() async {
    if (!_captureStarted) return;
    _captureStarted = false;
    _capturePaused = false;
    await _mic.stop();
  }

  /// 开始 TTS 流式播放。
  Future<void> startPlayback({int sampleRate = voiceDownstreamSampleRate}) {
    if (_playbackStarted && _playbackSampleRate == sampleRate) {
      return Future<void>.value();
    }
    // `tts/start` 与首个二进制帧可能几乎同时到达，复用同一个启动 Future，
    // 避免对底层播放器并发调用 start。
    final Future<void>? pending = _startingPlayback;
    if (pending != null) return pending;

    _playbackSampleRate = sampleRate;
    final Future<void> future = _tts.start(sampleRate: sampleRate).then((_) {
      _playbackStarted = true;
    });
    _startingPlayback = future;
    return future.whenComplete(() {
      _startingPlayback = null;
    });
  }

  /// 播放一帧下行 PCM16（自动按首次收到的采样率启动播放器）。
  Future<void> play(Uint8List pcm) async {
    if (pcm.isEmpty) return;
    if (!_playbackStarted) {
      await startPlayback();
    }
    await _tts.feed(pcm);
  }

  /// 停止播放。
  Future<void> stopPlayback() async {
    final Future<void>? pending = _startingPlayback;
    if (pending != null) {
      try {
        await pending;
      } catch (_) {
        // 启动失败时无需再停止。
      }
    }
    if (!_playbackStarted) return;
    _playbackStarted = false;
    await _tts.stop();
  }

  /// 释放全部资源。
  Future<void> dispose() async {
    await stopCapture();
    await stopPlayback();
    await _mic.dispose();
    await _tts.dispose();
  }
}
