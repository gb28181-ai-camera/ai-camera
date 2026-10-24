# mochi-mqtt broker（后端 RPC 鉴权）

MQTT 服务本身**不持有账号数据**：客户端 CONNECT 时，broker 通过 HTTP RPC 问后端
「这个用户名/密码能不能连」，发布/订阅时再问一次「这个 topic 能不能读写」。
账号来自 IOT 管理的「服务组」（`webrtc` 类型、包含 MQTT 服务项的组）。

## 为什么是 RPC 而不是 broker 直连数据库

1. 账号体系属于平台：服务组凭据、设备账号、产品授权都在 `ac_iot_*` 表里；
2. broker 不应持有数据库凭据，也不该在每次改账号策略时重新发布；
3. broker 保持无状态，可水平扩容。

## 契约

后端两个内部端点（**不是用户 JWT**，用共享 token；由 `MQTT_RPC_TOKEN` 配置）：

```http
POST /rpc/mqtt/auth
X-RPC-Token: <token>
{"username":"...","password":"...","client_id":"...","remote_addr":"..."}
→ {"allow":true,"group_code":"iot-webrtc-01","reason":""}

POST /rpc/mqtt/acl
X-RPC-Token: <token>
{"username":"...","client_id":"...","topic":"iot/xxx/data","action":"publish"}
→ {"allow":true,"reason":""}
```

**失败一律拒绝（fail-closed）**：后端不可达、超时、token 错误、响应非 2xx，
`OnConnectAuthenticate`/`OnACLCheck` 都返回 `false`。MQTT 是设备控制面，
鉴权服务故障时放行比连不上更危险。

## 运行

```bash
cd deploy/mochi-mqtt
MQTT_RPC_TOKEN=<与后端一致> BACKEND_RPC_URL=http://localhost:8080 go run .
```

| 环境变量 | 默认 | 说明 |
|:--|:--|:--|
| `MQTT_LISTEN` | `:1883` | MQTT TCP 监听 |
| `MQTT_WS_LISTEN` | `:8083` | MQTT over WebSocket；留空关闭 |
| `BACKEND_RPC_URL` | `http://backend:8080` | 后端基址 |
| `MQTT_RPC_TOKEN` | 无 | **必填**，为空时 broker 拒绝启动 |
| `MQTT_DEBUG` | 空 | 设为 `1` 打开调试日志 |

## 与内部 fork 的关系

本目录是一个**可直接运行的最小 broker**，基于公开的
`github.com/mochi-mqtt/server/v2`（v2.7.9）。如果团队使用的是内部 fork
（`cloud/mochi-mqtt`），把 `hook/rpc_auth.go` 原样拷贝进 fork，并在一处
`AddHook` 即可 —— hook 只依赖 `mqtt.HookBase`、`OnConnectAuthenticate`、
`OnACLCheck` 三个公开接口，不依赖本目录的其他文件。

## 测试

```bash
cd deploy/mochi-mqtt && go test ./... -count=1
```

覆盖：允许 / 拒绝 / RPC 故障 fail-closed / ACL 动作映射 / Provides 声明。
