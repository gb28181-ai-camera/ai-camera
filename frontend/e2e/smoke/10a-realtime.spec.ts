import { expect, gotoAdmin, shot, test } from '../fixtures/auth'
import { pickSelectOption } from '../helpers/crud'

const PAGE = '/monitor/realtime'
const SEED_REGION = '一号厂区'

test.describe('10a 实时监控', () => {
  test.beforeEach(async ({ page }) => {
    await gotoAdmin(page, PAGE)
    await expect(page.locator('.page-container-title').first()).toHaveText('实时监控')
  })

  test('页面渲染：区域选择 + 摄像头选择 + 布局切换器', async ({ page }) => {
    await expect(page.getByText('请先选择区域').first()).toBeVisible()
    await expect(page.getByRole('button', { name: '选择摄像头' })).toBeVisible()
    await expect(page.getByRole('button', { name: '选择摄像头' })).toBeDisabled()

    const layouts = page.locator('.ant-radio-button-wrapper')
    await expect(layouts).toHaveCount(3)
    await expect(layouts.nth(0)).toHaveText('单屏')
    await expect(layouts.nth(1)).toHaveText('4 宫格')
    await expect(layouts.nth(2)).toHaveText('9 宫格')
    // 默认单屏选中
    await shot(page, '10a-realtime-empty')
  })

  test('未选择摄像头时展示引导空态', async ({ page }) => {
    await expect(
      page.getByText('请先选择区域与摄像头，随后才会显示视频监控画面')
    ).toBeVisible()
    expect(await page.locator('video, canvas').count()).toBe(0)
  })

  test('单屏 / 4 宫格 / 9 宫格切换生效', async ({ page }) => {
    const layouts = page.locator('.ant-radio-button-wrapper')

    await layouts.nth(1).click()
    await expect(layouts.nth(1)).toHaveClass(/ant-radio-button-wrapper-checked/)

    await layouts.nth(2).click()
    await expect(layouts.nth(2)).toHaveClass(/ant-radio-button-wrapper-checked/)

    await layouts.nth(0).click()

    // 引导提示仍在（未选摄像头）
    await expect(page.getByText('请先选择区域与摄像头，随后才会显示视频监控画面')).toBeVisible()
  })

  test('选择区域后摄像头选择按钮可用，可打开摄像头选择弹窗', async ({ page }) => {
    // 工具栏中的区域下拉（非 Form.Item，直接定位首个 Select）
    await pickSelectOption(page, page.locator('.page-container .ant-select').first(), SEED_REGION)

    const pickBtn = page.getByRole('button', { name: '选择摄像头' })
    await expect(pickBtn).toBeEnabled()
    await pickBtn.click()

    // 摄像头选择弹窗（SelectorModal：.e2e-device-select-modal）
    await expect(page.locator('.e2e-device-select-modal')).toBeVisible()
    await expect(page.getByText('选择摄像头（可多选）')).toBeVisible()
    await shot(page, '10a-realtime-device-select')
  })
})
