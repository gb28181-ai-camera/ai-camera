package response

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）—— Wave 2 运维类响应类型
//
// 覆盖：回放/下载、云端录像、录像计划、国标报警、移动位置、
// 拉流代理、推流、级联平台、语音 Agent 配置。
//
// 命名遵循接口契约 §2.1 / §13：
//   - RPageXxx → Data PageXxxResp{Items, Total, Page, PageSize}
//   - RXxxList  → Data []XxxItem
//   - RXxx      → Data XxxDetailResp
//
// ★ 安全约束：这里刻意不直接返回 model.GbAlarm / model.GbCloudRecord 等实体，
// 因为它们 Preload 了 model.GbChannel，而后者携带 password 列。直接嵌入会把
// 通道密码序列化进响应，属于契约 §10.0-2 明确禁止的泄露路径。
// 因此除无敏感列的实体（拉流代理/推流）外，一律使用显式字段。
// ════════════════════════════════════════════════════════════════

import (
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// ── 回放检索 ──────────────────────────────────────────────────

// GbRecordItem 一条设备侧录像检索结果。
//
// StartTime/EndTime 保持设备上报的原样字符串（GB28181 为
// "2006-01-02T15:04:05"），前端按字符串直接展示与回传，
// 避免时区/格式二次转换导致 seek 时间对不上。
type GbRecordItem struct {
	ChannelID   int64  `json:"channel_id"`
	GbChannelID string `json:"gb_channel_id"`
	DeviceID    int64  `json:"device_id"`
	Name        string `json:"name"`
	FilePath    string `json:"file_path"`
	Address     string `json:"address"`
	StartTime   string `json:"start_time" example:"2026-01-01T08:00:00"`
	EndTime     string `json:"end_time" example:"2026-01-01T09:00:00"`
	Secrecy     int    `json:"secrecy"`
	Type        string `json:"type"`
	TimeLen     int64  `json:"time_len" example:"3600"`
}

// GbRecordQueryResp 录像检索结果。
//
// Timeout=true 表示设备在等待窗口内没有回齐响应（返回了已收到的部分，
// 可能为空）。这比让 HTTP 请求挂住更利于前端展示「设备无响应」。
type GbRecordQueryResp struct {
	ChannelID   int64          `json:"channel_id"`
	GbChannelID string         `json:"gb_channel_id"`
	Total       int            `json:"total"`
	Timeout     bool           `json:"timeout"`
	Items       []GbRecordItem `json:"items"`
}

// RGbRecordQuery 录像检索响应。
type RGbRecordQuery struct {
	BaseResponse
	Data GbRecordQueryResp `json:"data"`
}

// GbPlaybackResp 回放会话。
//
// App 恒为 "rtp"，Stream 是 SSRC 的大写十六进制（单端口模式，设计 §2.2）；
// StreamID 是 SIP Call-ID，停止与带内控制都回传它。
type GbPlaybackResp struct {
	StreamID      string `json:"stream_id"`
	App           string `json:"app" example:"rtp"`
	Stream        string `json:"stream" example:"0200000001"`
	SSRC          string `json:"ssrc" example:"0200000001"`
	HTTPBase      string `json:"http_base" example:"http://10.0.0.5:8082"`
	WebRTC        string `json:"webrtc"`
	FLV           string `json:"flv"`
	HLS           string `json:"hls"`
	WSFLV         string `json:"ws_flv"`
	MediaServerID string `json:"media_server_id"`
	ChannelID     int64  `json:"channel_id"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

// RGbPlayback 回放会话响应。
type RGbPlayback struct {
	BaseResponse
	Data GbPlaybackResp `json:"data"`
}

// ── 下载任务 ──────────────────────────────────────────────────

// GbDownloadItem 下载任务。
//
// 现有模型没有「下载任务」表（ac_gb_cloud_record 是 ZLM 落盘 MP4 的索引，
// 不是下载任务），因此本批次的任务表只存在于进程内存，详见
// playback_service.go 的注释。
type GbDownloadItem struct {
	ID          int64      `json:"id"`
	ChannelID   int64      `json:"channel_id"`
	ChannelName string     `json:"channel_name"`
	DeviceID    int64      `json:"device_id"`
	DeviceName  string     `json:"device_name"`
	StreamID    string     `json:"stream_id"`
	App         string     `json:"app"`
	Stream      string     `json:"stream"`
	SSRC        string     `json:"ssrc"`
	StartTime   string     `json:"start_time"`
	EndTime     string     `json:"end_time"`
	Status      string     `json:"status" example:"downloading"`
	Progress    float64    `json:"progress"`
	FilePath    string     `json:"file_path"`
	Message     string     `json:"message"`
	CreatedDT   time.Time  `json:"created_dt"`
	StoppedDT   *time.Time `json:"stopped_dt"`
}

// PageGbDownloadResp 下载任务分页载荷。
type PageGbDownloadResp struct {
	Items    []GbDownloadItem `json:"items"`
	Total    int64            `json:"total" example:"3"`
	Page     int              `json:"page" example:"1"`
	PageSize int              `json:"page_size" example:"20"`
}

// RPageGbDownload 下载任务分页响应。
type RPageGbDownload struct {
	BaseResponse
	Data PageGbDownloadResp `json:"data"`
}

// RGbDownload 单条下载任务响应。
type RGbDownload struct {
	BaseResponse
	Data GbDownloadItem `json:"data"`
}

// GbDownloadResp 发起下载后的任务回显（契约 §12 的命名）。
type GbDownloadResp struct {
	GbDownloadItem
}

// ── 云端录像 ──────────────────────────────────────────────────

// GbCloudRecordItem 云端录像列表项（已剥离 Channel.Password 等敏感字段）。
type GbCloudRecordItem struct {
	ID            int64      `json:"id"`
	App           string     `json:"app" example:"record"`
	Stream        string     `json:"stream"`
	CallID        string     `json:"call_id"`
	DeviceID      *int64     `json:"device_id"`
	DeviceName    string     `json:"device_name"`
	ChannelID     *int64     `json:"channel_id"`
	ChannelName   string     `json:"channel_name"`
	StartTime     *time.Time `json:"start_time"`
	EndTime       *time.Time `json:"end_time"`
	MediaServerID string     `json:"media_server_id"`
	FileName      string     `json:"file_name"`
	Folder        string     `json:"folder"`
	FilePath      string     `json:"file_path"`
	FileSize      int64      `json:"file_size"`
	TimeLen       int64      `json:"time_len"`
	Collect       *bool      `json:"collect"`
	Remark        string     `json:"remark"`
	CreatedDT     time.Time  `json:"created_dt"`
	UpdatedDT     time.Time  `json:"updated_dt"`
}

// PageGbCloudRecordResp 云端录像分页载荷。
type PageGbCloudRecordResp struct {
	Items    []GbCloudRecordItem `json:"items"`
	Total    int64               `json:"total" example:"128"`
	Page     int                 `json:"page" example:"1"`
	PageSize int                 `json:"page_size" example:"20"`
}

// RPageGbCloudRecord 云端录像分页响应。
type RPageGbCloudRecord struct {
	BaseResponse
	Data PageGbCloudRecordResp `json:"data"`
}

// RGbCloudRecordDates 云端录像有录像的日期列表响应。
type RGbCloudRecordDates struct {
	BaseResponse
	Data []string `json:"data"`
}

// GbCloudRecordPlayResp 云端录像播放地址。
type GbCloudRecordPlayResp struct {
	ID            int64      `json:"id"`
	App           string     `json:"app" example:"record"`
	Stream        string     `json:"stream"`
	HTTPBase      string     `json:"http_base"`
	WebRTC        string     `json:"webrtc"`
	FLV           string     `json:"flv"`
	HLS           string     `json:"hls"`
	WSFLV         string     `json:"ws_flv"`
	RTSP          string     `json:"rtsp"`
	RTMP          string     `json:"rtmp"`
	MediaServerID string     `json:"media_server_id"`
	FileName      string     `json:"file_name"`
	FilePath      string     `json:"file_path"`
	FileSize      int64      `json:"file_size"`
	TimeLen       int64      `json:"time_len"`
	StartTime     *time.Time `json:"start_time"`
	EndTime       *time.Time `json:"end_time"`
}

// RGbCloudRecordPlay 云端录像播放地址响应。
type RGbCloudRecordPlay struct {
	BaseResponse
	Data GbCloudRecordPlayResp `json:"data"`
}

// GbCloudRecordDownloadResp 云端录像下载地址。
//
// Packaging 恒为 false：本批次只做单文件直接下载（设计 §14.5 选项 A），
// 合并/切片/打包需要独立 Assist 服务，未实现时不得在响应里假装支持。
type GbCloudRecordDownloadResp struct {
	ID          int64  `json:"id"`
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type" example:"video/mp4"`
	Packaging   bool   `json:"packaging" example:"false"`
	Message     string `json:"message"`
}

// RGbCloudRecordDownload 云端录像下载地址响应。
type RGbCloudRecordDownload struct {
	BaseResponse
	Data GbCloudRecordDownloadResp `json:"data"`
}

// ── 录像计划 ──────────────────────────────────────────────────

// GbRecordPlanItemResp 录像计划时间段。
type GbRecordPlanItemResp struct {
	ID        int64  `json:"id"`
	WeekDay   int64  `json:"week_day" example:"1"`
	StartTime string `json:"start_time" example:"08:00:00"`
	EndTime   string `json:"end_time" example:"18:00:00"`
}

// GbRecordPlanChannelItem 绑定到计划的通道。
type GbRecordPlanChannelItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	GbChannelID string `json:"gb_channel_id"`
	DeviceCode  string `json:"device_code"`
	DeviceName  string `json:"device_name"`
	Status      string `json:"status" example:"ON"`
}

// GbRecordPlanItem 录像计划列表项。
type GbRecordPlanItem struct {
	ID           int64     `json:"id"`
	Active       *bool     `json:"active"`
	Name         string    `json:"name"`
	Snap         *bool     `json:"snap"`
	Remark       string    `json:"remark"`
	ItemCount    int       `json:"item_count"`
	ChannelCount int64     `json:"channel_count"`
	CreatedDT    time.Time `json:"created_dt"`
	UpdatedDT    time.Time `json:"updated_dt"`
}

// PageGbRecordPlanResp 录像计划分页载荷。
type PageGbRecordPlanResp struct {
	Items    []GbRecordPlanItem `json:"items"`
	Total    int64              `json:"total" example:"6"`
	Page     int                `json:"page" example:"1"`
	PageSize int                `json:"page_size" example:"20"`
}

// RPageGbRecordPlan 录像计划分页响应。
type RPageGbRecordPlan struct {
	BaseResponse
	Data PageGbRecordPlanResp `json:"data"`
}

// GbRecordPlanDetailResp 录像计划详情（含时间段与已绑定通道）。
type GbRecordPlanDetailResp struct {
	GbRecordPlanItem
	Items      []GbRecordPlanItemResp    `json:"items"`
	ChannelIDs []int64                   `json:"channel_ids"`
	Channels   []GbRecordPlanChannelItem `json:"channels"`
}

// RGbRecordPlan 录像计划详情响应。
type RGbRecordPlan struct {
	BaseResponse
	Data GbRecordPlanDetailResp `json:"data"`
}

// RGbRecordPlanCreated 新增录像计划响应（回显数据库实体）。
type RGbRecordPlanCreated struct {
	BaseResponse
	Data model.GbRecordPlan `json:"data"`
}

// ── 国标报警 ──────────────────────────────────────────────────

// GbAlarmItem 国标报警列表项。
type GbAlarmItem struct {
	ID               int64      `json:"id"`
	DeviceID         *int64     `json:"device_id"`
	ChannelID        *int64     `json:"channel_id"`
	DeviceName       string     `json:"device_name"`
	ChannelName      string     `json:"channel_name"`
	GbDeviceID       string     `json:"gb_device_id"`
	GbChannelID      string     `json:"gb_channel_id"`
	AlarmPriority    string     `json:"alarm_priority" example:"1"`
	AlarmMethod      int64      `json:"alarm_method"`
	AlarmType        string     `json:"alarm_type"`
	AlarmTime        *time.Time `json:"alarm_time"`
	AlarmDescription string     `json:"alarm_description"`
	Longitude        float64    `json:"longitude"`
	Latitude         float64    `json:"latitude"`
	SnapPath         string     `json:"snap_path"`
	RecordPath       string     `json:"record_path"`
	Handled          *bool      `json:"handled"`
	Remark           string     `json:"remark"`
	CreatedDT        time.Time  `json:"created_dt"`
	UpdatedDT        time.Time  `json:"updated_dt"`
	// UpdatedBy 是处理人。没有独立的 handler 列，复用 BaseModel.UpdatedBy：
	// 处理动作会写 updated_by，比新增一列更符合现有审计约定。
	UpdatedBy string `json:"updated_by"`
}

// PageGbAlarmResp 国标报警分页载荷。
type PageGbAlarmResp struct {
	Items    []GbAlarmItem `json:"items"`
	Total    int64         `json:"total" example:"42"`
	Page     int           `json:"page" example:"1"`
	PageSize int           `json:"page_size" example:"20"`
}

// RPageGbAlarm 国标报警分页响应。
type RPageGbAlarm struct {
	BaseResponse
	Data PageGbAlarmResp `json:"data"`
}

// GbAlarmDetailResp 国标报警详情。
type GbAlarmDetailResp struct {
	GbAlarmItem
}

// RGbAlarm 国标报警详情响应。
type RGbAlarm struct {
	BaseResponse
	Data GbAlarmDetailResp `json:"data"`
}

// ── 移动位置 ──────────────────────────────────────────────────

// GbPositionResp 一条移动位置。
type GbPositionResp struct {
	ID          int64      `json:"id"`
	DeviceID    *int64     `json:"device_id"`
	DeviceName  string     `json:"device_name"`
	ChannelID   *int64     `json:"channel_id"`
	ChannelName string     `json:"channel_name"`
	GbChannelID string     `json:"gb_channel_id"`
	ReportTime  *time.Time `json:"report_time"`
	Longitude   float64    `json:"longitude"`
	Latitude    float64    `json:"latitude"`
	Altitude    float64    `json:"altitude"`
	Speed       float64    `json:"speed"`
	Direction   float64    `json:"direction"`
}

// RGbPosition 单条位置响应。
type RGbPosition struct {
	BaseResponse
	Data GbPositionResp `json:"data"`
}

// RGbPositionList 实时位置简单列表响应。
type RGbPositionList struct {
	BaseResponse
	Data []GbPositionResp `json:"data"`
}

// GbPositionHistoryResp 位置历史分页载荷。
type GbPositionHistoryResp struct {
	Items    []GbPositionResp `json:"items"`
	Total    int64            `json:"total" example:"500"`
	Page     int              `json:"page" example:"1"`
	PageSize int              `json:"page_size" example:"20"`
}

// RGbPositionHistory 位置历史分页响应。
type RGbPositionHistory struct {
	BaseResponse
	Data GbPositionHistoryResp `json:"data"`
}

// ── 拉流代理 / 推流 ───────────────────────────────────────────

// GbStreamProxyItem 拉流代理列表项（模型无敏感列，直接内嵌）。
type GbStreamProxyItem struct {
	model.GbStreamProxy
}

// PageGbStreamProxyResp 拉流代理分页载荷。
type PageGbStreamProxyResp struct {
	Items    []GbStreamProxyItem `json:"items"`
	Total    int64               `json:"total" example:"8"`
	Page     int                 `json:"page" example:"1"`
	PageSize int                 `json:"page_size" example:"20"`
}

// RPageGbStreamProxy 拉流代理分页响应。
type RPageGbStreamProxy struct {
	BaseResponse
	Data PageGbStreamProxyResp `json:"data"`
}

// RGbStreamProxy 拉流代理单条响应（新增回显）。
type RGbStreamProxy struct {
	BaseResponse
	Data model.GbStreamProxy `json:"data"`
}

// GbStreamPushItem 推流列表项。
//
// PushURL 是平台根据媒体节点配置拼出的外部推流地址（rtmp://...）。
// 本批次没有 ZLM 侧「向外部推流」的 API，Start/Stop 只维护启停状态，
// 真正的推流由外部编码器按该地址推入 ZLM。
type GbStreamPushItem struct {
	model.GbStreamPush
	PushURL string `json:"push_url"`
}

// PageGbStreamPushResp 推流分页载荷。
type PageGbStreamPushResp struct {
	Items    []GbStreamPushItem `json:"items"`
	Total    int64              `json:"total" example:"3"`
	Page     int                `json:"page" example:"1"`
	PageSize int                `json:"page_size" example:"20"`
}

// RPageGbStreamPush 推流分页响应。
type RPageGbStreamPush struct {
	BaseResponse
	Data PageGbStreamPushResp `json:"data"`
}

// RGbStreamPush 推流单条响应（新增回显）。
type RGbStreamPush struct {
	BaseResponse
	Data model.GbStreamPush `json:"data"`
}

// ── 国标级联平台 ──────────────────────────────────────────────

// GbPlatformItem 上级平台列表项。
//
// 内嵌模型以保留全部配置列，但 password 必须在构造时置为 ****，
// 见 service 层的 toGbPlatformItem。
type GbPlatformItem struct {
	model.GbPlatform
}

// PageGbPlatformResp 上级平台分页载荷。
type PageGbPlatformResp struct {
	Items    []GbPlatformItem `json:"items"`
	Total    int64            `json:"total" example:"2"`
	Page     int              `json:"page" example:"1"`
	PageSize int              `json:"page_size" example:"20"`
}

// RPageGbPlatform 上级平台分页响应。
type RPageGbPlatform struct {
	BaseResponse
	Data PageGbPlatformResp `json:"data"`
}

// GbPlatformDetailResp 上级平台详情（含目录可见范围映射）。
type GbPlatformDetailResp struct {
	GbPlatformItem
	GroupIDs  []int64 `json:"group_ids"`
	RegionIDs []int64 `json:"region_ids"`
}

// RGbPlatform 上级平台详情响应。
type RGbPlatform struct {
	BaseResponse
	Data GbPlatformDetailResp `json:"data"`
}

// RGbPlatformCreated 新增上级平台响应。
type RGbPlatformCreated struct {
	BaseResponse
	Data model.GbPlatform `json:"data"`
}

// GbPlatformChannelItem 级联通道列表项。
//
// 不内嵌 model.GbPlatformChannel：它 Preload 了 model.GbChannel，
// 会连带序列化通道 password。
type GbPlatformChannelItem struct {
	ID         int64 `json:"id"`
	PlatformID int64 `json:"platform_id"`
	ChannelID  int64 `json:"channel_id"`

	ChannelName string `json:"channel_name"`
	GbChannelID string `json:"gb_channel_id"`
	DeviceCode  string `json:"device_code"`
	DeviceName  string `json:"device_name"`

	CustomDeviceID     string `json:"custom_device_id"`
	CustomName         string `json:"custom_name"`
	CustomManufacturer string `json:"custom_manufacturer"`
	CustomModel        string `json:"custom_model"`
	CustomOwner        string `json:"custom_owner"`
	CustomCivilCode    string `json:"custom_civil_code"`
	CustomAddress      string `json:"custom_address"`
	CustomParental     int64  `json:"custom_parental"`
	CustomParentID     string `json:"custom_parent_id"`
	CustomStatus       string `json:"custom_status"`
	CustomPTZType      int64  `json:"custom_ptz_type"`
	CustomChannelType  int64  `json:"custom_channel_type"`

	CreatedDT time.Time `json:"created_dt"`
	UpdatedDT time.Time `json:"updated_dt"`
}

// PageGbPlatformChannelResp 级联通道分页载荷。
type PageGbPlatformChannelResp struct {
	Items    []GbPlatformChannelItem `json:"items"`
	Total    int64                   `json:"total" example:"20"`
	Page     int                     `json:"page" example:"1"`
	PageSize int                     `json:"page_size" example:"20"`
}

// RPageGbPlatformChannel 级联通道分页响应。
type RPageGbPlatformChannel struct {
	BaseResponse
	Data PageGbPlatformChannelResp `json:"data"`
}

// ── 语音 Agent 配置 ───────────────────────────────────────────

// VoiceProviderItem 语音 provider 字典项。
type VoiceProviderItem struct {
	ID            int64  `json:"id"`
	Sequence      int    `json:"sequence"`
	Category      string `json:"category" example:"tts"`
	Code          string `json:"code" example:"edge_tts"`
	Name          string `json:"name"`
	ParamSchema   string `json:"param_schema"`
	DefaultParams string `json:"default_params"`
	Remark        string `json:"remark"`
}

// RGbVoiceProviderList 语音 provider 字典简单列表响应。
type RGbVoiceProviderList struct {
	BaseResponse
	Data []VoiceProviderItem `json:"data"`
}

// VoiceAgentConfigResp 语音 Agent 配置。
//
// 敏感键（*_api_key / pipecat_token）返回 ****，其余原样返回。
type VoiceAgentConfigResp struct {
	STTProvider string `json:"stt_provider" example:"local_whisper"`
	STTModel    string `json:"stt_model"`
	STTEndpoint string `json:"stt_endpoint"`
	STTAPIKey   string `json:"stt_api_key" example:"****"`
	STTLanguage string `json:"stt_language"`

	LLMProvider     string  `json:"llm_provider" example:"anthropic"`
	LLMModel        string  `json:"llm_model"`
	LLMEndpoint     string  `json:"llm_endpoint"`
	LLMAPIKey       string  `json:"llm_api_key" example:"****"`
	LLMSystemPrompt string  `json:"llm_system_prompt"`
	LLMTemperature  float64 `json:"llm_temperature" example:"0.7"`
	LLMMaxTokens    int     `json:"llm_max_tokens" example:"1024"`

	TTSProvider string  `json:"tts_provider" example:"edge_tts"`
	TTSVoice    string  `json:"tts_voice"`
	TTSEndpoint string  `json:"tts_endpoint"`
	TTSAPIKey   string  `json:"tts_api_key" example:"****"`
	TTSSpeed    float64 `json:"tts_speed" example:"1"`

	VADProvider string `json:"vad_provider"`
	VADParams   string `json:"vad_params"`

	BargeIn            bool `json:"barge_in" example:"false"`
	SessionIdleTimeout int  `json:"session_idle_timeout" example:"60"`
	MaxSessionSec      int  `json:"max_session_sec" example:"600"`

	PipecatEndpoint string `json:"pipecat_endpoint"`
	PipecatToken    string `json:"pipecat_token" example:"****"`
}

// RVoiceAgentConfig 语音 Agent 配置响应。
type RVoiceAgentConfig struct {
	BaseResponse
	Data VoiceAgentConfigResp `json:"data"`
}

// VoiceAgentTestResp provider 连通性测试结果。
type VoiceAgentTestResp struct {
	Success   bool   `json:"success"`
	Category  string `json:"category" example:"llm"`
	Provider  string `json:"provider"`
	Endpoint  string `json:"endpoint"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Message   string `json:"message"`
}

// RVoiceAgentTest provider 连通性测试响应。
type RVoiceAgentTest struct {
	BaseResponse
	Data VoiceAgentTestResp `json:"data"`
}
