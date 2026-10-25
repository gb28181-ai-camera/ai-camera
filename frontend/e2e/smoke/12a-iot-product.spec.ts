import {
  OK_BUTTON, expect, gotoAdmin, readPermCodes, shot, test, uniqName,
} from '../fixtures/auth'
import {
  apiCall, confirmRowDelete, expectApiOk, expectColumns, searchTable, tableRow,
} from '../helpers/crud'
import {
  cleanupIotFixture, closeIotModal, fixedServiceGroupId, iotIdByName, selectIotOption,
} from '../helpers/iot'

const MODAL = '.e2e-iot-product-modal'
const SHARE_MODAL = '.e2e-iot-product-share-modal'
const CRED_MODAL = '.e2e-iot-credentials-modal'
const TABLE = '.e2e-iot-product-table'
const LIST_API = '/web/iot/products'
const HINT = '请输入产品名称/编码/分类'
const SEED_SECURITY = '安防相机'
const SEED_ELDERLY = '适老化智能相机'

/** 在表单中按 label 找到 antd Switch 并切换。 */
async function toggleSwitch(
  page: import('@playwright/test').Page,
  modalClass: string,
  label: string
): Promise<void> {
  const item = page
    .locator(modalClass)
    .locator('.ant-form-item')
    .filter({ has: page.locator(`label:text-is("${label}")`) })
    .first()
  await item.locator('.ant-switch').click()
}

/** 在产品表单中多选系统预置服务组。 */
async function selectProductGroups(
  page: import('@playwright/test').Page,
  labels: string[]
): Promise<void> {
  const select = page.locator(MODAL).locator('.e2e-iot-product-groups-select')
  await select.click()
  for (const label of labels) {
    await page
      .locator('.ant-select-dropdown:visible .ant-select-item-option')
      .filter({ hasText: label })
      .first()
      .click()
  }
  await page.keyboard.press('Escape')
  for (const label of labels) {
    await expect(select).toContainText(label)
  }
}

test.describe('12a IOT 产品管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse(
      (res) => res.url().includes(LIST_API) && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/iot/product')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('产品管理')
    await expect(page.locator(TABLE)).toBeVisible()
  })

  test('列表渲染：列定义与两个种子产品', async ({ page }) => {
    await expectColumns(page, [
      '产品名称', '产品编码', '分类', '自动联网（4G）', '配网方式',
      '允许分享', '绑定服务组数', '启用状态', '操作',
    ])

    const security = tableRow(page, 'security_camera')
    await expect(security).toHaveCount(1)
    await expect(security).toContainText(SEED_SECURITY)
    await expect(security).toContainText('二维码')

    const elderly = tableRow(page, 'elderly_camera')
    await expect(elderly).toHaveCount(1)
    await expect(elderly).toContainText(SEED_ELDERLY)
    await expect(elderly).toContainText('蓝牙')

    await shot(page, '12a-iot-product-list')
  })

  test('新增 → 编辑 → 删除产品（校验字段保存）', async ({ page }) => {
    const name = uniqName('E2E-IOT产品')
    const renamed = `${name}-改`
    const code = `E2E_IOT_P_${Date.now()}`
    const fixture: { productId?: number } = {}

    try {
      await page.getByRole('button', { name: '新增产品' }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增产品')

      await page.locator(MODAL).getByPlaceholder('如：4G 球机').fill(name)
      await page.locator(MODAL).getByPlaceholder('如：CAM_4G_01').fill(code)
      await page.locator(MODAL).getByPlaceholder('如：摄像头').fill('E2E分类')
      await selectIotOption(page, page.locator(MODAL), '配网方式', 'AP 配网')
      await toggleSwitch(page, MODAL, '允许分享')
      await selectProductGroups(page, ['WebRTC 服务组'])

      const created = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      const createdResp = await created
      expect([200, 201]).toContain(createdResp.status())

      fixture.productId = await iotIdByName(page, LIST_API, code, 'code')

      await searchTable(page, HINT, code, LIST_API)
      const row = tableRow(page, code)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(name)
      await expect(row).toContainText('E2E分类')
      await expect(row).toContainText('AP')
      await expect(row).toContainText('是')
      await expect(row.locator('td').nth(6)).toHaveText('1')

      // 编辑：名称变更后重新检索
      await row.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('编辑产品')
      await expect(page.locator(MODAL).getByPlaceholder('如：4G 球机')).toHaveValue(name)
      await page.locator(MODAL).getByPlaceholder('如：4G 球机').fill(renamed)

      const updated = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await updated).status()).toBe(200)
      await expect(page.locator(MODAL)).toBeHidden()

      await searchTable(page, HINT, code, LIST_API)
      await expect(tableRow(page, code)).toContainText(renamed)
      await shot(page, '12a-iot-product-updated')

      await confirmRowDelete(page, tableRow(page, code))
      await expect(tableRow(page, code)).toHaveCount(0)
    } finally {
      if (fixture.productId) {
        await cleanupIotFixture(page, { productId: fixture.productId })
      }
    }
  })

  test('分享二维码：允许分享可生成，未开启分享时按钮禁用', async ({ page }) => {
    await searchTable(page, HINT, 'elderly_camera', LIST_API)
    const elderly = tableRow(page, 'elderly_camera')
    await expect(elderly.getByRole('button', { name: '分享二维码' })).toBeEnabled()
    await elderly.getByRole('button', { name: '分享二维码' }).click()

    await expect(page.locator(SHARE_MODAL)).toBeVisible()
    await expect(page.locator(SHARE_MODAL).getByRole('img', { name: '产品分享二维码' })).toBeVisible()
    await expect(page.locator(SHARE_MODAL)).toContainText('iot_product_share')
    await expect(page.locator(SHARE_MODAL)).toContainText('elderly_camera')
    await shot(page, '12a-iot-product-share-qr')
    await closeIotModal(page, SHARE_MODAL)

    await searchTable(page, HINT, 'security_camera', LIST_API)
    const security = tableRow(page, 'security_camera')
    await expect(security.getByRole('button', { name: '分享二维码' })).toBeDisabled()

    // 后端同样拒绝（不能只依赖前端禁用）。
    const id = await iotIdByName(page, LIST_API, 'security_camera', 'code')
    const res = await apiCall(page, 'POST', `${LIST_API}/${id}/share-qr`)
    expect(res.status).toBe(400)
    expect(String(res.body.message ?? '')).toContain('未开启分享')
  })

  test('复制鉴权：弹窗含 JSON/文本复制与密码显示切换', async ({ page }) => {
    await searchTable(page, HINT, 'security_camera', LIST_API)
    const row = tableRow(page, 'security_camera')
    await row.getByRole('button', { name: '复制鉴权' }).click()

    await expect(page.locator(CRED_MODAL)).toBeVisible()
    await expect(page.locator(CRED_MODAL).getByRole('button', { name: '复制 JSON' })).toBeVisible()
    await expect(page.locator(CRED_MODAL).getByRole('button', { name: '复制文本' })).toBeVisible()
    await expect(page.locator(CRED_MODAL).getByRole('button', { name: '隐藏密码' })).toBeVisible()
    await expect(page.locator(CRED_MODAL)).toContainText('该产品尚未绑定服务组')
    await shot(page, '12a-iot-product-credentials')

    await closeIotModal(page, CRED_MODAL)
  })

  test('新建产品时选择服务组，绑定数变为 1', async ({ page }) => {
    const productName = uniqName('E2E-IOT绑定产品')
    const productCode = `E2E_IOT_BIND_P_${Date.now()}`
    let productId: number | undefined
    const webrtcGroupId = await fixedServiceGroupId(page, 'webrtc')

    try {
      await page.getByRole('button', { name: '新增产品' }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await page.locator(MODAL).getByPlaceholder('如：4G 球机').fill(productName)
      await page.locator(MODAL).getByPlaceholder('如：CAM_4G_01').fill(productCode)
      await selectProductGroups(page, ['WebRTC 服务组'])

      const created = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect([200, 201]).toContain((await created).status())
      await expect(page.locator(MODAL)).toBeHidden()

      productId = await iotIdByName(page, LIST_API, productCode, 'code')
      await searchTable(page, HINT, productCode, LIST_API)
      await expect(tableRow(page, productCode).locator('td').nth(6)).toHaveText('1')

      const bound = await expectApiOk(
        await apiCall(page, 'GET', `${LIST_API}/${productId}/service-groups`)
      )
      const boundList = (bound as unknown as Array<Record<string, unknown>>)
      expect(boundList).toHaveLength(1)
      expect(boundList[0].id).toBe(webrtcGroupId)
      expect(boundList[0].code).toBe('webrtc')
    } finally {
      await cleanupIotFixture(page, { productId })
    }
  })

  test('权限码已入库：新增/编辑/删除按钮对超管渲染', async ({ page }) => {
    const codes = await readPermCodes(page)
    expect(codes).toContain('iot:product:create')
    expect(codes).toContain('iot:product:update')
    expect(codes).toContain('iot:product:delete')
    await expect(page.getByRole('button', { name: '新增产品' })).toBeVisible()
  })
})
