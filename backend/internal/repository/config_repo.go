package repository

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// ConfigRepo 系统参数（ac_system_config）数据访问。
type ConfigRepo struct{}

// NewConfigRepo 构造系统参数仓储。
func NewConfigRepo() *ConfigRepo { return &ConfigRepo{} }

// List 查询系统参数列表（非分页）。keyword 模糊匹配参数名称与参数键。
func (r *ConfigRepo) List(ctx context.Context, keyword string) ([]model.SystemConfig, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.SystemConfig{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ?", kw, kw)
	}

	var list []model.SystemConfig
	err := q.Order("group_name ASC, sequence ASC, id ASC").Find(&list).Error
	return list, err
}

// GetByID 查询单条系统参数。
func (r *ConfigRepo) GetByID(ctx context.Context, id int64) (*model.SystemConfig, error) {
	var c model.SystemConfig
	if err := bootstrap.DB().WithContext(ctx).First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ExistsCode 判断参数键是否已被占用（excludeID 用于更新时排除自身）。
func (r *ConfigRepo) ExistsCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	if code == "" {
		return false, nil
	}
	q := bootstrap.DB().WithContext(ctx).Model(&model.SystemConfig{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// Create 新增系统参数。
func (r *ConfigRepo) Create(ctx context.Context, c *model.SystemConfig) error {
	return bootstrap.DB().WithContext(ctx).Create(c).Error
}

// Update 全字段保存系统参数。
func (r *ConfigRepo) Update(ctx context.Context, c *model.SystemConfig) error {
	return bootstrap.DB().WithContext(ctx).Save(c).Error
}

// Delete 删除系统参数（软删除）。
func (r *ConfigRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.SystemConfig{}, id).Error
}

// SetActive 启用/停用系统参数。
func (r *ConfigRepo) SetActive(ctx context.Context, id int64, active bool) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.SystemConfig{}).
		Where("id = ?", id).Update("active", active).Error
}
