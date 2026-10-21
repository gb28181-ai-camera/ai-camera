package request

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）与 JT/T 1078 —— 分页查询请求
//
// 仅承载列表筛选条件；字段级契约见 docs/开发文档/后端开发/00-接口契约.md。
// 路由规划见 docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md §10。
// ════════════════════════════════════════════════════════════════

// ── 国标设备与通道 ────────────────────────────────────────────

// PageGbDeviceReq 国标设备分页查询。
//
// 只查询已配置国标接入的设备（gb_device_id 非空）；keyword 匹配设备名称/编号/国标编码。
type PageGbDeviceReq struct {
	PageReq
	Keyword        string `form:"keyword" json:"keyword" example:"东门"`
	RegisterStatus string `form:"register_status" json:"register_status" example:"pending"`
	Status         string `form:"status" json:"status" example:"online"`
	RegionID       *int64 `form:"region_id" json:"region_id"`
}

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

// ── 写请求（业务 CRUD 层）────────────────────────────────────

// GbSubscribeActionReq 设备订阅/取消订阅的请求体。
type GbSubscribeActionReq struct {
	// Type 订阅类型：catalog / alarm / mobile_position。
	Type string `json:"type" binding:"required" example:"catalog"`
}

// GbChannelRegionReq 设置通道区域。
//
// RegionID 传 null 表示清空归属区域；传正整数表示绑定该区域。
type GbChannelRegionReq struct {
	RegionID *int64 `json:"region_id"`
}

// GbChannelGroupReq 设置通道业务分组。
//
// GroupID 传 null 表示清空归属分组；传正整数表示绑定该分组。
type GbChannelGroupReq struct {
	GroupID *int64 `json:"group_id"`
}

// UpdateGbChannelReq 编辑国标通道。
//
// 增量语义：字符串非空才覆盖；指针字段非 nil 才覆盖（RegionID/GroupID/RecordPlanID
// 传 0 表示清空）。password 为敏感字段：空值或含 **** 时保持原值。
type UpdateGbChannelReq struct {
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Owner        string `json:"owner"`
	CivilCode    string `json:"civil_code"`
	Address      string `json:"address"`
	ParentID     string `json:"parent_id"`

	ChannelType     *int64 `json:"channel_type"`
	PtzType         *int64 `json:"ptz_type"`
	HasAudio        *bool  `json:"has_audio"`
	Status          string `json:"status" example:"ON"`
	RegionID        *int64 `json:"region_id"`
	GroupID         *int64 `json:"group_id"`
	StreamID        string `json:"stream_id"`
	RecordPlanID    *int64 `json:"record_plan_id"`
	EnableBroadcast *bool  `json:"enable_broadcast"`
	Password        string `json:"password"`
	Active          *bool  `json:"active"`
	Remark          string `json:"remark"`
}

// CreateGbGroupReq 新增国标业务分组。
type CreateGbGroupReq struct {
	Active         *bool  `json:"active"`
	Sequence       int    `json:"sequence"`
	Name           string `json:"name" binding:"required" example:"园区一组"`
	ParentID       *int64 `json:"parent_id"`
	DeviceID       string `json:"device_id"`
	ParentDeviceID string `json:"parent_device_id"`
	BusinessGroup  string `json:"business_group"`
	CivilCode      string `json:"civil_code"`
	Alias          string `json:"alias"`
	Remark         string `json:"remark"`
}

// UpdateGbGroupReq 编辑国标业务分组。
//
// 增量语义：字符串非空才覆盖；指针字段非 nil 才覆盖。
// ParentID 传 0 表示提升为根分组（便于前端用 0 表达「无父节点」）。
type UpdateGbGroupReq struct {
	Active         *bool  `json:"active"`
	Sequence       *int   `json:"sequence"`
	Name           string `json:"name"`
	ParentID       *int64 `json:"parent_id"`
	DeviceID       string `json:"device_id"`
	ParentDeviceID string `json:"parent_device_id"`
	BusinessGroup  string `json:"business_group"`
	CivilCode      string `json:"civil_code"`
	Alias          string `json:"alias"`
	Remark         string `json:"remark"`
}

// CreateGbSubscribeReq 新增订阅记录。
type CreateGbSubscribeReq struct {
	DeviceID   *int64 `json:"device_id"`
	GbDeviceID string `json:"gb_device_id"`
	Type       string `json:"type" binding:"required" example:"catalog"`
	CallID     string `json:"call_id"`
	Expires    int64  `json:"expires" example:"3600"`
	Remark     string `json:"remark"`
}

// CreateGbMediaServerReq 新增流媒体节点。
type CreateGbMediaServerReq struct {
	Active   *bool  `json:"active"`
	Name     string `json:"name" binding:"required"`
	ServerID string `json:"server_id" binding:"required"`
	Type     string `json:"type" example:"zlm"`

	IP       string `json:"ip"`
	HookIP   string `json:"hook_ip"`
	SDPIP    string `json:"sdp_ip"`
	StreamIP string `json:"stream_ip"`

	HTTPPort     int64 `json:"http_port"`
	HTTPSSLPort  int64 `json:"http_ssl_port"`
	RTMPPort     int64 `json:"rtmp_port"`
	RTMPSSLPort  int64 `json:"rtmp_ssl_port"`
	RTSPPort     int64 `json:"rtsp_port"`
	RTSPSSLPort  int64 `json:"rtsp_ssl_port"`
	FLVPort      int64 `json:"flv_port"`
	FLVSSLPort   int64 `json:"flv_ssl_port"`
	WSFLVPort    int64 `json:"ws_flv_port"`
	WSFLVSSLPort int64 `json:"ws_flv_ssl_port"`
	MP4Port      int64 `json:"mp4_port"`
	MP4SSLPort   int64 `json:"mp4_ssl_port"`

	Secret           string `json:"secret"`
	RTPEnable        *bool  `json:"rtp_enable"`
	RTPPortRange     string `json:"rtp_port_range"`
	RTPProxyPort     int64  `json:"rtp_proxy_port"`
	JTTProxyPort     int64  `json:"jtt_proxy_port"`
	SendRTPPortRange string `json:"send_rtp_port_range"`
	RecordAssistPort int64  `json:"record_assist_port"`

	RecordPath      string `json:"record_path"`
	RecordDay       int64  `json:"record_day"`
	TranscodeSuffix string `json:"transcode_suffix"`

	HookAliveInterval int64  `json:"hook_alive_interval"`
	DefaultServer     *bool  `json:"default_server"`
	AutoConfig        *bool  `json:"auto_config"`
	Remark            string `json:"remark"`
}

// UpdateGbMediaServerReq 编辑流媒体节点。
//
// 全部字段使用指针：0 与「未传」语义不同（端口、保留天数等）。
// secret 为敏感字段：空值或含 **** 时保持原值。
// online / last_keepalive_dt 由 ZLM 心跳维护，不开放写入。
type UpdateGbMediaServerReq struct {
	Active   *bool   `json:"active"`
	Name     *string `json:"name"`
	ServerID *string `json:"server_id"`
	Type     *string `json:"type"`

	IP       *string `json:"ip"`
	HookIP   *string `json:"hook_ip"`
	SDPIP    *string `json:"sdp_ip"`
	StreamIP *string `json:"stream_ip"`

	HTTPPort     *int64 `json:"http_port"`
	HTTPSSLPort  *int64 `json:"http_ssl_port"`
	RTMPPort     *int64 `json:"rtmp_port"`
	RTMPSSLPort  *int64 `json:"rtmp_ssl_port"`
	RTSPPort     *int64 `json:"rtsp_port"`
	RTSPSSLPort  *int64 `json:"rtsp_ssl_port"`
	FLVPort      *int64 `json:"flv_port"`
	FLVSSLPort   *int64 `json:"flv_ssl_port"`
	WSFLVPort    *int64 `json:"ws_flv_port"`
	WSFLVSSLPort *int64 `json:"ws_flv_ssl_port"`
	MP4Port      *int64 `json:"mp4_port"`
	MP4SSLPort   *int64 `json:"mp4_ssl_port"`

	Secret           *string `json:"secret"`
	RTPEnable        *bool   `json:"rtp_enable"`
	RTPPortRange     *string `json:"rtp_port_range"`
	RTPProxyPort     *int64  `json:"rtp_proxy_port"`
	JTTProxyPort     *int64  `json:"jtt_proxy_port"`
	SendRTPPortRange *string `json:"send_rtp_port_range"`
	RecordAssistPort *int64  `json:"record_assist_port"`

	RecordPath      *string `json:"record_path"`
	RecordDay       *int64  `json:"record_day"`
	TranscodeSuffix *string `json:"transcode_suffix"`

	HookAliveInterval *int64  `json:"hook_alive_interval"`
	DefaultServer     *bool   `json:"default_server"`
	AutoConfig        *bool   `json:"auto_config"`
	Remark            *string `json:"remark"`
}

// GbSipLogStatsReq 信令日志统计查询。
type GbSipLogStatsReq struct {
	DateRangeReq
	GbDeviceID string `form:"gb_device_id" json:"gb_device_id"`
	Direction  string `form:"direction" json:"direction" example:"in"`
	Method     string `form:"method" json:"method" example:"REGISTER"`
	CmdType    string `form:"cmd_type" json:"cmd_type" example:"Catalog"`
	Result     string `form:"result" json:"result" example:"success"`
}
