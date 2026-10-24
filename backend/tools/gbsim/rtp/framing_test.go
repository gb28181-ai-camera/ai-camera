package rtp

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

// captureConn 是最小的 net.Conn 替身：只关心 Write，Read 永不到。
type captureConn struct {
	net.Conn
	buf bytes.Buffer
}

func (c *captureConn) Write(p []byte) (int, error) { return c.buf.Write(p) }

func TestFramedConnWritesRFC4571(t *testing.T) {
	base := &captureConn{}
	conn := NewFramedConn(base, 0)

	payload := []byte{0x80, 0x60, 0x00, 0x01, 0xDE, 0xAD, 0xBE, 0xEF}
	n, err := conn.Write(payload)
	if err != nil {
		t.Fatalf("Write 失败：%v", err)
	}
	// 返回入参长度，不是线上字节数。
	if n != len(payload) {
		t.Fatalf("Write 应返回 %d（入参长度），实际 %d", len(payload), n)
	}

	got := base.buf.Bytes()
	if len(got) != FramedHeaderSize+len(payload) {
		t.Fatalf("分帧后长度错误：%d", len(got))
	}
	if got[0] != '$' {
		t.Fatalf("帧首字节应为 '$'，实际 0x%02X", got[0])
	}
	if got[1] != 0 {
		t.Fatalf("RTP 通道号应为 0，实际 %d", got[1])
	}
	if l := binary.BigEndian.Uint16(got[2:4]); int(l) != len(payload) {
		t.Fatalf("帧长度字段应为 %d，实际 %d", len(payload), l)
	}
	if !bytes.Equal(got[4:], payload) {
		t.Fatalf("负载被改写：%x", got[4:])
	}
}

func TestFramedConnMultipleWritesStayFramed(t *testing.T) {
	base := &captureConn{}
	conn := NewFramedConn(base, 1)
	for i := 0; i < 3; i++ {
		if _, err := conn.Write([]byte{byte(i), 0xAA}); err != nil {
			t.Fatalf("第 %d 次 Write 失败：%v", i, err)
		}
	}
	got := base.buf.Bytes()
	if len(got) != 3*(FramedHeaderSize+2) {
		t.Fatalf("三次写入应产生三段帧，实际 %d 字节", len(got))
	}
	// 每个包都必须自带帧头，不能只在第一条前加。
	for i := 0; i < 3; i++ {
		off := i * (FramedHeaderSize + 2)
		if got[off] != '$' || got[off+1] != 1 {
			t.Fatalf("第 %d 帧头错误：%x", i, got[off:off+4])
		}
	}
}

func TestFramedConnRejectsOversize(t *testing.T) {
	conn := NewFramedConn(&captureConn{}, 0)
	if _, err := conn.Write(make([]byte, 0x10000)); err == nil {
		t.Fatal("超过 65535 字节的帧应报错")
	}
}
