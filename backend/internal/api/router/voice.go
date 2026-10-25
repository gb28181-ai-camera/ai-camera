package router

import (
	webhandler "github.com/boticz/ai-camera-backend/internal/api/handler/web"
	iothandler "github.com/boticz/ai-camera-backend/internal/api/handler/iot"
	rpchandler "github.com/boticz/ai-camera-backend/internal/api/handler/rpc"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/gin-gonic/gin"
)

// registerVoiceRoutes 注册语音 WebSocket 网关与内部接口。
//
// ⚠️ 这些路由**不挂** authMW / casbinMW：
//   - /web/voice/ws、/app/voice/ws 用一次性 ticket（浏览器 WS 无法带 JWT header）；
//   - /device/voice/ws 用 HTTP Basic（产品×服务凭据）；
//   - /internal/* 用 INTERNAL_MCP_TOKEN（Pipecat → Go，内网调用）。
//
// ticket 签发端点仍在 /web|app 的认证组内（见 web.go / app.go）。
func registerVoiceRoutes(r *gin.Engine) {
	gateway := voiceagent.DefaultGateway()

	r.GET("/web/voice/ws", func(c *gin.Context) { gateway.HandleWeb(c, "web") })
	r.GET("/app/voice/ws", func(c *gin.Context) { gateway.HandleWeb(c, "app") })

	deviceH := iothandler.NewVoiceHandler()
	r.GET("/device/voice/ws", deviceH.WS)

	// 实时音视频：浏览器 WebSocket 无法带 JWT header，同样使用一次性 ticket。
	avH := webhandler.NewAvRoomHandler()
	r.GET("/web/av/ws", avH.ViewerWS)

	internalH := rpchandler.NewVoiceHandler()
	internal := r.Group("/internal")
	{
		internal.POST("/mcp", internalH.MCP)
		internal.POST("/voice/memory/retrieve", internalH.MemoryRetrieve)
		internal.POST("/voice/memory/write", internalH.MemoryWrite)
	}
}
