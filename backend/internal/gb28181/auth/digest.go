package auth

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
)

// 本文件实现 RFC 2617 的服务端 MD5 摘要计算与校验。
//
// 与客户端（tools/gbsim/sipua/digest.go）的算式完全对称：
//
//	HA1 = MD5(username:realm:password)
//	HA2 = MD5(method:uri)
//	qop=""    → response = MD5(HA1:nonce:HA2)                     （RFC 2069）
//	qop=auth  → response = MD5(HA1:nonce:nc:cnonce:qop:HA2)
//
// 服务端必须用**客户端原样回传**的 nc 字符串参与计算：RFC 只要求它是 8 位
// 十六进制，大小写都是合法输入；若我们统一格式化成小写，而设备用了大写，
// 摘要就会对不上，表现为「密码明明正确却一直 401」。

// DigestResponse 按 RFC 2617 §3.2.2 计算摘要。
//
// nc 原样参与计算（qop 为空时忽略），因此调用方不要自行格式化。
func DigestResponse(username, realm, password, method, uri, nonce, cnonce, qop, nc string) string {
	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(strings.ToUpper(method) + ":" + uri)
	var kd string
	if qop == "" {
		kd = ha1 + ":" + nonce + ":" + ha2
	} else {
		kd = ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2
	}
	return md5Hex(kd)
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// AuthParams 是从 Authorization 头解析出的参数集合（键统一小写）。
type AuthParams map[string]string

// ParseAuthorization 解析 `Digest username="x", realm="y", ...` 形式的头值。
func ParseAuthorization(value string) (AuthParams, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, fmt.Errorf("Authorization 头为空")
	}
	if i := strings.IndexByte(s, ' '); i >= 0 {
		scheme := strings.TrimSpace(s[:i])
		if !strings.EqualFold(scheme, "Digest") {
			return nil, fmt.Errorf("不支持的鉴权方案 %q，仅支持 Digest", scheme)
		}
		s = s[i+1:]
	}
	params, err := parseParams(s)
	if err != nil {
		return nil, err
	}
	if params["response"] == "" {
		return nil, fmt.Errorf("Authorization 缺少 response")
	}
	return params, nil
}

// parseParams 解析 `k="v", k2=v2` 形式的参数列表。
// 值可带引号也可不带；引号内允许出现逗号（例如 qop="auth,auth-int"）。
func parseParams(s string) (AuthParams, error) {
	out := make(AuthParams)
	for i := 0; i < len(s); {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == ',') {
			i++
		}
		if i >= len(s) {
			break
		}
		start := i
		for i < len(s) && s[i] != '=' && s[i] != ',' {
			i++
		}
		if i >= len(s) || s[i] != '=' {
			return nil, fmt.Errorf("Authorization 参数缺少 '='：%q", s[start:])
		}
		key := strings.ToLower(strings.TrimSpace(s[start:i]))
		i++
		var val string
		if i < len(s) && s[i] == '"' {
			i++
			vstart := i
			for i < len(s) && s[i] != '"' {
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("Authorization 参数的引号未闭合：%q", s)
			}
			val = s[vstart:i]
			i++
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

// QopAuth 判断 qop 是否可用于本服务端（只支持 auth，不支持 auth-int）。
func QopAuth(qop string) bool {
	for _, part := range strings.Split(qop, ",") {
		if strings.EqualFold(strings.TrimSpace(part), "auth") {
			return true
		}
	}
	return false
}

// secureEqual 做常量时间比较，避免通过响应时间侧信道爆破摘要。
func secureEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.ToLower(a)), []byte(strings.ToLower(b))) == 1
}
