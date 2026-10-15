/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** 后端 API 基地址，如 http://localhost:8080 */
  readonly VITE_API_BASE: string
  /** 开发态代理目标端口，默认 8080 */
  readonly VITE_BACKEND_PORT?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

/** 由各 vite 配置的 define 注入：管理后台 / 官网为 false，Electron 渲染进程为 true */
declare const __IS_ELECTRON__: boolean
