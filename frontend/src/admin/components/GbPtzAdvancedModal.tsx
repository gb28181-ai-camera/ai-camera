import { useEffect, useMemo, useState } from 'react'
import {
  Alert, Button, Divider, InputNumber, Modal, Select, Space, Switch, Typography, message,
} from 'antd'
import axios from 'axios'
import { GbPtzController } from '../api/generated/GbPtzController'
import type { PtzPresetOption } from './PtzControl'

/**
 * 云台高级操作（国标 PTZCmd 0x84–0x8A）：
 * - 巡航点：加入（0x84）/ 设置速度与停留时间（0x86 / 0x87）/ 删除（0x85）；
 * - 扫描边界：把当前位置设为左 / 右边界（0x89）与扫描速度（0x8A）。
 *
 * 这些指令全部是**带外 MESSAGE**，只有在单进程（embedded）或独立 sipsvc 可用时下发；
 * 组件本身不判断版本，权限由调用方按 `gb:ptz:cruise-point:*` / `gb:ptz:scan-bounds` 传入。
 */
export interface GbPtzAdvancedModalProps {
  open: boolean
  /** 通道主键；未选通道时面板禁用 */
  channelId?: number
  /** 平台侧预置位（用于选择预置位编号与展示名称） */
  presets?: PtzPresetOption[]
  /** 巡航组号默认值（与云台面板的组号保持一致） */
  defaultGroupID?: number
  /** gb:ptz:cruise-point:create */
  canCreatePoint: boolean
  /** gb:ptz:cruise-point:update */
  canUpdatePoint: boolean
  /** gb:ptz:cruise-point:delete */
  canDeletePoint: boolean
  /** gb:ptz:scan-bounds */
  canScanBounds: boolean
  onClose: () => void
}

/** 请求失败兜底提示（HTTP 错误已由 request 拦截器提示，避免重复） */
function notifyError(err: unknown, fallback: string) {
  if (axios.isAxiosError(err) && err.response) return
  message.error(fallback)
}

/** 组号 / 预置位号 / 速度 / 停留时间的后端取值范围 */
const ID_MIN = 1
const ID_MAX = 255
const SPEED_MIN = 1
const SPEED_MAX = 4095

export default function GbPtzAdvancedModal({
  open,
  channelId,
  presets,
  defaultGroupID = 1,
  canCreatePoint,
  canUpdatePoint,
  canDeletePoint,
  canScanBounds,
  onClose,
}: GbPtzAdvancedModalProps) {
  // ── 巡航点 ──
  const [groupID, setGroupID] = useState(defaultGroupID)
  const [presetID, setPresetID] = useState<number | undefined>(undefined)
  const [speed, setSpeed] = useState<number | null>(null)
  const [stayTime, setStayTime] = useState<number | null>(null)

  // ── 扫描边界 ──
  const [scanGroupID, setScanGroupID] = useState(defaultGroupID)
  const [scanSpeed, setScanSpeed] = useState<number | null>(null)
  const [setLeft, setSetLeft] = useState(false)
  const [setRight, setSetRight] = useState(false)

  const [submitting, setSubmitting] = useState<string>('')

  /** 预置位下拉 = 1..8 ∪ 平台侧列表（与云台面板一致） */
  const presetOptions = useMemo(() => {
    const merged = new Map<number, string | undefined>()
    for (let index = 1; index <= 8; index += 1) merged.set(index, undefined)
    for (const item of presets ?? []) {
      const id = Number(item.preset_id)
      if (!Number.isFinite(id) || id < ID_MIN) continue
      merged.set(id, item.name?.trim() || undefined)
    }
    return Array.from(merged.entries())
      .sort((a, b) => a[0] - b[0])
      .map(([id, name]) => ({ value: id, label: name ? `#${id} ${name}` : `#${id}` }))
  }, [presets])

  // 打开时回到调用方的默认组号，避免上次操作残留；预置位默认取第一个
  useEffect(() => {
    if (!open) return
    setGroupID(defaultGroupID)
    setScanGroupID(defaultGroupID)
    setPresetID(presetOptions[0]?.value)
    setSpeed(null)
    setStayTime(null)
    setScanSpeed(null)
    setSetLeft(false)
    setSetRight(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, defaultGroupID])

  const disabled = !channelId

  const run = async (key: string, action: () => Promise<void>, successText: string) => {
    if (!channelId) {
      message.warning('请先选择通道')
      return
    }
    setSubmitting(key)
    try {
      await action()
      message.success(successText)
    } catch (err) {
      notifyError(err, '指令下发失败')
    } finally {
      setSubmitting('')
    }
  }

  const handleAddPoint = () => {
    if (!presetID) {
      message.warning('请选择预置位')
      return
    }
    void run(
      'add',
      async () => {
        await GbPtzController.addGbPtzCruisePoint(channelId as number, {
          group_id: groupID,
          preset_id: presetID,
        })
      },
      `已把预置位 ${presetID} 加入巡航组 ${groupID}`,
    )
  }

  const handleSetPoint = () => {
    if (!speed && !stayTime) {
      message.warning('巡航速度与停留时间至少填写一项')
      return
    }
    void run(
      'set',
      async () => {
        await GbPtzController.setGbPtzCruisePoint(channelId as number, {
          group_id: groupID,
          ...(speed ? { speed } : {}),
          ...(stayTime ? { stay_time: stayTime } : {}),
        })
      },
      '巡航点参数已下发',
    )
  }

  const handleDeletePoint = () => {
    void run(
      'delete',
      async () => {
        // presetID 为空时后端按 0 处理：删除整条巡航
        await GbPtzController.deleteGbPtzCruisePoint(channelId as number, groupID, presetID)
      },
      presetID ? `已删除巡航组 ${groupID} 的预置位 ${presetID}` : `已删除巡航组 ${groupID} 的整条巡航`,
    )
  }

  const handleScanBounds = () => {
    if (!setLeft && !setRight && !scanSpeed) {
      message.warning('左边界 / 右边界 / 扫描速度至少填写一项')
      return
    }
    void run(
      'scan',
      async () => {
        await GbPtzController.setGbPtzScanBounds(channelId as number, {
          group_id: scanGroupID,
          set_left: setLeft || undefined,
          set_right: setRight || undefined,
          speed: scanSpeed ?? undefined,
        })
      },
      '扫描边界 / 速度已下发',
    )
  }

  const anyPermission = canCreatePoint || canUpdatePoint || canDeletePoint || canScanBounds

  return (
    <Modal
      className="e2e-gb-ptz-advanced-modal"
      title="云台高级操作"
      open={open}
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width={640}
      destroyOnHidden
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="巡航点与扫描边界通过带外 MESSAGE 下发；设备需在线且 SIP 信令服务可用。"
        />

        {anyPermission ? null : (
          <Alert type="warning" showIcon message="当前账号未授予巡航点 / 扫描边界操作权限。" />
        )}

        <Divider orientation="left" style={{ margin: '0' }}>巡航点</Divider>
        <Space wrap size={[12, 8]}>
          <Space size={6}>
            <Typography.Text type="secondary">巡航组号</Typography.Text>
            <InputNumber
              className="e2e-gb-ptz-cruise-point-group"
              min={ID_MIN}
              max={ID_MAX}
              precision={0}
              value={groupID}
              disabled={disabled}
              style={{ width: 90 }}
              onChange={(value) => setGroupID(typeof value === 'number' ? value : ID_MIN)}
            />
          </Space>
          <Space size={6}>
            <Typography.Text type="secondary">预置位</Typography.Text>
            <Select
              className="e2e-gb-ptz-cruise-point-preset"
              style={{ width: 180 }}
              value={presetID}
              options={presetOptions}
              disabled={disabled}
              allowClear
              placeholder="选择预置位（删除时可留空）"
              onChange={(value?: number) => setPresetID(value)}
            />
          </Space>
          {canCreatePoint ? (
            <Button
              type="primary"
              size="small"
              className="e2e-gb-ptz-cruise-point-add"
              disabled={disabled}
              loading={submitting === 'add'}
              onClick={handleAddPoint}
            >
              加入巡航点
            </Button>
          ) : null}
        </Space>

        <Space wrap size={[12, 8]}>
          <Space size={6}>
            <Typography.Text type="secondary">速度（1–4095）</Typography.Text>
            <InputNumber
              className="e2e-gb-ptz-cruise-point-speed"
              min={SPEED_MIN}
              max={SPEED_MAX}
              precision={0}
              value={speed}
              disabled={disabled || !canUpdatePoint}
              style={{ width: 110 }}
              onChange={(value) => setSpeed(typeof value === 'number' ? value : null)}
            />
          </Space>
          <Space size={6}>
            <Typography.Text type="secondary">停留秒数（1–4095）</Typography.Text>
            <InputNumber
              className="e2e-gb-ptz-cruise-point-stay"
              min={SPEED_MIN}
              max={SPEED_MAX}
              precision={0}
              value={stayTime}
              disabled={disabled || !canUpdatePoint}
              style={{ width: 110 }}
              onChange={(value) => setStayTime(typeof value === 'number' ? value : null)}
            />
          </Space>
          {canUpdatePoint ? (
            <Button
              size="small"
              className="e2e-gb-ptz-cruise-point-set"
              disabled={disabled}
              loading={submitting === 'set'}
              onClick={handleSetPoint}
            >
              设置速度/停留
            </Button>
          ) : null}
          {canDeletePoint ? (
            <Button
              size="small"
              danger
              className="e2e-gb-ptz-cruise-point-delete"
              disabled={disabled}
              loading={submitting === 'delete'}
              onClick={handleDeletePoint}
            >
              删除巡航点
            </Button>
          ) : null}
        </Space>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          删除时预置位留空表示删除整条巡航（PTZCmd 0x85 数据 2=0）。
        </Typography.Text>

        <Divider orientation="left" style={{ margin: '0' }}>扫描边界</Divider>
        <Space wrap size={[12, 8]}>
          <Space size={6}>
            <Typography.Text type="secondary">扫描组号</Typography.Text>
            <InputNumber
              className="e2e-gb-ptz-scan-group"
              min={ID_MIN}
              max={ID_MAX}
              precision={0}
              value={scanGroupID}
              disabled={disabled || !canScanBounds}
              style={{ width: 90 }}
              onChange={(value) => setScanGroupID(typeof value === 'number' ? value : ID_MIN)}
            />
          </Space>
          <Space size={6}>
            <Typography.Text type="secondary">速度（1–4095）</Typography.Text>
            <InputNumber
              className="e2e-gb-ptz-scan-speed"
              min={SPEED_MIN}
              max={SPEED_MAX}
              precision={0}
              value={scanSpeed}
              disabled={disabled || !canScanBounds}
              style={{ width: 110 }}
              onChange={(value) => setScanSpeed(typeof value === 'number' ? value : null)}
            />
          </Space>
          <Space size={6}>
            <Switch
              className="e2e-gb-ptz-scan-left"
              checked={setLeft}
              disabled={disabled || !canScanBounds}
              onChange={setSetLeft}
            />
            <Typography.Text>当前位置设为左边界</Typography.Text>
          </Space>
          <Space size={6}>
            <Switch
              className="e2e-gb-ptz-scan-right"
              checked={setRight}
              disabled={disabled || !canScanBounds}
              onChange={setSetRight}
            />
            <Typography.Text>当前位置设为右边界</Typography.Text>
          </Space>
          {canScanBounds ? (
            <Button
              type="primary"
              size="small"
              className="e2e-gb-ptz-scan-bounds"
              disabled={disabled}
              loading={submitting === 'scan'}
              onClick={handleScanBounds}
            >
              下发扫描边界
            </Button>
          ) : null}
        </Space>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          国标扫描边界不携带角度：请先用方向键把云台转到目标位置，再设置左 / 右边界；之后到云台面板点「开始扫描」。
        </Typography.Text>
      </Space>
    </Modal>
  )
}
