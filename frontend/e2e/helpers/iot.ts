import { expect, type Page } from '@playwright/test'
import { apiCall, expectApiOk } from './crud'

/**
 * v0.3.0 IOT 冒烟测试夹具。
 *
 * 这些函数只负责「造数据 / 清数据」，业务断言仍写在 spec 中。
 * 所有调用复用页面登录态（apiCall 会自动取 localStorage.token），
 * 因此用例可以先造数据再 goto，也可以先进入页面再调用。
 */

export interface IotProductOptions {
  name: string
  code: string
  allowShare?: boolean
  netConfigMode?: 'bluetooth' | 'ap' | 'qrcode'
  /** 产品必须选择至少一个服务组；创建时会自动生成产品×服务凭据。 */
  serviceGroupIds: number[]
}

export interface IotServiceInput {
  code: string
  name: string
  protocol: string
  host?: string
  port?: number
  path?: string
  extra?: string
  active?: boolean
  sequence?: number
}

export interface IotFixture {
  productId?: number
  groupId?: number
  deviceId?: number
  deviceCode?: string
}

/** 创建 IOT 产品并返回 ID。 */
export async function createIotProduct(
  page: Page,
  opts: IotProductOptions
): Promise<number> {
  const data = await expectApiOk(
    await apiCall(page, 'POST', '/web/iot/products', {
      name: opts.name,
      code: opts.code,
      auto_network: false,
      net_config_mode: opts.netConfigMode ?? 'bluetooth',
      allow_share: opts.allowShare ?? false,
      active: true,
      service_group_ids: opts.serviceGroupIds,
    })
  )
  const id = Number(data.id ?? (data as { ID?: number }).ID)
  if (!id) throw new Error(`创建 IOT 产品未返回 ID: ${JSON.stringify(data)}`)
  return id
}

/**
 * 查询系统预置服务组的 ID。
 *
 * 服务组固定为三条（gb28181 / webrtc / aichat），由 initdb 写入；
 * E2E 不再创建服务组，只按编码取 ID。
 */
export async function fixedServiceGroupId(
  page: Page,
  code: 'gb28181' | 'webrtc' | 'aichat'
): Promise<number> {
  const data = await expectApiOk(
    await apiCall(page, 'GET', `/web/iot/service-groups?page=1&page_size=200&keyword=${code}`)
  )
  const items = (data.items ?? []) as Array<Record<string, unknown>>
  const hit = items.find((item) => String(item.code ?? '') === code)
  if (!hit) throw new Error(`未找到系统预置服务组：${code}（请先执行 initdb）`)
  return Number(hit.id)
}

/** 整体替换服务组下的服务。 */
export async function replaceIotServices(
  page: Page,
  groupId: number,
  services: IotServiceInput[]
): Promise<void> {
  await expectApiOk(
    await apiCall(page, 'PUT', `/web/iot/service-groups/${groupId}/services`, {
      services: services.map((svc) => ({
        code: svc.code,
        name: svc.name,
        protocol: svc.protocol,
        host: svc.host ?? '',
        port: svc.port ?? 0,
        path: svc.path ?? '',
        extra: svc.extra ?? '',
        active: svc.active ?? true,
        sequence: svc.sequence ?? 10,
      })),
    })
  )
}

/** 查询产品下的服务凭据（密码脱敏）。 */
export async function listIotProductServices(
  page: Page,
  productId: number
): Promise<Array<Record<string, unknown>>> {
  const data = await expectApiOk(
    await apiCall(page, 'GET', `/web/iot/products/${productId}/services`)
  )
  return (data.services ?? []) as Array<Record<string, unknown>>
}

/** 产品绑定服务组（整体替换）。 */
export async function bindProductGroups(
  page: Page,
  productId: number,
  groupIds: number[]
): Promise<void> {
  await expectApiOk(
    await apiCall(page, 'PUT', `/web/iot/products/${productId}/service-groups`, {
      service_group_ids: groupIds,
    })
  )
}

/** 通过既有设备管理接口创建设备（IOT 复用 ac_device）。 */
export async function createIotDevice(
  page: Page,
  opts: { name: string; code: string; regionId?: number }
): Promise<number> {
  const data = await expectApiOk(
    await apiCall(page, 'POST', '/web/devices', {
      name: opts.name,
      device_code: opts.code,
      region_id: opts.regionId ?? 1,
    })
  )
  const id = Number(data.id ?? (data as { ID?: number }).ID)
  if (!id) throw new Error(`创建设备未返回 ID: ${JSON.stringify(data)}`)
  return id
}

/** 设置/解绑设备所属产品。 */
export async function setDeviceProduct(
  page: Page,
  deviceId: number,
  productId: number | null
): Promise<void> {
  await expectApiOk(
    await apiCall(page, 'PUT', `/web/iot/devices/${deviceId}/product`, {
      product_id: productId,
    })
  )
}

/** 激活设备主账号；返回服务端的一次性凭据。 */
export async function activateIotDevice(
  page: Page,
  deviceId: number
): Promise<Record<string, unknown>> {
  return expectApiOk(await apiCall(page, 'POST', `/web/iot/devices/${deviceId}/activate`))
}

/** 生成设备分享码；返回 token 与二维码内容。 */
export async function createIotShareToken(
  page: Page,
  deviceId: number
): Promise<Record<string, unknown>> {
  return expectApiOk(
    await apiCall(page, 'POST', `/web/iot/devices/${deviceId}/share-token`)
  )
}

/** 读取设备详情。 */
export async function getIotDevice(
  page: Page,
  deviceId: number
): Promise<Record<string, unknown>> {
  return expectApiOk(await apiCall(page, 'GET', `/web/iot/devices/${deviceId}`))
}

/** 查询产品/服务组/设备列表中的单条记录 ID。 */
export async function iotIdByName(
  page: Page,
  listApi: string,
  keyword: string,
  key: 'code' | 'name' | 'device_code'
): Promise<number> {
  const data = await expectApiOk(
    await apiCall(
      page,
      'GET',
      `${listApi}?page=1&page_size=200&keyword=${encodeURIComponent(keyword)}`
    )
  )
  const items = (data.items ?? []) as Array<Record<string, unknown>>
  const hit = items.find((item) => String(item[key] ?? '') === keyword)
  if (!hit) throw new Error(`未查询到记录：${keyword}`)
  return Number(hit.id)
}

/**
 * 关闭 IOT 弹窗。
 *
 * AntD Modal 的底部按钮在 Playwright 的可访问树里有时只暴露为 generic，
 * 用 `getByRole('button', { name: '关闭' })` 会一直等待。这里限定在
 * e2e 弹窗类名内部，再取 footer 的最后一个按钮（均为「关闭」）。
 */
export async function closeIotModal(page: Page, modalClass: string): Promise<void> {
  const modal = page.locator(modalClass)
  await modal.locator('.ant-modal-footer button').last().click()
  await expect(modal).toBeHidden()
}

/**
 * 选择 antd Select 的选项。
 *
 * 与 crud.selectFormOption 的区别：直接点击下拉项，不依赖输入过滤。
 * IOT 表单里的「配网方式」「服务组类型」「协议」都是非搜索型 Select，
 * 键入文本不会过滤，回车仍停留在原值。
 */
export async function selectIotOption(
  page: Page,
  scope: ReturnType<Page['locator']>,
  label: string,
  optionText: string
): Promise<void> {
  const item = scope
    .locator('.ant-form-item')
    .filter({ has: page.locator(`label:text-is("${label}")`) })
    .first()
  await item.locator('.ant-select').first().click()
  const option = page
    .locator('.ant-select-dropdown:visible .ant-select-item-option')
    .filter({ hasText: optionText })
    .first()
  await option.waitFor({ state: 'visible' })
  await option.click()
  await expect(item.locator('.ant-select-selection-item')).toContainText(optionText)
}

/** 尽力而为的清理：忽略 404/400 等失败，避免掩盖用例本身的断言失败。 */
async function bestEffort(page: Page, method: 'POST' | 'DELETE' | 'PUT', url: string, data?: unknown) {
  try {
    await apiCall(page, method, url, data)
  } catch {
    // 清理失败不影响用例结果；E2E 每次运行前会 resetdb + initdb。
  }
}

/**
 * 按依赖顺序清理 IOT 夹具：
 * 解绑设备 → 删设备 → 产品解绑服务组 → 删产品 → 删服务组。
 *
 * 顺序不能颠倒：服务组被产品引用时后端会拒绝删除。
 */
export async function cleanupIotFixture(page: Page, fixture: IotFixture): Promise<void> {
  if (fixture.deviceId) {
    await bestEffort(page, 'POST', `/web/iot/devices/${fixture.deviceId}/unbind`)
    await bestEffort(page, 'DELETE', `/web/devices/${fixture.deviceId}`)
  }
  if (fixture.productId) {
    await bestEffort(page, 'PUT', `/web/iot/products/${fixture.productId}/service-groups`, {
      service_group_ids: [],
    })
    await bestEffort(page, 'DELETE', `/web/iot/products/${fixture.productId}`)
  }
  // 服务组是系统预置数据，不能删除；清理只涉及设备与产品。
}
