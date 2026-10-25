package model

import "time"

// ════════════════════════════════════════════════════════════════
// v0.6.0 智能语音聊天：Agent / MCP / 记忆 / 会话摘要 / 逐轮记录
//
// 设计依据：docs/敏捷开发/v0.6.0 智能语音聊天/00-设计与协议.md §7
// DDL：docs/开发文档/数据库设计/sql/ddl/15-语音Agent.sql
//
// 约定：
//   1. 语音会话主体仍复用 ac_voice_session（见 gb28181.go），
//      本文件只新增配套表，不重复建模会话。
//   2. ac_voice_memory.embedding 为 pgvector vector(1024)：
//      写入/检索统一走原始 SQL（`?::vector`），普通查询必须 Omit 该列，
//      否则 database/sql 无法把 vector 扫描进 string。
//   3. ac_voice_mcp_call_log 为追加写审计表，不做更新与删除。
// ════════════════════════════════════════════════════════════════

// 语音 Agent 会话传输方式（ac_voice_session.transport）。
const (
	VoiceTransportWebSocket = "websocket"
	VoiceTransportRTP       = "rtp"
)

// 语音记忆归属类型（ac_voice_memory.owner_type）。
const (
	VoiceOwnerUser   = "user"
	VoiceOwnerDevice = "device"
	VoiceOwnerDept   = "dept"
	VoiceOwnerGlobal = "global"
)

// 语音记忆 embedding 状态。
const (
	VoiceEmbeddingOK      = "ok"
	VoiceEmbeddingFailed  = "failed"
	VoiceEmbeddingPending = "pending"
)

// MCP 工具风险等级。
const (
	VoiceRiskLow    = "low"
	VoiceRiskMedium = "medium"
	VoiceRiskHigh   = "high"
)

// MCP 工具来源。
const (
	VoiceToolSourceInternal = "internal"
	VoiceToolSourceDevice   = "device"
)

// MCP 工具权限主体类型。
const (
	VoiceSubjectRole   = "role"
	VoiceSubjectUser   = "user"
	VoiceSubjectDept   = "dept"
	VoiceSubjectDevice = "device"
)

// MCP 工具权限效果。
const (
	VoiceEffectAllow = "allow"
	VoiceEffectDeny  = "deny"
)

// VoiceAgent 语音 Agent 人格配置。表 ac_voice_agent。
//
// 与 ac_thirdparty_config(Category='voice_agent') 的分工：
//   - ac_thirdparty_config 存「provider 参数与密钥」（STT/LLM/TTS/VAD、Pipecat 控制面）；
//   - 本表存「可多套并存的人格」：唤醒词、greeting、音色、模型与 system prompt。
//
// 会话通过 agent_code 关联到某套人格；未指定时使用默认（active + sequence 最小）。
type VoiceAgent struct {
	BaseModel
	Active       *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence     int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Code         string `gorm:"column:code;size:100;not null;uniqueIndex:idx_ac_voice_agent_code;comment:Agent编码" json:"code"`
	Name         string `gorm:"column:name;size:255;not null;comment:Agent名称" json:"name"`
	Description  string `gorm:"column:description;type:text;comment:描述" json:"description"`
	Greeting     string `gorm:"column:greeting;type:text;comment:开场白" json:"greeting"`
	WakeWord     string `gorm:"column:wake_word;size:100;comment:设备端唤醒词" json:"wake_word"`
	Voice        string `gorm:"column:voice;size:100;comment:音色" json:"voice"`
	Model        string `gorm:"column:model;size:100;comment:LLM模型" json:"model"`
	SystemPrompt string `gorm:"column:system_prompt;type:text;comment:系统提示词" json:"system_prompt"`
	Persona      string `gorm:"column:persona;type:text;comment:人格设定(JSON字符串)" json:"persona"`
	Extra        string `gorm:"column:extra;type:text;comment:扩展配置(JSON字符串)" json:"extra"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceAgent) TableName() string { return "ac_voice_agent" }

// VoiceMcpServer MCP 服务端注册。表 ac_voice_mcp_server。
//
// 首期内置工具（device.list 等）由 Go 自身提供，source=internal；
// 设备端 MCP 由 Pipecat 与设备直连，不在本表登记。本表为后续
// 「外部 MCP Server」预留，Token 属敏感字段，读接口一律不返回。
type VoiceMcpServer struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	Code        string `gorm:"column:code;size:100;not null;uniqueIndex:idx_ac_voice_mcp_server_code;comment:服务编码" json:"code"`
	Name        string `gorm:"column:name;size:255;not null;comment:服务名称" json:"name"`
	Transport   string `gorm:"column:transport;size:20;default:http;comment:传输 http/stdio/ws" json:"transport"`
	URL         string `gorm:"column:url;size:500;comment:服务地址" json:"url"`
	Token       string `gorm:"column:token;size:500;comment:鉴权令牌(敏感,不返回)" json:"-"`
	TimeoutMS   int    `gorm:"column:timeout_ms;default:10000;comment:超时(毫秒)" json:"timeout_ms"`
	Description string `gorm:"column:description;type:text;comment:描述" json:"description"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceMcpServer) TableName() string { return "ac_voice_mcp_server" }

// VoiceMcpTool MCP 工具登记。表 ac_voice_mcp_tool。
//
// name 是 LLM 可见的名字（如 device.list）；wire_name 是设备端/外部服务
// 真实名字（设备工具可能带 "." 需映射为 device_<...>）。
// input_schema 存 JSON Schema 字符串，tools/list 原样返回。
type VoiceMcpTool struct {
	BaseModel
	Active         *bool  `gorm:"column:active;default:true" json:"active"`
	ServerID       *int64 `gorm:"column:server_id;index;comment:所属MCP服务(NULL=内置)" json:"server_id"`
	Source         string `gorm:"column:source;size:20;default:internal;comment:internal/device" json:"source"`
	Name           string `gorm:"column:name;size:100;not null;uniqueIndex:idx_ac_voice_mcp_tool_name;comment:工具名(LLM可见)" json:"name"`
	WireName       string `gorm:"column:wire_name;size:100;comment:原始工具名" json:"wire_name"`
	DisplayName    string `gorm:"column:display_name;size:255;comment:展示名" json:"display_name"`
	Description    string `gorm:"column:description;type:text;comment:描述" json:"description"`
	InputSchema    string `gorm:"column:input_schema;type:text;comment:输入JSON Schema" json:"input_schema"`
	RiskLevel      string `gorm:"column:risk_level;size:20;default:low;comment:风险等级 low/medium/high" json:"risk_level"`
	RequireConfirm bool   `gorm:"column:require_confirm;default:false;comment:写操作是否需要 confirm=true" json:"require_confirm"`
	Enabled        bool   `gorm:"column:enabled;default:true;comment:是否启用" json:"enabled"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceMcpTool) TableName() string { return "ac_voice_mcp_tool" }

// VoiceToolPermission MCP 工具权限。表 ac_voice_tool_permission。
//
// subject_type=role 时 subject_id 存角色编码（如 admin），
// user/dept/device 时存对应数字 ID 的字符串形式。
// 规则语义：同一工具下存在任意规则时「默认拒绝」，
// 必须命中 allow 且不命中 deny 才放行；无规则时按工具默认策略。
type VoiceToolPermission struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	ToolID      int64  `gorm:"column:tool_id;not null;index:idx_ac_voice_tool_permission_tool;comment:工具ID" json:"tool_id"`
	SubjectType string `gorm:"column:subject_type;size:20;not null;comment:role/user/dept/device" json:"subject_type"`
	SubjectID   string `gorm:"column:subject_id;size:100;not null;comment:主体标识(角色编码或ID)" json:"subject_id"`
	Effect      string `gorm:"column:effect;size:10;not null;default:allow;comment:allow/deny" json:"effect"`
	Remark      string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceToolPermission) TableName() string { return "ac_voice_tool_permission" }

// VoiceMcpCallLog MCP 工具调用审计（追加写）。表 ac_voice_mcp_call_log。
type VoiceMcpCallLog struct {
	BaseModel
	SessionID  string `gorm:"column:session_id;size:100;index;comment:语音会话ID" json:"session_id"`
	UserID     *int64 `gorm:"column:user_id;index;comment:调用用户" json:"user_id"`
	DeviceID   *int64 `gorm:"column:device_id;index;comment:设备ID" json:"device_id"`
	ServerCode string `gorm:"column:server_code;size:100;comment:MCP服务编码" json:"server_code"`
	ToolName   string `gorm:"column:tool_name;size:100;not null;index;comment:工具名" json:"tool_name"`
	RiskLevel  string `gorm:"column:risk_level;size:20;comment:风险等级" json:"risk_level"`
	Confirmed  bool   `gorm:"column:confirmed;default:false;comment:是否携带确认" json:"confirmed"`
	// status: ok / error / denied
	Status    string `gorm:"column:status;size:20;not null;index;comment:结果 ok/error/denied" json:"status"`
	Args      string `gorm:"column:args;type:text;comment:调用参数(截断)" json:"args"`
	Result    string `gorm:"column:result;type:text;comment:结果摘要(截断)" json:"result"`
	ErrorMsg  string `gorm:"column:error_msg;size:1000;comment:错误信息" json:"error_msg"`
	ElapsedMS int64  `gorm:"column:elapsed_ms;comment:耗时(毫秒)" json:"elapsed_ms"`
	IP        string `gorm:"column:ip;size:64;comment:来源IP" json:"ip"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceMcpCallLog) TableName() string { return "ac_voice_mcp_call_log" }

// VoiceMemory 长期记忆。表 ac_voice_memory。
//
// owner_key 是检索主键：`user:12` / `device:3` / `dept:2` / `global`。
// owner_id 为 0 时表示 global。content_hash 为 content 的 SHA-256，
// 与 owner_key 组成去重唯一键（软删除行不参与）。
type VoiceMemory struct {
	BaseModel
	OwnerType       string     `gorm:"column:owner_type;size:20;not null;index:idx_ac_voice_memory_owner;comment:user/device/dept/global" json:"owner_type"`
	OwnerID         int64      `gorm:"column:owner_id;default:0;comment:归属对象ID(global为0)" json:"owner_id"`
	OwnerKey        string     `gorm:"column:owner_key;size:100;not null;index:idx_ac_voice_memory_owner;comment:归属键 user:1/global" json:"owner_key"`
	Kind            string     `gorm:"column:kind;size:30;default:fact;index;comment:fact/preference/task/other" json:"kind"`
	Content         string     `gorm:"column:content;type:text;not null;comment:记忆内容" json:"content"`
	ContentHash     string     `gorm:"column:content_hash;size:64;not null;comment:内容SHA-256" json:"content_hash"`
	Importance      int        `gorm:"column:importance;default:3;comment:重要度 1-5" json:"importance"`
	Embedding       *string    `gorm:"column:embedding;type:vector(1024);comment:向量(1024维,pgvector)" json:"-"`
	EmbeddingStatus string     `gorm:"column:embedding_status;size:20;default:pending;index;comment:ok/pending/failed" json:"embedding_status"`
	SourceSessionID string     `gorm:"column:source_session_id;size:100;index;comment:来源会话" json:"source_session_id"`
	TurnID          string     `gorm:"column:turn_id;size:100;comment:来源轮次" json:"turn_id"`
	LastAccessDT    *time.Time `gorm:"column:last_access_dt;comment:最近召回时间" json:"last_access_dt"`
	ExpiresDT       *time.Time `gorm:"column:expires_dt;index;comment:过期时间" json:"expires_dt"`

	// EmbeddingDim 非持久化：避免前端误以为会返回向量。
	EmbeddingDim int `gorm:"-" json:"embedding_dim,omitempty"`

	UserID *int64 `gorm:"column:user_id;index;comment:归属用户(便于数据权限)" json:"user_id"`
	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceMemory) TableName() string { return "ac_voice_memory" }

// VoiceSessionSummary 会话摘要（短期记忆落库）。表 ac_voice_session_summary。
type VoiceSessionSummary struct {
	BaseModel
	SessionID    string     `gorm:"column:session_id;size:100;not null;index:idx_ac_voice_session_summary_session;comment:会话ID" json:"session_id"`
	UserID       *int64     `gorm:"column:user_id;index;comment:用户" json:"user_id"`
	DeviceID     *int64     `gorm:"column:device_id;index;comment:设备" json:"device_id"`
	Summary      string     `gorm:"column:summary;type:text;comment:摘要文本" json:"summary"`
	MessageCount int        `gorm:"column:message_count;default:0;comment:消息数" json:"message_count"`
	StartedDT    *time.Time `gorm:"column:started_dt;comment:会话开始时间" json:"started_dt"`
	EndedDT      *time.Time `gorm:"column:ended_dt;comment:会话结束时间" json:"ended_dt"`
	ExpiresDT    *time.Time `gorm:"column:expires_dt;index;comment:过期时间(默认30天)" json:"expires_dt"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceSessionSummary) TableName() string { return "ac_voice_session_summary" }

// VoiceTurn 逐轮对话记录（可选逐字稿）。表 ac_voice_turn。
//
// 不存原始音频；text 为 STT/LLM 文本。role: user/assistant/system/tool。
type VoiceTurn struct {
	BaseModel
	SessionID     string     `gorm:"column:session_id;size:100;not null;index:idx_ac_voice_turn_session;comment:会话ID" json:"session_id"`
	TurnID        string     `gorm:"column:turn_id;size:100;index;comment:轮次ID" json:"turn_id"`
	Seq           int        `gorm:"column:seq;not null;default:0;comment:轮次序号" json:"seq"`
	Role          string     `gorm:"column:role;size:20;not null;comment:user/assistant/system/tool" json:"role"`
	Text          string     `gorm:"column:text;type:text;comment:文本内容" json:"text"`
	STTConfidence float64    `gorm:"column:stt_confidence;default:0;comment:STT置信度" json:"stt_confidence"`
	LLMEmotion    string     `gorm:"column:llm_emotion;size:50;comment:LLM情绪" json:"llm_emotion"`
	ToolName      string     `gorm:"column:tool_name;size:100;comment:工具名(role=tool)" json:"tool_name"`
	AudioMS       int        `gorm:"column:audio_ms;default:0;comment:音频时长(毫秒)" json:"audio_ms"`
	StartedDT     *time.Time `gorm:"column:started_dt;comment:开始时间" json:"started_dt"`
	EndedDT       *time.Time `gorm:"column:ended_dt;comment:结束时间" json:"ended_dt"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceTurn) TableName() string { return "ac_voice_turn" }

func init() {
	RegisterModel(
		&VoiceAgent{},
		&VoiceMcpServer{},
		&VoiceMcpTool{},
		&VoiceToolPermission{},
		&VoiceMcpCallLog{},
		&VoiceMemory{},
		&VoiceSessionSummary{},
		&VoiceTurn{},
	)
}
