package model

// ════════════════════════════════════════════════════════════════
// 基础信息管理：区域 / 风险类型 / 告警类型
// ════════════════════════════════════════════════════════════════

// Region 区域。表 ac_region。
// 区域是设备、风险、告警、巡检的核心归属维度，几乎所有业务列表都支持按区域筛选。
type Region struct {
	BaseModel
	Active   *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name     string `gorm:"column:name;size:255;not null;comment:区域名称" json:"name"`
	Code     string `gorm:"column:code;size:50;comment:区域编码" json:"code"`
	Remark   string `gorm:"column:remark;size:500;comment:备注" json:"remark"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (Region) TableName() string { return "ac_region" }

// RiskType 风险类型。表 ac_risk_type。
// 风险等级用于首页/大屏的分级统计与告警配色。
type RiskType struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence    int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name        string `gorm:"column:name;size:255;not null;comment:风险类型名称" json:"name"`
	Code        string `gorm:"column:code;size:50;comment:风险编码" json:"code"`
	Level       string `gorm:"column:level;size:20;default:medium;comment:风险等级 high/medium/low" json:"level"`
	Description string `gorm:"column:description;type:text;comment:描述" json:"description"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (RiskType) TableName() string { return "ac_risk_type" }

// AlarmType 告警类型。表 ac_alarm_type。
// 平台内置离线告警、电量告警、流量告警，也可自定义扩展。
type AlarmType struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence    int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name        string `gorm:"column:name;size:255;not null;comment:告警名称" json:"name"`
	Code        string `gorm:"column:code;size:50;comment:告警编码" json:"code"`
	Level       string `gorm:"column:level;size:20;default:medium;comment:告警等级 high/medium/low" json:"level"`
	Description string `gorm:"column:description;type:text;comment:描述" json:"description"`

	DeptID *int64 `gorm:"column:dept_id;index;comment:所属部门(数据权限)" json:"dept_id"`
}

func (AlarmType) TableName() string { return "ac_alarm_type" }

// 内置告警编码常量（种子数据同时写入）。
const (
	AlarmCodeOffline = "offline" // 离线告警
	AlarmCodeBattery = "battery" // 电量告警
	AlarmCodeTraffic = "traffic" // 流量告警
	AlarmCodeFall    = "fall"    // 跌倒告警（适老化相机）
	AlarmCodeSOS     = "sos"     // 一键呼救（适老化相机）
)

// 风险/告警等级常量。
const (
	LevelHigh   = "high"
	LevelMedium = "medium"
	LevelLow    = "low"
)
