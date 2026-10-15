import type { ConsoleMessage, Page } from '@playwright/test'

/** 收集到的单条前端异常 */
export interface ConsoleIssue {
  /** console → `page.on('console')` 的 error 级别；pageerror → 未捕获异常 */
  source: 'console' | 'pageerror'
  text: string
}

/**
 * 可忽略的噪音：开发模式 / 第三方库 / 资源缺失，均与业务缺陷无关。
 * 注意不要在这里堆砌宽泛正则，否则会掩盖真实的页面报错。
 */
const IGNORED_PATTERNS: RegExp[] = [
  /favicon/i,
  /ResizeObserver loop/i,
  /Download the React DevTools/i,
  /React DevTools/i,
  /\[vite\]/i,
  /net::ERR_ABORTED/i,
  /net::ERR_CONNECTION_REFUSED/i,
  // 大屏/地图等页面引用的外部 CDN 资源在离线环境不可达
  /Failed to load resource.*(tile|amap|baidu|openstreetmap)/i,
  // antd 组件用法告警：antd 以 console.error 输出，属于「用法提示」而非运行时异常。
  // 已知：`<Spin tip="..." />` 未包裹内容（Dashboard / App.PageLoading）。
  // 这些告警会在测试报告中单列，但不作为「页面 JS 报错」处理。
  /^Warning: \[antd: /,
  // 测试数据引用的占位上传资源不存在（/uploads/e2e-*.png|mp3），属造数副产物
  /Failed to load resource.*404.*\/uploads\//i,
]

/** 判断一段控制台文本是否属于可忽略噪音 */
export function isIgnoredNoise(text: string): boolean {
  return IGNORED_PATTERNS.some((re) => re.test(text))
}

/** 控制台采集器 */
export interface ConsoleCollector {
  /** 已采集的问题（实时数组，可直接断言） */
  readonly issues: ConsoleIssue[]
  /** 仅文本，便于 `expect(errors).toEqual([])` 时输出可读 */
  texts(): string[]
  /** 清空已采集内容（用于「进入页面后再开始采集」的场景） */
  clear(): void
  /** 停止监听并移除监听器 */
  dispose(): void
}

/**
 * 挂载 console / pageerror 采集器。
 *
 * ```ts
 * const console = collectConsole(page)
 * await gotoAdmin(page, '/dashboard')
 * expect(console.texts()).toEqual([])
 * ```
 */
export function collectConsole(page: Page): ConsoleCollector {
  const issues: ConsoleIssue[] = []

  const onConsole = (msg: ConsoleMessage) => {
    if (msg.type() !== 'error') return
    const text = msg.text()
    // 「Failed to load resource」类消息文本不含 URL，需把 location().url 拼进来一起判断
    const url = msg.location()?.url ?? ''
    const full = url ? `${text} @ ${url}` : text
    if (isIgnoredNoise(full)) return
    issues.push({ source: 'console', text: full })
  }

  const onPageError = (error: Error) => {
    const text = error?.message ?? String(error)
    if (isIgnoredNoise(text)) return
    issues.push({ source: 'pageerror', text })
  }

  page.on('console', onConsole)
  page.on('pageerror', onPageError)

  return {
    issues,
    texts: () => issues.map((item) => `[${item.source}] ${item.text}`),
    clear: () => {
      issues.length = 0
    },
    dispose: () => {
      page.off('console', onConsole)
      page.off('pageerror', onPageError)
    },
  }
}
