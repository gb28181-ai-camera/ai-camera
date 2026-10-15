package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// OperationLogHandler 操作日志查询与清理。
type OperationLogHandler struct{}

// NewOperationLogHandler 构造操作日志处理器。
func NewOperationLogHandler() *OperationLogHandler { return &OperationLogHandler{} }

// Page 操作日志分页列表
// @Summary 操作日志分页列表
// @ID pageOperationLogs
// @Tags System
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param module query string false "系统模块"
// @Param operator query string false "操作人员"
// @Param action query string false "操作类型(新增/修改/删除)"
// @Param start_dt query string false "开始时间 2026-01-01 00:00:00"
// @Param end_dt query string false "结束时间 2026-12-31 23:59:59"
// @Success 200 {object} response.RPageOperationLog
// @Router /web/system/logs [get]
func (h *OperationLogHandler) Page(c *gin.Context) {
	var req request.PageOperationLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	resp, err := system.NewOperationLogService().Page(c.Request.Context(), &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, resp)
}

// Get 操作日志详情
// @Summary 操作日志详情
// @ID getOperationLog
// @Tags System
// @Param id path int true "日志ID"
// @Success 200 {object} response.ROperationLog
// @Router /web/system/logs/{id} [get]
func (h *OperationLogHandler) Get(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	log, err := system.NewOperationLogService().Get(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "日志不存在")
		return
	}
	utils.Success(c, log)
}

// Delete 删除操作日志
// @Summary 删除操作日志
// @ID deleteOperationLog
// @Tags System
// @Param id path int true "日志ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/logs/{id} [delete]
func (h *OperationLogHandler) Delete(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewOperationLogService().Delete(c.Request.Context(), id); err != nil {
		systemRespondErr(c, err, "日志不存在")
		return
	}
	utils.Success(c, nil)
}

// Clear 清空操作日志
// @Summary 清空操作日志
// @Description 清空全部操作日志（物理删除）。该路由必须在 /logs/{id} 之前注册。
// @ID clearOperationLogs
// @Tags System
// @Success 200 {object} response.BaseResponse
// @Router /web/system/logs/clear [delete]
func (h *OperationLogHandler) Clear(c *gin.Context) {
	if err := system.NewOperationLogService().Clear(c.Request.Context()); err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, nil)
}

// Export 导出操作日志
// @Summary 导出操作日志
// @Description 按筛选条件导出操作日志。响应文件名为 operation_logs.csv，
// 实际内容为 UTF-8 BOM + 逗号分隔的 CSV，列为
// 日志编号/系统模块/操作类型/请求方式/操作人员/主机/操作地点/状态/操作时间。
// @ID exportOperationLogs
// @Tags System
// @Param module query string false "系统模块"
// @Param operator query string false "操作人员"
// @Param action query string false "操作类型(新增/修改/删除)"
// @Param start_dt query string false "开始时间 2026-01-01 00:00:00"
// @Param end_dt query string false "结束时间 2026-12-31 23:59:59"
// @Success 200 {string} string "CSV 文件内容(UTF-8 BOM)"
// @Router /web/system/logs/export [get]
func (h *OperationLogHandler) Export(c *gin.Context) {
	var req request.PageOperationLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	list, err := system.NewOperationLogService().Export(c.Request.Context(), &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}

	const contentType = "text/csv; charset=utf-8"
	c.Header("Content-Disposition", "attachment; filename=operation_logs.csv")
	c.Data(http.StatusOK, contentType, buildOperationLogCSV(list))
}

// buildOperationLogCSV 生成带 UTF-8 BOM 的 CSV 内容。
func buildOperationLogCSV(list []model.OperationLog) []byte {
	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM，保证 Excel 正确识别中文
	sb.WriteString("日志编号,系统模块,操作类型,请求方式,操作人员,主机,操作地点,状态,操作时间\n")

	for _, l := range list {
		status := "失败"
		if l.Status == 1 {
			status = "成功"
		}
		fields := []string{
			fmt.Sprintf("%d", l.ID),
			l.Module,
			l.Action,
			l.Method,
			l.Operator,
			l.IP,
			l.Location,
			status,
			l.CreatedDT.Format("2006-01-02 15:04:05"),
		}
		for i, f := range fields {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(csvEscape(f))
		}
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

// csvEscape 转义 CSV 字段（含逗号/引号/换行时用双引号包裹）。
func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
