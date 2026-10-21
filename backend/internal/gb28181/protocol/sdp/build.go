// Package sdp 构造平台下发给设备的 SDP（RFC 4566 + GB28181 私有扩展）。
//
// 只覆盖「平台发起 INVITE，让设备把 PS 流推到 ZLM」这一种场景：
// 收流地址与端口来自配置（单端口模式下是 ZLM 的共享 RTP 代理端口），
// SSRC 由平台分配并经 `y=` 通告（设计 §6.4）。
//
// ⚠️ 契约 §15.5b 的坑：`o=` 的用户名必须是设备编码。部分设备用它校验
// 会话归属，写错表现为「INVITE 成功、200 OK 也收到了，但 ACK 打不到设备 →
// 推流永不开始」，且没有任何报错。因此 PlayParams 把 OriginUser 单独列出来，
// 调用方必须传设备国标编码。
package sdp

import (
	"strconv"
	"strings"
)

const (
	// PayloadTypePS 是 GB28181 视频（PS 封装）的 RTP 负载类型。
	PayloadTypePS = 96
	// SessionNamePlay 实时点播的会话名。
	SessionNamePlay = "Play"
	// SessionNamePlayback 回放会话名。
	SessionNamePlayback = "Playback"
	// SessionNameDownload 下载会话名。
	SessionNameDownload = "Download"
)

// PlayParams 是构造平台侧点播 SDP 的入参。
type PlayParams struct {
	// OriginUser 写进 `o=` 的用户名。点播时必须传**设备国标编码**（契约 §15.5b）。
	// 留空时回落到 ServerID（仅用于回放/广播等不校验该字段的场景）。
	OriginUser string
	// ServerID 平台 20 位国标编码，OriginUser 为空时的回落值。
	ServerID string
	// IP 是设备可达的收流地址：单端口模式下即 ZLM 所在主机。
	IP string
	// Port 是收流端口：单端口模式下是 ZLM 的 rtp_proxy_port。
	Port int
	// SSRC 是平台分配的 10 位 SSRC，写进 `y=`。
	SSRC string
	// SessionName 缺省 Play；回放用 Playback。
	SessionName string
	// PayloadTypes 覆盖默认的负载类型列表，测试/兼容用；留空用 96 97 98。
	PayloadTypes []int
	// TCP 为 true 时按 TCP-PASSIVE 构造（本期单端口模式不支持，保留扩展点）。
	TCP bool
}

// BuildPlay 构造点播 INVITE 的 SDP 正文。
//
// 输出的关键行：
//
//	v=0
//	o=<设备编码> 0 0 IN IP4 <收流IP>
//	s=Play
//	c=IN IP4 <收流IP>
//	t=0 0
//	m=video <端口> RTP/AVP 96 97 98
//	a=recvonly
//	a=rtpmap:96 PS/90000
//	y=<SSRC>
func BuildPlay(p PlayParams) string {
	ip := strings.TrimSpace(p.IP)
	name := strings.TrimSpace(p.SessionName)
	if name == "" {
		name = SessionNamePlay
	}
	origin := strings.TrimSpace(p.OriginUser)
	if origin == "" {
		origin = p.ServerID
	}
	payloads := p.PayloadTypes
	if len(payloads) == 0 {
		payloads = []int{PayloadTypePS, 97, 98}
	}
	ptList := make([]string, 0, len(payloads))
	for _, pt := range payloads {
		ptList = append(ptList, strconv.Itoa(pt))
	}

	var b strings.Builder
	b.WriteString("v=0\r\n")
	b.WriteString("o=" + origin + " 0 0 IN IP4 " + ip + "\r\n")
	b.WriteString("s=" + name + "\r\n")
	b.WriteString("c=IN IP4 " + ip + "\r\n")
	b.WriteString("t=0 0\r\n")
	proto := "RTP/AVP"
	if p.TCP {
		proto = "TCP/RTP/AVP"
	}
	b.WriteString("m=video " + strconv.Itoa(p.Port) + " " + proto + " " + strings.Join(ptList, " ") + "\r\n")
	b.WriteString("a=recvonly\r\n")
	b.WriteString("a=rtpmap:" + strconv.Itoa(PayloadTypePS) + " PS/90000\r\n")
	if p.TCP {
		// TCP-PASSIVE：设备主动连 ZLM 的收流端口。
		b.WriteString("a=setup:passive\r\n")
		b.WriteString("a=connection:new\r\n")
	}
	b.WriteString("y=" + p.SSRC + "\r\n")
	return b.String()
}
