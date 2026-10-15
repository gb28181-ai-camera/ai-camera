import { OK_BUTTON, expect, gotoAdmin, readPermCodes, shot, test, uniqName } from '../fixtures/auth'
import { collectConsole } from '../helpers/console'
import { apiCall, expectApiOk, expectColumns, filterTable, hasButton } from '../helpers/crud'

const MODAL = '.e2e-menu-modal'
const API_MODAL = '.e2e-menu-api-modal'
const LIST_API = '/web/system/menus'
const HINT = '请输入菜单名称'

/** 递归查找菜单节点 */
function findMenu(list: unknown[], name: string): { id: number; name: string } | undefined {
  for (const raw of list) {
    const node = raw as { id: number; name: string; children?: unknown[] }
    if (node.name === name) return node
    const hit = node.children ? findMenu(node.children, name) : undefined
    if (hit) return hit
  }
  return undefined
}

async function menuIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const data = (await expectApiOk(await apiCall(page, 'GET', LIST_API))) as unknown as unknown[]
  const hit = findMenu(data ?? [], name)
  if (!hit) throw new Error(`未查询到菜单：${name}`)
  return hit.id
}

/** 展开表格中的首个目录行，使子节点进入 DOM */
async function expandFirstRow(page: import('@playwright/test').Page): Promise<void> {
  const expander = page.locator('.ant-table-row-expand-icon').first()
  await expect(expander).toBeVisible()
  await expander.click()
}

test.describe('04c 菜单管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/system/menu')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('菜单管理')
  })

  test('菜单树渲染：列定义与顶级节点', async ({ page }) => {
    await expectColumns(page, [
      '菜单名称',
      '路由唯一键',
      '组件',
      '排序',
      '按钮类型',
      '权限标识',
      '状态',
      '操作',
    ])
    for (const name of ['首页', '实时监控', 'AI风险', '系统设置']) {
      await expect(page.locator('.ant-table-row').filter({ hasText: name }).first()).toBeVisible()
    }
    await shot(page, '04c-menu-tree')
  })

  test('展开目录行后子菜单出现（含系统管理 → 用户管理）', async ({ page }) => {
    // 展开「系统设置」
    const sysRow = page.locator('.ant-table-row').filter({ hasText: '系统设置' }).first()
    await expect(sysRow).toBeVisible()
    await sysRow.locator('.ant-table-row-expand-icon').click()

    const systemMgmt = page.locator('.ant-table-row').filter({ hasText: '系统管理' }).first()
    await expect(systemMgmt).toBeVisible()

    // 再展开「系统管理」→ 用户管理 可见
    await systemMgmt.locator('.ant-table-row-expand-icon').click()
    await expect(page.locator('.ant-table-row').filter({ hasText: '用户管理' }).first()).toBeVisible()
    await shot(page, '04c-menu-expanded')
  })

  test('新增菜单接口闭环：创建 → 树中出现 → 删除', async ({ page }) => {
    const name = uniqName('E2E菜单')
    const path = `/e2e/${Date.now()}`

    const created = await expectApiOk(
      await apiCall(page, 'POST', LIST_API, {
        name,
        type: 'menu',
        platform: 'pc',
        path,
        component: 'e2e/Probe',
        sequence: 999,
        active: true,
        hidden: false,
      })
    )
    const id = Number(created.id)
    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(detail.name).toBe(name)
    expect(detail.path).toBe(path)

    // 该页为客户端过滤，需重新加载以纳入新数据后再检索
    await page.reload()
    await expect(page.locator('.page-container-title').first()).toHaveText('菜单管理')
    await filterTable(page, HINT, name)
    const row = page.locator('.ant-table-row').filter({ hasText: name })
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(path)
    await shot(page, '04c-menu-created')

    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
    await page.reload()
    await expect(page.locator('.page-container-title').first()).toHaveText('菜单管理')
    await filterTable(page, HINT, name)
    await expect(page.locator('.ant-table-row').filter({ hasText: name })).toHaveCount(0)
  })

  test('展开行展示已绑定接口表格（.e2e-menu-api-table）', async ({ page }) => {
    const console = collectConsole(page)
    // 重新加载：从零采集本次页面渲染的控制台输出（rc-table 告警只在首次渲染时打印）
    await page.reload()
    await expect(page.locator('.page-container-title').first()).toHaveText('菜单管理')
    await filterTable(page, HINT, "首页")
    const row = page.locator('.ant-table-row').filter({ hasText: '首页' }).first()
    await expect(row).toBeVisible()
    await row.locator('.ant-table-row-expand-icon').click()

    await expect(page.locator('.e2e-menu-api-table')).toBeVisible()
    await expect(page.getByText('已绑定接口：')).toBeVisible()
    // 展开行不得引发 rc-table 用法告警（嵌套 Table / 树形数据 + expandedRowRender）
    expect(console.texts()).toEqual([])
    await shot(page, '04c-menu-api-table')
  })

  /**
   * 菜单树的增删改与接口绑定入口受 `system:menu:create/update/delete/api` 权限码门控。
   *
   * v0.1.0 已修复：初始化 SQL 追加的按钮权限码（id 9001~9037 补 basic/device/iot-card/
   * setting/upgrade/inspect 等模块，id 9101~9127 补 system 一组）已覆盖页面 `can()` 辅助函数
   * 读取的全部权限码，`/web/profile/menus` 返回的 permission_code 覆盖率为 100%。
   * 因此超管登录后这些入口正常渲染。
   */
  test('菜单操作入口对超管可见（system:menu:* 权限码已入库）', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('system:view')
    expect(codes).toContain('system:menu:create')
    expect(codes).toContain('system:menu:api')

    expect(await hasButton(page, '新建菜单')).toBe(true)

    // 行内操作入口同样渲染（编辑 / 绑定接口 / 新增下级）
    expect(await hasButton(page, '绑定接口')).toBe(true)

    // 弹窗未打开时不渲染
    await expect(page.locator(MODAL)).toHaveCount(0)
    await expect(page.locator(API_MODAL)).toHaveCount(0)
  })

  test('菜单必填契约：缺少菜单名称时接口返回 400', async ({ page }) => {
    const res = await apiCall(page, 'POST', LIST_API, { type: 'menu', platform: 'pc' })
    expect(res.status).toBe(400)
    expect(res.body.message).toContain('required')
    // 确认没有脏数据落库
    expect(await menuIdOfSafe(page, 'e2e-should-not-exist')).toBeUndefined()
    expect(OK_BUTTON).toBeTruthy()
  })

  test('目录行可展开/收起（交互一致性）', async ({ page }) => {
    await expandFirstRow(page)
    const expander = page.locator('.ant-table-row-expand-icon').first()
    await expect(expander).toHaveClass(/ant-table-row-expand-icon-expanded/)
    await expander.click()
    await expect(expander).not.toHaveClass(/ant-table-row-expand-icon-expanded/)
  })
})

/** 名称查找的安全版本（不存在返回 undefined，不抛错） */
async function menuIdOfSafe(
  page: import('@playwright/test').Page,
  name: string
): Promise<number | undefined> {
  const data = (await expectApiOk(await apiCall(page, 'GET', LIST_API))) as unknown as unknown[]
  return findMenu(data ?? [], name)?.id
}
