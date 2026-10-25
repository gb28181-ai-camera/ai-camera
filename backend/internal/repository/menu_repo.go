package repository

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// MenuRepo 菜单、菜单接口与角色-菜单关系的数据访问。
//
// 菜单接口（ac_menu_api）是 Casbin p 策略的唯一来源，因此本仓储同时提供
// 「角色 → 菜单 → 接口」的关联查询，供 Service 层重建策略使用。
type MenuRepo struct{}

// NewMenuRepo 构造菜单仓储。
func NewMenuRepo() *MenuRepo { return &MenuRepo{} }

// ListByPlatform 查询指定平台的菜单（扁平结构，按层级与顺序升序）。
//
// platform 为 "all" 或空时不过滤平台；includeInactive=false 时只返回启用菜单
// （角色授权树保持该默认行为），管理页传 true 可看到停用菜单以便重新启用。
func (r *MenuRepo) ListByPlatform(ctx context.Context, platform string, includeInactive bool) ([]model.Menu, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.Menu{})
	if platform != "" && platform != "all" {
		q = q.Where("platform = ?", platform)
	}
	if !includeInactive {
		q = q.Where("active = ?", true)
	}

	var list []model.Menu
	err := q.Order("level ASC, sequence ASC, id ASC").Find(&list).Error
	return list, err
}

// ListAll 查询全部菜单（含停用），供角色授权与 Casbin 策略重建使用。
func (r *MenuRepo) ListAll(ctx context.Context) ([]model.Menu, error) {
	var list []model.Menu
	err := bootstrap.DB().WithContext(ctx).
		Order("level ASC, sequence ASC, id ASC").
		Find(&list).Error
	return list, err
}

// GetByID 查询单条菜单（含绑定的接口列表 apis）。
func (r *MenuRepo) GetByID(ctx context.Context, id int64) (*model.Menu, error) {
	var m model.Menu
	if err := bootstrap.DB().WithContext(ctx).Preload("APIs").First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CountChildren 统计直接子菜单数量。
func (r *MenuRepo) CountChildren(ctx context.Context, parentID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.Menu{}).
		Where("parent_id = ?", parentID).Count(&n).Error
	return n, err
}

// Create 新增菜单。
func (r *MenuRepo) Create(ctx context.Context, m *model.Menu) error {
	return bootstrap.DB().WithContext(ctx).Omit("APIs").Create(m).Error
}

// Update 全字段保存菜单（不触碰 apis 关联）。
func (r *MenuRepo) Update(ctx context.Context, m *model.Menu) error {
	return bootstrap.DB().WithContext(ctx).Omit("APIs").Save(m).Error
}

// DeleteCascade 在事务内删除菜单及其绑定接口、角色-菜单关系。
func (r *MenuRepo) DeleteCascade(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_id = ?", id).Delete(&model.MenuAPI{}).Error; err != nil {
			return err
		}
		if err := tx.Where("menu_id = ?", id).Delete(&model.RoleMenuRel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Menu{}, id).Error
	})
}

// ─── 菜单接口 ────────────────────────────────────────────────

// ListAPIs 查询菜单绑定的接口列表。
func (r *MenuRepo) ListAPIs(ctx context.Context, menuID int64) ([]model.MenuAPI, error) {
	var list []model.MenuAPI
	err := bootstrap.DB().WithContext(ctx).
		Where("menu_id = ?", menuID).Order("id ASC").Find(&list).Error
	return list, err
}

// GetAPIByID 查询单条菜单接口。
func (r *MenuRepo) GetAPIByID(ctx context.Context, id int64) (*model.MenuAPI, error) {
	var a model.MenuAPI
	if err := bootstrap.DB().WithContext(ctx).First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateAPI 新增菜单接口。
func (r *MenuRepo) CreateAPI(ctx context.Context, a *model.MenuAPI) error {
	return bootstrap.DB().WithContext(ctx).Create(a).Error
}

// DeleteAPI 删除菜单接口（软删除）。
func (r *MenuRepo) DeleteAPI(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.MenuAPI{}, id).Error
}

// ─── Casbin 策略重建所需的关联查询 ───────────────────────────

// ListRolesByMenuID 查询拥有该菜单的角色（含停用角色，策略与启用状态无关）。
func (r *MenuRepo) ListRolesByMenuID(ctx context.Context, menuID int64) ([]model.Role, error) {
	var roles []model.Role
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.Role{}).
		Select("ac_role.*").
		Joins("JOIN ac_role_menu_rel rel ON rel.role_id = ac_role.id").
		Where("rel.menu_id = ?", menuID).
		Order("ac_role.id ASC").
		Find(&roles).Error
	return roles, err
}

// SetRoleIDsForMenu 覆盖式写入菜单的角色授权（ac_role_menu_rel）。
//
// 先删除该菜单的全部角色关系，再按 roleIDs 重建；roleIDs 由 Service 层去重与校验。
func (r *MenuRepo) SetRoleIDsForMenu(ctx context.Context, menuID int64, roleIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("menu_id = ?", menuID).Delete(&model.RoleMenuRel{}).Error; err != nil {
			return err
		}
		for _, roleID := range roleIDs {
			if roleID <= 0 {
				continue
			}
			if err := tx.Create(&model.RoleMenuRel{RoleID: roleID, MenuID: menuID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListMenuIDsByRoleID 查询角色已授权的菜单 ID。
func (r *MenuRepo) ListMenuIDsByRoleID(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.RoleMenuRel{}).
		Where("role_id = ?", roleID).
		Order("menu_id ASC").
		Pluck("menu_id", &ids).Error
	return ids, err
}

// ListAPIsByMenuIDs 查询一组菜单绑定的全部接口。
func (r *MenuRepo) ListAPIsByMenuIDs(ctx context.Context, menuIDs []int64) ([]model.MenuAPI, error) {
	if len(menuIDs) == 0 {
		return nil, nil
	}
	var list []model.MenuAPI
	err := bootstrap.DB().WithContext(ctx).
		Where("menu_id IN ?", menuIDs).Order("id ASC").Find(&list).Error
	return list, err
}
