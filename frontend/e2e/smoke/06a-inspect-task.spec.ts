import { expect, gotoAdmin, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  searchTable,
  selectFormOption,
  tableRow,
} from '../helpers/crud'

const MODAL = '.e2e-inspect-task-modal'
const LIST_API = '/web/inspects/tasks'
const HINT = '请输入任务名称'
const SEED_REGION = '一号厂区'

/** 巡检任务的计划时间（未来时间，ISO 字符串） */
function futurePlanDT(): string {
  return new Date(Date.now() + 24 * 3600 * 1000).toISOString().slice(0, 19).replace('T', ' ')
}

async function taskIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const data = await expectApiOk(
    await apiCall(
      page,
      'GET',
      `${LIST_API}?page=1&page_size=200&keyword=${encodeURIComponent(name)}`
    )
  )
  const hit = ((data.items ?? []) as { id: number; name: string }[]).find((i) => i.name === name)
  if (!hit) throw new Error(`未查询到巡检任务：${name}`)
  return hit.id
}

async function createTask(
  page: import('@playwright/test').Page,
  name: string
): Promise<'ui' | 'api'> {
  if (!(await hasButton(page, '新增巡检任务'))) {
    await expectApiOk(
      await apiCall(page, 'POST', LIST_API, {
        name,
        inspector_id: 1,
        region_id: 1,
        location: 'E2E 巡检点',
        plan_dt: futurePlanDT(),
        content: 'E2E 巡检内容',
        status: 'published',
      })
    )
    return 'api'
  }

  await page.getByRole('button', { name: '新增巡检任务' }).click()
  await expect(page.locator(MODAL)).toBeVisible()
  await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增巡检任务')
  await page.locator(MODAL).getByPlaceholder('请输入任务名称').fill(name)
  // 所属区域 / 巡检人为必填项（v0.1.0 起新增按钮可渲染，UI 路径需要真实选择才能提交）
  await selectFormOption(page, page.locator(MODAL), '所属区域', SEED_REGION)
  await selectFormOption(page, page.locator(MODAL), '巡检人', '系统管理员')
  await page.locator(MODAL).getByPlaceholder('请输入巡检位置').fill('E2E 巡检点')
  await page.locator(MODAL).getByPlaceholder('请输入巡检内容').fill('E2E 巡检内容')
  await page.locator(MODAL).getByPlaceholder('请选择巡检时间').fill('2026-12-31 09:00:00')
  await page.keyboard.press('Enter')

  const resp = page.waitForResponse(
    (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
  )
  await page.locator(MODAL).getByRole('button', { name: /确\s*定/ }).click()
  // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
  expect([200, 201]).toContain((await resp).status())
  // 等待弹窗关闭动画结束：弹窗内的「请输入任务名称」与页头搜索框同名，
  // 残留的 DOM 会让后续 getByPlaceholder 触发 strict mode 冲突
  await expect(page.locator(MODAL)).toBeHidden()
  return 'ui'
}

async function deleteTask(page: import('@playwright/test').Page, name: string): Promise<void> {
  const id = await taskIdOf(page, name)
  await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
}

test.describe('06a 巡检任务', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/inspect/task')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('巡检任务管理')
  })

  test('列表渲染：列定义完整（含所属区域名称）', async ({ page }) => {
    await expectColumns(page, [
      '任务名称',
      '所属区域',
      '巡检位置',
      '巡检人',
      '巡检内容',
      '巡检时间',
      '状态',
      '消息通知',
      '备注',
      '创建时间',
      '操作',
    ])
    await expect(page.getByPlaceholder(HINT)).toBeVisible()
    // 「巡检人」为 UserSelect（SelectorModal 触发按钮），此处只校验搜索框
    await shot(page, '06a-task-list')
  })

  test('新增巡检任务后列表出现该记录（状态=已发布）', async ({ page }) => {
    const name = uniqName('E2E巡检任务')
    await createTask(page, name)

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(SEED_REGION)
    await expect(row).toContainText('E2E 巡检点')
    await expect(row).toContainText('系统管理员')
    // v0.1.0 已修复：StatusTag 的 inspect 字典补齐 published/finished，
    // 「状态」列渲染中文文案（与 INSPECT_STATUS_LABELS 一致）
    await expect(row).toContainText('已发布')
    await expect(row).not.toContainText('published')
    const created = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${await taskIdOf(page, name)}`))
    expect(created.status).toBe('published')
    await shot(page, '06a-task-created')

    await deleteTask(page, name)
    await searchTable(page, HINT, name, LIST_API)
    await expectEmptyTable(page)
  })

  test('查看任务详情弹窗（含消息通知状态与巡检内容）', async ({ page }) => {
    const name = uniqName('E2E详情任务')
    await createTask(page, name)
    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)

    await row.getByRole('button', { name: '查看' }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    const modal = page.locator(MODAL)
    await expect(modal.locator('.ant-modal-title')).toHaveText('巡检任务详情')
    for (const label of ['任务名称', '所属区域', '巡检位置', '巡检人', '巡检时间', '状态', '消息通知', '巡检内容']) {
      await expect(modal.getByText(label, { exact: true })).toBeVisible()
    }
    await expect(modal.getByText('E2E 巡检内容')).toBeVisible()
    await shot(page, '06a-task-detail')
    await modal.getByRole('button', { name: /关\s*闭/ }).click()
    await expect(modal).toBeHidden()

    // 已修复（v0.1.0）：「查看」会调用任务级已读接口 PUT /web/inspects/tasks/:id/read，
    // 列表「消息通知」列由「未读」翻转为「已读」（操作手册 6.1）。
    const readRow = tableRow(page, name)
    await expect(readRow).toHaveCount(1)
    await expect(readRow).toContainText('已读')
    await expect(readRow).not.toContainText('未读')

    // 后端持久化确认：read 字段已落库，且任务其余字段未受影响
    const id = await taskIdOf(page, name)
    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(detail.read).toBe(true)
    expect(detail.name).toBe(name)
    expect(detail.status).toBe('published')
    await shot(page, '06a-task-read')

    await deleteTask(page, name)
  })

  test('编辑任务名称后列表字段已变更', async ({ page }) => {
    const name = uniqName('E2E改任务')
    const renamed = `${name}_新`
    await createTask(page, name)
    const id = await taskIdOf(page, name)

    if (await hasButton(page, /编\s*辑/)) {
      await searchTable(page, HINT, name, LIST_API)
      const row = tableRow(page, name)
      await expect(row).toHaveCount(1)
      await row.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).getByPlaceholder('请输入任务名称')).toHaveValue(name)
      await page.locator(MODAL).getByPlaceholder('请输入任务名称').fill(renamed)
      const resp = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
      )
      await page.locator(MODAL).getByRole('button', { name: /确\s*定/ }).click()
      expect((await resp).status()).toBe(200)
    } else {
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}`, {
          name: renamed,
          inspector_id: 1,
          region_id: 1,
          location: 'E2E 巡检点',
          plan_dt: futurePlanDT(),
          content: 'E2E 巡检内容',
          status: 'published',
        })
      )
    }

    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(detail.name).toBe(renamed)

    await deleteTask(page, renamed)
  })

  test('删除任务后列表为空态', async ({ page }) => {
    const name = uniqName('E2E删任务')
    await createTask(page, name)
    await searchTable(page, HINT, name, LIST_API)
    await expect(tableRow(page, name)).toHaveCount(1)

    await deleteTask(page, name)

    await searchTable(page, HINT, name, LIST_API)
    await expectEmptyTable(page)
  })
})
