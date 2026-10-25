package repository

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标运维增强数据访问：媒体节点采样清单 / 报警增量轮询
//
// 说明：媒体节点本身的 CRUD 仍在 gb28181_media_repo.go；
// 本文件只补「运维视图」需要的两类查询，避免改动他人的仓储文件。
// ════════════════════════════════════════════════════════════════

// GbOpsExtraRepo 运维增强查询。
type GbOpsExtraRepo struct{}

// NewGbOpsExtraRepo 构造运维增强仓储。
func NewGbOpsExtraRepo() *GbOpsExtraRepo { return &GbOpsExtraRepo{} }

// ListMediaServers 返回全部启用节点（指标采样器每轮遍历一次）。
func (r *GbOpsExtraRepo) ListMediaServers(ctx context.Context) ([]model.GbMediaServer, error) {
	var items []model.GbMediaServer
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbMediaServer{}).
		Where("(active IS NULL OR active = true)").
		Order("default_server DESC, id ASC").
		Find(&items).Error
	return items, err
}

// AlarmsAfterID 查询 id 大于 lastID 的报警（按 id 升序），用于 SSE 轮询。
//
// 报警表是追加写且 id 单调递增，用 id 游标比按时间戳轮询更可靠：
// 设备时钟漂移不会导致漏报/重报。
func (r *GbOpsExtraRepo) AlarmsAfterID(ctx context.Context, lastID int64, limit int) ([]model.GbAlarm, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var items []model.GbAlarm
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbAlarm{}).
		Preload("Device").
		Preload("Channel").
		Where("id > ?", lastID).
		Order("id ASC").
		Limit(limit).
		Find(&items).Error
	return items, err
}

// LatestAlarmID 返回当前最大报警 ID；SSE 建连时作为游标起点。
//
// 返回 0 表示表中没有报警：此时从 0 开始会重放历史记录，故调用方应
// 在「建连时只要新事件」的语义下直接使用该值。
func (r *GbOpsExtraRepo) LatestAlarmID(ctx context.Context) (int64, error) {
	var id int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbAlarm{}).
		Select("COALESCE(MAX(id), 0)").
		Scan(&id).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return id, err
	}
	return id, nil
}
