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
	// 国标视频（GB/T 28181）：薄封装 handler，业务逻辑复用 /web/gb 的 service 层。
	appGbH := handler.NewAppGbHandler()
	// IOT 设备认领/绑定与 ICE 凭据：所有读写都以 JWT 的 user_id 为边界。
	appIotH := handler.NewAppIotHandler()

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

		// ── 国标视频（GB/T 28181）────────────────────────────
		// 契约 §10.12：13 个移动端接口。复用 /web/gb 的 service 层，
		// handler 只做鉴权上下文取用、参数解析与字段裁剪。
		gb := aAuth.Group("/gb")
		{
			// 设备与通道
			gb.GET("/devices", appGbH.PageDevices)
			gb.GET("/channels", appGbH.PageChannels)

			// 实时播放
			gb.POST("/play/:channelId", appGbH.Play)
			gb.POST("/play/:channelId/stop", appGbH.StopPlay)
			gb.POST("/play/:channelId/snapshot", appGbH.Snapshot)

			// 云台（带外 MESSAGE）
			gb.POST("/ptz/:channelId", appGbH.Ptz)

			// 录像查询与回放
			gb.GET("/playback/records", appGbH.QueryRecord)
			gb.POST("/playback", appGbH.StartPlayback)
			gb.POST("/playback/control", appGbH.ControlPlayback)
			gb.POST("/playback/stop", appGbH.StopPlayback)

			// 语音对讲
			gb.POST("/talk/:channelId/start", appGbH.StartTalk)
			gb.POST("/talk/:channelId/stop", appGbH.StopTalk)

			// 国标报警
			gb.GET("/alarms", appGbH.PageAlarms)
		}

		// ── IOT 设备（App 端）────────────────────────────────
		// 认领（分享二维码）与绑定（蓝牙配网）都是把设备关联到当前用户；
		// 列表/详情/服务组均以 platform_user_id = JWT user_id 为边界。
		iot := aAuth.Group("/iot")
		{
			iot.POST("/devices/claim", appIotH.Claim)
			iot.POST("/devices/bind", appIotH.Bind)
			iot.POST("/devices/bind-qr", appIotH.BindByQR)
			iot.GET("/devices", appIotH.List)
			iot.GET("/devices/:id", appIotH.Get)
			iot.GET("/devices/:id/services", appIotH.Services)
			// 设备分享码：仅该设备的 owner 可生成，shared 账号返回 403。
			iot.POST("/devices/:id/share-token", appIotH.CreateShareToken)
		}

		// ── RTC ICE 服务器（TURN/STUN 短期凭据）───────────────
		aAuth.GET("/rtc/ice-servers", appIotH.IceServers)
	}
}
