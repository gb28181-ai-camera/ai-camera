package sdp

import (
	"strings"
	"testing"
)

// wvpPlaySDP 是 WVP 点播 INVITE 实际携带的 SDP，逐字抄自
// `SIPCommander.playStreamCmd`（seniorSdp=true 分支，UDP 模式）。
//
// 抄真家伙的意义：`o=` 的用户名是**设备编码**、`y=` 出现在 m= 之后（即媒体级）、
// 负载类型列表是 96 126 125 99 34 98 97 —— 这三点都是"照着一份想象出来的 SDP 写解析器"
// 时最容易写错的，用真实文本钉住比自造样例可靠。
const wvpPlaySDP = "v=0\r\n" +
	"o=34020000001320000001 0 0 IN IP4 192.168.1.227\r\n" +
	"s=Play\r\n" +
	"c=IN IP4 192.168.1.227\r\n" +
	"t=0 0\r\n" +
	"m=video 10000 RTP/AVP 96 126 125 99 34 98 97\r\n" +
	"a=recvonly\r\n" +
	"a=rtpmap:96 PS/90000\r\n" +
	"a=fmtp:126 profile-level-id=42e01e\r\n" +
	"a=rtpmap:126 H264/90000\r\n" +
	"a=rtpmap:125 H264S/90000\r\n" +
	"a=fmtp:125 profile-level-id=42e01e\r\n" +
	"a=rtpmap:99 H265/90000\r\n" +
	"a=rtpmap:98 H264/90000\r\n" +
	"a=rtpmap:97 MPEG4/90000\r\n" +
	"y=0200000001\r\n"

// TestParseWVPSdp 用 WVP 真实发出的 SDP 校验解析结果。
func TestParseWVPSdp(t *testing.T) {
	s, err := Parse(wvpPlaySDP)
	if err != nil {
		t.Fatalf("解析 WVP SDP 失败：%v", err)
	}

	if s.Version != "0" {
		t.Errorf("v= 应为 0，实际 %q", s.Version)
	}
	if s.Name != "Play" {
		t.Errorf("s= 应为 Play，实际 %q", s.Name)
	}
	// o= 的用户名是设备编码，WVP 靠它构造 ACK 的 Request-URI，不能解析错。
	if s.Origin.Username != "34020000001320000001" {
		t.Errorf("o= 用户名应为设备编码，实际 %q", s.Origin.Username)
	}
	if s.Origin.Address != "192.168.1.227" {
		t.Errorf("o= 地址解析错误：%q", s.Origin.Address)
	}
	if s.ConnectionIP != "192.168.1.227" {
		t.Errorf("会话级 c= 解析错误：%q", s.ConnectionIP)
	}
	if s.Time != "0 0" {
		t.Errorf("t= 解析错误：%q", s.Time)
	}

	v := s.VideoMedia()
	if v == nil {
		t.Fatal("未解析出视频媒体段")
	}
	if v.Port != 10000 {
		t.Errorf("m= 端口应为 10000，实际 %d", v.Port)
	}
	if v.Protocol != "RTP/AVP" {
		t.Errorf("m= 协议应为 RTP/AVP，实际 %q", v.Protocol)
	}
	if len(v.Formats) != 7 || v.Formats[0] != "96" {
		t.Errorf("m= 负载类型列表解析错误：%v", v.Formats)
	}
	if v.Rtpmap("96") != "PS/90000" {
		t.Errorf("a=rtpmap:96 解析错误：%q", v.Rtpmap("96"))
	}
	if !v.HasFormat("96") {
		t.Error("m= 负载类型应包含 96")
	}
	if _, ok := firstAttr(v.Attrs, "recvonly"); !ok {
		t.Error("a=recvonly 未被解析为无值属性")
	}
	if !v.IsUDP() {
		t.Error("RTP/AVP 应被判定为 UDP 收流")
	}
	if s.Setup() != "" {
		t.Errorf("UDP 模式不应有 a=setup，实际 %q", s.Setup())
	}
	if s.VideoConnectionIP() != "192.168.1.227" {
		t.Errorf("收流地址解析错误：%q", s.VideoConnectionIP())
	}
	if s.VideoSSRC() != "0200000001" {
		t.Errorf("y= 解析错误：%q", s.VideoSSRC())
	}
}

// TestParseTCPPassiveSdp 覆盖 TCP-PASSIVE 的 SDP：协议是 TCP/RTP/AVP 且带 a=setup:passive。
// 本批次不实现 TCP 推流，但解析必须能识别出来，好让上层明确回绝而不是当 UDP 处理。
func TestParseTCPPassiveSdp(t *testing.T) {
	text := "v=0\r\n" +
		"o=34020000001320000001 0 0 IN IP4 192.168.1.227\r\n" +
		"s=Play\r\n" +
		"c=IN IP4 192.168.1.227\r\n" +
		"t=0 0\r\n" +
		"m=video 10000 TCP/RTP/AVP 96 126 125 99 34 98 97\r\n" +
		"a=recvonly\r\n" +
		"a=rtpmap:96 PS/90000\r\n" +
		"a=setup:passive\r\n" +
		"a=connection:new\r\n" +
		"y=0200000001\r\n"

	s, err := Parse(text)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if v := s.VideoMedia(); v.Protocol != "TCP/RTP/AVP" {
		t.Errorf("协议应为 TCP/RTP/AVP，实际 %q", v.Protocol)
	}
	if s.Setup() != "passive" {
		t.Errorf("a=setup 应为 passive，实际 %q", s.Setup())
	}
	if s.VideoSSRC() != "0200000001" {
		t.Errorf("y= 解析错误：%q", s.VideoSSRC())
	}
}

// TestVideoSSRCMediaAndSessionLevel 锁定 y= 的两级落位：
// 媒体级优先，媒体级没有时回落到会话级。
func TestVideoSSRCMediaAndSessionLevel(t *testing.T) {
	// y= 放在 m= 之前 → 会话级
	sessionLevel := "v=0\r\ns=Play\r\nc=IN IP4 1.2.3.4\r\nt=0 0\r\ny=SESSIONSSRC\r\nm=video 10000 RTP/AVP 96\r\n"
	s, err := Parse(sessionLevel)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if got := s.VideoSSRC(); got != "SESSIONSSRC" {
		t.Errorf("会话级 y= 应被识别，实际 %q", got)
	}

	// 两级都出现 → 媒体级优先
	both := "v=0\r\ns=Play\r\nc=IN IP4 1.2.3.4\r\nt=0 0\r\ny=SESSIONSSRC\r\nm=video 10000 RTP/AVP 96\r\ny=MEDIASSRC\r\n"
	s2, err := Parse(both)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if got := s2.VideoSSRC(); got != "MEDIASSRC" {
		t.Errorf("媒体级 y= 应优先，实际 %q", got)
	}
}

// TestVideoConnectionIPMediaOverride 锁定媒体级 c= 覆盖会话级 c=。
func TestVideoConnectionIPMediaOverride(t *testing.T) {
	text := "v=0\r\ns=Play\r\nc=IN IP4 10.0.0.1\r\nt=0 0\r\nm=video 10000 RTP/AVP 96\r\nc=IN IP4 10.0.0.99\r\ny=1\r\n"
	s, err := Parse(text)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if got := s.VideoConnectionIP(); got != "10.0.0.99" {
		t.Errorf("媒体级 c= 应覆盖会话级，实际 %q", got)
	}
}

// TestParseRejectsGarbage 缺少 m= 的文本不是合法 SDP，必须报错而不是返回半个结构。
func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Error("空 SDP 应报错")
	}
	if _, err := Parse("v=0\r\ns=Play\r\n"); err == nil {
		t.Error("缺少 m= 的 SDP 应报错")
	}
	if _, err := Parse("not a sdp line\r\n"); err == nil {
		t.Error("格式非法的行应报错")
	}
}

// TestBuildPlayRoundTrip 构造设备侧 SDP 后再解析回来，锁死往返一致性。
func TestBuildPlayRoundTrip(t *testing.T) {
	const ssrc = "0200000001"
	text := BuildPlay(PlayParams{
		DeviceID:  "34020000001320000001",
		LocalIP:   "192.168.1.50",
		LocalPort: 30001,
		SSRC:      ssrc,
	})

	// 行尾必须是 CRLF —— 部分平台（含 WVP 的 jain-sip）对裸 LF 容忍度不同，
	// 统一 CRLF 最省事。
	if !strings.Contains(text, "\r\n") {
		t.Error("SDP 应使用 CRLF 行尾")
	}

	s, err := Parse(text)
	if err != nil {
		t.Fatalf("解析自建 SDP 失败：%v", err)
	}
	if s.Name != "Play" {
		t.Errorf("s= 应为 Play，实际 %q", s.Name)
	}
	if s.Version != "0" || s.Time != "0 0" {
		t.Errorf("v=/t= 不正确：v=%q t=%q", s.Version, s.Time)
	}
	if s.Origin.Username != "34020000001320000001" {
		t.Errorf("o= 用户名应为设备编码，实际 %q", s.Origin.Username)
	}
	if s.Origin.Address != "192.168.1.50" {
		t.Errorf("o= 地址错误：%q", s.Origin.Address)
	}
	if s.ConnectionIP != "192.168.1.50" {
		t.Errorf("c= 应为设备本地 IP，实际 %q", s.ConnectionIP)
	}

	v := s.VideoMedia()
	if v == nil {
		t.Fatal("未解析出视频媒体段")
	}
	if v.Port != 30001 {
		t.Errorf("m= 端口应为 30001，实际 %d", v.Port)
	}
	if v.Protocol != "RTP/AVP" {
		t.Errorf("m= 协议应为 RTP/AVP，实际 %q", v.Protocol)
	}
	if len(v.Formats) != 1 || v.Formats[0] != "96" {
		t.Errorf("负载类型应只有 96，实际 %v", v.Formats)
	}
	if v.Rtpmap("96") != "PS/90000" {
		t.Errorf("a=rtpmap:96 应为 PS/90000，实际 %q", v.Rtpmap("96"))
	}
	if _, ok := firstAttr(v.Attrs, "recvonly"); !ok {
		t.Error("缺少 a=recvonly")
	}
	// 最关键的一条：y= 必须原样带出，否则平台无法关联 SSRC。
	if got := s.VideoSSRC(); got != ssrc {
		t.Errorf("y= 应为 %q，实际 %q", ssrc, got)
	}
}

// TestBuildPlayDefaultsSessionName 未指定会话名时回落 Play。
func TestBuildPlayDefaultsSessionName(t *testing.T) {
	text := BuildPlay(PlayParams{DeviceID: "1", LocalIP: "127.0.0.1", LocalPort: 1, SSRC: "9"})
	if !strings.Contains(text, "\r\ns=Play\r\n") {
		t.Errorf("默认会话名应为 Play：\n%s", text)
	}
}

// TestBuildPlayTCP 设备侧对 TCP-PASSIVE INVITE 的应答：
// m= 用 TCP/RTP/AVP，a=setup 回 active（设备是主动连出方）。
func TestBuildPlayTCP(t *testing.T) {
	text := BuildPlay(PlayParams{
		DeviceID:  "34020000001320000001",
		LocalIP:   "192.168.1.50",
		LocalPort: 30001,
		SSRC:      "0200000001",
		TCP:       true,
		Setup:     "active",
	})
	for _, want := range []string{
		"m=video 30001 TCP/RTP/AVP 96\r\n",
		"a=rtpmap:96 PS/90000\r\n",
		"a=setup:active\r\n",
		"y=0200000001\r\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("TCP 设备 SDP 缺少 %q：\n%s", strings.TrimSpace(want), text)
		}
	}
	// active 侧不需要 connection:new（那是被动监听方的语义）。
	if strings.Contains(text, "a=connection:new") {
		t.Errorf("a=setup:active 不应带 connection:new：\n%s", text)
	}

	// 显式 passive 时补 connection:new，供对端方向的会话复用。
	passive := BuildPlay(PlayParams{DeviceID: "1", LocalIP: "127.0.0.1", LocalPort: 2, SSRC: "9", TCP: true})
	if !strings.Contains(passive, "a=setup:passive\r\n") || !strings.Contains(passive, "a=connection:new\r\n") {
		t.Errorf("TCP 默认应为 passive + connection:new：\n%s", passive)
	}
}
