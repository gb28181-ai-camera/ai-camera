package play

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// TestPlayRejectsTCPActive 锁定 TCP-ACTIVE 的明确拒绝路径。
//
// 单端口模式不创建流级 RTP server，ZLM 无法主动拨号设备，因此这条路径
// 必须**在分配 SSRC / 发 INVITE 之前**失败。静默按 UDP 处理会表现为
// 「接口成功、播放器一直转圈」——这是最难排查的一类故障。
func TestPlayRejectsTCPActive(t *testing.T) {
	o := NewOrchestratorWithRegistry(NewRegistry())
	dev := &model.Device{
		GbDeviceID: "34020000001320000001",
		StreamMode: model.GbStreamModeTCPActive,
	}
	ch := &model.GbChannel{GbChannelID: "34020000001310000001"}

	_, err := o.Play(context.Background(), dev, ch, "webrtc")
	if !errors.Is(err, ErrTCPActiveUnsupported) {
		t.Fatalf("应返回 ErrTCPActiveUnsupported，实际 %v", err)
	}
	if !strings.Contains(err.Error(), "单端口") {
		t.Fatalf("错误信息应说明单端口模式互斥：%v", err)
	}
}

// TestPlayUDPAndTCPPassiveNotRejectedEarly 确认只有 TCP-ACTIVE 被提前拒；
// UDP / TCP-PASSIVE 应继续走到信令层（这里没有 SIP 服务，因此期待的是
// ErrSipUnavailable —— 这恰好证明没有被 TCP 分支误拦）。
func TestPlayUDPAndTCPPassiveNotRejectedEarly(t *testing.T) {
	for _, mode := range []string{"", model.GbStreamModeUDP, model.GbStreamModeTCPPassive} {
		o := NewOrchestratorWithRegistry(NewRegistry())
		dev := &model.Device{GbDeviceID: "34020000001320000001", StreamMode: mode}
		ch := &model.GbChannel{GbChannelID: "34020000001310000001"}

		_, err := o.Play(context.Background(), dev, ch, "webrtc")
		if errors.Is(err, ErrTCPActiveUnsupported) {
			t.Errorf("stream_mode=%q 不应被 TCP-ACTIVE 分支拒绝", mode)
		}
		if !errors.Is(err, ErrSipUnavailable) {
			t.Errorf("stream_mode=%q 应走到信令层并因未启动而失败，实际 %v", mode, err)
		}
	}
}

// TestBuildAudioSDPTransportModes 对讲的 SDP 也必须跟随设备传输模式。
func TestBuildAudioSDPTransportModes(t *testing.T) {
	cases := []struct {
		mode      string
		wantM     string
		wantTCP   bool
		wantSetup string
	}{
		{mode: "", wantM: "m=audio 10000 RTP/AVP 8\r\n"},
		{mode: "UDP", wantM: "m=audio 10000 RTP/AVP 8\r\n"},
		{mode: "TCP-PASSIVE", wantM: "m=audio 10000 TCP/RTP/AVP 8\r\n", wantTCP: true, wantSetup: "a=setup:passive"},
		{mode: "TCP-ACTIVE", wantM: "m=audio 10000 TCP/RTP/AVP 8\r\n", wantTCP: true, wantSetup: "a=setup:active"},
	}
	for _, c := range cases {
		dev := &model.Device{GbDeviceID: "34020000001320000001", StreamMode: c.mode}
		out := BuildAudioSDP(dev, "10.0.0.5", 10000, "0342000001", SessionNameTalk)
		if !strings.Contains(out, c.wantM) {
			t.Errorf("mode=%q 的 m= 行错误：\n%s", c.mode, out)
		}
		if !strings.Contains(out, "a=rtpmap:8 PCMA/8000\r\n") {
			t.Errorf("mode=%q 缺少 PCMA rtpmap：\n%s", c.mode, out)
		}
		if c.wantTCP {
			if !strings.Contains(out, c.wantSetup+"\r\n") {
				t.Errorf("mode=%q 缺少 %s：\n%s", c.mode, c.wantSetup, out)
			}
			if c.wantSetup == "a=setup:passive" && !strings.Contains(out, "a=connection:new\r\n") {
				t.Errorf("TCP-PASSIVE 缺少 a=connection:new：\n%s", out)
			}
		} else if strings.Contains(out, "a=setup:") {
			t.Errorf("mode=%q 不应带 a=setup：\n%s", c.mode, out)
		}
	}
}
