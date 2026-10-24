package sipserver

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/metrics"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo/sip"
)

// 入向请求的处理入口。
//
// 每个 handler 的公共顺序（设计 §4.4）：
//  1. 白名单：From 的用户部分必须存在于 ac_device.gb_device_id，否则 403；
//  2. REGISTER 额外做 Digest（设备配了 sip_password 时）；MESSAGE/NOTIFY
//     不带 Authorization，这是真机行为，设计 §4.4 也不要求；
//  3. 对 MESSAGE/NOTIFY 额外校验「源地址 == 注册地址」；
//  4. 再解析 MANSCDP。解码失败必须显式记日志并拒绝，不能静默丢弃。
func (s *Service) registerHandlers() {
	s.server.OnRegister(s.handleRegister)
	s.server.OnMessage(s.handleMessage)
	s.server.OnNotify(s.handleNotify)
	s.server.OnInvite(s.handleInvite)
	s.server.OnAck(s.handleAck)
	s.server.OnBye(s.handleBye)
	s.server.OnInfo(s.handleInfo)
	s.server.OnOptions(s.handleOptions)
	s.server.OnSubscribe(s.handleSubscribeInbound)
	s.server.OnNoRoute(s.handleNoRoute)
}

// baseCtx 返回服务生命周期上下文；未启动时用 Background，方便单测直接调 handler。
func (s *Service) baseCtx() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

// dbCtx 给单次数据库操作一个上限，避免设备端等待过久。
func (s *Service) dbCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(s.baseCtx(), 5*time.Second)
}

// handleRegister 处理 REGISTER：白名单 → Digest → 写注册状态 → 200 + Expires。
func (s *Service) handleRegister(req *sip.Request, tx sip.ServerTransaction) {
	start := time.Now()
	metrics.IncSipMessage("in")
	dev, ok := s.authorize(req, tx)
	if !ok {
		return
	}

	expires := registerExpires(req, s.cfg.RegisterExpires)
	ctx, cancel := s.dbCtx()
	defer cancel()
	if err := s.devices.OnRegister(ctx, dev, req.Source(), transportOf(req), expires); err != nil {
		metrics.IncSipRegister("failed")
		s.respond(req, tx, sip.StatusInternalServerError, "Server Internal Error")
		s.logInbound(req, dev.GbDeviceID, s.DeviceCharset(dev), "", 500, "failed", "更新注册状态失败："+err.Error(), &start)
		return
	}

	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	exp := sip.ExpiresHeader(expires)
	res.AppendHeader(&exp)
	res.AppendHeader(&sip.ContactHeader{Address: s.contact})
	if err := tx.Respond(res); err != nil {
		s.log.Warn("回复 REGISTER 200 失败", "error", err, "call_id", callIDOf(req))
		return
	}

	action := "注册成功"
	if expires == 0 {
		action = "注销成功"
	}
	s.log.Info("设备"+action, "gb_device_id", dev.GbDeviceID,
		"remote", req.Source(), "transport", transportOf(req), "expires", expires)
	metrics.IncSipRegister("success")
	s.logInbound(req, dev.GbDeviceID, s.DeviceCharset(dev), "", 200, "success", action, &start)

	// 首次激活（尚未同步过目录）时主动拉一次 Catalog，避免管理员还要手动点「同步通道」。
	// 只在 channel_count=0 时触发：重注册、心跳重连不应该反复把目录打满。
	if expires > 0 && dev.ChannelCount == 0 {
		go s.syncCatalogAfterRegister(dev)
	}
}

// syncCatalogAfterRegister 在注册成功后异步拉取目录。
//
// 延迟 500ms：设备刚回完 200 OK 时往往还在初始化业务线程，
// 立刻发 Catalog 有的型号会回 486/超时。失败只告警，不影响注册。
func (s *Service) syncCatalogAfterRegister(dev *model.Device) {
	select {
	case <-time.After(500 * time.Millisecond):
	case <-s.baseCtx().Done():
		return
	}
	ctx, cancel := context.WithTimeout(s.baseCtx(), 15*time.Second)
	defer cancel()
	if err := s.QueryCatalog(ctx, dev); err != nil {
		s.log.Warn("注册后自动查询目录失败（可稍后手动同步）", "gb_device_id", dev.GbDeviceID, "error", err)
		return
	}
	s.log.Info("注册后已发送 Catalog 查询", "gb_device_id", dev.GbDeviceID)
}

// handleMessage 处理带 MANSCDP 正文的 MESSAGE。
func (s *Service) handleMessage(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	s.processInboundBody(req, tx)
}

// handleNotify 处理设备主动 NOTIFY（订阅推送）。
//
// 目录/报警/位置的订阅推送与 MESSAGE 的正文格式相同，因此复用同一套解析。
func (s *Service) handleNotify(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	s.processInboundBody(req, tx)
}

// processInboundBody 是 MESSAGE / NOTIFY 的共同处理流程。
func (s *Service) processInboundBody(req *sip.Request, tx sip.ServerTransaction) {
	start := time.Now()
	dev, ok := s.authorize(req, tx)
	if !ok {
		return
	}

	// 第 2 道校验：源 IP:Port 必须与注册时绑定的地址一致。
	// 这条是**本项目对 WVP 的加固**（WVP 只在设备与上级平台撞 ID 时才比对）：
	// 任何能到达 5060 的主机只要冒用设备编码，就能伪造目录/报警，而目录会创建数据。
	if ep, ok := s.devices.Endpoint(dev.GbDeviceID); ok {
		if !sameAddr(ep.Addr, req.Source()) {
			s.log.Warn("报文源地址与注册地址不一致，已丢弃",
				"gb_device_id", dev.GbDeviceID, "registered", ep.Addr, "source", req.Source())
			s.logInbound(req, dev.GbDeviceID, s.DeviceCharset(dev), "", 0, "rejected",
				fmt.Sprintf("源地址 %s 与注册地址 %s 不一致", req.Source(), ep.Addr), &start)
			return // 不回响应：不给伪造者任何反馈
		}
	} else {
		// 进程重启后内存端点丢失，用第一条到达的报文重新绑定。
		s.devices.SetEndpoint(dev.GbDeviceID, req.Source(), transportOf(req))
	}

	cs := s.DeviceCharset(dev)
	body := req.Body()
	env, err := manscdp.Decode(body, cs)
	if err != nil {
		s.respond(req, tx, sip.StatusBadRequest, "Bad Request")
		s.logInbound(req, dev.GbDeviceID, cs, "", 400, "failed", "MANSCDP 解码失败："+err.Error(), &start)
		return
	}
	if env.CharsetMismatch {
		// 声明与设备建档 charset 不一致：本次按声明解（字节为准），但要留痕。
		s.log.Warn("MANSCDP 声明字符集与设备配置不一致",
			"gb_device_id", dev.GbDeviceID, "declared", env.DeclaredCharset, "configured", cs)
	}

	// 先回 200：国标设备不等应答、只重发；处理落库放在应答之后，
	// 避免目录/报警的数据库抖动拖慢 SIP 事务。
	s.respond(req, tx, sip.StatusOK, "OK")
	s.logInbound(req, dev.GbDeviceID, cs, env.CmdType, 200, "success", "", &start)

	// 2022 能力探测：收到 2022 独有命令即把设备版本升到 2022。
	// 放在回 200 之后：版本落库失败不应影响信令事务。
	dbCtx, cancel := s.dbCtx()
	versionErr := s.devices.OnCommandSeen(dbCtx, dev, env.CmdType)
	cancel()
	if versionErr != nil {
		s.log.Warn("设备协议版本升级失败", "gb_device_id", dev.GbDeviceID, "error", versionErr)
	}

	s.dispatch(dev, env, body, req.Source(), transportOf(req))
}

// dispatch 按根元素与 CmdType 路由到业务处理。
func (s *Service) dispatch(dev *model.Device, env *manscdp.Envelope, body []byte, remote, transport string) {
	cs := s.DeviceCharset(dev)
	ctx, cancel := s.dbCtx()
	defer cancel()

	switch env.Root() {
	case "Notify":
		switch env.CmdType {
		case manscdp.CmdKeepalive:
			_, msg, err := manscdp.DecodeKeepalive(body, cs)
			if err != nil {
				s.log.Warn("解析 Keepalive 失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			if err := s.devices.OnKeepalive(ctx, dev, remote, transport); err != nil {
				s.log.Warn("更新心跳状态失败", "error", err, "gb_device_id", dev.GbDeviceID)
			}
			s.log.Debug("收到心跳", "gb_device_id", dev.GbDeviceID, "status", msg.Status)
		case manscdp.CmdAlarm:
			_, msg, err := manscdp.DecodeAlarm(body, cs)
			if err != nil {
				s.log.Warn("解析 Alarm 失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			if err := s.devices.OnAlarm(ctx, dev, msg); err != nil {
				s.log.Warn("写入国标报警失败", "error", err, "gb_device_id", dev.GbDeviceID)
			}
		case manscdp.CmdMobilePosition:
			_, msg, err := manscdp.DecodeMobilePosition(body, cs)
			if err != nil {
				s.log.Warn("解析 MobilePosition 失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			if err := s.devices.OnMobilePosition(ctx, dev, msg); err != nil {
				s.log.Warn("写入移动位置失败", "error", err, "gb_device_id", dev.GbDeviceID)
			}
		case manscdp.CmdCatalog:
			// 目录订阅的增量推送（Notify/Catalog）。事件语义见 manscdp.CatalogEvent*。
			// 不能复用 Response 的等齐逻辑：Notify 只带变化项，永远等不到 SumNum。
			_, notify, err := manscdp.DecodeCatalogNotify(body, cs)
			if err != nil {
				s.log.Warn("解析目录通知失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			if err := s.devices.OnCatalogNotify(ctx, dev, notify); err != nil {
				s.log.Warn("处理目录通知失败", "error", err, "gb_device_id", dev.GbDeviceID)
			}
		case manscdp.CmdMediaStatus:
			s.log.Info("收到媒体状态通知", "gb_device_id", dev.GbDeviceID)
		default:
			s.log.Debug("未处理的 Notify", "cmd_type", env.CmdType, "gb_device_id", dev.GbDeviceID)
		}

	case "Response":
		switch env.CmdType {
		case manscdp.CmdCatalog:
			_, resp, err := manscdp.DecodeCatalog(body, cs)
			if err != nil {
				s.log.Warn("解析目录响应失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			done, err := s.devices.OnCatalogResponse(ctx, dev, resp)
			if err != nil {
				s.log.Warn("目录落库失败", "error", err, "gb_device_id", dev.GbDeviceID)
			}
			if done {
				s.log.Info("目录接收完成", "gb_device_id", dev.GbDeviceID, "sum", resp.SumNum)
			}
		case manscdp.CmdDeviceInfo:
			_, resp, err := manscdp.DecodeDeviceInfo(body, cs)
			if err != nil {
				s.log.Warn("解析设备信息响应失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			s.log.Info("设备信息", "gb_device_id", dev.GbDeviceID,
				"name", resp.DeviceName, "manufacturer", resp.Manufacturer,
				"model", resp.Model, "firmware", resp.Firmware, "channel", resp.Channel)
		case manscdp.CmdDeviceStatus:
			_, resp, err := manscdp.DecodeDeviceStatus(body, cs)
			if err != nil {
				s.log.Warn("解析设备状态响应失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			if err := s.devices.OnDeviceStatus(ctx, dev, resp); err != nil {
				s.log.Warn("更新设备状态失败", "error", err, "gb_device_id", dev.GbDeviceID)
			}
		case manscdp.CmdRecordInfo:
			_, resp, err := manscdp.DecodeRecordInfo(body, cs)
			if err != nil {
				s.log.Warn("解析录像查询响应失败", "error", err, "gb_device_id", dev.GbDeviceID)
				return
			}
			s.recordMu.RLock()
			f := s.onRecordInfo
			s.recordMu.RUnlock()
			if f != nil {
				f(dev, resp)
			} else {
				s.log.Info("收到录像查询响应（暂无消费方）", "gb_device_id", dev.GbDeviceID, "sum", resp.SumNum)
			}
		case manscdp.CmdPTZPosition, manscdp.CmdCruiseTrackList, manscdp.CmdCruiseTrack,
			manscdp.CmdHomePosition, manscdp.CmdSDCardStatus:
			// 2022 查询应答：交给按 (设备, SN) 等待的调用方。
			if !s.DeliverQueryResponse(dev, env.SN, body) {
				s.log.Debug("收到 2022 查询应答（无等待方）",
					"cmd_type", env.CmdType, "sn", env.SN, "gb_device_id", dev.GbDeviceID)
			}
		case manscdp.CmdPresetQuery, manscdp.CmdConfigDownload:
			s.log.Debug("收到查询响应", "cmd_type", env.CmdType, "gb_device_id", dev.GbDeviceID,
				"bytes", len(body))
		default:
			s.log.Debug("未处理的 Response", "cmd_type", env.CmdType, "gb_device_id", dev.GbDeviceID)
		}

	default:
		s.log.Debug("未处理的 MANSCDP 根元素", "root", env.Root(), "cmd_type", env.CmdType,
			"gb_device_id", dev.GbDeviceID)
	}
}

// handleInvite 处理设备/上级平台发起的 INVITE。
//
// 本期只有「平台 → 设备」的点播方向；级联入向（上级平台点播本平台）属于三期，
// 明确回 488 而不是静默丢弃。
func (s *Service) handleInvite(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	start := time.Now()
	s.respond(req, tx, sip.StatusNotAcceptableHere, "Not Acceptable Here")
	s.logInbound(req, deviceIDOf(req), s.cfg.DefaultCharset, "", 488, "rejected",
		"平台不接受入向 INVITE（级联未实现）", &start)
}

// handleAck 处理 ACK。ACK 不期望响应，只做日志；用于发现「ACK 打不到设备」一类问题。
func (s *Service) handleAck(req *sip.Request, _ sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	s.log.Debug("收到 ACK", "call_id", callIDOf(req), "source", req.Source())
}

// handleBye 处理设备主动结束会话（掉电、用户挂断）。
func (s *Service) handleBye(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	start := time.Now()
	callID := callIDOf(req)
	if d, ok := s.sessions.Get(callID); ok {
		if err := d.AcceptBye(req, tx); err != nil {
			s.log.Warn("处理设备 BYE 失败", "error", err, "call_id", callID)
		}
		s.log.Info("设备结束会话", "call_id", callID, "gb_device_id", d.DeviceID)
		s.logInbound(req, d.DeviceID, s.cfg.DefaultCharset, "", 200, "success", "设备 BYE", &start)
		return
	}
	s.respond(req, tx, sip.StatusOK, "OK")
	s.logInbound(req, deviceIDOf(req), s.cfg.DefaultCharset, "", 200, "success", "无对应会话的 BYE", &start)
}

// handleInfo 处理入向 INFO（回放控制一般是平台发给设备，入向少见）。
func (s *Service) handleInfo(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	start := time.Now()
	s.respond(req, tx, sip.StatusOK, "OK")
	s.logInbound(req, deviceIDOf(req), s.cfg.DefaultCharset, "", 200, "success", "", &start)
}

// handleOptions 应答平台/设备探活。
func (s *Service) handleOptions(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	s.respond(req, tx, sip.StatusOK, "OK")
}

// handleSubscribeInbound 处理设备对平台的 SUBSCRIBE（级联场景）。
func (s *Service) handleSubscribeInbound(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	start := time.Now()
	s.respond(req, tx, sip.StatusMethodNotAllowed, "Method Not Allowed")
	s.logInbound(req, deviceIDOf(req), s.cfg.DefaultCharset, "", 405, "rejected",
		"平台不接受入向 SUBSCRIBE（级联未实现）", &start)
}

// handleNoRoute 兜底：记录未处理的方法并回 501，避免静默丢包。
func (s *Service) handleNoRoute(req *sip.Request, tx sip.ServerTransaction) {
	metrics.IncSipMessage("in")
	s.log.Warn("收到未处理的 SIP 方法", "method", req.Method.String(), "call_id", callIDOf(req))
	s.respond(req, tx, sip.StatusNotImplemented, "Not Implemented")
}

// ────────────────────────────────────────────────────────────────
// 鉴权与应答
// ────────────────────────────────────────────────────────────────

// authorize 执行白名单（所有请求）与 Digest 校验（仅 REGISTER）。
//
// ⚠️ 为什么 Digest 只作用于 REGISTER：
// GB28181 设备的实际行为是**只对 REGISTER 做 401 挑战**，心跳/目录响应的 MESSAGE
// **不带 Authorization**（WVP 也只校验 REGISTER 的摘要）。设计 §4.4 对 MESSAGE 的
// 入向校验定义为「白名单 + 源地址 + 域前缀」三道，并不含 Digest —— 若对 MESSAGE
// 强制摘要，所有真机与 tools/gbsim 都会卡在 401，表现为「注册成功但目录/心跳全丢」。
//
// 状态码取舍：
//   - 未建档 → 403（需求 2 的落点，写 ac_gb_sip_log）
//   - REGISTER 需要鉴权但没带凭证 → 401 + WWW-Authenticate
//   - REGISTER 带了凭证但校验失败 → 403（设计 §4.2 的「密码错误回 403」）
func (s *Service) authorize(req *sip.Request, tx sip.ServerTransaction) (*model.Device, bool) {
	ctx, cancel := s.dbCtx()
	defer cancel()

	gbID := deviceIDOf(req)
	dev, err := s.auth.Lookup(ctx, gbID)
	if err != nil {
		// 首次绑定：国标编码尚未建档，但设备在 REGISTER 上带了业务设备编码与产品编码。
		// 只在 REGISTER 上尝试；MESSAGE/NOTIFY 不参与绑定，避免伪造报文改绑。
		if req.Method == sip.REGISTER {
			if bound, bindErr := s.tryBindBySIP(ctx, req, gbID); bindErr == nil && bound != nil {
				dev = bound
				err = nil
				s.log.Info("SIP 设备首次绑定成功",
					"gb_device_id", gbID, "device_code", headerValue(req, "X-Device-Code"),
					"product_code", headerValue(req, "X-Product-Code"))
			} else if bindErr != nil {
				// 明确区分「没带绑定信息」与「带了但校验失败」：
				// 后者要写进日志，现场才知道为什么 403。
				s.logInbound(req, gbID, s.cfg.DefaultCharset, "", 403, "rejected",
					"SIP 绑定失败："+bindErr.Error(), nil)
				s.log.Warn("SIP 设备绑定失败", "gb_device_id", gbID,
					"device_code", headerValue(req, "X-Device-Code"),
					"product_code", headerValue(req, "X-Product-Code"), "error", bindErr)
			}
		}
		if err != nil {
			metrics.IncSipRegister("rejected")
			s.respond(req, tx, sip.StatusForbidden, "Forbidden")
			s.logInbound(req, gbID, s.cfg.DefaultCharset, "", 403, "rejected", "白名单拒绝："+err.Error(), nil)
			s.log.Warn("SIP 设备被拒绝（未建档）", "gb_device_id", gbID, "remote", req.Source(),
				"method", req.Method.String())
			return nil, false
		}
	}

	if req.Method != sip.REGISTER || !s.auth.NeedAuth(dev) {
		return dev, true
	}
	realm := s.auth.RealmFor(dev)

	if req.GetHeader("Authorization") == nil {
		s.respondChallenge(req, tx, gbID, realm)
		// 401 是 Digest 握手的第一步，不是拒绝：按 rejected 记会让
		// 「注册失败率」这类指标把每一次正常注册都算成失败。
		// rejected 只留给白名单/Digest 校验失败/方法不支持等真正的拒绝。
		s.logInbound(req, gbID, s.DeviceCharset(dev), "", 401, "success", "需要 Digest 鉴权（挑战）", nil)
		return nil, false
	}

	if err := s.auth.Verify(req, dev); err != nil {
		metrics.IncSipRegister("rejected")
		s.respond(req, tx, sip.StatusForbidden, "Forbidden")
		s.logInbound(req, gbID, s.DeviceCharset(dev), "", 403, "rejected", "Digest 校验失败："+err.Error(), nil)
		s.log.Warn("SIP 设备鉴权失败", "gb_device_id", gbID, "remote", req.Source(),
			"method", req.Method.String(), "error", err)
		if req.Method == sip.REGISTER {
			_ = s.devices.MarkRejected(ctx, dev)
		}
		return nil, false
	}
	return dev, true
}

// respond 发送响应。
func (s *Service) respond(req *sip.Request, tx sip.ServerTransaction, code int, reason string, headers ...sip.Header) {
	res := sip.NewResponseFromRequest(req, code, reason, nil)
	for _, h := range headers {
		res.AppendHeader(h)
	}
	if err := tx.Respond(res); err != nil {
		s.log.Warn("发送 SIP 响应失败", "code", code, "call_id", callIDOf(req), "error", err)
	}
}

// respondChallenge 发送 401 挑战。
func (s *Service) respondChallenge(req *sip.Request, tx sip.ServerTransaction, gbID, realm string) {
	challenge := s.auth.Challenge(gbID, realm)
	s.respond(req, tx, sip.StatusUnauthorized, "Unauthorized",
		sip.NewHeader("WWW-Authenticate", challenge))
}

// ────────────────────────────────────────────────────────────────
// 日志
// ────────────────────────────────────────────────────────────────

// logInbound 写一条入向信令日志。
//
// content 必须能存进 PostgreSQL 的 text 列（合法 UTF-8）：设备报文常常是 GB2312，
// 直接 string(body) 会让 INSERT 报 invalid byte sequence，反而把主流程拖崩。
func (s *Service) logInbound(req *sip.Request, gbID, cs, cmdType string, status int, result, reason string, start *time.Time) {
	elapsed := int64(0)
	if start != nil {
		elapsed = time.Since(*start).Milliseconds()
	}
	s.devices.LogSip(model.GbSipLog{
		GbDeviceID: gbID,
		Direction:  "in",
		Method:     req.Method.String(),
		CmdType:    cmdType,
		StatusCode: int64(status),
		RemoteAddr: req.Source(),
		LocalAddr:  s.localAddrString(),
		CallID:     callIDOf(req),
		Content:    safeLogContent(req.Body(), cs),
		Result:     result,
		Reason:     reason,
		ElapsedMS:  elapsed,
	})
}

// safeLogContent 把任意字符集的报文转成可入库的 UTF-8 文本。
func safeLogContent(body []byte, cs string) string {
	if len(body) == 0 {
		return ""
	}
	if utf8.Valid(body) {
		return truncate(string(body), 4000)
	}
	if converted, err := charset.ToUTF8(body, cs); err == nil && utf8.Valid(converted) {
		return truncate(string(converted), 4000)
	}
	// 最后兜底：十六进制，保证一定能入库（排障时仍可人工还原）。
	return truncate("hex:"+hex.EncodeToString(body), 4000)
}

// ────────────────────────────────────────────────────────────────
// 请求字段提取
// ────────────────────────────────────────────────────────────────

// deviceIDOf 取请求的deviceID：优先 From 头的用户部分（GB28181 的约定），
// 回落到 To / Request-URI，兼容少数把身份放在 To 头的设备。
func deviceIDOf(req *sip.Request) string {
	if req == nil {
		return ""
	}
	if h := req.From(); h != nil && strings.TrimSpace(h.Address.User) != "" {
		return strings.TrimSpace(h.Address.User)
	}
	if h := req.To(); h != nil && strings.TrimSpace(h.Address.User) != "" {
		return strings.TrimSpace(h.Address.User)
	}
	return strings.TrimSpace(req.Recipient.User)
}

// callIDOf 安全取 Call-ID。
func callIDOf(req *sip.Request) string {
	if req == nil {
		return ""
	}
	if h := req.CallID(); h != nil {
		return string(*h)
	}
	return ""
}

// transportOf 取入向请求的传输层（UDP / TCP）。
func transportOf(req *sip.Request) string {
	if req == nil {
		return TransportUDP
	}
	if t := strings.ToUpper(strings.TrimSpace(req.Transport())); t != "" {
		return t
	}
	return TransportUDP
}

// registerExpires 解析注册有效期：Expires 头优先，其次 Contact 的 expires 参数。
func registerExpires(req *sip.Request, def int) int {
	if h := req.GetHeader("Expires"); h != nil {
		if v, err := strconv.Atoi(strings.TrimSpace(h.Value())); err == nil && v >= 0 {
			return v
		}
	}
	if c := req.Contact(); c != nil {
		if v, ok := c.Params.Get("expires"); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 {
				return n
			}
		}
	}
	return def
}

// sameAddr 比较两个 host:port 是否指向同一端点（兼容 IPv6 的方括号写法）。
func sameAddr(a, b string) bool {
	if a == b {
		return true
	}
	ah, ap, err1 := net.SplitHostPort(a)
	bh, bp, err2 := net.SplitHostPort(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return strings.EqualFold(strings.Trim(ah, "[]"), strings.Trim(bh, "[]")) && ap == bp
}

// headerValue 读取自定义 SIP 头的值（不存在返回空串）。
//
// 设备编码/产品编码不是 GB28181 标准头，平台约定为：
//
//	X-Device-Code: <业务设备编码>
//	X-Product-Code: <产品编码>
func headerValue(req *sip.Request, name string) string {
	if req == nil {
		return ""
	}
	if h := req.GetHeader(name); h != nil {
		return strings.TrimSpace(h.Value())
	}
	return ""
}

// tryBindBySIP 在国标编码未建档时，尝试用设备编码 + 产品编码完成首次绑定。
//
// 返回 (nil, nil) 表示「没有携带绑定信息」或「未注入 binder」——调用方继续按
// 未建档拒绝；返回 error 表示携带了绑定信息但校验失败，需要留痕。
func (s *Service) tryBindBySIP(ctx context.Context, req *sip.Request, gbID string) (*model.Device, error) {
	binder := s.currentBinder()
	if binder == nil {
		return nil, nil
	}
	deviceCode := headerValue(req, "X-Device-Code")
	productCode := headerValue(req, "X-Product-Code")
	if deviceCode == "" || productCode == "" {
		return nil, nil
	}
	return binder.BindBySIP(ctx, productCode, deviceCode, gbID)
}
