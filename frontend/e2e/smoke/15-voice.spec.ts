import { expect, gotoAdmin, test } from '../fixtures/auth'

/**
 * 15 智能语音聊天
 *
 * 默认跳过：需要本机 Pipecat 已启动（docker compose --profile voice up -d pipecat）。
 * 本地联调设置 E2E_VOICE=1 启用。
 */
test.describe('15 智能语音聊天', () => {
  test('语音 Agent 配置页与 WebSocket 握手', async ({ page }) => {
    test.skip(!process.env.E2E_VOICE, '需要本机 Pipecat（E2E_VOICE=1）')

    await gotoAdmin(page, '/voice/agent')
    await expect(page.locator('.e2e-voice-agent-tabs')).toBeVisible()

    // 切到 WebSocket Tab
    await page.getByRole('tab', { name: /WebSocket/ }).click()
    await expect(page.locator('.e2e-voice-connect-btn')).toBeVisible()

    // 获取 ticket → 连 Go 网关 → Pipecat → 服务端 hello
    await page.locator('.e2e-voice-connect-btn').click()
    await expect(page.getByText('session_id', { exact: true })).toBeVisible({ timeout: 20_000 })
    await expect(page.locator('[data-testid="e2e-voice-ws-log"]')).toContainText(/hello/i, {
      timeout: 20_000,
    })
  })

  test('MCP 工具与记忆页签可加载', async ({ page }) => {
    await gotoAdmin(page, '/voice/agent')

    await page.getByRole('tab', { name: /MCP/ }).click()
    await expect(page.locator('.e2e-voice-mcp-tool-table')).toBeVisible()

    await page.getByRole('tab', { name: /记忆/ }).click()
    await expect(page.locator('.e2e-voice-memory-table')).toBeVisible()
  })
})
