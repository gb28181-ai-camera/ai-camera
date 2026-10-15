import { expect, gotoAdmin, readPermCodes, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  createRecord,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  searchTable,
  selectFormOption,
  tableRow,
} from '../helpers/crud'

const ALARM_MODAL = '.e2e-alarm-setting-modal'
const NOTIFY_MODAL = '.e2e-risk-notify-modal'
const ALARM_API = '/web/settings/alarm-settings'
const NOTIFY_API = '/web/settings/risk-notify-settings'

async function idsOf(page: import('@playwright/test').Page, api: string): Promise<number[]> {
  const data = await expectApiOk(await apiCall(page, 'GET', `${api}?page=1&page_size=200`))
  return (((data.items ?? []) as { id: number }[]) ?? []).map((i) => Number(i.id))
}

test.describe('08a 告警设置 / 风险通知设置', () => {
  test.describe('设备告警设置', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(ALARM_API))
      await gotoAdmin(page, '/settings/alarm')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('设备告警设置')
    })

    test('列表渲染：列定义与筛选控件', async ({ page }) => {
      await expectColumns(page, [
        '告警类型',
        '通知人员',
        '通知类型',
        '报警频率',
        '阈值',
        '状态',
        '操作',
      ])
      await expect(page.getByPlaceholder('请输入告警类型或通知人员')).toBeVisible()
      await shot(page, '08a-alarm-setting-list')
    })

    test('新增告警设置（关联告警类型 + 通知人员）后列表出现', async ({ page }) => {
      const path = await createRecord(
        page,
        '新增告警设置',
        async () => {
          await page.getByRole('button', { name: '新增告警设置' }).click()
          await expect(page.locator(ALARM_MODAL)).toBeVisible()
          await expect(page.locator(ALARM_MODAL).locator('.ant-modal-title')).toHaveText(
            '新增告警设置'
          )
          // 告警类型为必填项（v0.1.0 起按钮可渲染，UI 路径需要真实选择才能提交）
          await selectFormOption(page, page.locator(ALARM_MODAL), '告警类型', '离线告警')
          await page.locator(ALARM_MODAL).getByPlaceholder('如：20').fill('30')
          const resp = page.waitForResponse(
            (res) => res.url().includes(ALARM_API) && res.request().method() === 'POST'
          )
          await page.locator(ALARM_MODAL).getByRole('button', { name: /确\s*定/ }).click()
          // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
          expect([200, 201]).toContain((await resp).status())
          await expect(page.locator(ALARM_MODAL)).toBeHidden()
        },
        () =>
          apiCall(page, 'POST', ALARM_API, {
            alarm_type_id: 1,
            notify_type: 'app',
            notify_freq: 30,
            threshold: '30',
            active: true,
          })
      )

      // 新增后列表出现（需重新加载页面获取最新数据），等待首行渲染完成
      await page.reload()
      await expect(page.locator('.page-container-title').first()).toHaveText('设备告警设置')
      await expect(page.locator('.ant-table-row').first()).toBeVisible()
      await expect(tableRow(page, '离线告警')).toHaveCount(1)
      await shot(page, '08a-alarm-setting-created')

      // 清理
      for (const id of await idsOf(page, ALARM_API)) {
        await apiCall(page, 'DELETE', `${ALARM_API}/${id}`)
      }
      await page.reload()
      await expectEmptyTable(page)
    })

    test('关闭所有告警按钮对超管可见，点击后批量关闭全部告警', async ({ page }) => {
      await expectApiOk(
        await apiCall(page, 'POST', ALARM_API, {
          alarm_type_id: 2,
          notify_type: 'app',
          notify_freq: 60,
          active: true,
        })
      )

      // v0.1.0 已补齐 setting:alarm:disable-all → 「关闭所有告警」按钮对超管渲染
      const codes = await readPermCodes(page)
      expect(codes).toContain('setting:alarm:disable-all')

      const disableBtn = page.getByRole('button', { name: '关闭所有告警' })
      await expect(disableBtn).toBeVisible()
      await disableBtn.click()
      const pop = page.locator('.ant-popconfirm:visible')
      await expect(pop).toBeVisible()
      const disableResp = page.waitForResponse(
        (res) => /\/web\/settings\/alarm-settings\/disable-all$/.test(res.url()) && res.request().method() === 'PUT'
      )
      await pop.getByRole('button', { name: /确\s*定/ }).click()
      expect((await disableResp).status()).toBe(200)
      const items = ((await expectApiOk(
        await apiCall(page, 'GET', `${ALARM_API}?page=1&page_size=200`)
      )).items ?? []) as { active: boolean }[]
      for (const item of items) {
        expect(item.active).toBe(false)
      }

      for (const id of await idsOf(page, ALARM_API)) {
        await apiCall(page, 'DELETE', `${ALARM_API}/${id}`)
      }
    })

    test('告警设置 CRUD 接口闭环：创建 → 更新阈值 → 删除', async ({ page }) => {
      const created = await expectApiOk(
        await apiCall(page, 'POST', ALARM_API, {
          alarm_type_id: 3,
          notify_type: 'sms',
          notify_freq: 120,
          threshold: '500',
          remark: 'E2E 流量阈值',
          active: true,
        })
      )
      const id = Number(created.id)
      expect(Number(created.threshold)).toBe(500)

      await expectApiOk(
        await apiCall(page, 'PUT', `${ALARM_API}/${id}`, {
          alarm_type_id: 3,
          notify_type: 'sms',
          notify_freq: 60,
          threshold: '800',
          remark: 'E2E 流量阈值-改',
          active: false,
        })
      )
      const all = ((await expectApiOk(
        await apiCall(page, 'GET', `${ALARM_API}?page=1&page_size=200`)
      )).items ?? []) as { id: number; threshold?: string; notice_freq?: number; active: boolean }[]
      const hit = all.find((i) => Number(i.id) === id)
      expect(String(hit?.threshold)).toBe('800')
      expect(hit?.active).toBe(false)

      await expectApiOk(await apiCall(page, 'DELETE', `${ALARM_API}/${id}`))
    })
  })

  test.describe('风险通知设置', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(NOTIFY_API))
      await gotoAdmin(page, '/settings/risk-notify')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('风险通知设置')
    })

    test('列表渲染：列定义与新增入口', async ({ page }) => {
      await expectColumns(page, ['风险类型', '通知人员', '通知类型', '报警频率', '状态', '操作'])
      await expect(page.getByPlaceholder('请输入风险类型名称')).toBeVisible()
      await shot(page, '08a-risk-notify-list')
    })

    test('新增风险通知设置后列表出现该记录', async ({ page }) => {
      await createRecord(
        page,
        '新增风险通知',
        async () => {
          await page.getByRole('button', { name: '新增风险通知' }).click()
          await expect(page.locator(NOTIFY_MODAL)).toBeVisible()
          // 风险类型必填：先空提交校验提示
          await page.locator(NOTIFY_MODAL).getByRole('button', { name: /确\s*定/ }).click()
          await expect(page.locator(NOTIFY_MODAL).getByText('请选择风险类型')).toBeVisible()
          // v0.1.0 起按钮可渲染，UI 路径补全必填项后真实提交
          await selectFormOption(page, page.locator(NOTIFY_MODAL), '风险类型', '未佩戴安全帽')
          const resp = page.waitForResponse(
            (res) => res.url().includes(NOTIFY_API) && res.request().method() === 'POST'
          )
          await page.locator(NOTIFY_MODAL).getByRole('button', { name: /确\s*定/ }).click()
          expect([200, 201]).toContain((await resp).status())
          await expect(page.locator(NOTIFY_MODAL)).toBeHidden()
        },
        () =>
          apiCall(page, 'POST', NOTIFY_API, {
            risk_type_id: 1,
            notify_type: 'app',
            notify_freq: 30,
            active: true,
          })
      )

      const ids = await idsOf(page, NOTIFY_API)
      expect(ids.length).toBeGreaterThanOrEqual(1)

      await page.reload()
      await expect(page.locator('.page-container-title').first()).toHaveText('风险通知设置')
      if (await hasButton(page, '新增风险通知')) {
        await searchTable(page, '请输入风险类型名称', '未佩戴安全帽', NOTIFY_API)
        await expect(tableRow(page, '未佩戴安全帽')).toHaveCount(1)
        await shot(page, '08a-risk-notify-created')
      }

      for (const id of ids) {
        await apiCall(page, 'DELETE', `${NOTIFY_API}/${id}`)
      }
    })

    test('关闭所有通知按钮对超管可见，点击后批量关闭全部通知', async ({ page }) => {
      await expectApiOk(
        await apiCall(page, 'POST', NOTIFY_API, {
          risk_type_id: 1,
          notify_type: 'app',
          notify_freq: 60,
          active: true,
        })
      )

      // v0.1.0 已补齐 setting:risk-notify:disable-all → 按钮对超管渲染
      const codes = await readPermCodes(page)
      expect(codes).toContain('setting:risk-notify:disable-all')

      const disableBtn = page.getByRole('button', { name: '关闭所有通知' })
      await expect(disableBtn).toBeVisible()
      await disableBtn.click()
      const pop = page.locator('.ant-popconfirm:visible')
      await expect(pop).toBeVisible()
      const disableResp = page.waitForResponse(
        (res) =>
          /\/web\/settings\/risk-notify-settings\/disable-all$/.test(res.url()) &&
          res.request().method() === 'PUT'
      )
      await pop.getByRole('button', { name: /确\s*定/ }).click()
      expect((await disableResp).status()).toBe(200)
      const items = ((await expectApiOk(
        await apiCall(page, 'GET', `${NOTIFY_API}?page=1&page_size=200`)
      )).items ?? []) as { active: boolean }[]
      for (const item of items) {
        expect(item.active).toBe(false)
      }

      for (const id of await idsOf(page, NOTIFY_API)) {
        await apiCall(page, 'DELETE', `${NOTIFY_API}/${id}`)
      }
    })

    test('风险通知必填契约：缺少风险类型时接口 400', async ({ page }) => {
      const res = await apiCall(page, 'POST', NOTIFY_API, {})
      expect(res.status).toBe(400)
      expect(res.body.message).toContain('required')
      expect(await uniqName('x')).toContain('x')
    })
  })
})
