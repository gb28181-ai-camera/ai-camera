import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from 'antd'
import { PauseCircleOutlined, PlayCircleOutlined } from '@ant-design/icons'
import { h265WebBaseUrl, loadH265Web } from '../lib/h265webLoader'

/**
 * h265web.js 播放器封装（WASM，支持 H.264 / H.265）。
 *
 * 生命周期（对应 h265web 新版 SDK）：
 * `H265webjsPlayer()` → `build(config)` → `load_media(url)`
 * → `on_ready_show_done_callback`（首帧就绪，此时才 `play()`）→ `release()`。
 *
 * 两个容易踩的坑：
 * 1. **容器尺寸为 0 时 build 会失败**：本组件用 ResizeObserver 等待容器可见后再 build；
 * 2. **React StrictMode 会双挂载**：旧实例的异步回调必须被 generation token 拦截，
 *    否则会把已经 release 的播放器状态写回新实例。
 *
 * ⚠️ h265web.js 使用自定义许可证（CYL_Free-1.0），详见静态资源目录下的 README-LICENSE.md。
 */

export type WasmPlayerStatus = 'idle' | 'connecting' | 'playing' | 'error'

export interface WasmPlayerFrameInfo {
  width: number
  height: number
}

export interface H265WebPlayerProps {
  /** 播放地址（通常为 ZLM ws-flv） */
  url: string
  /** 静音（浏览器自动播放策略要求），默认 true */
  muted?: boolean
  /** 是否显示最简播放/暂停按钮 */
  controls?: boolean
  /** 保留与 LivePlayer 一致的填充方式 prop（WASM 播放器内部等比缩放） */
  objectFit?: React.CSSProperties['objectFit']
  /** 首帧渲染完成 */
  onFirstFrame?: (info: WasmPlayerFrameInfo) => void
  /** 播放失败 */
  onError?: (error: Error) => void
  /** 播放状态变化 */
  onStatusChange?: (status: WasmPlayerStatus) => void
}

let playerSeq = 0

function nextPlayerId(): string {
  playerSeq += 1
  return `h265web-player-${playerSeq}`
}

function toError(value: unknown): Error {
  if (value instanceof Error) return value
  if (typeof value === 'string' && value) return new Error(value)
  return new Error('h265web 播放失败')
}

function measure(element: HTMLElement): { width: number; height: number } {
  return {
    width: Math.floor(element.clientWidth),
    height: Math.floor(element.clientHeight),
  }
}

function readDimension(
  info: H265webMediaInfo | null,
  key: 'width' | 'height',
  fallback: number,
): number {
  if (!info) return fallback
  const candidates = key === 'width' ? [info.width, info.videoWidth] : [info.height, info.videoHeight]
  for (const value of candidates) {
    if (typeof value === 'number' && Number.isFinite(value) && value > 0) return Math.round(value)
  }
  return fallback
}

export default function H265WebPlayer({
  url,
  muted = true,
  controls = false,
  onFirstFrame,
  onError,
  onStatusChange,
}: H265WebPlayerProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const playerRef = useRef<H265webPlayerInstance | null>(null)
  const generationRef = useRef(0)
  const mutedRef = useRef(muted)
  const [playerId] = useState(nextPlayerId)
  const [status, setStatus] = useState<WasmPlayerStatus>('idle')
  const [playing, setPlaying] = useState(false)

  // 回调统一走 ref：父组件内联箭头函数不会导致播放器重建
  const handlersRef = useRef({ onFirstFrame, onError, onStatusChange })
  useEffect(() => {
    handlersRef.current = { onFirstFrame, onError, onStatusChange }
  })

  useEffect(() => {
    mutedRef.current = muted
    playerRef.current?.set_voice?.(muted ? 0 : 1)
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
    let observer: ResizeObserver | null = null
    let resizeObserver: ResizeObserver | null = null
    let instance: H265webPlayerInstance | null = null
    let probe: H265webMediaInfo | null = null

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

    const createAndBuild = (width: number, height: number) => {
      if (disposed || isStale()) return
      const factory = window.H265webjsPlayer
      if (typeof factory !== 'function') {
        throw new Error('h265web.js 未加载：window.H265webjsPlayer 不存在')
      }

      const player = factory()
      instance = player
      playerRef.current = player

      player.video_probe_callback = (mediaInfo) => {
        probe = mediaInfo
      }

      player.on_ready_show_done_callback = () => {
        if (disposed || isStale()) return
        try {
          player.play()
        } catch {
          /* 自动播放被拦截：保留画面，等待用户点击 play */
        }
        if (mutedRef.current) player.set_voice?.(0)
        setPlaying(true)
        reportStatus('playing')
        handlersRef.current.onFirstFrame?.({
          width: readDimension(probe, 'width', width),
          height: readDimension(probe, 'height', height),
        })
      }

      player.on_error_callback = (error) => {
        reportError(error)
      }

      player.on_play_finished = () => {
        if (!disposed && !isStale()) setPlaying(false)
      }

      const built = player.build({
        player_id: playerId,
        base_url: h265WebBaseUrl(),
        wasm_js_uri: 'h265web_wasm.js',
        wasm_wasm_uri: 'h265web_wasm.wasm',
        ext_src_js_uri: 'extjs.js',
        ext_wasm_js_uri: 'extwasm.js',
        width,
        height,
        color: '#000000',
        auto_play: false,
        ignore_audio: mutedRef.current,
      })
      if (!built) {
        throw new Error('h265web build 失败')
      }

      // 布局变化后同步画布尺寸（页签切换、窗口缩放）
      const container = containerRef.current
      if (container && typeof ResizeObserver !== 'undefined') {
        resizeObserver = new ResizeObserver(() => {
          if (disposed || isStale()) return
          const next = measure(container)
          if (next.width >= 2 && next.height >= 2) player.resize?.(next.width, next.height)
        })
        resizeObserver.observe(container)
      }

      player.load_media(url)
    }

    const boot = async () => {
      reportStatus('connecting')
      try {
        await loadH265Web()
        if (disposed || isStale()) return

        const container = containerRef.current
        if (!container) return

        const size = measure(container)
        if (size.width >= 2 && size.height >= 2) {
          createAndBuild(size.width, size.height)
          return
        }

        // 容器隐藏 / 尺寸为 0：等它可见后再 build（h265web 对 0 尺寸会直接失败）
        if (typeof ResizeObserver === 'undefined') {
          createAndBuild(Math.max(size.width, 640), Math.max(size.height, 360))
          return
        }
        observer = new ResizeObserver(() => {
          const next = measure(container)
          if (next.width < 2 || next.height < 2) return
          observer?.disconnect()
          observer = null
          try {
            createAndBuild(next.width, next.height)
          } catch (error) {
            reportError(error)
          }
        })
        observer.observe(container)
      } catch (error) {
        reportError(error)
      }
    }

    void boot()

    return () => {
      disposed = true
      // 让所有在途回调立即失效（StrictMode 旧实例 / 切地址）
      generationRef.current = generation + 1
      observer?.disconnect()
      observer = null
      resizeObserver?.disconnect()
      resizeObserver = null
      const player = instance
      instance = null
      playerRef.current = null
      if (player) {
        try {
          player.release()
        } catch {
          /* 已释放 */
        }
      }
    }
  }, [url, playerId])

  const handleToggle = useCallback(() => {
    const player = playerRef.current
    if (!player) return
    if (playing) {
      player.pause()
      setPlaying(false)
    } else {
      player.play()
      setPlaying(true)
    }
  }, [playing])

  return (
    <div
      ref={containerRef}
      className="e2e-h265web-player"
      data-status={status}
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        background: '#000',
        overflow: 'hidden',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      <div id={playerId} style={{ width: '100%', height: '100%' }} />

      {controls && status === 'playing' ? (
        <Button
          size="small"
          type="text"
          aria-label={playing ? '暂停' : '播放'}
          icon={playing ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
          onClick={handleToggle}
          style={{ position: 'absolute', left: 8, bottom: 8, color: '#fff' }}
        />
      ) : null}
    </div>
  )
}
