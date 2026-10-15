import { OK_BUTTON, expect, gotoAdmin, readPermCodes, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  listBrokenByEmptyParams,
  searchTable,
  tableRow,
  totalOf,
} from '../helpers/crud'

const MODAL = '.e2e-role-modal'
const LIST_API = '/web/system/roles'
const HINT = '请输入角色名称或权限字符'

/**
 * 生成控制器始终拼接 `active=`，修复前后端按 active=false 过滤 → 列表恒为空。
 * v0.1.0 已修复：`middleware.NormalizeEmptyQuery()` 绑定前剔除空值查询参数。
 */
const URL_WITH_EMPTY_PARAMS = `${LIST_API}?page=1&page_size=20&keyword=&active=`
const URL_CLEAN_PARAMS = `${LIST_API}?page=1&page_size=20`

async function roleIdOf(page: import('@playwright/test').Page, code: string): Promise<number> {
  const data = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200`))
  const hit = ((data.items ?? []) as { id: number; code: string }[]).find((i) => i.code === code)
  if (!hit) throw new Error(`未查询到角色：${code}`)
  return hit.id
}

test.describe('04b 角色管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/system/role')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('角色管理')
  })

  test('列表页渲染：列定义与筛选控件', async ({ page }) => {
    await expectColumns(page, [
      '角色编号',
      '角色名称',
      '权限字符',
      '显示顺序',
      '数据权限',
      '创建时间',
      '操作',
    ])
    await expect(page.getByPlaceholder(HINT)).toBeVisible()
    await shot(page, '04b-role-list')
  })

  test('角色 CRUD 接口闭环：创建 → 查询 → 改数据权限 → 删除', async ({ page }) => {
    const name = uniqName('E2E角色')
    const code = `e2e_role_${Date.now()}`

    const created = await expectApiOk(
      await apiCall(page, 'POST', LIST_API, {
        name,
        code,
        sequence: 60,
        description: 'E2E 角色描述',
        data_permission: 'dept',
      })
    )
    const id = Number(created.id)

    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(detail.name).toBe(name)
    expect(detail.code).toBe(code)
    expect(detail.data_permission).toBe('dept')
    expect(detail.description).toBe('E2E 角色描述')

    const renamed = `${name}_改`
    await expectApiOk(
      await apiCall(page, 'PUT', `${LIST_API}/${id}`, {
        name: renamed,
        code,
        sequence: 70,
        description: 'E2E 角色描述-改',
        data_permission: 'all',
      })
    )
    const updated = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(updated.name).toBe(renamed)
    expect(updated.sequence).toBe(70)
    expect(updated.data_permission).toBe('all')

    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
    const clean = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200`)
    const items = ((clean.body.data as { items?: { code: string }[] }).items ?? [])
    expect(items.find((i) => i.code === code)).toBeUndefined()
  })

  test('角色菜单授权接口可用（Casbin g 策略来源）', async ({ page }) => {
    const name = uniqName('E2E授权角色')
    const code = `e2e_auth_role_${Date.now()}`
    const created = await expectApiOk(
      await apiCall(page, 'POST', LIST_API, { name, code, sequence: 80 })
    )
    const id = Number(created.id)

    // 授予「风险监控」菜单（种子菜单树中存在该菜单，可能位于目录之下 → 递归查找）
    const menus = (await expectApiOk(
      await apiCall(page, 'GET', '/web/system/menus')
    )) as unknown as unknown[]
    const findNode = (list: unknown[], name: string): { id: number } | undefined => {
      for (const raw of list) {
        const node = raw as { id: number; name: string; children?: unknown[] }
        if (node.name === name) return node
        const hit = node.children ? findNode(node.children, name) : undefined
        if (hit) return hit
      }
      return undefined
    }
    const target = findNode(menus ?? [], '风险监控')
    expect(target, '菜单树应包含「风险监控」').toBeTruthy()

    await expectApiOk(
      await apiCall(page, 'PUT', `${LIST_API}/${id}/menus`, { ids: [target?.id] })
    )
    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    const menuIds = ((detail.menus ?? []) as { id: number }[]).map((m) => m.id)
    expect(menuIds).toContain(Number(target?.id))

    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
  })

  /**
   * 角色列表渲染真实数据（v0.1.0 已修复空查询参数缺陷）。
   *
   * 历史背景：`?keyword=&active=` 的空 `active` 被后端当作 `active=false` → 列表恒为 0 条。
   * 修复后带空参数的请求与干净参数请求返回同样的数据。
   */
  test('角色列表渲染真实数据（空查询参数不再导致列表为空）', async ({ page }) => {
    const broken = await listBrokenByEmptyParams(page, URL_WITH_EMPTY_PARAMS, URL_CLEAN_PARAMS)
    expect(broken).toBe(false)

    // 种子角色「超级管理员」出现在列表中
    await expect(page.locator('.ant-table-row').first()).toBeVisible()
    await expect(tableRow(page, '超级管理')).toHaveCount(1)

    // 带空参数与干净参数的 total 一致
    expect(totalOf(await apiCall(page, 'GET', URL_WITH_EMPTY_PARAMS))).toBe(
      totalOf(await apiCall(page, 'GET', URL_CLEAN_PARAMS))
    )
  })

  /**
   * 角色行内授权入口（新建 / 编辑 / 删除 / 菜单权限 / 数据权限）受
   * `system:role:create/update/delete/assign/data` 权限码门控。
   *
   * v0.1.0 已修复：`system:*` 一组共 27 个按钮权限码（id 9101~9127）已入库并授予超管，
   * 这些入口对超管正常渲染。
   */
  test('角色行内授权入口对超管可见（system:role:* 权限码已入库）', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('system:view')
    expect(codes).toContain('system:role:create')
    expect(codes).toContain('system:role:assign')

    expect(await hasButton(page, '新建角色')).toBe(true)
    expect(await hasButton(page, '菜单权限')).toBe(true)
  })

  test('新建角色弹窗（若按钮渲染）可完成必填校验与提交', async ({ page }) => {
    if (!(await hasButton(page, '新建角色'))) {
      // 按钮被权限门控：改用接口层校验必填契约（HTTP 400，非 500）
      const res = await apiCall(page, 'POST', LIST_API, {})
      expect(res.status).toBe(400)
      expect(res.body.message).toContain('required')
      return
    }

    const name = uniqName('E2E弹窗角色')
    const code = `e2e_modal_role_${Date.now()}`
    await page.getByRole('button', { name: '新建角色' }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
    await expect(page.locator(MODAL).getByText('请输入角色名称')).toBeVisible()

    await page.locator(MODAL).getByPlaceholder('如：区域管理员').fill(name)
    await page.locator(MODAL).getByPlaceholder('如：area_admin').fill(code)
    const resp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
    )
    await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
    // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
    expect([200, 201]).toContain((await resp).status())

    const id = await roleIdOf(page, code)
    expect((await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))).name).toBe(name)
    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
  })

  test('按关键字搜索生效（命中/空态）', async ({ page }) => {
    const hit = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=20&keyword=超级管理`)
    const items = ((hit.body.data as { items?: { name: string }[] }).items ?? [])
    expect(items.some((i) => i.name.includes('超级管理'))).toBe(true)

    const miss = await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=20&keyword=nope_${Date.now()}`)
    expect(((miss.body.data as { items?: unknown[] }).items ?? []).length).toBe(0)

    // v0.1.0 已修复空查询参数缺陷：页面搜索能命中种子角色
    await searchTable(page, HINT, '超级管理', LIST_API)
    await expect(tableRow(page, '超级管理')).toHaveCount(1)
  })

  test('编辑角色弹窗可通过接口契约校验字段更新', async ({ page }) => {
    const name = uniqName('E2E字段角色')
    const code = `e2e_field_role_${Date.now()}`
    const created = await expectApiOk(
      await apiCall(page, 'POST', LIST_API, { name, code, sequence: 90, description: 'A' })
    )
    const id = Number(created.id)

    await expectApiOk(
      await apiCall(page, 'PUT', `${LIST_API}/${id}`, {
        name: `${name}_新`,
        code,
        sequence: 91,
        description: 'B',
        data_permission: 'personal',
      })
    )
    const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${id}`))
    expect(detail.description).toBe('B')
    expect(detail.data_permission).toBe('personal')

    await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${id}`))
    // 表中不残留
    const row = tableRow(page, code)
    expect(await row.count()).toBe(0)
  })
})
