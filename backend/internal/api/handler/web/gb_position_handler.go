package handler

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标移动位置（GIS 轨迹）
// ════════════════════════════════════════════════════════════════

// GbPositionHandler 移动位置处理器。
type GbPositionHandler struct {
	svc *gbsvc.GbPositionService
}

// NewGbPositionHandler 构造移动位置处理器。
func NewGbPositionHandler() *GbPositionHandler {
	return &GbPositionHandler{svc: gbsvc.NewGbPositionService()}
}

// Latest 通道最新位置
//
// @Summary      通道最新位置
// @Description  返回指定通道最近一次上报的位置
// @Tags         GbPosition
// @Accept       json
// @Produce      json
// @Param        channel_id  query     int  true  "通道ID"
// @Success      200  {object}  response.RGbPosition
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/positions/latest [get]
// @ID           getGbPositionLatest
func (h *GbPositionHandler) Latest(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Query("channel_id"), 10, 64)
	if err != nil || channelID <= 0 {
		utils.BadRequest(c, "无效的通道ID")
		return
	}
	resp, err := h.svc.Latest(c.Request.Context(), channelID)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPositionNotFound)
		return
	}
	utils.Success(c, resp)
}

// Realtime 实时位置列表
//
// @Summary      实时位置列表
// @Description  每个通道取最新一条位置（DISTINCT ON），用于 GIS 实时图层
// @Tags         GbPosition
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RGbPositionList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/positions/realtime [get]
// @ID           listGbPositionsRealtime
func (h *GbPositionHandler) Realtime(c *gin.Context) {
	items, err := h.svc.Realtime(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, items)
}

// History 位置历史
//
// @Summary      位置历史
// @Description  按通道与时间范围分页查询轨迹点
// @Tags         GbPosition
// @Accept       json
// @Produce      json
// @Param        page           query     int     false  "页码"     default(1)
// @Param        page_size      query     int     false  "每页条数"  default(20)
// @Param        device_id      query     int     false  "设备ID"
// @Param        channel_id     query     int     false  "通道ID"
// @Param        gb_channel_id  query     string  false  "通道国标编码"
// @Param        start_dt       query     string  false  "开始时间"
// @Param        end_dt         query     string  false  "结束时间"
// @Success      200  {object}  response.RGbPositionHistory
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/positions/history [get]
// @ID           pageGbPositionHistory
func (h *GbPositionHandler) History(c *gin.Context) {
	var req request.GbPositionHistoryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.History(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Subscribe 订阅移动位置
//
// @Summary      订阅移动位置
// @Description  向设备发送 mobile_position 的 SUBSCRIBE（presence）；interval 缺省 60 秒
// @Tags         GbPosition
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbPositionSubscribeReq  true  "订阅参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/positions/subscribe [post]
// @ID           subscribeGbPosition
func (h *GbPositionHandler) Subscribe(c *gin.Context) {
	var req request.GbPositionSubscribeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Subscribe(c.Request.Context(), req.ChannelID, req.Interval); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound, gbsvc.ErrGbDeviceNotConfigured)
		return
	}
	utils.Success(c, nil)
}
