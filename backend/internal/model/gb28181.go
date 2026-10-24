package model

import "time"

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）与 JT/T 1078 —— 数据层模型
//
// 来源：docs/开发文档/数据库设计/sql/ddl/09-国标视频.sql（20 张表 + ac_device 扩展 14 列，含 gb_version）
// 设计依据：docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md
//   §4 设备接入与白名单、§5 数据库设计、§10.1 字段契约与数据权限
// 参考实现：wvp-GB28181-pro v2.7.4 的 wvp_* 表（语义对齐，表名改为 ac_ 前缀）
//
// 约定：
//  1. DDL 是规格，本文件逐列 1:1 对齐。DDL 与设计文档冲突时以 DDL 为准。
//  2. 除 GbSipLog 外，其余 19 张表均嵌入 BaseModel（id / 软删除 / 审计字段）。
//     GbSipLog 是追加写日志表：无 BaseModel、无 deleted_at、无 dept_id。
//  3. 区域复用既有 ac_region（不引入 wvp_common_region）。
//  4. 语音配置不建新表 —— 复用 ac_thirdparty_config（Category='voice_agent'），
//     仅 provider 字典（VoiceProvider）需要新表以驱动前端动态表单。
// ════════════════════════════════════════════════════════════════

// ────────────────────────────────────────────────────────────────
// 常量：注册状态 / 传输模式 / 字符集（对应 ac_device 扩展列取值）
// ────────────────────────────────────────────────────────────────

// 设备 SIP 注册状态（ac_device.register_status）。
const (
	GbRegisterStatusPending   = "pending"   // 已建档，等待设备首次注册
	GbRegisterStatusActivated = "activated" // 首次 REGISTER 鉴权通过
	GbRegisterStatusRejected  = "rejected"  // 鉴权失败
)

// SIP 传输模式（ac_device.sip_transport）。
const (
	GbSipTransportUDP = "UDP"
	GbSipTransportTCP = "TCP"
)

// 媒体传输模式（ac_device.stream_mode）。
const (
	GbStreamModeUDP        = "UDP"
	GbStreamModeTCPPassive = "TCP-PASSIVE"
	GbStreamModeTCPActive  = "TCP-ACTIVE"
)

// XML 报文编码（ac_device.charset）。
const (
	GbCharsetGB2312 = "GB2312"
	GbCharsetUTF8   = "UTF-8"
)

// 通道 / 设备在线状态取值（ac_gb_channel.status 等）。
const (
	GbStatusOn  = "ON"
	GbStatusOff = "OFF"
)

// 国标协议版本（ac_device.gb_version）。
//
// 默认 2016：老设备占绝大多数，未知时按老版本处理，避免下发 2022 独有命令。
// 判定逻辑在 internal/gb28181/protocol/version 包，不在模型层。
const (
	GbVersion2016 = "2016"
	GbVersion2022 = "2022"
)

// 国标订阅类型（ac_gb_subscribe.type）。
const (
	GbSubscribeCatalog        = "catalog"
	GbSubscribeAlarm          = "alarm"
	GbSubscribeMobilePosition = "mobile_position"
	// GbSubscribePTZPosition 是 GB/T 28181-2022 的 PTZ 精准位置变化订阅。
	// 与 catalog/alarm/mobile_position 并列；命名与 ac_gb_subscribe.type 一致。
	GbSubscribePTZPosition = "ptz_position"
)

// 语音会话类型（ac_voice_session.type）。
const (
	VoiceSessionBroadcast = "broadcast" // 广播
	VoiceSessionTalk      = "talk"      // 对讲
	VoiceSessionAgent     = "agent"     // 语音 Agent
)

// 语音 provider 类别（ac_voice_provider.category）。
const (
	VoiceProviderSTT = "stt"
	VoiceProviderLLM = "llm"
	VoiceProviderTTS = "tts"
	VoiceProviderVAD = "vad"
)

// ════════════════════════════════════════════════════════════════
// 第 1 节：国标通道与目录
// ════════════════════════════════════════════════════════════════

// GbChannel 国标通道。表 ac_gb_channel。
//
// GB28181 是「1 设备 + N 通道」，云台与点播均作用于通道，故通道是国标场景的核心对象。
// 归属设备用 device_id 指向 ac_device；gb_device_id / device_code 为冗余列，便于列表直接展示。
type GbChannel struct {
	BaseModel
	Active *bool `gorm:"column:active;default:true" json:"active"`

	// 归属设备
	DeviceID   int64  `gorm:"column:device_id;not null;index;comment:归属设备ID(ac_device)" json:"device_id"`
	DeviceCode string `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	GbDeviceID string `gorm:"column:gb_device_id;size:20;comment:归属设备的国标编码(冗余)" json:"gb_device_id"`

	// 通道身份
	GbChannelID  string `gorm:"column:gb_channel_id;size:20;not null;uniqueIndex:idx_ac_gb_channel_gb_channel_id;comment:通道国标20位编码,唯一" json:"gb_channel_id"`
	Name         string `gorm:"column:name;size:255;comment:通道名称(GB2312转码后存储)" json:"name"`
	Manufacturer string `gorm:"column:manufacturer;size:100;comment:厂商" json:"manufacturer"`
	Model        string `gorm:"column:model;size:100;comment:型号" json:"model"`
	Owner        string `gorm:"column:owner;size:100;comment:归属单位" json:"owner"`
	CivilCode    string `gorm:"column:civil_code;size:20;comment:行政区划代码" json:"civil_code"`
	Address      string `gorm:"column:address;size:255;comment:安装地址" json:"address"`
	Parental     int64  `gorm:"column:parental;default:0;comment:是否有子设备 0无/1有" json:"parental"`
	ParentID     string `gorm:"column:parent_id;size:20;comment:父通道国标编码" json:"parent_id"`

	// 能力
	ChannelType int64  `gorm:"column:channel_type;default:0;comment:通道类型 0普通/1球机/2半球/3固定枪机" json:"channel_type"`
	PtzType     int64  `gorm:"column:ptz_type;default:0;comment:云台类型 0不支持/1球机/2半球" json:"ptz_type"`
	HasAudio    *bool  `gorm:"column:has_audio;default:false;comment:是否支持音频" json:"has_audio"`
	Status      string `gorm:"column:status;size:20;default:OFF;comment:通道状态 ON/OFF" json:"status"`

	// 组织
	RegionID *int64 `gorm:"column:region_id;index;comment:所属区域(复用ac_region)" json:"region_id"`
	GroupID  *int64 `gorm:"column:group_id;index;comment:所属业务分组(ac_gb_group)" json:"group_id"`

	// 流与录像
	StreamID        string `gorm:"column:stream_id;size:100;comment:绑定流ID" json:"stream_id"`
	RecordPlanID    *int64 `gorm:"column:record_plan_id;comment:绑定录像计划" json:"record_plan_id"`
	EnableBroadcast *bool  `gorm:"column:enable_broadcast;default:false;comment:是否允许语音广播" json:"enable_broadcast"`

	// 设备上报的属性（对应「设备参数」页面）。WVP 保留 85 列，此处只留国标 2016 常用项。
	Block                string     `gorm:"column:block;size:20;comment:阻塞" json:"block"`
	SafetyWay            string     `gorm:"column:safety_way;size:50;comment:安全方式" json:"safety_way"`
	RegisterWay          string     `gorm:"column:register_way;size:20;comment:注册方式" json:"register_way"`
	CertNum              string     `gorm:"column:cert_num;size:50;comment:证书序号" json:"cert_num"`
	Certifiable          int64      `gorm:"column:certifiable;comment:证书有效" json:"certifiable"`
	ErrCode              int64      `gorm:"column:err_code;comment:错误码" json:"err_code"`
	EndTime              int64      `gorm:"column:end_time;comment:结束时间" json:"end_time"`
	Secrecy              int64      `gorm:"column:secrecy;comment:保密属性" json:"secrecy"`
	IPAddress            string     `gorm:"column:ip_address;size:50;comment:IP地址" json:"ip_address"`
	Port                 int64      `gorm:"column:port;comment:端口" json:"port"`
	Password             string     `gorm:"column:password;size:100;comment:设备密码" json:"password"`
	Longitude            float64    `gorm:"column:longitude;comment:经度" json:"longitude"`
	Latitude             float64    `gorm:"column:latitude;comment:纬度" json:"latitude"`
	PositionType         int64      `gorm:"column:position_type;comment:位置类型" json:"position_type"`
	RoomType             int64      `gorm:"column:room_type;comment:房间类型" json:"room_type"`
	UseType              int64      `gorm:"column:use_type;comment:用途类型" json:"use_type"`
	SupplyLightType      int64      `gorm:"column:supply_light_type;comment:补光类型" json:"supply_light_type"`
	DirectionType        int64      `gorm:"column:direction_type;comment:方向类型" json:"direction_type"`
	Resolution           string     `gorm:"column:resolution;size:50;comment:分辨率" json:"resolution"`
	DownloadSpeed        string     `gorm:"column:download_speed;size:50;comment:下载速度" json:"download_speed"`
	SubCount             int64      `gorm:"column:sub_count;comment:子设备数" json:"sub_count"`
	GPSTime              *time.Time `gorm:"column:gps_time;comment:GPS时间" json:"gps_time"`
	StreamIdentification string     `gorm:"column:stream_identification;size:100;comment:流标识" json:"stream_identification"`
	MapLevel             int64      `gorm:"column:map_level;comment:地图层级" json:"map_level"`

	// 用户定制值标记。本期不保留 WVP 的「gb_* 双列」语义（用户改名直接覆盖），
	// 已记入设计文档 §11+「有意差异」。
	IsCustom *bool `gorm:"column:is_custom;default:false;comment:名称等是否为用户定制值" json:"is_custom"`

	// 目录同步
	SyncDT *time.Time `gorm:"column:sync_dt;comment:最近一次目录同步时间" json:"sync_dt"`
	Remark string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	// 关联对象（列表展示名称用）
	Device     *Device       `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Region     *Region       `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
	Group      *GbGroup      `gorm:"foreignKey:GroupID;references:ID" json:"group,omitempty"`
	RecordPlan *GbRecordPlan `gorm:"foreignKey:RecordPlanID;references:ID" json:"record_plan,omitempty"`
}

func (GbChannel) TableName() string { return "ac_gb_channel" }

// GbGroup 国标业务分组。表 ac_gb_group。
// 树形结构，与区域（ac_region）正交；parent_id 为自引用父分组。
type GbGroup struct {
	BaseModel
	Active   *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name     string `gorm:"column:name;size:255;not null;comment:分组名称" json:"name"`
	ParentID *int64 `gorm:"column:parent_id;index;comment:父分组ID" json:"parent_id"`

	DeviceID       string `gorm:"column:device_id;size:20;comment:分组国标编码" json:"device_id"`
	ParentDeviceID string `gorm:"column:parent_device_id;size:20;comment:父分组国标编码" json:"parent_device_id"`
	BusinessGroup  string `gorm:"column:business_group;size:20;comment:业务分组类型编码" json:"business_group"`
	CivilCode      string `gorm:"column:civil_code;size:20;comment:行政区划代码" json:"civil_code"`
	Alias          string `gorm:"column:alias;size:255;comment:别名" json:"alias"`
	Remark         string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbGroup) TableName() string { return "ac_gb_group" }

// ════════════════════════════════════════════════════════════════
// 第 2 节：媒体节点与流
// ════════════════════════════════════════════════════════════════

// GbMediaServer 流媒体节点。表 ac_gb_media_server。
//
// 对应一个 ZLMediaKit 实例。ip / hook_ip / sdp_ip / stream_ip 用途不同，
// 多网卡/NAT 场景必须分开配置（见设计文档 §7.2）。
type GbMediaServer struct {
	BaseModel
	Active   *bool  `gorm:"column:active;default:true" json:"active"`
	Name     string `gorm:"column:name;size:255;not null;comment:节点名称" json:"name"`
	ServerID string `gorm:"column:server_id;size:100;not null;uniqueIndex:idx_ac_gb_media_server_server_id;comment:节点唯一标识(ZLM mediaServerId)" json:"server_id"`
	Type     string `gorm:"column:type;size:20;default:zlm;comment:节点类型,默认zlm" json:"type"`

	// 四个地址：用途不同，多网卡/NAT 场景必须分开配置
	IP       string `gorm:"column:ip;size:50;comment:ZLM API 访问地址" json:"ip"`
	HookIP   string `gorm:"column:hook_ip;size:50;comment:ZLM 回调本服务的地址" json:"hook_ip"`
	SDPIP    string `gorm:"column:sdp_ip;size:50;comment:SDP 中通告给设备的IP" json:"sdp_ip"`
	StreamIP string `gorm:"column:stream_ip;size:50;comment:对外播放地址IP" json:"stream_ip"`

	// 端口
	HTTPPort     int64 `gorm:"column:http_port;comment:HTTP端口" json:"http_port"`
	HTTPSSLPort  int64 `gorm:"column:http_ssl_port;comment:HTTPS端口" json:"http_ssl_port"`
	RTMPPort     int64 `gorm:"column:rtmp_port;comment:RTMP端口" json:"rtmp_port"`
	RTMPSSLPort  int64 `gorm:"column:rtmp_ssl_port;comment:RTMPS端口" json:"rtmp_ssl_port"`
	RTSPPort     int64 `gorm:"column:rtsp_port;comment:RTSP端口" json:"rtsp_port"`
	RTSPSSLPort  int64 `gorm:"column:rtsp_ssl_port;comment:RTSPS端口" json:"rtsp_ssl_port"`
	FLVPort      int64 `gorm:"column:flv_port;comment:HTTP-FLV端口" json:"flv_port"`
	FLVSSLPort   int64 `gorm:"column:flv_ssl_port;comment:HTTPS-FLV端口" json:"flv_ssl_port"`
	WSFLVPort    int64 `gorm:"column:ws_flv_port;comment:WS-FLV端口" json:"ws_flv_port"`
	WSFLVSSLPort int64 `gorm:"column:ws_flv_ssl_port;comment:WSS-FLV端口" json:"ws_flv_ssl_port"`
	MP4Port      int64 `gorm:"column:mp4_port;comment:MP4端口" json:"mp4_port"`
	MP4SSLPort   int64 `gorm:"column:mp4_ssl_port;comment:MP4S端口" json:"mp4_ssl_port"`
	// 收流
	Secret           string `gorm:"column:secret;size:100;comment:ZLM API密钥(敏感)" json:"secret"`
	RTPEnable        *bool  `gorm:"column:rtp_enable;default:true;comment:是否启用多端口RTP收流模式" json:"rtp_enable"`
	RTPPortRange     string `gorm:"column:rtp_port_range;size:50;default:30000-30500;comment:RTP收流端口段" json:"rtp_port_range"`
	RTPProxyPort     int64  `gorm:"column:rtp_proxy_port;comment:RTP代理端口" json:"rtp_proxy_port"`
	JTTProxyPort     int64  `gorm:"column:jtt_proxy_port;comment:JT1078代理端口" json:"jtt_proxy_port"`
	SendRTPPortRange string `gorm:"column:send_rtp_port_range;size:50;comment:语音回推端口段" json:"send_rtp_port_range"`
	// 云端录像的合并/切片/打包下载由独立的 Assist 服务承担（默认端口 18081）。
	// 0 表示未启用，此时云端录像只支持单文件直接下载（见设计文档 §14.5）。
	RecordAssistPort int64 `gorm:"column:record_assist_port;default:0;comment:Assist服务端口,0未启用" json:"record_assist_port"`
	// 录像
	RecordPath      string `gorm:"column:record_path;size:500;comment:录像存储路径" json:"record_path"`
	RecordDay       int64  `gorm:"column:record_day;default:7;comment:录像保留天数" json:"record_day"`
	TranscodeSuffix string `gorm:"column:transcode_suffix;size:255;comment:转码后缀" json:"transcode_suffix"`
	// 健康
	Online            *bool      `gorm:"column:online;default:false;comment:节点在线状态" json:"online"`
	LastKeepaliveDT   *time.Time `gorm:"column:last_keepalive_dt;comment:最近一次心跳时间" json:"last_keepalive_dt"`
	HookAliveInterval int64      `gorm:"column:hook_alive_interval;default:15;comment:心跳间隔(秒)" json:"hook_alive_interval"`
	DefaultServer     *bool      `gorm:"column:default_server;default:false;comment:是否默认节点" json:"default_server"`
	AutoConfig        *bool      `gorm:"column:auto_config;default:false;comment:是否自动配置" json:"auto_config"`
	Remark            string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbMediaServer) TableName() string { return "ac_gb_media_server" }

// GbStreamProxy 拉流代理。表 ac_gb_stream_proxy。
// 平台主动从外部（rtsp/rtmp/http）拉流交给 ZLM。app+stream 唯一。
type GbStreamProxy struct {
	BaseModel
	Active *bool  `gorm:"column:active;default:true" json:"active"`
	Name   string `gorm:"column:name;size:255;not null;comment:名称" json:"name"`
	Type   string `gorm:"column:type;size:20;default:default;comment:代理类型" json:"type"`
	App    string `gorm:"column:app;size:50;not null;comment:ZLM app 名" json:"app"`
	Stream string `gorm:"column:stream;size:100;not null;comment:ZLM stream 名(app+stream唯一)" json:"stream"`
	SrcURL string `gorm:"column:src_url;size:500;not null;comment:源地址(rtsp/rtmp/http)" json:"src_url"`

	TimeoutMS               int64  `gorm:"column:timeout_ms;default:10000;comment:拉流超时(毫秒)" json:"timeout_ms"`
	FFmpegCmdKey            string `gorm:"column:ffmpeg_cmd_key;size:50;comment:ffmpeg命令模板键" json:"ffmpeg_cmd_key"`
	RTSPType                int64  `gorm:"column:rtsp_type;default:0;comment:RTSP拉流方式 0TCP/1UDP" json:"rtsp_type"`
	MediaServerID           string `gorm:"column:media_server_id;size:50;comment:媒体节点" json:"media_server_id"`
	EnableAudio             *bool  `gorm:"column:enable_audio;default:true;comment:是否保留音频" json:"enable_audio"`
	EnableMP4               *bool  `gorm:"column:enable_mp4;default:false;comment:是否录制MP4" json:"enable_mp4"`
	Enable                  *bool  `gorm:"column:enable;default:true;comment:是否启用" json:"enable"`
	EnableDisableNoneReader *bool  `gorm:"column:enable_disable_none_reader;default:false;comment:无人观看时禁用" json:"enable_disable_none_reader"`
	EnableRemoveNoneReader  *bool  `gorm:"column:enable_remove_none_reader;default:false;comment:无人观看时移除" json:"enable_remove_none_reader"`
	Pulling                 *bool  `gorm:"column:pulling;default:false;comment:是否正在拉流" json:"pulling"`
	StreamKey               string `gorm:"column:stream_key;size:100;comment:流KEY" json:"stream_key"`
	Remark                  string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbStreamProxy) TableName() string { return "ac_gb_stream_proxy" }

// GbStreamPush 推流。表 ac_gb_stream_push。
// 把国标流推送到 ZLM 之外的平台。app+stream 唯一。
type GbStreamPush struct {
	BaseModel
	Active *bool  `gorm:"column:active;default:true" json:"active"`
	Name   string `gorm:"column:name;size:255;not null;comment:名称" json:"name"`
	App    string `gorm:"column:app;size:50;not null;comment:ZLM app 名" json:"app"`
	Stream string `gorm:"column:stream;size:100;not null;comment:ZLM stream 名" json:"stream"`

	MediaServerID    string     `gorm:"column:media_server_id;size:50;comment:媒体节点" json:"media_server_id"`
	PushTime         *time.Time `gorm:"column:push_time;comment:推流时间" json:"push_time"`
	Status           string     `gorm:"column:status;size:20;default:OFF;comment:推流状态 ON/OFF" json:"status"`
	Pushing          *bool      `gorm:"column:pushing;default:false;comment:是否正在推流" json:"pushing"`
	SelfPush         *bool      `gorm:"column:self_push;default:false;comment:是否本平台自身推流" json:"self_push"`
	StartOfflinePush *bool      `gorm:"column:start_offline_push;default:false;comment:离线自动推流" json:"start_offline_push"`
	Remark           string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbStreamPush) TableName() string { return "ac_gb_stream_push" }

// ════════════════════════════════════════════════════════════════
// 第 3 节：录像
// ════════════════════════════════════════════════════════════════

// GbCloudRecord 云端录像。表 ac_gb_cloud_record。
// 是 ZLM 落盘 MP4 的索引，不做文件内容管理。无 active 列。
type GbCloudRecord struct {
	BaseModel
	App       string `gorm:"column:app;size:50;comment:ZLM app 名" json:"app"`
	Stream    string `gorm:"column:stream;size:100;comment:ZLM stream 名" json:"stream"`
	CallID    string `gorm:"column:call_id;size:255;comment:会话 Call-ID" json:"call_id"`
	DeviceID  *int64 `gorm:"column:device_id;comment:归属设备" json:"device_id"`
	ChannelID *int64 `gorm:"column:channel_id;comment:归属通道" json:"channel_id"`

	StartTime     *time.Time `gorm:"column:start_time;index;comment:录像开始时间" json:"start_time"`
	EndTime       *time.Time `gorm:"column:end_time;comment:录像结束时间" json:"end_time"`
	MediaServerID string     `gorm:"column:media_server_id;size:50;comment:媒体节点" json:"media_server_id"`
	FileName      string     `gorm:"column:file_name;size:255;comment:文件名" json:"file_name"`
	Folder        string     `gorm:"column:folder;size:500;comment:目录" json:"folder"`
	FilePath      string     `gorm:"column:file_path;size:1000;comment:文件路径" json:"file_path"`
	FileSize      int64      `gorm:"column:file_size;comment:文件大小(字节)" json:"file_size"`
	TimeLen       int64      `gorm:"column:time_len;comment:时长(秒)" json:"time_len"`
	Collect       *bool      `gorm:"column:collect;default:false;comment:是否已收藏(收藏的不自动清理)" json:"collect"`
	Remark        string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	// 关联对象（列表展示名称用）
	Device  *Device    `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Channel *GbChannel `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
}

func (GbCloudRecord) TableName() string { return "ac_gb_cloud_record" }

// GbRecordPlan 录像计划（周计划）。表 ac_gb_record_plan。
type GbRecordPlan struct {
	BaseModel
	Active *bool  `gorm:"column:active;default:true" json:"active"`
	Name   string `gorm:"column:name;size:255;not null;comment:计划名称" json:"name"`
	Snap   *bool  `gorm:"column:snap;default:false;comment:计划期间是否同时抓图" json:"snap"`
	Remark string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbRecordPlan) TableName() string { return "ac_gb_record_plan" }

// GbRecordPlanItem 录像计划明细。表 ac_gb_record_plan_item。
// 一个计划在一周内的若干时间段。start_time/end_time 为 HH:mm:ss 字符串
// （有意区别于 WVP 的分钟整数，见设计文档 §11+.3）。
type GbRecordPlanItem struct {
	BaseModel
	PlanID    int64  `gorm:"column:plan_id;not null;index;comment:所属计划" json:"plan_id"`
	WeekDay   int64  `gorm:"column:week_day;not null;comment:星期 0周日~6周六" json:"week_day"`
	StartTime string `gorm:"column:start_time;size:20;not null;comment:开始时间 HH:mm:ss" json:"start_time"`
	EndTime   string `gorm:"column:end_time;size:20;not null;comment:结束时间 HH:mm:ss" json:"end_time"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	Plan *GbRecordPlan `gorm:"foreignKey:PlanID;references:ID" json:"plan,omitempty"`
}

func (GbRecordPlanItem) TableName() string { return "ac_gb_record_plan_item" }

// ════════════════════════════════════════════════════════════════
// 第 4 节：报警、位置、订阅
// ════════════════════════════════════════════════════════════════

// GbAlarm 国标报警。表 ac_gb_alarm。
// 设备主动上报（MANSCDP Alarm），与平台侧自研的 AlarmRecord 区分。无 active 列。
type GbAlarm struct {
	BaseModel
	DeviceID  *int64 `gorm:"column:device_id;comment:归属设备" json:"device_id"`
	ChannelID *int64 `gorm:"column:channel_id;index;comment:归属通道" json:"channel_id"`

	GbDeviceID  string `gorm:"column:gb_device_id;size:20;comment:设备国标编码(冗余)" json:"gb_device_id"`
	GbChannelID string `gorm:"column:gb_channel_id;size:20;comment:通道国标编码(冗余)" json:"gb_channel_id"`

	AlarmPriority    string     `gorm:"column:alarm_priority;size:20;comment:报警级别 1一级(特急)/2二级(紧急)/3三级(一般)/4四级(其他)" json:"alarm_priority"`
	AlarmMethod      int64      `gorm:"column:alarm_method;comment:报警方式 1电话/2短信/3网络/4设备" json:"alarm_method"`
	AlarmType        string     `gorm:"column:alarm_type;size:50;comment:报警类型编码" json:"alarm_type"`
	AlarmTime        *time.Time `gorm:"column:alarm_time;index;comment:报警时间" json:"alarm_time"`
	AlarmDescription string     `gorm:"column:alarm_description;size:500;comment:报警描述" json:"alarm_description"`
	Longitude        float64    `gorm:"column:longitude;comment:经度" json:"longitude"`
	Latitude         float64    `gorm:"column:latitude;comment:纬度" json:"latitude"`
	SnapPath         string     `gorm:"column:snap_path;size:500;comment:关联抓图" json:"snap_path"`
	RecordPath       string     `gorm:"column:record_path;size:500;comment:关联录像" json:"record_path"`
	Handled          *bool      `gorm:"column:handled;default:false;comment:是否已处理" json:"handled"`
	Remark           string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	Device  *Device    `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Channel *GbChannel `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
}

func (GbAlarm) TableName() string { return "ac_gb_alarm" }

// GbMobilePosition 移动位置。表 ac_gb_mobile_position。
// 移动设备/车载上报，支撑 GIS 轨迹。无 active 列。
type GbMobilePosition struct {
	BaseModel
	DeviceID    *int64 `gorm:"column:device_id;comment:归属设备" json:"device_id"`
	ChannelID   *int64 `gorm:"column:channel_id;index;comment:归属通道" json:"channel_id"`
	GbChannelID string `gorm:"column:gb_channel_id;size:20;comment:通道国标编码(冗余)" json:"gb_channel_id"`

	ReportTime *time.Time `gorm:"column:report_time;index;comment:上报时间" json:"report_time"`
	Longitude  float64    `gorm:"column:longitude;comment:经度" json:"longitude"`
	Latitude   float64    `gorm:"column:latitude;comment:纬度" json:"latitude"`
	Altitude   float64    `gorm:"column:altitude;comment:海拔(米)" json:"altitude"`
	Speed      float64    `gorm:"column:speed;comment:速度(km/h)" json:"speed"`
	Direction  float64    `gorm:"column:direction;comment:方向(度)" json:"direction"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	Device  *Device    `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Channel *GbChannel `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
}

func (GbMobilePosition) TableName() string { return "ac_gb_mobile_position" }

// GbSubscribe 国标订阅记录。表 ac_gb_subscribe。
// 目录/报警/位置/PTZ 位置订阅（RFC 3265），到期需续订。无 active 列。
type GbSubscribe struct {
	BaseModel
	DeviceID   *int64 `gorm:"column:device_id;comment:归属设备" json:"device_id"`
	GbDeviceID string `gorm:"column:gb_device_id;size:20;index;comment:设备国标编码" json:"gb_device_id"`

	Type        string     `gorm:"column:type;size:20;not null;comment:订阅类型 catalog/alarm/mobile_position/ptz_position" json:"type"`
	CallID      string     `gorm:"column:call_id;size:255;comment:订阅会话 Call-ID" json:"call_id"`
	Expires     int64      `gorm:"column:expires;comment:订阅有效期(秒)" json:"expires"`
	SubscribeDT *time.Time `gorm:"column:subscribe_dt;comment:订阅时间" json:"subscribe_dt"`
	ExpireDT    *time.Time `gorm:"column:expire_dt;index;comment:过期时间(到期需续订)" json:"expire_dt"`
	Status      string     `gorm:"column:status;size:20;default:active;comment:状态 active/expired/failed" json:"status"`
	Remark      string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbSubscribe) TableName() string { return "ac_gb_subscribe" }

// GbSipLog 国标信令日志。表 ac_gb_sip_log。
//
// ⚠️ 唯一不嵌入 BaseModel 的表：追加写日志，无 deleted_at / dept_id / 审计更新字段。
// 因此**无法做行级数据权限过滤** —— 接口层必须单独控制可见性（见设计文档 §10.1）。
// 写入量最大，需按时间分区或定期归档。
type GbSipLog struct {
	ID        int64     `gorm:"column:id;primarykey" json:"id"`
	CreatedDT time.Time `gorm:"column:created_dt;default:CURRENT_TIMESTAMP;index;comment:创建时间" json:"created_dt"`

	GbDeviceID  string `gorm:"column:gb_device_id;size:20;index;comment:设备国标编码" json:"gb_device_id"`
	GbChannelID string `gorm:"column:gb_channel_id;size:20;comment:通道国标编码" json:"gb_channel_id"`

	Direction  string `gorm:"column:direction;size:10;not null;comment:方向 in接收/out发送" json:"direction"`
	Method     string `gorm:"column:method;size:20;comment:SIP方法 REGISTER/MESSAGE/INVITE/ACK/BYE/INFO" json:"method"`
	CmdType    string `gorm:"column:cmd_type;size:30;comment:MANSCDP命令类型 Catalog/Keepalive/Alarm 等" json:"cmd_type"`
	StatusCode int64  `gorm:"column:status_code;comment:SIP响应码 200/401/403 等" json:"status_code"`
	RemoteAddr string `gorm:"column:remote_addr;size:100;comment:远端地址(设备)" json:"remote_addr"`
	LocalAddr  string `gorm:"column:local_addr;size:100;comment:本地地址" json:"local_addr"`
	CallID     string `gorm:"column:call_id;size:255;comment:Call-ID" json:"call_id"`
	Content    string `gorm:"column:content;type:text;comment:报文内容(截断存储)" json:"content"`
	Result     string `gorm:"column:result;size:20;index;comment:处理结果 success/failed" json:"result"`
	Reason     string `gorm:"column:reason;size:500;comment:拒绝原因等说明" json:"reason"`
	ElapsedMS  int64  `gorm:"column:elapsed_ms;comment:处理耗时(毫秒)" json:"elapsed_ms"`
}

func (GbSipLog) TableName() string { return "ac_gb_sip_log" }

// ════════════════════════════════════════════════════════════════
// 第 5 节：语音（广播 / 对讲 / provider 字典）
// ════════════════════════════════════════════════════════════════

// VoiceSession 语音广播/对讲/语音 Agent 会话。表 ac_voice_session。无 active 列。
type VoiceSession struct {
	BaseModel
	SessionID string `gorm:"column:session_id;size:100;not null;uniqueIndex:idx_ac_voice_session_session_id;comment:会话唯一标识" json:"session_id"`
	Type      string `gorm:"column:type;size:20;not null;comment:会话类型 broadcast广播/talk对讲/agent语音Agent" json:"type"`

	DeviceID    *int64 `gorm:"column:device_id;comment:归属设备" json:"device_id"`
	ChannelID   *int64 `gorm:"column:channel_id;index;comment:归属通道" json:"channel_id"`
	GbChannelID string `gorm:"column:gb_channel_id;size:20;comment:通道国标编码" json:"gb_channel_id"`
	UserID      *int64 `gorm:"column:user_id;comment:发起用户(广播/对讲)" json:"user_id"`

	MediaServerID string     `gorm:"column:media_server_id;size:50;comment:媒体节点" json:"media_server_id"`
	App           string     `gorm:"column:app;size:50;comment:ZLM app 名" json:"app"`
	Stream        string     `gorm:"column:stream;size:100;comment:ZLM stream 名" json:"stream"`
	SSRC          string     `gorm:"column:ssrc;size:20;comment:回推音频的SSRC" json:"ssrc"`
	AudioCodec    string     `gorm:"column:audio_codec;size:20;comment:音频编码 G.711A/PCM/Opus" json:"audio_codec"`
	AgentEnabled  *bool      `gorm:"column:agent_enabled;default:false;comment:是否接入语音Agent(Pipecat)" json:"agent_enabled"`
	StartDT       *time.Time `gorm:"column:start_dt;comment:开始时间" json:"start_dt"`
	EndDT         *time.Time `gorm:"column:end_dt;comment:结束时间" json:"end_dt"`
	// 状态取值：active / stopped / failed。
	// 注意是 **stopped** 不是 ended —— 以 voice_service.go 的实际写入为准，
	// 早期注释写的 ended 从未被使用。
	Status    string `gorm:"column:status;size:20;default:active;index;comment:状态 active/stopped/failed" json:"status"`
	EndReason string `gorm:"column:end_reason;size:500;comment:结束原因" json:"end_reason"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	Device  *Device    `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Channel *GbChannel `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
}

func (VoiceSession) TableName() string { return "ac_voice_session" }

// VoiceProvider 语音 provider 字典。表 ac_voice_provider。
//
// 只存「有哪些 provider 可选、各自需要哪些参数」，驱动前端动态表单（param_schema）。
// STT/LLM/TTS 的具体参数值存 ac_thirdparty_config（Category='voice_agent'），不在此表。
type VoiceProvider struct {
	BaseModel
	Active   *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Category string `gorm:"column:category;size:20;not null;uniqueIndex:idx_ac_voice_provider_category_code;comment:类别 stt/llm/tts/vad" json:"category"`
	Code     string `gorm:"column:code;size:50;not null;uniqueIndex:idx_ac_voice_provider_category_code;comment:provider编码 local_whisper/openai/deepgram/anthropic/edge_tts 等" json:"code"`
	Name     string `gorm:"column:name;size:255;not null;comment:展示名" json:"name"`

	ParamSchema   string `gorm:"column:param_schema;type:text;comment:JSON:该 provider 需要哪些参数,驱动前端动态表单" json:"param_schema"`
	DefaultParams string `gorm:"column:default_params;type:text;comment:JSON:默认参数值" json:"default_params"`
	Remark        string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (VoiceProvider) TableName() string { return "ac_voice_provider" }

// ════════════════════════════════════════════════════════════════
// 第 6 节：国标级联（上级平台）
// ════════════════════════════════════════════════════════════════

// GbPlatform 上级平台（本平台作为下级注册到上级）。表 ac_gb_platform。
type GbPlatform struct {
	BaseModel
	Active *bool `gorm:"column:active;default:true" json:"active"`
	Enable *bool `gorm:"column:enable;default:true;comment:是否启用" json:"enable"`

	Name           string `gorm:"column:name;size:255;not null;comment:上级平台名称" json:"name"`
	ServerGBID     string `gorm:"column:server_gb_id;size:20;not null;uniqueIndex:idx_ac_gb_platform_server_gb_id;comment:上级平台国标编码,唯一" json:"server_gb_id"`
	ServerGBDomain string `gorm:"column:server_gb_domain;size:20;comment:上级平台域" json:"server_gb_domain"`
	ServerIP       string `gorm:"column:server_ip;size:50;comment:上级平台IP" json:"server_ip"`
	ServerPort     int64  `gorm:"column:server_port;comment:上级平台端口" json:"server_port"`

	DeviceGBID   string `gorm:"column:device_gb_id;size:20;comment:本平台在该上级的国标编码" json:"device_gb_id"`
	DeviceIP     string `gorm:"column:device_ip;size:50;comment:本平台IP" json:"device_ip"`
	DevicePort   int64  `gorm:"column:device_port;comment:本平台端口" json:"device_port"`
	Username     string `gorm:"column:username;size:100;comment:注册用户名" json:"username"`
	Password     string `gorm:"column:password;size:100;comment:注册密码(敏感)" json:"password"`
	Expires      int64  `gorm:"column:expires;comment:注册有效期(秒)" json:"expires"`
	KeepTimeout  int64  `gorm:"column:keep_timeout;comment:心跳超时(秒)" json:"keep_timeout"`
	Transport    string `gorm:"column:transport;size:10;default:UDP;comment:SIP传输 UDP/TCP" json:"transport"`
	CivilCode    string `gorm:"column:civil_code;size:20;comment:行政区划代码" json:"civil_code"`
	Status       string `gorm:"column:status;size:20;default:OFF;index;comment:在线状态 ON/OFF" json:"status"`
	CatalogGroup int64  `gorm:"column:catalog_group;default:0;comment:目录分组" json:"catalog_group"`

	RegisterWay      string `gorm:"column:register_way;size:20;default:local;comment:注册方式 local本地/remote远程" json:"register_way"`
	AsMessageChannel *bool  `gorm:"column:as_message_channel;default:false;comment:是否作为消息通道" json:"as_message_channel"`
	// 上级平台的 XML 编码。结论六（GB2312 转码）在级联出/入向同样适用，
	// 不存该值则与 UTF-8 上级平台级联必然乱码。默认 GB2312（对齐 WVP PlatformController:118）。
	CharacterSet        string `gorm:"column:character_set;size:50;default:GB2312;comment:XML编码 GB2312/UTF-8" json:"character_set"`
	PTZ                 *bool  `gorm:"column:ptz;default:true;comment:是否支持云台" json:"ptz"`
	RTCP                *bool  `gorm:"column:rtcp;default:false;comment:是否使用RTCP" json:"rtcp"`
	ServerID            string `gorm:"column:server_id;size:50;comment:媒体节点" json:"server_id"`
	Manufacturer        string `gorm:"column:manufacturer;size:100;comment:厂商" json:"manufacturer"`
	Model               string `gorm:"column:model;size:100;comment:型号" json:"model"`
	Address             string `gorm:"column:address;size:255;comment:安装地址" json:"address"`
	Secrecy             int64  `gorm:"column:secrecy;default:0;comment:保密属性" json:"secrecy"`
	CatalogWithPlatform *bool  `gorm:"column:catalog_with_platform;default:true;comment:目录是否含平台" json:"catalog_with_platform"`
	CatalogWithGroup    int64  `gorm:"column:catalog_with_group;default:0;comment:目录是否含分组" json:"catalog_with_group"`
	CatalogWithRegion   int64  `gorm:"column:catalog_with_region;default:0;comment:目录是否含区域" json:"catalog_with_region"`
	AutoPushChannel     *bool  `gorm:"column:auto_push_channel;default:false;comment:是否自动推送通道" json:"auto_push_channel"`
	SendStreamIP        string `gorm:"column:send_stream_ip;size:50;comment:推流使用的IP" json:"send_stream_ip"`
	Remark              string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (GbPlatform) TableName() string { return "ac_gb_platform" }

// GbPlatformChannel 级联通道。表 ac_gb_platform_channel。
// 本平台推送给上级的通道及其在该上级的定制信息。无 active 列。
type GbPlatformChannel struct {
	BaseModel
	PlatformID int64 `gorm:"column:platform_id;not null;index;comment:上级平台ID" json:"platform_id"`
	ChannelID  int64 `gorm:"column:channel_id;not null;index;comment:本地通道ID" json:"channel_id"`

	CustomDeviceID     string `gorm:"column:custom_device_id;size:20;not null;uniqueIndex:idx_ac_gb_platform_channel_custom_id;comment:在该上级使用的国标编码,唯一" json:"custom_device_id"`
	CustomName         string `gorm:"column:custom_name;size:255;comment:定制名称(覆盖本地名称)" json:"custom_name"`
	CustomManufacturer string `gorm:"column:custom_manufacturer;size:100;comment:定制厂商" json:"custom_manufacturer"`
	CustomModel        string `gorm:"column:custom_model;size:100;comment:定制型号" json:"custom_model"`
	CustomOwner        string `gorm:"column:custom_owner;size:100;comment:定制归属单位" json:"custom_owner"`
	CustomCivilCode    string `gorm:"column:custom_civil_code;size:20;comment:定制行政区划" json:"custom_civil_code"`
	CustomAddress      string `gorm:"column:custom_address;size:255;comment:定制地址" json:"custom_address"`
	CustomParental     int64  `gorm:"column:custom_parental;default:0;comment:定制是否有子设备" json:"custom_parental"`
	CustomParentID     string `gorm:"column:custom_parent_id;size:20;comment:定制父通道编码" json:"custom_parent_id"`
	CustomStatus       string `gorm:"column:custom_status;size:20;default:OFF;comment:定制状态 ON/OFF" json:"custom_status"`
	CustomPTZType      int64  `gorm:"column:custom_ptz_type;default:0;comment:定制云台类型" json:"custom_ptz_type"`
	CustomChannelType  int64  `gorm:"column:custom_channel_type;default:0;comment:定制通道类型" json:"custom_channel_type"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	Platform *GbPlatform `gorm:"foreignKey:PlatformID;references:ID" json:"platform,omitempty"`
	Channel  *GbChannel  `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
}

func (GbPlatformChannel) TableName() string { return "ac_gb_platform_channel" }

// GbPlatformGroup 级联平台↔业务分组。表 ac_gb_platform_group。
// 决定「按分组推送目录」的可见范围。纯关联表，无 dept_id（随主表 ac_gb_platform 过滤）。
type GbPlatformGroup struct {
	BaseModel
	PlatformID int64 `gorm:"column:platform_id;not null;uniqueIndex:uk_ac_gb_platform_group_platform_group,priority:1;comment:上级平台ID" json:"platform_id"`
	GroupID    int64 `gorm:"column:group_id;not null;uniqueIndex:uk_ac_gb_platform_group_platform_group,priority:2;comment:业务分组ID" json:"group_id"`

	Platform *GbPlatform `gorm:"foreignKey:PlatformID;references:ID" json:"platform,omitempty"`
	Group    *GbGroup    `gorm:"foreignKey:GroupID;references:ID" json:"group,omitempty"`
}

func (GbPlatformGroup) TableName() string { return "ac_gb_platform_group" }

// GbPlatformRegion 级联平台↔区域。表 ac_gb_platform_region。
// 决定「按区域推送目录」的可见范围。纯关联表，无 dept_id（随主表 ac_gb_platform 过滤）。
type GbPlatformRegion struct {
	BaseModel
	PlatformID int64 `gorm:"column:platform_id;not null;uniqueIndex:uk_ac_gb_platform_region_platform_region,priority:1;comment:上级平台ID" json:"platform_id"`
	RegionID   int64 `gorm:"column:region_id;not null;uniqueIndex:uk_ac_gb_platform_region_platform_region,priority:2;comment:区域ID" json:"region_id"`

	Platform *GbPlatform `gorm:"foreignKey:PlatformID;references:ID" json:"platform,omitempty"`
	Region   *Region     `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
}

func (GbPlatformRegion) TableName() string { return "ac_gb_platform_region" }
