import { render } from '@testing-library/react'
import type { RenderOptions, RenderResult } from '@testing-library/react'
import type { ReactElement, ReactNode } from 'react'
import { App as AntdApp, ConfigProvider } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { MemoryRouter } from 'react-router-dom'

/** 测试用全局 Provider：与 src/admin/App.tsx 的主题 / 语言包保持一致 */
function AllProviders({ children }: { children: ReactNode }) {
  return (
    <ConfigProvider locale={zhCN} componentSize="small" theme={{ token: { fontSize: 12 } }}>
      <AntdApp>
        <MemoryRouter>{children}</MemoryRouter>
      </AntdApp>
    </ConfigProvider>
  )
}

/**
 * 带全局 Provider 的 render。
 *
 * @example
 * const { getByText } = renderWithProviders(<Login />)
 */
export function renderWithProviders(
  ui: ReactElement,
  options?: Omit<RenderOptions, 'wrapper'>
): RenderResult {
  return render(ui, { wrapper: AllProviders, ...options })
}

export * from '@testing-library/react'
export { default as userEvent } from '@testing-library/user-event'
