// Package turn 负责签发 coturn 的临时凭据（TURN REST API 方案）。
//
// 为什么不是「coturn 回调后端」：
// coturn **没有** HTTP 鉴权回调能力。它支持的外部授权方式只有 TURN REST API：
// 后端先做自己的授权判断，再用共享密钥算出 `timestamp:userid` + HMAC-SHA1
// 密码交给客户端；coturn 只负责用同一密钥验证签名与过期时间。整个过程中
// coturn 不发起任何网络回调，因此「自定义授权」的落点必须在**发凭据的后端**。
//
// 关键细节（踩过坑的）：
//   - 时间戳是**过期时间**，不是签发时间；
//   - 算法必须是 HMAC-**SHA1**，SHA256 不行；
//   - Base64 必须是**标准编码**（带 = 填充），URLEncoding 不行。
package turn

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// 环境变量名。
const (
	EnvSecret   = "TURN_STATIC_AUTH_SECRET"
	EnvRealm    = "TURN_REALM"
	EnvURLs     = "TURN_URLS"
	EnvTTL      = "TURN_TTL_SECONDS"
	EnvRPCToken = "TURN_RPC_TOKEN"
)

const (
	defaultRealm = "ai-camera"
	// 默认值刻意用回环地址：客户端（手机/浏览器）不在 compose 网络里，
	// 写服务名 "turn" 会解析失败。生产必须通过 TURN_URLS 配成可达地址。
	defaultURLs = "stun:127.0.0.1:3478,turn:127.0.0.1:3478?transport=udp,turn:127.0.0.1:3478?transport=tcp"
	defaultTTL  = 3600
	minTTL      = 60
	maxTTL      = 24 * 3600
)

// ErrSecretNotConfigured 表示后端没有配置 TURN 共享密钥。
var ErrSecretNotConfigured = errors.New("未配置 TURN_STATIC_AUTH_SECRET，无法签发 TURN 凭据")

// ErrSubjectInvalid 表示调用方传入的 subject 非法。
var ErrSubjectInvalid = errors.New("subject 不能为空，且不能包含冒号或逗号")

// Credential 是签发给客户端的短期凭据。
type Credential struct {
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	TTL       int      `json:"ttl"`
	ExpiresAt int64    `json:"expires_at"`
	Realm     string   `json:"realm"`
	URLs      []string `json:"urls"`
}

// Service 签发 TURN 凭据。
type Service struct{}

// New 构造 Service。
func New() *Service { return &Service{} }

// Configured 表示 TURN 是否已配置（未配置时上层应返回明确错误，而不是空凭据）。
func (s *Service) Configured() bool {
	return strings.TrimSpace(os.Getenv(EnvSecret)) != ""
}

// Issue 为 subject 签发一份 TTL 内的 TURN 凭据。
//
// subject 用调用方的稳定标识（用户 ID、设备编码、服务组编码），
// 它会出现在 coturn 的日志与统计里，便于按主体排查流量。
func (s *Service) Issue(subject string, ttlSeconds int) (*Credential, error) {
	secret := strings.TrimSpace(os.Getenv(EnvSecret))
	if secret == "" {
		return nil, ErrSecretNotConfigured
	}
	subject = strings.TrimSpace(subject)
	if subject == "" || strings.ContainsAny(subject, ":,") {
		return nil, ErrSubjectInvalid
	}

	ttl := ttlSeconds
	if ttl <= 0 {
		ttl = envInt(EnvTTL, defaultTTL)
	}
	if ttl < minTTL {
		ttl = minTTL
	}
	if ttl > maxTTL {
		ttl = maxTTL
	}

	expires := time.Now().Add(time.Duration(ttl) * time.Second).Unix()
	// coturn 的 usercombo 格式固定为 "<过期时间戳>:<用户标识>"。
	usercombo := fmt.Sprintf("%d:%s", expires, subject)
	password := sign(secret, usercombo)

	return &Credential{
		Username:  usercombo,
		Password:  password,
		TTL:       ttl,
		ExpiresAt: expires,
		Realm:     envString(EnvRealm, defaultRealm),
		URLs:      splitURLs(envString(EnvURLs, defaultURLs)),
	}, nil
}

// sign 计算 coturn 要求的 HMAC-SHA1 + 标准 Base64。
//
// 单独抽出来是为了能用固定向量单测 —— 这个算法一旦写错，
// 表现为「客户端一直 401」，而 coturn 日志只会说 Unauthorized。
func sign(secret, usercombo string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(usercombo))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func envString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func splitURLs(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
