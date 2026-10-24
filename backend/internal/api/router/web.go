package router

import (
	handler "github.com/boticz/ai-camera-backend/internal/api/handler/web"
	"github.com/boticz/ai-camera-backend/internal/api/middleware"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/gin-gonic/gin"
)

// @title           AI Camera 智能监控平台 Web API
// @version         1.0.0
// @description     AI Camera 后端系统接口文档 - Web 端（管理后台）
// @termsOfService  http://swagger.io/terms/

// @contact.name   技术支持
// @contact.email  support@boticz.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @servers.url          http://localhost:8080/web
// @servers.description  Web 端接口文档

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Bearer token认证，格式: Bearer {token}

// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        X-Api-Key
// @description                 设备上报等无用户会话场景使用的开放接口密钥
func registerWebRoutes(r *gin.Engine) {
	// 注入 JWT 服务到 middleware 与 handler 包（避免包循环依赖）
	jwtSvc := bootstrap.JWTService()
	middleware.SetDefaultJWTService(jwtSvc)
	handler.SetDefaultJWTService(jwtSvc)

	authMW := middleware.NewAuthMiddleware()

	// Casbin 权限校验中间件：初始化失败时降级为放行（bootstrap 已打印显著告警）
	var casbinMW gin.HandlerFunc
	if cs := bootstrap.CasbinService(); cs != nil {
		casbinMW = middleware.CasbinMiddleware(cs.Enforce)
	} else {
		casbinMW = func(c *gin.Context) { c.Next() }
	}

	// ── 构造全部 Handler ──
	authH := handler.NewAuthHandler()
	profileH := handler.NewProfileHandler()
	userH := handler.NewUserHandler()
	roleH := handler.NewRoleHandler()
	menuH := handler.NewMenuHandler()
	deptH := handler.NewDeptHandler()
	postH := handler.NewPostHandler()
	configH := handler.NewConfigHandler()
	logH := handler.NewOperationLogHandler()
	notifyH := handler.NewNotificationHandler()
	basicH := handler.NewBasicHandler()
	deviceH := handler.NewDeviceHandler()
	modeH := handler.NewDeviceModeHandler()
	cardH := handler.NewIotCardHandler()
	voiceH := handler.NewVoiceConfigHandler()
	riskH := handler.NewRiskHandler()
	alarmH := handler.NewAlarmHandler()
	inspectH := handler.NewInspectHandler()
	upgradeH := handler.NewUpgradeHandler()
	settingH := handler.NewSettingHandler()
	webhookH := handler.NewWebhookHandler()
	statsH := handler.NewStatsHandler()
	uploadH := handler.NewUploadHandler()

	// IOT 管理（产品 / 服务组 / 设备账号）
	iotProductH := handler.NewIotProductHandler()
	iotServiceGroupH := handler.NewIotServiceGroupHandler()
	iotDeviceH := handler.NewIotDeviceHandler()

	// 国标视频（GB/T 28181）业务 CRUD 层
	gbDeviceH := handler.NewGbDeviceHandler()
	gbChannelH := handler.NewGbChannelHandler()
	gbMediaServerH := handler.NewGbMediaServerHandler()
	gbGroupH := handler.NewGbGroupHandler()
	gbSubscribeH := handler.NewGbSubscribeHandler()
	gbSipLogH := handler.NewGbSipLogHandler()

	// ════════════════════════════════════════════════════════════════
	// 公开接口（无需认证）
	// ════════════════════════════════════════════════════════════════
	w := r.Group("/web")
	{
		w.POST("/auth/login", authH.Login)
	}

	// ════════════════════════════════════════════════════════════════
	// API Key 认证接口（设备上报 / 回调，不经过 Casbin）
	// ════════════════════════════════════════════════════════════════
	wApiKey := r.Group("/web")
	wApiKey.Use(middleware.ApiKeyAuth())
	{
		wApiKey.POST("/risks/report", riskH.Report)
		wApiKey.POST("/alarms/report", alarmH.Report)
		wApiKey.POST("/devices/:id/heartbeat", deviceH.Heartbeat)
	}

	// ════════════════════════════════════════════════════════════════
	// 需要认证的接口：authMW（身份）→ casbinMW（接口级 RBAC）→ 操作日志
	// ════════════════════════════════════════════════════════════════
	wAuth := r.Group("/web")
	wAuth.Use(authMW.Handler(), casbinMW, middleware.OperationLog())
	{
		// ═══ 个人中心 ═══
		wAuth.GET("/profile", profileH.GetProfile)
		wAuth.PUT("/profile", profileH.UpdateProfile)
		wAuth.PUT("/profile/password", profileH.ChangePassword)
		wAuth.GET("/profile/menus", profileH.GetMenus)
		wAuth.GET("/profile/api-keys", profileH.ListApiKeys)
		wAuth.POST("/profile/api-keys", profileH.GenerateApiKey)
		wAuth.DELETE("/profile/api-keys/:id", profileH.DeleteApiKey)

		// ═══ 上传 ═══
		wAuth.POST("/upload", uploadH.Upload)
		wAuth.POST("/upload/audio", uploadH.UploadAudio)

		// ═══ 系统管理 ═══
		wsg := wAuth.Group("/system")
		{
			// 用户
			wsg.GET("/users", userH.Page)
			wsg.GET("/users/options", userH.Options)
			wsg.POST("/users", userH.Create)
			wsg.GET("/users/:id", userH.Get)
			wsg.PUT("/users/:id", userH.Update)
			wsg.DELETE("/users/:id", userH.Delete)
			wsg.PUT("/users/:id/enable", userH.Enable)
			wsg.PUT("/users/:id/disable", userH.Disable)
			wsg.PUT("/users/:id/reset-password", userH.ResetPassword)

			// 角色
			wsg.GET("/roles", roleH.Page)
			wsg.GET("/roles/options", roleH.Options)
			wsg.POST("/roles", roleH.Create)
			wsg.GET("/roles/:id", roleH.Get)
			wsg.PUT("/roles/:id", roleH.Update)
			wsg.DELETE("/roles/:id", roleH.Delete)
			wsg.GET("/roles/:id/menus", roleH.GetMenus)
			wsg.PUT("/roles/:id/menus", roleH.AssignMenus)
			wsg.GET("/roles/:id/depts", roleH.GetDepts)
			wsg.PUT("/roles/:id/depts", roleH.AssignDepts)
			wsg.GET("/roles/:id/posts", roleH.GetPosts)
			wsg.PUT("/roles/:id/posts", roleH.AssignPosts)
			wsg.GET("/roles/:id/users", roleH.GetUsers)
			wsg.PUT("/roles/:id/users", roleH.AssignUsers)

			// 菜单（静态子路径写在 :id 之前）
			wsg.GET("/menus", menuH.Tree)
			wsg.POST("/menus", menuH.Create)
			wsg.DELETE("/menus/apis/:apiId", menuH.DeleteAPI)
			wsg.GET("/menus/:id", menuH.Get)
			wsg.PUT("/menus/:id", menuH.Update)
			wsg.DELETE("/menus/:id", menuH.Delete)
			wsg.GET("/menus/:id/apis", menuH.ListAPIs)
			wsg.POST("/menus/:id/apis", menuH.CreateAPI)

			// 部门
			wsg.GET("/depts", deptH.Tree)
			wsg.GET("/depts/options", deptH.Options)
			wsg.POST("/depts", deptH.Create)
			wsg.GET("/depts/:id", deptH.Get)
			wsg.PUT("/depts/:id", deptH.Update)
			wsg.DELETE("/depts/:id", deptH.Delete)

			// 岗位
			wsg.GET("/posts", postH.Page)
			wsg.GET("/posts/options", postH.Options)
			wsg.POST("/posts", postH.Create)
			wsg.GET("/posts/:id", postH.Get)
			wsg.PUT("/posts/:id", postH.Update)
			wsg.DELETE("/posts/:id", postH.Delete)

			// 系统参数
			wsg.GET("/configs", configH.List)
			wsg.POST("/configs", configH.Create)
			wsg.PUT("/configs/:id", configH.Update)
			wsg.DELETE("/configs/:id", configH.Delete)
			wsg.PUT("/configs/:id/enable", configH.Enable)
			wsg.PUT("/configs/:id/disable", configH.Disable)

			// 操作日志
			wsg.GET("/logs", logH.Page)
			wsg.GET("/logs/export", logH.Export)
			wsg.DELETE("/logs/clear", logH.Clear)
			wsg.GET("/logs/:id", logH.Get)
			wsg.DELETE("/logs/:id", logH.Delete)

			// 系统通知
			wsg.GET("/notifications", notifyH.Page)
			wsg.GET("/notifications/unread-count", notifyH.UnreadCount)
			wsg.PUT("/notifications/read-all", notifyH.MarkAllRead)
			wsg.PUT("/notifications/:id/read", notifyH.MarkRead)
			wsg.DELETE("/notifications/:id", notifyH.Delete)
		}

		// ═══ 基础信息 ═══
		wbg := wAuth.Group("/basic")
		{
			wbg.GET("/regions", basicH.PageRegions)
			wbg.GET("/regions/options", basicH.RegionOptions)
			wbg.POST("/regions", basicH.CreateRegion)
			wbg.GET("/regions/:id", basicH.GetRegion)
			wbg.PUT("/regions/:id", basicH.UpdateRegion)
			wbg.DELETE("/regions/:id", basicH.DeleteRegion)

			wbg.GET("/risk-types", basicH.PageRiskTypes)
			wbg.GET("/risk-types/options", basicH.RiskTypeOptions)
			wbg.POST("/risk-types", basicH.CreateRiskType)
			wbg.GET("/risk-types/:id", basicH.GetRiskType)
			wbg.PUT("/risk-types/:id", basicH.UpdateRiskType)
			wbg.DELETE("/risk-types/:id", basicH.DeleteRiskType)

			wbg.GET("/alarm-types", basicH.PageAlarmTypes)
			wbg.GET("/alarm-types/options", basicH.AlarmTypeOptions)
			wbg.POST("/alarm-types", basicH.CreateAlarmType)
			wbg.GET("/alarm-types/:id", basicH.GetAlarmType)
			wbg.PUT("/alarm-types/:id", basicH.UpdateAlarmType)
			wbg.DELETE("/alarm-types/:id", basicH.DeleteAlarmType)
		}

		// ═══ 资源管理（设备 / 设备模式 / 物联网卡 / 语音配置）═══
		// 静态子路径统一写在 /:id 之前（书写约定；gin v1.12 已自动规范化同级顺序，见 tests/api/route_test.go）。
		wdg := wAuth.Group("/devices")
		{
			// 设备历史（独立于设备详情，必须先注册）
			wdg.GET("/history", deviceH.PageHistory)

			// 设备模式
			wdg.GET("/modes", modeH.Page)
			wdg.GET("/modes/options", modeH.Options)
			wdg.POST("/modes", modeH.Create)
			wdg.PUT("/modes/:id", modeH.Update)
			wdg.DELETE("/modes/:id", modeH.Delete)

			// 物联网卡（换卡记录静态段写在 /iot-cards/:id 之前）
			wdg.GET("/iot-cards", cardH.Page)
			wdg.GET("/iot-cards/options", cardH.Options)
			wdg.POST("/iot-cards", cardH.Create)
			wdg.GET("/iot-card-replace-logs", cardH.PageReplaceLogs)
			wdg.POST("/iot-card-replace-logs", cardH.ReplaceCard)
			wdg.PUT("/iot-cards/:id", cardH.Update)
			wdg.DELETE("/iot-cards/:id", cardH.Delete)

			// 语音配置
			wdg.GET("/voice-configs", voiceH.Page)
			wdg.POST("/voice-configs", voiceH.Create)
			wdg.PUT("/voice-configs/:id", voiceH.Update)
			wdg.DELETE("/voice-configs/:id", voiceH.Delete)

			// 设备本体
			wdg.GET("", deviceH.Page)
			wdg.GET("/options", deviceH.Options)
			wdg.POST("", deviceH.Create)
			wdg.GET("/:id", deviceH.Get)
			wdg.PUT("/:id", deviceH.Update)
			wdg.DELETE("/:id", deviceH.Delete)
			wdg.PUT("/:id/mode", deviceH.SwitchMode)
			wdg.PUT("/:id/sync-time", deviceH.SyncTime)
			wdg.PUT("/:id/monitor", deviceH.ToggleMonitor)
			wdg.GET("/:id/history-chart", deviceH.HistoryChart)
		}

		// ═══ 风险 ═══
		wrg := wAuth.Group("/risks")
		{
			wrg.GET("", riskH.Page)
			wrg.GET("/pending-count", riskH.PendingCount)
			wrg.GET("/:id", riskH.Get)
			wrg.PUT("/:id/read", riskH.MarkRead)
			wrg.PUT("/:id/accurate", riskH.UpdateAccurate)
			wrg.PUT("/:id/handle", riskH.Handle)
			wrg.DELETE("/:id", riskH.Delete)
		}

		// ═══ 设备告警 ═══
		wag := wAuth.Group("/alarms")
		{
			wag.GET("", alarmH.Page)
			wag.GET("/pending-count", alarmH.PendingCount)
			wag.GET("/:id", alarmH.Get)
			wag.PUT("/:id/read", alarmH.MarkRead)
			wag.PUT("/:id/handle", alarmH.Handle)
			wag.DELETE("/:id", alarmH.Delete)
		}

		// ═══ 巡检中心 ═══
		wig := wAuth.Group("/inspects")
		{
			wig.GET("/tasks", inspectH.PageTasks)
			wig.POST("/tasks", inspectH.CreateTask)
			wig.GET("/tasks/:id", inspectH.GetTask)
			wig.PUT("/tasks/:id", inspectH.UpdateTask)
			// /tasks/:id/read 与 /tasks/:id 处于不同深度，可共存
			wig.PUT("/tasks/:id/read", inspectH.MarkTaskRead)
			wig.DELETE("/tasks/:id", inspectH.DeleteTask)

			wig.GET("/records", inspectH.PageRecords)
			wig.GET("/records/:id", inspectH.GetRecord)
		}

		// ═══ 升级管理 ═══
		wug := wAuth.Group("/upgrades")
		{
			// 固件类型
			wug.GET("/firmware-types", upgradeH.PageFirmwareTypes)
			wug.GET("/firmware-types/options", upgradeH.FirmwareTypeOptions)
			wug.POST("/firmware-types", upgradeH.CreateFirmwareType)
			wug.PUT("/firmware-types/:id", upgradeH.UpdateFirmwareType)
			wug.DELETE("/firmware-types/:id", upgradeH.DeleteFirmwareType)

			// 固件（/firmwares/options 等静态段写在 /firmwares/:id 之前）
			wug.GET("/firmwares", upgradeH.PageFirmwares)
			wug.GET("/firmwares/options", upgradeH.FirmwareOptions)
			wug.GET("/firmwares/:id", upgradeH.GetFirmware)
			wug.POST("/firmwares", upgradeH.CreateFirmware)
			wug.PUT("/firmwares/:id", upgradeH.UpdateFirmware)
			wug.DELETE("/firmwares/:id", upgradeH.DeleteFirmware)

			// AI 模型（/models/options 等静态段写在 /models/:id 之前）
			wug.GET("/models", upgradeH.PageModels)
			wug.GET("/models/options", upgradeH.ModelOptions)
			wug.GET("/models/:id", upgradeH.GetModel)
			wug.POST("/models", upgradeH.CreateModel)
			wug.PUT("/models/:id", upgradeH.UpdateModel)
			wug.DELETE("/models/:id", upgradeH.DeleteModel)

			// 升级下发 / 计划 / 记录
			wug.POST("/devices", upgradeH.UpgradeDevices)
			wug.GET("/plans", upgradeH.PagePlans)
			wug.PUT("/plans/cancel", upgradeH.BatchCancelPlans)
			wug.PUT("/plans/:id/cancel", upgradeH.CancelPlan)
			wug.GET("/records", upgradeH.PageRecords)
		}

		// ═══ 告警设置 ═══
		wstg := wAuth.Group("/settings")
		{
			// 设备告警设置（disable-all 静态段写在 /:id 之前）
			wstg.GET("/alarm-settings", settingH.PageAlarmSettings)
			wstg.POST("/alarm-settings", settingH.CreateAlarmSetting)
			wstg.PUT("/alarm-settings/disable-all", settingH.DisableAllAlarm)
			wstg.PUT("/alarm-settings/:id", settingH.UpdateAlarmSetting)
			wstg.DELETE("/alarm-settings/:id", settingH.DeleteAlarmSetting)

			// 风险通知设置
			wstg.GET("/risk-notify-settings", settingH.PageRiskNotifySettings)
			wstg.POST("/risk-notify-settings", settingH.CreateRiskNotifySetting)
			wstg.PUT("/risk-notify-settings/disable-all", settingH.DisableAllRiskNotify)
			wstg.PUT("/risk-notify-settings/:id", settingH.UpdateRiskNotifySetting)
			wstg.DELETE("/risk-notify-settings/:id", settingH.DeleteRiskNotifySetting)

			// 第三方平台
			wstg.GET("/getui", settingH.GetGetui)
			wstg.PUT("/getui", settingH.UpdateGetui)
			wstg.GET("/sms", settingH.GetSms)
			wstg.PUT("/sms", settingH.UpdateSms)

			// 消息推送 WebHook（飞书 / 钉钉 / 企业微信 / 自定义）
			// 静态子路径写在 /:id 之前（书写约定，非 gin 硬约束）
			wstg.GET("/webhooks", webhookH.Page)
			wstg.POST("/webhooks", webhookH.Create)
			wstg.GET("/webhooks/platforms", webhookH.Platforms)
			wstg.POST("/webhooks/test", webhookH.TestInline)
			wstg.GET("/webhooks/:id", webhookH.Get)
			wstg.PUT("/webhooks/:id", webhookH.Update)
			wstg.DELETE("/webhooks/:id", webhookH.Delete)
			wstg.PUT("/webhooks/:id/enable", webhookH.Enable)
			wstg.PUT("/webhooks/:id/disable", webhookH.Disable)
			wstg.POST("/webhooks/:id/test", webhookH.Test)
		}

		// ═══ 统计分析 ═══
		wtsg := wAuth.Group("/stats")
		{
			wtsg.GET("/dashboard", statsH.Dashboard)
			wtsg.GET("/risk", statsH.RiskStats)
			wtsg.GET("/alarm", statsH.AlarmStats)
			wtsg.GET("/screen", statsH.Screen)
			wtsg.GET("/traffic", statsH.TrafficStats)
			wtsg.GET("/device-status", statsH.DeviceStatus)
		}

		// ═══ 国标视频（GB/T 28181）═══
		// 静态子路径（options / tree / stats）统一写在 /:id 之前。
		gb := wAuth.Group("/gb")
		{
			// 国标设备
			gb.GET("/devices", gbDeviceH.Page)
			gb.GET("/devices/:id", gbDeviceH.Get)
			gb.POST("/devices/:id/sync-channel", gbDeviceH.SyncChannel)
			// 可达性 / NAT 诊断：只读 + 一条标准 DeviceInfo 查询，不修改设备状态。
			gb.POST("/devices/:id/diagnose", gbDeviceH.Diagnose)
			gb.POST("/devices/:id/subscribe", gbDeviceH.Subscribe)
			gb.DELETE("/devices/:id/subscribe", gbDeviceH.Unsubscribe)
			gb.POST("/devices/:id/activation", gbDeviceH.GenerateActivation)
			gb.DELETE("/devices/:id", gbDeviceH.Delete)

			// 国标通道
			gb.GET("/channels", gbChannelH.Page)
			gb.GET("/channels/options", gbChannelH.Options)
			gb.GET("/channels/:id", gbChannelH.Get)
			gb.PUT("/channels/:id", gbChannelH.Update)
			gb.DELETE("/channels/:id", gbChannelH.Delete)
			gb.PUT("/channels/:id/region", gbChannelH.UpdateRegion)
			gb.PUT("/channels/:id/group", gbChannelH.UpdateGroup)
			gb.POST("/channels/:id/reset", gbChannelH.Reset)

			// 媒体节点（简单列表，非分页）
			gb.GET("/media-servers", gbMediaServerH.List)
			gb.POST("/media-servers", gbMediaServerH.Create)
			gb.GET("/media-servers/:id", gbMediaServerH.Get)
			gb.PUT("/media-servers/:id", gbMediaServerH.Update)
			gb.DELETE("/media-servers/:id", gbMediaServerH.Delete)
			gb.GET("/media-servers/:id/check", gbMediaServerH.Check)
			gb.GET("/media-servers/:id/info", gbMediaServerH.ServerInfo)

			// 业务分组（树）
			gb.GET("/groups/tree", gbGroupH.Tree)
			gb.GET("/groups", gbGroupH.List)
			gb.POST("/groups", gbGroupH.Create)
			gb.PUT("/groups/:id", gbGroupH.Update)
			gb.DELETE("/groups/:id", gbGroupH.Delete)

			// 订阅记录
			gb.GET("/subscribes", gbSubscribeH.Page)
			gb.POST("/subscribes", gbSubscribeH.Create)
			gb.DELETE("/subscribes/:id", gbSubscribeH.Delete)

			// 信令日志（无 dept_id，不走 DataScope）
			gb.GET("/sip-logs", gbSipLogH.Page)
			gb.GET("/sip-logs/stats", gbSipLogH.Stats)
			gb.GET("/sip-logs/:id", gbSipLogH.Get)
		}

		// 播放 / 云台 / 语音：路由较多，拆到 gb_live.go
		registerGbLiveRoutes(wAuth)
		// 回放 / 录像 / 报警 / 位置 / 流代理 / 级联 / 语音 Agent：拆到 gb_ops.go
		registerGbOpsRoutes(wAuth)

		// ═══ IOT 管理 ═══
		// 前缀 /web/iot 与设备端 /iot 是两套鉴权模型，详见 router/device.go。
		wiot := wAuth.Group("/iot")
		{
			// 产品
			wiot.GET("/products", iotProductH.Page)
			wiot.GET("/products/:id", iotProductH.Get)
			wiot.POST("/products", iotProductH.Create)
			wiot.PUT("/products/:id", iotProductH.Update)
			wiot.DELETE("/products/:id", iotProductH.Delete)
			wiot.POST("/products/:id/share-qr", iotProductH.GenerateShareQR)
			wiot.GET("/products/:id/service-groups", iotProductH.GetServiceGroups)
			wiot.PUT("/products/:id/service-groups", iotProductH.BindServiceGroups)

			// 服务组
			wiot.GET("/service-groups", iotServiceGroupH.Page)
			wiot.GET("/service-groups/:id", iotServiceGroupH.Get)
			wiot.POST("/service-groups", iotServiceGroupH.Create)
			wiot.PUT("/service-groups/:id", iotServiceGroupH.Update)
			wiot.DELETE("/service-groups/:id", iotServiceGroupH.Delete)
			wiot.POST("/service-groups/:id/reset-credentials", iotServiceGroupH.ResetCredentials)
			wiot.PUT("/service-groups/:id/items", iotServiceGroupH.ReplaceItems)

			// 设备与账号
			wiot.GET("/devices", iotDeviceH.Page)
			wiot.GET("/devices/:id", iotDeviceH.Get)
			wiot.POST("/devices/:id/accounts", iotDeviceH.CreateAccount)
			wiot.PUT("/devices/:id/accounts/:accountId", iotDeviceH.UpdateAccount)
			wiot.DELETE("/devices/:id/accounts/:accountId", iotDeviceH.DeleteAccount)
			wiot.POST("/devices/:id/activate", iotDeviceH.Activate)
			wiot.POST("/devices/:id/reset-secret", iotDeviceH.ResetSecret)
			wiot.PUT("/devices/:id/product", iotDeviceH.SetProduct)
		}
	}
}
