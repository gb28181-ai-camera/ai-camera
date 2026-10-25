package response

import "time"

// ════════════════════════════════════════════════════════════════
// v0.6.0 智能语音聊天：WS ticket / 记忆 / MCP 管理
// ════════════════════════════════════════════════════════════════

// VoiceWsTicketResp WS 一次性 ticket。
type VoiceWsTicketResp struct {
	Ticket    string `json:"ticket" example:"eyJ1aWQiOjF9.xxxx"`
	ExpiresIn int    `json:"expires_in" example:"60"`
	WsURL     string `json:"ws_url" example:"/web/voice/ws"`
	SessionID string `json:"session_id"`
}

// RVoiceWsTicket ticket 响应包装。
type RVoiceWsTicket struct {
	BaseResponse
	Data VoiceWsTicketResp `json:"data"`
}

// VoiceEmbeddingTestResp Embedding 连通性测试结果。
type VoiceEmbeddingTestResp struct {
	Success   bool   `json:"success" example:"true"`
	Endpoint  string `json:"endpoint" example:"http://127.0.0.1:8091/v1"`
	Model     string `json:"model" example:"intfloat/multilingual-e5-large"`
	Dim       int    `json:"dim" example:"1024"`
	ElapsedMS int64  `json:"elapsed_ms" example:"38"`
	Message   string `json:"message" example:"Embedding 可用，返回 1024 维向量"`
}

// RVoiceEmbeddingTest Embedding 测试响应。
type RVoiceEmbeddingTest struct {
	BaseResponse
	Data VoiceEmbeddingTestResp `json:"data"`
}

// VoiceEmbeddingBackfillResp 历史记忆重建向量的结果。
type VoiceEmbeddingBackfillResp struct {
	Scanned   int   `json:"scanned" example:"100"`
	Updated   int   `json:"updated" example:"98"`
	Failed    int   `json:"failed" example:"2"`
	Remaining int64 `json:"remaining" example:"0"`
}

// RVoiceEmbeddingBackfill 重建向量响应。
type RVoiceEmbeddingBackfill struct {
	BaseResponse
	Data VoiceEmbeddingBackfillResp `json:"data"`
}

// VoiceMemoryItem 记忆列表项（不含向量）。
type VoiceMemoryItem struct {
	ID              int64      `json:"id"`
	OwnerType       string     `json:"owner_type" example:"user"`
	OwnerID         int64      `json:"owner_id"`
	OwnerKey        string     `json:"owner_key" example:"user:1"`
	Kind            string     `json:"kind" example:"fact"`
	Content         string     `json:"content"`
	ContentHash     string     `json:"content_hash"`
	Importance      int        `json:"importance" example:"3"`
	EmbeddingStatus string     `json:"embedding_status" example:"ok"`
	SourceSessionID string     `json:"source_session_id"`
	TurnID          string     `json:"turn_id"`
	LastAccessDT    *time.Time `json:"last_access_dt"`
	ExpiresDT       *time.Time `json:"expires_dt"`
	UserID          *int64     `json:"user_id"`
	DeptID          *int64     `json:"dept_id"`
	CreatedBy       string     `json:"created_by,omitempty"`
	CreatedDT       time.Time  `json:"created_dt"`
}

// PageVoiceMemoryResp 记忆分页载荷。
type PageVoiceMemoryResp struct {
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Items    []VoiceMemoryItem `json:"items"`
}

// RPageVoiceMemory 记忆分页响应。
type RPageVoiceMemory struct {
	BaseResponse
	Data PageVoiceMemoryResp `json:"data"`
}

// VoiceMemoryBrief 记忆导出项（精简）。
type VoiceMemoryBrief struct {
	ID         int64     `json:"id"`
	OwnerKey   string    `json:"owner_key"`
	Kind       string    `json:"kind"`
	Content    string    `json:"content"`
	Importance int       `json:"importance"`
	CreatedDT  time.Time `json:"created_dt"`
}

// RVoiceMemoryList 记忆简单列表响应（导出用）。
type RVoiceMemoryList struct {
	BaseResponse
	Data []VoiceMemoryBrief `json:"data"`
}

// VoiceMcpToolItem MCP 工具列表项。
type VoiceMcpToolItem struct {
	ID             int64     `json:"id"`
	ServerID       *int64    `json:"server_id"`
	Source         string    `json:"source" example:"internal"`
	Name           string    `json:"name" example:"device.list"`
	WireName       string    `json:"wire_name"`
	DisplayName    string    `json:"display_name"`
	Description    string    `json:"description"`
	InputSchema    string    `json:"input_schema"`
	RiskLevel      string    `json:"risk_level" example:"low"`
	RequireConfirm bool      `json:"require_confirm"`
	Enabled        bool      `json:"enabled"`
	Active         *bool     `json:"active"`
	CreatedDT      time.Time `json:"created_dt"`
}

// PageVoiceMcpToolResp MCP 工具分页载荷。
type PageVoiceMcpToolResp struct {
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Items    []VoiceMcpToolItem `json:"items"`
}

// RPageVoiceMcpTool MCP 工具分页响应。
type RPageVoiceMcpTool struct {
	BaseResponse
	Data PageVoiceMcpToolResp `json:"data"`
}

// VoiceMcpLogItem MCP 调用审计项。
type VoiceMcpLogItem struct {
	ID         int64     `json:"id"`
	SessionID  string    `json:"session_id"`
	UserID     *int64    `json:"user_id"`
	DeviceID   *int64    `json:"device_id"`
	ServerCode string    `json:"server_code"`
	ToolName   string    `json:"tool_name"`
	RiskLevel  string    `json:"risk_level"`
	Confirmed  bool      `json:"confirmed"`
	Status     string    `json:"status" example:"ok"`
	Args       string    `json:"args"`
	Result     string    `json:"result"`
	ErrorMsg   string    `json:"error_msg"`
	ElapsedMS  int64     `json:"elapsed_ms"`
	IP         string    `json:"ip"`
	CreatedBy  string    `json:"created_by,omitempty"`
	CreatedDT  time.Time `json:"created_dt"`
}

// PageVoiceMcpLogResp MCP 审计分页载荷。
type PageVoiceMcpLogResp struct {
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Items    []VoiceMcpLogItem `json:"items"`
}

// RPageVoiceMcpLog MCP 审计分页响应。
type RPageVoiceMcpLog struct {
	BaseResponse
	Data PageVoiceMcpLogResp `json:"data"`
}
