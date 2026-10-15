# AI Camera 前端

AI 智能监控平台前端。**一个 npm 工程、三个构建目标**：管理后台（Web）、官网介绍页（Web）、
桌面端（Electron 渲染进程）。

> 当前版本: **v0.1.0** | 最后更新: 2026-09-21

---

## 特性

- 🖥️ **三端构建** —— 同一份工程通过三套 Vite 配置产出管理后台（base `/admin/`）、官网（base `/`）与 Electron 渲染进程（base `./`）
- 🌐 **官网介绍页** —— `website.html` 第二入口，深色科技风；全部视觉由 CSS 渐变与内联 SVG 实现，**不引用任何外部静态资源**
- 🔐 **权限控制** —— JWT + Zustand 持久化；后端菜单树驱动侧边栏，`permission_code` 驱动按钮显隐
- 🎥 **视频监控网格** —— 实时监控页支持单屏 / 4 宫格 / 9 宫格切换（当前为占位播放器）
- 📊 **可视化大屏** —— `/screen` 大屏聚合视图，轮询 `/web/stats/screen` 刷新
- 🖨️ **图表分析** —— ECharts（`echarts` + `echarts-for-react`）实现风险统计、告警统计、流量趋势、设备状态分布
- 🧩 **自动生成 API 客户端** —— 20 个 Controller + 205 个类型定义由 Swagger 生成，**严禁手写 API 请求代码**
- 📐 **公共组件体系** —— PageContainer / SearchBar / ActionButtons / SelectorModal / OptionSelect / RegionSelect / DynamicIcon / EmptyState
- 🧪 **双测试体系** —— Vitest + MSW（单测，无需后端）/ Playwright（E2E，真实后端 + `e2e-xxx-modal` 定位规范）
- 📦 **多平台打包** —— Electron Forge 7 支持 NSIS（Windows）/ DMG（macOS）/ DEB + RPM + ZIP（Linux）
- 🔤 **严格类型** —— TypeScript 5.6；`build:admin` 内置 `tsc --noEmit`，**禁止 `as any`** 绕过响应类型
- 🚢 **单镜像双站点** —— `Dockerfile` + `nginx.conf`：`/admin/` 走管理后台产物，`/` 走官网产物

---

## Git 规范

### 换行符配置

本仓库统一**不自动转换换行符**（源文件按 LF 存储），克隆后请执行：

```bash
git config core.autocrlf false
```

原因：跨平台协作时 `core.autocrlf=true` 会在提交时把 LF 转为 CRLF、检出时再转回，导致：

| 问题 | 表现 |
|:---|:---|
| 无意义的整文件 diff | 仅换行符变化却显示整文件被修改，无法 review 真实改动 |
| Lint 规则批量报错 | `linebreak-style` 等规则在 Windows 上对全仓库报错 |
| Shell 脚本无法执行 | `scripts/api-gen.sh` 被转为 CRLF 后在 Linux / Git Bash 下报 `bad interpreter` |
| Playwright 文本断言失败 | 快照因换行符差异不一致 |

---

## 快速开始

```bash
# ── 安装依赖 ──
npm install

# ── 开发模式 ──
npm run dev             # 管理后台  http://localhost:5173/admin/
npm run dev:website     # 官网      http://localhost:5174/
npm run dev:desktop     # Electron 桌面端（渲染进程 :5199）

# ── 构建（依次产出管理后台与官网）──
npm run build           # = build:admin && build:website
npm run build:admin     # tsc --noEmit && vite build                  → dist/
npm run build:website   # vite build --config vite.website.config.ts  → dist-website/
npm run build:desktop   # electron-forge make（当前平台）
npm run typecheck       # tsc --noEmit

# ── 桌面端打包 ──
npm run package:desktop   # 仅打包不生成安装包，产物在 out/
npm run make:win          # Windows NSIS 安装包
npm run make:mac          # macOS DMG
npm run make:linux        # Linux DEB + RPM

# ── API 代码生成（需后端运行在 :8080）──
bash scripts/api-gen.sh

# ── 测试 ──
npm test                  # = vitest run（MSW mock，无需后端）
npm run test:watch        # 单测 watch 模式
npm run test:e2e          # = playwright test
npm run test:e2e:ui       # E2E 交互式调试
npm run test:e2e:report   # 查看 E2E HTML 报告
```

开发态可覆盖后端代理端口与代码生成源：

```bash
# 默认代理到 :8080（开发后端）；冒烟测试时指向测试后端的 8081
VITE_BACKEND_PORT=8081 npm run dev

# api-gen.sh 可指定后端地址（默认 http://localhost:8080）
BACKEND_URL=http://localhost:8081 bash scripts/api-gen.sh
```

---

## 项目结构

```
frontend/
├── index.html                    # 管理后台入口（base /admin/）
├── website.html                  # 官网入口（base /）              ← 单工程双入口
├── package.json                  # 依赖与三端构建脚本
│
├── vite.config.ts                # 管理后台：base '/admin/'，dev :5173，代理 /web /app /uploads
├── vite.website.config.ts        # 官网：base '/'，入口 website.html，产物 dist-website/，dev :5174
├── vite.renderer.config.ts       # Electron 渲染进程：__IS_ELECTRON__=true，dev :5199，removeCrossorigin 插件
├── vite.main.config.ts           # Electron 主进程构建
├── vite.preload.config.ts        # Electron 预加载脚本构建
├── vite-env.d.ts                 # Vite 环境类型声明（含 __IS_ELECTRON__）
│
├── forge.config.ts               # Electron Forge（vite 插件 + nsis / dmg / deb / rpm / zip）
├── forge.env.d.ts                # Forge 环境类型声明
│
├── playwright.config.ts          # E2E（baseURL :5173/admin/，Chromium，webServer 自动起 Vite）
├── vitest.config.ts              # 单测（jsdom + MSW setup，VITE_API_BASE=http://localhost:8080）
│
├── Dockerfile                    # 生产镜像：nginx 同时托管 /admin/（管理后台）与 /（官网），暴露 8080
├── nginx.conf                    # 双站点 Nginx 配置（admin alias + SPA 回退 + assets 强缓存 + gzip）
├── Dockerfile.website            # 官网独立镜像
├── nginx.website.conf            # 官网独立 Nginx 配置
│
├── .env                          # VITE_API_BASE=http://localhost:8080
├── .env.production               # VITE_API_BASE=https://api.aicamera.cn
│
├── electron/
│   ├── main.ts                   # 主进程：BrowserWindow（1440x900，无边框，最小 1024x700）
│   └── preload.ts                # contextBridge 暴露 window.electronAPI = { platform, version, isDesktop }
│
├── scripts/
│   ├── api-gen.sh                # 抓取 Swagger → 生成 Controller 与类型定义（自动选择生成器）
│   ├── api-gen-local.mjs         # 自带生成器（零依赖，swagger2-doc-parser 缺失时的回退）
│   ├── api-gen-preserve.mjs      # preserve 清单：登记生成器无法表达的手写方法/别名
│   ├── swagger-config-typescript.js  # 外部 CLI swagger2-doc-parser 的配置（命名与映射规则）
│   └── README.md                 # 代码生成说明
│
└── src/
    ├── admin/                    # ── 管理后台（39 个页面）──
    │   ├── main.tsx              # 挂载入口
    │   ├── App.tsx               # 路由与布局装配
    │   ├── index.css             # 全局样式与主题变量
    │   ├── api/
    │   │   ├── request.ts        # Axios 实例 + JWT 注入 + 统一错误处理
    │   │   └── generated/        # ★ 自动生成，严禁手工编辑
    │   │       ├── XxxController.ts   # 20 个控制器（静态方法）
    │   │       └── definitions/       # 205 个类型定义（RPageXxx / RXxxList / RXxx）
    │   ├── components/           # 公共组件
    │   │   ├── PageContainer.tsx    # 页面统一容器（标题 / 面包屑 / 操作区）
    │   │   ├── SearchBar.tsx        # 列表页统一搜索栏
    │   │   ├── ActionButtons.tsx    # 表格操作列按钮组（编辑 / 删除 + Popconfirm）
    │   │   ├── SelectorModal.tsx    # 通用选择器弹窗（className 透传，支持 E2E 定位）
    │   │   ├── OptionSelect.tsx     # 下拉选项（对接 ROptionList）
    │   │   ├── RegionSelect.tsx     # 区域选择器
    │   │   ├── DynamicIcon.tsx      # 菜单图标动态渲染
    │   │   └── EmptyState.tsx       # 空状态占位
    │   ├── layouts/
    │   │   └── MainLayout.tsx    # 主布局（侧边栏 + 顶栏 + 内容区）
    │   ├── pages/                # 业务页面（按模块分目录）
    │   ├── store/
    │   │   └── authStore.ts      # Zustand 认证 store（token / 用户信息 / 权限码持久化）
    │   └── types/
    │       └── electron.d.ts     # window.electronAPI 类型声明
    │
    ├── website/                  # ── 官网介绍页（9 个组件）──
    │   ├── main.tsx / App.tsx    # 挂载与组件装配
    │   ├── index.css             # 官网样式（纯 CSS 变量主题，渐变 + SVG）
    │   ├── pages/Home.tsx        # 首页（按区块组装各 Section）
    │   └── components/           # Header / HeroSection / CapabilitySection / SolutionSection
    │                             # / ScenarioSection / StatsSection / ContactSection / Footer
    │
    ├── shared/                   # ── 两个入口共享 ──
    │   └── utils/format.ts       # 日期 / 数字 / 文件大小等格式化工具
    │
    └── test/                     # ── 单测基础设施（Vitest + MSW）──
        ├── setup.ts              # 启动 MSW server，polyfill matchMedia / ResizeObserver
        ├── handlers.ts           # 全部 mock 端点 + httpResponse.success / page / error
        ├── mocks/server.ts       # MSW Node server 实例
        ├── test-utils.tsx        # renderWithProviders（ConfigProvider + AntdApp + MemoryRouter）
        ├── authStore.spec.ts     # 认证 store 单测
        └── format.spec.ts        # 格式化工具单测
```

---

## 双入口构建说明

| 构建目标 | 配置文件 | 入口 HTML | `base` | 产物目录 | 开发端口 |
|:---|:---|:---|:---:|:---|:---:|
| 管理后台（Web） | `vite.config.ts` | `index.html` | `/admin/` | `dist/` | 5173 |
| 官网（Web） | `vite.website.config.ts` | `website.html` | `/` | `dist-website/` | 5174 |
| 管理后台（Electron 渲染进程） | `vite.renderer.config.ts` | `index.html` | `./`（forge 注入） | forge 管理 | 5199 |

> **注意**：`base` 不同意味着**产物不可互换部署**。管理后台的资源路径以 `/admin/` 为根
> （`<script src="/admin/assets/index-xxx.js">`），官网以 `/` 为根。把管理后台产物部署到
> 官网域名下（或反之）会导致静态资源全部 404。

管理后台 / Electron 渲染进程的开发代理（`vite.config.ts` / `vite.renderer.config.ts`）：

| 路径前缀 | 目标 | 说明 |
|:---|:---|:---|
| `/web` | `http://localhost:${VITE_BACKEND_PORT \|\| 8080}` | 管理端 API |
| `/app` | 同上 | 移动端 API（管理后台对接调试用） |
| `/uploads` | 同上 | 上传文件直读（风险图片、处理图片、音频） |

官网配置**不含代理** —— 官网不依赖后端，纯静态展示。

单镜像双站点（`Dockerfile` + `nginx.conf`）：

```
nginx (:8080)
├── /admin/         → /usr/share/nginx/html/admin/   （dist/，base /admin/）
├── /admin/assets/  → 强缓存 1 年（public, immutable）
├── /             → /usr/share/nginx/html/         （dist-website/，base /）
├── /assets/        → 强缓存 1 年
└── /web/、/app/    → 已注释的 proxy_pass 段（由本容器统一入口时启用）
```

---

## API 代码生成

后端接口是唯一真源，前端**不手写任何 API 请求代码**。

```
后端 Swagger 注解（internal/api/handler/web/*.go）
      │  cd backend && bash scripts/gen-swagger.sh
      ▼
backend/docs/web/web_swagger.json
      │  后端运行在 :8080
      │  cd frontend && bash scripts/api-gen.sh
      ▼
src/admin/api/generated/XxxController.ts    ← 23 个控制器 / 190 个 operation
src/admin/api/generated/definitions/*.ts    ← 228 个类型定义（223 swagger definitions + 5 查询对象类型）
      │
      ▼
页面调用 Controller 静态方法（严格使用生成的响应类型）
```

```bash
# 前置条件：后端已启动在 :8080，且已执行过 gen-swagger.sh
cd frontend && bash scripts/api-gen.sh

# 脚本从 ${BACKEND_URL:-http://localhost:8080}/web/api-docs.json 抓取
BACKEND_URL=http://localhost:8081 bash scripts/api-gen.sh

# 离线 / CI：跳过 HTTP 拉取，直接用仓库里的 swagger JSON
SWAGGER_FILE=../backend/docs/web/web_swagger.json bash scripts/api-gen.sh
```

生成器优先使用外部 CLI `swagger2-doc-parser`；**PATH 中没有该 CLI 时自动回退到仓库自带的
零依赖生成器 `scripts/api-gen-local.mjs`**（用法、参数与 preserve 清单见
[scripts/README.md](scripts/README.md)）。

### 强制规范

| 规范 | 说明 |
|:---|:---|
| **必须走生成的 Controller** | 所有 API 调用通过 `DeviceController.pageDevices()` 这类静态方法 |
| **严禁手动拼接 URL** | 禁止 `import request from '../../api/request'` 后写 `request('/web/xxx', { method: 'PUT' })` |
| **严禁 `as any`** | 禁止 `data as any` 绕过类型；类型不匹配时修正后端 Swagger 注解并重新生成 |
| **禁止手动编辑生成物** | `generated/` 下所有文件会被下次生成覆盖（首次生成前的占位实现生成后即被覆盖） |
| **响应读取方式** | 简单列表 `res.data ?? []`（`RXxxList`）；分页列表 `res.data?.items ?? []`（`RPageXxx`） |

```typescript
// ✅ 正确：走生成的 Controller，类型由 RPageDevice 严格约束
import { DeviceController } from '@/admin/api/generated/DeviceController'

const res = await DeviceController.pageDevices({
  page: 1,
  page_size: 20,
  region_id: regionId,
})
const rows = res.data?.items ?? []        // 分页响应固定读 items
const total = res.data?.total ?? 0

// ✅ 正确：简单列表读 data 本身
import { BasicController } from '@/admin/api/generated/BasicController'
const regionRes = await BasicController.regionOptions()
const options = regionRes.data ?? []

// ❌ 错误：手动拼接 URL
// const res = await request('/web/devices', { method: 'GET' })

// ❌ 错误：用 as any 绕过类型
// const rows = (res.data as any)?.items ?? []
```

---

## 环境检测

同一份管理后台代码同时运行在 Web 与 Electron 中，通过两个标志判断当前环境。

| 判断方式 | 类型 | 取值 | 说明 |
|:---|:---|:---|:---|
| `__IS_ELECTRON__` | 编译期常量 | `false` / `true` | 由 Vite `define` 注入；可被 tree-shaking 消除 |
| `window.electronAPI` | 运行期对象 | `undefined` / `{ platform, version, isDesktop }` | 由 `electron/preload.ts` 通过 `contextBridge` 暴露 |

| 配置位置 | `__IS_ELECTRON__` |
|:---|:---:|
| `vite.config.ts`（管理后台 Web） | `false` |
| `vite.website.config.ts`（官网） | `false` |
| `vitest.config.ts`（单测） | `false` |
| `vite.renderer.config.ts`（Electron 渲染进程） | `true` |

```typescript
// 推荐：编译期常量做特性开关
if (__IS_ELECTRON__) {
  // 桌面端专属逻辑：调用 IPC 能力
}

// 运行期兜底：确认 preload 已注入
if (window.electronAPI !== undefined) {
  // 实际用法示例：菜单顶栏在桌面端显示「桌面版」徽标
}
```

> **注意**：桌面端为**无边框窗口**（1440×900，最小 1024×700），开发态加载
> `http://localhost:5199`，生产态加载 `file://` 下的打包产物。渲染进程构建使用
> `vite.renderer.config.ts` 中的 `removeCrossorigin` 插件去掉 `crossorigin` 属性 ——
> 否则 ES Module 脚本在 `file://` 协议下加载失败。
> `preload.ts` 只暴露最小能力集 `{ platform, version, isDesktop }`。

---

## 技术栈

| 项目 | 版本 | 说明 |
|:---|:---|:---|
| React | ^19.0.0 | 函数组件 + Hooks（配合 `@ant-design/v5-patch-for-react-19`） |
| TypeScript | ^5.6.0 | `tsc --noEmit` 作为 `build:admin` 前置 |
| Vite | ^5.4.0 | 开发服务器与构建（三套配置） |
| Ant Design | ^5.22.0 | 主 UI 组件库 |
| @ant-design/icons | ^5.5.0 | 图标库（`DynamicIcon` 动态渲染菜单图标） |
| React Router | ^7.0.0 | 前端路由（管理后台 SPA） |
| Zustand | ^4.5.0 | 轻量状态管理（`authStore`） |
| Axios | ^1.7.0 | HTTP 客户端（`api/request.ts` + JWT 拦截器） |
| ECharts | ^5.5.0 | 图表（`echarts-for-react` 包装） |
| dayjs | ^1.11.0 | 日期处理 |
| Less | ^4.6.7 | 样式预处理（antd 定制） |
| Electron | ^33.0.0 | 桌面端运行时 |
| Electron Forge | ^7.7.0 | 打包与 Maker（NSIS / DMG / DEB / RPM / ZIP） |
| Playwright | ^1.60.0 | E2E 测试 |
| Vitest | ^4.1.8 | 单元测试 |
| MSW | ^2.14.6 | API Mock（单测无需启动后端） |
| @testing-library/react | ^16.3.2 | 组件测试 |

Web 与桌面端共用同一套技术栈（React 19 + Vite 5 + Ant Design 5 + Zustand 4 + React Router 7 + ECharts 5），
差异只在构建配置与运行环境。

---

## 环境变量

| 文件 | 变量 | 值 |
|:---|:---|:---|
| `.env` | `VITE_API_BASE` | `http://localhost:8080` |
| `.env.production` | `VITE_API_BASE` | `https://api.aicamera.cn` |

开发态另有可选的 `VITE_BACKEND_PORT`（默认 `8080`），用于调整 `vite.config.ts` 的代理目标。

---

## 测试

### 单元测试（Vitest + MSW，无需后端）

```bash
npm test            # = vitest run
npm run test:watch
npx vitest run
```

| 配置项 | 值 |
|:---|:---|
| 测试环境 | `jsdom` |
| setup 文件 | `src/test/setup.ts`（启动 MSW server，polyfill `matchMedia` / `ResizeObserver`） |
| 用例匹配 | `src/**/*.{test,spec}.{ts,tsx}` |
| 排除目录 | `node_modules`、`dist`、`dist-website`、`e2e` |
| 环境变量 | `VITE_API_BASE=http://localhost:8080` |
| 超时 | 20 秒 |

约定：

- **新增 API 时必须同步在 `src/test/handlers.ts` 补充对应的 mock 端点**。
- 响应辅助对象：`httpResponse.success(data)` / `httpResponse.page(items, total)` / `httpResponse.error(msg)`。
- 组件测试用 `src/test/test-utils.tsx` 的 `renderWithProviders`（已装配 `ConfigProvider` + `AntdApp` + `MemoryRouter`）。
- 已有点位：`src/test/authStore.spec.ts`（认证 store）、`src/test/format.spec.ts`（格式化工具）。

### 端到端测试（Playwright）

```bash
npx playwright test                        # 全部用例
npx playwright test e2e/smoke --workers=1  # 冒烟测试（串行）
npx playwright test --ui                   # 交互式调试
npx playwright show-report                 # 查看 HTML 报告
```

| 配置项 | 值 |
|:---|:---|
| `testDir` | `./e2e` |
| `baseURL` | `http://localhost:5173/admin/`（可用 `E2E_PORT` / `E2E_BASE_URL` 覆盖） |
| 浏览器 | Chromium（Desktop Chrome） |
| `webServer` | 自动执行 `npx vite --port 5173`（已运行时复用） |
| 重试 | CI 下 2 次 |
| 失败产物 | `test-results/`（trace 在首次重试时记录）；失败截图在 `e2e/screenshots/`（已 gitignore） |

> **重要**：**Modal 必须使用 `e2e-xxx-modal` 类选择器定位**，禁止使用通用 `.ant-modal`、
> `.ant-modal .ant-modal-footer`、`page.locator('.ant-modal').last()`。
>
> 理由：存在嵌套弹窗（如处理风险 Modal 内打开图片预览、表单 Modal 内打开设备选择 Modal）时，
> 通用 `.ant-modal` 会匹配到**错误的弹窗**，导致测试不稳定。
>
> ```typescript
> // ✅ 正确
> await page.locator('.e2e-risk-handle-modal').waitFor()
> await page.locator('.e2e-device-select').click()
>
> // ❌ 错误
> await page.locator('.ant-modal').last().waitFor()
> await page.locator('.ant-modal-footer button.ant-btn-primary').click()
> ```

- 新增 antd `<Modal>` 必须添加 `className="e2e-xxx-modal"`。
- `SelectorModal` 会把传入的 `className` 应用到 Modal（`.e2e-xxx`）与触发按钮（`.e2e-xxx-btn`）。
- 冒烟测试（`e2e/smoke/`）串行执行，spec 按文件名字母序；功能性用例编号 `01a` ~ `10f`，
  **全页面覆盖类收尾用例固定 `99-` 前缀**。

---

## 部署

```bash
# 单镜像双站点（/admin/ 管理后台 + / 官网）
docker build -t ai-camera-frontend .
docker run -p 8080:8080 ai-camera-frontend

# 仅官网镜像
docker build -f Dockerfile.website -t ai-camera-website .
```

镜像内 nginx 同时托管两个站点：

| 路径 | 内容 | 产物来源 | base |
|:---|:---|:---|:---|
| `/admin/` | 管理后台 | `dist/` → `/usr/share/nginx/html/admin` | `/admin/` |
| `/` | 官网介绍页 | `dist-website/` → `/usr/share/nginx/html` | `/` |

细节见 [../docs/部署文档/01-部署与运维.md](../docs/部署文档/01-部署与运维.md) 第 3.2 / 3.3 节。

---

## 相关文档

- [../README.md](../README.md) — 仓库根说明
- [../backend/README.md](../backend/README.md) — 后端技术栈与 200 个 API 路由总览
- [../app/README.md](../app/README.md) — 移动端说明
- [../docs/介绍网站/README.md](../docs/介绍网站/README.md) — 官网设计风格与视觉实现方案
- [../docs/开发文档/页面功能和接口/00-页面功能与接口总览.md](../docs/开发文档/页面功能和接口/00-页面功能与接口总览.md) — 48 个页面目录与接口对照
- [../CLAUDE.md](../CLAUDE.md) — 前端开发规范与 E2E 规范
