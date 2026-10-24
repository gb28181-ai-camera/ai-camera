package platform

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// peer 是一个上级平台的注册状态机。
//
// 每个平台独立协程：一个上级不可达不会阻塞其它平台；退避、续期、
// 心跳都是平台级状态，也不应该共享。
type peer struct {
	svc      *Service
	mu       sync.Mutex
	platform model.GbPlatform

	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once

	state            string // registering / online / offline
	lastRegister     time.Time
	lastKeepalive    time.Time
	lastError        string
	keepMisses       int
	effectiveExpires int // 上级在 200 OK 中授予的有效期（可能小于配置值）
}

func newPeer(svc *Service, p *model.GbPlatform) *peer {
	cp := *p
	return &peer{
		svc:      svc,
		platform: cp,
		done:     make(chan struct{}),
		state:    "registering",
	}
}

// start 启动注册协程。
func (p *peer) start(ctx context.Context) {
	p.mu.Lock()
	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.mu.Unlock()
	go p.run(runCtx)
}

// stop 停止协程，并尽力发送一次 Expires:0 注销。可重复/并发调用。
func (p *peer) stop() {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		cancel := p.cancel
		p.cancel = nil
		p.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		select {
		case <-p.done:
		case <-time.After(3 * time.Second):
		}
		// 注销用独立 context：此时服务生命周期上下文可能已经取消。
		ctx, cc := context.WithTimeout(context.Background(), 3*time.Second)
		defer cc()
		if err := p.registerWithExpires(ctx, 0); err != nil {
			p.svc.log.Debug("级联注销未完成（上级可能已失联）",
				"server_gb_id", p.snapshot().ServerGBID, "error", err)
		}
		p.setStopped(ctx)
	})
}

// setStopped 把状态落成 OFF（停止服务 / 平台被删除或禁用时）。
func (p *peer) setStopped(ctx context.Context) {
	p.mu.Lock()
	p.state = "offline"
	prev := p.platform.Status
	p.platform.Status = model.GbStatusOff
	p.mu.Unlock()
	if prev != model.GbStatusOff {
		p.persistStatus(ctx, model.GbStatusOff)
	}
}

// sameConfig 判断平台配置是否变化到需要重建协程。
func (p *peer) sameConfig(next *model.GbPlatform) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, b := &p.platform, next
	return a.ServerGBID == b.ServerGBID &&
		a.ServerGBDomain == b.ServerGBDomain &&
		a.ServerIP == b.ServerIP &&
		a.ServerPort == b.ServerPort &&
		a.DeviceGBID == b.DeviceGBID &&
		a.Username == b.Username &&
		a.Password == b.Password &&
		a.Expires == b.Expires &&
		a.KeepTimeout == b.KeepTimeout &&
		transportOf(a.Transport) == transportOf(b.Transport) &&
		a.CharacterSet == b.CharacterSet
}

// snapshot 返回平台配置副本。
func (p *peer) snapshot() model.GbPlatform {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.platform
}

// status 返回运行时状态快照。
func (p *peer) status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := Status{
		PlatformID: p.platform.ID,
		Name:       p.platform.Name,
		ServerGBID: p.platform.ServerGBID,
		DeviceGBID: p.platform.DeviceGBID,
		Online:     p.state == "online",
		State:      p.state,
		LastError:  p.lastError,
	}
	if !p.lastRegister.IsZero() {
		t := p.lastRegister
		out.LastRegister = &t
	}
	if !p.lastKeepalive.IsZero() {
		t := p.lastKeepalive
		out.LastKeepalive = &t
	}
	return out
}

// run 是注册协程：注册成功 → 心跳/续期循环；失败按退避重试。
func (p *peer) run(ctx context.Context) {
	defer close(p.done)

	backoff := 5 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		rctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := p.registerWithExpires(rctx, p.expires())
		cancel()
		if err == nil {
			break
		}
		p.markOffline(err)
		p.svc.log.Warn("级联注册失败，稍后重试",
			"server_gb_id", p.snapshot().ServerGBID, "error", err, "retry_in", backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		if backoff < time.Minute {
			backoff *= 2
			if backoff > time.Minute {
				backoff = time.Minute
			}
		}
	}

	p.markOnline(context.Background(), "注册成功")
	p.svc.log.Info("级联注册成功", "server_gb_id", p.snapshot().ServerGBID,
		"device_gb_id", p.snapshot().DeviceGBID)

	refresh := p.refreshInterval()
	keep := p.keepInterval()
	refreshTimer := time.NewTimer(refresh)
	keepTimer := time.NewTimer(keep)
	defer refreshTimer.Stop()
	defer keepTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-keepTimer.C:
			kctx, kcancel := context.WithTimeout(ctx, 6*time.Second)
			err := p.sendKeepalive(kctx)
			kcancel()
			if err != nil {
				p.mu.Lock()
				p.keepMisses++
				misses := p.keepMisses
				p.mu.Unlock()
				p.svc.log.Warn("级联心跳失败", "server_gb_id", p.snapshot().ServerGBID,
					"error", err, "misses", misses)
				// 连续 3 次失败才认为离线：单次丢包在 UDP 上级链路很常见。
				if misses >= 3 {
					p.markOffline(err)
					if rerr := p.reRegister(ctx); rerr != nil {
						p.svc.log.Warn("心跳失败后重注册失败", "server_gb_id", p.snapshot().ServerGBID, "error", rerr)
					}
				}
			} else {
				p.mu.Lock()
				p.keepMisses = 0
				p.lastKeepalive = time.Now()
				p.lastError = ""
				p.state = "online"
				p.mu.Unlock()
			}
			keepTimer.Reset(keep)

		case <-refreshTimer.C:
			err := p.reRegister(ctx)
			if err != nil {
				p.svc.log.Warn("级联续期失败，5 秒后重试", "server_gb_id", p.snapshot().ServerGBID, "error", err)
				refreshTimer.Reset(5 * time.Second)
				continue
			}
			refreshTimer.Reset(refresh)
		}
	}
}

// reRegister 重新注册；失败时标记离线。
func (p *peer) reRegister(ctx context.Context) error {
	rctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := p.registerWithExpires(rctx, p.expires()); err != nil {
		p.markOffline(err)
		return err
	}
	p.markOnline(ctx, "续期成功")
	return nil
}

// registerOrRefresh 发送 REGISTER；遇到 401 时按 Digest 重发一次。
func (p *peer) registerWithExpires(ctx context.Context, expires int) error {
	if p.svc.client == nil {
		return fmt.Errorf("级联 SIP 客户端尚未初始化")
	}
	plat := p.snapshot()
	deviceID := strings.TrimSpace(plat.DeviceGBID)
	if deviceID == "" {
		return fmt.Errorf("上级平台 %s 未配置本平台国标编码 device_gb_id", plat.ServerGBID)
	}
	domain := serverDomain(&plat)
	host, port, err := splitHostPort(plat.ServerIP, int(plat.ServerPort))
	if err != nil || port <= 0 {
		return fmt.Errorf("上级平台 %s 的信令地址非法：%v", plat.ServerGBID, err)
	}
	if host == "" {
		host = domain
	}

	recipient := sip.Uri{Scheme: "sip", Host: domain}
	req := sip.NewRequest(sip.REGISTER, recipient)
	aor := sip.Uri{Scheme: "sip", User: deviceID, Host: domain}
	from := &sip.FromHeader{Address: aor}
	params := sip.NewParams()
	from.Params = params.Add("tag", sip.GenerateTagN(16))
	req.AppendHeader(from)
	req.AppendHeader(&sip.ToHeader{Address: aor})
	req.AppendHeader(&sip.ContactHeader{Address: p.svc.contactURI(deviceID, domain)})
	exp := sip.ExpiresHeader(expires)
	req.AppendHeader(&exp)

	req.SetDestination(net.JoinHostPort(host, strconv.Itoa(port)))
	req.SetTransport(transportOf(plat.Transport))
	p.svc.applyLocalAddr(req)

	resp, err := p.svc.client.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("发送 REGISTER 失败：%w", err)
	}

	// 401 是正常握手的第一步：带 Digest 凭证重发。sipgo 会用同一条请求
	// 计算摘要并递增 CSeq，因此必须复用 req，而不是新建一条。
	if resp.StatusCode == sip.StatusUnauthorized {
		if strings.TrimSpace(plat.Username) == "" {
			return fmt.Errorf("上级要求 Digest 鉴权，但未配置 username")
		}
		resp, err = p.svc.client.DoDigestAuth(ctx, req, resp, sipgo.DigestAuth{
			Username: plat.Username,
			Password: plat.Password,
		})
		if err != nil {
			return fmt.Errorf("Digest 鉴权重发失败：%w", err)
		}
	}

	if !resp.IsSuccess() {
		return fmt.Errorf("上级 %s 拒绝 REGISTER：%d %s", plat.ServerGBID, resp.StatusCode, resp.Reason)
	}
	// 上级可能把有效期改小（例如配置 3600、上级只给 60）。续期必须按
	// 上级授予的值来，否则会错过续期窗口，表现为「注册成功一分钟后掉线」。
	if expires > 0 {
		if h := resp.GetHeader("Expires"); h != nil {
			if v, err := strconv.Atoi(strings.TrimSpace(h.Value())); err == nil && v > 0 {
				p.mu.Lock()
				p.effectiveExpires = v
				p.mu.Unlock()
			}
		}
	}
	return nil
}

// sendKeepalive 发送 MANSCDP Notify/Keepalive。
func (p *peer) sendKeepalive(ctx context.Context) error {
	plat := p.snapshot()
	domain := serverDomain(&plat)
	host, port, err := splitHostPort(plat.ServerIP, int(plat.ServerPort))
	if err != nil || port <= 0 {
		return fmt.Errorf("上级信令地址非法：%v", err)
	}
	if host == "" {
		host = domain
	}

	body, err := manscdp.Marshal(&manscdp.Keepalive{
		CmdType:  manscdp.CmdKeepalive,
		SN:       p.svc.nextSN(),
		DeviceID: firstNonEmpty(plat.DeviceGBID, p.svc.cfg.ServerID),
		Status:   "OK",
	}, charset.Normalize(plat.CharacterSet))
	if err != nil {
		return err
	}

	recipient := sip.Uri{Scheme: "sip", User: plat.ServerGBID, Host: domain}
	req := p.svc.newPeerRequest(&plat, sip.MESSAGE, recipient)
	req.AppendHeader(sip.NewHeader("Content-Type", ContentTypeMANSCDP))
	req.SetBody(body)
	req.SetDestination(net.JoinHostPort(host, strconv.Itoa(port)))
	req.SetTransport(transportOf(plat.Transport))
	p.svc.applyLocalAddr(req)

	resp, err := p.svc.client.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("发送 Keepalive 失败：%w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("上级拒绝 Keepalive：%d %s", resp.StatusCode, resp.Reason)
	}
	return nil
}

// expires 返回注册有效期（秒）。
func (p *peer) expires() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.effectiveExpires > 0 {
		return p.effectiveExpires
	}
	if p.platform.Expires > 0 {
		return int(p.platform.Expires)
	}
	return 3600
}

// refreshInterval 返回续期间隔：expires 的 80%，最小 10 秒。
func (p *peer) refreshInterval() time.Duration {
	e := time.Duration(p.expires()) * time.Second
	d := e * 8 / 10
	if d < 10*time.Second {
		d = 10 * time.Second
	}
	return d
}

// keepInterval 返回心跳间隔：keep_timeout 的一半，限制在 [5s, 60s]。
func (p *peer) keepInterval() time.Duration {
	p.mu.Lock()
	v := p.platform.KeepTimeout
	p.mu.Unlock()
	if v <= 0 {
		v = 60
	}
	d := time.Duration(v) * time.Second / 2
	if d < 5*time.Second {
		d = 5 * time.Second
	}
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

// markOnline 更新内存状态并持久化 status=ON。
func (p *peer) markOnline(ctx context.Context, reason string) {
	p.mu.Lock()
	p.state = "online"
	p.lastRegister = time.Now()
	p.lastError = ""
	prev := p.platform.Status
	p.platform.Status = model.GbStatusOn
	p.keepMisses = 0
	p.mu.Unlock()
	if prev != model.GbStatusOn {
		p.persistStatus(ctx, model.GbStatusOn)
	}
	_ = reason
}

// markOffline 更新内存状态并持久化 status=OFF。
func (p *peer) markOffline(err error) {
	p.mu.Lock()
	p.state = "offline"
	p.lastError = err.Error()
	prev := p.platform.Status
	p.platform.Status = model.GbStatusOff
	p.mu.Unlock()
	if prev != model.GbStatusOff {
		p.persistStatus(context.Background(), model.GbStatusOff)
	}
}

// StatusSink 是可选的状态持久化能力；假数据源不实现即可跳过落库。
type StatusSink interface {
	SetPlatformStatus(ctx context.Context, id int64, status string) error
}

// persistStatus 写入 ac_gb_platform.status。
//
// 模型没有注册时间列：ON/OFF 是唯一持久化的运行状态，时间只在内存维护。
// 这不是缺失 —— 注册时间属于可重建的运维信息，加列会带来无意义的写放大。
func (p *peer) persistStatus(ctx context.Context, status string) {
	sink, ok := p.svc.source.(StatusSink)
	if !ok {
		return
	}
	id := p.snapshot().ID
	if id <= 0 {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sink.SetPlatformStatus(cctx, id, status); err != nil {
		p.svc.log.Warn("持久化级联状态失败", "platform_id", id, "status", status, "error", err)
	}
}
