package sdp

import (
	"strings"
	"testing"
)

// normalize 把 CRLF 统一成 LF，便于用可读的多行字符串做整份快照。
func normalize(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func TestBuildPlayUDP_DefaultCodecs(t *testing.T) {
	out := BuildPlay(PlayParams{
		OriginUser: "34020000001320000001",
		ServerID:   "34020000002000000001",
		IP:         "10.0.0.5",
		Port:       10000,
		SSRC:       "0200000001",
	})

	// 整份快照（含顺序）。默认编码列表来自 GB28181-2022：
	// 视频 PS/H264/H265 + 音频 PCMA/MPEG4-GENERIC/AAC，PS 固定 96，其余从 97 递增。
	want := `v=0
o=34020000001320000001 0 0 IN IP4 10.0.0.5
s=Play
c=IN IP4 10.0.0.5
t=0 0
m=video 10000 RTP/AVP 96 97 98 99 100 101
a=recvonly
a=rtpmap:96 PS/90000
a=rtpmap:97 H264/90000
a=rtpmap:98 H265/90000
a=rtpmap:99 PCMA/8000
a=rtpmap:100 MPEG4-GENERIC/44100
a=rtpmap:101 AAC/44100
y=0200000001
`
	if got := normalize(out); got != want {
		t.Fatalf("UDP SDP 快照不一致\n实际：\n%s\n期望：\n%s", got, want)
	}
}

func TestBuildPlayTCPPassive_Snapshot(t *testing.T) {
	out := BuildPlay(PlayParams{
		OriginUser:  "34020000001320000001",
		IP:          "10.0.0.5",
		Port:        10000,
		SSRC:        "0200000002",
		Transport:   TransportTCPPassive,
		VideoCodecs: []string{"PS/90000", "H264/90000"},
		AudioCodecs: []string{"PCMA/8000"},
	})

	want := `v=0
o=34020000001320000001 0 0 IN IP4 10.0.0.5
s=Play
c=IN IP4 10.0.0.5
t=0 0
m=video 10000 TCP/RTP/AVP 96 97 98
a=recvonly
a=rtpmap:96 PS/90000
a=rtpmap:97 H264/90000
a=rtpmap:98 PCMA/8000
a=setup:passive
a=connection:new
y=0200000002
`
	if got := normalize(out); got != want {
		t.Fatalf("TCP-PASSIVE SDP 快照不一致\n实际：\n%s\n期望：\n%s", got, want)
	}
}

func TestBuildPlayTCPActive_Snapshot(t *testing.T) {
	out := BuildPlay(PlayParams{
		OriginUser:  "34020000001320000001",
		IP:          "10.0.0.5",
		Port:        10000,
		SSRC:        "0200000003",
		Transport:   TransportTCPActive,
		VideoCodecs: []string{"PS/90000"},
		AudioCodecs: []string{"PCMA/8000"},
	})

	want := `v=0
o=34020000001320000001 0 0 IN IP4 10.0.0.5
s=Play
c=IN IP4 10.0.0.5
t=0 0
m=video 10000 TCP/RTP/AVP 96 97
a=recvonly
a=rtpmap:96 PS/90000
a=rtpmap:97 PCMA/8000
a=setup:active
y=0200000003
`
	if got := normalize(out); got != want {
		t.Fatalf("TCP-ACTIVE SDP 快照不一致\n实际：\n%s\n期望：\n%s", got, want)
	}
	// active 模式由设备监听，不应出现被动侧的 connection:new。
	if strings.Contains(out, "a=connection:new") {
		t.Fatalf("TCP-ACTIVE 不应带 a=connection:new：\n%s", out)
	}
}

func TestBuildPlayLegacyTCPBool(t *testing.T) {
	out := BuildPlay(PlayParams{
		OriginUser: "34020000001320000001",
		IP:         "10.0.0.5",
		Port:       10000,
		SSRC:       "0200000002",
		TCP:        true, // 旧写法：等价 tcp-passive
	})
	if !strings.Contains(out, "m=video 10000 TCP/RTP/AVP ") {
		t.Fatalf("TCP=true 应走 TCP/RTP/AVP：\n%s", out)
	}
	if !strings.Contains(out, "a=setup:passive\r\n") || !strings.Contains(out, "a=connection:new\r\n") {
		t.Fatalf("TCP=true 应等价 tcp-passive：\n%s", out)
	}
}

func TestBuildPlayOriginFallbackAndSessionName(t *testing.T) {
	out := BuildPlay(PlayParams{
		ServerID:    "34020000002000000001",
		IP:          "127.0.0.1",
		Port:        10000,
		SSRC:        "1200000001",
		SessionName: SessionNamePlayback,
	})
	if !strings.Contains(out, "o=34020000002000000001 0 0 IN IP4 127.0.0.1\r\n") {
		t.Fatalf("OriginUser 为空时应回落到 ServerID，实际：\n%s", out)
	}
	if !strings.Contains(out, "s=Playback\r\n") {
		t.Fatalf("回放会话名不正确：\n%s", out)
	}
	if !strings.Contains(out, "y=1200000001\r\n") {
		t.Fatalf("y= 不正确：\n%s", out)
	}
}

// TestBuildPlayCodecPriority 锁定「配置顺序即优先级」与 PS 固定 96。
func TestBuildPlayCodecPriority(t *testing.T) {
	out := BuildPlay(PlayParams{
		IP:          "1.2.3.4",
		Port:        9000,
		SSRC:        "0200000009",
		VideoCodecs: []string{"PS/90000", "H265/90000", "H264/90000"},
		AudioCodecs: []string{"AAC/44100", "PCMA/8000"},
	})
	if !strings.Contains(out, "m=video 9000 RTP/AVP 96 97 98 99 100\r\n") {
		t.Fatalf("负载类型应按配置顺序从 96/97 递增：\n%s", out)
	}
	wantRtpmaps := []string{
		"a=rtpmap:96 PS/90000\r\n",
		"a=rtpmap:97 H265/90000\r\n",
		"a=rtpmap:98 H264/90000\r\n",
		"a=rtpmap:99 AAC/44100\r\n",
		"a=rtpmap:100 PCMA/8000\r\n",
	}
	for _, want := range wantRtpmaps {
		if !strings.Contains(out, want) {
			t.Fatalf("缺少 rtpmap %q：\n%s", strings.TrimSpace(want), out)
		}
	}
}

// TestBuildPlayCodelessConfigFallsBack 配置编码列表全空时仍要给出可用的 PS 通告。
func TestBuildPlayCodelessConfigFallsBack(t *testing.T) {
	out := BuildPlay(PlayParams{
		IP:          "1.2.3.4",
		Port:        9000,
		SSRC:        "0200000009",
		VideoCodecs: nil,
		AudioCodecs: nil,
	})
	// 列表留空（nil / 零长度）→ 视频音频都用默认值。
	if !strings.Contains(out, "m=video 9000 RTP/AVP 96 97 98 99 100 101\r\n") {
		t.Fatalf("空列表应回落到默认编码：\n%s", out)
	}

	// 显式给了列表但全部是空项 —— 跳过无效项后一个都不剩，仍应以 PS 兜底，
	// 保证 m= 行不会出现「没有负载类型」这种非法形态。
	out = BuildPlay(PlayParams{
		IP:           "1.2.3.4",
		Port:         9000,
		SSRC:         "0200000009",
		VideoCodecs:  []string{"", "  "},
		AudioCodecs:  []string{"", "  "},
		PayloadTypes: nil,
	})
	if !strings.Contains(out, "m=video 9000 RTP/AVP 96\r\n") {
		t.Fatalf("空编码项应被跳过并以 PS 兜底：\n%s", out)
	}
	if !strings.Contains(out, "a=rtpmap:96 PS/90000\r\n") {
		t.Fatalf("兜底时仍须通告 PS：\n%s", out)
	}
}

func TestBuildPlayMalformedCodecUsesDefaultClock(t *testing.T) {
	out := BuildPlay(PlayParams{
		IP:          "1.2.3.4",
		Port:        9000,
		SSRC:        "0200000009",
		VideoCodecs: []string{"H265"}, // 漏写时钟频率：视频默认 90000
		AudioCodecs: []string{"PCMA"}, // 音频默认 8000
	})
	if !strings.Contains(out, "a=rtpmap:97 H265/90000\r\n") {
		t.Fatalf("视频编码缺省时钟应为 90000：\n%s", out)
	}
	if !strings.Contains(out, "a=rtpmap:98 PCMA/8000\r\n") {
		t.Fatalf("音频编码缺省时钟应为 8000：\n%s", out)
	}
}

func TestBuildPlayLegacyPayloadTypes(t *testing.T) {
	out := BuildPlay(PlayParams{IP: "1.2.3.4", Port: 9000, SSRC: "0200000009", PayloadTypes: []int{96}})
	if !strings.Contains(out, "m=video 9000 RTP/AVP 96\r\n") {
		t.Fatalf("自定义负载类型未生效：\n%s", out)
	}
	if strings.Count(out, "a=rtpmap:") != 1 {
		t.Fatalf("显式 PayloadTypes 时只应输出 PS 的 rtpmap：\n%s", out)
	}
}

func TestParseTransport(t *testing.T) {
	cases := map[string]Transport{
		"":            TransportUDP,
		"UDP":         TransportUDP,
		"udp":         TransportUDP,
		"TCP-PASSIVE": TransportTCPPassive,
		"tcp_passive": TransportTCPPassive,
		"tcp":         TransportTCPPassive,
		"TCP-ACTIVE":  TransportTCPActive,
		"tcp-active":  TransportTCPActive,
		"  unknown  ": TransportUDP,
		"RTP/AVP":     TransportUDP,
	}
	for in, want := range cases {
		if got := ParseTransport(in); got != want {
			t.Errorf("ParseTransport(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestTransportProtocol(t *testing.T) {
	if got := TransportUDP.Protocol(); got != "RTP/AVP" {
		t.Errorf("UDP 协议应为 RTP/AVP，实际 %q", got)
	}
	if got := TransportTCPPassive.Protocol(); got != "TCP/RTP/AVP" {
		t.Errorf("TCP 协议应为 TCP/RTP/AVP，实际 %q", got)
	}
	if !TransportTCPActive.IsTCP() || TransportUDP.IsTCP() {
		t.Error("IsTCP 判定错误")
	}
}
