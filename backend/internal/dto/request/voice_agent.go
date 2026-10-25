package request

import "encoding/json"

// ════════════════════════════════════════════════════════════════
// v0.6.0 智能语音聊天：WS ticket / MCP / 记忆
// ════════════════════════════════════════════════════════════════

// VoiceWsTicketReq 申请 WebSocket 一次性 ticket。
type VoiceWsTicketReq struct {
	// AgentCode 可选：指定 Agent 人格；为空使用默认人格。
	AgentCode string `json:"agent_code"`
	// ClientID 可选：客户端自生成的连接标识。
	ClientID string `json:"client_id"`
	// DeviceCode 可选（仅 /app/voice/ws-ticket）：App 绑定的设备编号。
	DeviceCode string `json:"device_code"`
}

// ChatContext 注入 Pipecat 的签名上下文。
//
// 该结构不直接从 HTTP 请求体读取：Web/App 由 JWT 派生，设备由 Basic 凭据派生，
// 然后由网关签名写入 X-Voice-Context 头。
type ChatContext struct {
	SessionID    string   `json:"session_id"`
	UserID       int64    `json:"user_id,omitempty"`
	UserName     string   `json:"user_name,omitempty"`
	RoleCode     string   `json:"role_code,omitempty"`
	DeptID       int64    `json:"dept_id,omitempty"`
	DataPerm     string   `json:"data_perm,omitempty"`
	CrossDeptIDs string   `json:"cross_dept_ids,omitempty"`
	DeviceID     int64    `json:"device_id,omitempty"`
	ProductID    int64    `json:"product_id,omitempty"`
	ClientID     string   `json:"client_id,omitempty"`
	Kind         string   `json:"kind"` // web / app / device
	AgentCode    string   `json:"agent_code,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	IssuedAt     int64    `json:"iat"`
	ExpiresAt    int64    `json:"exp"`
}

// MemoryRetrieveReq 内部记忆检索请求。
type MemoryRetrieveReq struct {
	OwnerKeys []string  `json:"owner_keys" binding:"required"`
	Query     string    `json:"query"`
	TopK      int       `json:"top_k"`
	Embedding []float32 `json:"embedding"`
}

// MemoryWriteReq 内部记忆写入请求。
type MemoryWriteReq struct {
	OwnerType       string    `json:"owner_type" binding:"required"`
	OwnerID         int64     `json:"owner_id"`
	Kind            string    `json:"kind"`
	Content         string    `json:"content" binding:"required"`
	Importance      int       `json:"importance"`
	SourceSessionID string    `json:"source_session_id"`
	TurnID          string    `json:"turn_id"`
	Embedding       []float32 `json:"embedding"`
}

// MemoryItem 内部写接口返回的单条记忆。
type MemoryItem struct {
	ID              int64   `json:"id"`
	OwnerType       string  `json:"owner_type"`
	OwnerID         int64   `json:"owner_id"`
	OwnerKey        string  `json:"owner_key"`
	Kind            string  `json:"kind"`
	Content         string  `json:"content"`
	Importance      int     `json:"importance"`
	EmbeddingStatus string  `json:"embedding_status"`
	Score           float64 `json:"score,omitempty"`
	CreatedDT       string  `json:"created_dt,omitempty"`
}

// PageVoiceMemoryReq 记忆列表查询（Web 管理）。
type PageVoiceMemoryReq struct {
	PageReq
	OwnerType       string `form:"owner_type" json:"owner_type"`
	OwnerKey        string `form:"owner_key" json:"owner_key"`
	Kind            string `form:"kind" json:"kind"`
	Keyword         string `form:"keyword" json:"keyword"`
	EmbeddingStatus string `form:"embedding_status" json:"embedding_status"`
}

// UpdateVoiceMcpToolReq 更新 MCP 工具策略。
type UpdateVoiceMcpToolReq struct {
	Enabled        *bool   `json:"enabled"`
	RiskLevel      *string `json:"risk_level"`
	RequireConfirm *bool   `json:"require_confirm"`
}

// VoiceEmbeddingBackfillReq 历史记忆重建向量。
type VoiceEmbeddingBackfillReq struct {
	// Limit 本次最多处理多少条（默认 100，上限 500）。
	Limit int `json:"limit" example:"100"`
}

// PageVoiceMcpToolReq MCP 工具列表查询。
type PageVoiceMcpToolReq struct {
	PageReq
	Keyword   string `form:"keyword" json:"keyword"`
	Source    string `form:"source" json:"source"`
	RiskLevel string `form:"risk_level" json:"risk_level"`
	Enabled   *bool  `form:"enabled" json:"enabled"`
}

// PageVoiceMcpLogReq MCP 调用审计查询。
type PageVoiceMcpLogReq struct {
	PageReq
	ToolName  string `form:"tool_name" json:"tool_name"`
	Status    string `form:"status" json:"status"`
	UserID    *int64 `form:"user_id" json:"user_id"`
	DeviceID  *int64 `form:"device_id" json:"device_id"`
	SessionID string `form:"session_id" json:"session_id"`
	StartDT   string `form:"start_dt" json:"start_dt"`
	EndDT     string `form:"end_dt" json:"end_dt"`
}

// ── JSON-RPC 2.0 ──────────────────────────────────────────────

// JSONRPCRequest JSON-RPC 2.0 请求。ID 用 RawMessage 以原样回传数字/字符串。
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCError JSON-RPC 2.0 错误对象。
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPCResponse JSON-RPC 2.0 响应。
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// McpToolCallParams tools/call 参数。
type McpToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// JSON-RPC 标准错误码。
const (
	JSONRPCParseError     = -32700
	JSONRPCInvalidRequest = -32600
	JSONRPCMethodNotFound = -32601
	JSONRPCInvalidParams  = -32602
	JSONRPCInternalError  = -32603
)
