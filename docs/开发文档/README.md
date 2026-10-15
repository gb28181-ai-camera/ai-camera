# 编码实现文档

> 版本: v0.1.0
> 最后更新: 2026-09-21

本目录是 AI Camera（v0.1.0）的**编码实现层**文档：向下冻结数据库结构与后端接口契约，向上约束
Web 管理后台、官网与 Flutter 移动端的编码方式，并逐页给出页面功能与接口的对照关系。

## 目录结构

```
开发文档/
├── README.md                        # 本文件：索引 + 接口代码生成流程
├── 后端开发/
│   └── 00-接口契约.md                # Repository/Service/Handler 三层签名 + 全部路由与响应类型
├── 前端Web/
│   ├── 01-前端编码规范.md            # 命名/TS/React/Zustand/API/页面模式/组件/样式/双入口/测试/E2E/Git/性能
│   └── 02-Zustand状态管理规范.md      # authStore 全景：为什么、定义规范、持久化、权限体系、反模式、测试
├── 前端APP/
│   └── README.md                    # Flutter 移动端：技术栈/目录/状态管理/路由/网络层/生成客户端/主题/页面模式/测试/构建
├── 技术选型/
│   ├── 00-总体技术选型.md            # 全栈技术决策总览与取舍
│   ├── 01-前端技术选型.md            # React 19 + Vite + antd + Electron 多入口方案
│   └── 02-后端技术选型.md            # Go + Gin + GORM + PostgreSQL + Casbin
├── 数据库设计/
│   ├── 01-数据字典与枚举值.md         # 全部表字段、类型、枚举取值
│   ├── 02-表关系全景图.md            # 实体关系与依赖方向
│   └── sql/
│       ├── ddl/                     # 建表脚本，按域拆分 8 个文件
│       └── init/                    # 初始数据（菜单、权限、字典、内置账号）
└── 页面功能和接口/
    ├── 00-页面功能与接口总览.md       # 阅读指南 + 页面/接口总索引
    └── 01~12-*.md                   # 逐页说明：功能项、交互、按钮行为、接口清单与出入参
```

## 各目录说明

| 目录 | 内容 | 面向角色 |
|:-----|:-----|:---------|
| `后端开发/` | 分层方法签名、路由清单、响应类型命名规则、通用约定（分页参数、错误码） | 后端开发、联调方 |
| `前端Web/` | 管理后台与官网的编码规范、状态管理规范、组件清单、测试与 E2E 规则 | Web 前端开发、测试 |
| `前端APP/` | Flutter 移动端的技术栈、目录约定、状态/路由/网络层实现细节与测试基座 | 移动端开发、测试 |
| `技术选型/` | 选型结论与理由、版本清单、备选方案对比 | 架构、技术负责人 |
| `数据库设计/` | 数据字典、枚举取值、表关系、可直接执行的 DDL 与初始数据 | 后端开发、DBA、测试 |
| `页面功能和接口/` | 逐页的功能项、搜索条件、按钮行为、状态流转、接口清单与出入参示例 | 产品、前后端开发、测试 |

> **注意**：接口的**唯一权威来源**是 [后端开发/00-接口契约.md](./后端开发/00-接口契约.md)。
> 「页面功能和接口」不改写、不新增、不省略任何路由；如两者冲突，以契约文档为准并同步修正。

## 对应源码目录

| 开发目录 | 对应源码路径 | 技术栈 |
|:---------|:-------------|:-------|
| `后端开发/` | `backend/` | Go + Gin + GORM + PostgreSQL + Casbin |
| `前端Web/` | `frontend/src/admin/`（管理后台）<br>`frontend/src/website/`（官网） | React 19 + Vite + TypeScript + Ant Design 5 + Zustand |
| `前端APP/` | `app/` | Flutter（Dart SDK `^3.9.0`）+ go_router + flutter_dva + dio |
| `技术选型/` | `backend/`、`frontend/`、`app/` | — |
| `数据库设计/` | `backend/internal/model/`（GORM 模型）+ `数据库设计/sql/ddl/`（DDL） | PostgreSQL |
| `页面功能和接口/` | `frontend/src/admin/pages/`、`backend/internal/api/handler/` | — |

## 接口代码生成

### 工作流程

```
  后端 handler 的 Swaggo 注解（@Summary / @Router / @Success 200 {object} response.RPageUser）
      │
      ▼
  cd backend && bash scripts/gen-swagger.sh
      │    swag init --generalInfo ../../router/web.go --output docs/web --instanceName web
      │    swag init --generalInfo ../../router/app.go --output docs/app  --instanceName app
      ▼
  Swagger JSON：backend/docs/web/web_swagger.json ｜ backend/docs/app/app_swagger.json
      │
      │    后端运行在 :8080 时对外暴露
      │      Web 端：GET /web/api-docs.json
      │      App 端：GET /app/api-docs.json
      ▼
  ┌───────────────────────────────┴───────────────────────────────┐
  ▼                                                               ▼
【Web (TypeScript)】                                        【APP (Dart)】
  cd frontend && bash scripts/api-gen.sh                      无代码生成器
    swagger2-doc-parser fetch  /web/api-docs.json             lib/services/generated/ 按生成的
    swagger2-doc-parser gen -c scripts/                          Controller 风格**手工对齐**
      swagger-config-typescript.js -o src/admin/api/generated    后端接口契约
  ▼                                                             ▼
  src/admin/api/generated/*Controller.ts                      app/lib/services/generated/
  src/admin/api/generated/definitions/*.ts                    ├── AuthController.dart
  （⚠️ 生成物，严禁手动编辑）                                   ├── AppController.dart
                                                              └── definitions/*.dart
```

### 使用方法

```bash
# ── ① 生成 Swagger JSON（执行目录：backend/）──
cd backend && bash scripts/gen-swagger.sh
# 产出 backend/docs/web/web_swagger.json 与 backend/docs/app/app_swagger.json

# ── ② 启动后端（api-gen 需要访问运行中的 /web/api-docs.json）──
cd backend && go run ./cmd/server/main.go

# ── ③ 生成前端 TypeScript 客户端（执行目录：frontend/）──
cd frontend && bash scripts/api-gen.sh
# 或：npm run api:gen
# 可覆盖后端地址：BACKEND_URL=http://192.168.1.227:8080 bash scripts/api-gen.sh
# 产出 src/admin/api/generated/{*Controller.ts, definitions/*.ts}

# ── ④ 类型校验 ──
cd frontend && npm run typecheck

# ── ⑤ APP 端：手工同步（无生成器）──
# 后端契约变更后，人工核对 app/lib/services/generated/ 下的
# AuthController.dart / AppController.dart / definitions/*.dart
cd app && flutter analyze      # 要求 0 error
```

> **注意**：`frontend/scripts/api-gen.sh` 仅生成 **Web (TypeScript)**。`app/lib/services/generated/`
> 是 Dart 侧按生成的 Controller 风格**手工对齐契约**的代码（字段名与后端 JSON tag 严格一一对应），
> 后端契约变更后必须人工同步核对。

### 生成的代码使用示例

**Web (TypeScript)** —— 走 `src/admin/api/generated/` 的 Controller 静态方法，禁止手写 `request()`：

```typescript
// src/admin/pages/system/ConfigList.tsx（片段）
import { ConfigController } from '../../api/generated/ConfigController'
import type { SystemConfig } from '../../api/generated/definitions/SystemConfig'

// 无分页列表：res.data 直接是数组
const res = await ConfigController.listConfigs()
if (res.code === 200) {
  setData(res.data ?? [])
}

// 分页列表：res.data.items / res.data.total
const pageRes = await UserController.pageUsers(page, pageSize, keyword || undefined, deptId, activeFilter)
if (pageRes.code === 200) {
  setData(pageRes.data?.items ?? [])
  setTotal(pageRes.data?.total ?? 0)
}

// 写操作：创建兼容 201，更新判 200
const saveRes = await ConfigController.updateConfig(String(editing.id), values)
```

**APP (Dart)** —— 走 `lib/services/generated/` 的 Controller 静态方法，返回强类型响应：

```dart
// app/lib/pages/risk/risk_list_page.dart（片段）
import '../../services/generated/AppController.dart';
import '../../services/generated/definitions/Response.dart';

// 分页查询：resp.items / resp.total / resp.page.hasMore
final RPageRiskRecord resp = await AppController.risksUsingGet(
  query: <String, dynamic>{
    'page': page,
    'page_size': pageSize,
    if (_status.isNotEmpty) 'status': _status,
  },
);

// 写操作：resp.ok 等价于 code == 200
final REmptyResponse handleResp = await AppController.handleRiskUsingPut(
  id,
  HandleReq(handleResult: result, handleImage: imageUrl),
);
if (!handleResp.ok) {
  // handleResp.errorMessage 即后端 message
}
```

## 文档索引

| 文件 | 内容 |
|:-----|:-----|
| [后端开发/00-接口契约.md](./后端开发/00-接口契约.md) | 契约用途与通用约定、Repository/Service/Handler 三层类型与方法签名、Web 端与 App 端全部路由及响应类型 |
| [前端Web/01-前端编码规范.md](./前端Web/01-前端编码规范.md) | 13 节：命名规范、TypeScript 规范、React 组件规范、Zustand、API 请求规范、页面开发模式、公共组件清单、样式规范、双入口工程约定、单元测试、E2E 测试、Git 提交、性能优化 |
| [前端Web/02-Zustand状态管理规范.md](./前端Web/02-Zustand状态管理规范.md) | 7 节：为什么用 Zustand、Store 定义规范（authStore 全文逐段说明）、持久化策略、权限标识体系、组件消费模式、常见反模式、测试 |
| [前端APP/README.md](./前端APP/README.md) | 11 节：技术栈、目录结构、状态管理（flutter_dva）、路由与登录态、网络层、生成的 API 客户端、主题与设计令牌、页面开发模式、测试规范、构建与运行、开发注意事项 |
| [技术选型/00-总体技术选型.md](./技术选型/00-总体技术选型.md) | 全栈技术决策总览、各层选型理由与取舍 |
| [技术选型/01-前端技术选型.md](./技术选型/01-前端技术选型.md) | 前端技术栈与版本、单工程双入口结构、核心依赖、API 请求封装、双平台（Web/Electron）构建 |
| [技术选型/02-后端技术选型.md](./技术选型/02-后端技术选型.md) | 后端技术栈、分层与目录、中间件、鉴权体系、数据权限 |
| [数据库设计/01-数据字典与枚举值.md](./数据库设计/01-数据字典与枚举值.md) | 全部表的字段清单、类型、约束与枚举取值说明 |
| [数据库设计/02-表关系全景图.md](./数据库设计/02-表关系全景图.md) | 实体关系全景、外键依赖方向与级联规则 |
| [数据库设计/sql/ddl/](./数据库设计/sql/ddl/) | 建表脚本，8 个文件：`01-系统基础`、`02-基础信息`、`03-资源管理`、`04-风险与告警`、`05-巡检`、`06-升级管理`、`07-告警设置`、`08-权限策略表` |
| [数据库设计/sql/init/01-初始数据.sql](./数据库设计/sql/init/01-初始数据.sql) | 初始数据：内置账号、菜单树、权限标识、字典与示例业务数据 |
| [页面功能和接口/00-页面功能与接口总览.md](./页面功能和接口/00-页面功能与接口总览.md) | 文档目的、阅读指南、页面与接口总索引 |
| [页面功能和接口/01-登录与个人中心页面与接口.md](./页面功能和接口/01-登录与个人中心页面与接口.md) | 登录、个人中心、API 密钥 |
| [页面功能和接口/02-系统管理页面与接口.md](./页面功能和接口/02-系统管理页面与接口.md) | 用户、菜单、角色、部门、岗位、操作日志、系统通知、系统参数 |
| [页面功能和接口/03-基础信息页面与接口.md](./页面功能和接口/03-基础信息页面与接口.md) | 区域、风险类型、告警类型 |
| [页面功能和接口/04-资源管理页面与接口.md](./页面功能和接口/04-资源管理页面与接口.md) | 设备、语音配置、物联网卡、物联网卡更换记录、设备模式 |
| [页面功能和接口/05-首页看板页面与接口.md](./页面功能和接口/05-首页看板页面与接口.md) | 首页今日/本周/本月预警与处理统计、图表 |
| [页面功能和接口/06-实时监控页面与接口.md](./页面功能和接口/06-实时监控页面与接口.md) | 实时监控（单屏 / 4 宫格 / 9 宫格） |
| [页面功能和接口/07-AI风险与AI监控页面与接口.md](./页面功能和接口/07-AI风险与AI监控页面与接口.md) | AI 风险列表与处理、AI 监控分屏联动 |
| [页面功能和接口/08-风险统计页面与接口.md](./页面功能和接口/08-风险统计页面与接口.md) | 风险监控、设备监控、GIS 查看、设备历史记录、设备告警、物联网卡监控 |
| [页面功能和接口/09-巡检中心页面与接口.md](./页面功能和接口/09-巡检中心页面与接口.md) | 巡检任务管理、巡检记录 |
| [页面功能和接口/10-统计分析页面与接口.md](./页面功能和接口/10-统计分析页面与接口.md) | 风险统计分析、设备告警分析、可视化大屏 |
| [页面功能和接口/11-升级管理与告警设置页面与接口.md](./页面功能和接口/11-升级管理与告警设置页面与接口.md) | 升级设备、升级计划、升级记录、固件、模型、固件类型、设备告警设置、风险通知设置、第三方平台 |
| [页面功能和接口/12-APP移动端页面与接口.md](./页面功能和接口/12-APP移动端页面与接口.md) | 移动端全部页面（`/app/*`） |
