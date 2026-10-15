import { expect, test as base, type Page } from '@playwright/test'

/** 超级管理员账号（Seed 数据，data_perm=all） */
export const ADMIN = { user_name: 'admin', password: 'admin123' } as const

/** 已注销状态：清空 storageState，用于登录页相关用例 */
export const ANONYMOUS_STATE = { cookies: [], origins: [] } as const

/**
 * antd Button 会为「恰好两个汉字」的文案自动插入空格（autoInsertSpace 默认开启），
 * 因此「登录」实际渲染为「登 录」、「确定」渲染为「确 定」。统一用正则匹配。
 */
export const LOGIN_BUTTON = /登\s*录/
export const OK_BUTTON = /确\s*定/
export const CANCEL_BUTTON = /取\s*消/
export const CLOSE_BUTTON = /关\s*闭/

/** localStorage 键（与 src/admin/store/authStore.ts 保持一致） */
export const LS_TOKEN = 'token'
export const LS_USER = 'user_info'
export const LS_PERM = 'perm_codes'

/** 管理后台路径拼接（baseURL 已含 `/admin/`，此处用相对路径解析） */
export function adminPath(path = '/dashboard'): string {
  return path.replace(/^\/+/, '')
}

/**
 * 打开管理后台某一页面。
 * baseURL 为 `http://localhost:5173/admin/`，传入 `/basic/region` 之类的站内路径即可。
 */
export async function gotoAdmin(page: Page, path = '/dashboard'): Promise<void> {
  await page.goto(adminPath(path))
}

/**
 * 通过 UI 登录并等待菜单树加载完成（`perm_codes` 落 localStorage）。
 * 成功后停留在 `/admin/dashboard`。
 */
export async function loginViaUI(
  page: Page,
  creds: { user_name: string; password: string } = ADMIN
): Promise<void> {
  await page.goto(adminPath('/login'))
  await page.getByPlaceholder('用户名').fill(creds.user_name)
  await page.getByPlaceholder('密码').fill(creds.password)

  const loginResp = page.waitForResponse((res) => res.url().includes('/web/auth/login'))
  // 主框架挂载后会拉取菜单树，菜单会写入 perm_codes —— 必须等它完成再保存 storageState
  const menusResp = page.waitForResponse((res) => res.url().includes('/web/profile/menus'))

  await page.getByRole('button', { name: LOGIN_BUTTON }).click()

  expect((await loginResp).status()).toBe(200)
  await menusResp
  await expect(page).toHaveURL(/\/admin\/dashboard/)
}

/** 读取 localStorage 中的登录 token */
export async function readToken(page: Page): Promise<string | null> {
  return page.evaluate((key) => window.localStorage.getItem(key), LS_TOKEN)
}

/** 读取 localStorage 中的权限码（由菜单树递归提取） */
export async function readPermCodes(page: Page): Promise<string[]> {
  const raw = await page.evaluate((key) => window.localStorage.getItem(key), LS_PERM)
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as string[]) : []
  } catch {
    return []
  }
}

/**
 * 等待表格数据加载结束。
 * 以列表接口响应为同步点，避免使用 waitForTimeout。
 */
export async function waitForListResponse(page: Page, urlPart: string): Promise<void> {
  await page.waitForResponse((res) => res.url().includes(urlPart) && res.request().method() === 'GET')
}

/** 生成带时间戳的用例数据名，避免重复运行冲突 */
export function uniqName(prefix: string): string {
  return `${prefix}_${Date.now()}${Math.floor(Math.random() * 1000)}`
}

/** 断言当前页面标题（PageContainer 标题栏）为指定文本 */
export async function expectPageTitle(page: Page, title: string): Promise<void> {
  await expect(page.locator('.page-container-title').first()).toHaveText(title)
}

/** 在指定 e2e Modal 内点击底部按钮（禁止使用 .ant-modal 系列通用选择器） */
export async function clickModalButton(
  page: Page,
  modalClass: string,
  name: '确定' | '取消' | string
): Promise<void> {
  await page.locator(modalClass).getByRole('button', { name }).click()
}

/** 保存截图到 `frontend/e2e/screenshots/` */
export async function shot(page: Page, name: string): Promise<void> {
  await page.screenshot({ path: `e2e/screenshots/${name}.png`, fullPage: true })
}

/** 通用登录态测试基类（沿用 storageState） */
export const test = base
export { expect }
