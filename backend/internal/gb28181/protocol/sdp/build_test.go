package sdp

import (
	"strings"
	"testing"
)

func TestBuildPlayUDP(t *testing.T) {
	out := BuildPlay(PlayParams{
		OriginUser: "34020000001320000001",
		ServerID:   "34020000002000000001",
		IP:         "10.0.0.5",
		Port:       10000,
		SSRC:       "0200000001",
	})
	lines := strings.Split(out, "\r\n")

	want := []string{
		"v=0",
		// o= 的用户名必须是设备编码（契约 §15.5b），不是平台 ID。
		"o=34020000001320000001 0 0 IN IP4 10.0.0.5",
		"s=Play",
		"c=IN IP4 10.0.0.5",
		"t=0 0",
		"m=video 10000 RTP/AVP 96 97 98",
		"a=recvonly",
		"a=rtpmap:96 PS/90000",
		"y=0200000001",
	}
	for _, w := range want {
		if !strings.Contains(out, w+"\r\n") {
			t.Fatalf("SDP 缺少行 %q\n实际：\n%s", w, out)
		}
	}
	if lines[0] != "v=0" {
		t.Fatalf("首行应为 v=0，实际 %q", lines[0])
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

func TestBuildPlayTCPPassive(t *testing.T) {
	out := BuildPlay(PlayParams{
		OriginUser: "34020000001320000001",
		IP:         "10.0.0.5",
		Port:       10000,
		SSRC:       "0200000002",
		TCP:        true,
	})
	if !strings.Contains(out, "m=video 10000 TCP/RTP/AVP 96 97 98\r\n") {
		t.Fatalf("TCP 模式 m= 行不正确：\n%s", out)
	}
	if !strings.Contains(out, "a=setup:passive\r\n") || !strings.Contains(out, "a=connection:new\r\n") {
		t.Fatalf("TCP 模式缺少 a=setup/a=connection：\n%s", out)
	}
}

func TestBuildPlayCustomPayloads(t *testing.T) {
	out := BuildPlay(PlayParams{IP: "1.2.3.4", Port: 9000, SSRC: "0200000009", PayloadTypes: []int{96}})
	if !strings.Contains(out, "m=video 9000 RTP/AVP 96\r\n") {
		t.Fatalf("自定义负载类型未生效：\n%s", out)
	}
}
