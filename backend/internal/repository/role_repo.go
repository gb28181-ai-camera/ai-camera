package repository

import (
	"context"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// RoleRepo 角色数据访问层。表 ac_role。
type RoleRepo struct{}

// NewRoleRepo 构造角色仓储。
func NewRoleRepo() *RoleRepo { return &RoleRepo{} }

// ────────────────────────────────────────────────────────────────
// 查询
// ────────────────────────────────────────────────────────────────

// Page 分页查询角色。ac_role 无 dept_id，不做行级数据权限过滤。
func (r *RoleRepo) Page(ctx context.Context, req *request.PageRoleReq) ([]model.Role, int64, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.Role{})
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		q = q.Where("ac_role.name ILIKE ? OR ac_role.code ILIKE ?", kw, kw)
	}
	if req.Active != nil {
		q = q.Where("ac_role.active = ?", *req.Active)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Role
	err := q.Order("ac_role.sequence ASC, ac_role.id ASC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Options 返回启用角色的下拉选项数据源。
func (r *RoleRepo) Options(ctx context.Context) ([]model.Role, error) {
	var items []model.Role
	err := bootstrap.DB().WithContext(ctx).
		Where("ac_role.active = ?", true).
		Order("ac_role.sequence ASC, ac_role.id ASC").
		Find(&items).Error
	return items, err
}

// GetByID 按主键查询角色（含已授权菜单）。
func (r *RoleRepo) GetByID(ctx context.Context, id int64) (*model.Role, error) {
	var m model.Role
	err := bootstrap.DB().WithContext(ctx).Preload("Menus").First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByCode 按角色编码查询（编码即 Casbin 的 sub，必须唯一）。
func (r *RoleRepo) GetByCode(ctx context.Context, code string) (*model.Role, error) {
	var m model.Role
	err := bootstrap.DB().WithContext(ctx).Where("ac_role.code = ?", code).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ────────────────────────────────────────────────────────────────
// 写入
// ────────────────────────────────────────────────────────────────

// Create 新增角色。
func (r *RoleRepo) Create(ctx context.Context, m *model.Role) error {
	return bootstrap.DB().WithContext(ctx).Omit("Menus").Create(m).Error
}

// Update 全量更新角色。调用前应先 GetByID 载入完整记录。
func (r *RoleRepo) Update(ctx context.Context, m *model.Role) error {
	// 清空已加载的关联，避免 Save 时联动重写角色-菜单关联表
	m.Menus = nil
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除角色，并清理菜单/部门/岗位/用户关联。
func (r *RoleRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.Role{}, id).Error; err != nil {
			return err
		}
		for _, rel := range []interface{}{
			&model.RoleMenuRel{},
			&model.DeptRoleRel{},
			&model.PostRoleRel{},
			&model.UserRoleRel{},
		} {
			if err := tx.Where("role_id = ?", id).Delete(rel).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SetActive 启用/禁用角色。
func (r *RoleRepo) SetActive(ctx context.Context, id int64, active bool) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.Role{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"active":     active,
			"updated_by": model.UsernameFromCtx(ctx),
			"updated_dt": time.Now(),
		}).Error
}

// ────────────────────────────────────────────────────────────────
// 关联查询：菜单 / 部门 / 岗位 / 用户
// ────────────────────────────────────────────────────────────────

// GetMenuIDs 角色已授权的菜单 ID。
func (r *RoleRepo) GetMenuIDs(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.RoleMenuRel{}).
		Where("role_id = ?", roleID).
		Order("menu_id ASC").
		Pluck("menu_id", &ids).Error
	return ids, err
}

// SetMenuIDs 覆盖式写入角色菜单授权。
func (r *RoleRepo) SetMenuIDs(ctx context.Context, roleID int64, menuIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RoleMenuRel{}).Error; err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(menuIDs))
		for _, menuID := range menuIDs {
			if menuID <= 0 {
				continue
			}
			if _, ok := seen[menuID]; ok {
				continue
			}
			seen[menuID] = struct{}{}
			if err := tx.Create(&model.RoleMenuRel{RoleID: roleID, MenuID: menuID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetDeptIDs 角色关联的部门 ID（数据权限 g2 来源）。
func (r *RoleRepo) GetDeptIDs(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.DeptRoleRel{}).
		Where("role_id = ?", roleID).
		Order("dept_id ASC").
		Pluck("dept_id", &ids).Error
	return ids, err
}

// SetDeptIDs 覆盖式写入角色关联部门。
func (r *RoleRepo) SetDeptIDs(ctx context.Context, roleID int64, deptIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.DeptRoleRel{}).Error; err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(deptIDs))
		for _, deptID := range deptIDs {
			if deptID <= 0 {
				continue
			}
			if _, ok := seen[deptID]; ok {
				continue
			}
			seen[deptID] = struct{}{}
			if err := tx.Create(&model.DeptRoleRel{DeptID: deptID, RoleID: roleID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetPostIDs 角色关联的岗位 ID（数据权限 g3 来源）。
func (r *RoleRepo) GetPostIDs(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.PostRoleRel{}).
		Where("role_id = ?", roleID).
		Order("post_id ASC").
		Pluck("post_id", &ids).Error
	return ids, err
}

// SetPostIDs 覆盖式写入角色关联岗位。
func (r *RoleRepo) SetPostIDs(ctx context.Context, roleID int64, postIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.PostRoleRel{}).Error; err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(postIDs))
		for _, postID := range postIDs {
			if postID <= 0 {
				continue
			}
			if _, ok := seen[postID]; ok {
				continue
			}
			seen[postID] = struct{}{}
			if err := tx.Create(&model.PostRoleRel{PostID: postID, RoleID: roleID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetUserIDs 角色直接关联的用户 ID。
func (r *RoleRepo) GetUserIDs(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.UserRoleRel{}).
		Where("role_id = ?", roleID).
		Order("user_id ASC").
		Pluck("user_id", &ids).Error
	return ids, err
}

// SetUserIDs 覆盖式写入角色关联用户。
func (r *RoleRepo) SetUserIDs(ctx context.Context, roleID int64, userIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.UserRoleRel{}).Error; err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(userIDs))
		for _, userID := range userIDs {
			if userID <= 0 {
				continue
			}
			if _, ok := seen[userID]; ok {
				continue
			}
			seen[userID] = struct{}{}
			if err := tx.Create(&model.UserRoleRel{UserID: userID, RoleID: roleID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetUsers 角色直接关联的用户（Casbin g 策略重建用，取登录名）。
func (r *RoleRepo) GetUsers(ctx context.Context, roleID int64) ([]model.User, error) {
	var users []model.User
	err := bootstrap.DB().WithContext(ctx).
		Joins("JOIN ac_user_role_rel ur ON ur.user_id = ac_user.id").
		Where("ur.role_id = ?", roleID).
		Order("ac_user.id ASC").
		Find(&users).Error
	return users, err
}

// GetDepts 角色关联的部门（Casbin g2 策略重建用，取部门编码）。
func (r *RoleRepo) GetDepts(ctx context.Context, roleID int64) ([]model.Dept, error) {
	var depts []model.Dept
	err := bootstrap.DB().WithContext(ctx).
		Joins("JOIN ac_dept_role_rel dr ON dr.dept_id = ac_dept.id").
		Where("dr.role_id = ?", roleID).
		Order("ac_dept.id ASC").
		Find(&depts).Error
	return depts, err
}

// GetPosts 角色关联的岗位（Casbin g3 策略重建用，取岗位编码）。
func (r *RoleRepo) GetPosts(ctx context.Context, roleID int64) ([]model.Post, error) {
	var posts []model.Post
	err := bootstrap.DB().WithContext(ctx).
		Joins("JOIN ac_post_role_rel pr ON pr.post_id = ac_post.id").
		Where("pr.role_id = ?", roleID).
		Order("ac_post.id ASC").
		Find(&posts).Error
	return posts, err
}

// GetAPIsByMenuIDs 菜单 ID 集合绑定的后端接口（Casbin p 策略重建用）。
func (r *RoleRepo) GetAPIsByMenuIDs(ctx context.Context, menuIDs []int64) ([]model.MenuAPI, error) {
	if len(menuIDs) == 0 {
		return []model.MenuAPI{}, nil
	}
	var apis []model.MenuAPI
	err := bootstrap.DB().WithContext(ctx).
		Where("ac_menu_api.menu_id IN ?", menuIDs).
		Order("ac_menu_api.id ASC").
		Find(&apis).Error
	return apis, err
}
