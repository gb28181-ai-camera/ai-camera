package request

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）—— Wave 2 运维类请求
//
// 分页筛选请求（PageGbCloudRecordReq 等）已在 gb28181.go 中定义，
// 本文件只补齐 Wave 2 新增的写请求与专用查询请求。
// ════════════════════════════════════════════════════════════════

// ── 回放与下载 ────────────────────────────────────────────────

// GbRecordQueryReq 录像检索请求。
//
// 时间格式固定为 GB28181 的 "2006-01-02T15:04:05"。
// 不接受本地时间字符串：设备侧按 XML 原样解析，格式不符会被静默忽略。
type GbRecordQueryReq struct {
	ChannelID int64  `form:"channel_id" json:"channel_id" binding:"required"`
	StartTime string `form:"start_time" json:"start_time" binding:"required" example:"2026-01-01T00:00:00"`
	EndTime   string `form:"end_time" json:"end_time" binding:"required" example:"2026-01-01T23:59:59"`
	Type      string `form:"type" json:"type" example:"time"` // 可选，透传给设备
}

// GbPlaybackStopReq 停止回放。
type GbPlaybackStopReq struct {
	StreamID string `json:"stream_id" binding:"required"`
}

// GbPlaybackReq 发起回放请求。
type GbPlaybackReq struct {
	ChannelID int64  `json:"channel_id" binding:"required"`
	StartTime string `json:"start_time" binding:"required" example:"2026-01-01T08:00:00"`
	EndTime   string `json:"end_time" binding:"required" example:"2026-01-01T09:00:00"`
}

// GbPlaybackControlReq 回放控制请求（契约 §15.3，字段不可改）。
//
// pause/resume/seek/speed 走**带内 SIP INFO + MANSRTSP**，
// 与云台的带外 MESSAGE 是两条完全不同的发送路径。
type GbPlaybackControlReq struct {
	StreamID string `json:"stream_id" binding:"required"`
	Action   string `json:"action" binding:"required" example:"pause"` // pause | resume | seek | speed
	// SeekTime 仅 Action=seek 时必填，格式 "2006-01-02T15:04:05"。
	SeekTime string `json:"seek_time"`
	// Scale 仅 Action=speed 时必填，1.0 为原速。
	Scale float64 `json:"scale"`
}

// GbDownloadReq 发起录像下载请求。
type GbDownloadReq struct {
	ChannelID int64  `json:"channel_id" binding:"required"`
	StartTime string `json:"start_time" binding:"required"`
	EndTime   string `json:"end_time" binding:"required"`
}

// PageGbDownloadReq 下载任务分页查询。
type PageGbDownloadReq struct {
	PageReq
	Keyword   string `form:"keyword" json:"keyword"`
	ChannelID *int64 `form:"channel_id" json:"channel_id"`
	Status    string `form:"status" json:"status" example:"downloading"`
}

// ── 云端录像 ──────────────────────────────────────────────────

// GbCloudRecordDateReq 云端录像「有录像的日期」查询。
type GbCloudRecordDateReq struct {
	DateRangeReq
	DeviceID      *int64 `form:"device_id" json:"device_id"`
	ChannelID     *int64 `form:"channel_id" json:"channel_id"`
	MediaServerID string `form:"media_server_id" json:"media_server_id"`
}

// GbCloudRecordCollectReq 批量收藏 / 取消收藏。
type GbCloudRecordCollectReq struct {
	IDs     []int64 `json:"ids" binding:"required,min=1"`
	Collect bool    `json:"collect"`
}

// GbCloudRecordBatchReq 批量删除（保留独立类型，便于 Swagger 生成准确文档）。
type GbCloudRecordBatchReq struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// GbAlarmClearReq 批量清除国标报警。
type GbAlarmClearReq struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// ── 录像计划 ──────────────────────────────────────────────────

// GbRecordPlanItemReq 一个录像时间段。
type GbRecordPlanItemReq struct {
	WeekDay   int64  `json:"week_day" example:"1"` // 0 周日 ~ 6 周六
	StartTime string `json:"start_time" example:"08:00:00"`
	EndTime   string `json:"end_time" example:"18:00:00"`
}

// CreateGbRecordPlanReq 新增录像计划。
type CreateGbRecordPlanReq struct {
	Active *bool                 `json:"active"`
	Name   string                `json:"name" binding:"required" example:"园区全天录像"`
	Snap   *bool                 `json:"snap"`
	Remark string                `json:"remark"`
	Items  []GbRecordPlanItemReq `json:"items"`
}

// UpdateGbRecordPlanReq 编辑录像计划。
//
// Items 为 nil 表示「不修改时间段」；显式传 [] 表示「清空全部时间段」——
// 这与「未提交该字段」在 JSON 反序列化后能区分（nil vs 空切片）。
type UpdateGbRecordPlanReq struct {
	Active *bool                 `json:"active"`
	Name   string                `json:"name"`
	Snap   *bool                 `json:"snap"`
	Remark *string               `json:"remark"`
	Items  []GbRecordPlanItemReq `json:"items"`
}

// LinkGbRecordPlanChannelsReq 计划绑定通道（整体替换）。
type LinkGbRecordPlanChannelsReq struct {
	ChannelIDs []int64 `json:"channel_ids"`
}

// ── 国标报警 ──────────────────────────────────────────────────

// GbAlarmHandleReq 处理国标报警。
type GbAlarmHandleReq struct {
	// Handled 为空时按 true 处理（处理接口的默认语义就是「已处理」）。
	Handled *bool  `json:"handled"`
	Remark  string `json:"remark"`
}

// ── 移动位置 ──────────────────────────────────────────────────

// GbPositionHistoryReq 位置历史查询。
type GbPositionHistoryReq struct {
	PageReq
	DateRangeReq
	DeviceID    *int64 `form:"device_id" json:"device_id"`
	ChannelID   *int64 `form:"channel_id" json:"channel_id"`
	GbChannelID string `form:"gb_channel_id" json:"gb_channel_id"`
}

// GbPositionSubscribeReq 位置订阅请求。
type GbPositionSubscribeReq struct {
	ChannelID int64 `json:"channel_id" binding:"required"`
	// Interval 订阅间隔（秒），<=0 时使用 60 秒。
	Interval int `json:"interval" example:"60"`
}

// ── 拉流代理 / 推流 ───────────────────────────────────────────

// CreateGbStreamProxyReq 新增拉流代理。
type CreateGbStreamProxyReq struct {
	Active                  *bool  `json:"active"`
	Name                    string `json:"name" binding:"required"`
	Type                    string `json:"type" example:"default"`
	App                     string `json:"app" binding:"required" example:"proxy"`
	Stream                  string `json:"stream" binding:"required"`
	SrcURL                  string `json:"src_url" binding:"required" example:"rtsp://10.0.0.9/live"`
	TimeoutMS               int64  `json:"timeout_ms" example:"10000"`
	FFmpegCmdKey            string `json:"ffmpeg_cmd_key"`
	RTSPType                int64  `json:"rtsp_type" example:"0"`
	MediaServerID           string `json:"media_server_id"`
	EnableAudio             *bool  `json:"enable_audio"`
	EnableMP4               *bool  `json:"enable_mp4"`
	Enable                  *bool  `json:"enable"`
	EnableDisableNoneReader *bool  `json:"enable_disable_none_reader"`
	EnableRemoveNoneReader  *bool  `json:"enable_remove_none_reader"`
	Remark                  string `json:"remark"`
}

// UpdateGbStreamProxyReq 编辑拉流代理（全部字段指针，0 与「未传」语义不同）。
type UpdateGbStreamProxyReq struct {
	Active                  *bool   `json:"active"`
	Name                    *string `json:"name"`
	Type                    *string `json:"type"`
	App                     *string `json:"app"`
	Stream                  *string `json:"stream"`
	SrcURL                  *string `json:"src_url"`
	TimeoutMS               *int64  `json:"timeout_ms"`
	FFmpegCmdKey            *string `json:"ffmpeg_cmd_key"`
	RTSPType                *int64  `json:"rtsp_type"`
	MediaServerID           *string `json:"media_server_id"`
	EnableAudio             *bool   `json:"enable_audio"`
	EnableMP4               *bool   `json:"enable_mp4"`
	Enable                  *bool   `json:"enable"`
	EnableDisableNoneReader *bool   `json:"enable_disable_none_reader"`
	EnableRemoveNoneReader  *bool   `json:"enable_remove_none_reader"`
	Remark                  *string `json:"remark"`
}

// CreateGbStreamPushReq 新增推流。
//
// 推流地址由平台按媒体节点生成（rtmp://<stream_ip>:<rtmp_port>/<app>/<stream>），
// 不在请求体中传入。
type CreateGbStreamPushReq struct {
	Active           *bool  `json:"active"`
	Name             string `json:"name" binding:"required"`
	App              string `json:"app" binding:"required" example:"push"`
	Stream           string `json:"stream" binding:"required"`
	MediaServerID    string `json:"media_server_id"`
	SelfPush         *bool  `json:"self_push"`
	StartOfflinePush *bool  `json:"start_offline_push"`
	Remark           string `json:"remark"`
}

// UpdateGbStreamPushReq 编辑推流。
type UpdateGbStreamPushReq struct {
	Active           *bool   `json:"active"`
	Name             *string `json:"name"`
	App              *string `json:"app"`
	Stream           *string `json:"stream"`
	MediaServerID    *string `json:"media_server_id"`
	SelfPush         *bool   `json:"self_push"`
	StartOfflinePush *bool   `json:"start_offline_push"`
	Remark           *string `json:"remark"`
}

// ── 国标级联平台 ──────────────────────────────────────────────

// CreateGbPlatformReq 新增上级平台。
type CreateGbPlatformReq struct {
	Active         *bool  `json:"active"`
	Enable         *bool  `json:"enable"`
	Name           string `json:"name" binding:"required"`
	ServerGBID     string `json:"server_gb_id" binding:"required" example:"34020000002000000001"`
	ServerGBDomain string `json:"server_gb_domain"`
	ServerIP       string `json:"server_ip"`
	ServerPort     int64  `json:"server_port" example:"5060"`

	DeviceGBID  string `json:"device_gb_id"`
	DeviceIP    string `json:"device_ip"`
	DevicePort  int64  `json:"device_port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Expires     int64  `json:"expires" example:"3600"`
	KeepTimeout int64  `json:"keep_timeout" example:"60"`
	Transport   string `json:"transport" example:"UDP"`
	CivilCode   string `json:"civil_code"`

	RegisterWay         string `json:"register_way" example:"local"`
	AsMessageChannel    *bool  `json:"as_message_channel"`
	CharacterSet        string `json:"character_set" example:"GB2312"`
	PTZ                 *bool  `json:"ptz"`
	RTCP                *bool  `json:"rtcp"`
	ServerID            string `json:"server_id"`
	Manufacturer        string `json:"manufacturer"`
	Model               string `json:"model"`
	Address             string `json:"address"`
	Secrecy             int64  `json:"secrecy"`
	CatalogWithPlatform *bool  `json:"catalog_with_platform"`
	CatalogWithGroup    int64  `json:"catalog_with_group"`
	CatalogWithRegion   int64  `json:"catalog_with_region"`
	AutoPushChannel     *bool  `json:"auto_push_channel"`
	SendStreamIP        string `json:"send_stream_ip"`
	Remark              string `json:"remark"`
}

// UpdateGbPlatformReq 编辑上级平台（全部指针，password 空值/**** 保持原值）。
type UpdateGbPlatformReq struct {
	Active         *bool   `json:"active"`
	Enable         *bool   `json:"enable"`
	Name           *string `json:"name"`
	ServerGBID     *string `json:"server_gb_id"`
	ServerGBDomain *string `json:"server_gb_domain"`
	ServerIP       *string `json:"server_ip"`
	ServerPort     *int64  `json:"server_port"`

	DeviceGBID  *string `json:"device_gb_id"`
	DeviceIP    *string `json:"device_ip"`
	DevicePort  *int64  `json:"device_port"`
	Username    *string `json:"username"`
	Password    *string `json:"password"`
	Expires     *int64  `json:"expires"`
	KeepTimeout *int64  `json:"keep_timeout"`
	Transport   *string `json:"transport"`
	CivilCode   *string `json:"civil_code"`

	RegisterWay         *string `json:"register_way"`
	AsMessageChannel    *bool   `json:"as_message_channel"`
	CharacterSet        *string `json:"character_set"`
	PTZ                 *bool   `json:"ptz"`
	RTCP                *bool   `json:"rtcp"`
	ServerID            *string `json:"server_id"`
	Manufacturer        *string `json:"manufacturer"`
	Model               *string `json:"model"`
	Address             *string `json:"address"`
	Secrecy             *int64  `json:"secrecy"`
	CatalogWithPlatform *bool   `json:"catalog_with_platform"`
	CatalogWithGroup    *int64  `json:"catalog_with_group"`
	CatalogWithRegion   *int64  `json:"catalog_with_region"`
	AutoPushChannel     *bool   `json:"auto_push_channel"`
	SendStreamIP        *string `json:"send_stream_ip"`
	Remark              *string `json:"remark"`
}

// GbPlatformChannelIDsReq 批量添加 / 移除级联通道。
type GbPlatformChannelIDsReq struct {
	ChannelIDs []int64 `json:"channel_ids" binding:"required,min=1"`
}

// UpdateGbPlatformChannelReq 修改通道在上级平台的定制信息。
type UpdateGbPlatformChannelReq struct {
	CustomDeviceID     *string `json:"custom_device_id"`
	CustomName         *string `json:"custom_name"`
	CustomManufacturer *string `json:"custom_manufacturer"`
	CustomModel        *string `json:"custom_model"`
	CustomOwner        *string `json:"custom_owner"`
	CustomCivilCode    *string `json:"custom_civil_code"`
	CustomAddress      *string `json:"custom_address"`
	CustomParental     *int64  `json:"custom_parental"`
	CustomParentID     *string `json:"custom_parent_id"`
	CustomStatus       *string `json:"custom_status"`
	CustomPTZType      *int64  `json:"custom_ptz_type"`
	CustomChannelType  *int64  `json:"custom_channel_type"`
}

// SetGbPlatformGroupsReq 设置平台可见的业务分组。
type SetGbPlatformGroupsReq struct {
	GroupIDs []int64 `json:"group_ids"`
}

// SetGbPlatformRegionsReq 设置平台可见的区域。
type SetGbPlatformRegionsReq struct {
	RegionIDs []int64 `json:"region_ids"`
}

// ── 语音 Agent 配置 ───────────────────────────────────────────

// UpdateVoiceAgentReq 保存语音 Agent 配置。
//
// 指针字段为 nil 表示「不修改」；敏感键传空或含 **** 时保持原值
// （由 setting 包的 saveThirdParty 统一处理）。
type UpdateVoiceAgentReq struct {
	STTProvider *string `json:"stt_provider"`
	STTModel    *string `json:"stt_model"`
	STTEndpoint *string `json:"stt_endpoint"`
	STTAPIKey   *string `json:"stt_api_key"`
	STTLanguage *string `json:"stt_language"`

	LLMProvider     *string  `json:"llm_provider"`
	LLMModel        *string  `json:"llm_model"`
	LLMEndpoint     *string  `json:"llm_endpoint"`
	LLMAPIKey       *string  `json:"llm_api_key"`
	LLMSystemPrompt *string  `json:"llm_system_prompt"`
	LLMTemperature  *float64 `json:"llm_temperature"`
	LLMMaxTokens    *int     `json:"llm_max_tokens"`

	TTSProvider *string  `json:"tts_provider"`
	TTSVoice    *string  `json:"tts_voice"`
	TTSEndpoint *string  `json:"tts_endpoint"`
	TTSAPIKey   *string  `json:"tts_api_key"`
	TTSSpeed    *float64 `json:"tts_speed"`

	VADProvider *string `json:"vad_provider"`
	VADParams   *string `json:"vad_params"`

	BargeIn            *bool `json:"barge_in"`
	SessionIdleTimeout *int  `json:"session_idle_timeout"`
	MaxSessionSec      *int  `json:"max_session_sec"`

	PipecatEndpoint *string `json:"pipecat_endpoint"`
	PipecatToken    *string `json:"pipecat_token"`

	// ExtraParams provider param_schema 中没有固定键的扩展参数（如 TTS 的 model、
	// Azure 的 region）。键为原始参数名（前端不带前缀），服务端统一加 `param_`
	// 前缀落库，避免与上面的固定键冲突；nil 表示不修改。
	ExtraParams map[string]string `json:"extra_params"`
}

// TestVoiceAgentReq provider 连通性测试。
type TestVoiceAgentReq struct {
	// Category 取值 stt / llm / tts / vad；为空时按 llm 处理。
	Category string `json:"category" example:"llm"`
	// Provider 为空时使用库中已保存的对应 provider。
	Provider string `json:"provider"`
	// Endpoint 为空时使用库中已保存的 endpoint。
	Endpoint string `json:"endpoint"`
	// APIKey 为空时使用库中已保存的密钥；支持只测连通性而不回显密钥。
	APIKey string `json:"api_key"`
}
