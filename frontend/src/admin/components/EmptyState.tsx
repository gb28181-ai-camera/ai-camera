import { Empty } from 'antd'
import type { ReactNode } from 'react'

export interface EmptyStateProps {
  /** 描述文案 */
  description?: ReactNode
  /** 自定义插图 */
  image?: ReactNode
  /** 额外操作区 */
  extra?: ReactNode
  className?: string
}

/** 空数据占位（antd Empty 包装，统一各页面空态样式） */
export default function EmptyState({ description = '暂无数据', image, extra, className }: EmptyStateProps) {
  return (
    <div className={['empty-state', className].filter(Boolean).join(' ')} style={{ padding: '32px 0', textAlign: 'center' }}>
      <Empty image={image ?? Empty.PRESENTED_IMAGE_SIMPLE} description={description}>
        {extra}
      </Empty>
    </div>
  )
}
