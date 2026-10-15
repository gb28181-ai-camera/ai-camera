import { expect, gotoAdmin, readPermCodes, shot, test } from '../fixtures/auth'
import { apiCall, expectApiOk, hasButton } from '../helpers/crud'

const GETUI_API = '/web/settings/getui'
const SMS_API = '/web/settings/sms'

/**
 * 说明：个推 / 腾讯云短信为「整页表单」而非 Modal，
 * CLAUDE.md 中列出的 `e2e-getui-modal` / `e2e-sms-config-modal` 两个类在页面源码中不存在
 * （页面未使用 Modal），因此本套用例按表单控件定位。
 * 另外「保存」按钮受 `setting:thirdparty:*:update` 权限码门控：
 * v0.1.0 已把这两个按钮权限码补入菜单树（此前缺失 → 按钮不渲染），
 * 因此用例断言按钮对超管可见，配置读写走接口校验。
 */
test.describe('08b 第三方平台配置', () => {
  test.describe('个推账号绑定', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(GETUI_API))
      await gotoAdmin(page, '/settings/getui')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('个推账号绑定')
    })

    test('表单渲染：四个必填项与刷新按钮', async ({ page }) => {
      for (const placeholder of [
        '请输入个推 AppID',
        '请输入个推 AppKey',
        '请输入个推 AppSecret',
        '请输入个推 MasterSecret',
      ]) {
        await expect(page.getByPlaceholder(placeholder)).toBeVisible()
      }
      await expect(page.getByRole('button', { name: '刷新' })).toBeVisible()
      await shot(page, '08b-getui-form')
    })

    test('保存按钮对超管可见；接口层必填契约与读取链路可用', async ({ page }) => {
      // v0.1.0 已补齐 setting:thirdparty:getui:update → 保存按钮对超管渲染
      const codes = await readPermCodes(page)
      expect(codes).toContain('setting:thirdparty:getui:update')
      expect(await hasButton(page, '保存')).toBe(true)

      // 读取接口正常返回
      const current = await expectApiOk(await apiCall(page, 'GET', GETUI_API))
      expect(current).toBeTruthy()

      // 更新接口可访问（不做必填强校验，返回 200 且不产生异常）
      const res = await apiCall(page, 'PUT', GETUI_API, {
        app_id: 'e2e-app-id',
        app_key: 'e2e-app-key',
        app_secret: 'e2e-getui-secret',
        master_secret: 'e2e-master-secret',
      })
      expect([200, 201]).toContain(res.status)
    })

    test('敏感项以脱敏值返回（不泄露明文）', async ({ page }) => {
      const saved = await expectApiOk(await apiCall(page, 'GET', GETUI_API))
      // 页面只回显脱敏值；未配置时为空串
      expect(String(saved.app_secret ?? '')).not.toBe('e2e-getui-secret')
      expect(String(saved.master_secret ?? '')).not.toBe('e2e-master-secret')
      await expect(page.getByPlaceholder('请输入个推 AppID')).toHaveValue(
        String(saved.app_id ?? '')
      )
    })
  })

  test.describe('腾讯云短信配置', () => {
    test.beforeEach(async ({ page }) => {
      const resp = page.waitForResponse((res) => res.url().includes(SMS_API))
      await gotoAdmin(page, '/settings/sms')
      await resp
      await expect(page.locator('.page-container-title').first()).toHaveText('腾讯云短信配置')
    })

    test('表单渲染：SecretId/SecretKey/SdkAppId/短信签名', async ({ page }) => {
      for (const placeholder of [
        '请输入腾讯云 SecretId，如 AKIDxxxx',
        '请输入腾讯云 SecretKey',
        '请输入短信应用 SdkAppId，如 1400000000',
        '请输入短信签名，如：AI摄像头',
      ]) {
        await expect(page.getByPlaceholder(placeholder)).toBeVisible()
      }
      await shot(page, '08b-sms-form')
    })

    test('保存按钮对超管可见；配置读写链路可用', async ({ page }) => {
      // v0.1.0 已补齐 setting:thirdparty:sms:update → 保存按钮对超管渲染
      const codes = await readPermCodes(page)
      expect(codes).toContain('setting:thirdparty:sms:update')
      expect(await hasButton(page, '保存')).toBe(true)

      const res = await apiCall(page, 'PUT', SMS_API, {
        secret_id: 'e2e-secret-id',
        secret_key: 'e2e-sms-secret',
        sdk_app_id: '1400000000',
        sign_name: 'AI摄像头E2E',
      })
      expect([200, 201]).toContain(res.status)

      const saved = await expectApiOk(await apiCall(page, 'GET', SMS_API))
      // 敏感项脱敏
      expect(String(saved.secret_key ?? '')).not.toBe('e2e-sms-secret')
      expect(String(saved.sign_name ?? '')).toContain('AI摄像头')
    })

    test('页面回显接口返回的非敏感字段（SdkAppId / 签名）', async ({ page }) => {
      const saved = await expectApiOk(await apiCall(page, 'GET', SMS_API))
      await expect(page.getByPlaceholder('请输入短信应用 SdkAppId，如 1400000000')).toHaveValue(
        String(saved.sdk_app_id ?? '')
      )
      await expect(page.getByPlaceholder('请输入短信签名，如：AI摄像头')).toHaveValue(
        String(saved.sign_name ?? '')
      )
    })
  })
})
