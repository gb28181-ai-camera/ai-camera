import { expect, gotoAdmin, shot, test } from '../fixtures/auth'
import {
  apiCall,
  createApiKey,
  deleteApiKey,
  expectApiOk,
  expectColumns,
  hasButton,
  reportRisk,
  searchTable,
  tableRow,
} from '../helpers/crud'

const LIST_API = '/web/risks'
const DETAIL_MODAL = '.e2e-risk-detail-modal'
const HANDLE_MODAL = '.e2e-risk-handle-modal'
const HINT = '请输入设备名称'
/** 种子设备（initdb）：东门摄像头 / CAM_001 */
const SEED_DEVICE = '东门摄像头'
const SEED_DEVICE_CODE = 'CAM_001'
const SEED_RISK_TYPE = '未佩戴安全帽'

let apiKeyId = 0
let apiKey = ''
let createdRiskIds: number[] = []

test.describe('05a 风险监控', () => {
  test.beforeEach(async ({ page }) => {
    // 通过设备上报接口准备真实风险数据（Web 端无「新建风险」入口）
    const key = await createApiKey(page, `e2e-risk-${Date.now()}`)
    apiKeyId = key.id
    apiKey = key.key
    createdRiskIds = []

    const first = await reportRisk(page, apiKey, SEED_DEVICE_CODE, 'no_helmet')
    const second = await reportRisk(page, apiKey, SEED_DEVICE_CODE, 'no_helmet', {
      image_url: '/uploads/e2e-risk-2.png',
      detect_score: 0.55,
    })
    createdRiskIds = [Number(first.id), Number(second.id)]

    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/risk/monitor')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('风险监控')
  })

  test.afterEach(async ({ page }) => {
    for (const id of createdRiskIds) {
      await apiCall(page, 'DELETE', `${LIST_API}/${id}`)
    }
    if (apiKeyId) await deleteApiKey(page, apiKeyId).catch(() => undefined)
    apiKeyId = 0
  })

  test('列表渲染：列定义完整且上报的风险可见（关联名称非 ID）', async ({ page }) => {
    await expectColumns(page, [
      '风险类型',
      '风险图片',
      '设备名称',
      '设备编号',
      '所属区域',
      '发生时间',
      '检测准确',
      '消息通知',
      '状态',
      '操作',
    ])

    await searchTable(page, HINT, SEED_DEVICE, LIST_API)
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThanOrEqual(2)

    const row = page.locator('.ant-table-row').first()
    await expect(row).toContainText(SEED_RISK_TYPE)
    await expect(row).toContainText(SEED_DEVICE)
    await expect(row).toContainText(SEED_DEVICE_CODE)
    // 冗余字段：所属区域名称
    await expect(row).toContainText('一号厂区')
    // 新上报风险状态为「待处理」
    await expect(row).toContainText('待处理')
    await shot(page, '05a-risk-list')
  })

  test('查看详情弹窗展示完整字段并转为已读', async ({ page }) => {
    await searchTable(page, HINT, SEED_DEVICE, LIST_API)
    const row = page.locator('.ant-table-row').first()
    await expect(row).toContainText('待处理')

    // 页面「查看」不请求详情接口，仅调用已读接口 → 从该请求的 URL 取出被打开记录的 id
    const readResp = page.waitForResponse((res) =>
      new RegExp(`${LIST_API}/\\d+/read$`).test(res.url())
    )
    await row.locator('button').filter({ hasText: /^查\s*看$/ }).first().click()
    const detailId = Number((await readResp).url().match(/\/(\d+)\/read$/)?.[1])
    expect(detailId).toBeGreaterThan(0)
    createdRiskIds.push(detailId)

    await expect(page.locator(DETAIL_MODAL)).toBeVisible()
    const modal = page.locator(DETAIL_MODAL)
    await expect(modal.locator('.ant-modal-title')).toContainText('风险详情')
    for (const label of ['风险类型', '风险等级', '设备名称', '设备编号', '所属区域', '发生时间', '状态']) {
      await expect(modal.getByText(label, { exact: true })).toBeVisible()
    }
    await expect(modal.getByText(SEED_RISK_TYPE)).toBeVisible()
    await expect(modal.getByText(SEED_DEVICE, { exact: true })).toBeVisible()
    await shot(page, '05a-risk-detail')

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

  test('处理风险：填写处理结果后行状态转为已处理', async ({ page }) => {
    // 状态机要求：先确认「检测准确 = 是」才允许处理（两条风险都需确认，否则首行可能被拒）
    for (const id of createdRiskIds) {
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}/accurate`, { accurate: 'yes' })
      )
    }
    await searchTable(page, HINT, SEED_DEVICE, LIST_API)
    const row = tableRow(page, SEED_DEVICE_CODE).filter({ hasText: '待处理' }).first()
    await expect(row).toHaveCount(1)

    const handleBtn = row.locator('button').filter({ hasText: /^处\s*理$/ }).first()
    await expect(handleBtn).toBeVisible()
    await handleBtn.click()

    await expect(page.locator(HANDLE_MODAL)).toBeVisible()
    const modal = page.locator(HANDLE_MODAL)
    await expect(modal.locator('.ant-modal-title')).toContainText('处理风险')
    // 处理结果必填
    await modal.getByRole('button', { name: /确\s*定/ }).click()
    await expect(modal.getByText('请填写处理结果')).toBeVisible()

    await modal.getByPlaceholder('请填写现场处理情况').fill('E2E 已现场清理')
    const resp = page.waitForResponse(
      (res) => res.url().includes('/handle') && res.request().method() === 'PUT'
    )
    await modal.getByRole('button', { name: /确\s*定/ }).click()
    expect((await resp).status()).toBe(200)
    await expect(modal).toBeHidden()

    // 处理后：「待处理」行数减少，且出现「已处理」行
    await searchTable(page, HINT, SEED_DEVICE, LIST_API)
    await expect(page.locator('.ant-table-row').filter({ hasText: '已处理' }).first()).toBeVisible()
    await shot(page, '05a-risk-handled')
  })

  test('检测准确=否（误报）后状态直接转已处理', async ({ page }) => {
    const riskId = createdRiskIds[0]
    await expectApiOk(
      await apiCall(page, 'PUT', `${LIST_API}/${riskId}/accurate`, { accurate: 'no' })
    )

    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${riskId}`))
    expect(detail.status).toBe('handled')
    expect(detail.read).toBe(true)
    // 误报跳过处理，处理人保持为空
    expect(detail.handler ?? '').toBe('')
  })

  test('状态筛选：待处理列表不含已处理记录', async ({ page }) => {
    // 先确认检测准确再处理（状态机要求）
    const riskId = createdRiskIds[0]
    for (const id of createdRiskIds) {
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}/accurate`, { accurate: 'yes' })
      )
    }
    await expectApiOk(
      await apiCall(page, 'PUT', `${LIST_API}/${riskId}/handle`, {
        handle_result: 'E2E 筛选用处理',
      })
    )

    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${riskId}`))
    expect(detail.status).toBe('handled')

    const pending = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=20&status=pending`)
    const pendingItems = ((pending.body.data as { items?: { id: number }[] }).items ?? [])
    expect(pendingItems.find((i) => i.id === riskId)).toBeUndefined()

    // 页面列表仍可正常检索（状态筛选为待处理时不包含该记录）
    await searchTable(page, HINT, SEED_DEVICE, LIST_API)
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThanOrEqual(1)
    await expect(page.getByPlaceholder(HINT)).toBeVisible()
  })
})
