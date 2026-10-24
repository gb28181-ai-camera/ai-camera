package router

import (
	rpchandler "github.com/boticz/ai-camera-backend/internal/api/handler/rpc"
	"github.com/gin-gonic/gin"
)

// registerRpcRoutes 注册内部服务间 RPC 端点。
//
// ⚠️ 独立路由组：**不挂 authMW / casbinMW / OperationLog**。
// 调用方是 MQTT broker 等内部进程，没有用户会话；鉴权用 X-RPC-Token
// （见 handler/rpc/mqtt_handler.go）。生产环境必须用网络策略限制来源，
// 不要把这些路径暴露到公网。
func registerRpcRoutes(r *gin.Engine) {
	mqttH := rpchandler.NewMqttAuthHandler()
	turnH := rpchandler.NewTurnHandler()

	rpc := r.Group("/rpc")
	{
		mqtt := rpc.Group("/mqtt")
		{
			mqtt.POST("/auth", mqttH.Authenticate)
			mqtt.POST("/acl", mqttH.CheckACL)
		}
		// TURN 凭据签发：coturn 没有鉴权回调，只能由后端签发短期 HMAC 凭据。
		rpc.POST("/turn/credentials", turnH.IssueCredential)
	}
}
