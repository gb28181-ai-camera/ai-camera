import { expect, gotoAdmin, readPermCodes, shot, test, uniqName } from '../fixtures/auth'
import {
  filterTable,
  apiCall,
  expectApiOk,
  expectColumns,
  hasButton,
  resetTable,
  searchTable,
  tableRow,
} from '../helpers/crud'

const CONFIG_MODAL = '.e2e-config-modal'
const LOG_MODAL = '.e2e-operation-log-modal'
const NOTIFY_MODAL = '.e2e-notification-modal'
const CONFIG_API = '/web/system/configs'
const LOG_API = '/web/system/logs'
const NOTIFY_API = '/web/system/notifications'
const CONFIG_HINT = '请输入参数名称或参数键'

async function configIdOf(page: import('@playwright/test').Page, key: string): Promise<number> {
  const list = (await expectApiOk(await apiCall(page, 'GET', `${CONFIG_API}?keyword=${key}`))) as unknown as {
    id: number
    key: string
  }[]
  const hit = (list ?? []).find((i) => i.key === key)
  if (!hit) throw new Error(`未查询到系统参数：${key}`)
  return hit.id
}

test.describe('04e 系统参数 / 操作日志 / 系统通知', () => {
  test.describe('系统参数', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(CONFIG_API))
      await gotoAdmin(page, '/system/config')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('系统参数')
    })

    test('列表渲染与关键字搜索（无匹配为空态）', async ({ page }) => {
      await expectColumns(page, ['参数名称', '参数键', '参数值', '类型', '分组', '排序', '状态', '操作'])
      await expect(page.getByPlaceholder(CONFIG_HINT)).toBeVisible()
      await shot(page, '04e-config-list')

      // 该页为客户端过滤（不发起请求）
      await filterTable(page, CONFIG_HINT, `nope_${Date.now()}`)
      await expect(page.locator('.ant-table-row')).toHaveCount(0)

      await resetTable(page)
      expect(await page.locator('.ant-table-row').count()).toBeGreaterThan(0)
    })

    test('参数 CRUD 接口闭环：创建 → 搜索命中 → 更新 → 删除', async ({ page }) => {
      const name = uniqName('E2E参数')
      const key = `e2e.param.${Date.now()}`

      const created = await expectApiOk(
        await apiCall(page, 'POST', CONFIG_API, {
          name,
          key,
          value: '120',
          data_type: 'int',
          group_name: 'e2e',
          sequence: 60,
          description: 'E2E 说明',
        })
      )
      const id = Number(created.id)

      // 该页为客户端过滤，需重新加载以纳入新数据后再检索
      await page.reload()
      await expect(page.locator('.page-container-title').first()).toHaveText('系统参数')
      await filterTable(page, CONFIG_HINT, key)
      const row = tableRow(page, key)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(name)
      await expect(row).toContainText('120')
      await shot(page, '04e-config-created')

      await expectApiOk(
        await apiCall(page, 'PUT', `${CONFIG_API}/${id}`, {
          name: `${name}_新`,
          key,
          value: '360',
          data_type: 'int',
          group_name: 'e2e',
          sequence: 61,
          description: 'E2E 说明-改',
        })
      )
      await page.reload()
      await expect(page.locator('.page-container-title').first()).toHaveText('系统参数')
      await filterTable(page, CONFIG_HINT, key)
      await expect(tableRow(page, key)).toContainText('360')

      await expectApiOk(await apiCall(page, 'DELETE', `${CONFIG_API}/${id}`))
      await page.reload()
      await expect(page.locator('.page-container-title').first()).toHaveText('系统参数')
      await filterTable(page, CONFIG_HINT, key)
      await expect(page.locator('.ant-table-row')).toHaveCount(0)
      expect(await configIdOfCount(page, key)).toBe(0)
    })

    test('新增参数：按钮对超管可见，空表单提交被拦截', async ({ page }) => {
      // v0.1.0 已修复：`system:config:create`（含 system:* 一组 27 个按钮权限码）已入库
      const codes = await readPermCodes(page)
      expect(codes).toContain('system:config:create')
      await expect(page.getByRole('button', { name: '新建参数' })).toBeVisible()

      await page.getByRole('button', { name: '新建参数' }).click()
      await expect(page.locator(CONFIG_MODAL)).toBeVisible()
      // v0.3.0：支持 Schema 编辑与更多数据类型
      await expect(page.locator(CONFIG_MODAL).getByText('配置 Schema')).toBeVisible()
      await page.locator(CONFIG_MODAL).getByRole('button', { name: /确\s*定/ }).click()
      await expect(page.locator(CONFIG_MODAL).getByText('请输入参数名称')).toBeVisible()
      await page.locator(CONFIG_MODAL).getByRole('button', { name: /取\s*消/ }).click()

      // 接口层同样拒绝缺少必填字段的请求
      const res = await apiCall(page, 'POST', CONFIG_API, {})
      expect(res.status).toBe(400)
      expect(res.body.message).toContain('required')
    })
  })

  test.describe('操作日志', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(LOG_API))
      await gotoAdmin(page, '/system/operation-log')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('操作日志')
    })

    test('日志列表渲染与分页', async ({ page }) => {
      await expectColumns(page, [
        '日志编号',
        '系统模块',
        '操作类型',
        '请求方式',
        '操作人员',
        '主机',
        '操作地点',
        '状态',
        '操作时间',
      ])
      expect(await page.locator('.ant-table-row').count()).toBeGreaterThan(0)
      await expect(page.locator('.ant-pagination')).toBeVisible()
      await shot(page, '04e-log-list')
    })

    test('详情弹窗展示请求路径与耗时', async ({ page }) => {
      const row = page.locator('.ant-table-row').first()
      await expect(row).toBeVisible()
      await row.getByRole('button', { name: '详情' }).click()

      await expect(page.locator(LOG_MODAL)).toBeVisible()
      const modal = page.locator(LOG_MODAL)
      await expect(modal.locator('.ant-modal-title')).toHaveText('操作日志详情')
      await expect(modal.getByText('日志编号')).toBeVisible()
      await expect(modal.getByText('请求路径')).toBeVisible()
      await expect(modal.getByText('耗时')).toBeVisible()
      await shot(page, '04e-log-detail')
      await modal.getByRole('button', { name: /关\s*闭/ }).click()
      await expect(modal).toBeHidden()
    })

    test('按系统模块筛选：无匹配为空态，重置恢复', async ({ page }) => {
      await searchTable(page, '请输入系统模块', `none_${Date.now()}`, LOG_API)
      await expect(page.locator('.ant-table-row')).toHaveCount(0)

      const resetResp = page.waitForResponse(
        (res) => res.url().includes(LOG_API) && res.request().method() === 'GET'
      )
      await page.getByRole('button', { name: '重置' }).click()
      await resetResp
      // 重置后列表恢复：等待首行可见（避免读取加载中的瞬时空表）
      await expect(page.locator('.ant-table-row').first()).toBeVisible()
    })

    test('导出/清空按钮对超管可见（system:log:* 权限码已入库）', async ({ page }) => {
      // v0.1.0 已修复：`system:log:export` / `system:log:clear` 已入库并授予超管
      const codes = await readPermCodes(page)
      expect(codes).toContain('system:log:export')
      expect(await hasButton(page, '导出')).toBe(true)
      expect(await hasButton(page, '清空')).toBe(true)
    })
  })

  test.describe('系统通知', () => {
    test('页面渲染：标题与分类 Tab', async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(NOTIFY_API))
      await gotoAdmin(page, '/system/notification')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('系统通知')
      await expect(page.locator('.ant-tabs')).toBeVisible()
      await expect(page.locator('.ant-tabs-tab').first()).toBeVisible()
      // v0.1.0 已修复：`system:notify:mark` 已入库并授予超管 →「全部已读」正常渲染
      const codes = await readPermCodes(page)
      expect(codes).toContain('system:notify:mark')
      expect(await hasButton(page, '全部已读')).toBe(true)
      await shot(page, '04e-notify-list')
    })

    test('切换分类 Tab 会重新拉取列表', async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(NOTIFY_API))
      await gotoAdmin(page, '/system/notification')
      await resp

      const tabs = page.locator('.ant-tabs-tab')
      expect(await tabs.count()).toBeGreaterThan(1)
      const second = tabs.nth(1)
      const tabResp = page.waitForResponse((res) => res.url().includes(NOTIFY_API))
      await second.click()
      await tabResp
      await expect(second).toHaveClass(/ant-tabs-tab-active/)
    })

    test('列表为空态时详情入口不存在（通知由业务事件产生）', async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(NOTIFY_API))
      await gotoAdmin(page, '/system/notification')
      await resp

      const rows = await page.locator('.ant-table-row').count()
      if (rows === 0) {
        await expect(page.locator('.ant-empty')).toBeVisible()
        expect(await hasButton(page, '查看')).toBe(false)
      } else {
        await page.locator('.ant-table-row').first().getByRole('button', { name: '查看' }).click()
        await expect(page.locator(NOTIFY_MODAL)).toBeVisible()
        await expect(page.locator(NOTIFY_MODAL).locator('.ant-modal-title')).toHaveText('通知详情')
      }
    })
  })
})

/** 统计指定 key 的系统参数条数 */
async function configIdOfCount(page: import('@playwright/test').Page, key: string): Promise<number> {
  const list = (await expectApiOk(await apiCall(page, 'GET', `${CONFIG_API}?keyword=${key}`))) as unknown as {
    key: string
  }[]
  return (list ?? []).filter((i) => i.key === key).length
}
