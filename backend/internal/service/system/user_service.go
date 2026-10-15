package system

import (
	"context"
	"errors"
	"fmt"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"golang.org/x/crypto/bcrypt"
)

// UserService 用户管理服务。
type UserService struct{}

// NewUserService 构造用户服务。
func NewUserService() *UserService { return &UserService{} }

// Page 分页查询用户。
func (s *UserService) Page(ctx context.Context, req *request.PageUserReq, ds *utils.DataScope) (*response.PageUserResp, error) {
	req.Normalize()

	items, total, err := repository.NewUserRepo().Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}

	return &response.PageUserResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// Options 返回用户下拉选项（仅启用用户）。
func (s *UserService) Options(ctx context.Context) ([]response.OptionItem, error) {
	users, err := repository.NewUserRepo().Options(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]response.OptionItem, 0, len(users))
	for _, u := range users {
		items = append(items, response.OptionItem{ID: u.ID, Name: u.Name, Code: u.Username})
	}
	return items, nil
}

// Get 查询用户详情。
func (s *UserService) Get(ctx context.Context, id int64) (*model.User, error) {
	return repository.NewUserRepo().GetByID(ctx, id)
}

// Create 新增用户，并按需写入角色/岗位关联与 Casbin g 策略。
func (s *UserService) Create(ctx context.Context, req *request.CreateUserReq) (*model.User, error) {
	userRepo := repository.NewUserRepo()

	if existing, _ := userRepo.GetByUsername(ctx, req.Username); existing != nil {
		return nil, fmt.Errorf("登录名 %s 已存在", req.Username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}
	isAdmin := false
	if req.IsAdmin != nil {
		isAdmin = *req.IsAdmin
	}

	user := &model.User{
		Name:         req.Name,
		Username:     req.Username,
		PasswordHash: string(hash),
		Email:        req.Email,
		Phone:        req.Phone,
		Avatar:       req.Avatar,
		DeptID:       req.DeptID,
		PostID:       req.PostID,
		Active:       &active,
		IsAdmin:      &isAdmin,
		Remark:       req.Remark,
	}

	if err := userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	if len(req.RoleIDs) > 0 {
		if err := userRepo.SetRoleIDs(ctx, user.ID, req.RoleIDs); err != nil {
			return nil, err
		}
		// 同步 Casbin g 策略（user_name -> role.code）
		_ = syncCasbinForUser(ctx, user.ID, "", nil)
	}
	if req.PostID != nil {
		_ = userRepo.SetPostIDs(ctx, user.ID, []int64{*req.PostID})
	}

	return userRepo.GetByID(ctx, user.ID)
}

// Update 更新用户信息。字符串字段为空表示不修改；指针字段 nil 表示不修改（显式传 null 表示清空）。
func (s *UserService) Update(ctx context.Context, id int64, req *request.UpdateUserReq) error {
	userRepo := repository.NewUserRepo()

	user, err := userRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("用户不存在")
	}

	oldUsername := user.Username
	oldRoles, err := userRepo.GetDirectRoles(ctx, id)
	if err != nil {
		return err
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	// 登录名（user_name）是 Casbin 的 sub 主体，创建后不可修改；
	// 即使请求携带 user_name 也一律忽略，保持原值。
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}
	if req.Remark != "" {
		user.Remark = req.Remark
	}
	if req.DeptID != nil {
		user.DeptID = req.DeptID
	}
	if req.PostID != nil {
		user.PostID = req.PostID
	}
	if req.Active != nil {
		user.Active = req.Active
	}
	if req.IsAdmin != nil {
		user.IsAdmin = req.IsAdmin
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return errors.New("密码加密失败")
		}
		user.PasswordHash = string(hash)
	}

	if err := userRepo.Update(ctx, user); err != nil {
		return err
	}

	if req.RoleIDs != nil {
		if err := userRepo.SetRoleIDs(ctx, id, req.RoleIDs); err != nil {
			return err
		}
	}
	if req.PostID != nil {
		if err := userRepo.SetPostIDs(ctx, id, []int64{*req.PostID}); err != nil {
			return err
		}
	}

	// 同步 Casbin g 策略：清理旧 (用户名, 角色) 后再按最新角色重建
	oldCodes := make([]string, 0, len(oldRoles))
	for _, r := range oldRoles {
		if r.Code != "" {
			oldCodes = append(oldCodes, r.Code)
		}
	}
	_ = syncCasbinForUser(ctx, id, oldUsername, oldCodes)

	return nil
}

// Delete 软删除用户，并清理角色/岗位关联与 Casbin g 策略。
func (s *UserService) Delete(ctx context.Context, id int64) error {
	userRepo := repository.NewUserRepo()

	user, err := userRepo.GetByID(ctx, id)
	if err != nil {
		return errors.New("用户不存在")
	}

	oldRoles, err := userRepo.GetDirectRoles(ctx, id)
	if err != nil {
		return err
	}
	oldCodes := make([]string, 0, len(oldRoles))
	for _, r := range oldRoles {
		if r.Code != "" {
			oldCodes = append(oldCodes, r.Code)
		}
	}

	if err := userRepo.Delete(ctx, id); err != nil {
		return err
	}

	// 用户已软删除，此处仅清理 Casbin 中遗留的 (用户名, 角色) 策略
	_ = syncCasbinForUser(ctx, id, user.Username, oldCodes)
	return nil
}

// SetActive 启用/禁用用户。
func (s *UserService) SetActive(ctx context.Context, id int64, active bool) error {
	userRepo := repository.NewUserRepo()
	if _, err := userRepo.GetByID(ctx, id); err != nil {
		return errors.New("用户不存在")
	}
	return userRepo.SetActive(ctx, id, active)
}

// ResetPassword 管理员重置用户密码（无需原密码）。
func (s *UserService) ResetPassword(ctx context.Context, id int64, newPassword string) error {
	userRepo := repository.NewUserRepo()
	if _, err := userRepo.GetByID(ctx, id); err != nil {
		return errors.New("用户不存在")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码加密失败")
	}
	return userRepo.UpdatePassword(ctx, id, string(hash))
}
