# HTTP 层测试（路由 + 中间件）

本目录补齐了测试体系中此前缺失的一层：**HTTP 层**。

## 为什么需要这一层

`tests/<domain>/` 下的 305 个用例全部集中在 **Service 层**，以 `testutil.WithTx`
在数据库事务中运行。Service 层再厚，也覆盖不到下列问题——而它们恰恰是本项目
真实出过 bug 的地方：

| 真实缺陷 | 症状 | 本目录的回归用例 |
|---|---|---|
| `/web/devices/modes` 被 `/web/devices/:id` 吞掉 | `modes` 被当作 ID，返回 400「无效的设备ID」 | `TestRoutePriority_StaticWinsOverParam` |
| Casbin matcher 不校验 `obj` / `act` | 任何角色可访问所有接口 | `TestCasbinMiddleware/matcher_校验_act`、`matcher_校验_obj` |
| 空查询参数导致列表恒为空 | `?active=` 被绑定成「非 nil 的 false」，`WHERE active = false` → 0 条 | `TestNormalizeEmptyQuery` |
| 路由漏注册 / 契约漂移 | 契约承诺的端点 404 | `TestRouteTable_MatchesContract` |

这些都属于**装配层**缺陷：Service 单元测试无论多完整都发现不了。

## 文件

| 文件 | 内容 |
|---|---|
| `helpers_test.go` | 路由构造（`buildRouter`）、请求工具（`doRequest` / `runChain`）、响应解析、测试配置与 JWT 注入 |
| `route_test.go` | 路由注册表、静态段 vs `:param` 段优先级、契约守门、公开端点 |
| `middleware_test.go` | AuthMiddleware、CasbinMiddleware、NormalizeEmptyQuery、ApiKeyAuth、OperationLog |

## 为什么不依赖数据库

绝大多数用例**不碰数据库**，理由有三：

1. 路由表与中间件的错误是**装配期**错误，判定它们不需要任何业务数据；
2. 本机可能同时跑着 :8081 的后端测试服务，共享物理库会互相污染；
3. 不连库的用例毫秒级完成，可以在改一行 `web.go` 后立刻重跑。

因此：

- **请求不监听端口**：全部经 `httptest.NewRecorder()` + `Engine.ServeHTTP()` 或
  `gin.New()` + `e.Any("/*path", ...)` 在内存中走完；
- **只有三类用例连库**，且一律用 `testutil.WithTx(t, fn)` 包住，结束即回滚：
  - `TestRoutePriority_StaticWinsOverParam`（要观察真实 handler 的响应特征）；
  - `TestApiKeyAuth` / `TestOperationLog`（本来就在测写库）；
  - `TestAuthMiddleware_AuditFieldsFlowIntoDB`（要验证 GORM 钩子真的落库）。

### 两个必要的测试注入点

1. `bootstrap.SetConfig(...)`
   `go test` 的 CWD 是包目录，`bootstrap.init()` 读不到 `config/config.yaml`，
   于是 `bootstrap.Config()` 为 `nil`，而 `router.SetupRouter()` 会立刻读取
   `cfg.Server.Mode` —— 不注入配置就是空指针崩溃。

2. `middleware.SetJWTServiceForTest(...)`
   `SetupRouter()` 内部注入的是 `bootstrap.JWTService()`（测试进程为 `nil`，
   且 `SetDefaultJWTService` 被 `sync.Once` 保护，只会生效一次）。
   不覆盖的话所有受保护端点都会以「认证服务未初始化」500 短路，
   路由用例就只能看到 500，失去意义。

## 一个重要的实现事实：gin 对同级顺序不敏感

`web.go` 与契约文档都强调「静态子路径必须先于 `/:id` 注册」。
但在当前锁定的 **gin v1.12.0** 下，静态子节点由路由树自行规范化到 `:param` 之前，
**与注册先后无关**：

- `TestGinNormalizesSiblingOrder` 先注册 `/probe/:id` 再注册 `/probe/modes`，
  路由表仍输出 `modes` 在前，且请求 `/probe/modes` 命中静态路由；
- `TestRouteRegistry_StaticSiblingsPrecedeParam` 中的「静态排在 `:param` 之前」
  断言因此**永远不会失败**——它的价值是固定住这一意图，
  并在将来 gin 升级改变该行为时立刻报警。

**真正的防线是 `TestRoutePriority_StaticWinsOverParam`**：它发真实请求，
按响应特征判断命中的 handler（例如被吞掉时会返回 400「无效的设备ID」）。
若 gin 回调行为，该用例会立刻失败。

## 契约守门

`route_test.go` 里的 `contractRoutes` 是从
`docs/开发文档/后端开发/00-接口契约.md` §3「路由总表」**人工抄录**的期望表
（211 条业务端点）。`TestRouteTable_MatchesContract` 双向比对：

- 契约有、路由无 → 失败（契约承诺未实现）；
- 路由有、契约无 → 失败，除非登记在 `contractGaps` 中（见下）。

### 已知契约缺口

`contractGaps` 登记了「实现已先行、契约尚未补齐」的端点，测试对其豁免但仍会
`t.Logf` 打印，且会校验该端点确实存在（防止豁免条目变成死代码）。

> `PUT /web/inspects/tasks/:id/read`（`inspectH.MarkTaskRead`）
> 已在 `web.go` 注册、Swagger 注释齐备（`@Router /web/inspects/tasks/{id}/read [put]`），
> 但契约 §3.9「巡检」表中没有它。修复方向是补契约文档，不是删路由。

## 运行

```bash
# 全部用例（含连库的）
cd backend && TEST_DB_HOST=192.168.1.227 TEST_DB_PORT=5432 TEST_DB_USER=boticz \
  TEST_DB_PASSWORD='<测试库密码>' TEST_DB_NAME=ai_camera_test \
  go test ./tests/api/ -count=1 -v -timeout 300s

# 只跑不依赖数据库的路由/中间件用例
cd backend && go test ./tests/api/ \
  -run 'TestRouteTable_MatchesContract|TestRouteRegistry_StaticSiblingsPrecedeParam|TestGinNormalizesSiblingOrder|TestPublicRoutes_NoAuthRequired|TestAuthMiddleware$|TestCasbinMiddleware$|TestNormalizeEmptyQuery' \
  -count=1 -v
```

> 注意：`tests/api` 与其它测试包共用同一个物理库，整仓运行时必须 `-p 1`。
