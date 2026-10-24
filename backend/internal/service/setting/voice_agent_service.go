package setting

// 语音 Agent（Pipecat）动态配置。
//
// 零新表：复用 ac_thirdparty_config（Category='voice_agent'）与 setting_service.go
// 已有的 loadThirdParty / saveThirdParty / maskIfSecret 原语。
//
// 敏感键（*_api_key、pipecat_token）的「只写不读」语义：
//   - 读接口一律返回 ****；
//   - 写接口收到空值或含 **** 的值时跳过，不会把展示值覆盖成真密钥。
//
// sensitiveCodes 是 setting_service.go 的包级变量。本文件通过 init() 追加
// 语音键，而不是去改动那个文件 —— 同一包内协作，保持文件所有权清晰。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// CategoryVoiceAgent 语音 Agent 配置分类。
const CategoryVoiceAgent = "voice_agent"

// 配置键（设计 §8.2）。
const (
	voiceSTTProvider = "stt_provider"
	voiceSTTModel    = "stt_model"
	voiceSTTEndpoint = "stt_endpoint"
	voiceSTTAPIKey   = "stt_api_key"
	voiceSTTLanguage = "stt_language"

	voiceLLMProvider     = "llm_provider"
	voiceLLMModel        = "llm_model"
	voiceLLMEndpoint     = "llm_endpoint"
	voiceLLMAPIKey       = "llm_api_key"
	voiceLLMSystemPrompt = "llm_system_prompt"
	voiceLLMTemperature  = "llm_temperature"
	voiceLLMMaxTokens    = "llm_max_tokens"

	voiceTTSProvider = "tts_provider"
	voiceTTSVoice    = "tts_voice"
	voiceTTSEndpoint = "tts_endpoint"
	voiceTTSAPIKey   = "tts_api_key"
	voiceTTSSpeed    = "tts_speed"

	voiceVADProvider = "vad_provider"
	voiceVADParams   = "vad_params"

	voiceBargeIn            = "barge_in"
	voiceSessionIdleTimeout = "session_idle_timeout"
	voiceMaxSessionSec      = "max_session_sec"

	// Pipecat 控制面（设计 §8.2「Pipecat 侧控制面」）。
	voicePipecatEndpoint = "pipecat_endpoint"
	voicePipecatToken    = "pipecat_token"
)

// voiceSecretKeys 只写不读的键。
var voiceSecretKeys = []string{voiceSTTAPIKey, voiceLLMAPIKey, voiceTTSAPIKey, voicePipecatToken}

// voiceExtraPrefix 扩展参数落库前缀。
//
// 前端提交的扩展参数键（如 tts 的 model、azure 的 region）直接落库会与上面的
// 固定键同名冲突（例如 model 本身不是固定键，但换 provider 后语义不同）。
// 统一加 param_ 前缀隔离；读接口返回时再剥掉前缀，前端无感知。
const voiceExtraPrefix = "param_"

// isSensitiveExtraKey 判断扩展参数是否敏感。
//
// 后端不读取 VoiceProvider.param_schema（那会引入一次额外查询，而且 schema
// 只在前端用于渲染），因此按命名约定判断：包含 key/token/secret/password
// 的扩展参数一律按 secret 存储并在读接口脱敏，宁可多脱敏也不泄露凭证。
func isSensitiveExtraKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	return strings.Contains(k, "api_key") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "secret") ||
		strings.Contains(k, "password") ||
		strings.HasSuffix(k, "_key") ||
		k == "key"
}

// voiceKeyNames 配置项展示名（写入 ac_thirdparty_config.name）。
var voiceKeyNames = map[string]string{
	voiceSTTProvider: "STT Provider", voiceSTTModel: "STT 模型",
	voiceSTTEndpoint: "STT 端点", voiceSTTAPIKey: "STT API Key", voiceSTTLanguage: "STT 语言",
	voiceLLMProvider: "LLM Provider", voiceLLMModel: "LLM 模型",
	voiceLLMEndpoint: "LLM 端点", voiceLLMAPIKey: "LLM API Key",
	voiceLLMSystemPrompt: "LLM 系统提示词", voiceLLMTemperature: "LLM 温度", voiceLLMMaxTokens: "LLM 最大 Token",
	voiceTTSProvider: "TTS Provider", voiceTTSVoice: "TTS 音色",
	voiceTTSEndpoint: "TTS 端点", voiceTTSAPIKey: "TTS API Key", voiceTTSSpeed: "TTS 语速",
	voiceVADProvider: "VAD Provider", voiceVADParams: "VAD 参数",
	voiceBargeIn: "允许打断", voiceSessionIdleTimeout: "会话空闲超时",
	voiceMaxSessionSec:   "会话最大时长",
	voicePipecatEndpoint: "Pipecat 控制面地址", voicePipecatToken: "Pipecat 控制面令牌",
}

func init() {
	// 同一包内扩充敏感键集合：saveThirdParty/maskIfSecret 立即对语音键生效。
	for _, k := range voiceSecretKeys {
		sensitiveCodes[k] = true
	}
}

// VoiceAgentService 语音 Agent 配置业务逻辑。
type VoiceAgentService struct {
	base *SettingService
}

// NewVoiceAgentService 构造语音 Agent 配置服务。
//
// 复用 SettingService 的仓库与原语；NewSettingService 只做一次
// repository 构造，没有额外资源开销。
func NewVoiceAgentService() *VoiceAgentService {
	return &VoiceAgentService{base: NewSettingService()}
}

// Get 读取配置（敏感键脱敏）。
func (s *VoiceAgentService) Get(ctx context.Context) (*response.VoiceAgentConfigResp, error) {
	values, secrets, err := s.base.loadThirdParty(ctx, CategoryVoiceAgent)
	if err != nil {
		return nil, err
	}
	return &response.VoiceAgentConfigResp{
		STTProvider: firstNonEmpty(values[voiceSTTProvider], "local_whisper"),
		STTModel:    values[voiceSTTModel],
		STTEndpoint: values[voiceSTTEndpoint],
		STTAPIKey:   maskIfSecret(voiceSTTAPIKey, values[voiceSTTAPIKey], secrets[voiceSTTAPIKey]),
		STTLanguage: values[voiceSTTLanguage],

		LLMProvider:     firstNonEmpty(values[voiceLLMProvider], "anthropic"),
		LLMModel:        firstNonEmpty(values[voiceLLMModel], "claude-sonnet-5-5"),
		LLMEndpoint:     values[voiceLLMEndpoint],
		LLMAPIKey:       maskIfSecret(voiceLLMAPIKey, values[voiceLLMAPIKey], secrets[voiceLLMAPIKey]),
		LLMSystemPrompt: values[voiceLLMSystemPrompt],
		LLMTemperature:  parseFloatOr(values[voiceLLMTemperature], 0.7),
		LLMMaxTokens:    parseIntOr(values[voiceLLMMaxTokens], 1024),

		TTSProvider: firstNonEmpty(values[voiceTTSProvider], "edge_tts"),
		TTSVoice:    values[voiceTTSVoice],
		TTSEndpoint: values[voiceTTSEndpoint],
		TTSAPIKey:   maskIfSecret(voiceTTSAPIKey, values[voiceTTSAPIKey], secrets[voiceTTSAPIKey]),
		TTSSpeed:    parseFloatOr(values[voiceTTSSpeed], 1.0),

		VADProvider: firstNonEmpty(values[voiceVADProvider], "silero"),
		VADParams:   values[voiceVADParams],

		// 回声消除结论（设计 §8.1.1）：默认半双工，barge_in 默认关闭。
		BargeIn:            parseBoolOr(values[voiceBargeIn], false),
		SessionIdleTimeout: parseIntOr(values[voiceSessionIdleTimeout], 60),
		MaxSessionSec:      parseIntOr(values[voiceMaxSessionSec], 600),

		PipecatEndpoint: values[voicePipecatEndpoint],
		PipecatToken:    maskIfSecret(voicePipecatToken, values[voicePipecatToken], secrets[voicePipecatToken]),

		ExtraParams: collectVoiceExtras(values, secrets),
	}, nil
}

// collectVoiceExtras 提取 param_ 前缀的扩展参数，并对敏感键脱敏。
//
// 返回的键已去掉 param_ 前缀，前端可直接按 param_schema 的 key 回填。
func collectVoiceExtras(values map[string]string, secrets map[string]bool) map[string]string {
	extras := make(map[string]string)
	for code, value := range values {
		if !strings.HasPrefix(code, voiceExtraPrefix) {
			continue
		}
		key := strings.TrimPrefix(code, voiceExtraPrefix)
		if key == "" {
			continue
		}
		extras[key] = maskIfSecret(code, value, secrets[code])
	}
	if len(extras) == 0 {
		return nil
	}
	return extras
}

// GetVoiceAgent 是 Get 的语义别名（任务清单中的命名）。
func (s *VoiceAgentService) GetVoiceAgent(ctx context.Context) (*response.VoiceAgentConfigResp, error) {
	return s.Get(ctx)
}

// Update 保存配置。空值与脱敏串由 saveThirdParty 统一跳过。
func (s *VoiceAgentService) Update(ctx context.Context, req *request.UpdateVoiceAgentReq) error {
	if req == nil {
		return errors.New("请求不能为空")
	}
	items := make([]thirdPartyItem, 0, 24)
	addStr := func(code string, v *string) {
		if v != nil {
			items = append(items, thirdPartyItem{code: code, name: voiceKeyNames[code], value: *v})
		}
	}
	addFloat := func(code string, v *float64) {
		if v != nil {
			items = append(items, thirdPartyItem{code: code, name: voiceKeyNames[code], value: strconv.FormatFloat(*v, 'f', -1, 64)})
		}
	}
	addInt := func(code string, v *int) {
		if v != nil {
			items = append(items, thirdPartyItem{code: code, name: voiceKeyNames[code], value: strconv.Itoa(*v)})
		}
	}
	addBool := func(code string, v *bool) {
		if v != nil {
			items = append(items, thirdPartyItem{code: code, name: voiceKeyNames[code], value: strconv.FormatBool(*v)})
		}
	}

	addStr(voiceSTTProvider, req.STTProvider)
	addStr(voiceSTTModel, req.STTModel)
	addStr(voiceSTTEndpoint, req.STTEndpoint)
	addStr(voiceSTTAPIKey, req.STTAPIKey)
	addStr(voiceSTTLanguage, req.STTLanguage)

	addStr(voiceLLMProvider, req.LLMProvider)
	addStr(voiceLLMModel, req.LLMModel)
	addStr(voiceLLMEndpoint, req.LLMEndpoint)
	addStr(voiceLLMAPIKey, req.LLMAPIKey)
	addStr(voiceLLMSystemPrompt, req.LLMSystemPrompt)
	addFloat(voiceLLMTemperature, req.LLMTemperature)
	addInt(voiceLLMMaxTokens, req.LLMMaxTokens)

	addStr(voiceTTSProvider, req.TTSProvider)
	addStr(voiceTTSVoice, req.TTSVoice)
	addStr(voiceTTSEndpoint, req.TTSEndpoint)
	addStr(voiceTTSAPIKey, req.TTSAPIKey)
	addFloat(voiceTTSSpeed, req.TTSSpeed)

	addStr(voiceVADProvider, req.VADProvider)
	addStr(voiceVADParams, req.VADParams)

	addBool(voiceBargeIn, req.BargeIn)
	addInt(voiceSessionIdleTimeout, req.SessionIdleTimeout)
	addInt(voiceMaxSessionSec, req.MaxSessionSec)

	addStr(voicePipecatEndpoint, req.PipecatEndpoint)
	addStr(voicePipecatToken, req.PipecatToken)

	// 扩展参数：原样落库，仅加 param_ 前缀避免与固定键冲突。
	//
	// 这里直接调用 repo.Upsert，而不是塞进 items 走 saveThirdParty：
	// saveThirdParty 的 secret 标记只认包级 sensitiveCodes（进程启动时静态注册），
	// 动态写入该 map 会在并发请求下产生数据竞争。直接传 secret 参数更安全，
	// 后续 Get 通过 ac_thirdparty_config.secret 列继续脱敏。
	// 空值与脱敏串的跳过语义与 saveThirdParty 保持一致。
	for key, value := range req.ExtraParams {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.Contains(trimmed, "****") {
			continue
		}
		code := voiceExtraPrefix + k
		if err := s.base.repo.UpsertThirdPartyConfig(
			ctx, CategoryVoiceAgent, code, "扩展参数 "+k, trimmed, isSensitiveExtraKey(k),
		); err != nil {
			return err
		}
	}

	return s.base.saveThirdParty(ctx, CategoryVoiceAgent, items)
}

// SaveVoiceAgent 是 Update 的语义别名（任务清单中的命名）。
func (s *VoiceAgentService) SaveVoiceAgent(ctx context.Context, req *request.UpdateVoiceAgentReq) error {
	return s.Update(ctx, req)
}

// ListProviders 读取 provider 字典（驱动前端动态表单）。
//
// category 为空时返回全部类别；仓库层没有「全部」方法，
// 因此全量场景直接查表（字典表很小，无需分页）。
func (s *VoiceAgentService) ListProviders(ctx context.Context, category ...string) ([]response.VoiceProviderItem, error) {
	var rows []model.VoiceProvider
	q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceProvider{}).
		Where("(active IS NULL OR active = true)")
	if len(category) > 0 && strings.TrimSpace(category[0]) != "" {
		q = q.Where("category = ?", strings.TrimSpace(category[0]))
	}
	if err := q.Order("category ASC, sequence ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]response.VoiceProviderItem, 0, len(rows))
	for _, m := range rows {
		out = append(out, response.VoiceProviderItem{
			ID:            m.ID,
			Sequence:      m.Sequence,
			Category:      m.Category,
			Code:          m.Code,
			Name:          m.Name,
			ParamSchema:   m.ParamSchema,
			DefaultParams: m.DefaultParams,
			Remark:        m.Remark,
		})
	}
	return out, nil
}

// Test provider 连通性。
//
// 无网络 / endpoint 未配置时返回明确 error（Handler → 400）；
// endpoint 可达但返回 4xx/5xx 时返回 Success=false + Message（HTTP 200），
// 因为「网络能通、凭证不对」属于测试结果而不是接口故障。
func (s *VoiceAgentService) Test(ctx context.Context, req *request.TestVoiceAgentReq) (*response.VoiceAgentTestResp, error) {
	if req == nil {
		req = &request.TestVoiceAgentReq{}
	}
	category := strings.ToLower(strings.TrimSpace(req.Category))
	if category == "" {
		category = model.VoiceProviderLLM
	}
	if category != model.VoiceProviderSTT && category != model.VoiceProviderLLM &&
		category != model.VoiceProviderTTS && category != model.VoiceProviderVAD {
		return nil, errors.New("category 仅支持 stt / llm / tts / vad")
	}

	values, _, err := s.base.loadThirdParty(ctx, CategoryVoiceAgent)
	if err != nil {
		return nil, err
	}
	prefix := category + "_"
	provider := firstNonEmpty(req.Provider, values[prefix+"provider"])
	endpoint := firstNonEmpty(req.Endpoint, values[prefix+"endpoint"])
	apiKey := firstNonEmpty(req.APIKey, values[prefix+"api_key"])

	if provider == "" {
		return nil, fmt.Errorf("未配置 %s provider，无法测试", category)
	}
	if endpoint == "" {
		// 本地 provider 没有外部端点：明确告知不需要联网测试，
		// 而不是返回一个假的「连通成功」。
		if isLocalProvider(provider) {
			return &response.VoiceAgentTestResp{
				Success:  true,
				Category: category,
				Provider: provider,
				Message:  "本地 provider，无需端点连通性检查",
			}, nil
		}
		return nil, fmt.Errorf("provider %s 未配置 endpoint，无法测试连通性", provider)
	}

	start := time.Now()
	testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(testCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("provider endpoint 非法: %w", err)
	}
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(httpReq)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("无法连接 provider 端点 %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	out := &response.VoiceAgentTestResp{
		Category:  category,
		Provider:  provider,
		Endpoint:  endpoint,
		ElapsedMS: elapsed,
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		out.Success = true
		out.Message = "端点可达"
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		out.Success = false
		out.Message = fmt.Sprintf("端点可达，但返回 HTTP %d（请检查密钥或路径）", resp.StatusCode)
	default:
		out.Success = false
		out.Message = fmt.Sprintf("端点返回 HTTP %d，服务可能不可用", resp.StatusCode)
	}
	return out, nil
}

// isLocalProvider 判断是否本地 provider（无需外部 endpoint）。
func isLocalProvider(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return strings.HasPrefix(p, "local") || p == "edge_tts" || p == "silero" || p == "pyannote"
}

// Apply 把当前配置推给 Pipecat 控制面。
//
// 未配置 endpoint/token 时返回明确错误（不尝试猜测默认地址）。
// Pipecat 控制面的契约见设计 §8.2：POST /config。
func (s *VoiceAgentService) Apply(ctx context.Context) error {
	values, _, err := s.base.loadThirdParty(ctx, CategoryVoiceAgent)
	if err != nil {
		return err
	}
	endpoint := strings.TrimSpace(values[voicePipecatEndpoint])
	token := strings.TrimSpace(values[voicePipecatToken])
	if endpoint == "" {
		return errors.New("未配置 Pipecat 控制面地址（pipecat_endpoint），无法下发配置")
	}
	if token == "" {
		return errors.New("未配置 Pipecat 控制面令牌（pipecat_token），无法下发配置")
	}

	url := strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(url, "/config") {
		url += "/config"
	}

	// 下发的 payload 排除控制面自身的连接信息，避免把 token 再回传给对端。
	payload := make(map[string]string, len(values))
	for k, v := range values {
		if k == voicePipecatEndpoint || k == voicePipecatToken {
			continue
		}
		// 扩展参数落库时加了 param_ 前缀；下发给 Pipecat 时还原为原始键，
		// 否则 Pipecat 侧会多出一个无法识别的 param_<name> 字段。
		if strings.HasPrefix(k, voiceExtraPrefix) {
			payload[strings.TrimPrefix(k, voiceExtraPrefix)] = v
			continue
		}
		payload[k] = v
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化语音配置失败: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Pipecat 控制面地址非法: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(httpReq)
	if err != nil {
		return fmt.Errorf("无法连接 Pipecat 控制面 %s: %w", url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Pipecat 控制面返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// ── 小工具 ────────────────────────────────────────────────────

func parseFloatOr(s string, def float64) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return v
	}
	return def
}

func parseIntOr(s string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return v
	}
	return def
}

func parseBoolOr(s string, def bool) bool {
	if v, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
		return v
	}
	return def
}
