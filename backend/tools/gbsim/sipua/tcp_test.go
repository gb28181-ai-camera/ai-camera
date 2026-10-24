package sipua

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// 本文件验证 SIP over TCP 的「客户端长连接」模型：
//
//	设备只由 sipgo 的 client dial 平台，不监听任何本地端口；
//	平台沿同一条连接下发的 MESSAGE 仍能进入 UA 的 handler 并得到 200 OK。
//
// 这个模型是早期实现失败的地方：照搬 UDP 的 ListenAndServe 会让 client
// 再 bind 一次同端口而报 `Only one usage of each socket address`。

// rawTCPSession 是一条已建立的平台侧 TCP 连接（含缓冲读）。
type rawTCPSession struct {
	t    *testing.T
	conn net.Conn
	br   *bufio.Reader
}

// rawTCPPlatform 是用裸 TCP 报文说话的假平台。
type rawTCPPlatform struct {
	t    *testing.T
	ln   net.Listener
	addr string // 平台信令地址 host:port
}

func startRawTCPPlatform(t *testing.T) *rawTCPPlatform {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("平台 TCP 监听失败：%v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return &rawTCPPlatform{t: t, ln: ln, addr: ln.Addr().String()}
}

// acceptDataConn 跳过「连上即关、没有数据」的探测连接，返回承载 SIP 的长连接。
//
// UA.Start 会先做一次可达性探测（dial 后立即 close），因此平台侧
// 收到的第一条连接通常没有数据。
func (p *rawTCPPlatform) acceptDataConn(timeout time.Duration) *rawTCPSession {
	p.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			p.t.Fatal("等待设备建立 SIP 长连接超时")
		}
		_ = p.ln.(*net.TCPListener).SetDeadline(time.Now().Add(remaining))
		conn, err := p.ln.Accept()
		if err != nil {
			p.t.Fatalf("接受 TCP 连接失败：%v", err)
		}

		br := bufio.NewReader(conn)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := br.Peek(1); err != nil {
			// 探测连接没有正文，关闭后继续等真正的 REGISTER 连接。
			_ = conn.Close()
			continue
		}
		_ = conn.SetReadDeadline(time.Time{})
		return &rawTCPSession{t: p.t, conn: conn, br: br}
	}
}

// readMessage 读取一条完整的 SIP 报文（头 + Content-Length 正文）。
func (s *rawTCPSession) readMessage() (string, error) {
	var b strings.Builder
	length := 0
	for {
		line, err := s.br.ReadString('\n')
		if err != nil {
			return "", err
		}
		b.WriteString(line)
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if name, value, ok := strings.Cut(trimmed, ":"); ok &&
			strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, _ = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	if length > 0 {
		body := make([]byte, length)
		if _, err := io.ReadFull(s.br, body); err != nil {
			return "", err
		}
		b.Write(body)
	}
	return b.String(), nil
}

// write 发送一段原始报文。
func (s *rawTCPSession) write(msg string) {
	s.t.Helper()
	if _, err := io.WriteString(s.conn, msg); err != nil {
		s.t.Fatalf("平台发送报文失败：%v", err)
	}
}

// header 取首行之后的某个头（大小写不敏感），不存在返回空串。
func sipHeader(msg, name string) string {
	lines := strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n")
	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), name) {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// okResponseFor 构造 200 OK：Via/From/To/Call-ID/CSeq 原样回抄，补一个 To tag。
func okResponseFor(req string) string {
	to := sipHeader(req, "To")
	if !strings.Contains(to, "tag=") {
		to += ";tag=platform-tcp-tag"
	}
	return strings.Join([]string{
		"SIP/2.0 200 OK",
		"Via: " + sipHeader(req, "Via"),
		"From: " + sipHeader(req, "From"),
		"To: " + to,
		"Call-ID: " + sipHeader(req, "Call-ID"),
		"CSeq: " + sipHeader(req, "CSeq"),
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
}

// reserveTCPPort 探一个当前空闲的 TCP 端口，供设备侧作为 client 的源端口。
func reserveTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("探测空闲 TCP 端口失败：%v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// TestSIPTCPClientLongConnection 是 SIP over TCP 的主用例。
func TestSIPTCPClientLongConnection(t *testing.T) {
	platform := startRawTCPPlatform(t)
	devPort := reserveTCPPort(t)

	ua, err := New(Config{
		DeviceID:  testDeviceID,
		SIPServer: platform.addr,
		SIPDomain: testDomain,
		Password:  testPassword,
		LocalIP:   "127.0.0.1",
		LocalPort: devPort,
		Transport: "tcp",
		Expires:   3600,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("构造 UA 失败：%v", err)
	}
	defer ua.Close()

	received := make(chan string, 1)
	ua.SetMessageHandler(func(body []byte, _ string) [][]byte {
		select {
		case received <- string(body):
		default:
		}
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 TCP UA 失败：%v", err)
	}

	// —— 关键断言：Start 后本地端口必须仍可被 bind，即 UA 没有监听 ——
	lnProbe, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(devPort)))
	if err != nil {
		t.Fatalf("UA 不应监听本地端口 %d（bind 失败说明开了监听）：%v", devPort, err)
	}
	_ = lnProbe.Close()

	// —— REGISTER：由 client 主动 dial 平台 ——
	registerErr := make(chan error, 1)
	go func() { registerErr <- ua.Register(ctx) }()

	sess := platform.acceptDataConn(5 * time.Second)
	defer sess.conn.Close()

	req, err := sess.readMessage()
	if err != nil {
		t.Fatalf("读取 REGISTER 失败：%v", err)
	}
	if !strings.HasPrefix(req, "REGISTER ") {
		t.Fatalf("首条报文应为 REGISTER，实际：\n%s", req)
	}
	sess.write(okResponseFor(req))

	if err := <-registerErr; err != nil {
		t.Fatalf("TCP 注册失败：%v", err)
	}
	if !ua.IsRegistered() {
		t.Fatal("注册成功后 IsRegistered 应为 true")
	}

	// —— 平台沿同一条连接下发 MESSAGE：证明入向请求复用 client 的长连接 ——
	deviceContact := ua.Contact()
	body := `<Query><CmdType>Catalog</CmdType><SN>7</SN><DeviceID>` + testDeviceID + `</DeviceID></Query>`
	msg := strings.Join([]string{
		"MESSAGE " + deviceContact + " SIP/2.0",
		"Via: SIP/2.0/TCP " + platform.addr + ";branch=z9hG4bKtcpmsg1",
		"Max-Forwards: 70",
		"From: <sip:34020000002000000001@" + testDomain + ">;tag=platform-tcp-msg",
		"To: <" + deviceContact + ">",
		"Call-ID: tcp-long-connection-1",
		"CSeq: 1 MESSAGE",
		"Contact: <sip:34020000002000000001@" + platform.addr + ">",
		"Content-Type: Application/MANSCDP+xml",
		"Content-Length: " + strconv.Itoa(len(body)),
		"",
		body,
	}, "\r\n")
	sess.write(msg)

	// 设备先回 200 OK。
	res, err := sess.readMessage()
	if err != nil {
		t.Fatalf("读取 MESSAGE 的 200 OK 失败（入向请求可能没走上长连接）：%v", err)
	}
	if !strings.HasPrefix(res, "SIP/2.0 200") {
		t.Fatalf("MESSAGE 应回 200 OK，实际：\n%s", res)
	}

	// handler 确实收到正文。
	select {
	case got := <-received:
		if !strings.Contains(got, "CmdType>Catalog") {
			t.Fatalf("handler 收到的正文不正确：%s", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("入向 MESSAGE 未进入 handler（TCP 长连接的读循环可能没接上 Server）")
	}
}

// TestStartTCPSkipsListening 直接锁定「TCP 不监听」这一模型选择。
func TestStartTCPSkipsListening(t *testing.T) {
	platform := startRawTCPPlatform(t)
	devPort := reserveTCPPort(t)

	ua, err := New(Config{
		DeviceID:  testDeviceID,
		SIPServer: platform.addr,
		SIPDomain: testDomain,
		LocalIP:   "127.0.0.1",
		LocalPort: devPort,
		Transport: "tcp",
		Expires:   3600,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("构造 UA 失败：%v", err)
	}
	defer ua.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动失败：%v", err)
	}
	// Start 只做可达性探测（dial 后立即关），平台侧会看到一个空连接。
	ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", devPort))
	if err != nil {
		t.Fatalf("TCP UA 不应监听本地端口：%v", err)
	}
	_ = ln.Close()
}
