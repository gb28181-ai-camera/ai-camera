import { http, HttpResponse } from 'msw'

/**
 * 后端 API 基地址。vitest.config.ts 中设置了 `VITE_API_BASE=http://localhost:8080`，
 * 因此请求会打到该地址，处理器需按绝对地址匹配。
 */
export const API_BASE = 'http://localhost:8080'

/**
 * 响应辅助对象，统一 `{ code, message, data }` 结构与分页结构。
 *
 * - `httpResponse.success(data)` —— 成功（code 200）
 * - `httpResponse.page(items, total)` —— 分页成功（data.items / data.total）
 * - `httpResponse.error(msg, code)` —— 业务错误（HTTP 200 + 非 200 的 code，与后端错误码语义一致）
 * - `httpResponse.unauthorized()` —— 未登录（HTTP 401，触发前端跳转登录）
 */
export const httpResponse = {
  success: (data: unknown = null, message = 'ok') =>
    HttpResponse.json({ code: 200, message, data }),

  page: (items: unknown[] = [], total?: number, message = 'ok') =>
    HttpResponse.json({
      code: 200,
      message,
      data: {
        items,
        total: total ?? items.length,
        page: 1,
        page_size: 20,
      },
    }),

  error: (message = '请求失败', code = 500) =>
    HttpResponse.json({ code, message, data: null }),

  unauthorized: (message = '未登录或登录已过期') =>
    HttpResponse.json({ code: 401, message, data: null }, { status: 401 }),
}

/* ── 通用 mock 数据 ──────────────────────────────────────────────────── */

export const mockMenus = [
  {
    id: 1,
    name: '首页',
    type: 'menu',
    path: '/dashboard',
    icon: 'dashboard',
    sequence: 1,
    active: true,
    permission_code: 'dashboard:view',
  },
  {
    id: 2,
    name: '系统管理',
    type: 'directory',
    icon: 'setting',
    sequence: 9,
    active: true,
    children: [
      {
        id: 21,
        name: '用户管理',
        type: 'menu',
        path: '/system/user',
        icon: 'user',
        sequence: 1,
        active: true,
        permission_code: 'system:user:list',
        children: [
          {
            id: 211,
            name: '新增用户',
            type: 'button',
            active: true,
            permission_code: 'system:user:create',
          },
        ],
      },
      {
        id: 22,
        name: '角色管理',
        type: 'menu',
        path: '/system/role',
        icon: 'team',
        sequence: 2,
        active: true,
        permission_code: 'system:role:list',
      },
    ],
  },
]

export const mockUser = {
  user_id: 1,
  user_name: 'admin',
  name: '系统管理员',
  dept_id: 1,
  data_perm: 'all',
  cross_dept_ids: '',
  permissions: ['system:user:list', 'system:role:list'],
}

export const mockDevice = {
  id: 1,
  device_code: 'DEV-0001',
  name: '东区-01',
  status: 'online',
  region_id: 1,
  region: { id: 1, name: '东区厂房' },
}

export const mockRegion = { id: 1, name: '东区厂房', code: 'EAST' }

export const mockDashboard = {
  today_risk_count: 6,
  today_alarm_count: 2,
  today_handled_count: 5,
  today_risk_handled_count: 4,
  today_alarm_handled_count: 1,
  week_risk_count: 38,
  week_alarm_count: 11,
  week_handled_count: 40,
  week_risk_handled_count: 31,
  week_alarm_handled_count: 9,
  month_risk_count: 152,
  month_alarm_count: 44,
  month_handled_count: 168,
  month_risk_handled_count: 131,
  month_alarm_handled_count: 37,
  device_stat: { total: 120, online: 96, offline: 24 },
  risk_type_stats: [
    { name: '未佩戴安全帽', count: 12 },
    { name: '明火', count: 5 },
  ],
  risk_handle_stats: { pending: 8, handled: 42 },
  region_risk_stats: [
    { name: '东区厂房', count: 9 },
    { name: '西区仓库', count: 4 },
  ],
  risk_trend: [
    { date: '2026-09-15', count: 3 },
    { date: '2026-09-16', count: 5 },
  ],
  alarm_trend: [
    { date: '2026-09-15', count: 1 },
    { date: '2026-09-16', count: 2 },
  ],
}

/* ── 默认处理器 ──────────────────────────────────────────────────────── */

export const handlers = [
  // 认证
  http.post(`${API_BASE}/web/auth/login`, async ({ request }) => {
    const body = (await request.json()) as { user_name?: string; password?: string } | null
    if (body?.user_name && body?.password) {
      return httpResponse.success({ token: 'mock-token', ...mockUser })
    }
    return httpResponse.error('用户名或密码错误', 400)
  }),

  // 当前用户菜单树
  http.get(`${API_BASE}/web/profile/menus`, () => httpResponse.success(mockMenus)),

  // 首页看板
  http.get(`${API_BASE}/web/stats/dashboard`, () => httpResponse.success(mockDashboard)),

  // 系统管理
  http.get(`${API_BASE}/web/system/users`, () =>
    httpResponse.page([
      { id: 1, user_name: 'admin', name: '系统管理员', active: true, dept_id: 1 },
      { id: 2, user_name: 'safety', name: '安全员', active: true, dept_id: 2 },
    ], 2)
  ),
  http.get(`${API_BASE}/web/system/users/options`, () =>
    httpResponse.success([
      { id: 1, name: '系统管理员', code: 'admin' },
      { id: 2, name: '安全员', code: 'safety' },
    ])
  ),
  http.get(`${API_BASE}/web/system/roles`, () =>
    httpResponse.page([{ id: 1, name: '系统管理员', code: 'admin', active: true }], 1)
  ),
  http.get(`${API_BASE}/web/system/depts/options`, () =>
    httpResponse.success([{ id: 1, name: '总部', code: 'HQ' }])
  ),

  // 基础信息
  http.get(`${API_BASE}/web/basic/regions`, () => httpResponse.page([mockRegion], 1)),
  http.get(`${API_BASE}/web/basic/regions/options`, () =>
    httpResponse.success([{ id: 1, name: '东区厂房', code: 'EAST' }])
  ),
  http.get(`${API_BASE}/web/basic/risk-types/options`, () =>
    httpResponse.success([{ id: 1, name: '未佩戴安全帽', code: 'helmet' }])
  ),
  http.get(`${API_BASE}/web/basic/alarm-types/options`, () =>
    httpResponse.success([{ id: 1, name: '设备离线', code: 'offline' }])
  ),

  // 设备
  http.get(`${API_BASE}/web/devices`, () => httpResponse.page([mockDevice], 1)),
  http.get(`${API_BASE}/web/devices/options`, () =>
    httpResponse.success([{ id: 1, name: '东区-01', code: 'DEV-0001' }])
  ),

  // 风险
  http.get(`${API_BASE}/web/risks`, () =>
    httpResponse.page([
      { id: 1, risk_type_id: 1, device_id: 1, status: 'pending', risk_type: { name: '未佩戴安全帽' } },
    ], 1)
  ),
  http.get(`${API_BASE}/web/risks/summary`, () =>
    httpResponse.success({
      total: 1, pending: 1, handled: 0, high: 1, unread: 1,
      accurate_yes: 0, accurate_no: 0, unconfirmed: 1, today: 1,
    })
  ),
  http.get(`${API_BASE}/web/risks/pending-count`, () => httpResponse.success(1)),

  // 实时音视频
  http.get(`${API_BASE}/web/av/devices`, () =>
    httpResponse.page([{ id: 1, name: '东门摄像头', device_code: 'CAM_001', status: 'online' }], 1)
  ),

  // 语音：Embedding 测试 / 重建向量
  http.post(`${API_BASE}/web/voice/embedding/test`, () =>
    httpResponse.success({
      success: true, endpoint: 'http://127.0.0.1:8091/v1', model: 'BAAI/bge-m3',
      dim: 1024, elapsed_ms: 12, message: 'Embedding 可用，返回 1024 维向量',
    })
  ),
  http.post(`${API_BASE}/web/voice/memories/backfill`, () =>
    httpResponse.success({ scanned: 0, updated: 0, failed: 0, remaining: 0 })
  ),

  // 告警
  http.get(`${API_BASE}/web/alarms`, () =>
    httpResponse.page([{ id: 1, alarm_type_id: 1, device_id: 1, status: 'pending' }], 1)
  ),

  // 巡检
  http.get(`${API_BASE}/web/inspects/tasks`, () =>
    httpResponse.page([{ id: 1, name: '9 月例行巡检', status: 'pending' }], 1)
  ),
]
