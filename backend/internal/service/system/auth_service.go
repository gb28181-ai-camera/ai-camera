package system

import (
	"context"
	"errors"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// errInvalidCredential 登录失败的统一模糊提示：不区分「用户不存在」与「密码错误」，
// 避免account enumeration。
var errInvalidCredential = errors.New("用户名或密码错误")

// AuthService 认证服务。
type AuthService struct{}

// NewAuthService 构造认证服务。
func NewAuthService() *AuthService { return &AuthService{} }

// Login 校验用户名密码并签发 JWT。
//
// 流程：
//  1. 按登录名取用户，失败与密码错误返回同一模糊错误；
//  2. bcrypt 校验密码，并拒绝已禁用账号；
//  3. 计算有效数据权限（直接/岗位/部门三路角色取最高）与主角色编码；
//  4. 反查 Casbin p 策略得到接口权限集合；
//  5. 签发 token 并刷新最后登录时间。
func (s *AuthService) Login(ctx context.Context, username, password string) (*response.LoginData, error) {
	userRepo := repository.NewUserRepo()

	user, err := userRepo.GetByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, errInvalidCredential
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errInvalidCredential
	}

	if user.Active != nil && !*user.Active {
		return nil, errors.New("用户已被禁用")
	}

	jwtSvc := bootstrap.JWTService()
	if jwtSvc == nil {
		return nil, errors.New("认证服务未初始化")
	}

	// 有效数据权限：is_admin 直接 all，否则三路角色取最高
	effPerm := userRepo.GetUserEffectiveDataPermission(ctx, user.ID)
	if effPerm == nil {
		effPerm = &repository.EffectivePermission{DataPerm: "personal"}
	}

	// 角色编码：取有效角色中 sequence 最小者，无角色则为空
	roleCode, err := userRepo.GetPrimaryRoleCode(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	permissions := userRepo.GetUserEffectivePermissions(ctx, user.ID)

	var deptID int64
	if user.DeptID != nil {
		deptID = *user.DeptID
	}

	token, err := jwtSvc.GenerateToken(
		user.ID,
		user.Username,
		roleCode,
		deptID,
		effPerm.DataPerm,
		0,
		effPerm.CrossDeptIDs,
	)
	if err != nil {
		return nil, err
	}

	// 最后登录时间与审计字段更新失败不影响登录结果
	_ = userRepo.UpdateLastLogin(ctx, user.ID, time.Now(), user.Username)

	return &response.LoginData{
		Token:        token,
		UserID:       user.ID,
		Name:         user.Name,
		DeptID:       user.DeptID,
		DataPerm:     effPerm.DataPerm,
		CrossDeptIDs: effPerm.CrossDeptIDs,
		Permissions:  permissions,
	}, nil
}
