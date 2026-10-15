package request

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）与 JT/T 1078 —— 分页查询请求
//
// 仅承载列表筛选条件；字段级契约见 docs/开发文档/后端开发/00-接口契约.md。
// 路由规划见 docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md §10。
// ════════════════════════════════════════════════════════════════

// ── 国标设备与通道 ────────────────────────────────────────────

// PageGbChannelReq 国标通道分页查询。
type PageGbChannelReq struct {
	PageReq
	Keyword    string `form:"keyword" json:"keyword" example:"东门"`
	DeviceID   *int64 `form:"device_id" json:"device_id"`
	RegionID   *int64 `form:"region_id" json:"region_id"`
	GroupID    *int64 `form:"group_id" json:"group_id"`
	Status     string `form:"status" json:"status" example:"ON"`
	GbDeviceID string `form:"gb_device_id" json:"gb_device_id" example:"34020000001320000001"`
}

// PageGbGroupReq 国标业务分组分页查询。
type PageGbGroupReq struct {
	PageReq
	Keyword  string `form:"keyword" json:"keyword"`
	ParentID *int64 `form:"parent_id" json:"parent_id"`
}

// ── 媒体节点与流 ──────────────────────────────────────────────

// PageGbMediaServerReq 流媒体节点分页查询。
type PageGbMediaServerReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword"`
	Online  *bool  `form:"online" json:"online"`
}

// PageGbStreamProxyReq 拉流代理分页查询。
type PageGbStreamProxyReq struct {
	PageReq
	Keyword       string `form:"keyword" json:"keyword"`
	Pulling       *bool  `form:"pulling" json:"pulling"`
	MediaServerID string `form:"media_server_id" json:"media_server_id"`
}

// PageGbStreamPushReq 推流分页查询。
type PageGbStreamPushReq struct {
	PageReq
	Keyword       string `form:"keyword" json:"keyword"`
	Pushing       *bool  `form:"pushing" json:"pushing"`
	MediaServerID string `form:"media_server_id" json:"media_server_id"`
	Status        string `form:"status" json:"status" example:"ON"`
}

// ── 录像 ──────────────────────────────────────────────────────

// PageGbCloudRecordReq 云端录像分页查询。
type PageGbCloudRecordReq struct {
	PageReq
	DateRangeReq
	Keyword       string `form:"keyword" json:"keyword"`
	App           string `form:"app" json:"app"`
	Stream        string `form:"stream" json:"stream"`
	DeviceID      *int64 `form:"device_id" json:"device_id"`
	ChannelID     *int64 `form:"channel_id" json:"channel_id"`
	MediaServerID string `form:"media_server_id" json:"media_server_id"`
	Collect       *bool  `form:"collect" json:"collect"`
}

// PageGbRecordPlanReq 录像计划分页查询。
type PageGbRecordPlanReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword"`
}

// ── 报警、位置、订阅、信令日志 ────────────────────────────────

// PageGbAlarmReq 国标报警分页查询。
type PageGbAlarmReq struct {
	PageReq
	DateRangeReq
	Keyword       string `form:"keyword" json:"keyword"`
	DeviceID      *int64 `form:"device_id" json:"device_id"`
	ChannelID     *int64 `form:"channel_id" json:"channel_id"`
	AlarmType     string `form:"alarm_type" json:"alarm_type"`
	AlarmPriority string `form:"alarm_priority" json:"alarm_priority"`
	Handled       *bool  `form:"handled" json:"handled"`
}

// PageGbMobilePositionReq 移动位置分页查询。
type PageGbMobilePositionReq struct {
	PageReq
	DateRangeReq
	DeviceID    *int64 `form:"device_id" json:"device_id"`
	ChannelID   *int64 `form:"channel_id" json:"channel_id"`
	GbChannelID string `form:"gb_channel_id" json:"gb_channel_id"`
}

// PageGbSubscribeReq 国标订阅记录分页查询。
type PageGbSubscribeReq struct {
	PageReq
	GbDeviceID string `form:"gb_device_id" json:"gb_device_id"`
	Type       string `form:"type" json:"type" example:"catalog"`
	Status     string `form:"status" json:"status" example:"active"`
}

// PageGbSipLogReq 信令日志分页查询。
// 注意：ac_gb_sip_log 无 dept_id，**不走数据权限**，可见性由接口层权限码控制。
type PageGbSipLogReq struct {
	PageReq
	DateRangeReq
	GbDeviceID  string `form:"gb_device_id" json:"gb_device_id"`
	GbChannelID string `form:"gb_channel_id" json:"gb_channel_id"`
	Direction   string `form:"direction" json:"direction" example:"in"`
	Method      string `form:"method" json:"method" example:"REGISTER"`
	CmdType     string `form:"cmd_type" json:"cmd_type" example:"Catalog"`
	Result      string `form:"result" json:"result" example:"success"`
}

// ── 语音 ──────────────────────────────────────────────────────

// PageVoiceSessionReq 语音会话分页查询。
type PageVoiceSessionReq struct {
	PageReq
	Type      string `form:"type" json:"type" example:"broadcast"`
	Status    string `form:"status" json:"status" example:"active"`
	DeviceID  *int64 `form:"device_id" json:"device_id"`
	ChannelID *int64 `form:"channel_id" json:"channel_id"`
}

// PageVoiceProviderReq 语音 provider 字典分页查询。
type PageVoiceProviderReq struct {
	PageReq
	Category string `form:"category" json:"category" example:"tts"`
	Keyword  string `form:"keyword" json:"keyword"`
}

// ── 级联 ──────────────────────────────────────────────────────

// PageGbPlatformReq 上级平台分页查询。
type PageGbPlatformReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword"`
	Status  string `form:"status" json:"status" example:"ON"`
}

// PageGbPlatformChannelReq 级联通道分页查询。
type PageGbPlatformChannelReq struct {
	PageReq
	Keyword    string `form:"keyword" json:"keyword"`
	PlatformID *int64 `form:"platform_id" json:"platform_id"`
	ChannelID  *int64 `form:"channel_id" json:"channel_id"`
}

// ── JT/T 1078 车载终端（三期）─────────────────────────────────

// PageJtTerminalReq JT1078 终端分页查询。
type PageJtTerminalReq struct {
	PageReq
	Keyword    string `form:"keyword" json:"keyword"`
	Status     string `form:"status" json:"status" example:"ON"`
	PlateNo    string `form:"plate_no" json:"plate_no"`
	TerminalID string `form:"terminal_id" json:"terminal_id"`
}

// PageJtChannelReq JT1078 通道分页查询。
type PageJtChannelReq struct {
	PageReq
	Keyword      string `form:"keyword" json:"keyword"`
	TerminalDBID *int64 `form:"terminal_db_id" json:"terminal_db_id"`
	Status       string `form:"status" json:"status" example:"ON"`
}
