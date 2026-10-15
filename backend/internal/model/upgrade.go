package model

import "time"

// ════════════════════════════════════════════════════════════════
// 升级管理：固件类型 / 固件 / AI模型 / 升级计划 / 升级记录
// ════════════════════════════════════════════════════════════════

// 升级计划状态常量。
const (
	UpgradePlanPending  = "pending"  // 待升级（定时任务已排期）
	UpgradePlanRunning  = "running"  // 升级中
	UpgradePlanDone     = "done"     // 已升级
	UpgradePlanFailed   = "failed"   // 升级失败
	UpgradePlanCanceled = "canceled" // 已取消
)

// FirmwareType 固件类型。表 ac_firmware_type。
type FirmwareType struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence    int    `gorm:"column:sequence;default:10" json:"sequence"`
	Name        string `gorm:"column:name;size:255;not null;comment:固件类型" json:"name"`
	Code        string `gorm:"column:code;size:50;comment:类型编码" json:"code"`
	Description string `gorm:"column:description;type:text;comment:描述" json:"description"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (FirmwareType) TableName() string { return "ac_firmware_type" }

// Firmware 固件。表 ac_firmware。
type Firmware struct {
	BaseModel
	Active         *bool      `gorm:"column:active;default:true" json:"active"`
	Name           string     `gorm:"column:name;size:255;not null;comment:固件名称" json:"name"`
	Version        string     `gorm:"column:version;size:50;not null;comment:版本号" json:"version"`
	FirmwareTypeID *int64     `gorm:"column:firmware_type_id;index;comment:固件类型ID" json:"firmware_type_id"`
	FileURL        string     `gorm:"column:file_url;size:500;comment:固件文件地址" json:"file_url"`
	FileSize       int64      `gorm:"column:file_size;comment:文件大小(字节)" json:"file_size"`
	Md5            string     `gorm:"column:md5;size:64;comment:MD5校验值" json:"md5"`
	ReleaseDT      *time.Time `gorm:"column:release_dt;comment:发布时间" json:"release_dt"`
	Description    string     `gorm:"column:description;type:text;comment:更新说明" json:"description"`

	DeptID       *int64        `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	FirmwareType *FirmwareType `gorm:"foreignKey:FirmwareTypeID;references:ID" json:"firmware_type,omitempty"`
}

func (Firmware) TableName() string { return "ac_firmware" }

// AIModel AI 识别模型。表 ac_ai_model。
// 模型必须绑定适用的固件（RequirementEC: 「选择适用固件中数据与固件管理中数据相关联」）。
type AIModel struct {
	BaseModel
	Active      *bool      `gorm:"column:active;default:true" json:"active"`
	Name        string     `gorm:"column:name;size:255;not null;comment:模型名称" json:"name"`
	Version     string     `gorm:"column:version;size:50;not null;comment:版本号" json:"version"`
	FirmwareID  *int64     `gorm:"column:firmware_id;index;comment:适用固件ID" json:"firmware_id"`
	FileURL     string     `gorm:"column:file_url;size:500;comment:模型文件地址" json:"file_url"`
	FileSize    int64      `gorm:"column:file_size;comment:文件大小(字节)" json:"file_size"`
	Md5         string     `gorm:"column:md5;size:64;comment:MD5校验值" json:"md5"`
	ReleaseDT   *time.Time `gorm:"column:release_dt;comment:发布时间" json:"release_dt"`
	Description string     `gorm:"column:description;type:text;comment:更新说明" json:"description"`

	DeptID   *int64    `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Firmware *Firmware `gorm:"foreignKey:FirmwareID;references:ID" json:"firmware,omitempty"`
}

func (AIModel) TableName() string { return "ac_ai_model" }

// DeviceUpgradePlan 设备定时升级计划。表 ac_device_upgrade_plan。
// 仅在用户选择「定时升级」时产生；选择「现在升级」会直接下发并写入 UpgradeRecord。
type DeviceUpgradePlan struct {
	BaseModel
	DeviceID     int64     `gorm:"column:device_id;not null;index;comment:设备ID" json:"device_id"`
	DeviceCode   string    `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	DeviceName   string    `gorm:"column:device_name;size:255;comment:设备名称(冗余)" json:"device_name"`
	RegionID     *int64    `gorm:"column:region_id;comment:所属区域" json:"region_id"`
	Location     string    `gorm:"column:location;size:255;comment:设备位置(冗余)" json:"location"`
	FirmwareID   *int64    `gorm:"column:firmware_id;comment:升级固件ID" json:"firmware_id"`
	AIModelID    *int64    `gorm:"column:ai_model_id;comment:升级模型ID" json:"ai_model_id"`
	FirmwareName string    `gorm:"column:firmware_name;size:255;comment:升级固件名称(冗余)" json:"firmware_name"`
	AIModelName  string    `gorm:"column:ai_model_name;size:255;comment:升级模型名称(冗余)" json:"ai_model_name"`
	PlanDT       time.Time `gorm:"column:plan_dt;index;comment:升级时间" json:"plan_dt"`
	Status       string    `gorm:"column:status;size:20;default:pending;index;comment:pending/running/done/failed/canceled" json:"status"`
	Remark       string    `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64  `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Device *Device `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
}

func (DeviceUpgradePlan) TableName() string { return "ac_device_upgrade_plan" }

// DeviceUpgradeRecord 设备升级记录。表 ac_device_upgrade_record。
// 保留升级前的固件/模型版本（prev_*），使同一设备升级两次后可对比版本变化。
type DeviceUpgradeRecord struct {
	BaseModel
	DeviceID   int64  `gorm:"column:device_id;not null;index;comment:设备ID" json:"device_id"`
	DeviceCode string `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	DeviceName string `gorm:"column:device_name;size:255;comment:设备名称(冗余)" json:"device_name"`
	RegionID   *int64 `gorm:"column:region_id;comment:所属区域" json:"region_id"`
	Location   string `gorm:"column:location;size:255;comment:设备位置(冗余)" json:"location"`
	IotCardNo  string `gorm:"column:iot_card_no;size:100;comment:流量卡号(冗余)" json:"iot_card_no"`

	PrevFirmwareID  *int64 `gorm:"column:prev_firmware_id;comment:上一固件ID" json:"prev_firmware_id"`
	PrevFirmwareVer string `gorm:"column:prev_firmware_ver;size:50;comment:上一固件版本" json:"prev_firmware_ver"`
	PrevAIModelID   *int64 `gorm:"column:prev_ai_model_id;comment:上一模型ID" json:"prev_ai_model_id"`
	PrevAIModelVer  string `gorm:"column:prev_ai_model_ver;size:50;comment:上一模型版本" json:"prev_ai_model_ver"`

	FirmwareID  *int64    `gorm:"column:firmware_id;comment:升级固件ID" json:"firmware_id"`
	FirmwareVer string    `gorm:"column:firmware_ver;size:50;comment:升级固件版本" json:"firmware_ver"`
	AIModelID   *int64    `gorm:"column:ai_model_id;comment:升级模型ID" json:"ai_model_id"`
	AIModelVer  string    `gorm:"column:ai_model_ver;size:50;comment:升级模型版本" json:"ai_model_ver"`
	UpgradeType string    `gorm:"column:upgrade_type;size:20;comment:升级方式 now/plan" json:"upgrade_type"`
	UpgradeDT   time.Time `gorm:"column:upgrade_dt;index;comment:升级时间" json:"upgrade_dt"`
	Status      string    `gorm:"column:status;size:20;default:done;comment:升级结果" json:"status"`
	Operator    string    `gorm:"column:operator;size:50;comment:操作人" json:"operator"`
	ErrorMsg    string    `gorm:"column:error_msg;type:text;comment:失败原因" json:"error_msg"`

	DeptID *int64  `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
	Device *Device `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
}

func (DeviceUpgradeRecord) TableName() string { return "ac_device_upgrade_record" }
