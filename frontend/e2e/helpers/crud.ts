import { expect, type Page } from '@playwright/test'
import { OK_BUTTON } from '../fixtures/auth'

/** 表内「确定」按钮在页面内的定位范围 */
const POPCONFIRM = '.ant-popconfirm:visible'

/** 按钮是否已渲染（用于按钮级权限门控的判断） */
export async function hasButton(page: Page, name: string | RegExp): Promise<boolean> {
  return (await page.getByRole('button', { name }).count()) > 0
}

export interface ApiResult {
  status: number
  body: { code?: number; message?: string; data?: unknown }
}

/**
 * 直接调用后端接口（经 Vite 代理，复用页面登录态）。
 * 用于「页面按钮被权限门控时」验证后端与列表渲染的数据链路。
 */
export async function apiCall(
  page: Page,
  method: 'GET' | 'POST' | 'PUT' | 'DELETE',
  url: string,
  data?: unknown
): Promise<ApiResult> {
  const res = await page.request.fetch(url, {
    method,
    data,
    headers: { Authorization: await authHeader(page) },
  })
  let body: ApiResult['body'] = {}
  try {
    body = (await res.json()) as ApiResult['body']
  } catch {
    body = {}
  }
  return { status: res.status(), body }
}

/**
 * 取当前登录态的 Authorization 头。
 * 页面尚未进入应用（如 beforeEach 中先造数据后 goto）时 localStorage 不可访问，
 * 此时直接调用登录接口换取 token。
 */
async function authHeader(page: Page): Promise<string> {
  let token: string | null = null
  try {
    token = await page.evaluate(() => window.localStorage.getItem('token'))
  } catch {
    token = null
  }
  if (token) return `Bearer ${token}`

  const res = await page.request.post('/web/auth/login', {
    data: { user_name: 'admin', password: 'admin123' },
  })
  const body = (await res.json()) as { data?: { token?: string } }
  return `Bearer ${body?.data?.token ?? ''}`
}

/** 断言接口调用成功（HTTP 200/201 且业务 code=200） */
export async function expectApiOk(result: ApiResult): Promise<Record<string, unknown>> {
  expect([200, 201], JSON.stringify(result.body)).toContain(result.status)
  expect(result.body.code, JSON.stringify(result.body)).toBe(200)
  return (result.body.data ?? {}) as Record<string, unknown>
}

/**
 * 新增记录：UI 按钮可用时走真实交互，否则退回 API（页面权限码与菜单树不一致时的兜底）。
 * 返回实际使用的路径，便于报告中区分。
 */
export async function createRecord(
  page: Page,
  createButton: string | RegExp,
  uiFlow: () => Promise<void>,
  apiFlow: () => Promise<ApiResult>
): Promise<'ui' | 'api'> {
  if (await hasButton(page, createButton)) {
    await uiFlow()
    return 'ui'
  }
  await expectApiOk(await apiFlow())
  return 'api'
}

/**
 * 在指定 e2e Modal 内点击「确定」。
 * 仅使用 `e2e-xxx-modal` 类定位，禁止 `.ant-modal` 系列通用选择器。
 */
export async function confirmModal(page: Page, modalClass: string): Promise<void> {
  await page.locator(modalClass).getByRole('button', { name: OK_BUTTON }).click()
}

/** 表内删除：点击行内「删除」→ 在可见 Popconfirm 中确认 */
export async function confirmRowDelete(page: Page, row: ReturnType<Page['locator']>): Promise<void> {
  await row.getByRole('button', { name: /删\s*除/ }).click()
  const pop = page.locator(POPCONFIRM)
  await expect(pop).toBeVisible()
  await pop.getByRole('button', { name: OK_BUTTON }).click()
}

/**
 * 搜索并等待列表刷新（SearchBar 通用组件：输入框 + 「搜索」按钮）。
 *
 * 注意：SearchBar 的查询由 `setKeyword` 触发，关键字与当前值相同时 React 不会重渲染，
 * 也就不会发起请求。此时先点「重置」制造一次状态变更，再按新关键字检索。
 */
export async function searchTable(
  page: Page,
  placeholder: string,
  keyword: string,
  listApiPart: string
): Promise<void> {
  const input = page.getByPlaceholder(placeholder)
  if ((await input.inputValue()) === keyword) {
    const resetResp = page.waitForResponse(
      (res) => res.url().includes(listApiPart) && res.request().method() === 'GET'
    )
    await searchBarButton(page, '重置').click()
    await resetResp
  }

  await input.fill(keyword)
  const resp = page.waitForResponse(
    (res) => res.url().includes(listApiPart) && res.request().method() === 'GET'
  )
  await searchBarButton(page, '搜索').click()
  await resp
}

/**
 * SearchBar 内的按钮（`.search-bar` 作用域）。
 *
 * 必须限定在搜索栏内：行内按钮的「重置密码」等文案同样包含「重置」，
 * 页面级 getByRole('button', { name: '重置' }) 会命中多个元素触发 strict mode 冲突
 * （如用户管理页在 system:user:* 权限码补齐后行内「重置密码」入口已渲染）。
 */
function searchBarButton(page: Page, name: string) {
  return page.locator('.search-bar').getByRole('button', { name })
}

/** 表格中按文本定位数据行 */
export function tableRow(page: Page, text: string) {
  return page.locator('.ant-table-row').filter({ hasText: text })
}

/**
 * 客户端过滤型列表的搜索（如菜单管理、系统参数：onSearch 只更新本地状态，不发起请求）。
 * 不能等待网络响应，直接依赖后续断言的自动等待。
 */
export async function filterTable(page: Page, placeholder: string, keyword: string): Promise<void> {
  await page.getByPlaceholder(placeholder).fill(keyword)
  await searchBarButton(page, '搜索').click()
}

/** 客户端过滤型列表的重置（不发起请求） */
export async function resetTable(page: Page): Promise<void> {
  await searchBarButton(page, '重置').click()
}

/**
 * 在表单范围内打开 antd Select 并选择选项。
 *
 * 说明：包装组件（RegionSelect / OptionSelect 等）不向下透传 Form.Item 注入的 `id`，
 * 因此 `getByLabel` 无法关联到 Select 输入框，这里改为按「表单项 label 文本」定位。
 * 下拉面板挂载在 body 下（Portal），需用 `:visible` 限定当前展开的那个。
 */
export async function selectFormOption(
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

  // 优先键盘选择：antd 下拉面板用虚拟列表时点击目标可能被判定为「移出视口/重新挂载」，
  // 输入关键字过滤后回车选中更稳定（本套用例选取的都是下拉中的可唯一过滤项）。
  await page.keyboard.type(optionText)
  const option = page
    .locator('.ant-select-dropdown:visible .ant-select-item-option')
    .filter({ hasText: optionText })
    .first()
  await option.waitFor({ state: 'visible' })
  await page.keyboard.press('Enter')

  await expect(item.locator('.ant-select-selection-item')).toContainText(optionText)
}

/**
 * 直接对某个 antd Select 元素选择选项（用于不在 Form.Item 内的筛选下拉）。
 * 输入关键字过滤 + 回车选中，避免虚拟列表点击不稳定。
 */
export async function pickSelectOption(
  page: Page,
  select: ReturnType<Page['locator']>,
  optionText: string
): Promise<void> {
  await select.click()
  await page.keyboard.type(optionText)
  await page
    .locator('.ant-select-dropdown:visible .ant-select-item-option')
    .filter({ hasText: optionText })
    .first()
    .waitFor({ state: 'visible' })
  await page.keyboard.press('Enter')
  await expect(select.locator('.ant-select-selection-item')).toContainText(optionText)
}

/**
 * 断言表格列头存在。
 * scope 可传 Page（单表格页）或某个 Locator（如 `.ant-tabs-tabpane-active`，多 Tab 页面下只校验活动面板）。
 */
export async function expectColumns(
  scope: Page | ReturnType<Page['locator']>,
  titles: string[]
): Promise<void> {
  for (const title of titles) {
    // 用列头的可访问名做精确匹配：避免「文件」误命中「文件大小」、「状态」误命中「结果」等前缀包含
    await expect(scope.getByRole('columnheader', { name: title, exact: true })).toHaveCount(1)
  }
}

/** 断言表格为空态 */
export async function expectEmptyTable(page: Page): Promise<void> {
  await expect(page.locator('.ant-table-row')).toHaveCount(0)
  await expect(page.locator('.ant-empty')).toBeVisible()
}

/** 读取 data.total（分页接口） */
export function totalOf(result: ApiResult): number {
  return Number((result.body.data as { total?: number } | undefined)?.total ?? 0)
}

/**
 * 探测「列表页因生成控制器始终拼接空查询参数而恒为空」的已知问题。
 *
 * 例：UserController.pageUsers 生成 `?dept_id=&active=`，后端把空串当有效过滤条件，
 * 结果恒为 0 条；同样的条件下参数省略时能查到数据 → 判定为缺陷。
 */
export async function listBrokenByEmptyParams(
  page: Page,
  urlWithEmptyParams: string,
  urlCleanParams: string
): Promise<boolean> {
  const broken = await apiCall(page, 'GET', urlWithEmptyParams)
  const clean = await apiCall(page, 'GET', urlCleanParams)
  return totalOf(broken) === 0 && totalOf(clean) > 0
}

/** 设备报文模拟：创建一把临时 API Key（返回 full_key，仅创建时可见） */
export async function createApiKey(page: Page, name: string): Promise<{ id: number; key: string }> {
  const data = await expectApiOk(
    await apiCall(page, 'POST', '/web/profile/api-keys', { name })
  )
  const key = String(data.full_key ?? '')
  if (!key) throw new Error('创建 API Key 失败：未返回 full_key')
  return { id: Number(data.id), key }
}

/** 删除临时 API Key */
export async function deleteApiKey(page: Page, id: number): Promise<void> {
  await expectApiOk(await apiCall(page, 'DELETE', `/web/profile/api-keys/${id}`))
}

/** 以 ApiKey 上报一条风险（设备侧上报，不经过 Casbin） */
export async function reportRisk(
  page: Page,
  apiKey: string,
  deviceCode: string,
  riskTypeCode: string,
  extra: Record<string, unknown> = {}
): Promise<Record<string, unknown>> {
  const res = await page.request.fetch('/web/risks/report', {
    method: 'POST',
    data: {
      device_code: deviceCode,
      risk_type_code: riskTypeCode,
      image_url: '/uploads/e2e-risk.png',
      detect_score: 0.93,
      ...extra,
    },
    headers: { 'X-Api-Key': apiKey },
  })
  const body = (await res.json()) as ApiResult['body']
  expect(body.code, JSON.stringify(body)).toBe(200)
  return (body.data ?? {}) as Record<string, unknown>
}

/** 以 ApiKey 上报一条设备告警 */
export async function reportAlarm(
  page: Page,
  apiKey: string,
  deviceCode: string,
  alarmTypeCode: string,
  extra: Record<string, unknown> = {}
): Promise<Record<string, unknown>> {
  const res = await page.request.fetch('/web/alarms/report', {
    method: 'POST',
    data: { device_code: deviceCode, alarm_type_code: alarmTypeCode, ...extra },
    headers: { 'X-Api-Key': apiKey },
  })
  const body = (await res.json()) as ApiResult['body']
  expect(body.code, JSON.stringify(body)).toBe(200)
  return (body.data ?? {}) as Record<string, unknown>
}
