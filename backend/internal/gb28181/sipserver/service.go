// Package sipserver 是自研 GB28181 SIP 信令服务（设计 §14.4 模块 C）。
//
// 分层：
//
//	sipserver   ← sipgo 的事务/传输层封装、请求分发、出向指令
//	auth        ← 白名单 + 服务端 Digest
//	device      ← 注册/心跳/目录/过期扫描/信令日志
//	session     ← INVITE 建立的点播/回放会话
//	protocol/   ← MANSCDP / SDP 编解码
//	ssrc        ← SSRC 分配
//
// 架构不变量（设计 §2.1）：**SIP 服务不碰媒体包**。这里只发 INVITE 与信令，
// RTP/PS 由 ZLM 直接接收；因此 sipsvc 不得调用 openRtpServer（设计 §15.5c）。
package sipserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/boticz/ai-camera-backend/internal/gb28181/auth"
	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/device"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/sdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/session"
	"github.com/boticz/ai-camera-backend/internal/gb28181/ssrc"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

const (
	// ContentTypeMANSCDP 是 GB28181 XML 报文的 Content-Type。
	ContentTypeMANSCDP = "Application/MANSCDP+xml"
	// ContentTypeSDP 是 INVITE 携带的 SDP。
	ContentTypeSDP = "application/sdp"
	// ContentTypeMANSRTSP 是带内回放控制（INFO）的 Content-Type。
	ContentTypeMANSRTSP = "Application/MANSRTSP"
)

// 编译期断言：Service 必须实现业务层依赖的 SipController 接口。
// 接口定义在 control 包（另一个模块负责），签名漂移会在编译期暴露而不是运行时 nil。
var _ control.SipController = (*Service)(nil)

// CallTarget 是一次 INVITE 的目标。
type CallTarget struct {
	Device      *model.Device
	ChannelID   string // 通道国标编码，作为 INVITE 的 Request-URI 用户部分
	SessionName string // Play / Playback / Download
	Subject     string
	ContentType string // 缺省 application/sdp
	// SSRC 平台分配的 10 位 SSRC；为空时从 sdpBody 的 y= 解析。
	SSRC string
}

// Service 是 SIP 信令服务。
type Service struct {
	cfg Config
	log *slog.Logger

	ua       *sipgo.UserAgent
	server   *sipgo.Server
	client   *sipgo.Client
	dialogUA *sipgo.DialogUA

	auth     *auth.Authenticator
	devices  *device.Manager
	sessions *session.Manager
	ssrc     *ssrc.Allocator

	aor     sip.Uri // 平台 AoR：sip:<serverID>@<domain>
	contact sip.Uri // 平台 Contact：sip:<serverID>@<advertiseIP>:<port>

	udpConn     *net.UDPConn
	tcpListener *net.TCPListener

	clientMu     sync.Mutex
	localClients map[string]*sipgo.Client

	ctx      context.Context
	cancel   context.CancelFunc
	started  bool
	startMu  sync.Mutex
	stopOnce sync.Once
	wg       sync.WaitGroup
	sn       atomic.Int64

	recordMu     sync.RWMutex
	onRecordInfo func(*model.Device, *manscdp.RecordInfoResponse)
}

// New 构造服务（不监听端口，由 Start 负责）。
func New(cfg Config) (*Service, error) {
	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}

	ua, err := sipgo.NewUA(sipgo.WithUserAgent(cfg.ServerName))
	if err != nil {
		return nil, fmt.Errorf("创建 sipgo UA 失败：%w", err)
	}
	client, err := sipgo.NewClient(ua,
		sipgo.WithClientAddr(net.JoinHostPort(cfg.AdvertiseIP, strconv.Itoa(cfg.Port))),
	)
	if err != nil {
		_ = ua.Close()
		return nil, fmt.Errorf("创建 sipgo Client 失败：%w", err)
	}
	server, err := sipgo.NewServer(ua)
	if err != nil {
		_ = client.Close()
		_ = ua.Close()
		return nil, fmt.Errorf("创建 sipgo Server 失败：%w", err)
	}

	s := &Service{
		cfg:    cfg,
		log:    cfg.Logger,
		ua:     ua,
		server: server,
		client: client,
		auth:   auth.New(auth.NewDeviceStore(), cfg.Realm, cfg.NonceSecret),
		devices: device.NewManager(device.NewGbStore(), device.Config{
			Domain:         cfg.Domain,
			DefaultExpires: cfg.RegisterExpires,
		}, cfg.Logger),
		sessions:     session.NewManager(),
		ssrc:         ssrc.New(cfg.Domain),
		localClients: map[string]*sipgo.Client{},
		aor:          sip.Uri{Scheme: "sip", User: cfg.ServerID, Host: cfg.Domain},
		contact:      sip.Uri{Scheme: "sip", User: cfg.ServerID, Host: cfg.AdvertiseIP, Port: cfg.Port},
	}
	s.dialogUA = &sipgo.DialogUA{
		Client:     client,
		ContactHDR: sip.ContactHeader{Address: s.contact},
	}
	s.registerHandlers()
	return s, nil
}

// Start 启动监听并注入 control.SipController。
//
// UDP 与 TCP 可同时监听（transport=BOTH）；任一监听失败都会回滚已启动的部分。
func (s *Service) Start(ctx context.Context) error {
	s.startMu.Lock()
	if s.started {
		s.startMu.Unlock()
		return fmt.Errorf("SIP 服务已启动")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = true
	s.startMu.Unlock()

	addr := net.JoinHostPort(s.cfg.ListenIP, strconv.Itoa(s.cfg.Port))
	switch s.cfg.Transport {
	case TransportUDP:
		if err := s.listenUDP(addr); err != nil {
			s.Stop()
			return err
		}
	case TransportTCP:
		if err := s.listenTCP(addr); err != nil {
			s.Stop()
			return err
		}
	case TransportBoth:
		if err := s.listenUDP(addr); err != nil {
			s.Stop()
			return err
		}
		if err := s.listenTCP(addr); err != nil {
			s.Stop()
			return err
		}
	}

	s.devices.Start(s.ctx)
	control.Set(s)

	s.log.Info("GB28181 信令服务已启动",
		"listen", addr, "transport", s.cfg.Transport,
		"domain", s.cfg.Domain, "server_id", s.cfg.ServerID,
		"advertise_ip", s.cfg.AdvertiseIP, "media_ip", s.cfg.MediaPublicIP,
		"rtp_proxy_port", s.cfg.RTPProxyPort)
	return nil
}

// listenUDP 自行绑定 UDP socket 再交给 sipgo。
//
// 为什么不用 Server.ListenAndServe：出向报文需要复用**同一个** socket，
// 才能让设备看到来自 5060 的指令。sipgo 的连接池以「本地地址字符串」为键，
// 自己 Listen 才能拿到确定的 LocalAddr；否则 0.0.0.0 的规范化差异会让
// 池查找落空，进而尝试二次绑定同端口而失败（address already in use）。
func (s *Service) listenUDP(addr string) error {
	uaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("解析 UDP 地址 %s 失败：%w", addr, err)
	}
	conn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		return fmt.Errorf("监听 UDP %s 失败：%w", addr, err)
	}
	s.udpConn = conn

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.ServeUDP(conn); err != nil && s.ctx != nil && s.ctx.Err() == nil {
			s.log.Error("UDP 信令监听退出", "error", err, "addr", addr)
		}
	}()
	s.log.Info("UDP 信令监听中", "addr", conn.LocalAddr().String())
	return nil
}

// listenTCP 绑定 TCP 监听。
func (s *Service) listenTCP(addr string) error {
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return fmt.Errorf("解析 TCP 地址 %s 失败：%w", addr, err)
	}
	l, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return fmt.Errorf("监听 TCP %s 失败：%w", addr, err)
	}
	s.tcpListener = l

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.ServeTCP(l); err != nil && s.ctx != nil && s.ctx.Err() == nil {
			s.log.Error("TCP 信令监听退出", "error", err, "addr", addr)
		}
	}()
	s.log.Info("TCP 信令监听中", "addr", l.Addr().String())
	return nil
}

// Stop 停止监听、结束会话并释放资源。可重复调用。
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.sessions != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			s.sessions.StopAll(ctx)
			cancel()
		}
		if s.udpConn != nil {
			_ = s.udpConn.Close()
		}
		if s.tcpListener != nil {
			_ = s.tcpListener.Close()
		}
		if s.server != nil {
			_ = s.server.Close()
		}
		if s.client != nil {
			_ = s.client.Close()
		}
		if s.ua != nil {
			_ = s.ua.Close()
		}
		if s.devices != nil {
			s.devices.Stop()
		}
		s.wg.Wait()
		if s.devices != nil {
			if n := s.devices.DroppedLogs(); n > 0 {
				s.log.Warn("退出时仍有信令日志被丢弃", "dropped", n)
			}
		}
		s.log.Info("GB28181 信令服务已停止")
	})
}

// ────────────────────────────────────────────────────────────────
// 对外访问器（Wave 2：播放 / 云台 / 回放直接使用）
// ────────────────────────────────────────────────────────────────

// Config 返回运行参数快照。
func (s *Service) Config() Config { return s.cfg }

// Sessions 返回会话管理器（停止播放、回放控制用）。
func (s *Service) Sessions() *session.Manager { return s.sessions }

// Devices 返回设备管理器（端点查询、手动刷新状态用）。
func (s *Service) Devices() *device.Manager { return s.devices }

// SSRC 返回 SSRC 分配器（点播编排用）。
func (s *Service) SSRC() *ssrc.Allocator { return s.ssrc }

// Domain 返回 SIP 域。
func (s *Service) Domain() string { return s.cfg.Domain }

// MediaPublicIP 返回 SDP 通告的收流 IP。
func (s *Service) MediaPublicIP() string { return s.cfg.MediaPublicIP }

// RTPProxyPort 返回 ZLM 单端口收流端口。
func (s *Service) RTPProxyPort() int { return s.cfg.RTPProxyPort }

// SetRecordInfoHandler 注册录像查询响应的处理回调（Wave 2 的回放模块用）。
//
// 本期不落库：回放检索结果的消费方是后续模块，这里只做转发，
// 避免 sipsvc 与尚未实现的业务层强耦合。
func (s *Service) SetRecordInfoHandler(f func(*model.Device, *manscdp.RecordInfoResponse)) {
	s.recordMu.Lock()
	s.onRecordInfo = f
	s.recordMu.Unlock()
}

// AllocateSSRC 分配一个实时点播 SSRC，并返回 ZLM 流名。
func (s *Service) AllocateSSRC() (ssrcStr, stream string, err error) {
	ssrcStr, err = s.ssrc.Allocate(true)
	if err != nil {
		return "", "", err
	}
	stream, err = ssrc.ToHexStream(ssrcStr)
	if err != nil {
		s.ssrc.Release(ssrcStr)
		return "", "", err
	}
	return ssrcStr, stream, nil
}

// ReleaseSSRC 归还 SSRC（停止播放时调用）。
func (s *Service) ReleaseSSRC(ssrcStr string) { s.ssrc.Release(ssrcStr) }

// BuildPlaySDP 构造点播 INVITE 的 SDP。
//
// 设备级 sdp_ip 优先于全局 media.public_ip（NAT / 多网卡场景，设计 §7.2）。
// o= 的用户名按契约 §15.5b 填设备国标编码。
func (s *Service) BuildPlaySDP(dev *model.Device, ssrcStr, sessionName string) string {
	ip := s.cfg.MediaPublicIP
	deviceID := ""
	if dev != nil {
		if strings.TrimSpace(dev.SDPIP) != "" {
			ip = strings.TrimSpace(dev.SDPIP)
		}
		deviceID = dev.GbDeviceID
	}
	return sdp.BuildPlay(sdp.PlayParams{
		OriginUser:  deviceID,
		ServerID:    s.cfg.ServerID,
		IP:          ip,
		Port:        s.cfg.RTPProxyPort,
		SSRC:        ssrcStr,
		SessionName: sessionName,
	})
}

// DeviceCharset 返回设备的 XML 字符集。
func (s *Service) DeviceCharset(dev *model.Device) string {
	if dev != nil && strings.TrimSpace(dev.Charset) != "" {
		return charset.Normalize(dev.Charset)
	}
	return s.cfg.DefaultCharset
}

// NewCallID 生成一个可读、可关联的 Call-ID。
func (s *Service) NewCallID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", s.cfg.ServerID, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%d-%s", s.cfg.ServerID, time.Now().UnixNano(), hex.EncodeToString(b[:]))
}

// ────────────────────────────────────────────────────────────────
// 出向：MESSAGE / 查询 / 订阅 / INVITE
// ────────────────────────────────────────────────────────────────

// SendMessage 发送带外 MESSAGE。
//
// body 通常是 UTF-8 的 MANSCDP XML，本方法负责按设备 charset 转码并补/改 XML 声明。
// 两种特殊情况按「已编码」原样发送（见 encodeOutboundBody）：
//   - PTZCmd 是 8 字节二进制，含 0xA5 等非法 UTF-8 字节；
//   - 调用方已经用 manscdp.Marshal(..., deviceCharset) 编好字节。
//
// 这是 control.SipController 的实现：PTZ、录像控制、设备控制、目录查询都走它。
func (s *Service) SendMessage(ctx context.Context, dev *model.Device, body string) error {
	cmdType := ""
	if env, err := manscdp.Decode([]byte(body), s.DeviceCharset(dev)); err == nil {
		cmdType = env.CmdType
	} else {
		// PTZ 报文含二进制字节，Decode 会按设计拒绝；日志仍要能标出 CmdType。
		cmdType = extractCmdType(body)
	}
	return s.sendMANSCDP(ctx, dev, body, cmdType)
}

// QueryCatalog 向设备发 Catalog 查询。
func (s *Service) QueryCatalog(ctx context.Context, dev *model.Device) error {
	return s.query(ctx, dev, manscdp.CmdCatalog, &manscdp.Query{
		CmdType: manscdp.CmdCatalog, SN: s.nextSN(), DeviceID: devID(dev),
	})
}

// QueryDeviceInfo 向设备发 DeviceInfo 查询。
func (s *Service) QueryDeviceInfo(ctx context.Context, dev *model.Device) error {
	return s.query(ctx, dev, manscdp.CmdDeviceInfo, &manscdp.Query{
		CmdType: manscdp.CmdDeviceInfo, SN: s.nextSN(), DeviceID: devID(dev),
	})
}

// QueryDeviceStatus 向设备发 DeviceStatus 查询。
func (s *Service) QueryDeviceStatus(ctx context.Context, dev *model.Device) error {
	return s.query(ctx, dev, manscdp.CmdDeviceStatus, &manscdp.Query{
		CmdType: manscdp.CmdDeviceStatus, SN: s.nextSN(), DeviceID: devID(dev),
	})
}

// QueryRecordInfo 向设备发录像查询。
func (s *Service) QueryRecordInfo(ctx context.Context, dev *model.Device, channelID, startTime, endTime string) error {
	if channelID == "" {
		channelID = devID(dev)
	}
	return s.query(ctx, dev, manscdp.CmdRecordInfo, &manscdp.RecordInfoQuery{
		CmdType:   manscdp.CmdRecordInfo,
		SN:        s.nextSN(),
		DeviceID:  channelID,
		StartTime: startTime,
		EndTime:   endTime,
	})
}

// QueryConfigDownload 向设备发配置下载查询（尽力实现）。
func (s *Service) QueryConfigDownload(ctx context.Context, dev *model.Device, configType string) error {
	return s.query(ctx, dev, manscdp.CmdConfigDownload, &manscdp.ConfigDownloadQuery{
		CmdType:    manscdp.CmdConfigDownload,
		SN:         s.nextSN(),
		DeviceID:   devID(dev),
		ConfigType: configType,
	})
}

// QueryPreset 发出预置位查询。
func (s *Service) QueryPreset(ctx context.Context, dev *model.Device, channelID string) error {
	if channelID == "" {
		channelID = devID(dev)
	}
	return s.query(ctx, dev, manscdp.CmdPresetQuery, &manscdp.Query{
		CmdType: manscdp.CmdPresetQuery, SN: s.nextSN(), DeviceID: channelID,
	})
}

// query 序列化并发送一条查询。
func (s *Service) query(ctx context.Context, dev *model.Device, cmdType string, v any) error {
	body, err := manscdp.MarshalString(v, charset.UTF8)
	if err != nil {
		return err
	}
	return s.sendMANSCDP(ctx, dev, body, cmdType)
}

// sendMANSCDP 编码并按设备字符集发送一条 MANSCDP 报文。
func (s *Service) sendMANSCDP(ctx context.Context, dev *model.Device, xmlUTF8, cmdType string) error {
	if dev == nil {
		return fmt.Errorf("设备为 nil")
	}
	payload, err := encodeOutboundBody(xmlUTF8, s.DeviceCharset(dev))
	if err != nil {
		return err
	}
	req := s.newRequest(sip.MESSAGE, s.deviceURI(dev))
	req.AppendHeader(sip.NewHeader("Content-Type", ContentTypeMANSCDP))
	req.SetBody(payload)

	resp, err := s.send(ctx, dev, req, cmdType)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("设备 %s 拒绝 MESSAGE：%d %s", dev.GbDeviceID, resp.StatusCode, resp.Reason)
	}
	return nil
}

// encodeOutboundBody 决定一条 MESSAGE 正文该怎么上线。
//
// 正常路径：UTF-8 文本 → 按设备字符集转码并同步声明。
//
// 例外路径（原样发送）：正文含二进制 PTZCmd 或已被调用方按设备字符集编码过，
// 表现为「不是合法 UTF-8」。此时再走一次 GB18030 编码会把二进制指令与
// 已编码的中文一起破坏，且往往是静默的 —— 所以以「是否合法 UTF-8」判定。
// 这类报文必须自带 XML 声明，否则无法确认编码，宁可直接报错。
func encodeOutboundBody(body, cs string) ([]byte, error) {
	raw := []byte(body)
	if utf8.Valid(raw) {
		return charset.EncodeForDevice(body, cs)
	}
	if charset.Declared(raw) == "" {
		return nil, fmt.Errorf("报文不是合法 UTF-8 且没有 XML 声明，无法确定编码（设备 charset=%s）", cs)
	}
	return raw, nil
}

// Subscribe 发送订阅（catalog / alarm / mobile_position）。
func (s *Service) Subscribe(ctx context.Context, dev *model.Device, subType string, interval int) error {
	return s.subscription(ctx, dev, subType, interval)
}

// Unsubscribe 取消订阅（Expires: 0）。
func (s *Service) Unsubscribe(ctx context.Context, dev *model.Device, subType string) error {
	return s.subscription(ctx, dev, subType, 0)
}

// subscription 实现 SUBSCRIBE 的建立与取消。
func (s *Service) subscription(ctx context.Context, dev *model.Device, subType string, interval int) error {
	if dev == nil {
		return fmt.Errorf("设备为 nil")
	}
	event, accept, err := subscriptionEvent(subType)
	if err != nil {
		return err
	}
	if interval <= 0 {
		interval = 0
	}
	req := s.newRequest(sip.SUBSCRIBE, s.deviceURI(dev))
	req.AppendHeader(sip.NewHeader("Event", event))
	if accept != "" {
		req.AppendHeader(sip.NewHeader("Accept", accept))
	}
	exp := sip.ExpiresHeader(interval)
	req.AppendHeader(&exp)

	resp, err := s.send(ctx, dev, req, "SUBSCRIBE:"+subType)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("设备 %s 拒绝 SUBSCRIBE(%s)：%d %s", dev.GbDeviceID, subType, resp.StatusCode, resp.Reason)
	}
	return nil
}

// subscriptionEvent 把订阅类型映射成 SIP Event/Accept 头。
func subscriptionEvent(subType string) (event, accept string, err error) {
	switch strings.ToLower(strings.TrimSpace(subType)) {
	case model.GbSubscribeCatalog:
		return "Catalog", "Application/MANSCDP+xml", nil
	case model.GbSubscribeAlarm:
		return "Alarm", "Application/MANSCDP+xml", nil
	case model.GbSubscribeMobilePosition:
		// GB28181 的移动位置订阅复用 RFC 3856 的 presence 事件。
		return "presence", "Application/MANSCDP+xml", nil
	default:
		return "", "", fmt.Errorf("不支持的订阅类型 %q（catalog / alarm / mobile_position）", subType)
	}
}

// SendInvite 发送 INVITE、等待 200 OK、发送 ACK，返回可复用的会话。
//
// sdpBody 是平台 SDP（见 BuildPlaySDP）；body 仅在 sdpBody 为空时作为正文使用
// （个别设备/场景需要额外正文）。失败时会释放底层对话框，不会泄漏事务。
func (s *Service) SendInvite(ctx context.Context, target CallTarget, sdpBody []byte, body string) (*session.Dialog, error) {
	dev := target.Device
	if dev == nil {
		return nil, fmt.Errorf("INVITE 目标设备为 nil")
	}
	ep, ok := s.devices.Endpoint(dev.GbDeviceID)
	if !ok {
		return nil, fmt.Errorf("设备 %s 尚未注册（无信令地址），无法发起 INVITE", dev.GbDeviceID)
	}
	channelID := strings.TrimSpace(target.ChannelID)
	if channelID == "" {
		channelID = dev.GbDeviceID
	}

	host, port, err := splitHostPort(ep.Addr)
	if err != nil {
		return nil, fmt.Errorf("设备 %s 的信令地址非法（%s）：%w", dev.GbDeviceID, ep.Addr, err)
	}
	recipient := sip.Uri{Scheme: "sip", User: channelID, Host: host, Port: port}

	req := sip.NewRequest(sip.INVITE, recipient)
	req.SetDestination(ep.Addr)
	req.SetTransport(upperTransport(ep.Transport))

	from := &sip.FromHeader{Address: s.aor}
	params := sip.NewParams()
	from.Params = params.Add("tag", sip.GenerateTagN(16))
	req.AppendHeader(from)
	req.AppendHeader(&sip.ToHeader{Address: recipient})
	req.AppendHeader(&sip.ContactHeader{Address: s.contact})

	ct := target.ContentType
	if ct == "" {
		ct = ContentTypeSDP
	}
	req.AppendHeader(sip.NewHeader("Content-Type", ct))
	if target.Subject != "" {
		req.AppendHeader(sip.NewHeader("Subject", target.Subject))
	}
	callID := s.NewCallID()
	callIDH := sip.CallIDHeader(callID)
	req.AppendHeader(&callIDH)

	payload := sdpBody
	if len(payload) == 0 && body != "" {
		payload = []byte(body)
	}
	if len(payload) > 0 {
		req.SetBody(payload)
	}

	// INITE 的构建选项：补全缺失头，并复用监听 socket（UDP）以保证源端口是 5060。
	opt := func(c *sipgo.Client, r *sip.Request) error {
		if err := sipgo.ClientRequestBuild(c, r); err != nil {
			return err
		}
		if strings.EqualFold(ep.Transport, TransportUDP) {
			if la, ok := s.outboundLaddr(); ok {
				r.Laddr = la
			}
		}
		return nil
	}

	start := time.Now()
	dlgSess, err := s.dialogUA.WriteInvite(ctx, req, opt)
	if err != nil {
		s.logInvite(dev, ep.Addr, callID, target, 0, "failed", err.Error(), start)
		return nil, fmt.Errorf("发送 INVITE 失败：%w", err)
	}
	if err := dlgSess.WaitAnswer(ctx, sipgo.AnswerOptions{}); err != nil {
		_ = dlgSess.Close()
		s.logInvite(dev, ep.Addr, callID, target, 0, "failed", "等待 200 OK 失败："+err.Error(), start)
		return nil, fmt.Errorf("设备 %s 未接受 INVITE：%w", dev.GbDeviceID, err)
	}
	res := dlgSess.InviteResponse
	if res == nil || !res.IsSuccess() {
		_ = dlgSess.Close()
		code := 0
		reason := "无响应"
		if res != nil {
			code = res.StatusCode
			reason = res.Reason
		}
		s.logInvite(dev, ep.Addr, callID, target, code, "failed", "设备拒绝 INVITE："+reason, start)
		return nil, fmt.Errorf("设备 %s 拒绝 INVITE：%d %s", dev.GbDeviceID, code, reason)
	}
	if err := dlgSess.Ack(ctx); err != nil {
		_ = dlgSess.Close()
		s.logInvite(dev, ep.Addr, callID, target, res.StatusCode, "failed", "发送 ACK 失败："+err.Error(), start)
		return nil, fmt.Errorf("发送 ACK 失败：%w", err)
	}

	ssrcStr := strings.TrimSpace(target.SSRC)
	if ssrcStr == "" {
		ssrcStr = parseSSRCFromSDP(string(payload))
	}
	stream := ""
	if ssrcStr != "" {
		stream, _ = ssrc.ToHexStream(ssrcStr)
	}
	sessionName := target.SessionName
	if sessionName == "" {
		sessionName = sdp.SessionNamePlay
	}

	d := session.New(
		callID,
		tagOfFrom(req.From()),
		tagOfTo(res.To()),
		dev.GbDeviceID,
		channelID,
		ep.Addr,
		ssrcStr,
		stream,
		sessionName,
		string(res.Body()),
		dlgSess,
	)
	s.sessions.Add(d)
	s.logInvite(dev, ep.Addr, callID, target, res.StatusCode, "success", "", start)
	return d, nil
}

// send 是出向请求的公共入口：选定目标地址、源地址与传输，并把结果写日志。
func (s *Service) send(ctx context.Context, dev *model.Device, req *sip.Request, cmdType string) (*sip.Response, error) {
	if dev == nil {
		return nil, fmt.Errorf("设备为 nil")
	}
	ep, ok := s.devices.Endpoint(dev.GbDeviceID)
	if !ok {
		return nil, fmt.Errorf("设备 %s 尚未注册（无信令地址）", dev.GbDeviceID)
	}
	req.SetDestination(ep.Addr)
	transport := upperTransport(ep.Transport)
	req.SetTransport(transport)

	client := s.client
	if ip := strings.TrimSpace(dev.LocalIP); ip != "" && ip != strings.TrimSpace(s.cfg.ListenIP) {
		// 设备级源 IP 覆盖：单独绑一块本地网卡，避免心跳/指令从错误的网卡出去。
		client = s.clientForLocalIP(ip)
	} else if transport == TransportUDP {
		if la, ok := s.outboundLaddr(); ok {
			req.Laddr = la
		}
	}

	start := time.Now()
	resp, err := client.Do(ctx, req)
	entry := model.GbSipLog{
		GbDeviceID: dev.GbDeviceID,
		Direction:  "out",
		Method:     req.Method.String(),
		CmdType:    cmdType,
		RemoteAddr: ep.Addr,
		LocalAddr:  s.localAddrString(),
		CallID:     callIDOfRequest(req),
		Content:    truncate(string(req.Body()), 4000),
		ElapsedMS:  time.Since(start).Milliseconds(),
	}
	if err != nil {
		entry.Result = "failed"
		entry.Reason = err.Error()
		s.devices.LogSip(entry)
		return nil, err
	}
	entry.StatusCode = int64(resp.StatusCode)
	entry.Result = "success"
	if resp.StatusCode >= 400 {
		entry.Result = "failed"
		entry.Reason = resp.Reason
	}
	s.devices.LogSip(entry)
	return resp, nil
}

// clientForLocalIP 为设备级源 IP 覆盖创建（并缓存）一个客户端。
//
// WithClientConnectionAddr 会把每个请求的 Laddr 设成该 IP 的临时端口，
// 传输层据此绑一块独立 socket；这是「多网卡」最小的实现代价。
func (s *Service) clientForLocalIP(ip string) *sipgo.Client {
	s.clientMu.Lock()
	defer s.clientMu.Unlock()
	if c, ok := s.localClients[ip]; ok {
		return c
	}
	c, err := sipgo.NewClient(s.ua,
		sipgo.WithClientConnectionAddr(net.JoinHostPort(ip, "0")),
		sipgo.WithClientHostname(ip),
	)
	if err != nil {
		s.log.Warn("按设备 local_ip 创建客户端失败，回落到默认客户端", "local_ip", ip, "error", err)
		return s.client
	}
	s.localClients[ip] = c
	return c
}

// outboundLaddr 返回 UDP 监听 socket 的本地地址。
//
// 出向报文复用它，设备看到的就是 5060 端口 —— 与 WVP 的行为一致，
// 也满足 RFC 3581 的「从收到报文的地方回」语义。
func (s *Service) outboundLaddr() (sip.Addr, bool) {
	if s.udpConn == nil {
		return sip.Addr{}, false
	}
	la, ok := s.udpConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return sip.Addr{}, false
	}
	return sip.Addr{IP: la.IP, Port: la.Port}, true
}

// localAddrString 返回日志用的本地地址。
func (s *Service) localAddrString() string {
	if s.udpConn != nil {
		return s.udpConn.LocalAddr().String()
	}
	if s.tcpListener != nil {
		return s.tcpListener.Addr().String()
	}
	return net.JoinHostPort(s.cfg.AdvertiseIP, strconv.Itoa(s.cfg.Port))
}

// deviceURI 返回发往设备的 Request-URI。
//
// 优先用设备当前的信令地址（IP:port），设备离线时回落到域。
func (s *Service) deviceURI(dev *model.Device) sip.Uri {
	uri := sip.Uri{Scheme: "sip", User: devID(dev), Host: s.cfg.Domain}
	if ep, ok := s.devices.Endpoint(dev.GbDeviceID); ok {
		if host, port, err := splitHostPort(ep.Addr); err == nil {
			uri.Host = host
			uri.Port = port
		}
	}
	return uri
}

// newRequest 构造一条平台 → 设备的请求，填好 From / To / Contact。
//
// Call-ID / CSeq / Max-Forwards / Via 交给 sipgo 的 clientRequestBuildReq 补齐，
// 因此这里不提前设置它们。
func (s *Service) newRequest(method sip.RequestMethod, recipient sip.Uri) *sip.Request {
	req := sip.NewRequest(method, recipient)
	from := &sip.FromHeader{Address: s.aor}
	params := sip.NewParams()
	from.Params = params.Add("tag", sip.GenerateTagN(16))
	req.AppendHeader(from)
	req.AppendHeader(&sip.ToHeader{Address: recipient})
	req.AppendHeader(&sip.ContactHeader{Address: s.contact})
	return req
}

// nextSN 生成 MANSCDP 的 SN（1–65535 循环）。
func (s *Service) nextSN() int {
	v := s.sn.Add(1) % 65535
	if v <= 0 {
		v += 65535
	}
	return int(v)
}

// logInvite 记录 INVITE 出向结果。
func (s *Service) logInvite(dev *model.Device, remote, callID string, target CallTarget, status int, result, reason string, start time.Time) {
	s.devices.LogSip(model.GbSipLog{
		GbDeviceID:  dev.GbDeviceID,
		GbChannelID: target.ChannelID,
		Direction:   "out",
		Method:      sip.INVITE.String(),
		CmdType:     target.SessionName,
		StatusCode:  int64(status),
		RemoteAddr:  remote,
		LocalAddr:   s.localAddrString(),
		CallID:      callID,
		Result:      result,
		Reason:      reason,
		ElapsedMS:   time.Since(start).Milliseconds(),
	})
}

// ────────────────────────────────────────────────────────────────
// 小工具
// ────────────────────────────────────────────────────────────────

func devID(dev *model.Device) string {
	if dev == nil {
		return ""
	}
	return dev.GbDeviceID
}

func upperTransport(t string) string {
	if strings.EqualFold(strings.TrimSpace(t), TransportTCP) {
		return TransportTCP
	}
	return TransportUDP
}

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("端口 %q 非法：%w", portStr, err)
	}
	return host, port, nil
}

func tagOfFrom(h *sip.FromHeader) string {
	if h == nil {
		return ""
	}
	v, _ := h.Params.Get("tag")
	return v
}

func tagOfTo(h *sip.ToHeader) string {
	if h == nil {
		return ""
	}
	v, _ := h.Params.Get("tag")
	return v
}

// parseSSRCFromSDP 从 SDP 的 y= 行取 SSRC（平台自己刚生成的，正常都应存在）。
func parseSSRCFromSDP(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "y=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "y="))
		}
	}
	return ""
}

func callIDOfRequest(req *sip.Request) string {
	if req == nil {
		return ""
	}
	if h := req.CallID(); h != nil {
		return string(*h)
	}
	return ""
}

// extractCmdType 从报文文本里硬取 CmdType（仅用于日志标注，不参与路由）。
func extractCmdType(body string) string {
	const open, close = "<CmdType>", "</CmdType>"
	i := strings.Index(body, open)
	if i < 0 {
		return ""
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:j])
}

// truncate 截断日志正文，避免把 UDP 大报文整段写进日志表。
func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
