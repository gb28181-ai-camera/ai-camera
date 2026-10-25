import { expect, gotoAdmin, test } from '../fixtures/auth'
import { apiCall, expectApiOk, tableRow } from '../helpers/crud'
import {
  cleanupIotFixture, createIotDevice, createIotProduct, fixedServiceGroupId, setDeviceProduct,
} from '../helpers/iot'

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
    // 列表新增「服务组」列，WebRTC 专用页的设备行应展示 WebRTC 标签
    await expect(page.locator('.e2e-iot-device-table')).toContainText('服务组')
    await expect(tableRow(page, 'CAM_001')).toContainText('WebRTC')
    // 固定页隐藏「服务组」筛选项
    await expect(page.locator('.search-bar')).not.toContainText('服务组')
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
    await expect(tableRow(page, 'VOICE_001')).toContainText('AIChat')
    await expect(page.locator('.search-bar')).not.toContainText('服务组')
  })

  test('同一设备绑定多个服务组：在 IOT / 语音 / 国标设备页都能看到', async ({ page }) => {
    const stamp = Date.now()
    const productCode = `E2E_IA_P_${stamp}`
    const productName = `E2E-IA多组产品_${stamp}`
    const deviceCode = `E2E_IA_D_${stamp}`
    const deviceName = `E2E-IA多组设备_${stamp}`

    const gbGroupId = await fixedServiceGroupId(page, 'gb28181')
    const aiGroupId = await fixedServiceGroupId(page, 'aichat')
    const productId = await createIotProduct(page, {
      name: productName,
      code: productCode,
      serviceGroupIds: [gbGroupId, aiGroupId],
    })
    const deviceId = await createIotDevice(page, { name: deviceName, code: deviceCode })

    try {
      await setDeviceProduct(page, deviceId, productId)
      // 生成国标编码：设备随后会出现在国标设备列表
      await expectApiOk(await apiCall(page, 'POST', `/web/gb/devices/${deviceId}/activation`))

      // 1) IOT 通用设备页：一台设备同时展示 GB28181 + AIChat
      await gotoAdmin(page, `/iot/device?keyword=${deviceCode}`)
      const iotRow = tableRow(page, deviceCode)
      await expect(iotRow).toHaveCount(1)
      const iotTags = iotRow.locator('.e2e-iot-device-service-groups')
      await expect(iotTags).toContainText('GB28181')
      await expect(iotTags).toContainText('AIChat')
      await expect(iotRow.getByRole('button', { name: '国标设备' })).toBeVisible()

      // 2) 智能语音聊天页：按 AIChat 过滤，仍能看到同一台设备
      await gotoAdmin(page, `/voice/device?keyword=${deviceCode}`)
      const voiceRow = tableRow(page, deviceCode)
      await expect(voiceRow).toHaveCount(1)
      await expect(voiceRow.locator('.e2e-iot-device-service-groups')).toContainText('AIChat')

      // 3) 国标设备页：所属产品与服务组两列同时展示
      await gotoAdmin(page, `/gb/device/list?keyword=${deviceCode}`)
      const gbRow = tableRow(page, deviceCode)
      await expect(gbRow).toHaveCount(1)
      await expect(gbRow).toContainText(productName)
      const gbTags = gbRow.locator('.e2e-gb-device-service-groups')
      await expect(gbTags).toContainText('GB28181')
      await expect(gbTags).toContainText('AIChat')
    } finally {
      await cleanupIotFixture(page, { deviceId, productId })
    }
  })

  test('语音 Agent 位于智能语音聊天，不在国标视频', async ({ page }) => {
    await gotoAdmin(page, '/dashboard')
    const voice = page.locator('.ant-menu-submenu').filter({ hasText: '智能语音聊天' }).first()
    await voice.locator('.ant-menu-submenu-title').click()
    await voice.getByRole('menuitem', { name: /语音 Agent/ }).click()

    await expect(page).toHaveURL(/\/voice\/agent$/)
  })
})
