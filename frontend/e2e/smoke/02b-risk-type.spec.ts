import { OK_BUTTON, expect, gotoAdmin, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  confirmRowDelete,
  createRecord,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  searchTable,
  tableRow,
} from '../helpers/crud'

const MODAL = '.e2e-risk-type-modal'
const LIST_API = '/web/basic/risk-types'
const HINT = '请输入风险类型名称/编码'

/**
 * 选择「风险等级」。
 * 该 Select 未开启 showSearch（键盘输入不参与过滤），因此不能走 selectFormOption 的
 * 「输入关键字 + 回车」路径（回车只会重新选中当前值），改为直接点选下拉项。
 */
async function pickRiskLevel(
  page: import('@playwright/test').Page,
  level: 'high' | 'medium' | 'low'
): Promise<void> {
  const text = level === 'high' ? '高' : level === 'medium' ? '中' : '低'
  const item = page
    .locator(`${MODAL} .ant-form-item`)
    .filter({ has: page.locator('label:text-is("风险等级")') })
    .first()
  await item.locator('.ant-select').first().click()
  await page
    .locator('.ant-select-dropdown:visible .ant-select-item-option')
    .filter({ hasText: text })
    .first()
    .click()
  await expect(item.locator('.ant-select-selection-item')).toHaveText(text)
}

/** 新增风险类型：走真实 UI 交互（v0.1.0 已补齐 basic:risk-type:create，按钮对超管渲染） */
async function createRiskType(
  page: import('@playwright/test').Page,
  name: string,
  code: string,
  level: 'high' | 'medium' | 'low'
): Promise<'ui' | 'api'> {
  return createRecord(
    page,
    '新增风险类型',
    async () => {
      await page.getByRole('button', { name: '新增风险类型' }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增风险类型')
      await page.locator(MODAL).getByPlaceholder('如：未佩戴安全帽').fill(name)
      await page.locator(MODAL).getByPlaceholder('如：RISK_HELMET').fill(code)
      await pickRiskLevel(page, level)
      const resp = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
      expect([200, 201]).toContain((await resp).status())
      await expect(page.locator(MODAL)).toBeHidden()
    },
    () => apiCall(page, 'POST', LIST_API, { name, code, level, sequence: 99 })
  )
}

async function riskTypeIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const data = await expectApiOk(
    await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200&keyword=${encodeURIComponent(name)}`)
  )
  const hit = ((data.items ?? []) as { id: number; name: string }[]).find((i) => i.name === name)
  if (!hit) throw new Error(`未查询到风险类型：${name}`)
  return hit.id
}

async function deleteRiskType(page: import('@playwright/test').Page, name: string): Promise<void> {
  await searchTable(page, HINT, name, LIST_API)
  const row = tableRow(page, name)
  await expect(row).toHaveCount(1)
  if (await hasButton(page, /删\s*除/)) {
    await confirmRowDelete(page, row)
  } else {
    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${await riskTypeIdOf(page, name)}`))
  }
}

test.describe('02b 风险类型管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/basic/risk-type')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('风险类型管理')
  })

  test('列表渲染：列定义与种子数据完整（含等级中文）', async ({ page }) => {
    await expectColumns(page, ['风险类型', '风险编码', '风险等级', '描述', '创建时间', '操作'])
    const row = tableRow(page, '未佩戴安全帽')
    await expect(row).toHaveCount(1)
    await expect(row).toContainText('no_helmet')
    // level=high 渲染为中文「高」
    await expect(row.locator('td').nth(2)).toHaveText('高')
    await shot(page, '02b-risk-type-list')
  })

  test('新增风险类型（level=high）后列表出现该记录', async ({ page }) => {
    const name = uniqName('E2E风险类型')
    const code = `E2E_RISK_${Date.now()}`
    await createRiskType(page, name, code, 'high')

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(code)
    await expect(row.locator('td').nth(2)).toHaveText('高')

    await deleteRiskType(page, name)
  })

  test('编辑风险类型：等级由高改为低', async ({ page }) => {
    const name = uniqName('E2E改级')
    const code = `E2E_LVL_${Date.now()}`
    await createRiskType(page, name, code, 'high')
    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row.locator('td').nth(2)).toHaveText('高')

    // v0.1.0 已修复权限码不一致：basic:risk-type:update 已入菜单树，编辑按钮对超管渲染
    await expect(row.getByRole('button', { name: /编\s*辑/ })).toBeVisible()

    await row.getByRole('button', { name: /编\s*辑/ }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await expect(page.locator(MODAL).getByPlaceholder('如：未佩戴安全帽')).toHaveValue(name)
    await pickRiskLevel(page, 'low')
    const resp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
    )
    await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
    expect((await resp).status()).toBe(200)
    await expect(page.locator(MODAL)).toBeHidden()

    await searchTable(page, HINT, name, LIST_API)
    await expect(tableRow(page, name).locator('td').nth(2)).toHaveText('低')

    await deleteRiskType(page, name)
  })

  test('按关键字搜索命中；无匹配时为空态；重置恢复全量', async ({ page }) => {
    await searchTable(page, HINT, '未佩戴', LIST_API)
    await expect(tableRow(page, '未佩戴安全帽')).toHaveCount(1)

    await searchTable(page, HINT, `不存在_${Date.now()}`, LIST_API)
    await expectEmptyTable(page)

    const resetResp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: '重置' }).click()
    await resetResp
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThan(0)
  })

  test('删除风险类型后该记录消失', async ({ page }) => {
    const name = uniqName('E2E删风险')
    await createRiskType(page, name, `E2E_DEL_RISK_${Date.now()}`, 'medium')
    await searchTable(page, HINT, name, LIST_API)
    await expect(tableRow(page, name)).toHaveCount(1)

    await deleteRiskType(page, name)

    await searchTable(page, HINT, name, LIST_API)
    await expect(page.locator('.ant-table-row').filter({ hasText: name })).toHaveCount(0)
  })
})
