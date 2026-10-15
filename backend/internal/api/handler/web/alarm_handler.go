package handler

import (
	"errors"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	// 空导入：本文件仅在 Swagger 注解中引用 response 包类型
	// （@Success {object} response.RPageAlarmRecord），代码中不直接构造响应体，
	// 显式导入以便 swag 解析注解中的类型引用。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/service/alarm"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// AlarmHandler 设备告警接口。路由前缀 /web/alarms。
type AlarmHandler struct {
	svc *alarm.AlarmService
}

// NewAlarmHandler 构造告警处理器。
func NewAlarmHandler() *AlarmHandler {
	return &AlarmHandler{svc: alarm.NewAlarmService()}
}

// respondErr 将告警服务层错误映射为合适的 HTTP 状态码与响应体。
func (h *AlarmHandler) respondErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, alarm.ErrNotFound):
		utils.NotFound(c, err.Error())
	case errors.Is(err, alarm.ErrDeviceNotFound),
		errors.Is(err, alarm.ErrAlarmTypeNotFound),
		errors.Is(err, alarm.ErrAlreadyHandled):
		utils.BadRequest(c, err.Error())
	default:
		utils.ServerError(c, err.Error())
	}
}

// Page 分页查询设备告警
//
// @Summary      分页查询设备告警
// @Description  支持关键字（设备编号/设备名称）、告警类型、区域、设备、处理状态、已读状态与告警时间范围过滤；默认按告警时间倒序，列表返回告警类型名称、设备名称/编号、区域名称
// @Tags         Alarm
// @Accept       json
// @Produce      json
// @Param        page          query  int     false  "页码，默认 1"
// @Param        page_size     query  int     false  "每页条数，默认 20"
// @Param        keyword       query  string  false  "关键字（模糊匹配设备编号/设备名称）"
// @Param        alarm_type_id query  int     false  "告警类型ID"
// @Param        region_id     query  int     false  "区域ID"
// @Param        device_id     query  int     false  "设备ID"
// @Param        device_code   query  string  false  "设备编号"
// @Param        status        query  string  false  "处理状态 pending/handled"
// @Param        read          query  bool    false  "是否已读 true/false"
// @Param        start_dt      query  string  false  "告警时间起（2026-01-01 00:00:00）"
// @Param        end_dt        query  string  false  "告警时间止（2026-12-31 23:59:59）"
// @Success      200  {object}  response.RPageAlarmRecord  "告警分页列表"
// @Failure      400  {object}  response.BaseResponse      "参数错误"
// @Failure      500  {object}  response.BaseResponse      "服务异常"
// @Router       /web/alarms [get]
// @ID           pageAlarms
func (h *AlarmHandler) Page(c *gin.Context) {
	var req request.PageAlarmReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "查询参数错误: "+err.Error())
		return
	}

	data, err := h.svc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, data)
}

// Get 查询告警详情
//
// @Summary      查询告警详情
// @Description  返回告警记录及其告警类型、设备、区域信息
// @Tags         Alarm
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "告警记录ID"
// @Success      200  {object}  response.RAlarmRecord  "告警详情"
// @Failure      400  {object}  response.BaseResponse  "参数错误"
// @Failure      404  {object}  response.BaseResponse  "记录不存在"
// @Router       /web/alarms/{id} [get]
// @ID           getAlarm
func (h *AlarmHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "告警记录ID无效")
		return
	}

	rec, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, rec)
}

// MarkRead 标记告警通知已读
//
// @Summary      标记告警通知已读
// @Description  仅更新已读状态与已读时间；重复调用幂等，不返回错误
// @Tags         Alarm
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "告警记录ID"
// @Success      200  {object}  response.BaseResponse  "操作成功"
// @Failure      400  {object}  response.BaseResponse  "参数错误"
// @Failure      404  {object}  response.BaseResponse  "记录不存在"
// @Router       /web/alarms/{id}/read [put]
// @ID           markAlarmRead
func (h *AlarmHandler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "告警记录ID无效")
		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), id); err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// Handle 处理告警
//
// @Summary      处理告警
// @Description  填写处理结果与处理图片并办结；告警无需「检测准确」确认，但已处理的告警不可重复处理
// @Tags         Alarm
// @Accept       json
// @Produce      json
// @Param        id    path      int                     true  "告警记录ID"
// @Param        body  body      request.HandleAlarmReq  true  "处理请求"
// @Success      200   {object}  response.BaseResponse   "操作成功"
// @Failure      400   {object}  response.BaseResponse   "参数错误"
// @Failure      404   {object}  response.BaseResponse   "记录不存在"
// @Router       /web/alarms/{id}/handle [put]
// @ID           handleAlarm
func (h *AlarmHandler) Handle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "告警记录ID无效")
		return
	}

	var req request.HandleAlarmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.svc.Handle(c.Request.Context(), id, &req, c.GetString("user_name")); err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除告警记录
//
// @Summary      删除告警记录
// @Description  软删除告警记录
// @Tags         Alarm
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "告警记录ID"
// @Success      200  {object}  response.BaseResponse  "操作成功"
// @Failure      400  {object}  response.BaseResponse  "参数错误"
// @Failure      404  {object}  response.BaseResponse  "记录不存在"
// @Router       /web/alarms/{id} [delete]
// @ID           deleteAlarm
func (h *AlarmHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "告警记录ID无效")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// PendingCount 未处理告警数量
//
// @Summary      未处理告警数量
// @Description  统计 status=pending 的告警记录总数，用于首页/移动端角标
// @Tags         Alarm
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RCount  "未处理数量"
// @Router       /web/alarms/pending-count [get]
// @ID           alarmPendingCount
func (h *AlarmHandler) PendingCount(c *gin.Context) {
	count, err := h.svc.PendingCount(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, count)
}

// Report 设备上报告警
//
// @Summary      设备上报告警
// @Description  设备端上报接口，**使用 X-Api-Key 请求头认证（ApiKey 认证，无用户登录态）**。依据 device_code 定位设备、alarm_type_code 定位告警类型；告警等级取上报值，未传时回落为告警类型配置的等级
// @Tags         Alarm
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body      request.ReportAlarmReq  true  "上报请求"
// @Success      200   {object}  response.RAlarmRecord    "上报成功，返回新建的告警记录"
// @Failure      400   {object}  response.BaseResponse    "参数错误（含设备不存在/告警类型不存在）"
// @Failure      401   {object}  response.BaseResponse    "API Key 无效"
// @Failure      500   {object}  response.BaseResponse    "服务异常"
// @Router       /web/alarms/report [post]
// @ID           reportAlarm
func (h *AlarmHandler) Report(c *gin.Context) {
	var req request.ReportAlarmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	rec, err := h.svc.Report(c.Request.Context(), &req)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, rec)
}
