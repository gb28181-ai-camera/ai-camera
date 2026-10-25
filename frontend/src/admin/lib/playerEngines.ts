/**
 * 播放器引擎的路由与地址映射（纯函数，便于单测）。
 *
 * 设计目标与 WVP-GB28181-Pro 对齐（且只多不少）：
 * | 引擎 | 说明 | 默认场景 |
 * |:--|:--|:--|
 * | webrtc | WHEP / ZLM WebRTC，低延迟 | 单通道默认 |
 * | flv | mpegts.js 播放 HTTP-FLV | 视频墙默认 |
 * | jessibuca | Jessibuca（WASM，GPL-3.0，动态加载） | 手动选择 |
 * | h265web | h265web.js（WASM，支持 H.264/H.265） | codec=H265 时默认 |
 * | hls | hls.js（或 Safari 原生 HLS） | 兜底 |
 *
 * ⚠️ 本模块只负责「前端引擎 → 播放地址」的映射，**不发起任何后端请求**。
 * 切换引擎只切前端实现，复用调用方已经拿到的点播地址。
 */

export type PlayerEngine = 'webrtc' | 'flv' | 'jessibuca' | 'h265web' | 'hls'

/** 页签顺序（单通道页按此顺序渲染） */
export const PLAYER_ENGINES: readonly PlayerEngine[] = ['webrtc', 'flv', 'jessibuca', 'h265web', 'hls']

export const PLAYER_ENGINE_LABELS: Record<PlayerEngine, string> = {
  webrtc: 'WebRTC',
  flv: 'HTTP-FLV',
  jessibuca: 'Jessibuca',
  h265web: 'H265web',
  hls: 'HLS',
}

/** 每个页签对应的 E2E class，便于 Playwright 精确定位 */
export const PLAYER_ENGINE_E2E_CLASS: Record<PlayerEngine, string> = {
  webrtc: 'e2e-player-tab-webrtc',
  flv: 'e2e-player-tab-flv',
  jessibuca: 'e2e-player-tab-jessibuca',
  h265web: 'e2e-player-tab-h265web',
  hls: 'e2e-player-tab-hls',
}

export interface PlayerEngineSource {
  /** ZLM 应用名，国标单端口模式恒为 `rtp` */
  app?: string
  /** 流名；国标场景为 SSRC 大写十六进制 */
  stream?: string
  /** ZLM HTTP 地址根，如 `http://192.168.1.227:8080` */
  httpBase?: string
  /** 调用方直接给出的整串地址（兼容 v0.1.0 的 device.stream_url） */
  url?: string
  /** 按引擎覆盖地址（优先级最高），如后端返回的 webrtc/flv/hls/ws_flv */
  urls?: Partial<Record<PlayerEngine, string>>
  /** 视频编码，H264 / H265（大小写不敏感，兼容 HEVC 写法） */
  codec?: string
}

/** 判定是否为 H.265 / HEVC */
export function isH265Codec(codec?: string): boolean {
  if (!codec) return false
  return /h\.?265|hevc/i.test(codec)
}

/**
 * 解析默认引擎：
 * 1. 调用方显式指定优先（视频墙传 flv、AI 监控传 webrtc）；
 * 2. codec=H265 → h265web（H.265 只能由 WASM 解码，浏览器 MSE 普遍不支持）；
 * 3. 其余单通道场景 → webrtc（低延迟主协议）。
 */
export function resolveDefaultEngine(input: {
  defaultEngine?: PlayerEngine
  codec?: string
}): PlayerEngine {
  if (input.defaultEngine) return input.defaultEngine
  if (isH265Codec(input.codec)) return 'h265web'
  return 'webrtc'
}

/**
 * 默认引擎失败时的自动回落链。
 *
 * - webrtc → flv → hls
 * - h265web → hls → flv
 * - flv → hls（视频墙场景一般会关闭 autoFallback）
 *
 * 用户**手动**切到某个页签后不再自动回落（由调用方控制），因此这里只描述默认链路。
 */
export function fallbackChain(engine: PlayerEngine, available?: readonly PlayerEngine[]): PlayerEngine[] {
  const chain: PlayerEngine[] =
    engine === 'webrtc'
      ? ['webrtc', 'flv', 'hls']
      : engine === 'h265web'
        ? ['h265web', 'hls', 'flv']
        : engine === 'flv'
          ? ['flv', 'hls']
          : [engine]
  if (!available) return chain
  const allowed = new Set(available)
  return chain.filter((item) => allowed.has(item))
}

function trimBase(value: string): string {
  return value.replace(/\/+$/, '')
}

/** `http(s)://` → `ws(s)://`；已经是 ws 协议时原样返回 */
export function toWebSocketBase(base: string): string {
  const trimmed = trimBase(base)
  if (trimmed.startsWith('https://')) return `wss://${trimmed.slice('https://'.length)}`
  if (trimmed.startsWith('http://')) return `ws://${trimmed.slice('http://'.length)}`
  return trimmed
}

/**
 * 按引擎拼接 ZLM 播放地址。
 * 前置条件：`httpBase` 与 `stream` 均非空；否则返回空串。
 */
export function buildEngineUrl(engine: PlayerEngine, source: PlayerEngineSource): string {
  const base = source.httpBase?.trim()
  const stream = source.stream?.trim()
  const app = source.app?.trim() || 'rtp'
  if (!base || !stream) return ''

  const appPart = encodeURIComponent(app)
  const streamPart = encodeURIComponent(stream)
  const httpBase = trimBase(base)

  switch (engine) {
    case 'webrtc':
      return `${httpBase}/index/api/webrtc?app=${appPart}&stream=${streamPart}&type=play`
    case 'flv':
      return `${httpBase}/${appPart}/${streamPart}.live.flv`
    case 'jessibuca':
    case 'h265web':
      // WVP 的 jessibuca / h265web 都用 ws_flv。ZLM 的 ws-flv 路径与 http-flv 相同，
      // 只是协议换成 ws(s)。
      return `${toWebSocketBase(httpBase)}/${appPart}/${streamPart}.live.flv`
    case 'hls':
      return `${httpBase}/${appPart}/${streamPart}/hls.m3u8`
    default:
      return ''
  }
}

/**
 * 解析某个引擎最终使用的地址。
 *
 * 优先级：`urls[engine]` > `url`（调用方直给的整串地址）> 按 app/stream/httpBase 拼装。
 * 返回空串表示无地址，调用方应展示占位而不是报错。
 */
export function resolveEngineUrl(engine: PlayerEngine, source: PlayerEngineSource): string {
  const override = source.urls?.[engine]?.trim()
  if (override) return override
  const direct = source.url?.trim()
  if (direct) return direct
  return buildEngineUrl(engine, source)
}

/**
 * 静态资源地址：兼容 Vite `base=/admin/` 与 Electron `base=./`（file://）。
 *
 * - Web：`/admin/` + `static/js/...` → `/admin/static/js/...`
 * - Electron：`./` + `static/js/...` → `./static/js/...`（相对 index.html）
 *
 * 返回值保持「可在 `<script src>` / `new URL(..., location.href)` 中直接使用」。
 */
export function resolvePlayerStaticUrl(relativePath: string): string {
  const base = import.meta.env.BASE_URL || '/'
  const normalizedBase = base.endsWith('/') ? base : `${base}/`
  return `${normalizedBase}${relativePath.replace(/^\/+/, '')}`
}
