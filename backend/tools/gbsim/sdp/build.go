package sdp

import (
	"strconv"
	"strings"
)

// 设备侧 200 OK 里声明的负载类型：96 = PS（GB28181 视频的约定）。
const (
	// PlayPayloadType 是 PS-over-RTP 的负载类型。
	PlayPayloadType = 96
	// PlaySessionName 是实时点播的会话名（回放为 Playback，本批次不做）。
	PlaySessionName = "Play"
)

// PlayParams 是构造设备侧点播应答 SDP 的入参。
type PlayParams struct {
	DeviceID    string // 20 位设备编码，作为 o= 的用户名（WVP 用它构造 ACK 的 Request-URI）
	LocalIP     string // 设备本地 IP，写进 o= 与 c=
	LocalPort   int    // 设备本地 RTP 端口，写进 m=
	SSRC        string // 设备要用于推流的 SSRC（默认等于平台 y=，--ssrc-override 时不同）
	SessionName string // s=，留空则用 Play
}

// BuildPlay 构造设备侧对实时点播 INVITE 的 200 OK SDP。
//
// 字段取舍（对齐 WVP `SIPCommander.playStreamCmd` 的对称行为，见设计 §7.1/§7.2）：
//
//	v=0
//	o=<设备编码> 0 0 IN IP4 <本地IP>
//	s=Play
//	c=IN IP4 <本地IP>
//	t=0 0
//	m=video <本地端口> RTP/AVP 96
//	a=recvonly
//	a=rtpmap:96 PS/90000
//	y=<SSRC>
//
// 说明：
//   - `o=` 的用户名必须是**设备编码**：WVP 的 `InviteResponseProcessor` 用
//     `sdp.getOrigin().getUsername()` + 200 OK 的源地址来拼 ACK 的 Request-URI，
//     这里写错会导致平台发出的 ACK 打不到设备上（表现为「推流一直不开始」）。
//   - `y=` 用设备实际推流用的 SSRC：默认与平台一致；`--ssrc-override` 时故意不同，
//     用于验证平台的 `updateRtpServerSSRC` 校正路径（设计 §13.3.2）。
//   - 本批次只做 UDP，因此 m= 用 `RTP/AVP` 且不带 `a=setup`；TCP 模式（下一批）会带
//     `TCP/RTP/AVP` 与 `a=setup:passive`。
func BuildPlay(p PlayParams) string {
	name := p.SessionName
	if name == "" {
		name = PlaySessionName
	}
	var b strings.Builder
	b.WriteString("v=0\r\n")
	b.WriteString("o=" + p.DeviceID + " 0 0 IN IP4 " + p.LocalIP + "\r\n")
	b.WriteString("s=" + name + "\r\n")
	b.WriteString("c=IN IP4 " + p.LocalIP + "\r\n")
	b.WriteString("t=0 0\r\n")
	b.WriteString("m=video " + strconv.Itoa(p.LocalPort) + " RTP/AVP " + strconv.Itoa(PlayPayloadType) + "\r\n")
	b.WriteString("a=recvonly\r\n")
	b.WriteString("a=rtpmap:" + strconv.Itoa(PlayPayloadType) + " PS/90000\r\n")
	b.WriteString("y=" + p.SSRC + "\r\n")
	return b.String()
}
