import { contextBridge } from 'electron'

/**
 * 暴露给渲染进程的最小能力集。
 * 本项目无串口 / 固件烧录需求，仅提供平台与版本信息用于界面适配。
 */
const electronAPI = {
  /** 运行平台，如 win32 / darwin / linux */
  platform: process.platform,
  /** 桌面端版本号（与应用版本一致） */
  version: process.env.npm_package_version ?? '',
  /** 恒为 true，渲染进程据此判定运行在 Electron 中 */
  isDesktop: true,
}

contextBridge.exposeInMainWorld('electronAPI', electronAPI)
