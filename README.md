# AI Camera 智能监控平台

以 AI 摄像头为采集终端的现场安全与合规监管平台：设备端完成风险识别并上报，平台侧完成风险汇聚、人工复核与处理闭环，同时监控设备自身的在线状态、电量、存储与物联网卡流量。

> 当前版本: **v0.5.1** | 最后更新: 2026-10-25

---

## 产品简介

AI Camera 把「设备看得见」转变为「风险管得住」，核心能力有四条：

| 能力 | 说明 |
|:---|:---|
| **风险闭环** | AI 识别 → 人工确认检测准确性 → 填写处理结果与处理图片 → 归档，全链路留痕 |
| **设备可运维** | 在线状态、电量、电压、存储、流量、固件与模型版本统一管理，支持远程升级 |
| **责任人可追溯** | 风险、告警、巡检均落到具体人员，消息通知带未读红点 |
| **现场可巡检** | 除 AI 自动识别外，补充人工巡检任务的派发与结果回传 |

在视频接入侧，平台完整支持 **GB/T 28181**（注册/目录/云台/点播/回放/下载/云端录像/
录像计划/报警/移动位置/语音/代理推流/级联），并在 v0.4.0 对齐 WVP-GB28181-Pro 的
设备配置与控制、媒体指标、日志、分享播放、录像合并等运维能力；播放器提供
WebRTC / HTTP-FLV / Jessibuca / H265web / HLS 五种引擎。

平台提供 **PC 管理后台（Web）**、**官网介绍页**、**移动端 App** 与 **桌面端（Electron）**
四个入口，共享同一套后端服务与业务数据。功能结构对标《AI摄像头操作手册》，逐条落地为
需求文档、DDL、接口契约与代码。

---

## 系统架构

```
┌──────────────────┐        ┌──────────────────────┐
│   AI 摄像头       │        │      边缘识别         │
│   （现场设备）    │ ─────▶ │  · 未戴安全帽         │
│  · 视频采集       │        │  · 明火 / 烟雾        │
│  · 本地缓存       │        │  · 人员闯入 / 区域入侵 │
│  · 断网续传       │        │  · 置信度打分         │
└────────┬─────────┘        └──────────┬───────────┘
         │                             │
         │  心跳 / 风险上报 / 告警上报（X-Api-Key）
         │  ◀──────── 升级下发 / 时间同步 / 模式切换
         ▼                             ▼
┌──────────────────────────────────────────────────────────────────┐
│                    AI Camera 后端平台（Go + Gin）                 │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │  四级鉴权：JWT → Auth 中间件 → Casbin RBAC → 数据权限        │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ 资源管理  │ │ 风险监控  │ │ 设备告警  │ │ 巡检中心  │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │ 实时监控  │ │ 升级管理  │ │ 统计分析  │ │ 系统管理  │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
│                                                                  │
│  ┌────────────────────┐  ┌────────────────────────────────┐    │
│  │ PostgreSQL（62 表）│  │ 文件存储（图片/录像/音频/固件） │    │
│  └────────────────────┘  └────────────────────────────────┘    │
└────────────────────────────┬─────────────────────────────────────┘
                             │  380+ 个 API 端点
        ┌────────────────────┼────────────────────┬──────────────────┐
        ▼                    ▼                    ▼                  ▼
┌───────────────┐  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│ 管理后台 Web   │  │ 官网介绍页     │  │ 桌面端 Desktop │  │ 移动端 App     │
│ React 19      │  │ React 19      │  │ Electron 33   │  │ Flutter       │
│ 39 个页面      │  │ 9 个组件      │  │ 复用管理后台   │  │ 9 个页面       │
│ base /admin/  │  │ base /        │  │               │  │ /app/* 端点    │
└───────────────┘  └───────────────┘  └───────────────┘  └───────────────┘
```

---

## 仓库结构

```
ai-camera/
├── backend/          # Go API 服务（Gin + GORM + PostgreSQL + Casbin）
│   ├── cmd/server/   # 启动入口（路由装配 + 优雅退出）
│   ├── config/       # viper 配置加载 + rbac_model.conf
│   ├── internal/     # 核心代码：model / repository / service / api / dto / bootstrap
│   ├── pkg/          # 跨模块工具：auth / database / perm / utils
│   ├── scripts/      # gen-swagger.sh / build.sh / publish.sh
│   ├── static/       # Swagger UI 静态页
│   ├── tests/        # 后端测试 + testutil（WithTx / resetdb / initdb）
│   └── README.md     # 后端说明（含 API 路由总览）
├── frontend/         # React 19 + Vite 5 + Ant Design 5 + Electron 33
│   ├── index.html    # 管理后台入口（base /admin/）
│   ├── website.html  # 官网入口（base /）
│   ├── electron/     # Electron 主进程与预加载脚本
│   ├── scripts/      # api-gen.sh（Swagger → Controller 生成）
│   └── src/
│       ├── admin/    # 管理后台（页面 / 组件 / 自动生成的 API 客户端）
│       ├── website/  # 官网介绍页
│       └── shared/   # 两个入口共享的工具
├── app/              # Flutter 移动端（GoRouter + Dio）
│   ├── pubspec.yaml  # 依赖清单
│   └── lib/          # theme / layout / （页面与服务）
├── docs/             # 设计文档（需求 / 开发 / 测试 / 部署 / 敏捷）
├── .mcp.json         # PostgreSQL MCP Server 配置（dev / test）
├── CLAUDE.md         # Claude Code 开发指引
└── README.md         # 本文件
```

> **注意**：`frontend/` 是**单工程双入口** —— 管理后台（`index.html`，base `/admin/`）与
> 官网（`website.html`，base `/`）通过两次构建产出 `dist/` 与 `dist-website/`，
> 两者 `base` 不同，**产物不可互换部署**。

---

## 快速开始

### 一键部署（Docker Compose，开源版）

```bash
# 1. 准备环境变量（.env 已被 gitignore，勿提交）
cp .env.example .env
#    必须填写：POSTGRES_PASSWORD、JWT_SECRET（生成：openssl rand -base64 48）

# 2. 起基础设施（PostgreSQL 16 + PostGIS + pgvector，以及 Redis）
docker compose up -d

# 3. 起全部服务（额外启动 backend + frontend）
docker compose up -d --profile full

# 4. 访问
#    管理后台  http://localhost:8080/admin/
#    官网      http://localhost:8080/
#    接口文档  http://localhost:8080/swagger/
```

- 两个数据库 `ai_camera_dev`（开发）与 `ai_camera_test`（测试）在首次启动时自动创建，并各自装好 **PostGIS**（GIS 定位与移动位置轨迹）与 **pgvector**（向量检索）扩展。
- 详见 [deploy/postgres/](deploy/postgres/) 与 [docker-compose.yml](docker-compose.yml)。

### 后端（Go，本地开发）

```bash
# 1. 起本地数据库（本地调试使用本地库，无需外部依赖）
docker compose up -d

# 2. 准备配置（config.yaml 已被 gitignore，从模板复制）
cp backend/config/config.example.yaml backend/config/config.yaml
#    填入 database.password（与 .env 的 POSTGRES_PASSWORD 一致），
#    或用环境变量免改文件：export DATABASE_PASSWORD=xxx

# 3. 启动开发服务（默认 8080）
cd backend && go run ./cmd/server/main.go

# 4. 启动测试服务（8081）
export CONFIG_PATH="config/config-test.yaml" && cd backend && go run ./cmd/server/main.go

# 5. 编译
cd backend && go build -o server ./cmd/server/main.go

# 6. 生成 Swagger 文档（前端代码生成的前置步骤）
cd backend && bash scripts/gen-swagger.sh

# 7. 运行测试（必须指向测试库，先导出一次连接信息）
export TEST_DB_HOST=localhost TEST_DB_PORT=5432 TEST_DB_USER=aicamera \
       TEST_DB_PASSWORD='<.env 中的 POSTGRES_PASSWORD>' TEST_DB_NAME=ai_camera_test
cd backend && go run ./tests/testutil/resetdb/main.go
cd backend && go test ./tests/... -count=1 -p 1 -timeout 600s
```

> 后端配置支持**环境变量覆盖**（容器化部署用），映射见 [backend/config/config.go](backend/config/config.go) 的 `envBindings`。

### 前端（React / Electron）

```bash
cd frontend && npm install

cd frontend && npm run dev            # 管理后台开发模式（:5173，代理 /web /app /uploads）
cd frontend && npm run dev:website    # 官网开发模式（:5174）
cd frontend && npm run dev:desktop    # Electron 桌面端开发模式

cd frontend && npm run build          # 同时构建管理后台 + 官网
cd frontend && npm run build:admin    # 仅管理后台（dist/）
cd frontend && npm run build:website  # 仅官网（dist-website/）
cd frontend && npm run make:win       # 打包 Windows NSIS 安装包

cd frontend && bash scripts/api-gen.sh   # 从 Swagger 生成 API 客户端（需后端运行）
cd frontend && npx vitest run            # 单元测试（MSW mock，无需后端）
cd frontend && npx playwright test       # 端到端测试
```

### 移动端（Flutter）

```bash
cd app && flutter pub get
cd app && flutter run --dart-define=API_BASE_URL=http://localhost:8080
cd app && flutter test
cd app && flutter analyze   # 要求 0 error 0 warning
```

### 数据库

**本地调试使用本地 PostgreSQL**（Docker Compose 提供），不依赖远程实例。

| 环境 | 数据库 | MCP Server | 用途 |
|:---|:---|:---|:---|
| 开发 | `ai_camera_dev` | `postgres_ai_camera_dev` | 日常编码自测（**仅供查阅表结构**） |
| 测试 | `ai_camera_test` | `postgres_ai_camera_test` | 后端单测、接口测试、冒烟测试 |

- 连接地址：`localhost:5432`（`docker compose up -d` 起的容器），用户 `aicamera`，密码见 `.env`。
- 镜像在 PostgreSQL 16 基础上装好了 **PostGIS** 与 **pgvector** 两个扩展，见 [deploy/postgres/](deploy/postgres/)。
- MCP 配置：`cp .mcp.json.example .mcp.json` 后填入密码（`.mcp.json` 已被 gitignore）。
- **测试必须使用测试库**，禁止操作开发库。

---

## 文档索引

| 文档 | 内容 |
|:---|:---|
| [docs/00-系统设计总览.md](docs/00-系统设计总览.md) | 系统设计总览（架构、技术选型、模块划分） |
| [docs/需求文档/00-需求总览.md](docs/需求文档/00-需求总览.md) | 产品定位、目标用户、系统边界、核心业务流程、权限矩阵、非功能需求 |
| [docs/需求文档/01-系统管理模块.md](docs/需求文档/01-系统管理模块.md) | 个人中心、用户、菜单、角色、部门、岗位、操作日志、系统通知 |
| [docs/需求文档/02-基础信息模块.md](docs/需求文档/02-基础信息模块.md) | 区域管理、风险类型管理、告警类型管理 |
| [docs/需求文档/03-资源管理模块.md](docs/需求文档/03-资源管理模块.md) | 设备管理、语音配置、物联网卡、换卡记录、设备模式 |
| [docs/开发文档/后端开发/00-接口契约.md](docs/开发文档/后端开发/00-接口契约.md) | **接口契约**：路由总表、四层方法签名、响应/请求类型清单（含 v0.2.0 国标、v0.3.0 IOT） |
| [docs/开发文档/数据库设计/sql/ddl/](docs/开发文档/数据库设计/sql/ddl/) | DDL 脚本（01 系统基础 / 02 基础信息 / 03 资源管理 / 04 风险与告警 / 05 巡检 / 07 告警设置） |
| [docs/开发文档/页面功能和接口/00-页面功能与接口总览.md](docs/开发文档/页面功能和接口/00-页面功能与接口总览.md) | 48 个页面目录 + 统一响应格式约定 |
| [docs/测试文档/测试规范/](docs/测试文档/测试规范/) | 测试流程、单元/集成/接口/回归/端到端五份测试规范 |
| [docs/测试文档/测试用例/](docs/测试文档/测试用例/) | 8 份测试用例清单（认证与权限 / 系统管理 / 基础信息 / 资源管理 / 风险与告警 / 巡检 / 升级） |
| [docs/部署文档/01-部署与运维.md](docs/部署文档/01-部署与运维.md) | 部署架构、Docker 镜像构建、K8s 清单、CI/CD 流水线、备份策略、运维命令 |
| [docs/部署文档/02-监控与容灾.md](docs/部署文档/02-监控与容灾.md) | 监控分层、业务指标清单、告警分级、Grafana 看板、容灾目标与故障演练 |
| [docs/敏捷开发/CHANGE_LOG.md](docs/敏捷开发/CHANGE_LOG.md) | 变更日志 |
| [docs/敏捷开发/v0.1.0 基础功能开发/](docs/敏捷开发/v0.1.0%20基础功能开发/) | 软件开发计划 / 软件测试计划 / 开发总结 / 待完成事项 |
| [docs/介绍网站/README.md](docs/介绍网站/README.md) | 官网技术栈、页面结构、设计风格、构建部署 |
| [backend/README.md](backend/README.md) | 后端技术栈、项目结构、**API 路由总览**、权限链路、响应类型规范 |
| [frontend/README.md](frontend/README.md) | 三端构建、双入口构建说明、API 代码生成、环境检测、测试 |
| [app/README.md](app/README.md) | 移动端技术栈、项目结构、页面清单、认证流程、接口约定 |
| [CLAUDE.md](CLAUDE.md) | Claude Code 开发指引（架构、鉴权体系、开发规范、E2E 规范、测试流程） |

---

## 功能模块

| 模块 | 说明 |
|:---|:---|
| **首页看板** | 设备在线数、风险/告警概览、趋势图、待办入口 |
| **实时监控** | 单屏 / 4 宫格 / 9 宫格视频监控（当前为占位播放器） |
| **风险监控** | AI 风险列表、刷新频率可调、检测准确性复核、处理闭环、风险统计分析 |
| **设备监控** | 设备实时状态、GIS 地图定位、历史记录曲线与一周运行状态回放 |
| **设备告警** | 离线 / 电量 / 流量告警的接收、处理与趋势分析 |
| **物联网卡** | 卡台账、流量用量与到期预警、换卡记录追溯 |
| **巡检中心** | 巡检任务创建与派发（Web）、现场巡检与结果回传（App）、巡检记录归档 |
| **升级管理** | 固件类型/固件/AI 模型版本管理，现在升级 / 定时升级 / 批量升级与计划取消 |
| **告警设置** | 设备告警设置（通知人员/通知类型/报警频率）、风险通知设置、个推与腾讯云短信配置 |
| **统计分析** | 风险统计、告警统计、物联网卡流量统计、可视化大屏 |
| **系统管理** | 用户、角色、菜单、部门、岗位、系统配置、操作日志、系统通知、个人中心与 API Key |
| **基础信息** | 区域管理、风险类型管理、告警类型管理 |
| **资源管理** | 设备管理、设备模式、语音配置 |
| **国标视频（GB/T 28181）** | SIP 信令接入、设备/通道目录、实时播放与云台、回放/下载、录像计划、报警、移动位置、媒体节点、拉流代理/推流、级联与语音 Agent |
| **IOT 管理与多协议接入** | IOT 产品/服务组/服务、产品×服务凭据、设备账号与激活、OTA 下发、MQTT 鉴权与 WebRTC 信令、TURN/STUN（coturn userdb）、设备/分享二维码、SIP 首绑、App 蓝牙配网与扫码 |

平台合计 **63 张业务表**、**386 个 Swagger 接口**（Web 345，含 ZLM `/media/hook/*`；App 41），
另有设备端 `/iot/*` 与内部 `/rpc/*` 端点。

---

## 版本历史

| 版本 | 日期 | 主题 | 主要交付 |
|:---|:---|:---|:---|
| v0.5.1 | 2026-10-25 | 多服务组并存与账号权限对齐 | 设备/产品/国标列表服务组标签与筛选、IOT 国标接入、菜单→角色、角色启停/数据权限、岗位分配角色、部门树修复、参数配置 schema/active |
| v0.5.0 | 2026-10-25 | 运营后台信息架构 | 一级目录重组为首页/系统管理/物联网管理/国标视频/实时音视频/智能语音聊天/AI 智能分析；新增 WebRTC/AIChat 独立设备与服务设置页；设备按服务组类型过滤；菜单点击跳转修复；Casbin 受限角色实测 |
| v0.4.0 | 2026-10-25 | WVP 功能对齐 | 设备编辑/重启/布控/时间统计、通道摄像机配置/手动录像/媒体信息、预置位与巡航点、媒体指标与流会话、平台信息与实时日志、报警 SSE、代理/推流播放与导入、云端录像合并、免登录分享、5 引擎播放器、录像时间轴、Leaflet 地图；MQTT/TURN 本地部署修复 |
| v0.3.0 | 2026-10-24 | IOT 管理与多协议接入 | IOT 产品/服务组/设备三页、产品×服务凭据、设备 OTA 与激活、MQTT 鉴权与 WebRTC 信令、TURN/STUN（coturn userdb）、设备/分享二维码、SIP 首绑、App 蓝牙配网与扫码；后台设备解绑 |
| v0.2.0 | 2026-10-21 | 设备接入与国标视频 | GB/T 28181 信令服务、ZLMediaKit 媒体链路、19 个国标管理页、App 实时播放/云台/回放、级联与语音控制面 |
| v0.1.0 | 2026-09-21 | 基础功能开发（平台骨架 + 全模块落地） | 后端 37 表 / 200 端点 / 四级鉴权；管理后台 39 页面；官网 9 组件；移动端 9 页面；50 份设计文档。详见 [CHANGE_LOG.md](docs/敏捷开发/CHANGE_LOG.md) |

---

## 许可

Proprietary — 版权归项目所有方所有，未经授权不得复制、分发或用于商业用途。
