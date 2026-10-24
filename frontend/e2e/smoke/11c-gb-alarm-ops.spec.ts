import { collectConsole, type ConsoleCollector } from '../helpers/console'
import { expectColumns } from '../helpers/crud'
import { CANCEL_BUTTON, expect, gotoAdmin, shot, test } from '../fixtures/auth'

/**
 * 已知产品问题：页面在 Modal 关闭态创建 `useForm`（Form 位于 destroyOnHidden Modal 内），
 * antd 以 console.error 输出 "Instance created by `useForm` is not connected to any Form element"。
 * 仅过滤这一条，其余 console error 仍会导致用例失败。
 */
function consoleErrors(collector: ConsoleCollector): string[] {
  return collector
    .texts()
    .filter((text) => !text.includes('Instance created by `useForm` is not connected to any Form element'))
}

/**
 * 11c 国标报警与运维：报警 / 移动位置 / 媒体节点 / 拉流代理 / 推流 / 信令日志
 */

const TITLE = '.page-container-title'

function isPath(url: string, path: string): boolean {
  try {
    return new URL(url).pathname === path
  } catch {
    return false
  }
}

test.describe('11c 国标报警与运维', () => {
  test('国标报警：列头与空态正常', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/alarms') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/alarm/list')
    await expect(page.locator(TITLE).first()).toHaveText('国标报警')
    expect((await initial).status()).toBe(200)

    await expectColumns(page, [
      '报警时间',
      '设备/通道名称',
      '报警类型',
      '等级',
      '处理状态',
      '处理人',
    ])
    await expect(page.locator('.ant-empty-description').filter({ hasText: '暂无数据' })).toBeVisible()
    // 没有选中行时「清空所选」不可用
    await expect(page.getByRole('button', { name: /清空所选/ })).toBeDisabled()

    await shot(page, '11c-gb-alarm')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('移动位置：最新位置与历史轨迹加载，订阅弹窗可打开', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/positions/realtime') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/alarm/position')
    await expect(page.locator(TITLE).first()).toHaveText('移动位置')
    expect((await initial).status()).toBe(200)

    await expect(page.getByText('最新位置', { exact: true })).toBeVisible()
    await expect(page.getByText('历史轨迹', { exact: true })).toBeVisible()
    // antd Select 的占位符渲染为 `.ant-select-selection-placeholder`（不是 input placeholder）
    await expect(page.locator('.ant-select-selection-placeholder').filter({ hasText: '请选择通道' })).toBeVisible()
    await expect(page.getByText('轨迹图最多展示最近 500 个坐标点', { exact: false })).toBeVisible()

    await page.getByRole('button', { name: /订阅位置/ }).click()
    const modal = page.locator('.e2e-gb-position-subscribe-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByText('订阅移动位置', { exact: true })).toBeVisible()
    await expect(modal.getByText('订阅间隔（秒）')).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11c-gb-position')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('媒体节点：列表加载（含种子节点），新增弹窗可打开', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/media-servers') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/ops/media-server')
    await expect(page.locator(TITLE).first()).toHaveText('媒体节点')
    expect((await initial).status()).toBe(200)

    await expect(page.getByText('单端口收流模式')).toBeVisible()
    await expectColumns(page, ['节点名称', '类型', 'API 地址', '在线状态'])
    // 种子数据自带一个 ZLM 节点
    await expect(page.getByText('本地 ZLMediaKit', { exact: true })).toBeVisible()

    await page.getByRole('button', { name: /新增节点/ }).click()
    const modal = page.locator('.e2e-gb-media-server-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByText('新增媒体节点', { exact: true })).toBeVisible()
    await expect(modal.getByPlaceholder('如：本地 ZLMediaKit')).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11c-gb-media-server')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('拉流代理：列表加载与新增弹窗', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/stream-proxies') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/ops/stream-proxy')
    await expect(page.locator(TITLE).first()).toHaveText('拉流代理')
    expect((await initial).status()).toBe(200)
    await expect(page.locator('.ant-empty-description').filter({ hasText: '暂无数据' })).toBeVisible()

    await page.getByRole('button', { name: /新增拉流代理/ }).click()
    const modal = page.locator('.e2e-gb-stream-proxy-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByPlaceholder('如：大门摄像头拉流')).toBeVisible()
    await expect(modal.getByText('源地址', { exact: false }).first()).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11c-gb-stream-proxy')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('推流：列表加载与新增弹窗', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/stream-pushes') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/ops/stream-push')
    await expect(page.locator(TITLE).first()).toHaveText('推流')
    expect((await initial).status()).toBe(200)

    await expect(page.getByText('启用 / 停用仅维护配置状态', { exact: false })).toBeVisible()
    await expect(page.locator('.ant-empty-description').filter({ hasText: '暂无数据' })).toBeVisible()

    await page.getByRole('button', { name: /新增推流/ }).click()
    const modal = page.locator('.e2e-gb-stream-push-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByPlaceholder('如：大门编码器')).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11c-gb-stream-push')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('信令日志：统计卡片可见，列头与空态正常', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/sip-logs') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/ops/sip-log')
    await expect(page.locator(TITLE).first()).toHaveText('信令日志')
    expect((await initial).status()).toBe(200)

    // 四张统计卡片
    for (const title of ['总日志数', '成功', '失败', '拒绝']) {
      await expect(page.locator('.ant-statistic-title').filter({ hasText: title }).first()).toBeVisible()
    }

    await expectColumns(page, [
      '时间',
      '方向',
      '方法',
      '命令类型',
      'SIP 状态码',
      '设备编码',
      '远端地址',
      '结果',
    ])
    await expect(page.locator('.ant-empty-description').filter({ hasText: '暂无数据' })).toBeVisible()

    await shot(page, '11c-gb-sip-log')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })
})
