package sipua

import (
	"context"
	"fmt"

	"github.com/boticz/ai-camera-backend/tools/gbsim/sdp"
	"github.com/emiago/sipgo/sip"
)

// InviteInfo 是一次实时点播 INVITE 解析出的关键信息。
//
// 它同时承担「给上层的输入」与「本次会话的事实记录」两个角色：
// 前者是解析平台 SDP 得到的字段，后者是设备侧最终决定的 SSRC 与 200 OK 正文。
type InviteInfo struct {
	CallID    string // 贯穿整条媒体链路的关联键（§13.3.3）
	ChannelID string // 通道编码，取自 INVITE 的 Request-URI 用户部分

	SessionName  string // 平台 SDP 的 s=，点播为 Play，回放为 Playback
	PlatformIP   string // 平台 SDP 的 c= —— 设备要把 RTP 发到这里
	PlatformPort int    // 平台 SDP 的 m= 端口 —— 单端口模式下是 ZLM 的收流端口
	Protocol     string // m= 的传输协议：RTP/AVP 或 TCP/RTP/AVP
	Setup        string // 平台 SDP 的 a=setup（UDP 模式下为空）

	PlatformSSRC string // 平台 SDP 的 y=，平台自己分配的 SSRC
	// DeviceSSRC 是设备实际用于推流的 SSRC。
	// 默认等于 PlatformSSRC（§6.4：设备要用平台分配的 SSRC，否则触发平台的校正路径）；
	// --ssrc-override 时故意取不同值，用于验证 updateRtpServerSSRC（§13.3.2）。
	DeviceSSRC string

	// SDP 是设备侧 200 OK 的正文。它在 MediaFactory 返回、拿到本地 RTP 端口后才填上，
	// 因此传给 MediaFactory 的那一份 InviteInfo 此字段为空。
	SDP string
}

// MediaSession 是一次推流会话，由上层（main）注入实现。
// UA 只负责在收到 ACK 时 Start、收到 BYE 时 Stop，不关心具体推什么。
type MediaSession interface {
	// Start 启动推流；应立即返回，实际推流在后台进行。
	Start(ctx context.Context) error
	// Stop 停止推流并等待资源释放；可重复调用。
	Stop()
}

// MediaFactory 由上层注入：为一次 INVITE 构造推流会话。
//
// 返回的 localRTPPort 会被写进设备侧 200 OK 的 m= 行 —— 之所以由工厂返回而不是 UA 猜一个，
// 是因为真实端口要等 UDP 连接绑定后才知道（见 stream.Streamer.LocalRTPPort）。
type MediaFactory func(info InviteInfo) (sess MediaSession, localRTPPort int, err error)

// inviteSession 是 UA 侧对一个已建立（或待确认）INVITE 会话的记账。
type inviteSession struct {
	info    InviteInfo
	media   MediaSession
	started bool
}

// SetMediaFactory 注册推流会话工厂。未注册时收到的 INVITE 会被回绝（488）。
func (u *UA) SetMediaFactory(f MediaFactory) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.mediaFactory = f
}

// handleInvite 处理平台的实时点播 INVITE：回 100 Trying → 回 200 OK（带设备 SDP）→ 等 ACK。
//
// 三条容易踩的坑（都在 §7.1/§7.2 里）：
//  1. **目标地址取自平台 SDP，不是配置**：设备要把 RTP 发到 `c=` 的地址 + `m=` 的端口。
//     单端口模式下该端口是 ZLM 的收流端口，写死一个值必然收不到流。
//  2. **设备侧 SDP 的 y= 决定平台的后续动作**：与平台一致 → 正常；不一致且平台开了
//     ssrc_check → 平台调 updateRtpServerSSRC 校正（§13.3.2 的场景）。
//  3. **o= 的用户名必须是设备编码**：WVP 用它拼 ACK 的 Request-URI，写错表现为「ACK 打不到设备」。
func (u *UA) handleInvite(req *sip.Request, tx sip.ServerTransaction) {
	callID := callIDOf(req)
	channelID := channelIDOf(req)

	// 立刻回 100 Trying。这一步不只是礼貌：sipgo 的 ServerTx 会在 200ms 后自动补一个
	// 100 Trying，主动回一个可以把那个定时器停掉，避免平台收到两条 100。
	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusTrying, "Trying", nil)); err != nil {
		u.log.Error("回复 INVITE 100 Trying 失败", "error", err, "call_id", callID)
		return
	}

	body := string(req.Body())
	parsed, err := sdp.Parse(body)
	if err != nil {
		u.log.Error("解析 INVITE 的 SDP 失败，回 488", "error", err, "call_id", callID, "body", body)
		u.rejectInvite(req, tx, callID, "SDP 解析失败")
		return
	}

	video := parsed.VideoMedia()
	if video == nil || video.Port <= 0 {
		u.log.Error("INVITE 的 SDP 没有可用的视频媒体段，回 488", "call_id", callID, "body", body)
		u.rejectInvite(req, tx, callID, "无视频媒体段")
		return
	}

	// 本批次只做 UDP。TCP-PASSIVE / TCP-ACTIVE 需要 `$` 分帧或主动连出，留给下一批。
	// 明确回绝而不是当 UDP 处理 —— 后者会「INVITE 成功但一直没画面」，最难排。
	if !video.IsUDP() {
		u.log.Warn("收到 TCP 模式的 INVITE，本批次尚未实现，回 488",
			"call_id", callID, "protocol", video.Protocol, "setup", parsed.Setup())
		u.rejectInvite(req, tx, callID, "本批次仅支持 UDP 收流")
		return
	}

	u.mu.RLock()
	factory := u.mediaFactory
	override := u.cfg.SSRCOverride
	u.mu.RUnlock()

	if factory == nil {
		u.log.Error("收到 INVITE 但未注册推流工厂（main 未接线？），回 488", "call_id", callID)
		u.rejectInvite(req, tx, callID, "未配置媒体源")
		return
	}

	platformSSRC := parsed.VideoSSRC()
	deviceSSRC := platformSSRC
	if override != "" {
		// 故意与平台不同：用于覆盖平台的 SSRC 校正路径（§13.3.2）。
		u.log.Warn("--ssrc-override 生效：设备将使用与平台不同的 SSRC",
			"call_id", callID, "platform_ssrc", platformSSRC, "device_ssrc", override)
		deviceSSRC = override
	}
	if deviceSSRC == "" {
		// 平台没给 y=（不合规但确实存在），设备自拟一个稳定的值。
		deviceSSRC = DefaultSSRC(u.cfg.DeviceID)
		u.log.Warn("平台 SDP 未携带 y=，设备自拟 SSRC",
			"call_id", callID, "device_ssrc", deviceSSRC)
	}

	info := InviteInfo{
		CallID:       callID,
		ChannelID:    channelID,
		SessionName:  parsed.Name,
		PlatformIP:   parsed.VideoConnectionIP(),
		PlatformPort: video.Port,
		Protocol:     video.Protocol,
		Setup:        parsed.Setup(),
		PlatformSSRC: platformSSRC,
		DeviceSSRC:   deviceSSRC,
	}
	if info.PlatformIP == "" {
		u.log.Error("INVITE 的 SDP 缺少 c=（收流地址），回 488", "call_id", callID)
		u.rejectInvite(req, tx, callID, "缺少 c=")
		return
	}

	sess, localPort, err := factory(info)
	if err != nil {
		u.log.Error("构造推流会话失败，回 488", "error", err, "call_id", callID)
		u.rejectInvite(req, tx, callID, "媒体源不可用")
		return
	}

	info.SDP = sdp.BuildPlay(sdp.PlayParams{
		DeviceID:    u.cfg.DeviceID,
		LocalIP:     u.cfg.LocalIP,
		LocalPort:   localPort,
		SSRC:        deviceSSRC,
		SessionName: parsed.Name,
	})

	res := sip.NewSDPResponseFromRequest(req, []byte(info.SDP))
	res.AppendHeader(&sip.ContactHeader{Address: u.contactURI})

	// ⚠️ 顺序很关键：**先登记会话，再回 200 OK**。
	// sipgo 每个入向请求各起一个协程，ACK 可能在 200 OK 发出的瞬间就到达并进入 handleAck；
	// 若登记晚于发送，那次 ACK 会查不到会话而被当成「488 后的确认」丢弃，表现为推流永不开始
	// （且在环回 + 低延迟网络下才偶发，最难查）。登记在发送之前，这个窗口就不存在。
	var old *inviteSession
	u.sessMu.Lock()
	// 同一 Call-ID 重复 INVITE（re-INVITE）：旧的推流要停掉，避免两条并存。
	old = u.sessions[callID]
	u.sessions[callID] = &inviteSession{info: info, media: sess}
	u.sessMu.Unlock()

	if err := tx.Respond(res); err != nil {
		u.log.Error("回复 INVITE 200 OK 失败", "error", err, "call_id", callID)
		u.sessMu.Lock()
		if cur := u.sessions[callID]; cur != nil && cur.media == sess {
			delete(u.sessions, callID)
		}
		u.sessMu.Unlock()
		sess.Stop()
		return
	}

	// Stop 可能阻塞（等推流协程退出），放在锁外调用。
	if old != nil {
		old.media.Stop()
	}

	u.log.Info("已应答点播 INVITE，等待 ACK",
		"call_id", callID, "channel_id", channelID,
		"platform", fmt.Sprintf("%s:%d", info.PlatformIP, info.PlatformPort),
		"platform_ssrc", info.PlatformSSRC, "device_ssrc", info.DeviceSSRC,
		"local_rtp_port", localPort)
}

// handleAck 处理 ACK：到这一步会话才真正建立，开始推流。
//
// ⚠️ ACK 的用途是**确认收到了 200 OK**，不是「可以推流了」的许可 —— 严格按 RFC 3261，
// 设备在发出 200 OK 后就可以开始推流。这里等 ACK 再推，是为了让模拟器的行为
// 更贴近真机（也便于测试断言「ACK 之后才有包」）。
func (u *UA) handleAck(req *sip.Request, _ sip.ServerTransaction) {
	callID := callIDOf(req)

	u.sessMu.Lock()
	sess := u.sessions[callID]
	if sess != nil && !sess.started {
		sess.started = true
	}
	u.sessMu.Unlock()

	if sess == nil {
		// 常见于两种情形：ACK 对应的是我们回绝过的 INVITE（回 488 后平台仍会发 ACK），
		// 或 200 OK 与 ACK 的 Call-ID 没对上。都属于正常范围，记 Debug 即可。
		u.log.Debug("收到无对应会话的 ACK（可能是 488 后的确认）", "call_id", callID)
		return
	}

	ctx := u.baseCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if err := sess.media.Start(ctx); err != nil {
		u.log.Error("启动推流失败", "error", err, "call_id", callID)
		return
	}
	u.log.Info("收到 ACK，开始推流",
		"call_id", callID, "channel_id", sess.info.ChannelID,
		"platform", fmt.Sprintf("%s:%d", sess.info.PlatformIP, sess.info.PlatformPort),
		"ssrc", sess.info.DeviceSSRC)
}

// handleBye 处理平台发起的停止：停推流、回 200 OK、清会话。
func (u *UA) handleBye(req *sip.Request, tx sip.ServerTransaction) {
	callID := callIDOf(req)

	u.sessMu.Lock()
	sess := u.sessions[callID]
	delete(u.sessions, callID)
	u.sessMu.Unlock()

	if sess != nil {
		sess.media.Stop()
		u.log.Info("收到 BYE，已停止推流", "call_id", callID, "channel_id", sess.info.ChannelID)
	} else {
		u.log.Warn("收到无对应会话的 BYE", "call_id", callID)
	}

	if err := tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)); err != nil {
		u.log.Error("回复 BYE 200 OK 失败", "error", err, "call_id", callID)
	}
}

// stopAllSessions 停止所有在推的会话，供 Close 时清理。
func (u *UA) stopAllSessions() {
	u.sessMu.Lock()
	sessions := u.sessions
	u.sessions = map[string]*inviteSession{}
	u.sessMu.Unlock()

	for callID, sess := range sessions {
		sess.media.Stop()
		u.log.Info("UA 关闭，停止推流会话", "call_id", callID)
	}
}

// rejectInvite 回一个失败响应并记日志，避免平台一直等到超时。
func (u *UA) rejectInvite(req *sip.Request, tx sip.ServerTransaction, callID, reason string) {
	res := sip.NewResponseFromRequest(req, sip.StatusNotAcceptableHere, "Not Acceptable Here", nil)
	if err := tx.Respond(res); err != nil {
		u.log.Error("回复 INVITE 488 失败", "error", err, "call_id", callID)
	}
}

// channelIDOf 从 INVITE 的 Request-URI 用户部分取出通道编码。
//
// WVP 的 INVITE 是 `sip:{channelId}@{设备地址}`，To 头同样是通道编码，
// 因此 Request-URI 缺失时回落到 To 头。
func channelIDOf(req *sip.Request) string {
	if req.Recipient.User != "" {
		return req.Recipient.User
	}
	if to := req.To(); to != nil {
		return to.Address.User
	}
	return ""
}

// DefaultSSRC 在平台未在 SDP 里给 y= 时自拟一个 SSRC。
//
// 取「0 + 设备编码末 9 位」凑成 10 位：与 §6.4 的格式（首位 0 表示实时点播）一致，
// 且对同一台设备稳定 —— 相比随机数，排障时至少能一眼认出是哪台设备。
func DefaultSSRC(deviceID string) string {
	digits := ""
	for i := len(deviceID) - 1; i >= 0 && len(digits) < 9; i-- {
		if deviceID[i] >= '0' && deviceID[i] <= '9' {
			digits = string(deviceID[i]) + digits
		}
	}
	if digits == "" {
		digits = "000000001"
	}
	return "0" + digits
}
