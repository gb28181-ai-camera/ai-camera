import {
  OK_BUTTON,
  expect,
  gotoAdmin,
  readPermCodes,
  shot,
  test,
  uniqName,
} from '../fixtures/auth'
import {
  apiCall,
  confirmRowDelete,
  createRecord,
  expectApiOk,
  hasButton,

  searchTable,
  tableRow,
} from '../helpers/crud'

const MODAL = '.e2e-region-modal'
const LIST_API = '/web/basic/regions'
const SEARCH_HINT = '请输入区域名称/编码'

/** 打开新增弹窗并填写表单（仅在「新增区域」按钮渲染时调用） */
async function createRegionViaUI(
  page: import('@playwright/test').Page,
  name: string,
  code: string,
  remark: string
): Promise<void> {
  await page.getByRole('button', { name: '新增区域' }).click()
  await expect(page.locator(MODAL)).toBeVisible()
  await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增区域')

  await page.locator(MODAL).getByPlaceholder('请输入区域名称').fill(name)
  await page.locator(MODAL).getByPlaceholder(/请输入区域编码/).fill(code)
  await page.locator(MODAL).getByPlaceholder('数值越小越靠前').fill('88')
  await page.locator(MODAL).getByPlaceholder('请输入备注').fill(remark)

  const createResp = page.waitForResponse(
    (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
  )
  await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
  // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
  expect([200, 201]).toContain((await createResp).status())
  await expect(page.locator(MODAL)).toBeHidden()
}

/** 新增区域：走真实 UI 交互（v0.1.0 已补齐 basic:region:create，按钮对超管渲染） */
async function createRegion(
  page: import('@playwright/test').Page,
  name: string,
  code: string,
  remark: string
): Promise<'ui' | 'api'> {
  return createRecord(
    page,
    '新增区域',
    () => createRegionViaUI(page, name, code, remark),
    () => apiCall(page, 'POST', LIST_API, { name, code, remark })
  )
}

test.describe('02a 区域管理', () => {
  test.beforeEach(async ({ page }) => {
    const listResp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/basic/region')
    await listResp
    await expect(page.locator('.page-container-title').first()).toHaveText('区域管理')
  })

  test('列表渲染：列定义与种子数据完整', async ({ page }) => {
    for (const title of ['区域名称', '区域编码', '排序', '备注', '创建时间', '操作']) {
      await expect(page.locator('.ant-table-thead th', { hasText: title })).toHaveCount(1)
    }
    // 种子数据「一号厂区」应出现在列表中
    await expect(tableRow(page, '一号厂区')).toHaveCount(1)
    await expect(tableRow(page, '一号厂区')).toContainText('AREA_A')
    await shot(page, '02a-region-list')
  })

  test('新增区域后列表中出现该记录', async ({ page }) => {
    const name = uniqName('E2E区域')
    const code = `E2E_${Date.now()}`
    const remark = 'E2E 冒烟测试创建'

    const path = await createRegion(page, name, code, remark)

    // 列表出现该条记录（用关键字搜索保证定位确定性）
    await searchTable(page, SEARCH_HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(code)
    await expect(row).toContainText(remark)
    if (path === 'ui') await expect(row).toContainText('88')
    await shot(page, '02a-region-created')

    await deleteRegion(page, name)
  })

  test('编辑区域后字段值已变更', async ({ page }) => {
    const name = uniqName('E2E编辑区')
    const renamed = `${name}_改`
    const code = `E2E_EDIT_${Date.now()}`
    await createRegion(page, name, code, '待编辑')

    await searchTable(page, SEARCH_HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)

    // v0.1.0 已修复权限码不一致：basic:region:update 已入菜单树，编辑按钮对超管渲染
    await expect(row.getByRole('button', { name: /编\s*辑/ })).toBeVisible()

    // UI 路径：弹窗应带出原值，改完提交后列表字段更新
    await row.getByRole('button', { name: /编\s*辑/ }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('编辑区域')
    await expect(page.locator(MODAL).getByPlaceholder('请输入区域名称')).toHaveValue(name)

    await page.locator(MODAL).getByPlaceholder('请输入区域名称').fill(renamed)
    await page.locator(MODAL).getByPlaceholder('请输入备注').fill('E2E 编辑后的备注')
    const updateResp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
    )
    await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
    expect((await updateResp).status()).toBe(200)
    await expect(page.locator(MODAL)).toBeHidden()

    // 断言：按唯一编码检索（名称会随编辑变化），行内字段已更新为新值
    await searchTable(page, SEARCH_HINT, code, LIST_API)
    const updated = tableRow(page, code)
    await expect(updated).toHaveCount(1)
    await expect(updated.locator('td').nth(0)).toHaveText(renamed)
    await expect(updated).toContainText('E2E 编辑后的备注')

    // 名称确实已改名：原名称不再是任一行「区域名称」列的完整取值
    await searchTable(page, SEARCH_HINT, name, LIST_API)
    await expect(page.locator('.ant-table-row').filter({ hasText: name })).toHaveCount(1)
    await expect(page.locator('.ant-table-row td').nth(0)).toHaveText(renamed)

    await deleteRegion(page, renamed)
  })

  test('名称必填校验：不填写名称时无法提交', async ({ page }) => {
    // v0.1.0 已修复权限码不一致：basic:region:create 已入菜单树，新增按钮对超管渲染
    await expect(page.getByRole('button', { name: '新增区域' })).toBeVisible()

    await page.getByRole('button', { name: '新增区域' }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
    await expect(page.locator(MODAL).getByText('请输入区域名称')).toBeVisible()
    await expect(page.locator(MODAL)).toBeVisible()
    await page.locator(MODAL).getByRole('button', { name: /取\s*消/ }).click()
    await expect(page.locator(MODAL)).toBeHidden()
  })

  test('按关键字搜索命中目标记录，重置后恢复全量', async ({ page }) => {
    const name = uniqName('E2E搜索区')
    await createRegion(page, name, `E2E_SEARCH_${Date.now()}`, '搜索用例')

    await searchTable(page, SEARCH_HINT, name, LIST_API)
    await expect(page.locator('.ant-table-row')).toHaveCount(1)
    await expect(tableRow(page, name)).toHaveCount(1)

    // 无匹配关键字 → 空列表
    await searchTable(page, SEARCH_HINT, `不存在的区域_${Date.now()}`, LIST_API)
    await expect(page.locator('.ant-table-row')).toHaveCount(0)
    await expect(page.locator('.ant-empty')).toBeVisible()

    // 重置后恢复全量列表
    const resetResp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: '重置' }).click()
    await resetResp
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThan(0)

    await deleteRegion(page, name)
  })

  test('删除区域后该记录从列表消失', async ({ page }) => {
    const name = uniqName('E2E删除区')
    await createRegion(page, name, `E2E_DEL_${Date.now()}`, '删除用例')

    await searchTable(page, SEARCH_HINT, name, LIST_API)
    await expect(tableRow(page, name)).toHaveCount(1)
    await shot(page, '02a-region-before-delete')

    await deleteRegion(page, name)

    await searchTable(page, SEARCH_HINT, name, LIST_API)
    await expect(page.locator('.ant-table-row').filter({ hasText: name })).toHaveCount(0)
  })

  /**
   * 区域 CRUD 按钮对超管可见并可点击。
   *
   * 历史背景：页面按钮级权限判定使用 `basic:region:create/update/delete`，
   * 而菜单树（ac_menu.permission_code）只提供 `region:create/update/delete`，
   * 两者不一致导致按钮对任何用户（含超管）都不渲染。
   * v0.1.0 已修复：初始化 SQL 追加了 type='button' 的按钮菜单行并授予 super_admin。
   */
  test('区域 CRUD 按钮对超管可见并可点击', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('basic:region:create')
    expect(codes).toContain('basic:region:update')
    expect(codes).toContain('basic:region:delete')

    await expect(page.getByRole('button', { name: '新增区域' })).toBeVisible()
    await expect(page.locator('.page-container-extra button')).toHaveCount(1)

    // 行内「编辑 / 删除」入口可达
    const row = tableRow(page, '一号厂区')
    await expect(row).toHaveCount(1)
    await expect(row.getByRole('button', { name: /编\s*辑/ })).toBeVisible()
    await expect(row.getByRole('button', { name: /删\s*除/ })).toBeVisible()

    // 新增按钮确实可用：点击后弹出表单弹窗
    await page.getByRole('button', { name: '新增区域' }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增区域')
    await page.locator(MODAL).getByRole('button', { name: /取\s*消/ }).click()
    await expect(page.locator(MODAL)).toBeHidden()
  })
})

/** 查询区域 id（用于按钮被门控时的 API 兜底路径） */
async function regionIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const res = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200&keyword=${encodeURIComponent(name)}`)
  const data = await expectApiOk(res)
  const items = (data.items ?? []) as { id: number; name: string }[]
  const hit = items.find((item) => item.name === name)
  if (!hit) throw new Error(`未查询到区域：${name}`)
  return hit.id
}

/** 删除指定名称的区域（UI 优先，按钮被门控时走 API） */
async function deleteRegion(page: import('@playwright/test').Page, name: string): Promise<void> {
  await searchTable(page, SEARCH_HINT, name, LIST_API)
  const row = tableRow(page, name)
  await expect(row).toHaveCount(1)

  if (await hasButton(page, /删\s*除/)) {
    await confirmRowDelete(page, row)
  } else {
    const id = await regionIdOf(page, name)
    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
  }
}
