/**
 * h265web.js 动态加载器。
 *
 * 为什么动态加载：
 * - 运行时约 8.4MB（js + wasm），不能在首屏阻塞管理后台；
 * - 只有切到 H265web 页签或 codec=H265 时才需要它。
 *
 * ESLint / TS 无需 npm 依赖：脚本由 `<script>` 注入，全局入口是 `window.H265webjsPlayer`。
 * 加载器保证：
 * 1. **单飞**：并发调用只注入一个 `<script>`，共享同一个 Promise；
 * 2. **失败可重试**：加载失败时移除脚本并清空缓存，下一次调用重新注入；
 * 3. **校验全局**：`load` 后必须存在 `window.H265webjsPlayer`，否则视为失败。
 */
import { resolvePlayerStaticUrl } from './playerEngines'

const SCRIPT_RELATIVE_PATH = 'static/js/h265web/h265web.js'
const SCRIPT_MARKER = 'data-h265web-loader'

let loadPromise: Promise<void> | null = null

/** 判断 h265web 全局入口是否可用 */
export function isH265WebAvailable(): boolean {
  return typeof window !== 'undefined' && typeof window.H265webjsPlayer === 'function'
}

/** h265web 静态资源目录（绝对 URL，供 build(config).base_url 使用） */
export function h265WebBaseUrl(): string {
  const relative = resolvePlayerStaticUrl('static/js/h265web/')
  if (typeof window === 'undefined' || !window.location?.href) return relative
  try {
    return new URL(relative, window.location.href).href
  } catch {
    return relative
  }
}

function toError(value: unknown): Error {
  if (value instanceof Error) return value
  if (typeof value === 'string' && value) return new Error(value)
  return new Error('h265web.js 加载失败')
}

/**
 * 加载 h265web.js。
 * - 已加载：立即 resolve；
 * - 加载中：复用同一个 Promise；
 * - 失败：抛错并清空缓存，允许下一次重试。
 */
export function loadH265Web(): Promise<void> {
  if (isH265WebAvailable()) return Promise.resolve()
  if (loadPromise) return loadPromise

  const promise = new Promise<void>((resolve, reject) => {
    if (typeof document === 'undefined') {
      reject(new Error('当前环境没有 document，无法加载 h265web.js'))
      return
    }

    const script = document.createElement('script')
    script.src = resolvePlayerStaticUrl(SCRIPT_RELATIVE_PATH)
    script.async = true
    script.setAttribute(SCRIPT_MARKER, 'true')

    const cleanupListeners = () => {
      script.removeEventListener('load', handleLoad)
      script.removeEventListener('error', handleError)
    }

    function handleLoad() {
      cleanupListeners()
      if (isH265WebAvailable()) {
        resolve()
        return
      }
      // 脚本返回 200 但不是预期产物（如网关返回 HTML）——移除后允许重试
      script.remove()
      reject(new Error('h265web.js 已加载，但未注册 window.H265webjsPlayer'))
    }

    function handleError() {
      cleanupListeners()
      script.remove()
      reject(new Error(`h265web.js 加载失败：${script.src}`))
    }

    script.addEventListener('load', handleLoad)
    script.addEventListener('error', handleError)
    document.head.appendChild(script)
  })

  // 失败时清空缓存，允许下一次重试；guarded 本身作为共享 Promise 返回，
  // 保证并发调用拿到的是同一个对象。
  const guarded = promise.catch((error: unknown) => {
    if (loadPromise === guarded) loadPromise = null
    throw toError(error)
  })
  loadPromise = guarded
  return guarded
}

/** 仅供单测重置模块级单飞状态 */
export function resetH265WebLoaderForTest(): void {
  loadPromise = null
}
