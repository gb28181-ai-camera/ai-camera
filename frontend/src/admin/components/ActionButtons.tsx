import type { ReactNode } from 'react'

export interface ActionButtonsProps {
  children?: ReactNode
  /** 最大宽度，默认 340px */
  maxWidth?: number
}

/**
 * 表格「操作列」按钮容器：自动换行，避免多个操作按钮撑破列宽。
 *
 * @example
 * <ActionButtons>
 *   <Button type="link" size="small">编辑</Button>
 *   <Button type="link" size="small" danger>删除</Button>
 * </ActionButtons>
 */
export default function ActionButtons({ children, maxWidth = 340 }: ActionButtonsProps) {
  return (
    <div
      className="action-buttons"
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        gap: 4,
        maxWidth,
      }}
    >
      {children}
    </div>
  )
}
