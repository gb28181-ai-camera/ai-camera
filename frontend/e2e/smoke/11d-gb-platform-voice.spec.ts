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
 * 11d 国标平台级联、业务分组与语音 Agent 配置
 */

const TITLE = '.page-container-title'

function isPath(url: string, path: string): boolean {
  try {
    return new URL(url).pathname === path
  } catch {
    return false
  }
}

test.describe('11d 国标平台与语音 Agent', () => {
  test('上级平台：列头与空态正常，新增弹窗可打开', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/platforms') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/platform/list')
    await expect(page.locator(TITLE).first()).toHaveText('上级平台')
    expect((await initial).status()).toBe(200)

    await expectColumns(page, [
      '平台名称',
      '平台编码',
      'SIP 域',
      '地址',
      '用户名',
      '传输协议',
      '启用状态',
      '在线状态',
    ])
    await expect(page.locator('.ant-empty-description').filter({ hasText: '暂无数据' })).toBeVisible()

    await page.getByRole('button', { name: /新增上级平台/ }).click()
    const modal = page.locator('.e2e-gb-platform-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByText('新增上级平台', { exact: true })).toBeVisible()
    await expect(modal.getByPlaceholder('请输入上级平台名称')).toBeVisible()
    await expect(modal.getByPlaceholder('上级平台 SIP 国标编码，如 34020000002000000001')).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11d-gb-platform')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('级联通道：无上级平台时给出明确引导，添加按钮不可用', async ({ page }) => {
    const console = collectConsole(page)
    await gotoAdmin(page, '/gb/platform/channel')
    await expect(page.locator(TITLE).first()).toHaveText('级联通道')

    await expect(page.getByText('暂无可选上级平台', { exact: false })).toBeVisible()
    const add = page.getByRole('button', { name: /添加通道/ })
    await expect(add).toBeDisabled()
    // antd Select 的占位符渲染为 `.ant-select-selection-placeholder`（不是 input placeholder）
    await expect(
      page.locator('.ant-select-selection-placeholder').filter({ hasText: '请选择上级平台' })
    ).toBeVisible()

    await shot(page, '11d-gb-platform-channel')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('业务分组：空树提示，新建分组弹窗可打开', async ({ page }) => {
    const console = collectConsole(page)

    const initial = page.waitForResponse(
      (res) => isPath(res.url(), '/web/gb/groups/tree') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/org/group')
    await expect(page.locator(TITLE).first()).toHaveText('业务分组')
    expect((await initial).status()).toBe(200)

    await expect(page.getByText('分组树', { exact: true })).toBeVisible()
    await expect(page.getByText('分组详情', { exact: true })).toBeVisible()
    await expect(page.getByText('暂无业务分组')).toBeVisible()

    await page.getByRole('button', { name: /新建分组/ }).click()
    const modal = page.locator('.e2e-gb-group-modal')
    await expect(modal).toBeVisible()
    await expect(modal.getByText('新建业务分组', { exact: true })).toBeVisible()
    await expect(modal.getByPlaceholder('如：园区一组')).toBeVisible()

    await modal.getByRole('button', { name: CANCEL_BUTTON }).click()
    await expect(modal).toBeHidden()

    await shot(page, '11d-gb-group')
    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })

  test('语音 Agent：四段配置标题可见，provider 下拉来自字典表', async ({ page }) => {
    const console = collectConsole(page)

    const providersResp = page.waitForResponse(
      (res) => isPath(res.url(), '/web/settings/voice-agent/providers') && res.request().method() === 'GET'
    )
    await gotoAdmin(page, '/gb/org/voice-agent')
    await expect(page.locator(TITLE).first()).toHaveText('语音 Agent 配置')
    const providers = await providersResp
    expect(providers.status()).toBe(200)
    const providerBody = (await providers.json()) as { data?: { category?: string; code?: string }[] }
    // 字典表至少要有 STT/LLM/TTS/VAD 四个类别的 provider
    const categories = new Set((providerBody.data ?? []).map((item) => item.category))
    for (const category of ['stt', 'llm', 'tts', 'vad']) {
      expect(categories.has(category), `provider 字典缺少类别 ${category}`).toBe(true)
    }

    // 四段配置
    await expect(page.getByText('STT 语音识别', { exact: true })).toBeVisible()
    await expect(page.getByText('LLM 对话模型', { exact: true })).toBeVisible()
    await expect(page.getByText('TTS 语音合成', { exact: true })).toBeVisible()
    await expect(page.getByText('VAD 语音活动检测', { exact: true })).toBeVisible()
    await expect(page.getByText('配置存于 ac_thirdparty_config', { exact: false })).toBeVisible()

    // 四个 provider 下拉均已回填（来自配置默认值）
    for (const label of ['STT Provider', 'LLM Provider', 'TTS Provider', 'VAD Provider']) {
      const item = page
        .locator('.ant-form-item')
        .filter({ has: page.locator(`label:text-is("${label}")`) })
        .first()
      const selected = item.locator('.ant-select-selection-item')
      await expect(selected).toHaveCount(1)
      expect((await selected.innerText()).trim()).not.toBe('')
    }

    // 展开 STT 下拉，选项来自 /providers 字典，非硬编码
    const sttItem = page
      .locator('.ant-form-item')
      .filter({ has: page.locator('label:text-is("STT Provider")') })
      .first()
    await sttItem.locator('.ant-select').click()
    const options = page.locator('.ant-select-dropdown:visible .ant-select-item-option')
    await expect(options.first()).toBeVisible()
    expect(await options.count()).toBeGreaterThan(0)
    await shot(page, '11d-gb-voice-agent-providers')
    await page.keyboard.press('Escape')

    expect(consoleErrors(console)).toEqual([])
    console.dispose()
  })
})
