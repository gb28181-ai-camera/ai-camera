package router

import (
	handler "github.com/boticz/ai-camera-backend/internal/api/handler/web"
	"github.com/gin-gonic/gin"
)

// registerMediaRoutes 注册 ZLMediaKit 回调路由。
//
// ⚠️ 这里**绝对不能**挂 authMW / casbinMW：ZLM 是外部进程，没有 JWT，
// 也不在 Casbin 的用户/角色模型里。鉴权完全依赖查询参数 secret
// （与 ac_gb_media_server.secret 匹配），由 hook.Handler 内部完成。
//
// 端点清单 = ZLM 的全部 10 个 hook（设计 §7.3 / 接口契约 §10.11）。
// 注意没有 on_media_arrival —— 那是 WVP 的内部事件，ZLM 配置里配不出来。
func registerMediaRoutes(r *gin.Engine) {
	mediaH := handler.NewMediaHookHandler()

	g := r.Group("/media/hook")
	{
		g.POST("/on_server_keepalive", mediaH.OnServerKeepalive)
		g.POST("/on_server_started", mediaH.OnServerStarted)
		g.POST("/on_publish", mediaH.OnPublish)
		g.POST("/on_play", mediaH.OnPlay)
		g.POST("/on_stream_changed", mediaH.OnStreamChanged)
		g.POST("/on_stream_none_reader", mediaH.OnStreamNoneReader)
		g.POST("/on_stream_not_found", mediaH.OnStreamNotFound)
		g.POST("/on_send_rtp_stopped", mediaH.OnSendRTPStopped)
		g.POST("/on_rtp_server_timeout", mediaH.OnRTPServerTimeout)
		g.POST("/on_record_mp4", mediaH.OnRecordMP4)
	}
}
