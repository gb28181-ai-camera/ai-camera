import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_webrtc/flutter_webrtc.dart';
import 'package:go_router/go_router.dart';
import 'package:media_kit/media_kit.dart';
import 'package:media_kit_video/media_kit_video.dart';

import '../../layout/Gaps.dart';
import '../../services/IotAppController.dart';
import '../../services/MqttSignalingClient.dart';
import '../../services/generated/GbController.dart';
import '../../services/generated/definitions/AppGb.dart';
import '../../services/generated/definitions/Response.dart';
import '../../theme/app_colors.dart';
import '../../theme/app_spacing.dart';
import '../../theme/app_text.dart';
import '../../utils/Extensions.dart';
import '../../widgets/entity_list_page.dart';
import 'gb_ptz_panel.dart';
import 'gb_whep_client.dart';

/// 播放器当前使用的内核。
enum _GbMode {
  /// 正在点播 / 正在 WebRTC 握手。
  connecting,
  /// WebRTC（flutter_webrtc + WHEP）。
  webrtc,
  /// WebRTC（flutter_webrtc + MQTT 信令）。
  mqtt,
  /// HTTP-FLV（media_kit / libmpv）。
  flv,
  /// 两种内核都失败。
  error,
  /// App 切到后台，播放已停止，等待恢复。
  paused,
}

/// 国标通道实时播放页。
///
/// 路由：`/gb/live?channelId=<通道主键>&name=<通道名称>`。
///
/// 播放器策略（依据 `player_spike_page.dart` 的实测结论）：
/// 1. 优先 `flutter_webrtc` —— 自己实现 WHEP 客户端（POST offer SDP → answer）；
/// 2. WebRTC 失败（含 H.265 不被 App 端解码器支持导致的「有 answer 无视频」）时，
///    自动回退 `media_kit` 播放 HTTP-FLV；
/// 3. 两者都失败时展示明确错误 + 重试按钮，不留下黑屏。
///
/// ★ 生命周期：页面离开 / App 切后台**必须**调用 `stop` 接口并释放播放器。
/// 否则 ZLM 侧会残留推流会话，设备持续推流（消耗流量），且 MediaCodec 硬解实例
/// 不会归还 —— 反复进出播放页后会表现为「后开的页面黑屏」。
class GbLivePage extends StatefulWidget {
  /// 构造播放页。
  const GbLivePage({super.key});

  @override
  State<GbLivePage> createState() => _GbLivePageState();
}

class _GbLivePageState extends State<GbLivePage>
    with WidgetsBindingObserver {
  // ── 路由参数 ──────────────────────────────────────────────
  int? _channelId;
  String _channelTitle = '国标通道';
  bool _initialized = false;

  /// 所属 IOT 设备 ID（`?deviceId=`）。只有它才能查到服务组 / MQTT 配置。
  int? _deviceId;

  /// 是否优先使用 MQTT 信令（`?mode=mqtt`）。
  bool _preferMqtt = false;

  // ── 会话状态 ──────────────────────────────────────────────
  _GbMode _mode = _GbMode.connecting;
  String _status = '正在点播…';
  String? _error;
  String? _webrtcError;
  String? _flvError;
  String? _mqttError;
  AppGbPlayInfo? _playInfo;
  String? _streamId;

  /// 是否希望播放（切后台后置 false 之前始终为 true，用于自动恢复）。
  bool _wantsPlay = false;

  /// 防止并发 `_start`；并发时记下「还需要再起一次」。
  bool _starting = false;
  bool _restartRequested = false;

  /// 代际计数：每次停止/重试递增，用于丢弃过期的异步回调。
  int _generation = 0;

  // ── flutter_webrtc ────────────────────────────────────────
  final RTCVideoRenderer _renderer = RTCVideoRenderer();
  bool _rendererReady = false;
  GbWhepClient? _whep;
  MqttWebRtcClient? _mqtt;
  /// ICE 配置按播放会话缓存：MQTT 失败回退 WHEP 时不再重复请求。
  List<IceServerConfig>? _iceCache;

  // ── media_kit ─────────────────────────────────────────────
  Player? _player;
  VideoController? _videoController;
  StreamSubscription<String>? _playerErrorSub;

  // ── 云台 ──────────────────────────────────────────────────
  int _ptzSpeed = 5;
  String? _heldDirection;
  String? _heldZoom;

  /// 待下发的云台指令（单飞 + 合并）。
  ///
  /// 不把每个手势事件都排进队列：拖拽时可能几百毫秒内切换多次方向，
  /// 而一次 SIP MESSAGE 往返可能很慢。这里只保留「最新一条待发指令」，
  /// 当前请求结束后立刻发它 —— 既能保证松开后的 stop 不被长队列拖延，
  /// 又保证同一时刻只有一条云台指令在飞行。
  String? _pendingPtzDirection;
  String? _pendingPtzAction;
  bool _ptzSending = false;

  // ── 其它 UI 状态 ──────────────────────────────────────────
  bool _fullscreen = false;
  double _videoScale = 1.0;
  bool _snapshotBusy = false;
  bool _talkBusy = false;
  String? _talkSessionId;

  /// 画面拖拽云台的手势状态。
  Offset _scaleStartPoint = Offset.zero;
  double _scaleStartValue = 1.0;
  String? _dragDirection;
  bool _gestureWasZoom = false;

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

    final GoRouterState state = GoRouterState.of(context);
    _channelId = int.tryParse(state.uri.queryParameters['channelId'] ?? '');
    _deviceId = int.tryParse(state.uri.queryParameters['deviceId'] ?? '');
    // 只有拿到设备 ID 才能查询服务组（MQTT broker / 凭据），否则保持 WHEP。
    _preferMqtt = state.uri.queryParameters['mode'] == 'mqtt' && _deviceId != null;
    final String? name = state.uri.queryParameters['name'];
    if (name != null && name.isNotEmpty) {
      _channelTitle = name;
    }

    if (_channelId == null) {
      _mode = _GbMode.error;
      _error = '缺少有效的通道 ID';
      return;
    }
    _wantsPlay = true;
    unawaited(_start());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _generation++;

    // 兜底：如果页面销毁时云台/对讲仍在进行，补发停止。
    _ptzStopQuietly();
    _stopTalkQuietly();

    final GbWhepClient? whep = _whep;
    _whep = null;
    if (whep != null) {
      unawaited(whep.close().catchError((Object _) {}));
    }
    final MqttWebRtcClient? mqtt = _mqtt;
    _mqtt = null;
    if (mqtt != null) {
      unawaited(mqtt.close().catchError((Object _) {}));
    }
    if (_rendererReady) {
      _renderer.srcObject = null;
    }
    _releasePlayerNow();
    // 未初始化时 dispose 可能报错，忽略即可。
    unawaited(_renderer.dispose().catchError((Object _) {}));

    // 离开页面必须停止点播，否则 ZLM 侧流与 SSRC 不会归还。
    unawaited(_stopBackend());

    if (_fullscreen) {
      _restoreSystemUi();
    }
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    // 只处理真正的「切后台」。inactive 会在下拉通知栏、iOS 控制中心等
    // 短暂触发，若也停流会导致误停；paused/hidden/detached 才是后台。
    if (state == AppLifecycleState.paused ||
        state == AppLifecycleState.hidden ||
        state == AppLifecycleState.detached) {
      unawaited(_suspend());
    } else if (state == AppLifecycleState.resumed) {
      _resume();
    }
  }

  // ══════════════════════════════════════════════════════════
  // 启停
  // ══════════════════════════════════════════════════════════

  /// 点播 + 选择播放器。
  Future<void> _start() async {
    if (_channelId == null) return;
    if (_starting) {
      _restartRequested = true;
      return;
    }
    _starting = true;
    _wantsPlay = true;
    final int gen = ++_generation;

    _safeSetState(() {
      _mode = _GbMode.connecting;
      _status = '正在点播…';
      _error = null;
      _webrtcError = null;
      _flvError = null;
      _mqttError = null;
    });

    try {
      await _stopTransports();
      // 重试 / 重连时先停掉上一次的服务端会话，避免旧的 SIP 会话悬挂。
      await _stopBackend();
      await _ensureRenderer();
      if (!mounted || gen != _generation) return;

      final int channelId = _channelId!;
      final RAppGbPlay resp = await GbController.playUsingPost(channelId);

      // 请求期间页面已关闭 / 已重试：把刚建好的流停掉，避免 ZLM 残留。
      if (!mounted || gen != _generation) {
        final String streamId = resp.value?.streamId ?? '';
        unawaited(
          GbController.stopPlayUsingPost(channelId, streamId: streamId),
        );
        return;
      }

      if (!resp.isSuccess || resp.value == null) {
        _safeSetState(() {
          _mode = _GbMode.error;
          _error = resp.errorMessage;
        });
        return;
      }

      final AppGbPlayInfo info = resp.value!;
      _playInfo = info;
      _streamId = info.streamId;

      // 1) MQTT 信令（仅当从「我的设备」带 deviceId 进入且用户选择 MQTT 时）。
      if (_preferMqtt && _deviceId != null) {
        if (await _tryMqtt(info, gen)) return;
        if (!mounted || gen != _generation) {
          unawaited(_stopBackend());
          return;
        }
        // MQTT 失败不影响媒体面：设备/媒体节点通常仍提供 WHEP，继续尝试。
        _safeSetState(() {
          _mode = _GbMode.connecting;
          _status = 'MQTT 信令不可用，正在回退 WebRTC(WHEP)…';
        });
      }

      // 2) WHEP（HTTP 交换 SDP）。
      if (await _tryWebRtc(info, gen)) return;
      if (!mounted || gen != _generation) {
        unawaited(_stopBackend());
        return;
      }

      // 3) HTTP-FLV（原生解码，兼容 H.265 机型差异）。
      _safeSetState(() {
        _mode = _GbMode.connecting;
        _status = 'WebRTC 不可用，正在回退 HTTP-FLV…';
      });
      if (await _tryMediaKit(info, gen)) return;
      if (!mounted || gen != _generation) {
        unawaited(_stopBackend());
        return;
      }

      _safeSetState(() {
        _mode = _GbMode.error;
        _error = '无法播放该通道画面。\n'
            '${_preferMqtt ? 'MQTT：${_mqttError ?? '未尝试'}\n' : ''}'
            'WebRTC：${_webrtcError ?? '未返回地址'}\n'
            'HTTP-FLV：${_flvError ?? '未返回地址'}';
      });
    } catch (e) {
      if (!mounted || gen != _generation) return;
      _safeSetState(() {
        _mode = _GbMode.error;
        _error = '播放失败：${_humanize(e)}';
      });
    } finally {
      _starting = false;
      if (_restartRequested) {
        _restartRequested = false;
        // 期间的停止/恢复请求（如切后台又立刻回来）在这里补一次。
        if (mounted && _wantsPlay) {
          unawaited(_start());
        }
      }
    }
  }

  /// 切后台：停止点播、释放播放器与云台。
  ///
  /// 先把 [_wantsPlay] 置 false，这样即便此时还有一次 `_start` 在飞行中，
  /// 它的 finally 也不会在后台把播放重新拉起来。
  Future<void> _suspend() async {
    if (!_wantsPlay) return;
    _wantsPlay = false;
    _generation++;
    _ptzStopQuietly();
    _stopTalkQuietly();
    // 先切到「已暂停」再释放：stop 接口可能较慢，不能让用户一直盯着最后一帧。
    _safeSetState(() {
      _mode = _GbMode.paused;
      _status = '已暂停';
      _error = null;
    });
    await _stopTransports();
    await _stopBackend();
  }

  /// 回到前台：重新点播。
  void _resume() {
    if (_wantsPlay) return;
    _wantsPlay = true;
    unawaited(_start());
  }

  /// 调 stop 接口并清空会话标识。
  ///
  /// 失败不额外提示：`ApiService` 已统一 toast；离开页面时再弹错误反而打扰用户。
  Future<void> _stopBackend() async {
    final int? id = _channelId;
    final String streamId = _streamId ?? '';
    _streamId = null;
    if (id == null) return;
    if (streamId.isEmpty && _playInfo == null) return;
    await GbController.stopPlayUsingPost(id, streamId: streamId);
    _playInfo = null;
  }

  /// 释放两套播放内核（不调后端 stop）。
  Future<void> _stopTransports() async {
    final GbWhepClient? whep = _whep;
    _whep = null;
    if (whep != null) {
      await whep.close();
    }
    final MqttWebRtcClient? mqtt = _mqtt;
    _mqtt = null;
    if (mqtt != null) {
      await mqtt.close();
    }
    if (_rendererReady) {
      _renderer.srcObject = null;
    }
    await _disposePlayer();
  }

  Future<void> _ensureRenderer() async {
    if (_rendererReady) return;
    await _renderer.initialize();
    _rendererReady = true;
  }

  // ── MQTT 信令 ─────────────────────────────────────────────

  /// 获取 ICE 配置（MQTT 与 WHEP 共用）。
  ///
  /// 未配置 TURN 时后端返回空数组，属于正常降级；请求失败同样按空处理，
  /// 不能让 ICE 不可用阻断播放。结果按会话缓存，避免回退时重复请求。
  Future<List<IceServerConfig>> _loadIceServers() async {
    final List<IceServerConfig>? cached = _iceCache;
    if (cached != null) return cached;
    final RIceServers resp = await IotAppController.iceServers();
    final List<IceServerConfig> servers =
        resp.isSuccess ? resp.servers : const <IceServerConfig>[];
    if (!resp.isSuccess) {
      debugPrint('[IOT] ICE 配置获取失败，按直连处理：${resp.errorMessage}');
    }
    _iceCache = servers;
    return servers;
  }

  /// 通过 MQTT 交换 SDP 建立 WebRTC。
  ///
  /// 需要三样东西：设备 ID（查服务组）、服务组里的 MQTT 服务（broker + 产品级凭据）、
  /// 以及可选的 ICE 配置。任何一步失败都记录原因并返回 false，由调用方回退 WHEP。
  Future<bool> _tryMqtt(AppGbPlayInfo info, int gen) async {
    final int? deviceId = _deviceId;
    final int? channelId = _channelId;
    if (deviceId == null || channelId == null) {
      _mqttError = '缺少设备 ID，无法使用 MQTT 信令';
      return false;
    }

    _safeSetState(() => _status = '正在获取 MQTT 服务配置…');
    final RIotServices services = await IotAppController.services(deviceId);
    if (!mounted || gen != _generation) return false;
    if (!services.isSuccess || services.credentials == null) {
      _mqttError = services.errorMessage;
      return false;
    }

    final MqttSignalingConfig config;
    try {
      config = MqttSignalingConfig.fromCredentials(
        services.credentials!,
        clientId: MqttSignalingClient.newClientId(),
      );
    } catch (e) {
      _mqttError = _humanize(e);
      return false;
    }

    // ICE 配置是可选的：为空表示直连（局域网 / 内网部署），不应阻断播放。
    final List<IceServerConfig> iceServers = await _loadIceServers();
    if (!mounted || gen != _generation) return false;

    _safeSetState(() => _status = '正在建立 MQTT 信令连接…');
    final MqttWebRtcClient client = MqttWebRtcClient();
    try {
      await client.connect(
        config: config,
        renderer: _renderer,
        channelId: channelId,
        stream: info.stream,
        iceServers: iceServers,
      );
      if (!mounted || gen != _generation) {
        await client.close();
        return false;
      }
      _mqtt = client;
      _safeSetState(() {
        _mode = _GbMode.mqtt;
        _status = '';
        _error = null;
      });
      return true;
    } catch (e) {
      _mqttError = _humanize(e);
      debugPrint('[IOT] MQTT 信令失败，回退 WHEP：$e');
      await client.close();
      if (_rendererReady) {
        _renderer.srcObject = null;
      }
      return false;
    }
  }

  // ── WebRTC ────────────────────────────────────────────────

  Future<bool> _tryWebRtc(AppGbPlayInfo info, int gen) async {
    if (!info.hasWebrtc) {
      _webrtcError = '后端未返回 WebRTC 地址';
      return false;
    }
    _safeSetState(() => _status = '正在建立 WebRTC 连接…');

    // WHEP 与 MQTT 共用同一份 ICE 配置：受限网络下通过 TURN 中继。
    final List<IceServerConfig> iceServers = await _loadIceServers();
    if (!mounted || gen != _generation) return false;

    final GbWhepClient client = GbWhepClient();
    try {
      await client.connect(
        url: info.webrtc,
        renderer: _renderer,
        iceServers: iceServers
            .map((IceServerConfig item) => item.toRtcIceServer())
            .toList(growable: false),
      );
      if (!mounted || gen != _generation) {
        await client.close();
        return false;
      }
      _whep = client;
      _safeSetState(() {
        _mode = _GbMode.webrtc;
        _status = '';
        _error = null;
      });
      return true;
    } catch (e) {
      _webrtcError = _humanize(e);
      debugPrint('[GB] WebRTC 失败，回退 HTTP-FLV：$e');
      await client.close();
      if (_rendererReady) {
        _renderer.srcObject = null;
      }
      return false;
    }
  }

  // ── media_kit（HTTP-FLV） ─────────────────────────────────

  Future<bool> _tryMediaKit(AppGbPlayInfo info, int gen) async {
    final String url = info.fallbackUrl;
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
            _mode = _GbMode.error;
            _error = '视频流播放失败：$message';
          });
          unawaited(_disposePlayer());
        },
      );

      // 先挂到字段上，保证错误监听里能释放到同一个 Player。
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
        _mode = _GbMode.flv;
        _status = '';
        _error = null;
      });
      debugPrint('[GB] HTTP-FLV started: $url');
      return true;
    } catch (e) {
      _flvError = _humanize(e);
      debugPrint('[GB] HTTP-FLV 失败：$e');
      await _disposePlayer();
      return false;
    }
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
      // player_spike_page.dart 的实测结论：先让 Video 组件从树上摘除，
      // 等本帧结束再释放，避免组件在已释放的 controller 上取值。
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
  // 云台
  // ══════════════════════════════════════════════════════════

  /// 是否允许下发云台指令（只有真正出画面时才允许）。
  bool get _canControlPtz =>
      _mode == _GbMode.webrtc || _mode == _GbMode.mqtt || _mode == _GbMode.flv;

  void _onPtzDirection(String direction, int speed) {
    _ptzSpeed = speed;
    _heldDirection =
        direction == GbPtzDirection.stop ? null : direction;
    _sendPtzDirection(direction);
  }

  void _onPtzAction(String action, int speed) {
    _ptzSpeed = speed;
    _heldZoom = action == GbPtzAction.zoomStop ? null : action;
    _sendPtzAction(action);
  }

  void _sendPtzDirection(String direction) {
    _pendingPtzDirection = direction;
    _pendingPtzAction = null;
    unawaited(_drainPtz());
  }

  void _sendPtzAction(String action) {
    _pendingPtzAction = action;
    _pendingPtzDirection = null;
    unawaited(_drainPtz());
  }

  /// 单飞发送：同一时刻只发一条，发送期间只保留最新的一条待发指令。
  Future<void> _drainPtz() async {
    if (_ptzSending) return;
    _ptzSending = true;
    try {
      while (_channelId != null &&
          (_pendingPtzDirection != null || _pendingPtzAction != null)) {
        final String? direction = _pendingPtzDirection;
        final String? action = _pendingPtzAction;
        _pendingPtzDirection = null;
        _pendingPtzAction = null;

        final int? id = _channelId;
        if (id == null) break;
        final int speed = _ptzSpeed;
        try {
          final REmptyResponse resp = await GbController.ptzUsingPost(
            id,
            direction != null
                ? GbPtzReq(direction: direction, speed: speed)
                : GbPtzReq(action: action!, speed: speed),
          );
          if (!resp.isSuccess) {
            debugPrint('[GB] 云台指令下发失败：${resp.errorMessage}');
          }
        } catch (error) {
          debugPrint('[GB] 云台指令请求异常：$error');
        }
      }
    } finally {
      _ptzSending = false;
      // 理论上是单线程无竞态；保留兜底以防 finally 前又有新指令写入。
      if (_pendingPtzDirection != null || _pendingPtzAction != null) {
        unawaited(_drainPtz());
      }
    }
  }

  /// 页面销毁 / 切后台时补发停止，避免镜头一直转。
  void _ptzStopQuietly() {
    if (_heldDirection != null) {
      _heldDirection = null;
      _sendPtzDirection(GbPtzDirection.stop);
    }
    if (_heldZoom != null) {
      _heldZoom = null;
      _sendPtzAction(GbPtzAction.zoomStop);
    }
  }

  // ── 画面手势：单指拖动云台、双指缩放视频 ──────────────────

  void _onScaleStart(ScaleStartDetails details) {
    _scaleStartPoint = details.localFocalPoint;
    _scaleStartValue = _videoScale;
    _dragDirection = null;
    _gestureWasZoom = false;
  }

  void _onScaleUpdate(ScaleUpdateDetails details) {
    if (details.pointerCount >= 2) {
      // 双指：走视频缩放，并确保单指期间下发的云台运动已停止。
      if (_dragDirection != null) {
        _dragDirection = null;
        _sendPtzDirection(GbPtzDirection.stop);
      }
      _gestureWasZoom = true;
      final double next =
          (_scaleStartValue * details.scale).clamp(1.0, 3.0);
      if ((next - _videoScale).abs() >= 0.01) {
        _safeSetState(() => _videoScale = next);
      }
      return;
    }

    if (_gestureWasZoom || !_canControlPtz) return;

    final Offset delta = details.localFocalPoint - _scaleStartPoint;
    if (delta.distance < 12.dp) return;
    final String direction = _directionOf(delta);
    if (direction == _dragDirection) return;
    _dragDirection = direction;
    _heldDirection = direction;
    _sendPtzDirection(direction);
  }

  void _onScaleEnd(ScaleEndDetails details) {
    if (_dragDirection != null) {
      _dragDirection = null;
      _heldDirection = null;
      _sendPtzDirection(GbPtzDirection.stop);
    }
  }

  /// 把拖拽位移映射为八方向（45° 一档，轴向 ±22.5° 内算正方向）。
  String _directionOf(Offset offset) {
    final double ax = offset.dx.abs();
    final double ay = offset.dy.abs();
    // tan(67.5°) ≈ 2.414：超过该比例视为纯水平/垂直。
    const double kSplit = 2.41421356237;
    if (ax > ay * kSplit) {
      return offset.dx > 0 ? GbPtzDirection.right : GbPtzDirection.left;
    }
    if (ay > ax * kSplit) {
      return offset.dy > 0 ? GbPtzDirection.down : GbPtzDirection.up;
    }
    if (offset.dx > 0) {
      return offset.dy > 0
          ? GbPtzDirection.downRight
          : GbPtzDirection.upRight;
    }
    return offset.dy > 0
        ? GbPtzDirection.downLeft
        : GbPtzDirection.upLeft;
  }

  // ══════════════════════════════════════════════════════════
  // 抓图 / 对讲 / 全屏
  // ══════════════════════════════════════════════════════════

  Future<void> _takeSnapshot() async {
    final int? id = _channelId;
    if (id == null || _snapshotBusy) return;
    _safeSetState(() => _snapshotBusy = true);
    try {
      final RAppGbSnapshot resp = await GbController.snapshotUsingPost(id);
      if (!mounted) return;
      if (!resp.isSuccess || resp.value == null) {
        context.showToast(resp.errorMessage);
        return;
      }
      final AppGbSnapshot snapshot = resp.value!;
      await showDialog<void>(
        context: context,
        builder: (BuildContext ctx) => AlertDialog(
          key: const Key('gb-snapshot-dialog'),
          title: const Text('通道抓图'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              SizedBox(
                width: 280.dp,
                child: AspectRatio(
                  aspectRatio: 16 / 9,
                  child: RemoteImage(url: snapshot.url, fit: BoxFit.contain),
                ),
              ),
              SizedBox(height: AppSpacing.sm),
              Text(
                '${snapshot.captureDt.orPlaceholder('--')} · '
                '${snapshot.readableSize}',
                style: AppText.hint,
              ),
            ],
          ),
          actions: <Widget>[
            TextButton(
              onPressed: () => Navigator.of(ctx).pop(),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } finally {
      _safeSetState(() => _snapshotBusy = false);
    }
  }

  /// 对讲会话开关。
  ///
  /// ⚠️ 本期只实现「建立 / 结束会话」：App 侧采集麦克风并把音频按 RTP 回推到
  /// [AppGbVoiceSession.ssrc] 尚未接入（需要原生音频推流能力），**需真机验证**。
  Future<void> _toggleTalk() async {
    final int? id = _channelId;
    if (id == null || _talkBusy) return;
    _safeSetState(() => _talkBusy = true);
    try {
      final String? sessionId = _talkSessionId;
      if (sessionId != null) {
        await GbController.stopTalkUsingPost(id, sessionId: sessionId);
        if (!mounted) return;
        _safeSetState(() => _talkSessionId = null);
        context.showToast('对讲已结束');
        return;
      }
      final RAppGbVoiceSession resp =
          await GbController.startTalkUsingPost(id);
      if (!mounted) return;
      if (!resp.isSuccess || resp.value == null) {
        context.showToast(resp.errorMessage);
        return;
      }
      _safeSetState(() => _talkSessionId = resp.value!.sessionId);
      context.showToast('对讲会话已建立（音频回传需真机验证）');
    } finally {
      _safeSetState(() => _talkBusy = false);
    }
  }

  /// 打开语音聊天（独立页面，不影响当前对讲会话）。
  void _openVoiceChat() {
    context.push('/voice');
  }

  /// 结束对讲（离开页面 / 切后台时调用，不提示）。
  void _stopTalkQuietly() {
    final int? id = _channelId;
    final String? sessionId = _talkSessionId;
    _talkSessionId = null;
    if (id == null || sessionId == null) return;
    unawaited(GbController.stopTalkUsingPost(id, sessionId: sessionId));
  }

  Future<void> _toggleFullscreen() async {
    final bool next = !_fullscreen;
    _safeSetState(() => _fullscreen = next);
    if (next) {
      await SystemChrome.setPreferredOrientations(<DeviceOrientation>[
        DeviceOrientation.landscapeLeft,
        DeviceOrientation.landscapeRight,
      ]);
      await SystemChrome.setEnabledSystemUIMode(
        SystemUiMode.immersiveSticky,
      );
    } else {
      _restoreSystemUi();
    }
  }

  /// 恢复竖屏与系统栏（离开全屏 / dispose 使用）。
  void _restoreSystemUi() {
    unawaited(
      SystemChrome.setPreferredOrientations(<DeviceOrientation>[
        DeviceOrientation.portraitUp,
      ]),
    );
    unawaited(
      SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge),
    );
  }

  // ══════════════════════════════════════════════════════════
  // UI
  // ══════════════════════════════════════════════════════════

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor:
          _fullscreen ? Colors.black : AppColors.background,
      appBar: _fullscreen
          ? null
          : AppBar(title: Text(_channelTitle)),
      body: _fullscreen ? _buildFullscreenBody() : _buildNormalBody(),
    );
  }

  Widget _buildNormalBody() {
    return ListView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: EdgeInsets.only(bottom: AppSpacing.xxl),
      children: <Widget>[
        AspectRatio(
          aspectRatio: 16 / 9,
          child: _buildVideoArea(),
        ),
        _buildToolbar(),
        if (_canControlPtz) _buildPtzCard(),
        _buildInfoCard(),
      ],
    );
  }

  Widget _buildFullscreenBody() {
    return SafeArea(
      child: Stack(
        fit: StackFit.expand,
        children: <Widget>[
          _buildVideoArea(),
          Positioned(
            top: AppSpacing.sm,
            right: AppSpacing.sm,
            child: Row(
              children: <Widget>[
                _RoundIconButton(
                  key: const Key('gb-snapshot-fullscreen'),
                  icon: Icons.camera_alt_outlined,
                  tooltip: '抓图',
                  onTap: _snapshotBusy ? null : _takeSnapshot,
                ),
                SizedBox(width: AppSpacing.sm),
                _RoundIconButton(
                  key: const Key('gb-fullscreen-exit'),
                  icon: Icons.fullscreen_exit,
                  tooltip: '退出全屏',
                  onTap: _toggleFullscreen,
                ),
              ],
            ),
          ),
          Positioned(
            left: 0,
            right: 0,
            bottom: 0,
            child: Container(
              color: Colors.black54,
              padding: EdgeInsets.symmetric(
                horizontal: AppSpacing.md,
                vertical: AppSpacing.sm,
              ),
              child: Row(
                children: <Widget>[
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: <Widget>[
                        Text(
                          _channelTitle,
                          style: AppText.bodyStrong.copyWith(
                            color: Colors.white,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        SizedBox(height: 2.dp),
                        Text(
                          _modeLabel,
                          style: AppText.hint.copyWith(
                            color: Colors.white70,
                          ),
                        ),
                      ],
                    ),
                  ),
                  TextButton.icon(
                    key: const Key('gb-ptz-sheet'),
                    onPressed: _canControlPtz ? _openPtzSheet : null,
                    icon: const Icon(
                      Icons.control_camera_outlined,
                      color: Colors.white,
                    ),
                    label: const Text(
                      '云台',
                      style: TextStyle(color: Colors.white),
                    ),
                  ),
                  IconButton(
                    key: const Key('gb-talk-fullscreen'),
                    onPressed: _talkBusy ? null : _toggleTalk,
                    tooltip: _talkSessionId == null ? '开始对讲' : '结束对讲',
                    icon: Icon(
                      _talkSessionId == null
                          ? Icons.mic_none_outlined
                          : Icons.mic,
                      color: _talkSessionId == null
                          ? Colors.white
                          : AppColors.success,
                    ),
                  ),
                  IconButton(
                    key: const Key('gb-voice-fullscreen'),
                    onPressed: _openVoiceChat,
                    tooltip: '语音聊天',
                    icon: const Icon(
                      Icons.record_voice_over_outlined,
                      color: Colors.white,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  /// 视频区域（含手势层）。
  ///
  /// 单指拖动 → 云台方向；双指缩放 → 画面缩放（与 PTZ 面板互不干扰）。
  Widget _buildVideoArea() {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onScaleStart: _onScaleStart,
      onScaleUpdate: _onScaleUpdate,
      onScaleEnd: _onScaleEnd,
      child: Container(
        color: Colors.black,
        child: Stack(
          fit: StackFit.expand,
          children: <Widget>[
            _buildVideoContent(),
            if (_mode == _GbMode.connecting || _mode == _GbMode.paused)
              _buildStatusOverlay(),
            if (_mode == _GbMode.error) _buildErrorOverlay(),
            if (_canControlPtz)
              Positioned(
                top: AppSpacing.sm,
                left: AppSpacing.sm,
                child: _buildModeBadge(),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildVideoContent() {
    if ((_mode == _GbMode.webrtc || _mode == _GbMode.mqtt) &&
        _rendererReady) {
      return Transform.scale(
        scale: _videoScale,
        child: RTCVideoView(
          _renderer,
          objectFit: RTCVideoViewObjectFit.RTCVideoViewObjectFitContain,
          mirror: false,
        ),
      );
    }
    if (_mode == _GbMode.flv && _videoController != null) {
      return Transform.scale(
        scale: _videoScale,
        child: Video(
          controller: _videoController!,
          controls: NoVideoControls,
          fit: BoxFit.contain,
        ),
      );
    }
    return const SizedBox.shrink();
  }

  Widget _buildStatusOverlay() {
    final bool paused = _mode == _GbMode.paused;
    return Container(
      color: Colors.black,
      alignment: Alignment.center,
      padding: EdgeInsets.all(AppSpacing.lg),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: <Widget>[
          if (paused)
            const Icon(
              Icons.pause_circle_outline,
              color: Colors.white70,
              size: 40,
            )
          else
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
            paused ? '已暂停，返回 App 后自动恢复' : _status,
            style: AppText.caption.copyWith(color: Colors.white70),
            textAlign: TextAlign.center,
          ),
        ],
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
                _error ?? '播放失败',
                style: AppText.caption.copyWith(color: Colors.white70),
                textAlign: TextAlign.center,
              ),
              SizedBox(height: AppSpacing.lg),
              FilledButton.icon(
                key: const Key('gb-retry'),
                // 缺少有效的通道 ID 时重试没有意义。
                onPressed: _channelId == null ? null : _start,
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
    final String label;
    final IconData icon;
    final Color color;
    switch (_mode) {
      case _GbMode.mqtt:
        label = 'MQTT/WebRTC';
        icon = Icons.hub_outlined;
        color = AppColors.success;
        break;
      case _GbMode.webrtc:
        label = 'WebRTC';
        icon = Icons.bolt;
        color = AppColors.success;
        break;
      case _GbMode.flv:
        label = 'HTTP-FLV';
        icon = Icons.movie_outlined;
        color = AppColors.warning;
        break;
      case _GbMode.connecting:
      case _GbMode.paused:
      case _GbMode.error:
        return const SizedBox.shrink();
    }
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
          Icon(icon, size: 12.dp, color: color),
          SizedBox(width: 3.dp),
          Text(
            label,
            style: AppText.tag.copyWith(color: Colors.white),
          ),
        ],
      ),
    );
  }

  Widget _buildToolbar() {
    return Container(
      color: AppColors.card,
      padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
      child: Row(
        children: <Widget>[
          _ToolButton(
            key: const Key('gb-snapshot'),
            icon: Icons.camera_alt_outlined,
            label: '抓图',
            busy: _snapshotBusy,
            onTap: _takeSnapshot,
          ),
          _ToolButton(
            key: const Key('gb-fullscreen'),
            icon: Icons.fullscreen,
            label: '全屏',
            onTap: _toggleFullscreen,
          ),
          _ToolButton(
            key: const Key('gb-talk'),
            icon: _talkSessionId == null
                ? Icons.mic_none_outlined
                : Icons.mic,
            label: _talkSessionId == null ? '对讲' : '结束对讲',
            busy: _talkBusy,
            active: _talkSessionId != null,
            onTap: _toggleTalk,
          ),
          _ToolButton(
            key: const Key('gb-voice-chat'),
            icon: Icons.record_voice_over_outlined,
            label: '语音聊天',
            onTap: _openVoiceChat,
          ),
          _ToolButton(
            key: const Key('gb-mqtt-toggle'),
            icon: Icons.hub_outlined,
            label: _preferMqtt ? 'MQTT 信令' : 'HTTP 信令',
            active: _preferMqtt,
            onTap: _toggleSignaling,
          ),
          _ToolButton(
            key: const Key('gb-reconnect'),
            icon: Icons.refresh,
            label: '重连',
            onTap: _start,
          ),
        ],
      ),
    );
  }

  /// 在 MQTT 与 WHEP 两种信令之间切换（仅带 `deviceId` 的 IOT 设备可用）。
  void _toggleSignaling() {
    if (_deviceId == null) {
      // 国标监控入口只带 channelId，拿不到设备服务组；引导用户走 IOT 入口。
      context.showToast('该通道缺少设备信息，请从「我的设备」进入以使用 MQTT 信令');
      return;
    }
    setState(() => _preferMqtt = !_preferMqtt);
    unawaited(_start());
  }

  Widget _buildPtzCard() {
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
          Row(
            children: <Widget>[
              Expanded(
                child: Text('云台控制', style: AppText.section),
              ),
              if (_playInfo != null && _playInfo!.stream.isNotEmpty)
                Text(
                  '流 ${_playInfo!.stream}',
                  style: AppText.hint,
                ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          GbPtzPanel(
            enabled: _canControlPtz,
            initialSpeed: _ptzSpeed,
            onSpeedChanged: (int value) => _ptzSpeed = value,
            onCommand: _onPtzDirection,
            onAction: _onPtzAction,
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            '按住方向键移动，松开立即停止；单指拖动画面同样可以控制云台，双指缩放画面。',
            style: AppText.hint,
          ),
        ],
      ),
    );
  }

  Widget _buildInfoCard() {
    final String codecHint;
    switch (_mode) {
      case _GbMode.webrtc:
      case _GbMode.mqtt:
        codecHint = 'WebRTC 对 H.265 的支持取决于机型；若黑屏会自动回退 HTTP-FLV。';
        break;
      case _GbMode.flv:
        codecHint = 'HTTP-FLV 由设备/系统原生解码，H.265 支持取决于设备与系统版本。';
        break;
      case _GbMode.connecting:
      case _GbMode.paused:
      case _GbMode.error:
        codecHint = '播放内核按「MQTT 信令 → WHEP → HTTP-FLV」顺序尝试。';
        break;
    }

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
          Text('播放信息', style: AppText.section),
          SizedBox(height: AppSpacing.sm),
          InfoRow(label: '通道 ID', value: '${_channelId ?? '--'}'),
          InfoRow(label: '播放状态', value: _modeLabel),
          InfoRow(
            label: '信令方式',
            value: _preferMqtt
                ? 'MQTT（失败自动回退 WHEP）'
                : 'HTTP（WHEP）',
          ),
          if (_playInfo != null) ...<Widget>[
            InfoRow(
              label: '应用 / 流',
              value: '${_playInfo!.app} / ${_playInfo!.stream}',
            ),
            if (_playInfo!.streamId.isNotEmpty)
              InfoRow(label: '会话标识', value: _playInfo!.streamId),
          ],
          if (_mqttError != null)
            InfoRow(
              label: 'MQTT 失败原因',
              value: _mqttError!,
              valueColor: AppColors.warning,
            ),
          SizedBox(height: AppSpacing.sm),
          Text(codecHint, style: AppText.hint),
          SizedBox(height: AppSpacing.xs),
          Text(
            '离开本页或切到后台会自动停止点播并释放播放器，避免服务端残留流。',
            style: AppText.hint,
          ),
        ],
      ),
    );
  }

  String get _modeLabel {
    switch (_mode) {
      case _GbMode.webrtc:
        return 'WebRTC 实时';
      case _GbMode.mqtt:
        return 'MQTT 信令实时';
      case _GbMode.flv:
        return 'HTTP-FLV 实时';
      case _GbMode.connecting:
        return _status;
      case _GbMode.paused:
        return '已暂停';
      case _GbMode.error:
        return '播放失败';
    }
  }

  void _openPtzSheet() {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: AppColors.card,
      isScrollControlled: true,
      builder: (BuildContext ctx) => SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.all(AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              Row(
                children: <Widget>[
                  Expanded(
                    child: Text('云台控制', style: AppText.section),
                  ),
                  IconButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.sm),
              GbPtzPanel(
                enabled: _canControlPtz,
                initialSpeed: _ptzSpeed,
                onSpeedChanged: (int value) => _ptzSpeed = value,
                onCommand: _onPtzDirection,
                onAction: _onPtzAction,
              ),
            ],
          ),
        ),
      ),
    );
  }

  /// 仅在组件仍挂载时 setState，统一避免 `setState() called after dispose`。
  void _safeSetState(VoidCallback fn) {
    if (!mounted) return;
    setState(fn);
  }

  /// 异常 → 可读中文（WHEP 异常自带文案）。
  String _humanize(Object error) {
    if (error is GbWhepException) return error.message;
    if (error is MqttSignalingException) return error.message;
    return error.toString();
  }
}

/// 播放页工具栏按钮。
class _ToolButton extends StatelessWidget {
  const _ToolButton({
    super.key,
    required this.icon,
    required this.label,
    required this.onTap,
    this.busy = false,
    this.active = false,
  });

  final IconData icon;
  final String label;
  final VoidCallback? onTap;
  final bool busy;
  final bool active;

  @override
  Widget build(BuildContext context) {
    final Color color = active ? AppColors.primary : AppColors.textSecondary;
    return Expanded(
      child: InkWell(
        onTap: busy ? null : onTap,
        child: Padding(
          padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              if (busy)
                SizedBox(
                  width: 22.dp,
                  height: 22.dp,
                  child: const CircularProgressIndicator(strokeWidth: 2),
                )
              else
                Icon(icon, size: 22.dp, color: color),
              SizedBox(height: AppSpacing.xs),
              Text(
                label,
                style: AppText.hint.copyWith(color: color),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// 全屏浮层上的圆形图标按钮。
class _RoundIconButton extends StatelessWidget {
  const _RoundIconButton({
    super.key,
    required this.icon,
    required this.tooltip,
    required this.onTap,
  });

  final IconData icon;
  final String tooltip;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.black54,
      shape: const CircleBorder(),
      child: IconButton(
        tooltip: tooltip,
        onPressed: onTap,
        icon: Icon(icon, color: Colors.white),
      ),
    );
  }
}
