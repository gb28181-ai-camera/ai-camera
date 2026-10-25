package repository

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// ════════════════════════════════════════════════════════════════
// 国标推流导入数据访问
//
// 导入是同步的逐行校验 + 落库，不建任务表。这里只补导入需要的
// 「按国标编码找通道」与「判重后创建」两类操作。
// ════════════════════════════════════════════════════════════════

// GbStreamImportRepo 推流导入数据访问。
type GbStreamImportRepo struct{}

// NewGbStreamImportRepo 构造导入仓储。
func NewGbStreamImportRepo() *GbStreamImportRepo { return &GbStreamImportRepo{} }

// GetChannelByGbID 按通道国标编码查询通道（带设备名称）。
func (r *GbStreamImportRepo) GetChannelByGbID(ctx context.Context, gbChannelID string) (*model.GbChannel, error) {
	var ch model.GbChannel
	if err := bootstrap.DB().WithContext(ctx).
		Preload("Device").
		Where("gb_channel_id = ?", gbChannelID).
		First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

// PushExists 判断 app+stream 是否已有推流配置（含软删除，避免复用已删除配置的流名）。
func (r *GbStreamImportRepo) PushExists(ctx context.Context, app, stream string) (bool, error) {
	var count int64
	err := bootstrap.DB().WithContext(ctx).Unscoped().Model(&model.GbStreamPush{}).
		Where("app = ? AND stream = ?", app, stream).
		Count(&count).Error
	return count > 0, err
}

// CreatePush 新增推流配置。
func (r *GbStreamImportRepo) CreatePush(ctx context.Context, m *model.GbStreamPush) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}
