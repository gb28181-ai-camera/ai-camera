# AI Camera 后端服务

Go 后端服务：Gin + GORM + PostgreSQL + Casbin，按 `Handler → Service → Repository → Model`
四层架构实现 AI Camera 智能监控平台的全部业务能力；v0.2.0 增加 GB/T 28181 信令入口
（`cmd/sipsvc`）与 ZLMediaKit 媒体适配；v0.3.0 增加 IOT 产品/服务组/设备接入、
MQTT 鉴权与 WebRTC 信令、TURN/STUN 凭据签发；v0.4.0 对齐 WVP-GB28181-Pro 的
设备配置/控制、媒体指标与流会话、日志 SSE、分享播放、录像合并等运维能力。

> 当前版本: **v0.5.1** | 最后更新: 2026-10-25

---

## 技术栈

| 项目 | 选择 | 说明 |
|:---|:---|:---|
| 语言 | Go 1.26 | `go.mod` 中 `go 1.26` |
| Web 框架 | Gin 1.12 | 路由分组 + 中间件链 |
| ORM | GORM 1.25 | `gorm.io/gorm` + `gorm.io/driver/postgres` |
| 数据库 | PostgreSQL 16 | 业务库 `ai_camera_dev` / `ai_camera_test` |
| 权限 | Casbin 2.135 + gorm-adapter 3.0 | `g`/`g2`/`g3` 三链角色继承，策略存 `casbin_rule` 表 |
| 认证 | golang-jwt/jwt v5 | HS256，claims 携带 `data_perm` / `dept_id` |
| 配置 | Viper 1.21 | YAML 加载，`CONFIG_PATH` 环境变量切换环境 |
| 日志 | zap 1.28 + lumberjack 2.2 | 支持 `json` / `console` 两种格式，按大小/天数滚动 |
| 定时任务 | robfig/cron v3 | 已引入依赖，调度器装配见「待完成事项」 |
| 接口文档 | swag v2 | `swag init` 生成 `docs/web` 与 `docs/app` 两份 Swagger JSON |
| 密码哈希 | golang.org/x/crypto | bcrypt |
| API 文档 UI | Swagger UI（静态页） | `static/swagger-ui/index.html` |

---

## 项目结构

```
backend/
├── cmd/
│   ├── server/
│   │   └── main.go                  # 入口：router 装配 + HTTP 启停 + 优雅退出（10s）
│   └── sipsvc/
│       └── main.go                  # GB28181 SIP 信令入口（副本固定 1；可 embedded 内嵌）
├── config/
│   ├── config.go                    # 配置结构体 + viper 加载 + DSN 拼接
│   ├── config.yaml                  # 开发环境配置（gitignored，端口 8080）
│   ├── config-test.yaml             # 测试环境配置（gitignored，端口 8081）
│   └── rbac_model.conf              # Casbin 模型：g/g2/g3 + keyMatch2 matcher
├── internal/
│   ├── bootstrap/
│   │   ├── main.go                  # init() 装配：配置→日志→DB→迁移→JWT→Casbin
│   │   └── migrate.go               # AutoMigrate(37 models...) 一次性 fail-fast
│   ├── api/
│   │   ├── router/
│   │   │   ├── router.go            # SetupRouter()：Engine + Recovery + 操作日志 + /health
│   │   │   ├── web.go               # /web 基础路由 + 国标设备/通道/媒体/分组/日志
│   │   │   ├── gb_live.go           # 国标播放 / 云台 / 语音路由（16 条）
│   │   │   ├── gb_ops.go            # 国标回放/录像/报警/位置/流/级联/语音 Agent（共 49 条）
│   │   │   ├── media.go             # ZLM 10 个回调（独立组，无 JWT/Casbin）
│   │   │   ├── app.go               # /app 移动端路由（含 /app/gb 13 条）
│   │   │   └── swagger.go           # Swagger UI + 两份 JSON 的静态文件服务
│   │   ├── handler/
│   │   │   ├── web/                 # 46 个管理端 Handler（含 gb_* / voice_agent / iot_*）
│   │   │   ├── app/                 # 4 个移动端 Handler（auth / app / gb / iot）
│   │   │   ├── iot/                 # 设备端 /iot/ota、/iot/activate
│   │   │   └── rpc/                 # MQTT 鉴权 / TURN 凭据内部 RPC
│   │   └── middleware/
│   │       ├── auth.go              # AuthMiddleware + CasbinMiddleware
│   │       ├── api_key_auth.go      # ApiKeyAuth（X-Api-Key → SHA256 → ac_api_key）
│   │       └── operation_log.go     # 增删改操作写入 ac_operation_log
│   ├── gb28181/                     # ★ 国标信令与媒体适配（v0.2.0）
│   │   ├── sipserver/               # sipgo 封装、请求分发、INVITE、SDP 构造
│   │   ├── protocol/manscdp/        # MANSCDP XML 编解码（Catalog/Alarm/PTZ/RecordInfo）
│   │   ├── protocol/sdp/            # 点播 / 回放 / 语音 SDP
│   │   ├── auth/                    # 白名单 + 服务端 Digest + 无状态 nonce
│   │   ├── device/                  # 注册 / 心跳 / 目录重组 / 过期扫描 / 信令日志
│   │   ├── session/                 # INVITE Dialog（ACK / BYE / INFO）
│   │   ├── ssrc/                    # 10 位 SSRC 分配
│   │   ├── mediaserver/             # ZLMediaKit REST 客户端 + 播放地址
│   │   ├── hook/                    # ZLM 10 个回调 + secret 校验 + 活跃流
│   │   ├── play/                    # 点播 / 回放编排
│   │   └── control/                 # HTTP 层 ↔ SIP 实现的接口（Default/Set）
│   ├── service/                     # 45 个 Service 文件（11 个领域子目录）
│   │   ├── system/                  # 认证/个人中心/用户/角色/菜单/部门/岗位/配置/日志/通知
│   │   ├── basic/                   # 区域 / 风险类型 / 告警类型
│   │   ├── device/                  # 设备 / 设备模式 / 物联网卡 / 语音配置
│   │   ├── risk/                    # 风险记录（含设备上报与处理闭环）
│   │   ├── alarm/                   # 告警记录
│   │   ├── inspect/                 # 巡检任务与记录
│   │   ├── upgrade/                 # 固件类型 / 固件 / AI 模型 / 升级下发 / 计划 / 记录
│   │   ├── setting/                 # 告警设置 / 风险通知设置 / 个推 / 短信 / 消息推送
│   │   ├── stats/                   # 看板 / 风险统计 / 告警统计 / 大屏 / 流量 / 设备状态
│   │   ├── gb28181/                 # 国标设备/通道/播放/云台/回放/录像/报警/位置/流/级联
│   │   ├── iot/                     # 产品 / 服务组 / 设备账号 / OTA / 分享码 / 凭据
│   │   ├── turn/                    # TURN REST 凭据签发
│   │   └── app/                     # 移动端全部业务（含 /app/gb、/app/iot 薄封装）
│   ├── signal/mqtt/                 # MQTT ↔ ZLM WebRTC SDP 信令桥
│   ├── mqttrpc/                     # mochi-mqtt gRPC 鉴权服务
│   ├── repository/                  # 30 个数据访问文件
│   ├── model/                       # 11 个模型文件 / 63 张表（ac_ 前缀）
│   │   ├── base.go                  # BaseModel + Context 注入 + 审计字段钩子
│   │   ├── system.go                # 16 张系统管理表
│   │   ├── basic.go                 # 3 张基础信息表
│   │   ├── device.go                # 6 张资源管理表
│   │   ├── risk.go                  # 2 张风险与告警表
│   │   ├── inspect.go               # 2 张巡检表
│   │   ├── upgrade.go               # 5 张升级管理表
│   │   ├── notify.go                # 3 张告警设置表
│   │   ├── webhook.go               # 1 张消息推送表（ac_webhook_config）
│   │   ├── gb28181.go               # 18 张国标视频表（通道/媒体/录像/报警/级联/语音）
│   │   └── iot.go                   # 7 张 IOT 表（产品/服务组/服务/产品-组/产品-服务/账号/事件）
│   └── dto/
│       ├── request/                 # 21 个请求结构体文件
│       └── response/                # 26 个响应结构体文件
├── pkg/
│   ├── auth/jwt.go                  # JWT 签发与解析
│   ├── database/postgres.go         # GORM 连接与连接池
│   ├── perm/casbin.go               # Casbin 封装 + 策略自动重载
│   └── utils/
│       ├── response.go              # 统一 {code, message, data} 响应
│       ├── data_scope.go            # 行级数据权限
│       └── logger.go                # zap + lumberjack
├── scripts/
│   ├── bashrc.sh                    # 环境变量辅助
│   ├── gen-swagger.sh               # 生成 docs/web + docs/app 两份 Swagger
│   ├── build.sh                     # docker build + push（--prod 切生产仓库）
│   └── publish.sh                   # kubectl rollout restart（--prod 切生产集群）
├── static/
│   └── swagger-ui/index.html        # Swagger UI 页面
├── tools/
│   └── gbsim/                       # ★ GB28181 智能相机模拟器（测试夹具，非生产代码）
│       ├── main.go                  # CLI：REGISTER / Keepalive / Catalog / INVITE / RTP
│       ├── sipua/                   # SIP UA、Digest、INVITE 会话
│       ├── manscdp/                 # 模拟器侧 MANSCDP 编解码
│       ├── ps/ rtp/ sdp/            # PS 封装、RTP 分包、SDP 解析
│       └── testdata/sample.h264     # H.264 Annex-B 样本
├── tests/
│   ├── gb28181/                     # 国标服务层测试（31 个）
│   └── testutil/
│       ├── testutil.go              # WithTx(t, fn)：事务隔离 + 自动回滚
│       ├── resetdb/main.go          # 清空测试库（跑测试前必做）
│       └── initdb/main.go           # 初始化测试基础数据（冒烟测试前必做）
├── docs/                            # Swagger 生成物（gitignored）
│   ├── web/swagger.json
│   └── app/swagger.json
├── go.mod / go.sum
└── README.md                        # 本文件
```

---

## 快速开始

### 1. 配置数据库

`config/config.yaml` 与 `config/config-test.yaml` 均已被 `.gitignore` 忽略（含明文口令），
需按下列骨架自行创建：

```yaml
# backend/config/config.yaml
server:
  port: 8080
  mode: debug          # debug / release / test
  read_timeout: 10s
  write_timeout: 10s

database:
  host: 192.168.1.227
  port: 5432
  user: boticz
  password: "<测试库密码>"
  dbname: ai_camera_dev
  sslmode: disable
  max_idle_conns: 10
  max_open_conns: 100
  conn_max_lifetime: 1h

redis:                 # 预留：验证码、令牌黑名单、实时推送
  host: 192.168.1.227
  port: 6379
  password: ""
  db: 0
  pool_size: 10

jwt:
  secret: "ai-camera-jwt-secret-key-2026"
  expire_hours: 7200

casbin:
  model_path: config/rbac_model.conf
  auto_load_interval: 5m      # 多副本部署下策略最终一致的同步间隔

media:
  upload_dir: ./uploads
  base_url: "http://localhost:8080/uploads"

thirdparty:
  getui:
    app_id: ""
    app_key: ""
    app_secret: ""
    master_secret: ""
  sms:
    secret_id: ""
    secret_key: ""
    sdk_app_id: ""
    sign_name: ""

swagger:
  enabled: true

log:
  level: info
  format: json          # json / console
  output: ./logs/app.log
  max_size: 100
  max_backups: 30
  max_age: 7
```

测试环境（`config-test.yaml`）差异：`server.port: 8081`、`database.dbname: ai_camera_test`、
`media.upload_dir: ./uploads-test`、`log.format: console`（便于冒烟排查）。

> **注意**：表结构由两部分维护 —— 初次建表执行 `docs/开发文档/数据库设计/sql/ddl/*.sql`；
> 增量（新增表/新增列/列类型变更）由 `bootstrap.AutoMigrate()` 在服务启动时完成。
> `AutoMigrate` 采用**一次性 fail-fast**：37 个模型中任一迁移失败即中止启动，
> 避免逐表循环静默跳过导致表结构漂移。

### 2. 启动服务

```bash
# 开发环境（默认 8080，读 config/config.yaml）
cd backend && go run ./cmd/server/main.go

# 测试环境（8081，读 config-test.yaml）
export CONFIG_PATH="config/config-test.yaml" && cd backend && go run ./cmd/server/main.go

# 编译为单文件二进制
cd backend && go build -o server ./cmd/server/main.go
```

`CONFIG_PATH` 是切换配置的**唯一开关**（缺省 `config/config.yaml`）。启动顺序由
`internal/bootstrap` 的 `init()` 完成：加载配置 → 初始化日志 → 连接数据库 → 执行迁移 →
初始化 JWT → 初始化 Casbin。

> **注意**：Casbin 初始化失败**不会**阻塞启动 —— `globalCasbin` 置为 `nil`，路由层退化为
> 放行中间件并打印「⚠️ 严重警告: Casbin初始化失败，所有接口权限校验将被跳过」。
> 生产环境必须通过日志告警与健康检查及时发现该状态。

### 3. 验证启动

```bash
# 健康检查（内联端点，不经过任何鉴权中间件）
curl -s http://localhost:8080/health
# → {"code":200,"data":{"status":"ok"}}

# 登录获取 token
curl -s -X POST http://localhost:8080/web/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"<口令>"}'
# → {"code":200,"data":{"token":"eyJ...","user_id":1,"data_perm":"all",...}}

# 带 token 调用受保护接口
curl -s 'http://localhost:8080/web/devices?page=1&page_size=10' \
  -H 'Authorization: Bearer eyJ...'
# → {"code":200,"data":{"items":[...],"total":0,"page":1,"page_size":10}}
```

### 4. 查看 API 文档

```bash
# 1. 生成 Swagger JSON（首次必须执行）
cd backend && bash scripts/gen-swagger.sh
#   → docs/web/swagger.json（管理端）
#   → docs/app/swagger.json（移动端）

# 2. 启动服务后访问
#   http://localhost:8080/swagger-ui              可视化文档
#   http://localhost:8080/web/api-docs.json       管理端 Swagger JSON
#   http://localhost:8080/app/api-docs.json       移动端 Swagger JSON
```

> **注意**：`docs/web/` 与 `docs/app/` 为生成物且已 gitignore。未执行 `gen-swagger.sh` 时，
> 上述三个路径返回 404，但**服务仍可正常启动**（`router/swagger.go` 使用 `StaticFile`
> 而非编译期嵌入，保证未生成文档时不影响启动）。

---

## API 路由总览

后端 Swagger 共收录 **418 个端点**（v0.4.0 实测：Web 377 + App 41）：

| 分组 | 数量 | 说明 |
|:---|:---:|:---|
| `/web/*` | 377 | 管理后台 + 官网；含国标与 IOT 接口 |
| `/app/*` | 41 | 移动端；含 `/app/gb/*` 与 `/app/iot/*` |
| `/media/hook/*` | 10 | ZLM 回调（独立中间件组） |

另有未纳入 Swagger 的 **设备端 `/iot/*`（2 条）**、**内部 RPC `/rpc/*`（3 条）** 与 `/health`。

其中 v0.1.0 的端点以 `docs/开发文档/后端开发/00-接口契约.md` 第 3 节为准，
v0.2.0 的国标端点以第 10 节为准，v0.3.0 的 IOT 端点以 §10.14–§10.16 为准，
v0.4.0 的 WVP 对齐端点以 §17 为准。

入口文件：[`internal/api/router/web.go`](internal/api/router/web.go)、[`internal/api/router/app.go`](internal/api/router/app.go)、
[`internal/api/router/gb_live.go`](internal/api/router/gb_live.go)、[`internal/api/router/gb_ops.go`](internal/api/router/gb_ops.go)、
[`internal/api/router/media.go`](internal/api/router/media.go)

### 1. 公开接口

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| POST | `/web/auth/login` | 管理后台登录（返回 `LoginData`，含 token / data_perm / permissions） |
| POST | `/app/auth/login` | 移动端登录（返回 `menu.platform = mobile` 的菜单） |
| GET | `/health` | 健康检查（内联实现，不经过鉴权中间件） |

### 2. 设备上报接口（ApiKey 认证）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| POST | `/web/risks/report` | 设备上报 AI 风险（落库 `status=pending`、`read=false`） |
| POST | `/web/alarms/report` | 设备上报告警（离线 / 电量 / 流量） |
| POST | `/web/devices/:id/heartbeat` | 设备心跳上报（刷新在线状态/电量/电压/存储 + 写历史采样） |

> **注意**：该分组使用 `X-Api-Key` 请求头（SHA256 后匹配 `ac_api_key.key_hash`，且
> `active = true`），**不经过 Casbin 校验**。请求成功会更新 `ac_api_key.last_used_dt`
> 的时间；该更新失败不影响请求返回。

### 3. 认证与个人中心（前缀 `/web`）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/profile` | 获取当前登录用户资料 |
| PUT | `/web/profile` | 修改当前用户资料 |
| PUT | `/web/profile/password` | 修改当前用户密码 |
| GET | `/web/profile/menus` | 获取当前用户的菜单树（按角色过滤，含祖先目录） |
| GET | `/web/profile/api-keys` | 列出当前用户的 API Key |
| POST | `/web/profile/api-keys` | 生成新 API Key（返回明文，仅此一次） |
| DELETE | `/web/profile/api-keys/:id` | 删除 API Key |

### 4. 系统管理（前缀 `/web/system`）

**用户**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/users` | 用户分页列表 |
| GET | `/web/system/users/options` | 用户下拉选项 |
| GET | `/web/system/users/:id` | 用户详情 |
| POST | `/web/system/users` | 新建用户 |
| PUT | `/web/system/users/:id` | 更新用户 |
| DELETE | `/web/system/users/:id` | 删除用户 |
| PUT | `/web/system/users/:id/enable` | 启用用户 |
| PUT | `/web/system/users/:id/disable` | 停用用户 |
| PUT | `/web/system/users/:id/reset-password` | 重置用户密码 |

**角色**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/roles` | 角色分页列表 |
| GET | `/web/system/roles/options` | 角色下拉选项 |
| GET | `/web/system/roles/:id` | 角色详情 |
| POST | `/web/system/roles` | 新建角色 |
| PUT | `/web/system/roles/:id` | 更新角色 |
| DELETE | `/web/system/roles/:id` | 删除角色 |
| GET | `/web/system/roles/:id/menus` | 查询角色已授权的菜单 ID 列表 |
| PUT | `/web/system/roles/:id/menus` | 分配角色菜单（触发 Casbin 策略重建） |
| GET | `/web/system/roles/:id/depts` | 查询角色关联的部门 ID 列表 |
| PUT | `/web/system/roles/:id/depts` | 分配角色部门（写入 Casbin `g2`） |
| GET | `/web/system/roles/:id/posts` | 查询角色关联的岗位 ID 列表 |
| PUT | `/web/system/roles/:id/posts` | 分配角色岗位（写入 Casbin `g3`） |
| GET | `/web/system/roles/:id/users` | 查询角色关联的用户 ID 列表 |
| PUT | `/web/system/roles/:id/users` | 分配角色用户（写入 Casbin `g`） |

**菜单**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/menus` | 菜单树（按 platform 过滤：web / mobile） |
| GET | `/web/system/menus/:id` | 菜单详情 |
| POST | `/web/system/menus` | 新建菜单 |
| PUT | `/web/system/menus/:id` | 更新菜单 |
| DELETE | `/web/system/menus/:id` | 删除菜单 |
| GET | `/web/system/menus/:id/apis` | 查询菜单登记的接口列表 |
| POST | `/web/system/menus/:id/apis` | 为菜单登记接口（Casbin `p` 策略来源） |
| DELETE | `/web/system/menus/apis/:apiId` | 删除菜单接口登记 |

**部门**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/depts` | 部门树 |
| GET | `/web/system/depts/options` | 部门下拉选项 |
| GET | `/web/system/depts/:id` | 部门详情 |
| POST | `/web/system/depts` | 新建部门 |
| PUT | `/web/system/depts/:id` | 更新部门 |
| DELETE | `/web/system/depts/:id` | 删除部门 |

**岗位**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/posts` | 岗位分页列表 |
| GET | `/web/system/posts/options` | 岗位下拉选项 |
| GET | `/web/system/posts/:id` | 岗位详情 |
| POST | `/web/system/posts` | 新建岗位 |
| PUT | `/web/system/posts/:id` | 更新岗位 |
| DELETE | `/web/system/posts/:id` | 删除岗位 |

**系统配置**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/configs` | 配置列表（支持 keyword 过滤） |
| POST | `/web/system/configs` | 新建配置 |
| PUT | `/web/system/configs/:id` | 更新配置 |
| DELETE | `/web/system/configs/:id` | 删除配置 |
| PUT | `/web/system/configs/:id/enable` | 启用配置 |
| PUT | `/web/system/configs/:id/disable` | 停用配置 |

**操作日志**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/logs` | 操作日志分页列表 |
| GET | `/web/system/logs/:id` | 操作日志详情 |
| DELETE | `/web/system/logs/:id` | 删除日志 |
| DELETE | `/web/system/logs/clear` | 清空日志 |
| GET | `/web/system/logs/export` | 导出日志（xlsx） |

**系统通知**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/system/notifications` | 通知分页列表（仅当前用户） |
| GET | `/web/system/notifications/unread-count` | 未读通知数量（红点） |
| PUT | `/web/system/notifications/:id/read` | 标记单条已读 |
| PUT | `/web/system/notifications/read-all` | 全部标记已读 |
| DELETE | `/web/system/notifications/:id` | 删除通知 |

### 5. 基础信息（前缀 `/web/basic`）

**区域**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/basic/regions` | 区域分页列表 |
| GET | `/web/basic/regions/options` | 区域下拉选项 |
| GET | `/web/basic/regions/:id` | 区域详情 |
| POST | `/web/basic/regions` | 新建区域 |
| PUT | `/web/basic/regions/:id` | 更新区域 |
| DELETE | `/web/basic/regions/:id` | 删除区域 |

**风险类型**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/basic/risk-types` | 风险类型分页列表 |
| GET | `/web/basic/risk-types/options` | 风险类型下拉选项 |
| GET | `/web/basic/risk-types/:id` | 风险类型详情 |
| POST | `/web/basic/risk-types` | 新建风险类型 |
| PUT | `/web/basic/risk-types/:id` | 更新风险类型 |
| DELETE | `/web/basic/risk-types/:id` | 删除风险类型 |

**告警类型**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/basic/alarm-types` | 告警类型分页列表 |
| GET | `/web/basic/alarm-types/options` | 告警类型下拉选项 |
| GET | `/web/basic/alarm-types/:id` | 告警类型详情 |
| POST | `/web/basic/alarm-types` | 新建告警类型 |
| PUT | `/web/basic/alarm-types/:id` | 更新告警类型 |
| DELETE | `/web/basic/alarm-types/:id` | 删除告警类型 |

### 6. 资源管理（前缀 `/web/devices`）

**设备**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/devices` | 设备分页列表（含关联的区域名称、物联网卡、固件、模型） |
| GET | `/web/devices/options` | 设备下拉选项 |
| GET | `/web/devices/history` | 设备历史记录分页列表（时序采样） |
| GET | `/web/devices/:id` | 设备详情 |
| GET | `/web/devices/:id/history-chart` | 设备历史曲线数据（近 3 日） |
| POST | `/web/devices` | 新建设备 |
| PUT | `/web/devices/:id` | 更新设备 |
| DELETE | `/web/devices/:id` | 删除设备 |
| PUT | `/web/devices/:id/mode` | 切换设备工作模式 |
| PUT | `/web/devices/:id/sync-time` | 同步设备时间 |
| PUT | `/web/devices/:id/monitor` | 加入 / 移出实时监控 |

**设备模式**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/devices/modes` | 设备模式分页列表 |
| GET | `/web/devices/modes/options` | 设备模式下拉选项 |
| POST | `/web/devices/modes` | 新建设备模式 |
| PUT | `/web/devices/modes/:id` | 更新设备模式 |
| DELETE | `/web/devices/modes/:id` | 删除设备模式 |

**物联网卡**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/devices/iot-cards` | 物联网卡分页列表 |
| GET | `/web/devices/iot-cards/options` | 物联网卡下拉选项 |
| POST | `/web/devices/iot-cards` | 新建物联网卡 |
| PUT | `/web/devices/iot-cards/:id` | 更新物联网卡 |
| DELETE | `/web/devices/iot-cards/:id` | 删除物联网卡 |
| GET | `/web/devices/iot-card-replace-logs` | 换卡记录分页列表 |
| POST | `/web/devices/iot-card-replace-logs` | 执行换卡（更新设备 + 记录新旧卡号） |

**语音配置**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/devices/voice-configs` | 语音配置分页列表 |
| POST | `/web/devices/voice-configs` | 新建语音配置 |
| PUT | `/web/devices/voice-configs/:id` | 更新语音配置 |
| DELETE | `/web/devices/voice-configs/:id` | 删除语音配置 |

> **注意**：`/modes`、`/iot-cards`、`/iot-card-replace-logs`、`/voice-configs`、`/history`
> 与 `/:id` **同级**。路由注册时**必须**把静态子路径写在 `/:id` 之前，否则
> `/web/devices/modes` 会被 `/:id` 吞掉（`id` 收到字符串 `"modes"`，查询报错或返回空）。

### 7. 风险监控（前缀 `/web/risks`）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/risks` | 风险分页列表（含风险类型名称、设备名称、区域名称） |
| GET | `/web/risks/pending-count` | 未处理风险数量 |
| GET | `/web/risks/:id` | 风险详情 |
| PUT | `/web/risks/:id/read` | 标记消息通知已读 |
| PUT | `/web/risks/:id/accurate` | 复核检测准确性（`no` 时直接置 `handled`） |
| PUT | `/web/risks/:id/handle` | 处理风险（填写处理结果与处理图片） |
| DELETE | `/web/risks/:id` | 删除风险 |
| POST | `/web/risks/report` | 设备上报风险（ApiKey 认证） |

**处理状态流转**：

| 操作 | 检测准确点「否」（误报） | 检测准确点「是」（真实风险） |
|:---|:---|:---|
| `status` | 直接置 `handled` | 仍为 `pending` |
| `read` | 置 `true` | 置 `true` |
| 后续动作 | 无需填写处理内容，流程结束 | 调用 `PUT /:id/handle` 填写处理结果与图片 |
| 处理结果 | — | `status` 由 `pending` 转为 `handled`，记录 `handler` / `handled_dt` |

### 8. 设备告警（前缀 `/web/alarms`）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/alarms` | 告警分页列表（含告警类型名称、设备名称、区域名称） |
| GET | `/web/alarms/pending-count` | 未处理告警数量 |
| GET | `/web/alarms/:id` | 告警详情 |
| PUT | `/web/alarms/:id/read` | 标记消息通知已读 |
| PUT | `/web/alarms/:id/handle` | 处理告警（填写处理结果与处理图片） |
| DELETE | `/web/alarms/:id` | 删除告警 |
| POST | `/web/alarms/report` | 设备上报告警（ApiKey 认证） |

### 9. 巡检中心（前缀 `/web/inspects`）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/inspects/tasks` | 巡检任务分页列表 |
| GET | `/web/inspects/tasks/:id` | 巡检任务详情 |
| POST | `/web/inspects/tasks` | 新建巡检任务（派发给巡检人） |
| PUT | `/web/inspects/tasks/:id` | 更新巡检任务 |
| PUT | `/web/inspects/tasks/:id/read` | 标记巡检任务消息通知已读（PC 端「查看」触发，幂等） |
| DELETE | `/web/inspects/tasks/:id` | 删除巡检任务 |
| GET | `/web/inspects/records` | 巡检记录分页列表 |
| GET | `/web/inspects/records/:id` | 巡检记录详情 |

### 10. 升级管理（前缀 `/web/upgrades`）

**固件类型**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/upgrades/firmware-types` | 固件类型分页列表 |
| GET | `/web/upgrades/firmware-types/options` | 固件类型下拉选项 |
| POST | `/web/upgrades/firmware-types` | 新建固件类型 |
| PUT | `/web/upgrades/firmware-types/:id` | 更新固件类型 |
| DELETE | `/web/upgrades/firmware-types/:id` | 删除固件类型 |

**固件**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/upgrades/firmwares` | 固件分页列表 |
| GET | `/web/upgrades/firmwares/options` | 固件下拉选项 |
| POST | `/web/upgrades/firmwares` | 新建固件（含文件上传） |
| PUT | `/web/upgrades/firmwares/:id` | 更新固件 |
| DELETE | `/web/upgrades/firmwares/:id` | 删除固件 |

**AI 模型**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/upgrades/models` | AI 模型分页列表 |
| GET | `/web/upgrades/models/options` | AI 模型下拉选项 |
| POST | `/web/upgrades/models` | 新建 AI 模型（含文件上传） |
| PUT | `/web/upgrades/models/:id` | 更新 AI 模型 |
| DELETE | `/web/upgrades/models/:id` | 删除 AI 模型 |

**升级下发 / 计划 / 记录**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| POST | `/web/upgrades/devices` | 下发升级（现在升级 / 定时升级 / 批量升级） |
| GET | `/web/upgrades/plans` | 升级计划分页列表（定时升级任务） |
| PUT | `/web/upgrades/plans/:id/cancel` | 取消单个升级计划 |
| PUT | `/web/upgrades/plans/cancel` | 批量取消升级计划 |
| GET | `/web/upgrades/records` | 升级记录分页列表 |

### 11. 告警设置（前缀 `/web/settings`）

**设备告警设置**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/settings/alarm-settings` | 告警设置分页列表（仅当前用户数据） |
| POST | `/web/settings/alarm-settings` | 新建告警设置（通知人员 / 通知类型 / 报警频率） |
| PUT | `/web/settings/alarm-settings/:id` | 更新告警设置 |
| DELETE | `/web/settings/alarm-settings/:id` | 删除告警设置 |
| PUT | `/web/settings/alarm-settings/disable-all` | 一键关闭全部告警设置 |

**风险通知设置**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/settings/risk-notify-settings` | 风险通知设置分页列表（仅当前用户数据） |
| POST | `/web/settings/risk-notify-settings` | 新建风险通知设置 |
| PUT | `/web/settings/risk-notify-settings/:id` | 更新风险通知设置 |
| DELETE | `/web/settings/risk-notify-settings/:id` | 删除风险通知设置 |
| PUT | `/web/settings/risk-notify-settings/disable-all` | 一键关闭全部风险通知设置 |

**第三方平台**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/settings/getui` | 读取个推配置 |
| PUT | `/web/settings/getui` | 更新个推配置 |
| GET | `/web/settings/sms` | 读取腾讯云短信配置 |
| PUT | `/web/settings/sms` | 更新腾讯云短信配置 |

**消息推送 WebHook**

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/settings/webhooks` | WebHook 分页列表（支持名称 / 平台 / 启用状态筛选） |
| POST | `/web/settings/webhooks` | 新建 WebHook 配置 |
| GET | `/web/settings/webhooks/platforms` | 读取支持的推送平台选项 |
| POST | `/web/settings/webhooks/test` | 即时测试未保存的配置（不落库） |
| GET | `/web/settings/webhooks/:id` | 读取 WebHook 配置详情 |
| PUT | `/web/settings/webhooks/:id` | 更新 WebHook 配置 |
| DELETE | `/web/settings/webhooks/:id` | 删除 WebHook 配置（软删除） |
| PUT | `/web/settings/webhooks/:id/enable` | 启用 WebHook 配置 |
| PUT | `/web/settings/webhooks/:id/disable` | 停用 WebHook 配置 |
| POST | `/web/settings/webhooks/:id/test` | 向已保存的配置发送测试消息 |

> **注意**：告警设置与风险通知设置**只能操作当前登录用户自己的数据**
> （`user_id` 取自 JWT claims，不接受前端传入）。
> WebHook 配置为**平台级**配置，不按部门 / 用户隔离，因此不做数据权限过滤。
> 静态路由 `/webhooks/platforms`、`/webhooks/test` 必须先于 `/webhooks/:id` 注册。

### 12. 统计分析（前缀 `/web/stats`）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/stats/dashboard` | 首页看板聚合数据 |
| GET | `/web/stats/risk` | 风险统计（按类型/等级/区域/时间维度） |
| GET | `/web/stats/alarm` | 设备告警统计 |
| GET | `/web/stats/screen` | 可视化大屏聚合数据 |
| GET | `/web/stats/traffic` | 物联网卡流量统计 |
| GET | `/web/stats/device-status` | 设备在线状态分布 |

### 13. 文件上传（前缀 `/web`）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| POST | `/web/upload` | 通用上传（风险图片 / 处理图片 / 固件 / 模型文件） |
| POST | `/web/upload/audio` | 音频上传（语音配置播报文件） |

上传响应结构：

```go
type UploadResp struct {
	URL      string `json:"url"`       // 完整访问地址（存库字段直接用此值）
	FileName string `json:"file_name"` // 原始文件名
	Size     int64  `json:"size"`      // 字节数
}
```

### 14. 移动端（前缀 `/app`）

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/app/menus` | 移动端菜单树（`platform = mobile`） |
| GET | `/app/home` | 移动端首页数据 |
| GET | `/app/todos` | 待办列表（待处理风险 + 待处理告警 + 待执行巡检） |
| GET | `/app/risks` | 风险分页列表 |
| GET | `/app/risks/:id` | 风险详情 |
| PUT | `/app/risks/:id/handle` | 处理风险 |
| PUT | `/app/risks/:id/accurate` | 复核检测准确性 |
| GET | `/app/alarms` | 告警分页列表 |
| PUT | `/app/alarms/:id/handle` | 处理告警 |
| GET | `/app/inspect-tasks` | 巡检任务分页列表（仅派发给当前用户） |
| GET | `/app/inspect-tasks/:id` | 巡检任务详情 |
| PUT | `/app/inspect-tasks/:id/finish` | 完成巡检（处理结果 + 处理结果图） |
| GET | `/app/devices` | 设备分页列表 |
| GET | `/app/devices/:id` | 设备详情 |
| GET | `/app/notifications` | 通知分页列表 |
| PUT | `/app/notifications/:id/read` | 标记通知已读 |
| GET | `/app/profile` | 个人资料 |
| PUT | `/app/profile` | 更新个人资料 |
| PUT | `/app/profile/password` | 修改密码 |

> **注意**：`/app/*` 与 `/web/*` 是**并列的两套端点**，共用同一份 Service 与数据权限逻辑。
> 因此 App 与管理后台看到的同一份数据（风险、告警、巡检、设备）**可见性完全一致**。

### 15. 国标视频（v0.2.0，前缀 `/web/gb`、`/app/gb`、`/media/hook`）

国标模块共 **128 条路由**（与 `backend/tests/api/route_test.go` 的 `contractGbRoutes` 逐条对齐）：

| 分组 | 前缀 | 数量 | 说明 |
|:---|:---|:---:|:---|
| 国标业务 | `/web/gb` | **100** | 设备/通道、播放/云台/语音、回放下载、云端录像、录像计划、报警、位置、媒体节点、拉流代理、推流、级联、分组、订阅、信令日志 |
| 语音 Agent | `/web/settings/voice-agent` | **5** | 复用 `ac_thirdparty_config`（`Category='voice_agent'`） |
| 移动端 | `/app/gb` | **13** | 薄封装，复用 `/web/gb` service 层 |
| ZLM 回调 | `/media/hook` | **10** | 独立中间件组，`secret` 校验，**无 JWT / Casbin** |

关键子路由：

| 方法 | 路径 | 描述 |
|:---|:---|:---|
| GET | `/web/gb/devices` | 国标设备分页（仅 `gb_device_id` 非空） |
| POST | `/web/gb/devices/:id/activation` | 生成激活凭证（幂等） |
| POST | `/web/gb/devices/:id/sync-channel` | 下发 Catalog 查询 |
| GET | `/web/gb/channels` | 国标通道分页 |
| POST | `/web/gb/play/:channelId` | 开始实时播放，返回 WebRTC/FLV/HLS 地址 |
| POST | `/web/gb/ptz/:channelId` | 云台控制（带外 MESSAGE） |
| POST | `/web/gb/playback` | 开始回放 |
| POST | `/web/gb/playback/control` | 回放控制（带内 INFO + MANSRTSP） |
| GET | `/web/gb/cloud-records` | 云端录像分页 |
| GET | `/web/gb/sip-logs` | 信令日志（不走 DataScope） |
| POST | `/media/hook/on_publish` | ZLM 推流鉴权（拒绝返回 `{"code":-1}`） |

> 字段级定义见 `docs/开发文档/后端开发/00-接口契约.md` §15。
> `architecture` 不变量：SIP 进程只发信令，RTP/PS 由 ZLM 处理（`internal/gb28181/sipserver` 包注释）。

启动：

```bash
# 独立信令进程（副本固定 1）
cd backend && go run ./cmd/sipsvc

# 开发期 / 单容器：把 SIP 内嵌进 API 进程
GB28181_ENABLED=true GB28181_EMBEDDED=true go run ./cmd/server/main.go

# 国标模拟器（联调 / 回归）
cd backend && go run ./tools/gbsim \
  --device-id 34020000001320000001 \
  --sip-server 127.0.0.1:5060 \
  --sip-domain 3402000000 \
  --password '<设备 SIP 密码>' \
  --channels 2 --stream-file testdata/sample.h264 --verbose
```

> ⚠️ 分进程部署时，API 进程的 `control.Default()` 为 nil，播放 / 云台 / 回放等
> HTTP 触发的 SIP 命令不可用；需改用单进程内嵌，或等待跨进程 RPC（本期未实现）。

### 16. IOT 管理与多协议接入（v0.3.0）

IOT 模块共 **39 条路由**（Web 29 + App 8 + 设备端 2），另有内部 RPC 3 条：

| 分组 | 前缀 | 数量 | 鉴权 | 说明 |
|:---|:---|:---:|:---|:---|
| Web 管理 | `/web/iot` | **29** | JWT + Casbin | 产品、服务组/服务、产品-服务凭据、设备账号、激活、密钥、二维码、分享码、解绑 |
| 移动端 | `/app/iot`、`/app/rtc` | **8** | JWT + Casbin | 认领、蓝牙绑定、扫码绑定、我的设备、服务凭据、分享码、ICE |
| 设备端 | `/iot` | **2** | `X-Device-Code` + `X-Device-Secret` | OTA 配置下发、主账号激活 |
| 内部 RPC | `/rpc/mqtt/*`、`/rpc/turn/credentials` | **3** | `X-RPC-Token` | MQTT CONNECT/ACL 鉴权、TURN REST 凭据签发 |

关键约定：

- 设备复用 `ac_device`，仅新增 `product_id` / `device_secret` / `activated_at` 三列。
  **独立 GB28181 设备**（无产品编码/设备编码）不绑定产品，只按 `gb_device_id` 走白名单。
- **数据模型**：`ac_iot_service_group`（固定三条）→ `ac_iot_service`（组内服务）→
  `ac_iot_product_service_group`（产品选组）→ `ac_iot_product_service`（产品×服务凭据）。
  组本身不持有凭据。
- **脱敏**：`device_secret`、`sip_password`、产品服务 `password` 在 Web 读接口统一返回 `****`；
  OTA 与 App 服务凭据接口按设备/用户身份返回明文。
- **二维码**：设备码 `https://boticz.cn/qrDevice/<product_code>/<device_code>`，
  分享码 `https://boticz.cn/deviceShareToken/<token>`；设备未绑定产品时设备码返回 400。
- **服务组**：系统预置 `gb28181` / `webrtc` / `aichat` 三条，不开放新增/删除，
  编码与类型不可修改；被任一产品绑定后只读，列表/详情返回 `referenced_by_products`。
- **分享令牌**只由 `share-token` 接口生成；账号新增/编辑接口不接受 `share_token`。
- **后台解绑**：`POST /web/iot/devices/:id/unbind` 删除设备全部账号（含主账号）、
  清理未认领令牌并清空 `activated_at`，权限码 `iot:device:unbind`；幂等。
- **TURN**：产品×TURN 服务凭据写入 coturn SQL userdb（`turnusers_lt`，
  `MD5(username:realm:password)`）；App 播放仍用 `/app/rtc/ice-servers` 的 REST 短期凭据。
  compose 与 K8s 已注入 `TURN_STATIC_AUTH_SECRET` / `TURN_REALM` / `TURN_URLS` /
  `TURN_TTL_SECONDS` / `TURN_RPC_TOKEN`。
- **按服务组过滤设备（v0.5.0）**：`GET /web/iot/devices` 支持
  `service_group_type=gb28181|webrtc|aichat`，通过「设备 → 产品 → 产品-服务组 →
  服务组」过滤，供「实时音视频」「智能语音聊天」页面复用。

> 设计依据：`docs/敏捷开发/v0.3.0 IOT与多协议接入/00-新增需求与设计.md`；
> 契约见 `docs/开发文档/后端开发/00-接口契约.md` §10.14–§10.16。

### 17. WVP 功能对齐（v0.4.0）

除 JT/T 1078 外，WVP-GB28181-Pro 的运维能力在本期补齐。新增 **32 条路由**
（其中 2 条为匿名分享播放），集中在：

| 分组 | 新增能力 |
|:---|:---|
| 设备控制 | 编辑 SIP/媒体/位置、目录同步进度、重启、布防撤防、报警复位、心跳/注册时间统计 |
| 通道控制 | 摄像机配置字段、手动录像、ZLM 媒体信息 |
| 云台 | 预置位重命名与名称持久化、巡航点 CRUD、扫描边界（标准 0x84–0x8A） |
| 媒体节点 | 本机 CPU/内存/磁盘/网络指标与 24h 历史、当前流会话 |
| 运维 | 平台信息、日志检索、日志/报警 SSE |
| 流媒体 | 拉流代理/推流播放地址、推流 CSV/TSV 导入 |
| 云端录像 | 多段 ffmpeg 合并任务 |
| 分享 | 免登录分享令牌与匿名播放 |

关键实现：

- **模型自注册**：新增模型在 `model` 包 `init()` 调用 `RegisterModel(&X{})`，
  `bootstrap.AutoMigrate` 自动追加，避免集中修改 `migrate.go`。
- **媒体指标**：gopsutil 采集后端所在主机（与 WVP 的 oshi 一致），5s 采样、24h ring。
- **日志 SSE**：zap core tee 到 5000 条内存 ring；慢客户端丢弃，不阻塞业务日志。
- **报警 SSE**：按 `ac_gb_alarm.id` 游标轮询，不改信令进程状态。
- **分享令牌**：库中只存 SHA-256，匿名路由不挂 authMW/casbinMW。
- **云端合并**：ffmpeg `concat -c copy`；镜像已安装 ffmpeg，失败原因落库。
- **索引修正**：`gb_device_id` 部分唯一索引排除空串，允许多台非国标设备。

> 契约见 `docs/开发文档/后端开发/00-接口契约.md` §17；
> 设计说明见 `docs/敏捷开发/v0.4.0 WVP功能对齐/00-实施说明.md`。

### 路由分组与中间件

| 分组 | 中间件 | 端点数 | 说明 |
|:---|:---|:---:|:---|
| 公开 | 无 | 3 | 两个登录接口 + `/health` |
| `wApiKey` | `ApiKeyAuth()` | 3 | 设备上报三端点，**不经过 Casbin** |
| `wAuth` | `AuthMiddleware` + `CasbinMiddleware` | 333 | `/web` 业务接口（Swagger `/web` 共 335，含 2 个公开登录） |
| `aAuth` | `AuthMiddleware` + `CasbinMiddleware` | 41 | `/app` 全部移动端业务接口 |

---

## 权限控制链路

```
┌──────────────────────────────────────────────────────────────────────────────┐
│  1. JWT 签发                                                                  │
│     pkg/auth/jwt.go  ·  internal/service/system/auth_service.go               │
│     claims: user_id / username / role_code / dept_id / data_perm /            │
│             company_id / cross_dept_ids                                       │
└────────────────────────────────┬─────────────────────────────────────────────┘
                                 ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  2. Auth 中间件（认证）                                                       │
│     internal/api/middleware/auth.go → AuthMiddleware.Handler()                │
│     ① 校验 Authorization: Bearer <token>                                      │
│     ② 写入 gin.Context（handler 读 c.GetInt64("user_id")）                    │
│     ③ 写入 Go Context → 驱动 BaseModel 钩子自动填充审计字段与 dept_id          │
└────────────────────────────────┬─────────────────────────────────────────────┘
                                 ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  3. Casbin RBAC（鉴权）                                                       │
│     pkg/perm/casbin.go + config/rbac_model.conf                               │
│     enforce(username, c.Request.URL.Path, c.Request.Method)                   │
│                                                                              │
│     matcher = keyMatch2(r.obj, p.obj) && r.act == p.act                       │
│               && (g(r.sub,p.sub) || g2(r.sub,p.sub) || g3(r.sub,p.sub))      │
│                                                                              │
│     p 策略：角色(role.code) → 菜单(ac_role_menu_rel) → 菜单API(ac_menu_api)    │
│     g ：用户登录名 → 角色        g2：部门编码 → 角色     g3：岗位编码 → 角色     │
└────────────────────────────────┬─────────────────────────────────────────────┘
                                 ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│  4. 数据权限（行级过滤）                                                       │
│     pkg/utils/data_scope.go                                                   │
│     utils.NewDataScope(c).ApplyToQuery(q, "alias.dept_id")                    │
│                                                                              │
│     all        → 不过滤                cross_dept → dept_id IN (本部门 + 跨部门)│
│     dept       → dept_id = 本部门      personal   → dept_id = 本部门 AND       │
│                                                     created_by = 当前用户       │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 四级权限叠加说明

| 层 | 机制 | 失败表现 | 位置 |
|:---:|:---|:---|:---|
| 1 | JWT HS256，claims 携带 `user_id`/`user_name`/`data_perm`/`company_id` | 401「无效的Token」 | `pkg/auth/jwt.go` |
| 2 | Auth 中间件：校验 Bearer token，写入 gin.Context + Go Context | 401「未提供认证Token」/「认证格式错误」 | `internal/api/middleware/auth.go` |
| 3 | Casbin RBAC：`g`（用户→角色）、`g2`（部门→角色）、`g3`（岗位→角色）三种继承 | 403「权限不足」 | `pkg/perm/casbin.go` + `config/rbac_model.conf` |
| 4 | 数据权限：按 `data_perm` 施加 `dept_id` / `created_by` 行级过滤 | 列表为空（数据被过滤） | `pkg/utils/data_scope.go` |

### 关键约束

1. **接口与方法都参与鉴权** —— matcher 中的 `keyMatch2(r.obj, p.obj) && r.act == p.act`
   是 v0.1.0 相对早期方案的核心修正。若省略这两项，只要用户拥有任意角色且该角色登记过任意
   一条 `p` 策略，就会被放行到**所有**接口 —— 接口级鉴权形同虚设。
2. **`keyMatch2` 支持路径参数** —— `p.obj = /web/devices/:id` 可匹配 `r.obj = /web/devices/42`。
3. **`ac_menu_api` 必须登记完整** —— 每个端点在「菜单管理 → 菜单 API」中登记后才会产生
   对应的 `p` 策略。**未登记的端点对所有非超级管理员用户都会被 403 拦截**。排查权限问题时，
   先确认 `ac_menu_api` 是否覆盖了该路径。
4. **角色授权变更后必须同步 Casbin** —— `internal/service/system/role_service.go` 中的
   `syncCasbinForRole(ctx, roleID)` 负责重建该角色的 `p`/`g`/`g2`/`g3` 策略并
   `ReloadPolicy()`。
5. **多副本部署依赖定时重载** —— `casbin.auto_load_interval`（默认 5m）保证各 Pod 的策略
   最终一致。对权限生效时效要求高的场景需缩短该间隔或改为消息通知。
6. **`dept_id` 为空的表不做行级过滤** —— 如 `ac_menu`、`ac_role` 等全局配置表。

---

## API 文档维护

Swagger 文档由 handler 文件上的注释生成，**修改返回类型时必须同步更新 Swagger 注释**。

```bash
# 生成两份 Swagger JSON（需先安装 swag）
cd backend && bash scripts/gen-swagger.sh
#   → docs/web/swagger.json   （从 internal/api/handler/web 生成）
#   → docs/app/swagger.json   （从 internal/api/handler/app 生成）
```

工作流（后端接口变更后的标准流程）：

```
后端 Swagger 注释（handler 文件）
      │
      │  bash scripts/gen-swagger.sh
      ▼
docs/web/swagger.json  +  docs/app/swagger.json
      │
      │  后端启动在 :8080
      │  cd frontend && bash scripts/api-gen.sh
      ▼
frontend/src/admin/api/generated/*Controller.ts   （严禁手动编辑）
      │
      ▼
更新对应 UI 代码（严格使用生成的响应类型，禁止 as any）
```

Swagger 注释规范：

| 规则 | 说明 |
|:---|:---|
| `@ID` | 取方法名首字母小写；分页接口命名为 `pageXxxx`（如 `Page` → `pageRisks`、`pageUsers`） |
| `@Success 200` | 必须与实际返回结构一致；分页必须用类型化 `response.RPageXxx`，**禁止** `response.PageResp` |
| `@Tags` | 与契约文档的 Tags 规范一致（System / Auth / Profile / Basic / Device / Risk / Alarm / Inspect / Upgrade / Setting / Stats / Common / AppAuth / App） |
| 关联名称 | 列表涉及多表关联时必须返回关联表的**名称字段** |

```go
// PageRisks 分页查询风险记录
//
//	@Summary		风险分页列表
//	@Description	按区域、风险类型、处理状态筛选风险记录
//	@Tags			Risk
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int		false	"页码，默认 1"
//	@Param			page_size	query		int		false	"每页条数，默认 20"
//	@Param			region_id	query		int		false	"所属区域"
//	@Param			status		query		string	false	"处理状态 pending/handled"
//	@Success		200			{object}	response.RPageRiskRecord
//	@Failure		400			{object}	response.RBase
//	@Router			/web/risks [get]
//	@ID				pageRisks
func (h *RiskHandler) Page(c *gin.Context) { ... }
```

---

## 响应类型命名规范

响应类型定义在 `internal/dto/response/`（21 个文件），分为四类。

### 基类

| 类型 | 结构 | 说明 |
|:---|:---|:---|
| `BaseResponse` | `{code int, message string, data any}` | 统一响应包装；`pkg/utils/response.go` 中的 `Response` 为其运行时表示 |
| `PageResp` | `{items, total, page, page_size}` | 通用分页载荷（**仅限工具场景**，业务端点禁止使用） |

`pkg/utils/response.go` 提供的响应辅助函数：

| 函数 | HTTP 状态 | code | 典型场景 |
|:---|:---:|:---:|:---|
| `Success(c, data)` | 200 | 200 | 查询成功 |
| `Created(c, data)` | 201 | 200 | 创建成功（前端以 `code` 判断成功） |
| `BadRequest(c, msg)` | 400 | 400 | 参数校验失败 |
| `Unauthorized(c, msg)` | 401 | 401 | 未认证 / token 无效 |
| `Forbidden(c, msg)` | 403 | 403 | 权限不足 |
| `NotFound(c, msg)` | 404 | 404 | 资源不存在 |
| `ServerError(c, msg)` | 500 | 500 | 服务端异常 |

### 命名规则

| 类型 | 命名 | 说明 | 示例 |
|:---|:---|:---|:---|
| 简单列表 | `RXxxList` | `Data []model.Xxx`，无分页 | `RRegionList`、`RRiskTypeList` |
| 分页列表 | `RPageXxx` | `Data PageXxxResp{Items, Total, Page, PageSize}` | `RPageRiskRecord`、`RPageUser` |
| 单条详情 | `RXxx` | `Data model.Xxx` | `RDevice`、`RRegion` |
| 下拉选项 | `ROptionList` | `Data []OptionItem{ID, Name, Code}` | `ROptionList` |

```go
// response.OptionItem 下拉选项通用结构
type OptionItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code,omitempty"`
}
```

**规则**：

- 列表接口的 Swagger 注解与实际返回**必须一致**：简单列表用 `RXxxList`，分页列表用 `RPageXxx`。
- 严禁一个端点在不同条件下返回不同格式（如有时分页有时不分页）—— 统一为一种格式。
- 修改后端返回类型时，同步更新 Swagger 注解 → 重新生成文档 → 重新生成前端类型 → 修改前端代码。

### 文件组织

| 文件 | 覆盖范围 |
|:---|:---|
| `base.go` | `BaseResponse` |
| `common.go` | `OptionItem`、`ROptionList` |
| `auth.go` | `LoginData`、`RLogin` |
| `profile.go` | `RUser`、`GenerateApiKeyResp`、`RGenerateApiKey`、`RApiKeyList` |
| `user.go` / `role.go` / `menu.go` / `dept.go` / `post.go` / `config.go` / `operation_log.go` / `notification.go` | 系统管理各模块的类型化分页与详情 |
| `basic.go` | 区域 / 风险类型 / 告警类型的 `PageXxxResp`/`RPageXxx`、`RXxx` |
| `device.go` | 设备 / 设备历史 / 设备模式 / 物联网卡 / 换卡记录 / 语音配置 |
| `risk.go` | `PageRiskRecordResp`/`RPageRiskRecord`、`RRiskRecord`、`RCount` |
| `alarm.go` | `PageAlarmRecordResp`/`RPageAlarmRecord`、`RAlarmRecord` |
| `inspect.go` | 巡检任务与巡检记录 |
| `upgrade.go` | 固件类型 / 固件 / AI 模型 / 升级计划 / 升级记录 + `RUpgradeDevices` |
| `setting.go` | 告警设置 / 风险通知设置 / `RGetuiConfig` / `RSmsConfig` |
| `stats.go` | `RDashboard`、`RRiskStats`、`RAlarmStats`、`RScreen`、`RTrafficStats`、`RDeviceStatus` |
| `app.go` | `RAppHome`、`RAppTodoList`、`RAppMenuList` |

### 使用示例

```go
// internal/api/handler/web/basic_handler.go（节选）

// RegionOptions 区域下拉选项
//
//	@Summary	区域下拉选项
//	@Tags		Basic
//	@Produce	json
//	@Success	200	{object}	response.ROptionList
//	@Router		/web/basic/regions/options [get]
//	@ID			regionOptions
func (h *BasicHandler) RegionOptions(c *gin.Context) {
	ds := utils.NewDataScope(c)

	items, err := h.regionSvc.Options(c.Request.Context(), ds)
	if err != nil {
		utils.ServerError(c, "查询区域选项失败")
		return
	}

	// 简单列表：Data 直接是数组
	utils.Success(c, response.ROptionList{BaseResponse: response.OK(), Data: items})
}

// PageRegions 区域分页列表
//
//	@Summary	区域分页列表
//	@Tags		Basic
//	@Produce	json
//	@Param		page		query		int		false	"页码，默认 1"
//	@Param		page_size	query		int		false	"每页条数，默认 20"
//	@Success	200			{object}	response.RPageRegion
//	@Router		/web/basic/regions [get]
//	@ID			pageRegions
func (h *BasicHandler) PageRegions(c *gin.Context) {
	var req request.PageRegionReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	req.Normalize() // page/page_size 默认值：1 / 20

	ds := utils.NewDataScope(c)

	// 分页：Data 是 {items, total, page, page_size} 对象
	resp, err := h.regionSvc.Page(c.Request.Context(), &req, ds)
	if err != nil {
		utils.ServerError(c, "查询区域列表失败")
		return
	}

	utils.Success(c, response.RPageRegion{BaseResponse: response.OK(), Data: *resp})
}
```

对应前端读取方式：

```typescript
// 简单列表：读 res.data 本身
const regionRes = await BasicController.regionOptions()
const options = regionRes.data ?? []

// 分页列表：读 res.data.items
const pageRes = await BasicController.pageRegions({ page: 1, page_size: 20 })
const rows = pageRes.data?.items ?? []
const total = pageRes.data?.total ?? 0
```

---

## 安全说明

| 项 | 措施 | 备注 |
|:---|:---|:---|
| 密码存储 | bcrypt 哈希（`golang.org/x/crypto`） | 不可逆，不支持找回原密码，只能重置 |
| 认证令牌 | JWT HS256，有效期由 `jwt.expire_hours` 控制 | 无状态；登出依赖前端清除 token（令牌黑名单需 Redis，见待完成事项） |
| 设备上报 | `X-Api-Key` 头 → SHA256 → 匹配 `ac_api_key`（同时要求 `active = true`） | Key 明文仅在生成时返回一次；`last_used_dt` 记录使用时间 |
| 接口鉴权 | Casbin，`keyMatch2(r.obj, p.obj) && r.act == p.act` | **路径与方法同时校验**，见「权限控制链路」 |
| 行级隔离 | `DataScope.ApplyToQuery`，四种 `data_perm` 模式 | `dept_id` 与 `created_by` 双重限定 |
| 跨库防护 | 测试一律使用 `ai_camera_test` | `config-test.yaml` 与 `config/config.yaml` 库名不同，避免误连 |
| 审计留痕 | 增删改写入 `ac_operation_log`，支持导出 xlsx | 由 `middleware/operation_log.go` 统一处理 |
| 配置隔离 | `config.yaml` / `config-test.yaml` 已 gitignore；生产走 K8s ConfigMap | 明文口令不入库 |
| 软删除 | 全部模型 `DeletedAt` 软删除 | 删除可追溯；注意唯一索引对软删行的占用（如需重新使用同一编号需物理删除） |

> **注意**：生产环境应关闭 Swagger（`swagger.enabled: false`）并在 Ingress 层限制
> `/swagger-ui` 与 `/web/api-docs.json`、`/app/api-docs.json` 的访问来源。

---

## v0.1.0 关键特性

| 特性 | 版本 | 说明 |
|:---|:---:|:---|
| 四层分层架构 | v0.1.0 | Handler → Service → Repository → Model；Handler 不得直接调用 Repository |
| 四级权限叠加 | v0.1.0 | JWT → Auth 中间件 → Casbin RBAC → 数据权限（`all`/`cross_dept`/`dept`/`personal`） |
| Casbin 接口与方法校验 | v0.1.0 | matcher 补齐 `keyMatch2(r.obj, p.obj) && r.act == p.act`（修复早期方案的鉴权漏洞） |
| 三链角色继承 | v0.1.0 | `g`（用户→角色）、`g2`（部门→角色）、`g3`（岗位→角色）取并集 |
| Casbin 策略自动重载 | v0.1.0 | `StartAutoReload(5m)`，保证多副本部署下策略最终一致 |
| 设备上报专用鉴权 | v0.1.0 | `ApiKeyAuth`（`X-Api-Key` → SHA256 → `ac_api_key`），独立于 Casbin 分组 |
| Bootstrap 自动装配 | v0.1.0 | `init()` 完成配置→日志→DB→迁移→JWT→Casbin，业务代码零样板 |
| 一次性 AutoMigrate | v0.1.0 | fail-fast，任一模型迁移失败即中止（避免逐表循环静默跳过） |
| 类型化分页响应 | v0.1.0 | 全部列表使用 `RPageXxx` / `RXxxList`，禁止 `response.PageResp` |
| 关联名称预加载 | v0.1.0 | Repository 层统一 `Preload`，列表展示名称而非裸 ID |
| 软删除 + 审计钩子 | v0.1.0 | `BaseModel` 提供 `DeletedAt`，`BeforeCreate`/`BeforeUpdate` 从 Context 自动填充 |
| 统一响应格式 | v0.1.0 | `{code, message, data}`；`Success`/`BadRequest`/`Unauthorized` 等辅助函数 |
| 双端 Swagger 文档 | v0.1.0 | `/web/api-docs.json` 与 `/app/api-docs.json` 分别生成，驱动前端代码生成 |
| 结构化日志 | v0.1.0 | zap + lumberjack，`json` / `console` 双格式，按大小与天数滚动 |
| 优雅退出 | v0.1.0 | 监听 `SIGINT`/`SIGTERM`，给在途请求 10 秒完成时间 |
| 事务隔离测试框架 | v0.1.0 | `testutil.WithTx(t, fn)`，每个用例在独立事务中执行并自动回滚 |
| 多平台消息推送 | v0.1.0 | `pkg/webhook` 支持飞书 / 钉钉（HMAC-SHA256 加签）/ 企业微信 / 自定义 WebHook；风险与告警上报落库后自动分发，失败仅记录通道状态 |

**待接入**（见 `docs/敏捷开发/v0.1.0 基础功能开发/待完成事项.md`）：
对象存储（`pkg/storage`）、Prometheus 指标（`/metrics`）、定时任务调度器、
Redis（令牌黑名单 / 分布式锁）、WebSocket 实时推送。

---

## 测试

后端测试使用 `testutil.WithTx(t, fn)` —— **每个测试在数据库事务中运行，结束后自动回滚**，
确保用例之间数据隔离。

### 单元测试 / 集成测试

```bash
# ── 1. 清空测试库（必须，避免与 seed 数据冲突）──
cd backend && go run ./tests/testutil/resetdb/main.go

# ── 2. 全量测试（必须设置环境变量指向测试库）──
cd backend && TEST_DB_HOST=192.168.1.227 TEST_DB_PORT=5432 TEST_DB_USER=boticz \
  TEST_DB_PASSWORD='<测试库密码>' TEST_DB_NAME=ai_camera_test \
  go test ./tests/... -count=1 -timeout 180s

# ── 3. 指定模块 ──
cd backend && TEST_DB_HOST=192.168.1.227 TEST_DB_PORT=5432 TEST_DB_USER=boticz \
  TEST_DB_PASSWORD='<测试库密码>' TEST_DB_NAME=ai_camera_test \
  go test ./tests/system/... -v

# ── 4. 单个用例 ──
cd backend && TEST_DB_HOST=192.168.1.227 TEST_DB_PORT=5432 TEST_DB_USER=boticz \
  TEST_DB_PASSWORD='<测试库密码>' TEST_DB_NAME=ai_camera_test \
  go test ./tests/risk/ -run TestRiskHandle -v

# ── 5. 覆盖率 ──
cd backend && TEST_DB_HOST=192.168.1.227 TEST_DB_PORT=5432 TEST_DB_USER=boticz \
  TEST_DB_PASSWORD='<测试库密码>' TEST_DB_NAME=ai_camera_test \
  go test ./tests/... -coverprofile=coverage.out
cd backend && go tool cover -html=coverage.out -o coverage.html
```

> **注意**：测试**必须**使用 `ai_camera_test`，**禁止**操作开发库 `ai_camera_dev`。
> 启动测试前需停止本地运行的服务，避免端口冲突。

**字段验证要求**：所有后端单元测试必须验证**绝大多数字段的更新**，防止「接口返回成功但
字段没存进去」。关键验证点包括风险处理状态机（`status` + `read` 双分支）、数据权限四模式
可见性、Casbin matcher 语义（有角色但未授权该接口应返回 403）。

**v0.3.0 IOT 测试**：`backend/tests/iot/` 覆盖产品/服务组 CRUD、凭据生成与脱敏、
协议白名单、OTA、激活、分享令牌一次性、SIP/扫码绑定、后台解绑、MQTT 鉴权/ACL；
`tests/api/route_test.go` 守门 29 条 Web IOT 路由与 8 条 App 路由。

### 冒烟测试

冒烟测试在测试环境执行（`config-test.yaml`，端口 8081）：

```bash
# ── 1. 清空测试库 ──
cd backend && go run ./tests/testutil/resetdb/main.go

# ── 2. 初始化测试基础数据 ──
cd backend && go run ./tests/testutil/initdb/main.go

# ── 3. 启动测试服务 ──
export CONFIG_PATH="config/config-test.yaml" && cd backend && go run ./cmd/server/main.go

# ── 4. 执行测试（前端 e2e/smoke 串行，spec 按文件名字母序）──
cd frontend && VITE_BACKEND_PORT=8081 npm run dev
cd frontend && npx playwright test e2e/smoke --workers=1

# ── 5. 测试完成后关闭前后端服务 ──
```

执行顺序规范：

- 冒烟测试串行执行（`--workers=1`），spec 文件按**文件名字母序**运行。
- **必须先执行所有功能测试（`01a` ~ `10f`），最后执行 `99-all-list-page.spec.ts`**
  （全页面覆盖验证）—— `99-` 前缀保证其排在最后。
- 新增冒烟测试文件编号必须小于 `99`。

### 静态检查

```bash
cd backend && go vet ./...
cd backend && gofmt -l .          # 输出非空即失败
cd backend && go mod tidy         # 整理依赖
```

---

## 依赖安装

```bash
# 拉取全部依赖
cd backend && go mod download

# 添加依赖
cd backend && go get github.com/xxx/xxx && go mod tidy

# 主要依赖（go.mod 直接依赖）
#   github.com/gin-gonic/gin          v1.12.0
#   github.com/emiago/sipgo           v1.6.0      （国标 SIP）
#   github.com/casbin/casbin/v2       v2.135.0
#   github.com/casbin/gorm-adapter/v3 v3.0.2
#   github.com/golang-jwt/jwt/v5      v5.3.1
#   github.com/robfig/cron/v3         v3.0.0
#   github.com/spf13/viper            v1.21.0
#   github.com/swaggo/swag/v2         v2.0.0-rc5
#   go.uber.org/zap                   v1.28.0
#   golang.org/x/crypto               v0.52.0
#   golang.org/x/text                 v0.37.0     （GB2312 ↔ UTF-8）
#   gopkg.in/natefinch/lumberjack.v2  v2.2.1
#   gorm.io/driver/postgres           v1.5.11
#   gorm.io/gorm                      v1.25.12

# Swagger 生成工具（gen-swagger.sh 内部会安装）
go install github.com/swaggo/swag/v2/cmd/swag@latest
export PATH="$(go env GOPATH)/bin:$PATH"

# 国内网络加速
go env -w GOPROXY=https://goproxy.cn,direct
```

---

## 相关文档

- [../docs/开发文档/后端开发/00-接口契约.md](../docs/开发文档/后端开发/00-接口契约.md) — **接口契约**（v0.1.0 §3 + v0.2.0 国标 §10/§15 + v0.3.0 IOT §10.14–§10.16）
- [../docs/需求文档/00-需求总览.md](../docs/需求文档/00-需求总览.md) — 功能边界、状态常量、权限矩阵、非功能需求
- [../docs/开发文档/数据库设计/sql/ddl/](../docs/开发文档/数据库设计/sql/ddl/) — DDL 脚本（63 张业务表）
- [../docs/部署文档/01-部署与运维.md](../docs/部署文档/01-部署与运维.md) — Docker 镜像、K8s 清单、CI/CD、运维命令
- [../docs/部署文档/02-监控与容灾.md](../docs/部署文档/02-监控与容灾.md) — 业务指标埋点清单、告警规则、容灾方案
- [../docs/敏捷开发/v0.1.0 基础功能开发/待完成事项.md](../docs/敏捷开发/v0.1.0%20基础功能开发/待完成事项.md) — 待接入能力清单
- [../CLAUDE.md](../CLAUDE.md) — Claude Code 开发指引
