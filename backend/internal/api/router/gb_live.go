package router

import (
	handler "github.com/boticz/ai-camera-backend/internal/api/handler/web"
	"github.com/gin-gonic/gin"
)

// registerGbLiveRoutes 注册国标实时播放 / 云台 / 语音路由。
//
// 由 web.go 在主装配时调用，传入的必须是已挂好
// authMW → casbinMW → OperationLog 的 /web 组：
// 本函数不重复添加中间件，也不创建新的顶层组。
//
// 路径与接口契约 §10.2 逐条对齐，且 ac_menu_api 必须使用同一组路径 ——
// 否则超管访问会返回 403 且无提示（契约 §15.6）。
func registerGbLiveRoutes(wAuth *gin.RouterGroup) {
	playH := handler.NewGbPlayHandler()
	ptzH := handler.NewGbPtzHandler()
	voiceH := handler.NewGbVoiceHandler()

	gb := wAuth.Group("/gb")
	{
		// ── 实时播放 ──────────────────────────────────────────
		gb.POST("/play/:channelId", playH.Play)
		gb.POST("/play/:channelId/stop", playH.StopPlay)
		gb.GET("/play/:channelId/status", playH.PlayStatus)
		gb.POST("/play/:channelId/snapshot", playH.Snapshot)

		// ── 云台（带外 MESSAGE，不是 INFO）────────────────────
		gb.POST("/ptz/:channelId", ptzH.Control)
		gb.GET("/ptz/:channelId/presets", ptzH.ListPresets)
		gb.POST("/ptz/:channelId/presets", ptzH.AddPreset)
		gb.DELETE("/ptz/:channelId/presets/:presetId", ptzH.DeletePreset)
		gb.POST("/ptz/:channelId/presets/:presetId/goto", ptzH.GotoPreset)
		gb.POST("/ptz/:channelId/cruise", ptzH.Cruise)
		gb.POST("/ptz/:channelId/scan", ptzH.Scan)
		gb.POST("/ptz/:channelId/wiper", ptzH.Wiper)

		// ── 语音广播 / 对讲 ──────────────────────────────────
		gb.POST("/voice/:channelId/broadcast/start", voiceH.StartBroadcast)
		gb.POST("/voice/:channelId/broadcast/stop", voiceH.StopBroadcast)
		gb.POST("/voice/:channelId/talk/start", voiceH.StartTalk)
		gb.POST("/voice/:channelId/talk/stop", voiceH.StopTalk)
	}
}
