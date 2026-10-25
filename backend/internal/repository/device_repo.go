package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 设备数据访问
// ════════════════════════════════════════════════════════════════

// DeviceRepo 设备数据访问层。
type DeviceRepo struct{}

// NewDeviceRepo 构造设备仓储。
func NewDeviceRepo() *DeviceRepo { return &DeviceRepo{} }

// deviceQuery 构造带表别名 d 的设备查询基座。
//
// 别名有两个作用：
//  1. 使 DataScope 能以 "d.dept_id" 限定列名（多表场景下无歧义）；
//  2. GORM 由 Table() 解析出别名后，软删除条件同样被限定为 "d.deleted_at"。
//
// Model() 显式提供 Schema，保证 Preload 关联与软删除子句都能生效。
func deviceQuery() *gorm.DB {
	return bootstrap.DB().Model(&model.Device{}).Table("ac_device AS d")
}

// deviceFilters 施加列表筛选条件。
func deviceFilters(q *gorm.DB, req *request.PageDeviceReq) *gorm.DB {
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(d.name ILIKE ? OR d.device_code ILIKE ? OR d.location ILIKE ?)", like, like, like)
	}
	if req.RegionID != nil {
		q = q.Where("d.region_id = ?", *req.RegionID)
	}
	if s := strings.TrimSpace(req.Status); s != "" {
		q = q.Where("d.status = ?", s)
	}
	if s := strings.TrimSpace(req.DeviceMode); s != "" {
		q = q.Where("d.device_mode = ?", s)
	}
	if s := strings.TrimSpace(req.WorkMode); s != "" {
		q = q.Where("d.work_mode = ?", s)
	}
	return q
}

// Page 分页查询设备，返回列表与总数。
//
// Preload 关联区域/物联网卡/固件/AI模型，使列表直接展示关联名称而非 ID。
func (r *DeviceRepo) Page(ctx context.Context, req *request.PageDeviceReq, ds *utils.DataScope) ([]model.Device, int64, error) {
	q := deviceFilters(deviceQuery().WithContext(ctx), req)
	q = ds.ApplyToQuery(q, "d.dept_id")

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Device
	err := q.Session(&gorm.Session{}).
		Preload("Region").
		Preload("IotCard").
		Preload("Firmware").
		Preload("AIModel").
		Order("d.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Options 设备下拉选项（仅启用状态）。
func (r *DeviceRepo) Options(ctx context.Context, ds *utils.DataScope) ([]model.Device, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Table("ac_device AS d").
		Select("d.id, d.name, d.device_code").
		Where("(d.active IS NULL OR d.active = true)")
	q = ds.ApplyToQuery(q, "d.dept_id")

	var items []model.Device
	if err := q.Order("d.id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询设备详情（含关联对象）。
func (r *DeviceRepo) GetByID(ctx context.Context, id int64) (*model.Device, error) {
	var m model.Device
	err := bootstrap.DB().WithContext(ctx).
		Preload("Region").
		Preload("IotCard").
		Preload("Firmware").
		Preload("AIModel").
		First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByCode 按设备编号查询（不含软删除）。
//
// 设备端 MQTT 上报没有用户上下文，只能以 device_code 作为身份的一部分；
// 调用方仍需校验该设备的产品是否绑定到消息所在服务组。
func (r *DeviceRepo) GetByCode(ctx context.Context, code string) (*model.Device, error) {
	var m model.Device
	if err := bootstrap.DB().WithContext(ctx).
		Where("device_code = ?", strings.TrimSpace(code)).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByIDScoped 按主键查询设备，并施加行级数据权限。
//
// 用于「按 ID 进入一个受控资源」的接口（如 WebRTC 查看）：列表过滤不能替代
// 详情越权校验，否则知道 ID 就能绕过列表拿到别人的设备。
func (r *DeviceRepo) GetByIDScoped(ctx context.Context, id int64, ds *utils.DataScope) (*model.Device, error) {
	q := deviceQuery().WithContext(ctx).Where("d.id = ?", id)
	if ds != nil {
		q = ds.ApplyToQuery(q, "d.dept_id")
	}
	var m model.Device
	if err := q.Select("d.*").
		Preload("Region").
		Preload("IotCard").
		Preload("Firmware").
		Preload("AIModel").
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsByCode 判断设备编号是否已存在（excludeID>0 时排除自身）。
func (r *DeviceRepo) ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.Device{}).Where("device_code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// Create 新增设备。
func (r *DeviceRepo) Create(ctx context.Context, m *model.Device) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存设备。
func (r *DeviceRepo) Update(ctx context.Context, m *model.Device) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新设备字段。fields 为空时不执行任何操作。
//
// 采用 map 更新可精确控制写入列（避免 Save 全字段回写误伤未提交的字段），
// 但 GORM 不会为 map 更新触发模型的 BeforeUpdate 钩子，故此处显式补审计字段。
func (r *DeviceRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
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
		Model(&model.Device{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete 软删除设备。
func (r *DeviceRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.Device{}, id).Error
}

// ── 设备历史 ──────────────────────────────────────────────────

// PageHistory 分页查询设备运行历史。
//
// 注意：此处不能使用 Select("h.*")。GORM 的 Count 会把 Selects[0] 直接当作计数列
// 拼成 COUNT("h"."*")，导致设备历史列表始终报错（column h.* does not exist）。
// 单表查询无需限定列，Table 别名已足以让软删除条件带上 "h" 前缀。
func (r *DeviceRepo) PageHistory(ctx context.Context, req *request.PageDeviceHistoryReq, ds *utils.DataScope) ([]model.DeviceHistory, int64, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.DeviceHistory{}).
		Table("ac_device_history AS h")

	if req.DeviceID != nil {
		q = q.Where("h.device_id = ?", *req.DeviceID)
	}
	if req.RegionID != nil {
		q = q.Where("h.region_id = ?", *req.RegionID)
	}
	if s := strings.TrimSpace(req.Status); s != "" {
		q = q.Where("h.status = ?", s)
	}
	if s := strings.TrimSpace(req.DeviceCode); s != "" {
		q = q.Where("h.device_code ILIKE ?", "%"+s+"%")
	}
	if s := strings.TrimSpace(req.StartDT); s != "" {
		q = q.Where("h.sampled_dt >= ?", s)
	}
	if s := strings.TrimSpace(req.EndDT); s != "" {
		q = q.Where("h.sampled_dt <= ?", s)
	}
	q = ds.ApplyToQuery(q, "h.dept_id")

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.DeviceHistory
	err := q.Session(&gorm.Session{}).
		Preload("Device").
		Preload("Device.Region").
		Order("h.sampled_dt DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CreateHistory 写入一条设备运行历史。
func (r *DeviceRepo) CreateHistory(ctx context.Context, m *model.DeviceHistory) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// DeviceHistoryAggPoint 设备历史区间聚合点（电量/存储/状态/电压）。
type DeviceHistoryAggPoint struct {
	Bucket      time.Time `gorm:"column:bucket"`
	Battery     float64   `gorm:"column:battery"`
	StorageFree float64   `gorm:"column:storage_free"`
	StatusValue float64   `gorm:"column:status_value"`
	Voltage     float64   `gorm:"column:voltage"`
}

// DeviceStatusSummaryRow 运行状态时长汇总（单位：秒）。
type DeviceStatusSummaryRow struct {
	OnlineSeconds  float64 `gorm:"column:online_seconds"`
	OfflineSeconds float64 `gorm:"column:offline_seconds"`
}

// AggregateHistory 按 hour/day 聚合 since 之后的设备历史采样点。
//
// 电量、剩余存储取区间均值；状态曲线 online=1/offline=0 取均值；
// 电压字段为字符串（可能带单位），先用正则剔除非数字字符再取均值。
// 全部聚合列以 COALESCE 兜底，避免区间无数据时扫描 NULL 报错。
func (r *DeviceRepo) AggregateHistory(ctx context.Context, deviceID int64, since time.Time, granularity string) ([]DeviceHistoryAggPoint, error) {
	if granularity != "day" {
		granularity = "hour"
	}

	sql := fmt.Sprintf(`
SELECT date_trunc('%s', h.sampled_dt) AS bucket,
       COALESCE(AVG(h.battery), 0)::float8      AS battery,
       COALESCE(AVG(h.storage_free), 0)::float8 AS storage_free,
       COALESCE(AVG(CASE WHEN h.status = 'online' THEN 1 ELSE 0 END), 0)::float8 AS status_value,
       COALESCE(AVG(NULLIF(regexp_replace(COALESCE(h.voltage, ''), '[^0-9.]', '', 'g'), '')::numeric), 0)::float8 AS voltage
FROM ac_device_history h
WHERE h.device_id = ? AND h.sampled_dt >= ? AND h.deleted_at IS NULL
GROUP BY bucket
ORDER BY bucket`, granularity)

	var points []DeviceHistoryAggPoint
	if err := bootstrap.DB().WithContext(ctx).Raw(sql, deviceID, since).Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

// StatusSummary 统计 since 之后的在线/离线时长（秒）。
//
// 用窗口函数取相邻采样点间隔，按前一条采样的状态归属时长；
// 单段间隔上限 1 小时，避免设备长时间失联导致的异常放大。末条采样无后继，计 0。
func (r *DeviceRepo) StatusSummary(ctx context.Context, deviceID int64, since time.Time) (*DeviceStatusSummaryRow, error) {
	sql := `
WITH ordered AS (
    SELECT COALESCE(h.status, 'offline') AS status,
           h.sampled_dt,
           LEAD(h.sampled_dt) OVER (ORDER BY h.sampled_dt) AS next_dt
    FROM ac_device_history h
    WHERE h.device_id = ? AND h.sampled_dt >= ? AND h.deleted_at IS NULL
)
SELECT
    COALESCE(SUM(CASE WHEN status = 'online'
        THEN LEAST(EXTRACT(EPOCH FROM (COALESCE(next_dt, sampled_dt) - sampled_dt)), 3600)
        ELSE 0 END), 0)::float8 AS online_seconds,
    COALESCE(SUM(CASE WHEN status <> 'online'
        THEN LEAST(EXTRACT(EPOCH FROM (COALESCE(next_dt, sampled_dt) - sampled_dt)), 3600)
        ELSE 0 END), 0)::float8 AS offline_seconds
FROM ordered`

	var row DeviceStatusSummaryRow
	if err := bootstrap.DB().WithContext(ctx).Raw(sql, deviceID, since).Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
