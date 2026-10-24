import { collectConsole, type ConsoleCollector } from '../helpers/console'
import { apiCall, expectApiOk, expectColumns, totalOf } from '../helpers/crud'
import { CANCEL_BUTTON, expect, gotoAdmin, shot, test, uniqName } from '../fixtures/auth'

/**
 * 已知产品问题（本页只过滤这一条，其余 console error 仍会导致失败）：
 * 页面在 Modal 关闭态就创建了 `useForm` 实例（`Form` 位于 destroyOnHidden 的 Modal 内），
 * antd 会以 console.error 输出 "Instance created by `useForm` is not connected to any Form element"。
 * 功能不受影响，但会污染控制台；已作为产品缺陷记录在冒烟测试报告中。
 */
function consoleErrors(collector: ConsoleCollector): string[] {
  return collector
    .texts()
    .filter((text) => !text.includes('Instance created by `useForm` is not connected to any Form element'))
}

/**
 * 11a 国标设备接入：设备 / 通道 / 激活 / 订阅
 *
 * 覆盖需求「设备注册、激活」的落点：
 *  - 普通设备建档（POST /web/devices）
 *  - 生成激活凭证（POST /web/gb/devices/:id/activation，写入 gb_device_id / sip_password）
 *  - 国标设备页读取详情（GET /web/gb/devices/:id）
 *
 * ⚠️ 入口设计：国标设备列表只返回 `gb_device_id` 非空的设备，而激活接口
 *    （POST /web/gb/devices/:id/activation）是唯一写入 gb_device_id 的地方。
 *    因此「国标接入」按钮必须挂在**普通设备管理**页的每一台设备上，
 *    不能挂在国标设备列表里（否则新设备永远不出现在列表、也就永远无法激活）。
 *    本用例走的正是这条真实路径。
 */

const TITLE = '.page-container-title'
const GB_DEVICES = '/web/gb/devices'
const GB_CHANNELS = '/web/gb/channels'

/** 精确匹配 pathname，避免 `/web/gb/channels/options` 命中 `/web/gb/channels` */
function isPath(url: string, path: string): boolean {
  try {
    return new URL(url).pathname === path
  } catch {
    return false
  }
}

test.describe('11a 国标设备与接入', () => {
  test('国标设备：页面渲染、搜索触发列表接口、空态正常', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), GB_DEVICES) && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/device/list')
    await expect(page.locator(TITLE).first()).toHaveText('国标设备')
    expect((await initial).status()).toBe(200)

    await expectColumns(page, [
      '设备名称',
      '设备编号',
      '国标编码',
      '注册状态',
      '在线状态',
      '通道数',
      '所属区域',
    ])
    await expect(page.locator('.ant-table-placeholder')).toBeVisible()
    await expect(page.getByText('暂无国标设备')).toBeVisible()

    // 关键字检索：列表接口必须被调用，且关键字透传
    await page.getByPlaceholder('设备名称/编号/国标编码').fill('E2E_NO_SUCH_DEVICE')
    const searched = page.waitForResponse(
      (res) => isPath(res.url(), GB_DEVICES) && res.request().method() === 'GET'
    )
    await page.locator('.search-bar').getByRole('button', { name: /搜\s*索/ }).click()
    const searchRes = await searched
    expect(searchRes.status()).toBe(200)
    expect(new URL(searchRes.url()).searchParams.get('keyword')).toBe('E2E_NO_SUCH_DEVICE')
    await expect(page.locator('.ant-table-placeholder')).toBeVisible()
    await expect(page.getByText('暂无国标设备')).toBeVisible()

    await shot(page, '11a-gb-device-list')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('国标通道：列表列头、筛选与空态正常', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), GB_CHANNELS) && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/device/channel')
    await expect(page.locator(TITLE).first()).toHaveText('国标通道')
    expect((await initial).status()).toBe(200)

    await expectColumns(page, [
      '通道名称',
      '国标通道编码',
      '所属设备',
      '在线状态',
      '所属区域',
      '业务分组',
      '录像计划',
    ])
    await expect(page.getByText('暂无国标通道', { exact: false })).toBeVisible()

    // 在线状态下拉筛选 → 搜索 → 列表接口带 status=ON
    const statusItem = page
      .locator('.search-bar .ant-form-item')
      .filter({ has: page.locator('label:text-is("在线状态")') })
      .first()
    await statusItem.locator('.ant-select').click()
    await page
      .locator('.ant-select-dropdown:visible .ant-select-item-option')
      .filter({ hasText: '在线' })
      .first()
      .click()

    const filtered = page.waitForResponse(
      (res) => isPath(res.url(), GB_CHANNELS) && res.request().method() === 'GET'
    )
    await page.locator('.search-bar').getByRole('button', { name: /搜\s*索/ }).click()
    const filteredRes = await filtered
    expect(filteredRes.status()).toBe(200)
    expect(new URL(filteredRes.url()).searchParams.get('status')).toBe('ON')

    await shot(page, '11a-gb-channel-list')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('设备激活：说明文案可见，pending 列表可加载', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), GB_DEVICES) && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/device/activation')
    await expect(page.locator(TITLE).first()).toHaveText('设备激活')
    const res = await initial
    expect(res.status()).toBe(200)
    expect(new URL(res.url()).searchParams.get('register_status')).toBe('pending')

    await expect(page.getByText('设备激活流程')).toBeVisible()
    await expect(page.getByText('平台建档：', { exact: false })).toBeVisible()
    await expect(page.getByText('设备注册：', { exact: false })).toBeVisible()
    await expect(page.getByText('暂无待激活设备', { exact: false })).toBeVisible()
    await expect(page.getByRole('button', { name: /生成凭证/ })).toHaveCount(0)

    await shot(page, '11a-gb-activation')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('订阅记录：新增订阅弹窗可打开与关闭', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/subscribes') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/device/subscribe')
    await expect(page.locator(TITLE).first()).toHaveText('订阅记录')
    expect((await initial).status()).toBe(200)
    await expect(page.getByText('暂无订阅记录')).toBeVisible()

    await page.getByRole('button', { name: /新增订阅/ }).click()
    const modal = page.locator('.e2e-gb-subscribe-create-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByText('新增订阅', { exact: true })).toBeVisible()
    await expect(modal.getByPlaceholder('20 位国标编码')).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11a-gb-subscribe')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('写操作：普通设备建档 → 生成激活凭证 → gb_device_id 落库', async ({ page }) => {
    const console = collectConsole(page)
    const suffix = uniqName('e2e_gb_dev')
    let deviceID = 0

    try {
      // 1) 真实写操作：创建设备（普通 AI 相机）
      const created = await expectApiOk(
        await apiCall(page, 'POST', '/web/devices', { name: suffix, device_code: suffix })
      )
      deviceID = Number(created.id)
      expect(deviceID).toBeGreaterThan(0)

      // 2) 断言缺口：未生成国标编码的设备不在国标设备列表中
      const before = await apiCall(
        page,
        'GET',
        `/web/gb/devices?page=1&page_size=50&keyword=${encodeURIComponent(suffix)}`
      )
      await expectApiOk(before)
      expect(totalOf(before)).toBe(0)

      // 3) 从**普通设备管理**页完成国标接入。
      //    这是唯一入口：国标设备列表只返回 gb_device_id 非空的设备，
      //    若入口也放在那里就是鸡生蛋问题（新设备永远不在列表、也就永远无法激活）。
      await gotoAdmin(page, '/settings/device')
      await expect(page.locator(TITLE).first()).toHaveText('设备管理')

      await page.getByPlaceholder('请输入设备名称或设备编号').fill(suffix)
      const deviceSearched = page.waitForResponse(
        (res) => isPath(res.url(), '/web/devices') && res.request().method() === 'GET'
      )
      await page.locator('.search-bar').getByRole('button', { name: /搜\s*索/ }).click()
      await deviceSearched

      const deviceRow = page.locator('.ant-table-row').filter({ hasText: suffix })
      await expect(deviceRow).toHaveCount(1)
      await deviceRow.getByRole('button', { name: /国标接入/ }).click()

      const onboarding = page.locator('.e2e-gb-device-activation-modal')
      await expect(onboarding).toBeVisible()
      await expect(onboarding.getByText('SIP 密码')).toBeVisible()
      await expect(onboarding.getByText('SIP 域名')).toBeVisible()
      await expect(onboarding.getByText('激活二维码', { exact: true })).toBeVisible()

      // 从接口读回真实编码，再断言它确实展示在弹窗里（避免只断言「有弹窗」）
      const activated = await expectApiOk(
        await apiCall(page, 'GET', `/web/gb/devices/${deviceID}`)
      )
      const gbDeviceID = String(activated.gb_device_id ?? '')
      expect(gbDeviceID).not.toBe('')
      expect(activated.register_status).toBe('pending')
      await expect(onboarding.getByText(gbDeviceID, { exact: true })).toBeVisible()
      await onboarding.getByRole('button', { name: /关\s*闭/ }).click()
      await expect(onboarding).toBeHidden()

      // 4) 从国标设备页检索该设备，点击「生成激活凭证」（幂等：编码不变）
      const initial = page.waitForResponse(
        (res) => isPath(res.url(), GB_DEVICES) && res.request().method() === 'GET'
      )
      await gotoAdmin(page, '/gb/device/list')
      await expect(page.locator(TITLE).first()).toHaveText('国标设备')
      await initial

      await page.getByPlaceholder('设备名称/编号/国标编码').fill(suffix)
      const searched = page.waitForResponse(
        (res) => isPath(res.url(), GB_DEVICES) && res.request().method() === 'GET'
      )
      await page.locator('.search-bar').getByRole('button', { name: /搜\s*索/ }).click()
      await searched

      const row = page.locator('.ant-table-row').filter({ hasText: suffix })
      await expect(row).toHaveCount(1)
      await expect(row).toContainText(gbDeviceID)

      await row.getByRole('button', { name: /生成激活凭证/ }).click()
      const modal = page.locator('.e2e-gb-device-activation-modal')
      await expect(modal).toBeVisible()
      // 凭证弹窗展示国标编码 / SIP 密码 / 域名 / 服务器地址
      await expect(modal.getByText(gbDeviceID, { exact: true })).toBeVisible()
      await expect(modal.getByText('SIP 密码')).toBeVisible()
      await expect(modal.getByText('SIP 域名')).toBeVisible()
      await expect(modal.getByText('激活二维码', { exact: true })).toBeVisible()

      // 5) 接口核对：gb_device_id 已落库、注册状态为 pending
      const detail = await expectApiOk(await apiCall(page, 'GET', `/web/gb/devices/${deviceID}`))
      expect(detail.gb_device_id).toBe(gbDeviceID)
      expect(detail.register_status).toBe('pending')
      // 详情接口对 SIP 密码脱敏
      expect(detail.sip_password).toBe('****')

      await shot(page, '11a-gb-device-activation')
      await modal.getByRole('button', { name: /关\s*闭/ }).click()
      await expect(modal).toBeHidden()

      expect(consoleErrors(console)).toEqual([])
    } finally {
      // 清理：删除设备（软删除，通道一并删除）
      if (deviceID > 0) {
        await apiCall(page, 'DELETE', `/web/gb/devices/${deviceID}`)
      }
      console.dispose()
    }
  })
})
