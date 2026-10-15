import { expect, gotoAdmin, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  createRecord,
  expectApiOk,
  expectColumns,
  hasButton,
  searchTable,
  selectFormOption,
  tableRow,
} from '../helpers/crud'

const FIRMWARE_MODAL = '.e2e-firmware-modal'
const TYPE_MODAL = '.e2e-firmware-type-modal'
const MODEL_MODAL = '.e2e-model-modal'
const FIRMWARE_API = '/web/upgrades/firmwares'
const TYPE_API = '/web/upgrades/firmware-types'
const MODEL_API = '/web/upgrades/models'

async function listOf(page: import('@playwright/test').Page, api: string, kw?: string) {
  const data = await expectApiOk(
    await apiCall(page, 'GET', `${api}?page=1&page_size=200${kw ? `&keyword=${kw}` : ''}`)
  )
  return ((data.items ?? []) as Record<string, unknown>[]) ?? []
}

async function idByCode(
  page: import('@playwright/test').Page,
  api: string,
  field: string,
  value: string
): Promise<number> {
  const items = await listOf(page, api)
  const hit = items.find((i) => String(i[field]) === value)
  if (!hit) throw new Error(`${api} 未查询到 ${field}=${value}`)
  return Number(hit.id)
}

/** 固件类型：UI 可用则走交互，否则走 API */
async function createFirmwareType(
  page: import('@playwright/test').Page,
  name: string,
  code: string
): Promise<'ui' | 'api'> {
  return createRecord(
    page,
    '新增固件类型',
    async () => {
      await page.getByRole('button', { name: '新增固件类型' }).click()
      await expect(page.locator(TYPE_MODAL)).toBeVisible()
      await page.locator(TYPE_MODAL).getByPlaceholder('如：主控固件').fill(name)
      await page.locator(TYPE_MODAL).getByPlaceholder('如：main').fill(code)
      const resp = page.waitForResponse(
        (res) => res.url().includes(TYPE_API) && res.request().method() === 'POST'
      )
      await page.locator(TYPE_MODAL).getByRole('button', { name: /确\s*定/ }).click()
      // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
      expect([200, 201]).toContain((await resp).status())
      await expect(page.locator(TYPE_MODAL)).toBeHidden()
    },
    () => apiCall(page, 'POST', TYPE_API, { name, code, sequence: 90 })
  )
}

test.describe('07a 固件 / 固件类型 / 模型管理', () => {
  test.describe('固件类型管理', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(TYPE_API))
      await gotoAdmin(page, '/settings/firmware-type')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('固件类型管理')
    })

    test('列表渲染：含种子类型「主控固件」', async ({ page }) => {
      await expectColumns(page, ['固件类型', '类型编码', '描述', '创建时间', '操作'])
      const row = tableRow(page, '主控固件')
      await expect(row).toHaveCount(1)
      await expect(row).toContainText('main')
      await shot(page, '07a-firmware-type-list')
    })

    test('新增固件类型后列表出现该记录，删除后消失', async ({ page }) => {
      const name = uniqName('E2E固件类型')
      const code = `e2e_ft_${Date.now()}`
      await createFirmwareType(page, name, code)

      await searchTable(page, '请输入固件类型名称/编码', code, TYPE_API)
      const row = tableRow(page, code)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(name)
      await shot(page, '07a-firmware-type-created')

      const id = await idByCode(page, TYPE_API, 'code', code)
      await expectApiOk(await apiCall(page, 'DELETE', `${TYPE_API}/${id}`))
      await searchTable(page, '请输入固件类型名称/编码', code, TYPE_API)
      await expect(page.locator('.ant-table-row')).toHaveCount(0)
    })

    test('固件类型 CRUD 接口闭环：更新描述与排序', async ({ page }) => {
      const name = uniqName('E2E改固件类型')
      const code = `e2e_ft_edit_${Date.now()}`
      await expectApiOk(
        await apiCall(page, 'POST', TYPE_API, { name, code, sequence: 90, description: 'A' })
      )
      const id = await idByCode(page, TYPE_API, 'code', code)

      await expectApiOk(
        await apiCall(page, 'PUT', `${TYPE_API}/${id}`, {
          name: `${name}_新`,
          code,
          sequence: 91,
          description: 'B',
        })
      )
      const items = await listOf(page, TYPE_API)
      const hit = items.find((i) => Number(i.id) === id)
      expect(String(hit?.name)).toBe(`${name}_新`)
      expect(String(hit?.description)).toBe('B')

      await expectApiOk(await apiCall(page, 'DELETE', `${TYPE_API}/${id}`))
    })
  })

  test.describe('固件管理', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(FIRMWARE_API))
      await gotoAdmin(page, '/settings/firmware')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('固件管理')
    })

    test('列表渲染：列定义与新增入口', async ({ page }) => {
      await expectColumns(page, [
        '固件名称',
        '版本号',
        '固件类型',
        '文件',
        '文件大小',
        'MD5',
        '发布时间',
        '操作',
      ])
      await expect(page.getByPlaceholder('请输入固件名称/版本号')).toBeVisible()
      await shot(page, '07a-firmware-list')
    })

    test('新增固件后列表出现该记录且固件类型列显示名称', async ({ page }) => {
      const name = uniqName('E2E固件')
      const version = `v${Date.now()}`

      const path = await createRecord(
        page,
        '新增固件',
        async () => {
          await page.getByRole('button', { name: '新增固件' }).click()
          await expect(page.locator(FIRMWARE_MODAL)).toBeVisible()
          await page.locator(FIRMWARE_MODAL).getByPlaceholder('如：主控固件V1.2.0').fill(name)
          await page.locator(FIRMWARE_MODAL).getByPlaceholder('如：v1.2.0').fill(version)
          await page
            .locator(FIRMWARE_MODAL)
            .getByPlaceholder('/uploads/firmware/v1.2.0.bin')
            .fill('/uploads/e2e/fw.bin')
          const resp = page.waitForResponse(
            (res) => res.url().includes(FIRMWARE_API) && res.request().method() === 'POST'
          )
          await page.locator(FIRMWARE_MODAL).getByRole('button', { name: /确\s*定/ }).click()
          // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
          expect([200, 201]).toContain((await resp).status())
          await expect(page.locator(FIRMWARE_MODAL)).toBeHidden()
        },
        () =>
          apiCall(page, 'POST', FIRMWARE_API, {
            name,
            version,
            firmware_type_id: 1,
            file_url: '/uploads/e2e/fw.bin',
            file_size: 2048,
          })
      )

      await searchTable(page, '请输入固件名称/版本号', version, FIRMWARE_API)
      const row = tableRow(page, version)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(name)
      if (path === 'api') await expect(row).toContainText('主控固件')
      await shot(page, '07a-firmware-created')

      const id = await idByCode(page, FIRMWARE_API, 'version', version)
      // v0.1.0 已实现 `GET /web/upgrades/firmwares/{id}`
      const detail = await expectApiOk(await apiCall(page, 'GET', `${FIRMWARE_API}/${id}`))
      expect(String(detail.name)).toBe(name)
      expect(String(detail.version)).toBe(version)
      await expectApiOk(await apiCall(page, 'DELETE', `${FIRMWARE_API}/${id}`))
      await searchTable(page, '请输入固件名称/版本号', version, FIRMWARE_API)
      await expect(page.locator('.ant-table-row')).toHaveCount(0)
    })
  })

  test.describe('模型管理', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(MODEL_API))
      await gotoAdmin(page, '/settings/model')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('模型管理')
    })

    test('列表渲染：列定义与新增入口（含适用固件名称列）', async ({ page }) => {
      await expectColumns(page, [
        '模型名称',
        '版本号',
        '适用固件',
        '文件',
        '文件大小',
        'MD5',
        '发布时间',
        '操作',
      ])
      await expect(page.getByPlaceholder('请输入模型名称/版本号')).toBeVisible()
      await shot(page, '07a-model-list')
    })

    test('新增模型（关联固件）后列表出现且适用固件列显示名称', async ({ page }) => {
      const name = uniqName('E2E模型')
      const version = `v${Date.now()}`
      // 模型必须关联已存在的固件 → 先造一条固件
      const fwVersion = `v${Date.now()}-fw`
      const fwName = uniqName('E2E模型用固件')
      const fw = await expectApiOk(
        await apiCall(page, 'POST', FIRMWARE_API, {
          name: fwName,
          version: fwVersion,
          firmware_type_id: 1,
          file_url: '/uploads/e2e/model-fw.bin',
          file_size: 1024,
        })
      )
      const firmwareId = Number(fw.id)

      // 「适用固件」下拉的选项在页面挂载时拉取一次，新固件需要重新加载页面才会出现
      const reloadResp = page.waitForResponse((res) => res.url().includes(MODEL_API))
      await page.reload()
      await reloadResp
      await expect(page.locator('.page-container-title').first()).toHaveText('模型管理')

      await createRecord(
        page,
        '新增模型',
        async () => {
          await page.getByRole('button', { name: '新增模型' }).click()
          await expect(page.locator(MODEL_MODAL)).toBeVisible()
          await page.locator(MODEL_MODAL).getByPlaceholder('如：安全帽识别').fill(name)
          await page.locator(MODEL_MODAL).getByPlaceholder('如：v1.0.0').fill(version)
          // 适用固件为必填项（v0.1.0 起按钮可渲染，UI 路径需要真实选择才能提交）
          await selectFormOption(page, page.locator(MODEL_MODAL), '适用固件', fwName)
          const resp = page.waitForResponse(
            (res) => res.url().includes(MODEL_API) && res.request().method() === 'POST'
          )
          await page.locator(MODEL_MODAL).getByRole('button', { name: /确\s*定/ }).click()
          // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
          expect([200, 201]).toContain((await resp).status())
          await expect(page.locator(MODEL_MODAL)).toBeHidden()
        },
        () =>
          apiCall(page, 'POST', MODEL_API, {
            name,
            version,
            firmware_id: firmwareId,
            file_url: '/uploads/e2e/model.zip',
            file_size: 4096,
          })
      )

      await searchTable(page, '请输入模型名称/版本号', version, MODEL_API)
      const row = tableRow(page, version)
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(name)

      // v0.1.0 已实现 `GET /web/upgrades/models/{id}`：直接请求详情校验关联固件字段
      const id = await idByCode(page, MODEL_API, 'version', version)
      const detail = await expectApiOk(await apiCall(page, 'GET', `${MODEL_API}/${id}`))
      expect(Number(detail.firmware_id)).toBe(firmwareId)

      await expectApiOk(await apiCall(page, 'DELETE', `${MODEL_API}/${id}`))
      await searchTable(page, '请输入模型名称/版本号', version, MODEL_API)
      await expect(page.locator('.ant-table-row')).toHaveCount(0)

      // 清理：模型用固件（清理失败不影响用例结论）
      try {
        const fwId = await idByCode(page, FIRMWARE_API, 'version', fwVersion)
        await apiCall(page, 'DELETE', `${FIRMWARE_API}/${fwId}`)
      } catch {
        /* ignore cleanup error */
      }
    })

    test('模型必填契约：缺少必填字段时接口 400，新增按钮对超管可见', async ({ page }) => {
      const res = await apiCall(page, 'POST', MODEL_API, {})
      expect(res.status).toBe(400)
      expect(res.body.message).toContain('required')
      // v0.1.0 已补齐 upgrade:model:create → 新增入口对超管渲染
      expect(await hasButton(page, '新增模型')).toBe(true)
    })
  })
})
