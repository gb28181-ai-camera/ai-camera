package handler

import (
	"sync"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
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
func SetDefaultJWTService(jwt *auth.JWTService) {
	jwtOnce.Do(func() { defaultJWTService = jwt })
}

// SetJWTServiceForTest 允许测试重复注入 JWT 服务。
func SetJWTServiceForTest(jwt *auth.JWTService) { defaultJWTService = jwt }

// AuthHandler 登录认证接口。
type AuthHandler struct{}

// NewAuthHandler 构造认证处理器。
func NewAuthHandler() *AuthHandler { return &AuthHandler{} }

// Login 管理后台登录
// @Summary 管理后台登录
// @Description 校验用户名密码，签发 JWT，并返回有效数据权限与接口权限集合
// @ID login
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body request.LoginReq true "登录信息"
// @Success 200 {object} response.RLogin
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Router /web/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	authSvc := system.NewAuthService()
	data, err := authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}

	utils.Success(c, data)
}
