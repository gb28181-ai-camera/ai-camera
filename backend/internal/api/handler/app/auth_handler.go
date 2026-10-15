package app

import (
	"sync"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	// 空导入：本文件仅在 Swagger 注解中引用 response 包类型
	// （@Success {object} response.RLogin），显式导入以便 swag 解析该引用。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/auth"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

var (
	defaultJWTService *auth.JWTService
	jwtOnce           sync.Once
)

// SetDefaultJWTService 注入 JWT 服务（由 router 在注册路由前调用）。
//
// 与 handler/web 的同名函数对称：router/app.go 通过本包的该函数注入 JWT，
// 避免 router 与 handler 之间的包循环依赖。
func SetDefaultJWTService(jwt *auth.JWTService) {
	jwtOnce.Do(func() { defaultJWTService = jwt })
}

// SetJWTServiceForTest 允许测试重复注入 JWT 服务。
func SetJWTServiceForTest(jwt *auth.JWTService) { defaultJWTService = jwt }

// AuthHandler 移动端认证处理器（无状态）。
//
// 登录逻辑**完全委托** service/system 的 AuthService，与管理后台共用同一套
// 密码校验与 JWT 签发流程，避免两处实现漂移导致行为不一致。
type AuthHandler struct {
	authSvc *system.AuthService
}

// NewAuthHandler 构造移动端认证处理器。
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{authSvc: system.NewAuthService()}
}

// Login 移动端登录。
// @Summary 移动端登录
// @Description 与 POST /web/auth/login 共用登录逻辑，返回相同的 JWT 载荷（token/user_id/name/dept_id/data_perm/cross_dept_ids/permissions）
// @Tags AppAuth
// @Accept json
// @Produce json
// @Param request body request.LoginReq true "登录参数（user_name / password）"
// @Success 200 {object} response.RLogin "登录成功，data 为 LoginData"
// @Failure 400 {object} response.BaseResponse "参数错误"
// @Failure 401 {object} response.BaseResponse "用户名或密码错误"
// @Router /app/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	data, err := h.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}
	utils.Success(c, data)
}
