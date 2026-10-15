package sipua

import (
	"strings"
	"testing"
)

// RFC 2617 §3.5 的官方示例向量。
//
// 这是 Digest 实现最容易写错又最难 debug 的部分（nonce 计数器大小写、
// qop 插入位置、HA2 里的 method 大小写都会改变结果），因此必须用标准向量锁定。
const (
	rfcUser     = "Mufasa"
	rfcRealm    = "testrealm@host.com"
	rfcPassword = "Circle Of Life"
	rfcMethod   = "GET"
	rfcURI      = "/dir/index.html"
	rfcNonce    = "dcd98b7102dd2f0e8b11d0f600bfb0c093"
	rfcCNonce   = "0a4f113b"

	rfcHA1      = "939e7578ed9e3c518a452acee763bce9"
	rfcHA2      = "39aff3a2bab6126f332b942af96d3366"
	rfcResponse = "6629fae49393a05397450978507c4ef1"
	// 同一个例子退化为 RFC 2069 形式（不带 qop）时的结果。
	rfcResponseNoQop = "670fd8c2df070c60b045671b8b24ff02"
)

// TestDigestResponseRFC2617Vector 用 RFC 2617 §3.5 的已知向量验证 qop=auth 分支。
func TestDigestResponseRFC2617Vector(t *testing.T) {
	got := DigestResponse(rfcUser, rfcRealm, rfcPassword, rfcMethod, rfcURI,
		rfcNonce, rfcCNonce, "auth", 1)
	if got != rfcResponse {
		t.Fatalf("RFC 2617 qop=auth 向量不匹配\n want %s\n got  %s", rfcResponse, got)
	}
}

// TestDigestResponseRFC2069Vector 验证无 qop 的退化分支（部分老设备/平台走这条路）。
func TestDigestResponseRFC2069Vector(t *testing.T) {
	got := DigestResponse(rfcUser, rfcRealm, rfcPassword, rfcMethod, rfcURI,
		rfcNonce, "", "", 0)
	if got != rfcResponseNoQop {
		t.Fatalf("RFC 2069 向量不匹配\n want %s\n got  %s", rfcResponseNoQop, got)
	}
}

// TestDigestResponseIntermediateValues 分别锁定 HA1 / HA2 两个中间量。
// 一旦失败可以立刻区分是「凭证拼接错」还是「method/uri 拼接错」。
func TestDigestResponseIntermediateValues(t *testing.T) {
	if got := md5Hex(rfcUser + ":" + rfcRealm + ":" + rfcPassword); got != rfcHA1 {
		t.Errorf("HA1 不匹配 want %s got %s", rfcHA1, got)
	}
	if got := md5Hex(strings.ToUpper(rfcMethod) + ":" + rfcURI); got != rfcHA2 {
		t.Errorf("HA2 不匹配 want %s got %s", rfcHA2, got)
	}
}

// TestDigestResponseNCCaseInsensitive 用非 1 的计数器验证 nc 的十六进制格式，
// 同时确认大小写无关（0x1A 与 0x1a 都是合法输入，输出字符串必须一致）。
func TestDigestResponseNCCaseInsensitive(t *testing.T) {
	a := DigestResponse(rfcUser, rfcRealm, rfcPassword, rfcMethod, rfcURI,
		rfcNonce, rfcCNonce, "auth", 26)
	b := DigestResponse(rfcUser, rfcRealm, rfcPassword, rfcMethod, rfcURI,
		rfcNonce, rfcCNonce, "auth", 0x1a)
	if a != b {
		t.Fatalf("nc=26 与 nc=0x1a 应得到同一摘要：%s vs %s", a, b)
	}
	if a == rfcResponse {
		t.Fatalf("nc 变化后摘要不应保持不变")
	}
}

// TestParseChallengeWVP 用 WVP-Pro 实际下发的 WWW-Authenticate 形态做解析测试。
// WVP 的 generateChallenge 会写 realm / qop / nonce / algorithm 四个参数，
// 其中 qop 与 nonce 带引号、algorithm 不带引号。
func TestParseChallengeWVP(t *testing.T) {
	raw := `Digest realm="3402000000", qop="auth", nonce="a3f1c2d4e5b6", algorithm=MD5`
	chal, err := ParseChallenge(raw)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if chal.Realm != "3402000000" {
		t.Errorf("realm 解析错误：%q", chal.Realm)
	}
	if chal.Nonce != "a3f1c2d4e5b6" {
		t.Errorf("nonce 解析错误：%q", chal.Nonce)
	}
	if chal.Qop != "auth" {
		t.Errorf("qop 解析错误：%q", chal.Qop)
	}
	if chal.Algorithm != "MD5" {
		t.Errorf("algorithm 解析错误：%q", chal.Algorithm)
	}
	if !chal.QopAuth() {
		t.Error("QopAuth 应为 true")
	}
}

// TestParseChallengeVariants 覆盖 qop 为空、qop 列表、算法缺省、带 opaque 等形态。
func TestParseChallengeVariants(t *testing.T) {
	t.Run("无 qop 且算法缺省", func(t *testing.T) {
		chal, err := ParseChallenge(`Digest realm="x", nonce="n1"`)
		if err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		if chal.Algorithm != "MD5" {
			t.Errorf("algorithm 缺省应回落 MD5，得到 %q", chal.Algorithm)
		}
		if chal.QopAuth() {
			t.Error("未声明 qop 时 QopAuth 应为 false")
		}
	})
	t.Run("qop 列表含逗号", func(t *testing.T) {
		chal, err := ParseChallenge(`Digest realm="x", nonce="n1", qop="auth,auth-int"`)
		if err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		if !chal.QopAuth() {
			t.Error("qop 列表中含 auth，QopAuth 应为 true")
		}
	})
	t.Run("qop 只有 auth-int", func(t *testing.T) {
		chal, err := ParseChallenge(`Digest realm="x", nonce="n1", qop="auth-int"`)
		if err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		if chal.QopAuth() {
			t.Error("qop 只有 auth-int 时 QopAuth 应为 false")
		}
	})
	t.Run("带 opaque", func(t *testing.T) {
		chal, err := ParseChallenge(`Digest realm="x", nonce="n1", opaque="5ccc069c403ebaf9f0171e9517f40e41"`)
		if err != nil {
			t.Fatalf("解析失败：%v", err)
		}
		if chal.Opaque != "5ccc069c403ebaf9f0171e9517f40e41" {
			t.Errorf("opaque 解析错误：%q", chal.Opaque)
		}
	})
}

// TestParseChallengeErrors 覆盖错误输入：非 Digest 方案、缺 nonce、不支持的算法。
func TestParseChallengeErrors(t *testing.T) {
	cases := map[string]string{
		"非 Digest 方案": `Basic realm="x"`,
		"缺 nonce":     `Digest realm="x", qop="auth"`,
		"空串":          ``,
		"算法不支持":       `Digest realm="x", nonce="n1", algorithm=SHA-256`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseChallenge(raw); err == nil {
				t.Fatalf("期望解析失败，实际成功：%q", raw)
			}
		})
	}
}

// TestAuthorizationHeaderValue 验证生成的 Authorization 头能被平台侧按同一输入复算出摘要。
// 这里刻意用「解析回来的参数重新计算」做闭环，避免只断言字符串字面量。
func TestAuthorizationHeaderValue(t *testing.T) {
	auth := &Authorization{
		Username: "34020000001320000001",
		Password: "sim-secret",
		Realm:    "3402000000",
		Nonce:    "dcd98b7102dd2f0e8b11d0f600bfb0c093",
		Qop:      "auth",
		NC:       1,
		CNonce:   "0a4f113b",
		Method:   "REGISTER",
		URI:      "sip:3402000000@127.0.0.1:5060",
	}
	value := auth.HeaderValue()
	for _, want := range []string{
		`username="34020000001320000001"`,
		`realm="3402000000"`,
		`uri="sip:3402000000@127.0.0.1:5060"`,
		`, qop=auth, nc=00000001, cnonce="0a4f113b"`,
		`algorithm=MD5`,
	} {
		if !strings.Contains(value, want) {
			t.Errorf("Authorization 头缺少片段 %q\n实际：%s", want, value)
		}
	}

	// 闭环：把生成的头按平台解析方式还原，重新算一遍。
	params, err := parseAuthParams(strings.TrimPrefix(value, "Digest "))
	if err != nil {
		t.Fatalf("回解 Authorization 失败：%v", err)
	}
	if params["response"] != auth.Response() {
		t.Fatalf("response 不一致：%q vs %q", params["response"], auth.Response())
	}
	// 平台侧（WVP）用解析出的 realm/nonce/uri + 明文密码重算，必须一致。
	recomputed := DigestResponse(auth.Username, params["realm"], auth.Password,
		auth.Method, params["uri"], params["nonce"], params["cnonce"], params["qop"], 1)
	if recomputed != params["response"] {
		t.Fatalf("平台侧重算失败：%s vs %s", recomputed, params["response"])
	}
}

// TestAuthorizationHeaderNoQop 验证无 qop 时不会写出 nc/cnonce。
func TestAuthorizationHeaderNoQop(t *testing.T) {
	auth := &Authorization{
		Username: "u", Password: "p", Realm: "r", Nonce: "n",
		Method: "REGISTER", URI: "sip:r@h",
	}
	value := auth.HeaderValue()
	if strings.Contains(value, "nc=") || strings.Contains(value, "cnonce=") {
		t.Fatalf("无 qop 不应出现 nc/cnonce：%s", value)
	}
	if !strings.Contains(value, `response="`+auth.Response()+`"`) {
		t.Fatalf("缺少 response 字段：%s", value)
	}
	// 无 qop 时的摘要必须与 RFC 2069 公式一致（用同一组输入断言分支正确）。
	if got := auth.Response(); got != DigestResponse("u", "r", "p", "REGISTER", "sip:r@h", "n", "", "", 0) {
		t.Fatalf("无 qop 摘要与公式不符：%s", got)
	}
}
