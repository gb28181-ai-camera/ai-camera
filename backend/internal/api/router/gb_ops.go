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
	opsExtraH := handler.NewGbOpsExtraHandler()
	streamImportH := handler.NewGbStreamImportHandler()
	cloudMergeH := handler.NewGbCloudMergeHandler()
	shareH := handler.NewGbShareHandler()

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
		// 多段合并任务：静态段 merge-tasks 必须在 /:id/... 之前。
		gb.GET("/cloud-records/merge-tasks", cloudMergeH.PageMergeTasks)
		gb.POST("/cloud-records/merge-tasks", cloudMergeH.CreateMergeTask)
		gb.GET("/cloud-records/merge-tasks/:id", cloudMergeH.GetMergeTask)
		gb.GET("/cloud-records/merge-tasks/:id/download", cloudMergeH.DownloadMergeTask)
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
		// SSE 实时报警：静态段必须写在 /:id 之前。
		gb.GET("/alarms/stream", opsExtraH.AlarmsStream)
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
		// 播放地址：仅当流确实存在于媒体节点时才返回，否则 409。
		gb.GET("/stream-proxies/:id/play", opsExtraH.PlayStreamProxy)

		gb.GET("/stream-pushes", gbStreamPushH.Page)
		gb.POST("/stream-pushes", gbStreamPushH.Create)
		// 批量导入：静态段必须写在 /:id 之前。
		gb.POST("/stream-pushes/import", streamImportH.ImportStreamPushes)
		gb.PUT("/stream-pushes/:id", gbStreamPushH.Update)
		gb.DELETE("/stream-pushes/:id", gbStreamPushH.Delete)
		gb.POST("/stream-pushes/:id/start", gbStreamPushH.Start)
		gb.POST("/stream-pushes/:id/stop", gbStreamPushH.Stop)
		gb.GET("/stream-pushes/:id/play", opsExtraH.PlayStreamPush)

		// ── 国标级联（§10.7）──
		// 运行时动作放在静态段，必须在 /:id 之前注册（gin 支持静态与参数同级，
		// 但按「静态优先」的阅读习惯排列，避免以后新增 :id 路由时误判冲突）。
		gb.GET("/platforms/cascade/status", gbPlatformH.CascadeStatus)
		gb.POST("/platforms/cascade/start", gbPlatformH.StartCascade)
		gb.POST("/platforms/cascade/stop", gbPlatformH.StopCascade)

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
		gb.POST("/platforms/:id/catalog-push", gbPlatformH.PushCatalog)

		// ── 媒体节点监控（WVP 对齐）──
		// 挂在既有 /media-servers/:id 之下，与 check/info 同级。
		gb.GET("/media-servers/:id/metrics", opsExtraH.Metrics)
		gb.GET("/media-servers/:id/metrics/history", opsExtraH.MetricsHistory)
		gb.GET("/media-servers/:id/streams", opsExtraH.Streams)

		// ── 平台信息 / 日志 ──
		gb.GET("/platform-info", opsExtraH.PlatformInfo)
		gb.GET("/logs", opsExtraH.Logs)
		// SSE 实时日志：静态段与 /logs 同级，无冲突。
		gb.GET("/logs/stream", opsExtraH.LogsStream)

		// ── 通道免登录分享（管理端）──
		// 匿名端在 /public/gb/shares/:token，由 registerGbPublicRoutes 注册，
		// 不经过 authMW / casbinMW。
		gb.GET("/channels/:id/share-tokens", shareH.ListShareTokens)
		gb.POST("/channels/:id/share-tokens", shareH.CreateShareToken)
		gb.DELETE("/channels/:id/share-tokens/:tokenId", shareH.RevokeShareToken)
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

// registerGbPublicRoutes 注册国标免登录分享的匿名路由。
//
// ★ 刻意不挂 authMW / casbinMW：分享链接给没有平台账号的人使用，
// 令牌本身即凭证。安全边界全部由 GbShareService 的 hash 校验、过期与
// 吊销、协议白名单、双层限频承担。该路由由 SetupRouter 在根引擎上注册。
func registerGbPublicRoutes(r *gin.Engine) {
	shareH := handler.NewGbShareHandler()
	public := r.Group("/public/gb")
	{
		public.GET("/shares/:token", shareH.PublicShareInfo)
		public.POST("/shares/:token/play", shareH.PublicSharePlay)
	}
}
