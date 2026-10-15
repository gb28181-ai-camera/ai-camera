package model

import "time"

// ════════════════════════════════════════════════════════════════
// AI 风险与设备告警
// ════════════════════════════════════════════════════════════════

// 风险/告警处理状态常量。
const (
	HandleStatusPending = "pending" // 未处理
	HandleStatusHandled = "handled" // 已处理
)

// 检测准确性常量：设备 AI 识别结果是否准确。
const (
	DetectAccurateYes = "yes"
	DetectAccurateNo  = "no"
)

// RiskRecord AI 风险记录。表 ac_risk_record。
//
// 生命周期（对应操作手册「AI风险」章节）：
//  1. 设备识别到风险 → 落库，status=pending，read=false，detect_accurate 待人工确认
//  2. 人工点「检测准确=否」→ 直接置 status=handled 且 read=true（误报，无需处理）
//  3. 人工点「检测准确=是」→ read=true，仍需处理，点击「处理」填写处理结果与图片
//  4. 处理完成 → status=handled，记录处理人/时间/结果/图片
type RiskRecord struct {
	BaseModel
	RiskTypeID  int64     `gorm:"column:risk_type_id;not null;index;comment:风险类型ID" json:"risk_type_id"`
	DeviceID    int64     `gorm:"column:device_id;not null;index;comment:设备ID" json:"device_id"`
	DeviceCode  string    `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	RegionID    *int64    `gorm:"column:region_id;index;comment:所属区域" json:"region_id"`
	Location    string    `gorm:"column:location;size:255;comment:设备位置(冗余)" json:"location"`
	ImageURL    string    `gorm:"column:image_url;size:500;comment:风险图片" json:"image_url"`
	VideoURL    string    `gorm:"column:video_url;size:500;comment:风险录像" json:"video_url"`
	RiskLevel   string    `gorm:"column:risk_level;size:20;comment:风险等级(冗余)" json:"risk_level"`
	DetectScore float64   `gorm:"column:detect_score;comment:AI 置信度" json:"detect_score"`
	OccurredDT  time.Time `gorm:"column:occurred_dt;index;comment:发生时间" json:"occurred_dt"`

	// 检测准确（人工确认）
	DetectAccurate string     `gorm:"column:detect_accurate;size:10;comment:检测准确 yes/no" json:"detect_accurate"`
	AccurateBy     string     `gorm:"column:accurate_by;size:50;comment:确认人" json:"accurate_by"`
	AccurateDT     *time.Time `gorm:"column:accurate_dt;comment:确认时间" json:"accurate_dt"`

	// 消息通知
	Read   *bool      `gorm:"column:read;default:false;comment:消息通知是否已读" json:"read"`
	ReadDT *time.Time `gorm:"column:read_dt;comment:已读时间" json:"read_dt"`

	// 处理
	Status       string     `gorm:"column:status;size:20;default:pending;index;comment:处理状态 pending/handled" json:"status"`
	HandleResult string     `gorm:"column:handle_result;type:text;comment:处理结果" json:"handle_result"`
	HandleImage  string     `gorm:"column:handle_image;size:500;comment:处理图片" json:"handle_image"`
	Handler      string     `gorm:"column:handler;size:50;comment:处理人" json:"handler"`
	HandledDT    *time.Time `gorm:"column:handled_dt;comment:处理时间" json:"handled_dt"`

	// 语音播报（若该风险类型配置了语音）
	VoicePlayed *bool `gorm:"column:voice_played;default:false;comment:是否已语音播报" json:"voice_played"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	RiskType *RiskType `gorm:"foreignKey:RiskTypeID;references:ID" json:"risk_type,omitempty"`
	Device   *Device   `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Region   *Region   `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
}

func (RiskRecord) TableName() string { return "ac_risk_record" }

// AlarmRecord 设备告警记录。表 ac_alarm_record。
// 结构与 RiskRecord 平行：告警来自设备自身状态（离线/电量/流量），
// 风险来自 AI 识别。两者在「系统通知」中统一汇聚。
type AlarmRecord struct {
	BaseModel
	AlarmTypeID int64     `gorm:"column:alarm_type_id;not null;index;comment:告警类型ID" json:"alarm_type_id"`
	DeviceID    int64     `gorm:"column:device_id;not null;index;comment:设备ID" json:"device_id"`
	DeviceCode  string    `gorm:"column:device_code;size:100;comment:设备编号(冗余)" json:"device_code"`
	RegionID    *int64    `gorm:"column:region_id;index;comment:所属区域" json:"region_id"`
	Location    string    `gorm:"column:location;size:255;comment:设备位置(冗余)" json:"location"`
	AlarmLevel  string    `gorm:"column:alarm_level;size:20;comment:告警等级(冗余)" json:"alarm_level"`
	Content     string    `gorm:"column:content;type:text;comment:告警内容" json:"content"`
	OccurredDT  time.Time `gorm:"column:occurred_dt;index;comment:告警时间" json:"occurred_dt"`

	Read   *bool      `gorm:"column:read;default:false;comment:消息通知是否已读" json:"read"`
	ReadDT *time.Time `gorm:"column:read_dt;comment:已读时间" json:"read_dt"`

	Status       string     `gorm:"column:status;size:20;default:pending;index;comment:处理状态 pending/handled" json:"status"`
	HandleResult string     `gorm:"column:handle_result;type:text;comment:处理结果" json:"handle_result"`
	HandleImage  string     `gorm:"column:handle_image;size:500;comment:处理图片" json:"handle_image"`
	Handler      string     `gorm:"column:handler;size:50;comment:处理人" json:"handler"`
	HandledDT    *time.Time `gorm:"column:handled_dt;comment:处理时间" json:"handled_dt"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`

	AlarmType *AlarmType `gorm:"foreignKey:AlarmTypeID;references:ID" json:"alarm_type,omitempty"`
	Device    *Device    `gorm:"foreignKey:DeviceID;references:ID" json:"device,omitempty"`
	Region    *Region    `gorm:"foreignKey:RegionID;references:ID" json:"region,omitempty"`
}

func (AlarmRecord) TableName() string { return "ac_alarm_record" }
