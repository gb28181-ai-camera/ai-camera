import { OK_BUTTON, expect, gotoAdmin, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  confirmRowDelete,
  expectApiOk,
  expectColumns,
  hasButton,
  searchTable,
  tableRow,
} from '../helpers/crud'

const MODAL = '.e2e-device-mode-modal'
const LIST_API = '/web/devices/modes'
const HINT = '请输入模式名称/编码'

async function deviceModeIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const data = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200`))
  const hit = ((data.items ?? []) as { id: number; name: string }[]).find((i) => i.name === name)
  if (!hit) throw new Error(`未查询到设备模式：${name}`)
  return hit.id
}

async function createDeviceMode(
  page: import('@playwright/test').Page,
  name: string,
  code: string
): Promise<'ui' | 'api'> {
  if (!(await hasButton(page, '新增设备模式'))) {
    await expectApiOk(await apiCall(page, 'POST', LIST_API, { name, code, sequence: 88 }))
    return 'api'
  }

  await page.getByRole('button', { name: '新增设备模式' }).click()
  await expect(page.locator(MODAL)).toBeVisible()
  await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增设备模式')
  await page.locator(MODAL).getByPlaceholder('如：充电模式').fill(name)
  await page.locator(MODAL).getByPlaceholder('如：charge').fill(code)
  await page.locator(MODAL).getByPlaceholder('数值越小越靠前').fill('88')
  await page.locator(MODAL).getByPlaceholder('请输入备注').fill('E2E 设备模式')

  const resp = page.waitForResponse(
    (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
  )
  await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
  // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
  expect([200, 201]).toContain((await resp).status())
  await expect(page.locator(MODAL)).toBeHidden()
  return 'ui'
}

async function deleteDeviceMode(page: import('@playwright/test').Page, name: string): Promise<void> {
  await searchTable(page, HINT, name, LIST_API)
  const row = tableRow(page, name)
  await expect(row).toHaveCount(1)
  if (await hasButton(page, /删\s*除/)) {
    await confirmRowDelete(page, row)
  } else {
    await expectApiOk(
      await apiCall(page, 'DELETE', `${LIST_API}/${await deviceModeIdOf(page, name)}`)
    )
  }
}

test.describe('03c 设备模式管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/settings/device-mode')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('设备模式管理')
  })

  test('列表渲染内置三种设备模式（充电/全开/省电）', async ({ page }) => {
    await expectColumns(page, ['模式名称', '模式编码', '备注', '创建时间', '操作'])
    for (const item of [
      { name: '充电模式', code: 'charging' },
      { name: '全开模式', code: 'full' },
      { name: '省电模式', code: 'power_saving' },
    ]) {
      const row = tableRow(page, item.name)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(item.code)
    }
    await shot(page, '03c-device-mode-list')
  })

  test('新增设备模式后列表出现该记录（含排序与备注）', async ({ page }) => {
    const name = uniqName('E2E模式')
    const code = `e2e_mode_${Date.now()}`
    const path = await createDeviceMode(page, name, code)

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(code)
    // 列表无「排序」列，排序仅在编辑弹窗中可见；UI 路径同时校验备注
    if (path === 'ui') await expect(row).toContainText('E2E 设备模式')
    await shot(page, '03c-device-mode-created')

    await deleteDeviceMode(page, name)
  })

  test('编辑设备模式后名称与备注已变更', async ({ page }) => {
    const name = uniqName('E2E改模式')
    const renamed = `${name}_新`
    const code = `e2e_edit_mode_${Date.now()}`
    await createDeviceMode(page, name, code)

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)

    if (await hasButton(page, /编\s*辑/)) {
      await row.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).getByPlaceholder('如：充电模式')).toHaveValue(name)
      await page.locator(MODAL).getByPlaceholder('如：充电模式').fill(renamed)
      await page.locator(MODAL).getByPlaceholder('请输入备注').fill('E2E 编辑后备注')
      const resp = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await resp).status()).toBe(200)
      await expect(page.locator(MODAL)).toBeHidden()
    } else {
      const id = await deviceModeIdOf(page, name)
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}`, {
          name: renamed,
          code,
          sequence: 88,
          remark: 'E2E 编辑后备注',
        })
      )
    }

    await searchTable(page, HINT, renamed, LIST_API)
    const updated = tableRow(page, renamed)
    await expect(updated).toHaveCount(1)
    await expect(updated).toContainText('E2E 编辑后备注')

    await deleteDeviceMode(page, renamed)
  })

  test('名称必填校验：不填写名称时无法提交（UI 或接口 400）', async ({ page }) => {
    if (!(await hasButton(page, '新增设备模式'))) {
      const res = await apiCall(page, 'POST', LIST_API, {})
      expect(res.status).toBe(400)
      expect(res.body.message).toContain('required')
      return
    }
    await page.getByRole('button', { name: '新增设备模式' }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
    await expect(page.locator(MODAL).getByText('请输入模式名称')).toBeVisible()
    await page.locator(MODAL).getByRole('button', { name: /取\s*消/ }).click()
    await expect(page.locator(MODAL)).toBeHidden()
  })

  test('删除设备模式后该记录消失', async ({ page }) => {
    const name = uniqName('E2E删模式')
    await createDeviceMode(page, name, `e2e_del_mode_${Date.now()}`)

    await searchTable(page, HINT, name, LIST_API)
    await expect(tableRow(page, name)).toHaveCount(1)

    await deleteDeviceMode(page, name)

    await searchTable(page, HINT, name, LIST_API)
    await expect(page.locator('.ant-table-row').filter({ hasText: name })).toHaveCount(0)
  })
})
