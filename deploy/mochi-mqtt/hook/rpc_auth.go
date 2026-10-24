// Package hook 把 mochi-mqtt 的连接鉴权与 ACL 判定委托给 AI Camera 后端。
//
// 为什么不在 broker 侧直接查库：
//  1. 账号体系（IOT 服务组凭据、设备账号）属于平台，broker 不应该持有数据库凭据；
//  2. 鉴权策略会随产品/服务组变化，放在后端才能一次发布、所有 broker 生效；
//  3. broker 只负责协议，保持成无状态进程，便于水平扩容。
//
// 契约：后端提供两个内部 RPC 端点（非用户 JWT，使用共享 token）：
//
//	POST {base}/rpc/mqtt/auth  {"username","password","client_id","remote_addr"}
//	     → {"allow":true,"group_code":"...","reason":""}
//	POST {base}/rpc/mqtt/acl   {"username","client_id","topic","action"}
//	     → {"allow":true,"reason":""}
//
// **失败一律拒绝（fail-closed）**：后端不可达、超时、token 错误、响应非 2xx，
// 都返回 false。MQTT 是设备控制面，宁可连不上也不能在鉴权服务故障时放行。
package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// Config 是 RPC 鉴权 Hook 的配置。
type Config struct {
	// BackendURL 是后端基址，如 http://backend:8080（不要带尾部斜杠）。
	BackendURL string
	// Token 与后端 MQTT_RPC_TOKEN 一致。
	Token string
	// Timeout 单次 RPC 超时；默认 2 秒。设备连接对延迟敏感，不能等太久。
	Timeout time.Duration
	// RemoteAddrFrom 从连接信息里取对端地址，便于后端做来源审计。
	// 默认使用 cl.Net.Remote。
	Logger *slog.Logger
}

// RPCClient 是后端鉴权接口的客户端，抽成接口便于单测替身。
type RPCClient interface {
	Authenticate(ctx context.Context, req AuthRequest) (AuthResponse, error)
	CheckACL(ctx context.Context, req ACLRequest) (ACLResponse, error)
}

// AuthRequest 对应后端的 /rpc/mqtt/auth 请求体。
type AuthRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	ClientID   string `json:"client_id"`
	RemoteAddr string `json:"remote_addr"`
}

// AuthResponse 是鉴权结果。GroupCode 用于 ACL 推导，broker 不解释其含义。
type AuthResponse struct {
	Allow     bool   `json:"allow"`
	GroupCode string `json:"group_code"`
	Reason    string `json:"reason"`
}

// ACLRequest 对应后端的 /rpc/mqtt/acl 请求体。
type ACLRequest struct {
	Username string `json:"username"`
	ClientID string `json:"client_id"`
	Topic    string `json:"topic"`
	Action   string `json:"action"` // publish / subscribe
}

// ACLResponse 是 ACL 结果。
type ACLResponse struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

// Hook 实现 mochi-mqtt 的 OnConnectAuthenticate 与 OnACLCheck。
type Hook struct {
	mqtt.HookBase
	cfg    Config
	client RPCClient
	// groupCodes 记录 clientID → group_code，ACL 判定时复用，
	// 避免每次发布都重新用用户名查一次组。
	groupCodes map[string]string
}

// New 构造 Hook。
func New(cfg Config, client RPCClient) *Hook {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if client == nil {
		client = NewHTTPClient(cfg)
	}
	return &Hook{cfg: cfg, client: client, groupCodes: map[string]string{}}
}

// ID 返回 hook 标识。
func (h *Hook) ID() string { return "ai-camera-rpc-auth" }

// Provides 声明本 hook 负责的两个扩展点。
func (h *Hook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

// OnConnectAuthenticate 在 CONNECT 阶段调用后端鉴权。
func (h *Hook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)

	ctx, cancel := context.WithTimeout(context.Background(), h.cfg.Timeout)
	defer cancel()

	resp, err := h.client.Authenticate(ctx, AuthRequest{
		Username:   username,
		Password:   password,
		ClientID:   cl.ID,
		RemoteAddr: remoteAddrOf(cl),
	})
	if err != nil {
		h.log().Warn("MQTT 鉴权 RPC 失败，按拒绝处理",
			"client_id", cl.ID, "username", username, "error", err)
		return false
	}
	if !resp.Allow {
		h.log().Info("MQTT 鉴权拒绝", "client_id", cl.ID, "username", username, "reason", resp.Reason)
		return false
	}

	if resp.GroupCode != "" {
		h.groupCodes[cl.ID] = resp.GroupCode
	}
	h.log().Debug("MQTT 鉴权通过", "client_id", cl.ID, "username", username, "group", resp.GroupCode)
	return true
}

// OnACLCheck 在 PUBLISH / SUBSCRIBE 阶段调用后端。
//
// 注意：mochi-mqtt 在客户端断开后不会通知 hook 清理缓存，这里按 clientID
// 覆盖写入即可；clientID 复用时新连接会重新鉴权并覆盖旧值。
func (h *Hook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	action := "subscribe"
	if write {
		action = "publish"
	}

	ctx, cancel := context.WithTimeout(context.Background(), h.cfg.Timeout)
	defer cancel()

	resp, err := h.client.CheckACL(ctx, ACLRequest{
		Username: usernameOf(cl),
		ClientID: cl.ID,
		Topic:    topic,
		Action:   action,
	})
	if err != nil {
		h.log().Warn("MQTT ACL RPC 失败，按拒绝处理",
			"client_id", cl.ID, "topic", topic, "error", err)
		return false
	}
	if !resp.Allow {
		h.log().Debug("MQTT ACL 拒绝", "client_id", cl.ID, "topic", topic, "action", action, "reason", resp.Reason)
	}
	return resp.Allow
}

func (h *Hook) log() *slog.Logger { return h.cfg.Logger }

func remoteAddrOf(cl *mqtt.Client) string {
	if cl == nil {
		return ""
	}
	// Net.Remote 是 mochi-mqtt 解析好的对端地址；Net.Conn 在部分监听器
	// （如内嵌 inline 客户端）可能为空，优先用 Remote。
	return strings.TrimSpace(cl.Net.Remote)
}

func usernameOf(cl *mqtt.Client) string {
	if cl == nil {
		return ""
	}
	// mochi-mqtt 把 CONNECT 的用户名保存在 Client.Properties.Username（[]byte）。
	return strings.TrimSpace(string(cl.Properties.Username))
}

// ---------------------------------------------------------------------------
// HTTP 实现
// ---------------------------------------------------------------------------

// HTTPClient 用 HTTP POST 调用后端 RPC。
//
// 选 HTTP 而不是 gRPC/自定义 TCP：后端已有 HTTP 栈与统一鉴权中间件，
// 两个端点、QPS 不高（只在连接与发布/订阅时调用），HTTP 足够且便于排查。
type HTTPClient struct {
	base  string
	token string
	hc    *http.Client
}

// NewHTTPClient 构造 HTTP 客户端。
func NewHTTPClient(cfg Config) *HTTPClient {
	return &HTTPClient{
		base:  strings.TrimRight(cfg.BackendURL, "/"),
		token: cfg.Token,
		hc:    &http.Client{Timeout: cfg.Timeout},
	}
}

// Authenticate 调用 /rpc/mqtt/auth。
func (c *HTTPClient) Authenticate(ctx context.Context, req AuthRequest) (AuthResponse, error) {
	var out AuthResponse
	err := c.post(ctx, "/rpc/mqtt/auth", req, &out)
	return out, err
}

// CheckACL 调用 /rpc/mqtt/acl。
func (c *HTTPClient) CheckACL(ctx context.Context, req ACLRequest) (ACLResponse, error) {
	var out ACLResponse
	err := c.post(ctx, "/rpc/mqtt/acl", req, &out)
	return out, err
}

func (c *HTTPClient) post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("X-RPC-Token", c.token)
	}

	resp, err := c.hc.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &statusError{Code: resp.StatusCode}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type statusError struct{ Code int }

func (e *statusError) Error() string {
	return "后端 RPC 返回 HTTP " + http.StatusText(e.Code)
}
