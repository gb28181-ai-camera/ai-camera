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

const DEPT_MODAL = '.e2e-dept-modal'
const POST_MODAL = '.e2e-post-modal'
const DEPT_API = '/web/system/depts'
const POST_API = '/web/system/posts'
const POST_HINT = '请输入岗位编码或岗位名称'

/**
 * 岗位列表因空 `active=` 参数被后端当作 active=false → 恒为空。
 * v0.1.0 已修复：`middleware.NormalizeEmptyQuery()` 绑定前剔除空值查询参数。
 */
const POST_URL_EMPTY = `${POST_API}?page=1&page_size=20&keyword=&active=`
const POST_URL_CLEAN = `${POST_API}?page=1&page_size=20`

/** 递归查找部门节点（GET /web/system/depts 返回树结构，子部门嵌套在 children 下） */
function findIn(list: unknown[], field: string, value: string): { id: number } | undefined {
  for (const raw of list ?? []) {
    const node = raw as Record<string, unknown> & { id: number; children?: unknown[] }
    if (String(node[field]) === value) return node
    const hit = node.children ? findIn(node.children, field, value) : undefined
    if (hit) return hit
  }
  return undefined
}

test.describe('04d 部门 + 岗位管理', () => {
  test.describe('部门管理', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(DEPT_API))
      await gotoAdmin(page, '/system/dept')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('部门管理')
    })

    test('组织架构树与下级部门表格渲染', async ({ page }) => {
      await expect(page.locator('.e2e-dept-tree')).toBeVisible()
      await expect(page.locator('.e2e-dept-tree').getByText('总公司')).toBeVisible()
      await expectColumns(page, ['部门名称', '部门编码', '排序', '负责人', '操作'])
      await expect(tableRow(page, '总公司')).toHaveCount(1)
      await shot(page, '04d-dept-tree')
    })

    test('部门 CRUD 接口闭环：创建 → 树中出现 → 删除', async ({ page }) => {
      const name = uniqName('E2E部门')
      const code = `e2e_dept_${Date.now()}`

      // 建为顶级部门，便于在默认的「顶级部门」表格中直接看到
      const created = await expectApiOk(
        await apiCall(page, 'POST', DEPT_API, {
          name,
          code,
          sequence: 60,
          description: 'E2E 部门描述',
        })
      )
      const id = Number(created.id)
      const detail = await expectApiOk(await apiCall(page, 'GET', DEPT_API))
      const hit = findIn(detail as unknown as unknown[], 'code', code)
      expect(hit?.id).toBe(id)

      // 页面表格中出现（表格为「选中部门的下级」）
      await page.reload()
      await expect(page.locator('.page-container-title').first()).toHaveText('部门管理')
      const row = tableRow(page, name)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(code)
      await shot(page, '04d-dept-created')

      await expectApiOk(await apiCall(page, 'DELETE', `${DEPT_API}/${id}`))
      await page.reload()
      await expect(tableRow(page, name)).toHaveCount(0)
    })

    test('部门名称必填契约：UI 弹窗拦截 + 接口 400', async ({ page }) => {
      // v0.1.0 已修复：`system:dept:create` 已入库并授予超管，「新建部门」入口正常渲染
      const codes = await readPermCodes(page)
      expect(codes).toContain('system:dept:create')
      expect(await hasButton(page, '新建部门')).toBe(true)

      await page.getByRole('button', { name: '新建部门' }).click()
      await expect(page.locator(DEPT_MODAL)).toBeVisible()
      await page.locator(DEPT_MODAL).getByRole('button', { name: /确\s*定/ }).click()
      await expect(page.locator(DEPT_MODAL).getByText('请输入部门名称')).toBeVisible()
      await page.locator(DEPT_MODAL).getByRole('button', { name: /取\s*消/ }).click()

      // 接口层同样拒绝缺少名称的请求
      const res = await apiCall(page, 'POST', DEPT_API, {})
      expect(res.status).toBe(400)
      expect(res.body.message).toContain('required')
    })

    test('编辑部门接口可更新名称与负责人', async ({ page }) => {
      const name = uniqName('E2E改部门')
      const code = `e2e_edit_dept_${Date.now()}`
      const created = await expectApiOk(
        await apiCall(page, 'POST', DEPT_API, { name, code, parent_id: 1, sequence: 70 })
      )
      const id = Number(created.id)

      await expectApiOk(
        await apiCall(page, 'PUT', `${DEPT_API}/${id}`, {
          name: `${name}_新`,
          code,
          parent_id: 1,
          sequence: 71,
          leader_id: 1,
          description: 'E2E 改后描述',
        })
      )
      const all = (await expectApiOk(await apiCall(page, 'GET', DEPT_API))) as unknown as unknown[]
      const hit = findIn(all, 'code', code)
      expect(hit?.id).toBe(id)
      expect(String((hit as unknown as Record<string, unknown>).name)).toBe(`${name}_新`)
      expect(Number((hit as unknown as Record<string, unknown>).leader_id)).toBe(1)

      await expectApiOk(await apiCall(page, 'DELETE', `${DEPT_API}/${id}`))
    })
  })

  test.describe('岗位管理', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(POST_API))
      await gotoAdmin(page, '/system/post')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('岗位管理')
    })

    test('列表页渲染：列定义与筛选控件', async ({ page }) => {
      await expectColumns(page, [
        '岗位编号',
        '岗位编码',
        '岗位名称',
        '排序',
        '状态',
        '创建时间',
        '操作',
      ])
      await expect(page.getByPlaceholder(POST_HINT)).toBeVisible()
      await shot(page, '04d-post-list')
    })

    test('岗位 CRUD 接口闭环：创建 → 查询 → 更新 → 删除', async ({ page }) => {
      const name = uniqName('E2E岗位')
      const code = `e2e_post_${Date.now()}`

      const created = await expectApiOk(
        await apiCall(page, 'POST', POST_API, { name, code, sequence: 60, description: 'E2E 岗位' })
      )
      const id = Number(created.id)

      const detail = await expectApiOk(await apiCall(page, 'GET', `${POST_API}/${id}`))
      expect(detail.name).toBe(name)
      expect(detail.code).toBe(code)

      await expectApiOk(
        await apiCall(page, 'PUT', `${POST_API}/${id}`, {
          name: `${name}_新`,
          code,
          sequence: 61,
          description: 'E2E 岗位-改',
        })
      )
      const updated = await expectApiOk(await apiCall(page, 'GET', `${POST_API}/${id}`))
      expect(updated.sequence).toBe(61)

      await expectApiOk(await apiCall(page, 'DELETE', `${POST_API}/${id}`))
      const clean = await apiCall(page, 'GET', `${POST_API}?page=1&page_size=200`)
      const items = ((clean.body.data as { items?: { code: string }[] }).items ?? [])
      expect(items.find((i) => i.code === code)).toBeUndefined()
    })

    /**
     * 岗位列表渲染真实数据（v0.1.0 已修复空查询参数缺陷）。
     *
     * 历史背景：`?keyword=&active=` 的空 `active` 被后端当作 active=false → 列表恒为 0 条。
     */
    test('岗位列表渲染真实数据（空查询参数不再导致列表为空）', async ({ page }) => {
      // 初始化 SQL 未内置岗位种子数据 → 先造一条用于验证「空查询参数不再过滤掉数据」
      const name = uniqName('E2E岗位查询')
      const code = `e2e_query_post_${Date.now()}`
      const created = await expectApiOk(
        await apiCall(page, 'POST', POST_API, { name, code, sequence: 63 })
      )
      const id = Number(created.id)

      const broken = await listBrokenByEmptyParams(page, POST_URL_EMPTY, POST_URL_CLEAN)
      expect(broken).toBe(false)
      // 带空参数（keyword=&active=）与干净参数的 total 一致，且能查到刚创建的数据
      const withEmpty = totalOf(await apiCall(page, 'GET', POST_URL_EMPTY))
      expect(withEmpty).toBe(totalOf(await apiCall(page, 'GET', POST_URL_CLEAN)))
      expect(withEmpty).toBeGreaterThan(0)

      // 页面检索到该记录（v0.1.0 修复前列表恒为空）
      await searchTable(page, POST_HINT, name, POST_API)
      await expect(tableRow(page, name)).toHaveCount(1)

      await expectApiOk(await apiCall(page, 'DELETE', `${POST_API}/${id}`))
      await searchTable(page, POST_HINT, name, POST_API)
      await expectEmptyTable(page)
    })

    /**
     * 「新建岗位 / 编辑 / 删除」入口受 `system:post:create/update/delete` 权限码门控。
     *
     * v0.1.0 已修复：`system:*` 一组共 27 个按钮权限码（id 9101~9127）已入库并授予超管，
     * `/web/profile/menus` 的 permission_code 覆盖页面 `can()` 读取的全部取值。
     */
    test('岗位新增入口对超管可见（system:post:* 权限码已入库）', async ({ page }) => {
      const codes = await readPermCodes(page)
      expect(codes).toContain('system:post:create')

      expect(await hasButton(page, '新建岗位')).toBe(true)
      await expect(page.locator(POST_MODAL)).toHaveCount(0)
    })

    test('分配角色弹窗：回显岗位角色并保存', async ({ page }) => {
      const roles = (await expectApiOk(
        await apiCall(page, 'GET', '/web/system/roles?page=1&page_size=1')
      )) as unknown as { items?: { id: number; name: string }[] }
      const role = roles.items?.[0]
      expect(role).toBeTruthy()

      const name = uniqName('E2E角色岗位')
      const code = `e2e_role_post_${Date.now()}`
      const created = await expectApiOk(
        await apiCall(page, 'POST', POST_API, {
          name,
          code,
          sequence: 64,
          role_ids: [role!.id],
        })
      )
      const id = Number(created.id)

      await page.reload()
      await expect(page.locator('.page-container-title').first()).toHaveText('岗位管理')
      await searchTable(page, POST_HINT, name, POST_API)
      const row = tableRow(page, name)
      await expect(row).toHaveCount(1)
      await row.getByRole('button', { name: '分配角色' }).click()

      const modal = page.locator('.e2e-post-role-modal')
      await expect(modal).toBeVisible()
      // 回显：getPost 返回的 role_ids 以标签形式出现在多选框中
      await expect(modal.getByText(role!.name, { exact: false }).first()).toBeVisible()

      const resp = page.waitForResponse(
        (res) => res.url().includes(`${POST_API}/${id}`) && res.request().method() === 'PUT'
      )
      await modal.getByRole('button', { name: OK_BUTTON }).click()
      expect((await resp).status()).toBe(200)
      const detail = await expectApiOk(await apiCall(page, 'GET', `${POST_API}/${id}`))
      expect(detail.role_ids).toEqual([role!.id])

      await expectApiOk(await apiCall(page, 'DELETE', `${POST_API}/${id}`))
    })

    test('按关键字搜索接口生效（命中/空态）', async ({ page }) => {
      const created = await expectApiOk(
        await apiCall(page, 'POST', POST_API, {
          name: uniqName('E2E搜索岗位'),
          code: `e2e_search_post_${Date.now()}`,
          sequence: 62,
        })
      )

      const hit = await apiCall(page, 'GET', `${POST_API}?page=1&page_size=20&keyword=e2e_search_post`)
      expect(((hit.body.data as { items?: unknown[] }).items ?? []).length).toBeGreaterThan(0)

      await searchTable(page, POST_HINT, 'nope', POST_API)
      await expectEmptyTable(page)

      await expectApiOk(await apiCall(page, 'DELETE', `${POST_API}/${Number(created.id)}`))
    })
  })
})
