import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Spin } from 'antd'
import { CloseCircleOutlined } from '@ant-design/icons'

/**
 * 直播播放器内核：WebRTC(WHEP) 优先 / HTTP-FLV 兜底 / HLS 保底。
 *
 * 设计依据 `docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md` §7.4：
 * | 协议 | 延迟 | 用途 |
 * |:--|:--|:--|
 * | WebRTC / WHEP | 200–500ms | **主**：选中窗格、云台操作、语音对讲 |
 * | HTTP-FLV | 1–3s | **兜底**：视频墙铺格子（mpegts.js） |
 * | HLS | 5–10s | 仅保底 |
 *
 * ⚠️ 视频墙注意 WebRTC 连接数上限（浏览器约 10–50），**16 宫格必须用 FLV 铺格子，
 * WebRTC 只给选中格子** —— 因此本组件把播放协议做成 prop，由调用方决定。
 *
 * 组件的职责边界：**只吃流地址参数，不调用任何后端接口**。播放地址由调用方
 * 从 `/web/gb/play`（或 v0.1.0 的 `device.stream_url`）拿到后传入。
 *
 * @example 国标流名 = SSRC 的大写十六进制（如 `0BEBC201`），见设计 §2.2 与契约 §15.1。
 *          `{deviceId}_{channelId}` 是 **WVP 的内部别名**，ZLM 不认识，不要用。
 * <LivePlayer httpBase="http://192.168.1.227:8080" app="rtp" stream="34020000001320000001_34020000001320000001" />
 *
 * @example 视频墙铺格子用 FLV
 * <LivePlayer httpBase={zlm} stream={stream} protocol="flv" objectFit="cover" />
 *
 * @example 兼容 v0.1.0 的整串地址
 * <LivePlayer url={device.stream_url} />
 */
export type LiveProtocol = 'webrtc' | 'flv' | 'hls' | 'native'

/** 播放状态：idle 无地址 / connecting 连接中 / playing 已启动 / error 失败 */
export type LivePlayerStatus = 'idle' | 'connecting' | 'playing' | 'error'

/** 首帧渲染事件 —— E2E 用它断言「画面真的出来了」，而不是「页面没崩」 */
export interface LiveFirstFrameEvent {
  /** 实际生效的协议（可能因降级与请求协议不同） */
  protocol: LiveProtocol
  width: number
  height: number
  /** 从开始播放到解出首帧的耗时（毫秒） */
  elapsedMs: number
}

export interface LivePlayerProps {
  /** ZLM 应用名，单端口模式下恒为 `rtp`（§7.3），默认 `rtp` */
  app?: string
  /** 流名；国标场景为 SSRC 的大写十六进制（如 `0BEBC201`） */
  stream?: string
  /** 媒体服务对外地址根（ZLM 的 http 端口），如 `http://192.168.1.227:8080` */
  httpBase?: string
  /**
   * 直接给出完整播放地址（兼容 v0.1.0 的 `device.stream_url`）。
   * 提供时忽略 `app` / `stream` / `httpBase`。
   */
  url?: string
  /** 播放协议；缺省时：给了 `url` 按扩展名推断，否则 `webrtc` */
  protocol?: LiveProtocol
  /** WebRTC 失败时自动回落 HTTP-FLV，默认 true */
  fallbackToFlv?: boolean
  /** 静音，默认 true（浏览器自动播放策略要求） */
  muted?: boolean
  /** 显示原生控制条，默认 false */
  controls?: boolean
  /** 视频填充方式，默认 `contain`（视频墙可用 `cover`） */
  objectFit?: React.CSSProperties['objectFit']
  className?: string
  style?: React.CSSProperties
  /** 无地址或未开始时展示的占位内容 */
  placeholder?: React.ReactNode
  /** 播放失败（含回落之后仍失败） */
  onError?: (error: Error, protocol: LiveProtocol) => void
  /** 首帧渲染完成 */
  onFirstFrame?: (event: LiveFirstFrameEvent) => void
  /** 协议发生实际变化（如 WebRTC 失败后回落 FLV） */
  onProtocolChange?: (protocol: LiveProtocol) => void
  /** 播放状态变化 */
  onStatusChange?: (status: LivePlayerStatus, protocol: LiveProtocol) => void
}

const DEFAULT_APP = 'rtp'
const DEFAULT_PROTOCOL: LiveProtocol = 'webrtc'
/** 首帧轮询间隔 */
const FIRST_FRAME_POLL_MS = 200
/** 首帧等待上限，超时则不再上报 onFirstFrame（不视为错误） */
const FIRST_FRAME_TIMEOUT_MS = 20_000
/** ICE 收集等待上限：超过则用已有候选直接发起信令，避免 LAN 场景下无限等待 */
const ICE_GATHERING_TIMEOUT_MS = 3_000

const OVERLAY_STYLE: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  pointerEvents: 'none',
}

/* ------------------------------------------------------------------ *
 * 纯函数工具（模块级，便于单测与复用）
 * ------------------------------------------------------------------ */

/** 去掉地址根的尾部斜杠 */
function trimBase(httpBase: string): string {
  return httpBase.replace(/\/+$/, '')
}

/** 按协议拼装 ZLM 播放地址；`native` 无法由 app/stream 推导，返回空串 */
function buildZlmStreamUrl(base: string, app: string, stream: string, protocol: LiveProtocol): string {
  const appPart = encodeURIComponent(app)
  const streamPart = encodeURIComponent(stream)
  switch (protocol) {
    case 'webrtc':
      // ZLM WebRTC：POST offer SDP，响应体为 answer SDP（§7.4）
      return `${base}/index/api/webrtc?app=${appPart}&stream=${streamPart}&type=play`
    case 'flv':
      // ZLM HTTP-FLV：{app}/{stream}.live.flv
      return `${base}/${appPart}/${streamPart}.live.flv`
    case 'hls':
      // ZLM HLS：{app}/{stream}/hls.m3u8
      return `${base}/${appPart}/${streamPart}/hls.m3u8`
    case 'native':
      return ''
    default:
      return ''
  }
}

/** 推断协议：显式指定优先，否则按 url 扩展名判断 */
function resolveProtocol(input: { url?: string; protocol?: LiveProtocol }): LiveProtocol {
  if (input.protocol) return input.protocol
  const direct = input.url?.trim()
  if (direct) {
    const path = direct.split('?')[0]?.toLowerCase() ?? ''
    if (path.endsWith('.flv')) return 'flv'
    if (path.endsWith('.m3u8')) return 'hls'
    return 'native'
  }
  return DEFAULT_PROTOCOL
}

/** 计算最终播放地址；返回空串表示「无地址」，组件进入 idle 占位 */
function resolvePlayAddress(input: {
  app: string
  stream?: string
  httpBase?: string
  url?: string
  protocol: LiveProtocol
}): string {
  const direct = input.url?.trim()
  if (direct) return direct
  const base = input.httpBase?.trim()
  const stream = input.stream?.trim()
  if (!base || !stream) return ''
  return buildZlmStreamUrl(trimBase(base), input.app, stream, input.protocol)
}

/** 统一错误类型 */
function toError(value: unknown): Error {
  if (value instanceof Error) return value
  if (typeof value === 'string' && value) return new Error(value)
  return new Error('播放失败')
}

/** ZLM 在信令失败时会返回 JSON（`{"code":-1,"msg":"..."}`）而非 SDP，这里提取可读信息 */
function extractZlmMessage(body: string): string | null {
  const text = body.trim()
  if (!text.startsWith('{')) return null
  try {
    const parsed: unknown = JSON.parse(text)
    if (parsed !== null && typeof parsed === 'object' && 'msg' in parsed) {
      const msg = (parsed as { msg?: unknown }).msg
      if (typeof msg === 'string' && msg) return msg
    }
  } catch {
    /* 非 JSON 体，按原文截断返回 */
  }
  return text.slice(0, 200)
}

/** 等待 ICE 收集完成；超时后用已有候选继续，避免 LAN 部署无限等待 */
function waitIceGatheringComplete(
  pc: RTCPeerConnection,
  timeoutMs = ICE_GATHERING_TIMEOUT_MS,
): Promise<void> {
  if (pc.iceGatheringState === 'complete') return Promise.resolve()
  return new Promise((resolve) => {
    let settled = false
    let timer = 0
    const finish = () => {
      if (settled) return
      settled = true
      window.clearTimeout(timer)
      pc.removeEventListener('icegatheringstatechange', handleChange)
      resolve()
    }
    function handleChange() {
      if (pc.iceGatheringState === 'complete') finish()
    }
    timer = window.setTimeout(finish, timeoutMs)
    pc.addEventListener('icegatheringstatechange', handleChange)
  })
}

/**
 * 轮询等待首帧真的解出来（`videoWidth > 0`）。
 *
 * 用轮询而不是 `requestVideoFrameCallback`：WebRTC 与 FLV(MSE) 两条路径的
 * 元素状态机不同，`videoWidth` 是两者共有的、最可靠的「已有画面」信号。
 * 超时后静默停止（不触发 onFirstFrame），由调用方按需处理。
 *
 * @returns 取消函数
 */
function watchFirstFrame(video: HTMLVideoElement, onFirstFrame: () => void): () => void {
  let stopped = false
  let elapsed = 0
  let timer = 0

  const tick = () => {
    if (stopped) return
    if (video.videoWidth > 0 && video.videoHeight > 0) {
      stopped = true
      onFirstFrame()
      return
    }
    elapsed += FIRST_FRAME_POLL_MS
    if (elapsed >= FIRST_FRAME_TIMEOUT_MS) {
      stopped = true
      return
    }
    timer = window.setTimeout(tick, FIRST_FRAME_POLL_MS)
  }

  tick()
  return () => {
    stopped = true
    window.clearTimeout(timer)
  }
}

/** 播放前清空上一次的媒体源，避免 WebRTC 的 srcObject 与 FLV/HLS 的 src 互相干扰 */
function resetVideoElement(video: HTMLVideoElement): void {
  try {
    video.pause()
  } catch {
    /* 元素已卸载 */
  }
  video.srcObject = null
  video.removeAttribute('src')
}

/** 启动 WebRTC 播放，返回释放函数（**必须**调用，否则连接泄漏） */
async function startWebRtc(
  video: HTMLVideoElement,
  url: string,
  isDisposed: () => boolean,
): Promise<() => void> {
  const pc = new RTCPeerConnection({ iceServers: [] })
  const remoteStream = new MediaStream()
  let released = false

  const release = () => {
    if (released) return
    released = true
    pc.ontrack = null
    pc.onconnectionstatechange = null
    try {
      pc.close()
    } catch {
      /* 已关闭 */
    }
    remoteStream.getTracks().forEach((track) => {
      try {
        remoteStream.removeTrack(track)
      } catch {
        /* 忽略 */
      }
    })
    if (video.srcObject === remoteStream) video.srcObject = null
  }

  try {
    video.srcObject = remoteStream
    // ZLM 要求 offer 里同时声明音视频收流能力
    pc.addTransceiver('video', { direction: 'recvonly' })
    pc.addTransceiver('audio', { direction: 'recvonly' })
    pc.ontrack = (event: RTCTrackEvent) => {
      remoteStream.addTrack(event.track)
    }

    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    await waitIceGatheringComplete(pc)
    const localSdp = pc.localDescription?.sdp ?? offer.sdp
    if (!localSdp) throw new Error('WebRTC 本地 SDP 生成失败')

    const response = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/sdp' },
      body: localSdp,
    })
    const answerSdp = await response.text()

    if (isDisposed()) return release
    if (!response.ok || answerSdp.trimStart().startsWith('{')) {
      throw new Error(extractZlmMessage(answerSdp) ?? `WebRTC 信令失败（HTTP ${response.status}）`)
    }

    await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp })
    await video.play().catch(() => {
      /* 自动播放被拦截：保持连接，等待用户交互后再播放 */
    })
    return release
  } catch (error) {
    release()
    throw toError(error)
  }
}

/**
 * 启动 HTTP-FLV 播放（mpegts.js；`flv.js` 已停止维护，按设计 §7.4 不用）。
 * 引擎按需动态引入，WebRTC 场景不加载该 chunk。
 */
async function startFlv(
  video: HTMLVideoElement,
  url: string,
  isDisposed: () => boolean,
  onRuntimeError: (error: Error) => void,
): Promise<() => void> {
  const { default: mpegts } = await import('mpegts.js')
  if (typeof mpegts.isSupported === 'function' && !mpegts.isSupported()) {
    throw new Error('当前浏览器不支持 MSE（Media Source Extensions），无法播放 HTTP-FLV')
  }
  if (isDisposed()) throw new Error('已取消播放')

  const player = mpegts.createPlayer(
    { type: 'flv', isLive: true, url, hasAudio: true, hasVideo: true },
    {
      // 低延迟优先：不启用 stash 缓冲，并追平 HTMLMediaElement 内部缓冲累积的延迟
      enableStashBuffer: false,
      stashInitialSize: 128,
      isLive: true,
      liveBufferLatencyChasing: true,
      liveBufferLatencyMaxLatency: 1.5,
      liveBufferLatencyMinRemain: 0.5,
      autoCleanupSourceBuffer: true,
      lazyLoad: false,
      deferLoadAfterSourceOpen: false,
    },
  )

  const handleError = (errorType: string, errorDetail: string) => {
    onRuntimeError(new Error(`HTTP-FLV 播放失败（${errorType}: ${errorDetail}）`))
  }
  player.on(mpegts.Events.ERROR, handleError)

  let released = false
  const release = () => {
    if (released) return
    released = true
    player.off(mpegts.Events.ERROR, handleError)
    try {
      player.pause()
    } catch {
      /* 已销毁 */
    }
    try {
      player.unload()
    } catch {
      /* 已销毁 */
    }
    try {
      player.detachMediaElement()
    } catch {
      /* 已销毁 */
    }
    try {
      player.destroy()
    } catch {
      /* 已销毁 */
    }
  }

  try {
    player.attachMediaElement(video)
    player.load()
    const playResult: unknown = player.play()
    if (playResult instanceof Promise) {
      await playResult.catch(() => {
        /* 自动播放被拦截 */
      })
    }
    return release
  } catch (error) {
    release()
    throw toError(error)
  }
}

/**
 * 直接交给浏览器原生播放（mp4 / Safari 的原生 HLS）。
 *
 * ⚠️ 设计 §7.4 为 HLS 指定的前端库是 `hls.js`，**当前未安装**（本批交付只加了 `mpegts.js`）。
 * 因此 `hls` / `native` 两条路径都走这里的原生实现：Chrome / Electron 下能播 mp4，
 * 但 **HLS（.m3u8）在 Chromium 内核里原生不支持**。若要在浏览器播放 ZLM 的 HLS，
 * 需 `npm i hls.js` 并在此分支接入（改动量约 20 行）。
 */
async function startNative(
  video: HTMLVideoElement,
  url: string,
  isDisposed: () => boolean,
  onRuntimeError: (error: Error) => void,
): Promise<() => void> {
  const handleError = () => {
    if (isDisposed()) return
    onRuntimeError(new Error('浏览器原生播放失败：地址不可播或格式不受支持'))
  }
  video.addEventListener('error', handleError)
  video.src = url
  await video.play().catch(() => {
    /* 自动播放被拦截 */
  })
  return () => {
    video.removeEventListener('error', handleError)
    video.removeAttribute('src')
  }
}

/* ------------------------------------------------------------------ *
 * 组件
 * ------------------------------------------------------------------ */

export default function LivePlayer({
  app = DEFAULT_APP,
  stream,
  httpBase,
  url,
  protocol,
  fallbackToFlv = true,
  muted = true,
  controls = false,
  objectFit = 'contain',
  className,
  style,
  placeholder,
  onError,
  onFirstFrame,
  onProtocolChange,
  onStatusChange,
}: LivePlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const frameWatchRef = useRef<(() => void) | null>(null)

  const [status, setStatus] = useState<LivePlayerStatus>('idle')
  const [activeProtocol, setActiveProtocol] = useState<LiveProtocol>(protocol ?? DEFAULT_PROTOCOL)
  const [errorText, setErrorText] = useState('')
  const [frameReady, setFrameReady] = useState(false)

  // 回调统一走 ref：父组件用内联箭头函数时不会导致播放被反复重建
  const handlersRef = useRef({ onError, onFirstFrame, onProtocolChange, onStatusChange })
  useEffect(() => {
    handlersRef.current = { onError, onFirstFrame, onProtocolChange, onStatusChange }
  })

  const resolvedProtocol = useMemo(() => resolveProtocol({ url, protocol }), [url, protocol])
  const playAddress = useMemo(
    () => resolvePlayAddress({ app, stream, httpBase, url, protocol: resolvedProtocol }),
    [app, stream, httpBase, url, resolvedProtocol],
  )

  useEffect(() => {
    const video = videoRef.current
    if (!video || !playAddress) {
      setStatus('idle')
      setFrameReady(false)
      return
    }

    let disposed = false
    let release: (() => void) | null = null
    const isDisposed = () => disposed

    const releaseCurrent = () => {
      release?.()
      release = null
    }
    const releaseFrameWatch = () => {
      frameWatchRef.current?.()
      frameWatchRef.current = null
    }
    const reportStatus = (next: LivePlayerStatus, used: LiveProtocol) => {
      setStatus(next)
      handlersRef.current.onStatusChange?.(next, used)
    }
    const reportError = (error: Error, used: LiveProtocol) => {
      setErrorText(error.message)
      reportStatus('error', used)
      handlersRef.current.onError?.(error, used)
    }
    const startFrameWatch = (used: LiveProtocol) => {
      releaseFrameWatch()
      const startedAt = performance.now()
      frameWatchRef.current = watchFirstFrame(video, () => {
        if (isDisposed()) return
        setFrameReady(true)
        handlersRef.current.onFirstFrame?.({
          protocol: used,
          width: video.videoWidth,
          height: video.videoHeight,
          elapsedMs: Math.round(performance.now() - startedAt),
        })
      })
    }

    const start = (target: LiveProtocol): Promise<() => void> => {
      resetVideoElement(video)
      if (target === 'webrtc') return startWebRtc(video, playAddress, isDisposed)
      if (target === 'flv') {
        return startFlv(video, playAddress, isDisposed, (error) => {
          if (isDisposed()) return
          reportError(error, 'flv')
        })
      }
      return startNative(video, playAddress, isDisposed, (error) => {
        if (isDisposed()) return
        reportError(error, target)
      })
    }

    const boot = async () => {
      setFrameReady(false)
      setErrorText('')
      setActiveProtocol(resolvedProtocol)
      reportStatus('connecting', resolvedProtocol)
      releaseCurrent()
      releaseFrameWatch()

      try {
        release = await start(resolvedProtocol)
        if (disposed) {
          releaseCurrent()
          return
        }
        startFrameWatch(resolvedProtocol)
        reportStatus('playing', resolvedProtocol)
      } catch (error) {
        if (disposed) return
        // WebRTC 失败 → 自动回落 HTTP-FLV（视频墙 / 不支持 WebRTC 的环境）
        if (resolvedProtocol === 'webrtc' && fallbackToFlv) {
          try {
            setActiveProtocol('flv')
            release = await start('flv')
            if (disposed) {
              releaseCurrent()
              return
            }
            startFrameWatch('flv')
            reportStatus('playing', 'flv')
            handlersRef.current.onProtocolChange?.('flv')
            return
          } catch (fallbackError) {
            reportError(toError(fallbackError), 'flv')
            return
          }
        }
        reportError(toError(error), resolvedProtocol)
      }
    }

    void boot()

    return () => {
      disposed = true
      releaseCurrent()
      releaseFrameWatch()
    }
  }, [playAddress, resolvedProtocol, fallbackToFlv])

  const showPlaceholder = status === 'idle'
  const showConnecting = status === 'connecting'
  const showError = status === 'error'

  return (
    <div
      className={['e2e-live-player', className].filter(Boolean).join(' ')}
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        background: '#000',
        overflow: 'hidden',
        ...style,
      }}
      data-protocol={activeProtocol}
      data-status={status}
      data-first-frame={frameReady ? 'true' : 'false'}
    >
      <video
        ref={videoRef}
        autoPlay
        muted={muted}
        controls={controls}
        playsInline
        style={{
          width: '100%',
          height: '100%',
          objectFit,
          background: '#000',
          display: showPlaceholder || showError ? 'none' : 'block',
        }}
      />

      {showPlaceholder ? (
        <div style={OVERLAY_STYLE}>
          {placeholder ?? <span style={{ color: '#8c8c8c' }}>无信号</span>}
        </div>
      ) : null}

      {showConnecting ? (
        <div style={OVERLAY_STYLE}>
          <Spin size="small" />
          <span style={{ marginLeft: 8, color: '#bfbfbf' }}>连接中…</span>
        </div>
      ) : null}

      {showError ? (
        <div style={OVERLAY_STYLE}>
          <div style={{ color: '#bfbfbf', textAlign: 'center', padding: '0 12px' }}>
            <CloseCircleOutlined style={{ marginRight: 6 }} />
            播放失败
            <div style={{ marginTop: 4, fontSize: 12, color: '#8c8c8c' }}>{errorText}</div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
