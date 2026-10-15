import { CANCEL_BUTTON, expect, gotoAdmin, readPermCodes, shot, test } from '../fixtures/auth'
import { apiCall, expectApiOk, expectColumns, searchTable, tableRow } from '../helpers/crud'

const DEVICE_API = '/web/devices'
const PLAN_API = '/web/upgrades/plans'
const RECORD_API = '/web/upgrades/records'
const UPGRADE_MODAL = '.e2e-device-upgrade-modal'
const HINT = '请输入设备名称或设备编号'

test.describe('07b 设备升级（批量升级 / 升级计划 / 升级记录）', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(DEVICE_API))
    await gotoAdmin(page, '/settings/upgrade')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('设备升级管理')
  })

  test('三个 Tab 渲染：可升级设备 / 升级计划 / 升级记录', async ({ page }) => {
    const tabs = page.locator('.ant-tabs-tab')
    await expect(tabs).toHaveCount(3)
    await expect(tabs.nth(0)).toHaveText('可升级设备')
    await expect(tabs.nth(1)).toHaveText('升级计划')
    await expect(tabs.nth(2)).toHaveText('升级记录')
    await shot(page, '07b-upgrade-tabs')
  })

  test('可升级设备列表渲染（含所属区域名称与状态）', async ({ page }) => {
    await expectColumns(page.locator('.ant-tabs-tabpane-active'), [
      '设备名称',
      '设备编号',
      '所属区域',
      '设备位置',
      '状态',
      '操作',
    ])
    await expect(page.getByPlaceholder(HINT)).toBeVisible()
    // 区域筛选为 RegionSelect（下拉框，非输入框）
    await expect(page.getByText('所属区域').first()).toBeVisible()

    const row = tableRow(page, '东门摄像头')
    await expect(row).toHaveCount(1)
    await expect(row).toContainText('CAM_001')
    await expect(row).toContainText('一号厂区')
    await shot(page, '07b-upgrade-devices')
  })

  test('升级计划 Tab 渲染计划列表（含冗余名称列）', async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(PLAN_API))
    await page.locator('.ant-tabs-tab').nth(1).click()
    await resp

    await expectColumns(page.locator('.ant-tabs-tabpane-active'), [
      '设备名称',
      '设备编号',
      '升级固件',
      '升级模型',
      '升级时间',
      '状态',
      '操作',
    ])
    await shot(page, '07b-upgrade-plans')
  })

  test('升级记录 Tab 渲染记录列表（含上一版本与流量卡列）', async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(RECORD_API))
    await page.locator('.ant-tabs-tab').nth(2).click()
    await resp

    await expectColumns(page.locator('.ant-tabs-tabpane-active'), [
      '设备名称',
      '设备编号',
      '所属区域',
      '设备位置',
      '流量卡',
      '上一固件',
      '上一模型',
      '升级固件',
      '升级模型',
      '升级时间',
      '结果',
    ])
    await shot(page, '07b-upgrade-records')
  })

  test('升级下发接口契约：缺少设备时返回明确错误（不产生脏数据）', async ({ page }) => {
    const before = await apiCall(page, 'GET', `${RECORD_API}?page=1&page_size=200`)
    const beforeTotal = Number((before.body.data as { total?: number })?.total ?? 0)

    const res = await apiCall(page, 'POST', '/web/upgrades/devices', {
      device_ids: [],
      upgrade_type: 'now',
    })
    expect(res.body.code).not.toBe(200)

    const after = await apiCall(page, 'GET', `${RECORD_API}?page=1&page_size=200`)
    const afterTotal = Number((after.body.data as { total?: number })?.total ?? 0)
    expect(afterTotal).toBe(beforeTotal)
  })

  /**
   * 「升级」按钮对超管可用。
   *
   * 历史背景：页面用 `upgrade:issue` 判定按钮是否可点击，而菜单树中缺少该权限码 →
   * 行内「升级」按钮始终 disabled。v0.1.0 已补齐 `upgrade:issue` 按钮菜单行。
   */
  test('升级按钮对超管可用并能打开批量升级弹窗', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('upgrade:issue')

    const row = tableRow(page, '东门摄像头')
    await expect(row).toHaveCount(1)
    const btn = row.getByRole('button', { name: '升级' })
    await expect(btn).toBeVisible()
    await expect(btn).toBeEnabled()

    // 点击后打开批量升级弹窗
    await btn.click()
    const modal = page.locator(UPGRADE_MODAL)
    await expect(modal).toBeVisible()
    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()
  })

  test('升级记录与升级计划的接口均可访问（数据链路可用）', async ({ page }) => {
    const plans = await apiCall(page, 'GET', `${PLAN_API}?page=1&page_size=20`)
    expect(plans.status).toBe(200)
    expect(plans.body.code).toBe(200)

    const records = await expectApiOk(
      await apiCall(page, 'GET', `${RECORD_API}?page=1&page_size=20`)
    )
    expect(Array.isArray(records.items ?? [])).toBe(true)

    await searchTable(page, HINT, '东门摄像头', DEVICE_API)
    await expect(tableRow(page, '东门摄像头')).toHaveCount(1)
  })
})
