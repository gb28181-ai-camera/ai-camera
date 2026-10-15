package rtp

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

// memConn 是一个把写入内容攒在内存里的假连接，用于确定性断言 RTP 包内容。
// 只实现 Write；其余 net.Conn 方法在测试里用不到（嵌入的 nil 接口足够满足类型）。
//
// ⚠️ mu 是必需的：真实 net.Conn 允许并发 Write，Sender 也确实是并发使用的
// （TestSendPSBytesConcurrent 从 8 个 goroutine 同时发）。没有这把锁，
// 测试用的 bytes.Buffer 会在 -race 下报数据竞争 —— 那是假连接的问题，不是 Sender 的。
type memConn struct {
	net.Conn
	mu  sync.Mutex
	buf bytes.Buffer
}

func (m *memConn) Write(b []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Write(b)
}

type parsedPacket struct {
	version     byte
	marker      bool
	payloadType byte
	seq         uint16
	timestamp   uint32
	ssrc        uint32
	payload     []byte
}

// splitPackets 按给定负载长度把字节流切成 RTP 包并解析头部。
func splitPackets(t *testing.T, data []byte, payloadLen int) []parsedPacket {
	t.Helper()
	packetLen := HeaderSize + payloadLen
	if len(data)%packetLen != 0 {
		t.Fatalf("字节流长度 %d 不是包长 %d 的整数倍", len(data), packetLen)
	}
	var out []parsedPacket
	for off := 0; off < len(data); off += packetLen {
		raw := data[off : off+packetLen]
		p := parsedPacket{
			version:     raw[0] >> 6,
			marker:      raw[1]&0x80 != 0,
			payloadType: raw[1] & 0x7F,
			seq:         binary.BigEndian.Uint16(raw[2:4]),
			timestamp:   binary.BigEndian.Uint32(raw[4:8]),
			ssrc:        binary.BigEndian.Uint32(raw[8:12]),
			payload:     append([]byte(nil), raw[12:]...),
		}
		out = append(out, p)
	}
	return out
}

// TestSendPSBytesFragmentation 验证分包：字节流被正确切开、序号递增、只有最后一包带 marker。
func TestSendPSBytesFragmentation(t *testing.T) {
	conn := &memConn{}
	s := NewSender(conn, 0x0A1B2C3D, WithMTU(1400), WithInitialSequence(1000))

	// 3000 字节 → 1400 + 1400 + 200，共 3 包
	payload := make([]byte, 3000)
	for i := range payload {
		payload[i] = byte(i % 253)
	}
	n, err := s.SendPSBytes(payload, 90000)
	if err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	if n != 3 {
		t.Fatalf("期望切成 3 包，实际 %d", n)
	}

	// 前两包满 1400，最后一包 200
	data := conn.buf.Bytes()
	if len(data) != 2*(HeaderSize+1400)+(HeaderSize+200) {
		t.Fatalf("总字节数不符：%d", len(data))
	}
	pkts := splitPackets(t, data[:2*(HeaderSize+1400)], 1400)
	last := splitPackets(t, data[2*(HeaderSize+1400):], 200)
	pkts = append(pkts, last...)

	for i, p := range pkts {
		if p.version != 2 {
			t.Fatalf("第 %d 包 RTP 版本应为 2，实际 %d", i, p.version)
		}
		if p.payloadType != DefaultPayloadType {
			t.Fatalf("第 %d 包负载类型应为 %d，实际 %d", i, DefaultPayloadType, p.payloadType)
		}
		if p.ssrc != 0x0A1B2C3D {
			t.Fatalf("第 %d 包 SSRC 不符：%08x", i, p.ssrc)
		}
		if p.timestamp != 90000 {
			t.Fatalf("第 %d 包时间戳不符：%d", i, p.timestamp)
		}
		if wantSeq := uint16(1000 + i); p.seq != wantSeq {
			t.Fatalf("第 %d 包序号不符：want %d got %d", i, wantSeq, p.seq)
		}
		wantMarker := i == len(pkts)-1
		if p.marker != wantMarker {
			t.Fatalf("第 %d 包 marker 应为 %v，实际 %v", i, wantMarker, p.marker)
		}
	}

	// 负载拼回来必须与原始字节流逐字节一致
	var joined []byte
	for _, p := range pkts {
		joined = append(joined, p.payload...)
	}
	if !bytes.Equal(joined, payload) {
		t.Fatalf("负载重组不一致：len want %d got %d", len(payload), len(joined))
	}
	if pkts, bytes := s.Stats(); pkts != 3 || bytes != 3000 {
		t.Fatalf("统计不符：packets=%d bytes=%d", pkts, bytes)
	}
}

// TestSendSmallPayloadSinglePacket 验证小负载只产生一包且 marker 置位。
func TestSendSmallPayloadSinglePacket(t *testing.T) {
	conn := &memConn{}
	s := NewSender(conn, 1, WithInitialSequence(7))
	if _, err := s.SendPSBytes([]byte{0x00, 0x00, 0x01, 0xBA}, 3600); err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	pkts := splitPackets(t, conn.buf.Bytes(), 4)
	if len(pkts) != 1 {
		t.Fatalf("期望 1 包，实际 %d", len(pkts))
	}
	if !pkts[0].marker {
		t.Fatal("单包场景 marker 应为 1")
	}
	if pkts[0].seq != 7 {
		t.Fatalf("初始序号未生效：%d", pkts[0].seq)
	}
	if !bytes.Equal(pkts[0].payload, []byte{0x00, 0x00, 0x01, 0xBA}) {
		t.Fatalf("负载不符：%x", pkts[0].payload)
	}
}

// TestSendEmptyDoesNothing 验证空输入不产生任何包。
func TestSendEmptyDoesNothing(t *testing.T) {
	conn := &memConn{}
	s := NewSender(conn, 1)
	n, err := s.SendPSBytes(nil, 0)
	if err != nil || n != 0 {
		t.Fatalf("空输入应返回 (0, nil)，实际 (%d, %v)", n, err)
	}
	if conn.buf.Len() != 0 {
		t.Fatalf("空输入不应写出字节，实际 %d", conn.buf.Len())
	}
}

// TestSequenceWrapsAround 验证 16 位序号回绕（模拟器长时间推流必然遇到）。
func TestSequenceWrapsAround(t *testing.T) {
	conn := &memConn{}
	s := NewSender(conn, 1, WithInitialSequence(65534), WithMTU(4))
	if _, err := s.SendPSBytes(make([]byte, 12), 0); err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	pkts := splitPackets(t, conn.buf.Bytes(), 4)
	if len(pkts) != 3 {
		t.Fatalf("期望 3 包，实际 %d", len(pkts))
	}
	want := []uint16{65534, 65535, 0}
	for i, p := range pkts {
		if p.seq != want[i] {
			t.Fatalf("第 %d 包序号回绕错误：want %d got %d", i, want[i], p.seq)
		}
	}
}

// TestSendPacketRejectsOversize 验证超 MTU 的负载被拒绝，而不是被静默截断。
func TestSendPacketRejectsOversize(t *testing.T) {
	s := NewSender(&memConn{}, 1, WithMTU(100))
	if err := s.SendPacket(make([]byte, 101), 0, false); err == nil {
		t.Fatal("超过 MTU 的负载应当报错")
	}
}

// TestMTUFallback 验证非正 MTU 回落到默认值，避免分包循环死循环。
// 小的正数 MTU 是合法值（负载上限），不应被改写。
func TestMTUFallback(t *testing.T) {
	for _, bad := range []int{0, -1, -1400} {
		s := NewSender(&memConn{}, 1, WithMTU(bad))
		if s.mtu != DefaultMTU {
			t.Fatalf("MTU=%d 应回落到默认 %d，实际 %d", bad, DefaultMTU, s.mtu)
		}
	}
	s := NewSender(&memConn{}, 1, WithMTU(4))
	if s.mtu != 4 {
		t.Fatalf("MTU=4 是合法负载上限，不应被改写，实际 %d", s.mtu)
	}
}

// TestTimestampConversions 验证 90kHz 时基换算（PES 的 PTS 与 RTP 时间戳必须同源）。
func TestTimestampConversions(t *testing.T) {
	if got := TimestampFromMillis(1000); got != 90000 {
		t.Fatalf("1 秒应为 90000 个 90kHz 时钟：%d", got)
	}
	if got := TimestampFromMillis(40); got != 3600 {
		t.Fatalf("40ms（25fps 帧间隔）应为 3600：%d", got)
	}
	if got := TimestampFromPTS(90000); got != 90000 {
		t.Fatalf("PTS 与 RTP 时间戳应同源：%d", got)
	}
	if got := MillisFromTimestamp(90000); got != 1000 {
		t.Fatalf("90000 个时钟应为 1000ms：%d", got)
	}
}

// TestSendOverRealUDP 用真实 UDP 回环做一次端到端发送，确认 Sender 与 net.Conn 组合可用。
func TestSendOverRealUDP(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Skipf("无法建立回环 UDP 监听，跳过：%v", err)
	}
	defer server.Close()

	client, err := net.Dial("udp4", server.LocalAddr().String())
	if err != nil {
		t.Fatalf("拨号失败：%v", err)
	}
	defer client.Close()

	s := NewSender(client, 0x11223344, WithMTU(500), WithInitialSequence(1))
	payload := bytes.Repeat([]byte{0xAB}, 1200) // 3 包
	n, err := s.SendPSBytes(payload, 12345)
	if err != nil {
		t.Fatalf("发送失败：%v", err)
	}
	if n != 3 {
		t.Fatalf("期望 3 包，实际 %d", n)
	}

	if err := server.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("设置读超时失败：%v", err)
	}
	buf := make([]byte, 2048)
	var got []byte
	for i := 0; i < 3; i++ {
		read, _, err := server.ReadFromUDP(buf)
		if err != nil {
			t.Fatalf("读取第 %d 个包失败：%v", i, err)
		}
		got = append(got, buf[12:read]...)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("经真实 UDP 重组后负载不一致：len want %d got %d", len(payload), len(got))
	}
}

// TestSendPSBytesConcurrent 验证并发发送时序号不重复、字节数统计正确。
func TestSendPSBytesConcurrent(t *testing.T) {
	conn := &memConn{}
	s := NewSender(conn, 1, WithMTU(100))
	const workers = 8
	const perWorker = 10
	const payloadLen = 250 // 每个 worker 每次发 3 包

	done := make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < perWorker; j++ {
				if _, err := s.SendPSBytes(make([]byte, payloadLen), uint32(j)); err != nil {
					t.Errorf("并发发送失败：%v", err)
					return
				}
			}
		}()
	}
	for i := 0; i < workers; i++ {
		<-done
	}

	wantPackets := uint64(workers * perWorker * 3)
	if packets, byts := s.Stats(); packets != wantPackets || byts != uint64(workers*perWorker*payloadLen) {
		t.Fatalf("并发统计不符：packets=%d want %d, bytes=%d want %d",
			packets, wantPackets, byts, workers*perWorker*payloadLen)
	}
	if got := s.NextSeq(); got != uint16(wantPackets) {
		t.Fatalf("序号总数不符：%d want %d", got, wantPackets)
	}
}
