package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标设备扩展数据访问（设备档案编辑 / 时间统计）
//
// 与 gb28181_repo.go 的分工：那个文件只服务信令层的白名单与注册状态；
// 本文件服务 WVP 对齐新增的「设备编辑 / 目录同步进度 / 心跳间隔统计」。
// 两者操作同一张 ac_device / ac_gb_sip_log，不新增表。
// ════════════════════════════════════════════════════════════════

// GbDeviceExtraRepo 国标设备扩展数据访问。
type GbDeviceExtraRepo struct{}

// NewGbDeviceExtraRepo 构造国标设备扩展仓储。
func NewGbDeviceExtraRepo() *GbDeviceExtraRepo { return &GbDeviceExtraRepo{} }

// GetByID 按主键读取设备（含区域）。不限制 gb_device_id 是否为空，
// 由 service 层根据操作语义决定是否要求国标编码。
func (r *GbDeviceExtraRepo) GetByID(ctx context.Context, id int64) (*model.Device, error) {
	var d model.Device
	if err := bootstrap.DB().WithContext(ctx).
		Preload("Region").
		First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// UpdateProfile 局部更新设备档案字段，并补写审计字段。
//
// 同时处理 gb_device_id 变更：ac_gb_channel.gb_device_id 是冗余列，
// 设备编码改了却不同步通道会让通道列表展示旧编码，排障时非常误导。
// 两步放在同一事务，避免中途失败留下不一致。
func (r *GbDeviceExtraRepo) UpdateProfile(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}

	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Device{}).Where("id = ?", id).Updates(fields).Error; err != nil {
			return err
		}
		if v, ok := fields["gb_device_id"].(string); ok && strings.TrimSpace(v) != "" {
			if err := tx.Model(&model.GbChannel{}).
				Where("device_id = ?", id).
				Update("gb_device_id", v).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// MediaServerExists 判断媒体节点（server_id）是否存在。
func (r *GbDeviceExtraRepo) MediaServerExists(ctx context.Context, serverID string) (bool, error) {
	if strings.TrimSpace(serverID) == "" {
		return false, nil
	}
	var n int64
	if err := bootstrap.DB().WithContext(ctx).Model(&model.GbMediaServer{}).
		Where("server_id = ?", serverID).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListInboundLogsAsc 查询设备最近 limit 条指定类型的入向信令，按时间升序返回。
//
// 为什么不复用 GbSipLogRepo.Page：分页接口按时间倒序、且需要 Request DTO，
// 时间间隔计算必须升序拿相邻两条。这里用「先取最新 N 条（倒序子查询）再升序」
// 的方式，仍走 created_dt 索引，不新增表。
//
// method 为空表示不限；cmdType 为空表示不限。
func (r *GbDeviceExtraRepo) ListInboundLogsAsc(ctx context.Context, gbDeviceID, method, cmdType string, limit int) ([]model.GbSipLog, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbSipLog{}).
			Where("gb_device_id = ?", gbDeviceID).
			Where("direction = ?", "in")
		if v := strings.TrimSpace(method); v != "" {
			q = q.Where("method = ?", v)
		}
		if v := strings.TrimSpace(cmdType); v != "" {
			q = q.Where("cmd_type = ?", v)
		}
		return q
	}

	sub := build().Select("id").Order("created_dt DESC, id DESC").Limit(limit)
	var items []model.GbSipLog
	if err := build().Where("id IN (?)", sub).
		Order("created_dt ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
