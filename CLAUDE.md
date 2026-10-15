# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 提供项目开发指引。

## 项目概述

**AI Camera — AI 智能监控平台**。以 AI 摄像头为采集终端，提供实时视频监控、AI 风险识别与处理、设备告警、巡检中心、统计分析（含可视化大屏）、设备/固件/模型远程升级、物联网卡管理以及完整的系统权限体系。

对标产品：AI摄像头操作手册（见 [docs/需求文档/](docs/需求文档/)），本仓库按该手册的功能结构实现。

当前版本 **v0.1.0（基础功能开发）**，未进入敏捷开发。开发必须严格按照 `docs/` 中的设计文档进行。设计总览参考 [docs/00-系统设计总览.md](docs/00-系统设计总览.md)，进度参考 [docs/敏捷开发/CHANGE_LOG.md](docs/敏捷开发/CHANGE_LOG.md)。

## 仓库布局

```
ai-camera/
├── backend/          # Go API 服务 (Gin + GORM + PostgreSQL + Casbin)
├── frontend/         # React 19 + Vite + Ant Design + Electron
│   └── src/
│       ├── admin/    # 管理后台（控制台）
│       ├── website/  # 前台介绍页（官网）
│       └── shared/   # 两个入口共享的组件与工具
├── app/              # Flutter 移动端
└── docs/             # 系统设计文档（需求、数据库设计、编码规范）
```

> **注意**：前端管理后台与官网介绍页同属 `frontend/` 一个 npm 工程，通过多入口构建：
> 管理后台入口 `index.html`（base `/admin/`），官网入口 `website.html`（base `/`）。

## 数据库 MCP

本项目的 PostgreSQL MCP Server：

| MCP Server | 数据库名称 | 用途 |
|---|---|---|
| `postgres_ai_camera_dev` | `ai_camera_dev` | 开发环境数据库 |
| `postgres_ai_camera_test` | `ai_camera_test` | 测试环境数据库（冒烟测试用） |

MCP 配置文件位于：
`C:\Users\Administrator\AppData\Roaming\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json`

## 常用命令

### 后端 (Go)

```bash
# 启动开发服务
cd backend && go run ./cmd/server/main.go

# 启动测试服务
export CONFIG_PATH="config/config-test.yaml" && cd backend && go run ./cmd/server/main.go

# 编译
cd backend && go build -o server ./cmd/server/main.go

# 生成 Swagger 文档
cd backend && bash scripts/gen-swagger.sh

# 运行指定测试
cd backend && go test ./tests/system/ -run TestName -v

# 运行全部测试
cd backend && go test ./tests/... -v

# 添加依赖 / 整理
cd backend && go get github.com/xxx/xxx && go mod tidy
```

### 前端 (React/Electron)

```bash
# 安装依赖
cd frontend && npm install

# 管理后台开发模式（Vite 代理 /web/*、/app/* 到 localhost:8080）
cd frontend && npm run dev

# 官网介绍页开发模式
cd frontend && npm run dev:website

# Electron 开发模式
cd frontend && npm run dev:desktop

# 构建（同时产出管理后台与官网）
cd frontend && npm run build

# 从 Swagger 生成 API 客户端代码（需要后端运行）
cd frontend && bash scripts/api-gen.sh

# 端到端测试 (Playwright)
cd frontend && npx playwright test

# 单元测试 (Vitest)
cd frontend && npx vitest
```

### Flutter 移动端

```bash
cd app && flutter run        # 在连接的设备上运行
cd app && flutter test       # 运行测试
cd app && flutter analyze    # 静态检查（要求 0 error 0 warning）
```

## 架构

### 后端：分层架构 Handler → Service → Repository → Model

```
cmd/server/main.go          # 入口：路由注册 → 启动
internal/bootstrap/         # init() 加载配置、数据库、JWT、Casbin，暴露全局变量
internal/api/router/        # web.go（管理后台 + 官网）、app.go（移动端）
internal/api/handler/web/   # HTTP 处理器，按领域分组（system_handler、device_handler 等）
internal/api/handler/app/   # 移动端专用处理器
internal/api/middleware/    # 中间件：Auth (JWT Bearer)、ApiKeyAuth、NormalizeEmptyQuery、OperationLog
internal/service/           # 业务逻辑，按领域分组（system/、device/、risk/ 等）
internal/repository/        # 数据访问层（user_repo、device_repo 等）
internal/model/             # GORM 模型，按领域分组（system.go、device.go 等）
internal/dto/request/       # 请求结构体
pkg/                        # 跨模块工具：auth/jwt、database/postgres、perm/casbin、utils
config/                     # Viper 配置加载 + config.yaml（gitignored）
```

- **Bootstrap 自动初始化**：`internal/bootstrap/init()` 在 import 时运行，加载 `config/config.yaml`，连接数据库，自动迁移所有模型，初始化 JWT 和 Casbin。测试工具以事务作用域的数据库覆盖全局变量。
- **BaseModel**（`internal/model/base.go`）：所有模型嵌入此结构体，提供 `ID`、`DeletedAt`（软删除）、`CreatedBy/DT`、`UpdatedBy/DT`。
- **认证流程**：POST `/web/auth/login` → JWT token → Auth 中间件提取 claims 到 `gin.Context` → Casbin 中间件执行 RBAC。
- **数据权限**：JWT 携带 `data_perm` / `company_id`，经 `pkg/utils/data_scope.go` 的 `NewDataScope(c).ApplyToQuery(q, "alias.company_id")` 施加行级过滤。
- **响应格式**：`pkg/utils/response.go` — 统一 `{code, message, data}` 格式，提供 `Success`、`BadRequest`、`Unauthorized` 等辅助函数。
- **空查询参数**：全局中间件 `middleware.NormalizeEmptyQuery()` 在绑定前剔除值为空串的查询参数。
  前端生成的 Controller 对未填写的筛选项会发 `?keyword=&active=`，而 `active=` 绑定到 `*bool` 会得到
  「非 nil 的 false」，导致 `WHERE active = false` —— 列表恒为 0 条。此中间件一处修复覆盖全部接口，
  且对任何客户端都成立；`?active=false` 这类显式传值不受影响。

#### 鉴权体系（核心设计）

与 boticz-mes 保持一致的鉴权方案，四层叠加：

| 层 | 机制 | 位置 |
|---|---|---|
| 1 | JWT HS256，claims 携带 user_id/user_name/data_perm/company_id | `pkg/auth/jwt.go` |
| 2 | Auth 中间件：校验 Bearer token，写入 gin.Context + Go Context | `internal/api/middleware/auth.go` |
| 3 | Casbin RBAC：`g`（用户→角色）、`g2`（部门→角色）、`g3`（岗位→角色）三种继承 | `pkg/perm/casbin.go` + `config/rbac_model.conf` |
| 4 | 数据权限：按 data_perm 施加 company_id / created_by 行级过滤 | `pkg/utils/data_scope.go` |

- Casbin 的 `p` 策略由「角色 → 菜单 → 菜单API」三级推导，写入 `casbin_rule` 表。
- 角色授权变更后需调用 `syncCasbinForRole(roleID)` 重建策略并 `ReloadPolicy()`。
- 菜单树同时驱动前端侧边栏与按钮权限（`permission_code`）。

#### 后端开发规范

- **分页响应类型**：必须使用 `response.PageXxxResp` 类型化分页响应，**禁止**使用 `response.PageResp` 模糊类型。不符合此规则的需要新增类型定义。
- **Swagger 注释**：修改返回类型时必须同步更新 Swagger 注释。`@ID` 为方法名首字母小写，分页接口命名为 `pageXxxx`。`@Success 200` 需认真核对返回结果。
- **关联表查询**：列表涉及多张表关联时，必须展示关联表的**名称字段**，不能只展示 ID 字段。
- **设备展示字段**：设备编号 `device.device_code`、设备名称 `device.name`、所属区域 `region.name`。

### 前端：API 代码生成流程

```
后端 Swagger 注释（handler 文件）
  → cd backend && bash scripts/gen-swagger.sh   # swag init → docs/web/web_swagger.json
  → 后端运行在 :8080（测试环境 :8081）
  → cd frontend && bash scripts/api-gen.sh      # 抓取 swagger + 生成
  → src/admin/api/generated/{Xxx}Controller.ts  # 自动生成，严禁手动编辑
  → src/admin/api/generated/definitions/*.ts    # 自动生成，严禁手动编辑
```

**生成器有两套，脚本自动选择**：

| 优先级 | 生成器 | 说明 |
|:---:|:---|:---|
| 1 | `swagger2-doc-parser`（外部 CLI） | 在 `PATH` 中时使用，与 boticz-mes 同款工具 |
| 2 | `frontend/scripts/api-gen-local.mjs` | **本仓库自带的零依赖 Node 生成器**，CLI 缺失时自动回退 |

> 本机通常**没有装 `swagger2-doc-parser`**，因此实际走的是自带的 `api-gen-local.mjs` ——
> 链路完整可跑、可复现，不再依赖外部 CLI。常用参数见 `frontend/scripts/README.md`。
>
> 生成器支持离线模式：`SWAGGER_FILE=../backend/docs/web/web_swagger.json bash scripts/api-gen.sh`，
> 无需后端在运行（但该 JSON 需先由 `gen-swagger.sh` 产出，且 `backend/docs/` 已 gitignore）。

**手写逻辑的保留**：个别方法有生成器表达不了的兼容别名（如 `MainLayout` 依赖的 `getCurrentUserMenus()`），
登记在 `frontend/scripts/api-gen-preserve.mjs` 的 preserve 清单里，生成时自动追加并做失效/重名告警。

- **生成的 Controller** 使用 `src/admin/api/request.ts`（Axios + JWT 拦截器）
- **状态管理**：Zustand store（`src/admin/store/authStore.ts`）
- **多入口**：`vite.config.ts`（Web，含 admin + website 两个入口）、`vite.renderer.config.ts`（Electron 渲染进程）

#### 前端开发规范

- **API 调用必须走生成的 Controller**：所有 API 调用必须通过 `frontend/src/admin/api/generated/` 中的 Controller 静态方法（如 `DeviceController.pageDevices()`）。
- **严禁手动编写 API 请求代码**：禁止 `import request from '../../api/request'` 后手动拼接 URL 调用 API（如 `request('/web/xxx', { method: 'PUT' })`）。
- **API 变更流程**：后端新增/修改 API 端点后 → 重新生成前端代码（`cd frontend && bash scripts/api-gen.sh`，需要后端运行）→ 更新对应 UI 代码。
- **严格使用生成的响应类型**：前端代码**禁止**使用 `as any`、`data as any` 等强制类型转换来访问后端返回数据。必须通过 Swagger 生成的控制器返回类型（如 `RRegionList`、`RPageUser`）来严格约束数据访问。如果类型不匹配，应当修正后端 Swagger 注解并重新生成前端类型，而非在前端绕过类型检查。

#### 按钮权限码规范（易踩坑）

前端用 `permCodes.includes('<权限码>')` 控制按钮显隐，而 `permCodes` 来自
`GET /web/profile/menus` 返回的菜单树中每个节点的 `permission_code` 字段。因此：

1. **权限码必须真实存在于 `ac_menu` 表**，且该菜单已通过 `ac_role_menu_rel` 授予当前用户的角色。
   只在前端写 `permCodes.includes('xxx')` 而库里没有对应行 → **按钮对超管也不渲染**（静默失效，无报错）。
2. 权限码命名统一为 **`<模块>:<实体>:<动作>`**，如 `basic:region:create`、`upgrade:firmware:update`、
   `setting:alarm:disable-all`。按钮型菜单行使用 `type='button'`、`path=''`、`component=''`。
3. **新增按钮权限时必须同时**：① 在 `docs/开发文档/数据库设计/sql/init/01-初始数据.sql` 追加 `type='button'` 的
   `ac_menu` 行；② 授予相应角色（`ac_role_menu_rel`）；③ 前端再使用该权限码。
4. 排查方法：`/web/profile/menus` 返回的 `permission_code` 集合应覆盖前端 `grep -rhoE "permCodes\.includes\('[^']+'\)" src/` 的全部取值。

#### E2E 测试规范

- **Modal 必须使用 e2e class 定位**：端到端测试脚本中**禁止**使用通用 `.ant-modal` 选择器（如 `.ant-modal`、`.ant-modal .ant-modal-footer`、`page.locator('.ant-modal').last()`）。必须使用页面上已定义的 `e2e-xxx-modal` CSS class 来精确定位 Modal。
  - 理由：当存在嵌套弹窗（如处理风险 Modal 内打开图片预览）时，通用 `.ant-modal` 会匹配到错误的弹窗，导致测试不稳定。
  - 正确示例：`page.locator('.e2e-region-modal')`、`page.locator('.e2e-device-select')`
  - 错误示例：`page.locator('.ant-modal').last()`、`page.locator('.ant-modal-footer button.ant-btn-primary')`
- **新增 Modal 必须添加 e2e class**：任何新建的 Ant Design `<Modal>` 组件，必须添加 `className="e2e-xxx-modal"` 属性，用于 E2E 测试定位。
- **SelectorModal 已支持 className 传递**：`<SelectorModal className="e2e-xxx">` 会将 class 应用到 Modal（`.e2e-xxx`）和触发按钮（`.e2e-xxx-btn`），测试中可分别使用。
- **现有 e2e class 清单**（以 `frontend/src/admin/` 源码实测为准，共 49 个 + 新增 1 个）：
  - 基础信息：`e2e-region-modal`、`e2e-risk-type-modal`、`e2e-alarm-type-modal`
  - 资源管理：`e2e-device-modal`、`e2e-device-mode-modal`、`e2e-device-mode-switch-modal`、`e2e-iot-card-modal`、`e2e-iot-card-replace-modal`、`e2e-iot-card-replace-device-btn`、`e2e-voice-config-modal`、`e2e-voice-preview-modal`
  - 设备选择器（`DeviceSelect` / `SelectorModal`）：`e2e-device-select`（下拉框）、`e2e-device-select-modal`（弹窗）、`e2e-device-select-btn`（触发按钮）
  - 风险/告警：`e2e-risk-handle-modal`、`e2e-risk-detail-modal`、`e2e-risk-image-modal`、`e2e-alarm-handle-modal`、`e2e-alarm-detail-modal`、`e2e-video-modal`
  - 设备监控：`e2e-device-chart-modal`、`e2e-device-week-modal`
  - 巡检：`e2e-inspect-task-modal`、`e2e-inspect-record-modal`
  - 升级：`e2e-firmware-modal`、`e2e-firmware-type-modal`、`e2e-model-modal`、`e2e-device-upgrade-modal`
  - 告警设置：`e2e-alarm-setting-modal`、`e2e-risk-notify-modal`、`e2e-webhook-modal`（消息推送配置）
  - 系统：`e2e-config-modal`、`e2e-user-modal`、`e2e-user-resetpwd-modal`、`e2e-role-modal`、`e2e-role-menu-modal`、`e2e-role-dept-modal`、`e2e-role-post-modal`、`e2e-role-user-modal`、`e2e-role-dataperm-modal`、`e2e-menu-modal`、`e2e-menu-api-modal`、`e2e-menu-api-table`、`e2e-dept-modal`、`e2e-org-modal`（与 `e2e-dept-modal` 同挂一个 Modal 的别名）、`e2e-dept-tree`、`e2e-post-modal`、`e2e-operation-log-modal`、`e2e-notification-modal`
  - 个人中心：`e2e-profile-apikey-modal`

> **说明**：上表由 `grep -rhoE "e2e-[a-z0-9-]+" frontend/src/admin/ | sort -u` 实测得出。
> 其中 `e2e-dept-tree`、`e2e-menu-api-table` 为表格/树容器类，`e2e-device-select` 为下拉框类，
> `e2e-device-select-btn`、`e2e-iot-card-replace-device-btn` 为触发按钮类，非 Modal 本体的 class。
> 「第三方平台」下的个推（`GetuiConfig`）与短信（`SmsConfig`）页面为**单卡片表单，不含 Modal**，
> 因此**不存在** `e2e-getui-modal` / `e2e-sms-config-modal`。

#### 响应类型规范

后端响应类型定义在 `backend/internal/dto/response/`，分为三类：

| 类型 | 结构 | 用途 |
|---|---|---|
| `RXxxList` | `Data []model.Xxx` | 简单列表（无分页），如区域、风险类型、设备模式 |
| `RPageXxx` | `Data PageXxxResp{Items, Total, Page, PageSize}` | 分页列表，如用户、风险监控、设备 |
| `RXxx` | `Data model.Xxx` | 单条详情 |

**规则**：
- 列表接口的 Swagger 注解和实际返回必须一致：简单列表用 `RXxxList`，分页列表用 `RPageXxx`
- 前端简单列表用 `res.data ?? []` 读取，分页列表用 `res.data?.items ?? []` 读取
- 严禁一个端点在不同条件下返回不同格式（如有时分页有时不分页）— 统一为一种格式
- 修改后端返回类型时，同步更新 Swagger 注解 → 重新生成 Swagger 文档 → 重新生成前端类型 → 修改前端代码

## 测试

如果修改了前后端功能，对应的单元测试、集成测试、端到端测试也需要同步调整。

**重要**：
- 测试时必须使用测试数据库 MCP `postgres_ai_camera_test`，**禁止**操作开发库 `postgres_ai_camera_dev`。
- 启动测试前，必须停止本地正在运行的后端服务，避免端口冲突。

### 后端单元测试

测试位于 `backend/tests/<domain>/`，使用 `testutil.WithTx(t, fn)` — 每个测试在数据库事务中运行，结束后自动回滚，确保测试隔离。

**测试前准备**：
```bash
# 1. 清空测试数据库（必须，避免与 seed 数据冲突）
cd backend && go run ./tests/testutil/resetdb/main.go
```

**执行测试**（必须设置环境变量指向测试数据库）：
```bash
cd backend && TEST_DB_HOST=192.168.1.227 TEST_DB_PORT=5432 TEST_DB_USER=boticz TEST_DB_PASSWORD='<测试库密码>' TEST_DB_NAME=ai_camera_test go test ./tests/... -count=1 -p 1 -timeout 600s
```

> **`<测试库密码>` 从何而来**：测试库密码不再写入版本库。请向团队管理员索取，
> 或从本地 `backend/config/config-test.yaml` 的 `database.password` 读取（该文件已被 gitignore）。
> 建议先导出一次，后续命令即可省略：
> ```bash
> export TEST_DB_PASSWORD='<向管理员索取>'
> ```
> `resetdb` / `initdb` / `dbtool` 已改为**不提供密码默认值**，未设置环境变量时会直接报错退出，
> 避免明文凭据再次进入版本库。

> **必须加 `-p 1`（包间串行）**。`go test ./...` 默认**并行执行多个测试包**，而本项目所有包共用同一个
> 物理库 `ai_camera_test`。`WithTx` 只保证**单进程内**的事务隔离（它覆盖的是该进程的全局 DB 句柄），
> 跨进程的并发写入仍会相互干扰：A 包按数量统计断言时，B 包恰好插入了同类型数据就会让断言失败。
> 实测：不加 `-p 1` 时 `tests/upgrade` 有 2 个用例随机失败，加 `-p 1` 后全绿。

运行指定模块：
```bash
cd backend && TEST_DB_HOST=192.168.1.227 TEST_DB_PORT=5432 TEST_DB_USER=boticz TEST_DB_PASSWORD='<测试库密码>' TEST_DB_NAME=ai_camera_test go test ./tests/system/... -v
```

**字段验证**：所有后端单元测试必须验证绝大多数字段的更新，防止字段保存失效。

### 前端单元测试 (Vitest)

前端测试使用 **MSW (Mock Service Worker)** 模拟 API 请求，**无需启动后端服务**。

- 测试配置：`frontend/vitest.config.ts` — `VITE_API_BASE=http://localhost:8080`
- MSW 处理器：`frontend/src/test/handlers.ts` — 定义所有模拟 API 端点及返回数据
- 测试设置：`frontend/src/test/setup.ts` — 启动 MSW Server + jsdom polyfill（matchMedia）
- 新增 API 时必须同步更新 `handlers.ts` 添加对应的 mock 端点
- 响应格式使用 `httpResponse.success(data)` / `httpResponse.page(items, total)` / `httpResponse.error(msg)`

**执行测试**：
```bash
cd frontend && npx vitest run
```

### 端到端测试 (Playwright)

端到端测试代码位于 `frontend/e2e/`，截图保存在 `frontend/e2e/screenshots/` 目录下。

**重要**：测试脚本必须使用 `e2e-xxx-modal` 类选择器定位 Modal，禁止使用通用 `.ant-modal`。详见上方「前端开发规范 → E2E 测试规范」。

```bash
cd frontend && npx playwright test
```

### 冒烟测试

冒烟测试在测试环境执行：

- 测试数据库 MCP：`postgres_ai_camera_test`
- 后端测试配置：`config-test.yaml`

**执行顺序规范**：

- 冒烟测试（`frontend/e2e/smoke/`）串行执行（`--workers=1`），spec 文件按文件名字母序运行
- **必须先执行所有功能测试（`01a` ~ `10f`），最后执行 `99-all-list-page.spec.ts`（全页面覆盖验证）** — `99-` 前缀保证其排在最后
- 新增冒烟测试文件编号必须小于 `99`；全页面覆盖类收尾测试保留 `99-` 前缀

**流程**：

1. 清空测试数据库：`cd backend && go run ./tests/testutil/resetdb/main.go`
2. 初始化测试数据：`cd backend && go run ./tests/testutil/initdb/main.go`
3. 启动测试服务：
   ```bash
   export CONFIG_PATH="config/config-test.yaml" && cd backend && go run ./cmd/server/main.go
   ```
4. 执行测试
5. 测试完成后关闭前后端服务

> ⚠️ **重新灌库后必须重启后端**。Casbin 策略在进程启动时一次性加载进内存，
> `resetdb + initdb` 会重建 `casbin_rule`，但**已在运行的后端仍持有旧策略**，
> 表现为所有写接口返回 403「权限不足」。顺序务必是「先清库灌种子，再启动/重启后端」。

> ⚠️ **`initdb` 会自动对齐自增序列**（`syncSequences`）。初始化脚本用显式 ID 写入种子，
> 若不 setval，序列会落后于 `max(id)`，后续任何 INSERT 都会撞主键。

## 版本发布

每次版本更新时，需要补充以下文档：

- [docs/敏捷开发/CHANGE_LOG.md](docs/敏捷开发/CHANGE_LOG.md)
- [docs/敏捷开发/v<x.x.x> <版本描述>/xxx.md](docs/敏捷开发/)
- [backend/README.md](backend/README.md)
- [frontend/README.md](frontend/README.md)
- [app/README.md](app/README.md)
