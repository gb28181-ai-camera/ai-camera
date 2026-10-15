package router

import (
	handler "github.com/boticz/ai-camera-backend/internal/api/handler/app"
	"github.com/boticz/ai-camera-backend/internal/api/middleware"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/gin-gonic/gin"
)

// @title           AI Camera 智能监控平台 App API
// @version         1.0.0
// @description     AI Camera 后端系统接口文档 - 移动端
// @termsOfService  http://swagger.io/terms/

// @contact.name   技术支持
// @contact.email  support@boticz.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @servers.url          http://localhost:8080/app
// @servers.description  App 端接口文档

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Bearer token认证，格式: Bearer {token}
func registerAppRoutes(r *gin.Engine) {
	jwtSvc := bootstrap.JWTService()
	middleware.SetDefaultJWTService(jwtSvc)
	handler.SetDefaultJWTService(jwtSvc)

	authMW := middleware.NewAuthMiddleware()

	var casbinMW gin.HandlerFunc
	if cs := bootstrap.CasbinService(); cs != nil {
		casbinMW = middleware.CasbinMiddleware(cs.Enforce)
	} else {
		casbinMW = func(c *gin.Context) { c.Next() }
	}

	appAuthH := handler.NewAuthHandler()
	appH := handler.NewAppHandler()

	// ── 公开接口 ──
	a := r.Group("/app")
	{
		a.POST("/auth/login", appAuthH.Login)
	}

	// ── 需要认证的接口 ──
	aAuth := r.Group("/app")
	aAuth.Use(authMW.Handler(), casbinMW)
	{
		// 菜单与首页
		aAuth.GET("/menus", appH.Menus)
		aAuth.GET("/home", appH.Home)
		aAuth.GET("/todos", appH.Todos)

		// 风险
		aAuth.GET("/risks", appH.PageRisks)
		aAuth.GET("/risks/:id", appH.GetRisk)
		aAuth.PUT("/risks/:id/handle", appH.HandleRisk)
		aAuth.PUT("/risks/:id/accurate", appH.UpdateRiskAccurate)

		// 设备告警
		aAuth.GET("/alarms", appH.PageAlarms)
		aAuth.PUT("/alarms/:id/handle", appH.HandleAlarm)

		// 巡检
		aAuth.GET("/inspect-tasks", appH.PageInspectTasks)
		aAuth.GET("/inspect-tasks/:id", appH.GetInspectTask)
		aAuth.PUT("/inspect-tasks/:id/finish", appH.FinishInspectTask)

		// 设备
		aAuth.GET("/devices", appH.PageDevices)
		aAuth.GET("/devices/:id", appH.GetDevice)

		// 通知
		aAuth.GET("/notifications", appH.PageNotifications)
		aAuth.PUT("/notifications/:id/read", appH.MarkNotificationRead)

		// 个人中心
		aAuth.GET("/profile", appH.GetProfile)
		aAuth.PUT("/profile", appH.UpdateProfile)
		aAuth.PUT("/profile/password", appH.ChangePassword)
	}
}
