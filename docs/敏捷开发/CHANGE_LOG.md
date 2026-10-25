# AI Camera 变更日志 (CHANGE LOG)

> 项目: AI Camera 智能监控平台
> 当前版本: v0.6.0
> 当前开发: v0.6.0 智能语音聊天 / v0.2.0 真机验收
> 最后更新: 2026-10-25

---

## v0.6.0 — 智能语音聊天（Pipecat + WebSocket）

> 状态: **开发完成** · 设计与协议: [v0.6.0 智能语音聊天/00-设计与协议.md](v0.6.0%20智能语音聊天/00-设计与协议.md) ·
> 实现说明: [01-实现说明.md](v0.6.0%20智能语音聊天/01-实现说明.md)

**范围**：

| # | 内容 | 状态 |
|:--:|:--|:--|
| 1 | Pipecat `/ws/voice`，与 GB28181 RTP 并存 | ✅ |
| 2 | 小智兼容协议（hello/listen/detect/abort/tts/mcp） | ✅ |
| 3 | PCM16（App/Web）与 Opus（ESP32）协商 | ✅ |
| 4 | 半双工 + 服务端 VAD + 快速应答 | ✅ |
| 5 | Go WS ticket 与 Web/App/设备 WS 网关 | ✅ |
| 6 | 服务端 MCP（6 工具、权限、审计） | ✅ |
| 7 | 设备端 MCP bridge | ✅ |
| 8 | 短期上下文 + pgvector 长期记忆 | ✅ |
| 9 | Flutter App 语音聊天页 | ✅ |
| 10 | Web 语音 Agent 配置/MCP/记忆页 | ✅ |

**关键设计**：
- Pipecat 仅内网可达；生产由 Go 网关统一鉴权并注入签名上下文。
- ticket 60s 一次性；设备用产品×AIChat 凭据 HTTP Basic。
- 服务端 MCP 用 JSON-RPC over HTTP；设备 MCP 复用同一 WS 的 `type=mcp`。
- 长期记忆默认 BGE-M3 1024 维 + HNSW；未配置 embedding 时降级为时间/关键词检索。
- 默认半双工，`barge_in` 预留未启用；设备端唤醒主路径。

**已知限制**：见 [01-实现说明.md §3](v0.6.0%20智能语音聊天/01-实现说明.md)。

---

## v0.5.1 — 多服务组并存与账号权限对齐

> 说明文档: [v0.5.1 多服务组与账号权限对齐/00-实施说明.md](v0.5.1%20多服务组与账号权限对齐/00-实施说明.md)

- 设备/产品/国标列表返回服务组摘要；`service_group_type` 过滤支持同一设备命中多组。
- IOT 设备页新增国标接入；国标设备页显示产品/服务组并支持筛选。
- 菜单管理：停用/移动端菜单可见、状态开关、菜单→角色授权、父级类型校验。
- 角色启停与数据权限表单；岗位分配角色；部门叶子/后代 Bug 修复。
- 参数配置 active/schema、类型渲染与 JSON 校验；用户岗位筛选。
- 后端全量测试、前端 tsc/vitest/build、Playwright 04b/04c/14-ia 通过。

---

## v0.5.0 — 运营后台信息架构重组

> 状态: **开发完成** · 说明文档: [v0.5.0 运营后台信息架构/00-实施说明.md](v0.5.0%20运营后台信息架构/00-实施说明.md)

**一级目录**：

| 顺序 | 目录 | 内容 |
|:--:|:--|:--|
| 1 | 首页 | 看板 |
| 2 | 系统管理 | 账号与权限、基础信息、告警设置、第三方平台 |
| 3 | 物联网管理 | 产品、服务组、设备、设备资源、升级、设备监控 |
| 4 | 国标视频 | GB28181 全部能力 |
| 5 | 实时音视频 | WebRTC 服务组设备管理、服务设置 |
| 6 | 智能语音聊天 | AIChat 服务组设备管理、服务设置、语音 Agent |
| 7 | AI 智能分析 | 实时监控、AI 风险/监控、风险监控、巡检、统计分析 |

**交付**：
- 新增 `/av/device`、`/av/service`、`/voice/device`、`/voice/service`、
  `/voice/agent` 页面；
- `GET /web/iot/devices` 新增 `service_group_type` 过滤；
- 菜单树迁移 SQL（init §14，幂等）、祖先目录自动补齐、新菜单 API 绑定；
- 修复左侧菜单点击不跳转（`MainLayout` 增加 `onClick`）；
- Casbin 受限角色实测：允许接口 200、未授权接口 403；
- Playwright `14-ia.spec.ts` 覆盖新 IA。

---

## v0.4.0 — WVP-GB28181-Pro 功能对齐

> 状态: **开发完成** · 说明文档: [v0.4.0 WVP功能对齐/00-实施说明.md](v0.4.0%20WVP功能对齐/00-实施说明.md)
>
> 范围：除 JT/T 1078 部标模块外，WVP-GB28181-Pro 的页面与能力在本项目补齐。

**范围**：

| # | 内容 | 状态 |
|:--:|:--|:--|
| 1 | 国标设备编辑（SIP/媒体/位置） | ✅ |
| 2 | 目录同步进度 | ✅ |
| 3 | 设备重启 / 布防撤防 / 报警复位 | ✅ |
| 4 | 心跳/注册时间统计 | ✅ |
| 5 | 通道摄像机配置字段 | ✅ |
| 6 | 手动录像控制 / 媒体信息 | ✅ |
| 7 | 预置位名称持久化与重命名 | ✅ |
| 8 | 巡航点 CRUD / 扫描边界（标准 MANSCDP 码） | ✅ |
| 9 | 媒体节点 CPU/内存/磁盘/网络指标与历史 | ✅ |
| 10 | 当前流会话面板 | ✅ |
| 11 | 平台信息 / 实时日志 / 日志检索 | ✅ |
| 12 | 报警 SSE 实时推送 | ✅ |
| 13 | 拉流代理/推流内嵌播放 | ✅ |
| 14 | 推流通道 CSV/TSV 导入 | ✅ |
| 15 | 云端录像多选打包下载 + ffmpeg 合并任务 | ✅ |
| 16 | 免登录分享播放 | ✅ |
| 17 | 播放器 5 引擎（WebRTC/FLV/Jessibuca/H265web/HLS） | ✅ |
| 18 | 录像时间轴（Canvas 片段/拖拽 seek） | ✅ |
| 19 | 电子地图底图（Leaflet + OSM） | ✅ |
| 20 | 国标设备树/通道选择等页面字段级补齐 | ✅ |

**关键设计决策**：
- 播放器引擎选择与后端点播解耦：切换引擎只切前端，不重新 INVITE；
- H.265 用 h265web WASM，HLS 用 hls.js；Jessibuca 作为可选第五引擎动态加载
  （开源版 GPL-3.0，已在静态资源目录标注许可）；
- 云端录像合并依赖 ffmpeg，失败原因落库；打包 zip 仍走既有 `?ids=` 流式接口；
- 分享令牌独立于用户 JWT，库中只存 SHA-256，匿名路由不挂 authMW/casbinMW；
- 媒体指标采集后端所在主机（与 WVP 的 oshi 一致），不是 ZLM API；
- 修复 `gb_device_id` 部分唯一索引：排除空串，允许多台非国标设备。

**范围外**：
- JT/T 1078 部标终端/通道/参数/录像/对讲——独立协议栈，单独立项；
- 云端录像合并仅支持编码参数一致的片段；xlsx 导入暂不支持（仅 CSV/TSV）。

**配套**：
- 新增 3 张表：`ac_gb_ptz_preset`、`ac_gb_share_token`、`ac_gb_record_merge_task`；
- 新增 28 个菜单/按钮权限、30 条 `ac_menu_api`；
- 部署文档新增「宿主机后端 + 容器 MQTT/TURN」联调章节；
- 修复 mochi-mqtt v2.7.9 `Serve()` 不阻塞导致容器反复重启；
- 后端测试全绿（`go test ./tests/... -p 1`），前端 Vitest/类型检查/构建通过。

---

## v0.3.0 — IOT 管理与多协议接入

> 状态: **开发中** · 设计文档: [v0.3.0 IOT与多协议接入/00-新增需求与设计.md](v0.3.0%20IOT与多协议接入/00-新增需求与设计.md)

本版本收录 2026-10-24 起**陆续新增**的需求，与 v0.2.0 的 GB28181 范围并行推进。

**范围**：

| # | 内容 | 状态 |
|:--:|:--|:--|
| 1 | IOT 产品管理（自动联网、配网方式、分享、服务组配置） | ✅ |
| 2 | IOT 设备管理（按产品筛选、主/分享账号） | ✅ |
| 3 | 服务组/服务项（GB28181、WebRTC、AI 聊天三类的协议白名单） | ✅ |
| 4 | 设备端 OTA / 激活接口 | ✅ |
| 5 | 产品×服务凭据自动生成与设备端下发 | ✅ |
| 6 | 服务鉴权信息复制（产品/服务组页面） | ✅ |
| 7 | MQTT 鉴权（mochi-mqtt → 后端 RPC，双层 fail-closed） | ✅ |
| 8 | TURN/STUN（coturn + 后端签发 TURN REST 凭据） | ✅ |
| 9 | MQTT 作为 WebRTC 信令 | ✅ |
| 10 | App 添加设备（蓝牙配网 + 扫一扫） | ✅ |
| 11 | SIP 携带设备编码/产品编码用于绑定 | ✅ |
| 12 | 设备二维码（产品编码 + 设备编码，URL 格式） | ✅ |
| 13 | 两类产品基线（配网方式/服务组/分享均由产品配置，种子预置） | ✅ |

**关键设计决策**：
- 设备复用 `ac_device`，不新建 IOT 设备表；
- 凭据是**产品 × 服务级**，产品选择服务组后自动为每个服务生成；服务组不持有凭据；
- coturn **没有** HTTP 鉴权回调，自定义授权在后端签发 TURN REST 凭据；
- MQTT 鉴权与 ACL 全部回后端 RPC，broker 与后端都 fail-closed；
- SIP 绑定只在「产品+设备编码匹配」时允许写入国标编码，不允许静默改绑。

**本次完善与收口（2026-10-24）**：

| 项 | 内容 |
|:--|:--|
| 安全 | IOT 设备读接口同时脱敏 `device_secret` 与 `sip_password`；OTA 不回传两者；账号新增/编辑不再接受调用方传入 `share_token`；产品分享码与设备分享码均校验产品 `allow_share` / 产品绑定 |
| 权限 | 修正产品分享码权限码为 `iot:product:update`（种子旧库通过 `UPDATE` 修复）；新增 `iot:device:unbind` |
| 功能 | 新增后台解绑 `POST /web/iot/devices/:id/unbind`（删除全部账号、清空激活状态、幂等）；设备管理新增「服务授权」弹窗；分享码按钮按 `allow_share` 禁用 |
| 数据模型 | 四层模型：服务组（固定三条）→ 服务 → 产品-服务组 → 产品-服务凭据；服务组不再持有凭据 |
| 服务组约束 | 编码固定 `gb28181` / `webrtc` / `aichat`，不开放新增/删除，编码与类型不可改；被产品引用后只读（编辑/编辑服务/删除均拒绝），列表返回 `referenced_by_products` |
| 产品凭据 | 产品创建/编辑时选择服务组，平台为组内每个服务自动生成独立用户名/密码；支持按产品×服务重置 |
| TURN | 产品×TURN 凭据同步 coturn SQL userdb（`turnusers_lt`）；App 播放仍用 REST 短期凭据 |
| 部署 | compose / K8s 为 backend 注入 TURN 变量；coturn 增加 `--userdb` 指向应用库 |
| App | 补 iOS 蓝牙/局域网权限说明；WHEP 与 MQTT 共用 `/app/rtc/ice-servers` 的 TURN 凭据；服务凭据改为按服务解析 |
| 测试 | 后端新增脱敏/分享校验/二维码前置/解绑/TURN 同步测试；Playwright 新增 `12a`/`12b`/`12c` 共 23 个 IOT 用例 |
| 文档 | 根 / backend / frontend / app README、设计文档状态、接口契约、页面总览、新增 `14-IOT管理页面与接口.md` |

---

## v0.2.0 — 设备接入与国标视频平台（开发中）

> 状态: **一期开发中** · 设计文档: [v0.2.0 设备接入与国标视频/00-设计与工作量评估.md](v0.2.0%20设备接入与国标视频/00-设计与工作量评估.md)

**目标**：把平台从「纯管理」推进到「能接真实相机、能看真实画面」。落地 GB28181 设备接入、SIP 信令、
ZLMediaKit 媒体链路、Pipecat 语音 Agent，并让 Web 与 App 两端都能看实时画面、控云台、查回放。

**核心需求与落点**：

| 需求 | 落点 |
|:--|:--|
| 设备注册、激活 | `ac_device` 新增 `gb_device_id` / `sip_password` / `register_status` 等 13 列 + 激活状态机 |
| **SIP 设备必须是设备管理里已有的设备，否则拒绝接入** | SIP 信令层收到 REGISTER 直查 `ac_device.gb_device_id`，查不到即回 **403 Forbidden** 并记 `ac_gb_sip_log` |
| ZLMediaKit 推流 + Pipecat 语音 Agent | ZLM 独占 RTP/PS 与转协议；Pipecat 走 Docker，STT/LLM/TTS 可后台动态配置 |
| App 看画面/回放/云台/AI 事件 | 新增 `flutter_webrtc` + `media_kit` 播放器与 6 个页面，走 `/app/gb/*` |
| 开源版 docker compose 部署 + 本地数据库 | 仓库根 `docker-compose.yml`（profiles 区分用法）；`deploy/postgres/` 提供 PostgreSQL 16 + PostGIS + pgvector |

**已交付（本次）**：

- 设计文档 `docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md`（17 节，含**六条经代码核实的技术结论**、**18 张表设计**、测试策略、**15 条风险**、15 条验收标准、文档回写清单）。已经过**两轮独立评审**并据其修订
- DDL 脚本 `docs/开发文档/数据库设计/sql/ddl/09-国标视频.sql`（**18 张新表** + `ac_device` 扩展 13 列）
- `docker-compose.yml` + `deploy/postgres/`（PostgreSQL 16 + PostGIS + pgvector 自定义镜像）
- 配置模板 `config.example.yaml` / `config-test.example.yaml` / `config.compose.yaml` / `.env.example` / `.mcp.json.example`
- 后端配置支持**环境变量覆盖**（`envBindings`），容器化部署密钥不落盘
- 修复 `DSN()` 在密码为空时输出 `password=` 导致 pgx 吞掉后续键的 bug

**工作量**：**515 人天**（2026-10-15 砍掉 JT/T 1078 后），三期 232 / **92** / 107。
> 估算经历五次上修 + 一次下调：255 → 386 → 442 → 460 → 490 → 570 → 515。详见设计文档 §14 与风险 R0。

### 实施进度（2026-10-21）

| 批次 | 内容 | 状态 |
|:--|:--|:--:|
| 数据层 | 18 张 `ac_*` 表 + `ac_device` 13 列；DDL / GORM 模型 / Repository / 请求 DTO | ✅ |
| 部署 | ZLM 接入 compose；单端口收流实测贯通（PS→H264+AAC→HTTP-FLV）；PG 含 PostGIS + pgvector | ✅ |
| 模拟器 | `tools/gbsim`：注册/心跳/目录/INVITE/SDP/PS 推流，82 单测 + `-race` | ✅ |
| **Wave 1** | **SIP 信令服务**（`cmd/sipsvc`：白名单 403、服务端 Digest、MANSCDP、Catalog、SSRC、INVITE/BYE）、**ZLM 客户端 + 10 个 hook**、**设备/通道/媒体节点/分组/订阅/信令日志 CRUD** | ✅ |
| Wave 2 | 播放/云台/语音 + 回放/录像/报警/位置/级联配置 + `/app/gb` | ✅ |
| Wave 3 | React 19 页 + Flutter 4 页 + Playwright 21 例 + 权限 SQL 对齐（117 条）+ 文档收口 | ✅ |
| 语音控制面 | `deploy/pipecat`：控制面（配置下发/会话/连通性检查）、G.711↔PCM16、compose `voice` profile | ✅（音频桥接未接通） |

**Wave 1 实测结论**（已同步到设计文档 §6.2b）：

- REGISTER → 401 → Digest → 200；`register_status=activated`、`status=online`
- 未建档设备 → **403 Forbidden** + `ac_gb_sip_log` 拒绝记录（核心需求 2 的验收点）
- 平台下发 Catalog → 设备 GB2312 响应 → `ac_gb_channel` 落库，中文通道名正确
- INVITE → 200(SDP) → ACK → 推流 → BYE 全链路通过
- **Digest 只能作用于 REGISTER**：设备的 Keepalive / Catalog 响应 MESSAGE 不带凭证
- **目录分片必须按字节**：2 个通道的目录报文就已触到 sipgo 的 UDP MTU 上限

**已知限制与待办**（均为显式边界，不是遗漏）：

| 项 | 状态 |
|:--|:--|
| 真机验收 | ⏳ 一期硬门槛，需至少 1 台真实 GB28181 相机（模拟器只能做回归夹具） |
| 语音音频链路 | ⏳ `deploy/pipecat` 控制面已落地，但音频桥接为 `not_connected`；Web/App 的对讲能建会话、听不到声音 |
| 级联信令 | ⏳ 只做平台/通道/分组/区域配置与映射，不发起 REGISTER、不推 Catalog、不处理上级 INVITE |
| 推流 | ⏳ Start/Stop 只维护状态，不向外部真实推流 |
| 录像计划 | ⏳ 只做配置入库，无定时调度 |
| 下载任务 | ⏳ 仅进程内存，重启丢失；进度依赖 ZLM `on_record_mp4` 回调 |
| `ac_gb_subscribe` | ⏳ 无通道字段，订阅只能到设备级 |
| 二维码 | ⏳ 前端未引入二维码库，激活凭证以文本 + 复制展示 |
| `useForm` 控制台告警 | ⏳ 多个 GB 页面的 Modal 在关闭态创建表单；功能无影响，E2E 已窄过滤 |
| K8s 清单 | ⏳ 仅提供 compose 与运行约束 |
| `01-数据字典` / `02-表关系全景图` | ✅ 已补 18 张表 |

---

## v0.1.0 — 基础功能开发（平台骨架 + 全模块落地） (2026-09-21)

### 🎯 本版本目标

从零构建 **AI 摄像头智能监控平台** 的完整骨架与全部业务模块。本版本以 `docs/需求文档/`
中「AI摄像头操作手册」的功能结构为基准，一次性落地 **三个端**：Go 后端
（`Gin 1.12 + GORM 1.25 + PostgreSQL 16 + Casbin 2.135`）、React 19 前端（`Vite 5`，
一个 npm 工程双入口产出 **管理后台** 与 **官网介绍页**，并支持 `Electron 33` 桌面端打包）、
Flutter 移动端（`GoRouter + Dio + flutter_dva`）。后端严格按
`Handler → Service → Repository → Model` 分层，**38 张业务表**（全部以 `ac_` 前缀）与
**209 个 API 端点**（`/web/*` 189 个 + `/app/*` 20 个）在 `docs/开发文档/后端开发/00-接口契约.md`
中先冻结后实现，四层命名与方法签名逐字对齐契约，保证 `internal/api/router/web.go` 可一次性装配。

本版本在鉴权上刻意做了**相对 boticz-mes 的加固**：四级权限叠加（JWT → Auth 中间件 →
Casbin RBAC → 数据权限）中的 Casbin matcher 补齐了 `keyMatch2(r.obj, p.obj) && r.act == p.act`
（见下方「🔐 鉴权体系」一节），修复了原方案「只校验角色成员、不校验接口与方法的鉴权漏洞」——
在旧写法下，只要用户持有任一角色，就能访问**任意** `/web/*` 接口，接口级鉴权形同虚设。
全平台业务闭环围绕 `ac_risk_record` 展开：设备经 ApiKey 上报风险 → 安全员复核「检测准确 yes/no」
→ 点「否」直接归档、点「是」填写处理结果与图片 → 状态 `pending → handled`，与操作手册的
「日常操作流程」逐步对应。

### 📋 模块概览: 🏗后端骨架 / 🔐鉴权体系 / 📹设备与风险 / 🖥管理后台 / 🌐官网 / 📱移动端 / 📚文档体系

### 🏗 后端骨架（分层装配与基础设施）

| 文件 | 变更 |
|:---|:---|
| `backend/go.mod` | 新增；`module github.com/boticz/ai-camera-backend`，Go 1.26；直接依赖 gin/casbin+gorm-adapter/golang-jwt/v5/robfig-cron/viper/swag v2/zap/lumberjack/gorm-postgres |
| `backend/cmd/server/main.go` | 新增；入口仅做 `router.SetupRouter()` + `http.Server` 启停；注册 `SIGINT/SIGTERM` 优雅退出（10 秒在途请求窗口） |
| `backend/config/config.go` | 新增；viper 加载 YAML 为 `Config` 结构（server/database/redis/jwt/casbin/media/thirdparty/log/swagger 九组）；`DatabaseConfig.DSN()` 拼接 `sslmode` + `TimeZone=Asia/Shanghai` |
| `backend/config/config.yaml` | 新增（已 gitignore）；开发环境配置，端口 8080，库 `ai_camera_dev`，`media.upload_dir=./uploads` |
| `backend/config/config-test.yaml` | 新增（已 gitignore）；测试环境配置，端口 **8081**，库 `ai_camera_test`，日志输出到 console 便于冒烟排查 |
| `backend/config/rbac_model.conf` | 新增；`g`/`g2`/`g3` 三条角色继承链；matcher 为 `keyMatch2(r.obj, p.obj) && r.act == p.act && (g \|\| g2 \|\| g3)` |
| `backend/internal/bootstrap/main.go` | 新增；`init()` 自动装配配置→日志→数据库→迁移→JWT→Casbin；暴露 `DB()`/`Config()`/`JWTService()`/`CasbinService()` 访问器与 `SetXxx()` 测试注入点；**Casbin 初始化失败不阻塞启动**，退化为放行并打印显著告警 |
| `backend/internal/bootstrap/migrate.go` | 新增；一次性 `AutoMigrate(38 models...)`（fail-fast，任一模型失败即 `log.Fatalf` 中止，避免静默跳过导致表结构漂移）；初次建表走 `docs/开发文档/数据库设计/sql/ddl/*.sql` |
| `backend/pkg/database/postgres.go` | 新增；GORM PostgreSQL 连接，连接池按配置设置 `MaxIdleConns/MaxOpenConns/ConnMaxLifetime` |
| `backend/pkg/auth/jwt.go` | 新增；JWT HS256 签发与解析，claims 携带 `user_id`/`username`/`role_code`/`dept_id`/`data_perm`/`company_id`/`cross_dept_ids` |
| `backend/pkg/perm/casbin.go` | 新增；封装 enforcer，提供 `Enforce/AddPolicy/RemoveFilteredPolicyByRole/Add(Dept\|Post)GroupingPolicy/ReloadPolicy/StartAutoReload`；`StartAutoReload(5m)` 保证**多副本部署下策略最终一致** |
| `backend/pkg/utils/response.go` | 新增；统一 `{code, message, data}` 响应体，提供 `Success/Created/BadRequest/Unauthorized/Forbidden/NotFound/ServerError` |
| `backend/pkg/utils/data_scope.go` | 新增；`DataScope` 行级权限，`ApplyToQuery(q, "alias.dept_id")` 按 `all/cross_dept/dept/personal` 四种模式附加过滤；`created_by` 列名由 dept 列前缀推导，避免多表 JOIN 歧义 |
| `backend/pkg/utils/logger.go` | 新增；zap + lumberjack 日志（滚动大小/备份数/保留天数按配置），支持 `json` 与 `console` 两种格式 |
| `backend/internal/api/router/router.go` | 新增；`SetupRouter()` 装配 Gin Engine、Recovery、操作日志中间件与 `/health` |
| `backend/internal/api/router/web.go` | 新增（355 行）；`/web` 全部路由注册，含 `wApiKey`（ApiKey 上报）、`wAuth`（JWT + Casbin）两组；**静态子路径先于 `/:id` 注册**（`/devices/modes`、`/devices/iot-cards`、`/upgrades/plans/cancel` 等） |
| `backend/internal/api/router/app.go` | 新增（88 行）；`/app` 移动端 20 个端点注册，`/app/auth/login` 公开，其余走 JWT + Casbin |
| `backend/internal/api/router/swagger.go` | 新增；以 `StaticFile` 提供 `/swagger-ui`、`/web/api-docs.json`、`/app/api-docs.json`，未生成文档时服务仍可启动 |
| `backend/scripts/gen-swagger.sh` | 新增；`swag init` 分别从 `handler/web` 与 `handler/app` 生成 `docs/web/web_swagger.json` 与 `docs/app/app_swagger.json`（`--parseDependency --parseInternal`） |
| `backend/scripts/build.sh` | 新增；`docker build` + `docker push`；`--prod` 切换 `Dockerfile.prod` 与生产镜像仓库 |
| `backend/scripts/publish.sh` | 新增；`kubectl rollout restart` 重启 Deployment；`--prod` 切换生产 K8s context 并等待就绪 |
| `backend/static/swagger-ui/index.html` | 新增；Swagger UI 静态页面，通过 `StaticFile` 挂载到 `/swagger-ui` |

### 🗄 数据模型（38 张业务表）

| 文件 | 变更 |
|:---|:---|
| `backend/internal/model/base.go` | 新增；`BaseModel` 嵌入 `ID`/`DeletedAt`（软删除）/`CreatedBy`/`CreatedDT`/`UpdatedBy`/`UpdatedDT`；`BeforeCreate`/`BeforeUpdate` 从 Go Context 自动填充审计字段与 `dept_id`；提供 `WithUsername/WithDeptID/WithUserID` |
| `backend/internal/model/system.go` | 新增（246 行）；`User`/`Role`/`Menu`/`MenuAPI`/`Dept`/`Post`/`UserRoleRel`/`UserPostRel`/`PostRoleRel`/`DeptRoleRel`/`RoleMenuRel`/`SystemConfig`/`ApiKey`/`OperationLog`/`Notification`/`Sequence`（16 张表） |
| `backend/internal/model/basic.go` | 新增；`Region`/`RiskType`/`AlarmType`（3 张表） |
| `backend/internal/model/device.go` | 新增（177 行）；`Device`（含电量/电压/存储/`last_beat_dt`/`monitor_enabled`/`stream_url`）、`DeviceMode`、`IotCard`（流量三字段 + 四态状态）、`IotCardReplaceLog`、`VoiceConfig`、`DeviceHistory`（时序采样，复合索引 `(device_id, sampled_dt)`）（6 张表） |
| `backend/internal/model/risk.go` | 新增；`RiskRecord`（`detect_accurate`/`read`/`status` 三态字段 + `voice_played`）、`AlarmRecord`（结构平行的告警表）（2 张表） |
| `backend/internal/model/inspect.go` | 新增；`InspectTask`/`InspectRecord`（2 张表） |
| `backend/internal/model/upgrade.go` | 新增；`FirmwareType`/`Firmware`/`AIModel`/`DeviceUpgradePlan`/`DeviceUpgradeRecord`（5 张表） |
| `backend/internal/model/notify.go` | 新增；`AlarmSetting`（通知人员/通知类型/报警频率）、`RiskNotifySetting`、`ThirdPartyConfig`（3 张表） |
| `backend/internal/model/webhook.go` | 新增；`WebhookConfig`（消息推送通道：平台/地址/密钥/订阅事件/最近推送结果快照），`Subscribes()` 按事件精确匹配（1 张表） |

> **注意**：所有业务表统一 `ac_` 前缀（`ac_device`、`ac_risk_record` …），与 Casbin 适配器
> 自动创建的 `casbin_rule` 表明确区分；`ac_` 前缀同时是备份校验脚本（`verify-backup.sh`）
> 统计表数量的筛选条件。

### 🔐 鉴权体系（四级权限叠加 + 漏洞修复）

| 文件 | 变更 |
|:---|:---|
| `backend/config/rbac_model.conf` | matcher **补齐接口与方法校验**：`keyMatch2(r.obj, p.obj) && r.act == p.act && (g(r.sub, p.sub) \|\| g2(r.sub, p.sub) \|\| g3(r.sub, p.sub))`；`keyMatch2` 支持 `/web/devices/:id` 形式的路径参数匹配 |
| `backend/pkg/perm/casbin.go` | `p` 策略由「角色 → 菜单 → 菜单API」三级推导写入 `casbin_rule`；`g`(用户→角色) / `g2`(部门→角色) / `g3`(岗位→角色) 三种继承分别提供增删与按角色批量清理方法 |
| `backend/internal/api/middleware/auth.go` | 新增；`AuthMiddleware` 解析 Bearer token 并写入 gin.Context（`user_id`/`user_name`/`role_code`/`dept_id`/`data_perm`/`company_id`/`cross_dept_ids`），同时写入 Go Context 驱动 GORM 钩子自动填充；`CasbinMiddleware(enforce)` 以 **登录名** 为 sub 调用 `enforce(username, path, method)`，拒绝返回 403「权限不足」 |
| `backend/internal/api/middleware/api_key_auth.go` | 新增；`X-Api-Key` 头 → SHA256 → 查 `ac_api_key`（`key_hash + active=true`）→ 写 `user_id`/`api_key_id`；更新 `last_used_dt` 失败不影响请求；**该分组不经过 Casbin**，仅挂载到设备上报三个端点 |
| `backend/internal/api/middleware/operation_log.go` | 新增；记录增/删/改操作的 `ac_operation_log`（支持导出 CSV 的审计要求） |
| `backend/internal/service/system/role_service.go` | 新增；`syncCasbinForRole(ctx, roleID)` 重建该角色的 `p`/`g`/`g2`/`g3` 策略并 `ReloadPolicy()`，在 `AssignMenus/AssignDepts/AssignPosts/AssignUsers` 后调用 |
| `backend/pkg/utils/data_scope.go` | 第四级权限：`all` / `cross_dept` / `dept` / `personal` 四种模式按 `dept_id` 与 `created_by` 施加行级过滤；`dept_id` 为空的表（如 `ac_menu`、`ac_role`）不做过滤 |

> **注意**：本版本最关键的一处修正 —— 旧方案 matcher 只写 `g(r.sub, p.sub) || g2(...) || g3(...)`，
> 意味着 `r.obj`（请求路径）与 `r.act`（请求方法）**完全不参与判定**：只要用户拥有任意一个角色，
> 该角色在 `casbin_rule` 中登记过**任意一条**策略，就会被放行到**所有**接口。表现上「登录后
> 什么都能点」，接口级鉴权形同虚设。补齐 `keyMatch2 + r.act == p.act` 后，鉴权粒度回到
> 「角色 → 接口路径 + HTTP 方法」，与「角色管理 → 菜单授权 → 菜单 API」的三级模型一致。

### 📹 资源管理与设备上报

| 文件 | 变更 |
|:---|:---|
| `backend/internal/service/device/device_service.go` | 新增；设备 CRUD + `SwitchMode`/`SyncTime`/`ToggleMonitor` + `Heartbeat`（刷新在线状态/电量/电压/存储/`last_beat_dt` 并写 `ac_device_history` 时序）+ `PageHistory`/`HistoryChart`（近 3 日曲线与一周状态回放） |
| `backend/internal/service/device/device_mode_service.go` | 新增；设备模式（充电/全开/省电）CRUD |
| `backend/internal/service/device/iot_card_service.go` | 新增；物联网卡 CRUD + `ReplaceCard`（换卡同时写 `ac_iot_card_replace_log` 并更新设备 `iot_card_id`）+ `PageReplaceLogs` |
| `backend/internal/service/device/voice_config_service.go` | 新增；语音配置（按风险类型绑定播报音频）CRUD |
| `backend/internal/service/risk/risk_service.go` | 新增；`Report`（设备上报，落库 `status=pending`/`read=false`）、`MarkRead`、`UpdateAccurate`（点「否」直接置 `status=handled` + `read=true`）、`Handle`（填写处理结果与图片 → `status=handled` + `handler`/`handled_dt`）、`PendingCount` |
| `backend/internal/service/alarm/alarm_service.go` | 新增；结构与风险平行（`Report`/`Page`/`Get`/`MarkRead`/`Handle`/`Delete`/`PendingCount`） |
| `backend/internal/service/inspect/inspect_service.go` | 新增；巡检任务 CRUD + `PageRecords`/`GetRecord` + `FinishTask`（App 端回传处理结果与图片，写 `ac_inspect_record` 并置任务 `finished`） |
| `backend/internal/service/upgrade/upgrade_service.go` | 新增；固件类型/固件/AI 模型三套 CRUD + `UpgradeDevices`（现在升级/定时升级/批量升级）+ `PagePlans`/`CancelPlan`/`BatchCancelPlans`/`PageRecords` |
| `backend/internal/service/basic/region_service.go` | 新增；区域 CRUD（列表按 `region.name` 展示，作为全平台归属维度） |
| `backend/internal/service/basic/risk_type_service.go` | 新增；风险类型 CRUD（`high`/`medium`/`low` 等级驱动首页与大屏配色） |
| `backend/internal/service/basic/alarm_type_service.go` | 新增；告警类型 CRUD |
| `backend/internal/service/setting/setting_service.go` | 新增；设备告警设置、风险通知设置（均**只能操作当前登录用户的数据**，`user_id` 取自上下文）+ 个推/腾讯云短信配置读写 + `DisableAllAlarm`/`DisableAllRiskNotify` |
| `backend/internal/service/stats/stats_service.go` | 新增；`Dashboard`（首页看板）、`RiskStats`、`AlarmStats`、`Screen`（可视化大屏）、`TrafficStats`（物联网卡流量）、`DeviceStatus`（设备状态分布）六个聚合接口 |
| `backend/internal/api/handler/web/upload_handler.go` | 新增；`Upload`（风险图片/处理图片/固件/模型）与 `UploadAudio`（播报音频），URL 存库、文件落 `media.upload_dir` |
| `backend/internal/repository/*.go` | 新增 22 个仓储文件；`Page` 类方法返回 `(items, total, err)`，列表通过 GORM `Preload` 带出关联名称字段（`risk_type.name`、`device.name`、`region.name`），满足「列表不展示裸 ID」规范 |

> **注意**：设备模式、物联网卡、语音配置与设备本体共用 `/web/devices` 前缀，且 `/:id`
> 与静态子路径同级。契约文档与 `web.go` 都显式约定 **静态子路径必须注册在 `/:id` 之前**，
> 否则 `/web/devices/modes` 会被 `/:id` 吞掉（`id` 收到字符串 `"modes"`）。

### 🐛 端到端联调暴露并修复的阻断级缺陷

E2E 全量冒烟（27 个 spec / 164 用例）跑通后，暴露了两个**「页面看起来是空的」**类缺陷。
两者都不是崩溃，而是**静默失效**，靠单元测试发现不了，必须端到端才看得见。

| 文件 | 变更 |
|:---|:---|
| `backend/internal/api/middleware/normalize_query.go` | **新增**。全局中间件，绑定前剔除查询串中值为空串的参数 |
| `backend/internal/api/router/router.go` | `r.Use(middleware.NormalizeEmptyQuery())` |
| `docs/开发文档/数据库设计/sql/init/01-初始数据.sql` | 追加 **37 个 `type='button'` 菜单行**（id 9001~9037），补齐页面检查的权限码，并授予 `super_admin` |

**缺陷一：空查询参数导致列表恒为 0 条**

前端生成的 Controller 对未填写的筛选项发 `?keyword=&active=`（而非省略该键）。
后端 `active` 是 `*bool`，绑定空串会得到「**非 nil 且值为 false**」的指针，
于是生成 `WHERE active = false` —— 用户、角色、岗位、系统通知列表**恒为空**。
表现为「管理页面点进去一条数据都没有」，但接口返回 200，无任何报错。

```
修复前： GET /web/system/users?page=1&page_size=5&keyword=&active=  → total=0
修复后： 同上                                                        → total=6
         GET /web/system/users?...&active=false                     → total=0（显式传值仍生效）
```

选择在**中间件层统一修复**而非逐个改 DTO：一处改动覆盖全部 207 个接口，
且对任何客户端（含第三方调用方）都成立。中间件只删除「取值全为空白」的参数，
`?active=false` 这类显式传值不受影响。

**缺陷二：权限码不一致导致按钮对超管也不渲染**

前端用 `permCodes.includes('basic:region:create')`（`<模块>:<实体>:<动作>`）判定按钮显隐，
而种子菜单树里只有 `region:create` 这类旧码。`permCodes` 完全来自
`GET /web/profile/menus` 返回的 `permission_code`，库里没有对应行 → **按钮静默不渲染**，
页面上「新增/编辑/删除」全部消失，无任何报错。实测受影响的有 64 处，涉及基础信息、
资源管理、巡检、告警设置、风险通知、第三方平台、升级管理、系统管理等全部子页。

已补齐 37 个按钮权限码并授权给 `super_admin`；校验结果：

```
前端 permCodes.includes('...') 取值集合：60 个
/web/profile/menus 返回的 permission_code：192 个
差集（页面检查但接口未返回）：0 个  ✅
```

> **注意**：按钮权限码必须**先入库**（`ac_menu` 中 `type='button'` 的行）**再在前端引用**，
> 否则既不报错也不渲染。排查手法见 `CLAUDE.md` 的「按钮权限码规范」。

### 🔧 三处遗留缺陷修复（v0.1.0 收尾）

| 缺陷 | 修复 |
|:---|:---|
| 巡检任务「查看」不转已读 | 新增 `PUT /web/inspects/tasks/:id/read`（Service 幂等、不存在返回 404「巡检任务不存在」，复用 `response.RBool`）；前端 `openView` 调用后刷新列表。补齐 `ac_menu_api` + Casbin p 策略 + 契约/接口文档/README。实测：200 / 二次调用幂等 / 999999999 → 404 |
| 登录失败不弹提示 | `request.ts` 拦截器对 `/auth/login` 的 401 **不再整页跳转**（那会让用户只看到页面「闪一下」），改为正常 reject，登录页 catch 后 `message.error` 展示后端文案 |
| 菜单管理页 rc-table 弃用告警 | 见下 |

> **认知修正：菜单页告警的真因不是「嵌套 Table」。**
> 原先判断是 `expandedRowRender` 里嵌套了 `<Table>` 导致 rc-table 告警。实测（移除忽略清单后跑 99）
> 两条告警实为 `` `expandedRowRender` should not use with nested Table `` 与 `` `expandIconColumnIndex` is deprecated ``，
> **只把嵌套 `<Table>` 换掉告警依旧存在**；真因是 **dataSource 携带树形 `children` 与 `expandedRowRender` 同时使用**
> （后者由 antd 对 nest 型表格回填 `expandIconColumnIndex` 引起）。
> 最终做法：菜单树自行平铺（行上不带 `children`，改用受控 `expandedRowKeys` + level 缩进），
> 展开行内容改为原生 `<table class="e2e-menu-api-table">`，两条告警才真正消失。
> 另注：rc-util 的 `warningOnce` **每页只打一次**，原用例在 `page.goto` 之后才挂采集器会漏采首帧 —— 已改为先挂采集器再 `page.reload()`。

### 🧪 路由层与中间件层测试补齐（含一处长期误判的修正）

此前 305 个后端用例**全部集中在 Service 层**，路由与中间件没有测试覆盖 ——
而这恰恰是项目里出过真 bug 的地方。新增 `backend/tests/api/`（**12 个顶层 Test / 87 个子用例**）：

| 文件 | 覆盖 |
|:---|:---|
| `route_test.go` | 路由注册表、静态段 vs `:param` 优先级、**契约守门**（契约 §3 的 211 条端点与注册表双向零差异）、公开端点可达性 |
| `middleware_test.go` | Auth（4 种 401 分支 + gin.Context 7 个键 + Go Context 注入）、Casbin（放行/403/降级）、`NormalizeEmptyQuery`、ApiKeyAuth、OperationLog |
| `helpers_test.go` | router 构造与请求工具（纯路由/中间件用例**不依赖数据库**，7 个 Test 无 DB 环境变量也能跑，0.13s） |

**其中 Casbin 用例是安全回归防线**：构造「只授 `GET /a`」的主体，断言 `POST /a` 与 `GET /b` 均被拒、
`/web/devices/:id` 的授权不外溢到 `/web/devices/42/mode`。若 matcher 再退化为「只校验角色成员」，这些用例立刻失败。

> **认知修正：「静态路由必须先于 `:id` 注册」不是 gin 的硬约束。**
> 本项目文档与代码注释原先都断言「不按顺序就会被 `:id` 吞掉」。实测 gin **v1.12** 的路由树会把同级
> 静态子节点**规范化到 `:param` 之前，与注册先后无关** —— 先注册 `/probe/:id` 再注册 `/probe/modes`，
> 请求 `/probe/modes` 仍命中静态 handler，`Routes()` 输出中 `modes` 也排在 `:id` 前。
> 因此「按 `Routes()` 下标断言静态段在前」是一条**永不会失败的空转断言**（其价值仅在于 gin 升级改变行为时报警，
> 已由 `TestGinNormalizesSiblingOrder` 单独钉住）。
> 真正的防线是 `TestRoutePriority_StaticWinsOverParam`：**发真实请求、按响应特征判定命中的 handler**。
> 该用例已实证非空转 —— 临时移除 `wdg.GET("/modes", modeH.Page)` 注册后立即报
> `期望 200，实际 status=400 message="无效的设备ID"`，正是路由被参数分支吞掉的原始症状。
> 已据此修正 7 份文档与 `web.go` 的 8 处注释（保留「静态在前」的书写顺序，但改为说明其为可读性约定）。

### 🛡 基础数据引用保护补全

需求文档「02-基础信息模块 §5.2 引用保护流程」列出了三张字典表的完整引用清单，
但实现只覆盖了其中一部分 —— 删除一个**仅被低频表引用**的区域会成功，留下悬挂记录。

| 文件 | 变更 |
|:---|:---|
| `backend/internal/service/basic/region_service.go` | 删除改用**表驱动清单** `regionRefs`，9 张引用表逐条对应 §5.2；新增引用区域的功能时只需扩这张表 |
| `backend/internal/repository/region_repo.go` | 新增 `CountByColumn(ctx, table, column, id)` 通用计数（表名/列名为代码内常量，无注入面） |
| `backend/internal/service/basic/risk_type_service.go` | 补 `ac_risk_notify_setting` 引用检查（原只查风险记录与语音配置） |
| `backend/internal/repository/risk_type_repo.go` | 同上补 `CountByColumn` |

补全前后覆盖对比：

```
区域   补全前：ac_device, ac_risk_record                     （2/9）
       补全后：+ ac_alarm_record, ac_inspect_task, ac_inspect_record,
                 ac_iot_card, ac_voice_config,
                 ac_device_upgrade_plan, ac_device_upgrade_record   （9/9）

风险类型 补全前：ac_risk_record, ac_voice_config              （2/3）
       补全后：+ ac_risk_notify_setting                      （3/3）

告警类型 补全前/后：ac_alarm_record, ac_alarm_setting         （2/2，本就完整）
```

> **注意**：本项目对**被引用的基础数据**采取「拒绝删除 + 提示引用数量」而非静默软删除。
> 区域是设备、风险、告警、巡检的公共归属维度，允许删除会让大量记录悬挂在失效区域上、
> 使按区域筛选失效。确需清理时应先转移引用方，或改用停用（`active=false`）。
> 该策略以需求文档 §5.2 为准，测试用例文档原先描述的「软删除成功、不阻断」已按实现修正。

### 🔔 消息推送（多平台 WebHook）

| 文件 | 变更 |
|:---|:---|
| `backend/pkg/webhook/webhook.go` | 新增；`Send(ctx, cfg, msg)` 单入口按平台组装报文。飞书 `text`、钉钉 `markdown`+**HMAC-SHA256 加签**（`timestamp\nsecret` → base64 → urlencode 追加到 query）、企业微信 `markdown`、自定义平台输出通用 JSON（`event`/`event_label`/`title`/`content`/`fields`/`link`/`occurred_dt`）。三平台均校验 HTTP 200 外壳下的业务错误码（`code`/`StatusCode`/`errcode`） |
| `backend/internal/model/webhook.go` | 新增；`WebhookConfig` + 平台/事件/格式常量；`Subscribes(event)` 在内存精确匹配（避免 `LIKE 'risk%'` 误命中 `risky` 之类） |
| `backend/internal/repository/webhook_repo.go` | 新增；CRUD + `ListActiveByEvent`（先按 `active` 取回再内存过滤）+ `SaveSendResult`（记录 `last_send_dt`/`last_status`/`last_error`） |
| `backend/internal/service/setting/webhook_service.go` | 新增；配置 CRUD、`Test`（已保存配置）/`TestInline`（未保存即时校验）、`Dispatch`（按事件分发）、`Platforms`。`secret` 读取脱敏、更新时遇到空值或含 `****` 的串保持原值，避免把脱敏串写回库 |
| `backend/internal/api/handler/web/webhook_handler.go` | 新增；10 个端点（分页/详情/新增/编辑/删除/启用/停用/测试/即时测试/平台选项），`@Tags Setting` |
| `backend/internal/service/risk/risk_service.go`、`alarm_service.go` | 各在 `Report()` 落库后调用 `Dispatch`：风险与告警一旦上报即推送到订阅了 `risk`/`alarm` 的通道 |

> **注意**：推送失败**只**写入通道的 `last_status`/`last_error` 并打日志，**不改变上报接口的返回值** ——
> 设备上报链路不能因为外部群机器人的可用性而失败。

实测（测试环境 8081，真实接收端抓包）：设备上报离线告警 → ApiKey 认证 → 落库 →
接收端收到

```json
{"event":"alarm","event_label":"设备告警","title":"离线告警",
 "content":"设备 东门摄像头 于 2026-09-21 11:20:18 触发告警",
 "fields":{"设备名称":"东门摄像头","设备编号":"CAM_001","设备位置":"东门岗亭",
           "告警类型":"离线告警","告警等级":"high","处理状态":"未处理"},
 "occurred_dt":"2026-09-21 11:20:18"}
```

`casbin_rule` 的 p 策略由 192 条增至 **202 条**（新增 10 条 WebHook 接口授权）。

### 🖥 管理后台（React 19 + Ant Design 5 + Vite）

| 文件 | 变更 |
|:---|:---|
| `frontend/package.json` | 新增；`ai-camera@0.1.0`；React 19 + antd 5 + axios + dayjs + echarts(-for-react) + react-router-dom 7 + zustand 4；devDeps 含 electron 33 + electron-forge 7 + playwright + vitest 4 + msw 2 |
| `frontend/vite.config.ts` | 新增；管理后台构建，**base `/admin/`**，`define __IS_ELECTRON__: false`；dev server `:5173` 代理 `/web`、`/app`、`/uploads` 到 `VITE_BACKEND_PORT \|\| 8080` |
| `frontend/vite.website.config.ts` | 新增；官网构建，**base `/`**，入口 `website.html`，产物 `dist-website/`，不配 proxy（官网不依赖后端） |
| `frontend/vite.renderer.config.ts` / `vite.main.config.ts` / `vite.preload.config.ts` | 新增；Electron 主进程/预加载/渲染进程三份 Vite 配置 |
| `frontend/forge.config.ts` | 新增；Electron Forge 打包配置（NSIS/DMG/DEB/RPM maker + Vite 插件） |
| `frontend/src/admin/api/request.ts` | 新增；Axios 实例 + JWT 请求拦截器 + 统一错误处理 |
| `frontend/src/admin/api/generated/*Controller.ts` | 新增 20 个控制器；由 `scripts/api-gen.sh` 从 Swagger 生成，**严禁手动编辑**（`UserController`/`RoleController`/`MenuController`/`DeptController`/`PostController`/`ConfigController`/`OperationLogController`/`NotificationController`/`ProfileController`/`AuthWebController`/`RegionController`/`RiskTypeController`/`AlarmTypeController`/`DeviceController`/`RiskController`/`AlarmController`/`InspectController`/`UpgradeController`/`StatsController`/`BasicController`） |
| `frontend/src/admin/api/generated/definitions/*.ts` | 新增 205 个类型定义；含 `RPageXxx`（分页）与 `RXxxList`（简单列表）两类响应类型，前端禁止用 `as any` 绕过 |
| `frontend/src/admin/store/authStore.ts` | 新增；Zustand 认证 store（token/用户信息/权限码持久化） |
| `frontend/src/admin/components/PageContainer.tsx` | 新增；页面统一容器（标题/面包屑/操作区） |
| `frontend/src/admin/components/SearchBar.tsx` | 新增；列表页统一搜索栏 |
| `frontend/src/admin/components/ActionButtons.tsx` | 新增；表格操作列统一按钮组（编辑/删除/Popconfirm） |
| `frontend/src/admin/components/SelectorModal.tsx` | 新增；通用选择器弹窗，支持 `className` 透传（同时作用于 Modal 与触发按钮），是 E2E 定位的基础 |
| `frontend/src/admin/components/OptionSelect.tsx` | 新增；下拉选项组件，对接 `ROptionList` 简单列表响应 |
| `frontend/src/admin/components/RegionSelect.tsx` | 新增；区域选择器（复用 `OptionSelect` + 区域接口） |
| `frontend/src/admin/components/DynamicIcon.tsx` | 新增；菜单图标动态渲染（菜单树驱动侧边栏） |
| `frontend/src/admin/components/EmptyState.tsx` | 新增；空状态占位组件 |
| `frontend/src/shared/utils/format.ts` | 新增；日期/数字/文件大小等格式化工具（管理后台与官网共享） |
| `frontend/index.html` | 新增；管理后台入口（base `/admin/`） |
| `frontend/website.html` | 新增；官网入口（base `/`），与 `index.html` 同属一个 npm 工程 |
| `frontend/scripts/api-gen.sh` | 新增；抓取后端 `swagger.json` → `swagger2-doc-parser` 生成 Controller 与 definitions |
| `frontend/Dockerfile.website` / `nginx.website.conf` | 新增；官网静态站镜像与 Nginx 配置 |
| `frontend/playwright.config.ts` / `vitest.config.ts` | 新增；E2E 与单测配置（vitest 走 MSW mock，`VITE_API_BASE=http://localhost:8080`） |

> **注意**：`frontend/` 是**单工程双入口**。`npm run build` 依次执行 `build:admin` 与
> `build:website`，产出 `dist/`（base `/admin/`）与 `dist-website/`（base `/`）。两者的
> `base` 不同，**产物不可互换部署**，混用会导致静态资源 404。

### 🌐 官网介绍页（同工程第二入口）

| 文件 | 变更 |
|:---|:---|
| `frontend/src/website/main.tsx` | 新增；官网入口挂载 |
| `frontend/src/website/App.tsx` | 新增；官网组件装配（单页锚点结构） |
| `frontend/src/website/pages/Home.tsx` | 新增；官网首页，按区块组装各 Section |
| `frontend/src/website/components/Header.tsx` | 新增；顶部导航（含锚点跳转与登录入口） |
| `frontend/src/website/components/HeroSection.tsx` | 新增；首屏主视觉（深色科技风，主色 `#2b7fff` + 点缀 `#22d3ee`） |
| `frontend/src/website/components/CapabilitySection.tsx` | 新增；能力概览（AI 识别/视频监控/设备运维/风险闭环） |
| `frontend/src/website/components/SolutionSection.tsx` | 新增；解决方案介绍 |
| `frontend/src/website/components/ScenarioSection.tsx` | 新增；应用场景（工地/厂区/仓库/园区） |
| `frontend/src/website/components/StatsSection.tsx` | 新增；平台数据亮点（数字滚动展示） |
| `frontend/src/website/components/ContactSection.tsx` | 新增；联系我们 |
| `frontend/src/website/components/Footer.tsx` | 新增；页脚 |
| `frontend/src/website/index.css` | 新增；官网样式（纯 CSS 变量主题，视觉效果全部用渐变/SVG，**不依赖位图资源**） |

### 📱 移动端（Flutter）

| 文件 | 变更 |
|:---|:---|
| `app/pubspec.yaml` | 新增；`ai_camera_app`，SDK ^3.9.0；依赖 `go_router` 14 + `dio` 5 + `shared_preferences` 2 + `intl` + `flutter_dva` + `fluttertoast`；devDeps 含 `mockito` |
| `app/lib/theme/app_colors.dart` | 新增；移动端配色常量（与官网/管理后台同一套主色） |
| `app/lib/theme/app_spacing.dart` | 新增；间距规范常量 |
| `app/lib/theme/app_text.dart` | 新增；字号与字重规范常量 |
| `app/lib/layout/Gaps.dart` | 新增；布局间隙组件 |

> **注意**：App 页面与路由的完整清单见 [v0.1.0 软件开发计划.md](v0.1.0%20基础功能开发/v0.1.0-软件开发计划.md)
> 第七节与 [待完成事项.md](v0.1.0%20基础功能开发/待完成事项.md)。

### 📚 文档体系

| 文件 | 变更 |
|:---|:---|
| `README.md` | 新增；仓库根说明（产品简介、系统架构、仓库结构、快速开始、文档索引） |
| `backend/README.md` | 新增；后端技术栈、项目结构、快速开始、**209 个 API 路由总览**、权限控制链路、响应类型命名规范、测试 |
| `frontend/README.md` | 新增；三端构建、双入口构建说明、API 代码生成、环境检测、测试 |
| `app/README.md` | 新增；移动端技术栈、项目结构、页面清单、认证流程、接口约定 |
| `CLAUDE.md` | 新增；Claude Code 开发指引（架构、鉴权体系、开发规范、E2E 规范、测试流程） |
| `docs/00-系统设计总览.md` | 新增；系统架构、技术选型、模块划分与整体设计思路 |
| `docs/需求文档/00-需求总览.md` | 新增；产品定位、目标用户、系统边界、模块列表（37 表）、模块依赖、核心业务流程、全局状态常量、权限矩阵、非功能需求 |
| `docs/需求文档/01-系统管理模块.md` | 新增（978 行）；个人中心、用户、菜单、角色、部门、岗位、操作日志、系统通知 |
| `docs/需求文档/02-基础信息模块.md` | 新增（402 行）；区域管理、风险类型管理、告警类型管理 |
| `docs/需求文档/03-资源管理模块.md` | 新增（735 行）；设备管理、语音配置、物联网卡、换卡记录、设备模式 |
| `docs/需求文档/04~10-*.md` | 新增 7 份；风险监控、设备监控、设备告警、实时监控、巡检中心、统计分析、首页与告警设置 |
| `docs/开发文档/后端开发/00-接口契约.md` | 新增（788 行）；**v0.1.0 冻结版契约**，含 200 条路由总表、Handler 装配清单、Service 方法签名、响应/请求类型清单 |
| `docs/开发文档/数据库设计/sql/ddl/*.sql` | 新增 6 个 DDL 脚本；01-系统基础(16表)/02-基础信息(3表)/03-资源管理(6表)/04-风险与告警(2表)/05-巡检(2表)/07-告警设置(3表)，共 35 张 `CREATE TABLE`（另 2 张由模型迁移补充） |
| `docs/开发文档/页面功能和接口/00-页面功能与接口总览.md` | 新增；**48 个页面目录**（39 个 Web 页面 + 9 个移动端页面）与统一响应格式约定 |
| `docs/开发文档/页面功能和接口/01~12-*.md` | 新增 12 份；登录与个人中心、系统管理、基础信息、资源管理、首页看板、实时监控、AI风险与AI监控、风险统计、巡检中心、统计分析、升级管理与告警设置、APP移动端 |
| `docs/测试文档/测试规范/00~05-*.md` | 新增 6 份；测试流程、单元测试、集成测试、接口测试、回归测试、端到端测试 |
| `docs/测试文档/测试用例/00~09-*.md` | 新增 10 份；用例总览 + 认证与权限 / 系统管理 / 基础信息 / 资源管理 / 风险与告警 / 巡检 / 升级管理 / 告警设置与统计 / 冒烟测试 |
| `docs/部署文档/01-部署与运维.md` | 新增；部署架构、已部署基础设施、Docker 镜像构建、K8s 清单、CI/CD、监控、备份、运维命令 |
| `docs/部署文档/02-监控与容灾.md` | 新增；监控分层、Prometheus 配置与业务指标清单、AlertManager 分级、Grafana 看板、容灾目标与演练 |
| `docs/敏捷开发/CHANGE_LOG.md` | 新增；本文件 |
| `docs/敏捷开发/v0.1.0 基础功能开发/v0.1.0-软件开发计划.md` | 新增；阶段划分、进度总览、四层代码状态、三端开发计划、接口代码生成规范、API 路由规划、文件结构 |
| `docs/敏捷开发/v0.1.0 基础功能开发/v0.1.0-软件测试计划.md` | 新增；测试体系架构、测试环境、五个测试层次、当前执行状态、产出物 |
| `docs/敏捷开发/v0.1.0 基础功能开发/v0.1.0-开发总结.md` | 新增；交付范围、分层统计、鉴权实现要点、与 boticz-mes 的差异与改进 |
| `docs/敏捷开发/v0.1.0 基础功能开发/待完成事项.md` | 新增；按 P0~P3 分级的真实待完成事项 |
| `docs/介绍网站/README.md` | 新增；官网技术栈、页面结构、设计风格、视觉实现方案、构建部署 |

### ⚠️ 已知限制

| 限制 | 影响 | 后续计划 |
|:---|:---|:---|
| **视频流为占位播放器** | 实时监控页的单屏/4 宫格/9 宫格可正常渲染与切换，但播放的是占位组件，未接入真实 RTSP/WebRTC 流；`ac_device.stream_url` 字段已预留 | v0.2.0 接入流媒体网关 |
| **文件存储为本地 `uploads/`** | 风险图片、录像、音频、固件、模型文件落 K8s PVC，存在单点、扩容僵化、多副本挂载受限三个问题 | v0.2.0 迁移对象存储（方案见 [02-监控与容灾.md](../部署文档/02-监控与容灾.md) 5.5 节） |
| **个推/短信未真实下发** | 配置读写接口与数据表已就绪，实际推送调用为桩实现；站内通知（`ac_notification`）正常写入 | v0.2.0 接入个推与腾讯云短信 SDK |
| **大屏数据非实时推送** | 可视化大屏通过轮询拉取 `/web/stats/screen`，无 WebSocket 推送 | v0.2.0 引入 WebSocket |
| **物联网卡流量为平台侧记录** | 未与第三方物联网卡平台对账，平台值与运营商权威值可能不一致 | v0.2.0 实现对账任务 |
| **App 相机拍照上传未接入** | 巡检与风险处理的结果图片需从相册选取，未接入 `camera` 插件现场拍照 | v0.2.0 接入 |
| **E2E 冒烟脚本未补齐** | `frontend/e2e/` 与 `frontend/e2e/smoke/` 的用例框架已定义（含 e2e-modal class 规范），用例覆盖度不足 | v0.2.0 补齐 01a~10f + 99 全页面覆盖 |
| **Redis 未接入业务** | `config` 中已配置 Redis 但代码未使用；令牌黑名单、验证码、分布式锁、WebSocket 会话均未实现 | v0.2.0 按需接入 |
| **桌面端自动更新未实现** | Electron 打包链路（`make:win/mac/linux`）可用，但无版本自检与增量更新 | v0.2.0 |

### 📊 变更统计

| 维度 | 数量 | 说明 |
|:---|:---:|:---|
| 后端 Go 文件 | 133 | `cmd/` 1 + `config/` 2 + `internal/` 126 + `pkg/` 6（不含测试与脚本） |
| 后端 Go 代码行数 | 22,411 | `find backend -name '*.go' \| xargs wc -l` |
| 数据模型文件 | 8 | `base`/`system`/`basic`/`device`/`risk`/`inspect`/`upgrade`/`notify` |
| 业务表（模型） | **37** | 全部 `ac_` 前缀；DDL 脚本 35 张 `CREATE TABLE` |
| Repository 文件 | 22 | 数据访问层 |
| Service 文件 | 24 | 分布在 10 个子目录（system/basic/device/risk/alarm/inspect/upgrade/setting/stats/app） |
| Handler 文件 | 24 | `handler/web` 22 + `handler/app` 2 |
| 中间件 | 3 | Auth + CasbinMiddleware、ApiKeyAuth、OperationLog |
| 请求 DTO 文件 | 17 | `internal/dto/request/` |
| 响应 DTO 文件 | 21 | `internal/dto/response/` |
| **API 端点** | **200** | `/web/*` 178 + `/app/*` 20 + `/health`（契约文档路由总表） |
| 前端生成的 Controller | 20 | 由 Swagger 自动生成，禁止手写 |
| 前端生成的类型定义 | 205 | definitions 下 `.ts` 文件数 |
| 管理后台页面 | **39** | 见页面目录 `01`~`11` 分组 |
| 移动端页面 | **9** | 登录/首页/待办/风险/告警/巡检/设备/通知/我的 |
| 官网组件 | 9 | Header + 7 个 Section + Footer |
| 文档文件 | **50 份 + 4 个 README** | `docs/` 下 50 份 Markdown（需求 11 + 开发 14 + 测试 16 + 部署 2 + 敏捷 5 + 介绍网站 1 + 系统设计总览 1），另加根/后端/前端/移动端 4 个 README |

---

## 版本规划（后续）

| 版本 | 主题 | 核心内容 |
|:---|:---|:---|
| v0.2.0 | 真实接入与工程化补齐 | 视频流接入、对象存储迁移、个推/短信真实下发、WebSocket 实时推送、物联网卡对账、E2E 冒烟补齐 |
| v0.3.0 | AI 能力增强 | 多模型版本灰度、置信度阈值在线调优、风险趋势预测 |
| v0.4.0 | 规模化运维 | 多租户隔离、设备分组批量运维、跨区域大屏聚合 |

> **说明**：后续版本规划仅为方向性指引，具体范围以对应版本的《软件开发计划》为准。
