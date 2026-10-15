package repository

import (
	"context"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ApiKeyRepo 开放接口密钥数据访问层。表 ac_api_key。
type ApiKeyRepo struct{}

// NewApiKeyRepo 构造 API Key 仓储。
func NewApiKeyRepo() *ApiKeyRepo { return &ApiKeyRepo{} }

// ListByUser 列出指定用户创建的 API Key（不含密钥明文，KeyHash 已被 json:"-" 隐藏）。
func (r *ApiKeyRepo) ListByUser(ctx context.Context, userID int64) ([]model.ApiKey, error) {
	var items []model.ApiKey
	err := bootstrap.DB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.ApiKey{}
	}
	return items, nil
}

// GetByID 按主键查询 API Key。
func (r *ApiKeyRepo) GetByID(ctx context.Context, id int64) (*model.ApiKey, error) {
	var m model.ApiKey
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByHash 按密钥 SHA256 摘要查询（开放接口鉴权用）。
func (r *ApiKeyRepo) GetByHash(ctx context.Context, keyHash string) (*model.ApiKey, error) {
	var m model.ApiKey
	err := bootstrap.DB().WithContext(ctx).
		Where("key_hash = ? AND active = ?", keyHash, true).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增 API Key（仅存储摘要与前缀，不存储明文）。
func (r *ApiKeyRepo) Create(ctx context.Context, m *model.ApiKey) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Delete 删除 API Key，并校验归属，防止越权删除他人密钥。
func (r *ApiKeyRepo) Delete(ctx context.Context, id, userID int64) error {
	res := bootstrap.DB().WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.ApiKey{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateLastUsed 更新最后使用时间。
func (r *ApiKeyRepo) UpdateLastUsed(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.ApiKey{}).
		Where("id = ?", id).
		Update("last_used_dt", time.Now()).Error
}
