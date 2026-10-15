package model

import "time"

// ════════════════════════════════════════════════════════════════
// 资源管理：设备 / 设备模式 / 物联网卡 / 物联网卡更换记录 / 语音配置
// ════════════════════════════════════════════════════════════════

// 设备运行状态常量。
const (
	DeviceStatusOnline  = "online"
	DeviceStatusOffline = "offline"
)

// Device AI 摄像头设备。表 ac_device。
//
// 运行时指标（电量/电压/存储/在线时间）由设备心跳上报，写入本表作为最新快照，
// 同时写入 DeviceHistory 作为时序历史。
type Device struct {
	BaseModel
	Active     *bool  `gorm:"column:active;default:true" json:"active"`
	Name       string `gorm:"column:name;size:255;not null;comment:设备名称" json:"name"`
	DeviceCode string `gorm:"column:device_code;size:100;not null;uniqueIndex:idx_ac_device_code;comment:设备编号" json:"device_code"`
	RegionID   *int64 `gorm:"column:region_id;index;comment:所属区域" json:"region_id"`
	Location   string `gorm:"column:location;size:255;comment:设备位置" json:"location"`
	Latitude   string `gorm:"column:latitude;size:32;comment:纬度(GIS)" json:"latitude"`
	Longitude  string `gorm:"column:longitude;size:32;comment:经度(GIS)" json:"longitude"`

	// 版本与模式
	FirmwareID   *int64 `gorm:"column:firmware_id;comment:当前固件ID" json:"firmware_id"`
	FirmwareVer  string `gorm:"column:firmware_ver;size:50;comment:当前固件版本(冗余便于列表展示)" json:"firmware_ver"`
	ModelID      *int64 `gorm:"column:model_id;comment:当前AI模型ID" json:"model_id"`
	ModelVer     string `gorm:"column:model_ver;size:50;comment:当前模型版本" json:"model_ver"`
	SystemVer    string `gorm:"column:system_ver;size:50;comment:系统版本" json:"system_ver"`
	WorkMode     string `gorm:"column:work_mode;size:50;comment:设备工作模式" json:"work_mode"`
	DeviceMode   string `gorm:"column:device_mode;size:50;comment:设备模式(充电/全开/省电)" json:"device_mode"`
	InfoInterval int    `gorm:"column:info_interval;default:60;comment:信息上传间隔(秒)" json:"info_interval"`

	// 物联网卡
	IotCardID *int64 `gorm:"column:iot_card_id;comment:物联网卡ID" json:"iot_card_id"`

	// ── 国标接入（GB28181，v0.2.0 新增 13 列）──────────────────────
	//
	// ⚠️ gb_device_id 与 device_code 是**两个不同的 ID 命名空间**：
	//   device_code 为业务编号（如 CAM_001），供 /web/devices/:id/heartbeat 使用；
	//   gb_device_id 为 20 位国标编码，是 SIP From 头携带的值。
	// 二者不得互相复用。gb_device_id 为空的设备不参与 SIP（信令层回 403）。
	// 唯一约束为**部分唯一索引**（WHERE gb_device_id IS NOT NULL AND deleted_at IS NULL），
	// 无法用 GORM tag 表达，由 DDL 脚本创建；AutoMigrate 只负责列本身。
	// 依据：docs/开发文档/数据库设计/sql/ddl/09-国标视频.sql 第 0 节。
	GbDeviceID     string     `gorm:"column:gb_device_id;size:20;comment:国标20位编码,SIP From头携带" json:"gb_device_id"`
	SipPassword    string     `gorm:"column:sip_password;size:100;comment:设备SIP Digest密码(敏感,接口返回时脱敏)" json:"sip_password"`
	SipRealm       string     `gorm:"column:sip_realm;size:50;comment:SIP鉴权域,默认取gb28181.sip_domain" json:"sip_realm"`
	SipTransport   string     `gorm:"column:sip_transport;size:10;default:UDP;comment:SIP传输 UDP/TCP" json:"sip_transport"`
	StreamMode     string     `gorm:"column:stream_mode;size:20;default:UDP;comment:媒体传输 UDP/TCP-PASSIVE/TCP-ACTIVE" json:"stream_mode"`
	Charset        string     `gorm:"column:charset;size:20;default:GB2312;comment:XML报文编码 GB2312/UTF-8" json:"charset"`
	RegisterStatus string     `gorm:"column:register_status;size:20;default:pending;index;comment:注册状态 pending/activated/rejected" json:"register_status"`
	RegisterDT     *time.Time `gorm:"column:register_dt;comment:首次注册成功时间" json:"register_dt"`
	SipExpires     *time.Time `gorm:"column:sip_expires;index;comment:SIP注册过期时间" json:"sip_expires"`
	MediaServerID  string     `gorm:"column:media_server_id;size:50;default:auto;comment:绑定媒体节点,auto为自动选择" json:"media_server_id"`
	ChannelCount   int64      `gorm:"column:channel_count;default:0;comment:通道数(冗余,列表展示用)" json:"channel_count"`
	LocalIP        string     `gorm:"column:local_ip;size:50;comment:设备级源IP覆盖(多网卡场景)" json:"local_ip"`
	SDPIP          string     `gorm:"column:sdp_ip;size:50;comment:设备级SDP IP覆盖(NAT场景)" json:"sdp_ip"`

	// 运行时状态（设备心跳/平台探测更新）
	Status       string     `gorm:"column:status;size:20;default:offline;comment:运行状态 online/offline" json:"status"`
	Battery      int        `gorm:"column:battery;comment:电量百分比" json:"battery"`
	Voltage      string     `gorm:"column:voltage;size:32;comment:电压" json:"voltage"`
	StorageTotal int64      `gorm:"column:storage_total;comment:存储总容量(MB)" json:"storage_total"`
	StorageFree  int64      `gorm:"column:storage_free;comment:剩余存储容量(MB)" json:"storage_free"`
	OnlineDT     *time.Time `gorm:"column:online_dt;comment:最近上线时间" json:"online_dt"`
	OfflineDT    *time.Time `gorm:"column:offline_dt;comment:最近离线时间" json:"offline_dt"`
	LastBeatDT   *time.Time `gorm:"column:last_beat_dt;comment:最近心跳时间" json:"last_beat_dt"`

	// 实时监控
	MonitorEnabled *bool  `gorm:"column:monitor_enabled;default:false;comment:是否加入实时监控" json:"monitor_enabled"`
	StreamURL      string `gorm:"column:stream_url;size:500;comment:实时视频流地址" json:"stream_url"`
	Remark         string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	// 关联对象（列表展示名称用）
	Region   *Region   `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
	IotCard  *IotCard  `gorm:"foreignKey:IotCardID;references:ID" json:"iot_card,omitempty"`
	Firmware *Firmware `gorm:"foreignKey:FirmwareID;references:ID" json:"firmware,omitempty"`
	AIModel  *AIModel  `gorm:"foreignKey:ModelID;references:ID" json:"ai_model,omitempty"`
}

func (Device) TableName() string { return "ac_device" }

// DeviceMode 设备工作模式。表 ac_device_mode。
// 内置充电模式、全开模式、省电模式，可扩展。
type DeviceMode struct {
	BaseModel
	Active   *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name     string `gorm:"column:name;size:255;not null;comment:模式名称" json:"name"`
	Code     string `gorm:"column:code;size:50;comment:模式编码" json:"code"`
	Remark   string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (DeviceMode) TableName() string { return "ac_device_mode" }

// 物联网卡状态常量。
const (
	IotCardStatusNormal   = "normal"
	IotCardStatusArrears  = "arrears"  // 欠费
	IotCardStatusExpired  = "expired"  // 到期
	IotCardStatusDisabled = "disabled" // 停用
)

// IotCard 物联网卡。表 ac_iot_card。
type IotCard struct {
	BaseModel
	Active       *bool      `gorm:"column:active;default:true" json:"active"`
	Name         string     `gorm:"column:name;size:255;not null;comment:物联网卡名称" json:"name"`
	CardNo       string     `gorm:"column:card_no;size:100;not null;uniqueIndex:idx_ac_iot_card_no;comment:卡号" json:"card_no"`
	RegionID     *int64     `gorm:"column:region_id;index;comment:所属区域" json:"region_id"`
	DeviceID     *int64     `gorm:"column:device_id;index;comment:所属设备" json:"device_id"`
	TrafficTotal int64      `gorm:"column:traffic_total;comment:流量总额(MB)" json:"traffic_total"`
	TrafficUsed  int64      `gorm:"column:traffic_used;comment:已使用流量(MB)" json:"traffic_used"`
	TrafficLeft  int64      `gorm:"column:traffic_left;comment:剩余流量(MB)" json:"traffic_left"`
	StatDT       *time.Time `gorm:"column:stat_dt;comment:统计时间" json:"stat_dt"`
	ExpireDT     *time.Time `gorm:"column:expire_dt;comment:到期时间" json:"expire_dt"`
	Status       string     `gorm:"column:status;size:20;default:normal;comment:normal/arrears/expired/disabled" json:"status"`
	Remark       string     `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64  `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Region *Region `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`

	// 反向关联：ac_iot_card.device_id 与 ac_device.iot_card_id 互为引用，
	// 若两侧都交给 GORM 建外键会形成循环依赖导致 AutoMigrate 失败。
	// 这里用 -:migration 保留 Preload 能力但不参与建表；外键约束由
	// docs/开发文档/数据库设计/sql/ddl 下的 DDL 脚本用 ALTER TABLE 补齐。
	Device *Device `gorm:"foreignKey:DeviceID;references:ID;-:migration" json:"device,omitempty"`
}

func (IotCard) TableName() string { return "ac_iot_card" }

// IotCardReplaceLog 物联网卡更换记录。表 ac_iot_card_replace_log。
// 追加写入，不做修改；用于追溯某台设备历史使用过的卡。
type IotCardReplaceLog struct {
	BaseModel
	DeviceID     int64     `gorm:"column:device_id;not null;index;comment:设备ID" json:"device_id"`
	DeviceCode   string    `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	RegionID     *int64    `gorm:"column:region_id;comment:所属区域" json:"region_id"`
	Location     string    `gorm:"column:location;size:255;comment:设备位置(冗余)" json:"location"`
	OldIotCardID *int64    `gorm:"column:old_iot_card_id;comment:原物联网卡ID" json:"old_iot_card_id"`
	NewIotCardID *int64    `gorm:"column:new_iot_card_id;comment:新物联网卡ID" json:"new_iot_card_id"`
	OldCardNo    string    `gorm:"column:old_card_no;size:100;comment:原卡号" json:"old_card_no"`
	NewCardNo    string    `gorm:"column:new_card_no;size:100;comment:新卡号" json:"new_card_no"`
	ReplaceDT    time.Time `gorm:"column:replace_dt;comment:更换时间" json:"replace_dt"`
	Operator     string    `gorm:"column:operator;size:50;comment:操作人" json:"operator"`
	Remark       string    `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64  `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Device *Device `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Region *Region `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
}

func (IotCardReplaceLog) TableName() string { return "ac_iot_card_replace_log" }

// VoiceConfig 语音配置：为风险类型配置播报音频。表 ac_voice_config。
type VoiceConfig struct {
	BaseModel
	Active     *bool  `gorm:"column:active;default:true" json:"active"`
	Name       string `gorm:"column:name;size:255;not null;comment:语音名称" json:"name"`
	VoiceType  string `gorm:"column:voice_type;size:50;comment:语音类型(关联风险类型编码)" json:"voice_type"`
	RiskTypeID *int64 `gorm:"column:risk_type_id;index;comment:风险类型ID" json:"risk_type_id"`
	RegionID   *int64 `gorm:"column:region_id;index;comment:所属区域" json:"region_id"`
	DeviceID   *int64 `gorm:"column:device_id;index;comment:应用设备" json:"device_id"`
	AudioURL   string `gorm:"column:audio_url;size:500;comment:播报音频地址" json:"audio_url"`
	AudioName  string `gorm:"column:audio_name;size:255;comment:音频文件名" json:"audio_name"`
	Remark     string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID   *int64    `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Region   *Region   `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
	Device   *Device   `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	RiskType *RiskType `gorm:"foreignKey:RiskTypeID;references:ID" json:"risk_type,omitempty"`
}

func (VoiceConfig) TableName() string { return "ac_voice_config" }

// DeviceHistory 设备运行历史（时序采样）。表 ac_device_history。
// 支撑「设备历史记录」页的近3日曲线与一周运行状态回放。
type DeviceHistory struct {
	BaseModel
	DeviceID    int64     `gorm:"column:device_id;not null;index:idx_ac_device_history_device_dt,priority:1;comment:设备ID" json:"device_id"`
	DeviceCode  string    `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	IotCardNo   string    `gorm:"column:iot_card_no;size:100;comment:流量卡号(冗余)" json:"iot_card_no"`
	RegionID    *int64    `gorm:"column:region_id;comment:所属区域" json:"region_id"`
	Location    string    `gorm:"column:location;size:255;comment:设备位置(冗余)" json:"location"`
	Status      string    `gorm:"column:status;size:20;comment:运行状态 online/offline" json:"status"`
	Battery     int       `gorm:"column:battery;comment:电量百分比" json:"battery"`
	Voltage     string    `gorm:"column:voltage;size:32;comment:电压" json:"voltage"`
	StorageFree int64     `gorm:"column:storage_free;comment:剩余存储容量(MB)" json:"storage_free"`
	SampledDT   time.Time `gorm:"column:sampled_dt;index:idx_ac_device_history_device_dt,priority:2;comment:采样时间" json:"sampled_dt"`

	DeptID *int64  `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Device *Device `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
}

func (DeviceHistory) TableName() string { return "ac_device_history" }
