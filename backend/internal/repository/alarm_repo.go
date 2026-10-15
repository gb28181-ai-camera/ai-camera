package repository

import (
	"context"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// AlarmRepo 设备告警数据访问层。表 ac_alarm_record。
type AlarmRepo struct{}

// NewAlarmRepo 构造设备告警仓储。
func NewAlarmRepo() *AlarmRepo { return &AlarmRepo{} }

// baseQuery 构造告警列表查询基座。
// 与 RiskRepo 一致：别名 a 使 DataScope 能限定 "a.dept_id"，
// 且 GORM 解析别名后软删除条件同样被限定为 "a"."deleted_at"。
func (r *AlarmRepo) baseQuery(ctx context.Context, req *request.PageAlarmReq, ds *utils.DataScope) *gorm.DB {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.AlarmRecord{}).
		Table("ac_alarm_record AS a").
		Joins("LEFT JOIN ac_device AS d ON d.id = a.device_id AND d.deleted_at IS NULL")

	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(a.device_code ILIKE ? OR d.name ILIKE ?)", like, like)
	}
	if req.AlarmTypeID > 0 {
		q = q.Where("a.alarm_type_id = ?", req.AlarmTypeID)
	}
	if req.RegionID > 0 {
		q = q.Where("a.region_id = ?", req.RegionID)
	}
	if req.DeviceID > 0 {
		q = q.Where("a.device_id = ?", req.DeviceID)
	}
	if code := strings.TrimSpace(req.DeviceCode); code != "" {
		q = q.Where("a.device_code = ?", code)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		q = q.Where("a.status = ?", status)
	}
	if req.Read != nil {
		// read 是 SQL 非保留关键字，显式加引号避免歧义。
		q = q.Where(`a."read" = ?`, *req.Read)
	}
	if start, end := parseFilterDateRange(req.StartDT, req.EndDT); start != nil || end != nil {
		if start != nil {
			q = q.Where("a.occurred_dt >= ?", *start)
		}
		if end != nil {
			q = q.Where("a.occurred_dt <= ?", *end)
		}
	}

	return ds.ApplyToQuery(q, "a.dept_id")
}

// Page 分页查询告警记录，默认按告警时间倒序。
// items 已预加载告警类型、设备、区域名称。
func (r *AlarmRepo) Page(ctx context.Context, req *request.PageAlarmReq, ds *utils.DataScope) ([]model.AlarmRecord, int64, error) {
	var total int64
	if err := r.baseQuery(ctx, req, ds).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]model.AlarmRecord, 0)
	if total == 0 {
		return items, 0, nil
	}

	if err := r.baseQuery(ctx, req, ds).
		Preload("AlarmType").
		Preload("Device").
		Preload("Region").
		Order("a.occurred_dt DESC").
		Order("a.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetByID 按主键查询告警记录，预加载关联名称。
func (r *AlarmRepo) GetByID(ctx context.Context, id int64) (*model.AlarmRecord, error) {
	var rec model.AlarmRecord
	err := bootstrap.DB().WithContext(ctx).
		Preload("AlarmType").
		Preload("Device").
		Preload("Region").
		First(&rec, id).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// Create 新增告警记录。
func (r *AlarmRepo) Create(ctx context.Context, m *model.AlarmRecord) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// UpdateFields 按主键更新指定列（显式携带 updated_by）。
func (r *AlarmRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}

	res := bootstrap.DB().WithContext(ctx).
		Model(&model.AlarmRecord{}).
		Where("id = ?", id).
		Updates(fields)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 软删除告警记录。
func (r *AlarmRepo) Delete(ctx context.Context, id int64) error {
	res := bootstrap.DB().WithContext(ctx).Delete(&model.AlarmRecord{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountPending 统计未处理告警数量（status = pending）。
func (r *AlarmRepo) CountPending(ctx context.Context, ds *utils.DataScope) (int64, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.AlarmRecord{}).
		Table("ac_alarm_record AS a").
		Where("a.status = ?", model.HandleStatusPending)

	var total int64
	if err := ds.ApplyToQuery(q, "a.dept_id").Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// ────────────────────────────────────────────────────────────────
// 上报接口辅助查询（避免跨切片依赖，保证切片可独立编译）
// ────────────────────────────────────────────────────────────────

// FindDeviceByCode 按设备编号查询设备（不含软删除记录）。
func (r *AlarmRepo) FindDeviceByCode(ctx context.Context, code string) (*model.Device, error) {
	var dev model.Device
	if err := bootstrap.DB().WithContext(ctx).
		Where("device_code = ?", strings.TrimSpace(code)).
		First(&dev).Error; err != nil {
		return nil, err
	}
	return &dev, nil
}

// FindAlarmTypeByCode 按告警编码查询告警类型。
func (r *AlarmRepo) FindAlarmTypeByCode(ctx context.Context, code string) (*model.AlarmType, error) {
	var at model.AlarmType
	if err := bootstrap.DB().WithContext(ctx).
		Where("code = ?", strings.TrimSpace(code)).
		First(&at).Error; err != nil {
		return nil, err
	}
	return &at, nil
}
