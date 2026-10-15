export {}

declare global {
  /** preload 通过 contextBridge 暴露的桌面端能力（见 electron/preload.ts） */
  interface ElectronAPI {
    /** 运行平台，如 win32 / darwin / linux */
    platform: string
    /** 桌面端版本号 */
    version: string
    /** 恒为 true，用于渲染进程判定当前是否运行在 Electron 中 */
    isDesktop: boolean
  }

  interface Window {
    /** 仅桌面端存在；Web 端为 undefined */
    electronAPI?: ElectronAPI
  }
}
