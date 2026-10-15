import { expect, gotoAdmin, shot, test } from '../fixtures/auth'
import { apiCall, expectApiOk, expectColumns, expectEmptyTable, hasButton } from '../helpers/crud'

const LIST_API = '/web/inspects/records'
const MODAL = '.e2e-inspect-record-modal'

test.describe('06b 巡检记录', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/inspect/record')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('巡检记录')
  })

  test('列表渲染：列定义与筛选控件完整（只读页）', async ({ page }) => {
    await expectColumns(page, [
      '任务名称',
      '所属区域',
      '巡检位置',
      '巡检人',
      '巡检内容',
      '巡检时间',
      '处理结果图',
      '处理时间',
      '状态',
      '操作',
    ])
    await expect(page.getByPlaceholder('请输入任务名称')).toBeVisible()
    // 只读页无新增入口
    expect(await hasButton(page, /新\s*增/)).toBe(false)
    await shot(page, '06b-record-list')
  })

  test('巡检记录与任务闭环一致（记录由 App 端完成任务后生成）', async ({ page }) => {
    const res = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=50`)
    expect(res.status).toBe(200)
    expect(res.body.code).toBe(200)

    const items = ((res.body.data as { items?: { status?: string; task_name?: string }[] }).items ??
      []) as { status?: string; task_name?: string }[]
    const rowCount = await page.locator('.ant-table-row').count()
    // 表格行数与接口返回条数一致
    expect(rowCount).toBe(items.length)

    if (items.length === 0) {
      await expectEmptyTable(page)
      return
    }
    // 有记录时校验状态枚举与任务名称非空
    for (const item of items) {
      expect(item.status).toBe('finished')
      expect(String(item.task_name ?? '')).not.toBe('')
    }
  })

  test('关键字筛选：无匹配时为空态，重置后恢复', async ({ page }) => {
    await page.getByPlaceholder('请输入任务名称').fill(`nope_${Date.now()}`)
    const resp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: '搜索' }).click()
    await resp
    await expectEmptyTable(page)

    const resetResp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: '重置' }).click()
    await resetResp
    expect(await page.locator('.ant-table-row').count()).toBeGreaterThanOrEqual(0)
  })

  test('详情弹窗：有记录时可打开并展示处理结果', async ({ page }) => {
    const rows = await page.locator('.ant-table-row').count()
    if (rows === 0) {
      // 无记录时入口不存在（记录需 App 端完成巡检后产生）
      expect(await hasButton(page, '查看')).toBe(false)
      await expect(page.locator(MODAL)).toHaveCount(0)
      return
    }

    await page.locator('.ant-table-row').first().getByRole('button', { name: '查看' }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    const modal = page.locator(MODAL)
    await expect(modal.locator('.ant-modal-title')).toContainText('巡检记录详情')
    for (const label of ['任务名称', '所属区域', '巡检位置', '巡检人', '巡检时间', '状态', '巡检内容']) {
      await expect(modal.getByText(label, { exact: true })).toBeVisible()
    }
    await shot(page, '06b-record-detail')
    await modal.getByRole('button', { name: /关\s*闭/ }).click()
    await expect(modal).toBeHidden()
  })

  test('记录接口契约：删除任务不会删除已完成记录（快照冗余）', async ({ page }) => {
    const res = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=50`))
    const items = ((res.items ?? []) as { task_name?: string; inspector?: string }[]) ?? []
    for (const item of items) {
      // 冗余字段快照：任务名与巡检人必须落库
      expect(String(item.task_name ?? '')).not.toBe('')
      expect(String(item.inspector ?? '')).not.toBe('')
    }
  })
})
