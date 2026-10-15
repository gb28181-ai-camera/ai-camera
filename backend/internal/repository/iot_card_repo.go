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

// ════════════════════════════════════════════════════════════════
// 物联网卡 / 换卡记录数据访问
// ════════════════════════════════════════════════════════════════

// IotCardRepo 物联网卡数据访问层。
type IotCardRepo struct{}

// NewIotCardRepo 构造物联网卡仓储。
func NewIotCardRepo() *IotCardRepo { return &IotCardRepo{} }

// iotCardQuery 构造带表别名 c 的物联网卡查询基座。
func iotCardQuery() *gorm.DB {
	return bootstrap.DB().Model(&model.IotCard{}).Table("ac_iot_card AS c")
}

// Page 分页查询物联网卡。
//
// Preload 区域与设备，使列表直接展示关联名称而非 ID。
func (r *IotCardRepo) Page(ctx context.Context, req *request.PageIotCardReq, ds *utils.DataScope) ([]model.IotCard, int64, error) {
	q := iotCardQuery().WithContext(ctx)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(c.name ILIKE ? OR c.card_no ILIKE ?)", like, like)
	}
	if req.RegionID != nil {
		q = q.Where("c.region_id = ?", *req.RegionID)
	}
	if req.DeviceID != nil {
		q = q.Where("c.device_id = ?", *req.DeviceID)
	}
	if s := strings.TrimSpace(req.Status); s != "" {
		q = q.Where("c.status = ?", s)
	}
	q = ds.ApplyToQuery(q, "c.dept_id")

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.IotCard
	err := q.Session(&gorm.Session{}).
		Preload("Region").
		Preload("Device").
		Order("c.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Options 物联网卡下拉选项（仅启用状态）。
func (r *IotCardRepo) Options(ctx context.Context, ds *utils.DataScope) ([]model.IotCard, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.IotCard{}).
		Table("ac_iot_card AS c").
		Select("c.id, c.name, c.card_no").
		Where("(c.active IS NULL OR c.active = true)")
	q = ds.ApplyToQuery(q, "c.dept_id")

	var items []model.IotCard
	if err := q.Order("c.id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询物联网卡。
func (r *IotCardRepo) GetByID(ctx context.Context, id int64) (*model.IotCard, error) {
	var m model.IotCard
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsByCardNo 判断卡号是否存在（excludeID>0 时排除自身）。
func (r *IotCardRepo) ExistsByCardNo(ctx context.Context, cardNo string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.IotCard{}).Where("card_no = ?", cardNo)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// Create 新增物联网卡。
func (r *IotCardRepo) Create(ctx context.Context, m *model.IotCard) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存物联网卡。
func (r *IotCardRepo) Update(ctx context.Context, m *model.IotCard) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除物联网卡。
func (r *IotCardRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.IotCard{}, id).Error
}

// BindDevice 设置物联网卡的绑定设备（deviceID 为 nil 表示解绑）。
//
// 供设备的增删改与换卡流程维护 device.iot_card_id 与 card.device_id 的双向一致。
func (r *IotCardRepo) BindDevice(ctx context.Context, cardID int64, deviceID *int64) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.IotCard{}).
		Where("id = ?", cardID).
		Update("device_id", deviceID).Error
}

// ── 换卡记录 ──────────────────────────────────────────────────

// PageReplaceLogs 分页查询换卡记录。
//
// 注意：此处不能使用 Select("l.*")。GORM 的 Count 会把 Selects[0] 直接当作计数列
// 拼成 COUNT("l"."*")，导致换卡记录列表始终报错（column l.* does not exist）。
// 单表查询无需限定列，Table 别名已足以让软删除条件带上 "l" 前缀。
func (r *IotCardRepo) PageReplaceLogs(ctx context.Context, req *request.PageIotCardReplaceLogReq, ds *utils.DataScope) ([]model.IotCardReplaceLog, int64, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.IotCardReplaceLog{}).
		Table("ac_iot_card_replace_log AS l")

	if req.RegionID != nil {
		q = q.Where("l.region_id = ?", *req.RegionID)
	}
	if req.DeviceID != nil {
		q = q.Where("l.device_id = ?", *req.DeviceID)
	}
	if s := strings.TrimSpace(req.DeviceCode); s != "" {
		q = q.Where("l.device_code ILIKE ?", "%"+s+"%")
	}
	if s := strings.TrimSpace(req.CardNo); s != "" {
		like := "%" + s + "%"
		q = q.Where("(l.old_card_no ILIKE ? OR l.new_card_no ILIKE ?)", like, like)
	}
	if s := strings.TrimSpace(req.StartDT); s != "" {
		q = q.Where("l.replace_dt >= ?", s)
	}
	if s := strings.TrimSpace(req.EndDT); s != "" {
		q = q.Where("l.replace_dt <= ?", s)
	}
	q = ds.ApplyToQuery(q, "l.dept_id")

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.IotCardReplaceLog
	err := q.Session(&gorm.Session{}).
		Preload("Device").
		Preload("Region").
		Order("l.replace_dt DESC, l.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
