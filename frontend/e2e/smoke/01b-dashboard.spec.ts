import { collectConsole } from '../helpers/console'
import { expect, gotoAdmin, shot, test } from '../fixtures/auth'

test.describe('01b 首页看板', () => {
  test.beforeEach(async ({ page }) => {
    await gotoAdmin(page, '/dashboard')
    await expect(page.locator('.page-container-title').first()).toHaveText('首页看板')
  })

  test('统计卡片渲染完整（今日/本周/本月 预警与处理）', async ({ page }) => {
    const console = collectConsole(page)

    // 6 张预警/处理统计卡片
    for (const title of ['今日预警', '今日处理', '本周预警', '本周处理', '本月预警', '本月处理']) {
      await expect(
        page.locator('.dashboard-stat-row .ant-statistic-title', { hasText: title })
      ).toHaveCount(1)
    }

    // 设备统计三块
    await expect(page.locator('.ant-card-head-title', { hasText: '设备统计' })).toBeVisible()
    for (const title of ['设备总数', '在线设备', '离线设备']) {
      await expect(page.locator('.ant-statistic-title', { hasText: title })).toHaveCount(1)
    }

    // 数值必须是数字（不能渲染成 undefined / NaN）
    const deviceTotal = page
      .locator('.ant-col', { has: page.locator('.ant-statistic-title', { hasText: '设备总数' }) })
      .locator('.ant-statistic-content-value')
    await expect(deviceTotal).toHaveText(/^\d+$/)

    const body = await page.locator('.page-container-body').innerText()
    expect(body).not.toContain('undefined')
    expect(body).not.toContain('NaN')

    expect(console.texts()).toEqual([])
    await shot(page, '01b-dashboard-cards')
  })

  test('分析图表区域渲染（图表 canvas 或空状态）', async ({ page }) => {
    for (const title of [
      '风险类型分析（近 30 天）',
      '风险处理统计（近 30 天）',
      '区域风险统计（近 30 天）',
      '风险分析（近 7 天）',
      '告警分析（近 7 天）',
    ]) {
      await expect(page.locator('.ant-card-head-title', { hasText: title })).toBeVisible()
    }

    // 近 7 天风险/告警趋势恒有数据点 → 至少渲染出一个 echarts canvas
    await expect(page.locator('canvas').first()).toBeVisible()
    expect(await page.locator('canvas').count()).toBeGreaterThanOrEqual(1)
    await shot(page, '01b-dashboard-charts')
  })

  test('设备统计数值与统计接口一致（在线+离线=总数）', async ({ page }) => {
    const statValue = async (label: string) => {
      const text = await page
        .locator('.ant-col', { has: page.locator('.ant-statistic-title', { hasText: label }) })
        .locator('.ant-statistic-content-value')
        .innerText()
      return Number(text.replace(/\D/g, ''))
    }

    const total = await statValue('设备总数')
    const online = await statValue('在线设备')
    const offline = await statValue('离线设备')

    expect(online + offline).toBe(total)

    // 与后端 `/web/stats/dashboard` 的 device_stat 对齐
    const api = await page.evaluate(async () => {
      const token = window.localStorage.getItem('token')
      const res = await fetch('/web/stats/dashboard', {
        headers: { Authorization: `Bearer ${token}` },
      })
      return res.json()
    })
    expect(total).toBe(api.data.device_stat.total)
    expect(online).toBe(api.data.device_stat.online)
    expect(offline).toBe(api.data.device_stat.offline)
  })
})
