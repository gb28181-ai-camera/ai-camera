import type { ReactNode } from 'react'

export interface PageContainerProps {
  /** 页面标题 */
  title: string
  /** 副标题 / 说明 */
  subTitle?: string
  /** 右上角操作区 */
  extra?: ReactNode
  children?: ReactNode
}

/**
 * 页面容器：统一的标题区 + 内容区。
 * 业务页面最外层使用，保证标题、操作按钮位置一致。
 */
export default function PageContainer({ title, subTitle, extra, children }: PageContainerProps) {
  return (
    <div className="page-container">
      <div className="page-container-header">
        <div className="page-container-heading">
          <span className="page-container-title">{title}</span>
          {subTitle ? <span className="page-container-subtitle">{subTitle}</span> : null}
        </div>
        {extra ? <div className="page-container-extra">{extra}</div> : null}
      </div>
      <div className="page-container-body">{children}</div>
    </div>
  )
}
