// Package platform 实现 GB28181 级联中**下级平台**这一侧的信令能力。
//
// 职责边界（对齐设计 §14 的模块划分）：
//   - 向上级平台注册（REGISTER + Digest）、按 expires 续期、按 keep_timeout 发心跳；
//   - 接受上级的 Catalog / DeviceInfo / DeviceStatus 查询并应答；
//   - 接受上级的 INVITE（实时点播），把本平台已有的 ZLM 流转推给上级；
//   - 目录变更后按需主动推送 Notify Catalog。
//
// ★ 为什么级联使用独立端口（config.gb28181.cascade.port，默认 5061）：
// 5060 上的 `sipserver.Service` 维护的是「设备 → 平台」的注册表、事务与
// source-address 绑定；级联则是「本平台 → 上级」的独立身份（From/Contact 都是
// 平台自己的国标编码）。两者混在同一个 listener 上会有三个直接后果：
//  1. 上级的 REGISTER/MESSAGE 会进入设备侧的鉴权与白名单逻辑，永远 403；
//  2. 出向报文复用 5060 socket 后，排障时分不清某个事务属于设备侧还是级联侧；
//  3. 级联重注册/心跳失败会与设备注册共享事务状态，故障互相放大。
//
// 因此这里自建 sipgo UA/Server/Client，与设备侧完全隔离。
package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/ssrc"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// 默认值。FromAppConfig 会把 cascade.port 的 0 值补成 5061；
// 直接 New(Config{Port: 0}) 仅用于单测，此时由操作系统分配临时端口。
const (
	defaultCascadePort = 5061
	defaultSIPDomain   = "3402000000"
)

// Config 是级联信令的本地运行参数。
type Config struct {
	Enabled     bool
	ListenIP    string
	Port        int
	AdvertiseIP string
	// Domain / ServerID 来自设备侧的 SIP 配置，用于 From/AoR 的兜底。
	Domain     string
	ServerID   string
	ServerName string
	// Charset 是上级平台未配置 character_set 时的缺省 XML 编码。
	Charset string
	Logger  *slog.Logger
}

// FromAppConfig 从全局配置推导级联配置。
func FromAppConfig(c *config.Config, logger *slog.Logger) (Config, error) {
	if c == nil {
		return Config{}, fmt.Errorf("配置为 nil")
	}
	cc := c.GB28181.Cascade
	cfg := Config{
		Enabled:     cc.Enabled,
		ListenIP:    strings.TrimSpace(cc.ListenIP),
		Port:        cc.Port,
		AdvertiseIP: strings.TrimSpace(cc.AdvertiseIP),
		Domain:      strings.TrimSpace(c.GB28181.SIP.Domain),
		ServerID:    strings.TrimSpace(c.GB28181.SIP.ServerID),
		ServerName:  strings.TrimSpace(c.GB28181.SIP.ServerName),
		Charset:     charset.Normalize(c.GB28181.Charset),
		Logger:      logger,
	}
	if cfg.Port == 0 {
		cfg.Port = defaultCascadePort
	}
	if err := cfg.applyDefaults(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyDefaults 补齐缺省值并校验。Port 允许为 0（测试用临时端口）。
func (c *Config) applyDefaults() error {
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("gb28181.cascade.port 非法：%d", c.Port)
	}
	if c.Domain == "" {
		c.Domain = defaultSIPDomain
	}
	if c.ServerID == "" {
		c.ServerID = c.Domain + "2000000001"
	}
	if c.ServerName == "" {
		c.ServerName = "AI Camera Cascade"
	}
	if c.Charset == "" {
		c.Charset = model.GbCharsetGB2312
	}
	if c.AdvertiseIP == "" {
		c.AdvertiseIP = resolveAdvertiseIP(c.ListenIP)
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// resolveAdvertiseIP 选出上级可达的本机地址。
//
// listen_ip 是具体地址时直接使用；0.0.0.0 时用一次 UDP 拨号让内核选默认网卡，
// 失败回落到 127.0.0.1（本地联调仍可用）。与 sipserver 的策略一致，
// 但级联需要独立的出口地址，不能复用设备侧的服务实例。
func resolveAdvertiseIP(listenIP string) string {
	if listenIP != "" && listenIP != "0.0.0.0" && listenIP != "::" {
		return listenIP
	}
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 500*time.Millisecond)
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil && !addr.IP.IsUnspecified() {
			return addr.IP.String()
		}
	}
	return "127.0.0.1"
}

// MediaSender 是 INVITE 推流所需的最小媒体能力。
//
// 抽成接口的唯一目的是单测：级联用例不启动 ZLM，用假实现断言
// 「StartSendRTP 被调用、目标地址/端口/SSRC 正确、BYE 时被停止」。
type MediaSender interface {
	StartSendRTP(ctx context.Context, p mediaserver.SendRTPParam) (*mediaserver.SendRTPResult, error)
	StopSendRTP(ctx context.Context, app, stream string, ssrc uint32) error
}

// MediaFactory 按上级平台解析媒体节点（默认取 platform.server_id，未配置则默认节点）。
type MediaFactory func(ctx context.Context, p *model.GbPlatform) (MediaSender, error)

// Status 是一个上级平台的运行时状态快照。
//
// 模型里没有「注册时间 / 心跳时间」列（见 model.GbPlatform，只有 status），
// 因此这些时间只在内存中维护，进程重启后由首次注册重建；不额外加列，
// 避免为纯运维信息扩表。持久化的只有 status（ON/OFF）。
type Status struct {
	PlatformID    int64      `json:"platform_id"`
	Name          string     `json:"name"`
	ServerGBID    string     `json:"server_gb_id"`
	DeviceGBID    string     `json:"device_gb_id"`
	Online        bool       `json:"online"`
	State         string     `json:"state"` // registering / online / offline
	LastRegister  *time.Time `json:"last_register,omitempty"`
	LastKeepalive *time.Time `json:"last_keepalive,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
}

// Snapshot 是级联服务的整体状态。
type Snapshot struct {
	Running   bool     `json:"running"`
	Listening string   `json:"listening"`
	Platforms []Status `json:"platforms"`
}

// Service 是级联信令服务（一个进程只有一个实例）。
type Service struct {
	cfg    Config
	log    *slog.Logger
	source Source

	ua       *sipgo.UserAgent
	server   *sipgo.Server
	client   *sipgo.Client
	dialogUA *sipgo.DialogUA

	udpConn     *net.UDPConn
	tcpListener *net.TCPListener
	listenAddr  string

	ctx      context.Context
	cancel   context.CancelFunc
	startMu  sync.Mutex
	started  bool
	stopOnce sync.Once
	wg       sync.WaitGroup

	peerMu   sync.RWMutex
	peers    map[int64]*peer
	reloadMu sync.Mutex

	sessionMu sync.Mutex
	sessions  map[string]*inviteSession

	ssrc *ssrc.Allocator
	sn   atomic.Int64

	mediaFactory MediaFactory
}

// Option 是 Service 的可选构造参数（单测注入用）。
type Option func(*Service)

// WithMediaFactory 注入媒体发送实现。
func WithMediaFactory(f MediaFactory) Option {
	return func(s *Service) { s.mediaFactory = f }
}

// New 构造级联服务（不监听端口，由 Start 负责）。
//
// src 为 nil 时使用数据库实现；单测可传入假实现，完全脱离数据库与 ZLM。
func New(cfg Config, src Source, opts ...Option) (*Service, error) {
	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}
	if src == nil {
		src = NewDBSource()
	}

	ua, err := sipgo.NewUA(sipgo.WithUserAgent(cfg.ServerName))
	if err != nil {
		return nil, fmt.Errorf("创建级联 sipgo UA 失败：%w", err)
	}
	server, err := sipgo.NewServer(ua)
	if err != nil {
		_ = ua.Close()
		return nil, fmt.Errorf("创建级联 sipgo Server 失败：%w", err)
	}

	s := &Service{
		cfg:      cfg,
		log:      cfg.Logger,
		source:   src,
		ua:       ua,
		server:   server,
		peers:    map[int64]*peer{},
		sessions: map[string]*inviteSession{},
		ssrc:     ssrc.New(cfg.Domain),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.registerHandlers()
	return s, nil
}

// Start 绑定端口、启动 sipgo 监听，并按数据库配置启动各上级平台的注册协程。
func (s *Service) Start(ctx context.Context) error {
	s.startMu.Lock()
	if s.started {
		s.startMu.Unlock()
		return fmt.Errorf("级联服务已启动")
	}
	if !s.cfg.Enabled {
		s.startMu.Unlock()
		return fmt.Errorf("级联未启用：gb28181.cascade.enabled=false")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = true
	s.startMu.Unlock()

	addr := net.JoinHostPort(s.cfg.ListenIP, strconv.Itoa(s.cfg.Port))
	uaddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		s.Stop()
		return fmt.Errorf("解析级联 UDP 地址 %s 失败：%w", addr, err)
	}
	udpConn, err := net.ListenUDP("udp", uaddr)
	if err != nil {
		s.Stop()
		return fmt.Errorf("监听级联 UDP %s 失败：%w", addr, err)
	}
	s.udpConn = udpConn

	// Port=0 时用系统分配的真实端口回填，Contact/Via 才能指向正确地址。
	ephemeral := s.cfg.Port == 0
	if ephemeral {
		if la, ok := udpConn.LocalAddr().(*net.UDPAddr); ok {
			s.cfg.Port = la.Port
		}
	}
	s.listenAddr = net.JoinHostPort(s.cfg.AdvertiseIP, strconv.Itoa(s.cfg.Port))

	// TCP 仅在显式端口时监听：Port=0 时 UDP 已拿到临时端口，
	// 再让 TCP 去绑同一端口会与 UDP 的随机结果不一致（Windows 上还可能
	// 因为该端口落在系统排除区间而直接 bind 失败），且单测无需 TCP。
	if !ephemeral {
		laddr, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(s.cfg.ListenIP, strconv.Itoa(s.cfg.Port)))
		if err != nil {
			s.Stop()
			return fmt.Errorf("解析级联 TCP 地址失败：%w", err)
		}
		l, err := net.ListenTCP("tcp", laddr)
		if err != nil {
			s.Stop()
			return fmt.Errorf("监听级联 TCP %s 失败：%w", laddr, err)
		}
		s.tcpListener = l
	}

	client, err := sipgo.NewClient(s.ua,
		sipgo.WithClientAddr(net.JoinHostPort(s.cfg.AdvertiseIP, strconv.Itoa(s.cfg.Port))),
	)
	if err != nil {
		s.Stop()
		return fmt.Errorf("创建级联 sipgo Client 失败：%w", err)
	}
	s.client = client
	s.dialogUA = &sipgo.DialogUA{
		Client:     client,
		ContactHDR: sip.ContactHeader{Address: s.contactURI(s.cfg.ServerID, s.cfg.Domain)},
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.server.ServeUDP(udpConn); err != nil && s.ctx.Err() == nil {
			s.log.Error("级联 UDP 监听退出", "error", err, "addr", addr)
		}
	}()
	if s.tcpListener != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.server.ServeTCP(s.tcpListener); err != nil && s.ctx.Err() == nil {
				s.log.Error("级联 TCP 监听退出", "error", err, "addr", s.tcpListener.Addr())
			}
		}()
	}

	// 配置加载失败不阻断监听：端口已经可用，管理员修复数据库后可通过
	// 「启动/重载」再次触发，比让整个进程启动失败更符合运维预期。
	if err := s.Reload(s.ctx); err != nil {
		s.log.Warn("加载级联平台配置失败（服务仍在监听）", "error", err)
	}

	s.log.Info("GB28181 级联信令服务已启动",
		"listen", udpConn.LocalAddr().String(), "advertise", s.listenAddr,
		"domain", s.cfg.Domain, "server_id", s.cfg.ServerID)
	return nil
}

// Stop 停止注册协程、关闭监听与 SIP 资源。可重复调用。
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		// 先让注册协程退出；Stop 内部会给上级发一次 Expires:0（尽力而为）。
		s.peerMu.Lock()
		peers := make([]*peer, 0, len(s.peers))
		for _, p := range s.peers {
			peers = append(peers, p)
		}
		s.peers = map[int64]*peer{}
		s.peerMu.Unlock()
		// 各平台的注销互不依赖，并发执行避免 N 个失联上级把退出拖成 N×6 秒。
		var stopWG sync.WaitGroup
		for _, p := range peers {
			stopWG.Add(1)
			go func(p *peer) {
				defer stopWG.Done()
				p.stop()
			}(p)
		}
		stopWG.Wait()

		if s.cancel != nil {
			s.cancel()
		}

		// 结束所有上级点播会话：停止 ZLM 推流后再释放 SIP 对话框。
		s.sessionMu.Lock()
		sessions := make([]*inviteSession, 0, len(s.sessions))
		for _, it := range s.sessions {
			sessions = append(sessions, it)
		}
		s.sessions = map[string]*inviteSession{}
		s.sessionMu.Unlock()
		for _, it := range sessions {
			it.close(context.Background())
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
		s.wg.Wait()
		s.log.Info("GB28181 级联信令服务已停止")
	})
}

// Running 返回是否已启动。
func (s *Service) Running() bool {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	return s.started
}

// Addr 返回上级可达的 host:port（测试据此发送报文）。
func (s *Service) Addr() string { return s.listenAddr }

// Snapshot 返回运行状态。
func (s *Service) Snapshot() Snapshot {
	snap := Snapshot{
		Running:   s.Running(),
		Listening: s.listenAddr,
		Platforms: make([]Status, 0),
	}
	s.peerMu.RLock()
	defer s.peerMu.RUnlock()
	for _, p := range s.peers {
		snap.Platforms = append(snap.Platforms, p.status())
	}
	// 按 platform_id 排序，避免前端每次刷新顺序漂移。
	sort.Slice(snap.Platforms, func(i, j int) bool {
		return snap.Platforms[i].PlatformID < snap.Platforms[j].PlatformID
	})
	return snap
}

// Reload 按数据源对账：新增平台开始注册，配置变化重建协程，删除/禁用停止。
func (s *Service) Reload(ctx context.Context) error {
	if s.source == nil {
		return nil
	}
	// 串行化对账：并发 Reload 会为同一平台各建一个 peer，后者覆盖前者时
	// 前者的协程仍在跑（重复注册/重复心跳）。
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	// Reload 也可能在 Start 之前被调用（例如运维动作先于监听）：
	// 此时 s.ctx 为空，用调用方 context 作为注册协程的生命周期。
	baseCtx := s.ctx
	if baseCtx == nil {
		baseCtx = ctx
	}
	platforms, err := s.source.ListPlatforms(ctx)
	if err != nil {
		return err
	}

	seen := make(map[int64]bool, len(platforms))
	var toStart []*peer
	var toStop []*peer

	s.peerMu.Lock()
	for i := range platforms {
		p := platforms[i]
		if p.Enable != nil && !*p.Enable {
			continue
		}
		if p.Active != nil && !*p.Active {
			continue
		}
		seen[p.ID] = true
		if old, ok := s.peers[p.ID]; ok {
			if old.sameConfig(&p) {
				continue
			}
			delete(s.peers, p.ID)
			toStop = append(toStop, old)
		}
		np := newPeer(s, &p)
		s.peers[p.ID] = np
		toStart = append(toStart, np)
	}
	for id, p := range s.peers {
		if !seen[id] {
			delete(s.peers, id)
			toStop = append(toStop, p)
		}
	}
	s.peerMu.Unlock()

	var stopWG sync.WaitGroup
	for _, p := range toStop {
		stopWG.Add(1)
		go func(p *peer) {
			defer stopWG.Done()
			p.stop()
		}(p)
	}
	stopWG.Wait()
	for _, p := range toStart {
		p.start(baseCtx)
	}
	return nil
}

// NotifyCatalog 主动向上级推送一次完整目录。
//
// 推送策略选择：**目录映射变更后主动下发 MESSAGE Notify/Catalog**，
// 而不是依赖上级的 SUBSCRIBE/NOTIFY 订阅。原因：
//  1. 本项目没有为级联维护订阅生命周期（ac_gb_subscribe 记录的是设备侧订阅）；
//  2. 上级（尤其是国标测试工具）普遍接受带外 Notify，且配置了
//     auto_push_channel=true 时语义最直观；
//  3. 映射变更点集中在本服务的 CRUD 层，触发时机明确，不需要额外定时器。
//
// auto_push_channel=false 时静默跳过（避免每次编辑都打扰只用 Catalog 查询的上级）。
// force=true 表示管理员显式点击「推送目录」，忽略该开关。
func (s *Service) NotifyCatalog(ctx context.Context, platformID int64, force bool) error {
	p := s.getPeer(platformID)
	if p == nil {
		return fmt.Errorf("上级平台 %d 未启用或未加载", platformID)
	}
	plat := p.snapshot()
	if !force && (plat.AutoPushChannel == nil || !*plat.AutoPushChannel) {
		return nil
	}
	entries, err := s.source.Catalog(ctx, &plat)
	if err != nil {
		return err
	}
	return s.pushCatalog(ctx, p, entries, "Notify", s.nextSN(), nil)
}

// ReloadAndNotify 在通道/分组/区域映射变化后调用：先对账再推送当前平台的目录。
func (s *Service) ReloadAndNotify(ctx context.Context, platformID int64) error {
	if err := s.Reload(ctx); err != nil {
		return err
	}
	return s.NotifyCatalog(ctx, platformID, false)
}

// nextSN 生成 MANSCDP SN（1–65535 循环）。
func (s *Service) nextSN() int {
	v := s.sn.Add(1) % 65535
	if v <= 0 {
		v += 65535
	}
	return int(v)
}

func (s *Service) getPeer(id int64) *peer {
	s.peerMu.RLock()
	defer s.peerMu.RUnlock()
	return s.peers[id]
}

// ────────────────────────────────────────────────────────────────
// SIP 地址与小工具
// ────────────────────────────────────────────────────────────────

// localURI 是本平台在某个上级视角下的 AoR：sip:<device_gb_id>@<server_gb_domain>。
func (s *Service) localURI(deviceGBID, domain string) sip.Uri {
	if strings.TrimSpace(domain) == "" {
		domain = s.cfg.Domain
	}
	if strings.TrimSpace(deviceGBID) == "" {
		deviceGBID = s.cfg.ServerID
	}
	return sip.Uri{Scheme: "sip", User: deviceGBID, Host: domain}
}

// contactURI 是本平台在级联端口上的 Contact。
func (s *Service) contactURI(deviceGBID, _ string) sip.Uri {
	if strings.TrimSpace(deviceGBID) == "" {
		deviceGBID = s.cfg.ServerID
	}
	return sip.Uri{Scheme: "sip", User: deviceGBID, Host: s.cfg.AdvertiseIP, Port: s.cfg.Port}
}

// outboundLaddr 返回 UDP 监听 socket 的本地地址，出向报文复用它。
//
// 复用监听 socket（而不是每次新建）有两个好处：上级看到的源端口与 Contact 一致；
// sipgo 的连接池以本地地址为键，能直接命中 ServeUDP 注册的 listener 连接。
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

// applyLocalAddr 让 UDP 出向报文复用级联监听端口。
func (s *Service) applyLocalAddr(req *sip.Request) {
	if s.udpConn == nil || req == nil {
		return
	}
	if !strings.EqualFold(req.Transport(), "TCP") {
		if la, ok := s.outboundLaddr(); ok {
			req.Laddr = la
		}
	}
}

// transportOf 规整平台的 transport 字段。
func transportOf(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), model.GbSipTransportTCP) {
		return model.GbSipTransportTCP
	}
	return model.GbSipTransportUDP
}

// splitHostPort 拆分 host:port，缺端口时用 fallbackPort。
func splitHostPort(addr string, fallbackPort int) (string, int, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", 0, errors.New("地址为空")
	}
	if !strings.Contains(addr, ":") {
		return addr, fallbackPort, nil
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return "", 0, fmt.Errorf("端口 %q 非法", portStr)
	}
	return host, port, nil
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
