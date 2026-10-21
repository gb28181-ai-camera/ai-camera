package gb28181_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 实时播放 / 云台 / 语音业务层测试
//
// 纯逻辑（注册表、URL、SDP、PTZ 指令）不依赖数据库；
// 通道/设备相关用例使用 testutil.WithTx，跑在 ai_camera_test 上。
//
// 运行：
//   cd backend && go run ./tests/testutil/resetdb/main.go
//   export TEST_DB_HOST=localhost TEST_DB_PORT=5432 TEST_DB_USER=aicamera \
//          TEST_DB_PASSWORD=... TEST_DB_NAME=ai_camera_test
//   go test ./tests/gb28181/ -count=1 -p 1 -v
// ════════════════════════════════════════════════════════════════

// ── 播放注册表 ────────────────────────────────────────────────

// TestPlayRegistryConcurrent 验证并发注册/读取不会丢索引，且 Unregister 清理干净。
func TestPlayRegistryConcurrent(t *testing.T) {
	r := play.NewRegistry()
	const n = 64

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			st := &play.Stream{
				ChannelID:   int64(i + 1),
				GbChannelID: fmt.Sprintf("3402000000132000%04d", i+1),
				SessionID:   fmt.Sprintf("call-%d", i),
				App:         "rtp",
				Stream:      fmt.Sprintf("S%08X", i),
				SSRC:        fmt.Sprintf("034200%04d", i),
			}
			if err := r.Register(st); err != nil {
				t.Errorf("注册第 %d 路失败: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if got := len(r.List()); got != n {
		t.Fatalf("注册后应有 %d 路，实际 %d", n, got)
	}

	// 并发读：按通道、按流名、按 Call-ID 三种索引都应命中。
	var rwg sync.WaitGroup
	for i := 0; i < 128; i++ {
		rwg.Add(1)
		go func(i int) {
			defer rwg.Done()
			idx := i % n
			if _, ok := r.GetByChannel(int64(idx + 1)); !ok {
				t.Errorf("按通道 %d 未命中", idx+1)
			}
			if _, ok := r.GetByStream(fmt.Sprintf("S%08X", idx)); !ok {
				t.Errorf("按流名 %d 未命中", idx)
			}
			if _, ok := r.GetBySessionID(fmt.Sprintf("call-%d", idx)); !ok {
				t.Errorf("按 Call-ID %d 未命中", idx)
			}
		}(i)
	}
	rwg.Wait()

	// 注销一半，校验三种索引同时消失。
	for i := 0; i < n/2; i++ {
		r.Unregister(fmt.Sprintf("S%08X", i))
	}
	if got := len(r.List()); got != n-n/2 {
		t.Fatalf("注销后应剩 %d 路，实际 %d", n-n/2, got)
	}
	if _, ok := r.GetByStream(fmt.Sprintf("S%08X", 0)); ok {
		t.Fatal("已注销的流仍能按流名查到")
	}
	if _, ok := r.GetBySessionID("call-0"); ok {
		t.Fatal("已注销的会话仍能按 Call-ID 查到")
	}
	if _, ok := r.GetByChannel(1); ok {
		t.Fatal("已注销的会话仍能按通道查到")
	}
}

// TestPlayRegistryOverwriteSameChannel 同通道重复注册时只保留最新一路。
func TestPlayRegistryOverwriteSameChannel(t *testing.T) {
	r := play.NewRegistry()
	old := &play.Stream{ChannelID: 7, GbChannelID: "34020000001320000007", SessionID: "call-old", App: "rtp", Stream: "AAAA0001"}
	newer := &play.Stream{ChannelID: 7, GbChannelID: "34020000001320000007", SessionID: "call-new", App: "rtp", Stream: "AAAA0002"}

	if err := r.Register(old); err != nil {
		t.Fatalf("注册旧会话失败: %v", err)
	}
	if err := r.Register(newer); err != nil {
		t.Fatalf("注册新会话失败: %v", err)
	}

	if got := len(r.List()); got != 1 {
		t.Fatalf("同通道应只保留一路，实际 %d", got)
	}
	if _, ok := r.GetByStream(old.Stream); ok {
		t.Fatal("旧流索引未清理")
	}
	if _, ok := r.GetBySessionID(old.SessionID); ok {
		t.Fatal("旧 Call-ID 索引未清理")
	}
	st, ok := r.GetByChannel(7)
	if !ok || st.Stream != newer.Stream {
		t.Fatalf("按通道应取到新流 %s，实际 %+v", newer.Stream, st)
	}
}

// ── URL / SDP ─────────────────────────────────────────────────

// TestHTTPBaseAndPlayURLs 验证 ZLM 播放地址与 http_base 的拼接规则。
func TestHTTPBaseAndPlayURLs(t *testing.T) {
	ms := &model.GbMediaServer{
		StreamIP: "10.0.0.5",
		IP:       "172.16.0.2",
		HTTPPort: 8082,
		RTSPPort: 554,
	}
	if got := play.HTTPBase(ms); got != "http://10.0.0.5:8082" {
		t.Fatalf("http_base 应为对外 stream_ip，实际 %q", got)
	}

	urls := mediaserver.BuildPlayURLs(ms, "rtp", "3B9ACA01")
	if want := "http://10.0.0.5:8082/rtp/3B9ACA01.live.flv"; urls.FLV != want {
		t.Fatalf("FLV 地址错误：want %s got %s", want, urls.FLV)
	}
	if want := "http://10.0.0.5:8082/rtp/3B9ACA01/hls.m3u8"; urls.HLS != want {
		t.Fatalf("HLS 地址错误：want %s got %s", want, urls.HLS)
	}
	if !strings.Contains(urls.WebRTC, "app=rtp") || !strings.Contains(urls.WebRTC, "stream=3B9ACA01") {
		t.Fatalf("WebRTC 地址缺少 app/stream：%s", urls.WebRTC)
	}
	if want := "rtsp://10.0.0.5:554/rtp/3B9ACA01"; urls.RTSP != want {
		t.Fatalf("RTSP 地址错误：want %s got %s", want, urls.RTSP)
	}

	// StreamIP 为空时回落到 IP。
	ms2 := &model.GbMediaServer{IP: "172.16.0.2", HTTPPort: 80}
	if got := play.HTTPBase(ms2); got != "http://172.16.0.2:80" {
		t.Fatalf("StreamIP 为空应回落 IP，实际 %q", got)
	}
	if got := play.HTTPBase(nil); got != "" {
		t.Fatalf("nil 节点应返回空串，实际 %q", got)
	}
}

// TestBuildAudioSDPAndParseEndpoint 验证语音 SDP 的关键行与设备侧地址解析。
func TestBuildAudioSDPAndParseEndpoint(t *testing.T) {
	dev := &model.Device{GbDeviceID: "34020000001320000001"}
	body := play.BuildAudioSDP(dev, "10.0.0.5", 10000, "0342000001", play.SessionNameBroadcast)

	checks := []string{
		"o=34020000001320000001 ",
		"s=Broadcast",
		"m=audio 10000 RTP/AVP 8",
		"a=rtpmap:8 PCMA/8000",
		"y=0342000001",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Fatalf("语音 SDP 缺少 %q：\n%s", want, body)
		}
	}

	// 设备 200 OK：会话级 c= + m=audio 端口。
	deviceSDP := "v=0\r\n" +
		"o=34020000001320000001 0 0 IN IP4 192.168.1.50\r\n" +
		"s=Play\r\n" +
		"c=IN IP4 192.168.1.50\r\n" +
		"t=0 0\r\n" +
		"m=audio 30000 RTP/AVP 8\r\n" +
		"a=rtpmap:8 PCMA/8000\r\n"
	ip, port, ok := play.ParseAudioEndpoint(deviceSDP)
	if !ok || ip != "192.168.1.50" || port != 30000 {
		t.Fatalf("解析设备音频地址失败：ip=%q port=%d ok=%v", ip, port, ok)
	}

	// 只有视频行时必须显式失败，不能用平台自己的地址去推流。
	if _, _, ok := play.ParseAudioEndpoint("v=0\r\nc=IN IP4 192.168.1.50\r\nm=video 30000 RTP/AVP 96\r\n"); ok {
		t.Fatal("纯视频 SDP 不应解析出音频地址")
	}
}

// ── PTZ 指令 ─────────────────────────────────────────────────

// TestBuildPtzCommand 覆盖方向/动作互斥、速度量纲与 8 字节校验和。
func TestBuildPtzCommand(t *testing.T) {
	cmd, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Direction: "up", Speed: 5})
	if err != nil {
		t.Fatalf("向上指令构造失败: %v", err)
	}
	if len(cmd) != 8 || cmd[0] != 0xA5 || cmd[1] != 0x0F || cmd[2] != 0x01 {
		t.Fatalf("指令头错误: % X", cmd)
	}
	if cmd[3] != 0x08 {
		t.Fatalf("up 的指令码应为 0x08，实际 0x%02X", cmd[3])
	}
	if cmd[4] != 5 || cmd[5] != 5 || cmd[6] != 5 {
		t.Fatalf("速度字节应均为 5，实际 % X", cmd)
	}
	var sum byte
	for _, b := range cmd[:7] {
		sum += b
	}
	if cmd[7] != sum {
		t.Fatalf("校验和错误：want 0x%02X got 0x%02X", sum, cmd[7])
	}

	// 缺省速度 5。
	cmdDefault, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Direction: "down"})
	if err != nil {
		t.Fatalf("缺省速度应可用: %v", err)
	}
	if cmdDefault[4] != 5 {
		t.Fatalf("缺省速度应为 5，实际 %d", cmdDefault[4])
	}

	// 多方向按位或：左上 = 0x08 | 0x02。
	cmdUL, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Direction: "up-left", Speed: 8})
	if err != nil {
		t.Fatalf("左上指令构造失败: %v", err)
	}
	if cmdUL[3] != 0x0A {
		t.Fatalf("up-left 的指令码应为 0x0A，实际 0x%02X", cmdUL[3])
	}

	// 动作：变焦放大 0x10。
	cmdZoom, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Action: "zoom-in", Speed: 4})
	if err != nil {
		t.Fatalf("变焦指令构造失败: %v", err)
	}
	if cmdZoom[3] != 0x10 {
		t.Fatalf("zoom-in 的指令码应为 0x10，实际 0x%02X", cmdZoom[3])
	}

	// 停止：指令码 0。
	cmdStop, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Direction: "stop"})
	if err != nil {
		t.Fatalf("停止指令构造失败: %v", err)
	}
	if cmdStop[3] != 0x00 {
		t.Fatalf("stop 的指令码应为 0x00，实际 0x%02X", cmdStop[3])
	}

	// 互斥 / 缺失 / 非法速度 / 未知方向。
	if _, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Direction: "up", Action: "zoom-in"}); !errors.Is(err, gbsvc.ErrGbPtzDirectionConflict) {
		t.Fatalf("direction+action 应返回互斥错误，实际 %v", err)
	}
	if _, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{}); !errors.Is(err, gbsvc.ErrGbPtzCommandMissing) {
		t.Fatalf("缺参数应返回 ErrGbPtzCommandMissing，实际 %v", err)
	}
	if _, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Direction: "up", Speed: 9}); !errors.Is(err, gbsvc.ErrGbPtzSpeedInvalid) {
		t.Fatalf("速度 9 应被拒绝，实际 %v", err)
	}
	if _, err := gbsvc.BuildPtzCommand(&request.GbPtzReq{Direction: "diagonal"}); err == nil {
		t.Fatal("未知方向应报错")
	}
}

// TestSSRCToUint32 验证 SSRC 转 uint32（startSendRtp 用）。
func TestSSRCToUint32(t *testing.T) {
	v, err := play.SSRCToUint32("0342000001")
	if err != nil {
		t.Fatalf("合法 SSRC 转换失败: %v", err)
	}
	if v != 342000001 {
		t.Fatalf("SSRC 转换错误：%d", v)
	}
	if _, err := play.SSRCToUint32("not-a-number"); err == nil {
		t.Fatal("非法 SSRC 应报错")
	}
}

// ── 业务层：通道/设备校验 ─────────────────────────────────────

// TestGbPlayServiceChannelNotFound 通道不存在返回 404 语义错误。
func TestGbPlayServiceChannelNotFound(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		svc := gbsvc.NewGbPlayService()
		_, err := svc.Play(ctx, 99999999, &request.GbPlayReq{})
		if !errors.Is(err, gbsvc.ErrGbChannelNotFound) {
			t.Fatalf("应返回 ErrGbChannelNotFound，实际 %v", err)
		}
	})
}

// TestGbPlayServiceProtocolInvalid 协议白名单在查库之前生效。
func TestGbPlayServiceProtocolInvalid(t *testing.T) {
	svc := gbsvc.NewGbPlayService()
	_, err := svc.Play(context.Background(), 1, &request.GbPlayReq{Protocol: "rtmp"})
	if !errors.Is(err, gbsvc.ErrGbPlayProtocolInvalid) {
		t.Fatalf("应返回 ErrGbPlayProtocolInvalid，实际 %v", err)
	}
}

// TestGbPlayServiceOfflineDevice 设备离线时给出明确中文错误。
func TestGbPlayServiceOfflineDevice(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_LIVE_OFF", "离线相机", "34020000001320000021")
		ch := createChannel(t, ctx, d, "34020000001320000022", "离线通道", "")
		if err := tx.Model(&model.Device{}).Where("id = ?", d.ID).
			Update("status", model.DeviceStatusOffline).Error; err != nil {
			t.Fatalf("更新设备状态失败: %v", err)
		}

		svc := gbsvc.NewGbPlayService()
		_, err := svc.Play(ctx, ch.ID, &request.GbPlayReq{})
		if !errors.Is(err, gbsvc.ErrGbDeviceOffline) {
			t.Fatalf("应返回 ErrGbDeviceOffline，实际 %v", err)
		}
	})
}

// TestGbPlayServiceSIPUnavailable 设备在线但未启动信令服务 → 503 语义错误。
func TestGbPlayServiceSIPUnavailable(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		d := createDevice(t, ctx, "CAM_LIVE_ON", "在线相机", "34020000001320000031")
		ch := createChannel(t, ctx, d, "34020000001320000032", "在线通道", "")

		svc := gbsvc.NewGbPlayService()
		_, err := svc.Play(ctx, ch.ID, &request.GbPlayReq{})
		if !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("应按 503 语义返回 ErrSipServiceUnavailable，实际 %v", err)
		}
	})
}

// TestPlayStatusIdle 未开播时返回 playing=false 的空状态，而不是错误。
func TestPlayStatusIdle(t *testing.T) {
	orch := play.NewOrchestratorWithRegistry(play.NewRegistry())
	st, err := orch.Status(context.Background(), 42)
	if err != nil {
		t.Fatalf("空状态查询不应报错: %v", err)
	}
	if st.Playing || st.ChannelID != 42 {
		t.Fatalf("空状态不正确: %+v", st)
	}
}

// ── 云台业务层 ────────────────────────────────────────────────

// TestGbPtzControlValidationAndSIPUnavailable 参数校验先于信令可用性检查。
func TestGbPtzControlValidationAndSIPUnavailable(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		d := createDevice(t, ctx, "CAM_PTZ_001", "云台相机", "34020000001320000041")
		ch := createChannel(t, ctx, d, "34020000001320000042", "云台通道", "")
		svc := gbsvc.NewGbPtzService()

		if err := svc.Control(ctx, ch.ID, &request.GbPtzReq{Direction: "up", Speed: 9}); !errors.Is(err, gbsvc.ErrGbPtzSpeedInvalid) {
			t.Fatalf("速度 9 应返回 ErrGbPtzSpeedInvalid，实际 %v", err)
		}
		if err := svc.Control(ctx, ch.ID, &request.GbPtzReq{Direction: "up", Action: "zoom-in"}); !errors.Is(err, gbsvc.ErrGbPtzDirectionConflict) {
			t.Fatalf("互斥参数应返回冲突错误，实际 %v", err)
		}
		if err := svc.Control(ctx, ch.ID, &request.GbPtzReq{Direction: "up", Speed: 5}); !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("合法参数应因信令未启动返回 ErrSipServiceUnavailable，实际 %v", err)
		}

		// 预置位编号越界。
		if err := svc.AddPreset(ctx, ch.ID, 0); !errors.Is(err, gbsvc.ErrGbPtzPresetIDInvalid) {
			t.Fatalf("预置位 0 应被拒绝，实际 %v", err)
		}
	})
}

// ── 语音业务层 ────────────────────────────────────────────────

// TestGbVoiceBroadcastDisabled 通道未开启广播时拒绝。
func TestGbVoiceBroadcastDisabled(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		d := createDevice(t, ctx, "CAM_VOICE_001", "语音相机", "34020000001320000051")
		ch := createChannel(t, ctx, d, "34020000001320000052", "语音通道", "")

		svc := gbsvc.NewGbVoiceService()
		if _, err := svc.StartBroadcast(ctx, ch.ID, 1); !errors.Is(err, gbsvc.ErrGbBroadcastDisabled) {
			t.Fatalf("应返回 ErrGbBroadcastDisabled，实际 %v", err)
		}
	})
}

// TestGbVoiceTalkSIPUnavailable 对讲在信令未启动时明确失败。
func TestGbVoiceTalkSIPUnavailable(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		d := createDevice(t, ctx, "CAM_VOICE_002", "对讲相机", "34020000001320000061")
		ch := createChannel(t, ctx, d, "34020000001320000062", "对讲通道", "")

		svc := gbsvc.NewGbVoiceService()
		if _, err := svc.StartTalk(ctx, ch.ID, 1, true); !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("应返回 ErrSipServiceUnavailable，实际 %v", err)
		}
	})
}

// TestGbVoiceSessionNotFound 停止不存在的会话返回明确定义的错误。
func TestGbVoiceSessionNotFound(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		svc := gbsvc.NewGbVoiceService()
		if err := svc.StopBroadcast(ctx, "not-exist"); !errors.Is(err, gbsvc.ErrGbVoiceSessionNotFound) {
			t.Fatalf("应返回 ErrGbVoiceSessionNotFound，实际 %v", err)
		}
	})
}

// TestGotoPresetIDValidation 预置位编号越界必须在触达数据库/信令前就被拒绝。
//
// 这条守的是「预置位只有存、没有用」的回归：GotoPreset 是 PTZCmd 0x82，
// 与 AddPreset(0x81) 只差一个命令码，写错时不会报错、设备也不会动。
func TestGotoPresetIDValidation(t *testing.T) {
	svc := gbsvc.NewGbPtzService()
	ctx := context.Background()
	for _, id := range []int{0, -1, 256, 999} {
		if err := svc.GotoPreset(ctx, 1, id); !errors.Is(err, gbsvc.ErrGbPtzPresetIDInvalid) {
			t.Fatalf("presetID=%d 应返回 ErrGbPtzPresetIDInvalid，实际 %v", id, err)
		}
	}
}
