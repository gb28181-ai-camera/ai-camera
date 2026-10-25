/**
 * SSE（Server-Sent Events）客户端工具。
 *
 * 后端的 `/web/gb/logs/stream` 与 `/web/gb/alarms/stream` 都挂在 JWT 鉴权组上，
 * 而浏览器原生 `EventSource` 无法自定义 `Authorization` 头。因此统一用
 * `fetch` + `ReadableStream` 手工解析 SSE 帧，并在请求头里带 Bearer token。
 *
 * 只做「传输 + 解析」，重连策略（指数退避等）由调用方决定，便于不同页面
 * 分别控制暂停/继续与卸载 abort。
 */

/** 一条 SSE 事件 */
export interface SseMessage {
  /** 事件名；缺省为 `message` */
  event: string
  /** data 字段（多行 data 按规范用 \n 连接） */
  data: string
}

/** 非 2xx 的 SSE 响应错误（带 HTTP 状态码，便于区分 401/409 等） */
export class SseHttpError extends Error {
  readonly status: number

  constructor(status: number, message?: string) {
    super(message || `SSE 连接失败（HTTP ${status}）`)
    this.name = 'SseHttpError'
    this.status = status
  }
}

/** 拼接 SSE 地址：路径来自 generated Controller 的 `*Path` 常量。 */
export function buildSseUrl(path: string, params?: Record<string, string | undefined>): string {
  const base = import.meta.env.VITE_API_BASE || ''
  const search = new URLSearchParams()
  Object.entries(params ?? {}).forEach(([key, value]) => {
    if (value !== undefined && value !== '') search.set(key, value)
  })
  const query = search.toString()
  return `${base}${path}${query ? `?${query}` : ''}`
}

/** 解析一个 SSE 事件块（不含结尾空行） */
function parseChunk(chunk: string, onMessage: (message: SseMessage) => void): void {
  const lines = chunk.split('\n')
  let event = 'message'
  const dataLines: string[] = []

  for (const line of lines) {
    if (!line || line.startsWith(':')) continue
    const colon = line.indexOf(':')
    const field = colon === -1 ? line : line.slice(0, colon)
    let value = colon === -1 ? '' : line.slice(colon + 1)
    if (value.startsWith(' ')) value = value.slice(1)

    if (field === 'event') {
      event = value
    } else if (field === 'data') {
      dataLines.push(value)
    }
  }

  if (dataLines.length === 0) return
  onMessage({ event, data: dataLines.join('\n') })
}

/**
 * 建立 SSE 连接并持续读取，直到流结束 / abort / 出错。
 *
 * 调用方需要自己处理重连；abort 后本函数会静默结束（不抛错）。
 */
export async function streamSse(
  url: string,
  signal: AbortSignal,
  onMessage: (message: SseMessage) => void,
): Promise<void> {
  const token = localStorage.getItem('token')
  const headers: Record<string, string> = { Accept: 'text/event-stream' }
  if (token) headers.Authorization = `Bearer ${token}`

  const response = await fetch(url, { method: 'GET', headers, signal, cache: 'no-store' })
  if (!response.ok) {
    // 401/403 等由调用方决定提示；这里只抛出状态，不触发全局跳转
    throw new SseHttpError(response.status)
  }
  if (!response.body) {
    throw new SseHttpError(response.status, '浏览器不支持流式响应')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  try {
    for (;;) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')

      let boundary = buffer.indexOf('\n\n')
      while (boundary >= 0) {
        const chunk = buffer.slice(0, boundary)
        buffer = buffer.slice(boundary + 2)
        parseChunk(chunk, onMessage)
        boundary = buffer.indexOf('\n\n')
      }
    }

    // 服务端正常结束：处理没有以空行结尾的残留帧
    buffer += decoder.decode().replace(/\r\n/g, '\n')
    if (buffer.trim()) parseChunk(buffer, onMessage)
  } finally {
    try {
      reader.releaseLock()
    } catch {
      /* 已被取消读取，忽略 */
    }
  }
}

/** 可在 abort 时立即结束的 sleep，用于断线退避等待。 */
export function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal?.aborted) {
      resolve()
      return
    }
    const timer = window.setTimeout(() => {
      signal?.removeEventListener('abort', handleAbort)
      resolve()
    }, ms)
    function handleAbort() {
      window.clearTimeout(timer)
      resolve()
    }
    signal?.addEventListener('abort', handleAbort, { once: true })
  })
}

/**
 * 指数退避：2s → 4s → 8s → … 上限 30s。
 * attempt 从 1 开始（第一次失败后等待 2s）。
 */
export function backoffDelay(attempt: number, baseMs = 2000, maxMs = 30000): number {
  return Math.min(maxMs, baseMs * 2 ** Math.max(0, attempt - 1))
}
