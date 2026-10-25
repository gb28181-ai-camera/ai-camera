package handler

import (
	"errors"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	// 空导入：本文件仅在 Swagger 注解中引用 response 包类型
	// （@Success {object} response.RPageRiskRecord），代码中不直接构造响应体，
	// 显式导入以便 swag 解析注解中的类型引用。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/service/risk"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// RiskHandler AI 风险接口。路由前缀 /web/risks。
type RiskHandler struct {
	svc *risk.RiskService
}

// NewRiskHandler 构造风险处理器。
func NewRiskHandler() *RiskHandler {
	return &RiskHandler{svc: risk.NewRiskService()}
}

// respondErr 将风险服务层错误映射为合适的 HTTP 状态码与响应体。
// 采用方法而非包级函数，避免与其它 handler 文件的辅助函数重名。
func (h *RiskHandler) respondErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, risk.ErrNotFound):
		utils.NotFound(c, err.Error())
	case errors.Is(err, risk.ErrDeviceNotFound),
		errors.Is(err, risk.ErrRiskTypeNotFound),
		errors.Is(err, risk.ErrAccurateRequired),
		errors.Is(err, risk.ErrAlreadyHandled),
		errors.Is(err, risk.ErrInvalidAccurate):
		utils.BadRequest(c, err.Error())
	default:
		utils.ServerError(c, err.Error())
	}
}

// Page 分页查询风险记录
//
// @Summary      分页查询风险记录
// @Description  支持关键字（设备编号/设备名称）、风险类型、区域、设备、处理状态、检测准确性、已读状态与发生时间范围过滤；默认按发生时间倒序，列表返回风险类型名称、设备名称/编号、区域名称
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Param        page             query  int     false  "页码，默认 1"
// @Param        page_size        query  int     false  "每页条数，默认 20"
// @Param        keyword          query  string  false  "关键字（模糊匹配设备编号/设备名称）"
// @Param        risk_type_id     query  int     false  "风险类型ID"
// @Param        region_id        query  int     false  "区域ID"
// @Param        device_id        query  int     false  "设备ID"
// @Param        device_code      query  string  false  "设备编号"
// @Param        status           query  string  false  "处理状态 pending/handled"
// @Param        detect_accurate  query  string  false  "检测准确 yes/no"
// @Param        read             query  bool    false  "是否已读 true/false"
// @Param        start_dt         query  string  false  "发生时间起（2026-01-01 00:00:00）"
// @Param        end_dt           query  string  false  "发生时间止（2026-12-31 23:59:59）"
// @Success      200  {object}  response.RPageRiskRecord  "风险分页列表"
// @Failure      400  {object}  response.BaseResponse     "参数错误"
// @Failure      500  {object}  response.BaseResponse     "服务异常"
// @Router       /web/risks [get]
// @ID           pageRisks
func (h *RiskHandler) Page(c *gin.Context) {
	var req request.PageRiskReq
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

// Get 查询风险详情
//
// @Summary      查询风险详情
// @Description  返回风险记录及其风险类型、设备、区域信息
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "风险记录ID"
// @Success      200  {object}  response.RRiskRecord   "风险详情"
// @Failure      400  {object}  response.BaseResponse  "参数错误"
// @Failure      404  {object}  response.BaseResponse  "记录不存在"
// @Router       /web/risks/{id} [get]
// @ID           getRisk
func (h *RiskHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "风险记录ID无效")
		return
	}

	rec, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, rec)
}

// MarkRead 标记风险通知已读
//
// @Summary      标记风险通知已读
// @Description  仅更新已读状态与已读时间；重复调用幂等，不返回错误
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "风险记录ID"
// @Success      200  {object}  response.BaseResponse  "操作成功"
// @Failure      400  {object}  response.BaseResponse  "参数错误"
// @Failure      404  {object}  response.BaseResponse  "记录不存在"
// @Router       /web/risks/{id}/read [put]
// @ID           markRiskRead
func (h *RiskHandler) MarkRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "风险记录ID无效")
		return
	}

	if err := h.svc.MarkRead(c.Request.Context(), id); err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// UpdateAccurate 确认检测准确性
//
// @Summary      确认检测准确性
// @Description  accurate=no（误报）直接办结：status=handled、read=true、处理结果写「误报，无需处理」；accurate=yes（确认风险）仅置 read=true，status 保持 pending 等待处理
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "风险记录ID"
// @Param        body  body      request.UpdateAccurateReq true  "确认请求"
// @Success      200   {object}  response.BaseResponse     "操作成功"
// @Failure      400   {object}  response.BaseResponse     "参数错误"
// @Failure      404   {object}  response.BaseResponse     "记录不存在"
// @Router       /web/risks/{id}/accurate [put]
// @ID           updateRiskAccurate
func (h *RiskHandler) UpdateAccurate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "风险记录ID无效")
		return
	}

	var req request.UpdateAccurateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.svc.UpdateAccurate(c.Request.Context(), id, req.Accurate, c.GetString("user_name")); err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// Handle 处理风险
//
// @Summary      处理风险
// @Description  需先确认「检测准确=是」，否则返回「请先确认检测准确后方可处理」；已处理的风险不可重复处理
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Param        id    path      int                    true  "风险记录ID"
// @Param        body  body      request.HandleRiskReq  true  "处理请求"
// @Success      200   {object}  response.BaseResponse  "操作成功"
// @Failure      400   {object}  response.BaseResponse  "参数错误"
// @Failure      404   {object}  response.BaseResponse  "记录不存在"
// @Router       /web/risks/{id}/handle [put]
// @ID           handleRisk
func (h *RiskHandler) Handle(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "风险记录ID无效")
		return
	}

	var req request.HandleRiskReq
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

// Delete 删除风险记录
//
// @Summary      删除风险记录
// @Description  软删除风险记录
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "风险记录ID"
// @Success      200  {object}  response.BaseResponse  "操作成功"
// @Failure      400  {object}  response.BaseResponse  "参数错误"
// @Failure      404  {object}  response.BaseResponse  "记录不存在"
// @Router       /web/risks/{id} [delete]
// @ID           deleteRisk
func (h *RiskHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "风险记录ID无效")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.respondErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// PendingCount 已确认待处理风险数量
//
// @Summary      已确认待处理风险数量
// @Description  统计 status=pending 的风险记录总数（含未读），用于首页/移动端角标
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RCount  "待处理数量"
// @Router       /web/risks/pending-count [get]
// @ID           riskPendingCount
func (h *RiskHandler) PendingCount(c *gin.Context) {
	count, err := h.svc.PendingCount(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, count)
}

// Summary 风险总览
//
// @Summary      风险总览指标
// @Description  返回总数/待处理/已处理/高风险/未读/检测准确分布/今日新增，口径与风险列表的数据权限一致
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RRiskSummary
// @Router       /web/risks/summary [get]
// @ID           riskSummary
func (h *RiskHandler) Summary(c *gin.Context) {
	resp, err := h.svc.Summary(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// Report 设备上报风险
//
// @Summary      设备上报风险
// @Description  设备端上报接口，**使用 X-Api-Key 请求头认证（ApiKey 认证，无用户登录态）**。依据 device_code 定位设备、risk_type_code 定位风险类型；记录初始状态为 pending、未读、检测准确待人工确认，并冗余设备编号/区域/位置/风险等级
// @Tags         Risk
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body      request.ReportRiskReq  true  "上报请求"
// @Success      200   {object}  response.RRiskRecord    "上报成功，返回新建的风险记录"
// @Failure      400   {object}  response.BaseResponse   "参数错误（含设备不存在/风险类型不存在）"
// @Failure      401   {object}  response.BaseResponse   "API Key 无效"
// @Failure      500   {object}  response.BaseResponse   "服务异常"
// @Router       /web/risks/report [post]
// @ID           reportRisk
func (h *RiskHandler) Report(c *gin.Context) {
	var req request.ReportRiskReq
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
