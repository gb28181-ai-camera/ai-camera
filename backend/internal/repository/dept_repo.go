package repository

import (
	"context"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// DeptRepo 部门（组织）数据访问。
//
// 部门 Code 是 Casbin g2 主体，删除时需同步清理 ac_dept_role_rel。
type DeptRepo struct{}

// NewDeptRepo 构造部门仓储。
func NewDeptRepo() *DeptRepo { return &DeptRepo{} }

// ListAll 查询全部部门（扁平结构，按层级与顺序升序）。
func (r *DeptRepo) ListAll(ctx context.Context) ([]model.Dept, error) {
	var list []model.Dept
	err := bootstrap.DB().WithContext(ctx).
		Order("level ASC, sequence ASC, id ASC").
		Find(&list).Error
	return list, err
}

// ListActive 查询启用部门（供下拉选项使用）。
func (r *DeptRepo) ListActive(ctx context.Context) ([]model.Dept, error) {
	var list []model.Dept
	err := bootstrap.DB().WithContext(ctx).
		Where("active = ?", true).
		Order("level ASC, sequence ASC, id ASC").
		Find(&list).Error
	return list, err
}

// GetByID 查询单条部门。
func (r *DeptRepo) GetByID(ctx context.Context, id int64) (*model.Dept, error) {
	var d model.Dept
	if err := bootstrap.DB().WithContext(ctx).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// CountChildren 统计直接子部门数量。
func (r *DeptRepo) CountChildren(ctx context.Context, parentID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.Dept{}).
		Where("parent_id = ?", parentID).Count(&n).Error
	return n, err
}

// CountUsers 统计部门下的用户数量（判断部门是否可删除）。
func (r *DeptRepo) CountUsers(ctx context.Context, deptID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.User{}).
		Where("dept_id = ?", deptID).Count(&n).Error
	return n, err
}

// ExistsCode 判断部门编码是否已被占用（excludeID 用于更新时排除自身）。
func (r *DeptRepo) ExistsCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	if code == "" {
		return false, nil
	}
	q := bootstrap.DB().WithContext(ctx).Model(&model.Dept{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// Create 新增部门。
func (r *DeptRepo) Create(ctx context.Context, d *model.Dept) error {
	return bootstrap.DB().WithContext(ctx).Create(d).Error
}

// Update 全字段保存部门。
func (r *DeptRepo) Update(ctx context.Context, d *model.Dept) error {
	return bootstrap.DB().WithContext(ctx).Save(d).Error
}

// UpdateSubtree 批量重算某部门下全部下级的 level / path。
//
// 参数为「旧层级路径前缀 → 新层级路径前缀」与层级差值；仅影响
// path LIKE '<oldPrefix>/%' 的后代节点（dept 自身由调用方单独保存）。
func (r *DeptRepo) UpdateSubtree(ctx context.Context, oldPrefix, newPrefix string, levelDelta int) error {
	if oldPrefix == "" || oldPrefix == newPrefix {
		return nil
	}
	db := bootstrap.DB().WithContext(ctx)

	var list []model.Dept
	if err := db.Where("path LIKE ?", oldPrefix+"/%").Find(&list).Error; err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for i := range list {
			d := list[i]
			newPath := newPrefix + strings.TrimPrefix(d.Path, oldPrefix)
			newLevel := d.Level + levelDelta
			if err := tx.Model(&model.Dept{}).Where("id = ?", d.ID).
				Updates(map[string]interface{}{"path": newPath, "level": newLevel}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteCascade 在事务内删除部门及其角色关联（ac_dept_role_rel）。
func (r *DeptRepo) DeleteCascade(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dept_id = ?", id).Delete(&model.DeptRoleRel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Dept{}, id).Error
	})
}
