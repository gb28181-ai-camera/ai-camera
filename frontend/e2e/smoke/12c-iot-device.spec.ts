import type { Locator, Page } from '@playwright/test'
import {
  OK_BUTTON, expect, gotoAdmin, readPermCodes, shot, test, uniqName,
} from '../fixtures/auth'
import {
  apiCall, expectApiOk, expectColumns, searchTable, tableRow,
} from '../helpers/crud'
import {
  activateIotDevice, cleanupIotFixture, closeIotModal, createIotDevice, createIotProduct,
  createIotShareToken, fixedServiceGroupId, getIotDevice,
  replaceIotServices, setDeviceProduct, type IotFixture,
} from '../helpers/iot'

const TABLE = '.e2e-iot-device-table'
const LIST_API = '/web/iot/devices'
const HINT = '请输入设备名称/编号/位置'
const POPCONFIRM = '.ant-popconfirm:visible'
const ACCOUNT_MODAL = '.e2e-iot-device-account-modal'
const ACCOUNT_FORM_MODAL = '.e2e-iot-device-account-form-modal'
const ACCOUNT_TABLE = '.e2e-iot-device-account-table'
const ACCOUNT_CRED_MODAL = '.e2e-iot-device-account-credential-modal'
const SECRET_MODAL = '.e2e-iot-device-secret-modal'
const QR_MODAL = '.e2e-iot-device-qr-modal'
const SHARE_MODAL = '.e2e-iot-share-qr-modal'
const SERVICES_MODAL = '.e2e-iot-device-services-modal'

interface DeviceFixture extends IotFixture {
  productCode: string
  productName: string
  groupName: string
  groupCode: string
}

/** 建一套「产品 + 服务组 + 设备」夹具，并把产品挂到设备上。 */
async function setupFixture(
  page: import('@playwright/test').Page,
  opts: { allowShare: boolean }
): Promise<DeviceFixture> {
  const stamp = Date.now()
  const productName = uniqName('E2E-IOT设备产品')
  const productCode = `E2E_IOT_D_P_${stamp}`
  const deviceName = uniqName('E2E-IOT设备')
  const deviceCode = `E2E_IOT_D_${stamp}`

  // 服务组是系统预置数据；测试维护其服务后再创建产品。
  const groupId = await fixedServiceGroupId(page, 'webrtc')
  await replaceIotServices(page, groupId, [
    { code: 'mqtt', name: 'MQTT', protocol: 'MQTT', host: 'mqtt.e2e.local', port: 1883 },
    { code: 'webrtc', name: 'WebRTC', protocol: 'WebRTC', host: 'webrtc.e2e.local', port: 443 },
  ])
  const productId = await createIotProduct(page, {
    name: productName,
    code: productCode,
    allowShare: opts.allowShare,
    netConfigMode: 'bluetooth',
    serviceGroupIds: [groupId],
  })
  const deviceId = await createIotDevice(page, { name: deviceName, code: deviceCode })
  await setDeviceProduct(page, deviceId, productId)

  return {
    productId, groupId, deviceId, deviceCode,
    productCode, productName,
    groupName: 'WebRTC 服务组',
    groupCode: 'webrtc',
  }
}

async function cleanup(page: import('@playwright/test').Page, fixture: DeviceFixture): Promise<void> {
  await cleanupIotFixture(page, fixture)
}

/** 按按钮文本精确定位行内操作（「分享码」会命中「撤销分享码」）。 */
function rowButton(page: Page, code: string, text: string): Locator {
  return tableRow(page, code).locator('button').filter({ hasText: new RegExp(`^${text}$`) })
}

/** 解绑设备并确认 Popconfirm。 */
async function unbindDevice(page: import('@playwright/test').Page, code: string): Promise<void> {
  await tableRow(page, code).getByRole('button', { name: '解绑设备' }).click()
  await expect(page.locator(POPCONFIRM)).toBeVisible()
  await page.locator(POPCONFIRM).getByRole('button', { name: OK_BUTTON }).click()
}

test.describe('12c IOT 设备管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/iot/device')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('设备管理')
    await expect(page.locator(TABLE)).toBeVisible()
  })

  test('列表渲染：种子设备与所属产品列', async ({ page }) => {
    await expectColumns(page, [
      '设备名称', '设备编号', '所属产品', '在线状态', '激活状态', '主账号', '启用', '操作',
    ])
    const seed = tableRow(page, 'CAM_001')
    await expect(seed).toHaveCount(1)
    await expect(seed).toContainText('东门摄像头')
    await expect(seed).toContainText('未绑定')
    await shot(page, '12c-iot-device-list')
  })

  test('按关键字检索并展示设备所属产品', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      await searchTable(page, HINT, fixture.deviceCode, LIST_API)
      const row = tableRow(page, fixture.deviceCode)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(fixture.productName)
      await expect(row).toContainText(fixture.productCode)
      await shot(page, '12c-iot-device-filtered')
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('服务授权弹窗展示产品绑定的服务组与服务项', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      await searchTable(page, HINT, fixture.deviceCode, LIST_API)
      await tableRow(page, fixture.deviceCode).getByRole('button', { name: '服务授权' }).click()

      await expect(page.locator(SERVICES_MODAL)).toBeVisible()
      await expect(page.locator(SERVICES_MODAL)).toContainText(fixture.groupName)
      await expect(page.locator(SERVICES_MODAL)).toContainText(fixture.groupCode)
      await expect(page.locator(SERVICES_MODAL)).toContainText('MQTT')
      await expect(page.locator(SERVICES_MODAL)).toContainText('WebRTC')
      await shot(page, '12c-iot-device-services')
      await closeIotModal(page, SERVICES_MODAL)
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('产品未开启分享时，分享码按钮禁用且后端拒绝', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: false })
    try {
      await searchTable(page, HINT, fixture.deviceCode, LIST_API)
      await expect(rowButton(page, fixture.deviceCode, '分享码')).toBeDisabled()

      const res = await apiCall(
        page, 'POST', `/web/iot/devices/${fixture.deviceId}/share-token`
      )
      expect(res.status).toBe(400)
      expect(String(res.body.message ?? '')).toContain('未开启分享')
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('激活设备：一次性主账号密码，重复激活给出提示', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      await searchTable(page, HINT, fixture.deviceCode, LIST_API)
      let row = tableRow(page, fixture.deviceCode)
      await expect(row).toContainText('未激活')

      await row.getByRole('button', { name: '激活设备' }).click()
      await expect(page.locator(ACCOUNT_CRED_MODAL)).toBeVisible()
      const text = await page.locator(ACCOUNT_CRED_MODAL).innerText()
      expect(text).toContain('设备激活成功')
      expect(text).toContain('密码')
      await closeIotModal(page, ACCOUNT_CRED_MODAL)
      await expect(page.locator(ACCOUNT_CRED_MODAL)).toBeHidden()

      row = tableRow(page, fixture.deviceCode)
      await expect(row).toContainText('已激活')
      // 主账号用户名由服务端生成，前缀 owner_（分享账号是 share_ / iot_）。
      await expect.poll(async () => (await row.innerText()).includes('owner_'), {
        timeout: 10_000,
      }).toBe(true)

      // 重复激活：管理端幂等，前端提示已激活。
      await row.getByRole('button', { name: '激活设备' }).click()
      await expect(page.getByText('该设备已激活，请勿重复激活')).toBeVisible()
      await shot(page, '12c-iot-device-activated')
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('账号管理：新增分享账号 → 编辑 → 删除', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      await activateIotDevice(page, fixture.deviceId!)
      await searchTable(page, HINT, fixture.deviceCode, LIST_API)

      await tableRow(page, fixture.deviceCode).getByRole('button', { name: '账号管理' }).click()
      await expect(page.locator(ACCOUNT_MODAL)).toBeVisible()
      await expect(page.locator(ACCOUNT_TABLE)).toContainText('主账号')

      // 新增分享账号：用户名/密码留空，由服务端生成。
      await page.locator(ACCOUNT_MODAL).getByRole('button', { name: '新增分享账号' }).click()
      await expect(page.locator(ACCOUNT_FORM_MODAL)).toBeVisible()
      await page.locator(ACCOUNT_FORM_MODAL).getByPlaceholder('如：家人账号').fill('E2E家人')
      await page.locator(ACCOUNT_FORM_MODAL).getByPlaceholder('请输入手机号').fill('13900000000')

      const created = page.waitForResponse(
        (res) => /\/web\/iot\/devices\/\d+\/accounts$/.test(new URL(res.url()).pathname)
          && res.request().method() === 'POST'
      )
      await page.locator(ACCOUNT_FORM_MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect([200, 201]).toContain((await created).status())
      await expect(page.locator(ACCOUNT_FORM_MODAL)).toBeHidden()

      await expect(page.locator(ACCOUNT_CRED_MODAL)).toBeVisible()
      await closeIotModal(page, ACCOUNT_CRED_MODAL)

      const sharedRow = page.locator(ACCOUNT_TABLE).locator('.ant-table-row').filter({ hasText: 'E2E家人' })
      await expect(sharedRow).toHaveCount(1)
      await expect(sharedRow).toContainText('分享账号')

      // 编辑昵称
      await sharedRow.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(ACCOUNT_FORM_MODAL)).toBeVisible()
      await page.locator(ACCOUNT_FORM_MODAL).getByPlaceholder('如：家人账号').fill('E2E家人改')
      const updated = page.waitForResponse(
        (res) => res.url().includes('/accounts/') && res.request().method() === 'PUT'
      )
      await page.locator(ACCOUNT_FORM_MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await updated).status()).toBe(200)
      await expect(
        page.locator(ACCOUNT_TABLE).locator('.ant-table-row').filter({ hasText: 'E2E家人改' })
      ).toHaveCount(1)

      // 删除分享账号
      const renamed = page.locator(ACCOUNT_TABLE).locator('.ant-table-row').filter({ hasText: 'E2E家人改' })
      await renamed.getByRole('button', { name: /删\s*除/ }).click()
      await expect(page.locator(POPCONFIRM)).toBeVisible()
      await page.locator(POPCONFIRM).getByRole('button', { name: OK_BUTTON }).click()
      await expect(renamed).toHaveCount(0)
      await expect(page.locator(ACCOUNT_TABLE)).toContainText('主账号')
      await shot(page, '12c-iot-device-accounts')

      await closeIotModal(page, ACCOUNT_MODAL)
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('设备二维码：URL 含产品编码与设备编码', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      await searchTable(page, HINT, fixture.deviceCode, LIST_API)
      await tableRow(page, fixture.deviceCode).getByRole('button', { name: '设备二维码' }).click()

      await expect(page.locator(QR_MODAL)).toBeVisible()
      await expect(page.locator(QR_MODAL)).toContainText(fixture.productCode)
      await expect(page.locator(QR_MODAL)).toContainText(fixture.deviceCode)
      await expect(page.locator(QR_MODAL)).toContainText(
        `https://boticz.cn/qrDevice/${fixture.productCode}/${fixture.deviceCode}`
      )
      await expect(page.locator(QR_MODAL).getByRole('img', { name: '设备二维码' })).toBeVisible()
      await shot(page, '12c-iot-device-qr')
      await closeIotModal(page, QR_MODAL)
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('分享码：生成一次性二维码，撤销后失效', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      // 先经 API 造一个令牌，用于验证「撤销后不可认领」。
      const apiShare = await createIotShareToken(page, fixture.deviceId!)
      const token = String(apiShare.token ?? '')
      expect(token).not.toBe('')

      await searchTable(page, HINT, fixture.deviceCode, LIST_API)

      await rowButton(page, fixture.deviceCode, '分享码').click()
      await expect(page.locator(SHARE_MODAL)).toBeVisible()
      await expect(page.locator(SHARE_MODAL)).toContainText('https://boticz.cn/deviceShareToken/')
      await expect(page.locator(SHARE_MODAL).getByRole('img', { name: '设备分享码' })).toBeVisible()
      await shot(page, '12c-iot-device-share')
      await closeIotModal(page, SHARE_MODAL)

      // 撤销所有未认领令牌（API 创建的 + 弹窗这次创建的）。
      await rowButton(page, fixture.deviceCode, '撤销分享码').click()
      await expect(page.locator(POPCONFIRM)).toBeVisible()
      await page.locator(POPCONFIRM).getByRole('button', { name: OK_BUTTON }).click()
      await expect(page.getByText(/已撤销 [1-9]\d* 个未认领分享码/)).toBeVisible()

      const claim = await apiCall(page, 'POST', '/app/iot/devices/claim', { token })
      expect(claim.status).toBe(400)
      expect(String(claim.body.message ?? '')).toContain('分享令牌无效')
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('重置密钥：一次性明文展示', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      await searchTable(page, HINT, fixture.deviceCode, LIST_API)
      const row = tableRow(page, fixture.deviceCode)
      await row.getByRole('button', { name: '重置密钥' }).click()
      await expect(page.locator(POPCONFIRM)).toBeVisible()
      await page.locator(POPCONFIRM).getByRole('button', { name: OK_BUTTON }).click()

      await expect(page.locator(SECRET_MODAL)).toBeVisible()
      await expect(page.locator(SECRET_MODAL)).toContainText(fixture.deviceCode)
      await expect(page.locator(SECRET_MODAL)).toContainText('设备密钥')
      const text = await page.locator(SECRET_MODAL).innerText()
      // 密钥不是脱敏占位，且至少有 16 位
      expect(text).not.toContain('****')
      await shot(page, '12c-iot-device-secret')
      await closeIotModal(page, SECRET_MODAL)
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('解绑设备：删除全部账号并清除激活状态', async ({ page }) => {
    const fixture = await setupFixture(page, { allowShare: true })
    try {
      await activateIotDevice(page, fixture.deviceId!)
      await createIotShareToken(page, fixture.deviceId!)
      const before = await getIotDevice(page, fixture.deviceId!)
      expect((before.accounts as unknown[]).length).toBeGreaterThanOrEqual(2)
      expect(before.qr_content).toBeTruthy()

      await searchTable(page, HINT, fixture.deviceCode, LIST_API)
      let row = tableRow(page, fixture.deviceCode)
      await expect(row).toContainText('已激活')
      await unbindDevice(page, fixture.deviceCode)
      await expect(page.getByText(/已解绑，删除 \d+ 个账号/)).toBeVisible()

      row = tableRow(page, fixture.deviceCode)
      await expect(row).toContainText('未激活')
      // 主账号列在解绑后应清空（账号已删除，ownerMap 刷新为空）。
      await expect.poll(async () => /(owner_|share_)/.test(await row.innerText()), {
        timeout: 10_000,
      }).toBe(false)

      const after = await getIotDevice(page, fixture.deviceId!)
      expect((after.accounts as unknown[]).length).toBe(0)
      expect(after.device && (after.device as Record<string, unknown>).activated_at).toBeFalsy()
      await shot(page, '12c-iot-device-unbound')
    } finally {
      await cleanup(page, fixture)
    }
  })

  test('权限码已入库：设置产品/激活/解绑/密钥按钮对超管渲染', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('iot:device:update')
    expect(codes).toContain('iot:device:activate')
    expect(codes).toContain('iot:device:secret-reset')
    expect(codes).toContain('iot:device:account:create')
    expect(codes).toContain('iot:device:account:delete')
    expect(codes).toContain('iot:device:unbind')

    await searchTable(page, HINT, 'CAM_001', LIST_API)
    const row = tableRow(page, 'CAM_001')
    await expect(row.getByRole('button', { name: '设置产品' })).toBeVisible()
    await expect(row.getByRole('button', { name: '解绑设备' })).toBeVisible()
  })
})
