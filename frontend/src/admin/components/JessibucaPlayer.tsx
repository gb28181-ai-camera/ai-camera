import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from 'antd'
import { CameraOutlined, PauseCircleOutlined, PlayCircleOutlined } from '@ant-design/icons'
import { jessibucaDecoderUrl, loadJessibuca } from '../lib/jessibucaLoader'
import type { WasmPlayerFrameInfo, WasmPlayerStatus } from './H265WebPlayer'

/**
 * Jessibuca 播放器封装（WASM，H.264 / H.265）。
 *
 * ⚠️ **许可证：GPL-3.0**。详见 `public/static/js/jessibuca/README-LICENSE.md`。
 * 因此本组件只在用户手动选择 Jessibuca 页签时动态加载，**默认不作为首选引擎**。
 *
 * 封装的能力：play / pause / destroy / screenshot，首帧通过 `on('play')` 或
 * `videoInfo` 上报。decoder.js 路径由 `import.meta.env.BASE_URL` 推导，
 * 兼容 Vite `base=/admin/` 与 Electron `file://`。
 */

export interface JessibucaPlayerProps {
  /** 播放地址（通常为 ZLM ws-flv） */
  url: string
  /** 静音（浏览器自动播放策略要求），默认 true */
  muted?: boolean
  /** 是否显示最简播放/暂停按钮 */
  controls?: boolean
  /** 保留与 LivePlayer 一致的填充方式 prop */
  objectFit?: React.CSSProperties['objectFit']
  /** 首帧渲染完成 */
  onFirstFrame?: (info: WasmPlayerFrameInfo) => void
  /** 播放失败 */
  onError?: (error: Error) => void
  /** 播放状态变化 */
  onStatusChange?: (status: WasmPlayerStatus) => void
}

function toError(value: unknown): Error {
  if (value instanceof Error) return value
  if (typeof value === 'string' && value) return new Error(value)
  return new Error('Jessibuca 播放失败')
}

function readVideoDimension(payload: unknown, key: 'width' | 'height'): number | null {
  if (typeof payload !== 'object' || payload === null) return null
  const record = payload as Record<string, unknown>
  const candidates = key === 'width' ? [record.width, record.videoWidth] : [record.height, record.videoHeight]
  for (const value of candidates) {
    if (typeof value === 'number' && Number.isFinite(value) && value > 0) return Math.round(value)
  }
  return null
}

function errorMessage(payload: unknown, fallback: string): string {
  if (typeof payload === 'string' && payload) return payload
  if (payload instanceof Error && payload.message) return payload.message
  if (typeof payload === 'object' && payload !== null) {
    const record = payload as Record<string, unknown>
    const message = record.msg ?? record.message
    if (typeof message === 'string' && message) return message
  }
  return fallback
}

export default function JessibucaPlayer({
  url,
  muted = true,
  controls = false,
  onFirstFrame,
  onError,
  onStatusChange,
}: JessibucaPlayerProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const playerRef = useRef<JessibucaInstance | null>(null)
  const generationRef = useRef(0)
  const mutedRef = useRef(muted)
  const [status, setStatus] = useState<WasmPlayerStatus>('idle')
  const [playing, setPlaying] = useState(false)

  const handlersRef = useRef({ onFirstFrame, onError, onStatusChange })
  useEffect(() => {
    handlersRef.current = { onFirstFrame, onError, onStatusChange }
  })

  useEffect(() => {
    mutedRef.current = muted
    const player = playerRef.current
    if (!player) return
    if (muted) player.mute?.()
    else player.cancelMute?.()
  }, [muted])

  useEffect(() => {
    if (!url) {
      setStatus('idle')
      setPlaying(false)
      handlersRef.current.onStatusChange?.('idle')
      return undefined
    }

    const generation = generationRef.current + 1
    generationRef.current = generation
    const isStale = () => generationRef.current !== generation

    let disposed = false
    let instance: JessibucaInstance | null = null
    let probeWidth: number | null = null
    let probeHeight: number | null = null
    let firstFrameReported = false
    let playRequested = false
    let playFallbackTimer = 0

    const reportStatus = (next: WasmPlayerStatus) => {
      setStatus(next)
      handlersRef.current.onStatusChange?.(next)
    }

    const reportError = (error: unknown) => {
      if (disposed || isStale()) return
      setPlaying(false)
      reportStatus('error')
      handlersRef.current.onError?.(toError(error))
    }

    const reportFirstFrame = () => {
      if (disposed || isStale() || firstFrameReported) return
      firstFrameReported = true
      setPlaying(true)
      reportStatus('playing')
      const container = containerRef.current
      handlersRef.current.onFirstFrame?.({
        width: probeWidth ?? Math.max(1, Math.floor(container?.clientWidth ?? 0)),
        height: probeHeight ?? Math.max(1, Math.floor(container?.clientHeight ?? 0)),
      })
    }

    const requestPlay = (player: JessibucaInstance) => {
      if (playRequested || disposed || isStale()) return
      playRequested = true
      player.play(url)
    }

    const boot = async () => {
      reportStatus('connecting')
      try {
        await loadJessibuca()
        if (disposed || isStale()) return

        const container = containerRef.current
        const Constructor = window.Jessibuca
        if (!container || typeof Constructor !== 'function') {
          throw new Error('jessibuca.js 未加载：window.Jessibuca 不存在')
        }

        const player = new Constructor({
          container,
          decoder: jessibucaDecoderUrl(),
          videoBuffer: 0,
          isResize: true,
          // 与 WVP 保持一致：走 WASM 解码（MSE 只支持 H.264）
          useMSE: false,
          useWCS: false,
          isFlv: false,
          forceNoOffscreen: true,
          hasAudio: true,
          isNotMute: !mutedRef.current,
          timeout: 10,
          heartTimeout: 5,
          loadingTimeout: 10,
          loadingText: '视频加载中…',
          debug: false,
          hotKey: false,
          supportDblclickFullscreen: false,
          operateBtns: { fullscreen: false, screenshot: false, play: false, audio: false, recorder: false },
        })
        instance = player
        playerRef.current = player

        player.on('play', () => {
          if (disposed || isStale()) return
          reportFirstFrame()
        })
        player.on('pause', () => {
          if (disposed || isStale()) return
          setPlaying(false)
        })
        player.on('videoInfo', (payload) => {
          const width = readVideoDimension(payload, 'width')
          const height = readVideoDimension(payload, 'height')
          if (width !== null) probeWidth = width
          if (height !== null) probeHeight = height
          // 部分版本先给 videoInfo 再触发 play；这里兜底上报首帧
          reportFirstFrame()
        })
        player.on('error', (payload) => {
          reportError(errorMessage(payload, 'Jessibuca 播放错误'))
        })
        player.on('timeout', (payload) => {
          reportError(errorMessage(payload, 'Jessibuca 连接超时'))
        })
        player.on('loadingTimeout', (payload) => {
          reportError(errorMessage(payload, 'Jessibuca 加载超时'))
        })

        const loaded = player.hasLoaded?.() ?? false
        if (loaded) {
          requestPlay(player)
        } else {
          player.on('load', () => {
            if (!disposed && !isStale()) requestPlay(player)
          })
          // 兜底：部分版本不触发 load 事件；延迟一拍再播放，重复调用由 playRequested 拦截
          playFallbackTimer = window.setTimeout(() => {
            if (!disposed && !isStale()) requestPlay(player)
          }, 500)
        }
      } catch (error) {
        reportError(error)
      }
    }

    void boot()

    return () => {
      disposed = true
      generationRef.current = generation + 1
      window.clearTimeout(playFallbackTimer)
      const player = instance
      instance = null
      playerRef.current = null
      if (player) {
        try {
          player.destroy()
        } catch {
          /* 已销毁 */
        }
      }
    }
  }, [url])

  const handleToggle = useCallback(() => {
    const player = playerRef.current
    if (!player) return
    if (playing) {
      player.pause()
      setPlaying(false)
    } else {
      player.play(url)
      setPlaying(true)
    }
  }, [playing, url])

  const handleScreenshot = useCallback(() => {
    playerRef.current?.screenshot?.()
  }, [])

  return (
    <div
      className="e2e-jessibuca-player"
      data-status={status}
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        background: '#000',
        overflow: 'hidden',
      }}
    >
      {/* Jessibuca 会直接操作这个容器的 DOM（追加 canvas），不要让它与 React 子节点混在一起 */}
      <div ref={containerRef} style={{ width: '100%', height: '100%' }} />

      {controls && status === 'playing' ? (
        <div style={{ position: 'absolute', left: 8, bottom: 8, zIndex: 2, display: 'flex', gap: 4 }}>
          <Button
            size="small"
            type="text"
            aria-label={playing ? '暂停' : '播放'}
            icon={playing ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
            onClick={handleToggle}
            style={{ color: '#fff' }}
          />
          <Button
            size="small"
            type="text"
            aria-label="截图"
            icon={<CameraOutlined />}
            onClick={handleScreenshot}
            style={{ color: '#fff' }}
          />
        </div>
      ) : null}
    </div>
  )
}
