import { collectConsole } from '../helpers/console'
import {
  ADMIN,
  ANONYMOUS_STATE,
  adminPath,
  expect,
  gotoAdmin,
  loginViaUI,
  readPermCodes,
  readToken,
  LOGIN_BUTTON,
  shot,
  test,
} from '../fixtures/auth'

/** 登录相关用例一律以「未登录」状态运行 */
test.use({ storageState: ANONYMOUS_STATE })

test.describe('01a 登录', () => {
  test('空表单提交触发必填校验，不发起请求', async ({ page }) => {
    const console = collectConsole(page)
    await page.goto(adminPath('/login'))

    await expect(page.locator('.login-card')).toBeVisible()
    await expect(page.getByText('AI 智能监控平台 · 管理后台')).toBeVisible()

    await page.getByRole('button', { name: LOGIN_BUTTON }).click()

    await expect(page.getByText('请输入用户名')).toBeVisible()
    await expect(page.getByText('请输入密码')).toBeVisible()
    // 未发起登录请求，localStorage 不应有 token
    expect(await readToken(page)).toBeNull()
    expect(console.texts()).toEqual([])
    await shot(page, '01a-login-empty-validate')
  })

  test('密码错误登录失败：停留登录页并提示错误信息', async ({ page }) => {
    await page.goto(adminPath('/login'))

    await page.getByPlaceholder('用户名').fill(ADMIN.user_name)
    await page.getByPlaceholder('密码').fill('e2e-wrong-password')

    const respPromise = page.waitForResponse((res) => res.url().includes('/web/auth/login'))
    await page.getByRole('button', { name: LOGIN_BUTTON }).click()
    expect((await respPromise).status()).toBe(401)

    // 已修复（v0.1.0）：401 不再触发响应拦截器的整页跳转，登录页原地提示后端返回的原因，
    // 用户能区分「密码错误」与其它异常（此前只看到页面「闪一下」）。
    await expect(page.locator('.ant-message').getByText('用户名或密码错误')).toBeVisible()
    await expect(page).toHaveURL(/\/admin\/login/)
    await expect(page.locator('.login-card')).toBeVisible()
    await expect(page.getByPlaceholder('用户名')).toBeVisible()
    expect(await readToken(page)).toBeNull()
    await shot(page, '01a-login-wrong-password')

    // 接口契约：错误密码返回 401 + 明确提示语
    const apiResp = await page.request.post('/web/auth/login', {
      data: { user_name: ADMIN.user_name, password: 'e2e-wrong-password' },
    })
    expect(apiResp.status()).toBe(401)
    expect((await apiResp.json()).message).toBe('用户名或密码错误')
  })

  test('用户名+密码为空提交时相同字段不重复提示（仅两条必填）', async ({ page }) => {
    await page.goto(adminPath('/login'))
    await page.getByRole('button', { name: LOGIN_BUTTON }).click()
    await expect(page.locator('.ant-form-item-explain-error')).toHaveCount(2)
  })

  test('正确账号密码登录成功并跳转首页看板', async ({ page }) => {
    const console = collectConsole(page)
    await loginViaUI(page)

    await expect(page).toHaveURL(/\/admin\/dashboard/)
    // 登录态三要素均写入 localStorage（perm_codes 由菜单树异步落盘，需轮询）
    expect(await readToken(page)).not.toBeNull()
    await expect.poll(() => readPermCodes(page), { timeout: 10_000 }).toContain('dashboard:view')

    await expect(page.locator('.page-container-title').first()).toHaveText('首页看板')
    await expect(page.locator('.main-header-username')).toHaveText('系统管理员')
    expect(console.texts()).toEqual([])
    await shot(page, '01a-login-success')
  })

  test('未登录访问受保护路由被重定向到登录页', async ({ page }) => {
    await page.goto(adminPath('/basic/region'))
    await expect(page).toHaveURL(/\/admin\/login/)
    await expect(page.locator('.login-card')).toBeVisible()
  })

  test('退出登录后回到登录页且清空 token', async ({ page }) => {
    await loginViaUI(page)
    await expect(page).toHaveURL(/\/admin\/dashboard/)

    await page.locator('.main-header-user').click()
    await page.getByRole('menuitem', { name: '退出登录' }).click()

    await expect(page).toHaveURL(/\/admin\/login/)
    expect(await readToken(page)).toBeNull()
    expect(await readPermCodes(page)).toEqual([])
  })

  test('登录后可访问首页以外的受保护页面（标题正确）', async ({ page }) => {
    await loginViaUI(page)
    await gotoAdmin(page, '/basic/region')
    await expect(page.locator('.page-container-title').first()).toHaveText('区域管理')
  })
})
