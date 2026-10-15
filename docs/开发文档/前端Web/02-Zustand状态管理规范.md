# Zustand状态管理规范

> 版本: v0.1.0
> 最后更新: 2026-09-21
> 范围: `frontend/src/admin/store/authStore.ts` —— 管理后台唯一的全局 store

## 1. 为什么用Zustand

| 维度 | 说明 |
|:-----|:-----|
| 依赖体积 | `zustand ^4.5.0` 无 Provider 包裹要求，不需要在 `App.tsx` 外层套 Context |
| 使用成本 | `create()` 一个函数即完成 store 定义，无 action / reducer / dispatch 样板 |
| 组件外可取 | `useAuthStore.getState()` 在 React 组件树之外也能读写（当前代码未用到，但保留了能力） |
| 选择器订阅 | `useAuthStore((s) => s.token)` 天然按字段订阅，避免 Context 全量重渲染 |
| 与 React 19 兼容 | zustand v4 基于 `useSyncExternalStore`，在 React 19 下无并发渲染告警 |

**判定边界**：只有跨页面共享、且生命周期与登录态一致的数据才进 store。以下内容**不进 store**：

- 列表数据、分页页码、弹窗开关 → 页面内 `useState`（生命周期跟随页面）；
- 下拉选项（区域 / 角色 / 岗位） → 组件内 `useState`（见 `OptionSelect.tsx`）；
- 后端菜单树本身不落盘，仅在会话内共享（详见第 3 节）。

> **注意**：本项目全工程只有 `src/admin/store/authStore.ts` 一个 store。新增 store 前必须论证「为什么不能用 `useState` + props 传递」，避免 store 退化为全局变量桶。

## 2. Store定义规范

### 2.1 完整代码

```typescript
// src/admin/store/authStore.ts（全文）
import { create } from 'zustand'
import type { Menu } from '../api/generated/definitions/Menu'

const TOKEN_KEY = 'token'
const PERM_CODES_KEY = 'perm_codes'
const USER_KEY = 'user_info'

/** 当前登录用户（对应后端 `response.LoginData` 的非 token 部分） */
export interface AuthUser {
  user_id?: number
  name?: string
  user_name?: string
  dept_id?: number | null
  data_perm?: string
  cross_dept_ids?: string
  avatar?: string
  [key: string]: unknown
}

/** localStorage 读取（容错：隐私模式 / 存储被禁用时返回兜底值） */
function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writeStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    /* ignore */
  }
}

function removeStorage(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    /* ignore */
  }
}

function readJSON<T>(key: string, fallback: T): T {
  const raw = readStorage(key)
  if (!raw) return fallback
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function readPermCodes(): string[] {
  const parsed = readJSON<unknown>(PERM_CODES_KEY, [])
  return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
}

/**
 * 递归提取菜单树中的全部 `permission_code`（去重）。
 * 菜单树既驱动侧边栏，也驱动按钮级权限，因此这里收集的是全量权限码。
 */
export function extractPermCodes(menus?: Menu[] | null): string[] {
  const codes = new Set<string>()
  const walk = (list?: Menu[] | null) => {
    if (!Array.isArray(list)) return
    for (const menu of list) {
      if (!menu) continue
      const code = menu.permission_code
      if (typeof code === 'string' && code.trim()) {
        codes.add(code.trim())
      }
      if (Array.isArray(menu.children) && menu.children.length > 0) {
        walk(menu.children)
      }
    }
  }
  walk(menus)
  return Array.from(codes)
}

interface AuthState {
  token: string | null
  user: AuthUser | null
  permissions: string[]
  permCodes: string[]
  menus: Menu[]
  setToken: (token: string) => void
  setUser: (user: AuthUser | null) => void
  setPermissions: (permissions: string[]) => void
  setPermCodes: (permCodes: string[]) => void
  setMenus: (menus: Menu[]) => void
  logout: () => void
  isAuthenticated: () => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  token: readStorage(TOKEN_KEY),
  user: readJSON<AuthUser | null>(USER_KEY, null),
  permissions: [],
  permCodes: readPermCodes(),
  menus: [],

  setToken: (token: string) => {
    writeStorage(TOKEN_KEY, token)
    set({ token })
  },

  setUser: (user: AuthUser | null) => {
    if (user) {
      writeStorage(USER_KEY, JSON.stringify(user))
    } else {
      removeStorage(USER_KEY)
    }
    set({ user })
  },

  setPermissions: (permissions: string[]) => {
    set({ permissions: Array.isArray(permissions) ? permissions : [] })
  },

  setPermCodes: (permCodes: string[]) => {
    const list = Array.isArray(permCodes) ? permCodes : []
    writeStorage(PERM_CODES_KEY, JSON.stringify(list))
    set({ permCodes: list })
  },

  /** 写入菜单树，并自动递归提取 permission_code 持久化到 permCodes */
  setMenus: (menus: Menu[]) => {
    const list = Array.isArray(menus) ? menus : []
    const permCodes = extractPermCodes(list)
    writeStorage(PERM_CODES_KEY, JSON.stringify(permCodes))
    set({ menus: list, permCodes })
  },

  logout: () => {
    removeStorage(TOKEN_KEY)
    removeStorage(PERM_CODES_KEY)
    removeStorage(USER_KEY)
    set({ token: null, user: null, permissions: [], permCodes: [], menus: [] })
  },

  isAuthenticated: () => Boolean(get().token),
}))
```

### 2.2 逐段说明

**① 状态字段的类型必须显式声明为 interface `AuthState`**，而不是靠 `create` 的类型推导。原因：组件侧 `useAuthStore((s) => s.token)` 的返回类型完全来自这个 interface，推导出来的类型一旦字段增多会出现联合类型污染。

**② 初始值直接来自 `localStorage`，是「同步初始化」而非「effect 里 hydrate」**：

```typescript
token: readStorage(TOKEN_KEY),
user: readJSON<AuthUser | null>(USER_KEY, null),
permCodes: readPermCodes(),
```

这带来两个直接收益：

- `ProtectedRoute` 在**首帧**就能拿到 `token`，无需先渲染空态再跳转，不会出现「刷新页面闪一下登录页」；
- `request.ts` 的请求拦截器直接读 `localStorage`（不读 store），因此即使某个请求在 React 挂载前发出，`Authorization` 头也一定是正确的。

**③ 所有 `localStorage` 访问都包 `try/catch`**。Safari 隐私模式、企业策略禁用存储、存储配额耗尽时 `localStorage.setItem` 会抛 `QuotaExceededError`。这些异常不允许冒泡到 store 初始化路径上——一旦 `create()` 抛异常，整个应用白屏。

**④ `readJSON` 必须容错**。`localStorage` 里的内容可能是上一个版本写下的、结构已变的 JSON；`JSON.parse` 失败时返回 `fallback` 而不是抛错：

```typescript
function readJSON<T>(key: string, fallback: T): T {
  const raw = readStorage(key)
  if (!raw) return fallback
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}
```

**⑤ `readPermCodes` 额外做数组元素类型过滤**。`perm_codes` 是字符串数组，若被外部写入了 `[1, 2, null]` 这类内容，必须过滤掉非字符串项，保证 `permCodes.includes(code)` 的语义正确：

```typescript
function readPermCodes(): string[] {
  const parsed = readJSON<unknown>(PERM_CODES_KEY, [])
  return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
}
```

**⑥ setter 的写入顺序：先落盘，再 `set()`**。反例是先 `set()` 再写盘——一旦写盘抛异常，内存状态已变更而磁盘未变更，下次刷新会出现「内存与磁盘不一致」的鬼状态。

**⑦ `logout` 是全量清理的唯一入口**。三个 key 必须同时删除，且 `set()` 必须把 5 个字段全部复位（不能只置 `token: null`），否则残留的 `menus` / `permCodes` 会在下一个用户登录前被短暂读取，造成越权 UI。

**⑧ `isAuthenticated` 定义成函数而非派生字段**。作为字段会在每次 `set` 时都需要重算维护；作为函数则按调用时读取最新值：

```typescript
isAuthenticated: () => Boolean(get().token),
```

## 3. 持久化策略

### 3.1 为什么不用 `persist` 中间件

`zustand ^4.5.0` 自带 `persist` 中间件，但本项目**不使用**，原因如下：

| 原因 | 说明 |
|:-----|:-----|
| 同步初始化需求 | `persist` 默认异步 rehydrate，首帧拿到的是初始值（`token: null`），`ProtectedRoute` 会先跳 `/login` 再跳回；改造成同步需要 `skipHydration` + 手动 `rehydrate()`，比手写更复杂 |
| 持久化粒度不可控 | `persist` 的 `partialize` 只能按字段选择，无法对「同一字段写入时同时派生另一个字段」建模（`setMenus` 必须顺带写 `perm_codes`） |
| `menus` 不应落盘 | 菜单树体积大且属于服务端权威数据，落盘会产生「后端已改菜单、前端仍用旧树」的陈旧问题 |
| 异常不可控 | `persist` 内部直接调用 `localStorage`，隐私模式下抛出的异常会冒泡到 store 创建路径，导致白屏；本项目的 `try/catch` 包装是刻意为之 |
| 键名可控 | 后端与桌面端共用同一批 key（`token` / `perm_codes` / `user_info`），需要精确控制键名，`persist` 的 `name` 只是前缀 |

### 3.2 token 与 perm_codes 的持久化边界

| key | 写入时机 | 读取时机 | 清理时机 |
|:----|:---------|:---------|:---------|
| `token` | `Login.tsx` 登录成功后 `setToken(data.token)` | ① `create()` 初始化；② `request.ts` 请求拦截器（**直接读 localStorage，不读 store**） | `logout()`；`request.ts` 收到 401 时 `localStorage.removeItem('token')` |
| `perm_codes` | `setMenus()` 递归提取后写入；`setPermCodes()` 显式写入 | `create()` 初始化（`readPermCodes()`） | `logout()` |
| `user_info` | `setUser()` | `create()` 初始化 | `logout()`；`setUser(null)` |

> **注意**：`request.ts` 与 store **共用** `token` 这个 key，这是刻意的——拦截器运行在 React 组件树之外，读 store 需要 `useAuthStore.getState()`，而直接读 `localStorage` 少一层耦合。代价是两处必须保持 key 一致，改动时必须同步。

### 3.3 menus / user 为什么这样处理

**`menus` 只存内存**（不落盘）：

- 菜单树由 `GET /web/profile/menus` 返回（`MainLayout.tsx` 首次挂载时拉取），是服务端权威数据；
- 落盘会让「管理员刚改了某角色的菜单权限、用户刷新页面仍是旧菜单」，而重新拉一次的成本很低（一次请求）；
- 体积大：完整菜单树含目录 + 菜单 + 按钮三级节点，落盘会挤占 `localStorage` 配额。

```typescript
// src/admin/layouts/MainLayout.tsx（片段）—— 菜单拉取失败不阻塞页面
useEffect(() => {
  let alive = true
  ProfileController.getCurrentUserMenus()
    .then((res) => {
      if (alive && Array.isArray(res?.data)) {
        setMenus(res.data)
      }
    })
    .catch(() => {
      /* 菜单拉取失败时保留静态兜底菜单 */
    })
  return () => {
    alive = false
  }
}, [setMenus])
```

**`user` 落盘**（`user_info`）：

- 顶栏组件（`MainLayout.tsx`）在任何页面都可能渲染用户名 / 头像，若只存内存，页面刷新后会短暂显示「未登录」；
- `user` 内容很小（`user_id` / `name` / `dept_id` / `data_perm` / `cross_dept_ids` / `avatar`），代价可忽略；
- 它是**展示用快照**，不是鉴权依据——鉴权只看 `token`，因此陈旧风险可接受。

**`permissions` 只存内存**：

- 它是登录接口返回的原始数组，仅在 `Login.tsx` 中由 `setPermissions(data.permissions ?? [])` 写入一次；
- 页面级的按钮权限实际消费的是 `permCodes`（来自菜单树），`permissions` 目前未参与渲染判断，落盘无收益。

**空数组的兜底语义**（贯穿三个字段）：`setPermissions` / `setPermCodes` / `setMenus` 都先把入参 `Array.isArray` 归一化，`undefined` / `null` 一律转成 `[]`。消费侧因此只需判断 `length === 0`，不必处理 `undefined`。

## 4. 权限标识体系

### 4.1 `extractPermCodes` 递归提取 `permission_code`

菜单树是三级结构 `directory → menu → button`，`permission_code` 在**每一级都可能存在**（目录节点也可带码）。因此提取必须递归且**收集全量**，不能只取 `type === 'button'` 的叶子：

```typescript
export function extractPermCodes(menus?: Menu[] | null): string[] {
  const codes = new Set<string>()
  const walk = (list?: Menu[] | null) => {
    if (!Array.isArray(list)) return
    for (const menu of list) {
      if (!menu) continue
      const code = menu.permission_code
      if (typeof code === 'string' && code.trim()) {
        codes.add(code.trim())
      }
      if (Array.isArray(menu.children) && menu.children.length > 0) {
        walk(menu.children)
      }
    }
  }
  walk(menus)
  return Array.from(codes)
}
```

设计要点：

| 要点 | 原因 |
|:-----|:-----|
| `Set` 去重 | 同一权限码可能挂在多个菜单下（如多个入口共享 `risk:view`），`Set` 保证数组无重复 |
| `code.trim()` | 后端菜单名可能带空格（人工录入），去空格后比较才可靠 |
| `Array.isArray` 三重防御 | 入参可能是 `undefined`（菜单未加载）、`null`、或 `children` 被写成非数组 |
| 空值节点跳过 | `if (!menu) continue` 兼容后端返回 `[null, {...}]` 这类脏数据 |

**调用链**：`MainLayout` 拉菜单 → `setMenus(res.data)` → 内部调 `extractPermCodes` → 写 `localStorage['perm_codes']` + `set({ menus, permCodes })`。因此页面**永远不需要**自己遍历菜单树算权限。

### 4.2 按钮级权限的消费方式

统一形态：`permCodes.includes('xxx:yyy:zzz')`，命名遵循 `<域>:<资源>:<动作>`：

```typescript
// 风险监控（src/admin/pages/risk/RiskMonitorList.tsx 片段）
const permCodes = useAuthStore((s) => s.permCodes)
const canHandle = permCodes.includes('risk:handle')
const canAccurate = permCodes.includes('risk:accurate')
const canDelete = permCodes.includes('risk:delete')
```

消费位置固定为三处：

| 位置 | 写法 |
|:-----|:-----|
| 页面右上角主按钮 | `extra={canCreate ? <Button .../> : undefined}` |
| 表格操作列按钮 | `{canUpdate && <Button ... />}` |
| 表单内受保护控件的 `disabled` | `disabled={!canUpdate}` |

### 4.3 空数组时放行的取舍

列表页普遍采用**「空数组放行」**语义：

```typescript
// src/admin/pages/system/ConfigList.tsx（片段）
const permCodes = useAuthStore((s) => s.permCodes)
// permCodes 为空说明菜单权限尚未加载，此时不隐藏按钮，避免整页操作被锁死
const can = (code: string) => permCodes.length === 0 || permCodes.includes(code)
const canCreate = can('system:config:create')
const canUpdate = can('system:config:update')
const canDelete = can('system:config:delete')
```

**取舍分析**：

| 方案 | 表现 | 后果 |
|:-----|:-----|:-----|
| ✅ 空数组放行（本项目列表页采用） | 菜单接口失败 / 慢时按钮仍可见可点 | 按钮可见性仅作**体验优化**，真正的拦截在后端 Casbin；菜单接口挂掉时页面仍可用 |
| ❌ 空数组全部隐藏 | 菜单接口失败时整页按钮消失，页面变成只读 | 用户无法操作，且无法自救（菜单接口正是失败的那个） |
| 严格判定（风险监控采用） | `permCodes.includes(...)` 直接判定 | 风险处理是**写操作 + 有审批语义**，宁可少给不可多给；该页依赖菜单接口成功返回 |

> **注意**：这不是「安全边界」。前端 `permCodes` 只控制渲染，**任何写操作都必须由后端 Casbin 中间件二次校验**。前端的取舍只影响可用性，不影响安全性。

## 5. 组件中的消费模式

### 5.1 选择器订阅

Zustand 的 `useStore(selector)` 会在 selector 返回值变化时触发重渲染。**必须按字段拆开订阅**：

```typescript
// src/admin/layouts/MainLayout.tsx（片段）—— 每个字段一行，各自独立订阅
const user = useAuthStore((state) => state.user)
const menus = useAuthStore((state) => state.menus)
const setMenus = useAuthStore((state) => state.setMenus)
const logout = useAuthStore((state) => state.logout)
```

```typescript
// src/admin/App.tsx（片段）
function ProtectedRoute({ children }: { children: ReactNode }) {
  const token = useAuthStore((state) => state.token)
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}
```

### 5.2 全量订阅的问题

```typescript
// ❌ 全量订阅：任一字段变化都重渲染
const { user, menus, permCodes } = useAuthStore()
```

后果链：

1. `setMenus()` 会同时更新 `menus` 与 `permCodes` 两个字段 → 触发 2 次 `set`；
2. 全量订阅的组件（往往是整个页面）会因此重渲染 2 次；
3. 若该组件的渲染成本高（含 `Table` + `columns` 数组 + `ActionButtons`），会表现为登录后菜单加载完成瞬间的明显卡顿。

**规避手段**（本项目已全部采用）：

| 手段 | 位置 |
|:-----|:-----|
| 按字段拆订阅 | `MainLayout.tsx`、`App.tsx`、各列表页 |
| 派生数据用 `useMemo` | `MainLayout.tsx` 的 `menuItems` / `selectedKeys` |
| 只在需要处订阅 `permCodes`（不订阅 `menus`） | 各列表页 —— 页面渲染不依赖菜单树本身 |
| 静态常量提到组件外 | `StatusTag.tsx` 的 `STATUS_MAP`、`DynamicIcon.tsx` 的 `ICON_MAP` |

> **注意**：不需要用 `useShallow`。本项目 store 里的对象字段（`user` / `menus`）都是整体替换（`set({ user })`），不存在「同一引用内部字段变化」的情况，浅比较收益为零。

## 6. 常见反模式

### 6.1 在组件里直接操作 localStorage

```typescript
// ❌ 绕过 store，且没有容错：隐私模式下抛异常导致白屏
const login = (token: string) => {
  localStorage.setItem('token', token)
  navigate('/')
}

// ✅ 统一走 store setter（内部落盘 + 容错 + 触发订阅者更新）
const setToken = useAuthStore((state) => state.setToken)
const login = (token: string) => {
  setToken(token)
  navigate('/')
}
```

### 6.2 手动遍历菜单树算权限

```typescript
// ❌ 每个页面重复实现遍历，且与 store 的 permCodes 可能不一致
const hasPerm = (menus: Menu[], code: string): boolean => {
  for (const m of menus) {
    if (m.permission_code === code) return true
    if (m.children && hasPerm(m.children, code)) return true
  }
  return false
}

// ✅ 直接用 store 已提取好的扁平数组
const permCodes = useAuthStore((s) => s.permCodes)
const canDelete = permCodes.includes('risk:delete')
```

### 6.3 只复位 token

```typescript
// ❌ 残留 menus / permCodes，下一个用户登录前会被短暂读取
logout: () => {
  removeStorage(TOKEN_KEY)
  set({ token: null })
}

// ✅ 三个 key 全删 + 五个字段全复位（见 2.1 完整代码的 logout）
```

### 6.4 把列表数据塞进 store

```typescript
// ❌ 列表数据生命周期跟随页面，放 store 会造成「离开页面后数据仍在」的脏状态
const [riskList, setRiskList] = useState<RiskRecord[]>([])

// ✅ 页面内 useState + useCallback 取数（见前端编码规范 6.1）
```

### 6.5 在 selector 里创建新对象

```typescript
// ❌ 每次渲染都返回新数组，引用不等 → 无限重渲染
const codes = useAuthStore((s) => s.permCodes.filter((c) => c.startsWith('risk')))

// ✅ 订阅原字段，派生逻辑放到组件内 useMemo
const permCodes = useAuthStore((s) => s.permCodes)
const riskCodes = useMemo(() => permCodes.filter((c) => c.startsWith('risk')), [permCodes])
```

### 6.6 依赖重渲染做副作用

```typescript
// ❌ 用 subscribe 手动监听，绕过了 React 的渲染协调
useEffect(() => useAuthStore.subscribe((s) => { if (!s.token) navigate('/login') }), [])

// ✅ 声明式：由 ProtectedRoute 的 token 订阅 + <Navigate> 完成跳转
```

## 7. 测试

### 7.1 测试文件

`src/test/authStore.spec.ts`（Vitest + jsdom，无需 MSW——store 不直接发请求）。

### 7.2 隔离每个用例

store 是**模块级单例**，用例之间必须显式复位，否则会互相污染：

```typescript
describe('admin/store/authStore', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.setState({ token: null, user: null, permissions: [], permCodes: [], menus: [] })
  })
  // ...
})
```

> **注意**：这里用 `setState` 直接复位而不是调 `logout()`，因为 `logout()` 会顺带清 `localStorage`——而本用例可能正需要保留 `localStorage` 里预置的数据来验证初始化行为。

### 7.3 断言要点

store 测试**必须**覆盖持久化副作用（只断言内存状态是不够的，无法发现「忘记写盘」的回归）：

```typescript
it('setMenus 自动提取 permCodes 并持久化', () => {
  useAuthStore.getState().setMenus(tree)

  expect(useAuthStore.getState().menus).toHaveLength(2)
  expect(useAuthStore.getState().permCodes).toContain('system:user:create')
  expect(JSON.parse(localStorage.getItem('perm_codes') ?? '[]')).toContain('system:user:list')
})

it('setToken / logout 同步 localStorage', () => {
  useAuthStore.getState().setToken('abc')
  expect(useAuthStore.getState().isAuthenticated()).toBe(true)
  expect(localStorage.getItem('token')).toBe('abc')

  useAuthStore.getState().setMenus(tree)
  useAuthStore.getState().logout()

  expect(useAuthStore.getState().isAuthenticated()).toBe(false)
  expect(useAuthStore.getState().menus).toEqual([])
  expect(useAuthStore.getState().permCodes).toEqual([])
  expect(localStorage.getItem('token')).toBeNull()
  expect(localStorage.getItem('perm_codes')).toBeNull()
})
```

**必测清单**：

| 用例 | 断言重点 |
|:-----|:---------|
| `extractPermCodes` 递归 + 去重 | 目录 / 菜单 / 按钮三级的码都被收集，重复码只出现一次 |
| `extractPermCodes` 空值安全 | `undefined` / `[]` 返回 `[]`，不抛异常 |
| `setMenus` 持久化 | `permCodes` 派生正确 + `localStorage['perm_codes']` 内容正确 |
| `setToken` / `logout` | 内存与 `localStorage` **双向**同步 |
| `setPermissions` / `setPermCodes` 入参为 `undefined` | 归一化为 `[]`，不写入 `"undefined"` 字符串 |

### 7.4 组件测试中 Mock store

**不需要** mock 模块。直接在用例里 `setState` 注入权限码，组件读取时自然生效：

```typescript
import { useAuthStore } from '../admin/store/authStore'

beforeEach(() => {
  localStorage.clear()
  useAuthStore.setState({ token: 'test-token', permCodes: [], menus: [], permissions: [], user: null })
})

it('无 risk:delete 权限时不渲染删除按钮', () => {
  useAuthStore.setState({ permCodes: ['risk:handle'] })
  // renderWithProviders(<RiskMonitorList />) 后断言删除按钮不存在
})

it('permCodes 为空时按钮全部放行', () => {
  useAuthStore.setState({ permCodes: [] })
  // 断言删除按钮可见（对应 4.3 的「空数组放行」语义）
})
```

> **注意**：`src/test/test-utils.tsx` 的 `renderWithProviders` 已包含 `ConfigProvider`（中文语言包 + 紧凑尺寸）与 `MemoryRouter`，组件测试统一用它，不要在用例里各自重搭 Provider。
