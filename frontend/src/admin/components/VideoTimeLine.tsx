import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'

/**
 * 录像时间轴（Canvas）。
 *
 * 用于国标回放页：绘制 RecordInfo 返回的录像片段，支持点击 / 拖拽 seek 与滚轮缩放。
 *
 * ⚠️ 本组件**只负责交互与可视化**，真正的 seek 必须由调用方通过后端接口下发
 * （国标回放是带内 SIP INFO + MANSRTSP，不能用播放器本地 currentTime 控制）。
 *
 * 时间单位统一为「相对本次回放起点的秒数」：
 * `segments: [{ start: 0, end: 300 }]`、`duration: 600`、`current: 42`。
 */

export interface VideoTimeLineSegment {
  start: number
  end: number
}

export interface VideoTimeLineProps {
  /** 录像片段（相对秒） */
  segments: VideoTimeLineSegment[]
  /** 回放总时长（秒） */
  duration: number
  /** 当前播放位置（秒） */
  current: number
  /** 用户点击或拖拽结束后的 seek 回调（秒） */
  onSeek?: (seconds: number) => void
  /** 是否禁用交互 */
  disabled?: boolean
  /** 最大缩放倍数，默认 24 */
  maxZoom?: number
  /** 是否允许滚轮缩放，默认 true */
  enableZoom?: boolean
  /** 高度（px），默认 56 */
  height?: number
  className?: string
  style?: React.CSSProperties
}

const TICK_INTERVALS = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200] as const

function clamp(value: number, min: number, max: number): number {
  if (max < min) return min
  return Math.min(max, Math.max(min, value))
}

/** 秒 → HH:mm:ss */
export function formatTimelineTime(totalSeconds: number): string {
  const sec = Math.max(0, Math.floor(Number.isFinite(totalSeconds) ? totalSeconds : 0))
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(Math.floor(sec / 3600))}:${pad(Math.floor((sec % 3600) / 60))}:${pad(sec % 60)}`
}

/** 按可见跨度选择刻度间隔，使画面里大约 6–10 个刻度 */
function pickTickInterval(span: number): number {
  if (span <= 0) return TICK_INTERVALS[0]
  for (const interval of TICK_INTERVALS) {
    if (span / interval <= 10) return interval
  }
  return TICK_INTERVALS[TICK_INTERVALS.length - 1]
}

function roundRectPath(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius: number,
): void {
  const r = Math.max(0, Math.min(radius, width / 2, height / 2))
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.lineTo(x + width - r, y)
  ctx.quadraticCurveTo(x + width, y, x + width, y + r)
  ctx.lineTo(x + width, y + height - r)
  ctx.quadraticCurveTo(x + width, y + height, x + width - r, y + height)
  ctx.lineTo(x + r, y + height)
  ctx.quadraticCurveTo(x, y + height, x, y + height - r)
  ctx.lineTo(x, y + r)
  ctx.quadraticCurveTo(x, y, x + r, y)
  ctx.closePath()
}

export default function VideoTimeLine({
  segments,
  duration,
  current,
  onSeek,
  disabled = false,
  maxZoom = 24,
  enableZoom = true,
  height = 56,
  className,
  style,
}: VideoTimeLineProps) {
  const wrapperRef = useRef<HTMLDivElement | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const draggingRef = useRef(false)

  const [size, setSize] = useState({ width: 0, height })
  const [zoom, setZoom] = useState(1)
  const [viewStart, setViewStart] = useState(0)
  const [hover, setHover] = useState<number | null>(null)
  const [preview, setPreview] = useState<number | null>(null)

  const safeDuration = Math.max(0, Number.isFinite(duration) ? duration : 0)
  const safeCurrent = clamp(Number.isFinite(current) ? current : 0, 0, safeDuration)
  const safeZoom = clamp(zoom, 1, Math.max(1, maxZoom))
  const span = safeDuration > 0 ? safeDuration / safeZoom : 0
  const viewEnd = viewStart + span

  // 容器尺寸跟随布局变化
  useEffect(() => {
    const element = wrapperRef.current
    if (!element) return undefined
    const update = () => {
      setSize((prev) => {
        const width = element.clientWidth
        const nextHeight = height
        return prev.width === width && prev.height === nextHeight ? prev : { width, height: nextHeight }
      })
    }
    update()
    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', update)
      return () => window.removeEventListener('resize', update)
    }
    const observer = new ResizeObserver(update)
    observer.observe(element)
    return () => observer.disconnect()
  }, [height])

  // 缩放/时长变化后夹紧视窗起点
  useEffect(() => {
    setViewStart((prev) => clamp(prev, 0, Math.max(0, safeDuration - span)))
  }, [safeDuration, span])

  // 播放位置离开视窗时自动居中
  useEffect(() => {
    if (safeDuration <= 0 || span >= safeDuration) return
    setViewStart((prev) => {
      if (safeCurrent >= prev && safeCurrent <= prev + span) return prev
      return clamp(safeCurrent - span / 2, 0, safeDuration - span)
    })
  }, [safeCurrent, span, safeDuration])

  // 滚轮缩放必须用非 passive 的原生监听，React 的 onWheel 无法 preventDefault
  useEffect(() => {
    const element = wrapperRef.current
    if (!element || !enableZoom || disabled) return undefined
    const handleWheel = (event: WheelEvent) => {
      if (safeDuration <= 0) return
      event.preventDefault()
      const rect = element.getBoundingClientRect()
      if (rect.width <= 0) return
      const ratio = clamp((event.clientX - rect.left) / rect.width, 0, 1)
      const anchor = viewStart + ratio * span
      const factor = event.deltaY < 0 ? 1.5 : 1 / 1.5
      const nextZoom = clamp(safeZoom * factor, 1, Math.max(1, maxZoom))
      const nextSpan = safeDuration / nextZoom
      const nextStart = clamp(anchor - ratio * nextSpan, 0, Math.max(0, safeDuration - nextSpan))
      setZoom(nextZoom)
      setViewStart(nextStart)
    }
    element.addEventListener('wheel', handleWheel, { passive: false })
    return () => element.removeEventListener('wheel', handleWheel)
  }, [disabled, enableZoom, maxZoom, safeDuration, safeZoom, span, viewStart])

  const timeFromClientX = useCallback(
    (clientX: number): number => {
      const element = wrapperRef.current
      if (!element || safeDuration <= 0) return 0
      const rect = element.getBoundingClientRect()
      if (rect.width <= 0) return viewStart
      const ratio = clamp((clientX - rect.left) / rect.width, 0, 1)
      return clamp(viewStart + ratio * span, 0, safeDuration)
    },
    [safeDuration, span, viewStart],
  )

  const handlePointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    if (disabled || safeDuration <= 0) return
    draggingRef.current = true
    try {
      event.currentTarget.setPointerCapture(event.pointerId)
    } catch {
      /* 部分环境不支持 pointer capture */
    }
    setPreview(timeFromClientX(event.clientX))
  }

  const handlePointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    if (disabled || safeDuration <= 0) return
    const time = timeFromClientX(event.clientX)
    setHover(time)
    if (draggingRef.current) setPreview(time)
  }

  const finishDrag = (event: React.PointerEvent<HTMLDivElement>) => {
    if (disabled || safeDuration <= 0) return
    if (draggingRef.current) {
      draggingRef.current = false
      const time = timeFromClientX(event.clientX)
      setPreview(null)
      onSeek?.(time)
    }
  }

  const handlePointerLeave = () => {
    setHover(null)
    if (!draggingRef.current) setPreview(null)
  }

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (disabled || safeDuration <= 0) return
    const step = event.shiftKey ? 10 : 1
    if (event.key === 'ArrowLeft') {
      event.preventDefault()
      onSeek?.(clamp(safeCurrent - step, 0, safeDuration))
    } else if (event.key === 'ArrowRight') {
      event.preventDefault()
      onSeek?.(clamp(safeCurrent + step, 0, safeDuration))
    } else if (event.key === 'Home') {
      event.preventDefault()
      onSeek?.(0)
    } else if (event.key === 'End') {
      event.preventDefault()
      onSeek?.(safeDuration)
    }
  }

  const visibleSegments = useMemo(
    () =>
      segments
        .map((segment) => ({
          start: clamp(Math.min(segment.start, segment.end), 0, safeDuration),
          end: clamp(Math.max(segment.start, segment.end), 0, safeDuration),
        }))
        .filter((segment) => segment.end > segment.start && segment.end > viewStart && segment.start < viewEnd),
    [segments, safeDuration, viewStart, viewEnd],
  )

  // 绘制
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const width = size.width
    const canvasHeight = size.height
    if (width <= 0 || canvasHeight <= 0) return
    const dpr = window.devicePixelRatio || 1
    canvas.width = Math.max(1, Math.round(width * dpr))
    canvas.height = Math.max(1, Math.round(canvasHeight * dpr))
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.clearRect(0, 0, width, canvasHeight)

    // 背景
    ctx.fillStyle = '#141414'
    ctx.fillRect(0, 0, width, canvasHeight)

    const paddingX = 8
    const usableWidth = Math.max(1, width - paddingX * 2)
    const trackTop = Math.round(canvasHeight * 0.46)
    const trackHeight = Math.max(8, Math.round(canvasHeight * 0.26))
    const trackBottom = trackTop + trackHeight

    const timeToX = (time: number): number =>
      paddingX + ((time - viewStart) / (span || 1)) * usableWidth

    // 底色槽
    ctx.fillStyle = '#262626'
    ctx.fillRect(paddingX, trackTop, usableWidth, trackHeight)

    // 刻度 + 时间文字
    const interval = pickTickInterval(span)
    const firstTick = Math.ceil(viewStart / interval) * interval
    const tickCount = Math.max(0, Math.floor((viewEnd - firstTick) / interval))
    ctx.font = '11px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif'
    ctx.textAlign = 'center'
    ctx.textBaseline = 'alphabetic'
    for (let index = 0; index <= tickCount; index += 1) {
      const time = firstTick + index * interval
      if (time < 0 || time > safeDuration) continue
      const x = Math.round(timeToX(time)) + 0.5
      ctx.strokeStyle = 'rgba(255,255,255,0.12)'
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(x, trackTop)
      ctx.lineTo(x, trackBottom)
      ctx.stroke()
      ctx.fillStyle = '#8c8c8c'
      ctx.fillText(formatTimelineTime(time), x, trackTop - 4)
    }

    // 录像片段
    ctx.fillStyle = '#1677ff'
    for (const segment of visibleSegments) {
      const start = clamp(segment.start, viewStart, viewEnd)
      const end = clamp(segment.end, viewStart, viewEnd)
      if (end <= start) continue
      const x = timeToX(start)
      const w = Math.max(1, timeToX(end) - x)
      roundRectPath(ctx, x, trackTop + 2, w, trackHeight - 4, 3)
      ctx.fill()
    }

    // 预览线（拖拽中）
    if (preview !== null) {
      const x = Math.round(timeToX(preview)) + 0.5
      ctx.strokeStyle = '#fadb14'
      ctx.lineWidth = 1
      ctx.setLineDash([4, 3])
      ctx.beginPath()
      ctx.moveTo(x, trackTop - 10)
      ctx.lineTo(x, trackBottom + 4)
      ctx.stroke()
      ctx.setLineDash([])
    }

    // 当前播放位置
    const currentX = Math.round(timeToX(safeCurrent)) + 0.5
    if (currentX >= paddingX - 1 && currentX <= width - paddingX + 1) {
      ctx.strokeStyle = '#ff4d4f'
      ctx.lineWidth = 2
      ctx.beginPath()
      ctx.moveTo(currentX, trackTop - 8)
      ctx.lineTo(currentX, trackBottom + 4)
      ctx.stroke()
    }

    // 悬停位置文字
    if (hover !== null) {
      ctx.fillStyle = 'rgba(0,0,0,0.65)'
      const label = formatTimelineTime(hover)
      ctx.font = '11px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif'
      const textWidth = ctx.measureText(label).width
      const labelX = clamp(timeToX(hover) - textWidth / 2 - 4, 0, width - textWidth - 8)
      ctx.fillRect(labelX, canvasHeight - 18, textWidth + 8, 16)
      ctx.fillStyle = '#e6e6e6'
      ctx.textAlign = 'left'
      ctx.fillText(label, labelX + 4, canvasHeight - 6)
      ctx.textAlign = 'center'
    }
  }, [size, safeDuration, safeCurrent, segments, viewStart, viewEnd, span, hover, preview, visibleSegments])

  const displayTime = preview ?? hover ?? safeCurrent

  return (
    <div
      ref={wrapperRef}
      className={['e2e-video-timeline', className].filter(Boolean).join(' ')}
      style={{ position: 'relative', width: '100%', height, userSelect: 'none', ...style }}
      role="slider"
      tabIndex={disabled ? -1 : 0}
      aria-label="录像时间轴"
      aria-valuemin={0}
      aria-valuemax={safeDuration}
      aria-valuenow={safeCurrent}
      aria-disabled={disabled}
      data-duration={safeDuration}
      data-current={safeCurrent}
      data-zoom={safeZoom}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={finishDrag}
      onPointerCancel={finishDrag}
      onPointerLeave={handlePointerLeave}
      onKeyDown={handleKeyDown}
    >
      <canvas
        ref={canvasRef}
        style={{
          display: 'block',
          width: '100%',
          height: '100%',
          cursor: disabled ? 'default' : draggingRef.current ? 'grabbing' : 'pointer',
        }}
      />
      <span
        style={{
          position: 'absolute',
          left: 6,
          top: 2,
          fontSize: 10,
          color: '#bfbfbf',
          pointerEvents: 'none',
        }}
      >
        {formatTimelineTime(displayTime)} / {formatTimelineTime(safeDuration)}
      </span>
      <span
        style={{
          position: 'absolute',
          right: 6,
          top: 2,
          fontSize: 10,
          color: '#8c8c8c',
          pointerEvents: 'none',
        }}
      >
        {safeZoom > 1 ? `${safeZoom.toFixed(1)}x` : ''}
      </span>
    </div>
  )
}
