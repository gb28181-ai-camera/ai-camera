import { expect, gotoAdmin, shot, test } from '../fixtures/auth'
import {
  apiCall,
  createApiKey,
  deleteApiKey,
  expectApiOk,
  expectColumns,
  reportAlarm,
  searchTable,
} from '../helpers/crud'

const LIST_API = '/web/alarms'
const DETAIL_MODAL = '.e2e-alarm-detail-modal'
const HANDLE_MODAL = '.e2e-alarm-handle-modal'
const SEED_DEVICE = '东门摄像头'
const SEED_DEVICE_CODE = 'CAM_001'

let apiKeyId = 0
let createdAlarmIds: number[] = []

test.describe('05b 设备告警', () => {
  test.beforeEach(async ({ page }) => {
    const key = await createApiKey(page, `e2e-alarm-${Date.now()}`)
    apiKeyId = key.id
    createdAlarmIds = []

    const offline = await reportAlarm(page, key.key, SEED_DEVICE_CODE, 'offline', {
      content: 'E2E 设备离线',
      alarm_level: 'high',
    })
    const battery = await reportAlarm(page, key.key, SEED_DEVICE_CODE, 'battery', {
      content: 'E2E 电量不足',
      alarm_level: 'medium',
    })
    createdAlarmIds = [Number(offline.id), Number(battery.id)]

    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/alarm/list')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('设备告警')
  })

  test.afterEach(async ({ page }) => {
    for (const id of createdAlarmIds) {
      await apiCall(page, 'DELETE', `${LIST_API}/${id}`)
    }
    if (apiKeyId) await deleteApiKey(page, apiKeyId).catch(() => undefined)
    apiKeyId = 0
  })

  test('列表渲染：列定义完整且上报告警可见（关联名称非 ID）', async ({ page }) => {
    await expectColumns(page, [
      '告警类型',
      '所属区域',
      '设备名称',
      '设备编号',
      '设备位置',
      '告警内容',
      '告警等级',
      '状态',
      '消息通知',
      '告警时间',
      '处理人',
      '处理时间',
      '操作',
    ])

    await searchTable(page, '请输入设备编号', SEED_DEVICE_CODE, LIST_API)
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThanOrEqual(2)

    const row = page.locator('.ant-table-row').first()
    await expect(row).toContainText(SEED_DEVICE)
    await expect(row).toContainText('一号厂区')
    await expect(row).toContainText('待处理')
    await shot(page, '05b-alarm-list')
  })

  test('告警列表支持按类型/内容检索（离线告警命中）', async ({ page }) => {
    await searchTable(page, '请输入设备名称', SEED_DEVICE, LIST_API)
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThanOrEqual(2)

    // 接口层筛选：告警类型 offline
    const offline = await apiCall(
      page,
      'GET',
      `${LIST_API}?page=1&page_size=20&alarm_type_id=1&status=pending`
    )
    const items = ((offline.body.data as { items?: { content: string }[] }).items ?? [])
    expect(items.some((i) => String(i.content).includes('E2E'))).toBe(true)

    const miss = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=20&device_code=NOPE_${Date.now()}`)
    expect(((miss.body.data as { items?: unknown[] }).items ?? []).length).toBe(0)
  })

  test('查看详情弹窗展示完整字段并转为已读', async ({ page }) => {
    await searchTable(page, '请输入设备编号', SEED_DEVICE_CODE, LIST_API)
    const row = page.locator('.ant-table-row').first()
    await expect(row).toContainText('待处理')

    // 页面「查看」不请求详情接口，仅调用已读接口 → 从该请求的 URL 取出被打开记录的 id
    const readResp = page.waitForResponse((res) =>
      new RegExp(`${LIST_API}/\\d+/read$`).test(res.url())
    )
    await row.locator('button').filter({ hasText: /^查\s*看$/ }).first().click()
    const detailId = Number((await readResp).url().match(/\/(\d+)\/read$/)?.[1])
    expect(detailId).toBeGreaterThan(0)

    await expect(page.locator(DETAIL_MODAL)).toBeVisible()
    const modal = page.locator(DETAIL_MODAL)
    await expect(modal.locator('.ant-modal-title')).toContainText('告警详情')
    for (const label of ['告警类型', '告警等级', '设备名称', '设备编号', '所属区域', '告警时间', '状态']) {
      await expect(modal.getByText(label, { exact: true })).toBeVisible()
    }
    await expect(modal.getByText(SEED_DEVICE, { exact: true })).toBeVisible()
    await shot(page, '05b-alarm-detail')

    await modal.getByRole('button', { name: /关\s*闭/ }).click()
    await expect(modal).toBeHidden()

    // 查看详情后页面会调用 markRead 接口（异步），轮询确认已读
    await expect
      .poll(
        async () => {
          const readBack = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${detailId}`))
          return readBack.read
        },
        { timeout: 5_000 }
      )
      .toBe(true)
    const after = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${detailId}`))
    expect(after.status).toBe('pending')
  })

  test('处理告警：填写处理情况后状态转为已处理', async ({ page }) => {
    const alarmId = createdAlarmIds[0]
    await searchTable(page, '请输入设备编号', SEED_DEVICE_CODE, LIST_API)

    const row = page.locator('.ant-table-row').filter({ hasText: '待处理' }).first()
    await expect(row).toBeVisible()
    await row.getByRole('button', { name: /处\s*理/ }).click()

    await expect(page.locator(HANDLE_MODAL)).toBeVisible()
    const modal = page.locator(HANDLE_MODAL)
    await expect(modal.locator('.ant-modal-title')).toContainText('处理告警')
    await modal.getByRole('button', { name: /确\s*定/ }).click()
    await expect(modal.getByText('请填写处理结果')).toBeVisible()

    await modal
      .getByPlaceholder('请填写处理情况，如已更换电池并重启设备')
      .fill('E2E 已重启设备')
    const resp = page.waitForResponse(
      (res) => res.url().includes('/handle') && res.request().method() === 'PUT'
    )
    await modal.getByRole('button', { name: /确\s*定/ }).click()
    expect((await resp).status()).toBe(200)
    await expect(modal).toBeHidden()

    // UI 处理后，被点击的那条（createdAlarmIds 中之一）应转为已处理
    const statuses = await Promise.all(
      createdAlarmIds.map(async (id) => {
        const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
        return String(detail.status)
      })
    )
    expect(statuses).toContain('handled')
    expect(alarmId).toBeGreaterThan(0)
    await shot(page, '05b-alarm-handled')
  })

  test('处理接口契约：已处理告警再次处理返回明确错误', async ({ page }) => {
    const alarmId = createdAlarmIds[1]
    await expectApiOk(
      await apiCall(page, 'PUT', `${LIST_API}/${alarmId}/handle`, { handle_result: 'E2E 首次处理' })
    )
    const again = await apiCall(page, 'PUT', `${LIST_API}/${alarmId}/handle`, {
      handle_result: 'E2E 重复处理',
    })
    expect(again.body.code).not.toBe(200)
    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${alarmId}`))
    expect(detail.status).toBe('handled')
  })
})
