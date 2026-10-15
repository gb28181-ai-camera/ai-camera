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
// 设备模式数据访问
// ════════════════════════════════════════════════════════════════

// DeviceModeRepo 设备模式数据访问层。
type DeviceModeRepo struct{}

// NewDeviceModeRepo 构建设备模式仓储。
func NewDeviceModeRepo() *DeviceModeRepo { return &DeviceModeRepo{} }

// deviceModeQuery 构造带表别名 m 的设备模式查询基座。
func deviceModeQuery() *gorm.DB {
	return bootstrap.DB().Model(&model.DeviceMode{}).Table("ac_device_mode AS m")
}

// Page 分页查询设备模式。
func (r *DeviceModeRepo) Page(ctx context.Context, req *request.PageDeviceModeReq, ds *utils.DataScope) ([]model.DeviceMode, int64, error) {
	q := deviceModeQuery().WithContext(ctx)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(m.name ILIKE ? OR m.code ILIKE ?)", like, like)
	}
	q = ds.ApplyToQuery(q, "m.dept_id")

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.DeviceMode
	err := q.Session(&gorm.Session{}).
		Order("m.sequence ASC, m.id ASC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Options 设备模式下拉选项（仅启用状态）。
func (r *DeviceModeRepo) Options(ctx context.Context, ds *utils.DataScope) ([]model.DeviceMode, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.DeviceMode{}).
		Table("ac_device_mode AS m").
		Select("m.id, m.name, m.code").
		Where("(m.active IS NULL OR m.active = true)")
	q = ds.ApplyToQuery(q, "m.dept_id")

	var items []model.DeviceMode
	if err := q.Order("m.sequence ASC, m.id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询设备模式。
func (r *DeviceModeRepo) GetByID(ctx context.Context, id int64) (*model.DeviceMode, error) {
	var m model.DeviceMode
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsByName 判断模式名称是否存在（excludeID>0 时排除自身）。
func (r *DeviceModeRepo) ExistsByName(ctx context.Context, name string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.DeviceMode{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// Create 新增设备模式。
func (r *DeviceModeRepo) Create(ctx context.Context, m *model.DeviceMode) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存设备模式。
func (r *DeviceModeRepo) Update(ctx context.Context, m *model.DeviceMode) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除设备模式。
func (r *DeviceModeRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.DeviceMode{}, id).Error
}
