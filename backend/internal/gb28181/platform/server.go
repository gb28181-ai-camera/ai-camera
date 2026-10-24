package platform

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo/sip"
)

// ContentTypeMANSCDP 是国标 XML 报文的 Content-Type。
const ContentTypeMANSCDP = "Application/MANSCDP+xml"

// registerHandlers 注册入向请求处理。
//
// 级联端口只接受「已配置的上级平台」的报文：
//   - REGISTER：上级一般不会反向注册，回 405；
//   - MESSAGE / NOTIFY：Catalog / DeviceInfo / DeviceStatus 查询与心跳；
//   - INVITE / ACK / BYE：上级点播本平台通道；
//   - OPTIONS：探活；
//   - SUBSCRIBE：接受并回 200（部分上级在建立目录订阅前会先发一次）。
func (s *Service) registerHandlers() {
	s.server.OnRegister(s.handleRegisterInbound)
	s.server.OnMessage(s.handleMessage)
	s.server.OnNotify(s.handleNotify)
	s.server.OnInvite(s.handleInvite)
	s.server.OnAck(s.handleAck)
	s.server.OnBye(s.handleBye)
	s.server.OnOptions(s.handleOptions)
	s.server.OnSubscribe(s.handleSubscribe)
	s.server.OnInfo(s.handleInfo)
	s.server.OnNoRoute(s.handleNoRoute)
}

// baseCtx 返回服务生命周期上下文。
func (s *Service) baseCtx() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

// ────────────────────────────────────────────────────────────────
// 入向处理
// ────────────────────────────────────────────────────────────────

// handleRegisterInbound 级联端口不承担「接受注册」的职责。
func (s *Service) handleRegisterInbound(req *sip.Request, tx sip.ServerTransaction) {
	s.respond(req, tx, sip.StatusMethodNotAllowed, "Method Not Allowed")
}

// handleMessage 处理带 MANSCDP 正文的 MESSAGE（上级查询）。
func (s *Service) handleMessage(req *sip.Request, tx sip.ServerTransaction) {
	s.processInbound(req, tx)
}

// handleNotify 处理上级的 NOTIFY（部分上级用 NOTIFY 传查询/事件）。
func (s *Service) handleNotify(req *sip.Request, tx sip.ServerTransaction) {
	s.processInbound(req, tx)
}

// processInbound 校验来源 → 解 MANSCDP → 先回 200 → 再异步应答。
func (s *Service) processInbound(req *sip.Request, tx sip.ServerTransaction) {
	p := s.matchPeer(req)
	if p == nil {
		s.respond(req, tx, sip.StatusForbidden, "Forbidden")
		s.log.Warn("级联入向报文来自未配置的上级，已拒绝", "from", fromUser(req), "source", req.Source())
		return
	}

	plat := p.snapshot()
	cs := charset.Normalize(plat.CharacterSet)
	body := req.Body()
	env, err := manscdp.Decode(body, cs)
	if err != nil {
		s.respond(req, tx, sip.StatusBadRequest, "Bad Request")
		s.log.Warn("级联 MANSCDP 解码失败", "server_gb_id", plat.ServerGBID, "error", err)
		return
	}

	// 先应答再处理：国标设备/上级不等业务处理完，早回 200 可避免重发。
	s.respond(req, tx, sip.StatusOK, "OK")

	go func() {
		ctx, cancel := context.WithTimeout(s.baseCtx(), 15*time.Second)
		defer cancel()
		s.dispatchInbound(ctx, p, env, req.Source())
	}()
}

// dispatchInbound 按根元素与 CmdType 分发。
func (s *Service) dispatchInbound(ctx context.Context, p *peer, env *manscdp.Envelope, destination string) {
	plat := p.snapshot()

	switch env.Root() {
	case "Query":
		switch env.CmdType {
		case manscdp.CmdCatalog:
			s.answerCatalogQuery(ctx, p, destination, env.SN)
		case manscdp.CmdDeviceInfo:
			s.answerDeviceInfoQuery(ctx, p, destination, env.SN)
		case manscdp.CmdDeviceStatus:
			s.answerDeviceStatusQuery(ctx, p, destination, env.SN)
		default:
			s.log.Debug("级联查询暂不支持", "cmd_type", env.CmdType, "server_gb_id", plat.ServerGBID)
		}
	case "Notify":
		// 上级主动发 Keepalive 的情况少见（通常由下级发），记录即可。
		if env.CmdType == manscdp.CmdKeepalive {
			s.log.Debug("收到上级心跳", "server_gb_id", plat.ServerGBID)
		}
	default:
		s.log.Debug("级联未处理的 MANSCDP 根元素", "root", env.Root(), "cmd_type", env.CmdType)
	}
}

// handleSubscribe 接受上级的目录订阅，并立即推送一次当前目录。
//
// 虽然主动推送策略选择的是「映射变更后 Notify」，但部分上级（WVP 等）
// 会先发 SUBSCRIBE 再等 NOTIFY。这里回 200 并发一次 Notify，避免出现
// 「订阅成功但目录一直为空」的现象。
func (s *Service) handleSubscribe(req *sip.Request, tx sip.ServerTransaction) {
	p := s.matchPeer(req)
	if p == nil {
		s.respond(req, tx, sip.StatusForbidden, "Forbidden")
		return
	}
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	if h := req.GetHeader("Expires"); h != nil {
		res.AppendHeader(h)
	} else {
		exp := sip.ExpiresHeader(3600)
		res.AppendHeader(&exp)
	}
	if err := tx.Respond(res); err != nil {
		s.log.Warn("回复级联 SUBSCRIBE 失败", "error", err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(s.baseCtx(), 15*time.Second)
		defer cancel()
		entries, err := s.source.Catalog(ctx, ptr(p.snapshot()))
		if err != nil {
			s.log.Warn("订阅后推送目录失败", "error", err)
			return
		}
		if err := s.pushCatalog(ctx, p, entries, "Notify", s.nextSN(), nil); err != nil {
			s.log.Warn("订阅后推送目录失败", "error", err)
		}
	}()
}

// handleOptions 应答上级探活。
func (s *Service) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	s.respond(req, tx, sip.StatusOK, "OK")
}

// handleInfo 处理带内控制（本期级联不支持回放控制，回 200 避免上级卡住）。
func (s *Service) handleInfo(req *sip.Request, tx sip.ServerTransaction) {
	s.respond(req, tx, sip.StatusOK, "OK")
}

// handleNoRoute 兜底：回 501 并留日志，避免静默丢包。
func (s *Service) handleNoRoute(req *sip.Request, tx sip.ServerTransaction) {
	s.log.Warn("级联收到未处理的 SIP 方法", "method", req.Method.String(), "source", req.Source())
	s.respond(req, tx, sip.StatusNotImplemented, "Not Implemented")
}

// respond 发送 SIP 响应。
func (s *Service) respond(req *sip.Request, tx sip.ServerTransaction, code int, reason string, headers ...sip.Header) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	for _, h := range headers {
		res.AppendHeader(h)
	}
	if err := tx.Respond(res); err != nil {
		s.log.Warn("发送级联 SIP 响应失败", "code", code, "error", err)
	}
}

// matchPeer 按 From/To 的国标编码定位上级平台。
//
// 优先 From（上级平台编码），其次 To（本平台在该上级的编码）；
// 两者都匹配不上但当前只有一个已配置平台时，回落到该平台 ——
// 兼容部分上级在 From 里只写域名的实现。
func (s *Service) matchPeer(req *sip.Request) *peer {
	if req == nil {
		return nil
	}
	from := fromUser(req)
	to := toUser(req)

	s.peerMu.RLock()
	defer s.peerMu.RUnlock()

	var only *peer
	for _, p := range s.peers {
		plat := p.snapshot()
		if (from != "" && from == plat.ServerGBID) || (to != "" && to == plat.DeviceGBID) {
			return p
		}
		only = p
	}
	// 只有一个平台时的回落：必须校验源 IP，否则同网段任意主机都能冒充
	// 「唯一的一个上级」发查询/INVITE。ServerIP 为空（例如上级用域名且
	// 由 DNS 解析）时才放行，避免把合法部署卡死。
	if len(s.peers) == 1 && only != nil {
		plat := only.snapshot()
		if strings.TrimSpace(plat.ServerIP) == "" {
			return only
		}
		if host, _, err := net.SplitHostPort(req.Source()); err == nil &&
			strings.EqualFold(host, strings.TrimSpace(plat.ServerIP)) {
			return only
		}
	}
	return nil
}

// ────────────────────────────────────────────────────────────────
// 出向：发往上级的 MESSAGE
// ────────────────────────────────────────────────────────────────

// pushCatalog 把目录按字节分片后逐条 MESSAGE 发给上级。
//
// root 为 "Response"（查询应答）或 "Notify"（主动推送/订阅通知）。
// destination 为空时使用平台配置的 server_ip:server_port；查询应答优先用
// 入向报文的源地址，这样上级位于 NAT 后也能收到（rport 语义）。
func (s *Service) pushCatalog(ctx context.Context, p *peer, entries []CatalogEntry, root string, sn int, destination *string) error {
	plat := p.snapshot()
	cs := charset.Normalize(plat.CharacterSet)
	chunks, err := buildCatalogChunks(root, cs, sn,
		firstNonEmpty(plat.DeviceGBID, s.cfg.ServerID), int(plat.Secrecy), entries)
	if err != nil {
		return err
	}
	for _, body := range chunks {
		if err := s.sendToSuperior(ctx, p, body, ContentTypeMANSCDP, destination); err != nil {
			return err
		}
	}
	return nil
}

// sendToSuperior 构造并发送一条 MESSAGE。
func (s *Service) sendToSuperior(ctx context.Context, p *peer, body []byte, contentType string, destination *string) error {
	plat := p.snapshot()
	domain := serverDomain(&plat)
	recipient := sip.Uri{Scheme: "sip", User: plat.ServerGBID, Host: domain}
	req := s.newPeerRequest(&plat, sip.MESSAGE, recipient)
	req.AppendHeader(sip.NewHeader("Content-Type", contentType))
	req.SetBody(body)

	if destination != nil && strings.TrimSpace(*destination) != "" {
		req.SetDestination(strings.TrimSpace(*destination))
	} else {
		host, port, err := splitHostPort(plat.ServerIP, int(plat.ServerPort))
		if err != nil || port <= 0 {
			return fmt.Errorf("上级平台 %s 的信令地址非法：%v", plat.ServerGBID, err)
		}
		if host == "" {
			host = domain
		}
		req.SetDestination(net.JoinHostPort(host, strconv.Itoa(port)))
	}
	req.SetTransport(transportOf(plat.Transport))
	s.applyLocalAddr(req)

	resp, err := s.client.Do(ctx, req)
	if err != nil {
		return fmt.Errorf("发送级联 MESSAGE 失败：%w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("上级 %s 拒绝 MESSAGE：%d %s", plat.ServerGBID, resp.StatusCode, resp.Reason)
	}
	return nil
}

// newPeerRequest 构造一条发往上级的请求，填好 From/To/Contact。
//
// Call-ID / CSeq / Max-Forwards / Via 由 sipgo 的 clientRequestBuildReq 补齐。
func (s *Service) newPeerRequest(plat *model.GbPlatform, method sip.RequestMethod, recipient sip.Uri) *sip.Request {
	req := sip.NewRequest(method, recipient)
	from := &sip.FromHeader{Address: s.localURI(plat.DeviceGBID, serverDomain(plat))}
	params := sip.NewParams()
	from.Params = params.Add("tag", sip.GenerateTagN(16))
	req.AppendHeader(from)
	req.AppendHeader(&sip.ToHeader{Address: recipient})
	req.AppendHeader(&sip.ContactHeader{Address: s.contactURI(plat.DeviceGBID, serverDomain(plat))})
	return req
}

// ────────────────────────────────────────────────────────────────
// 请求字段提取
// ────────────────────────────────────────────────────────────────

// fromUser 取 From 头的用户部分。
func fromUser(req *sip.Request) string {
	if req == nil {
		return ""
	}
	if h := req.From(); h != nil {
		return strings.TrimSpace(h.Address.User)
	}
	return ""
}

// toUser 取 To 头的用户部分。
func toUser(req *sip.Request) string {
	if req == nil {
		return ""
	}
	if h := req.To(); h != nil {
		return strings.TrimSpace(h.Address.User)
	}
	return ""
}

// callIDOf 取 Call-ID。
func callIDOf(req *sip.Request) string {
	if req == nil {
		return ""
	}
	if h := req.CallID(); h != nil {
		return string(*h)
	}
	return ""
}

// serverDomain 返回上级的 SIP 域：server_gb_domain 优先，
// 否则取 server_gb_id 的前 10 位，最后回落到 server_gb_id。
func serverDomain(plat *model.GbPlatform) string {
	if plat == nil {
		return defaultSIPDomain
	}
	if v := strings.TrimSpace(plat.ServerGBDomain); v != "" {
		return v
	}
	if id := strings.TrimSpace(plat.ServerGBID); len(id) >= 10 {
		return id[:10]
	}
	if id := strings.TrimSpace(plat.ServerGBID); id != "" {
		return id
	}
	return defaultSIPDomain
}

// ptr 返回指针（构造 Catalog 入参时用）。
func ptr[T any](v T) *T { return &v }
