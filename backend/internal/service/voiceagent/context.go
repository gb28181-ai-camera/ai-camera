package voiceagent

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
)

// Context 签名错误。
var (
	ErrContextSecretMissing = errors.New("未配置 PIPECAT_CONTEXT_SECRET")
	ErrContextInvalid       = errors.New("X-Voice-Context 无效")
	ErrContextExpired       = errors.New("X-Voice-Context 已过期")
)

// ContextSigner 负责 X-Voice-Context 的 HMAC-SHA256 签名与校验。
//
// 格式：base64url(JSON) + "." + base64url(HMAC-SHA256)。
// Pipecat 只信任签名上下文，禁止直接采信客户端自报的 user_id/device_id。
type ContextSigner struct {
	secret []byte
	now    func() time.Time
}

// NewContextSigner 构造签名器；secret 为空时 Sign/Verify 均返回错误。
func NewContextSigner(secret string) *ContextSigner {
	return &ContextSigner{secret: []byte(strings.TrimSpace(secret)), now: time.Now}
}

// Sign 生成签名串。
func (s *ContextSigner) Sign(ctx request.ChatContext) (string, error) {
	if len(s.secret) == 0 {
		return "", ErrContextSecretMissing
	}
	now := s.now()
	if ctx.IssuedAt == 0 {
		ctx.IssuedAt = now.Unix()
	}
	if ctx.ExpiresAt == 0 {
		ctx.ExpiresAt = now.Add(ContextTTL).Unix()
	}
	raw, err := json.Marshal(ctx)
	if err != nil {
		return "", err
	}
	body := base64.RawURLEncoding.EncodeToString(raw)
	return body + "." + s.sign(body), nil
}

// Verify 校验签名与有效期。
func (s *ContextSigner) Verify(value string) (*request.ChatContext, error) {
	if len(s.secret) == 0 {
		return nil, ErrContextSecretMissing
	}
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, ErrContextInvalid
	}
	expected := s.sign(parts[0])
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return nil, ErrContextInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrContextInvalid
	}
	var ctx request.ChatContext
	if err := json.Unmarshal(raw, &ctx); err != nil {
		return nil, ErrContextInvalid
	}
	if ctx.ExpiresAt <= 0 || s.now().Unix() > ctx.ExpiresAt {
		return nil, ErrContextExpired
	}
	return &ctx, nil
}

func (s *ContextSigner) sign(body string) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
