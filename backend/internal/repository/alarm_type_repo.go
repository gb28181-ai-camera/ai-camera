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

// AlarmTypeRepo 告警类型数据访问。表 ac_alarm_type（含 dept_id，需施加数据权限）。
type AlarmTypeRepo struct{}

// NewAlarmTypeRepo 构造告警类型仓储。
func NewAlarmTypeRepo() *AlarmTypeRepo { return &AlarmTypeRepo{} }

// Page 分页查询告警类型，keyword 模糊匹配名称/编码，level 精确过滤，按 sequence 升序。
func (r *AlarmTypeRepo) Page(ctx context.Context, req *request.PageAlarmTypeReq, ds *utils.DataScope) ([]model.AlarmType, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.AlarmType{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("name LIKE ? OR code LIKE ?", like, like)
		}
		if lv := strings.TrimSpace(req.Level); lv != "" {
			q = q.Where("level = ?", lv)
		}
		return ds.ApplyToQuery(q, "ac_alarm_type.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.AlarmType
	err := build().Order("sequence ASC, id ASC").
		Offset(req.Offset()).Limit(req.PageSize).Find(&list).Error
	return list, total, err
}

// Options 返回启用状态的告警类型选项，按 sequence 升序。
func (r *AlarmTypeRepo) Options(ctx context.Context, ds *utils.DataScope) ([]model.AlarmType, error) {
	var list []model.AlarmType
	q := bootstrap.DB().WithContext(ctx).Model(&model.AlarmType{}).
		Where("active = ?", true)
	q = ds.ApplyToQuery(q, "ac_alarm_type.dept_id")
	err := q.Order("sequence ASC, id ASC").Find(&list).Error
	return list, err
}

// GetByID 按主键查询告警类型（含已停用）。
func (r *AlarmTypeRepo) GetByID(ctx context.Context, id int64) (*model.AlarmType, error) {
	var m model.AlarmType
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增告警类型。
func (r *AlarmTypeRepo) Create(ctx context.Context, m *model.AlarmType) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 保存告警类型（整行更新，允许把字段置空）。
func (r *AlarmTypeRepo) Update(ctx context.Context, m *model.AlarmType) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除告警类型。
func (r *AlarmTypeRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.AlarmType{}, id).Error
}

// CountAlarms 统计引用该告警类型的告警记录数（删除前引用校验）。
func (r *AlarmTypeRepo) CountAlarms(ctx context.Context, alarmTypeID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.AlarmRecord{}).
		Where("alarm_type_id = ?", alarmTypeID).Count(&n).Error
	return n, err
}

// CountAlarmSettings 统计引用该告警类型的告警设置数（删除前引用校验）。
func (r *AlarmTypeRepo) CountAlarmSettings(ctx context.Context, alarmTypeID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.AlarmSetting{}).
		Where("alarm_type_id = ?", alarmTypeID).Count(&n).Error
	return n, err
}
