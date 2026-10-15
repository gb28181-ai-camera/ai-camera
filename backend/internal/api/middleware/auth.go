package middleware

import (
	"strings"
	"sync"

	"github.com/boticz/ai-camera-backend/internal/model"
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

// SetJWTServiceForTest 允许测试重复注入。
func SetJWTServiceForTest(jwt *auth.JWTService) { defaultJWTService = jwt }

// AuthMiddleware 解析 Bearer token 并把用户上下文写入 gin.Context 与 Go Context。
type AuthMiddleware struct{}

// NewAuthMiddleware 构造认证中间件。
func NewAuthMiddleware() *AuthMiddleware { return &AuthMiddleware{} }

// Handler 返回 gin 中间件。
func (m *AuthMiddleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if defaultJWTService == nil {
			utils.ServerError(c, "认证服务未初始化")
			c.Abort()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.Unauthorized(c, "未提供认证Token")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.Unauthorized(c, "认证格式错误")
			c.Abort()
			return
		}

		claims, err := defaultJWTService.ParseToken(parts[1])
		if err != nil {
			utils.Unauthorized(c, "无效的Token")
			c.Abort()
			return
		}

		// 写入 gin.Context（handler 通过 c.GetInt64("user_id") 等读取）
		c.Set("user_id", claims.UserID)
		c.Set("user_name", claims.Username)
		c.Set("role_code", claims.RoleCode)
		c.Set("dept_id", claims.DeptID)
		c.Set("data_perm", claims.DataPerm)
		c.Set("company_id", claims.CompanyID)
		c.Set("cross_dept_ids", claims.CrossDeptIDs)

		// 写入 Go Context：驱动 GORM 钩子自动填充审计字段与部门归属
		ctx := c.Request.Context()
		ctx = model.WithUsername(ctx, claims.Username)
		if claims.DeptID > 0 {
			ctx = model.WithDeptID(ctx, claims.DeptID)
		}
		if claims.UserID > 0 {
			ctx = model.WithUserID(ctx, claims.UserID)
		}
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// CasbinMiddleware 基于 Casbin 策略执行接口级鉴权。
//
// 主体使用登录名（user_name），与角色继承链 g/g2/g3 的主体取值保持一致。
func CasbinMiddleware(enforce func(sub, obj, act string) (bool, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := c.GetString("user_name")
		if username == "" {
			utils.Unauthorized(c, "未提供认证Token")
			c.Abort()
			return
		}

		ok, err := enforce(username, c.Request.URL.Path, c.Request.Method)
		if err != nil {
			utils.ServerError(c, "权限校验失败")
			c.Abort()
			return
		}
		if !ok {
			utils.Forbidden(c, "权限不足")
			c.Abort()
			return
		}

		c.Next()
	}
}
