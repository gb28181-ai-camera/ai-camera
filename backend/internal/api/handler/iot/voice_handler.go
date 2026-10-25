package iot

import (
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/gin-gonic/gin"
)

// VoiceHandler AIChat 设备语音 WebSocket 处理器。
//
// 鉴权在 Handler/Service 内完成（HTTP Basic：产品×服务凭据），
// 路由层不挂 JWT / Casbin —— 与 /iot/ota 的模型一致。
type VoiceHandler struct {
	gateway *voiceagent.Gateway
}

// NewVoiceHandler 构造设备语音处理器。
func NewVoiceHandler() *VoiceHandler {
	return &VoiceHandler{gateway: voiceagent.DefaultGateway()}
}

// WS AIChat 设备语音 WebSocket
//
// @Summary      AIChat 设备语音 WebSocket
// @Description  HTTP Basic（产品×服务凭据）后反代到 Pipecat；可选 device_code 查询参数标明设备身份
// @Tags         VoiceAgent
// @Produce      json
// @Param        device_code  query  string  false  "设备编号"
// @Success      101  {string}  string  "Switching Protocols"
// @Failure      401  {object}  response.BaseResponse
// @Failure      403  {object}  response.BaseResponse
// @Router       /device/voice/ws [get]
// @ID           deviceVoiceWs
func (h *VoiceHandler) WS(c *gin.Context) {
	h.gateway.HandleDevice(c)
}
