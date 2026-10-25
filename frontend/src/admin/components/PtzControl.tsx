import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, Divider, Input, InputNumber, Segmented, Slider, Space, Spin, Tooltip } from 'antd'
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

/** 预置位面板的动作：调用 / 设置 / 删除 / 重命名（重命名仅平台侧，不下发设备） */
type PtzPresetMode = PtzPresetAction | 'rename'

/** 启动 / 停止类指令（巡航、扫描） */
export type PtzRunAction = 'start' | 'stop'

/**
 * 平台侧预置位（来自 `ac_gb_ptz_preset`，后端 `GET /web/gb/ptz/{id}/presets`）。
 *
 * 名称只在平台侧保存，不下发设备；列表为空时组件回退为 1..presetCount 的编号按钮。
 */
export interface PtzPresetOption {
  /** 预置位编号，1–255（缺失的条目会被忽略） */
  preset_id?: number
  /** 平台侧展示名，可为空（设备侧只认编号） */
  name?: string
}

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
  /**
   * 预置位：调用 / 设置 / 删除。
   * `name` 只在 action=set 时有意义 —— 平台侧展示名，必填。
   */
  onPreset?: (action: PtzPresetAction, presetIndex: number, name?: string) => void
  /** 预置位数量，默认 8（与后端返回的列表合并展示） */
  presetCount?: number
  /** 平台侧预置位列表（含名称）；为空时回退为编号按钮 */
  presets?: PtzPresetOption[]
  /** 预置位列表加载中 */
  presetsLoading?: boolean
  /**
   * 重命名预置位（仅改平台侧名称，不下发设备）。
   * 传入时才渲染「重命名」动作；由页面按 `gb:ptz:preset-update` 权限决定是否传入。
   */
  onRenamePreset?: (presetIndex: number, name: string) => void
  /** 巡航（按预置位序列自动巡视），groupId 为巡航组号 1–255 */
  onCruise?: (action: PtzRunAction, groupId: number) => void
  /** 扫描（左右自动扫描），groupId 为扫描组号 1–255 */
  onScan?: (action: PtzRunAction, groupId: number) => void
  /** 雨刷开 / 关；不传时不渲染雨刷按钮（无权限） */
  onWiper?: (on: boolean) => void
  /**
   * 附加区域（例如 GB28181-2022 的精准位置/精准控制/巡航轨迹）。
   *
   * 组件本身保持「纯 UI、不调接口」：2022 面板的数据请求与权限判断放在
   * 页面层，通过该槽位注入，避免 PtzControl 依赖 API 生成代码。
   */
  extra?: React.ReactNode
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
  presets: presetList,
  presetsLoading = false,
  onRenamePreset,
  onCruise,
  onScan,
  onWiper,
  extra,
  className,
  style,
}: PtzControlProps) {
  const [speed, setSpeed] = useState(defaultSpeed)
  const [presetAction, setPresetAction] = useState<PtzPresetMode>('goto')
  /** 设置预置位时填写的平台侧名称（不写死「预置位N」） */
  const [presetName, setPresetName] = useState('')
  /** 巡航 / 扫描组号，默认 1；后端字段 group_id 取值 1–255 */
  const [groupID, setGroupID] = useState(1)
  const [cruising, setCruising] = useState(false)
  const [scanning, setScanning] = useState(false)

  /**
   * 预置位按钮 = 1..presetCount 的编号 ∪ 后端返回的列表。
   * 名称取平台侧持久化值；没有名称时只显示编号，绝不伪造「预置位N」。
   */
  const presetButtons = useMemo(() => {
    const merged = new Map<number, string | undefined>()
    for (let index = 1; index <= Math.max(0, presetCount); index += 1) {
      merged.set(index, undefined)
    }
    for (const item of presetList ?? []) {
      const id = Number(item.preset_id)
      if (!Number.isFinite(id) || id < 1) continue
      merged.set(id, item.name?.trim() || undefined)
    }
    return Array.from(merged.entries())
      .sort((a, b) => a[0] - b[0])
      .map(([presetIndex, name]) => ({ presetIndex, name }))
  }, [presetCount, presetList])

  const hasPresetName = presetButtons.some((item) => Boolean(item.name))

  /** 「重命名」仅在页面传入 onRenamePreset（有权限）时出现 */
  const presetActionOptions = useMemo(
    () =>
      onRenamePreset
        ? [...PRESET_ACTIONS, { label: '重命名', value: 'rename' as const }]
        : PRESET_ACTIONS,
    [onRenamePreset],
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
      if (presetAction === 'rename') {
        onRenamePreset?.(presetIndex, presetName)
        return
      }
      onPreset?.(presetAction, presetIndex, presetAction === 'set' ? presetName : undefined)
    },
    [onPreset, onRenamePreset, presetAction, presetName],
  )

  const handlePresetActionChange = (value: string | number) => {
    if (value === 'goto' || value === 'set' || value === 'delete') {
      setPresetAction(value)
      return
    }
    if (value === 'rename' && onRenamePreset) setPresetAction('rename')
  }

  const handleCruiseToggle = () => {
    if (disabled) return
    const next = !cruising
    setCruising(next)
    onCruise?.(next ? 'start' : 'stop', groupID)
  }

  const handleScanToggle = () => {
    if (disabled) return
    const next = !scanning
    setScanning(next)
    onScan?.(next ? 'start' : 'stop', groupID)
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

      {/* 预置位：先选动作，再点编号；名称来自平台侧列表 */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span style={{ fontSize: 12, color: '#8c8c8c' }}>预置位</span>
        {presetsLoading ? <Spin size="small" /> : null}
      </div>
      <Segmented
        className="e2e-ptz-preset-action"
        size="small"
        block
        value={presetAction}
        options={presetActionOptions}
        onChange={handlePresetActionChange}
      />
      {presetAction === 'set' || presetAction === 'rename' ? (
        <Input
          size="small"
          className="e2e-ptz-preset-name"
          style={{ marginTop: 6 }}
          maxLength={64}
          placeholder={presetAction === 'rename' ? '新名称，如：东门' : '预置位名称，如：大门'}
          value={presetName}
          disabled={disabled}
          onChange={(event) => setPresetName(event.target.value)}
        />
      ) : null}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: `repeat(${hasPresetName ? 2 : 4}, 1fr)`,
          gap: 4,
          marginTop: 6,
        }}
      >
        {presetButtons.map(({ presetIndex, name }) => (
          <Tooltip key={presetIndex} title={name ? `#${presetIndex} ${name}` : `预置位 ${presetIndex}`}>
            <Button
              size="small"
              className={`e2e-ptz-preset-${presetIndex}`}
              disabled={disabled}
              style={{ width: '100%', overflow: 'hidden', textOverflow: 'ellipsis' }}
              onClick={() => handlePresetClick(presetIndex)}
            >
              {name || presetIndex}
            </Button>
          </Tooltip>
        ))}
      </div>

      <Divider style={{ margin: '8px 0' }} />

      {/* 巡航 / 扫描：组号可选（后端 group_id 1–255） */}
      <Space size={6} wrap>
        <span style={{ fontSize: 12, color: '#8c8c8c' }}>组号</span>
        <InputNumber
          size="small"
          className="e2e-ptz-group"
          min={1}
          max={255}
          precision={0}
          value={groupID}
          disabled={disabled}
          style={{ width: 72 }}
          onChange={(value) => {
            setGroupID(typeof value === 'number' && value >= 1 ? value : 1)
          }}
        />
      </Space>
      <Space style={{ marginTop: 6 }} wrap>
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

      {/* 雨刷：仅在账号有 gb:ptz:wiper 权限时由页面注入 onWiper */}
      {onWiper ? (
        <Space className="e2e-ptz-wiper" style={{ marginTop: 6 }} size={6}>
          <span style={{ fontSize: 12, color: '#8c8c8c' }}>雨刷</span>
          <Button
            size="small"
            className="e2e-ptz-wiper-on"
            disabled={disabled}
            onClick={() => onWiper(true)}
          >
            开启
          </Button>
          <Button
            size="small"
            className="e2e-ptz-wiper-off"
            disabled={disabled}
            onClick={() => onWiper(false)}
          >
            关闭
          </Button>
        </Space>
      ) : null}

      {/* 附加区域：由页面注入（例如 GB28181-2022 能力面板） */}
      {extra ? <div className="e2e-ptz-extra">{extra}</div> : null}
    </div>
  )
}
