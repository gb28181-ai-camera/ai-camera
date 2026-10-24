package rtp

import (
	"encoding/binary"
	"fmt"
	"net"
)

// RFC 4571 分帧的固定头长度：`$` + channel + 2 字节长度。
const FramedHeaderSize = 4

// FramedConn 把 net.Conn 包装成 RFC 4571（RTP over TCP）的发送端。
//
// 为什么需要它：TCP 是字节流，RTP 包之间没有边界；GB28181 的 TCP-PASSIVE
// 收流约定每包前置 `$ <channel> <length:uint16>`，ZLM 靠它切包。
// 少了这 4 个字节，ZLM 会把 RTP 头当成帧头解析，表现为「TCP 连上了，
// 但流永远建不起来、也没有任何错误日志」——这是 TCP 模式最隐蔽的坑。
//
// 只重写 Write：每次 Write 必须是一次完整的 RTP 包（rtp.Sender 正是这样写的）。
// 若调用方把多个包拼在一次 Write 里，会被包成一个超长帧；Sender 不会这么做。
type FramedConn struct {
	net.Conn
	// Channel 是交织通道号：0 = RTP，1 = RTCP。
	Channel byte
}

// NewFramedConn 包装一个已建立的 TCP 连接。
func NewFramedConn(conn net.Conn, channel byte) *FramedConn {
	return &FramedConn{Conn: conn, Channel: channel}
}

// Write 输出 `$ <channel> <len16> <payload>`。
//
// 返回值遵循 io.Writer 约定：返回**入参 p 的长度**（而不是线上字节数），
// 否则上层会误判为短写并重试，造成重复发包。
func (c *FramedConn) Write(p []byte) (int, error) {
	if len(p) > 0xFFFF {
		return 0, fmt.Errorf("RFC 4571 单帧上限 65535 字节，实际 %d", len(p))
	}
	buf := make([]byte, FramedHeaderSize+len(p))
	buf[0] = '$'
	buf[1] = c.Channel
	binary.BigEndian.PutUint16(buf[2:FramedHeaderSize], uint16(len(p)))
	copy(buf[FramedHeaderSize:], p)

	// 一次 Write 写完帧头 + 负载：TCP 上分两次写理论可行，但会多一次系统调用，
	// 也给了其他协程插入字节的机会（Sender 虽然串行，但接口层不做这个假设）。
	if _, err := c.Conn.Write(buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

// 编译期断言：FramedConn 必须满足 net.Conn（rtp.NewSender 接收 net.Conn）。
var _ net.Conn = (*FramedConn)(nil)
