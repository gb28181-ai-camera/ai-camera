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

// RiskTypeRepo 风险类型数据访问。表 ac_risk_type（含 dept_id，需施加数据权限）。
type RiskTypeRepo struct{}

// NewRiskTypeRepo 构造风险类型仓储。
func NewRiskTypeRepo() *RiskTypeRepo { return &RiskTypeRepo{} }

// Page 分页查询风险类型，keyword 模糊匹配名称/编码，level 精确过滤，按 sequence 升序。
func (r *RiskTypeRepo) Page(ctx context.Context, req *request.PageRiskTypeReq, ds *utils.DataScope) ([]model.RiskType, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.RiskType{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("name LIKE ? OR code LIKE ?", like, like)
		}
		if lv := strings.TrimSpace(req.Level); lv != "" {
			q = q.Where("level = ?", lv)
		}
		return ds.ApplyToQuery(q, "ac_risk_type.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.RiskType
	err := build().Order("sequence ASC, id ASC").
		Offset(req.Offset()).Limit(req.PageSize).Find(&list).Error
	return list, total, err
}

// Options 返回启用状态的风险类型选项，按 sequence 升序。
func (r *RiskTypeRepo) Options(ctx context.Context, ds *utils.DataScope) ([]model.RiskType, error) {
	var list []model.RiskType
	q := bootstrap.DB().WithContext(ctx).Model(&model.RiskType{}).
		Where("active = ?", true)
	q = ds.ApplyToQuery(q, "ac_risk_type.dept_id")
	err := q.Order("sequence ASC, id ASC").Find(&list).Error
	return list, err
}

// GetByID 按主键查询风险类型（含已停用）。
func (r *RiskTypeRepo) GetByID(ctx context.Context, id int64) (*model.RiskType, error) {
	var m model.RiskType
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增风险类型。
func (r *RiskTypeRepo) Create(ctx context.Context, m *model.RiskType) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 保存风险类型（整行更新，允许把字段置空）。
func (r *RiskTypeRepo) Update(ctx context.Context, m *model.RiskType) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除风险类型。
func (r *RiskTypeRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.RiskType{}, id).Error
}

// CountRisks 统计引用该风险类型的风险记录数（删除前引用校验）。
func (r *RiskTypeRepo) CountRisks(ctx context.Context, riskTypeID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.RiskRecord{}).
		Where("risk_type_id = ?", riskTypeID).Count(&n).Error
	return n, err
}

// CountVoiceConfigs 统计引用该风险类型的语音配置数（删除前引用校验）。
func (r *RiskTypeRepo) CountVoiceConfigs(ctx context.Context, riskTypeID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.VoiceConfig{}).
		Where("risk_type_id = ?", riskTypeID).Count(&n).Error
	return n, err
}

// CountByColumn 统计指定表中引用该风险类型的记录数（删除前引用校验的通用实现）。
// table/column 为代码内硬编码常量，不来自请求参数。
func (r *RiskTypeRepo) CountByColumn(ctx context.Context, table, column string, riskTypeID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).
		Table(table).
		Where(column+" = ?", riskTypeID).
		Where("deleted_at IS NULL").
		Count(&n).Error
	return n, err
}
