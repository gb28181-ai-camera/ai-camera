import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, Divider, Segmented, Slider, Space, Tooltip } from 'antd'
import {
  ArrowDownOutlined,
  ArrowLeftOutlined,
  ArrowRightOutlined,
  ArrowUpOutlined,
  ColumnWidthOutlined,
  MinusOutlined,
  PauseOutlined,
  PlusOutlined,
  SyncOutlined,
} from '@ant-design/icons'

/**
 * 云台（PTZ）控制面板 —— **纯 UI 组件**。
 *
 * 只向上抛语义化指令，不调任何接口、不拼 8 字节 PTZCmd。
 * GB28181 的 `<Control><PTZCmd>A5 0F 01 …</PTZCmd></Control>` 报文由后端
 * `internal/gb28181/device` 组装（见设计文档 §6.5）：PTZ 走 `MESSAGE`，**不是 INFO**。
 *
 * 交互：方向键与变焦键**按住持续下发、松开下发 stop** —— 这是 GB28181
 * 「云台开始运动 / 停止运动」两个独立指令的语义，点击式单发会导致镜头一直转。
 *
 * @example
 * <PtzControl
 *   onCommand={(direction, speed) => ptz({ direction, speed })}
 *   onZoom={(action, speed) => ptz({ zoom: action, speed })}
 *   onPreset={(action, index) => ptz({ preset: action, index })}
 * />
 */
export type PtzDirection =
  | 'up'
  | 'up-right'
  | 'right'
  | 'down-right'
  | 'down'
  | 'down-left'
  | 'left'
  | 'up-left'
  | 'stop'

export type PtzZoomAction = 'zoom-in' | 'zoom-out' | 'zoom-stop'

export type PtzPresetAction = 'goto' | 'set' | 'delete'

/** 启动 / 停止类指令（巡航、扫描） */
export type PtzRunAction = 'start' | 'stop'

export interface PtzControlProps {
  /** 整块面板禁用（设备离线、通道未选中时使用） */
  disabled?: boolean
  /** 速度档位，取值 1–8，默认 5 */
  defaultSpeed?: number
  /** 是否显示速度滑杆，默认 true */
  showSpeed?: boolean
  /** 八方向 + 停止 */
  onCommand?: (direction: PtzDirection, speed: number) => void
  /** 变焦（按住持续、松开停止） */
  onZoom?: (action: PtzZoomAction, speed: number) => void
  /** 预置位：调用 / 设置 / 删除 */
  onPreset?: (action: PtzPresetAction, presetIndex: number) => void
  /** 预置位数量，默认 8 */
  presetCount?: number
  /** 巡航（按预置位序列自动巡视） */
  onCruise?: (action: PtzRunAction) => void
  /** 扫描（左右自动扫描） */
  onScan?: (action: PtzRunAction) => void
  className?: string
  style?: React.CSSProperties
}

const PTZ_SPEED_MIN = 1
const PTZ_SPEED_MAX = 8
const DEFAULT_PTZ_SPEED = 5
const DEFAULT_PRESET_COUNT = 8

/** 3×3 方向盘，按行主序排列 */
const DIRECTION_BUTTONS: { direction: PtzDirection; title: string; icon: React.ReactNode }[] = [
  { direction: 'up-left', title: '左上', icon: <ArrowUpOutlined rotate={-45} /> },
  { direction: 'up', title: '上', icon: <ArrowUpOutlined /> },
  { direction: 'up-right', title: '右上', icon: <ArrowUpOutlined rotate={45} /> },
  { direction: 'left', title: '左', icon: <ArrowLeftOutlined /> },
  { direction: 'stop', title: '停止', icon: <PauseOutlined /> },
  { direction: 'right', title: '右', icon: <ArrowRightOutlined /> },
  { direction: 'down-left', title: '左下', icon: <ArrowDownOutlined rotate={45} /> },
  { direction: 'down', title: '下', icon: <ArrowDownOutlined /> },
  { direction: 'down-right', title: '右下', icon: <ArrowDownOutlined rotate={-45} /> },
]

const PRESET_ACTIONS: { label: string; value: PtzPresetAction }[] = [
  { label: '调用', value: 'goto' },
  { label: '设置', value: 'set' },
  { label: '删除', value: 'delete' },
]

interface HoldButtonProps {
  title: string
  icon: React.ReactNode
  className?: string
  disabled?: boolean
  style?: React.CSSProperties
  /** 按下 —— 下发「开始」指令 */
  onHold: () => void
  /** 松开 / 移出 / 卸载 —— 下发「停止」指令 */
  onRelease: () => void
}

/**
 * 按住持续下发 / 松开下发停止的按钮。
 *
 * 卸载时若仍处于按住状态会补发一次停止，避免「组件已销毁但镜头还在转」。
 */
function HoldButton({
  title,
  icon,
  className,
  disabled,
  style,
  onHold,
  onRelease,
}: HoldButtonProps) {
  const holdingRef = useRef(false)
  const onReleaseRef = useRef(onRelease)

  useEffect(() => {
    onReleaseRef.current = onRelease
  })

  // 卸载兜底：按住过程中组件被销毁（切分屏 / 关弹窗）时补发停止
  useEffect(
    () => () => {
      if (holdingRef.current) {
        holdingRef.current = false
        onReleaseRef.current()
      }
    },
    [],
  )

  const handleHoldStart = () => {
    if (disabled || holdingRef.current) return
    holdingRef.current = true
    onHold()
  }

  const handleHoldEnd = () => {
    if (!holdingRef.current) return
    holdingRef.current = false
    onRelease()
  }

  return (
    <Tooltip title={title}>
      <Button
        size="small"
        className={className}
        disabled={disabled}
        icon={icon}
        style={style}
        onPointerDown={handleHoldStart}
        onPointerUp={handleHoldEnd}
        onPointerLeave={handleHoldEnd}
        onPointerCancel={handleHoldEnd}
        onContextMenu={(event) => event.preventDefault()}
      />
    </Tooltip>
  )
}

export default function PtzControl({
  disabled = false,
  defaultSpeed = DEFAULT_PTZ_SPEED,
  showSpeed = true,
  onCommand,
  onZoom,
  onPreset,
  presetCount = DEFAULT_PRESET_COUNT,
  onCruise,
  onScan,
  className,
  style,
}: PtzControlProps) {
  const [speed, setSpeed] = useState(defaultSpeed)
  const [presetAction, setPresetAction] = useState<PtzPresetAction>('goto')
  const [cruising, setCruising] = useState(false)
  const [scanning, setScanning] = useState(false)

  const presets = useMemo(
    () => Array.from({ length: Math.max(0, presetCount) }, (_, index) => index + 1),
    [presetCount],
  )

  const handleMoveHold = useCallback(
    (direction: PtzDirection) => {
      onCommand?.(direction, speed)
    },
    [onCommand, speed],
  )

  const handleMoveStop = useCallback(() => {
    onCommand?.('stop', speed)
  }, [onCommand, speed])

  const handleZoomHold = useCallback(
    (action: 'zoom-in' | 'zoom-out') => {
      onZoom?.(action, speed)
    },
    [onZoom, speed],
  )

  const handleZoomStop = useCallback(() => {
    onZoom?.('zoom-stop', speed)
  }, [onZoom, speed])

  const handlePresetClick = useCallback(
    (presetIndex: number) => {
      onPreset?.(presetAction, presetIndex)
    },
    [onPreset, presetAction],
  )

  const handlePresetActionChange = (value: string | number) => {
    if (value === 'goto' || value === 'set' || value === 'delete') setPresetAction(value)
  }

  const handleCruiseToggle = () => {
    if (disabled) return
    const next = !cruising
    setCruising(next)
    onCruise?.(next ? 'start' : 'stop')
  }

  const handleScanToggle = () => {
    if (disabled) return
    const next = !scanning
    setScanning(next)
    onScan?.(next ? 'start' : 'stop')
  }

  return (
    <div
      className={['e2e-ptz-control', className].filter(Boolean).join(' ')}
      style={{ width: 224, ...style }}
    >
      {/* 八方向方向盘 + 中心停止 */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(3, 1fr)',
          gap: 4,
          width: 132,
          margin: '0 auto',
        }}
      >
        {DIRECTION_BUTTONS.map((item) =>
          item.direction === 'stop' ? (
            <Tooltip key={item.direction} title={item.title}>
              <Button
                size="small"
                danger
                className="e2e-ptz-stop"
                disabled={disabled}
                icon={item.icon}
                style={{ width: '100%' }}
                onClick={handleMoveStop}
              />
            </Tooltip>
          ) : (
            <HoldButton
              key={item.direction}
              title={item.title}
              icon={item.icon}
              className={`e2e-ptz-direction-${item.direction}`}
              disabled={disabled}
              style={{ width: '100%' }}
              onHold={() => handleMoveHold(item.direction)}
              onRelease={handleMoveStop}
            />
          ),
        )}
      </div>

      {/* 速度档位 */}
      {showSpeed ? (
        <div style={{ marginTop: 8 }}>
          <div style={{ fontSize: 12, color: '#8c8c8c' }}>速度 {speed}</div>
          <Slider
            className="e2e-ptz-speed"
            min={PTZ_SPEED_MIN}
            max={PTZ_SPEED_MAX}
            value={speed}
            disabled={disabled}
            onChange={(value: number) => setSpeed(value)}
          />
        </div>
      ) : null}

      {/* 变焦 */}
      <Space className="e2e-ptz-zoom" style={{ marginTop: 4 }}>
        <HoldButton
          title="变焦缩小"
          icon={<MinusOutlined />}
          className="e2e-ptz-zoom-out"
          disabled={disabled}
          onHold={() => handleZoomHold('zoom-out')}
          onRelease={handleZoomStop}
        />
        <span style={{ fontSize: 12, color: '#8c8c8c' }}>变焦</span>
        <HoldButton
          title="变焦放大"
          icon={<PlusOutlined />}
          className="e2e-ptz-zoom-in"
          disabled={disabled}
          onHold={() => handleZoomHold('zoom-in')}
          onRelease={handleZoomStop}
        />
      </Space>

      <Divider style={{ margin: '8px 0' }} />

      {/* 预置位：先选动作，再点序号 */}
      <Segmented
        className="e2e-ptz-preset-action"
        size="small"
        block
        value={presetAction}
        options={PRESET_ACTIONS}
        onChange={handlePresetActionChange}
      />
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(4, 1fr)',
          gap: 4,
          marginTop: 6,
        }}
      >
        {presets.map((presetIndex) => (
          <Button
            key={presetIndex}
            size="small"
            className={`e2e-ptz-preset-${presetIndex}`}
            disabled={disabled}
            onClick={() => handlePresetClick(presetIndex)}
          >
            {presetIndex}
          </Button>
        ))}
      </div>

      <Divider style={{ margin: '8px 0' }} />

      {/* 巡航 / 扫描 */}
      <Space>
        <Button
          size="small"
          type={cruising ? 'primary' : 'default'}
          className="e2e-ptz-cruise"
          icon={<SyncOutlined />}
          disabled={disabled}
          onClick={handleCruiseToggle}
        >
          {cruising ? '停止巡航' : '开始巡航'}
        </Button>
        <Button
          size="small"
          type={scanning ? 'primary' : 'default'}
          className="e2e-ptz-scan"
          icon={<ColumnWidthOutlined />}
          disabled={disabled}
          onClick={handleScanToggle}
        >
          {scanning ? '停止扫描' : '开始扫描'}
        </Button>
      </Space>
    </div>
  )
}
