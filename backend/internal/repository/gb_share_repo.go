package repository

import (
	"context"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标通道免登录分享数据访问
// ════════════════════════════════════════════════════════════════

// GbShareRepo 分享令牌数据访问。
type GbShareRepo struct{}

// NewGbShareRepo 构造分享令牌仓储。
func NewGbShareRepo() *GbShareRepo { return &GbShareRepo{} }

// CreateToken 新增令牌。
func (r *GbShareRepo) CreateToken(ctx context.Context, t *model.GbShareToken) error {
	return bootstrap.DB().WithContext(ctx).Create(t).Error
}

// GetTokenByID 按主键查询令牌。
func (r *GbShareRepo) GetTokenByID(ctx context.Context, id int64) (*model.GbShareToken, error) {
	var t model.GbShareToken
	if err := bootstrap.DB().WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTokenByHash 按 SHA-256 查询令牌（匿名接口使用）。
func (r *GbShareRepo) GetTokenByHash(ctx context.Context, hash string) (*model.GbShareToken, error) {
	var t model.GbShareToken
	if err := bootstrap.DB().WithContext(ctx).
		Where("token_hash = ?", hash).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ListActiveTokens 列出通道下未过期且未吊销的令牌（最新在前）。
func (r *GbShareRepo) ListActiveTokens(ctx context.Context, channelID int64, now time.Time, ds *utils.DataScope) ([]model.GbShareToken, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbShareToken{}).
		Where("channel_id = ?", channelID).
		Where("expires_dt > ?", now).
		Where("(revoked IS NULL OR revoked = false)")
	if ds != nil {
		q = ds.ApplyToQuery(q, "ac_gb_share_token.dept_id")
	}
	var items []model.GbShareToken
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

// RevokeToken 吊销令牌（幂等：已吊销时保留首次吊销时间）。
func (r *GbShareRepo) RevokeToken(ctx context.Context, id int64, now time.Time) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.GbShareToken{}).
		Where("id = ?", id).
		Where("(revoked IS NULL OR revoked = false)").
		Updates(map[string]interface{}{
			"revoked":    true,
			"revoked_dt": now,
		}).Error
}

// IncrementUse 原子累加使用次数并刷新最近使用时间。
func (r *GbShareRepo) IncrementUse(ctx context.Context, id int64, now time.Time) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.GbShareToken{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"use_count":    gorm.Expr("use_count + 1"),
			"last_used_dt": now,
		}).Error
}

// GetChannelByID 查询通道（带设备，用于展示通道/设备名称）。
func (r *GbShareRepo) GetChannelByID(ctx context.Context, id int64) (*model.GbChannel, error) {
	var ch model.GbChannel
	if err := bootstrap.DB().WithContext(ctx).
		Preload("Device").
		First(&ch, id).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}
