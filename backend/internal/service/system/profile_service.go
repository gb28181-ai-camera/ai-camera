package system

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// ProfileService 个人中心服务：个人信息、密码、菜单树与 API Key。
type ProfileService struct{}

// NewProfileService 构造个人中心服务。
func NewProfileService() *ProfileService { return &ProfileService{} }

// Get 查询当前登录用户的完整信息。
func (s *ProfileService) Get(ctx context.Context, userID int64) (*model.User, error) {
	return repository.NewUserRepo().GetByID(ctx, userID)
}

// Update 更新当前登录用户的个人资料。
func (s *ProfileService) Update(ctx context.Context, userID int64, req *request.UpdateProfileReq) error {
	userRepo := repository.NewUserRepo()

	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	user.Avatar = req.Avatar
	user.Remark = req.Remark

	return userRepo.Update(ctx, user)
}

// ChangePassword 校验原密码后更新为新密码。
func (s *ProfileService) ChangePassword(ctx context.Context, userID int64, req *request.ChangePasswordReq) error {
	userRepo := repository.NewUserRepo()

	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return errors.New("用户不存在")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return errors.New("原密码错误")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.New("密码加密失败")
	}

	return userRepo.UpdatePassword(ctx, userID, string(hash))
}

// Menus 返回当前用户可见的 PC 端菜单树。
//
// 菜单来源：三路角色（直接/岗位/部门）→ ac_role_menu_rel → ac_menu（platform=pc）。
// 树中同时包含 directory/menu/button 节点：前端据 type 渲染侧边栏，
// 并由 button 节点的 permission_code 驱动按钮级鉴权。
func (s *ProfileService) Menus(ctx context.Context, username string) ([]model.Menu, error) {
	userRepo := repository.NewUserRepo()

	user, err := userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return []model.Menu{}, nil
	}

	menuIDs, err := userRepo.GetAssignedMenuIDs(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if len(menuIDs) == 0 {
		return []model.Menu{}, nil
	}

	menus, err := userRepo.GetMenusByIDs(ctx, menuIDs, "pc")
	if err != nil {
		return nil, err
	}
	return buildProfileMenuTree(menus), nil
}

// buildProfileMenuTree 在内存中把扁平菜单组装为树。
//
// 父节点不在集合内（未授权父级）的节点会被提升为根节点，避免菜单丢失。
func buildProfileMenuTree(menus []model.Menu) []model.Menu {
	present := make(map[int64]struct{}, len(menus))
	for _, m := range menus {
		present[m.ID] = struct{}{}
	}

	childrenOf := make(map[int64][]model.Menu, len(menus))
	roots := make([]model.Menu, 0, len(menus))
	for _, m := range menus {
		if m.ParentID != nil && *m.ParentID != 0 {
			if _, ok := present[*m.ParentID]; ok {
				childrenOf[*m.ParentID] = append(childrenOf[*m.ParentID], m)
				continue
			}
		}
		roots = append(roots, m)
	}

	var attach func(nodes []model.Menu) []model.Menu
	attach = func(nodes []model.Menu) []model.Menu {
		out := make([]model.Menu, 0, len(nodes))
		for _, n := range nodes {
			n.Children = attach(childrenOf[n.ID])
			out = append(out, n)
		}
		return out
	}
	return attach(roots)
}

// ListApiKeys 列出当前用户创建的 API Key。
func (s *ProfileService) ListApiKeys(ctx context.Context, userID int64) ([]model.ApiKey, error) {
	return repository.NewApiKeyRepo().ListByUser(ctx, userID)
}

// GenerateApiKey 生成新的 API Key。
//
// 随机 32 字节 → 64 位 hex 全key；仅存 SHA256 摘要与前 8 位前缀，
// FullKey 只在本次响应中返回，之后无法再次获取。
func (s *ProfileService) GenerateApiKey(ctx context.Context, userID int64, name string) (*response.GenerateApiKeyResp, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}
	fullKey := hex.EncodeToString(raw)
	prefix := fullKey[:8]
	sum := sha256.Sum256([]byte(fullKey))

	active := true
	key := &model.ApiKey{
		Active:    &active,
		UserID:    userID,
		Name:      name,
		KeyPrefix: prefix,
		KeyHash:   hex.EncodeToString(sum[:]),
	}

	if err := repository.NewApiKeyRepo().Create(ctx, key); err != nil {
		return nil, fmt.Errorf("创建API Key失败: %w", err)
	}

	return &response.GenerateApiKeyResp{
		ID:        key.ID,
		Name:      key.Name,
		KeyPrefix: prefix,
		FullKey:   fullKey,
	}, nil
}

// DeleteApiKey 删除当前用户自己的 API Key。
func (s *ProfileService) DeleteApiKey(ctx context.Context, userID, keyID int64) error {
	apiKeyRepo := repository.NewApiKeyRepo()

	key, err := apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return errors.New("密钥不存在")
	}
	if key.UserID != userID {
		return errors.New("无权操作该密钥")
	}

	return apiKeyRepo.Delete(ctx, keyID, userID)
}
