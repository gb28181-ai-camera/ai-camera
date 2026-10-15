import { OK_BUTTON, expect, gotoAdmin, shot, test, uniqName } from '../fixtures/auth'
import {
  apiCall,
  confirmRowDelete,
  expectApiOk,
  expectColumns,
  expectEmptyTable,
  hasButton,
  searchTable,
  tableRow,
} from '../helpers/crud'

const MODAL = '.e2e-voice-config-modal'
const PREVIEW_MODAL = '.e2e-voice-preview-modal'
const LIST_API = '/web/devices/voice-configs'
const HINT = '请输入语音名称'

async function voiceIdOf(page: import('@playwright/test').Page, name: string): Promise<number> {
  const data = await expectApiOk(await apiCall(page, 'GET', `${LIST_API}?page=1&page_size=200`))
  const hit = ((data.items ?? []) as { id: number; name: string }[]).find((i) => i.name === name)
  if (!hit) throw new Error(`未查询到语音配置：${name}`)
  return hit.id
}

async function createVoice(
  page: import('@playwright/test').Page,
  name: string,
  voiceType: string,
  audioUrl?: string
): Promise<'ui' | 'api'> {
  if (!(await hasButton(page, '新增语音配置'))) {
    await expectApiOk(
      await apiCall(page, 'POST', LIST_API, {
        name,
        voice_type: voiceType,
        audio_url: audioUrl ?? '',
      })
    )
    return 'api'
  }

  await page.getByRole('button', { name: '新增语音配置' }).click()
  await expect(page.locator(MODAL)).toBeVisible()
  await expect(page.locator(MODAL).locator('.ant-modal-title')).toHaveText('新增语音配置')
  await page.locator(MODAL).getByPlaceholder('如：闯入告警播报').fill(name)
  await page
    .locator(MODAL)
    .getByPlaceholder(/关联风险类型编码/)
    .fill(voiceType)

  const resp = page.waitForResponse(
    (res) => res.url().includes(LIST_API) && res.request().method() === 'POST'
  )
  await page.locator(MODAL).getByRole('button', { name: OK_BUTTON }).click()
  // 新增接口走 utils.Created → HTTP 201（响应体 code 仍为 200）
  expect([200, 201]).toContain((await resp).status())
  await expect(page.locator(MODAL)).toBeHidden()
  return 'ui'
}

async function deleteVoice(page: import('@playwright/test').Page, name: string): Promise<void> {
  await searchTable(page, HINT, name, LIST_API)
  const row = tableRow(page, name)
  if ((await row.count()) === 1 && (await hasButton(page, /删\s*除/))) {
    await confirmRowDelete(page, row)
    return
  }
  await expectApiOk(await apiCall(page, 'DELETE', `${LIST_API}/${await voiceIdOf(page, name)}`))
}

test.describe('03d 语音配置管理', () => {
  test.beforeEach(async ({ page }) => {
    const resp = page.waitForResponse((res) => res.url().includes(LIST_API))
    await gotoAdmin(page, '/settings/voice')
    await resp
    await expect(page.locator('.page-container-title').first()).toHaveText('语音配置管理')
  })

  test('列表渲染：列定义完整', async ({ page }) => {
    await expectColumns(page, ['语音名称', '语音类型', '所属区域', '应用设备', '播报音频', '操作'])
    await expect(page.getByPlaceholder(HINT)).toBeVisible()
    await shot(page, '03d-voice-list')
  })

  test('新增语音配置后列表出现该记录', async ({ page }) => {
    const name = uniqName('E2E语音')
    const voiceType = `e2e_voice_${Date.now()}`
    const path = await createVoice(page, name, voiceType)

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)
    await expect(row).toContainText(voiceType)
    if (path === 'api') expect(path).toBe('api')
    await shot(page, '03d-voice-created')

    await deleteVoice(page, name)
  })

  test('编辑语音配置后名称已变更', async ({ page }) => {
    const name = uniqName('E2E改语音')
    const renamed = `${name}_新`
    await createVoice(page, name, 'e2e_edit_voice')

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)

    const id = await voiceIdOf(page, name)
    if (await hasButton(page, /编\s*辑/)) {
      await row.getByRole('button', { name: /编\s*辑/ }).click()
      await expect(page.locator(MODAL)).toBeVisible()
      await expect(page.locator(MODAL).getByPlaceholder('如：闯入告警播报')).toHaveValue(name)
      await page.locator(MODAL).getByPlaceholder('如：闯入告警播报').fill(renamed)
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
          voice_type: 'e2e_edit_voice',
        })
      )
    }

    await searchTable(page, HINT, renamed, LIST_API)
    await expect(tableRow(page, renamed)).toHaveCount(1)

    await deleteVoice(page, renamed)
  })

  test('音频试听弹窗：配置了 audio_url 的记录可打开试听', async ({ page }) => {
    const name = uniqName('E2E预览')
    // 直接建带音频的记录（UI 上传需真实音频文件，此处用接口准备数据）
    await expectApiOk(
      await apiCall(page, 'POST', LIST_API, {
        name,
        voice_type: 'e2e_preview',
        audio_url: '/uploads/e2e-voice.mp3',
        audio_name: 'e2e-voice.mp3',
      })
    )

    await searchTable(page, HINT, name, LIST_API)
    const row = tableRow(page, name)
    await expect(row).toHaveCount(1)

    // 「试听」是行内 <a> 链接，仅在 audio_url 非空时渲染
    const preview = row.locator('a:has-text("试听")')
    await expect(preview).toHaveCount(1)
    await preview.click()

    await expect(page.locator(PREVIEW_MODAL)).toBeVisible()
    await expect(page.locator(PREVIEW_MODAL).locator('.ant-modal-title')).toHaveText('音频试听')
    await expect(page.locator(PREVIEW_MODAL).locator('audio')).toHaveAttribute(
      'src',
      /e2e-voice\.mp3/
    )
    await shot(page, '03d-voice-preview')
    await page.locator(PREVIEW_MODAL).locator('.ant-modal-close').click()
    await expect(page.locator(PREVIEW_MODAL)).toBeHidden()

    await deleteVoice(page, name)
  })

  test('删除语音配置后该记录消失', async ({ page }) => {
    const name = uniqName('E2E删语音')
    await createVoice(page, name, 'e2e_del_voice')
    await searchTable(page, HINT, name, LIST_API)
    await expect(tableRow(page, name)).toHaveCount(1)

    await deleteVoice(page, name)

    await searchTable(page, HINT, name, LIST_API)
    await expectEmptyTable(page)
  })
})
