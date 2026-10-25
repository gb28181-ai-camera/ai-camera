import {
  OK_BUTTON, expect, gotoAdmin, readPermCodes, shot, test, uniqName,
} from '../fixtures/auth'
import {
  apiCall, expectApiOk, expectColumns, searchTable, tableRow,
} from '../helpers/crud'
import {
  cleanupIotFixture, closeIotModal, createIotProduct, fixedServiceGroupId,
} from '../helpers/iot'

const MODAL = '.e2e-iot-service-group-modal'
const SERVICES_MODAL = '.e2e-iot-service-group-items-modal'
const TABLE = '.e2e-iot-service-group-table'
const LIST_API = '/web/iot/service-groups'
const HINT = '请输入名称/编码'
const FIXED_CODES = ['gb28181', 'webrtc', 'aichat'] as const

/** 清空指定服务组的服务（必须在未引用状态）。 */
async function clearServices(page: import('@playwright/test').Page, groupId: number): Promise<void> {
  await apiCall(page, 'PUT', `${LIST_API}/${groupId}/services`, { services: [] })
}

test.describe('12b IOT 服务组', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/iot/service-group')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('服务组')
    await expect(page.locator(TABLE)).toBeVisible()
  })

  test('列表渲染：固定三条服务组，禁止新增/删除', async ({ page }) => {
    await expectColumns(page, ['名称', '编码', '类型', '被引用', '启用', '操作'])
    await expect(page.getByRole('button', { name: '新增服务组' })).toHaveCount(0)

    const rows = page.locator(TABLE).locator('.ant-table-row')
    await expect(rows).toHaveCount(3)
    for (const code of FIXED_CODES) {
      const row = tableRow(page, code)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText('未引用')
      await expect(row.getByRole('button', { name: /删\s*除/ })).toHaveCount(0)
    }
    await shot(page, '12b-iot-service-group-list')
  })

  test('编辑服务：添加 SIP 服务后落库，清空后恢复', async ({ page }) => {
    const groupId = await fixedServiceGroupId(page, 'gb28181')
    try {
      // 种子已预置 SIP/RTSP；先清空，确保弹窗只有本次新增的一行。
      await clearServices(page, groupId)
      await searchTable(page, HINT, 'gb28181', LIST_API)
      await tableRow(page, 'gb28181').getByRole('button', { name: '编辑服务' }).click()
      await expect(page.locator(SERVICES_MODAL)).toBeVisible()
      await page.locator(SERVICES_MODAL).getByRole('button', { name: '添加服务' }).click()

      await page.locator(SERVICES_MODAL).getByPlaceholder('如 mqtt', { exact: true }).fill('sip')
      await page.locator(SERVICES_MODAL).getByPlaceholder('如 MQTT 接入', { exact: true }).fill('SIP 接入')
      await page.locator(SERVICES_MODAL).locator('.ant-select').first().click()
      await page
        .locator('.ant-select-dropdown:visible .ant-select-item-option')
        .filter({ hasText: 'SIP' })
        .first()
        .click()
      await page.locator(SERVICES_MODAL).getByPlaceholder('服务地址').fill('sip.e2e.local')
      await page.locator(SERVICES_MODAL).getByPlaceholder('0=默认').fill('5060')

      const saved = page.waitForResponse(
        (res) => res.url().includes(`/web/iot/service-groups/${groupId}/services`)
          && res.request().method() === 'PUT'
      )
      await page.locator(SERVICES_MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await saved).status()).toBe(200)
      await expect(page.locator(SERVICES_MODAL)).toBeHidden()

      const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${groupId}`))
      const services = (detail.services ?? []) as Array<Record<string, unknown>>
      expect(services).toHaveLength(1)
      expect(services[0].code).toBe('sip')
      expect(services[0].protocol).toBe('SIP')
      expect(services[0].host).toBe('sip.e2e.local')
      await shot(page, '12b-iot-service-group-services')
    } finally {
      await clearServices(page, groupId)
    }
  })

  test('协议白名单：gb28181 拒绝 MQTT/RTSP 之外的协议，webrtc 允许 TURN', async ({ page }) => {
    const gbId = await fixedServiceGroupId(page, 'gb28181')
    const rtcId = await fixedServiceGroupId(page, 'webrtc')
    try {
      const bad = await apiCall(page, 'PUT', `${LIST_API}/${gbId}/services`, {
        services: [{ code: 'mqtt', name: 'MQTT', protocol: 'MQTT', host: 'mqtt.local', port: 1883 }],
      })
      expect(bad.status).toBe(400)
      expect(String(bad.body.message ?? '')).toContain('MQTT')

      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${rtcId}/services`, {
          services: [{ code: 'turn', name: 'TURN', protocol: 'TURN', host: 'turn.local', port: 3478 }],
        })
      )
      const detail = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}/${rtcId}`))
      expect((detail.services as unknown[]).length).toBe(1)
    } finally {
      await clearServices(page, gbId)
      await clearServices(page, rtcId)
    }
  })

  test('编辑弹窗：编码不可修改，不提供类型选择', async ({ page }) => {
    await searchTable(page, HINT, 'gb28181', LIST_API)
    await tableRow(page, 'gb28181').locator('button').filter({ hasText: /^编辑$/ }).click()
    await expect(page.locator(MODAL)).toBeVisible()
    await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('编辑服务组')
    await expect(page.locator(MODAL).locator('input#code')).toHaveValue('gb28181')
    await expect(page.locator(MODAL).locator('input#code')).toBeDisabled()
    await expect(
      page.locator(MODAL).locator('.ant-form-item').filter({ hasText: '类型' })
    ).toHaveCount(0)
    await closeIotModal(page, MODAL)
  })

  test('被产品引用的服务组只读，解绑后恢复可编辑', async ({ page }) => {
    const groupId = await fixedServiceGroupId(page, 'webrtc')
    const productId = await createIotProduct(page, {
      name: uniqName('E2E-IOT锁定产品'),
      code: `E2E_IOT_G_LOCK_P_${Date.now()}`,
      allowShare: true,
      netConfigMode: 'bluetooth',
      serviceGroupIds: [groupId],
    })

    try {
      await searchTable(page, HINT, 'webrtc', LIST_API)
      const row = tableRow(page, 'webrtc')
      await expect(row).toContainText('1 个产品')
      await expect(row.locator('button').filter({ hasText: /^编辑服务$/ })).toBeDisabled()
      await expect(row.locator('button').filter({ hasText: /^编辑$/ })).toBeDisabled()

      // 后端拒绝：即使绕过 UI 也不能修改服务。
      const res = await apiCall(page, 'PUT', `${LIST_API}/${groupId}/services`, {
        services: [{ code: 'mqtt', name: 'MQTT', protocol: 'MQTT', host: 'mqtt.lock.local', port: 1883 }],
      })
      expect(res.status).toBe(400)
      expect(String(res.body.message ?? '')).toContain('被 1 个产品引用')

      // 删除产品后（自动清理绑定与凭据）恢复可编辑。
      await cleanupIotFixture(page, { productId })
      await searchTable(page, HINT, 'webrtc', LIST_API)
      const unlocked = tableRow(page, 'webrtc')
      await expect(unlocked).toContainText('未引用')
      await expect(unlocked.locator('button').filter({ hasText: /^编辑服务$/ })).toBeEnabled()
      await shot(page, '12b-iot-service-group-locked')
    } finally {
      await cleanupIotFixture(page, { productId })
    }
  })

  test('权限码已入库：编辑权限可见，新增/删除不暴露入口', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('iot:service-group:update')
    await expect(page.getByRole('button', { name: '新增服务组' })).toHaveCount(0)
  })
})
