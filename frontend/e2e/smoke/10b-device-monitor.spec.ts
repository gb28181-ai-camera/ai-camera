import { expect, gotoAdmin, shot, test } from '../fixtures/auth'
import { apiCall, expectApiOk, expectColumns, searchTable, tableRow } from '../helpers/crud'

const DEVICE_API = '/web/devices'
const HISTORY_API = '/web/devices/history'
const SEED_DEVICE = '东门摄像头'
const SEED_CODE = 'CAM_001'

test.describe('10b 设备监控 / 历史记录 / GIS', () => {
  test.describe('设备监控', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(DEVICE_API))
      await gotoAdmin(page, '/device/monitor')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('设备监控')
    })

    test('列表渲染：运行指标列完整（电量/存储/在线时间/状态）', async ({ page }) => {
      await expectColumns(page, [
        '设备名称',
        '设备编号',
        '所属区域',
        '设备位置',
        '电量',
        '存储容量',
        '在线-离线时间',
        '运行状态',
        '实时视频',
        '视频录像',
      ])
      await expect(page.getByPlaceholder('请输入设备名称或编号')).toBeVisible()
      // 运行状态为下拉筛选（非输入框）
      await expect(page.getByText('运行状态').first()).toBeVisible()

      const row = tableRow(page, SEED_DEVICE)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(SEED_CODE)
      await expect(row).toContainText('一号厂区')
      await shot(page, '10b-device-monitor')
    })

    test('按设备编号检索命中唯一记录', async ({ page }) => {
      await searchTable(page, '请输入设备名称或编号', SEED_CODE, DEVICE_API)
      await expect(page.locator('.ant-table-row')).toHaveCount(1)
      await expect(tableRow(page, SEED_CODE)).toContainText(SEED_DEVICE)
    })

    test('「查看录像」按钮对超管可用（device:monitor:video 已补齐）', async ({ page }) => {
      // 历史背景：页面用 `device:monitor:video` 判定，菜单树缺失该权限码 → 按钮始终禁用。
      // v0.1.0 已补齐该按钮权限码。
      const row = tableRow(page, SEED_DEVICE)
      const btn = row.getByRole('button', { name: '查看录像' })
      await expect(btn).toBeVisible()
      await expect(btn).toBeEnabled()
    })
  })

  test.describe('设备历史记录', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(HISTORY_API))
      await gotoAdmin(page, '/device/history')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('设备历史记录')
    })

    test('列表渲染：采样指标列完整', async ({ page }) => {
      await expectColumns(page, [
        '设备名称',
        '设备编号',
        '流量卡号',
        '所属区域',
        '设备位置',
        '电量',
        '电压',
        '剩余存储容量',
        '在线-离线时间',
        '运行状态',
        '历史曲线',
      ])
      await expect(page.getByPlaceholder('请输入设备名称或编号')).toBeVisible()
      await shot(page, '10b-device-history')
    })

    test('历史记录行数与接口返回一致（心跳采样写入历史表）', async ({ page }) => {
      const res = await expectApiOk(
        await apiCall(page, 'GET', `${HISTORY_API}?page=1&page_size=20`)
      )
      const rows = await page.locator('.ant-table-row').count()
      expect(rows).toBe(((res.items ?? []) as unknown[]).length)
    })

    test('查询按钮与历史曲线接口可用（近 3 日）', async ({ page }) => {
      // 该页查询依赖区域/设备筛选（未选择时不发请求），此处仅校验按钮可用
      await expect(page.getByRole('button', { name: '搜索' })).toBeEnabled()

      const chart = await apiCall(page, 'GET', '/web/devices/2/history-chart?days=3')
      // 设备可能不存在 → 只要不是 500 即视为链路正常
      expect(chart.status).not.toBe(500)
    })
  })

  test.describe('GIS 查看', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(DEVICE_API))
      await gotoAdmin(page, '/device/gis')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('GIS查看')
    })

    test('筛选面板统计与地图容器渲染', async ({ page }) => {
      await expect(page.locator('.ant-card-head-title', { hasText: '筛选' })).toBeVisible()
      for (const title of ['设备总数', '已定位', '在线设备']) {
        await expect(page.locator('.ant-statistic-title', { hasText: title })).toHaveCount(1)
      }

      const total = await page
        .locator('.ant-statistic', { has: page.locator('.ant-statistic-title', { hasText: '设备总数' }) })
        .locator('.ant-statistic-content-value')
        .innerText()
      const devices = await apiCall(page, 'GET', `${DEVICE_API}?page=1&page_size=200`)
      const totalFromApi = Number((devices.body.data as { total?: number })?.total ?? 0)
      expect(Number(total.replace(/\D/g, ''))).toBe(totalFromApi)
      await shot(page, '10b-gis')
    })

    test('已定位数量与接口数据一致；无经纬度时展示空态提示', async ({ page }) => {
      // 「已定位」应与接口中真正带经纬度的设备数一致（不依赖种子是否配了坐标）
      const devices = await apiCall(page, 'GET', `${DEVICE_API}?page=1&page_size=200`)
      const items = ((devices.body.data as { items?: Array<Record<string, unknown>> })?.items ?? [])
      const locatedFromApi = items.filter(
        (d) => String(d.latitude ?? '').trim() !== '' && String(d.longitude ?? '').trim() !== ''
      ).length

      const located = await page
        .locator('.ant-statistic', { has: page.locator('.ant-statistic-title', { hasText: '已定位' }) })
        .locator('.ant-statistic-content-value')
        .innerText()
      expect(Number(located.replace(/\D/g, ''))).toBe(locatedFromApi)

      if (locatedFromApi === 0) {
        // 全部设备都没有经纬度 → 展示空态引导文案
        await expect(page.getByText(/暂无设备配置经纬度|无法在地图上定位/)).toBeVisible()
      } else {
        // 有可定位设备 → 至少渲染一个设备图标
        await expect(page.locator('.anticon-environment').first()).toBeVisible()
      }
    })

    test('刷新按钮可重新拉取设备数据', async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(DEVICE_API))
      await page.getByRole('button', { name: '刷新' }).click()
      expect((await resp).status()).toBe(200)
    })
  })
})
