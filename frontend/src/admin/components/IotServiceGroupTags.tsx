import { Space, Tag, Typography } from 'antd'
import type { IotServiceGroupBrief } from '../api/generated/definitions/IotServiceGroupBrief'

/**
 * 服务组类型 → 展示文案 / 标签颜色。
 *
 * 同一台设备可以通过所属产品同时绑定多个服务组（如 GB28181 + AIChat），
 * 各专用页面按服务组类型筛选时，标签必须全部展示，不能只显示第一个。
 */
const SERVICE_GROUP_META: Record<string, { label: string; color: string }> = {
  gb28181: { label: 'GB28181', color: 'geekblue' },
  webrtc: { label: 'WebRTC', color: 'cyan' },
  aichat: { label: 'AIChat', color: 'purple' },
}

/** 服务组类型取值：优先 type，兼容历史数据只填 code 的情况 */
function groupKey(group: IotServiceGroupBrief): string {
  return String(group.type || group.code || '')
}

/** 服务组展示文案：未知类型回退到名称 / 编码 */
export function serviceGroupLabel(group: IotServiceGroupBrief): string {
  const meta = SERVICE_GROUP_META[groupKey(group)]
  return meta?.label ?? group.name ?? group.code ?? '-'
}

/** 判断设备/产品是否绑定了指定类型的服务组 */
export function hasServiceGroupType(
  groups: IotServiceGroupBrief[] | undefined,
  type: 'gb28181' | 'webrtc' | 'aichat',
): boolean {
  return (groups ?? []).some((group) => groupKey(group) === type)
}

/**
 * 服务组标签列表。
 *
 * 没有绑定服务组时统一显示 `-`，避免表格出现空白单元格。
 */
export default function IotServiceGroupTags({
  groups,
  className,
  emptyText = '-',
}: {
  groups?: IotServiceGroupBrief[]
  className?: string
  emptyText?: string
}) {
  const list = groups ?? []
  if (list.length === 0) {
    return <Typography.Text type="secondary">{emptyText}</Typography.Text>
  }
  return (
    <Space size={[4, 4]} wrap className={className}>
      {list.map((group, index) => {
        const meta = SERVICE_GROUP_META[groupKey(group)]
        return (
          <Tag
            key={group.id ?? group.code ?? `${groupKey(group)}-${index}`}
            color={meta?.color ?? 'default'}
            title={group.name || group.code}
          >
            {serviceGroupLabel(group)}
          </Tag>
        )
      })}
    </Space>
  )
}
