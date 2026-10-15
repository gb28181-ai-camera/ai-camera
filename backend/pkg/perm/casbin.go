package perm

import (
	"log"
	"time"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

// CasbinService 封装 Casbin enforcer，提供 RBAC 校验与策略维护能力。
//
// 策略来源：角色(ac_role.code) → 菜单(ac_role_menu_rel) → 菜单接口(ac_menu_api)，
// 即 p 策略为 (roleCode, apiPath, method)。角色继承关系有三条链：
//
//	g  : 用户登录名  → 角色编码（直接分配）
//	g2 : 部门编码    → 角色编码（部门继承）
//	g3 : 岗位编码    → 角色编码（岗位继承）
type CasbinService struct {
	enforcer *casbin.Enforcer
}

// NewCasbinService 基于 model 配置文件与 GORM 适配器创建 enforcer。
// 策略持久化在 casbin_rule 表，由适配器自动建表。
func NewCasbinService(db *gorm.DB, modelPath string) (*CasbinService, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewEnforcer(modelPath, adapter)
	if err != nil {
		return nil, err
	}

	if err := enforcer.LoadPolicy(); err != nil {
		return nil, err
	}

	return &CasbinService{enforcer: enforcer}, nil
}

// Enforce 校验 (用户登录名, 请求路径, 请求方法) 是否放行。
func (s *CasbinService) Enforce(username, path, method string) (bool, error) {
	return s.enforcer.Enforce(username, path, method)
}

// AddPolicy 新增 p 策略（角色 → 接口）。
func (s *CasbinService) AddPolicy(roleCode, path, method string) (bool, error) {
	return s.enforcer.AddPolicy(roleCode, path, method)
}

// RemovePolicy 删除单条 p 策略。
func (s *CasbinService) RemovePolicy(roleCode, path, method string) (bool, error) {
	return s.enforcer.RemovePolicy(roleCode, path, method)
}

// RemoveFilteredPolicyByRole 删除某角色的全部 p 策略。
func (s *CasbinService) RemoveFilteredPolicyByRole(roleCode string) (bool, error) {
	return s.enforcer.RemoveFilteredPolicy(0, roleCode)
}

// AddGroupingPolicy 新增 g 策略（用户 → 角色）。
func (s *CasbinService) AddGroupingPolicy(user, role string) (bool, error) {
	return s.enforcer.AddGroupingPolicy(user, role)
}

// RemoveGroupingPolicy 删除单条 g 策略。
func (s *CasbinService) RemoveGroupingPolicy(user, role string) (bool, error) {
	return s.enforcer.RemoveGroupingPolicy(user, role)
}

// RemoveFilteredGroupingPolicyByRole 删除某角色的全部 g 策略。
func (s *CasbinService) RemoveFilteredGroupingPolicyByRole(roleCode string) (bool, error) {
	return s.enforcer.RemoveFilteredGroupingPolicy(1, roleCode)
}

// AddDeptGroupingPolicy 新增 g2 策略（部门 → 角色）。
func (s *CasbinService) AddDeptGroupingPolicy(deptCode, role string) (bool, error) {
	return s.enforcer.AddNamedGroupingPolicy("g2", deptCode, role)
}

// RemoveDeptGroupingPolicy 删除单条 g2 策略。
func (s *CasbinService) RemoveDeptGroupingPolicy(deptCode, role string) (bool, error) {
	return s.enforcer.RemoveNamedGroupingPolicy("g2", deptCode, role)
}

// RemoveFilteredDeptGroupingPolicyByRole 删除某角色的全部 g2 策略。
func (s *CasbinService) RemoveFilteredDeptGroupingPolicyByRole(roleCode string) (bool, error) {
	return s.enforcer.RemoveFilteredNamedGroupingPolicy("g2", 1, roleCode)
}

// AddPostGroupingPolicy 新增 g3 策略（岗位 → 角色）。
func (s *CasbinService) AddPostGroupingPolicy(postCode, role string) (bool, error) {
	return s.enforcer.AddNamedGroupingPolicy("g3", postCode, role)
}

// RemovePostGroupingPolicy 删除单条 g3 策略。
func (s *CasbinService) RemovePostGroupingPolicy(postCode, role string) (bool, error) {
	return s.enforcer.RemoveNamedGroupingPolicy("g3", postCode, role)
}

// RemoveFilteredPostGroupingPolicyByRole 删除某角色的全部 g3 策略。
func (s *CasbinService) RemoveFilteredPostGroupingPolicyByRole(roleCode string) (bool, error) {
	return s.enforcer.RemoveFilteredNamedGroupingPolicy("g3", 1, roleCode)
}

// ReloadPolicy 从存储重新加载全部策略（角色授权变更后调用）。
func (s *CasbinService) ReloadPolicy() error {
	return s.enforcer.LoadPolicy()
}

// StartAutoReload 启动定时重载，保证多实例部署时策略最终一致。
func (s *CasbinService) StartAutoReload(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := s.enforcer.LoadPolicy(); err != nil {
				log.Printf("Casbin 策略自动重载失败: %v", err)
			}
		}
	}()
}
