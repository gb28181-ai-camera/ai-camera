package app

import (
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// PushHandler App 推送令牌注册。
type PushHandler struct {
	tokens *repository.PushTokenRepo
}

// NewPushHandler 构造处理器。
func NewPushHandler() *PushHandler {
	return &PushHandler{tokens: repository.NewPushTokenRepo()}
}

// Register 注册个推 CID
//
// @Summary      App 上报推送 CID
// @Description  幂等注册；同一 CID 换用户时迁移到新用户。设备离线呼叫时用于唤醒 App
// @Tags         AppPush
// @Accept       json
// @Produce      json
// @Param        body  body  request.RegisterPushTokenReq  true  "CID"
// @Success      200   {object}  response.BaseResponse
// @Failure      400   {object}  response.BaseResponse
// @Router       /app/push-tokens [post]
// @ID           registerPushToken
func (h *PushHandler) Register(c *gin.Context) {
	var req request.RegisterPushTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	cid := strings.TrimSpace(req.CID)
	if cid == "" {
		utils.BadRequest(c, "cid 不能为空")
		return
	}
	if err := h.tokens.Register(c.Request.Context(), c.GetInt64("user_id"), cid,
		strings.TrimSpace(req.Platform), strings.TrimSpace(req.AppVersion)); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// Delete 删除个推 CID
//
// @Summary      App 注销推送 CID
// @Description  退出登录/关闭推送时调用；只删除当前用户名下的 CID
// @Tags         AppPush
// @Accept       json
// @Produce      json
// @Param        cid  path  string  true  "CID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /app/push-tokens/{cid} [delete]
// @ID           deletePushToken
func (h *PushHandler) Delete(c *gin.Context) {
	cid := strings.TrimSpace(c.Param("cid"))
	if cid == "" {
		utils.BadRequest(c, "cid 不能为空")
		return
	}
	if err := h.tokens.Delete(c.Request.Context(), c.GetInt64("user_id"), cid); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}
