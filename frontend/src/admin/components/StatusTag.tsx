import { Tag } from 'antd'

export type StatusTagType = 'risk' | 'alarm' | 'device' | 'inspect' | 'upgrade' | 'iotcard'

export interface StatusTagProps {
  /** 后端状态值（字符串或数字） */
  status?: string | number | null
  /** 业务域，决定文案与配色映射 */
  type: StatusTagType
  className?: string
}

interface StatusMeta {
  text: string
  color: string
}

type StatusMap = Record<string, StatusMeta>

/** 各业务域的状态 → 文案 / 颜色映射 */
const STATUS_MAP: Record<StatusTagType, StatusMap> = {
  // 风险：待处理 / 已处理 / 已确认准确 / 误报 / 已忽略
  risk: {
    pending: { text: '待处理', color: 'orange' },
    unhandled: { text: '待处理', color: 'orange' },
    handled: { text: '已处理', color: 'green' },
    processed: { text: '已处理', color: 'green' },
    accurate: { text: '检测准确', color: 'blue' },
    inaccurate: { text: '检测有误', color: 'red' },
    ignored: { text: '已忽略', color: 'default' },
  },
  // 告警：离线 / 电量低 / 流量超限 / 已处理
  alarm: {
    pending: { text: '待处理', color: 'orange' },
    unhandled: { text: '待处理', color: 'orange' },
    handled: { text: '已处理', color: 'green' },
    offline: { text: '设备离线', color: 'red' },
    low_battery: { text: '电量过低', color: 'volcano' },
    low_voltage: { text: '电压异常', color: 'volcano' },
    flow_exceed: { text: '流量超限', color: 'magenta' },
    storage_low: { text: '存储不足', color: 'gold' },
    ignored: { text: '已忽略', color: 'default' },
  },
  // 设备：在线 / 离线 / 未激活
  device: {
    online: { text: '在线', color: 'green' },
    offline: { text: '离线', color: 'red' },
    inactive: { text: '未激活', color: 'default' },
    active: { text: '已激活', color: 'blue' },
    fault: { text: '故障', color: 'volcano' },
  },
  // 巡检任务：待发布 / 已发布 / 已完成 / 已取消
  // v0.1.0 补充 published/finished：巡检任务状态字典（types/business.ts 的
  // INSPECT_STATUS_OPTIONS）使用 pending/published/finished/canceled，
  // 此前缺 published/finished 会导致表格直接显示英文原始值。
  inspect: {
    pending: { text: '待执行', color: 'orange' },
    todo: { text: '待执行', color: 'orange' },
    published: { text: '已发布', color: 'blue' },
    doing: { text: '执行中', color: 'blue' },
    processing: { text: '执行中', color: 'blue' },
    done: { text: '已完成', color: 'green' },
    completed: { text: '已完成', color: 'green' },
    finished: { text: '已完成', color: 'green' },
    canceled: { text: '已取消', color: 'default' },
    timeout: { text: '已超时', color: 'red' },
  },
  // 升级：待升级 / 升级中 / 成功 / 失败
  upgrade: {
    pending: { text: '待升级', color: 'orange' },
    upgrading: { text: '升级中', color: 'blue' },
    running: { text: '升级中', color: 'blue' },
    success: { text: '升级成功', color: 'green' },
    failed: { text: '升级失败', color: 'red' },
    cancel: { text: '已取消', color: 'default' },
  },
  // 物联网卡：正常 / 停用 / 已更换
  iotcard: {
    normal: { text: '正常', color: 'green' },
    active: { text: '正常', color: 'green' },
    disabled: { text: '已停用', color: 'default' },
    stopped: { text: '已停机', color: 'red' },
    replaced: { text: '已更换', color: 'blue' },
    expired: { text: '已过期', color: 'volcano' },
  },
}

/**
 * 状态标签：统一各业务域的状态文案与颜色。
 *
 * @example
 * <StatusTag type="device" status="online" />
 * <StatusTag type="risk" status="pending" />
 */
export default function StatusTag({ status, type, className }: StatusTagProps) {
  if (status === null || status === undefined || status === '') {
    return <span className={className}>-</span>
  }

  const key = String(status).trim().toLowerCase()
  const meta = STATUS_MAP[type]?.[key]

  if (!meta) {
    // 未收录的状态原样展示，避免信息丢失
    return <Tag className={className}>{String(status)}</Tag>
  }

  return (
    <Tag className={className} color={meta.color}>
      {meta.text}
    </Tag>
  )
}
