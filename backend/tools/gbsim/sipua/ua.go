// Package sipua 实现 gbsim 所需的极简 SIP 用户代理。
//
// 它只覆盖 GB28181 **设备侧**用到的动作：REGISTER（含 Digest）、周期 Keepalive、
// 接收平台下发的 MESSAGE 并回 200 OK。INVITE / BYE / 云台应答等留给后续批次。
//
// 底层用 github.com/emiago/sipgo 的事务层，不自己实现 RFC 3261 的状态机。
// 注意 sipgo 只提供**客户端** Digest（Client.DoDigestAuth），服务端 Digest 是另一个项目
// （emiago/diago）的能力 —— 本模拟器是客户端，用不着服务端能力，但摘要计算仍自研（见 digest.go）。
package sipua

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

const (
	// ContentTypeMANSCDP 是 GB28181 MANSCDP XML 报文的 Content-Type。
	ContentTypeMANSCDP = "Application/MANSCDP+xml"
	// ContentTypeSDP 是 INVITE 携带的 SDP。
	ContentTypeSDP = "application/sdp"
)

var (
	// ErrForbidden 表示平台明确拒绝（403）：多为密码错误或设备未建档（§4.2 白名单）。
	ErrForbidden = errors.New("平台返回 403 Forbidden")
	// ErrUnauthorized 表示带凭证重发后仍被挑战，说明 Digest 凭证或算法不匹配。
	ErrUnauthorized = errors.New("平台持续返回 401，Digest 凭证可能不正确")
)

// Config 是 UA 的构造参数。
type Config struct {
	DeviceID  string // 20 位国标设备编码
	SIPServer string // 平台地址 host:port
	SIPDomain string // SIP 域（Digest realm）
	Password  string // SIP 密码
	LocalIP   string // 本地信令 IP，必须已解析（见 config.ResolveLocalIP）
	LocalPort int    // 本地信令端口
	Transport string // udp / tcp
	Expires   int    // 注册有效期（秒）
	UserAgent string // User-Agent 头，仅用于日志辨识
	Logger    *slog.Logger
}

// MessageHandler 处理入向 MESSAGE 的 XML 正文，返回若干条**出向 MESSAGE** 的正文。
//
// ⚠️ 这是 GB28181 最容易被写错的一处：平台发来 `<Query><CmdType>Catalog</CmdType>` 时，
// 设备要**先回一个空的 200 OK**，再**另发一条独立的 MESSAGE** 携带 `<Response><CmdType>Catalog</CmdType>`。
// 把目录塞进 200 OK 的 body 里是发不通的 —— WVP 的 `CatalogQueryMessageHandler` 只对
// 独立的响应 MESSAGE 做解析。
//
// 返回 nil 表示只回 200 OK、不发后续 MESSAGE（心跳应答、未知报文的场合）。
type MessageHandler func(body []byte, contentType string) [][]byte

// UA 是一个 GB28181 设备侧用户代理。
type UA struct {
	cfg Config
	log *slog.Logger

	ua     *sipgo.UserAgent
	client *sipgo.Client
	server *sipgo.Server

	recipient  sip.Uri // Request-URI：sip:{domain}@{sipServer}
	aor        sip.Uri // 设备的 AoR：sip:{deviceID}@{domain}
	contactURI sip.Uri // Contact：sip:{deviceID}@{localIP}:{localPort}

	fromTag string
	baseCtx context.Context

	mu           sync.RWMutex
	registered   bool
	expiresAt    time.Time
	lastResponse *sip.Response
	msgHandler   MessageHandler
}

// New 构造 UA 并完成 sipgo 的 UA/Client/Server 组装，但不开始监听 —— 由 Start 负责。
func New(cfg Config) (*UA, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "gbsim/" + cfg.DeviceID
	}
	if cfg.Transport == "" {
		cfg.Transport = "udp"
	}

	ua, err := sipgo.NewUA(sipgo.WithUserAgent(cfg.UserAgent))
	if err != nil {
		return nil, fmt.Errorf("创建 sipgo UA 失败：%w", err)
	}

	// 关键配置：让 client 的 Laddr 固定为本地监听地址。
	// sipgo 的传输层按 laddr 字符串在连接池里查找已有连接，命中后
	// **出向请求直接复用监听 socket** —— 这正是 GB28181 需要的：
	// 设备用同一个端口注册、发心跳，平台也必须能从这个端口把下行指令送回。
	// 若这里退化为临时端口，平台回复会打到另一个端口，表现为「注册成功但收不到指令」。
	localAddr := fmt.Sprintf("%s:%d", cfg.LocalIP, cfg.LocalPort)
	client, err := sipgo.NewClient(ua,
		sipgo.WithClientConnectionAddr(localAddr),
		sipgo.WithClientHostname(cfg.LocalIP),
		sipgo.WithClientPort(cfg.LocalPort),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 sipgo Client 失败：%w", err)
	}
	server, err := sipgo.NewServer(ua)
	if err != nil {
		return nil, fmt.Errorf("创建 sipgo Server 失败：%w", err)
	}

	u := &UA{
		cfg:    cfg,
		log:    cfg.Logger,
		ua:     ua,
		client: client,
		server: server,
		// Request-URI：sip:{domain}@{sipServer}。
		// 注意 Digest 的 uri 参数必须与它逐字节一致，两边都取自 u.recipient.Addr()。
		recipient:  sip.Uri{Scheme: "sip", Host: cfg.SIPDomain},
		aor:        sip.Uri{Scheme: "sip", User: cfg.DeviceID, Host: cfg.SIPDomain},
		contactURI: sip.Uri{Scheme: "sip", User: cfg.DeviceID, Host: cfg.LocalIP, Port: cfg.LocalPort},
		fromTag:    sip.GenerateTagN(16),
	}

	u.server.OnMessage(u.handleMessage)
	u.server.OnOptions(u.handleOptions)
	u.server.OnNoRoute(u.handleNoRoute)
	// 本批次不实现 INVITE 应答（媒体点播留给后续批次），但必须明确回绝而不是静默丢弃，
	// 否则平台侧会一直等到超时，排障时看不出「设备收到了但不会做」。
	u.server.OnInvite(func(req *sip.Request, tx sip.ServerTransaction) {
		u.log.Warn("收到 INVITE，但本批次尚未实现媒体点播应答", "call_id", callIDOf(req))
		tx.Respond(sip.NewResponseFromRequest(req, sip.StatusNotImplemented, "Not Implemented", nil))
	})

	return u, nil
}

// SetMessageHandler 注册入向 MESSAGE 的处理器。
func (u *UA) SetMessageHandler(h MessageHandler) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.msgHandler = h
}

// LocalAddr 返回本地信令地址 host:port。
func (u *UA) LocalAddr() string {
	return fmt.Sprintf("%s:%d", u.cfg.LocalIP, u.cfg.LocalPort)
}

// AOR 返回设备的 AoR（sip:{deviceID}@{domain}）。
func (u *UA) AOR() string { return u.aor.String() }

// Contact 返回 Contact 头使用的 URI。
func (u *UA) Contact() string { return u.contactURI.String() }

// IsRegistered 返回当前是否处于已注册状态。
func (u *UA) IsRegistered() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.registered
}

// ExpiresAt 返回注册的过期时刻。
func (u *UA) ExpiresAt() time.Time {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.expiresAt
}

// LastRegisterResponse 返回最近一次 REGISTER 的最终响应，便于排障。
func (u *UA) LastRegisterResponse() *sip.Response {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.lastResponse
}

// Start 开始监听本地信令端口。它会阻塞直到监听就绪或失败。
//
// 传入的 ctx 同时作为「UA 生命周期」的基准上下文：入向 MESSAGE 触发的**出向** MESSAGE
// （如目录响应）会沿用它，ctx 取消时这些发送也会一并中止。
func (u *UA) Start(ctx context.Context) error {
	u.baseCtx = ctx
	network := u.transport()
	ready := make(chan struct{})
	listenCtx := context.WithValue(ctx, sipgo.ListenReadyCtxKey, sipgo.ListenReadyCtxValue(ready))
	go func() {
		u.log.Info("信令监听启动", "network", network, "addr", u.LocalAddr())
		if err := u.server.ListenAndServe(listenCtx, network, u.LocalAddr()); err != nil && ctx.Err() == nil {
			u.log.Error("信令监听退出", "error", err)
		}
	}()

	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
		return fmt.Errorf("信令监听 %d 秒内未就绪（%s %s）", 3, network, u.LocalAddr())
	}
}

// Close 释放 UA 资源。
func (u *UA) Close() error {
	if u.server != nil {
		_ = u.server.Close()
	}
	if u.client != nil {
		_ = u.client.Close()
	}
	if u.ua != nil {
		return u.ua.Close()
	}
	return nil
}

// transport 返回 sipgo 期望的网络名字（小写）。
func (u *UA) transport() string {
	if u.cfg.Transport == "" {
		return "udp"
	}
	return u.cfg.Transport
}

// handleMessage 处理入向 MESSAGE。
//
// 入向校验（§4.4 的三道）由平台侧实现，模拟器这里只负责「把 XML 交给上层并回 200 OK」。
func (u *UA) handleMessage(req *sip.Request, tx sip.ServerTransaction) {
	body := req.Body()
	ct := ""
	if h := req.ContentType(); h != nil {
		ct = h.Value()
	}
	u.log.Debug("收到 MESSAGE", "call_id", callIDOf(req), "content_type", ct,
		"bytes", len(body), "body", string(body))

	u.mu.RLock()
	h := u.msgHandler
	u.mu.RUnlock()

	var replies [][]byte
	if h != nil {
		replies = h(body, ct)
	}

	// 先回 200 OK（不带正文），再另发响应 MESSAGE —— 顺序不能颠倒。
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	if err := tx.Respond(res); err != nil {
		u.log.Error("回复 MESSAGE 200 OK 失败", "error", err, "call_id", callIDOf(req))
		return
	}

	for i, r := range replies {
		if len(r) == 0 {
			continue
		}
		ctx := u.baseCtx
		if ctx == nil {
			ctx = context.Background()
		}
		if _, err := u.SendMessage(ctx, r); err != nil {
			u.log.Error("发送响应 MESSAGE 失败", "error", err, "index", i, "bytes", len(r))
		}
	}
}

// handleOptions 应答平台心跳探测。部分平台用 OPTIONS 探活，不回会导致设备被判离线。
func (u *UA) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	if err := tx.Respond(res); err != nil {
		u.log.Error("回复 OPTIONS 200 OK 失败", "error", err)
	}
}

// handleNoRoute 兜底：打印未处理的方法并回 405，避免静默丢包。
func (u *UA) handleNoRoute(req *sip.Request, tx sip.ServerTransaction) {
	u.log.Warn("收到未处理的方法", "method", req.Method.String(), "call_id", callIDOf(req))
	res := sip.NewResponseFromRequest(req, sip.StatusNotImplemented, "Not Implemented", nil)
	_ = tx.Respond(res)
}

// SendMessage 向平台发送带 MANSCDP 正文的 MESSAGE（心跳、报警、目录响应都用它）。
func (u *UA) SendMessage(ctx context.Context, body []byte) (*sip.Response, error) {
	req := u.newRequest(sip.MESSAGE, u.recipient)
	req.AppendHeader(sip.NewHeader("Content-Type", ContentTypeMANSCDP))
	req.SetBody(body)
	return u.client.Do(ctx, req)
}

// newRequest 构造一条「设备 → 平台」的请求，填好 From / To / Contact / 传输层。
//
// Call-ID / CSeq / Max-Forwards / Via 交给 sipgo 的 clientRequestBuildReq 补齐，
// 因此这里不能提前设置它们，否则会被认为「已有」而不生成。
func (u *UA) newRequest(method sip.RequestMethod, to sip.Uri) *sip.Request {
	req := sip.NewRequest(method, to)
	from := &sip.FromHeader{Address: u.aor}
	params := sip.NewParams()
	from.Params = params.Add("tag", u.fromTag)
	req.AppendHeader(from)
	req.AppendHeader(&sip.ToHeader{Address: to})
	req.AppendHeader(&sip.ContactHeader{Address: u.contactURI})
	req.SetTransport(upperTransport(u.transport()))

	// ⚠️ 必须显式指定发往哪里。GB28181 的 Request-URI 是 `sip:{SIP域}`（如 sip:3402000000），
	// 域不是可解析的主机名；若让传输层从 Recipient 推导目的地，它会去解析 "3402000000"
	// 这个"主机名"并必然失败。平台的真实地址在 --sip-server 里，只用于路由。
	req.SetDestination(u.cfg.SIPServer)
	return req
}

// upperTransport 把网络名转成 SIP 头里惯例的大写形式。
func upperTransport(network string) string {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return "TCP"
	default:
		return "UDP"
	}
}

// callIDOf 安全地取出 Call-ID，用于日志关联（§13.3.3）。
func callIDOf(req *sip.Request) string {
	if req == nil {
		return ""
	}
	if h := req.CallID(); h != nil {
		return string(*h)
	}
	return ""
}
