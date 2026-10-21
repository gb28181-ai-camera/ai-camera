package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 录像计划（周计划）
//
// ★ 本期只做配置 CRUD，不做定时调度：到点调用 ZLM startRecord /
// 设备 RecordCmd 的调度器属于后续批次。
// ════════════════════════════════════════════════════════════════

// GbRecordPlanHandler 录像计划处理器。
type GbRecordPlanHandler struct {
	svc *gbsvc.GbRecordPlanService
}

// NewGbRecordPlanHandler 构造录像计划处理器。
func NewGbRecordPlanHandler() *GbRecordPlanHandler {
	return &GbRecordPlanHandler{svc: gbsvc.NewGbRecordPlanService()}
}

// Page 录像计划分页
//
// @Summary      录像计划分页
// @Description  返回计划基本信息、时间段数量与已绑定通道数量
// @Tags         GbRecordPlan
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "页码"     default(1)
// @Param        page_size  query     int     false  "每页条数"  default(20)
// @Param        keyword    query     string  false  "计划名称关键字"
// @Success      200  {object}  response.RPageGbRecordPlan
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/record-plans [get]
// @ID           pageGbRecordPlans
func (h *GbRecordPlanHandler) Page(c *gin.Context) {
	var req request.PageGbRecordPlanReq
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

// Get 录像计划详情
//
// @Summary      录像计划详情
// @Description  返回计划、一周时间段与已绑定通道（含设备名称）
// @Tags         GbRecordPlan
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "计划ID"
// @Success      200  {object}  response.RGbRecordPlan
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/record-plans/{id} [get]
// @ID           getGbRecordPlan
func (h *GbRecordPlanHandler) Get(c *gin.Context) {
	id, ok := gbPathID(c, "录像计划")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbRecordPlanNotFound)
		return
	}
	utils.Success(c, resp)
}

// Create 新增录像计划
//
// @Summary      新增录像计划
// @Description  同时写入一周时间段；week_day 0 为周日，时间为 HH:mm:ss
// @Tags         GbRecordPlan
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateGbRecordPlanReq  true  "计划与时间段"
// @Success      200  {object}  response.RGbRecordPlanCreated
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/record-plans [post]
// @ID           createGbRecordPlan
func (h *GbRecordPlanHandler) Create(c *gin.Context) {
	var req request.CreateGbRecordPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, m)
}

// Update 编辑录像计划
//
// @Summary      编辑录像计划
// @Description  增量更新；items 传 null/缺省表示不改时间段，传 [] 表示清空
// @Tags         GbRecordPlan
// @Accept       json
// @Produce      json
// @Param        id    path      int                          true  "计划ID"
// @Param        body  body      request.UpdateGbRecordPlanReq  true  "计划与时间段"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/record-plans/{id} [put]
// @ID           updateGbRecordPlan
func (h *GbRecordPlanHandler) Update(c *gin.Context) {
	id, ok := gbPathID(c, "录像计划")
	if !ok {
		return
	}
	var req request.UpdateGbRecordPlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbRecordPlanNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除录像计划
//
// @Summary      删除录像计划
// @Description  仍被通道引用时拒绝删除；删除时级联清理时间段
// @Tags         GbRecordPlan
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "计划ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/record-plans/{id} [delete]
// @ID           deleteGbRecordPlan
func (h *GbRecordPlanHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "录像计划")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbRecordPlanNotFound, gbsvc.ErrGbRecordPlanInUse)
		return
	}
	utils.Success(c, nil)
}

// LinkChannels 计划绑定通道
//
// @Summary      计划绑定通道
// @Description  整体替换：提交的通道绑定到该计划，原先绑定但未提交的通道解绑；channel_ids 传 [] 表示全部解绑
// @Tags         GbRecordPlan
// @Accept       json
// @Produce      json
// @Param        id    path      int                                    true  "计划ID"
// @Param        body  body      request.LinkGbRecordPlanChannelsReq     true  "通道ID集合"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/record-plans/{id}/channels [put]
// @ID           linkGbRecordPlanChannels
func (h *GbRecordPlanHandler) LinkChannels(c *gin.Context) {
	id, ok := gbPathID(c, "录像计划")
	if !ok {
		return
	}
	var req request.LinkGbRecordPlanChannelsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.LinkChannels(c.Request.Context(), id, req.ChannelIDs); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbRecordPlanNotFound)
		return
	}
	utils.Success(c, nil)
}
