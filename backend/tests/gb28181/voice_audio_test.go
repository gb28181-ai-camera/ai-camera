package gb28181_test

// 语音音频链路的纯函数测试。
//
// 这些用例**不碰数据库、不碰 SIP、不碰 ZLM**：只验证 G.711、RTP 头、
// 序号重排与降级决策。这样在没有真实设备/媒体节点的环境里也能跑，
// 与同目录下依赖 testutil.WithTx 的业务测试互不影响。
//
// 运行：
//   go test ./tests/gb28181/ -run Voice -count=1 -p 1
// 若本机 config/config.yaml 指向不可用的库，加上 CONFIG_PATH=/nonexistent
// 让 bootstrap 跳过初始化（这些用例本身不需要它）。

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/gb28181/audio"
)

// TestVoiceAudioG711RoundTripStable 验证 A-law 字节 → PCM16 → A-law
// 对 256 个取值都能回到原字节（G.711 是量化编码，PCM 原值不可逆，
// 但「字节往返稳定」是链路不漂移的硬要求）。
func TestVoiceAudioG711RoundTripStable(t *testing.T) {
	for value := 0; value < 256; value++ {
		pcm := audio.PCMAToPCM16([]byte{byte(value)})
		if len(pcm) != 2 {
			t.Fatalf("A-law 0x%02X 解码后应得到 2 字节，实际 %d", value, len(pcm))
		}
		encoded, err := audio.PCM16ToPCMA(pcm)
		if err != nil {
			t.Fatalf("PCM16ToPCMA 失败: %v", err)
		}
		if len(encoded) != 1 || encoded[0] != byte(value) {
			t.Fatalf("A-law 往返不稳定: 0x%02X -> 0x%02X", value, encoded[0])
		}
	}
}

// TestVoiceAudioG711KnownValues 固定几个已知样点，防止实现被无意改坏。
func TestVoiceAudioG711KnownValues(t *testing.T) {
	cases := []struct {
		alaw byte
		pcm  int16
	}{
		{0xD5, 8},     // 静音附近的量化值（A-law 无 0，最小正值为 8）
		{0x55, -8},    // 对应的负值
		{0xAA, 32256}, // 饱和正端
		{0x2A, -32256},
	}
	for _, tc := range cases {
		if got := audio.PCMADecodeSample(tc.alaw); got != tc.pcm {
			t.Fatalf("0x%02X 解码应为 %d，实际 %d", tc.alaw, tc.pcm, got)
		}
		if got := audio.PCMAEncodeSample(tc.pcm); got != tc.alaw {
			t.Fatalf("%d 编码应为 0x%02X，实际 0x%02X", tc.pcm, tc.alaw, got)
		}
	}

	if _, err := audio.PCM16ToPCMA([]byte{0x00, 0x01, 0x02}); !errors.Is(err, audio.ErrOddPCM16Length) {
		t.Fatalf("奇数字节 PCM16 应返回 ErrOddPCM16Length，实际 %v", err)
	}
}

// TestVoiceAudioRTPBuildAndParse 验证最小头封装与解析的字段一致性。
func TestVoiceAudioRTPBuildAndParse(t *testing.T) {
	payload := []byte{0xD5, 0x55, 0xAA, 0x2A}
	raw := audio.BuildRTP(audio.RTPHeader{
		Marker:         true,
		PayloadType:    8,
		SequenceNumber: 65534,
		Timestamp:      123456,
		SSRC:           0x0BEBC201,
	}, payload)

	p, err := audio.ParseRTP(raw)
	if err != nil {
		t.Fatalf("ParseRTP 失败: %v", err)
	}
	if p.Version != 2 || !p.Marker || p.PayloadType != 8 {
		t.Fatalf("固定头字段不符: %+v", p)
	}
	if p.SequenceNumber != 65534 || p.Timestamp != 123456 || p.SSRC != 0x0BEBC201 {
		t.Fatalf("序号/时间戳/SSRC 不符: %+v", p)
	}
	if string(p.Payload) != string(payload) {
		t.Fatalf("载荷不符: %x != %x", p.Payload, payload)
	}
	if !p.IsPCMA() {
		t.Fatalf("PT=8 应识别为 PCMA")
	}
}

// TestVoiceAudioRTPParseRejectsMalformed 验证长度字段越界不会被静默接受。
func TestVoiceAudioRTPParseRejectsMalformed(t *testing.T) {
	if _, err := audio.ParseRTP([]byte{0x80}); !errors.Is(err, audio.ErrRTPTooShort) {
		t.Fatalf("短包应返回 ErrRTPTooShort，实际 %v", err)
	}

	// 版本号不是 2。
	bad := make([]byte, 12)
	bad[0] = 0x40
	if _, err := audio.ParseRTP(bad); !errors.Is(err, audio.ErrRTPVersion) {
		t.Fatalf("版本错误应返回 ErrRTPVersion，实际 %v", err)
	}

	// CSRC 计数声称 15 个，但报文只有固定头。
	bad = make([]byte, 12)
	bad[0] = 0x82 | 0x0F
	if _, err := audio.ParseRTP(bad); !errors.Is(err, audio.ErrRTPTruncated) {
		t.Fatalf("CSRC 越界应返回 ErrRTPTruncated，实际 %v", err)
	}

	// padding 计数大于可用载荷长度（CC=0，否则会先触发 CSRC 越界）。
	bad = make([]byte, 13)
	bad[0] = 0x80 | 0x20
	bad[12] = 5
	if _, err := audio.ParseRTP(bad); !errors.Is(err, audio.ErrRTPPadding) {
		t.Fatalf("padding 越界应返回 ErrRTPPadding，实际 %v", err)
	}
}

// TestVoiceAudioRTPExtensionAndPadding 验证扩展头与 padding 的剥离。
func TestVoiceAudioRTPExtensionAndPadding(t *testing.T) {
	// 固定头 + 1 个 32-bit 扩展字 + 载荷 + 2 字节 padding。
	ext := []byte{0xBE, 0xDE, 0x00, 0x01, 0x11, 0x22, 0x33, 0x44}
	raw := make([]byte, 12)
	raw[0] = 0x80 | 0x10 | 0x20 // V=2, X=1, P=1
	raw[1] = 8
	binary.BigEndian.PutUint16(raw[2:4], 7)
	raw = append(raw, ext...)
	raw = append(raw, []byte{0xD5, 0x55}...)
	raw = append(raw, 0x00, 0x02)

	p, err := audio.ParseRTP(raw)
	if err != nil {
		t.Fatalf("ParseRTP 失败: %v", err)
	}
	if !p.Extension || p.PaddingSize != 2 {
		t.Fatalf("扩展/padding 标记不符: %+v", p)
	}
	if string(p.Payload) != string([]byte{0xD5, 0x55}) {
		t.Fatalf("载荷应剔除 padding，实际 %x", p.Payload)
	}
	if len(p.ExtensionData) != len(ext) {
		t.Fatalf("扩展头长度不符: %d", len(p.ExtensionData))
	}
}

// TestVoiceAudioSequenceBufferReorders 验证乱序重排。
func TestVoiceAudioSequenceBufferReorders(t *testing.T) {
	buf := audio.NewSequenceBuffer(8)
	packet := func(seq uint16) *audio.RTPPacket {
		return &audio.RTPPacket{SequenceNumber: seq, Payload: []byte{byte(seq)}}
	}

	if got := buf.Push(packet(100)); len(got) != 1 || got[0].SequenceNumber != 100 {
		t.Fatalf("首包应立即输出，实际 %v", seqs(got))
	}

	// 102 先到、101 后到：102 暂存，101 到达后 101、102 连续输出。
	if got := buf.Push(packet(102)); len(got) != 0 {
		t.Fatalf("102 应暂存，实际输出 %v", seqs(got))
	}
	got := buf.Push(packet(101))
	if len(got) != 2 || got[0].SequenceNumber != 101 || got[1].SequenceNumber != 102 {
		t.Fatalf("应按 101,102 输出，实际 %v", seqs(got))
	}

	// 重复包丢弃。
	if got := buf.Push(packet(101)); len(got) != 0 {
		t.Fatalf("重复包应丢弃，实际 %v", seqs(got))
	}

	stats := buf.Stats()
	if stats.Duplicates != 1 || stats.Reordered == 0 {
		t.Fatalf("统计不符: %+v", stats)
	}
}

// TestVoiceAudioSequenceBufferToleratesLoss 验证缺口超过容量时强制推进，
// 并把跳过的序号计入 Lost —— 实时语音不能无限等一帧。
func TestVoiceAudioSequenceBufferToleratesLoss(t *testing.T) {
	buf := audio.NewSequenceBuffer(3)
	packet := func(seq uint16) *audio.RTPPacket {
		return &audio.RTPPacket{SequenceNumber: seq}
	}

	if got := buf.Push(packet(10)); len(got) != 1 {
		t.Fatalf("首包应输出: %v", seqs(got))
	}
	// 12..15 都缺 11：pending 超过容量后强制吐出。
	var out []*audio.RTPPacket
	for seq := uint16(12); seq <= 15; seq++ {
		out = append(out, buf.Push(packet(seq))...)
	}
	if len(out) == 0 {
		t.Fatalf("超过容量后应强制输出，实际为空")
	}
	last := out[len(out)-1].SequenceNumber
	if last != 15 {
		t.Fatalf("最终应推进到 15，实际 %d", last)
	}
	if buf.Stats().Lost == 0 {
		t.Fatalf("应统计到丢包")
	}
	if buf.Flush() != nil {
		t.Fatalf("强制输出后 pending 应为空")
	}
}

// TestVoiceAudioSequenceBufferWraparound 验证 16 位序号回绕。
func TestVoiceAudioSequenceBufferWraparound(t *testing.T) {
	buf := audio.NewSequenceBuffer(8)
	packet := func(seq uint16) *audio.RTPPacket {
		return &audio.RTPPacket{SequenceNumber: seq}
	}

	got := buf.Push(packet(65534))
	if len(got) != 1 {
		t.Fatalf("首包应输出")
	}
	// 65535 之后是 0；先到 0 再补 65535。
	if got := buf.Push(packet(0)); len(got) != 0 {
		t.Fatalf("0 应暂存等待 65535，实际 %v", seqs(got))
	}
	got = buf.Push(packet(65535))
	if len(got) != 2 || got[0].SequenceNumber != 65535 || got[1].SequenceNumber != 0 {
		t.Fatalf("回绕应按 65535,0 输出，实际 %v", seqs(got))
	}
}

// TestVoiceAudioSSRCAndStreamNaming 验证 SSRC 解析与 Agent 流名推导。
func TestVoiceAudioSSRCAndStreamNaming(t *testing.T) {
	v, err := audio.ParseSSRC("0038200001")
	if err != nil {
		t.Fatalf("ParseSSRC 失败: %v", err)
	}
	if v != 38200001 {
		t.Fatalf("SSRC 解析错误: %d", v)
	}
	if got := audio.StreamNameFromSSRC(v); got != "0246E2C1" {
		t.Fatalf("流名应为大写十六进制，实际 %s", got)
	}
	if _, err := audio.ParseSSRC("not-a-number"); err == nil {
		t.Fatalf("非法 SSRC 应报错")
	}

	deviceStream := "0BEBC201"
	agentStream := audio.AgentStreamName(deviceStream)
	if agentStream != "AI0BEBC201" {
		t.Fatalf("Agent 流名不符: %s", agentStream)
	}
	if agentStream == deviceStream {
		t.Fatalf("Agent 流名绝不能与设备流相同（否则会自激）")
	}
	// 非法字符被剔除，但仍保持确定性。
	if got := audio.AgentStreamName(" a/b c "); got != "AIabc" {
		t.Fatalf("非法字符应被剔除，实际 %q", got)
	}
}

// TestVoiceAudioSessionStreamEncoding 验证「设备流 + Pipecat 端口」在
// ac_voice_session.Stream 这一个字段里的编码/解码（没有端口列）。
func TestVoiceAudioSessionStreamEncoding(t *testing.T) {
	encoded := audio.EncodeSessionStream("0BEBC201", 41234)
	if encoded != "0BEBC201|41234" {
		t.Fatalf("编码结果不符: %q", encoded)
	}
	stream, port := audio.DecodeSessionStream(encoded)
	if stream != "0BEBC201" || port != 41234 {
		t.Fatalf("解码结果不符: %q %d", stream, port)
	}

	// 纯对讲/广播：没有端口，保持旧语义。
	if got := audio.EncodeSessionStream("0BEBC201", 0); got != "0BEBC201" {
		t.Fatalf("无端口时不应追加分隔符: %q", got)
	}
	stream, port = audio.DecodeSessionStream("0BEBC201")
	if stream != "0BEBC201" || port != 0 {
		t.Fatalf("旧数据应按设备流解析: %q %d", stream, port)
	}

	// 非法端口不报错，退化为纯设备流（停止会话不能被展示字段卡住）。
	stream, port = audio.DecodeSessionStream("0BEBC201|not-a-port")
	if stream != "0BEBC201" || port != 0 {
		t.Fatalf("非法端口应退化为 0: %q %d", stream, port)
	}
}

// TestVoiceAudioDecideAgentDegrade 验证降级决策：
// 用户没请求、控制面不可达、Pipecat 自报未就绪，任一条件都必须是 false。
//
// 注意：这里判断的是 agent_ready，不是会话开始瞬间的 audio_bridge
// （那时设备还没推流，必然是 degraded，不能据此放弃）。
func TestVoiceAudioDecideAgentDegrade(t *testing.T) {
	cases := []struct {
		want      bool
		reachable bool
		ready     bool
		expect    bool
	}{
		{true, true, true, true},
		{false, true, true, false},
		{true, false, true, false},
		{true, true, false, false},
	}
	for _, tc := range cases {
		if got := audio.DecideAgent(tc.want, tc.reachable, tc.ready); got != tc.expect {
			t.Fatalf("DecideAgent(%v,%v,%v)=%v，期望 %v",
				tc.want, tc.reachable, tc.ready, got, tc.expect)
		}
	}

	// AudioBridgeConnected 只认 connected；degraded/not_connected 都不算闭环。
	if !audio.AudioBridgeConnected("connected") || !audio.AudioBridgeConnected("CONNECTED") {
		t.Fatalf("connected 应视为可用")
	}
	if audio.AudioBridgeConnected("degraded") || audio.AudioBridgeConnected("not_connected") {
		t.Fatalf("degraded/not_connected 不应视为可用")
	}
}

// seqs 把包列表转成序号切片，便于失败信息阅读。
func seqs(packets []*audio.RTPPacket) []uint16 {
	out := make([]uint16, 0, len(packets))
	for _, p := range packets {
		out = append(out, p.SequenceNumber)
	}
	return out
}
