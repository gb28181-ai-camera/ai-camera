import { OK_BUTTON, expect, gotoAdmin, readPermCodes, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  confirmRowDelete,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  searchTable,
  selectFormOption,
  tableRow,
} from '../helpers/crud'

const MODAL = '.e2e-iot-card-modal'
const REPLACE_MODAL = '.e2e-iot-card-replace-modal'
const LIST_API = '/web/devices/iot-cards'
const REPLACE_API = '/web/devices/iot-card-replace-logs'
const HINT = '请输入卡名称或卡号'
const SEED_REGION = '一号厂区'

async function iotCardIdOf(page: import('@playwright/test').Page, cardNo: string): Promise<number> {
  const data = await expectApiOk(
    await apiCall(
      page,
      'GET',
      `${LIST_API}?page=1&page_size=200&keyword=${encodeURIComponent(cardNo)}`
    )
  )
  const hit = ((data.items ?? []) as { id: number; card_no: string }[]).find(
    (i) => i.card_no === cardNo
  )
  if (!hit) throw new Error(`未查询到物联网卡：${cardNo}`)
  return hit.id
}

async function createIotCard(
  page: import('@playwright/test').Page,
  name: string,
  cardNo: string
): Promise<'ui' | 'api'> {
  if (!(await hasButton(page, '新增物联网卡'))) {
    await expectApiOk(
      await apiCall(page, 'POST', LIST_API, {
        name,
        card_no: cardNo,
        region_id: 1,
        traffic_total: 1024,
        traffic_used: 100,
        traffic_left: 924,
      })
    )
    return 'api'
  }

  await page.getByRole('button', { name: '新增物联网卡' }).click()
  await expect(page.locator(MODAL)).toBeVisible()
  await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增物联网卡')
  await page.locator(MODAL).getByPlaceholder('如：移动卡A').fill(name)
  await page.locator(MODAL).getByPlaceholder('如：8986001234567890').fill(cardNo)
  await selectFormOption(page, page.locator(MODAL), '所属区域', SEED_REGION)
  await page.locator(MODAL).getByPlaceholder('单位 MB').first().fill('1024')

  const resp = page.waitForResponse(
    (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
  )
  await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
  // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
  expect([200, 201]).toContain((await resp).status())
  await expect(page.locator(MODAL)).toBeHidden()
  return 'ui'
}

async function deleteIotCard(page: import('@playwright/test').Page, cardNo: string): Promise<void> {
  await searchTable(page, HINT, cardNo, LIST_API)
  const row = tableRow(page, cardNo)
  if ((await row.count()) === 1 && (await hasButton(page, /删\s*除/))) {
    await confirmRowDelete(page, row)
    return
  }
  await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${await iotCardIdOf(page, cardNo)}`))
}

test.describe('03b 物联网卡管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/settings/iot-card')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('物联网卡管理')
  })

  test('列表渲染：列定义完整（含流量与关联区域/设备名称）', async ({ page }) => {
    await expectColumns(page, [
      '物联网卡名称',
      '卡号',
      '所属区域',
      '所属设备',
      '流量总额',
      '剩余流量',
      '到期时间',
      '状态',
      '操作',
    ])
    // 初始无卡数据 → 空态
    await expect(page.locator('.ant-table')).toBeVisible()
    await shot(page, '03b-iot-card-list')
  })

  test('新增物联网卡后列表出现该记录（流量字段正确）', async ({ page }) => {
    const name = uniqName('E2E卡')
    const cardNo = `E2E${Date.now()}`
    const path = await createIotCard(page, name, cardNo)

    await searchTable(page, HINT, cardNo, LIST_API)
    const row = tableRow(page, cardNo)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(name)
    if (path === 'ui') {
      await expect(row).toContainText(SEED_REGION)
      // 流量列按 GB 格式化展示
      await expect(row).toContainText('GB')
    }
    await shot(page, '03b-iot-card-created')

    await deleteIotCard(page, cardNo)
  })

  test('编辑物联网卡后名称已变更', async ({ page }) => {
    const name = uniqName('E2E改卡')
    const renamed = `${name}_新`
    const cardNo = `E2EEDIT${Date.now()}`
    await createIotCard(page, name, cardNo)

    await searchTable(page, HINT, cardNo, LIST_API)
    const row = tableRow(page, cardNo)
    await expect(row).toHaveCount(1)

    const id = await iotCardIdOf(page, cardNo)
    if (await hasButton(page, /编\s*辑/)) {
      await row.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('编辑物联网卡')
      await expect(page.locator(MODAL).getByPlaceholder('如：移动卡A')).toHaveValue(name)
      await page.locator(MODAL).getByPlaceholder('如：移动卡A').fill(renamed)
      const resp = page.waitForResponse(
        (res) => res.url().includes(LIST_API) && res.request().method() === 'PUT'
      )
      await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
      expect((await resp).status()).toBe(200)
      await expect(page.locator(MODAL)).toBeHidden()
    } else {
      await expectApiOk(
        await apiCall(page, 'PUT', `${LIST_API}/${id}`, {
          name: renamed,
          card_no: cardNo,
          region_id: 1,
        })
      )
    }

    await searchTable(page, HINT, cardNo, LIST_API)
    await expect(tableRow(page, cardNo)).toContainText(renamed)

    await deleteIotCard(page, cardNo)
  })

  test('删除物联网卡后该记录消失', async ({ page }) => {
    const cardNo = `E2EDEL${Date.now()}`
    await createIotCard(page, uniqName('E2E删卡'), cardNo)
    await searchTable(page, HINT, cardNo, LIST_API)
    await expect(tableRow(page, cardNo)).toHaveCount(1)

    await deleteIotCard(page, cardNo)

    await searchTable(page, HINT, cardNo, LIST_API)
    await expectEmptyTable(page)
  })

  test('换卡记录登记弹窗可打开（含设备选择与换卡记录列表）', async ({ page }) => {
    const logResp = page.waitForResponse((res) => res.url().includes(REPLACE_API))
    await gotoAdmin(page, '/settings/iot-card-replace')
    await logResp
    await expect(page.locator('.page-container-title').first()).toHaveText('物联网卡更换记录')

    await expectColumns(page, [
      '设备名称',
      '设备编号',
      '所属区域',
      '设备位置',
      '原物联网卡',
      '新物联网卡',
      '更换时间',
      '操作人',
    ])

    // v0.1.0 已修复权限码不一致：页面判定用的 `iot-card:replace` 已作为按钮菜单行补齐，
    // 「更换记录登记」按钮对超管渲染
    const codes = await readPermCodes(page)
    expect(codes).toContain('iot-card:replace')
    await expect(page.getByRole('button', { name: '更换记录登记' })).toBeVisible()
    await expect(page.locator('.page-container-extra button')).toHaveCount(1)

    await page.getByRole('button', { name: '更换记录登记' }).click()
    await expect(page.locator(REPLACE_MODAL)).toBeVisible()
    await expect(page.locator(REPLACE_MODAL).locator('.ant-modal-title')).toHaveText('更换记录登记')

    // 未选设备直接提交 → 提示先选择设备（页面前置校验）
    await page.locator(REPLACE_MODAL).getByRole('button', { name: OK_BUTTON }).click()
    await expect(page.locator('.ant-message')).toContainText('请先选择需要换卡的设备')

    // 通过设备选择弹窗选中种子设备（东门摄像头 / CAM_001）
    await page.locator('.e2e-iot-card-replace-device-btn').click()
    const picker = page.locator('.e2e-device-select-modal')
    await expect(picker).toBeVisible()
    await picker.locator('.ant-table-row').filter({ hasText: '东门摄像头' }).first().click()
    await picker.getByRole('button', { name: OK_BUTTON }).click()
    await expect(picker).toBeHidden()

    // 设备已选、新物联网卡未选 → 必填校验
    await page.locator(REPLACE_MODAL).getByRole('button', { name: OK_BUTTON }).click()
    await expect(page.locator(REPLACE_MODAL).getByText('请选择新物联网卡')).toBeVisible()
    await shot(page, '03b-iot-card-replace-modal')

    await page.locator(REPLACE_MODAL).getByRole('button', { name: /取\s*消/ }).click()
    await expect(page.locator(REPLACE_MODAL)).toBeHidden()
  })
})
