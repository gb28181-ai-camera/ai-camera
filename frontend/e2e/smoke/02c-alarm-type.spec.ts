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

const MODAL = '.e2e-alarm-type-modal'
const LIST_API = '/web/basic/alarm-types'
const HINT = '请输入告警名称/编码'

/** 种子内置告警类型（与 initdb 数据一致） */
const BUILTIN = [
  { name: '离线告警', code: 'offline' },
  { name: '电量告警', code: 'battery' },
  { name: '流量告警', code: 'traffic' },
]

async function createAlarmType(
  page: import('@playwright/test').Page,
  name: string,
  code: string
): Promise<'ui' | 'api'> {
  return createRecord(
    page,
    '新增告警类型',
    async () => {
      await page.getByRole('button', { name: '新增告警类型' }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await page.locator(MODAL).getByPlaceholder('如：离线告警').fill(name)
      await page.locator(MODAL).getByPlaceholder('如：offline').fill(code)
      const resp = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
      expect([200, 201]).toContain((await resp).status())
      await expect(page.locator(MODAL)).toBeHidden()
    },
    () => apiCall(page, 'POST', LIST_API, { name, code, level: 'medium', sequence: 99 })
  )
}

async function alarmTypeIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const data = await expectApiOk(
    await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200&keyword=${encodeURIComponent(name)}`)
  )
  const hit = ((data.items ?? []) as { id: number; name: string }[]).find((i) => i.name === name)
  if (!hit) throw new Error(`未查询到告警类型：${name}`)
  return hit.id
}

async function deleteAlarmType(page: import('@playwright/test').Page, name: string): Promise<void> {
  await searchTable(page, HINT, name, LIST_API)
  const row = tableRow(page, name)
  await expect(row).toHaveCount(1)
  if (await hasButton(page, /删\s*除/)) {
    await confirmRowDelete(page, row)
  } else {
    await expectApiOk(
      await apiCall(page, 'DELETE', `${LIST_API}/${await alarmTypeIdOf(page, name)}`)
    )
  }
}

test.describe('02c 告警类型管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/basic/alarm-type')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('告警类型管理')
  })

  test('列表含内置三条告警类型（离线/电量/流量）', async ({ page }) => {
    await expectColumns(page, ['告警名称', '告警编码', '告警等级', '描述', '创建时间', '操作'])
    for (const item of BUILTIN) {
      const row = tableRow(page, item.name)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(item.code)
    }
    await shot(page, '02c-alarm-type-list')
  })

  test('新增告警类型后列表出现该记录', async ({ page }) => {
    const name = uniqName('E2E告警类型')
    const code = `e2e_alarm_${Date.now()}`
    await createAlarmType(page, name, code)

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(code)

    await deleteAlarmType(page, name)
  })

  test('编辑告警类型后名称与描述已变更', async ({ page }) => {
    const name = uniqName('E2E改告警')
    const renamed = `${name}_新`
    const code = `e2e_edit_alarm_${Date.now()}`
    await createAlarmType(page, name, code)
    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)

    if (await hasButton(page, /编\s*辑/)) {
      await row.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).getByPlaceholder('如：离线告警')).toHaveValue(name)
      await page.locator(MODAL).getByPlaceholder('如：离线告警').fill(renamed)
      await page.locator(MODAL).getByPlaceholder('请输入告警描述').fill('E2E 编辑告警描述')
      const resp = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await resp).status()).toBe(200)
      await expect(page.locator(MODAL)).toBeHidden()
    } else {
      const id = await alarmTypeIdOf(page, name)
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}`, {
          name: renamed,
          code,
          level: 'medium',
          sequence: 99,
          description: 'E2E 编辑告警描述',
        })
      )
    }

    await searchTable(page, HINT, code, LIST_API)
    const updated = tableRow(page, code)
    await expect(updated).toHaveCount(1)
    await expect(updated.locator('td').nth(0)).toHaveText(renamed)
    await expect(updated).toContainText('E2E 编辑告警描述')

    await deleteAlarmType(page, renamed)
  })

  test('按编码搜索命中唯一记录，重置恢复全量', async ({ page }) => {
    await searchTable(page, HINT, 'offline', LIST_API)
    await expect(page.locator('.ant-table-row')).toHaveCount(1)
    await expect(tableRow(page, '离线告警')).toHaveCount(1)

    await searchTable(page, HINT, `nope_${Date.now()}`, LIST_API)
    await expectEmptyTable(page)

    const resetResp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: '重置' }).click()
    await resetResp
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThanOrEqual(3)
  })

  test('删除告警类型后该记录消失', async ({ page }) => {
    const name = uniqName('E2E删告警')
    await createAlarmType(page, name, `e2e_del_alarm_${Date.now()}`)
    await searchTable(page, HINT, name, LIST_API)
    await expect(tableRow(page, name)).toHaveCount(1)

    await deleteAlarmType(page, name)

    await searchTable(page, HINT, name, LIST_API)
    await expect(page.locator('.ant-table-row').filter({ hasText: name })).toHaveCount(0)
  })
})
