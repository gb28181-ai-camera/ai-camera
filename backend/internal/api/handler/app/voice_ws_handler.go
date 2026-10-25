package app

import (
	"errors"
	"io"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// VoiceWsHandler 移动端语音 WebSocket ticket 处理器。
type VoiceWsHandler struct{}

// NewVoiceWsHandler 构造处理器。
func NewVoiceWsHandler() *VoiceWsHandler { return &VoiceWsHandler{} }

// WsTicket 申请 App 语音 WS ticket
//
// @Summary      申请语音 WS ticket
// @Description  使用当前 JWT 身份签发 60 秒一次性 ticket；可选绑定设备编号
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        body  body      request.VoiceWsTicketReq  false  "可选参数"
// @Success      200  {object}  response.RVoiceWsTicket
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Router       /app/voice/ws-ticket [post]
// @ID           createAppVoiceWsTicket
func (h *VoiceWsHandler) WsTicket(c *gin.Context) {
	var req request.VoiceWsTicketReq
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	deviceID, err := voiceagent.ResolveDeviceID(c.Request.Context(), req.DeviceCode)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	ticket, claims, err := voiceagent.DefaultTicketService().Issue(voiceagent.TicketClaims{
		UserID:    c.GetInt64("user_id"),
		UserName:  c.GetString("user_name"),
		RoleCode:  c.GetString("role_code"),
		DeptID:    c.GetInt64("dept_id"),
		DataPerm:  c.GetString("data_perm"),
		DeviceID:  deviceID,
		ClientID:  strings.TrimSpace(req.ClientID),
		Kind:      "app",
		AgentCode: strings.TrimSpace(req.AgentCode),
	})
	if err != nil {
		utils.ServerError(c, "签发 ticket 失败: "+err.Error())
		return
	}
	utils.Success(c, response.VoiceWsTicketResp{
		Ticket:    ticket,
		ExpiresIn: int(time.Until(time.Unix(claims.ExpiresAt, 0)).Seconds()),
		WsURL:     appWsURL(c, "/app/voice/ws"),
	})
}

func appWsURL(c *gin.Context, path string) string {
	scheme := "ws"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}
	if c.Request.Host == "" {
		return path
	}
	return scheme + "://" + c.Request.Host + path
}
