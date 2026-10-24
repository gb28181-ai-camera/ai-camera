// Package sdp 构造平台下发给设备的 SDP（RFC 4566 + GB28181 私有扩展）。
//
// 覆盖「平台发起 INVITE，让设备把 PS 流推到 ZLM」这一种场景：
// 收流地址与端口来自配置（单端口模式下是 ZLM 的共享 RTP 代理端口），
// SSRC 由平台分配并经 `y=` 通告（设计 §6.4）。
//
// 传输模式（设计 §7.2）：
//   - UDP：`m=video <port> RTP/AVP ...`，最常见的默认形态；
//   - TCP-PASSIVE：`m=video <port> TCP/RTP/AVP ...` + `a=setup:passive`，
//     平台（ZLM）被动等设备连上 10000/tcp —— 本项目单端口模式下唯一支持的 TCP 形态；
//   - TCP-ACTIVE：`a=setup:active`，要求 ZLM 主动拨号设备。**与单端口收流互斥**
//     （单端口不创建流级 RTP server，ZLM 无处拨号），本包只负责如实构造 SDP，
//     是否允许由 play 层决定：那里会直接返回明确错误，绝不静默降级成 UDP。
//
// ⚠️ 契约 §15.5b 的坑：`o=` 的用户名必须是设备编码。部分设备用它校验
// 会话归属，写错表现为「INVITE 成功、200 OK 也收到了，但 ACK 打不到设备 →
// 推流永不开始」，且没有任何报错。因此 PlayParams 把 OriginUser 单独列出来，
// 调用方必须传设备国标编码。
package sdp

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// PayloadTypePS 是 GB28181 视频（PS 封装）的 RTP 负载类型。
	// 无论编码列表怎么配，PS 恒占 96 —— 设备与 ZLM 都把它当固定值。
	PayloadTypePS = 96
	// PayloadTypeStart 是除 PS 外其余编码的起始负载类型。
	PayloadTypeStart = 97
	// PayloadTypeMax 是动态负载类型的上限（RFC 3551：96–127）。
	PayloadTypeMax = 127
	// SessionNamePlay 实时点播的会话名。
	SessionNamePlay = "Play"
	// SessionNamePlayback 回放会话名。
	SessionNamePlayback = "Playback"
	// SessionNameDownload 下载会话名。
	SessionNameDownload = "Download"
)

// Transport 是 SDP 通告的媒体传输模式。
type Transport string

const (
	// TransportUDP 设备主动把 RTP 打到平台（默认）。
	TransportUDP Transport = "udp"
	// TransportTCPPassive 平台监听、设备主动连（`a=setup:passive`）。
	TransportTCPPassive Transport = "tcp-passive"
	// TransportTCPActive 平台主动连设备（`a=setup:active`）。单端口模式不支持。
	TransportTCPActive Transport = "tcp-active"
)

// ParseTransport 把设备档案里的 stream_mode 归一化成 Transport。
//
// 兼容 ac_device.stream_mode 的大写取值（UDP / TCP-PASSIVE / TCP-ACTIVE）
// 以及配置/接口里常见的小写、下划线写法。**无法识别时回落到 UDP**：
// 这是 WVP 与本项目的默认值，且历史数据里 stream_mode 可能为空。
func ParseTransport(raw string) Transport {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "", "udp", "rtp/avp", "rtp_avp":
		return TransportUDP
	case "tcp", "tcp-passive", "tcp_passive", "tcppassive", "passive":
		return TransportTCPPassive
	case "tcp-active", "tcp_active", "tcpactive", "active":
		return TransportTCPActive
	default:
		return TransportUDP
	}
}

// IsTCP 判断是否为 TCP 类传输。
func (t Transport) IsTCP() bool { return t != TransportUDP }

// Protocol 返回 `m=` 行的传输协议。
func (t Transport) Protocol() string {
	if t.IsTCP() {
		return "TCP/RTP/AVP"
	}
	return "RTP/AVP"
}

// Codec 是一条编码描述（`a=rtpmap:<pt> <Name>/<ClockRate>`）。
type Codec struct {
	Name      string // 如 PS / H264 / H265 / PCMA / AAC
	ClockRate int    // 如 90000 / 8000 / 44100
}

// String 返回 rtpmap 里的编码串，如 "PS/90000"。
func (c Codec) String() string {
	return fmt.Sprintf("%s/%d", c.Name, c.ClockRate)
}

// DefaultVideoCodecs 返回 GB28181-2022 的默认视频编码列表（顺序即优先级）。
// 2022 新增 H.265；PS 仍是实际推流封装，H264/H265 用于能力通告。
func DefaultVideoCodecs() []string {
	return []string{"PS/90000", "H264/90000", "H265/90000"}
}

// DefaultAudioCodecs 返回 GB28181-2022 的默认音频编码列表。
// 2022 新增 G.722.1（MPEG4-GENERIC）与 AAC。
func DefaultAudioCodecs() []string {
	return []string{"PCMA/8000", "MPEG4-GENERIC/44100", "AAC/44100"}
}

// ParseCodecs 把 "PS/90000" 这类配置串解析成 Codec 列表。
//
// defaultClock 在配置项漏写时钟频率时兜底（视频 90000、音频 8000）；
// 空项与无法识别的项会被跳过，避免生成 `a=rtpmap:97 /90000` 这种非法行。
func ParseCodecs(raw []string, defaultClock int) []Codec {
	out := make([]Codec, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, clockStr, found := strings.Cut(item, "/")
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		clock := defaultClock
		if found {
			if v, err := strconv.Atoi(strings.TrimSpace(clockStr)); err == nil && v > 0 {
				clock = v
			}
		}
		out = append(out, Codec{Name: name, ClockRate: clock})
	}
	return out
}

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
	// Transport 传输模式；留空按 UDP。TCP-ACTIVE 由调用方决定是否允许
	// （本项目单端口模式不支持，play 层会提前拒绝）。
	Transport Transport
	// TCP 是 Transport 的旧写法（true 等价于 tcp-passive），保留以兼容既有调用方。
	// Transport 非空时以 Transport 为准。
	TCP bool
	// VideoCodecs / AudioCodecs 是按优先级排列的编码列表（"PS/90000" 形式）。
	// 留空用 DefaultVideoCodecs / DefaultAudioCodecs。
	VideoCodecs []string
	AudioCodecs []string
	// PayloadTypes 覆盖默认的负载类型列表，测试/兼容用；留空按编码列表自动分配。
	// 设置后 m= 只列这些 PT，且仅输出 PS 的 rtpmap（保持旧行为）。
	PayloadTypes []int
}

// BuildPlay 构造点播 INVITE 的 SDP 正文。
//
// UDP 输出：
//
//	v=0
//	o=<设备编码> 0 0 IN IP4 <收流IP>
//	s=Play
//	c=IN IP4 <收流IP>
//	t=0 0
//	m=video <端口> RTP/AVP 96 97 98 99 100 101
//	a=recvonly
//	a=rtpmap:96 PS/90000
//	a=rtpmap:97 H264/90000
//	...
//	y=<SSRC>
//
// TCP-PASSIVE 把 m= 换成 `TCP/RTP/AVP` 并补 `a=setup:passive` + `a=connection:new`；
// TCP-ACTIVE 补 `a=setup:active`（不带 connection，因为它由设备监听、平台拨号）。
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
	transport := p.Transport
	if transport == "" {
		if p.TCP {
			transport = TransportTCPPassive
		} else {
			transport = TransportUDP
		}
	}

	formats, rtpmaps := buildCodecLines(p)

	var b strings.Builder
	b.WriteString("v=0\r\n")
	b.WriteString("o=" + origin + " 0 0 IN IP4 " + ip + "\r\n")
	b.WriteString("s=" + name + "\r\n")
	b.WriteString("c=IN IP4 " + ip + "\r\n")
	b.WriteString("t=0 0\r\n")
	b.WriteString("m=video " + strconv.Itoa(p.Port) + " " + transport.Protocol() + " " + strings.Join(formats, " ") + "\r\n")
	b.WriteString("a=recvonly\r\n")
	for _, line := range rtpmaps {
		b.WriteString(line + "\r\n")
	}
	if transport.IsTCP() {
		if transport == TransportTCPActive {
			// 平台主动连设备：设备负责监听，因此平台侧声明 active。
			b.WriteString("a=setup:active\r\n")
		} else {
			// 平台被动等设备连入（ZLM 的 10000/tcp）。
			b.WriteString("a=setup:passive\r\n")
			b.WriteString("a=connection:new\r\n")
		}
	}
	b.WriteString("y=" + p.SSRC + "\r\n")
	return b.String()
}

// buildCodecLines 生成 m= 的负载类型列表与 a=rtpmap 行。
//
// 规则（GB28181-2022 编码协商）：
//   - PS 固定 96 —— 实际推流就是 PS-over-RTP，与编码优先级无关；
//   - 其余编码按「视频在前、音频在后」的配置顺序，从 97 起递增；
//   - 动态 PT 上限 127，超出的编码静默丢弃（配置写错时宁可少通告，也不发非法 PT）。
//
// 显式给了 PayloadTypes 时沿用旧行为：只列这些 PT，且只输出 PS 的 rtpmap。
func buildCodecLines(p PlayParams) (formats []string, rtpmaps []string) {
	if len(p.PayloadTypes) > 0 {
		formats = make([]string, 0, len(p.PayloadTypes))
		for _, pt := range p.PayloadTypes {
			formats = append(formats, strconv.Itoa(pt))
		}
		return formats, []string{"a=rtpmap:" + strconv.Itoa(PayloadTypePS) + " PS/90000"}
	}

	videoRaw := p.VideoCodecs
	if len(videoRaw) == 0 {
		videoRaw = DefaultVideoCodecs()
	}
	audioRaw := p.AudioCodecs
	if len(audioRaw) == 0 {
		audioRaw = DefaultAudioCodecs()
	}
	codecs := append(ParseCodecs(videoRaw, 90000), ParseCodecs(audioRaw, 8000)...)

	formats = make([]string, 0, len(codecs))
	rtpmaps = make([]string, 0, len(codecs))
	next := PayloadTypeStart
	for _, c := range codecs {
		pt := 0
		if strings.EqualFold(c.Name, "PS") {
			pt = PayloadTypePS
		} else {
			if next > PayloadTypeMax {
				continue
			}
			pt = next
			next++
		}
		formats = append(formats, strconv.Itoa(pt))
		rtpmaps = append(rtpmaps, "a=rtpmap:"+strconv.Itoa(pt)+" "+c.String())
	}
	if len(formats) == 0 {
		// 配置被过滤空时兜底，至少保证 PS 在通告里。
		formats = []string{strconv.Itoa(PayloadTypePS)}
		rtpmaps = []string{"a=rtpmap:" + strconv.Itoa(PayloadTypePS) + " PS/90000"}
	}
	return formats, rtpmaps
}
