import { OK_BUTTON, expect, gotoAdmin, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  confirmRowDelete,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  listBrokenByEmptyParams,
  searchTable,
  selectFormOption,
  tableRow,
  totalOf,
} from '../helpers/crud'

const MODAL = '.e2e-user-modal'
const RESET_MODAL = '.e2e-user-resetpwd-modal'
const LIST_API = '/web/system/users'
const HINT = '请输入用户名或姓名'
const SEED_DEPT = '总公司'
const PASSWORD = 'E2e@Test1234'

/**
 * 生成控制器 UserController.pageUsers 始终拼接 `dept_id=&active=`，
 * 修复前后端把空串当有效过滤条件（dept_id=0 / active=false）→ 列表恒为空。
 * v0.1.0 已修复：后端新增 NormalizeEmptyQuery 中间件，绑定前剔除空值查询参数，
 * 因此 listBroken 恒为 false，下列用例断言列表**有数据**。
 *
 * 同时 `system:user:*` 权限码（id 9101~9104）已入库并授予超管，
 * 「新建用户 / 编辑 / 重置密码 / 删除」与状态开关对超管均可用，用例走完整 UI 路径。
 */
const URL_WITH_EMPTY_PARAMS = `${LIST_API}?page=1&page_size=20&keyword=&dept_id=&active=`
const URL_CLEAN_PARAMS = `${LIST_API}?page=1&page_size=20`

async function userIdOf(page: import('@playwright/test').Page, userName: string): Promise<number> {
  const data = await expectApiOk(
    await apiCall(
      page,
      'GET',
      `${LIST_API}?page=1&page_size=200&keyword=${encodeURIComponent(userName)}`
    )
  )
  const hit = ((data.items ?? []) as { id: number; user_name: string }[]).find(
    (i) => i.user_name === userName
  )
  if (!hit) throw new Error(`未查询到用户：${userName}`)
  return hit.id
}

async function createUser(
  page: import('@playwright/test').Page,
  userName: string,
  name: string
): Promise<'ui' | 'api'> {
  if (!(await hasButton(page, '新建用户'))) {
    await expectApiOk(
      await apiCall(page, 'POST', LIST_API, {
        user_name: userName,
        name,
        password: PASSWORD,
        dept_id: 1,
      })
    )
    return 'api'
  }

  await page.getByRole('button', { name: '新建用户' }).click()
  await expect(page.locator(MODAL)).toBeVisible()
  await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新建用户')
  await page.locator(MODAL).getByPlaceholder('登录名，创建后不可修改').fill(userName)
  await page.locator(MODAL).getByPlaceholder('请输入姓名').fill(name)
  await page.locator(MODAL).getByPlaceholder('请输入初始密码').fill(PASSWORD)
  await selectFormOption(page, page.locator(MODAL), '部门', SEED_DEPT)

  const resp = page.waitForResponse(
    (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
  )
  await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
  // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
  expect([200, 201]).toContain((await resp).status())
  await expect(page.locator(MODAL)).toBeHidden()
  return 'ui'
}

async function deleteUser(page: import('@playwright/test').Page, userName: string): Promise<void> {
  const id = await userIdOf(page, userName)
  if (await hasButton(page, /删\s*除/)) {
    await searchTable(page, HINT, userName, LIST_API)
    const row = tableRow(page, userName)
    if ((await row.count()) === 1) {
      await confirmRowDelete(page, row)
      return
    }
  }
  await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
}

test.describe('04a 用户管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/system/user')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('用户管理')
  })

  test('列表页渲染：列定义与筛选控件完整', async ({ page }) => {
    await expectColumns(page, [
      '用户名',
      '姓名',
      '部门',
      '岗位',
      '角色',
      '邮箱',
      '手机号',
      '状态',
      '最后登录',
      '操作',
    ])
    // 部门 / 岗位 / 状态下拉筛选（SearchBar 之外的额外汇总条件）
    await expect(page.getByText('部门：')).toBeVisible()
    await expect(page.getByText('岗位：')).toBeVisible()
    await expect(page.getByText('状态：')).toBeVisible()
    await expect(page.getByPlaceholder('请输入用户名或姓名')).toBeVisible()
    await shot(page, '04a-user-list')
  })

  test('新建用户后可按用户名查询到该记录（含部门名称）', async ({ page }) => {
    const userName = `e2e_user_${Date.now()}`
    const name = uniqName('E2E用户')
    const path = await createUser(page, userName, name)

    const id = await userIdOf(page, userName)
    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(detail.name).toBe(name)
    expect(detail.dept_id).toBe(1)

    // v0.1.0 已修复：带空查询参数（keyword=&dept_id=&active=）的列表不再恒为空
    const broken = await listBrokenByEmptyParams(page, URL_WITH_EMPTY_PARAMS, URL_CLEAN_PARAMS)
    expect(broken).toBe(false)

    await searchTable(page, HINT, userName, LIST_API)
    const row = tableRow(page, userName)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(name)
    await expect(row).toContainText(SEED_DEPT)
    if (path === 'ui') await expect(row).toContainText('未分配')
    await shot(page, '04a-user-created')

    await deleteUser(page, userName)
  })

  test('编辑用户后姓名与邮箱已变更（用户名只读）', async ({ page }) => {
    const userName = `e2e_edit_${Date.now()}`
    const name = uniqName('E2E改用户')
    const renamed = `${name}_新`
    await createUser(page, userName, name)

    // v0.1.0 已修复：带空查询参数（keyword=&dept_id=&active=）的列表不再恒为空
    const broken = await listBrokenByEmptyParams(page, URL_WITH_EMPTY_PARAMS, URL_CLEAN_PARAMS)
    expect(broken).toBe(false)
    // v0.1.0 已修复：`system:user:update` 已入库 → 行内「编辑」入口对超管渲染
    const uiEditable = await hasButton(page, /编\s*辑/)
    expect(uiEditable).toBe(true)

    if (uiEditable) {
      await searchTable(page, HINT, userName, LIST_API)
      const row = tableRow(page, userName)
      await expect(row).toHaveCount(1)
      await row.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('编辑用户')
      await expect(page.locator(MODAL).getByPlaceholder('登录名，创建后不可修改')).toBeDisabled()
      await page.locator(MODAL).getByPlaceholder('请输入姓名').fill(renamed)
      await page.locator(MODAL).getByPlaceholder('请输入邮箱').fill('e2e@example.com')
      const resp = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await resp).status()).toBe(200)
      await expect(page.locator(MODAL)).toBeHidden()
    } else {
      const id = await userIdOf(page, userName)
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}`, {
          name: renamed,
          email: 'e2e@example.com',
          dept_id: 1,
        })
      )
    }

    // 接口层校验字段确实已更新
    const detail = await expectApiOk(
      await apiCall(page, 'GET', `${LIST_API}/${await userIdOf(page, userName)}`)
    )
    expect(detail.name).toBe(renamed)
    expect(detail.email).toBe('e2e@example.com')

    // v0.1.0 已修复：列表可按用户名检索到该记录，且展示更新后的姓名
    await searchTable(page, HINT, userName, LIST_API)
    await expect(tableRow(page, userName)).toContainText(renamed)

    await deleteUser(page, userName)
  })

  test('启用/禁用接口可切换用户状态', async ({ page }) => {
    const userName = `e2e_status_${Date.now()}`
    await createUser(page, userName, uniqName('E2E状态'))
    const id = await userIdOf(page, userName)

    // v0.1.0 已修复：带空查询参数（keyword=&dept_id=&active=）的列表不再恒为空
    const broken = await listBrokenByEmptyParams(page, URL_WITH_EMPTY_PARAMS, URL_CLEAN_PARAMS)
    expect(broken).toBe(false)

    // v0.1.0 已修复：`system:user:update` 已入库 → 状态开关对超管可用
    await searchTable(page, HINT, userName, LIST_API)
    const row = tableRow(page, userName)
    await expect(row).toHaveCount(1)
    const toggle = row.locator('.ant-switch')
    await expect(toggle).toBeEnabled()

    // UI 切换：启用 → 禁用
    const resp = page.waitForResponse((res) =>
      /\/web\/system\/users\/\d+\/(enable|disable)$/.test(res.url())
    )
    await toggle.click()
    expect((await resp).status()).toBe(200)
    await expect(toggle).toHaveAttribute('aria-checked', 'false')

    // 接口层确认已禁用
    const disabled = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(disabled.active).toBe(false)

    // 复原为启用
    await expectApiOk(await apiCall(page, 'PUT', `${LIST_API}/${id}/enable`))
    const enabled = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(enabled.active).toBe(true)

    await deleteUser(page, userName)
  })

  test('重置密码接口可用（UI 弹窗或 API）', async ({ page }) => {
    const userName = `e2e_pwd_${Date.now()}`
    await createUser(page, userName, uniqName('E2E改密'))
    const id = await userIdOf(page, userName)

    // v0.1.0 已修复：空查询参数不再导致列表为空；`system:user:reset` 已入库 →
    // 「重置密码」入口对超管渲染
    const broken = await listBrokenByEmptyParams(page, URL_WITH_EMPTY_PARAMS, URL_CLEAN_PARAMS)
    expect(broken).toBe(false)
    if (await hasButton(page, '重置密码')) {
      await searchTable(page, HINT, userName, LIST_API)
      const row = tableRow(page, userName)
      await expect(row).toHaveCount(1)
      await row.getByRole('button', { name: '重置密码' }).click()
      await expect(page.locator(RESET_MODAL)).toBeVisible()
      await page.locator(RESET_MODAL).getByPlaceholder('请输入新密码').fill('NewPass@123')
      await page.locator(RESET_MODAL).getByPlaceholder('请再次输入新密码').fill('NewPass@123')
      const resp = page.waitForResponse((res) =>
        /\/web\/system\/users\/\d+\/reset-password$/.test(res.url())
      )
      await page.locator(RESET_MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await resp).status()).toBe(200)
      await expect(page.locator(RESET_MODAL)).toBeHidden()
    } else {
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}/reset-password`, {
          new_password: 'NewPass@123',
        })
      )
    }

    await deleteUser(page, userName)
  })

  test('删除用户后该记录消失', async ({ page }) => {
    const userName = `e2e_del_${Date.now()}`
    await createUser(page, userName, uniqName('E2E删除'))
    const id = await userIdOf(page, userName)

    // v0.1.0 已修复：列表不再因空查询参数恒为空
    const broken = await listBrokenByEmptyParams(page, URL_WITH_EMPTY_PARAMS, URL_CLEAN_PARAMS)
    expect(broken).toBe(false)

    await searchTable(page, HINT, userName, LIST_API)
    await expect(tableRow(page, userName)).toHaveCount(1)

    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))

    const clean = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200`)
    const items = ((clean.body.data as { items?: { user_name: string }[] }).items ?? [])
    expect(items.find((i) => i.user_name === userName)).toBeUndefined()

    await searchTable(page, HINT, userName, LIST_API)
    await expectEmptyTable(page)
  })

  /**
   * 空查询参数不再导致用户列表恒为空。
   *
   * 历史背景：`UserController.pageUsers` 生成 `?dept_id=&active=`，后端把空串解析为
   * 有效过滤条件（dept_id=0 / active=false）→ 用户列表恒为 0 条，页面永远显示「暂无数据」。
   * v0.1.0 已修复：后端新增 `middleware.NormalizeEmptyQuery()`，绑定前剔除空值查询参数；
   * 显式传值（如 `active=false`）仍正常过滤。
   */
  test('空查询参数不再导致用户列表恒为空（列表有数据）', async ({ page }) => {
    const broken = await listBrokenByEmptyParams(page, URL_WITH_EMPTY_PARAMS, URL_CLEAN_PARAMS)
    expect(broken).toBe(false)

    // 页面渲染的是真实数据而非空态
    await expect(page.locator('.ant-table-row').first()).toBeVisible()
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThan(0)
    await expect(page.locator('.page-container-body')).not.toContainText('暂无数据')

    // 空参数接口返回与干净参数一致的 total
    const withEmpty = totalOf(await apiCall(page, 'GET', URL_WITH_EMPTY_PARAMS))
    const clean = totalOf(await apiCall(page, 'GET', URL_CLEAN_PARAMS))
    expect(withEmpty).toBe(clean)
    expect(withEmpty).toBeGreaterThan(0)

    // 显式传 `active=false` 仍生效：只返回已禁用用户
    const inactive = await expectApiOk(
      await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=20&active=false`)
    )
    const inactiveItems = (inactive.items ?? []) as { active?: boolean }[]
    for (const item of inactiveItems) {
      expect(item.active).toBe(false)
    }
  })
})
