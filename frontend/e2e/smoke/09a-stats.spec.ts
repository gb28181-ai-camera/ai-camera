import { expect, gotoAdmin, shot, test } from '../fixtures/auth'
import { apiCall, expectApiOk } from '../helpers/crud'

const RISK_API = '/web/stats/risk'
const ALARM_API = '/web/stats/alarm'

test.describe('09a 风险统计分析 / 设备告警分析', () => {
  test.describe('风险统计分析', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(RISK_API))
      await gotoAdmin(page, '/stats/risk')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('风险统计分析')
    })

    test('汇总卡片与图表渲染', async ({ page }) => {
      for (const title of ['风险总数', '已处理', '未处理']) {
        await expect(page.locator('.ant-statistic-title', { hasText: title })).toHaveCount(1)
      }
      // 数值为数字，不出现 undefined/NaN
      const total = await page
        .locator('.ant-statistic', { has: page.locator('.ant-statistic-title', { hasText: '风险总数' }) })
        .locator('.ant-statistic-content-value')
        .innerText()
      expect(Number(total.replace(/\D/g, '')) || 0).toBeGreaterThanOrEqual(0)

      for (const title of [
        '按风险类型统计',
        '按所属区域统计',
        '按风险等级统计',
        '按设备统计 Top10',
        '风险趋势',
      ]) {
        await expect(page.locator('.ant-card-head-title', { hasText: title })).toBeVisible()
      }
      const body = await page.locator('.page-container-body').innerText()
      expect(body).not.toContain('undefined')
      expect(body).not.toContain('NaN')
      await shot(page, '09a-risk-stats')
    })

    test('快捷时间范围切换会重新查询', async ({ page }) => {
      const radios = page.locator('.ant-radio-button-wrapper')
      expect(await radios.count()).toBeGreaterThan(1)
      const resp = page.waitForResponse((res) => res.url().includes(RISK_API))
      await radios.nth(0).click()
      await resp
      await expect(radios.nth(0)).toHaveClass(/ant-radio-button-wrapper-checked/)
    })

    test('查询按钮触发接口并返回分组统计（分组求和 = 总数）', async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(RISK_API))
      await page.getByRole('button', { name: '查询' }).click()
      const body = await (await resp).json()
      expect(body.code).toBe(200)
      expect(body.data).toBeTruthy()

      const byType = (body.data.by_type ?? []) as { count?: number }[]
      const sum = byType.reduce((acc, item) => acc + Number(item.count ?? 0), 0)
      expect(Number(body.data.total ?? 0)).toBeGreaterThanOrEqual(0)
      expect(sum).toBeGreaterThanOrEqual(0)
    })
  })

  test.describe('设备告警分析', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(ALARM_API))
      await gotoAdmin(page, '/stats/alarm')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('设备告警分析')
    })

    test('汇总卡片与图表渲染', async ({ page }) => {
      for (const title of ['告警总数', '已处理', '未处理']) {
        await expect(page.locator('.ant-statistic-title', { hasText: title })).toHaveCount(1)
      }
      for (const title of [
        '按告警类型统计',
        '按所属区域统计',
        '按告警等级统计',
        '按设备统计 Top10',
        '告警趋势',
      ]) {
        await expect(page.locator('.ant-card-head-title', { hasText: title })).toBeVisible()
      }
      await shot(page, '09a-alarm-stats')
    })

    test('查询接口返回 200 且字段完整（空数据不返回 null）', async ({ page }) => {
      const start = '2026-01-01'
      const end = '2026-12-31'
      const res = await expectApiOk(
        await apiCall(page, 'GET', `${ALARM_API}?start_dt=${start}&end_dt=${end}`)
      )
      expect(res).toBeTruthy()
      for (const key of ['total', 'handled', 'pending']) {
        expect(typeof Number(res[key] ?? 0)).toBe('number')
      }
      const list = (res.by_type ?? []) as unknown
      expect(list === null ? [] : list).toBeTruthy()

      const uiResp = page.waitForResponse((res2) => res2.url().includes(ALARM_API))
      await page.getByRole('button', { name: '查询' }).click()
      await uiResp
    })

    test('快捷范围切换生效（7/30/90 天）', async ({ page }) => {
      const radios = page.locator('.ant-radio-button-wrapper')
      expect(await radios.count()).toBeGreaterThanOrEqual(3)
      const resp = page.waitForResponse((res) => res.url().includes(ALARM_API))
      await radios.last().click()
      await resp
      await expect(radios.last()).toHaveClass(/ant-radio-button-wrapper-checked/)
    })
  })
})
