package system

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// validDataPermission 校验数据权限取值是否合法（与 model.DataScopeOption 一致）。
func validDataPermission(v string) bool {
	for _, opt := range model.DataScopeOption {
		if opt == v {
			return true
		}
	}
	return false
}

// RoleService 角色与授权管理服务。
type RoleService struct{}

// NewRoleService 构造角色服务。
func NewRoleService() *RoleService { return &RoleService{} }

// Page 分页查询角色。
func (s *RoleService) Page(ctx context.Context, req *request.PageRoleReq) (*response.PageRoleResp, error) {
	req.Normalize()

	items, total, err := repository.NewRoleRepo().Page(ctx, req)
	if err != nil {
		return nil, err
	}

	return &response.PageRoleResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// Options 返回角色下拉选项（仅启用角色，Code 为 Casbin 角色编码）。
func (s *RoleService) Options(ctx context.Context) ([]response.OptionItem, error) {
	roles, err := repository.NewRoleRepo().Options(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]response.OptionItem, 0, len(roles))
	for _, r := range roles {
		items = append(items, response.OptionItem{ID: r.ID, Name: r.Name, Code: r.Code})
	}
	return items, nil
}

// Get 查询角色详情（含已授权菜单）。
func (s *RoleService) Get(ctx context.Context, id int64) (*model.Role, error) {
	return repository.NewRoleRepo().GetByID(ctx, id)
}

// Create 新增角色，并按需授权菜单后重建 Casbin 策略。
func (s *RoleService) Create(ctx context.Context, req *request.CreateRoleReq) (*model.Role, error) {
	roleRepo := repository.NewRoleRepo()

	if existing, _ := roleRepo.GetByCode(ctx, req.Code); existing != nil {
		return nil, fmt.Errorf("角色编码 %s 已存在", req.Code)
	}

	sequence := req.Sequence
	if sequence <= 0 {
		sequence = 10
	}
	dataPerm := req.DataPermission
	if dataPerm == "" {
		dataPerm = "all"
	}
	if !validDataPermission(dataPerm) {
		return nil, fmt.Errorf("非法的数据权限: %s", dataPerm)
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	role := &model.Role{
		Name:           req.Name,
		Code:           req.Code,
		Sequence:       sequence,
		Description:    req.Description,
		DataPermission: dataPerm,
		CrossDeptIDs:   req.CrossDeptIDs,
		Active:         &active,
	}

	if err := roleRepo.Create(ctx, role); err != nil {
		return nil, err
	}

	if len(req.MenuIDs) > 0 {
		if err := roleRepo.SetMenuIDs(ctx, role.ID, req.MenuIDs); err != nil {
			return nil, err
		}
	}
	_ = syncCasbinForRole(ctx, role.ID)

	return roleRepo.GetByID(ctx, role.ID)
}

// Update 更新角色信息。字符串为空表示不修改；MenuIDs 为 nil 表示不调整菜单授权。
func (s *RoleService) Update(ctx context.Context, id int64, req *request.UpdateRoleReq) error {
	roleRepo := repository.NewRoleRepo()

	role, err := roleRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("角色不存在")
	}

	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Code != "" && req.Code != role.Code {
		if existing, _ := roleRepo.GetByCode(ctx, req.Code); existing != nil && existing.ID != id {
			return fmt.Errorf("角色编码 %s 已存在", req.Code)
		}
		// 角色编码是 Casbin 的 sub，变更前先清理旧编码下的全部策略
		removeCasbinPoliciesByRole(role.Code)
		role.Code = req.Code
	}
	if req.Sequence > 0 {
		role.Sequence = req.Sequence
	}
	role.Description = req.Description
	if req.DataPermission != "" {
		if !validDataPermission(req.DataPermission) {
			return fmt.Errorf("非法的数据权限: %s", req.DataPermission)
		}
		role.DataPermission = req.DataPermission
	}
	role.CrossDeptIDs = req.CrossDeptIDs
	if req.Active != nil {
		role.Active = req.Active
	}

	if err := roleRepo.Update(ctx, role); err != nil {
		return err
	}

	if req.MenuIDs != nil {
		if err := roleRepo.SetMenuIDs(ctx, id, req.MenuIDs); err != nil {
			return err
		}
	}

	return syncCasbinForRole(ctx, id)
}

// Delete 软删除角色，清理全部关联关系与 Casbin 策略。
func (s *RoleService) Delete(ctx context.Context, id int64) error {
	roleRepo := repository.NewRoleRepo()

	role, err := roleRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("角色不存在")
	}

	if err := roleRepo.Delete(ctx, id); err != nil {
		return err
	}
	removeCasbinPoliciesByRole(role.Code)

	return nil
}

// ────────────────────────────────────────────────────────────────
// 授权关系：菜单 / 部门 / 岗位 / 用户
// ────────────────────────────────────────────────────────────────

// GetMenus 角色已授权的菜单 ID。
func (s *RoleService) GetMenus(ctx context.Context, roleID int64) ([]int64, error) {
	return repository.NewRoleRepo().GetMenuIDs(ctx, roleID)
}

// AssignMenus 覆盖式授权菜单，并重建 Casbin 策略。
func (s *RoleService) AssignMenus(ctx context.Context, roleID int64, menuIDs []int64) error {
	roleRepo := repository.NewRoleRepo()
	if _, err := roleRepo.GetByID(ctx, roleID); err != nil {
		return errors.New("角色不存在")
	}
	if err := roleRepo.SetMenuIDs(ctx, roleID, menuIDs); err != nil {
		return err
	}
	return syncCasbinForRole(ctx, roleID)
}

// GetDepts 角色关联的部门 ID。
func (s *RoleService) GetDepts(ctx context.Context, roleID int64) ([]int64, error) {
	return repository.NewRoleRepo().GetDeptIDs(ctx, roleID)
}

// AssignDepts 覆盖式关联部门，并重建 Casbin 策略（g2）。
func (s *RoleService) AssignDepts(ctx context.Context, roleID int64, deptIDs []int64) error {
	roleRepo := repository.NewRoleRepo()
	if _, err := roleRepo.GetByID(ctx, roleID); err != nil {
		return errors.New("角色不存在")
	}
	if err := roleRepo.SetDeptIDs(ctx, roleID, deptIDs); err != nil {
		return err
	}
	return syncCasbinForRole(ctx, roleID)
}

// GetPosts 角色关联的岗位 ID。
func (s *RoleService) GetPosts(ctx context.Context, roleID int64) ([]int64, error) {
	return repository.NewRoleRepo().GetPostIDs(ctx, roleID)
}

// AssignPosts 覆盖式关联岗位，并重建 Casbin 策略（g3）。
func (s *RoleService) AssignPosts(ctx context.Context, roleID int64, postIDs []int64) error {
	roleRepo := repository.NewRoleRepo()
	if _, err := roleRepo.GetByID(ctx, roleID); err != nil {
		return errors.New("角色不存在")
	}
	if err := roleRepo.SetPostIDs(ctx, roleID, postIDs); err != nil {
		return err
	}
	return syncCasbinForRole(ctx, roleID)
}

// GetUsers 角色直接关联的用户 ID。
func (s *RoleService) GetUsers(ctx context.Context, roleID int64) ([]int64, error) {
	return repository.NewRoleRepo().GetUserIDs(ctx, roleID)
}

// AssignUsers 覆盖式关联用户，并重建 Casbin 策略（g）。
func (s *RoleService) AssignUsers(ctx context.Context, roleID int64, userIDs []int64) error {
	roleRepo := repository.NewRoleRepo()
	if _, err := roleRepo.GetByID(ctx, roleID); err != nil {
		return errors.New("角色不存在")
	}
	if err := roleRepo.SetUserIDs(ctx, roleID, userIDs); err != nil {
		return err
	}
	return syncCasbinForRole(ctx, roleID)
}

// ────────────────────────────────────────────────────────────────
// Casbin 同步
// ────────────────────────────────────────────────────────────────

// removeCasbinPoliciesByRole 清理某角色编码名下的 p/g/g2/g3 策略（角色删除或改码时使用）。
//
// Casbin 未初始化（降级）时静默跳过。
func removeCasbinPoliciesByRole(roleCode string) {
	cs := bootstrap.CasbinService()
	if cs == nil || roleCode == "" {
		return
	}

	if _, err := cs.RemoveFilteredPolicyByRole(roleCode); err != nil {
		log.Printf("Casbin清理p策略失败 role=%s: %v", roleCode, err)
	}
	if _, err := cs.RemoveFilteredGroupingPolicyByRole(roleCode); err != nil {
		log.Printf("Casbin清理g策略失败 role=%s: %v", roleCode, err)
	}
	if _, err := cs.RemoveFilteredDeptGroupingPolicyByRole(roleCode); err != nil {
		log.Printf("Casbin清理g2策略失败 role=%s: %v", roleCode, err)
	}
	if _, err := cs.RemoveFilteredPostGroupingPolicyByRole(roleCode); err != nil {
		log.Printf("Casbin清理g3策略失败 role=%s: %v", roleCode, err)
	}
}

// syncCasbinForRole 重建某角色的 Casbin 策略，并在角色授权变更后调用。
//
//   - p  ：role.Menus → ac_menu_api，即 (角色编码, 接口路径, HTTP方法)
//   - g  ：该角色下的用户 → 角色编码（直接分配）
//   - g2 ：该角色关联的部门 → 角色编码（部门继承）
//   - g3 ：该角色关联的岗位 → 角色编码（岗位继承）
//
// 采用「先清空再重建」保证与数据库授权关系一致；Casbin 初始化失败降级时直接跳过。
func syncCasbinForRole(ctx context.Context, roleID int64) error {
	cs := bootstrap.CasbinService()
	if cs == nil {
		return nil
	}

	roleRepo := repository.NewRoleRepo()
	role, err := roleRepo.GetByID(ctx, roleID)
	if err != nil {
		return err
	}

	// ── 1. 清除该角色的全部旧策略 ──
	removeCasbinPoliciesByRole(role.Code)

	// ── 2. 重建 p 策略：角色 → 菜单 → 接口 ──
	menuIDs, err := roleRepo.GetMenuIDs(ctx, roleID)
	if err == nil && len(menuIDs) > 0 {
		apis, err := roleRepo.GetAPIsByMenuIDs(ctx, menuIDs)
		if err == nil {
			for _, api := range apis {
				if api.Path == "" || api.Method == "" {
					continue
				}
				if _, err := cs.AddPolicy(role.Code, api.Path, api.Method); err != nil {
					log.Printf("Casbin添加p策略失败 role=%s path=%s: %v", role.Code, api.Path, err)
				}
			}
		}
	}

	// ── 3. 重建 g 策略：用户 → 角色 ──
	if users, err := roleRepo.GetUsers(ctx, roleID); err == nil {
		for _, u := range users {
			if u.Username == "" {
				continue
			}
			if _, err := cs.AddGroupingPolicy(u.Username, role.Code); err != nil {
				log.Printf("Casbin添加g策略失败 user=%s role=%s: %v", u.Username, role.Code, err)
			}
		}
	}

	// ── 4. 重建 g2 策略：部门 → 角色 ──
	if depts, err := roleRepo.GetDepts(ctx, roleID); err == nil {
		for _, d := range depts {
			if d.Code == "" {
				continue
			}
			if _, err := cs.AddDeptGroupingPolicy(d.Code, role.Code); err != nil {
				log.Printf("Casbin添加g2策略失败 dept=%s role=%s: %v", d.Code, role.Code, err)
			}
		}
	}

	// ── 5. 重建 g3 策略：岗位 → 角色 ──
	if posts, err := roleRepo.GetPosts(ctx, roleID); err == nil {
		for _, p := range posts {
			if p.Code == "" {
				continue
			}
			if _, err := cs.AddPostGroupingPolicy(p.Code, role.Code); err != nil {
				log.Printf("Casbin添加g3策略失败 post=%s role=%s: %v", p.Code, role.Code, err)
			}
		}
	}

	return cs.ReloadPolicy()
}

// syncCasbinForUser 按用户**直接**关联的角色重建 g 策略。
//
// oldUsername / oldRoleCodes 用于清理变更前的策略（用户名或角色调整、用户删除）。
// 用户已软删除时仅执行清理。Casbin 降级时静默跳过。
func syncCasbinForUser(ctx context.Context, userID int64, oldUsername string, oldRoleCodes []string) error {
	cs := bootstrap.CasbinService()
	if cs == nil {
		return nil
	}

	if oldUsername != "" {
		for _, code := range oldRoleCodes {
			if code == "" {
				continue
			}
			if _, err := cs.RemoveGroupingPolicy(oldUsername, code); err != nil {
				log.Printf("Casbin清理g策略失败 user=%s role=%s: %v", oldUsername, code, err)
			}
		}
	}

	userRepo := repository.NewUserRepo()
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		// 用户已被删除，清理完成后即可返回
		return cs.ReloadPolicy()
	}

	roles, err := userRepo.GetDirectRoles(ctx, userID)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if r.Code == "" {
			continue
		}
		if _, err := cs.AddGroupingPolicy(user.Username, r.Code); err != nil {
			log.Printf("Casbin添加g策略失败 user=%s role=%s: %v", user.Username, r.Code, err)
		}
	}

	return cs.ReloadPolicy()
}
