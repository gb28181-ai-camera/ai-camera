import { OK_BUTTON, expect, gotoAdmin, readPermCodes, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  confirmRowDelete,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  searchTable,
  selectFormOption,
  tableRow,
} from '../helpers/crud'

const MODAL = '.e2e-device-modal'
const LIST_API = '/web/devices'
const HINT = '请输入设备名称或设备编号'
/** 种子区域（initdb）：一号厂区 */
const SEED_REGION = '一号厂区'

async function deviceIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const data = await expectApiOk(
    await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200&keyword=${encodeURIComponent(name)}`)
  )
  const hit = ((data.items ?? []) as { id: number; name: string }[]).find((i) => i.name === name)
  if (!hit) throw new Error(`未查询到设备：${name}`)
  return hit.id
}

/** 新增设备：走真实 UI 交互（v0.1.0 已补齐 device:create，按钮对超管渲染） */
async function createDevice(
  page: import('@playwright/test').Page,
  name: string,
  code: string
): Promise<'ui' | 'api'> {
  if (!(await hasButton(page, '新增设备'))) {
    await expectApiOk(
      await apiCall(page, 'POST', LIST_API, { name, device_code: code, region_id: 1 })
    )
    return 'api'
  }

  await page.getByRole('button', { name: '新增设备' }).click()
  await expect(page.locator(MODAL)).toBeVisible()
  await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增设备')
  await page.locator(MODAL).getByPlaceholder('如：1号摄像头').fill(name)
  await page.locator(MODAL).getByPlaceholder('如：CAM001').fill(code)
  await selectFormOption(page, page.locator(MODAL), '所属区域', SEED_REGION)
  await page.locator(MODAL).getByPlaceholder('如：东门入口').fill('E2E 测试点位')

  const resp = page.waitForResponse(
    (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
  )
  await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
  // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
  expect([200, 201]).toContain((await resp).status())
  await expect(page.locator(MODAL)).toBeHidden()
  return 'ui'
}

async function deleteDevice(page: import('@playwright/test').Page, name: string): Promise<void> {
  await searchTable(page, HINT, name, LIST_API)
  const row = tableRow(page, name)
  await expect(row).toHaveCount(1)
  if (await hasButton(page, /删\s*除/)) {
    await confirmRowDelete(page, row)
  } else {
    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${await deviceIdOf(page, name)}`))
  }
}

test.describe('03a 设备管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/settings/device')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('设备管理')
  })

  test('列表渲染：关联区域显示名称而非 ID', async ({ page }) => {
    await expectColumns(page, [
      '设备名称',
      '设备编号',
      '所属区域',
      '设备位置',
      '模型',
      '系统版本',
      '设备工作模式',
      '设备模式',
      '信息间隔',
      '状态',
      '设备监控',
      '操作',
    ])

    const row = tableRow(page, '东门摄像头')
    await expect(row).toHaveCount(1)
    await expect(row).toContainText('CAM_001')
    // 关联表必须展示名称列（CLAUDE.md 规范）
    await expect(row).toContainText(SEED_REGION)
    await shot(page, '03a-device-list')
  })

  test('新增设备后列表出现且区域列显示区域名称', async ({ page }) => {
    const name = uniqName('E2E设备')
    const code = `E2E_A_${Date.now()}`
    const path = await createDevice(page, name, code)

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(code)
    await expect(row).toContainText(SEED_REGION)
    await expect(row).toContainText('E2E 测试点位')

    await deleteDevice(page, name)
  })

  test('编辑设备后名称与位置已变更', async ({ page }) => {
    const name = uniqName('E2E改设备')
    const renamed = `${name}_新`
    const code = `E2E_B_${Date.now()}`
    await createDevice(page, name, code)
    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)

    // v0.1.0 已修复权限码不一致：device:update 已入菜单树，编辑按钮对超管渲染
    await expect(row.getByRole('button', { name: /编\s*辑/ })).toBeVisible()

    await row.getByRole('button', { name: /编\s*辑/ }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('编辑设备')
    await expect(page.locator(MODAL).getByPlaceholder('如：1号摄像头')).toHaveValue(name)
    await expect(page.locator(MODAL).getByPlaceholder('如：CAM001')).toHaveValue(code)
    await page.locator(MODAL).getByPlaceholder('如：1号摄像头').fill(renamed)
    await page.locator(MODAL).getByPlaceholder('如：东门入口').fill('E2E 编辑后位置')
    const resp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
    )
    await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
    expect((await resp).status()).toBe(200)
    await expect(page.locator(MODAL)).toBeHidden()

    await searchTable(page, HINT, code, LIST_API)
    const updated = tableRow(page, code)
    await expect(updated).toHaveCount(1)
    await expect(updated).toContainText(renamed)
    await expect(updated).toContainText('E2E 编辑后位置')

    await deleteDevice(page, renamed)
  })

  test('设备监控开关切换后状态翻转（设备监控列）', async ({ page }) => {
    await searchTable(page, HINT, '东门摄像头', LIST_API)
    const row = tableRow(page, '东门摄像头')
    await expect(row).toHaveCount(1)

    const monitorSwitch = row.locator('.ant-switch')
    await expect(monitorSwitch).toBeEnabled()
    const before = await monitorSwitch.getAttribute('aria-checked')

    const toggleResp = page.waitForResponse(
      (res) => /\/web\/devices\/\d+\/monitor$/.test(res.url()) && res.request().method() === 'PUT'
    )
    await monitorSwitch.click()
    const resp = await toggleResp
    expect(resp.status()).toBe(200)
    expect((await resp.json()).code).toBe(200)

    await expect(monitorSwitch).not.toHaveAttribute('aria-checked', before ?? 'false')

    // 复原：再次点击切回原状态
    const revert = page.waitForResponse(
      (res) => /\/web\/devices\/\d+\/monitor$/.test(res.url()) && res.request().method() === 'PUT'
    )
    await monitorSwitch.click()
    await revert
    await expect(monitorSwitch).toHaveAttribute('aria-checked', before ?? 'false')
  })

  test('同步时间按钮调用同步接口并提示成功', async ({ page }) => {
    await searchTable(page, HINT, '东门摄像头', LIST_API)
    const row = tableRow(page, '东门摄像头')
    await expect(row).toHaveCount(1)

    const syncResp = page.waitForResponse(
      (res) => /\/web\/devices\/\d+\/sync-time$/.test(res.url()) && res.request().method() === 'PUT'
    )
    await row.getByRole('button', { name: '同步时间' }).click()
    const resp = await syncResp
    expect(resp.status()).toBe(200)
    const body = await resp.json()
    expect(body.code).toBe(200)
    expect(String(body.data?.sync_time ?? '')).not.toBe('')
    await expect(page.locator('.ant-message')).toContainText('时间同步成功')
  })

  test('搜索命中 / 无匹配空态 / 重置恢复', async ({ page }) => {
    await searchTable(page, HINT, '东门摄像头', LIST_API)
    await expect(page.locator('.ant-table-row')).toHaveCount(1)

    await searchTable(page, HINT, `没有这个设备_${Date.now()}`, LIST_API)
    await expectEmptyTable(page)

    const resetResp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: '重置' }).click()
    await resetResp
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThan(0)
  })

  /**
   * 「切换模式」按钮对超管可见并可点击。
   *
   * 历史背景：页面用 `device:switch-mode` 判定该按钮，而菜单树只提供 `device:mode`，
   * 两者不一致导致按钮不渲染。v0.1.0 已补齐 `device:switch-mode` 按钮菜单行。
   */
  test('切换模式按钮对超管可见且可点击', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('device:switch-mode')

    await searchTable(page, HINT, '东门摄像头', LIST_API)
    const row = tableRow(page, '东门摄像头')
    await expect(row).toHaveCount(1)
    const btn = row.getByRole('button', { name: '切换模式' })
    await expect(btn).toBeVisible()
    await expect(btn).toBeEnabled()

    // 点击后弹出模式切换弹窗（设备管理页的切换模式弹窗 class）
    await btn.click()
    const modeModal = page.locator('.e2e-device-mode-switch-modal')
    await expect(modeModal).toBeVisible()
    await modeModal.getByRole('button', { name: /取\s*消/ }).click()
    await expect(modeModal).toBeHidden()
  })
})
