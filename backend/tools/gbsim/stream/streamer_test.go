package stream

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"
)

// sampleNALs 是一段最小可用的 H.264 访问单元序列（SPS+PPS+IDR，再一帧非 IDR）。
// 不读文件：单测要能在没有 testdata 的环境里跑，且帧数可控便于断言。
func sampleNALs() [][]byte {
	sps := append([]byte{0x67}, bytes.Repeat([]byte{0xAB}, 20)...)
	pps := append([]byte{0x68}, bytes.Repeat([]byte{0xCD}, 8)...)
	idr := append([]byte{0x65}, bytes.Repeat([]byte{0x11}, 3000)...) // 大到触发 RTP 分包
	nonIDR := append([]byte{0x41}, bytes.Repeat([]byte{0x22}, 100)...)
	return [][]byte{sps, pps, idr, nonIDR}
}

// startSink 起一个假平台收流端，返回监听地址与读包函数。
func startSink(t *testing.T) (*net.UDPConn, string) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("收流端监听失败：%v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn, conn.LocalAddr().String()
}

// drain 收满 timeout 内的所有报文后返回。
func drain(t *testing.T, conn *net.UDPConn, timeout time.Duration) [][]byte {
	t.Helper()
	var out [][]byte
	buf := make([]byte, 2048)
	deadline := time.Now().Add(timeout)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("设置读超时失败：%v", err)
		}
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return out
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		out = append(out, pkt)
	}
}

// TestStreamerSendsRTPToTargetFromSDP 是本包的核心用例，锁定三件事：
//  1. RTP 包真的发到了 Config 里给的**平台收流地址**（模拟 SDP 的 c= / m=）；
//  2. 包里的 SSRC 等于配置值（即设备侧 SDP 的 y=）；
//  3. 负载是合法的 PS 字节流（以 PS 包头起始码开头）。
func TestStreamerSendsRTPToTargetFromSDP(t *testing.T) {
	sink, addr := startSink(t)
	host, portStr, _ := net.SplitHostPort(addr)
	port := mustAtoi(t, portStr)

	const wantSSRC = 0x02000001
	st, err := New(Config{
		NALs:       sampleNALs(),
		SSRC:       wantSSRC,
		TargetIP:   host,
		TargetPort: port,
		LocalIP:    "127.0.0.1",
		FPS:        0, // 不限速，测试里快速跑完
		Loop:       false,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("构造推流会话失败：%v", err)
	}
	defer st.Close()

	if st.LocalRTPPort() == 0 {
		t.Error("LocalRTPPort 应返回真实绑定的端口（用于设备侧 SDP 的 m=）")
	}

	if err := st.Start(context.Background()); err != nil {
		t.Fatalf("启动推流失败：%v", err)
	}
	// 等推流循环自然结束（非 loop，样本放完即返回）。
	waitForStop(t, st)

	packets := drain(t, sink, 500*time.Millisecond)
	if len(packets) == 0 {
		t.Fatal("平台收流端一个 RTP 包都没收到 —— 目标地址或连接有问题")
	}

	// 逐个校验 RTP 头（12 字节固定头）。
	var payload []byte
	for i, pkt := range packets {
		if len(pkt) < 12 {
			t.Fatalf("第 %d 个包短于 RTP 固定头：%d 字节", i, len(pkt))
		}
		if pkt[0]>>6 != 2 {
			t.Errorf("第 %d 个包 RTP 版本应为 2，实际 %d", i, pkt[0]>>6)
		}
		if pt := pkt[1] & 0x7F; pt != 96 {
			t.Errorf("第 %d 个包负载类型应为 96(PS)，实际 %d", i, pt)
		}
		if ssrc := binary.BigEndian.Uint32(pkt[8:12]); ssrc != wantSSRC {
			t.Fatalf("第 %d 个包 SSRC 应为 %08X，实际 %08X —— RTP 未使用配置的 y=", i, wantSSRC, ssrc)
		}
		seq := binary.BigEndian.Uint16(pkt[2:4])
		if i > 0 {
			if seq != (binary.BigEndian.Uint16(packets[i-1][2:4])+1)&0xFFFF {
				t.Errorf("RTP 序号不连续：第 %d 个包 seq=%d", i, seq)
			}
		}
		payload = append(payload, pkt[12:]...)
	}

	if !bytes.HasPrefix(payload, []byte{0x00, 0x00, 0x01, 0xBA}) {
		t.Fatalf("PS 字节流应以 PS 包头起始码 00 00 01 BA 开头，实际 % X", payload[:min(8, len(payload))])
	}
	// PSM 与 PES 也应出现：说明封装链路完整，不只是发了个孤零零的包头。
	if !bytes.Contains(payload, []byte{0x00, 0x00, 0x01, 0xBC}) {
		t.Error("PS 字节流里没有 PSM（00 00 01 BC）")
	}
	if !bytes.Contains(payload, []byte{0x00, 0x00, 0x01, 0xE0}) {
		t.Error("PS 字节流里没有视频 PES（00 00 01 E0）")
	}

	if packets2, bytes2 := st.Stats(); packets2 != uint64(len(packets)) || bytes2 == 0 {
		t.Errorf("统计与实收不符：stats packets=%d bytes=%d，实收 %d 包",
			packets2, bytes2, len(packets))
	}
}

// TestStreamerRejectsMissingTarget 目标地址缺失时必须构造失败，
// 而不是发到 0.0.0.0:0 静默丢包。
func TestStreamerRejectsMissingTarget(t *testing.T) {
	if _, err := New(Config{NALs: sampleNALs(), SSRC: 1, TargetPort: 10000, LocalIP: "127.0.0.1"}); err == nil {
		t.Error("缺少 TargetIP 应报错")
	}
	if _, err := New(Config{NALs: sampleNALs(), SSRC: 1, TargetIP: "127.0.0.1"}); err == nil {
		t.Error("缺少 TargetPort 应报错")
	}
	if _, err := New(Config{SSRC: 1, TargetIP: "127.0.0.1", TargetPort: 10000}); err == nil {
		t.Error("缺少样本码流应报错")
	}
	if _, err := New(Config{NALs: sampleNALs(), SSRC: 1, TargetIP: "not-an-ip", TargetPort: 10000}); err == nil {
		t.Error("目标 IP 不是合法 IP 应报错（SDP 的 c= 只能是 IP）")
	}
}

// TestStreamerDurationStops 验证 --duration 到期后自动停推。
func TestStreamerDurationStops(t *testing.T) {
	sink, addr := startSink(t)
	host, portStr, _ := net.SplitHostPort(addr)

	st, err := New(Config{
		NALs:       sampleNALs(),
		SSRC:       0x02000002,
		TargetIP:   host,
		TargetPort: mustAtoi(t, portStr),
		LocalIP:    "127.0.0.1",
		FPS:        100, // 放快些，确保 duration 内一直在推
		Loop:       true,
		Duration:   300 * time.Millisecond,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("构造推流会话失败：%v", err)
	}
	defer st.Close()

	start := time.Now()
	if err := st.Start(context.Background()); err != nil {
		t.Fatalf("启动推流失败：%v", err)
	}
	waitForStop(t, st)
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Fatalf("--duration 未生效，推流持续了 %s", elapsed)
	}
	packets := drain(t, sink, 300*time.Millisecond)
	if len(packets) == 0 {
		t.Error("duration 期间应至少推出一部分包")
	}
}

// TestStreamerLoopKeepsSending 验证 loop=true 时不会一轮就停。
func TestStreamerLoopKeepsSending(t *testing.T) {
	_, addr := startSink(t)
	host, portStr, _ := net.SplitHostPort(addr)

	st, err := New(Config{
		NALs:       sampleNALs(),
		SSRC:       0x02000003,
		TargetIP:   host,
		TargetPort: mustAtoi(t, portStr),
		LocalIP:    "127.0.0.1",
		FPS:        200,
		Loop:       true,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("构造推流会话失败：%v", err)
	}
	defer st.Close()

	ctx, cancel := context.WithCancel(context.Background())
	if err := st.Start(ctx); err != nil {
		t.Fatalf("启动推流失败：%v", err)
	}
	time.Sleep(400 * time.Millisecond)
	packets1, _ := st.Stats()
	time.Sleep(400 * time.Millisecond)
	packets2, _ := st.Stats()
	cancel()
	st.Stop()

	if packets1 == 0 || packets2 <= packets1 {
		t.Errorf("loop 模式下包数应持续增长：t1=%d t2=%d", packets1, packets2)
	}
}

// waitForStop 等待推流循环自然结束（非 loop / duration 到期）。
func waitForStop(t *testing.T, s *Streamer) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		done, started := s.done, s.started
		s.mu.Unlock()
		if started {
			select {
			case <-done:
				return
			default:
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("推流循环未在 3 秒内结束")
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("端口不是数字：%q", s)
	}
	return n
}
