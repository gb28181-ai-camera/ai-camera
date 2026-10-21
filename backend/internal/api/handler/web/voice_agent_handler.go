package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	svcsetting "github.com/boticz/ai-camera-backend/internal/service/setting"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 语音 Agent（Pipecat）配置
//
// 复用 ac_thirdparty_config（Category='voice_agent'），
// 敏感键只写不读；provider 字典由 ac_voice_provider 驱动前端动态表单。
// ════════════════════════════════════════════════════════════════

// VoiceAgentHandler 语音 Agent 配置处理器。
type VoiceAgentHandler struct {
	svc *svcsetting.VoiceAgentService
}

// NewVoiceAgentHandler 构造语音 Agent 配置处理器。
func NewVoiceAgentHandler() *VoiceAgentHandler {
	return &VoiceAgentHandler{svc: svcsetting.NewVoiceAgentService()}
}

// Get 读取语音 Agent 配置
//
// @Summary      读取语音 Agent 配置
// @Description  返回 STT/LLM/TTS/VAD 与 Pipecat 控制面配置；*_api_key 与 pipecat_token 脱敏为 ****
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RVoiceAgentConfig
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/settings/voice-agent [get]
// @ID           getVoiceAgentConfig
func (h *VoiceAgentHandler) Get(c *gin.Context) {
	resp, err := h.svc.Get(c.Request.Context())
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// Update 保存语音 Agent 配置
//
// @Summary      保存语音 Agent 配置
// @Description  增量保存；敏感键传空或含 **** 时保持原值，配置在下一个会话生效
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        body  body      request.UpdateVoiceAgentReq  true  "配置"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/settings/voice-agent [put]
// @ID           updateVoiceAgentConfig
func (h *VoiceAgentHandler) Update(c *gin.Context) {
	var req request.UpdateVoiceAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), &req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, nil)
}

// ListProviders 语音 provider 字典
//
// @Summary      语音 provider 字典
// @Description  返回启用的 provider 及其 param_schema，驱动前端动态表单；可按 category 过滤
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        category  query     string  false  "类别 stt/llm/tts/vad"
// @Success      200  {object}  response.RGbVoiceProviderList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/settings/voice-agent/providers [get]
// @ID           listVoiceAgentProviders
func (h *VoiceAgentHandler) ListProviders(c *gin.Context) {
	items, err := h.svc.ListProviders(c.Request.Context(), c.Query("category"))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, items)
}

// Test provider 连通性
//
// @Summary      provider 连通性测试
// @Description  endpoint 未配置或网络不可达时返回明确错误；端点可达但鉴权失败返回 success=false 与说明
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Param        body  body      request.TestVoiceAgentReq  true  "测试参数"
// @Success      200  {object}  response.RVoiceAgentTest
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/settings/voice-agent/test [post]
// @ID           testVoiceAgent
func (h *VoiceAgentHandler) Test(c *gin.Context) {
	var req request.TestVoiceAgentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Test(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// Apply 下发配置到 Pipecat
//
// @Summary      下发配置到 Pipecat
// @Description  把当前配置 POST 到 Pipecat 控制面 /config；未配置 pipecat_endpoint / pipecat_token 时返回明确错误
// @Tags         VoiceAgent
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/settings/voice-agent/apply [post]
// @ID           applyVoiceAgentConfig
func (h *VoiceAgentHandler) Apply(c *gin.Context) {
	if err := h.svc.Apply(c.Request.Context()); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, nil)
}
