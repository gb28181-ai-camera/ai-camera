package handler

import (
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标语音广播 / 对讲
//
// 一期默认半双工（设计 §8.1.1）；Pipecat 控制面不可达时降级为纯对讲。
// ════════════════════════════════════════════════════════════════

// GbVoiceHandler 语音广播/对讲处理器。
type GbVoiceHandler struct {
	svc *gbsvc.GbVoiceService
}

// NewGbVoiceHandler 构造语音处理器。
func NewGbVoiceHandler() *GbVoiceHandler {
	return &GbVoiceHandler{svc: gbsvc.NewGbVoiceService()}
}

// gbVoiceSessionID 从请求体或 query 取 session_id。
func gbVoiceSessionID(c *gin.Context) string {
	var req request.GbVoiceStopReq
	if c.Request.ContentLength > 0 {
		// 停止接口允许空 body；JSON 非法时仍退回 query，避免前端
		// 因为多传了一个字段就停不掉会话。
		_ = c.ShouldBindJSON(&req)
	}
	if v := strings.TrimSpace(req.SessionID); v != "" {
		return v
	}
	return strings.TrimSpace(c.Query("session_id"))
}

// StartBroadcast 开始语音广播
//
// @Summary      开始语音广播
// @Description  向设备发 Broadcast INVITE（m=audio PCMA/8000），落库 ac_voice_session 并尝试 ZLM startSendRtp 回推。要求通道 enable_broadcast=true
// @Tags         GbVoice
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                  true  "通道ID"
// @Param        body       body      request.GbVoiceStartReq  false "启动参数（广播可省略）"
// @Success      200  {object}  response.RGbVoiceSession
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/voice/{channelId}/broadcast/start [post]
// @ID           startGbBroadcast
func (h *GbVoiceHandler) StartBroadcast(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbVoiceStartReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	resp, err := h.svc.StartBroadcast(c.Request.Context(), channelID, c.GetInt64("user_id"))
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// StopBroadcast 停止语音广播
//
// @Summary      停止语音广播
// @Description  session_id 可放在请求体或 query；缺省时按通道停止进行中的广播
// @Tags         GbVoice
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                   true  "通道ID"
// @Param        body       body      request.GbVoiceStopReq  false "会话标识（可选）"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/voice/{channelId}/broadcast/stop [post]
// @ID           stopGbBroadcast
func (h *GbVoiceHandler) StopBroadcast(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	sessionID := gbVoiceSessionID(c)

	var err error
	if sessionID == "" {
		err = h.svc.StopBroadcastByChannel(c.Request.Context(), channelID)
	} else {
		err = h.svc.StopBroadcast(c.Request.Context(), sessionID)
	}
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbVoiceSessionNotFound, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, nil)
}

// StartTalk 开始语音对讲
//
// @Summary      开始语音对讲
// @Description  向设备发 Talk INVITE 并落库。agent_enabled=true 时尝试接入 Pipecat，控制面不可达则降级为纯对讲（响应 agent_enabled=false）
// @Tags         GbVoice
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                  true  "通道ID"
// @Param        body       body      request.GbVoiceStartReq  false "是否接入语音 Agent"
// @Success      200  {object}  response.RGbVoiceSession
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/voice/{channelId}/talk/start [post]
// @ID           startGbTalk
func (h *GbVoiceHandler) StartTalk(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbVoiceStartReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	resp, err := h.svc.StartTalk(c.Request.Context(), channelID, c.GetInt64("user_id"), req.AgentEnabled)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// StopTalk 停止语音对讲
//
// @Summary      停止语音对讲
// @Description  session_id 可放在请求体或 query；缺省时按通道停止进行中的对讲
// @Tags         GbVoice
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                   true  "通道ID"
// @Param        body       body      request.GbVoiceStopReq  false "会话标识（可选）"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/voice/{channelId}/talk/stop [post]
// @ID           stopGbTalk
func (h *GbVoiceHandler) StopTalk(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	sessionID := gbVoiceSessionID(c)

	var err error
	if sessionID == "" {
		err = h.svc.StopTalkByChannel(c.Request.Context(), channelID)
	} else {
		err = h.svc.StopTalk(c.Request.Context(), sessionID)
	}
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbVoiceSessionNotFound, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, nil)
}
