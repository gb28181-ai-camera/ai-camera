package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// UserRepo 用户数据访问层。表 ac_user。
type UserRepo struct{}

// NewUserRepo 构造用户仓储。
func NewUserRepo() *UserRepo { return &UserRepo{} }

// EffectivePermission 用户的有效数据权限（直接/岗位/部门三路角色合并后的最高权限）。
type EffectivePermission struct {
	DataPerm     string // all / cross_dept / dept / personal
	CrossDeptIDs string // 合并去重后的跨部门ID列表（逗号分隔，仅 cross_dept 时非空）
}

// permPriority 数据权限优先级：personal(0) < dept(1) < cross_dept(2) < all(3)。
// 用户的有效数据权限取所有来源角色中的最高者。
var permPriority = map[string]int{
	"personal":   0,
	"dept":       1,
	"cross_dept": 2,
	"all":        3,
}

// ────────────────────────────────────────────────────────────────
// 查询
// ────────────────────────────────────────────────────────────────

// Page 分页查询用户，携带部门/岗位/角色名称。
func (r *UserRepo) Page(ctx context.Context, req *request.PageUserReq, ds *utils.DataScope) ([]model.User, int64, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.User{})

	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		q = q.Where(
			"ac_user.name ILIKE ? OR ac_user.user_name ILIKE ? OR ac_user.phone ILIKE ?",
			kw, kw, kw,
		)
	}
	if req.Active != nil {
		q = q.Where("ac_user.active = ?", *req.Active)
	}
	if req.DeptID != nil {
		q = q.Where("ac_user.dept_id = ?", *req.DeptID)
	}
	if req.PostID != nil {
		q = q.Where("ac_user.post_id = ?", *req.PostID)
	}
	if ds != nil {
		q = ds.ApplyToQuery(q, "ac_user.dept_id")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.User
	err := q.Preload("Dept").Preload("Post").Preload("Roles").
		Order("ac_user.id ASC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Options 返回启用用户的下拉选项数据源。
func (r *UserRepo) Options(ctx context.Context) ([]model.User, error) {
	var items []model.User
	err := bootstrap.DB().WithContext(ctx).
		Where("ac_user.active = ?", true).
		Order("ac_user.id ASC").
		Find(&items).Error
	return items, err
}

// GetByID 按主键查询用户（含部门/岗位/角色）。
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var m model.User
	err := bootstrap.DB().WithContext(ctx).
		Preload("Dept").Preload("Post").Preload("Roles").
		First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByUsername 按登录名查询用户（登录校验用，不预加载关联）。
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var m model.User
	err := bootstrap.DB().WithContext(ctx).
		Where("ac_user.user_name = ?", username).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ────────────────────────────────────────────────────────────────
// 写入
// ────────────────────────────────────────────────────────────────

// Create 新增用户（不处理关联关系）。
func (r *UserRepo) Create(ctx context.Context, m *model.User) error {
	return bootstrap.DB().WithContext(ctx).
		Omit("Dept", "Post", "Roles").
		Create(m).Error
}

// Update 全量更新用户字段。调用前应先 GetByID 载入完整记录，
// 以保证 created_by / created_dt / password_hash 等未被覆盖。
func (r *UserRepo) Update(ctx context.Context, m *model.User) error {
	// 清空已加载的关联，避免 Save 时联动写关联表
	m.Dept = nil
	m.Post = nil
	m.Roles = nil
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除用户，并清理角色/岗位关联。
func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.User{}, id).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&model.UserRoleRel{}).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ?", id).Delete(&model.UserPostRel{}).Error
	})
}

// SetActive 启用/禁用用户。
func (r *UserRepo) SetActive(ctx context.Context, id int64, active bool) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"active":     active,
			"updated_by": model.UsernameFromCtx(ctx),
			"updated_dt": time.Now(),
		}).Error
}

// UpdatePassword 更新密码哈希。
func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, hash string) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"password_hash": hash,
			"updated_by":    model.UsernameFromCtx(ctx),
			"updated_dt":    time.Now(),
		}).Error
}

// UpdateLastLogin 更新最后登录时间，并把审计字段刷新为登录名。
//
// 登录接口无 Auth 中间件写入的上下文，故 updated_by 由调用方显式传入。
func (r *UserRepo) UpdateLastLogin(ctx context.Context, id int64, t time.Time, operator string) error {
	updates := map[string]interface{}{
		"last_login": t,
		"updated_dt": time.Now(),
	}
	if operator != "" {
		updates["updated_by"] = operator
	}
	return bootstrap.DB().WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ────────────────────────────────────────────────────────────────
// 用户 ↔ 角色 / 岗位 关联
// ────────────────────────────────────────────────────────────────

// GetRoleIDs 用户直接关联的角色 ID。
func (r *UserRepo) GetRoleIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.UserRoleRel{}).
		Where("user_id = ?", userID).
		Pluck("role_id", &ids).Error
	return ids, err
}

// SetRoleIDs 覆盖式写入用户直接关联的角色。
func (r *UserRepo) SetRoleIDs(ctx context.Context, userID int64, roleIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRoleRel{}).Error; err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(roleIDs))
		for _, roleID := range roleIDs {
			if roleID <= 0 {
				continue
			}
			if _, ok := seen[roleID]; ok {
				continue
			}
			seen[roleID] = struct{}{}
			if err := tx.Create(&model.UserRoleRel{UserID: userID, RoleID: roleID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetDirectRoles 用户**直接**关联的角色（不含岗位/部门继承），按 sequence 升序。
func (r *UserRepo) GetDirectRoles(ctx context.Context, userID int64) ([]model.Role, error) {
	var roles []model.Role
	err := bootstrap.DB().WithContext(ctx).
		Joins("JOIN ac_user_role_rel ur ON ur.role_id = ac_role.id").
		Where("ur.user_id = ?", userID).
		Order("ac_role.sequence ASC, ac_role.id ASC").
		Find(&roles).Error
	return roles, err
}

// SetPostIDs 覆盖式写入用户岗位关联（ac_user_post_rel），首个岗位为主岗位。
func (r *UserRepo) SetPostIDs(ctx context.Context, userID int64, postIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserPostRel{}).Error; err != nil {
			return err
		}
		seen := make(map[int64]struct{}, len(postIDs))
		for i, postID := range postIDs {
			if postID <= 0 {
				continue
			}
			if _, ok := seen[postID]; ok {
				continue
			}
			seen[postID] = struct{}{}
			rel := model.UserPostRel{UserID: userID, PostID: postID, IsPrimary: i == 0}
			if err := tx.Create(&rel).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ────────────────────────────────────────────────────────────────
// 有效角色与权限（直接 / 岗位 / 部门 三路并集）
// ────────────────────────────────────────────────────────────────

// effectiveRoleCond 三路角色来源的 UNION 子查询（参数顺序：userID, userID, userID）。
const effectiveRoleUnion = `
	SELECT ur.role_id FROM ac_user_role_rel ur WHERE ur.user_id = ?
	UNION
	SELECT pr.role_id FROM ac_post_role_rel pr
	JOIN ac_user u ON u.post_id = pr.post_id
	WHERE u.id = ? AND u.deleted_at IS NULL
	UNION
	SELECT dr.role_id FROM ac_dept_role_rel dr
	JOIN ac_user u ON u.dept_id = dr.dept_id
	WHERE u.id = ? AND u.deleted_at IS NULL
`

// GetEffectiveRoles 用户的有效角色（直接关联 + 岗位继承 + 部门继承），按 sequence 升序。
//
// 多路来源可能命中同一角色，故用 DISTINCT 去重。
func (r *UserRepo) GetEffectiveRoles(ctx context.Context, userID int64) ([]model.Role, error) {
	var roles []model.Role
	err := bootstrap.DB().WithContext(ctx).Raw(`
		SELECT DISTINCT r.* FROM ac_role r
		WHERE r.deleted_at IS NULL
		  AND COALESCE(r.active, TRUE) = TRUE
		  AND r.id IN (`+effectiveRoleUnion+`)
		ORDER BY r.sequence ASC, r.id ASC
	`, userID, userID, userID).Scan(&roles).Error
	if err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []model.Role{}
	}
	return roles, nil
}

// GetPrimaryRoleCode 取用户有效角色中 sequence 最小的角色编码，无有效角色时返回空串。
func (r *UserRepo) GetPrimaryRoleCode(ctx context.Context, userID int64) (string, error) {
	roles, err := r.GetEffectiveRoles(ctx, userID)
	if err != nil {
		return "", err
	}
	if len(roles) == 0 {
		return "", nil
	}
	return roles[0].Code, nil
}

// GetUserEffectiveDataPermission 计算用户的有效数据权限。
//
// 规则：
//   - is_admin = true 直接返回 all；
//   - 否则汇总三路角色（直接/岗位/部门）的 data_permission，按
//     personal < dept < cross_dept < all 取最高；
//   - 结果为 cross_dept 时合并去重各角色的 cross_dept_ids。
func (r *UserRepo) GetUserEffectiveDataPermission(ctx context.Context, userID int64) *EffectivePermission {
	var user model.User
	if err := bootstrap.DB().WithContext(ctx).First(&user, userID).Error; err != nil {
		return &EffectivePermission{DataPerm: "personal"}
	}
	if user.IsAdmin != nil && *user.IsAdmin {
		return &EffectivePermission{DataPerm: "all"}
	}

	roles, err := r.GetEffectiveRoles(ctx, userID)
	if err != nil || len(roles) == 0 {
		return &EffectivePermission{DataPerm: "personal"}
	}

	highest := "personal"
	var allCross []string
	for _, role := range roles {
		if permPriority[role.DataPermission] > permPriority[highest] {
			highest = role.DataPermission
		}
		if role.DataPermission == "cross_dept" && role.CrossDeptIDs != "" {
			allCross = append(allCross, strings.Split(role.CrossDeptIDs, ",")...)
		}
	}

	crossDeptIDs := ""
	if highest == "cross_dept" && len(allCross) > 0 {
		seen := make(map[string]struct{}, len(allCross))
		unique := make([]string, 0, len(allCross))
		for _, id := range allCross {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
		crossDeptIDs = strings.Join(unique, ",")
	}

	return &EffectivePermission{DataPerm: highest, CrossDeptIDs: crossDeptIDs}
}

// GetUserEffectivePermissions 用户可见的接口权限集合，元素形如 "METHOD:路径"。
//
// 由三路角色编码在 casbin_rule 中反查 p 策略（v0=角色, v1=路径, v2=方法）。
// 注意：策略类型列名为 p_type（gorm-adapter 的 CasbinRule.PType 经 GORM 命名策略
// 映射为 p_type，见 ddl/08-权限策略表.sql）。写成 ptype 会导致查询静默返回空集合。
func (r *UserRepo) GetUserEffectivePermissions(ctx context.Context, userID int64) []string {
	perms := []string{}
	_ = bootstrap.DB().WithContext(ctx).Raw(`
		SELECT DISTINCT cr.v2 || ':' || cr.v1 FROM casbin_rule cr
		WHERE cr.p_type = 'p' AND cr.v0 IN (
			SELECT r.code FROM ac_role r
			WHERE r.deleted_at IS NULL AND r.id IN (`+effectiveRoleUnion+`)
		)
	`, userID, userID, userID).Scan(&perms).Error
	return perms
}

// GetAssignedMenuIDs 用户三路角色已分配的菜单 ID 并集。
func (r *UserRepo) GetAssignedMenuIDs(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Raw(`
		SELECT DISTINCT rmr.menu_id FROM ac_role_menu_rel rmr
		WHERE rmr.role_id IN (`+effectiveRoleUnion+`)
	`, userID, userID, userID).Scan(&ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// GetMenusByIDs 按 ID 集合查询启用菜单（可限定平台），按 sequence 升序返回。
func (r *UserRepo) GetMenusByIDs(ctx context.Context, ids []int64, platform string) ([]model.Menu, error) {
	if len(ids) == 0 {
		return []model.Menu{}, nil
	}
	q := bootstrap.DB().WithContext(ctx).Model(&model.Menu{}).
		Where("ac_menu.id IN ?", ids).
		Where("COALESCE(ac_menu.active, TRUE) = TRUE")
	if platform != "" {
		q = q.Where("ac_menu.platform = ?", platform)
	}
	var menus []model.Menu
	if err := q.Order("ac_menu.sequence ASC, ac_menu.id ASC").Find(&menus).Error; err != nil {
		return nil, err
	}
	if menus == nil {
		menus = []model.Menu{}
	}
	return menus, nil
}
