import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Spin, Tabs } from 'antd'
import { CloseCircleOutlined } from '@ant-design/icons'
import LivePlayer, { type LivePlayerStatus } from './LivePlayer'
import H265WebPlayer, { type WasmPlayerFrameInfo } from './H265WebPlayer'
import JessibucaPlayer from './JessibucaPlayer'
import {
  PLAYER_ENGINES,
  PLAYER_ENGINE_E2E_CLASS,
  PLAYER_ENGINE_LABELS,
  fallbackChain,
  resolveDefaultEngine,
  resolveEngineUrl,
  type PlayerEngine,
  type PlayerEngineSource,
} from '../lib/playerEngines'

/**
 * 多引擎直播播放器：WebRTC / HTTP-FLV / Jessibuca / H265web / HLS。
 *
 * 与 WVP-GB28181-Pro 的 playerTabs 对齐，并补上 HTTP-FLV 与 HLS 两个页签：
 * - **单通道**默认 WebRTC（低延迟），**codec=H265** 默认 H265web（WASM）；
 * - **视频墙**由调用方传 `defaultEngine="flv"` 且 `allowSwitch={false}`，不渲染页签；
 * - 切换页签只切前端引擎，**不重新调用后端点播**（复用调用方已拿到的地址）；
 * - 用户手动选择后不再自动回落；只有默认引擎失败时按
 *   `webrtc→flv→hls` 或 `h265web→hls→flv` 回落。
 *
 * E2E：
 * - 容器 `e2e-live-player-tabs`，属性 `data-engine` / `data-status` / `data-first-frame`
 * - 页签 `e2e-player-tab-webrtc|flv|jessibuca|h265web|hls`
 * - 兼容原有 `e2e-live-player`（WebRTC / FLV / HLS 子组件保留）
 */

export interface LivePlayerTabsFirstFrameEvent {
  /** 实际出画的引擎（可能因回落与默认引擎不同） */
  engine: PlayerEngine
  width: number
  height: number
  /** 从本次引擎启动到首帧的耗时（毫秒） */
  elapsedMs: number
}

export interface LivePlayerTabsProps {
  /** ZLM 应用名，默认 `rtp` */
  app?: string
  /** 流名（国标为 SSRC 大写十六进制） */
  stream?: string
  /** ZLM HTTP 地址根 */
  httpBase?: string
  /** 调用方直给的整串地址（兼容 device.stream_url） */
  url?: string
  /** 按引擎覆盖地址（优先级最高），如后端返回的 webrtc/flv/hls/ws_flv */
  urls?: Partial<Record<PlayerEngine, string>>
  /** 允许的引擎（同时决定页签顺序），默认全部 5 个 */
  engines?: readonly PlayerEngine[]
  /** 显式指定默认引擎；不传时按 codec 推导（H265→h265web，其余→webrtc） */
  defaultEngine?: PlayerEngine
  /** 是否渲染页签允许手动切换，默认 true */
  allowSwitch?: boolean
  /** 视频编码，如 H264 / H265 / HEVC */
  codec?: string
  /** 默认引擎失败时是否自动回落，默认 true（用户手选后不再回落） */
  autoFallback?: boolean
  /** 静音，默认 true */
  muted?: boolean
  /** 显示控制条，默认 false */
  controls?: boolean
  /** 视频填充方式，默认 `contain` */
  objectFit?: React.CSSProperties['objectFit']
  className?: string
  style?: React.CSSProperties
  /** 无地址时展示的占位内容 */
  placeholder?: React.ReactNode
  /** 引擎变化（手动切换 / 自动回落） */
  onEngineChange?: (engine: PlayerEngine, reason: 'manual' | 'fallback') => void
  /** 首帧渲染完成 */
  onFirstFrame?: (event: LivePlayerTabsFirstFrameEvent) => void
  /** 所有可用引擎均失败 */
  onError?: (error: Error, engine: PlayerEngine) => void
  /** 播放状态变化 */
  onStatusChange?: (status: LivePlayerStatus, engine: PlayerEngine) => void
}

const OVERLAY_STYLE: React.CSSProperties = {
  position: 'absolute',
  inset: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  pointerEvents: 'none',
}

const DEFAULT_PLACEHOLDER = <span style={{ color: '#8c8c8c' }}>无信号</span>

export default function LivePlayerTabs({
  app,
  stream,
  httpBase,
  url,
  urls,
  engines,
  defaultEngine,
  allowSwitch = true,
  codec,
  autoFallback = true,
  muted = true,
  controls = false,
  objectFit = 'contain',
  className,
  style,
  placeholder,
  onEngineChange,
  onFirstFrame,
  onError,
  onStatusChange,
}: LivePlayerTabsProps) {
  const availableEngines = useMemo<PlayerEngine[]>(() => {
    const requested = engines && engines.length > 0 ? Array.from(engines) : Array.from(PLAYER_ENGINES)
    return Array.from(new Set(requested))
  }, [engines])

  // 依赖数组使用 join 后的稳定字符串，避免调用方内联数组导致重置循环
  const enginesKey = availableEngines.join(',')

  const initialEngine = useMemo<PlayerEngine>(() => {
    const preferred = resolveDefaultEngine({ defaultEngine, codec })
    return availableEngines.includes(preferred) ? preferred : availableEngines[0]
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [defaultEngine, codec, enginesKey])

  const [engine, setEngine] = useState<PlayerEngine>(initialEngine)
  const [status, setStatus] = useState<LivePlayerStatus>('idle')
  const [frameReady, setFrameReady] = useState(false)
  const [errorText, setErrorText] = useState('')

  const engineRef = useRef<PlayerEngine>(initialEngine)
  const userPickedRef = useRef(false)
  const attemptedRef = useRef<PlayerEngine[]>([initialEngine])
  const startedAtRef = useRef(0)
  const frameReportedRef = useRef(false)

  const handlersRef = useRef({ onEngineChange, onFirstFrame, onError, onStatusChange })
  useEffect(() => {
    handlersRef.current = { onEngineChange, onFirstFrame, onError, onStatusChange }
  })

  // urls 通常是内联对象，用序列化后的值做依赖，避免每次渲染都重置引擎
  const urlsKey = JSON.stringify(urls ?? null)
  const source = useMemo<PlayerEngineSource>(
    () => ({ app, stream, httpBase, url, urls }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [app, stream, httpBase, url, urlsKey],
  )
  const resolvedUrl = useMemo(() => resolveEngineUrl(engine, source), [engine, source])

  const reportStatus = useCallback((next: LivePlayerStatus, used: PlayerEngine) => {
    setStatus(next)
    handlersRef.current.onStatusChange?.(next, used)
  }, [])

  const reportFirstFrame = useCallback((info: WasmPlayerFrameInfo) => {
    if (frameReportedRef.current) return
    frameReportedRef.current = true
    setFrameReady(true)
    reportStatus('playing', engineRef.current)
    handlersRef.current.onFirstFrame?.({
      engine: engineRef.current,
      width: info.width,
      height: info.height,
      elapsedMs: Math.max(0, Math.round(performance.now() - startedAtRef.current)),
    })
  }, [reportStatus])

  /** 初始化 / 地址变化：回到默认引擎并清空回落记录 */
  useEffect(() => {
    const address = resolveEngineUrl(initialEngine, { app, stream, httpBase, url, urls })
    attemptedRef.current = [initialEngine]
    userPickedRef.current = false
    frameReportedRef.current = false
    startedAtRef.current = performance.now()
    engineRef.current = initialEngine
    setEngine(initialEngine)
    setFrameReady(false)
    setErrorText('')
    setStatus(address ? 'connecting' : 'idle')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialEngine, app, stream, httpBase, url, urlsKey])

  const handleEngineError = useCallback(
    (error: Error) => {
      const failed = engineRef.current

      // 用户手选后不自动回落
      if (!userPickedRef.current && autoFallback) {
        const chain = fallbackChain(initialEngine, availableEngines)
        const index = chain.indexOf(failed)
        const candidates = index >= 0 ? chain.slice(index + 1) : chain.filter((item) => item !== failed)
        const next = candidates.find((item) => !attemptedRef.current.includes(item))
        if (next) {
          attemptedRef.current.push(next)
          startedAtRef.current = performance.now()
          frameReportedRef.current = false
          engineRef.current = next
          setEngine(next)
          setFrameReady(false)
          setErrorText('')
          setStatus('connecting')
          handlersRef.current.onStatusChange?.('connecting', next)
          handlersRef.current.onEngineChange?.(next, 'fallback')
          return
        }
      }

      setErrorText(error.message)
      reportStatus('error', failed)
      handlersRef.current.onError?.(error, failed)
    },
    [autoFallback, availableEngines, initialEngine, reportStatus],
  )

  const handleEngineStatus = useCallback(
    (next: LivePlayerStatus) => {
      // 首帧后保持 playing，避免子组件后置的 connecting 覆盖
      if (frameReportedRef.current && next !== 'error') return
      reportStatus(next, engineRef.current)
    },
    [reportStatus],
  )

  const handleManualSwitch = useCallback(
    (key: string) => {
      const next = availableEngines.find((item) => item === key)
      if (!next || next === engineRef.current) return
      userPickedRef.current = true
      attemptedRef.current = [next]
      startedAtRef.current = performance.now()
      frameReportedRef.current = false
      engineRef.current = next
      setEngine(next)
      setFrameReady(false)
      setErrorText('')
      setStatus('connecting')
      handlersRef.current.onStatusChange?.('connecting', next)
      handlersRef.current.onEngineChange?.(next, 'manual')
    },
    [availableEngines],
  )

  const hasAddress = resolvedUrl !== ''
  const isWasmEngine = engine === 'h265web' || engine === 'jessibuca'
  const showPlaceholder = !hasAddress
  const showConnecting = hasAddress && status === 'connecting'
  const showError = hasAddress && status === 'error'

  const tabItems = availableEngines.map((item) => ({
    key: item,
    label: <span className={PLAYER_ENGINE_E2E_CLASS[item]}>{PLAYER_ENGINE_LABELS[item]}</span>,
  }))

  const renderEngine = () => {
    if (!hasAddress) return null
    if (engine === 'h265web') {
      return (
        <H265WebPlayer
          url={resolvedUrl}
          muted={muted}
          controls={controls}
          objectFit={objectFit}
          onFirstFrame={reportFirstFrame}
          onError={handleEngineError}
          onStatusChange={handleEngineStatus}
        />
      )
    }
    if (engine === 'jessibuca') {
      return (
        <JessibucaPlayer
          url={resolvedUrl}
          muted={muted}
          controls={controls}
          objectFit={objectFit}
          onFirstFrame={reportFirstFrame}
          onError={handleEngineError}
          onStatusChange={handleEngineStatus}
        />
      )
    }
    // webrtc / flv / hls 复用既有 LivePlayer（保持 e2e-live-player 与状态机不变）
    return (
      <LivePlayer
        url={resolvedUrl}
        protocol={engine}
        fallbackToFlv={false}
        muted={muted}
        controls={controls}
        objectFit={objectFit}
        style={{ width: '100%', height: '100%' }}
        placeholder={placeholder}
        onFirstFrame={(event) => reportFirstFrame({ width: event.width, height: event.height })}
        onError={handleEngineError}
        onStatusChange={handleEngineStatus}
      />
    )
  }

  return (
    <div
      className={['e2e-live-player-tabs', className].filter(Boolean).join(' ')}
      data-engine={engine}
      data-status={status}
      data-first-frame={frameReady ? 'true' : 'false'}
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        background: '#000',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
        ...style,
      }}
    >
      {allowSwitch && availableEngines.length > 1 ? (
        <Tabs
          size="small"
          activeKey={engine}
          items={tabItems}
          onChange={handleManualSwitch}
          tabBarStyle={{ marginBottom: 0, paddingLeft: 8 }}
          style={{ flex: '0 0 auto' }}
        />
      ) : null}

      <div style={{ position: 'relative', flex: 1, minHeight: 0, overflow: 'hidden' }}>
        {renderEngine()}

        {/* LivePlayer 自带占位/连接/错误浮层；WASM 引擎由这里统一渲染 */}
        {isWasmEngine && showPlaceholder ? (
          <div style={OVERLAY_STYLE}>{placeholder ?? DEFAULT_PLACEHOLDER}</div>
        ) : null}

        {isWasmEngine && showConnecting ? (
          <div style={OVERLAY_STYLE}>
            <Spin size="small" />
            <span style={{ marginLeft: 8, color: '#bfbfbf' }}>连接中…</span>
          </div>
        ) : null}

        {isWasmEngine && showError ? (
          <div style={OVERLAY_STYLE}>
            <div style={{ color: '#bfbfbf', textAlign: 'center', padding: '0 12px' }}>
              <CloseCircleOutlined style={{ marginRight: 6 }} />
              播放失败
              <div style={{ marginTop: 4, fontSize: 12, color: '#8c8c8c' }}>{errorText}</div>
            </div>
          </div>
        ) : null}
      </div>
    </div>
  )
}
