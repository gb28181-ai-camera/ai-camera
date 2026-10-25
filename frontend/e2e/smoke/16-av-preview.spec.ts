import { expect, gotoAdmin, test } from '../fixtures/auth'

/**
 * 16 实时音视频 → 实时预览
 *
 * 默认跳过：需要本机 MQTT broker、Go 后端与 tools/webrtcsim 已启动。
 * 本地联调：
 *   docker compose --profile iot up -d mqtt
 *   cd backend && go run ./tools/webrtcsim --broker tcp://127.0.0.1:1883 ...
 *   E2E_AV=1 E2E_AV_VIDEO=1 npx playwright test e2e/smoke/16-av-preview.spec.ts
 *
 * H.264 解码依赖浏览器构建：Playwright 自带 Chromium 往往不含专有编解码器，
 * 因此“画面帧”断言只在 E2E_AV_VIDEO=1 时启用，并建议用系统 Chrome：
 *   E2E_BROWSER_CHANNEL=chrome
 */
test.describe('16 实时音视频实时预览', () => {
  test.skip(!process.env.E2E_AV, '需要本机 MQTT + webrtcsim（E2E_AV=1）')

  test('连接设备并完成 MQTT 房间信令', async ({ page }) => {
    await gotoAdmin(page, '/av/preview')
    await expect(page.locator('.e2e-av-preview')).toBeVisible()

    // 下拉默认选中第一台 WebRTC 设备。
    await page.locator('.e2e-av-device-select').click()
    await page.keyboard.press('Enter')

    await page.locator('.e2e-av-connect-btn').click()
    await expect(page.locator('[data-testid="e2e-av-log"]')).toContainText('ready', {
      timeout: 15_000,
    })
    await expect(page.locator('[data-testid="e2e-av-log"]')).toContainText('offer', {
      timeout: 20_000,
    })
  })

  test('真实视频帧（H.264）', async ({ page }) => {
    test.skip(!process.env.E2E_AV_VIDEO, 'H.264 需系统 Chrome（E2E_AV_VIDEO=1）')

    await gotoAdmin(page, '/av/preview')
    await page.locator('.e2e-av-device-select').click()
    await page.keyboard.press('Enter')
    await page.locator('.e2e-av-connect-btn').click()

    const video = page.locator('.e2e-av-video')
    await expect
      .poll(async () => video.evaluate((el: HTMLVideoElement) => el.videoWidth), {
        timeout: 30_000,
        message: '等待视频首帧',
      })
      .toBeGreaterThan(0)

    await expect(page.locator('[data-testid="e2e-av-log"]')).toContainText(
      /connection=connected|ice=connected/,
      { timeout: 20_000 },
    )
  })
})
