import { expect, test as setup } from '@playwright/test'
import { loginViaUI, readPermCodes, readToken } from './auth'

/** storageState 落盘位置（smoke 项目的 `use.storageState` 指向同一路径） */
export const ADMIN_STATE = 'e2e/.auth/admin.json'

/**
 * 全局准备：真实登录一次，保存 token / user_info / perm_codes 到 storageState。
 * 后续 smoke 用例直接复用登录态，避免每个用例都走一遍登录流程。
 */
setup('登录并保存认证状态', async ({ page }) => {
  await loginViaUI(page)

  // 校验登录态确实写入了 localStorage（缺 perm_codes 会导致按钮级权限渲染异常）
  const token = await readToken(page)
  if (!token) throw new Error('登录后 localStorage.token 为空')

  // 菜单树写入 perm_codes 是 React effect 里的异步落盘，需轮询等待
  await expect.poll(() => readPermCodes(page), { timeout: 10_000 }).not.toHaveLength(0)

  await page.context().storageState({ path: ADMIN_STATE })
})
