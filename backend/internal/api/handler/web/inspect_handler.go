package handler

import (
	"errors"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/inspect"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// InspectHandler 巡检中心处理器（管理后台）。
type InspectHandler struct {
	svc *inspect.InspectService
}

// NewInspectHandler 构造巡检中心处理器。
func NewInspectHandler() *InspectHandler {
	return &InspectHandler{svc: inspect.NewInspectService()}
}

// inspectFail 统一巡检域错误响应：记录不存在返回 404，其余业务/参数错误返回 400。
func inspectFail(c *gin.Context, err error, notFoundMsg string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.NotFound(c, notFoundMsg)
		return
	}
	utils.BadRequest(c, err.Error())
}

// ════════════════════════════════════════════════════════════════
// 巡检任务
// ════════════════════════════════════════════════════════════════

// PageTasks 巡检任务分页列表
// @Summary 巡检任务分页列表
// @ID pageInspectTasks
// @Tags Inspect
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（任务名称/巡检位置）"
// @Param region_id query int false "所属区域ID"
// @Param inspector_id query int false "巡检人用户ID"
// @Param status query string false "状态 pending/published/finished/canceled"
// @Param start_dt query string false "巡检时间起（YYYY-MM-DD）"
// @Param end_dt query string false "巡检时间止（YYYY-MM-DD）"
// @Success 200 {object} response.RPageInspectTask
// @Router /web/inspects/tasks [get]
func (h *InspectHandler) PageTasks(c *gin.Context) {
	var req request.PageInspectTaskReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageTasks(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		inspectFail(c, err, "巡检任务不存在")
		return
	}
	utils.Success(c, resp)
}

// GetTask 巡检任务详情
// @Summary 巡检任务详情
// @ID getInspectTask
// @Tags Inspect
// @Param id path int true "巡检任务ID"
// @Success 200 {object} response.RInspectTask
// @Router /web/inspects/tasks/{id} [get]
func (h *InspectHandler) GetTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的巡检任务ID")
		return
	}
	m, err := h.svc.GetTask(c.Request.Context(), id)
	if err != nil {
		inspectFail(c, err, "巡检任务不存在")
		return
	}
	utils.Success(c, m)
}

// CreateTask 创建巡检任务
// @Summary 创建巡检任务
// @ID createInspectTask
// @Tags Inspect
// @Param body body request.CreateInspectTaskReq true "巡检任务信息"
// @Success 200 {object} response.RInspectTask
// @Router /web/inspects/tasks [post]
func (h *InspectHandler) CreateTask(c *gin.Context) {
	var req request.CreateInspectTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.CreateTask(c.Request.Context(), &req)
	if err != nil {
		inspectFail(c, err, "巡检任务不存在")
		return
	}
	utils.Success(c, m)
}

// UpdateTask 更新巡检任务
// @Summary 更新巡检任务
// @ID updateInspectTask
// @Tags Inspect
// @Param id path int true "巡检任务ID"
// @Param body body request.UpdateInspectTaskReq true "巡检任务信息"
// @Success 200 {object} response.BaseResponse
// @Router /web/inspects/tasks/{id} [put]
func (h *InspectHandler) UpdateTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的巡检任务ID")
		return
	}
	var req request.UpdateInspectTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateTask(c.Request.Context(), id, &req); err != nil {
		inspectFail(c, err, "巡检任务不存在")
		return
	}
	utils.Success(c, nil)
}

// MarkTaskRead 标记巡检任务消息通知已读
//
// @Summary 标记巡检任务消息通知已读
// @Description PC 端「查看」巡检任务后调用，将该任务的消息通知由未读转为已读；重复调用幂等，已读时不再写库
// @ID markInspectTaskRead
// @Tags Inspect
// @Accept json
// @Produce json
// @Param id path int true "巡检任务ID"
// @Success 200 {object} response.RBool "操作成功，data 固定为 true"
// @Failure 400 {object} response.BaseResponse "参数错误"
// @Failure 404 {object} response.BaseResponse "巡检任务不存在"
// @Router /web/inspects/tasks/{id}/read [put]
func (h *InspectHandler) MarkTaskRead(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的巡检任务ID")
		return
	}
	if err := h.svc.MarkTaskRead(c.Request.Context(), id); err != nil {
		inspectFail(c, err, "巡检任务不存在")
		return
	}
	utils.Success(c, true)
}

// DeleteTask 删除巡检任务
// @Summary 删除巡检任务
// @ID deleteInspectTask
// @Tags Inspect
// @Param id path int true "巡检任务ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/inspects/tasks/{id} [delete]
func (h *InspectHandler) DeleteTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的巡检任务ID")
		return
	}
	if err := h.svc.DeleteTask(c.Request.Context(), id); err != nil {
		inspectFail(c, err, "巡检任务不存在")
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 巡检记录
// ════════════════════════════════════════════════════════════════

// PageRecords 巡检记录分页列表
// @Summary 巡检记录分页列表
// @ID pageInspectRecords
// @Tags Inspect
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（任务名称/巡检位置）"
// @Param region_id query int false "所属区域ID"
// @Param inspector_id query int false "巡检人用户ID"
// @Param start_dt query string false "巡检时间起（YYYY-MM-DD）"
// @Param end_dt query string false "巡检时间止（YYYY-MM-DD）"
// @Success 200 {object} response.RPageInspectRecord
// @Router /web/inspects/records [get]
func (h *InspectHandler) PageRecords(c *gin.Context) {
	var req request.PageInspectRecordReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageRecords(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		inspectFail(c, err, "巡检记录不存在")
		return
	}
	utils.Success(c, resp)
}

// GetRecord 巡检记录详情
// @Summary 巡检记录详情
// @ID getInspectRecord
// @Tags Inspect
// @Param id path int true "巡检记录ID"
// @Success 200 {object} response.RInspectRecord
// @Router /web/inspects/records/{id} [get]
func (h *InspectHandler) GetRecord(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的巡检记录ID")
		return
	}
	m, err := h.svc.GetRecord(c.Request.Context(), id)
	if err != nil {
		inspectFail(c, err, "巡检记录不存在")
		return
	}
	utils.Success(c, m)
}
