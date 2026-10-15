package app

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	// 空导入：本文件仅在 Swagger 注解中引用 response 包类型
	// （@Success {object} response.RAppHome 等），代码中不直接构造响应体，
	// 显式导入以便 swag 解析注解中的类型引用。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"
	appsvc "github.com/boticz/ai-camera-backend/internal/service/app"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// AppHandler 移动端业务处理器（无状态）。
//
// 所有方法均从 Auth 中间件写入的上下文中取当前用户：
// user_id / user_name / data_perm / dept_id / cross_dept_ids。
type AppHandler struct {
	svc *appsvc.AppService
}

// NewAppHandler 构造移动端业务处理器。
func NewAppHandler() *AppHandler {
	return &AppHandler{svc: appsvc.NewAppService()}
}

// ────────────────────────────────────────────────────────────────
// 菜单 / 首页 / 待办
// ────────────────────────────────────────────────────────────────

// Menus 获取移动端菜单树。
// @Summary 移动端菜单树
// @Description 仅返回平台为 mobile 且启用、未隐藏的菜单，按当前用户三路角色（用户/部门/岗位）授权过滤后组树
// @Tags App
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.RAppMenuList "菜单树，children 为子菜单"
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/menus [get]
func (h *AppHandler) Menus(c *gin.Context) {
	menus, err := h.svc.Menus(c.Request.Context(), c.GetString("user_name"))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, menus)
}

// Home 获取移动端首页仪表盘数据。
// @Summary 移动端首页仪表盘
// @Description 返回待处理风险/告警数、我的待巡检任务数、今日新增风险/告警数、设备在线统计、最新 5 条风险与未读通知数
// @Tags App
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.RAppHome "首页数据"
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/home [get]
func (h *AppHandler) Home(c *gin.Context) {
	data, err := h.svc.Home(c.Request.Context(), c.GetInt64("user_id"), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, data)
}

// Todos 获取移动端待办汇总。
// @Summary 移动端待办汇总
// @Description 返回四类待办：risk（待处理风险）、alarm（待处理告警）、inspect（我的待巡检任务）、notification（未读通知）
// @Tags App
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.RAppTodoList "待办项列表"
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/todos [get]
func (h *AppHandler) Todos(c *gin.Context) {
	items, err := h.svc.Todos(c.Request.Context(), c.GetInt64("user_id"), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, items)
}

// ────────────────────────────────────────────────────────────────
// 风险
// ────────────────────────────────────────────────────────────────

// PageRisks 分页查询风险监控。
// @Summary 分页查询风险监控
// @Description 支持关键字、风险类型、区域、设备、处理状态、检测准确性、已读状态与发生时间范围过滤；含风险类型/设备/区域名称
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（设备编号/设备名称）"
// @Param risk_type_id query int false "风险类型ID"
// @Param region_id query int false "区域ID"
// @Param device_id query int false "设备ID"
// @Param device_code query string false "设备编号（精确匹配）"
// @Param status query string false "处理状态 pending/handled"
// @Param detect_accurate query string false "检测准确 yes/no"
// @Param read query bool false "是否已读"
// @Param start_dt query string false "发生时间起"
// @Param end_dt query string false "发生时间止"
// @Success 200 {object} response.RPageRiskRecord "分页风险列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/risks [get]
func (h *AppHandler) PageRisks(c *gin.Context) {
	var req request.PageRiskReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageRisks(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// GetRisk 获取风险详情。
// @Summary 获取风险详情
// @Description 返回风险记录详情，含风险类型、设备、区域关联对象
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险记录ID"
// @Success 200 {object} response.RRiskRecord "风险详情"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /app/risks/{id} [get]
func (h *AppHandler) GetRisk(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	data, err := h.svc.GetRisk(c.Request.Context(), id)
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, data)
}

// HandleRisk 处理风险。
// @Summary 处理风险
// @Description 提交处理结果与处理图片，风险置为 handled 并记录处理人/时间
// @Tags App
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险记录ID"
// @Param request body request.HandleRiskReq true "处理参数"
// @Success 200 {object} response.BaseResponse "处理成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/risks/{id}/handle [put]
func (h *AppHandler) HandleRisk(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req request.HandleRiskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.HandleRisk(c.Request.Context(), id, &req, c.GetString("user_name")); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// UpdateRiskAccurate 确认风险检测准确性。
// @Summary 确认风险检测准确性
// @Description accurate=yes 表示确认为真实风险（待处理）；accurate=no 表示误报，风险直接置为已处理
// @Tags App
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "风险记录ID"
// @Param request body request.UpdateAccurateReq true "确认参数"
// @Success 200 {object} response.BaseResponse "确认成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/risks/{id}/accurate [put]
func (h *AppHandler) UpdateRiskAccurate(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req request.UpdateAccurateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateRiskAccurate(c.Request.Context(), id, req.Accurate, c.GetString("user_name")); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ────────────────────────────────────────────────────────────────
// 告警
// ────────────────────────────────────────────────────────────────

// PageAlarms 分页查询设备告警。
// @Summary 分页查询设备告警
// @Description 支持关键字、告警类型、区域、设备、处理状态、已读状态与告警时间范围过滤；含告警类型/设备/区域名称
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（设备编号/设备名称）"
// @Param alarm_type_id query int false "告警类型ID"
// @Param region_id query int false "区域ID"
// @Param device_id query int false "设备ID"
// @Param device_code query string false "设备编号（精确匹配）"
// @Param status query string false "处理状态 pending/handled"
// @Param read query bool false "是否已读"
// @Param start_dt query string false "告警时间起"
// @Param end_dt query string false "告警时间止"
// @Success 200 {object} response.RPageAlarmRecord "分页告警列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/alarms [get]
func (h *AppHandler) PageAlarms(c *gin.Context) {
	var req request.PageAlarmReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageAlarms(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// HandleAlarm 处理设备告警。
// @Summary 处理设备告警
// @Description 提交处理结果与处理图片，告警置为 handled 并记录处理人/时间
// @Tags App
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "告警记录ID"
// @Param request body request.HandleAlarmReq true "处理参数"
// @Success 200 {object} response.BaseResponse "处理成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/alarms/{id}/handle [put]
func (h *AppHandler) HandleAlarm(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req request.HandleAlarmReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.HandleAlarm(c.Request.Context(), id, &req, c.GetString("user_name")); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ────────────────────────────────────────────────────────────────
// 巡检
// ────────────────────────────────────────────────────────────────

// PageInspectTasks 分页查询我的巡检任务。
// @Summary 分页查询我的巡检任务
// @Description 仅返回巡检人为当前登录用户的巡检任务；支持状态、区域、时间范围过滤
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（任务名称/巡检位置）"
// @Param region_id query int false "区域ID"
// @Param status query string false "任务状态 pending/published/finished/canceled"
// @Param start_dt query string false "巡检时间起"
// @Param end_dt query string false "巡检时间止"
// @Success 200 {object} response.RPageInspectTask "分页巡检任务列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/inspect-tasks [get]
func (h *AppHandler) PageInspectTasks(c *gin.Context) {
	var req request.PageInspectTaskReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageInspectTasks(c.Request.Context(), c.GetInt64("user_id"), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// GetInspectTask 获取巡检任务详情。
// @Summary 获取巡检任务详情
// @Description 返回巡检任务详情，含所属区域
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param id path int true "巡检任务ID"
// @Success 200 {object} response.RInspectTask "巡检任务详情"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /app/inspect-tasks/{id} [get]
func (h *AppHandler) GetInspectTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	data, err := h.svc.GetInspectTask(c.Request.Context(), id)
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, data)
}

// FinishInspectTask 完成巡检任务。
// @Summary 完成巡检任务
// @Description 提交巡检结果与结果图，任务置为 finished 并生成一条巡检记录
// @Tags App
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "巡检任务ID"
// @Param request body request.FinishInspectTaskReq true "巡检结果"
// @Success 200 {object} response.RInspectRecord "生成的巡检记录"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/inspect-tasks/{id}/finish [put]
func (h *AppHandler) FinishInspectTask(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req request.FinishInspectTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	record, err := h.svc.FinishInspectTask(c.Request.Context(), id, c.GetInt64("user_id"), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, record)
}

// ────────────────────────────────────────────────────────────────
// 设备
// ────────────────────────────────────────────────────────────────

// PageDevices 分页查询设备。
// @Summary 分页查询设备
// @Description 支持关键字、区域、运行状态、工作模式过滤；含区域/物联网卡/固件/AI模型名称
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（设备编号/设备名称）"
// @Param region_id query int false "区域ID"
// @Param status query string false "运行状态 online/offline"
// @Param device_mode query string false "设备模式（充电/全开/省电）"
// @Param work_mode query string false "设备工作模式"
// @Success 200 {object} response.RPageDevice "分页设备列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/devices [get]
func (h *AppHandler) PageDevices(c *gin.Context) {
	var req request.PageDeviceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageDevices(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// GetDevice 获取设备详情。
// @Summary 获取设备详情
// @Description 返回设备详情，含所属区域、物联网卡、固件与 AI 模型关联对象
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param id path int true "设备ID"
// @Success 200 {object} response.RDevice "设备详情"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /app/devices/{id} [get]
func (h *AppHandler) GetDevice(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	data, err := h.svc.GetDevice(c.Request.Context(), id)
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, data)
}

// ────────────────────────────────────────────────────────────────
// 通知
// ────────────────────────────────────────────────────────────────

// PageNotifications 分页查询我的通知。
// @Summary 分页查询我的通知
// @Description 仅返回接收人为当前登录用户的通知；支持类别与已读状态过滤
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param category query string false "通知类别 alarm/risk/inspect"
// @Param read query bool false "是否已读"
// @Success 200 {object} response.RPageNotification "分页通知列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/notifications [get]
func (h *AppHandler) PageNotifications(c *gin.Context) {
	var req request.PageNotificationReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageNotifications(c.Request.Context(), c.GetInt64("user_id"), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// MarkNotificationRead 标记通知已读。
// @Summary 标记通知已读
// @Description 仅允许标记当前登录用户自己的通知
// @Tags App
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知ID"
// @Success 200 {object} response.BaseResponse "标记成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/notifications/{id}/read [put]
func (h *AppHandler) MarkNotificationRead(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.MarkNotificationRead(c.Request.Context(), c.GetInt64("user_id"), id); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ────────────────────────────────────────────────────────────────
// 个人中心
// ────────────────────────────────────────────────────────────────

// GetProfile 获取当前用户资料。
// @Summary 获取当前用户资料
// @Description 返回当前登录用户的个人信息（不含密码哈希）
// @Tags App
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.RUser "用户资料"
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /app/profile [get]
func (h *AppHandler) GetProfile(c *gin.Context) {
	data, err := h.svc.GetProfile(c.Request.Context(), c.GetInt64("user_id"))
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, data)
}

// UpdateProfile 更新当前用户资料。
// @Summary 更新当前用户资料
// @Description 仅允许修改当前登录用户自己的资料
// @Tags App
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.UpdateProfileReq true "资料参数"
// @Success 200 {object} response.BaseResponse "更新成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/profile [put]
func (h *AppHandler) UpdateProfile(c *gin.Context) {
	var req request.UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateProfile(c.Request.Context(), c.GetInt64("user_id"), &req); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ChangePassword 修改当前用户密码。
// @Summary 修改当前用户密码
// @Description 校验原密码后更新为新密码
// @Tags App
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body request.ChangePasswordReq true "密码参数"
// @Success 200 {object} response.BaseResponse "修改成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/profile/password [put]
func (h *AppHandler) ChangePassword(c *gin.Context) {
	var req request.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), c.GetInt64("user_id"), &req); err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ────────────────────────────────────────────────────────────────
// 内部工具
// ────────────────────────────────────────────────────────────────

// parseID 解析路径参数 :id，非法时写入 400 并返回 false。
func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "非法的ID参数")
		return 0, false
	}
	return id, true
}
