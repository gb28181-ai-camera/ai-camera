package model

// ════════════════════════════════════════════════════════════════
// 告警设置：设备告警设置 / 风险通知设置
// ════════════════════════════════════════════════════════════════

// 通知类型常量。
const (
	NotifyTypeApp   = "app"   // APP 推送（个推）
	NotifyTypeSms   = "sms"   // 短信（腾讯云）
	NotifyTypeInner = "inner" // 站内通知
)

// 报警频率常量（分钟）。
const (
	NotifyFreqRealtime = 0    // 实时
	NotifyFreqHourly   = 60   // 每小时
	NotifyFreqDaily    = 1440 // 每天
)

// AlarmSetting 设备告警设置。表 ac_alarm_setting。
//
// 约束：只能配置当前登录用户自己的数据（UserID = 当前用户）。
// 一条记录对应「某用户 + 某告警类型」的通知策略；
// Active=false 即为「关闭该类型的所有告警通知」。
type AlarmSetting struct {
	BaseModel
	UserID      int64  `gorm:"column:user_id;not null;index:idx_ac_alarm_setting_user_type,priority:1;comment:配置所属用户" json:"user_id"`
	AlarmTypeID int64  `gorm:"column:alarm_type_id;not null;index:idx_ac_alarm_setting_user_type,priority:2;comment:告警类型ID" json:"alarm_type_id"`
	Active      *bool  `gorm:"column:active;default:true;comment:是否启用(关闭所有告警通知)" json:"active"`
	NotifyType  string `gorm:"column:notify_type;size:20;default:app;comment:通知类型 app/sms/inner" json:"notify_type"`
	NotifyFreq  int    `gorm:"column:notify_freq;default:0;comment:报警频率(分钟)" json:"notify_freq"`

	// 通知人员（多个用户 ID，逗号分隔）
	NotifyUserIDs string `gorm:"column:notify_user_ids;type:text;comment:通知人员用户ID列表" json:"notify_user_ids"`
	// 阈值：电量告警阈值（%）、流量告警阈值（MB），按告警类型取用
	Threshold string `gorm:"column:threshold;size:50;comment:阈值" json:"threshold"`
	Remark    string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	AlarmType *AlarmType `gorm:"foreignKey:AlarmTypeID;references:ID" json:"alarm_type,omitempty"`
}

func (AlarmSetting) TableName() string { return "ac_alarm_setting" }

// RiskNotifySetting 风险通知设置。表 ac_risk_notify_setting。
// 与 AlarmSetting 结构平行，维度换成风险类型。
type RiskNotifySetting struct {
	BaseModel
	UserID     int64  `gorm:"column:user_id;not null;index:idx_ac_risk_notify_user_type,priority:1;comment:配置所属用户" json:"user_id"`
	RiskTypeID int64  `gorm:"column:risk_type_id;not null;index:idx_ac_risk_notify_user_type,priority:2;comment:风险类型ID" json:"risk_type_id"`
	Active     *bool  `gorm:"column:active;default:true;comment:是否启用(关闭所有风险通知)" json:"active"`
	NotifyType string `gorm:"column:notify_type;size:20;default:app;comment:通知类型 app/sms/inner" json:"notify_type"`
	NotifyFreq int    `gorm:"column:notify_freq;default:0;comment:报警频率(分钟)" json:"notify_freq"`

	NotifyUserIDs string `gorm:"column:notify_user_ids;type:text;comment:通知人员用户ID列表" json:"notify_user_ids"`
	Remark        string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	RiskType *RiskType `gorm:"foreignKey:RiskTypeID;references:ID" json:"risk_type,omitempty"`
}

func (RiskNotifySetting) TableName() string { return "ac_risk_notify_setting" }

// ThirdPartyConfig 第三方平台配置（个推账号 / 腾讯云短信）。表 ac_thirdparty_config。
//
// 以键值形式存储，避免为两组固定配置各建一张窄表。
// Category 取值：getui / sms。
type ThirdPartyConfig struct {
	BaseModel
	Category string `gorm:"column:category;size:50;not null;index;comment:getui/sms" json:"category"`
	Name     string `gorm:"column:name;size:255;comment:配置名称" json:"name"`
	Code     string `gorm:"column:code;size:100;not null;comment:配置键" json:"code"`
	Value    string `gorm:"column:value;type:text;comment:配置值" json:"value"`
	Secret   *bool  `gorm:"column:secret;default:false;comment:是否敏感值(接口返回时脱敏)" json:"secret"`
	Remark   string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (ThirdPartyConfig) TableName() string { return "ac_thirdparty_config" }

// 第三方配置键常量。
const (
	GetuiCfgAppID        = "getui_app_id"
	GetuiCfgAppKey       = "getui_app_key"
	GetuiCfgAppSecret    = "getui_app_secret"
	GetuiCfgMasterSecret = "getui_master_secret"

	SmsCfgSecretID  = "sms_secret_id"
	SmsCfgSecretKey = "sms_secret_key"
	SmsCfgSdkAppID  = "sms_sdk_app_id"
	SmsCfgSignName  = "sms_sign_name"
)
