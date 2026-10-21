package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标报警（设备 MANSCDP Alarm 上报）
// ════════════════════════════════════════════════════════════════

// GbAlarmHandler 国标报警处理器。
type GbAlarmHandler struct {
	svc *gbsvc.GbAlarmService
}

// NewGbAlarmHandler 构造国标报警处理器。
func NewGbAlarmHandler() *GbAlarmHandler {
	return &GbAlarmHandler{svc: gbsvc.NewGbAlarmService()}
}

// Page 国标报警分页
//
// @Summary      国标报警分页
// @Description  支持关键字、设备/通道、报警类型、级别、处理状态与时间范围筛选；返回设备/通道名称
// @Tags         GbAlarm
// @Accept       json
// @Produce      json
// @Param        page            query     int     false  "页码"     default(1)
// @Param        page_size       query     int     false  "每页条数"  default(20)
// @Param        keyword         query     string  false  "关键字(描述/国标编码/类型)"
// @Param        device_id       query     int     false  "设备ID"
// @Param        channel_id      query     int     false  "通道ID"
// @Param        alarm_type      query     string  false  "报警类型编码"
// @Param        alarm_priority  query     string  false  "报警级别 1-4"
// @Param        handled         query     bool    false  "是否已处理"
// @Param        start_dt        query     string  false  "开始时间"
// @Param        end_dt          query     string  false  "结束时间"
// @Success      200  {object}  response.RPageGbAlarm
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/alarms [get]
// @ID           pageGbAlarms
func (h *GbAlarmHandler) Page(c *gin.Context) {
	var req request.PageGbAlarmReq
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

// Get 国标报警详情
//
// @Summary      国标报警详情
// @Description  返回报警完整字段与设备/通道名称
// @Tags         GbAlarm
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "报警ID"
// @Success      200  {object}  response.RGbAlarm
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/alarms/{id} [get]
// @ID           getGbAlarm
func (h *GbAlarmHandler) Get(c *gin.Context) {
	id, ok := gbPathID(c, "报警")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbAlarmNotFound)
		return
	}
	utils.Success(c, resp)
}

// Snapshot 报警抓图
//
// @Summary      报警抓图
// @Description  对报警关联通道调用 ZLM getSnap；通道当前无视频流时返回明确错误。图片落 uploads/snapshots 并回写 snap_path
// @Tags         GbAlarm
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "报警ID"
// @Success      200  {object}  response.RGbSnapshot
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/alarms/{id}/snapshot [post]
// @ID           snapshotGbAlarm
func (h *GbAlarmHandler) Snapshot(c *gin.Context) {
	id, ok := gbPathID(c, "报警")
	if !ok {
		return
	}
	resp, err := h.svc.Snapshot(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbAlarmNotFound, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, resp)
}

// Handle 处理国标报警
//
// @Summary      处理国标报警
// @Description  handled 缺省为 true；remark 非空时覆盖备注
// @Tags         GbAlarm
// @Accept       json
// @Produce      json
// @Param        id    path      int                        true  "报警ID"
// @Param        body  body      request.GbAlarmHandleReq    true  "处理参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/alarms/{id}/handle [put]
// @ID           handleGbAlarm
func (h *GbAlarmHandler) Handle(c *gin.Context) {
	id, ok := gbPathID(c, "报警")
	if !ok {
		return
	}
	var req request.GbAlarmHandleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	operator := utils.NewDataScope(c).UserName
	if err := h.svc.Handle(c.Request.Context(), id, &req, operator); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbAlarmNotFound)
		return
	}
	utils.Success(c, nil)
}

// Clear 批量清除国标报警
//
// @Summary      批量清除国标报警
// @Description  软删除选中的报警
// @Tags         GbAlarm
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbAlarmClearReq  true  "报警ID集合"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/alarms [delete]
// @ID           clearGbAlarms
func (h *GbAlarmHandler) Clear(c *gin.Context) {
	var req request.GbAlarmClearReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Clear(c.Request.Context(), req.IDs); err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}
