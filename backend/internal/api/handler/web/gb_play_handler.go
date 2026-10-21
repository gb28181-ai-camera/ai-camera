package handler

import (
	"strconv"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标实时播放
//
// 路由参数统一为 :channelId（数据库主键），不是国标编码（契约 §10.0）。
// ════════════════════════════════════════════════════════════════

// GbPlayHandler 实时播放处理器。
type GbPlayHandler struct {
	svc *gbsvc.GbPlayService
}

// NewGbPlayHandler 构造实时播放处理器。
func NewGbPlayHandler() *GbPlayHandler {
	return &GbPlayHandler{svc: gbsvc.NewGbPlayService()}
}

// gbChannelIDParam 解析路径中的通道主键（:channelId）。
func gbChannelIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("channelId"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的通道ID")
		return 0, false
	}
	return id, true
}

// Play 发起实时点播
//
// @Summary      发起实时点播
// @Description  分配 SSRC 并向设备发 INVITE，等待 ZLM 建流后返回 WebRTC/HTTP-FLV/HLS 地址。同一通道已有在线流时直接复用
// @Tags         GbPlay
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                     true  "通道ID"
// @Param        body       body      request.GbPlayReq      false "播放参数（protocol 缺省 webrtc）"
// @Success      200  {object}  response.RGbPlay
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/play/{channelId} [post]
// @ID           playGbChannel
func (h *GbPlayHandler) Play(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}

	var req request.GbPlayReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}

	resp, err := h.svc.Play(c.Request.Context(), channelID, &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// StopPlay 停止实时点播
//
// @Summary      停止实时点播
// @Description  向设备发 BYE、关闭 ZLM 媒体流并归还 SSRC。stream_id 可省略（按通道停止）
// @Tags         GbPlay
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                 true  "通道ID"
// @Param        body       body      request.GbPlayReq  false "stream_id（可选）"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/play/{channelId}/stop [post]
// @ID           stopGbPlay
func (h *GbPlayHandler) StopPlay(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}

	var req request.GbPlayReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	streamID := strings.TrimSpace(req.StreamID)
	if streamID == "" {
		streamID = strings.TrimSpace(c.Query("stream_id"))
	}

	if err := h.svc.StopPlay(c.Request.Context(), channelID, streamID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbPlaySessionNotFound)
		return
	}
	utils.Success(c, nil)
}

// PlayStatus 查询点播状态
//
// @Summary      查询通道点播状态
// @Description  playing 表示平台有会话；ready 表示 ZLM 实时确认流在线。未开播时返回 playing=false 而不是 404
// @Tags         GbPlay
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbPlayStatus
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/play/{channelId}/status [get]
// @ID           getGbPlayStatus
func (h *GbPlayHandler) PlayStatus(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.PlayStatus(c.Request.Context(), channelID)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, resp)
}

// Snapshot 抓图
//
// @Summary      通道抓图
// @Description  调用 ZLM getSnap 抓取当前画面并保存到上传目录，返回可直接访问的图片 URL。未开播时先自动点播
// @Tags         GbPlay
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbSnapshot
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/play/{channelId}/snapshot [post]
// @ID           snapshotGbChannel
func (h *GbPlayHandler) Snapshot(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.Snapshot(c.Request.Context(), channelID)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}
