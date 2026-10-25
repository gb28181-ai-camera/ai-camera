package gb28181

// 通道免登录分享。
//
// 安全模型：
//   - 令牌明文只在创建时返回一次，库中仅存 SHA-256，泄库不等于可直接播放；
//   - 匿名接口**不复用**用户 JWT，也不经过 Auth/Casbin 中间件，只按 hash 校验；
//   - 默认 1 小时、最长 24 小时；支持显式吊销；
//   - 协议白名单 webrtc/flv/hls，并对「令牌 + IP」与「令牌」双层限频，
//     避免分享链接被当成开放代理刷流量。

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

const (
	shareTokenPrefix    = "gbs_"
	shareDefaultTTLSec  = 3600
	shareMinTTLSec      = 60
	shareMaxTTLSec      = 86400 // 24h
	shareTokenRandomLen = 32    // 256 bit
)

// 分享相关业务错误。
var (
	ErrGbShareChannelNotFound = errors.New("国标通道不存在")
	ErrGbShareTokenInvalid    = errors.New("分享令牌无效或已过期")
	ErrGbShareTokenExpired    = errors.New("分享令牌已过期")
	ErrGbShareTokenRevoked    = errors.New("分享令牌已被吊销")
	ErrGbShareProtocolInvalid = errors.New("播放协议不支持，仅支持 webrtc/flv/hls")
	ErrGbShareRateLimited     = errors.New("分享播放过于频繁，请稍后再试")
)

// shareRateLimiter 进程内滑动窗口限频器。
//
// 单副本部署下足够；多副本时每个副本独立计数，实际额度会放大到
// 副本数倍，但仍能挡住单点批量刷流。
type shareRateLimiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newShareRateLimiter() *shareRateLimiter {
	return &shareRateLimiter{hits: make(map[string][]time.Time)}
}

// allow 判断是否允许一次请求，并记录本次命中。
func (l *shareRateLimiter) allow(key string, limit int, window time.Duration, now time.Time) bool {
	if limit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-window)
	hits := l.hits[key]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.hits[key] = kept
		return false
	}
	kept = append(kept, now)
	l.hits[key] = kept

	// 轻量清理，避免 key 无限增长（每 200 次写入扫一遍）。
	if len(l.hits) > 200 && len(l.hits)%200 == 0 {
		for k, v := range l.hits {
			if len(v) == 0 || v[len(v)-1].Before(cutoff) {
				delete(l.hits, k)
			}
		}
	}
	return true
}

// GbShareService 分享令牌服务。
type GbShareService struct {
	repo    *repository.GbShareRepo
	limiter *shareRateLimiter
}

// NewGbShareService 构造分享服务。
func NewGbShareService() *GbShareService {
	return &GbShareService{repo: repository.NewGbShareRepo(), limiter: newShareRateLimiter()}
}

// Create 生成分享令牌，返回唯一一次明文。
func (s *GbShareService) Create(ctx context.Context, channelID int64, req *request.GbShareTokenCreateReq) (*response.GbShareTokenCreatedResp, error) {
	if req == nil {
		req = &request.GbShareTokenCreateReq{}
	}
	if channelID <= 0 {
		return nil, ErrGbShareChannelNotFound
	}
	if _, err := s.repo.GetChannelByID(ctx, channelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbShareChannelNotFound
		}
		return nil, err
	}

	ttl := req.TTLSeconds
	if ttl <= 0 {
		ttl = shareDefaultTTLSec
	}
	if ttl < shareMinTTLSec {
		return nil, fmt.Errorf("ttl_seconds 不能小于 %d 秒", shareMinTTLSec)
	}
	if ttl > shareMaxTTLSec {
		ttl = shareMaxTTLSec
	}

	plain, err := generateShareToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	m := &model.GbShareToken{
		ChannelID:   channelID,
		TokenHash:   hashShareToken(plain),
		TokenPrefix: plain[:12],
		ExpiresDT:   now.Add(time.Duration(ttl) * time.Second),
		Revoked:     boolOr(nil, false),
		Remark:      trim(req.Remark),
	}
	if err := s.repo.CreateToken(ctx, m); err != nil {
		return nil, err
	}
	return &response.GbShareTokenCreatedResp{
		GbShareTokenItem: toGbShareTokenItem(*m),
		Token:            plain,
	}, nil
}

// List 列出通道下未过期且未吊销的令牌。
func (s *GbShareService) List(ctx context.Context, channelID int64, ds *utils.DataScope) ([]response.GbShareTokenItem, error) {
	if _, err := s.repo.GetChannelByID(ctx, channelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbShareChannelNotFound
		}
		return nil, err
	}
	rows, err := s.repo.ListActiveTokens(ctx, channelID, time.Now(), ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.GbShareTokenItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, toGbShareTokenItem(m))
	}
	return items, nil
}

// Revoke 吊销令牌。已吊销时幂等返回成功。
func (s *GbShareService) Revoke(ctx context.Context, channelID, tokenID int64) error {
	m, err := s.repo.GetTokenByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbShareTokenInvalid
		}
		return err
	}
	if m.ChannelID != channelID {
		return ErrGbShareTokenInvalid
	}
	return s.repo.RevokeToken(ctx, tokenID, time.Now())
}

// Info 匿名访问：返回通道与可用协议（不消耗使用次数）。
func (s *GbShareService) Info(ctx context.Context, token string) (*response.GbShareInfoResp, error) {
	rec, ch, err := s.validate(ctx, token)
	if err != nil {
		return nil, err
	}
	deviceName := ""
	if ch.Device != nil {
		deviceName = ch.Device.Name
	}
	return &response.GbShareInfoResp{
		ChannelID:    ch.ID,
		ChannelName:  ch.Name,
		GbChannelID:  ch.GbChannelID,
		DeviceName:   deviceName,
		ExpiresDT:    rec.ExpiresDT,
		Protocols:    []string{"webrtc", "flv", "hls"},
		PlayEndpoint: "/public/gb/shares/" + token + "/play",
	}, nil
}

// Play 匿名点播：校验令牌、限频后发起国标点播。
func (s *GbShareService) Play(ctx context.Context, token, clientIP, protocol string) (*response.GbSharePlayResp, error) {
	rec, ch, err := s.validate(ctx, token)
	if err != nil {
		return nil, err
	}

	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		protocol = "webrtc"
	}
	switch protocol {
	case "webrtc", "flv", "hls":
	default:
		return nil, ErrGbShareProtocolInvalid
	}

	now := time.Now()
	ip := strings.TrimSpace(clientIP)
	if ip == "" {
		ip = "unknown"
	}
	if !s.limiter.allow(fmt.Sprintf("share:%d:ip:%s", rec.ID, ip), 10, time.Minute, now) ||
		!s.limiter.allow(fmt.Sprintf("share:%d", rec.ID), 60, time.Hour, now) {
		return nil, ErrGbShareRateLimited
	}

	play, err := NewGbPlayService().Play(ctx, ch.ID, &request.GbPlayReq{Protocol: protocol})
	if err != nil {
		return nil, err
	}
	_ = s.repo.IncrementUse(ctx, rec.ID, now)

	expires := int64(time.Until(rec.ExpiresDT).Seconds())
	if expires < 0 {
		expires = 0
	}
	return &response.GbSharePlayResp{
		ChannelID:    ch.ID,
		ChannelName:  ch.Name,
		App:          play.App,
		Stream:       play.Stream,
		Protocol:     protocol,
		WebRTC:       play.WebRTC,
		FLV:          play.FLV,
		HLS:          play.HLS,
		ExpiresInSec: expires,
	}, nil
}

// validate 校验令牌并加载通道。
func (s *GbShareService) validate(ctx context.Context, token string) (*model.GbShareToken, *model.GbChannel, error) {
	token = strings.TrimSpace(token)
	if len(token) < 16 || len(token) > 128 {
		return nil, nil, ErrGbShareTokenInvalid
	}
	rec, err := s.repo.GetTokenByHash(ctx, hashShareToken(token))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrGbShareTokenInvalid
		}
		return nil, nil, err
	}
	if boolValue(rec.Revoked) {
		return nil, nil, ErrGbShareTokenRevoked
	}
	if !rec.ExpiresDT.IsZero() && time.Now().After(rec.ExpiresDT) {
		return nil, nil, ErrGbShareTokenExpired
	}
	ch, err := s.repo.GetChannelByID(ctx, rec.ChannelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrGbShareChannelNotFound
		}
		return nil, nil, err
	}
	return rec, ch, nil
}

// generateShareToken 生成高熵令牌：gbs_ + base64url(32 字节随机)。
func generateShareToken() (string, error) {
	buf := make([]byte, shareTokenRandomLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成分享令牌失败: %w", err)
	}
	return shareTokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashShareToken 计算 SHA-256 hex。
func hashShareToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// toGbShareTokenItem 模型转 DTO（不含 hash）。
func toGbShareTokenItem(m model.GbShareToken) response.GbShareTokenItem {
	return response.GbShareTokenItem{
		ID:          m.ID,
		ChannelID:   m.ChannelID,
		TokenPrefix: m.TokenPrefix,
		ExpiresDT:   m.ExpiresDT,
		Revoked:     boolValue(m.Revoked),
		UseCount:    m.UseCount,
		LastUsedDT:  m.LastUsedDT,
		Remark:      m.Remark,
		CreatedBy:   m.CreatedBy,
		CreatedDT:   m.CreatedDT,
	}
}
