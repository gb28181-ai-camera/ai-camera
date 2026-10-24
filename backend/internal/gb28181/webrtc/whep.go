// Package webrtc 提供 ZLMediaKit 的 WHEP（WebRTC-HTTP Egress Protocol）客户端。
//
// 边界：本包**只做 SDP 转发**，不解析 RTP/PS、不维护 ICE 状态，也不做 ICE trickle。
// 它把 App 发来的 SDP offer 通过 HTTP POST 交给 ZLM 的
// /index/api/webrtc?app=rtp&stream=<stream>&type=play，取回 answer SDP。
// 媒体面始终是 App ↔ ZLM（必要时经 TURN），后端只做一个无状态的协商代理。
//
// 为什么不用 Pion 在 Go 侧终结 WebRTC：
//   - 设计 §7.4 已选定 ZLM 作为媒体节点，浏览器/App 直接与 ZLM 交换媒体；
//   - Go 侧引入 ICE/DTLS/SRTP 会与 ZLM 的端口、NAT 配置重复，且增加一个必须扩容的媒体组件；
//   - App 通过 MQTT 只是**信令通道**，不改变媒体拓扑。
package webrtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
)

const (
	// DefaultApp 与 GB28181 点播层保持一致，ZLM 的 app 恒为 rtp（设计 §7.3）。
	DefaultApp = "rtp"
	// defaultTimeout 单次 WHEP 协商的超时。
	//
	// ZLM 的 WebRTC 建流通常是毫秒级，但首次拉流要等关键帧；15s 既覆盖
	// 设备刚上线时的抖动，也避免 App 侧的 MQTT 请求无限挂起。
	defaultTimeout = 15 * time.Second
	// maxAnswerSize 限制响应体大小，防止异常节点把内存打满。
	maxAnswerSize = 1 << 20 // 1 MiB
)

var (
	// ErrStreamNotFound 流不存在或尚未就绪（ZLM 明确返回 not found）。
	ErrStreamNotFound = errors.New("流不存在或尚未就绪")
	// ErrBadRequest 参数非法（空 stream / 空 offer）。
	ErrBadRequest = errors.New("WHEP 请求参数非法")
	// ErrBadAnswer ZLM 返回 2xx，但响应体不是合法的 SDP answer。
	ErrBadAnswer = errors.New("ZLM 未返回有效的 answer SDP")
)

// Resolver 解析本次协商使用的媒体节点。
//
// 抽成函数类型是为了单测可以注入 httptest 的地址，而不必连数据库。
type Resolver func(ctx context.Context) (*model.GbMediaServer, error)

// Client 是 ZLM WHEP 协商客户端，可被多个 goroutine 并发使用。
type Client struct {
	// Resolve 解析默认媒体节点；为 nil 时 Answer 直接报错（fail-closed）。
	Resolve Resolver
	// HTTPClient 可注入自定义客户端；为 nil 时按 Timeout 新建。
	HTTPClient *http.Client
	// Timeout 仅在 HTTPClient 为 nil 时生效。
	Timeout time.Duration
	// App 为 ZLM app，默认 rtp。
	App string
}

// NewClient 用调用方提供的节点解析函数构造客户端。
func NewClient(resolve Resolver) *Client {
	return &Client{Resolve: resolve, Timeout: defaultTimeout, App: DefaultApp}
}

// NewDefaultClient 从数据库读取默认媒体节点（ac_gb_media_server.default_server）。
func NewDefaultClient() *Client {
	return NewClient(func(ctx context.Context) (*model.GbMediaServer, error) {
		_, ms, err := mediaserver.FromDefaultServer(ctx)
		if err != nil {
			return nil, err
		}
		if ms == nil {
			return nil, errors.New("没有可用的默认媒体节点（ac_gb_media_server.default_server）")
		}
		return ms, nil
	})
}

// WHEPURL 构造 ZLM 的 WHEP 地址。
//
// 地址优先级与 play.HTTPBase / mediaserver.BuildPlayURLs 保持一致：
// stream_ip（对外播放地址）优先于 ip（API 地址），避免多网卡/NAT 场景下
// 把容器内部地址发给 App。两者都为空时回退 127.0.0.1。
func WHEPURL(ms *model.GbMediaServer, app, stream string) string {
	host := "127.0.0.1"
	port := int64(80)
	if ms != nil {
		if v := strings.TrimSpace(ms.StreamIP); v != "" {
			host = v
		} else if v := strings.TrimSpace(ms.IP); v != "" {
			host = v
		}
		if ms.HTTPPort > 0 {
			port = ms.HTTPPort
		}
	}
	// 兼容配置里带协议的地址，避免拼出 http://http://...
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://"), "/")

	q := url.Values{}
	q.Set("app", app)
	q.Set("stream", stream)
	q.Set("type", "play")
	return fmt.Sprintf("http://%s/index/api/webrtc?%s", net.JoinHostPort(host, strconv.FormatInt(port, 10)), q.Encode())
}

// HTTPError 表示 ZLM 返回了非 2xx 状态码。
type HTTPError struct {
	StatusCode int
	Body       string
}

// Error 实现 error 接口。
func (e *HTTPError) Error() string {
	return fmt.Sprintf("ZLM WHEP 返回 HTTP %d: %s", e.StatusCode, e.Body)
}

// Unwrap 让 404 可以被 errors.Is 识别为流不存在。
func (e *HTTPError) Unwrap() error {
	if e.StatusCode == http.StatusNotFound {
		return ErrStreamNotFound
	}
	return nil
}

// ZLMError 表示 ZLM 返回 HTTP 200 但业务码非 0（如流不存在）。
type ZLMError struct {
	Code int
	Msg  string
}

// Error 实现 error 接口。
func (e *ZLMError) Error() string {
	return fmt.Sprintf("ZLM WHEP 业务错误: code=%d msg=%s", e.Code, e.Msg)
}

// Unwrap 把「流不存在」类文案映射成统一哨兵错误，供上层回 stream_not_found。
func (e *ZLMError) Unwrap() error {
	if isNotFoundMessage(e.Msg) {
		return ErrStreamNotFound
	}
	return nil
}

// Answer 把 offer SDP 交给 ZLM，返回 answer SDP。
//
// 不做 ICE trickle：ZLM 的 WHEP 端点在响应 answer 之前已完成 ICE 候选收集，
// 返回的是非 trickle answer；App 发来的 candidate 消息由 bridge 忽略。
func (c *Client) Answer(ctx context.Context, stream, offer string) (string, error) {
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return "", fmt.Errorf("%w: stream 为空", ErrStreamNotFound)
	}
	if strings.TrimSpace(offer) == "" {
		return "", fmt.Errorf("%w: offer SDP 为空", ErrBadRequest)
	}
	if c == nil || c.Resolve == nil {
		return "", errors.New("WHEP 客户端未配置媒体节点解析器")
	}

	ms, err := c.Resolve(ctx)
	if err != nil {
		return "", fmt.Errorf("解析默认媒体节点失败: %w", err)
	}
	if ms == nil {
		return "", errors.New("没有可用的默认媒体节点")
	}

	app := strings.TrimSpace(c.App)
	if app == "" {
		app = DefaultApp
	}
	target := WHEPURL(ms, app, stream)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(offer))
	if err != nil {
		return "", fmt.Errorf("构造 WHEP 请求失败: %w", err)
	}
	// ZLM 通过 Content-Type 判断请求体是 SDP；缺省会被当成不支持的载荷。
	req.Header.Set("Content-Type", "application/sdp")
	req.Header.Set("Accept", "application/sdp")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 ZLM WHEP 失败: %w", err)
	}
	defer resp.Body.Close()

	// 多读 1 字节用于判断是否超限：截断的 SDP 会让 App 协商失败，
	// 若被当成功下发，排查时只会看到「answer 收到了但连不上」。
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswerSize+1))
	if err != nil {
		return "", fmt.Errorf("读取 ZLM WHEP 响应失败: %w", err)
	}
	if len(body) > maxAnswerSize {
		return "", fmt.Errorf("%w: 响应体超过 %d 字节", ErrBadAnswer, maxAnswerSize)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &HTTPError{StatusCode: resp.StatusCode, Body: snippet(body)}
	}

	// ZLM 在流不存在等场景下可能返回 HTTP 200 + {"code":-1,"msg":"..."}。
	if looksLikeJSON(resp.Header.Get("Content-Type"), body) {
		var zr struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if err := json.Unmarshal(body, &zr); err == nil {
			if zr.Code != 0 {
				return "", &ZLMError{Code: zr.Code, Msg: zr.Msg}
			}
			return "", fmt.Errorf("%w: 响应是 JSON 而非 SDP", ErrBadAnswer)
		}
		return "", fmt.Errorf("%w: 响应是 JSON 而非 SDP", ErrBadAnswer)
	}

	answer := string(body)
	if !strings.HasPrefix(strings.TrimSpace(answer), "v=") {
		return "", fmt.Errorf("%w: %s", ErrBadAnswer, snippet(body))
	}
	return answer, nil
}

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

// looksLikeJSON 判断响应体是否为 JSON。
//
// 优先看 Content-Type；部分 ZLM 版本不带 Content-Type，再看首字符。
func looksLikeJSON(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	trimmed := strings.TrimSpace(string(body))
	return strings.HasPrefix(trimmed, "{")
}

// isNotFoundMessage 识别不同 ZLM 版本/语言的「流不存在」文案。
func isNotFoundMessage(msg string) bool {
	l := strings.ToLower(strings.TrimSpace(msg))
	if l == "" {
		return false
	}
	for _, kw := range []string{"not find", "not found", "no such", "can not find", "cannot find"} {
		if strings.Contains(l, kw) {
			return true
		}
	}
	return strings.Contains(msg, "不存在") || strings.Contains(msg, "未找到") || strings.Contains(msg, "找不到")
}

// snippet 截断错误响应体，避免把超长 HTML/JSON 写进日志。
func snippet(b []byte) string {
	const max = 256
	if len(b) > max {
		return string(b[:max]) + "..."
	}
	return string(b)
}
