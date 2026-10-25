package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm/clause"
)

// ════════════════════════════════════════════════════════════════
// 国标云台预置位名称（ac_gb_ptz_preset）
//
// 平台侧只保存「编号 → 名称」。设备侧预置位的位置与存在性仍以设备为准，
// 因此这里的写入是「尽力而为」的展示层元数据，不参与 PTZCmd 的构造。
// ════════════════════════════════════════════════════════════════

// GbPtzPresetRepo 预置位名称数据访问。
type GbPtzPresetRepo struct{}

// NewGbPtzPresetRepo 构造预置位名称仓储。
func NewGbPtzPresetRepo() *GbPtzPresetRepo { return &GbPtzPresetRepo{} }

// ListByChannel 查询通道下全部预置位名称（按编号升序）。
func (r *GbPtzPresetRepo) ListByChannel(ctx context.Context, channelID int64) ([]model.GbPtzPreset, error) {
	var items []model.GbPtzPreset
	if err := bootstrap.DB().WithContext(ctx).
		Where("channel_id = ?", channelID).
		Order("preset_id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Get 查询单个预置位名称。
func (r *GbPtzPresetRepo) Get(ctx context.Context, channelID int64, presetID int) (*model.GbPtzPreset, error) {
	var m model.GbPtzPreset
	if err := bootstrap.DB().WithContext(ctx).
		Where("channel_id = ? AND preset_id = ?", channelID, presetID).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Upsert 写入或更新预置位名称。
//
// 用 ON CONFLICT(channel_id, preset_id) DO UPDATE 而不是先查后写：
// 并发设置同一预置位时不会撞唯一键；也不会因为软删除行残留而失败。
func (r *GbPtzPresetRepo) Upsert(ctx context.Context, channelID int64, presetID int, name string) error {
	m := &model.GbPtzPreset{
		ChannelID: channelID,
		PresetID:  presetID,
		Name:      strings.TrimSpace(name),
	}
	m.UpdatedBy = model.UsernameFromCtx(ctx)
	m.UpdatedDT = time.Now()

	return bootstrap.DB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}, {Name: "preset_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"name":       m.Name,
			"updated_by": m.UpdatedBy,
			"updated_dt": m.UpdatedDT,
			"deleted_at": nil,
		}),
	}).Create(m).Error
}

// Delete 删除预置位名称。
//
// ★ 物理删除（Unscoped）：唯一键 (channel_id, preset_id) 不区分软删除，
// 软删除行会让「删除后重新设置同一编号」撞唯一键。预置位名称只是展示元数据，
// 没有审计/追溯价值，物理删除是安全的。
func (r *GbPtzPresetRepo) Delete(ctx context.Context, channelID int64, presetID int) error {
	return bootstrap.DB().WithContext(ctx).Unscoped().
		Where("channel_id = ? AND preset_id = ?", channelID, presetID).
		Delete(&model.GbPtzPreset{}).Error
}

// DeleteByChannel 删除通道下全部预置位名称（重置通道时用）。
func (r *GbPtzPresetRepo) DeleteByChannel(ctx context.Context, channelID int64) error {
	return bootstrap.DB().WithContext(ctx).Unscoped().
		Where("channel_id = ?", channelID).
		Delete(&model.GbPtzPreset{}).Error
}
