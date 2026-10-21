package response

import (
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）—— 业务 CRUD 层响应类型
//
// 命名遵循接口契约 §2.1 / §13：
//   - RPageXxx  → Data PageXxxResp{Items, Total, Page, PageSize}
//   - RXxxList  → Data []XxxItem
//   - RXxx      → Data XxxDetailResp
//
// 列表项刻意不直接返回 model 实体，原因有二：
//   1) model.Device 携带 sip_password 明文，直接序列化会泄露敏感字段；
//   2) 列表需要展示关联表名称（region_name / device_name / group_name），
//      而不是让前端再从嵌套对象里取，避免前端为了一个名称做二次请求。
// ════════════════════════════════════════════════════════════════

// ── 国标设备 ──────────────────────────────────────────────────

// GbDeviceItem 国标设备列表项。
//
// 包含 ac_device 的 13 个国标接入列（sip_password 已脱敏为 ****）。
type GbDeviceItem struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	DeviceCode string `json:"device_code"`
	RegionID   *int64 `json:"region_id"`
	RegionName string `json:"region_name" example:"东门区域"`
	Location   string `json:"location"`
	Latitude   string `json:"latitude"`
	Longitude  string `json:"longitude"`
	Status     string `json:"status" example:"online"`
	Active     *bool  `json:"active"`

	// ── 国标接入 13 列 ──
	GbDeviceID     string     `json:"gb_device_id" example:"34020000001320000001"`
	SipPassword    string     `json:"sip_password" example:"****"`
	SipRealm       string     `json:"sip_realm"`
	SipTransport   string     `json:"sip_transport" example:"UDP"`
	StreamMode     string     `json:"stream_mode" example:"UDP"`
	Charset        string     `json:"charset" example:"GB2312"`
	RegisterStatus string     `json:"register_status" example:"pending"`
	RegisterDT     *time.Time `json:"register_dt"`
	SipExpires     *time.Time `json:"sip_expires"`
	MediaServerID  string     `json:"media_server_id" example:"auto"`
	ChannelCount   int64      `json:"channel_count"`
	LocalIP        string     `json:"local_ip"`
	SDPIP          string     `json:"sdp_ip"`

	Remark    string    `json:"remark"`
	CreatedDT time.Time `json:"created_dt"`
	UpdatedDT time.Time `json:"updated_dt"`
}

// PageGbDeviceResp 国标设备分页载荷。
type PageGbDeviceResp struct {
	Items    []GbDeviceItem `json:"items"`
	Total    int64          `json:"total" example:"12"`
	Page     int            `json:"page" example:"1"`
	PageSize int            `json:"page_size" example:"20"`
}

// RPageGbDevice 国标设备分页响应。
type RPageGbDevice struct {
	BaseResponse
	Data PageGbDeviceResp `json:"data"`
}

// GbDeviceDetailResp 国标设备详情。
type GbDeviceDetailResp struct {
	GbDeviceItem
	// MediaServerName 绑定节点的展示名（media_server_id 为 auto 时为空）。
	MediaServerName string        `json:"media_server_name"`
	Region          *model.Region `json:"region,omitempty"`
}

// RGbDevice 国标设备详情响应。
type RGbDevice struct {
	BaseResponse
	Data GbDeviceDetailResp `json:"data"`
}

// GbActivationResp 设备激活凭证。
//
// ★ SipPassword 这里是**明文**：设备首次配置必须拿到真实密码才能注册，
// 与详情接口的 **** 语义不同。该接口必须由 gb:device:activation 权限码保护。
type GbActivationResp struct {
	DeviceID       int64  `json:"device_id"`
	GbDeviceID     string `json:"gb_device_id" example:"34020000001320000001"`
	SipPassword    string `json:"sip_password" example:"Ab3dEf6hJk9mNp2q"`
	SipServer      string `json:"sip_server" example:"192.168.1.10:5060"`
	SipPort        int    `json:"sip_port" example:"5060"`
	SipDomain      string `json:"sip_domain" example:"3402000000"`
	Transport      string `json:"transport" example:"UDP"`
	RegisterStatus string `json:"register_status" example:"pending"`
	// QrContent 二维码内容：一段 JSON 字符串，前端直接画二维码即可。
	QrContent string `json:"qr_content"`
}

// RGbActivation 设备激活凭证响应。
type RGbActivation struct {
	BaseResponse
	Data GbActivationResp `json:"data"`
}

// ── 国标通道 ──────────────────────────────────────────────────

// GbChannelItem 国标通道列表项。
//
// 关联名称（device_name / region_name / group_name）由 service 层填充，
// 免去前端为展示一个名称而二次请求。
type GbChannelItem struct {
	ID         int64  `json:"id"`
	Active     *bool  `json:"active"`
	DeviceID   int64  `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceCode string `json:"device_code"`
	GbDeviceID string `json:"gb_device_id"`
	RegionName string `json:"region_name"`
	GroupName  string `json:"group_name"`

	GbChannelID  string `json:"gb_channel_id" example:"34020000001320000001"`
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Owner        string `json:"owner"`
	CivilCode    string `json:"civil_code"`
	Address      string `json:"address"`
	Parental     int64  `json:"parental"`
	ParentID     string `json:"parent_id"`

	ChannelType int64  `json:"channel_type"`
	PtzType     int64  `json:"ptz_type"`
	HasAudio    *bool  `json:"has_audio"`
	Status      string `json:"status" example:"ON"`

	RegionID *int64 `json:"region_id"`
	GroupID  *int64 `json:"group_id"`

	StreamID        string `json:"stream_id"`
	RecordPlanID    *int64 `json:"record_plan_id"`
	RecordPlanName  string `json:"record_plan_name"`
	EnableBroadcast *bool  `json:"enable_broadcast"`

	IsCustom  *bool      `json:"is_custom"`
	SyncDT    *time.Time `json:"sync_dt"`
	Remark    string     `json:"remark"`
	CreatedDT time.Time  `json:"created_dt"`
	UpdatedDT time.Time  `json:"updated_dt"`
}

// PageGbChannelResp 国标通道分页载荷。
type PageGbChannelResp struct {
	Items    []GbChannelItem `json:"items"`
	Total    int64           `json:"total" example:"32"`
	Page     int             `json:"page" example:"1"`
	PageSize int             `json:"page_size" example:"20"`
}

// RPageGbChannel 国标通道分页响应。
type RPageGbChannel struct {
	BaseResponse
	Data PageGbChannelResp `json:"data"`
}

// GbChannelDetailResp 国标通道详情。
type GbChannelDetailResp struct {
	GbChannelItem
	// Password 设备侧上报的通道密码，读接口脱敏为 ****。
	Password string `json:"password" example:"****"`
}

// RGbChannel 国标通道详情响应。
type RGbChannel struct {
	BaseResponse
	Data GbChannelDetailResp `json:"data"`
}

// ── 媒体节点 ──────────────────────────────────────────────────

// GbMediaServerItem 流媒体节点列表项（节点数量少，接口为简单列表）。
type GbMediaServerItem struct {
	ID       int64  `json:"id"`
	Active   *bool  `json:"active"`
	Name     string `json:"name"`
	ServerID string `json:"server_id"`
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

	Secret           string `json:"secret" example:"****"`
	RTPEnable        *bool  `json:"rtp_enable"`
	RTPPortRange     string `json:"rtp_port_range"`
	RTPProxyPort     int64  `json:"rtp_proxy_port"`
	JTTProxyPort     int64  `json:"jtt_proxy_port"`
	SendRTPPortRange string `json:"send_rtp_port_range"`
	RecordAssistPort int64  `json:"record_assist_port"`

	RecordPath      string `json:"record_path"`
	RecordDay       int64  `json:"record_day"`
	TranscodeSuffix string `json:"transcode_suffix"`

	Online            *bool      `json:"online"`
	LastKeepaliveDT   *time.Time `json:"last_keepalive_dt"`
	HookAliveInterval int64      `json:"hook_alive_interval"`
	DefaultServer     *bool      `json:"default_server"`
	AutoConfig        *bool      `json:"auto_config"`

	Remark    string    `json:"remark"`
	CreatedDT time.Time `json:"created_dt"`
	UpdatedDT time.Time `json:"updated_dt"`
}

// RGbMediaServerList 流媒体节点简单列表响应。
type RGbMediaServerList struct {
	BaseResponse
	Data []GbMediaServerItem `json:"data"`
}

// GbMediaServerDetailResp 流媒体节点详情。
type GbMediaServerDetailResp struct {
	GbMediaServerItem
}

// RGbMediaServer 流媒体节点详情响应。
type RGbMediaServer struct {
	BaseResponse
	Data GbMediaServerDetailResp `json:"data"`
}

// RGbMediaServerCreated 新增节点响应（回显数据库实体，含自动生成的主键/默认值）。
type RGbMediaServerCreated struct {
	BaseResponse
	Data model.GbMediaServer `json:"data"`
}

// GbMediaServerCheckResp 媒体节点连通性检查结果。
type GbMediaServerCheckResp struct {
	Online          bool   `json:"online"`
	Version         string `json:"version"`
	RTPPort         int64  `json:"rtp_port"`
	RTCPPort        int64  `json:"rtcp_port"`
	HTTPPort        int64  `json:"http_port"`
	MediaServerID   string `json:"media_server_id"`
	SecretMatched   bool   `json:"secret_matched"`
	ResponseElapsed int64  `json:"response_elapsed_ms"`
	Message         string `json:"message"`
}

// RGbMediaServerCheck 媒体节点连通性检查响应。
type RGbMediaServerCheck struct {
	BaseResponse
	Data GbMediaServerCheckResp `json:"data"`
}

// GbMediaServerThreadLoad 单线程负载。
type GbMediaServerThreadLoad struct {
	ThreadName string  `json:"thread_name"`
	Load       float64 `json:"load"`
}

// GbMediaServerInfoResp 媒体节点运行信息汇总。
type GbMediaServerInfoResp struct {
	Online        bool                      `json:"online"`
	Version       string                    `json:"version"`
	ActiveStreams int                       `json:"active_streams"`
	ThreadCount   int                       `json:"thread_count"`
	AvgLoad       float64                   `json:"avg_load"`
	MaxLoad       float64                   `json:"max_load"`
	Loads         []GbMediaServerThreadLoad `json:"loads"`
	Message       string                    `json:"message"`
}

// RGbMediaServerInfo 媒体节点运行信息响应。
type RGbMediaServerInfo struct {
	BaseResponse
	Data GbMediaServerInfoResp `json:"data"`
}

// ── 业务分组 ──────────────────────────────────────────────────

// GbGroupTreeNode 业务分组树节点。
type GbGroupTreeNode struct {
	model.GbGroup
	Children []*GbGroupTreeNode `json:"children"`
}

// RGbGroupTree 业务分组树响应。
type RGbGroupTree struct {
	BaseResponse
	Data []*GbGroupTreeNode `json:"data"`
}

// RGbGroupList 业务分组简单列表响应。
type RGbGroupList struct {
	BaseResponse
	Data []model.GbGroup `json:"data"`
}

// RGbGroup 单条业务分组响应（新增回显）。
type RGbGroup struct {
	BaseResponse
	Data model.GbGroup `json:"data"`
}

// ── 订阅 ──────────────────────────────────────────────────────

// GbSubscribeItem 订阅记录列表项。
type GbSubscribeItem struct {
	model.GbSubscribe
	// DeviceName 归属设备名称（列表展示用，关联 ac_device）。
	DeviceName string `json:"device_name"`
}

// PageGbSubscribeResp 订阅记录分页载荷。
type PageGbSubscribeResp struct {
	Items    []GbSubscribeItem `json:"items"`
	Total    int64             `json:"total" example:"8"`
	Page     int               `json:"page" example:"1"`
	PageSize int               `json:"page_size" example:"20"`
}

// RPageGbSubscribe 订阅记录分页响应。
type RPageGbSubscribe struct {
	BaseResponse
	Data PageGbSubscribeResp `json:"data"`
}

// RGbSubscribe 订阅记录详情响应（创建后回显）。
type RGbSubscribe struct {
	BaseResponse
	Data model.GbSubscribe `json:"data"`
}

// ── 信令日志 ──────────────────────────────────────────────────

// PageGbSipLogResp 信令日志分页载荷。
//
// ac_gb_sip_log 不嵌 BaseModel，无 dept_id，不走 DataScope；
// 可见性由接口级权限码 gb:sip-log:list 控制。
type PageGbSipLogResp struct {
	Items    []model.GbSipLog `json:"items"`
	Total    int64            `json:"total" example:"1280"`
	Page     int              `json:"page" example:"1"`
	PageSize int              `json:"page_size" example:"20"`
}

// RPageGbSipLog 信令日志分页响应。
type RPageGbSipLog struct {
	BaseResponse
	Data PageGbSipLogResp `json:"data"`
}

// RGbSipLog 信令日志详情响应。
type RGbSipLog struct {
	BaseResponse
	Data model.GbSipLog `json:"data"`
}

// GbSipLogStatItem 分组统计项（按 method / cmd_type 聚合）。
type GbSipLogStatItem struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// GbSipLogStatsResp 信令日志统计结果。
type GbSipLogStatsResp struct {
	Total    int64              `json:"total"`
	Success  int64              `json:"success"`
	Failed   int64              `json:"failed"`
	Rejected int64              `json:"rejected"`
	Inbound  int64              `json:"inbound"`
	Outbound int64              `json:"outbound"`
	ByMethod []GbSipLogStatItem `json:"by_method"`
	ByCmd    []GbSipLogStatItem `json:"by_cmd_type"`
}

// RGbSipLogStats 信令日志统计响应。
type RGbSipLogStats struct {
	BaseResponse
	Data GbSipLogStatsResp `json:"data"`
}
