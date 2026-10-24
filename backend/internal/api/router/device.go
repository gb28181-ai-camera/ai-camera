package router

import (
	iothandler "github.com/boticz/ai-camera-backend/internal/api/handler/iot"
	"github.com/gin-gonic/gin"
)

// registerIotRoutes 注册设备端 IOT 接口。
//
// ⚠️ 这里**不能**挂 authMW / casbinMW：设备是无人值守进程，没有用户会话，
// 也不在 Casbin 的角色模型里。鉴权由 handler 读取
// X-Device-Code / X-Device-Secret 后交给 service 校验（device_secret 比对）。
//
// 路由前缀 /iot 与 Web 端的 /web/iot 刻意分开：
// 两者鉴权模型完全不同，混在同一组里迟早会被误挂中间件。
func registerIotRoutes(r *gin.Engine) {
	h := iothandler.NewDeviceHandler()

	g := r.Group("/iot")
	{
		g.POST("/ota", h.OTA)
		g.POST("/activate", h.Activate)
	}
}
