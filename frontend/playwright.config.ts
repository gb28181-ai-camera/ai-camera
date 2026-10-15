import { defineConfig, devices } from '@playwright/test'

const PORT = Number(process.env.E2E_PORT || 5173)
const BASE_URL = process.env.E2E_BASE_URL || `http://localhost:${PORT}/admin/`

/** 后端端口：测试环境使用 8081（backend/config/config-test.yaml） */
const BACKEND_PORT = process.env.VITE_BACKEND_PORT || '8081'

/**
 * 生成的 Controller 以 `import.meta.env.VITE_API_BASE` 作为 axios baseURL。
 * 仓库 `.env` 中该值为 `http://localhost:8080`，会让浏览器直连 8080（绕过 Vite 代理），
 * 而测试后端监听 8081 且**未启用 CORS 中间件**，跨域请求会被浏览器拦截。
 * 因此测试时将 VITE_API_BASE 置空 → 请求走相对路径 → 命中 Vite 代理 → :8081（同源）。
 * `VITE_BACKEND_PORT` 同时决定了 vite.config.ts 中代理的 target。
 */
const API_BASE = process.env.VITE_API_BASE ?? ''

/** 冒烟测试目录（串行执行，按文件名字母序） */
const SMOKE_GLOB = '**/smoke/**/*.spec.ts'

/**
 * 浏览器 channel：默认使用 Playwright 自带的 chromium（`npx playwright install chromium`）。
 * 若所在环境无法下载浏览器，可改用系统已安装的浏览器：
 *   `E2E_BROWSER_CHANNEL=msedge npx playwright test`
 */
const CHANNEL = process.env.E2E_BROWSER_CHANNEL as 'chrome' | 'msedge' | undefined

/** 统一的项目 use（可选 channel 覆盖，未设置时为 undefined 即默认 chromium） */
const BROWSER_USE = {
  ...devices['Desktop Chrome'],
  ...(CHANNEL ? { channel: CHANNEL } : {}),
}

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [['html', { outputFolder: 'playwright-report' }], ['list']],
  outputDir: 'test-results',
  use: {
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    // 登录一次，保存 storageState 供 smoke 项目复用（避免每个用例重复登录）
    {
      name: 'setup',
      testMatch: '**/auth.setup.ts',
      use: { ...BROWSER_USE },
    },
    // 冒烟套件：串行执行（--workers=1），必须先跑完 01a~10b，最后跑 99-
    {
      name: 'smoke',
      testMatch: SMOKE_GLOB,
      fullyParallel: false,
      workers: 1,
      dependencies: ['setup'],
      use: {
        ...BROWSER_USE,
        storageState: 'e2e/.auth/admin.json',
      },
    },
    // 通用项目：冒烟以外的用例（默认无认证态）
    {
      name: 'chromium',
      testIgnore: SMOKE_GLOB,
      use: { ...BROWSER_USE },
    },
  ],
  webServer: {
    command: `npx vite --port ${PORT}`,
    url: `http://localhost:${PORT}/admin/`,
    reuseExistingServer: !process.env.CI,
    timeout: 60000,
    env: {
      VITE_BACKEND_PORT: BACKEND_PORT,
      VITE_API_BASE: API_BASE,
    },
  },
})
