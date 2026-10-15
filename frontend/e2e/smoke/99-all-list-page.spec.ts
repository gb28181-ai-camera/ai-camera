import { collectConsole } from '../helpers/console'
import { apiCall, expectApiOk } from '../helpers/crud'
import { expect, gotoAdmin, shot, test } from '../fixtures/auth'

/**
 * 全页面覆盖验证（收尾用例，必须最后执行）。
 *
 * 覆盖范围 = 路由表（src/admin/App.tsx 的 ROUTES）+ 后端菜单树中所有 `type=menu` 的路径。
 * 每条路由断言：
 *  1) 页面无白屏（主内容区有可见内容）
 *  2) 无 console error / 未捕获异常（忽略 favicon 等噪音，见 helpers/console.ts）
 *  3) 列表页存在标题或表格/表单主体
 */

/** 路由表（与 src/admin/App.tsx 的 ROUTES + /login 一一对应） */
const ROUTES: { path: string; marker: string }[] = [
  { path: '/dashboard', marker: '首页看板' },
  { path: '/profile', marker: '个人中心' },
  { path: '/system/user', marker: '用户管理' },
  { path: '/system/role', marker: '角色管理' },
  { path: '/system/menu', marker: '菜单管理' },
  { path: '/system/dept', marker: '部门管理' },
  { path: '/system/post', marker: '岗位管理' },
  { path: '/system/config', marker: '系统参数' },
  { path: '/system/operation-log', marker: '操作日志' },
  { path: '/system/notification', marker: '系统通知' },
  { path: '/basic/region', marker: '区域管理' },
  { path: '/basic/risk-type', marker: '风险类型管理' },
  { path: '/basic/alarm-type', marker: '告警类型管理' },
  { path: '/settings/device', marker: '设备管理' },
  { path: '/settings/voice', marker: '语音配置管理' },
  { path: '/settings/iot-card', marker: '物联网卡管理' },
  { path: '/settings/iot-card-replace', marker: '物联网卡更换记录' },
  { path: '/settings/device-mode', marker: '设备模式管理' },
  { path: '/settings/alarm', marker: '设备告警设置' },
  { path: '/settings/risk-notify', marker: '风险通知设置' },
  { path: '/settings/upgrade', marker: '设备升级管理' },
  { path: '/settings/firmware', marker: '固件管理' },
  { path: '/settings/model', marker: '模型管理' },
  { path: '/settings/firmware-type', marker: '固件类型管理' },
  { path: '/settings/getui', marker: '个推账号绑定' },
  { path: '/settings/sms', marker: '腾讯云短信配置' },
  { path: '/risk/ai-risk', marker: 'AI风险' },
  { path: '/risk/ai-monitor', marker: 'AI监控' },
  { path: '/risk/monitor', marker: '风险监控' },
  { path: '/monitor/realtime', marker: '实时监控' },
  { path: '/device/monitor', marker: '设备监控' },
  { path: '/device/history', marker: '设备历史记录' },
  { path: '/device/iot-card-monitor', marker: '物联网卡监控' },
  { path: '/device/gis', marker: 'GIS查看' },
  { path: '/alarm/list', marker: '设备告警' },
  { path: '/inspect/task', marker: '巡检任务管理' },
  { path: '/inspect/record', marker: '巡检记录' },
  { path: '/stats/risk', marker: '风险统计分析' },
  { path: '/stats/alarm', marker: '设备告警分析' },
  { path: '/screen', marker: '风险类型统计' },
]

/** 主内容区（MainLayout 的 .main-content-inner） */
const CONTENT = '.main-content-inner'

test.describe('99 全页面覆盖验证', () => {
  test('遍历全部路由：均正常渲染且无 console error', async ({ page }) => {
    const console = collectConsole(page)
    const failures: string[] = []

    for (const route of ROUTES) {
      console.clear()
      await gotoAdmin(page, route.path)

      try {
        await expect(page.locator(CONTENT)).toBeVisible({ timeout: 10_000 })
        // 无白屏：主内容区必须有可见文本或表格/表单。
        // 用 expect.poll 重试：首屏数据接口（如首页看板统计）返回较慢时，
        // 容器已可见但内容尚未渲染，直接读 innerText 会误判白屏。
        await expect
          .poll(
            async () => {
              const text = (await page.locator(CONTENT).innerText()).trim()
              if (text.length > 0) return true
              return (await page.locator(`${CONTENT} table, ${CONTENT} form`).count()) > 0
            },
            { timeout: 10_000, message: '主内容区为空（疑似白屏）' }
          )
          .toBe(true)

        await expect(page.getByText(route.marker, { exact: false }).first()).toBeVisible({
          timeout: 10_000,
        })
      } catch (error) {
        failures.push(`[渲染] ${route.path} → ${(error as Error).message.split('\n')[0]}`)
      }

      const issues = console.texts()
      if (issues.length > 0) {
        failures.push(`[控制台] ${route.path} → ${issues.join(' | ')}`)
      }
    }

    await shot(page, '99-last-route')
    expect(failures, `存在问题的路由：\n${failures.join('\n')}`).toEqual([])
  })

  test('后端菜单树中的全部菜单路径均可访问', async ({ page }) => {
    const data = (await expectApiOk(
      await apiCall(page, 'GET', '/web/profile/menus')
    )) as unknown as unknown[]

    const paths: { name: string; path: string }[] = []
    const walk = (list: unknown[]) => {
      for (const raw of list) {
        const node = raw as { name?: string; path?: string; type?: string; children?: unknown[] }
        if (node.type === 'menu' && node.path) {
          paths.push({ name: String(node.name ?? ''), path: String(node.path) })
        }
        if (Array.isArray(node.children)) walk(node.children)
      }
    }
    walk(data ?? [])

    // 菜单树至少包含全部前端路由（首页与个人中心为静态兜底，可能不在树中）
    expect(paths.length).toBeGreaterThanOrEqual(20)

    const console = collectConsole(page)
    const failures: string[] = []

    for (const item of paths) {
      console.clear()
      await gotoAdmin(page, item.path)
      try {
        await expect(page.locator(CONTENT)).toBeVisible({ timeout: 10_000 })
        const text = (await page.locator(CONTENT).innerText()).trim()
        if (text.length === 0) throw new Error('主内容区为空（疑似白屏）')
      } catch (error) {
        failures.push(`[渲染] ${item.name}(${item.path}) → ${(error as Error).message.split('\n')[0]}`)
      }
      const issues = console.texts()
      if (issues.length > 0) {
        failures.push(`[控制台] ${item.name}(${item.path}) → ${issues.join(' | ')}`)
      }
    }

    expect(failures, `菜单路由存在问题：\n${failures.join('\n')}`).toEqual([])
  })

  test('未知路径重定向到首页看板（不白屏）', async ({ page }) => {
    await gotoAdmin(page, '/not-exist-page/xyz')
    await expect(page).toHaveURL(/\/admin\/dashboard/)
    await expect(page.locator('.page-container-title').first()).toHaveText('首页看板')
  })

  test('列表页共性：表格容器与分页/空态至少渲染其一', async ({ page }) => {
    const listRoutes = ROUTES.filter(
      (route) =>
        ![
          '/dashboard',
          '/profile',
          '/screen',
          '/monitor/realtime',
          '/device/gis',
          // AI监控需先选择区域/设备才渲染风险表格
          '/risk/ai-monitor',
        ].includes(route.path)
    )

    const failures: string[] = []
    for (const route of listRoutes) {
      await gotoAdmin(page, route.path)
      await expect(page.locator(CONTENT)).toBeVisible({ timeout: 10_000 })
      const hasTable = (await page.locator(`${CONTENT} .ant-table`).count()) > 0
      const hasForm = (await page.locator(`${CONTENT} form`).count()) > 0
      const hasCards = (await page.locator(`${CONTENT} .ant-card`).count()) > 0
      if (!hasTable && !hasForm && !hasCards) {
        failures.push(`[结构缺失] ${route.path} 无表格/表单/卡片主体`)
      }
    }
    expect(failures, failures.join('\n')).toEqual([])
  })
})
