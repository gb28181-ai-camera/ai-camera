package sipua

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

// 本文件实现 RFC 2617 的**客户端侧** MD5 摘要计算。
//
// 平台（如 WVP-Pro）的服务端校验逻辑见
// `DigestServerAuthenticationHelper.doAuthenticatePlainTextPassword`：
//
//	HA1 = MD5(username:realm:password)
//	HA2 = MD5(method:uri)
//	KD  = HA1:nonce[:nc:cnonce:qop]:HA2        // qop=auth 时插入中间四段
//	response = MD5(KD)
//
// 注意：SIP 的 Digest 与 HTTP 的差别在于 method 用的是 SIP 方法名（REGISTER/MESSAGE/...），
// uri 用的是 Request-URI 原文。二者必须与真正发出的报文完全一致，否则平台侧算出另一个摘要。

const algorithmMD5 = "MD5"

// DigestChallenge 是从 401/407 响应的 WWW-Authenticate 头解析出的挑战参数。
type DigestChallenge struct {
	Realm     string
	Nonce     string
	Opaque    string
	Algorithm string // 缺省为 MD5
	Qop       string // 原样保留。可能为空，也可能是 "auth" / "auth,auth-int" 列表
}

// QopAuth 判断挑战是否声明支持 qop=auth。
//
// GB28181 平台普遍发 `qop="auth"`；若为空则退化为 RFC 2069 形式（不带 nc/cnonce）。
// 若平台声明了 qop 但值里没有 auth（例如只有 auth-int），返回 false —— 调用方据此决定是否降级。
func (c *DigestChallenge) QopAuth() bool {
	for _, part := range strings.Split(c.Qop, ",") {
		if strings.EqualFold(strings.TrimSpace(part), "auth") {
			return true
		}
	}
	return false
}

// ParseChallenge 解析 `Digest realm="x", nonce="y", ...` 形式的 WWW-Authenticate 值。
func ParseChallenge(v string) (*DigestChallenge, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil, fmt.Errorf("WWW-Authenticate 为空")
	}
	// scheme 之后的整个部分都是参数列表
	if i := strings.IndexByte(s, ' '); i >= 0 {
		scheme := strings.TrimSpace(s[:i])
		if !strings.EqualFold(scheme, "Digest") {
			return nil, fmt.Errorf("不支持的鉴权方案 %q，仅支持 Digest", scheme)
		}
		s = s[i+1:]
	}
	params, err := parseAuthParams(s)
	if err != nil {
		return nil, err
	}
	chal := &DigestChallenge{
		Realm:     params["realm"],
		Nonce:     params["nonce"],
		Opaque:    params["opaque"],
		Algorithm: params["algorithm"],
		Qop:       params["qop"],
	}
	if chal.Nonce == "" {
		return nil, fmt.Errorf("WWW-Authenticate 缺少 nonce：%q", v)
	}
	if chal.Algorithm == "" {
		chal.Algorithm = algorithmMD5
	}
	if !strings.EqualFold(chal.Algorithm, algorithmMD5) {
		return nil, fmt.Errorf("不支持的摘要算法 %q，仅支持 MD5", chal.Algorithm)
	}
	return chal, nil
}

// parseAuthParams 解析 `k="v", k2=v2` 形式的参数列表，键统一转小写。
// 值可带引号也可不带；引号内允许出现逗号（例如 qop="auth,auth-int"）。
func parseAuthParams(s string) (map[string]string, error) {
	out := make(map[string]string)
	for i := 0; i < len(s); {
		// 跳过前导空白与逗号
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		// 键
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ',' {
			i++
		}
		if i >= len(s) || s[i] != '=' {
			return nil, fmt.Errorf("WWW-Authenticate 参数缺少 '='：%q", s[start:])
		}
		key := strings.ToLower(strings.TrimSpace(s[start:i]))
		i++ // 跳过 '='
		var val string
		if i < len(s) && s[i] == '"' {
			i++
			vstart := i
			for i < len(s) && s[i] != '"' {
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("WWW-Authenticate 参数的引号未闭合：%q", s)
			}
			val = s[vstart:i]
			i++ // 跳过收尾引号
		} else {
			vstart := i
			for i < len(s) && s[i] != ',' {
				i++
			}
			val = strings.TrimSpace(s[vstart:i])
		}
		if key != "" {
			out[key] = val
		}
	}
	return out, nil
}

// DigestResponse 按 RFC 2617 §3.2.2 计算 response 摘要。
//
// qop 为空时退化为 RFC 2069 形式：MD5(HA1:nonce:HA2)。
// qop 非空时使用 qop=auth 形式：MD5(HA1:nonce:nc:cnonce:qop:HA2)，其中 nc 为 8 位十六进制。
func DigestResponse(username, realm, password, method, uri, nonce, cnonce, qop string, nc uint32) string {
	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(strings.ToUpper(method) + ":" + uri)
	var kd string
	if qop == "" {
		kd = ha1 + ":" + nonce + ":" + ha2
	} else {
		kd = fmt.Sprintf("%s:%s:%08x:%s:%s:%s", ha1, nonce, nc, cnonce, qop, ha2)
	}
	return md5Hex(kd)
}

// Authorization 描述一次摘要应答所需的全部输入。
type Authorization struct {
	Username string
	Password string
	Realm    string
	Nonce    string
	Qop      string // 空串表示 RFC 2069 形式
	NC       uint32 // nonce 计数器，qop 非空时必填，从 1 开始
	CNonce   string // 客户端随机数，qop 非空时必填
	Opaque   string
	Method   string // SIP 方法名，如 REGISTER
	URI      string // Request-URI 原文
}

// Response 计算摘要值。
func (a *Authorization) Response() string {
	return DigestResponse(a.Username, a.Realm, a.Password, a.Method, a.URI, a.Nonce, a.CNonce, a.Qop, a.NC)
}

// HeaderValue 生成 Authorization 头的值（不含头部名）。
func (a *Authorization) HeaderValue() string {
	var b strings.Builder
	b.WriteString(`Digest username="`)
	b.WriteString(a.Username)
	b.WriteString(`", realm="`)
	b.WriteString(a.Realm)
	b.WriteString(`", nonce="`)
	b.WriteString(a.Nonce)
	b.WriteString(`", uri="`)
	b.WriteString(a.URI)
	b.WriteString(`", response="`)
	b.WriteString(a.Response())
	b.WriteString(`", algorithm=`)
	b.WriteString(algorithmMD5)
	if a.Qop != "" {
		fmt.Fprintf(&b, `, qop=%s, nc=%08x, cnonce="%s"`, a.Qop, a.NC, a.CNonce)
	}
	if a.Opaque != "" {
		b.WriteString(`, opaque="`)
		b.WriteString(a.Opaque)
		b.WriteString(`"`)
	}
	return b.String()
}

// md5Hex 返回字符串的 MD5 十六进制小写表示。
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
