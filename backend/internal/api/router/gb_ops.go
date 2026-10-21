package router

import (
	handler "github.com/boticz/ai-camera-backend/internal/api/handler/web"
	"github.com/gin-gonic/gin"
)

// registerGbOpsRoutes 注册国标视频 Wave 2 的运维类路由。
//
// 由 web.go 在 wAuth 组（authMW → casbinMW → OperationLog）上调用，
// 本函数不自行创建中间件组，避免绕过鉴权。
//
// 路由与 docs/开发文档/后端开发/00-接口契约.md §10.3 / §10.4 / §10.5 /
// §10.6（流代理、推流）/ §10.7 / §10.9 一一对应。
func registerGbOpsRoutes(wAuth *gin.RouterGroup) {
	gbPlaybackH := handler.NewGbPlaybackHandler()
	gbCloudRecordH := handler.NewGbCloudRecordHandler()
	gbRecordPlanH := handler.NewGbRecordPlanHandler()
	gbAlarmH := handler.NewGbAlarmHandler()
	gbPositionH := handler.NewGbPositionHandler()
	gbStreamProxyH := handler.NewGbStreamProxyHandler()
	gbStreamPushH := handler.NewGbStreamPushHandler()
	gbPlatformH := handler.NewGbPlatformHandler()
	voiceAgentH := handler.NewVoiceAgentHandler()

	gb := wAuth.Group("/gb")
	{
		// ── 回放与下载（§10.3）──
		pb := gb.Group("/playback")
		{
			pb.GET("/records", gbPlaybackH.QueryRecord)
			pb.POST("", gbPlaybackH.StartPlayback)
			pb.POST("/control", gbPlaybackH.Control)
			pb.POST("/stop", gbPlaybackH.StopPlayback)
			// 下载任务的静态 /download 写在 /download/:id 之前，语义更清晰。
			pb.POST("/download", gbPlaybackH.StartDownload)
			pb.GET("/download", gbPlaybackH.PageDownload)
			pb.DELETE("/download/:id", gbPlaybackH.StopDownload)
		}

		// ── 云端录像（§10.4）──
		// /dates 与 /collect 是静态段，必须写在 /:id/... 之前。
		gb.GET("/cloud-records/dates", gbCloudRecordH.ListDates)
		gb.GET("/cloud-records", gbCloudRecordH.Page)
		gb.DELETE("/cloud-records", gbCloudRecordH.Delete)
		gb.PUT("/cloud-records/collect", gbCloudRecordH.Collect)
		gb.GET("/cloud-records/:id/play", gbCloudRecordH.PlayPath)
		gb.GET("/cloud-records/:id/download", gbCloudRecordH.Download)

		// ── 录像计划（§10.4）──
		gb.GET("/record-plans", gbRecordPlanH.Page)
		gb.GET("/record-plans/:id", gbRecordPlanH.Get)
		gb.POST("/record-plans", gbRecordPlanH.Create)
		gb.PUT("/record-plans/:id", gbRecordPlanH.Update)
		gb.DELETE("/record-plans/:id", gbRecordPlanH.Delete)
		gb.PUT("/record-plans/:id/channels", gbRecordPlanH.LinkChannels)

		// ── 国标报警与移动位置（§10.5）──
		gb.GET("/alarms", gbAlarmH.Page)
		gb.GET("/alarms/:id", gbAlarmH.Get)
		gb.POST("/alarms/:id/snapshot", gbAlarmH.Snapshot)
		gb.PUT("/alarms/:id/handle", gbAlarmH.Handle)
		gb.DELETE("/alarms", gbAlarmH.Clear)

		gb.GET("/positions/latest", gbPositionH.Latest)
		gb.GET("/positions/realtime", gbPositionH.Realtime)
		gb.GET("/positions/history", gbPositionH.History)
		gb.POST("/positions/subscribe", gbPositionH.Subscribe)

		// ── 拉流代理 / 推流（§10.6）──
		gb.GET("/stream-proxies", gbStreamProxyH.Page)
		gb.POST("/stream-proxies", gbStreamProxyH.Create)
		gb.PUT("/stream-proxies/:id", gbStreamProxyH.Update)
		gb.DELETE("/stream-proxies/:id", gbStreamProxyH.Delete)
		gb.POST("/stream-proxies/:id/start", gbStreamProxyH.Start)
		gb.POST("/stream-proxies/:id/stop", gbStreamProxyH.Stop)

		gb.GET("/stream-pushes", gbStreamPushH.Page)
		gb.POST("/stream-pushes", gbStreamPushH.Create)
		gb.PUT("/stream-pushes/:id", gbStreamPushH.Update)
		gb.DELETE("/stream-pushes/:id", gbStreamPushH.Delete)
		gb.POST("/stream-pushes/:id/start", gbStreamPushH.Start)
		gb.POST("/stream-pushes/:id/stop", gbStreamPushH.Stop)

		// ── 国标级联（§10.7）──
		gb.GET("/platforms", gbPlatformH.Page)
		gb.GET("/platforms/:id", gbPlatformH.Get)
		gb.POST("/platforms", gbPlatformH.Create)
		gb.PUT("/platforms/:id", gbPlatformH.Update)
		gb.DELETE("/platforms/:id", gbPlatformH.Delete)
		gb.GET("/platforms/:id/channels", gbPlatformH.PageChannel)
		gb.POST("/platforms/:id/channels", gbPlatformH.AddChannel)
		gb.DELETE("/platforms/:id/channels", gbPlatformH.RemoveChannel)
		gb.PUT("/platforms/:id/channels/:channelId", gbPlatformH.UpdateChannelCustom)
		gb.PUT("/platforms/:id/groups", gbPlatformH.SetGroups)
		gb.PUT("/platforms/:id/regions", gbPlatformH.SetRegions)
	}

	// ── 语音 Agent 配置（§10.9）──
	// 挂在现有 /web/settings 之下，与个推/短信同级：存储复用 ac_thirdparty_config。
	wa := wAuth.Group("/settings/voice-agent")
	{
		wa.GET("", voiceAgentH.Get)
		wa.PUT("", voiceAgentH.Update)
		wa.GET("/providers", voiceAgentH.ListProviders)
		wa.POST("/test", voiceAgentH.Test)
		wa.POST("/apply", voiceAgentH.Apply)
	}
}
