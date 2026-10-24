package sipua

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// 本文件用「裸 SIP 报文」构造一台假平台，而不是 sipgo 的 client ——
// 理由有两个，都不是洁癖：
//  1. sipgo 的客户端事务在收到 INVITE 的 2xx 后会**自动补发一个 ACK**（且复用了 INVITE 的
//     Via branch），这会和真机行为不一致，干扰「ACK 触发推流」这条断言的因果；
//  2. WVP 的 ACK 用的是**全新 branch**（`createAckRequest` 里 getNewViaTag()），
//     手写报文才能原样复现这个形态。
//
// 设备侧（被测试对象）仍是真实的 sipgo Server。

// rawResponse 是一份最小解析的 SIP 响应。
type rawResponse struct {
	status  int
	reason  string
	headers map[string]string
	body    string
	raw     string
}

func (r *rawResponse) header(name string) string {
	return r.headers[strings.ToLower(name)]
}

// rawPlatform 是一台用裸 UDP 报文说话的假平台。
type rawPlatform struct {
	t    *testing.T
	conn *net.UDPConn
	addr string // 平台信令地址 host:port

	mu       sync.Mutex
	fromTag  string
	callID   string
	cseq     int
	branchNo int
}

func startRawPlatform(t *testing.T) *rawPlatform {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("平台监听失败：%v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &rawPlatform{
		t:       t,
		conn:    conn,
		addr:    conn.LocalAddr().String(),
		fromTag: "platfrom-tag-1",
		callID:  "test-call-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		cseq:    100,
	}
}

func (p *rawPlatform) nextBranch() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.branchNo++
	return fmt.Sprintf("z9hG4bKplat%d", p.branchNo)
}

// send 把一段报文发到设备。
func (p *rawPlatform) send(deviceAddr, msg string) {
	p.t.Helper()
	if _, err := p.conn.WriteToUDP([]byte(msg), mustUDPAddr(p.t, deviceAddr)); err != nil {
		p.t.Fatalf("平台发送报文失败：%v", err)
	}
}

// recv 读一条响应，超时则返回 error。
func (p *rawPlatform) recv(timeout time.Duration) (*rawResponse, error) {
	buf := make([]byte, 8192)
	if err := p.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	n, _, err := p.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	return parseRawResponse(string(buf[:n])), nil
}

// mustRecvStatus 读到指定状态码的响应为止（丢弃中间的 100 Trying 等）。
func (p *rawPlatform) mustRecvStatus(t *testing.T, want int, timeout time.Duration) *rawResponse {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("等待 %d 响应超时", want)
		}
		res, err := p.recv(remaining)
		if err != nil {
			t.Fatalf("等待 %d 响应时读包失败：%v", want, err)
		}
		if res.status == want {
			return res
		}
		if res.status >= 200 && res.status != want {
			t.Fatalf("期望 %d，实际收到 %d %s", want, res.status, res.reason)
		}
	}
}

// invite 构造并发送一条实时点播 INVITE，返回 200 OK 响应。
func (p *rawPlatform) invite(t *testing.T, deviceAddr, channelID, sdpBody string) *rawResponse {
	t.Helper()
	host, portStr, _ := net.SplitHostPort(p.addr)
	msg := strings.Join([]string{
		"INVITE sip:" + channelID + "@" + deviceAddr + " SIP/2.0",
		"Via: SIP/2.0/UDP " + host + ":" + portStr + ";rport;branch=" + p.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:34020000002000000001@" + testDomain + ">;tag=" + p.fromTag,
		"To: <sip:" + channelID + "@" + deviceAddr + ">",
		"Call-ID: " + p.callID,
		"CSeq: " + strconv.Itoa(p.cseq) + " INVITE",
		"Contact: <sip:34020000002000000001@" + p.addr + ">",
		"Content-Type: application/sdp",
		"Content-Length: " + strconv.Itoa(len(sdpBody)),
		"",
		sdpBody,
	}, "\r\n")

	p.send(deviceAddr, msg)
	return p.mustRecvStatus(t, 200, 3*time.Second)
}

// ack 发送一条与 WVP 同形态的 ACK（全新 branch）。
func (p *rawPlatform) ack(deviceAddr, channelID, toTag string) {
	p.t.Helper()
	host, portStr, _ := net.SplitHostPort(p.addr)
	msg := strings.Join([]string{
		"ACK sip:" + channelID + "@" + deviceAddr + " SIP/2.0",
		"Via: SIP/2.0/UDP " + host + ":" + portStr + ";rport;branch=" + p.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:34020000002000000001@" + testDomain + ">;tag=" + p.fromTag,
		"To: <sip:" + channelID + "@" + deviceAddr + ">;tag=" + toTag,
		"Call-ID: " + p.callID,
		"CSeq: " + strconv.Itoa(p.cseq) + " ACK",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	p.send(deviceAddr, msg)
}

// bye 发送 BYE 并等待 200 OK。
func (p *rawPlatform) bye(t *testing.T, deviceAddr, channelID, toTag string) *rawResponse {
	t.Helper()
	host, portStr, _ := net.SplitHostPort(p.addr)
	p.mu.Lock()
	p.cseq++
	cseq := p.cseq
	p.mu.Unlock()
	msg := strings.Join([]string{
		"BYE sip:" + channelID + "@" + deviceAddr + " SIP/2.0",
		"Via: SIP/2.0/UDP " + host + ":" + portStr + ";rport;branch=" + p.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:34020000002000000001@" + testDomain + ">;tag=" + p.fromTag,
		"To: <sip:" + channelID + "@" + deviceAddr + ">;tag=" + toTag,
		"Call-ID: " + p.callID,
		"CSeq: " + strconv.Itoa(cseq) + " BYE",
		"Content-Length: 0",
		"",
		"",
	}, "\r\n")
	p.send(deviceAddr, msg)
	return p.mustRecvStatus(t, 200, 3*time.Second)
}

// parseRawResponse 解析一段 SIP 响应文本。
func parseRawResponse(raw string) *rawResponse {
	head, body, _ := strings.Cut(raw, "\r\n\r\n")
	lines := strings.Split(head, "\r\n")
	res := &rawResponse{headers: map[string]string{}, body: body, raw: raw}
	if len(lines) > 0 {
		parts := strings.SplitN(lines[0], " ", 3)
		if len(parts) >= 2 {
			res.status, _ = strconv.Atoi(parts[1])
		}
		if len(parts) >= 3 {
			res.reason = parts[2]
		}
	}
	for _, line := range lines[1:] {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		res.headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}
	return res
}

// toTagOf 从 To 头里取出 tag 参数。
func toTagOf(t *testing.T, res *rawResponse) string {
	t.Helper()
	to := res.header("to")
	_, params, ok := strings.Cut(to, ";")
	if !ok {
		t.Fatalf("200 OK 的 To 头缺少 tag：%q", to)
	}
	for _, p := range strings.Split(params, ";") {
		if name, val, found := strings.Cut(strings.TrimSpace(p), "="); found && name == "tag" {
			return val
		}
	}
	t.Fatalf("未能从 To 头解析出 tag：%q", to)
	return ""
}

func mustUDPAddr(t *testing.T, addr string) *net.UDPAddr {
	t.Helper()
	a, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		t.Fatalf("解析地址失败：%v", err)
	}
	return a
}

// platformPlaySDP 是平台 INVITE 携带的 SDP（与 WVP playStreamCmd 的输出同形）。
func platformPlaySDP(ip string, port int, ssrc string) string {
	return strings.Join([]string{
		"v=0",
		"o=34020000001320000001 0 0 IN IP4 " + ip,
		"s=Play",
		"c=IN IP4 " + ip,
		"t=0 0",
		"m=video " + strconv.Itoa(port) + " RTP/AVP 96 98 97 99",
		"a=recvonly",
		"a=rtpmap:96 PS/90000",
		"y=" + ssrc,
		"",
	}, "\r\n")
}

// fakeMedia 记录 Start / Stop 调用，供断言「ACK 之后才推流」。
type fakeMedia struct {
	started chan struct{}
	stopped chan struct{}

	mu     sync.Mutex
	nStart int
	nStop  int
}

func newFakeMedia() *fakeMedia {
	return &fakeMedia{started: make(chan struct{}, 4), stopped: make(chan struct{}, 4)}
}

func (m *fakeMedia) Start(context.Context) error {
	m.mu.Lock()
	m.nStart++
	m.mu.Unlock()
	select {
	case m.started <- struct{}{}:
	default:
	}
	return nil
}

func (m *fakeMedia) Stop() {
	m.mu.Lock()
	m.nStop++
	m.mu.Unlock()
	select {
	case m.stopped <- struct{}{}:
	default:
	}
}

func (m *fakeMedia) counts() (start, stop int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nStart, m.nStop
}

// TestInviteAckByeDrivesMediaSession 是本批次的端到端主用例：
// INVITE → 100 Trying → 200 OK(设备 SDP) → ACK(起流) → BYE(停流)。
//
// 它同时钉住三条**会在真机上表现为「点播成功但黑屏」**的约束：
//   - 推流目标取自**平台 SDP 的 c= 与 m=**（不是配置里的硬编码地址）；
//   - 200 OK 的 y= 等于**平台 SDP 的 y=**（设备擅自改 SSRC 会触发平台的校正路径）；
//   - 200 OK 的 o= 用户名是**设备编码**（WVP 用它拼 ACK 的 Request-URI）。
func TestInviteAckByeDrivesMediaSession(t *testing.T) {
	platform := startRawPlatform(t)
	devPort := reserveUDPPort(t)
	devAddr := "127.0.0.1:" + strconv.Itoa(devPort)

	ua := newTestUA(t, platform.addr, devPort)
	defer ua.Close()

	var mu sync.Mutex
	var got InviteInfo
	media := newFakeMedia()
	ua.SetMediaFactory(func(info InviteInfo) (MediaSession, int, error) {
		mu.Lock()
		got = info
		mu.Unlock()
		return media, 30001, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	// 平台的收流地址用一个独立端口模拟 ZLM 的 openRtpServer 返回值。
	zlmHost, zlmPortStr, _ := net.SplitHostPort(platform.addr)
	zlmPort := 34567
	const channelID = "34020000001310000001"
	const platformSSRC = "0200000001"

	res := platform.invite(t, devAddr, channelID, platformPlaySDP(zlmHost, zlmPort, platformSSRC))

	// —— 校验 200 OK ——
	if ct := res.header("content-type"); !strings.Contains(strings.ToLower(ct), "sdp") {
		t.Errorf("200 OK 的 Content-Type 应为 SDP，实际 %q", ct)
	}
	if res.header("contact") == "" {
		t.Error("200 OK 应带 Contact 头")
	}
	body := res.body
	for _, want := range []string{
		"v=0",
		"s=Play",
		"c=IN IP4 127.0.0.1",
		"m=video 30001 RTP/AVP 96",
		"a=recvonly",
		"a=rtpmap:96 PS/90000",
		"y=" + platformSSRC, // 未开 override 时必须沿用平台的 SSRC
		"o=" + testDeviceID, // WVP 用它拼 ACK 的 Request-URI
	} {
		if !strings.Contains(body, want) {
			t.Errorf("200 OK 的 SDP 缺少 %q：\n%s", want, body)
		}
	}
	_ = zlmPortStr

	// —— 校验工厂拿到的信息来自平台 SDP ——
	mu.Lock()
	info := got
	mu.Unlock()
	if info.ChannelID != channelID {
		t.Errorf("ChannelID 应为 %q，实际 %q", channelID, info.ChannelID)
	}
	if info.PlatformIP != zlmHost {
		t.Errorf("PlatformIP 应取自平台 SDP 的 c= (%q)，实际 %q", zlmHost, info.PlatformIP)
	}
	if info.PlatformPort != zlmPort {
		t.Errorf("PlatformPort 应取自平台 SDP 的 m= (%d)，实际 %d", zlmPort, info.PlatformPort)
	}
	if info.PlatformSSRC != platformSSRC {
		t.Errorf("PlatformSSRC 应为 %q，实际 %q", platformSSRC, info.PlatformSSRC)
	}
	if info.DeviceSSRC != platformSSRC {
		t.Errorf("未开 override 时 DeviceSSRC 应等于 PlatformSSRC，实际 %q", info.DeviceSSRC)
	}
	if info.SessionName != "Play" {
		t.Errorf("SessionName 应为 Play，实际 %q", info.SessionName)
	}
	if info.CallID != platform.callID {
		t.Errorf("CallID 应为 %q，实际 %q", platform.callID, info.CallID)
	}

	// ACK 之前不应起流。
	if start, _ := media.counts(); start != 0 {
		t.Fatalf("ACK 之前不应启动推流，实际 Start 调用 %d 次", start)
	}

	// —— 发 ACK，应触发推流 ——
	platform.ack(devAddr, channelID, toTagOf(t, res))
	select {
	case <-media.started:
	case <-time.After(3 * time.Second):
		t.Fatal("收到 ACK 后 3 秒内未启动推流")
	}

	// —— 发 BYE，应停流并回 200 ——
	byeRes := platform.bye(t, devAddr, channelID, toTagOf(t, res))
	if byeRes.status != 200 {
		t.Errorf("BYE 应回 200，实际 %d", byeRes.status)
	}
	select {
	case <-media.stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("收到 BYE 后 3 秒内未停止推流")
	}
}

// TestInviteSSRCOverride 验证 --ssrc-override：设备在 200 OK 里回一个与平台不同的 y=，
// 同时**推流也用这个值** —— 这正是平台 updateRtpServerSSRC 校正路径的输入（§13.3.2）。
func TestInviteSSRCOverride(t *testing.T) {
	platform := startRawPlatform(t)
	devPort := reserveUDPPort(t)
	devAddr := "127.0.0.1:" + strconv.Itoa(devPort)

	ua := newTestUA(t, platform.addr, devPort)
	// 走的是同一个 Config，这里直接改（测试内构造，无并发）。
	ua.cfg.SSRCOverride = "0999999999"
	defer ua.Close()

	var mu sync.Mutex
	var got InviteInfo
	ua.SetMediaFactory(func(info InviteInfo) (MediaSession, int, error) {
		mu.Lock()
		got = info
		mu.Unlock()
		return newFakeMedia(), 30002, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	const platformSSRC = "0200000001"
	res := platform.invite(t, devAddr, "34020000001310000001",
		platformPlaySDP("127.0.0.1", 34567, platformSSRC))

	if !strings.Contains(res.body, "y=0999999999") {
		t.Errorf("200 OK 的 SDP 应回 override 的 SSRC：\n%s", res.body)
	}
	if strings.Contains(res.body, "y="+platformSSRC) {
		t.Errorf("200 OK 的 SDP 不应出现平台的 SSRC：\n%s", res.body)
	}

	mu.Lock()
	info := got
	mu.Unlock()
	if info.PlatformSSRC != platformSSRC {
		t.Errorf("PlatformSSRC 应保持平台值 %q，实际 %q", platformSSRC, info.PlatformSSRC)
	}
	if info.DeviceSSRC != "0999999999" {
		t.Errorf("DeviceSSRC 应为 override 值，实际 %q", info.DeviceSSRC)
	}
	// 传给工厂的那一份 InviteInfo 里 SDP 尚未构造（要先拿到工厂返回的本地端口才能写 m=），
	// 这里钉住这个契约，避免将来有人误以为工厂能读到 SDP。
	if info.SDP != "" {
		t.Errorf("传给工厂的 InviteInfo.SDP 应为空（端口确定后才构造），实际：\n%s", info.SDP)
	}
}

// TestInviteMissingYFallsBackSSRC 平台 SDP 没给 y= 时（不合规但确实存在），设备要自拟一个。
func TestInviteMissingYFallsBackSSRC(t *testing.T) {
	platform := startRawPlatform(t)
	devPort := reserveUDPPort(t)
	devAddr := "127.0.0.1:" + strconv.Itoa(devPort)

	ua := newTestUA(t, platform.addr, devPort)
	defer ua.Close()

	var mu sync.Mutex
	var got InviteInfo
	ua.SetMediaFactory(func(info InviteInfo) (MediaSession, int, error) {
		mu.Lock()
		got = info
		mu.Unlock()
		return newFakeMedia(), 30003, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	// SDP 里故意不带 y=
	sdpNoSSRC := "v=0\r\no=34020000001320000001 0 0 IN IP4 127.0.0.1\r\ns=Play\r\n" +
		"c=IN IP4 127.0.0.1\r\nt=0 0\r\nm=video 34567 RTP/AVP 96\r\na=recvonly\r\n"
	res := platform.invite(t, devAddr, "34020000001310000001", sdpNoSSRC)

	mu.Lock()
	info := got
	mu.Unlock()
	want := DefaultSSRC(testDeviceID)
	if info.PlatformSSRC != "" {
		t.Errorf("平台未给 y= 时 PlatformSSRC 应为空，实际 %q", info.PlatformSSRC)
	}
	if info.DeviceSSRC != want {
		t.Errorf("DeviceSSRC 应回落到 %q，实际 %q", want, info.DeviceSSRC)
	}
	if !strings.Contains(res.body, "y="+want) {
		t.Errorf("200 OK 应带上自拟的 SSRC：\n%s", res.body)
	}
}

// sendInviteRaw 发送一条自定义 SDP 的 INVITE，返回首个最终响应（2xx/4xx）。
//
// 与 invite() 的区别：它不预设 200，专门用于断言拒绝路径（488）。
func (p *rawPlatform) sendInviteRaw(t *testing.T, deviceAddr, channelID, sdpBody string) *rawResponse {
	t.Helper()
	host, portStr, _ := net.SplitHostPort(p.addr)
	msg := strings.Join([]string{
		"INVITE sip:" + channelID + "@" + deviceAddr + " SIP/2.0",
		"Via: SIP/2.0/UDP " + host + ":" + portStr + ";rport;branch=" + p.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:34020000002000000001@" + testDomain + ">;tag=" + p.fromTag,
		"To: <sip:" + channelID + "@" + deviceAddr + ">",
		"Call-ID: " + p.callID,
		"CSeq: " + strconv.Itoa(p.cseq) + " INVITE",
		"Contact: <sip:34020000002000000001@" + p.addr + ">",
		"Content-Type: application/sdp",
		"Content-Length: " + strconv.Itoa(len(sdpBody)),
		"",
		sdpBody,
	}, "\r\n")
	p.send(deviceAddr, msg)

	deadline := time.Now().Add(3 * time.Second)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatal("等待 INVITE 最终响应超时")
		}
		res, err := p.recv(remaining)
		if err != nil {
			t.Fatalf("读取 INVITE 响应失败：%v", err)
		}
		if res.status >= 200 {
			return res
		}
	}
}

// tcpPlaySDP 是平台在 TCP 模式下发的 SDP：proto + a=setup 由调用方指定。
func tcpPlaySDP(ip string, port int, ssrc, setup string) string {
	lines := []string{
		"v=0",
		"o=34020000001320000001 0 0 IN IP4 " + ip,
		"s=Play",
		"c=IN IP4 " + ip,
		"t=0 0",
		"m=video " + strconv.Itoa(port) + " TCP/RTP/AVP 96",
		"a=recvonly",
		"a=rtpmap:96 PS/90000",
	}
	if setup != "" {
		lines = append(lines, "a=setup:"+setup)
	}
	lines = append(lines, "y="+ssrc, "")
	return strings.Join(lines, "\r\n")
}

// TestInviteTCPPassiveAccepted 验证 a=setup:passive 的 INVITE 会被接受：
//   - 工厂拿到的 Transport 必须是 tcp-passive（main 据此主动 dial + `$` 分帧）；
//   - 200 OK 的设备侧 SDP 必须是 TCP/RTP/AVP + a=setup:active（对称角色）。
func TestInviteTCPPassiveAccepted(t *testing.T) {
	platform := startRawPlatform(t)
	devPort := reserveUDPPort(t)
	devAddr := "127.0.0.1:" + strconv.Itoa(devPort)

	ua := newTestUA(t, platform.addr, devPort)
	defer ua.Close()

	var mu sync.Mutex
	var got InviteInfo
	media := newFakeMedia()
	ua.SetMediaFactory(func(info InviteInfo) (MediaSession, int, error) {
		mu.Lock()
		got = info
		mu.Unlock()
		return media, 30005, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	const channelID = "34020000001310000001"
	res := platform.invite(t, devAddr, channelID,
		tcpPlaySDP("127.0.0.1", 34567, "0200000001", "passive"))

	for _, want := range []string{
		"m=video 30005 TCP/RTP/AVP 96",
		"a=recvonly",
		"a=rtpmap:96 PS/90000",
		"a=setup:active", // 设备是主动连出方
		"y=0200000001",
		"o=" + testDeviceID,
	} {
		if !strings.Contains(res.body, want) {
			t.Errorf("TCP-PASSIVE 的 200 OK 缺少 %q：\n%s", want, res.body)
		}
	}
	// 设备侧不是被动监听方，不应带 connection:new。
	if strings.Contains(res.body, "a=connection:new") {
		t.Errorf("设备侧 SDP 不应带 a=connection:new：\n%s", res.body)
	}

	mu.Lock()
	info := got
	mu.Unlock()
	if info.Transport != TransportTCPPassive {
		t.Errorf("工厂应收到 Transport=%q，实际 %q", TransportTCPPassive, info.Transport)
	}
	if info.Protocol != "TCP/RTP/AVP" {
		t.Errorf("Protocol 应为 TCP/RTP/AVP，实际 %q", info.Protocol)
	}
	if info.Setup != "passive" {
		t.Errorf("Setup 应为 passive，实际 %q", info.Setup)
	}

	// ACK 之后仍应正常起流（TCP 不影响会话状态机）。
	platform.ack(devAddr, channelID, toTagOf(t, res))
	select {
	case <-media.started:
	case <-time.After(3 * time.Second):
		t.Fatal("TCP-PASSIVE 会话在 ACK 后未启动推流")
	}
}

// TestInviteRejectsTCPActive 平台要求主动连设备（setup:active）时，单端口模式无法满足，
// 必须明确回 488 —— 不能当 UDP 处理，否则是「INVITE 成功但一直没画面」。
func TestInviteRejectsTCPActive(t *testing.T) {
	platform := startRawPlatform(t)
	devPort := reserveUDPPort(t)
	devAddr := "127.0.0.1:" + strconv.Itoa(devPort)

	ua := newTestUA(t, platform.addr, devPort)
	defer ua.Close()
	ua.SetMediaFactory(func(info InviteInfo) (MediaSession, int, error) {
		t.Error("TCP-ACTIVE 的 INVITE 不应调用媒体工厂")
		return newFakeMedia(), 30004, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	res := platform.sendInviteRaw(t, devAddr, "34020000001310000001",
		tcpPlaySDP("127.0.0.1", 34567, "0200000001", "active"))
	if res.status != 488 {
		t.Fatalf("TCP-ACTIVE 应回 488，实际 %d %s", res.status, res.reason)
	}
	if res.reason == "" {
		t.Error("488 应带原因短语")
	}
}

// TestInviteRejectsWhenMediaModeTCPActive --mode tcp-active 时即使平台发 UDP INVITE
// 也一律回 488：该模式与平台单端口收流互斥，模拟器不该假装能推。
func TestInviteRejectsWhenMediaModeTCPActive(t *testing.T) {
	platform := startRawPlatform(t)
	devPort := reserveUDPPort(t)
	devAddr := "127.0.0.1:" + strconv.Itoa(devPort)

	ua := newTestUA(t, platform.addr, devPort)
	defer ua.Close()
	ua.cfg.MediaMode = TransportTCPActive
	ua.SetMediaFactory(func(info InviteInfo) (MediaSession, int, error) {
		t.Error("tcp-active 模式下不应调用媒体工厂")
		return newFakeMedia(), 30006, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	res := platform.sendInviteRaw(t, devAddr, "34020000001310000001",
		platformPlaySDP("127.0.0.1", 34567, "0200000001"))
	if res.status != 488 {
		t.Fatalf("--mode tcp-active 应回 488，实际 %d %s", res.status, res.reason)
	}
}

// TestInviteWithoutFactoryRejected 没有注册媒体工厂时（main 未接线）也应回 488，不能静默丢弃。
func TestInviteWithoutFactoryRejected(t *testing.T) {
	platform := startRawPlatform(t)
	devPort := reserveUDPPort(t)
	devAddr := "127.0.0.1:" + strconv.Itoa(devPort)

	ua := newTestUA(t, platform.addr, devPort)
	defer ua.Close()
	// 故意不注册 MediaFactory。

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	host, portStr, _ := net.SplitHostPort(platform.addr)
	sdpBody := platformPlaySDP("127.0.0.1", 34567, "0200000001")
	msg := strings.Join([]string{
		"INVITE sip:34020000001310000001@" + devAddr + " SIP/2.0",
		"Via: SIP/2.0/UDP " + host + ":" + portStr + ";rport;branch=" + platform.nextBranch(),
		"Max-Forwards: 70",
		"From: <sip:34020000002000000001@" + testDomain + ">;tag=" + platform.fromTag,
		"To: <sip:34020000001310000001@" + devAddr + ">",
		"Call-ID: " + platform.callID,
		"CSeq: 100 INVITE",
		"Content-Type: application/sdp",
		"Content-Length: " + strconv.Itoa(len(sdpBody)),
		"",
		sdpBody,
	}, "\r\n")
	platform.send(devAddr, msg)

	platform.mustRecvStatus(t, 488, 3*time.Second)
}

// TestDefaultSSRC 锁定兜底 SSRC 的形状：10 位、首位 0、对同一设备稳定。
func TestDefaultSSRC(t *testing.T) {
	got := DefaultSSRC("34020000001320000001")
	if len(got) != 10 {
		t.Errorf("SSRC 应为 10 位，实际 %d 位：%q", len(got), got)
	}
	if got[0] != '0' {
		t.Errorf("SSRC 首位应为 0（表示实时点播），实际 %q", got)
	}
	if got != DefaultSSRC("34020000001320000001") {
		t.Error("同一设备的兜底 SSRC 应稳定")
	}
	if got == DefaultSSRC("34020000001320000002") {
		t.Error("不同设备应得到不同 SSRC")
	}
}
