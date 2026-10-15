// Package rtp 实现 GB28181 设备侧的 RTP 发送。
//
// 平台的 ZLMediaKit 直接收裸 PS-over-RTP（§2.1 结论一），因此本包只承担两件事：
//
//  1. 把 PS 字节流按 MTU 切包 —— PS 是**字节流**，不像 H.264 那样有 FU-A 之类的
//     分片语义，直接按 1400 字节一段切开即可，靠 RTP 序号还原顺序；
//  2. 填对 RTP 头：SSRC 取自 SDP 的 y= 字段（§6.4），时间戳用 90kHz 时基与 PES 的 PTS 对齐。
//
// 本批次只覆盖 UDP；TCP-PASSIVE / TCP-ACTIVE 需要额外的 `$` 分帧与主动连出，留给后续批次。
package rtp

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
)

// RTP 发送参数。
const (
	// DefaultPayloadType 是 GB28181 视频约定的动态负载类型。
	DefaultPayloadType = 96
	// DefaultMTU 是单个 RTP 包的负载上限。
	// 1400 = 1500(以太网 MTU) - 20(IP) - 8(UDP) - 12(RTP) - 余量。
	DefaultMTU = 1400
	// DefaultClockRate 是 RTP 时间戳的时钟频率，与 PS/PES 的 PTS 时基一致。
	DefaultClockRate = 90000
	// HeaderSize 是 RTP 固定头的长度。
	HeaderSize = 12
)

// Sender 把 PS 字节流打成 RTP 包写入底层连接。
//
// 所有方法都是并发安全的：模拟器可能有多个通道共用一个连接（§2.2 单端口模式）。
type Sender struct {
	conn        net.Conn
	ssrc        uint32
	payloadType byte
	mtu         int

	mu      sync.Mutex
	seq     uint16
	packets uint64
	bytes   uint64
}

// Option 定制 Sender。
type Option func(*Sender)

// WithPayloadType 指定 RTP 负载类型（默认 96）。
func WithPayloadType(pt byte) Option {
	return func(s *Sender) { s.payloadType = pt }
}

// WithMTU 指定单个 RTP 包的负载上限（默认 1400）。
func WithMTU(mtu int) Option {
	return func(s *Sender) { s.mtu = mtu }
}

// WithInitialSequence 指定初始序列号（默认随机/从 0 起），便于测试与排障对齐。
func WithInitialSequence(seq uint16) Option {
	return func(s *Sender) { s.seq = seq }
}

// NewSender 构造 Sender。conn 可以是 *net.UDPConn（UDP 模式）或 TCP 连接（后续批次）。
// ssrc 必须是平台在 SDP y= 字段里通告的值，否则 ZLM 会因为 SSRC 不匹配而无法关联流。
func NewSender(conn net.Conn, ssrc uint32, opts ...Option) *Sender {
	s := &Sender{
		conn:        conn,
		ssrc:        ssrc,
		payloadType: DefaultPayloadType,
		mtu:         DefaultMTU,
	}
	for _, o := range opts {
		o(s)
	}
	// mtu 是**负载**上限，任何正数都合法（测试里会用很小的值来放大分包行为）；
	// 只有非正数才需要回落，否则分包循环会死循环。
	if s.mtu <= 0 {
		s.mtu = DefaultMTU
	}
	return s
}

// SSRC 返回本发送器使用的 SSRC。
func (s *Sender) SSRC() uint32 { return s.ssrc }

// Stats 返回累计发送的包数与负载字节数，模拟器用它打印推流进度。
func (s *Sender) Stats() (packets, payloadBytes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.packets, s.bytes
}

// NextSeq 返回下一个将要使用的序列号，仅用于测试断言。
func (s *Sender) NextSeq() uint16 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seq
}

// SendPSBytes 把一段 PS 字节流按 MTU 切成若干 RTP 包发出。
//
// timestamp 是 90kHz 单位的 RTP 时间戳（通常等于该段 PS 首包的 PTS）。
// 只有**最后一片**的 marker 位为 1，与「标记一个访问单元结束」的约定一致。
// 返回实际发出的包数。
func (s *Sender) SendPSBytes(ps []byte, timestamp uint32) (int, error) {
	if len(ps) == 0 {
		return 0, nil
	}
	sent := 0
	for off := 0; off < len(ps); off += s.mtu {
		end := off + s.mtu
		if end > len(ps) {
			end = len(ps)
		}
		marker := end == len(ps)
		if err := s.SendPacket(ps[off:end], timestamp, marker); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

// SendPacket 发送单个 RTP 包。
func (s *Sender) SendPacket(payload []byte, timestamp uint32, marker bool) error {
	if len(payload) > s.mtu {
		return fmt.Errorf("RTP 负载 %d 字节超过 MTU %d", len(payload), s.mtu)
	}

	s.mu.Lock()
	seq := s.seq
	s.seq++
	buf := make([]byte, HeaderSize+len(payload))
	encodeHeader(buf[:HeaderSize], seq, timestamp, s.ssrc, s.payloadType, marker)
	copy(buf[HeaderSize:], payload)
	s.mu.Unlock()

	if _, err := s.conn.Write(buf); err != nil {
		return fmt.Errorf("发送 RTP 包失败（seq=%d）：%w", seq, err)
	}

	s.mu.Lock()
	s.packets++
	s.bytes += uint64(len(payload))
	s.mu.Unlock()
	return nil
}

// encodeHeader 按 RFC 3550 写 12 字节固定头：
//
//	V=2 | P=0 | X=0 | CC=0        (1 字节)
//	M | PT                        (1 字节)
//	sequence number               (2 字节)
//	timestamp                     (4 字节)
//	SSRC                          (4 字节)
func encodeHeader(dst []byte, seq uint16, ts uint32, ssrc uint32, pt byte, marker bool) {
	dst[0] = 0x80 // 版本 2，无填充、无扩展、CSRC 计数 0
	dst[1] = pt & 0x7F
	if marker {
		dst[1] |= 0x80
	}
	binary.BigEndian.PutUint16(dst[2:4], seq)
	binary.BigEndian.PutUint32(dst[4:8], ts)
	binary.BigEndian.PutUint32(dst[8:12], ssrc)
}

// TimestampFromPTS 把 PS 的 PTS（90kHz）转成 RTP 时间戳。
func TimestampFromPTS(pts uint64) uint32 { return uint32(pts) }

// TimestampFromMillis 把毫秒时刻转成 90kHz 的 RTP 时间戳。
func TimestampFromMillis(ms uint64) uint32 { return uint32(ms * DefaultClockRate / 1000) }

// MillisFromTimestamp 是 TimestampFromMillis 的逆运算。
func MillisFromTimestamp(ts uint32) uint64 { return uint64(ts) * 1000 / DefaultClockRate }
