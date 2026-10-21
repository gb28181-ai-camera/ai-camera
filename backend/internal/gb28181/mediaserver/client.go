// Package mediaserver 封装 ZLMediaKit 的 REST API。
//
// 设计边界（为什么只做 REST 客户端）：
//   - 本项目 GB28181 媒体路径由 ZLM 独占 —— Go 侧不接收 RTP、不解析 PS，
//     只做 SSRC 编排与 RTP server 生命周期管理（设计 §2.1）。
//   - 因此这里只映射 ZLM 的 /index/api/* 接口，不引入任何媒体库。
//   - 单端口收流模式下（deploy/zlm/config.ini），平台**不调用 OpenRtpServer**；
//     该方法仅为对齐 WVP 能力而保留，注释中明确标注禁用原因。
package mediaserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// DefaultVhost ZLM 未显式配置虚拟主机时使用的 vhost。
const DefaultVhost = "__defaultVhost__"

// defaultTimeout 单次 API 调用的默认超时。
//
// 与 WVP 一致取 10s：ZLM 的 API 大多是内存操作，正常毫秒级返回；
// 10s 已经足够覆盖节点短暂 GC/负载抖动，同时避免编排链路被长时间挂住。
const defaultTimeout = 10 * time.Second

// Client 是一个 ZLM 节点（ac_gb_media_server 的一行）对应的 REST 客户端。
//
// 字段均为只读语义，可被多个 goroutine 并发使用。
type Client struct {
	// BaseURL 形如 http://127.0.0.1:80，**不带**末尾斜杠与 /index/api 前缀。
	BaseURL string
	// Secret 对应 ZLM 的 api.secret（ac_gb_media_server.secret）。
	Secret string
	// HTTPClient 可注入自定义客户端（测试、代理、连接池复用）。
	HTTPClient *http.Client
	// Timeout 仅在 HTTPClient 为 nil 时生效。
	Timeout time.Duration
}

// APIError 表示 ZLM 返回了 HTTP 200 但业务码非 0 的响应。
//
// 单独成类型是为了让调用方能区分「网络不通」与「ZLM 明确拒绝」：
// 前者应重试/标记节点离线，后者通常意味着参数或状态不对，重试无意义。
type APIError struct {
	API  string
	Code int
	Msg  string
}

// Error 实现 error 接口。
func (e *APIError) Error() string {
	return fmt.Sprintf("ZLM API %s 失败: code=%d msg=%s", e.API, e.Code, e.Msg)
}

// New 创建客户端。baseURL 可带或不带末尾斜杠；缺少协议时补 http://。
func New(baseURL, secret string) *Client {
	return &Client{
		BaseURL: normalizeBaseURL(baseURL),
		Secret:  secret,
		Timeout: defaultTimeout,
	}
}

// NewWithHTTPClient 使用调用方提供的 http.Client 创建客户端。
//
// 测试中通过它注入 httptest.Server.Client()，避免绕过 TLS/连接管理逻辑。
func NewWithHTTPClient(baseURL, secret string, hc *http.Client) *Client {
	c := New(baseURL, secret)
	c.HTTPClient = hc
	return c
}

// FromModel 按媒体节点配置构造客户端。
//
// 为什么用 ip + http_port：ZLM 的 REST API 与 HTTP 播放服务共用同一个端口
// （新版 ZLM 已把 flv/hls 合并进 http 端口）。ip 为空时回退 127.0.0.1，
// 端口为 0 时回退 80，避免生成 :0 这种不可连接地址。
func FromModel(ms *model.GbMediaServer) *Client {
	if ms == nil {
		return New("http://127.0.0.1:80", "")
	}
	host := strings.TrimSpace(ms.IP)
	if host == "" {
		host = "127.0.0.1"
	}
	// 允许节点地址里已经带协议（多网卡/NAT 场景常见），避免拼出 http://http://...
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return New(fmt.Sprintf("%s:%d", strings.TrimRight(host, "/"), portOr(ms.HTTPPort, 80)), ms.Secret)
	}
	return New(fmt.Sprintf("http://%s:%d", host, portOr(ms.HTTPPort, 80)), ms.Secret)
}

// FromDefaultServer 从数据库读取默认节点并构造客户端。
//
// 只读调用 repository，不改动仓储层实现（模块 D/E 的编排入口会复用）。
func FromDefaultServer(ctx context.Context) (*Client, *model.GbMediaServer, error) {
	ms, err := repository.NewGbMediaServerRepo().GetDefault(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("查询默认流媒体节点失败: %w", err)
	}
	return FromModel(ms), ms, nil
}

// FromServerID 按 ZLM mediaServerId 读取节点并构造客户端。
func FromServerID(ctx context.Context, serverID string) (*Client, *model.GbMediaServer, error) {
	ms, err := repository.NewGbMediaServerRepo().GetByServerID(ctx, serverID)
	if err != nil {
		return nil, nil, fmt.Errorf("查询流媒体节点 %s 失败: %w", serverID, err)
	}
	return FromModel(ms), ms, nil
}

// ── 通用请求 ──────────────────────────────────────────────────

// apiResponse ZLM 的统一响应。
//
// 注意：ZLM 的返回**并非全部把结果放在 data 里**。以下 API 把结果直接放在根层级：
//   - openRtpServer → 根级 port
//   - closeRtpServer → 根级 hit
//   - getRtpInfo → 根级 exist/peer_ip/peer_port/local_ip/local_port
//   - isMediaOnline → 根级 online
//   - startSendRtp → 根级 local_port
//
// 用指针接收这些可选字段，才能区分「字段不存在」与「值为零」。
// 少数 ZLM 版本也会把它们放进 data，因此调用方两边都读。
type apiResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`

	Port      *int   `json:"port"`
	Hit       *int   `json:"hit"`
	Online    *bool  `json:"online"`
	Exist     *bool  `json:"exist"`
	PeerIP    string `json:"peer_ip"`
	PeerPort  *int   `json:"peer_port"`
	LocalIP   string `json:"local_ip"`
	LocalPort *int   `json:"local_port"`
	Changed   *int   `json:"changed"`
}

// hasData 判断响应是否携带 data 负载（null/空都视为没有）。
func (r *apiResponse) hasData() bool {
	return len(r.Data) > 0 && string(r.Data) != "null"
}

// apiURL 拼接 API 地址。
func (c *Client) apiURL(api string) string {
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = "http://127.0.0.1:80"
	}
	return base + "/index/api/" + api
}

// httpClient 返回本次请求使用的客户端。
func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &http.Client{Timeout: timeout}
}

// do 发起一次 POST（application/x-www-form-urlencoded），只返回 data 负载。
//
// 对于结果在根层级的 API，请使用 doResponse。
func (c *Client) do(ctx context.Context, api string, params map[string]any) (json.RawMessage, error) {
	r, err := c.doResponse(ctx, api, params)
	if err != nil {
		return nil, err
	}
	return r.Data, nil
}

// doResponse 发起一次 POST，并返回完整的统一响应（含根层级字段）。
//
// 为什么用 POST 表单而不是 GET：WVP 的 ZLMRESTfulUtils 全量使用表单 POST，
// ZLM 两种都支持，保持与参考实现一致可减少联调差异；表单也不受 URL 长度限制。
func (c *Client) doResponse(ctx context.Context, api string, params map[string]any) (*apiResponse, error) {
	form := url.Values{}
	if c.Secret != "" {
		form.Set("secret", c.Secret)
	}
	for k, v := range params {
		if v == nil {
			continue
		}
		form.Set(k, fmt.Sprint(v))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL(api), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("构造 ZLM 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 ZLM API %s 失败: %w", api, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 ZLM API %s 响应失败: %w", api, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ZLM API %s 返回 HTTP %d: %s", api, resp.StatusCode, snippet(body))
	}

	var r apiResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("解析 ZLM API %s 响应失败: %w (body=%s)", api, err, snippet(body))
	}
	if r.Code != 0 {
		return nil, &APIError{API: api, Code: r.Code, Msg: r.Msg}
	}
	return &r, nil
}

// doGetRaw 发起 GET 并返回原始字节（getSnap 等返回图片的接口）。
func (c *Client) doGetRaw(ctx context.Context, api string, params map[string]any) ([]byte, error) {
	q := url.Values{}
	if c.Secret != "" {
		q.Set("secret", c.Secret)
	}
	for k, v := range params {
		if v == nil {
			continue
		}
		q.Set(k, fmt.Sprint(v))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(api)+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("构造 ZLM GET 请求失败: %w", err)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 ZLM API %s 失败: %w", api, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 ZLM API %s 响应失败: %w", api, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ZLM API %s 返回 HTTP %d: %s", api, resp.StatusCode, snippet(body))
	}
	// 失败时 ZLM 返回 JSON（{code:-1,msg:...}），成功时返回图片二进制。
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") ||
		(len(body) > 0 && body[0] == '{') {
		var r apiResponse
		if json.Unmarshal(body, &r) == nil && r.Code != 0 {
			return nil, &APIError{API: api, Code: r.Code, Msg: r.Msg}
		}
	}
	return body, nil
}

// decode 把 data 解到 out；data 为空或 null 时视为「无数据」。
func decode(api string, data json.RawMessage, out any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("解析 ZLM API %s 的 data 失败: %w", api, err)
	}
	return nil
}

// snippet 截断响应体，避免把整段二进制/超长 JSON 写进日志与错误。
func snippet(b []byte) string {
	const max = 512
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}

// portOr 返回 port；port <= 0 时返回 fallback。
func portOr(port, fallback int64) int64 {
	if port > 0 {
		return port
	}
	return fallback
}

// vhostOr 返回 vhost；空串时回退 __defaultVhost__。
func vhostOr(vhost string) string {
	if strings.TrimSpace(vhost) == "" {
		return DefaultVhost
	}
	return vhost
}

// normalizeBaseURL 补齐协议并去掉末尾斜杠。
func normalizeBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return "http://127.0.0.1:80"
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	return strings.TrimRight(base, "/")
}

// IsAPIError 判断 err 是否为 ZLM 业务错误，并返回其错误码。
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
