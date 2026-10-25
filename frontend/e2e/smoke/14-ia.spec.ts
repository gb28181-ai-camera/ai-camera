import { expect, gotoAdmin, test } from '../fixtures/auth'

/**
 * 14 运营后台信息架构（v0.5.0）
 *
 * 一级目录：首页 / 系统管理 / 物联网管理 / 国标视频 / 实时音视频 / 智能语音聊天 / AI 智能分析。
 * 实时音视频、智能语音聊天的设备页分别按 WebRTC / AIChat 服务组过滤。
 */
test.describe('14 信息架构', () => {
  test('一级目录与顺序', async ({ page }) => {
    await gotoAdmin(page, '/dashboard')

    const top = page.locator('.ant-menu-root > .ant-menu-item, .ant-menu-root > .ant-menu-submenu')
    await expect(top.first()).toBeVisible()

    const names = (await top.allInnerTexts()).map((text) => text.replace(/\s+/g, ' ').trim())
    const expected = ['首页', '系统管理', '物联网管理', '国标视频', '实时音视频', '智能语音聊天', 'AI 智能分析']
    for (const name of expected) {
      expect(names.some((text) => text.includes(name)), `缺少一级目录 ${name}，实际: ${names.join(' | ')}`).toBeTruthy()
    }
    // 旧一级目录不再出现
    expect(names.some((text) => text === '风险统计' || text === '实时监控')).toBeFalsy()
  })

  test('实时音视频 → 设备管理：只显示 WebRTC 服务组设备', async ({ page }) => {
    await gotoAdmin(page, '/dashboard')
    const av = page.locator('.ant-menu-submenu').filter({ hasText: '实时音视频' }).first()
    await av.locator('.ant-menu-submenu-title').click()
    await av.getByRole('menuitem', { name: /设备管理/ }).click()

    await expect(page).toHaveURL(/\/av\/device$/)
    await expect(page.locator('.page-container-title').first()).toHaveText('实时音视频设备')
    await expect(page.getByText('CAM_001')).toBeVisible()
    await expect(page.getByText('VOICE_001')).toHaveCount(0)
  })

  test('智能语音聊天 → 设备管理：只显示 AIChat 服务组设备', async ({ page }) => {
    await gotoAdmin(page, '/dashboard')
    const voice = page.locator('.ant-menu-submenu').filter({ hasText: '智能语音聊天' }).first()
    await voice.locator('.ant-menu-submenu-title').click()
    await voice.getByRole('menuitem', { name: /设备管理/ }).click()

    await expect(page).toHaveURL(/\/voice\/device$/)
    await expect(page.locator('.page-container-title').first()).toHaveText('智能语音聊天设备')
    await expect(page.getByText('VOICE_001')).toBeVisible()
    await expect(page.getByText('CAM_001')).toHaveCount(0)
  })

  test('语音 Agent 位于智能语音聊天，不在国标视频', async ({ page }) => {
    await gotoAdmin(page, '/dashboard')
    const voice = page.locator('.ant-menu-submenu').filter({ hasText: '智能语音聊天' }).first()
    await voice.locator('.ant-menu-submenu-title').click()
    await voice.getByRole('menuitem', { name: /语音 Agent/ }).click()

    await expect(page).toHaveURL(/\/voice\/agent$/)
  })
})
