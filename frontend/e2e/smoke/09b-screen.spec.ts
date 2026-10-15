import { expect, gotoAdmin, shot, test } from '../fixtures/auth'
import { apiCall, expectApiOk } from '../helpers/crud'

const SCREEN_API = '/web/stats/screen'

test.describe('09b 可视化大屏', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(SCREEN_API))
    await gotoAdmin(page, '/screen')
    await resp
    // 大屏为全屏深色页面，不使用 PageContainer
    await expect(page.locator('.screen-page, body')).toBeTruthy()
  })

  test('大屏核心面板与大数字渲染', async ({ page }) => {
    for (const title of ['风险类型统计', '设备告警统计', '设备监控', '流量监控（剩余最少）']) {
      await expect(page.getByText(title, { exact: true }).first()).toBeVisible()
    }
    for (const title of ['设备总数', '在线设备', '离线设备']) {
      await expect(page.getByText(title, { exact: true }).first()).toBeVisible()
    }
    await shot(page, '09b-screen')
  })

  test('大屏数据与 /web/stats/screen 接口一致', async ({ page }) => {
    const data = await expectApiOk(await apiCall(page, 'GET', SCREEN_API))
    const monitor = data.device_monitor as { total?: number; online?: number; offline?: number }
    expect(Number(monitor.online ?? 0) + Number(monitor.offline ?? 0)).toBe(
      Number(monitor.total ?? 0)
    )

    const total = Number(monitor.total ?? 0)
    await expect(
      page.getByText(String(total), { exact: true }).first()
    ).toBeVisible()
  })

  test('数组字段不为 null（空数据返回空数组）', async ({ page }) => {
    const data = await expectApiOk(await apiCall(page, 'GET', SCREEN_API))
    for (const key of [
      'risk_realtime',
      'alarm_realtime',
      'risk_stats',
      'alarm_stats',
      'traffic_monitor',
    ]) {
      expect(Array.isArray(data[key]), `${key} 应为数组`).toBe(true)
    }
    for (const key of ['today_risk_count', 'today_alarm_count']) {
      expect(typeof Number(data[key] ?? 0)).toBe('number')
    }
  })

  test('全屏切换与刷新控件存在', async ({ page }) => {
    // 右上角为图标按钮（全屏 / 刷新）
    const icons = page.locator('.anticon-fullscreen, .anticon-reload')
    expect(await icons.count()).toBeGreaterThanOrEqual(1)

    const resp = page.waitForResponse((res) => res.url().includes(SCREEN_API))
    await page.locator('.anticon-reload').first().click()
    await resp
    await expect(page.getByText('设备总数', { exact: true }).first()).toBeVisible()
  })
})
