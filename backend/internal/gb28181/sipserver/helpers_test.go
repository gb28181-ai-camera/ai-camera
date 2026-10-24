package sipserver

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo/sip"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestFromAppConfigDefaults(t *testing.T) {
	cfg, err := FromAppConfig(&config.Config{}, nil)
	if err != nil {
		t.Fatalf("FromAppConfig 失败：%v", err)
	}
	if cfg.ListenIP != "0.0.0.0" || cfg.Port != 5060 || cfg.Transport != TransportUDP {
		t.Fatalf("监听缺省值错误：%+v", cfg)
	}
	if cfg.Domain != "3402000000" || cfg.Realm != "3402000000" {
		t.Fatalf("域缺省值错误：%+v", cfg)
	}
	if cfg.ServerID != "34020000002000000001" {
		t.Fatalf("server_id 缺省值 = %q", cfg.ServerID)
	}
	if cfg.RegisterExpires != 3600 || cfg.RTPProxyPort != 10000 {
		t.Fatalf("注册有效期/端口缺省值错误：%+v", cfg)
	}
	if cfg.AdvertiseIP == "" || cfg.MediaPublicIP == "" {
		t.Fatalf("通告地址不应为空：%+v", cfg)
	}
	if cfg.DefaultCharset != charset.GB2312 {
		t.Fatalf("默认字符集 = %q", cfg.DefaultCharset)
	}
	// 编码列表留空时必须填入 GB28181-2022 的默认值，否则 SDP 只剩 PS 一条通告。
	if len(cfg.VideoCodecs) != 3 || cfg.VideoCodecs[0] != "PS/90000" || cfg.VideoCodecs[2] != "H265/90000" {
		t.Fatalf("默认视频编码列表错误：%v", cfg.VideoCodecs)
	}
	if len(cfg.AudioCodecs) != 3 || cfg.AudioCodecs[0] != "PCMA/8000" || cfg.AudioCodecs[2] != "AAC/44100" {
		t.Fatalf("默认音频编码列表错误：%v", cfg.AudioCodecs)
	}
}

// TestFromAppConfigCodecs 配置里的编码列表要原样（去空白后）进入 sipserver。
func TestFromAppConfigCodecs(t *testing.T) {
	app := &config.Config{}
	app.GB28181.Media.VideoCodecs = []string{" PS/90000 ", "", "H265/90000"}
	app.GB28181.Media.AudioCodecs = []string{"AAC/44100"}
	cfg, err := FromAppConfig(app, nil)
	if err != nil {
		t.Fatalf("FromAppConfig 失败：%v", err)
	}
	if strings.Join(cfg.VideoCodecs, "|") != "PS/90000|H265/90000" {
		t.Fatalf("视频编码列表未按配置生效：%v", cfg.VideoCodecs)
	}
	if strings.Join(cfg.AudioCodecs, "|") != "AAC/44100" {
		t.Fatalf("音频编码列表未按配置生效：%v", cfg.AudioCodecs)
	}
}

func TestFromAppConfigValidation(t *testing.T) {
	bad := &config.Config{}
	bad.GB28181.SIP.Port = 70000
	if _, err := FromAppConfig(bad, nil); err == nil {
		t.Fatalf("非法端口应报错")
	}

	badDomain := &config.Config{}
	badDomain.GB28181.SIP.Domain = "123"
	if _, err := FromAppConfig(badDomain, nil); err == nil {
		t.Fatalf("域名长度非法应报错")
	}

	badTransport := &config.Config{}
	badTransport.GB28181.SIP.Transport = "SCTP"
	if _, err := FromAppConfig(badTransport, nil); err == nil {
		t.Fatalf("非法传输应报错")
	}
}

func TestDeviceIDOf(t *testing.T) {
	req := sip.NewRequest(sip.MESSAGE, sip.Uri{Scheme: "sip", Host: "3402000000"})
	from := &sip.FromHeader{Address: sip.Uri{Scheme: "sip", User: "34020000001320000001", Host: "3402000000"}}
	req.AppendHeader(from)
	if got := deviceIDOf(req); got != "34020000001320000001" {
		t.Fatalf("deviceIDOf = %q", got)
	}

	// From 缺失时回落到 To
	req2 := sip.NewRequest(sip.MESSAGE, sip.Uri{Scheme: "sip", Host: "3402000000"})
	req2.AppendHeader(&sip.ToHeader{Address: sip.Uri{Scheme: "sip", User: "34020000001320000002", Host: "3402000000"}})
	if got := deviceIDOf(req2); got != "34020000001320000002" {
		t.Fatalf("To 回落失败：%q", got)
	}
	if got := deviceIDOf(nil); got != "" {
		t.Fatalf("nil 请求应返回空：%q", got)
	}
}

func TestRegisterExpires(t *testing.T) {
	req := sip.NewRequest(sip.REGISTER, sip.Uri{Scheme: "sip", Host: "3402000000"})
	exp := sip.ExpiresHeader(1800)
	req.AppendHeader(&exp)
	if got := registerExpires(req, 3600); got != 1800 {
		t.Fatalf("Expires 头未生效：%d", got)
	}

	// Expires: 0（注销）必须被识别为 0，而不是被当成「缺失」。
	zero := sip.ExpiresHeader(0)
	req.ReplaceHeader(&zero)
	if got := registerExpires(req, 3600); got != 0 {
		t.Fatalf("Expires:0 未识别：%d", got)
	}

	// 只带 Contact 的 expires 参数
	req2 := sip.NewRequest(sip.REGISTER, sip.Uri{Scheme: "sip", Host: "3402000000"})
	contact := &sip.ContactHeader{Address: sip.Uri{Scheme: "sip", User: "x", Host: "1.2.3.4"}}
	params := sip.NewParams()
	contact.Params = params.Add("expires", "900")
	req2.AppendHeader(contact)
	if got := registerExpires(req2, 3600); got != 900 {
		t.Fatalf("Contact expires 参数未生效：%d", got)
	}

	// 都没有 → 缺省值
	req3 := sip.NewRequest(sip.REGISTER, sip.Uri{Scheme: "sip", Host: "3402000000"})
	if got := registerExpires(req3, 3600); got != 3600 {
		t.Fatalf("缺省值未生效：%d", got)
	}
}

func TestSameAddr(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"10.0.0.9:5060", "10.0.0.9:5060", true},
		{"10.0.0.9:5060", "10.0.0.9:5061", false},
		{"10.0.0.9:5060", "10.0.0.10:5060", false},
		{"[::1]:5060", "[::1]:5060", true},
		{"bad", "10.0.0.9:5060", false},
	}
	for _, c := range cases {
		if got := sameAddr(c.a, c.b); got != c.want {
			t.Fatalf("sameAddr(%q,%q)=%v，期望 %v", c.a, c.b, got, c.want)
		}
	}
}

func TestParseSSRCFromSDP(t *testing.T) {
	body := "v=0\r\no=34020000001320000001 0 0 IN IP4 1.2.3.4\r\nm=video 10000 RTP/AVP 96\r\ny=0200000003\r\n"
	if got := parseSSRCFromSDP(body); got != "0200000003" {
		t.Fatalf("SSRC = %q", got)
	}
	if got := parseSSRCFromSDP("v=0\r\n"); got != "" {
		t.Fatalf("无 y= 时应为空：%q", got)
	}
}

// safeLogContent 必须保证输出是合法 UTF-8，否则 ac_gb_sip_log.content(text) 会写库失败。
func TestSafeLogContentGB2312(t *testing.T) {
	gb, err := simplifiedchinese.GB18030.NewEncoder().String("<Name>东门球机</Name>")
	if err != nil {
		t.Fatalf("构造 GB2312 样本失败：%v", err)
	}
	out := safeLogContent([]byte(gb), charset.GB2312)
	if !utf8.ValidString(out) {
		t.Fatalf("日志内容不是合法 UTF-8：%x", out)
	}
	if !strings.Contains(out, "东门球机") {
		t.Fatalf("GB2312 内容未转成 UTF-8：%q", out)
	}

	// 完全无法解码的字节也必须给出可入库的兜底形式。
	bad := safeLogContent([]byte{0xFF, 0xFE, 0xFF}, charset.GB2312)
	if !utf8.ValidString(bad) {
		t.Fatalf("兜底输出不是合法 UTF-8：%q", bad)
	}
}

func TestSubscriptionEvent(t *testing.T) {
	cases := map[string]string{
		model.GbSubscribeCatalog:        "Catalog",
		model.GbSubscribeAlarm:          "Alarm",
		model.GbSubscribeMobilePosition: "presence",
	}
	for in, want := range cases {
		event, accept, err := subscriptionEvent(in)
		if err != nil || event != want || accept == "" {
			t.Fatalf("subscriptionEvent(%q) = (%q,%q,%v)", in, event, accept, err)
		}
	}
	if _, _, err := subscriptionEvent("unknown"); err == nil {
		t.Fatalf("未知订阅类型应报错")
	}
}

func TestBuildPlaySDP(t *testing.T) {
	s := &Service{cfg: Config{
		ServerID:      "34020000002000000001",
		MediaPublicIP: "10.0.0.5",
		RTPProxyPort:  10000,
	}}
	dev := &model.Device{GbDeviceID: "34020000001320000001"}

	out := s.BuildPlaySDP(dev, "0200000001", "")
	if !strings.Contains(out, "o=34020000001320000001 0 0 IN IP4 10.0.0.5\r\n") {
		t.Fatalf("o= 用户名应为设备编码（契约 §15.5b）：\n%s", out)
	}
	if !strings.Contains(out, "c=IN IP4 10.0.0.5\r\n") || !strings.Contains(out, "y=0200000001\r\n") {
		t.Fatalf("SDP 内容错误：\n%s", out)
	}

	// 设备级 sdp_ip 覆盖全局配置（NAT / 多网卡）。
	dev.SDPIP = "192.168.1.50"
	out = s.BuildPlaySDP(dev, "0200000002", "Play")
	if !strings.Contains(out, "c=IN IP4 192.168.1.50\r\n") {
		t.Fatalf("设备级 sdp_ip 未生效：\n%s", out)
	}
}

// TestBuildPlaySDPStreamModes 锁定「按设备 stream_mode 分支」的三种形态。
//
// 这是本期补的真实缺口：改之前 stream_mode 字段形同虚设，SDP 恒发 UDP，
// 配了 TCP-PASSIVE 的设备只会收到 UDP 通告 → 黑屏且无报错。
func TestBuildPlaySDPStreamModes(t *testing.T) {
	s := &Service{cfg: Config{
		ServerID:      "34020000002000000001",
		MediaPublicIP: "10.0.0.5",
		RTPProxyPort:  10000,
		VideoCodecs:   []string{"PS/90000", "H264/90000"},
		AudioCodecs:   []string{"PCMA/8000"},
	}}

	cases := []struct {
		mode    string
		proto   string
		setup   string
		noSetup bool
	}{
		{mode: "", proto: "RTP/AVP", noSetup: true},
		{mode: "UDP", proto: "RTP/AVP", noSetup: true},
		{mode: "TCP-PASSIVE", proto: "TCP/RTP/AVP", setup: "passive"},
		{mode: "tcp-passive", proto: "TCP/RTP/AVP", setup: "passive"},
		{mode: "TCP-ACTIVE", proto: "TCP/RTP/AVP", setup: "active"},
	}
	for _, c := range cases {
		dev := &model.Device{GbDeviceID: "34020000001320000001", StreamMode: c.mode}
		out := s.BuildPlaySDP(dev, "0200000001", "Play")
		wantM := "m=video 10000 " + c.proto + " 96 97 98\r\n"
		if !strings.Contains(out, wantM) {
			t.Errorf("stream_mode=%q 的 m= 行错误，期望 %q，实际：\n%s", c.mode, strings.TrimSpace(wantM), out)
		}
		if c.noSetup {
			if strings.Contains(out, "a=setup:") {
				t.Errorf("stream_mode=%q 不应带 a=setup：\n%s", c.mode, out)
			}
			continue
		}
		if !strings.Contains(out, "a=setup:"+c.setup+"\r\n") {
			t.Errorf("stream_mode=%q 缺少 a=setup:%s：\n%s", c.mode, c.setup, out)
		}
	}
}

// TestStreamModeOf 锁定大小写与空值的归一化。
func TestStreamModeOf(t *testing.T) {
	cases := map[string]string{
		"":            "udp",
		"UDP":         "udp",
		"TCP-PASSIVE": "tcp-passive",
		"tcp_active":  "tcp-active",
		"unknown":     "udp",
	}
	for in, want := range cases {
		got := string(StreamModeOf(&model.Device{StreamMode: in}))
		if got != want {
			t.Errorf("StreamModeOf(%q) = %q，期望 %q", in, got, want)
		}
	}
	if got := string(StreamModeOf(nil)); got != "udp" {
		t.Errorf("nil 设备应回落到 udp，实际 %q", got)
	}
}

func TestNewCallIDUnique(t *testing.T) {
	s := &Service{cfg: Config{ServerID: "34020000002000000001"}}
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		id := s.NewCallID()
		if id == "" || seen[id] {
			t.Fatalf("Call-ID 重复或为空：%q", id)
		}
		seen[id] = true
		if !strings.HasPrefix(id, "34020000002000000001-") {
			t.Fatalf("Call-ID 应带平台编码前缀：%q", id)
		}
	}
}
