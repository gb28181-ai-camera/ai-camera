package handler

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// v0.6.0 智能语音聊天 - Web 端
//   - WS ticket
//   - 长期记忆管理（列表 / 删除 / 导出）
//   - MCP 工具与审计（最小管理）
// ════════════════════════════════════════════════════════════════

// VoiceWsHandler 语音 WebSocket 与记忆/MCP 管理处理器。
type VoiceWsHandler struct {
	admin *voiceagent.AdminService
}

// NewVoiceWsHandler 构造处理器。
func NewVoiceWsHandler() *VoiceWsHandler {
	return &VoiceWsHandler{admin: voiceagent.DefaultAdminService()}
}

// WsTicket 申请 WebSocket 一次性 ticket
//
// @Summary      申请语音 WS ticket
// @Description  使用当前 JWT 身份签发 60 秒一次性 ticket；ticket 只能使用一次
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        body  body      request.VoiceWsTicketReq  false  "可选参数"
// @Success      200  {object}  response.RVoiceWsTicket
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Router       /web/voice/ws-ticket [post]
// @ID           createWebVoiceWsTicket
func (h *VoiceWsHandler) WsTicket(c *gin.Context) {
	var req request.VoiceWsTicketReq
	// body 可为空：允许前端直接 POST {}。
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	deviceID, err := voiceagent.ResolveDeviceID(c.Request.Context(), req.DeviceCode)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	ticket, claims, err := voiceagent.DefaultTicketService().Issue(voiceagent.TicketClaims{
		UserID:       c.GetInt64("user_id"),
		UserName:     c.GetString("user_name"),
		RoleCode:     c.GetString("role_code"),
		DeptID:       c.GetInt64("dept_id"),
		DataPerm:     c.GetString("data_perm"),
		CrossDeptIDs: c.GetString("cross_dept_ids"),
		DeviceID:     deviceID,
		ClientID:     strings.TrimSpace(req.ClientID),
		Kind:         "web",
		AgentCode:    strings.TrimSpace(req.AgentCode),
	})
	if err != nil {
		utils.ServerError(c, "签发 ticket 失败: "+err.Error())
		return
	}
	utils.Success(c, response.VoiceWsTicketResp{
		Ticket:    ticket,
		ExpiresIn: int(time.Until(time.Unix(claims.ExpiresAt, 0)).Seconds()),
		WsURL:     wsURL(c, "/web/voice/ws"),
	})
}

// TestEmbedding 测试当前 Embedding 配置
//
// @Summary      测试 Embedding 服务
// @Description  用当前配置真实调用一次 /v1/embeddings，返回模型、维度与耗时；未配置端点时 success=false
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RVoiceEmbeddingTest
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/voice/embedding/test [post]
// @ID           testVoiceEmbedding
func (h *VoiceWsHandler) TestEmbedding(c *gin.Context) {
	resp, err := h.admin.TestEmbedding(c.Request.Context())
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// BackfillEmbeddings 为历史记忆重建向量
//
// @Summary      历史记忆重建向量
// @Description  分批为 embedding 为空/非 ok 的记忆生成向量并写入 pgvector；返回本次处理、成功、失败与剩余量
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        body  body      request.VoiceEmbeddingBackfillReq  false  "批量参数"
// @Success      200   {object}  response.RVoiceEmbeddingBackfill
// @Failure      400   {object}  response.BaseResponse
// @Router       /web/voice/memories/backfill [post]
// @ID           backfillVoiceMemoryEmbeddings
func (h *VoiceWsHandler) BackfillEmbeddings(c *gin.Context) {
	var req request.VoiceEmbeddingBackfillReq
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.admin.BackfillEmbeddings(c.Request.Context(), req.Limit)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// PageMemories 记忆列表
//
// @Summary      语音记忆列表
// @Description  分页查询长期记忆（不返回向量）
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        page              query     int     false  "页码"
// @Param        page_size         query     int     false  "每页条数"
// @Param        owner_type        query     string  false  "user/device/dept/global"
// @Param        owner_key         query     string  false  "归属键"
// @Param        kind              query     string  false  "记忆类型"
// @Param        keyword           query     string  false  "内容关键字"
// @Param        embedding_status  query     string  false  "ok/pending/failed"
// @Success      200  {object}  response.RPageVoiceMemory
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/voice/memories [get]
// @ID           pageVoiceMemories
func (h *VoiceWsHandler) PageMemories(c *gin.Context) {
	var req request.PageVoiceMemoryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.admin.PageMemories(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// DeleteMemory 删除记忆
//
// @Summary      删除语音记忆
// @Description  软删除一条长期记忆
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "记忆ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/voice/memories/{id} [delete]
// @ID           deleteVoiceMemory
func (h *VoiceWsHandler) DeleteMemory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "记忆ID无效")
		return
	}
	if err := h.admin.DeleteMemory(c.Request.Context(), id, utils.NewDataScope(c)); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ExportMemories 导出记忆
//
// @Summary      导出语音记忆
// @Description  按筛选条件导出 CSV（最多 10000 条）
// @Tags         VoiceAgent
// @Produce      text/csv
// @Param        owner_type  query     string  false  "user/device/dept/global"
// @Param        owner_key   query     string  false  "归属键"
// @Param        kind        query     string  false  "记忆类型"
// @Param        keyword     query     string  false  "内容关键字"
// @Success      200  {file}    file
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/voice/memories/export [get]
// @ID           exportVoiceMemories
func (h *VoiceWsHandler) ExportMemories(c *gin.Context) {
	var req request.PageVoiceMemoryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	items, err := h.admin.ExportMemories(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM：Excel 直接打开不乱码
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "owner_key", "kind", "content", "importance", "created_dt"})
	for _, it := range items {
		_ = w.Write([]string{
			strconv.FormatInt(it.ID, 10),
			it.OwnerKey,
			it.Kind,
			it.Content,
			strconv.Itoa(it.Importance),
			it.CreatedDT.Format("2006-01-02 15:04:05"),
		})
	}
	w.Flush()
	c.Header("Content-Disposition", `attachment; filename="voice-memories.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}

// PageMcpTools MCP 工具列表
//
// @Summary      MCP 工具列表
// @Description  分页查询 MCP 工具（含风险等级与确认策略）
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"
// @Param        page_size   query     int     false  "每页条数"
// @Param        keyword     query     string  false  "工具名/描述关键字"
// @Param        source      query     string  false  "internal/device"
// @Param        risk_level  query     string  false  "low/medium/high"
// @Param        enabled     query     bool    false  "是否启用"
// @Success      200  {object}  response.RPageVoiceMcpTool
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/voice/mcp/tools [get]
// @ID           pageVoiceMcpTools
func (h *VoiceWsHandler) PageMcpTools(c *gin.Context) {
	var req request.PageVoiceMcpToolReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.admin.PageTools(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// UpdateMcpTool 更新 MCP 工具策略
//
// @Summary      更新 MCP 工具策略
// @Description  更新启用状态、风险等级、是否需要 confirm=true
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        id    path      int                              true  "工具ID"
// @Param        body  body      request.UpdateVoiceMcpToolReq    true  "策略"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/voice/mcp/tools/{id} [put]
// @ID           updateVoiceMcpTool
func (h *VoiceWsHandler) UpdateMcpTool(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "工具ID无效")
		return
	}
	var req request.UpdateVoiceMcpToolReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.admin.UpdateTool(c.Request.Context(), id, &req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// PageMcpLogs MCP 调用审计
//
// @Summary      MCP 调用审计
// @Description  分页查询工具调用记录（ok/error/denied）
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"
// @Param        page_size   query     int     false  "每页条数"
// @Param        tool_name   query     string  false  "工具名"
// @Param        status      query     string  false  "ok/error/denied"
// @Param        user_id     query     int     false  "用户ID"
// @Param        device_id   query     int     false  "设备ID"
// @Param        session_id  query     string  false  "会话ID"
// @Param        start_dt    query     string  false  "开始时间"
// @Param        end_dt      query     string  false  "结束时间"
// @Success      200  {object}  response.RPageVoiceMcpLog
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/voice/mcp/logs [get]
// @ID           pageVoiceMcpLogs
func (h *VoiceWsHandler) PageMcpLogs(c *gin.Context) {
	var req request.PageVoiceMcpLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.admin.PageLogs(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// wsURL 构造 WebSocket 绝对地址（兼容反向代理的 X-Forwarded-Proto）。
func wsURL(c *gin.Context, path string) string {
	scheme := "ws"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}
	host := c.Request.Host
	if host == "" {
		return path
	}
	return scheme + "://" + host + path
}
