/**
 * 第三方播放器全局变量声明。
 *
 * h265web.js 与 jessibuca.js 都是以 `<script>` 动态注入的 UMD 产物，不通过 npm 引入，
 * 因此这里只声明它们在 `window` 上暴露的构造入口与实例方法，供 TypeScript 严格校验。
 *
 * 运行时代码见 `src/admin/lib/h265webLoader.ts` / `jessibucaLoader.ts`。
 * 许可证说明见 `public/static/js/h265web/README-LICENSE.md` 与
 * `public/static/js/jessibuca/README-LICENSE.md`。
 */

/** h265web 探测到的媒体信息（不同版本字段略有差异，取常见的宽高字段） */
interface H265webMediaInfo {
  width?: number
  height?: number
  videoWidth?: number
  videoHeight?: number
  [key: string]: unknown
}

interface H265webBuildConfig {
  player_id: string
  base_url?: string
  wasm_js_uri?: string
  wasm_wasm_uri?: string
  ext_src_js_uri?: string
  ext_wasm_js_uri?: string
  width?: number
  height?: number
  color?: string
  auto_play?: boolean
  ignore_audio?: boolean
  [key: string]: unknown
}

interface H265webPlayerInstance {
  /** 首帧画面渲染完成（在 build 之前挂载） */
  on_ready_show_done_callback?: () => void
  /** 媒体探测完成，可拿到宽高等信息 */
  video_probe_callback?: (mediaInfo: H265webMediaInfo) => void
  on_play_time?: (videoPTS: number) => void
  on_play_finished?: () => void
  on_error_callback?: (error: unknown) => void
  build(config: H265webBuildConfig): boolean
  load_media(url: string): void
  play(): boolean
  pause(): void
  release(): void
  isPlaying?(): boolean
  resize?(width: number, height: number): void
  set_voice?(volume: number): void
  set_playback_rate?(rate: number): void
  screenshot?(imgElementId: string): void
  fullScreen?(): void
  closeFullScreen?(): void
}

type H265webFactory = () => H265webPlayerInstance

/** Jessibuca 对外事件回调载荷，官方 d.ts 中不同事件的参数类型不同，这里按需收窄 */
type JessibucaEventName =
  | 'play'
  | 'pause'
  | 'error'
  | 'timeout'
  | 'loadingTimeout'
  | 'delayTimeout'
  | 'videoInfo'
  | 'audioInfo'
  | 'kBps'
  | 'performance'
  | 'timeUpdate'
  | 'load'
  | 'mute'
  | 'fullscreen'
  | 'playToRenderTimes'

interface JessibucaVideoInfo {
  width?: number
  height?: number
  videoWidth?: number
  videoHeight?: number
  [key: string]: unknown
}

interface JessibucaOptions {
  container: HTMLElement
  decoder?: string
  videoBuffer?: number
  isResize?: boolean
  useMSE?: boolean
  useWCS?: boolean
  isFlv?: boolean
  isNotMute?: boolean
  hasAudio?: boolean
  text?: string
  loadingText?: string
  loadingTimeout?: number
  timeout?: number
  debug?: boolean
  hotKey?: boolean
  supportDblclickFullscreen?: boolean
  operateBtns?: {
    fullscreen?: boolean
    screenshot?: boolean
    play?: boolean
    audio?: boolean
    recorder?: boolean
  }
  [key: string]: unknown
}

interface JessibucaInstance {
  play(url?: string): void
  pause(): void
  destroy(): void
  screenshot?(): void
  resize?(): void
  setFullscreen?(fullscreen: boolean): void
  hasLoaded?(): boolean
  mute?(): void
  cancelMute?(): void
  on(event: JessibucaEventName, callback: (payload?: unknown) => void): void
}

type JessibucaConstructor = new (options: JessibucaOptions) => JessibucaInstance

interface Window {
  /** h265web.js 注入后挂载；调用一次创建一个播放器实例 */
  H265webjsPlayer?: H265webFactory
  /** jessibuca.js 注入后挂载 */
  Jessibuca?: JessibucaConstructor
}
