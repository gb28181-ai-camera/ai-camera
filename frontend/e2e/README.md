# 端到端测试（Playwright）

管理后台 E2E 测试，覆盖登录、系统管理、基础信息、资源管理、风险/告警、巡检、升级、告警设置、统计与大屏等模块。

## 目录结构

```
e2e/
├── fixtures/
│   ├── auth.ts              # 登录 fixture（真实登录 + storageState 复用）
│   └── auth.setup.ts        # 全局准备：登录一次并落盘 e2e/.auth/admin.json
├── helpers/
│   ├── console.ts           # console error / pageerror 采集器
│   └── crud.ts              # 通用 CRUD / 搜索 / 表格断言 / 接口造数工具
├── smoke/                   # 冒烟测试（串行执行，按文件名字母序）
│   ├── 01a-login.spec.ts
│   ├── ...
│   └── 99-all-list-page.spec.ts   # 全页面覆盖（必须最后执行）
└── screenshots/             # 用例截图
```

## 环境准备

测试后端端口为 **8081**（`backend/config/config-test.yaml`）。

```bash
# 1. 清空测试库
cd backend && go run ./tests/testutil/resetdb/main.go
# 2. 灌入初始数据
cd backend && go run ./tests/testutil/initdb/main.go
# 3. 启动测试后端（8081）
cd backend && CONFIG_PATH=config/config-test.yaml go run ./cmd/server/main.go
# 4. 启动前端（5173）
cd frontend && VITE_BACKEND_PORT=8081 npm run dev
```

### 关于 `VITE_API_BASE`（重要）

生成的 Controller 用 `import.meta.env.VITE_API_BASE` 作为 axios baseURL，而仓库 `.env` 中该值为
`http://localhost:8080` —— 浏览器会**直连 8080 并绕过 Vite 代理**，但测试后端监听 8081 且后端
**没有 CORS 中间件**，跨域请求会被浏览器拦截（表现为页面全空、接口报 500/无响应）。

因此测试运行时必须把 `VITE_API_BASE` 置空（走相对路径 → Vite 代理 → 8081）：

- `playwright.config.ts` 的 `webServer.env` 已自动注入
  `VITE_BACKEND_PORT=8081` 与 `VITE_API_BASE=`；
- 若是手工启动 dev server，请显式指定：`VITE_API_BASE= VITE_BACKEND_PORT=8081 npm run dev`。

### 浏览器

默认使用 Playwright 自带 chromium（首次需 `npx playwright install chromium`）。
若环境无法下载浏览器，可复用系统已安装的 Edge：

```bash
cd frontend && E2E_BROWSER_CHANNEL=msedge npx playwright test
```

## 执行

```bash
cd frontend

# 全部冒烟（串行，含 setup；99- 因文件名排序最后执行）
E2E_BROWSER_CHANNEL=msedge npx playwright test

# 单文件
E2E_BROWSER_CHANNEL=msedge npx playwright test e2e/smoke/02a-region.spec.ts

# 只跑全页面覆盖（必须最后）
E2E_BROWSER_CHANNEL=msedge npx playwright test e2e/smoke/99-all-list-page.spec.ts
```

- `smoke` project 配置为 `fullyParallel: false, workers: 1`（串行），并 `dependencies: ['setup']`，
  复用 `e2e/.auth/admin.json` 中的登录态，避免每个用例重复登录。
- `chromium` project 覆盖 `smoke/` 之外的用例（无认证态）。

## 编写约定

1. **Modal 只允许用 `e2e-xxx-modal` 类定位**，禁止 `.ant-modal`、`.ant-modal-footer button`、
   `page.locator('.ant-modal').last()`。嵌套弹窗存在时通用选择器会匹配错弹窗。
2. **禁止 `waitForTimeout` 作同步手段**：用 `expect(...).toBeVisible()` 自动等待或
   `page.waitForResponse(...)`。
3. **测试数据带时间戳**（`uniqName('E2E区域')`）并在用例内清理。
4. `antd` 会为「恰好两个汉字」的按钮文案插入空格（`确 定` / `取 消` / `登 录`），
   统一用 `OK_BUTTON` / `CANCEL_BUTTON` / `LOGIN_BUTTON` 正则常量匹配。
5. 按钮带图标时其可访问名包含图标名（如 `eye 查看`），需要精确匹配时用
   `locator('button').filter({ hasText: /^查\s*看$/ })`。

## 已知问题（用例已用「已知问题」用例固化，修复后需同步调整）

1. **按钮级权限码与菜单树不一致（阻断级）**
   页面用 `basic:region:create`、`system:user:create`、`upgrade:issue`、`iot-card:replace`
   等 **64 个**权限码判定按钮显隐，而菜单树 `permission_code` 提供的是 `region:create`、
   `system:view` 等 → **新增/编辑/删除等按钮对任何用户（含超管）都不渲染**。
   受影响模块：基础信息、巡检、告警设置、风险通知、第三方平台、升级管理、系统管理全部子页。
2. **空查询参数导致列表恒为空（阻断级）**
   `UserController.pageUsers`、`RoleController.pageRoles`、`PostController.pagePosts`、
   `NotificationController.pageNotifications` 生成 `?keyword=&active=` / `?dept_id=&active=`，
   后端把空串当作有效过滤条件（`active=false` / `dept_id=0`）→
   用户/角色/岗位/通知列表恒为 0 条（页面永远显示「暂无数据」）。
   对照：`BasicController`/`DeviceController` 使用 `q()` 助手跳过空值，行为正常。
3. **巡检任务状态文案未本地化**：`StatusTag` 的 `inspect` 字典只覆盖
   `pending/todo/doing/processing/done/completed`，而实际状态是
   `pending/published/finished/canceled` → 表格「状态」列直接显示英文 `published`。
4. **巡检任务「查看」不转已读**：`InspectTaskList.openView` 仅打开弹窗（复用列表行数据，
   不发请求），后端也没有任务级已读接口 → `read` 恒为 `false`，
   与冒烟用例 4.3「查看任务转已读」预期不符。
5. **详情接口缺失**：`GET /web/upgrades/firmwares/{id}`、`GET /web/upgrades/models/{id}`
   返回 404（列表/创建/删除均正常）。
6. **登录失败无错误提示**：`POST /web/auth/login` 返回 401 时，`request.ts` 响应拦截器
   直接 `window.location.href` 回跳登录页并清除 token，但**不弹 message** →
   页面只表现为「闪一下」，冒烟用例 E2 期望的「错误提示」缺失。
7. `CLAUDE.md` 中列出的 `e2e-getui-modal` / `e2e-sms-config-modal` 在页面源码中不存在
   （个推 / 短信配置为整页表单，未使用 Modal）。
8. `MenuTree` 在 `expandedRowRender` 内嵌套 `<Table>`，rc-table 会输出
   `Warning: expandedRowRender should not use with nested Table` 与
   `Warning: expandIconColumnIndex is deprecated`（控制台告警，非运行时异常）。
9. 后端**没有 CORS 中间件**，测试必须依赖 Vite 代理同源访问（见上文 `VITE_API_BASE`）。
