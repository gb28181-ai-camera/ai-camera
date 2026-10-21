package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/emiago/sipgo/sip"
	"gorm.io/gorm"
)

const (
	testGBID     = "34020000001320000001"
	testRealm    = "3402000000"
	testPassword = "12345678"
	testURI      = "sip:3402000000"
)

type fakeLookup struct {
	dev *model.Device
	err error
}

func (f fakeLookup) FindByGbDeviceID(_ context.Context, id string) (*model.Device, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.dev == nil || f.dev.GbDeviceID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return f.dev, nil
}

func testDevice() *model.Device {
	return &model.Device{
		BaseModel:   model.BaseModel{ID: 1},
		GbDeviceID:  testGBID,
		SipPassword: testPassword,
	}
}

func newAuthenticator(lookup DeviceLookup) *Authenticator {
	a := New(lookup, testRealm, "unit-test-secret")
	a.SetNow(func() time.Time { return time.Unix(1_700_000_000, 0) })
	return a
}

// RFC 2617 §3.5 的官方示例向量。它验证的是算法本身，与 SIP 无关：
// 同一个 MD5 算式在 HTTP/SIP 里通用，method 只是参与 HA2。
func TestDigestResponseRFC2617Vector(t *testing.T) {
	got := DigestResponse(
		"Mufasa", "testrealm@host.com", "Circle Of Life", "GET", "/dir/index.html",
		"dcd98b7102dd2f0e8b11d0f600bfb0c093", "0a4f113b", "auth", "00000001",
	)
	const want = "6629fae49393a05397450978507c4ef1"
	if got != want {
		t.Fatalf("RFC 2617 向量不匹配：got=%s want=%s", got, want)
	}
}

func TestChallengeDeterministicAndRotates(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	first := a.Challenge(testGBID, testRealm)
	second := a.Challenge(testGBID, testRealm)
	if first != second {
		t.Fatalf("同一窗口内 nonce 必须确定：\n%s\n%s", first, second)
	}
	if !strings.Contains(first, `realm="`+testRealm+`"`) || !strings.Contains(first, "qop=\"auth\"") {
		t.Fatalf("挑战头内容不完整：%s", first)
	}

	a.SetNow(func() time.Time { return time.Unix(1_700_000_000+7200, 0) })
	third := a.Challenge(testGBID, testRealm)
	if third == first {
		t.Fatalf("跨窗口后 nonce 应变化")
	}
}

func TestLookupWhitelist(t *testing.T) {
	a := newAuthenticator(fakeLookup{dev: testDevice()})
	ctx := context.Background()

	dev, err := a.Lookup(ctx, testGBID)
	if err != nil || dev == nil || dev.GbDeviceID != testGBID {
		t.Fatalf("已建档设备应命中：dev=%v err=%v", dev, err)
	}

	if _, err := a.Lookup(ctx, "34020000001320000002"); !errors.Is(err, ErrUnknownDevice) {
		t.Fatalf("未建档设备应返回 ErrUnknownDevice，实际 %v", err)
	}
	if _, err := a.Lookup(ctx, ""); !errors.Is(err, ErrUnknownDevice) {
		t.Fatalf("空编码应返回 ErrUnknownDevice，实际 %v", err)
	}
}

func TestNeedAuth(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	if a.NeedAuth(nil) {
		t.Fatalf("nil 设备不应需要鉴权")
	}
	if a.NeedAuth(&model.Device{GbDeviceID: testGBID}) {
		t.Fatalf("sip_password 为空时不应要求鉴权")
	}
	if !a.NeedAuth(testDevice()) {
		t.Fatalf("sip_password 非空时必须要求鉴权")
	}
}

// 正确凭证放行。
func TestVerifyCorrect(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	dev := testDevice()
	nonce := nonceOf(t, a.Challenge(testGBID, testRealm))

	req := buildRequest(sip.REGISTER)
	req.AppendHeader(sip.NewHeader("Authorization",
		buildAuthHeader(t, dev, "REGISTER", testRealm, nonce, testPassword, "auth", "00000001", "abcdef01", testURI)))

	if err := a.Verify(req, dev); err != nil {
		t.Fatalf("正确凭证应通过：%v", err)
	}
}

// 无 qop（RFC 2069）也应支持：部分老设备不带 qop。
func TestVerifyWithoutQop(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	dev := testDevice()
	nonce := nonceOf(t, a.Challenge(testGBID, testRealm))

	req := buildRequest(sip.MESSAGE)
	req.AppendHeader(sip.NewHeader("Authorization",
		buildAuthHeader(t, dev, "MESSAGE", testRealm, nonce, testPassword, "", "", "", testURI)))

	if err := a.Verify(req, dev); err != nil {
		t.Fatalf("无 qop 的正确凭证应通过：%v", err)
	}
}

// nc 大小写都必须接受：摘要按客户端原样回传的字符串计算。
func TestVerifyUppercaseNC(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	dev := testDevice()
	nonce := nonceOf(t, a.Challenge(testGBID, testRealm))

	req := buildRequest(sip.REGISTER)
	req.AppendHeader(sip.NewHeader("Authorization",
		buildAuthHeader(t, dev, "REGISTER", testRealm, nonce, testPassword, "auth", "0000000A", "cafebabe", testURI)))

	if err := a.Verify(req, dev); err != nil {
		t.Fatalf("大写 nc 应通过：%v", err)
	}
}

// 错密码 → 失败。
func TestVerifyWrongPassword(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	dev := testDevice()
	nonce := nonceOf(t, a.Challenge(testGBID, testRealm))

	req := buildRequest(sip.REGISTER)
	req.AppendHeader(sip.NewHeader("Authorization",
		buildAuthHeader(t, dev, "REGISTER", testRealm, nonce, "wrong-password", "auth", "00000001", "abcdef01", testURI)))

	err := a.Verify(req, dev)
	if err == nil {
		t.Fatalf("错误密码必须被拒绝")
	}
	if !strings.Contains(err.Error(), "摘要校验失败") {
		t.Fatalf("错误信息应说明摘要失败：%v", err)
	}
}

// 无凭证 → 失败（handler 据此回 401）。
func TestVerifyNoAuthorization(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	req := buildRequest(sip.REGISTER)
	if err := a.Verify(req, testDevice()); err == nil {
		t.Fatalf("缺少 Authorization 时必须失败")
	}
}

// 过期 nonce（两个窗口以前）→ 失败。
func TestVerifyExpiredNonce(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	dev := testDevice()
	nonce := nonceOf(t, a.Challenge(testGBID, testRealm))

	// 前进 3 个 nonce 窗口（TTL=1h，接受当前与上一窗口）
	a.SetNow(func() time.Time { return time.Unix(1_700_000_000+3*3600, 0) })

	req := buildRequest(sip.REGISTER)
	req.AppendHeader(sip.NewHeader("Authorization",
		buildAuthHeader(t, dev, "REGISTER", testRealm, nonce, testPassword, "auth", "00000001", "abcdef01", testURI)))

	if err := a.Verify(req, dev); err == nil || !strings.Contains(err.Error(), "nonce") {
		t.Fatalf("过期 nonce 必须被拒绝，实际：%v", err)
	}
}

// 用户名冒用：凭证里的 username 与 From 不一致时必须拒绝。
func TestVerifyUsernameMismatch(t *testing.T) {
	a := newAuthenticator(fakeLookup{})
	dev := testDevice()
	nonce := nonceOf(t, a.Challenge(testGBID, testRealm))

	other := *dev
	other.GbDeviceID = "34020000001320000002"
	req := buildRequest(sip.REGISTER)
	req.AppendHeader(sip.NewHeader("Authorization",
		buildAuthHeader(t, &other, "REGISTER", testRealm, nonce, testPassword, "auth", "00000001", "abcdef01", testURI)))

	if err := a.Verify(req, dev); err == nil {
		t.Fatalf("用户名与设备编码不一致时必须拒绝")
	}
}

// ────────────────────────────────────────────────────────────────
// 测试辅助
// ────────────────────────────────────────────────────────────────

func buildRequest(method sip.RequestMethod) *sip.Request {
	recipient := sip.Uri{Scheme: "sip", Host: testRealm}
	req := sip.NewRequest(method, recipient)
	from := &sip.FromHeader{Address: sip.Uri{Scheme: "sip", User: testGBID, Host: testRealm}}
	params := sip.NewParams()
	from.Params = params.Add("tag", "testtag")
	req.AppendHeader(from)
	req.AppendHeader(&sip.ToHeader{Address: sip.Uri{Scheme: "sip", User: testGBID, Host: testRealm}})
	return req
}

func nonceOf(t *testing.T, challenge string) string {
	t.Helper()
	i := strings.Index(challenge, `nonce="`)
	if i < 0 {
		t.Fatalf("挑战头里没有 nonce：%s", challenge)
	}
	rest := challenge[i+len(`nonce="`):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		t.Fatalf("nonce 引号未闭合：%s", challenge)
	}
	return rest[:j]
}

func buildAuthHeader(t *testing.T, dev *model.Device, method, realm, nonce, password, qop, nc, cnonce, uri string) string {
	t.Helper()
	resp := DigestResponse(dev.GbDeviceID, realm, password, method, uri, nonce, cnonce, qop, nc)
	var b strings.Builder
	fmt.Fprintf(&b, `Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm=MD5`,
		dev.GbDeviceID, realm, nonce, uri, resp)
	if qop != "" {
		fmt.Fprintf(&b, `, qop=%s, nc=%s, cnonce="%s"`, qop, nc, cnonce)
	}
	return b.String()
}
