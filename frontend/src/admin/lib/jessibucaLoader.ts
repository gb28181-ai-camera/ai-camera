/**
 * Jessibuca 动态加载器。
 *
 * Jessibuca 采用 **GPL-3.0** 许可证（详见 `public/static/js/jessibuca/README-LICENSE.md`），
 * 因此本项目管理策略是：
 * - 仅在用户手动切到 Jessibuca 页签时才动态加载，**默认不作为首选引擎**；
 * - 不做代码内联/打包进主 bundle，保持独立静态资源，便于整体替换与合规审计。
 *
 * 与 h265webLoader 一致：单飞、失败可重试、校验 `window.Jessibuca`。
 */
import { resolvePlayerStaticUrl } from './playerEngines'

const SCRIPT_RELATIVE_PATH = 'static/js/jessibuca/jessibuca.js'
const SCRIPT_MARKER = 'data-jessibuca-loader'

let loadPromise: Promise<void> | null = null

/** 判断 Jessibuca 全局入口是否可用 */
export function isJessibucaAvailable(): boolean {
  return typeof window !== 'undefined' && typeof window.Jessibuca === 'function'
}

/** Jessibuca 的 decoder.js 路径（decoder.wasm 必须与它同目录） */
export function jessibucaDecoderUrl(): string {
  return resolvePlayerStaticUrl('static/js/jessibuca/decoder.js')
}

function toError(value: unknown): Error {
  if (value instanceof Error) return value
  if (typeof value === 'string' && value) return new Error(value)
  return new Error('jessibuca.js 加载失败')
}

export function loadJessibuca(): Promise<void> {
  if (isJessibucaAvailable()) return Promise.resolve()
  if (loadPromise) return loadPromise

  const promise = new Promise<void>((resolve, reject) => {
    if (typeof document === 'undefined') {
      reject(new Error('当前环境没有 document，无法加载 jessibuca.js'))
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
      if (isJessibucaAvailable()) {
        resolve()
        return
      }
      script.remove()
      reject(new Error('jessibuca.js 已加载，但未注册 window.Jessibuca'))
    }

    function handleError() {
      cleanupListeners()
      script.remove()
      reject(new Error(`jessibuca.js 加载失败：${script.src}`))
    }

    script.addEventListener('load', handleLoad)
    script.addEventListener('error', handleError)
    document.head.appendChild(script)
  })

  const guarded = promise.catch((error: unknown) => {
    if (loadPromise === guarded) loadPromise = null
    throw toError(error)
  })
  loadPromise = guarded
  return guarded
}

/** 仅供单测重置模块级单飞状态 */
export function resetJessibucaLoaderForTest(): void {
  loadPromise = null
}
