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

// RiskRepo 风险记录数据访问层。表 ac_risk_record。
type RiskRepo struct{}

// NewRiskRepo 构造风险记录仓储。
func NewRiskRepo() *RiskRepo { return &RiskRepo{} }

// baseQuery 构造风险列表查询基座。
//
// 别名 r 有两个作用：使 DataScope 能以 "r.dept_id" 限定列名；
// GORM 由 Table() 解析出别名后，软删除条件同样被限定为 "r"."deleted_at"。
// 关联设备表以别名 d 左连接，仅用于关键字同时匹配设备编号与设备名称。
func (r *RiskRepo) baseQuery(ctx context.Context, req *request.PageRiskReq, ds *utils.DataScope) *gorm.DB {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.RiskRecord{}).
		Table("ac_risk_record AS r").
		Joins("LEFT JOIN ac_device AS d ON d.id = r.device_id AND d.deleted_at IS NULL")

	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(r.device_code ILIKE ? OR d.name ILIKE ?)", like, like)
	}
	if req.RiskTypeID > 0 {
		q = q.Where("r.risk_type_id = ?", req.RiskTypeID)
	}
	if req.RegionID > 0 {
		q = q.Where("r.region_id = ?", req.RegionID)
	}
	if req.DeviceID > 0 {
		q = q.Where("r.device_id = ?", req.DeviceID)
	}
	if code := strings.TrimSpace(req.DeviceCode); code != "" {
		q = q.Where("r.device_code = ?", code)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		q = q.Where("r.status = ?", status)
	}
	if accurate := strings.TrimSpace(req.DetectAccurate); accurate != "" {
		q = q.Where("r.detect_accurate = ?", accurate)
	}
	if req.Read != nil {
		// read 是 SQL 非保留关键字，显式加引号避免歧义。
		q = q.Where(`r."read" = ?`, *req.Read)
	}
	if start, end := parseFilterDateRange(req.StartDT, req.EndDT); start != nil || end != nil {
		if start != nil {
			q = q.Where("r.occurred_dt >= ?", *start)
		}
		if end != nil {
			q = q.Where("r.occurred_dt <= ?", *end)
		}
	}

	return ds.ApplyToQuery(q, "r.dept_id")
}

// Page 分页查询风险记录，默认按发生时间倒序。
// 返回 items + total，items 已预加载风险类型、设备、区域名称。
func (r *RiskRepo) Page(ctx context.Context, req *request.PageRiskReq, ds *utils.DataScope) ([]model.RiskRecord, int64, error) {
	// 计数与取数分别构造查询，避免复用同一 gorm.DB 造成语句污染。
	var total int64
	if err := r.baseQuery(ctx, req, ds).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]model.RiskRecord, 0)
	if total == 0 {
		return items, 0, nil
	}

	if err := r.baseQuery(ctx, req, ds).
		Preload("RiskType").
		Preload("Device").
		Preload("Region").
		Order("r.occurred_dt DESC").
		Order("r.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// GetByID 按主键查询风险记录，预加载关联名称。
func (r *RiskRepo) GetByID(ctx context.Context, id int64) (*model.RiskRecord, error) {
	var rec model.RiskRecord
	err := bootstrap.DB().WithContext(ctx).
		Preload("RiskType").
		Preload("Device").
		Preload("Region").
		First(&rec, id).Error
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// Create 新增风险记录。
func (r *RiskRepo) Create(ctx context.Context, m *model.RiskRecord) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// UpdateFields 按主键更新指定列。
// 显式携带 updated_by：map 形式的 Updates 不会回填模型钩子写入的审计字段。
func (r *RiskRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}

	res := bootstrap.DB().WithContext(ctx).
		Model(&model.RiskRecord{}).
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

// Delete 软删除风险记录。
func (r *RiskRepo) Delete(ctx context.Context, id int64) error {
	res := bootstrap.DB().WithContext(ctx).Delete(&model.RiskRecord{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountPending 统计未处理风险数量（status = pending，含未读）。
func (r *RiskRepo) CountPending(ctx context.Context, ds *utils.DataScope) (int64, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.RiskRecord{}).
		Table("ac_risk_record AS r").
		Where("r.status = ?", model.HandleStatusPending)

	var total int64
	if err := ds.ApplyToQuery(q, "r.dept_id").Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// RiskSummaryRow 全局风险摘要（用于「AI风险」台账顶部指标）。
//
// 与分页列表使用同一数据权限过滤，保证指标与列表口径一致。
type RiskSummaryRow struct {
	Total       int64
	Pending     int64
	Handled     int64
	High        int64
	Unread      int64
	AccurateYes int64
	AccurateNo  int64
	Unconfirmed int64
	Today       int64
}

// Summary 统计风险总览。
func (r *RiskRepo) Summary(ctx context.Context, ds *utils.DataScope) (*RiskSummaryRow, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.RiskRecord{}).
		Table("ac_risk_record AS r").
		Select(`
			COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN r.status = 'pending' THEN 1 ELSE 0 END), 0) AS pending,
			COALESCE(SUM(CASE WHEN r.status = 'handled' THEN 1 ELSE 0 END), 0) AS handled,
			COALESCE(SUM(CASE WHEN r.risk_level = 'high' THEN 1 ELSE 0 END), 0) AS high,
			COALESCE(SUM(CASE WHEN r.read = FALSE THEN 1 ELSE 0 END), 0) AS unread,
			COALESCE(SUM(CASE WHEN r.detect_accurate = 'yes' THEN 1 ELSE 0 END), 0) AS accurate_yes,
			COALESCE(SUM(CASE WHEN r.detect_accurate = 'no' THEN 1 ELSE 0 END), 0) AS accurate_no,
			COALESCE(SUM(CASE WHEN r.detect_accurate IS NULL OR r.detect_accurate = '' THEN 1 ELSE 0 END), 0) AS unconfirmed,
			COALESCE(SUM(CASE WHEN r.occurred_dt >= CURRENT_DATE THEN 1 ELSE 0 END), 0) AS today`)
	if ds != nil {
		q = ds.ApplyToQuery(q, "r.dept_id")
	}
	var row RiskSummaryRow
	if err := q.Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// ────────────────────────────────────────────────────────────────
// 上报接口辅助查询
//
// 设备与风险类型的按编码查询放在本仓储内，避免风险切片依赖其它切片的
// 仓储接口命名，保证分层切片可独立编译。
// ────────────────────────────────────────────────────────────────

// FindDeviceByCode 按设备编号查询设备（不含软删除记录）。
func (r *RiskRepo) FindDeviceByCode(ctx context.Context, code string) (*model.Device, error) {
	var dev model.Device
	if err := bootstrap.DB().WithContext(ctx).
		Where("device_code = ?", strings.TrimSpace(code)).
		First(&dev).Error; err != nil {
		return nil, err
	}
	return &dev, nil
}

// FindRiskTypeByCode 按风险编码查询风险类型。
func (r *RiskRepo) FindRiskTypeByCode(ctx context.Context, code string) (*model.RiskType, error) {
	var rt model.RiskType
	if err := bootstrap.DB().WithContext(ctx).
		Where("code = ?", strings.TrimSpace(code)).
		First(&rt).Error; err != nil {
		return nil, err
	}
	return &rt, nil
}

// ────────────────────────────────────────────────────────────────
// 包内共享工具（仅本切片的风险/告警查询使用）
// ────────────────────────────────────────────────────────────────

// filterDateTimeLayouts 支持的时间字符串格式，覆盖前端日期控件与设备上报。
var filterDateTimeLayouts = []string{
	"2006-01-02 15:04:05",
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// parseFilterDateTime 解析时间字符串；空串或格式不识别时返回 false。
func parseFilterDateTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range filterDateTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseFilterDateRange 解析时间范围。仅传日期时，结束时间补足为当日 23:59:59，
// 避免「按日筛选」漏掉当天下午的数据。
func parseFilterDateRange(start, end string) (*time.Time, *time.Time) {
	var startT, endT *time.Time

	if t, ok := parseFilterDateTime(start); ok {
		startT = &t
	}
	if t, ok := parseFilterDateTime(end); ok {
		if len(strings.TrimSpace(end)) == len("2006-01-02") {
			t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
		}
		endT = &t
	}
	return startT, endT
}
