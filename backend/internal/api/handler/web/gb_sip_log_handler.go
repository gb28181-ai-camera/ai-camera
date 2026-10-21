package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标信令日志
//
// ★ 该表无 dept_id，接口不做 DataScope；权限由 gb:sip-log:list 控制。
// ════════════════════════════════════════════════════════════════

// GbSipLogHandler 信令日志处理器。
type GbSipLogHandler struct {
	svc *gbsvc.GbSipLogService
}

// NewGbSipLogHandler 构造信令日志处理器。
func NewGbSipLogHandler() *GbSipLogHandler {
	return &GbSipLogHandler{svc: gbsvc.NewGbSipLogService()}
}

// Page 信令日志分页列表
//
// @Summary      信令日志分页列表
// @Description  支持设备、通道、方向、SIP 方法、命令类型、结果与时间区间筛选；该表无部门字段，不做数据权限过滤
// @Tags         GbSipLog
// @Accept       json
// @Produce      json
// @Param        page           query     int     false  "页码"        default(1)
// @Param        page_size      query     int     false  "每页条数"     default(20)
// @Param        gb_device_id   query     string  false  "设备国标编码"
// @Param        gb_channel_id  query     string  false  "通道国标编码"
// @Param        direction      query     string  false  "方向 in/out"
// @Param        method         query     string  false  "SIP方法 REGISTER/MESSAGE/INVITE..."
// @Param        cmd_type       query     string  false  "MANSCDP命令类型 Catalog/Keepalive/Alarm"
// @Param        result         query     string  false  "结果 success/failed/rejected"
// @Param        start_dt       query     string  false  "开始时间"
// @Param        end_dt         query     string  false  "结束时间"
// @Success      200  {object}  response.RPageGbSipLog
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/sip-logs [get]
// @ID           pageGbSipLogs
func (h *GbSipLogHandler) Page(c *gin.Context) {
	var req request.PageGbSipLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Stats 信令日志统计
//
// @Summary      信令日志统计
// @Description  返回总量、成功/失败/拒绝数、收发方向计数，以及按 SIP 方法与命令类型的分组统计
// @Tags         GbSipLog
// @Accept       json
// @Produce      json
// @Param        gb_device_id  query     string  false  "设备国标编码"
// @Param        direction     query     string  false  "方向 in/out"
// @Param        method        query     string  false  "SIP方法"
// @Param        cmd_type      query     string  false  "命令类型"
// @Param        result        query     string  false  "结果"
// @Param        start_dt      query     string  false  "开始时间"
// @Param        end_dt        query     string  false  "结束时间"
// @Success      200  {object}  response.RGbSipLogStats
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/sip-logs/stats [get]
// @ID           gbSipLogStats
func (h *GbSipLogHandler) Stats(c *gin.Context) {
	var req request.GbSipLogStatsReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Stats(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Get 信令日志详情
//
// @Summary      信令日志详情
// @Description  按 ID 查询日志，含截断存储的报文内容
// @Tags         GbSipLog
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "日志ID"
// @Success      200  {object}  response.RGbSipLog
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/sip-logs/{id} [get]
// @ID           getGbSipLog
func (h *GbSipLogHandler) Get(c *gin.Context) {
	id, ok := gbPathID(c, "信令日志")
	if !ok {
		return
	}
	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbSipLogNotFound)
		return
	}
	utils.Success(c, m)
}
