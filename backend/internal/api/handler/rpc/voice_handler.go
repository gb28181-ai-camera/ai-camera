package rpc

import (
	"io"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 内部语音接口（Pipecat → Go）
//
//   POST /internal/mcp                    MCP JSON-RPC 2.0
//   POST /internal/voice/memory/retrieve  记忆检索
//   POST /internal/voice/memory/write     记忆写入
//
// 鉴权：Authorization: Bearer <INTERNAL_MCP_TOKEN>。
// 这些路由**不挂** authMW / casbinMW（调用方是内网进程，没有用户会话）。
// ════════════════════════════════════════════════════════════════

const internalBodyLimit = 4 << 20

// VoiceHandler 内部语音接口处理器。
type VoiceHandler struct {
	mcp    *voiceagent.MCPService
	memory *voiceagent.MemoryService
}

// NewVoiceHandler 构造内部语音处理器。
func NewVoiceHandler() *VoiceHandler {
	return &VoiceHandler{
		mcp:    voiceagent.DefaultMCPService(),
		memory: voiceagent.DefaultMemoryService(),
	}
}

// MCP 内部 MCP JSON-RPC
//
// @Summary      内部 MCP JSON-RPC
// @Description  initialize / tools/list / tools/call；工具身份只取自 X-Voice-Context 签名上下文
// @Tags         VoiceInternal
// @Accept       json
// @Produce      json
// @Param        Authorization     header  string  true   "Bearer <INTERNAL_MCP_TOKEN>"
// @Param        X-Voice-Context   header  string  false  "base64url(JSON).HMAC"
// @Success      200  {object}  object
// @Failure      401  {object}  object
// @Router       /internal/mcp [post]
// @ID           internalMcp
func (h *VoiceHandler) MCP(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, internalBodyLimit))
	if err != nil {
		c.JSON(400, gin.H{"jsonrpc": "2.0", "error": gin.H{"code": -32700, "message": "读取请求体失败"}})
		return
	}
	status, out := h.mcp.Handle(
		c.Request.Context(),
		body,
		c.GetHeader("Authorization"),
		c.GetHeader("X-Voice-Context"),
		c.ClientIP(),
	)
	c.Data(status, "application/json; charset=utf-8", out)
}

// MemoryRetrieve 记忆检索
//
// @Summary      内部记忆检索
// @Description  owner_keys + query（可选 embedding）→ 记忆列表；无向量时按关键词/时间降级
// @Tags         VoiceInternal
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                        true  "Bearer <INTERNAL_MCP_TOKEN>"
// @Param        body           body    request.MemoryRetrieveReq    true  "检索参数"
// @Success      200  {object}  object
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Router       /internal/voice/memory/retrieve [post]
// @ID           internalVoiceMemoryRetrieve
func (h *VoiceHandler) MemoryRetrieve(c *gin.Context) {
	if !h.checkToken(c) {
		return
	}
	var req request.MemoryRetrieveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	items, err := h.memory.Retrieve(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	c.JSON(200, gin.H{"items": items})
}

// MemoryWrite 记忆写入
//
// @Summary      内部记忆写入
// @Description  owner_type/owner_id + content → 去重写入；未配置 embedding 时 embedding_status=failed
// @Tags         VoiceInternal
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                     true  "Bearer <INTERNAL_MCP_TOKEN>"
// @Param        body           body    request.MemoryWriteReq    true  "写入参数"
// @Success      200  {object}  object
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Router       /internal/voice/memory/write [post]
// @ID           internalVoiceMemoryWrite
func (h *VoiceHandler) MemoryWrite(c *gin.Context) {
	if !h.checkToken(c) {
		return
	}
	var req request.MemoryWriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	item, dedup, err := h.memory.Write(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	c.JSON(200, gin.H{"item": item, "deduplicated": dedup})
}

// checkToken 校验内部令牌；失败时已写响应。
func (h *VoiceHandler) checkToken(c *gin.Context) bool {
	err := voiceagent.CheckInternalToken(c.Request.Context(), c.GetHeader("Authorization"))
	if err == nil {
		return true
	}
	if strings.Contains(err.Error(), "未配置") {
		utils.ServerError(c, err.Error())
	} else {
		utils.Unauthorized(c, err.Error())
	}
	return false
}
