import { collectConsole, type ConsoleCollector } from '../helpers/console'
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
    // 已知产品问题（src/admin/pages/gb/RecordPlanList.tsx:463/476/486）：
    // Form.List 的 `{...field}` 把 key 一并展开到 Form.Item，React 19 会以 console.error 告警。
    .filter((text) => !text.includes('React keys must be passed directly to JSX without using spread'))
}

/**
 * 11b 国标媒体：实时播放 / 录像回放 / 录像下载 / 云端录像 / 录像计划
 *
 * 环境没有真实国标设备与 SIP 信令服务，因此：
 *  - 实时播放断言「明确的空态」，不做「假装能播放」的放宽；
 *  - 回放断言未选通道时的明确提示，而不是静默无响应；
 *  - 其余页面断言页面加载 + 主按钮/弹窗可用（超管权限码应齐全）。
 */

const TITLE = '.page-container-title'

function isPath(url: string, path: string): boolean {
  try {
    return new URL(url).pathname === path
  } catch {
    return false
  }
}

test.describe('11b 国标媒体', () => {
  test('实时播放：空态降级明确，播放相关按钮不可用', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/channels') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/media/live')
    await expect(page.locator(TITLE).first()).toHaveText('实时播放')
    expect((await initial).status()).toBe(200)

    // 左树 / 播放窗口 / 云台控制三个主体
    await expect(page.getByText('设备 / 通道', { exact: true })).toBeVisible()
    await expect(page.getByText('播放窗口', { exact: true })).toBeVisible()
    await expect(page.getByText('云台控制', { exact: true })).toBeVisible()

    // 无设备时必须有明确空态，不能白屏
    await expect(page.getByText('暂无国标设备')).toBeVisible()
    await expect(page.locator('.e2e-gb-channel-tree')).toHaveCount(0)
    await expect(page.getByText('请从左侧选择通道')).toBeVisible()

    // 未选通道时播放 / 停止 / 抓图 / 语音按钮不可点击
    for (const name of [/播\s*放/, /停\s*止/, /抓\s*图/, /语音广播/, /语音对讲/]) {
      await expect(page.getByRole('button', { name })).toBeDisabled()
    }

    // 刷新数据仍不报错
    const refreshed = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/channels') && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: /刷\s*新/ }).click()
    expect((await refreshed).status()).toBe(200)
    await expect(page.getByText('暂无国标设备')).toBeVisible()

    await shot(page, '11b-gb-live-empty')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('录像回放：查询按钮可用，未选通道时给出明确提示', async ({ page }) => {
    const console = collectConsole(page)
    await gotoAdmin(page, '/gb/media/playback')
    await expect(page.locator(TITLE).first()).toHaveText('录像回放')

    // antd Select 的占位符渲染为 `.ant-select-selection-placeholder`（不是 input placeholder）
    await expect(page.locator('.ant-select-selection-placeholder').filter({ hasText: '请选择通道' })).toBeVisible()
    await expect(page.getByText('请选择通道与时间范围后查询')).toBeVisible()

    const query = page.getByRole('button', { name: /查询录像/ })
    await expect(query).toBeEnabled()
    await query.click()
    await expect(page.getByText('请先选择通道')).toBeVisible()

    // 回放窗口仍在（不白屏）
    await expect(page.getByText('回放窗口', { exact: true })).toBeVisible()

    await shot(page, '11b-gb-playback')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('录像下载：页面说明可见，开始下载弹窗可打开', async ({ page }) => {
    const console = collectConsole(page)
    await gotoAdmin(page, '/gb/media/download')
    await expect(page.locator(TITLE).first()).toHaveText('录像下载')

    await expect(
      page.getByText('下载任务仅保存在服务进程内存', { exact: false })
    ).toBeVisible()

    const start = page.getByRole('button', { name: /开始下载/ }).first()
    await expect(start).toBeEnabled()
    await start.click()

    const modal = page.locator('.e2e-gb-download-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByText('开始录像下载', { exact: true })).toBeVisible()
    await expect(modal.getByText('通道', { exact: true })).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11b-gb-download')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('云端录像：页面加载、主按钮存在、刷新正常', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/cloud-records') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/media/cloud-record')
    await expect(page.locator(TITLE).first()).toHaveText('云端录像')
    expect((await initial).status()).toBe(200)

    await expect(page.getByRole('button', { name: /刷\s*新/ })).toBeEnabled()
    // 未勾选记录时批量操作不可用
    await expect(page.getByRole('button', { name: '批量收藏' })).toBeDisabled()
    await expect(page.getByRole('button', { name: '批量取消收藏' })).toBeDisabled()
    await expect(page.getByRole('button', { name: '批量删除' })).toBeDisabled()
    await expect(page.getByText('暂无云端录像')).toBeVisible()

    const refreshed = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/cloud-records') && res.request().method() === 'GET'
    )
    await page.getByRole('button', { name: /刷\s*新/ }).click()
    expect((await refreshed).status()).toBe(200)

    await shot(page, '11b-gb-cloud-record')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('录像计划：页面说明可见，新增计划弹窗可打开', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/record-plans') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/media/record-plan')
    await expect(page.locator(TITLE).first()).toHaveText('录像计划')
    expect((await initial).status()).toBe(200)

    await expect(page.getByText('本期只做配置，不做定时调度')).toBeVisible()
    await expect(page.locator('.ant-empty-description').filter({ hasText: '暂无数据' })).toBeVisible()

    await page.getByRole('button', { name: /新增计划/ }).click()
    const modal = page.locator('.e2e-gb-record-plan-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByText('新增录像计划', { exact: true })).toBeVisible()
    await expect(modal.getByPlaceholder('如：园区全天录像')).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11b-gb-record-plan')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })
})
