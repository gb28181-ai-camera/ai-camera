package platform

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// inviteSession 是上级点播本平台通道时建立的一路推流会话。
//
// 与设备侧 session.Dialog 的区别：这里的方向是「本平台发 RTP 给上级」，
// 没有 ZLM 收流，也没有平台分配 SSRC 的必要（优先用上级 SDP 的 y=）；
// 因此只保留 BYE 时停止推流所需的 app/stream/ssrc。
type inviteSession struct {
	svc        *Service
	callID     string
	dialog     *sipgo.DialogServerSession
	sender     MediaSender
	app        string
	stream     string
	ssrc       uint32
	ssrcStr    string
	allocated  bool
	platformID int64
	deviceID   string
	closeOnce  sync.Once
}

// close 停止推流并释放对话框。BYE 与「200 未被 ACK」两条路径都可能调到，
// 用 Once 保证 StopSendRTP / Release 只执行一次。
func (it *inviteSession) close(ctx context.Context) {
	if it == nil {
		return
	}
	it.closeOnce.Do(func() {
		if it.sender != nil && it.app != "" && it.stream != "" {
			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			if err := it.sender.StopSendRTP(cctx, it.app, it.stream, it.ssrc); err != nil {
				it.svc.log.Warn("停止级联推流失败", "call_id", it.callID, "app", it.app,
					"stream", it.stream, "ssrc", it.ssrc, "error", err)
			}
			cancel()
		}
		if it.dialog != nil {
			_ = it.dialog.Close()
		}
		if it.allocated && it.ssrcStr != "" {
			it.svc.ssrc.Release(it.ssrcStr)
		}
	})
}

// handleInvite 接受上级的实时点播 INVITE。
//
// 流程：鉴权（平台白名单）→ 解析上级 SDP 的接收地址/端口/SSRC →
// 找到对应通道与本地 ZLM 流 → StartSendRTP → 200 OK(SDP) → 记录会话。
func (s *Service) handleInvite(req *sip.Request, tx sip.ServerTransaction) {
	s.log.Info("收到级联 INVITE", "from", fromUser(req), "target", req.Recipient.User, "source", req.Source())
	p := s.matchPeer(req)
	if p == nil {
		s.respond(req, tx, sip.StatusForbidden, "Forbidden")
		s.log.Warn("级联 INVITE 来自未配置的上级", "from", fromUser(req), "source", req.Source())
		return
	}
	plat := p.snapshot()

	target := strings.TrimSpace(req.Recipient.User)
	if target == "" {
		target = toUser(req)
	}
	if target == "" {
		s.respond(req, tx, sip.StatusBadRequest, "Bad Request")
		return
	}

	dstIP, dstPort, ssrcStr, err := parseInviteSDP(string(req.Body()))
	if err != nil {
		s.log.Warn("解析上级 INVITE SDP 失败", "server_gb_id", plat.ServerGBID, "error", err)
		s.respond(req, tx, sip.StatusNotAcceptableHere, "Not Acceptable Here")
		return
	}

	ctx, cancel := context.WithTimeout(s.baseCtx(), 10*time.Second)
	defer cancel()

	entries, err := s.source.Catalog(ctx, &plat)
	if err != nil {
		s.log.Warn("级联 INVITE 查询通道失败", "error", err)
		s.respond(req, tx, sip.StatusInternalServerError, "Server Internal Error")
		return
	}
	var entry *CatalogEntry
	for i := range entries {
		if entries[i].DeviceID == target {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		s.respond(req, tx, sip.StatusNotFound, "Not Found")
		s.log.Warn("级联 INVITE 通道不在该上级目录中", "server_gb_id", plat.ServerGBID, "channel", target)
		return
	}

	// 本地流未就绪时明确拒绝：此时 StartSendRTP 只会得到 ZLM 的
	// 「stream not found」，上级侧表现为 INVITE 成功但一直黑屏，更难排查。
	if entry.App == "" || entry.Stream == "" {
		s.respond(req, tx, sip.StatusTemporarilyUnavailable, "Temporarily Unavailable")
		s.log.Warn("级联 INVITE 被拒绝：本地通道没有可用的 ZLM 流",
			"server_gb_id", plat.ServerGBID, "channel", target, "local_channel_id", entry.ChannelID)
		return
	}

	sender, err := s.mediaSender(ctx, &plat)
	if err != nil {
		s.log.Warn("解析级联媒体节点失败", "error", err)
		s.respond(req, tx, sip.StatusInternalServerError, "Server Internal Error")
		return
	}

	var allocated bool
	if strings.TrimSpace(ssrcStr) == "" {
		ssrcStr, err = s.ssrc.Allocate(true)
		if err != nil {
			s.respond(req, tx, sip.StatusInternalServerError, "Server Internal Error")
			return
		}
		allocated = true
	}
	ssrc, err := parseUint32(ssrcStr)
	if err != nil {
		if allocated {
			s.ssrc.Release(ssrcStr)
		}
		s.respond(req, tx, sip.StatusBadRequest, "Bad Request")
		return
	}

	dlg, err := s.dialogUA.ReadInvite(req, tx)
	if err != nil {
		if allocated {
			s.ssrc.Release(ssrcStr)
		}
		s.log.Warn("建立级联 INVITE 对话框失败", "error", err)
		s.respond(req, tx, sip.StatusBadRequest, "Bad Request")
		return
	}

	// 先起推流，再回 200：上级收到 200 后可能立刻开始收流，
	// 反过来（先 200 再 StartSendRTP）会白白丢开头几秒的 GOP。
	if _, err := sender.StartSendRTP(ctx, mediaserver.SendRTPParam{
		App:     entry.App,
		Stream:  entry.Stream,
		SSRC:    ssrc,
		DstURL:  dstIP,
		DstPort: dstPort,
		IsUDP:   true,
		PT:      96,
		UsePS:   true,
	}); err != nil {
		_ = dlg.Close()
		if allocated {
			s.ssrc.Release(ssrcStr)
		}
		s.log.Warn("级联推流启动失败", "server_gb_id", plat.ServerGBID,
			"channel", target, "dst", fmt.Sprintf("%s:%d", dstIP, dstPort), "error", err)
		_ = dlg.Respond(sip.StatusInternalServerError, "Server Internal Error", nil)
		return
	}

	it := &inviteSession{
		svc:        s,
		callID:     callIDOf(req),
		dialog:     dlg,
		sender:     sender,
		app:        entry.App,
		stream:     entry.Stream,
		ssrc:       ssrc,
		ssrcStr:    ssrcStr,
		allocated:  allocated,
		platformID: plat.ID,
		deviceID:   target,
	}
	// 先登记会话再回 200：上级可能在收到 200 的同一瞬间发 ACK/BYE，
	// 若此时会话还没入表，BYE 会被当成「无匹配会话」，推流泄漏。
	s.sessionMu.Lock()
	// 同一 Call-ID 重发 INVITE（上级超时重试）：先停掉旧会话，避免重复推流。
	if old := s.sessions[it.callID]; old != nil {
		go old.close(context.Background())
	}
	s.sessions[it.callID] = it
	s.sessionMu.Unlock()

	answer := buildAnswerSDP(firstNonEmpty(plat.DeviceGBID, s.cfg.ServerID),
		s.cfg.AdvertiseIP, dstPort, ssrcStr)
	if err := dlg.RespondSDP([]byte(answer)); err != nil {
		// sipgo 的 RespondSDP 会一直等到 ACK；若上级的 ACK 与 BYE 几乎同时
		// 到达（或只发 BYE），对话框会先被 BYE 关闭，这里得到的是
		// "No ACK received" 的假错误。会话已被 BYE 收走时按正常结束处理。
		if s.getInviteSession(it.callID) == nil {
			s.log.Debug("级联 INVITE 对话框已由 BYE 结束", "call_id", it.callID, "reason", err)
			return
		}
		s.takeInviteSession(it.callID)
		it.close(context.Background())
		s.log.Warn("回复级联 INVITE 200 失败", "call_id", it.callID, "error", err)
		return
	}

	s.log.Info("已接受上级点播 INVITE", "server_gb_id", plat.ServerGBID,
		"call_id", it.callID, "channel", target, "src", entry.App+"/"+entry.Stream,
		"dst", fmt.Sprintf("%s:%d", dstIP, dstPort), "ssrc", ssrcStr)
}

// handleAck 处理上级的 ACK（确认对话框建立）。
func (s *Service) handleAck(req *sip.Request, tx sip.ServerTransaction) {
	it := s.getInviteSession(callIDOf(req))
	if it == nil || it.dialog == nil {
		s.log.Debug("级联 ACK 无匹配会话", "call_id", callIDOf(req), "source", req.Source())
		return
	}
	if err := it.dialog.ReadAck(req, tx); err != nil {
		s.log.Warn("级联 ACK 校验失败", "call_id", it.callID, "error", err)
		return
	}
	s.log.Debug("级联 ACK 已确认", "call_id", it.callID)
}

// handleBye 处理上级结束点播：停止推流、回 200、清理会话。
func (s *Service) handleBye(req *sip.Request, tx sip.ServerTransaction) {
	callID := callIDOf(req)
	it := s.takeInviteSession(callID)
	if it == nil {
		s.log.Warn("级联 BYE 无匹配会话（可能已结束或 Call-ID 不一致）", "call_id", callID, "source", req.Source())
		s.respond(req, tx, sip.StatusOK, "OK")
		return
	}
	if it.dialog != nil {
		if err := it.dialog.ReadBye(req, tx); err != nil {
			s.log.Warn("处理级联 BYE 失败", "call_id", callID, "error", err)
		}
	} else {
		s.respond(req, tx, sip.StatusOK, "OK")
	}
	it.close(context.Background())
	s.log.Info("上级结束点播", "call_id", callID, "channel", it.deviceID)
}

func (s *Service) getInviteSession(callID string) *inviteSession {
	if callID == "" {
		return nil
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	return s.sessions[callID]
}

func (s *Service) takeInviteSession(callID string) *inviteSession {
	if callID == "" {
		return nil
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	it := s.sessions[callID]
	delete(s.sessions, callID)
	return it
}

// mediaSender 解析上级平台绑定的 ZLM 节点。
func (s *Service) mediaSender(ctx context.Context, plat *model.GbPlatform) (MediaSender, error) {
	if s.mediaFactory != nil {
		return s.mediaFactory(ctx, plat)
	}
	if plat != nil && strings.TrimSpace(plat.ServerID) != "" {
		client, _, err := mediaserver.FromServerID(ctx, plat.ServerID)
		if err != nil {
			return nil, err
		}
		return client, nil
	}
	client, _, err := mediaserver.FromDefaultServer(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// ────────────────────────────────────────────────────────────────
// SDP
// ────────────────────────────────────────────────────────────────

// parseInviteSDP 从上级 INVITE 中解析收流地址、端口与 SSRC。
//
// 只看第一个 m=video；媒体级 c= 覆盖会话级 c=；y= 是上级分配的 SSRC。
// UDP 是级联默认传输，TCP 主动/被动模式不在本期范围（会按 UDP 推流）。
func parseInviteSDP(body string) (ip string, port int, ssrc string, err error) {
	if strings.TrimSpace(body) == "" {
		return "", 0, "", fmt.Errorf("SDP 为空")
	}
	var sessionIP, mediaIP string
	inVideo := false
	for _, raw := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "c="):
			// c=IN IP4 127.0.0.1
			parts := strings.Fields(strings.TrimPrefix(line, "c="))
			if len(parts) >= 3 {
				if inVideo {
					mediaIP = parts[2]
				} else {
					sessionIP = parts[2]
				}
			}
		case strings.HasPrefix(line, "m="):
			if strings.HasPrefix(line, "m=video ") {
				parts := strings.Fields(strings.TrimPrefix(line, "m=video "))
				if len(parts) >= 1 {
					p, perr := strconv.Atoi(parts[0])
					if perr != nil || p <= 0 {
						return "", 0, "", fmt.Errorf("SDP m=video 端口非法：%q", parts[0])
					}
					port = p
					inVideo = true
				}
			} else {
				inVideo = false
			}
		case strings.HasPrefix(line, "y="):
			ssrc = strings.TrimSpace(strings.TrimPrefix(line, "y="))
		}
	}
	ip = mediaIP
	if ip == "" {
		ip = sessionIP
	}
	if ip == "" || port <= 0 {
		return "", 0, "", fmt.Errorf("SDP 缺少 c= 或 m=video 端口")
	}
	return ip, port, ssrc, nil
}

// buildAnswerSDP 构造 200 OK 的 SDP。
//
// GB28181 级联的下级应答只需声明「sendonly + PS/90000 + y=SSRC」：
// 真正的推流目标是上级 SDP 里的 c=/m=，应答里的端口沿用上级端口仅为兼容
// 部分上级对两端端口一致性的检查。
func buildAnswerSDP(origin, advertiseIP string, dstPort int, ssrc string) string {
	var b strings.Builder
	b.WriteString("v=0\r\n")
	b.WriteString("o=" + origin + " 0 0 IN IP4 " + advertiseIP + "\r\n")
	b.WriteString("s=Play\r\n")
	b.WriteString("c=IN IP4 " + advertiseIP + "\r\n")
	b.WriteString("t=0 0\r\n")
	b.WriteString("m=video " + strconv.Itoa(dstPort) + " RTP/AVP 96\r\n")
	b.WriteString("a=sendonly\r\n")
	b.WriteString("a=rtpmap:96 PS/90000\r\n")
	b.WriteString("y=" + ssrc + "\r\n")
	return b.String()
}
