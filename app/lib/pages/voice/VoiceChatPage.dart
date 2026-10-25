/// 语音聊天页（半双工）。
///
/// 交互：
/// - **按住说话**：长按麦克风按钮开始上行，松开发送（`listen/mode=manual`）；
/// - **点击开始 / 停止**：点击麦克风按钮切换（`listen/mode=auto`，由服务端 VAD 断句）；
/// - `tts/start` 时客户端关闭麦克风门闸，`tts/stop` 后保留 300ms 尾巴再恢复；
/// - 断线自动指数退避重连；也可点击右上角「重连」立即重试。
///
/// 页面只负责编排 [VoiceWsClient] 与 [VoiceAudioSession]，协议细节在服务层。
library;

import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../../layout/Gaps.dart';
import '../../services/voice_audio.dart';
import '../../services/voice_ws_client.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';

/// 字幕来源。
enum _LineKind {
  /// 用户（STT）。
  user,

  /// 助手（LLM / TTS）。
  assistant,

  /// 系统提示。
  system,
}

/// 一条字幕。
class _Line {
  /// 构造字幕。
  _Line(this.kind, this.text);

  /// 来源。
  final _LineKind kind;

  /// 文本（LLM 增量会就地追加）。
  String text;
}

/// 语音聊天页。
class VoiceChatPage extends StatefulWidget {
  /// 构造页面。
  const VoiceChatPage({super.key});

  @override
  State<VoiceChatPage> createState() => _VoiceChatPageState();
}

class _VoiceChatPageState extends State<VoiceChatPage>
    with WidgetsBindingObserver {
  late final VoiceWsClient _client;
  late final VoiceAudioSession _audio;
  final ScrollController _scroll = ScrollController();

  VoiceClientState _state = VoiceClientState.idle;
  final List<_Line> _lines = <_Line>[];
  final List<VoiceToolEvent> _tools = <VoiceToolEvent>[];

  int? _assistantIndex;
  String _sttPartial = '';
  String? _connectionError;
  String? _reconnectInfo;

  /// 用户是否处于「说话」意图（长按中或点击开启）。
  bool _talking = false;

  /// 正在申请权限 / 启动采集，防止连点并发启动。
  bool _startingTalk = false;

  /// 长按是否仍被按住（权限申请耗时较长时，用户可能已松手）。
  bool _longPressHeld = false;

  bool _permissionDenied = false;
  bool _disposed = false;

  Future<void> _playbackQueue = Future<void>.value();
  Timer? _playbackStopTimer;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);

    _client = VoiceWsClient()
      ..onStateChanged = _onStateChanged
      ..onServerHello = _onServerHello
      ..onStt = _onStt
      ..onLlm = _onLlm
      ..onTts = _onTts
      ..onTool = _onTool
      ..onError = _onError
      ..onWake = _onWake
      ..onAudio = _onAudio
      ..onMicGateChanged = _onMicGateChanged
      ..onReconnecting = _onReconnecting;

    _audio = VoiceAudioSession();
    unawaited(_client.connect());
  }

  @override
  void dispose() {
    _disposed = true;
    WidgetsBinding.instance.removeObserver(this);
    _playbackStopTimer?.cancel();
    _scroll.dispose();
    unawaited(_client.dispose());
    unawaited(_audio.dispose());
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    final bool paused = state != AppLifecycleState.resumed;
    _client.setPaused(paused);
    if (paused) {
      unawaited(_audio.pauseCapture());
    } else if (_client.micGateOpen && _talking) {
      unawaited(_audio.resumeCapture());
    }
  }

  // ══════════════════════════════════════════════════════════
  // 客户端回调
  // ══════════════════════════════════════════════════════════

  void _onStateChanged(VoiceClientState state) {
    if (_disposed || !mounted) return;
    setState(() {
      _state = state;
      if (state == VoiceClientState.ready ||
          state == VoiceClientState.listening) {
        _connectionError = null;
      }
      if (state == VoiceClientState.reconnecting ||
          state == VoiceClientState.error ||
          state == VoiceClientState.closed) {
        _talking = false;
      }
    });
    if (state == VoiceClientState.reconnecting ||
        state == VoiceClientState.error ||
        state == VoiceClientState.closed) {
      unawaited(_audio.pauseCapture());
    }
  }

  void _onReconnecting(int attempt, Duration delay, String reason) {
    if (_disposed || !mounted) return;
    setState(() {
      final int seconds = (delay.inMilliseconds / 1000).ceil();
      _reconnectInfo = '第 $attempt 次重连（${seconds}s 后）：$reason';
    });
  }

  void _onServerHello(VoiceServerHello hello) {
    if (_disposed || !mounted) return;
    setState(() {
      _connectionError = null;
      _reconnectInfo = null;
      _addSystemLine('语音助手已连接（下行 ${hello.audioParams.sampleRate}Hz）');
    });
  }

  void _onStt(VoiceSttEvent event) {
    if (_disposed || !mounted) return;
    if (!event.isFinal) {
      setState(() => _sttPartial = event.text);
      _scrollToBottom();
      return;
    }
    setState(() {
      _sttPartial = '';
      _assistantIndex = null;
      final String text = event.text.trim();
      if (text.isNotEmpty) {
        _lines.add(_Line(_LineKind.user, text));
      }
    });
    _scrollToBottom();
  }

  void _onLlm(VoiceLlmEvent event) {
    if (_disposed || !mounted) return;
    final String text = event.text;
    if (text.isEmpty) return;
    setState(() => _appendAssistant(text));
    _scrollToBottom();
  }

  void _onTts(VoiceTtsEvent event) {
    if (_disposed || !mounted) return;
    if (event.state == 'start') {
      _playbackStopTimer?.cancel();
      _playbackStopTimer = null;
      final int sampleRate =
          _client.serverHello?.audioParams.sampleRate ??
          voiceDownstreamSampleRate;
      unawaited(
        _audio.startPlayback(sampleRate: sampleRate).catchError((Object _) {}),
      );
    } else if (event.state == 'stop') {
      _schedulePlaybackStop();
    }

    final String text = event.text.trim();
    if (text.isEmpty) return;
    // 服务端未下发 LLM 文本时，用 TTS 句子兜底字幕。
    setState(() => _appendAssistant(text));
    _scrollToBottom();
  }

  /// 追加 LLM 文本，兼容「增量」与「累计全量」两种服务端实现。
  void _appendAssistant(String text) {
    final int? index = _assistantIndex;
    if (index == null || index >= _lines.length) {
      _lines.add(_Line(_LineKind.assistant, text));
      _assistantIndex = _lines.length - 1;
      return;
    }
    final String current = _lines[index].text;
    if (text == current || current.endsWith(text)) return;
    _lines[index].text = text.startsWith(current) ? text : '$current$text';
  }

  void _onTool(VoiceToolEvent event) {
    if (_disposed || !mounted) return;
    setState(() {
      _tools.insert(0, event);
      if (_tools.length > 5) _tools.removeLast();
    });
  }

  void _onError(VoiceErrorEvent event) {
    if (_disposed || !mounted) return;
    final String message = event.message.isNotEmpty
        ? event.message
        : '语音服务返回错误';
    setState(() {
      _connectionError = message;
      if (event.code == 'ws_reconnect_failed') {
        _state = VoiceClientState.error;
      }
    });
    context.showToast(message);
  }

  void _onWake(VoiceWakeEvent event) {
    if (_disposed || !mounted) return;
    setState(() {
      _addSystemLine(event.text.isNotEmpty ? '唤醒：${event.text}' : '设备已唤醒');
    });
  }

  void _onMicGateChanged(bool open) {
    if (_disposed) return;
    if (open) {
      if (_talking) {
        unawaited(_audio.resumeCapture().catchError((Object _) {}));
      }
    } else {
      unawaited(_audio.pauseCapture().catchError((Object _) {}));
    }
  }

  void _onAudio(Uint8List pcm) {
    if (_disposed) return;
    _playbackStopTimer?.cancel();
    _playbackStopTimer = null;
    // 串行播放，保证 PCM 帧顺序。
    _playbackQueue = _playbackQueue.then((_) async {
      if (_disposed) return;
      try {
        await _audio.play(pcm);
      } catch (_) {
        // 单帧播放失败不打断整条链路。
      }
    });
  }

  void _schedulePlaybackStop() {
    _playbackStopTimer?.cancel();
    _playbackStopTimer = Timer(const Duration(milliseconds: 600), () {
      if (_disposed) return;
      unawaited(_audio.stopPlayback().catchError((Object _) {}));
    });
  }

  void _addSystemLine(String text) {
    _lines.add(_Line(_LineKind.system, text));
    _scrollToBottom();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scroll.hasClients) return;
      _scroll.animateTo(
        _scroll.position.maxScrollExtent,
        duration: const Duration(milliseconds: 160),
        curve: Curves.easeOut,
      );
    });
  }

  // ══════════════════════════════════════════════════════════
  // 说话控制
  // ══════════════════════════════════════════════════════════

  Future<void> _beginTalk({required String mode}) async {
    if (_talking || _startingTalk) return;
    _startingTalk = true;
    try {
      if (!_client.isReady) {
        if (_state == VoiceClientState.connecting ||
            _state == VoiceClientState.reconnecting) {
          context.showToast('正在连接语音服务…');
          return;
        }
        await _client.connect();
        if (!mounted) return;
        if (!_client.isReady) {
          context.showToast('语音服务未连接，请点击重连');
          return;
        }
      }
      if (_client.state == VoiceClientState.speaking) {
        context.showToast('正在播放回复，请稍候');
        return;
      }

      bool granted;
      try {
        granted = await _audio.requestPermission();
      } catch (error) {
        if (!mounted) return;
        context.showToast('无法访问麦克风：$error');
        return;
      }
      if (!mounted) return;
      if (!granted) {
        setState(() => _permissionDenied = true);
        context.showToast('未获得麦克风权限，请在系统设置中开启');
        return;
      }

      try {
        if (!_audio.isCapturing) {
          await _audio.startCapture(_onMicFrame);
        } else {
          await _audio.resumeCapture();
        }
      } catch (error) {
        if (!mounted) return;
        context.showToast('麦克风启动失败：$error');
        return;
      }
      if (!mounted) return;

      // 长按启动期间用户已松手：不要进入聆听态，保持采集暂停。
      if (mode == 'manual' && !_longPressHeld) {
        unawaited(_audio.pauseCapture().catchError((Object _) {}));
        return;
      }

      setState(() {
        _talking = true;
        _permissionDenied = false;
        _connectionError = null;
      });
      _client.sendListenStart(mode: mode);
    } finally {
      _startingTalk = false;
    }
  }

  void _endTalk() {
    if (!_talking) return;
    _talking = false;
    _client.sendListenStop();
    unawaited(_audio.pauseCapture().catchError((Object _) {}));
    if (mounted) {
      setState(() {
        _sttPartial = '';
      });
    }
  }

  void _onMicFrame(Uint8List frame) {
    _client.sendAudio(frame);
  }

  void _onMicTap() {
    if (_talking) {
      _endTalk();
      return;
    }
    // 点击模式交给服务端 VAD 断句；长按模式由松手触发 listen/stop。
    unawaited(_beginTalk(mode: 'auto'));
  }

  void _onMicLongPressStart() {
    _longPressHeld = true;
    unawaited(_beginTalk(mode: 'manual'));
  }

  void _onMicLongPressEnd() {
    _longPressHeld = false;
    _endTalk();
  }

  Future<void> _reconnect() async {
    setState(() {
      _connectionError = null;
      _reconnectInfo = null;
      _assistantIndex = null;
      _sttPartial = '';
    });
    await _client.reconnectNow();
  }

  void _abort() {
    _client.abort();
    unawaited(_audio.stopPlayback().catchError((Object _) {}));
    if (mounted) {
      setState(() {
        _talking = false;
        _sttPartial = '';
      });
    }
    context.showToast('已打断');
  }

  // ══════════════════════════════════════════════════════════
  // UI
  // ══════════════════════════════════════════════════════════

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      key: const Key('voice-chat-page'),
      backgroundColor: AppColors.background,
      appBar: AppBar(
        title: const Text('语音聊天'),
        actions: <Widget>[
          if (_state == VoiceClientState.speaking ||
              _state == VoiceClientState.thinking)
            IconButton(
              key: const Key('voice-abort'),
              tooltip: '打断',
              onPressed: _abort,
              icon: const Icon(Icons.stop_circle_outlined),
            ),
          IconButton(
            key: const Key('voice-reconnect'),
            tooltip: '重新连接',
            onPressed: _reconnect,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: SafeArea(
        child: Column(
          children: <Widget>[
            _buildStatusBar(),
            if (_connectionError != null) _buildErrorBanner(),
            if (_tools.isNotEmpty) _buildToolBar(),
            Expanded(child: _buildTranscript()),
            _buildFooter(),
          ],
        ),
      ),
    );
  }

  Widget _buildStatusBar() {
    final _StatusView view = _statusView(_state);
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.md,
      ),
      color: AppColors.card,
      child: Row(
        children: <Widget>[
          Container(
            width: 34.dp,
            height: 34.dp,
            decoration: BoxDecoration(
              color: view.color.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(AppSpacing.radiusRound),
            ),
            child: Icon(view.icon, size: 20.dp, color: view.color),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(view.label, style: AppText.bodyStrong),
                SizedBox(height: 2.dp),
                Text(
                  _statusHint(view),
                  style: AppText.hint,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          if (_permissionDenied)
            Text(
              '麦克风未授权',
              style: AppText.tag.copyWith(color: AppColors.danger),
            ),
        ],
      ),
    );
  }

  String _statusHint(_StatusView view) {
    final String? sessionId = _client.serverHello?.sessionId;
    switch (_state) {
      case VoiceClientState.idle:
        return '准备连接语音服务';
      case VoiceClientState.connecting:
        return '正在获取票据并握手…';
      case VoiceClientState.ready:
        return sessionId == null || sessionId.isEmpty
            ? '按住下方麦克风开始说话'
            : '会话 $sessionId · 按住下方麦克风开始说话';
      case VoiceClientState.listening:
        return _talking ? '正在聆听，请说话…' : '正在聆听…';
      case VoiceClientState.thinking:
        return '正在理解并生成回复…';
      case VoiceClientState.speaking:
        return '正在播放回复（麦克风已暂停）';
      case VoiceClientState.reconnecting:
        return _reconnectInfo ?? '连接中断，正在自动重连…';
      case VoiceClientState.error:
        return _connectionError ?? '连接失败，请点击重连';
      case VoiceClientState.closed:
        return '连接已关闭';
    }
  }

  Widget _buildErrorBanner() {
    return Container(
      width: double.infinity,
      color: AppColors.dangerLight,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.sm,
      ),
      child: Row(
        children: <Widget>[
          const Icon(Icons.error_outline, color: AppColors.danger, size: 18),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              _connectionError!,
              style: AppText.hint.copyWith(color: AppColors.danger),
            ),
          ),
          TextButton(
            key: const Key('voice-error-reconnect'),
            onPressed: _reconnect,
            child: const Text('重连'),
          ),
        ],
      ),
    );
  }

  Widget _buildToolBar() {
    return SizedBox(
      height: 46.dp,
      child: ListView.separated(
        key: const Key('voice-tool-list'),
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(
          horizontal: AppSpacing.pagePadding,
          vertical: AppSpacing.sm,
        ),
        itemCount: _tools.length,
        separatorBuilder: (BuildContext context, int index) =>
            SizedBox(width: AppSpacing.sm),
        itemBuilder: (BuildContext context, int index) {
          final VoiceToolEvent event = _tools[index];
          final bool busy = event.state == 'calling';
          final bool failed = event.state == 'error';
          final Color color = failed
              ? AppColors.danger
              : busy
              ? AppColors.warning
              : AppColors.success;
          final String label = event.display.isNotEmpty
              ? event.display
              : (event.name.isNotEmpty ? event.name : 'MCP 工具');
          return Chip(
            avatar: busy
                ? const SizedBox(
                    width: 12,
                    height: 12,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : Icon(
                    failed ? Icons.error_outline : Icons.build_circle_outlined,
                    size: 14,
                    color: color,
                  ),
            label: Text(
              busy ? '调用 $label…' : (failed ? '$label 失败' : '$label 完成'),
              style: AppText.tag.copyWith(color: color),
            ),
            visualDensity: VisualDensity.compact,
          );
        },
      ),
    );
  }

  Widget _buildTranscript() {
    final bool empty = _lines.isEmpty && _sttPartial.isEmpty;
    if (empty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(
              Icons.record_voice_over_outlined,
              size: 56.dp,
              color: AppColors.textTertiary,
            ),
            SizedBox(height: AppSpacing.md),
            Text('按住麦克风说话，松开发送', style: AppText.empty),
            SizedBox(height: AppSpacing.xs),
            Text('也可以点击麦克风开始 / 停止', style: AppText.hint),
          ],
        ),
      );
    }

    final int count = _lines.length + (_sttPartial.isEmpty ? 0 : 1);
    return ListView.builder(
      key: const Key('voice-transcript'),
      controller: _scroll,
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      itemCount: count,
      itemBuilder: (BuildContext context, int index) {
        if (index >= _lines.length) {
          return _buildLine(_Line(_LineKind.user, _sttPartial), pending: true);
        }
        return _buildLine(_lines[index]);
      },
    );
  }

  Widget _buildLine(_Line line, {bool pending = false}) {
    final bool user = line.kind == _LineKind.user;
    final bool system = line.kind == _LineKind.system;
    if (system) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Center(child: Text(line.text, style: AppText.hint)),
      );
    }

    final Color background = user ? AppColors.primary : AppColors.card;
    final Color foreground = user
        ? AppColors.textOnPrimary
        : AppColors.textPrimary;
    return Align(
      alignment: user ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        margin: EdgeInsets.symmetric(vertical: AppSpacing.xs),
        padding: EdgeInsets.symmetric(
          horizontal: AppSpacing.md,
          vertical: AppSpacing.sm,
        ),
        constraints: BoxConstraints(
          maxWidth: MediaQuery.of(context).size.width * 0.78,
        ),
        decoration: BoxDecoration(
          color: background,
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        ),
        child: Text(
          pending ? '${line.text}…' : line.text,
          style: AppText.body.copyWith(
            color: pending ? AppColors.textTertiary : foreground,
            fontStyle: pending ? FontStyle.italic : FontStyle.normal,
          ),
        ),
      ),
    );
  }

  Widget _buildFooter() {
    final bool busy = _state == VoiceClientState.connecting;
    final bool speaking = _state == VoiceClientState.speaking;
    final Color color = _talking ? AppColors.danger : AppColors.primary;
    final String hint;
    if (_state == VoiceClientState.reconnecting) {
      hint = '正在重连，恢复后可继续说话';
    } else if (_state == VoiceClientState.error) {
      hint = '连接失败，请先点击重连';
    } else if (speaking) {
      hint = '正在播放回复，麦克风已自动静音';
    } else if (_talking) {
      hint = '松开结束 · 再次点击结束';
    } else {
      hint = '按住说话 · 点击开始 / 停止';
    }

    return Container(
      width: double.infinity,
      color: AppColors.card,
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.lg,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Text(hint, style: AppText.hint),
          SizedBox(height: AppSpacing.md),
          GestureDetector(
            key: const Key('voice-mic-button'),
            onTap: busy ? null : _onMicTap,
            onLongPressStart: busy
                ? null
                : (LongPressStartDetails _) => _onMicLongPressStart(),
            onLongPressEnd: busy
                ? null
                : (LongPressEndDetails _) => _onMicLongPressEnd(),
            child: AnimatedContainer(
              duration: const Duration(milliseconds: 150),
              width: 76.dp,
              height: 76.dp,
              decoration: BoxDecoration(
                color: color,
                shape: BoxShape.circle,
                boxShadow: <BoxShadow>[
                  BoxShadow(
                    color: color.withValues(alpha: 0.25),
                    blurRadius: 16,
                    offset: const Offset(0, 6),
                  ),
                ],
              ),
              child: busy
                  ? const Padding(
                      padding: EdgeInsets.all(24),
                      child: CircularProgressIndicator(
                        strokeWidth: 3,
                        color: AppColors.textOnPrimary,
                      ),
                    )
                  : Icon(
                      _talking ? Icons.stop : Icons.mic,
                      size: 34.dp,
                      color: AppColors.textOnPrimary,
                    ),
            ),
          ),
          if (_state == VoiceClientState.reconnecting ||
              _state == VoiceClientState.error) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            TextButton(
              key: const Key('voice-footer-reconnect'),
              onPressed: _reconnect,
              child: const Text('重新连接'),
            ),
          ],
        ],
      ),
    );
  }

  _StatusView _statusView(VoiceClientState state) {
    switch (state) {
      case VoiceClientState.idle:
        return const _StatusView(
          '未连接',
          Icons.cloud_off_outlined,
          AppColors.textTertiary,
        );
      case VoiceClientState.connecting:
        return const _StatusView(
          '连接中',
          Icons.cloud_sync_outlined,
          AppColors.primary,
        );
      case VoiceClientState.ready:
        return const _StatusView(
          '已连接',
          Icons.cloud_done_outlined,
          AppColors.success,
        );
      case VoiceClientState.listening:
        return const _StatusView('聆听中', Icons.graphic_eq, AppColors.primary);
      case VoiceClientState.thinking:
        return const _StatusView(
          '思考中',
          Icons.psychology_outlined,
          AppColors.warning,
        );
      case VoiceClientState.speaking:
        return const _StatusView(
          '说话中',
          Icons.volume_up_outlined,
          AppColors.success,
        );
      case VoiceClientState.reconnecting:
        return const _StatusView(
          '重连中',
          Icons.sync_problem_outlined,
          AppColors.warning,
        );
      case VoiceClientState.error:
        return const _StatusView('错误', Icons.error_outline, AppColors.danger);
      case VoiceClientState.closed:
        return const _StatusView('已断开', Icons.link_off, AppColors.textTertiary);
    }
  }
}

/// 状态栏展示数据。
class _StatusView {
  const _StatusView(this.label, this.icon, this.color);

  final String label;
  final IconData icon;
  final Color color;
}
