package sipua

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

// 这些测试用一台「假平台」在回环上验证真实 SIP 交互。
//
// 它锁定的不是摘要公式（那由 RFC 2617 向量锁定），而是**接线**：
//   - sipgo 的 Client 与 Server 共用同一个 UDP 端口（否则平台回复打不到设备）；
//   - 无凭证 REGISTER → 401 → 带 Authorization 重发 → 200 的完整握手；
//   - 403 被映射成 ErrForbidden（需求 2 的验收点）。
//
// 其中「同一端口收发」是本设计里最大的未知数：它在不可用时不会报错，
// 只会表现为「注册成功但收不到下行指令」，所以必须有测试钉住。

const (
	testDeviceID = "34020000001320000001"
	testDomain   = "3402000000"
	testPassword = "sim-secret"
)

// fakePlatform 是一台最小 SIP 平台：只处理 REGISTER。
type fakePlatform struct {
	t        *testing.T
	srv      *sipgo.Server
	ua       *sipgo.UserAgent
	conn     net.UDPConn
	addr     string
	realm    string
	password string
	reject   bool // 为 true 时直接回 403（模拟「设备未建档」）

	mu          sync.Mutex
	challenges  int
	authTries   int
	lastAuth    []byte
	lastSource  string
	sawFromUser string
}

func startFakePlatform(t *testing.T, realm, password string, reject bool) *fakePlatform {
	t.Helper()
	ua, err := sipgo.NewUA()
	if err != nil {
		t.Fatalf("构造平台 UA 失败：%v", err)
	}
	srv, err := sipgo.NewServer(ua)
	if err != nil {
		t.Fatalf("构造平台 Server 失败：%v", err)
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("平台监听失败：%v", err)
	}
	p := &fakePlatform{
		t: t, srv: srv, ua: ua, conn: *conn,
		addr: conn.LocalAddr().String(), realm: realm, password: password, reject: reject,
	}
	srv.OnRegister(p.handleRegister)
	go func() { _ = srv.ServeUDP(conn) }()
	return p
}

func (p *fakePlatform) close() {
	_ = p.srv.Close()
	_ = p.ua.Close()
	_ = p.conn.Close()
}

func (p *fakePlatform) handleRegister(req *sip.Request, tx sip.ServerTransaction) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.lastSource = req.Source()
	if from := req.From(); from != nil {
		p.sawFromUser = from.Address.User
	}

	if p.reject {
		_ = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusForbidden, "Forbidden", nil))
		return
	}

	auth := req.GetHeader("Authorization")
	if auth == nil {
		p.challenges++
		res := sip.NewResponseFromRequest(req, sip.StatusUnauthorized, "Unauthorized", nil)
		res.AppendHeader(sip.NewHeader("WWW-Authenticate",
			fmt.Sprintf(`Digest realm="%s", qop="auth", nonce="%s", algorithm=MD5`,
				p.realm, "0f1e2d3c4b5a6978")))
		_ = tx.Respond(res)
		return
	}

	p.authTries++
	p.lastAuth = []byte(auth.Value())

	params, err := parseAuthParams(strings.TrimPrefix(auth.Value(), "Digest "))
	if err != nil {
		p.t.Errorf("平台侧解析 Authorization 失败：%v", err)
		_ = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusForbidden, "Bad Authorization", nil))
		return
	}
	// 平台侧独立复算：用解析出的参数 + 明文密码重算摘要。
	want := DigestResponse(params["username"], params["realm"], p.password,
		req.Method.String(), params["uri"], params["nonce"], params["cnonce"], params["qop"], 1)
	if want != params["response"] {
		p.t.Logf("平台侧摘要不匹配：want %s got %s（uri=%q）", want, params["response"], params["uri"])
		_ = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusForbidden, "Wrong password", nil))
		return
	}
	res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
	exp := sip.ExpiresHeader(3600)
	res.AppendHeader(&exp)
	_ = tx.Respond(res)
}

func (p *fakePlatform) stats() (challenges, authTries int, lastAuth string, lastSource, fromUser string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.challenges, p.authTries, string(p.lastAuth), p.lastSource, p.sawFromUser
}

// reserveUDPPort 探一个当前空闲的 UDP 端口，供被测试的 UA 监听。
func reserveUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("探测空闲端口失败：%v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	_ = conn.Close()
	return port
}

func newTestUA(t *testing.T, platformAddr string, port int) *UA {
	t.Helper()
	ua, err := New(Config{
		DeviceID:  testDeviceID,
		SIPServer: platformAddr,
		SIPDomain: testDomain,
		Password:  testPassword,
		LocalIP:   "127.0.0.1",
		LocalPort: port,
		Transport: "udp",
		Expires:   3600,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("构造 UA 失败：%v", err)
	}
	return ua
}

// TestRegisterDigestHandshake 走完整握手：401 挑战 → 带凭证重发 → 200。
func TestRegisterDigestHandshake(t *testing.T) {
	platform := startFakePlatform(t, testDomain, testPassword, false)
	defer platform.close()

	ua := newTestUA(t, platform.addr, reserveUDPPort(t))
	defer ua.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}

	if err := ua.Register(ctx); err != nil {
		t.Fatalf("注册失败：%v", err)
	}
	if !ua.IsRegistered() {
		t.Fatal("注册成功后 IsRegistered 应为 true")
	}
	if ua.ExpiresAt().Before(time.Now()) {
		t.Fatalf("过期时刻应晚于当前时间：%s", ua.ExpiresAt())
	}

	challenges, authTries, auth, source, fromUser := platform.stats()
	if challenges != 1 {
		t.Errorf("期望平台发出 1 次 401 挑战，实际 %d", challenges)
	}
	if authTries != 1 {
		t.Errorf("期望平台收到 1 次带凭证 REGISTER，实际 %d", authTries)
	}
	if fromUser != testDeviceID {
		t.Errorf("From 头的 user 应为设备编码，实际 %q", fromUser)
	}
	// 摘要五要素齐全
	for _, must := range []string{"username=", "realm=", "nonce=", "uri=", `qop=auth`, "nc=00000001", "cnonce=", "response="} {
		if !strings.Contains(auth, must) {
			t.Errorf("Authorization 头缺少 %q：%s", must, auth)
		}
	}

	// ⚠️ 关键断言：设备发给平台的报文，源端口必须等于设备自己的监听端口。
	// 若 sipgo 的 Client 没复用监听 socket，这里会是一个临时端口 ——
	// 平台随后的 INVITE / MESSAGE 就会打到一个没人收的端口上。
	_, devicePortStr, err := net.SplitHostPort(source)
	if err != nil {
		t.Fatalf("解析源地址失败：%v", err)
	}
	if devicePortStr != fmt.Sprint(ua.cfg.LocalPort) {
		t.Fatalf("出向报文的源端口（%s）必须等于设备监听端口（%d）——"+
			"否则平台无法把下行指令送回设备", devicePortStr, ua.cfg.LocalPort)
	}
	if ua.LastRegisterResponse() == nil || ua.LastRegisterResponse().StatusCode != sip.StatusOK {
		t.Fatalf("LastRegisterResponse 应为 200：%+v", ua.LastRegisterResponse())
	}
}

// TestRegisterForbiddenMapsToErrForbidden 平台 403（设备未建档）必须映射成 ErrForbidden。
// 这是需求 2「SIP 设备必须是设备管理里有的设备」在模拟器侧的验收点。
func TestRegisterForbiddenMapsToErrForbidden(t *testing.T) {
	platform := startFakePlatform(t, testDomain, testPassword, true)
	defer platform.close()

	ua := newTestUA(t, platform.addr, reserveUDPPort(t))
	defer ua.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}
	err := ua.Register(ctx)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("期望 ErrForbidden，实际 %v", err)
	}
	if ua.IsRegistered() {
		t.Fatal("403 后不应处于已注册状态")
	}
}

// TestRegisterWrongPasswordGetsForbidden 密码错误时平台会先挑战再 403，同样应得到 ErrForbidden。
func TestRegisterWrongPasswordGetsForbidden(t *testing.T) {
	platform := startFakePlatform(t, testDomain, "the-right-password", false)
	defer platform.close()

	ua := newTestUA(t, platform.addr, reserveUDPPort(t))
	ua.cfg.Password = "the-wrong-password"
	defer ua.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}
	if err := ua.Register(ctx); !errors.Is(err, ErrForbidden) {
		t.Fatalf("密码错误应得到 ErrForbidden，实际 %v", err)
	}
	if _, authTries, _, _, _ := platform.stats(); authTries != 1 {
		t.Fatalf("平台应收到 1 次带错凭证的 REGISTER，实际 %d", authTries)
	}
}

// TestRegisterWithoutAuthChallenge 平台未开启鉴权时首个 REGISTER 即 200。
func TestRegisterWithoutAuthChallenge(t *testing.T) {
	platform := startFakePlatform(t, testDomain, testPassword, false)
	defer platform.close()

	// 让平台直接放行：改造成「收到 REGISTER 就回 200」
	platform.srv.OnRegister(func(req *sip.Request, tx sip.ServerTransaction) {
		res := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
		exp := sip.ExpiresHeader(1800)
		res.AppendHeader(&exp)
		_ = tx.Respond(res)
	})

	ua := newTestUA(t, platform.addr, reserveUDPPort(t))
	defer ua.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}
	if err := ua.Register(ctx); err != nil {
		t.Fatalf("无挑战注册应成功：%v", err)
	}
	if got := time.Until(ua.ExpiresAt()); got > 1801*time.Second || got < 1700*time.Second {
		t.Fatalf("应采纳平台返回的 Expires=1800，实际剩余 %s", got)
	}
}

// TestKeepaliveReachesPlatformOnSamePort 验证 Keepalive MESSAGE 也能被平台在设备端口上收到。
func TestKeepaliveReachesPlatformOnSamePort(t *testing.T) {
	platform := startFakePlatform(t, testDomain, testPassword, false)
	defer platform.close()

	received := make(chan string, 4)
	sourceCh := make(chan string, 4)
	platform.srv.OnMessage(func(req *sip.Request, tx sip.ServerTransaction) {
		received <- string(req.Body())
		sourceCh <- req.Source()
		_ = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil))
	})

	ua := newTestUA(t, platform.addr, reserveUDPPort(t))
	defer ua.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ua.Start(ctx); err != nil {
		t.Fatalf("启动 UA 失败：%v", err)
	}
	if err := ua.Register(ctx); err != nil {
		t.Fatalf("注册失败：%v", err)
	}

	if _, err := ua.SendMessage(ctx, []byte("<Notify><CmdType>Keepalive</CmdType></Notify>")); err != nil {
		t.Fatalf("发送心跳失败：%v", err)
	}

	select {
	case body := <-received:
		if !strings.Contains(body, "Keepalive") {
			t.Fatalf("平台收到的正文不符：%s", body)
		}
		src := <-sourceCh
		_, port, _ := net.SplitHostPort(src)
		if port != fmt.Sprint(ua.cfg.LocalPort) {
			t.Fatalf("心跳的源端口 %s 应等于设备监听端口 %d", port, ua.cfg.LocalPort)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("平台未收到心跳 MESSAGE")
	}
}

// TestBuildRegisterRequestLayout 单测 REGISTER 报文的关键头部。
func TestBuildRegisterRequestLayout(t *testing.T) {
	ua := newTestUA(t, "127.0.0.1:5060", 5062)
	defer ua.Close()

	req := ua.buildRegister(3600)
	if req.Method != sip.REGISTER {
		t.Fatalf("方法应为 REGISTER：%s", req.Method)
	}
	if got := req.Recipient.Addr(); got != "sip:"+testDomain {
		t.Fatalf("Request-URI 应为 sip:%s，实际 %s", testDomain, got)
	}
	if got := req.Destination(); got != "127.0.0.1:5060" {
		t.Fatalf("目的地应显式为平台地址，实际 %s", got)
	}
	if to := req.To(); to == nil || to.Address.User != testDeviceID {
		t.Fatalf("To 应为设备 AoR：%+v", to)
	}
	if from := req.From(); from == nil || from.Address.User != testDeviceID {
		t.Fatalf("From 应为设备 AoR：%+v", from)
	} else if tag, _ := from.Params.Get("tag"); tag == "" {
		t.Fatal("From 应带 tag 参数")
	}
	contact := req.Contact()
	if contact == nil {
		t.Fatal("缺少 Contact 头")
	}
	if contact.Address.Port != 5062 || contact.Address.Host != "127.0.0.1" {
		t.Fatalf("Contact 应指向本地监听地址：%+v", contact.Address)
	}
	if exp := req.GetHeader("Expires"); exp == nil || exp.Value() != "3600" {
		t.Fatalf("Expires 头不符：%v", exp)
	}
	if got := req.Transport(); got != "UDP" {
		t.Fatalf("传输层应为 UDP，实际 %q", got)
	}

	// 注销：Expires 必须为 0
	bye := ua.buildRegister(0)
	if exp := bye.GetHeader("Expires"); exp == nil || exp.Value() != "0" {
		t.Fatalf("注销的 Expires 应为 0：%v", exp)
	}
}
