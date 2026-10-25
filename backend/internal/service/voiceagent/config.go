// Package voiceagent 实现 v0.6.0 智能语音聊天的 Go 侧能力：
// WebSocket ticket / WS 反向代理 / 内部 MCP / 长期记忆。
//
// 分层：handler（HTTP/WS）→ service（本包）→ repository（数据访问）。
// 本包不解析音频，只做鉴权、上下文签名与双向字节转发。
package voiceagent

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/repository"
)

// 配置键（与 service/setting/voice_agent_service.go 保持一致）。
const (
	cfgWSEnabled         = "ws_enabled"
	cfgWSPath            = "ws_path"
	cfgDeviceWakeWord    = "device_wake_word"
	cfgFastAckText       = "fast_ack_text"
	cfgMCPEnabled        = "mcp_enabled"
	cfgMCPServerURL      = "mcp_server_url"
	cfgInternalMCPToken  = "internal_mcp_token"
	cfgMemoryEnabled     = "memory_enabled"
	cfgMemoryTopK        = "memory_top_k"
	cfgMemoryTTLDays     = "memory_ttl_days"
	cfgEmbeddingEndpoint = "embedding_endpoint"
	cfgEmbeddingModel    = "embedding_model"
	cfgEmbeddingAPIKey   = "embedding_api_key"
	cfgPipecatEndpoint   = "pipecat_endpoint"
	cfgPipecatToken      = "pipecat_token"
)

// 环境变量（容器部署时优先于数据库配置，便于密钥不落盘）。
const (
	envPipecatEndpoint   = "PIPECAT_ENDPOINT"
	envPipecatToken      = "PIPECAT_TOKEN"
	envPipecatCtxSecret  = "PIPECAT_CONTEXT_SECRET"
	envInternalMCPToken  = "INTERNAL_MCP_TOKEN"
	envEmbeddingEndpoint = "VOICE_EMBEDDING_ENDPOINT"
	envEmbeddingModel    = "VOICE_EMBEDDING_MODEL"
	envEmbeddingAPIKey   = "VOICE_EMBEDDING_API_KEY"
	envVoiceTicketSecret = "VOICE_TICKET_SECRET"

	// 默认值
	defaultWSPath        = "/ws/voice"
	defaultTopK          = 6
	defaultMemoryTTLDays = 180
)

// RuntimeConfig 运行期语音配置（配置表 + 环境变量合并后的结果）。
type RuntimeConfig struct {
	WSEnabled      bool
	WSPath         string
	DeviceWakeWord string
	FastAckText    string

	PipecatEndpoint string
	PipecatToken    string

	MCPEnabled       bool
	MCPServerURL     string
	InternalMCPToken string

	MemoryEnabled     bool
	MemoryTopK        int
	MemoryTTLDays     int
	EmbeddingEndpoint string
	EmbeddingModel    string
	EmbeddingAPIKey   string
}

// PipecatWSURL 把控制面 HTTP 地址转换为 WebSocket 地址。
//
// 例：http://pipecat:8090 + /ws/voice → ws://pipecat:8090/ws/voice
func (c *RuntimeConfig) PipecatWSURL() (string, error) {
	ep := strings.TrimSpace(c.PipecatEndpoint)
	if ep == "" {
		return "", errors.New("未配置 pipecat_endpoint")
	}
	u, err := url.Parse(ep)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.New("pipecat_endpoint 仅支持 http/https/ws/wss")
	}
	path := strings.TrimSpace(c.WSPath)
	if path == "" {
		path = defaultWSPath
	}
	// 配置的 endpoint 可能已带 /ws/voice：避免拼成 /ws/voice/ws/voice。
	if !strings.HasSuffix(strings.TrimRight(u.Path, "/"), strings.TrimRight(path, "/")) {
		u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	}
	return u.String(), nil
}

// ConfigProvider 提供运行期配置。
type ConfigProvider interface {
	Runtime(ctx context.Context) (*RuntimeConfig, error)
}

// DBConfigProvider 从 ac_thirdparty_config 读取配置，并允许环境变量覆盖。
type DBConfigProvider struct {
	repo *repository.VoiceAgentConfigRepo
}

// NewConfigProvider 构造默认配置提供者。
func NewConfigProvider() *DBConfigProvider {
	return &DBConfigProvider{repo: repository.NewVoiceAgentConfigRepo()}
}

// Runtime 读取并合并配置。
func (p *DBConfigProvider) Runtime(ctx context.Context) (*RuntimeConfig, error) {
	values := map[string]string{}
	if p.repo != nil {
		v, err := p.repo.LoadMap(ctx)
		if err == nil {
			values = v
		} else if !isNoDBError(err) {
			return nil, err
		}
	}
	return BuildRuntimeConfig(values), nil
}

// BuildRuntimeConfig 把配置键映射为 RuntimeConfig（含环境变量覆盖）。
//
// 抽成纯函数便于测试；不访问数据库。
func BuildRuntimeConfig(values map[string]string) *RuntimeConfig {
	if values == nil {
		values = map[string]string{}
	}
	pick := func(env, key, def string) string {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
		if v := strings.TrimSpace(values[key]); v != "" {
			return v
		}
		return def
	}
	cfg := &RuntimeConfig{
		WSEnabled:      parseBool(values[cfgWSEnabled], true),
		WSPath:         pick("", cfgWSPath, defaultWSPath),
		DeviceWakeWord: strings.TrimSpace(values[cfgDeviceWakeWord]),
		FastAckText:    pick("", cfgFastAckText, "我在"),

		PipecatEndpoint: pick(envPipecatEndpoint, cfgPipecatEndpoint, ""),
		PipecatToken:    pick(envPipecatToken, cfgPipecatToken, ""),

		MCPEnabled:       parseBool(values[cfgMCPEnabled], true),
		MCPServerURL:     strings.TrimSpace(values[cfgMCPServerURL]),
		InternalMCPToken: pick(envInternalMCPToken, cfgInternalMCPToken, ""),

		MemoryEnabled:     parseBool(values[cfgMemoryEnabled], true),
		MemoryTopK:        parseInt(values[cfgMemoryTopK], defaultTopK),
		MemoryTTLDays:     parseInt(values[cfgMemoryTTLDays], defaultMemoryTTLDays),
		EmbeddingEndpoint: pick(envEmbeddingEndpoint, cfgEmbeddingEndpoint, ""),
		EmbeddingModel:    pick(envEmbeddingModel, cfgEmbeddingModel, ""),
		EmbeddingAPIKey:   pick(envEmbeddingAPIKey, cfgEmbeddingAPIKey, ""),
	}
	if cfg.WSPath == "" {
		cfg.WSPath = defaultWSPath
	}
	if cfg.MemoryTopK <= 0 || cfg.MemoryTopK > 50 {
		cfg.MemoryTopK = defaultTopK
	}
	if cfg.MemoryTTLDays <= 0 {
		cfg.MemoryTTLDays = defaultMemoryTTLDays
	}
	return cfg
}

// ContextSecret 返回 Pipecat 上下文签名密钥（只允许环境变量）。
func ContextSecret() string {
	return strings.TrimSpace(os.Getenv(envPipecatCtxSecret))
}

// TicketSecret 返回 ticket HMAC 密钥：优先环境变量，回退 JWT secret。
func TicketSecret(fallback string) string {
	if v := strings.TrimSpace(os.Getenv(envVoiceTicketSecret)); v != "" {
		return v
	}
	return strings.TrimSpace(fallback)
}

func parseBool(s string, def bool) bool {
	if strings.TrimSpace(s) == "" {
		return def
	}
	v, err := strconv.ParseBool(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}

func parseInt(s string, def int) int {
	if strings.TrimSpace(s) == "" {
		return def
	}
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return v
}

// isNoDBError 在未初始化数据库时（纯单元测试）视为「无配置」。
func isNoDBError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nil") || strings.Contains(msg, "not initialized") ||
		strings.Contains(msg, "database")
}

// ContextTTL Pipecat 上下文有效期：覆盖一次会话（默认 1 小时）。
const ContextTTL = time.Hour

// TicketTTL WS ticket 有效期（设计 §2：60 秒一次性）。
const TicketTTL = 60 * time.Second
