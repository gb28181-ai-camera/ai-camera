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

// RegionRepo 区域数据访问。表 ac_region（含 dept_id，需施加数据权限）。
type RegionRepo struct{}

// NewRegionRepo 构造区域仓储。
func NewRegionRepo() *RegionRepo { return &RegionRepo{} }

// Page 分页查询区域，keyword 模糊匹配名称/编码，按 sequence 升序。
func (r *RegionRepo) Page(ctx context.Context, req *request.PageRegionReq, ds *utils.DataScope) ([]model.Region, int64, error) {
	// 分页统计与取数各自构造查询，避免 finisher 方法（Count）污染后续链式条件。
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.Region{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("name LIKE ? OR code LIKE ?", like, like)
		}
		return ds.ApplyToQuery(q, "ac_region.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.Region
	err := build().Order("sequence ASC, id ASC").
		Offset(req.Offset()).Limit(req.PageSize).Find(&list).Error
	return list, total, err
}

// Options 返回启用状态的区域选项，按 sequence 升序。
func (r *RegionRepo) Options(ctx context.Context, ds *utils.DataScope) ([]model.Region, error) {
	var list []model.Region
	q := bootstrap.DB().WithContext(ctx).Model(&model.Region{}).
		Where("active = ?", true)
	q = ds.ApplyToQuery(q, "ac_region.dept_id")
	err := q.Order("sequence ASC, id ASC").Find(&list).Error
	return list, err
}

// GetByID 按主键查询区域（含已停用）。
func (r *RegionRepo) GetByID(ctx context.Context, id int64) (*model.Region, error) {
	var m model.Region
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增区域。
func (r *RegionRepo) Create(ctx context.Context, m *model.Region) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 保存区域（整行更新，允许把字段置空）。
func (r *RegionRepo) Update(ctx context.Context, m *model.Region) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除区域。
func (r *RegionRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.Region{}, id).Error
}

// CountDevices 统计归属该区域的设备数（删除前引用校验）。
func (r *RegionRepo) CountDevices(ctx context.Context, regionID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.Device{}).
		Where("region_id = ?", regionID).Count(&n).Error
	return n, err
}

// CountRisks 统计归属该区域的风险记录数（删除前引用校验）。
func (r *RegionRepo) CountRisks(ctx context.Context, regionID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.RiskRecord{}).
		Where("region_id = ?", regionID).Count(&n).Error
	return n, err
}

// CountByColumn 统计指定表中引用该区域的记录数（删除前引用校验的通用实现）。
//
// 区域被 9 张业务表以外键引用（见需求文档 02-基础信息模块 §5.2），
// 逐表写一个 CountXxx 会重复且容易漏，故用表名 + 列名参数化。
// table/column 均为**代码内硬编码的常量**，不来自请求参数，不存在注入面。
func (r *RegionRepo) CountByColumn(ctx context.Context, table, column string, regionID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).
		Table(table).
		Where(column+" = ?", regionID).
		Where("deleted_at IS NULL").
		Count(&n).Error
	return n, err
}
