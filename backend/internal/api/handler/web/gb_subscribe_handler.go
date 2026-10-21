package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标订阅记录
// ════════════════════════════════════════════════════════════════

// GbSubscribeHandler 订阅记录处理器。
type GbSubscribeHandler struct {
	svc *gbsvc.GbSubscribeService
}

// NewGbSubscribeHandler 构造订阅记录处理器。
func NewGbSubscribeHandler() *GbSubscribeHandler {
	return &GbSubscribeHandler{svc: gbsvc.NewGbSubscribeService()}
}

// Page 订阅记录分页列表
//
// @Summary      订阅记录分页列表
// @Description  支持按设备国标编码、订阅类型、状态筛选；返回关联设备名称
// @Tags         GbSubscribe
// @Accept       json
// @Produce      json
// @Param        page          query     int     false  "页码"        default(1)
// @Param        page_size     query     int     false  "每页条数"     default(20)
// @Param        gb_device_id  query     string  false  "设备国标编码"
// @Param        type          query     string  false  "订阅类型 catalog/alarm/mobile_position"
// @Param        status        query     string  false  "状态 active/expired/failed"
// @Success      200  {object}  response.RPageGbSubscribe
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/subscribes [get]
// @ID           pageGbSubscribes
func (h *GbSubscribeHandler) Page(c *gin.Context) {
	var req request.PageGbSubscribeReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Create 新增订阅记录
//
// @Summary      新增订阅记录
// @Description  仅登记订阅关系（不发送 SIP SUBSCRIBE）；expires 缺省 3600 秒，自动计算 expire_dt
// @Tags         GbSubscribe
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateGbSubscribeReq  true  "订阅信息"
// @Success      200  {object}  response.RGbSubscribe
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/subscribes [post]
// @ID           createGbSubscribe
func (h *GbSubscribeHandler) Create(c *gin.Context) {
	var req request.CreateGbSubscribeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, m)
}

// Delete 删除订阅记录
//
// @Summary      删除订阅记录
// @Description  软删除订阅记录；不会向设备发送取消订阅报文
// @Tags         GbSubscribe
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "订阅记录ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/subscribes/{id} [delete]
// @ID           deleteGbSubscribe
func (h *GbSubscribeHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "订阅记录")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbSubscribeNotFound)
		return
	}
	utils.Success(c, nil)
}
