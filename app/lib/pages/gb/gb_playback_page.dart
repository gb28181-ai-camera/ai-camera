import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:go_router/go_router.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../../layout/Gaps.dart';
import '../../services/generated/GbController.dart';
import '../../services/generated/definitions/AppGb.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/empty_state.dart';
import '../../widgets/entity_list_page.dart';
import '../../widgets/stat_card.dart';
import 'gb_whep_client.dart';

/// 回放播放器当前使用的内核。
enum _PlaybackMode {
  /// 尚未选择片段 / 已主动停止。
  idle,
  /// 正在向设备发 INVITE，或正在做 WebRTC 握手。
  connecting,
  /// WebRTC（flutter_webrtc + WHEP）。
  webrtc,
  /// HTTP-FLV / HLS（media_kit / libmpv）。
  flv,
  /// 两种内核都失败。
  error,
  /// App 切到后台，回放已停止。
  paused,
}

/// 国标录像回放页。
///
/// 路由：`/gb/playback?channelId=<通道主键>&name=<通道名称>`。
/// 从首页进入且未带通道参数时，页面先展示通道选择列表（见 [_buildChannelPicker]）。
///
/// ## 播放器复用方式
/// 实时播放页 `gb_live_page.dart` 里的播放内核（WHEP 优先、HTTP-FLV 回退）
/// 与页面状态、云台、对讲等耦合在一起。本期文件所有权不允许修改实时播放页，
/// 因此这里**复制了最小可用的播放实现**（`_tryWebRtc` / `_tryMediaKit` /
/// `_disposePlayer`），并直接复用 `gb_whep_client.dart`（未修改）。
/// 后续若允许改动实时播放页，应把这两段抽成共享组件并删除本文件的副本。
///
/// ## 会话生命周期（stream_id）
/// - `POST /app/gb/playback` 返回的 `stream_id` 是 SIP 对话的 Call-ID，
///   停止回放**必须**原样回传；
/// - 每次成功开始回放后立即保存；主动停止 / 离开页面 / 切后台时消费一次并清空，
///   避免重复 stop；
/// - 若 `startPlayback` 的响应回来时页面已销毁或已开始新会话（代际计数不匹配），
///   必须立刻用该 `stream_id` 补发 stop，否则 ZLM 与设备侧会残留会话。
///
/// ## 时间与超时语义
/// - 录像检索 / 回放的时间格式固定为 `2006-01-02T15:04:05`（GB28181 XML 原样解析）；
/// - 后端等待设备响应最多 5 秒，超时返回 `timeout=true` + 已收到的部分条目，
///   而不是 HTTP 错误 ——「设备没回」是业务结果，需要单独提示用户缩小时间范围。
class GbPlaybackPage extends StatefulWidget {
  /// 构造回放页。
  ///
  /// [channelId] / [channelName] 仅用于测试或调用方直接传参；
  /// 正常路由场景下由 `GoRouterState` 读取，保持与实时播放页一致的入参形式。
  const GbPlaybackPage({
    super.key,
    this.channelId,
    this.channelName,
  });

  /// 通道主键（可选）。
  final int? channelId;

  /// 通道名称（可选）。
  final String? channelName;

  @override
  State<GbPlaybackPage> createState() => _GbPlaybackPageState();
}

class _GbPlaybackPageState extends State<GbPlaybackPage>
    with WidgetsBindingObserver {
  // ── 路由 / 通道 ────────────────────────────────────────────
  int? _channelId;
  String _channelTitle = '国标通道';
  bool _initialized = false;

  // ── 录像检索 ──────────────────────────────────────────────
  DateTime _recordDate = DateTime.now();
  TimeOfDay _startTime = const TimeOfDay(hour: 0, minute: 0);
  TimeOfDay _endTime = const TimeOfDay(hour: 23, minute: 59);
  bool _querying = false;
  String? _queryError;
  AppGbRecordQueryResult? _records;

  /// 检索代际：日期/筛选连续变化时丢弃过期响应。
  int _queryGeneration = 0;

  // ── 回放会话 ──────────────────────────────────────────────
  AppGbRecord? _selected;
  String? _streamId;
  _PlaybackMode _mode = _PlaybackMode.idle;
  String _status = '';
  String? _error;
  String? _webrtcError;
  String? _flvError;

  /// 是否希望播放；切后台后置 false，用于丢弃仍在飞行中的启动流程。
  bool _wantsPlay = false;

  /// 防止并发启动。
  bool _starting = false;

  /// 代际计数：停止 / 重新开始 / 切后台时递增，用于丢弃过期异步回调。
  int _generation = 0;

  // ── 进度控制 ──────────────────────────────────────────────
  DateTime? _segmentStart;
  int _segmentDuration = 0;
  double _positionSeconds = 0;
  bool _paused = false;
  bool _dragging = false;
  bool _controlBusy = false;
  double _speed = 1;
  Timer? _ticker;
  DateTime _lastTick = DateTime.now();

  // ── 播放内核（与 gb_live_page.dart 的副本，原因见类注释）────────
  RTCVideoRenderer? _renderer;
  GbWhepClient? _whep;
  Player? _player;
  VideoController? _videoController;
  StreamSubscription<String>? _playerErrorSub;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_initialized) return;
    _initialized = true;

    // 构造参数优先（测试 / 已知通道场景），否则从路由读取。
    if (widget.channelId != null) {
      _channelId = widget.channelId;
      final String? name = widget.channelName;
      if (name != null && name.isNotEmpty) _channelTitle = name;
    } else {
      final GoRouterState state = GoRouterState.of(context);
      _channelId = int.tryParse(
        state.uri.queryParameters['channelId'] ?? '',
      );
      final String? name = state.uri.queryParameters['name'];
      if (name != null && name.isNotEmpty) _channelTitle = name;
    }

    // 未指定通道时展示通道选择列表，选中后再检索录像。
    if (_channelId != null) {
      unawaited(_queryRecords());
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _generation++;
    _queryGeneration++;
    _stopTicker();

    final GbWhepClient? whep = _whep;
    _whep = null;
    if (whep != null) {
      unawaited(whep.close().catchError((Object _) {}));
    }

    final RTCVideoRenderer? renderer = _renderer;
    _renderer = null;
    if (renderer != null) {
      try {
        renderer.srcObject = null;
      } catch (_) {
        // renderer 可能已被释放，忽略。
      }
      unawaited(renderer.dispose().catchError((Object _) {}));
    }

    _releasePlayerNow();
    // 离开页面必须停止回放，否则 ZLM 侧流与设备侧会话都不会归还。
    _stopBackendQuietly();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // 与实时播放页一致：只处理真正的后台状态。
    if (state == AppLifecycleState.paused ||
        state == AppLifecycleState.hidden ||
        state == AppLifecycleState.detached) {
      unawaited(_suspend());
    }
  }

  // ══════════════════════════════════════════════════════════
  // 通道选择（首页进入时未带 channelId）
  // ══════════════════════════════════════════════════════════

  Widget _buildChannelPicker() {
    return EntityListPage<AppGbChannel>(
      title: '录像回放',
      fetcher: _fetchChannels,
      emptyText: '暂无可选通道',
      emptyIcon: Icons.videocam_off_outlined,
      header: Container(
        color: AppColors.card,
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.md,
          AppSpacing.pagePadding,
          AppSpacing.md,
        ),
        child: Text(
          '请选择要检索录像的国标通道',
          style: AppText.hint,
        ),
      ),
      separator: SizedBox(height: AppSpacing.xs),
      padding: EdgeInsets.only(
        bottom: AppSpacing.xxl,
        top: AppSpacing.xs,
      ),
      itemBuilder: (
        BuildContext context,
        AppGbChannel channel,
        int index,
      ) =>
          _buildChannelTile(channel),
    );
  }

  Future<PageDataLike<AppGbChannel>> _fetchChannels(
    int page,
    int pageSize,
  ) async {
    final RAppGbChannelPage resp = await GbController.channelsUsingGet(
      page: page,
      pageSize: pageSize,
    );
    return PageDataAdapter<AppGbChannel>(
      total: resp.total,
      items: resp.items,
      hasMore: resp.page.hasMore,
    );
  }

  Widget _buildChannelTile(AppGbChannel channel) {
    final Color statusColor = AppColors.deviceStatusColor(
      channel.isOnline ? 'online' : 'offline',
    );

    return EntityCard(
      key: Key('gb-playback-channel-${channel.id}'),
      onTap: () => _selectChannel(channel),
      child: Row(
        children: <Widget>[
          Container(
            width: 40.dp,
            height: 40.dp,
            decoration: BoxDecoration(
              color: statusColor.withValues(alpha: 0.12),
              borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
            ),
            child: Icon(
              Icons.videocam_outlined,
              size: 20.dp,
              color: statusColor,
            ),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text(
                  channel.name.orPlaceholder(channel.gbChannelId),
                  style: AppText.bodyStrong,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                SizedBox(height: 2.dp),
                Text(
                  channel.deviceName.orPlaceholder('未知设备'),
                  style: AppText.hint,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          StatusTag(
            text: channel.isOnline ? '在线' : '离线',
            color: statusColor,
          ),
          SizedBox(width: AppSpacing.xs),
          Icon(
            Icons.chevron_right,
            size: 20.dp,
            color: AppColors.textTertiary,
          ),
        ],
      ),
    );
  }

  void _selectChannel(AppGbChannel channel) {
    setState(() {
      _channelId = channel.id;
      _channelTitle = channel.name.isNotEmpty
          ? channel.name
          : channel.gbChannelId;
    });
    unawaited(_queryRecords());
  }

  // ══════════════════════════════════════════════════════════
  // 录像检索
  // ══════════════════════════════════════════════════════════

  /// 按当前日期与时间范围检索录像。
  Future<void> _queryRecords() async {
    final int? channelId = _channelId;
    if (channelId == null || _querying) return;

    // 结束时间默认补到 59 秒：用户选择的 23:59 语义上应包含整分钟。
    final DateTime start = _combine(_recordDate, _startTime);
    final DateTime end = _combine(_recordDate, _endTime, second: 59);
    if (!end.isAfter(start)) {
      if (mounted) context.showToast('结束时间必须晚于开始时间');
      return;
    }

    final int gen = ++_queryGeneration;
    setState(() {
      _querying = true;
      _queryError = null;
      _records = null;
    });

    try {
      final RAppGbRecordQuery resp = await GbController.recordsUsingGet(
        channelId: channelId,
        startTime: _gbTime(start),
        endTime: _gbTime(end),
      );
      if (!mounted || gen != _queryGeneration) return;
      if (!resp.isSuccess || resp.value == null) {
        setState(() {
          _querying = false;
          _queryError = resp.errorMessage;
        });
        return;
      }
      setState(() {
        _querying = false;
        _records = resp.value;
      });
    } catch (e) {
      if (!mounted || gen != _queryGeneration) return;
      setState(() {
        _querying = false;
        _queryError = '检索失败：$e';
      });
    }
  }

  Future<void> _pickDate() async {
    final DateTime? picked = await showDatePicker(
      context: context,
      initialDate: _recordDate,
      firstDate: DateTime(2020),
      lastDate: DateTime.now().add(const Duration(days: 1)),
      helpText: '选择录像日期',
    );
    if (picked == null || !mounted) return;
    setState(() {
      _recordDate = DateTime(picked.year, picked.month, picked.day);
    });
  }

  Future<void> _pickTime(bool start) async {
    final TimeOfDay? picked = await showTimePicker(
      context: context,
      initialTime: start ? _startTime : _endTime,
      helpText: start ? '选择开始时间' : '选择结束时间',
    );
    if (picked == null || !mounted) return;
    setState(() {
      if (start) {
        _startTime = picked;
      } else {
        _endTime = picked;
      }
    });
  }

  // ══════════════════════════════════════════════════════════
  // 回放会话
  // ══════════════════════════════════════════════════════════

  /// 开始播放某个录像片段。
  Future<void> _startPlayback(AppGbRecord record) async {
    final int? channelId = _channelId;
    if (channelId == null || _starting) return;

    _starting = true;
    _wantsPlay = true;
    // 先递增代际：上一次启动/控制的回调从这里开始全部作废。
    final int gen = ++_generation;

    try {
      _stopTicker();
      // 同一通道同一时间只保留一个回放会话，先停掉上一段。
      await _stopSession();
      if (!mounted || gen != _generation) return;

      final DateTime? start = parseServerTime(record.startTime);
      final DateTime? end = parseServerTime(record.endTime);
      final int diff = (start != null && end != null)
          ? end.difference(start).inSeconds
          : 0;
      // timeLen 来自设备上报，优先使用；缺失时用时间窗差值兜底。
      final int duration = record.timeLen > 0
          ? record.timeLen
          : (diff > 0 ? diff : 0);

      _safeSetState(() {
        _selected = record;
        _segmentStart = start;
        _segmentDuration = duration;
        _positionSeconds = 0;
        _paused = false;
        _dragging = false;
        _speed = 1;
        _mode = _PlaybackMode.connecting;
        _status = '正在向设备请求回放…';
        _error = null;
        _webrtcError = null;
        _flvError = null;
      });

      final RAppGbPlayback resp =
          await GbController.startPlaybackUsingPost(
        GbPlaybackReq(
          channelId: channelId,
          startTime: record.startTime,
          endTime: record.endTime,
        ),
      );

      // 响应期间页面已销毁 / 已切后台 / 已开始新会话：
      // 新会话已经建好，必须立刻用它的 stream_id 停掉，避免 ZLM 残留。
      if (!mounted || gen != _generation) {
        final String staleId = resp.value?.streamId ?? '';
        if (staleId.isNotEmpty) {
          unawaited(GbController.stopPlaybackUsingPost(staleId));
        }
        return;
      }

      if (!resp.isSuccess || resp.value == null) {
        _safeSetState(() {
          _mode = _PlaybackMode.error;
          _error = resp.errorMessage;
        });
        return;
      }

      final AppGbPlaybackInfo info = resp.value!;
      _streamId = info.streamId;

      if (await _tryWebRtc(info, gen)) {
        _startTicker();
        return;
      }
      if (!mounted || gen != _generation) {
        unawaited(_stopBackendQuietlyAsync());
        return;
      }

      _safeSetState(() {
        _mode = _PlaybackMode.connecting;
        _status = 'WebRTC 不可用，正在回退 HTTP-FLV…';
      });
      if (await _tryMediaKit(info, gen)) {
        _startTicker();
        return;
      }
      if (!mounted || gen != _generation) {
        unawaited(_stopBackendQuietlyAsync());
        return;
      }

      _safeSetState(() {
        _mode = _PlaybackMode.error;
        _error = '无法播放该录像片段。\n'
            'WebRTC：${_webrtcError ?? '未返回地址'}\n'
            'HTTP-FLV：${_flvError ?? '未返回地址'}';
      });
    } catch (e) {
      if (!mounted || gen != _generation) return;
      _safeSetState(() {
        _mode = _PlaybackMode.error;
        _error = '回放启动失败：${_humanize(e)}';
      });
    } finally {
      _starting = false;
    }
  }

  /// 主动停止回放。
  Future<void> _stopPlayback() async {
    if (_streamId == null && _mode == _PlaybackMode.idle) return;
    _wantsPlay = false;
    _generation++;
    _stopTicker();

    final String? streamId = _streamId;
    _streamId = null;
    await _stopTransports();
    if (streamId != null && streamId.isNotEmpty) {
      // 失败时 ApiService 已统一 toast；这里不重复提示。
      await GbController.stopPlaybackUsingPost(streamId);
    }
    if (!mounted) return;
    _safeSetState(() {
      _mode = _PlaybackMode.idle;
      _status = '';
      _error = null;
      _paused = false;
      _positionSeconds = 0;
      _selected = null;
    });
  }

  /// 切后台：停止回放并释放播放器。
  ///
  /// 回放是点播而非直播，切回前台后不自动续播 —— 用户可点击重试从头播放，
  /// 避免在用户无感知的情况下消耗设备与媒体节点资源。
  Future<void> _suspend() async {
    if (!_wantsPlay && _streamId == null) return;
    _wantsPlay = false;
    _generation++;
    _stopTicker();
    _safeSetState(() {
      _mode = _PlaybackMode.paused;
      _status = '回放已停止';
      _error = null;
      _paused = true;
    });
    await _stopSession();
  }

  /// 停止当前会话（释放内核 + 调 stop），不改变 UI 模式。
  Future<void> _stopSession() async {
    final String? streamId = _streamId;
    _streamId = null;
    await _stopTransports();
    if (streamId != null && streamId.isNotEmpty) {
      await GbController.stopPlaybackUsingPost(streamId);
    }
  }

  /// dispose 中不能 await：捕获 stream_id 后异步补发。
  void _stopBackendQuietly() {
    final String? streamId = _streamId;
    _streamId = null;
    if (streamId == null || streamId.isEmpty) return;
    unawaited(GbController.stopPlaybackUsingPost(streamId));
  }

  /// 异步补发 stop（用于启动流程被代际作废的分支）。
  Future<void> _stopBackendQuietlyAsync() async {
    final String? streamId = _streamId;
    _streamId = null;
    if (streamId == null || streamId.isEmpty) return;
    await GbController.stopPlaybackUsingPost(streamId);
  }

  // ══════════════════════════════════════════════════════════
  // 回放控制（pause / resume / seek / speed）
  // ══════════════════════════════════════════════════════════

  Future<void> _togglePause() async {
    final String? streamId = _streamId;
    if (streamId == null || streamId.isEmpty || _controlBusy) return;

    final bool nextPaused = !_paused;
    _safeSetState(() => _controlBusy = true);
    final REmptyResponse resp =
        await GbController.controlPlaybackUsingPost(
      GbPlaybackControlReq(
        streamId: streamId,
        action: nextPaused ? 'pause' : 'resume',
      ),
    );
    if (!mounted) return;
    _safeSetState(() => _controlBusy = false);
    if (!resp.isSuccess) {
      context.showToast(resp.errorMessage);
      return;
    }
    _safeSetState(() {
      _paused = nextPaused;
      _lastTick = DateTime.now();
    });
  }

  /// 拖动进度条后 seek。
  ///
  /// 后端不提供「当前播放进度」查询，进度条是本地按墙钟时间估算的近似值；
  /// seek 的目标时间仍以片段开始时间 + 偏移量计算，精度取决于设备侧 MANSRTSP。
  Future<void> _seekTo(double seconds) async {
    final String? streamId = _streamId;
    final DateTime? start = _segmentStart;
    if (streamId == null || streamId.isEmpty || start == null) return;

    final int offset = seconds
        .round()
        .clamp(0, _segmentDuration > 0 ? _segmentDuration : 0)
        .toInt();
    final DateTime seekAt = start.add(Duration(seconds: offset));
    _safeSetState(() {
      _positionSeconds = offset.toDouble();
      _lastTick = DateTime.now();
    });

    final REmptyResponse resp =
        await GbController.controlPlaybackUsingPost(
      GbPlaybackControlReq(
        streamId: streamId,
        action: 'seek',
        seekTime: _gbTime(seekAt),
      ),
    );
    if (!mounted) return;
    if (!resp.isSuccess) context.showToast(resp.errorMessage);
  }

  Future<void> _changeSpeed(double scale) async {
    final String? streamId = _streamId;
    if (streamId == null || streamId.isEmpty || _controlBusy) return;
    if (scale == _speed) return;

    _safeSetState(() => _controlBusy = true);
    final REmptyResponse resp =
        await GbController.controlPlaybackUsingPost(
      GbPlaybackControlReq(
        streamId: streamId,
        action: 'speed',
        scale: scale,
      ),
    );
    if (!mounted) return;
    _safeSetState(() => _controlBusy = false);
    if (!resp.isSuccess) {
      context.showToast(resp.errorMessage);
      return;
    }
    _safeSetState(() {
      _speed = scale;
      _lastTick = DateTime.now();
    });
  }

  // ── 本地进度估算 ────────────────────────────────────────────

  void _startTicker() {
    _stopTicker();
    _lastTick = DateTime.now();
    _ticker = Timer.periodic(const Duration(seconds: 1), (Timer timer) {
      if (!mounted) return;
      final DateTime now = DateTime.now();
      final double delta = now.difference(_lastTick).inMilliseconds / 1000.0;
      _lastTick = now;
      if (_paused || _dragging || _segmentDuration <= 0) return;
      final double next = _positionSeconds + delta * _speed;
      _safeSetState(() {
        _positionSeconds = next >= _segmentDuration
            ? _segmentDuration.toDouble()
            : next;
      });
    });
  }

  void _stopTicker() {
    _ticker?.cancel();
    _ticker = null;
  }

  // ══════════════════════════════════════════════════════════
  // 播放内核（复制自 gb_live_page.dart 的最小实现）
  // ══════════════════════════════════════════════════════════

  Future<void> _ensureRenderer() async {
    if (_renderer != null) return;
    final RTCVideoRenderer renderer = RTCVideoRenderer();
    await renderer.initialize();
    _renderer = renderer;
  }

  Future<bool> _tryWebRtc(AppGbPlaybackInfo info, int gen) async {
    if (info.webrtc.isEmpty) {
      _webrtcError = '后端未返回 WebRTC 地址';
      return false;
    }
    _safeSetState(() => _status = '正在建立 WebRTC 连接…');

    try {
      await _ensureRenderer();
      if (!mounted || gen != _generation) return false;

      final RTCVideoRenderer renderer = _renderer!;
      final GbWhepClient client = GbWhepClient();
      await client.connect(url: info.webrtc, renderer: renderer);
      if (!mounted || gen != _generation) {
        await client.close();
        return false;
      }
      _whep = client;
      _safeSetState(() {
        _mode = _PlaybackMode.webrtc;
        _status = '';
        _error = null;
      });
      return true;
    } catch (e) {
      _webrtcError = _humanize(e);
      debugPrint('[GB/Playback] WebRTC 失败，回退 HTTP-FLV：$e');
      final RTCVideoRenderer? renderer = _renderer;
      if (renderer != null) {
        try {
          renderer.srcObject = null;
        } catch (_) {
          // renderer 可能已被释放，忽略。
        }
      }
      return false;
    }
  }

  Future<bool> _tryMediaKit(AppGbPlaybackInfo info, int gen) async {
    final String url = info.flv.isNotEmpty ? info.flv : info.hls;
    if (url.isEmpty) {
      _flvError = '后端未返回 FLV/HLS 地址';
      return false;
    }
    _safeSetState(() => _status = '正在以 HTTP-FLV 播放…');

    Player? player;
    try {
      MediaKit.ensureInitialized();
      player = Player();
      final VideoController controller = VideoController(player);
      final StreamSubscription<String> sub = player.stream.error.listen(
        (String message) {
          if (!mounted || gen != _generation) return;
          _flvError = message;
          _safeSetState(() {
            _mode = _PlaybackMode.error;
            _error = '视频流播放失败：$message';
          });
          unawaited(_disposePlayer());
        },
      );

      _player = player;
      _videoController = controller;
      _playerErrorSub = sub;

      await player.open(Media(url));

      if (!mounted || gen != _generation) {
        await sub.cancel();
        await player.dispose();
        if (identical(_player, player)) {
          _player = null;
          _videoController = null;
          _playerErrorSub = null;
        }
        return false;
      }

      _safeSetState(() {
        _mode = _PlaybackMode.flv;
        _status = '';
        _error = null;
      });
      debugPrint('[GB/Playback] HTTP-FLV started: $url');
      return true;
    } catch (e) {
      _flvError = _humanize(e);
      debugPrint('[GB/Playback] HTTP-FLV 失败：$e');
      await _disposePlayer();
      return false;
    }
  }

  Future<void> _stopTransports() async {
    final GbWhepClient? whep = _whep;
    _whep = null;
    if (whep != null) {
      await whep.close();
    }
    final RTCVideoRenderer? renderer = _renderer;
    if (renderer != null) {
      try {
        renderer.srcObject = null;
      } catch (_) {
        // renderer 可能已被释放，忽略。
      }
    }
    await _disposePlayer();
  }

  /// 释放 media_kit，并等一帧让 `Video` 从树上摘除后再 dispose。
  Future<void> _disposePlayer() async {
    final StreamSubscription<String>? sub = _playerErrorSub;
    _playerErrorSub = null;
    await sub?.cancel();

    final Player? player = _player;
    final VideoController? controller = _videoController;
    _player = null;
    _videoController = null;
    if (player == null) return;

    if (controller != null && mounted) {
      _safeSetState(() {});
      await WidgetsBinding.instance.endOfFrame;
    }
    await player.dispose();
  }

  /// dispose 中同步释放（不能 await，也不能 setState）。
  void _releasePlayerNow() {
    unawaited(_playerErrorSub?.cancel());
    _playerErrorSub = null;
    final Player? player = _player;
    _player = null;
    _videoController = null;
    if (player != null) {
      unawaited(player.dispose().catchError((Object _) {}));
    }
  }

  // ══════════════════════════════════════════════════════════
  // UI
  // ══════════════════════════════════════════════════════════

  @override
  Widget build(BuildContext context) {
    if (_channelId == null) {
      return _buildChannelPicker();
    }

    return Scaffold(
      appBar: AppBar(title: Text(_channelTitle)),
      body: RefreshIndicator(
        onRefresh: _queryRecords,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.only(bottom: AppSpacing.xxl),
          children: <Widget>[
            AspectRatio(
              aspectRatio: 16 / 9,
              child: _buildVideoArea(),
            ),
            if (_selected != null) _buildControlBar(),
            _buildQueryCard(),
            _buildRecordSection(),
            _buildFootnote(),
          ],
        ),
      ),
    );
  }

  // ── 视频区域 ────────────────────────────────────────────────

  Widget _buildVideoArea() {
    final bool playing = _mode == _PlaybackMode.webrtc ||
        _mode == _PlaybackMode.flv;

    return Container(
      color: Colors.black,
      child: Stack(
        fit: StackFit.expand,
        children: <Widget>[
          _buildVideoContent(),
          if (_mode == _PlaybackMode.idle) _buildIdleOverlay(),
          if (_mode == _PlaybackMode.connecting) _buildStatusOverlay(),
          if (_mode == _PlaybackMode.paused) _buildPausedOverlay(),
          if (_mode == _PlaybackMode.error) _buildErrorOverlay(),
          if (playing)
            Positioned(
              top: AppSpacing.sm,
              left: AppSpacing.sm,
              child: _buildModeBadge(),
            ),
        ],
      ),
    );
  }

  Widget _buildVideoContent() {
    final RTCVideoRenderer? renderer = _renderer;
    if (_mode == _PlaybackMode.webrtc && renderer != null) {
      return RTCVideoView(
        renderer,
        objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitContain,
        mirror: false,
      );
    }
    final VideoController? controller = _videoController;
    if (_mode == _PlaybackMode.flv && controller != null) {
      return Video(
        controller: controller,
        controls: NoVideoControls,
        fit: BoxFit.contain,
      );
    }
    return const SizedBox.shrink();
  }

  Widget _buildIdleOverlay() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          const Icon(
            Icons.movie_filter_outlined,
            color: Colors.white54,
            size: 40,
          ),
          SizedBox(height: AppSpacing.md),
          Text(
            '检索后点击录像片段开始回放',
            style: AppText.caption.copyWith(color: Colors.white70),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }

  Widget _buildStatusOverlay() {
    return Container(
      color: Colors.black,
      alignment: Alignment.center,
      padding: EdgeInsets.all(AppSpacing.lg),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          const SizedBox(
            width: 28,
            height: 28,
            child: CircularProgressIndicator(
              strokeWidth: 2.4,
              valueColor: AlwaysStoppedAnimation<Color>(Colors.white),
            ),
          ),
          SizedBox(height: AppSpacing.md),
          Text(
            _status,
            style: AppText.caption.copyWith(color: Colors.white70),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }

  Widget _buildPausedOverlay() {
    return Container(
      color: Colors.black,
      child: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            const Icon(
              Icons.pause_circle_outline,
              color: Colors.white70,
              size: 40,
            ),
            SizedBox(height: AppSpacing.md),
            Text(
              '回放已停止（切到后台会自动停流）',
              style: AppText.caption.copyWith(color: Colors.white70),
              textAlign: TextAlign.center,
            ),
            SizedBox(height: AppSpacing.lg),
            FilledButton.icon(
              key: const Key('gb-playback-resume'),
              onPressed: _selected == null
                  ? null
                  : () => unawaited(_startPlayback(_selected!)),
              icon: const Icon(Icons.replay),
              label: const Text('重新回放'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildErrorOverlay() {
    return Container(
      color: Colors.black,
      child: Center(
        child: SingleChildScrollView(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              const Icon(
                Icons.error_outline,
                color: AppColors.danger,
                size: 40,
              ),
              SizedBox(height: AppSpacing.md),
              Text(
                _error ?? '回放失败',
                style: AppText.caption.copyWith(color: Colors.white70),
                textAlign: TextAlign.center,
              ),
              SizedBox(height: AppSpacing.lg),
              FilledButton.icon(
                key: const Key('gb-playback-retry'),
                onPressed: _selected == null || _starting
                    ? null
                    : () => unawaited(_startPlayback(_selected!)),
                icon: const Icon(Icons.refresh),
                label: const Text('重试'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildModeBadge() {
    final bool webrtc = _mode == _PlaybackMode.webrtc;
    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.sm,
        vertical: 2.dp,
      ),
      decoration: BoxDecoration(
        color: Colors.black54,
        borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          Icon(
            webrtc ? Icons.bolt : Icons.movie_outlined,
            size: 12.dp,
            color: webrtc ? AppColors.success : AppColors.warning,
          ),
          SizedBox(width: 3.dp),
          Text(
            webrtc ? 'WebRTC' : 'HTTP-FLV',
            style: AppText.tag.copyWith(color: Colors.white),
          ),
        ],
      ),
    );
  }

  // ── 控制条 ──────────────────────────────────────────────────

  Widget _buildControlBar() {
    final bool hasSession = _streamId != null && _streamId!.isNotEmpty;
    final double duration = _segmentDuration > 0
        ? _segmentDuration.toDouble()
        : 1;
    final double value = _positionSeconds.clamp(0, duration).toDouble();

    return Container(
      color: AppColors.card,
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: Column(
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text('回放控制', style: AppText.section),
              ),
              if (_segmentDuration > 0)
                Text(
                  '${_formatPosition(_positionSeconds)} / '
                  '${_formatPosition(_segmentDuration.toDouble())}',
                  style: AppText.hint,
                ),
            ],
          ),
          Slider(
            key: const Key('gb-playback-seek'),
            min: 0,
            max: duration,
            value: value,
            onChanged: hasSession
                ? (double v) {
                    _dragging = true;
                    _safeSetState(() => _positionSeconds = v);
                  }
                : null,
            onChangeEnd: hasSession
                ? (double v) {
                    _dragging = false;
                    unawaited(_seekTo(v));
                  }
                : null,
          ),
          Row(
            children: <Widget>[
              IconButton(
                key: const Key('gb-playback-pause'),
                tooltip: _paused ? '继续' : '暂停',
                onPressed:
                    !hasSession || _controlBusy ? null : _togglePause,
                icon: Icon(_paused ? Icons.play_arrow : Icons.pause),
              ),
              SizedBox(width: AppSpacing.xs),
              DropdownButton<double>(
                key: const Key('gb-playback-speed'),
                value: _speed,
                isDense: true,
                underline: const SizedBox.shrink(),
                onChanged: !hasSession || _controlBusy
                    ? null
                    : (double? v) {
                        if (v != null) unawaited(_changeSpeed(v));
                      },
                items: const <DropdownMenuItem<double>>[
                  DropdownMenuItem<double>(
                    value: 0.5,
                    child: Text('0.5x'),
                  ),
                  DropdownMenuItem<double>(
                    value: 1,
                    child: Text('1.0x'),
                  ),
                  DropdownMenuItem<double>(
                    value: 2,
                    child: Text('2.0x'),
                  ),
                  DropdownMenuItem<double>(
                    value: 4,
                    child: Text('4.0x'),
                  ),
                ],
              ),
              const Spacer(),
              IconButton(
                key: const Key('gb-playback-stop'),
                tooltip: '停止回放',
                onPressed: !hasSession
                    ? null
                    : () => unawaited(_stopPlayback()),
                icon: const Icon(
                  Icons.stop_circle_outlined,
                  color: AppColors.danger,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  // ── 检索卡片 ────────────────────────────────────────────────

  Widget _buildQueryCard() {
    final AppGbRecordQueryResult? records = _records;
    final bool timeout = records?.timeout ?? false;

    return Container(
      margin: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        0,
      ),
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Text('录像检索', style: AppText.section),
          SizedBox(height: AppSpacing.sm),
          _SelectorRow(
            key: const Key('gb-playback-date'),
            icon: Icons.calendar_today_outlined,
            label: '日期',
            value: _dateText,
            onTap: _pickDate,
          ),
          _SelectorRow(
            key: const Key('gb-playback-start'),
            icon: Icons.schedule,
            label: '开始',
            value: _hhmm(_startTime),
            onTap: () => _pickTime(true),
          ),
          _SelectorRow(
            key: const Key('gb-playback-end'),
            icon: Icons.schedule,
            label: '结束',
            value: _hhmm(_endTime),
            onTap: () => _pickTime(false),
          ),
          SizedBox(height: AppSpacing.md),
          SizedBox(
            width: double.infinity,
            child: FilledButton.icon(
              key: const Key('gb-playback-query'),
              onPressed: _querying ? null : _queryRecords,
              icon: _querying
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        valueColor:
                            AlwaysStoppedAnimation<Color>(Colors.white),
                      ),
                    )
                  : const Icon(Icons.search),
              label: Text(_querying ? '查询中…' : '查询录像'),
            ),
          ),
          if (_queryError != null) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Text(
              _queryError!,
              style: AppText.hint.copyWith(color: AppColors.danger),
            ),
          ],
          if (timeout) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Container(
              width: double.infinity,
              padding: EdgeInsets.all(AppSpacing.sm),
              decoration: BoxDecoration(
                color: AppColors.warningLight,
                borderRadius: BorderRadius.circular(AppSpacing.radiusTag),
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: <Widget>[
                  const Icon(
                    Icons.timer_off_outlined,
                    size: 16,
                    color: AppColors.warning,
                  ),
                  SizedBox(width: AppSpacing.xs),
                  Expanded(
                    child: Text(
                      '设备未在 5 秒内响应，请重试或缩小时间范围',
                      style: AppText.hint.copyWith(
                        color: AppColors.warning,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  // ── 录像片段列表 ────────────────────────────────────────────

  Widget _buildRecordSection() {
    final AppGbRecordQueryResult? records = _records;

    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        0,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text('录像片段', style: AppText.section),
              ),
              if (records != null)
                Text('共 ${records.items.length} 段', style: AppText.hint),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          if (_querying)
            Padding(
              padding: EdgeInsets.symmetric(vertical: AppSpacing.xl),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: <Widget>[
                  SizedBox(
                    width: 16.dp,
                    height: 16.dp,
                    child: const CircularProgressIndicator(strokeWidth: 2),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  Text('正在检索设备录像…', style: AppText.hint),
                ],
              ),
            )
          else if (records == null)
            Container(
              width: double.infinity,
              padding: EdgeInsets.symmetric(vertical: AppSpacing.xxl),
              decoration: BoxDecoration(
                color: AppColors.card,
                borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
              ),
              // 错误文案已在检索卡片内展示，这里不再重复。
              child: EmptyState(
                icon: Icons.movie_filter_outlined,
                title: _queryError == null ? '请先检索录像' : '检索失败',
              ),
            )
          else if (records.items.isEmpty)
            Container(
              width: double.infinity,
              padding: EdgeInsets.symmetric(vertical: AppSpacing.xxl),
              decoration: BoxDecoration(
                color: AppColors.card,
                borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
              ),
              child: EmptyState(
                icon: Icons.movie_filter_outlined,
                title: records.timeout ? '设备未返回录像片段' : '该时间范围暂无录像',
                description: records.timeout
                    ? '可以缩小时间范围或稍后重试'
                    : '可调整日期与时间范围后重新检索',
              ),
            )
          else
            ...records.items.asMap().entries.map(
                  (MapEntry<int, AppGbRecord> entry) =>
                      _buildRecordTile(entry.value, entry.key),
                ),
        ],
      ),
    );
  }

  Widget _buildRecordTile(AppGbRecord record, int index) {
    final bool selected = identical(_selected, record);
    final String title = record.name.isNotEmpty
        ? record.name
        : '录像片段 ${index + 1}';

    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Material(
        key: Key('gb-record-${record.startTime}-${record.endTime}'),
        color: selected ? AppColors.primaryLight : AppColors.card,
        borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
        child: InkWell(
          borderRadius: BorderRadius.circular(AppSpacing.radiusCard),
          onTap: _starting ? null : () => unawaited(_startPlayback(record)),
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.md),
            child: Row(
              children: <Widget>[
                Container(
                  width: 40.dp,
                  height: 40.dp,
                  decoration: BoxDecoration(
                    color: AppColors.fill,
                    borderRadius:
                        BorderRadius.circular(AppSpacing.radiusControl),
                  ),
                  child: const Icon(
                    Icons.video_library_outlined,
                    size: 20,
                    color: AppColors.primary,
                  ),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: <Widget>[
                      Text(
                        title,
                        style: AppText.bodyStrong,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      SizedBox(height: 2.dp),
                      Text(
                        '${record.startTime.formatMinute} ~ '
                        '${record.endTime.formatMinute}',
                        style: AppText.hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      SizedBox(height: 2.dp),
                      Text(
                        '时长 ${record.readableDuration}',
                        style: AppText.hint,
                      ),
                    ],
                  ),
                ),
                SizedBox(width: AppSpacing.sm),
                Icon(
                  selected && _mode != _PlaybackMode.idle
                      ? Icons.graphic_eq
                      : Icons.play_circle_outline,
                  size: 22.dp,
                  color: AppColors.primary,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildFootnote() {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.lg,
        AppSpacing.pagePadding,
        0,
      ),
      child: Text(
        '离开本页或切到后台会自动停止回放并释放会话。'
        '暂停 / 拖动 / 倍速经带内 SIP INFO（MANSRTSP）下发，'
        '部分设备可能不支持拖动或倍速。',
        style: AppText.hint,
      ),
    );
  }

  // ── 工具 ────────────────────────────────────────────────────

  DateTime _combine(DateTime date, TimeOfDay time, {int second = 0}) {
    return DateTime(
      date.year,
      date.month,
      date.day,
      time.hour,
      time.minute,
      second,
    );
  }

  String get _dateText {
    final DateTime d = _recordDate;
    return '${d.year}-${_two(d.month)}-${_two(d.day)}';
  }

  String _hhmm(TimeOfDay time) =>
      '${_two(time.hour)}:${_two(time.minute)}';

  /// GB28181 固定时间格式 `2006-01-02T15:04:05`（设备按 XML 原样解析）。
  String _gbTime(DateTime time) {
    return '${time.year}-${_two(time.month)}-${_two(time.day)}'
        'T${_two(time.hour)}:${_two(time.minute)}:${_two(time.second)}';
  }

  static String _two(int value) => value.toString().padLeft(2, '0');

  /// 进度文本：`mm:ss`，超过 1 小时用 `hh:mm:ss`。
  String _formatPosition(double seconds) {
    final int total = seconds.round().clamp(0, 359999).toInt();
    final int hours = total ~/ 3600;
    final int minutes = (total % 3600) ~/ 60;
    final int secs = total % 60;
    if (hours > 0) {
      return '${_two(hours)}:${_two(minutes)}:${_two(secs)}';
    }
    return '${_two(minutes)}:${_two(secs)}';
  }

  /// 仅在组件仍挂载时 setState。
  void _safeSetState(VoidCallback fn) {
    if (!mounted) return;
    setState(fn);
  }

  /// 异常 → 可读中文（WHEP 异常自带文案）。
  String _humanize(Object error) {
    if (error is GbWhepException) return error.message;
    return error.toString();
  }
}

/// 检索条件行：图标 + 标签 + 值 + 箭头。
class _SelectorRow extends StatelessWidget {
  const _SelectorRow({
    super.key,
    required this.icon,
    required this.label,
    required this.value,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final String value;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.fill,
      borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
      child: InkWell(
        borderRadius: BorderRadius.circular(AppSpacing.radiusControl),
        onTap: onTap,
        child: Padding(
          padding: EdgeInsets.symmetric(
            horizontal: AppSpacing.md,
            vertical: AppSpacing.sm,
          ),
          child: Row(
            children: <Widget>[
              Icon(icon, size: 16.dp, color: AppColors.textTertiary),
              SizedBox(width: AppSpacing.sm),
              SizedBox(
                width: 48.dp,
                child: Text(label, style: AppText.caption),
              ),
              Expanded(
                child: Text(
                  value,
                  style: AppText.body,
                  textAlign: TextAlign.right,
                ),
              ),
              SizedBox(width: AppSpacing.xs),
              Icon(
                Icons.chevron_right,
                size: 18.dp,
                color: AppColors.textTertiary,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
