package system

import (
	"context"
	"errors"
	"fmt"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// defaultPlatform 菜单查询缺省平台。
const defaultPlatform = "pc"

// maxTreeDepth 树的构造深度上限，防止脏数据形成环导致无限递归。
const maxTreeDepth = 20

// ────────────────────────────────────────────────────────────────
// 包内共用的错误约定（系统管理各 Service 共用）
// ────────────────────────────────────────────────────────────────

// BizError 业务校验错误。Handler 依据该类型把响应码映射为 400。
type BizError struct{ Msg string }

// Error 实现 error 接口。
func (e *BizError) Error() string { return e.Msg }

// bizErr 构造业务校验错误。
func bizErr(format string, args ...interface{}) error {
	return &BizError{Msg: fmt.Sprintf(format, args...)}
}

// IsNotFound 判断错误是否为「记录不存在」。
func IsNotFound(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

// ────────────────────────────────────────────────────────────────
// MenuService
// ────────────────────────────────────────────────────────────────

// MenuService 菜单与菜单接口业务逻辑。
type MenuService struct{}

// NewMenuService 构造菜单服务。
func NewMenuService() *MenuService { return &MenuService{} }

// Tree 返回指定平台的菜单树（仅启用菜单），platform 为空时默认 pc。
//
// 全部菜单一次性查出后在内存按 parent_id 组装，children 按 sequence 升序；
// 父节点不在结果集内（例如父菜单已停用）的节点按根节点返回，避免丢节点。
func (s *MenuService) Tree(ctx context.Context, platform string) ([]model.Menu, error) {
	if platform == "" {
		platform = defaultPlatform
	}
	list, err := repository.NewMenuRepo().ListByPlatform(ctx, platform)
	if err != nil {
		return nil, err
	}
	return buildMenuTree(list), nil
}

// Get 查询菜单详情（含绑定的接口列表）。
func (s *MenuService) Get(ctx context.Context, id int64) (*model.Menu, error) {
	return repository.NewMenuRepo().GetByID(ctx, id)
}

// Create 新增菜单，层级 level 依据父菜单自动推导（根为 1）。
func (s *MenuService) Create(ctx context.Context, req *request.CreateMenuReq) (*model.Menu, error) {
	repo := repository.NewMenuRepo()

	active := true
	if req.Active != nil {
		active = *req.Active
	}
	hidden := false
	if req.Hidden != nil {
		hidden = *req.Hidden
	}

	m := &model.Menu{
		Name:           req.Name,
		Path:           req.Path,
		Icon:           req.Icon,
		Component:      req.Component,
		Type:           req.Type,
		Platform:       req.Platform,
		PermissionCode: req.PermissionCode,
		Description:    req.Description,
		Sequence:       req.Sequence,
		ParentID:       req.ParentID,
		Active:         &active,
		Hidden:         &hidden,
	}
	if m.Type == "" {
		m.Type = "menu"
	}
	if m.Platform == "" {
		m.Platform = defaultPlatform
	}
	if m.Sequence == 0 {
		m.Sequence = 10
	}

	level, err := s.resolveLevel(ctx, req.ParentID)
	if err != nil {
		return nil, err
	}
	m.Level = level

	if err := repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 更新菜单。未传（零值）的字段不覆盖原值；变更父菜单时自动重算 level。
func (s *MenuService) Update(ctx context.Context, id int64, req *request.UpdateMenuReq) error {
	repo := repository.NewMenuRepo()

	m, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if req.Name != "" {
		m.Name = req.Name
	}
	if req.Path != "" {
		m.Path = req.Path
	}
	if req.Icon != "" {
		m.Icon = req.Icon
	}
	if req.Component != "" {
		m.Component = req.Component
	}
	if req.Type != "" {
		m.Type = req.Type
	}
	if req.Platform != "" {
		m.Platform = req.Platform
	}
	if req.PermissionCode != "" {
		m.PermissionCode = req.PermissionCode
	}
	if req.Description != "" {
		m.Description = req.Description
	}
	if req.Sequence != 0 {
		m.Sequence = req.Sequence
	}
	if req.Active != nil {
		m.Active = req.Active
	}
	if req.Hidden != nil {
		m.Hidden = req.Hidden
	}
	if req.ParentID != nil {
		if *req.ParentID == id {
			return bizErr("父菜单不能是菜单自身")
		}
		if err := s.ensureNotDescendant(ctx, id, *req.ParentID); err != nil {
			return err
		}
		level, err := s.resolveLevel(ctx, req.ParentID)
		if err != nil {
			return err
		}
		m.ParentID = req.ParentID
		m.Level = level
	}

	return repo.Update(ctx, m)
}

// Delete 删除菜单。存在子菜单时拒绝删除；删除后清理绑定接口与角色关系，
// 并为受影响角色重建 Casbin 策略。
func (s *MenuService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewMenuRepo()

	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}
	n, err := repo.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return bizErr("该菜单下存在 %d 个子菜单，请先删除子菜单", n)
	}

	roles, err := repo.ListRolesByMenuID(ctx, id)
	if err != nil {
		return err
	}

	if err := repo.DeleteCascade(ctx, id); err != nil {
		return err
	}

	s.syncCasbinForRoles(ctx, roles)
	return nil
}

// ListAPIs 查询菜单绑定的接口列表。
func (s *MenuService) ListAPIs(ctx context.Context, menuID int64) ([]model.MenuAPI, error) {
	repo := repository.NewMenuRepo()
	if _, err := repo.GetByID(ctx, menuID); err != nil {
		return nil, err
	}
	return repo.ListAPIs(ctx, menuID)
}

// CreateAPI 新增菜单接口，并为拥有该菜单的全部角色重建 Casbin 策略。
func (s *MenuService) CreateAPI(ctx context.Context, menuID int64, req *request.CreateMenuAPIReq) (*model.MenuAPI, error) {
	repo := repository.NewMenuRepo()
	if _, err := repo.GetByID(ctx, menuID); err != nil {
		return nil, err
	}

	api := &model.MenuAPI{
		MenuID:      menuID,
		Method:      req.Method,
		Path:        req.Path,
		Description: req.Description,
	}
	if err := repo.CreateAPI(ctx, api); err != nil {
		return nil, err
	}

	s.syncCasbinForMenu(ctx, menuID)
	return api, nil
}

// DeleteAPI 删除菜单接口，并为拥有该菜单的全部角色重建 Casbin 策略。
//
// 路由 DELETE /web/system/menus/apis/:apiId 只携带接口 ID，此时 menuID 传 0，
// 服务按接口自身归属的菜单处理；传非 0 时校验接口是否属于该菜单。
func (s *MenuService) DeleteAPI(ctx context.Context, menuID, apiID int64) error {
	repo := repository.NewMenuRepo()

	api, err := repo.GetAPIByID(ctx, apiID)
	if err != nil {
		return err
	}
	if menuID > 0 && api.MenuID != menuID {
		return bizErr("接口不属于该菜单")
	}

	if err := repo.DeleteAPI(ctx, apiID); err != nil {
		return err
	}

	s.syncCasbinForMenu(ctx, api.MenuID)
	return nil
}

// ─── 内部方法 ────────────────────────────────────────────────

// resolveLevel 依据父菜单推导层级：根为 1，子为父 level+1。
func (s *MenuService) resolveLevel(ctx context.Context, parentID *int64) (int, error) {
	if parentID == nil || *parentID <= 0 {
		return 1, nil
	}
	parent, err := repository.NewMenuRepo().GetByID(ctx, *parentID)
	if err != nil {
		if IsNotFound(err) {
			return 0, bizErr("父菜单不存在")
		}
		return 0, err
	}
	lv := parent.Level + 1
	if lv < 2 {
		lv = 2
	}
	return lv, nil
}

// ensureNotDescendant 校验 newParentID 不是 id 的后代，防止菜单树成环。
func (s *MenuService) ensureNotDescendant(ctx context.Context, id, newParentID int64) error {
	repo := repository.NewMenuRepo()
	cur := newParentID
	for depth := 0; depth < maxTreeDepth; depth++ {
		parent, err := repo.GetByID(ctx, cur)
		if err != nil {
			if IsNotFound(err) {
				return bizErr("父菜单不存在")
			}
			return err
		}
		if parent.ParentID == nil || *parent.ParentID <= 0 {
			return nil
		}
		if *parent.ParentID == id {
			return bizErr("父菜单不能是当前菜单的子菜单")
		}
		cur = *parent.ParentID
	}
	return bizErr("菜单层级过深或存在循环引用")
}

// syncCasbinForMenu 为拥有指定菜单的全部角色重建 p 策略。
func (s *MenuService) syncCasbinForMenu(ctx context.Context, menuID int64) {
	roles, err := repository.NewMenuRepo().ListRolesByMenuID(ctx, menuID)
	if err != nil {
		utils.Error(fmt.Sprintf("查询菜单关联角色失败 menu_id=%d: %v", menuID, err))
		return
	}
	s.syncCasbinForRoles(ctx, roles)
}

// syncCasbinForRoles 重建给定角色的全部 p 策略。
//
// 步骤：删除角色现有策略 → 按「角色全部菜单的接口」重新写入 → 重载策略。
// Casbin 未初始化（nil）时静默跳过，与 bootstrap 的降级策略保持一致。
func (s *MenuService) syncCasbinForRoles(ctx context.Context, roles []model.Role) {
	cs := bootstrap.CasbinService()
	if cs == nil || len(roles) == 0 {
		return
	}

	repo := repository.NewMenuRepo()
	for _, role := range roles {
		if role.Code == "" {
			continue
		}

		menuIDs, err := repo.ListMenuIDsByRoleID(ctx, role.ID)
		if err != nil {
			utils.Error(fmt.Sprintf("查询角色菜单失败 role=%s: %v", role.Code, err))
			continue
		}
		apis, err := repo.ListAPIsByMenuIDs(ctx, menuIDs)
		if err != nil {
			utils.Error(fmt.Sprintf("查询菜单接口失败 role=%s: %v", role.Code, err))
			continue
		}

		if _, err := cs.RemoveFilteredPolicyByRole(role.Code); err != nil {
			utils.Error(fmt.Sprintf("清理角色策略失败 role=%s: %v", role.Code, err))
			continue
		}
		for _, api := range apis {
			if _, err := cs.AddPolicy(role.Code, api.Path, api.Method); err != nil {
				utils.Error(fmt.Sprintf("写入角色策略失败 role=%s path=%s method=%s: %v",
					role.Code, api.Path, api.Method, err))
			}
		}
	}

	if err := cs.ReloadPolicy(); err != nil {
		utils.Error(fmt.Sprintf("重载 Casbin 策略失败: %v", err))
	}
}

// ─── 树构造 ──────────────────────────────────────────────────

// buildMenuTree 将扁平菜单列表组装为树（children 按输入顺序，即 sequence 升序）。
func buildMenuTree(list []model.Menu) []model.Menu {
	exists := make(map[int64]struct{}, len(list))
	for _, m := range list {
		exists[m.ID] = struct{}{}
	}

	childrenOf := make(map[int64][]model.Menu, len(list))
	roots := make([]model.Menu, 0, len(list))
	for _, m := range list {
		if m.ParentID != nil && *m.ParentID > 0 && *m.ParentID != m.ID {
			if _, ok := exists[*m.ParentID]; ok {
				childrenOf[*m.ParentID] = append(childrenOf[*m.ParentID], m)
				continue
			}
		}
		roots = append(roots, m)
	}

	var fill func(n model.Menu, depth int) model.Menu
	fill = func(n model.Menu, depth int) model.Menu {
		kids := childrenOf[n.ID]
		if len(kids) == 0 || depth >= maxTreeDepth {
			n.Children = nil
			return n
		}
		out := make([]model.Menu, 0, len(kids))
		for _, k := range kids {
			if k.ID == n.ID {
				continue
			}
			out = append(out, fill(k, depth+1))
		}
		n.Children = out
		return n
	}

	for i := range roots {
		roots[i] = fill(roots[i], 1)
	}
	return roots
}
