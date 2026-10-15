package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标视频数据访问（三）：国标报警 / 移动位置 / 订阅 / 信令日志
// ════════════════════════════════════════════════════════════════

// ── 国标报警 ──────────────────────────────────────────────────

// GbAlarmRepo 国标报警数据访问。表 ac_gb_alarm（含 dept_id）。
type GbAlarmRepo struct{}

// NewGbAlarmRepo 构造国标报警仓储。
func NewGbAlarmRepo() *GbAlarmRepo { return &GbAlarmRepo{} }

// Page 分页查询国标报警。Preload 关联设备/通道，列表直接展示名称。
func (r *GbAlarmRepo) Page(ctx context.Context, req *request.PageGbAlarmReq, ds *utils.DataScope) ([]model.GbAlarm, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbAlarm{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(alarm_description ILIKE ? OR gb_device_id ILIKE ? OR gb_channel_id ILIKE ? OR alarm_type ILIKE ?)", like, like, like, like)
		}
		if req.DeviceID != nil {
			q = q.Where("device_id = ?", *req.DeviceID)
		}
		if req.ChannelID != nil {
			q = q.Where("channel_id = ?", *req.ChannelID)
		}
		if s := strings.TrimSpace(req.AlarmType); s != "" {
			q = q.Where("alarm_type = ?", s)
		}
		if s := strings.TrimSpace(req.AlarmPriority); s != "" {
			q = q.Where("alarm_priority = ?", s)
		}
		if req.Handled != nil {
			q = q.Where("handled = ?", *req.Handled)
		}
		if s := strings.TrimSpace(req.StartDT); s != "" {
			q = q.Where("alarm_time >= ?", s)
		}
		if s := strings.TrimSpace(req.EndDT); s != "" {
			q = q.Where("alarm_time <= ?", s)
		}
		return ds.ApplyToQuery(q, "ac_gb_alarm.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbAlarm
	err := build().
		Preload("Device").
		Preload("Channel").
		Order("alarm_time DESC, id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID 按主键查询报警详情（含关联对象）。
func (r *GbAlarmRepo) GetByID(ctx context.Context, id int64) (*model.GbAlarm, error) {
	var m model.GbAlarm
	err := bootstrap.DB().WithContext(ctx).
		Preload("Device").Preload("Channel").First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 写入一条国标报警（设备 MANSCDP Alarm 上报落库）。
func (r *GbAlarmRepo) Create(ctx context.Context, m *model.GbAlarm) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// UpdateFields 局部更新（处理标记 / 附抓图路径等）。
func (r *GbAlarmRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbAlarm{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除报警。
func (r *GbAlarmRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbAlarm{}, id).Error
}

// ── 移动位置 ──────────────────────────────────────────────────

// GbMobilePositionRepo 移动位置数据访问。表 ac_gb_mobile_position（含 dept_id）。
type GbMobilePositionRepo struct{}

// NewGbMobilePositionRepo 构造移动位置仓储。
func NewGbMobilePositionRepo() *GbMobilePositionRepo { return &GbMobilePositionRepo{} }

// Page 分页查询移动位置轨迹。Preload 关联设备/通道。
func (r *GbMobilePositionRepo) Page(ctx context.Context, req *request.PageGbMobilePositionReq, ds *utils.DataScope) ([]model.GbMobilePosition, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbMobilePosition{})
		if req.DeviceID != nil {
			q = q.Where("device_id = ?", *req.DeviceID)
		}
		if req.ChannelID != nil {
			q = q.Where("channel_id = ?", *req.ChannelID)
		}
		if s := strings.TrimSpace(req.GbChannelID); s != "" {
			q = q.Where("gb_channel_id = ?", s)
		}
		if s := strings.TrimSpace(req.StartDT); s != "" {
			q = q.Where("report_time >= ?", s)
		}
		if s := strings.TrimSpace(req.EndDT); s != "" {
			q = q.Where("report_time <= ?", s)
		}
		return ds.ApplyToQuery(q, "ac_gb_mobile_position.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbMobilePosition
	err := build().
		Preload("Device").
		Preload("Channel").
		Order("report_time DESC, id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetLatestByChannel 查询某通道的最新位置（GIS「实时位置」用）。
func (r *GbMobilePositionRepo) GetLatestByChannel(ctx context.Context, channelID int64) (*model.GbMobilePosition, error) {
	var m model.GbMobilePosition
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbMobilePosition{}).
		Where("channel_id = ?", channelID).
		Order("report_time DESC, id DESC").First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 写入一条移动位置。
func (r *GbMobilePositionRepo) Create(ctx context.Context, m *model.GbMobilePosition) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// ── 订阅记录 ──────────────────────────────────────────────────

// GbSubscribeRepo 国标订阅记录数据访问。表 ac_gb_subscribe（含 dept_id）。
type GbSubscribeRepo struct{}

// NewGbSubscribeRepo 构造订阅记录仓储。
func NewGbSubscribeRepo() *GbSubscribeRepo { return &GbSubscribeRepo{} }

// Page 分页查询订阅记录。
func (r *GbSubscribeRepo) Page(ctx context.Context, req *request.PageGbSubscribeReq, ds *utils.DataScope) ([]model.GbSubscribe, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbSubscribe{})
		if s := strings.TrimSpace(req.GbDeviceID); s != "" {
			q = q.Where("gb_device_id = ?", s)
		}
		if s := strings.TrimSpace(req.Type); s != "" {
			q = q.Where("type = ?", s)
		}
		if s := strings.TrimSpace(req.Status); s != "" {
			q = q.Where("status = ?", s)
		}
		return ds.ApplyToQuery(q, "ac_gb_subscribe.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbSubscribe
	err := build().Order("id DESC").Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// GetByCallID 按 Call-ID 查询订阅（续订/终止时定位）。
func (r *GbSubscribeRepo) GetByCallID(ctx context.Context, callID string) (*model.GbSubscribe, error) {
	var m model.GbSubscribe
	if err := bootstrap.DB().WithContext(ctx).
		Where("call_id = ?", callID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListExpiring 查询 expire_dt 之前即将过期的活跃订阅（定时续订任务用）。
func (r *GbSubscribeRepo) ListExpiring(ctx context.Context, before time.Time) ([]model.GbSubscribe, error) {
	var items []model.GbSubscribe
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbSubscribe{}).
		Where("status = ? AND expire_dt IS NOT NULL AND expire_dt <= ?", "active", before).
		Order("expire_dt ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Create 新增订阅记录。
func (r *GbSubscribeRepo) Create(ctx context.Context, m *model.GbSubscribe) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存订阅记录。
func (r *GbSubscribeRepo) Update(ctx context.Context, m *model.GbSubscribe) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新（续订刷新 expire_dt / 置过期）。
func (r *GbSubscribeRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbSubscribe{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除订阅记录。
func (r *GbSubscribeRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbSubscribe{}, id).Error
}

// ── 信令日志 ──────────────────────────────────────────────────

// GbSipLogRepo 信令日志数据访问。表 ac_gb_sip_log。
//
// ⚠️ 本表是**追加写日志**，无 dept_id、无软删除，**不做数据权限过滤**；
// 可见性由接口层的 gb:sip-log:list 权限码控制（设计文档 §10.1 的唯一真例外）。
// 查询一律按 created_dt 倒序，且必须配合时间范围条件，避免全表扫描。
type GbSipLogRepo struct{}

// NewGbSipLogRepo 构造信令日志仓储。
func NewGbSipLogRepo() *GbSipLogRepo { return &GbSipLogRepo{} }

// Page 分页查询信令日志（不做 DataScope —— 见类型注释）。
func (r *GbSipLogRepo) Page(ctx context.Context, req *request.PageGbSipLogReq) ([]model.GbSipLog, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbSipLog{})
		if s := strings.TrimSpace(req.GbDeviceID); s != "" {
			q = q.Where("gb_device_id = ?", s)
		}
		if s := strings.TrimSpace(req.GbChannelID); s != "" {
			q = q.Where("gb_channel_id = ?", s)
		}
		if s := strings.TrimSpace(req.Direction); s != "" {
			q = q.Where("direction = ?", s)
		}
		if s := strings.TrimSpace(req.Method); s != "" {
			q = q.Where("method = ?", s)
		}
		if s := strings.TrimSpace(req.CmdType); s != "" {
			q = q.Where("cmd_type = ?", s)
		}
		if s := strings.TrimSpace(req.Result); s != "" {
			q = q.Where("result = ?", s)
		}
		if s := strings.TrimSpace(req.StartDT); s != "" {
			q = q.Where("created_dt >= ?", s)
		}
		if s := strings.TrimSpace(req.EndDT); s != "" {
			q = q.Where("created_dt <= ?", s)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbSipLog
	err := build().Order("created_dt DESC, id DESC").
		Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// ListRecent 查询最近 N 条信令日志（实时排障面板用）。
func (r *GbSipLogRepo) ListRecent(ctx context.Context, limit int) ([]model.GbSipLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var items []model.GbSipLog
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbSipLog{}).
		Order("created_dt DESC, id DESC").Limit(limit).Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Create 追加写入一条信令日志（路径上最高频的写操作）。
func (r *GbSipLogRepo) Create(ctx context.Context, m *model.GbSipLog) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// BatchCreate 批量追加写入信令日志。
func (r *GbSipLogRepo) BatchCreate(ctx context.Context, items []model.GbSipLog) error {
	if len(items) == 0 {
		return nil
	}
	return bootstrap.DB().WithContext(ctx).Create(&items).Error
}

// DeleteBefore 物理删除 before 之前的日志（归档/清理任务用；日志表无软删除）。
func (r *GbSipLogRepo) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	tx := bootstrap.DB().WithContext(ctx).
		Where("created_dt < ?", before).
		Delete(&model.GbSipLog{})
	return tx.RowsAffected, tx.Error
}
