import { useCallback, useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { Select } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { DeviceController } from '../api/generated/DeviceController'
import type { Device } from '../api/generated/definitions/Device'
import SelectorModal from './SelectorModal'
import type { SelectorFetchParams, SelectorFetchResult } from './SelectorModal'
import StatusTag from './StatusTag'

export interface DeviceSelectProps {
  /* ── 表单控件模式（默认）：渲染为设备下拉框，value 为设备 ID ── */
  /** 选中的设备 ID */
  value?: number | null
  onChange?: (value?: number) => void
  placeholder?: string
  allowClear?: boolean
  disabled?: boolean
  style?: CSSProperties
  className?: string
  /** 按区域过滤（弹窗模式与下拉模式均生效） */
  regionId?: number

  /* ── 弹窗选择模式：传入 open / onOk 时生效，回传完整设备行 ── */
  open?: boolean
  onCancel?: () => void
  onOk?: (devices: Device[]) => void
  mode?: 'single' | 'multiple'
  /** 传入后渲染触发按钮（`.e2e-device-select-btn`） */
  triggerText?: string
  title?: string
}

const columns: ColumnsType<Device> = [
  { title: '设备编号', dataIndex: 'device_code', width: 150, ellipsis: true },
  { title: '设备名称', dataIndex: 'name', width: 160, ellipsis: true },
  {
    title: '所属区域',
    dataIndex: ['region', 'name'],
    width: 140,
    ellipsis: true,
    render: (value: unknown) => (value ? String(value) : '-'),
  },
  {
    title: '状态',
    dataIndex: 'status',
    width: 90,
    render: (value: unknown) => <StatusTag type="device" status={typeof value === 'string' ? value : undefined} />,
  },
  {
    title: '在线时间',
    dataIndex: 'online_dt',
    width: 170,
    ellipsis: true,
    render: (value: unknown) => (value ? String(value) : '-'),
  },
]

/** 设备下拉选项：优先「设备名称（设备编号）」，缺失时退化为编号 */
function toDeviceOptions(items?: { id?: number; name?: string; code?: string }[]) {
  if (!Array.isArray(items)) return []
  return items.map((item) => {
    const name = item.name ?? ''
    const code = item.code ?? ''
    const label = name && code ? `${name}（${code}）` : name || code || String(item.id ?? '')
    return { label, value: Number(item.id ?? 0) }
  })
}

/**
 * 设备选择器，支持两种用法：
 *
 * 1. 表单控件（默认）—— 渲染为设备下拉框，`value` / `onChange` 为设备 ID：
 * ```tsx
 * <DeviceSelect placeholder="请选择设备" value={deviceId} onChange={setDeviceId} />
 * ```
 * 2. 弹窗选择 —— 传入 `open` / `onOk` 时渲染基于 `SelectorModal` 的设备列表弹窗，
 *    回传完整设备行（含区域、状态等关联字段）：
 * ```tsx
 * <DeviceSelect open={open} regionId={regionId} onCancel={close} onOk={(rows) => setDevice(rows[0])} />
 * ```
 *
 * E2E 定位：下拉框为 `.e2e-device-select`，弹窗为 `.e2e-device-select-modal`，
 * 触发按钮为 `.e2e-device-select-btn`。
 */
export default function DeviceSelect({
  value,
  onChange,
  placeholder = '请选择设备',
  allowClear = true,
  disabled,
  style,
  className,
  regionId,
  open,
  onCancel,
  onOk,
  mode = 'single',
  triggerText,
  title = '选择设备',
}: DeviceSelectProps) {
  const modalMode = open !== undefined || onOk !== undefined

  /* ── 弹窗模式 ── */
  const fetchDevices = useCallback(
    async ({ page, page_size, keyword }: SelectorFetchParams): Promise<SelectorFetchResult<Device>> => {
      const res = await DeviceController.pageDevices(page, page_size, keyword, regionId)
      return { items: res.data?.items ?? [], total: Number(res.data?.total ?? 0) }
    },
    [regionId]
  )

  /* ── 下拉模式 ── */
  const [options, setOptions] = useState<{ label: string; value: number }[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (modalMode) return
    let alive = true
    setLoading(true)
    DeviceController.deviceOptions()
      .then((res) => {
        if (alive) setOptions(toDeviceOptions(res?.data))
      })
      .catch(() => {
        if (alive) setOptions([])
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [modalMode])

  const selectNode = useMemo(
    () => (
      <Select
        className={['e2e-device-select', className].filter(Boolean).join(' ')}
        value={value ?? undefined}
        onChange={(next?: number) => onChange?.(next)}
        placeholder={placeholder}
        allowClear={allowClear}
        disabled={disabled}
        loading={loading}
        options={options}
        showSearch
        optionFilterProp="label"
        style={style ?? { minWidth: 180 }}
      />
    ),
    [allowClear, className, disabled, loading, onChange, options, placeholder, style, value]
  )

  if (!modalMode) {
    return selectNode
  }

  return (
    <>
      {triggerText ? selectNode : null}
      <SelectorModal<Device>
        className="e2e-device-select-modal"
        title={title}
        mode={mode}
        open={open}
        onCancel={onCancel ?? (() => {})}
        onOk={onOk ?? (() => {})}
        columns={columns}
        fetchData={fetchDevices}
        rowKey="id"
        searchPlaceholder="请输入设备编号 / 名称"
        triggerText={undefined}
        disabled={disabled}
      />
    </>
  )
}
