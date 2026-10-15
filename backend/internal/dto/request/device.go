package request

// ════════════════════════════════════════════════════════════════
// 资源管理：设备 / 设备模式 / 物联网卡 / 换卡记录 / 语音配置
// ════════════════════════════════════════════════════════════════

// ── 设备 ──────────────────────────────────────────────────────

// PageDeviceReq 设备分页查询。
type PageDeviceReq struct {
	PageReq
	Keyword    string `form:"keyword" json:"keyword" example:"CAM"`
	RegionID   *int64 `form:"region_id" json:"region_id" example:"1"`
	Status     string `form:"status" json:"status" example:"online"`
	DeviceMode string `form:"device_mode" json:"device_mode" example:"全开模式"`
	WorkMode   string `form:"work_mode" json:"work_mode" example:"正常"`
}

// CreateDeviceReq 新增设备。
type CreateDeviceReq struct {
	Name           string `json:"name" binding:"required" example:"1号摄像头"`
	DeviceCode     string `json:"device_code" binding:"required" example:"CAM001"`
	RegionID       *int64 `json:"region_id"`
	Location       string `json:"location" example:"东门入口"`
	Latitude       string `json:"latitude" example:"30.123456"`
	Longitude      string `json:"longitude" example:"120.123456"`
	FirmwareID     *int64 `json:"firmware_id"`
	FirmwareVer    string `json:"firmware_ver" example:"1.0.0"`
	ModelID        *int64 `json:"model_id"`
	ModelVer       string `json:"model_ver" example:"1.0.0"`
	SystemVer      string `json:"system_ver" example:"1.0.0"`
	WorkMode       string `json:"work_mode" example:"正常"`
	DeviceMode     string `json:"device_mode" example:"全开模式"`
	InfoInterval   int    `json:"info_interval" example:"60"`
	IotCardID      *int64 `json:"iot_card_id"`
	StorageTotal   int64  `json:"storage_total" example:"65536"`
	MonitorEnabled *bool  `json:"monitor_enabled" example:"false"`
	StreamURL      string `json:"stream_url" example:"rtsp://192.168.1.10/live"`
	Remark         string `json:"remark"`
	Active         *bool  `json:"active" example:"true"`
}

// UpdateDeviceReq 更新设备。设备编号可修改，需保持全局唯一。
type UpdateDeviceReq struct {
	Name           string `json:"name" example:"1号摄像头"`
	DeviceCode     string `json:"device_code" example:"CAM001"`
	RegionID       *int64 `json:"region_id"`
	Location       string `json:"location"`
	Latitude       string `json:"latitude"`
	Longitude      string `json:"longitude"`
	FirmwareID     *int64 `json:"firmware_id"`
	FirmwareVer    string `json:"firmware_ver"`
	ModelID        *int64 `json:"model_id"`
	ModelVer       string `json:"model_ver"`
	SystemVer      string `json:"system_ver"`
	WorkMode       string `json:"work_mode"`
	DeviceMode     string `json:"device_mode"`
	InfoInterval   int    `json:"info_interval"`
	IotCardID      *int64 `json:"iot_card_id"`
	StorageTotal   int64  `json:"storage_total"`
	MonitorEnabled *bool  `json:"monitor_enabled"`
	StreamURL      string `json:"stream_url"`
	Remark         string `json:"remark"`
	Active         *bool  `json:"active"`
}

// SwitchModeReq 切换设备模式。mode 必须在设备模式表中存在。
type SwitchModeReq struct {
	Mode string `json:"mode" binding:"required" example:"省电模式"`
}

// ToggleMonitorReq 加入/移出实时监控。
type ToggleMonitorReq struct {
	Enabled bool `json:"enabled" example:"true"`
}

// DeviceHeartbeatReq 设备心跳上报（ApiKey 鉴权）。
type DeviceHeartbeatReq struct {
	Status      string `json:"status" example:"online"`
	Battery     int    `json:"battery" example:"88"`
	Voltage     string `json:"voltage" example:"12.4"`
	StorageFree int64  `json:"storage_free" example:"32768"`
}

// PageDeviceHistoryReq 设备历史记录分页查询。
type PageDeviceHistoryReq struct {
	PageReq
	DateRangeReq
	DeviceID   *int64 `form:"device_id" json:"device_id"`
	RegionID   *int64 `form:"region_id" json:"region_id"`
	Status     string `form:"status" json:"status" example:"online"`
	DeviceCode string `form:"device_code" json:"device_code" example:"CAM001"`
}

// ── 设备模式 ──────────────────────────────────────────────────

// PageDeviceModeReq 设备模式分页查询。
type PageDeviceModeReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"充电"`
}

// CreateDeviceModeReq 新增设备模式。
type CreateDeviceModeReq struct {
	Name     string `json:"name" binding:"required" example:"充电模式"`
	Code     string `json:"code" example:"charge"`
	Sequence int    `json:"sequence" example:"10"`
	Remark   string `json:"remark"`
	Active   *bool  `json:"active" example:"true"`
}

// UpdateDeviceModeReq 更新设备模式。
type UpdateDeviceModeReq struct {
	Name     string `json:"name" example:"充电模式"`
	Code     string `json:"code" example:"charge"`
	Sequence int    `json:"sequence" example:"10"`
	Remark   string `json:"remark"`
	Active   *bool  `json:"active"`
}

// ── 物联网卡 ──────────────────────────────────────────────────

// PageIotCardReq 物联网卡分页查询。
type PageIotCardReq struct {
	PageReq
	Keyword  string `form:"keyword" json:"keyword" example:"8986"`
	RegionID *int64 `form:"region_id" json:"region_id"`
	DeviceID *int64 `form:"device_id" json:"device_id"`
	Status   string `form:"status" json:"status" example:"normal"`
}

// CreateIotCardReq 新增物联网卡。
// status 由 expire_dt 与 traffic_left 自动推导，入参仅用于标记停用。
type CreateIotCardReq struct {
	Name         string `json:"name" binding:"required" example:"移动卡A"`
	CardNo       string `json:"card_no" binding:"required" example:"8986001234567890"`
	RegionID     *int64 `json:"region_id"`
	DeviceID     *int64 `json:"device_id"`
	TrafficTotal int64  `json:"traffic_total" example:"10240"`
	TrafficUsed  int64  `json:"traffic_used" example:"1024"`
	TrafficLeft  int64  `json:"traffic_left" example:"9216"`
	StatDT       string `json:"stat_dt" example:"2026-09-21 10:00:00"`
	ExpireDT     string `json:"expire_dt" example:"2027-09-21 00:00:00"`
	Status       string `json:"status" example:"disabled"`
	Remark       string `json:"remark"`
	Active       *bool  `json:"active" example:"true"`
}

// UpdateIotCardReq 更新物联网卡。
type UpdateIotCardReq struct {
	Name         string `json:"name" example:"移动卡A"`
	CardNo       string `json:"card_no" example:"8986001234567890"`
	RegionID     *int64 `json:"region_id"`
	DeviceID     *int64 `json:"device_id"`
	TrafficTotal int64  `json:"traffic_total"`
	TrafficUsed  int64  `json:"traffic_used"`
	TrafficLeft  int64  `json:"traffic_left"`
	StatDT       string `json:"stat_dt"`
	ExpireDT     string `json:"expire_dt"`
	Status       string `json:"status"`
	Remark       string `json:"remark"`
	Active       *bool  `json:"active"`
}

// PageIotCardReplaceLogReq 换卡记录分页查询。
type PageIotCardReplaceLogReq struct {
	PageReq
	DateRangeReq
	RegionID   *int64 `form:"region_id" json:"region_id"`
	DeviceID   *int64 `form:"device_id" json:"device_id"`
	DeviceCode string `form:"device_code" json:"device_code" example:"CAM001"`
	CardNo     string `form:"card_no" json:"card_no" example:"8986"`
}

// ReplaceIotCardReq 设备换卡。
// OldIotCardID 缺省时取设备当前绑定的卡；NewIotCardID 必填。
type ReplaceIotCardReq struct {
	DeviceID     int64  `json:"device_id" binding:"required" example:"1"`
	OldIotCardID *int64 `json:"old_iot_card_id"`
	NewIotCardID int64  `json:"new_iot_card_id" binding:"required" example:"2"`
	Remark       string `json:"remark"`
}

// ── 语音配置 ──────────────────────────────────────────────────

// PageVoiceConfigReq 语音配置分页查询。
type PageVoiceConfigReq struct {
	PageReq
	Keyword   string `form:"keyword" json:"keyword" example:"闯入"`
	VoiceType string `form:"voice_type" json:"voice_type" example:"intrusion"`
	RegionID  *int64 `form:"region_id" json:"region_id"`
	DeviceID  *int64 `form:"device_id" json:"device_id"`
}

// CreateVoiceConfigReq 新增语音配置。
type CreateVoiceConfigReq struct {
	Name       string `json:"name" binding:"required" example:"闯入告警播报"`
	VoiceType  string `json:"voice_type" example:"intrusion"`
	RiskTypeID *int64 `json:"risk_type_id"`
	RegionID   *int64 `json:"region_id"`
	DeviceID   *int64 `json:"device_id"`
	AudioURL   string `json:"audio_url" example:"/uploads/audio/intrusion.mp3"`
	AudioName  string `json:"audio_name" example:"intrusion.mp3"`
	Remark     string `json:"remark"`
	Active     *bool  `json:"active" example:"true"`
}

// UpdateVoiceConfigReq 更新语音配置。
type UpdateVoiceConfigReq struct {
	Name       string `json:"name" example:"闯入告警播报"`
	VoiceType  string `json:"voice_type" example:"intrusion"`
	RiskTypeID *int64 `json:"risk_type_id"`
	RegionID   *int64 `json:"region_id"`
	DeviceID   *int64 `json:"device_id"`
	AudioURL   string `json:"audio_url"`
	AudioName  string `json:"audio_name"`
	Remark     string `json:"remark"`
	Active     *bool  `json:"active"`
}
