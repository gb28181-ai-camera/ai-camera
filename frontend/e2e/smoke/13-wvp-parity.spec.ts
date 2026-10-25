import { expect, gotoAdmin, test } from '../fixtures/auth'

/**
 * 13 WVP 功能对齐（v0.4.0）冒烟
 *
 * 只验证「页面能打开、关键区块渲染、接口 200」，不依赖录像/流等测试数据：
 *  - 平台信息 / 实时日志：新增页面
 *  - 云端录像：合并任务 / 分享入口
 *  - 电子地图：Leaflet 容器
 *  - 免登录分享：无效令牌的匿名错误态（不应跳登录页）
 */
test.describe('13 WVP 功能对齐', () => {
  test('平台信息：版本与 SIP/节点状态渲染', async ({ page }) => {
    const resp = page.waitForResponse(
      (res) => res.url().includes('/web/gb/platform-info') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/ops/platform-info')
    await expect(page.locator('.e2e-gb-platform-info')).toBeVisible()
    expect((await resp).status()).toBe(200)
    await expect(page.getByText('v0.4.0').first()).toBeVisible()
    await expect(page.getByText('SIP 信令')).toBeVisible()
    await expect(page.getByText('默认媒体节点')).toBeVisible()
  })

  test('实时日志：检索区与 SSE 状态渲染', async ({ page }) => {
    const resp = page.waitForResponse(
      (res) => res.url().includes('/web/gb/logs?') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/ops/logs')
    await expect(page.locator('.e2e-gb-log-page')).toBeVisible()
    expect((await resp).status()).toBe(200)
    await expect(page.locator('.e2e-gb-log-live-status')).toBeVisible()
  })

  test('云端录像：合并任务与分享入口', async ({ page }) => {
    await gotoAdmin(page, '/gb/media/cloud-record')
    await expect(page.getByRole('button', { name: /合并任务/ })).toBeVisible()
    await expect(page.getByRole('button', { name: /分享/ })).toBeVisible()
  })

  test('电子地图：Leaflet 容器渲染', async ({ page }) => {
    await gotoAdmin(page, '/device/gis')
    await expect(page.locator('.e2e-gis-map')).toBeVisible()
    await expect(page.locator('.leaflet-container').first()).toBeVisible()
  })

  test('免登录分享：无效令牌显示错误态而不跳登录', async ({ page }) => {
    await page.goto('share/gb/invalid-token-for-smoke')
    await expect(page.locator('.e2e-gb-share-play-page')).toBeVisible()
    await expect(page.locator('.e2e-gb-share-play-error')).toBeVisible()
    await expect(page).toHaveURL(/\/share\/gb\/invalid-token-for-smoke/)
  })

  /**
   * 真实流首帧需要本地 gbsim 与已同步的模拟通道；
   * CI/测试库没有这些数据，默认跳过，本地联调时设置 E2E_GB_STREAM=1 启用。
   */
  test('实时播放：5 引擎页签与模拟通道首帧', async ({ page }) => {
    test.skip(!process.env.E2E_GB_STREAM, '需要本地 gbsim 与模拟通道（E2E_GB_STREAM=1）')

    await gotoAdmin(page, '/gb/media/live')
    const tree = page.locator('.e2e-gb-channel-tree')
    await expect(tree).toBeVisible()

    // 设备节点默认折叠：点 switcher 展开（点标题是选中，不是展开）。
    const deviceNode = tree.locator('.ant-tree-treenode').filter({ hasText: '模拟国标相机' }).first()
    await deviceNode.locator('.ant-tree-switcher').click()
    await tree.getByText('模拟国标相机通道1').click()
    await page.getByRole('button', { name: /播放/ }).first().click()

    const player = page.locator('.e2e-live-player-tabs')
    await expect(player).toBeVisible()
    for (const engine of ['webrtc', 'flv', 'jessibuca', 'h265web', 'hls']) {
      await expect(page.locator(`.e2e-player-tab-${engine}`)).toBeVisible()
    }
    await expect(player).toHaveAttribute('data-first-frame', 'true', { timeout: 25_000 })
  })
})
