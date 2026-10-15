package response

import "github.com/boticz/ai-camera-backend/internal/model"

// ════════════════════════════════════════════════════════════════
// 统计分析：首页看板 / 风险分析 / 设备告警分析 / 可视化大屏 / 流量监控
// ════════════════════════════════════════════════════════════════

// ── 统计公共小结构 ──────────────────────────────────────────────

// NameCount 名称-数量统计项。按风险类型/区域/等级/设备分组统计时统一使用。
type NameCount struct {
	Name  string `json:"name" example:"未佩戴安全帽"`
	Count int64  `json:"count" example:"12"`
}

// TrendPoint 日期-数量趋势点。Date 为 yyyy-MM-dd。
type TrendPoint struct {
	Date  string `json:"date" example:"2026-09-21"`
	Count int64  `json:"count" example:"5"`
}

// DeviceStat 设备数量统计。
type DeviceStat struct {
	Total   int64 `json:"total" example:"120"`
	Online  int64 `json:"online" example:"96"`
	Offline int64 `json:"offline" example:"24"`
}

// HandleStat 风险处理情况统计。
type HandleStat struct {
	Pending int64 `json:"pending" example:"8"`
	Handled int64 `json:"handled" example:"42"`
}

// IotCardTrafficStat 物联网卡流量统计项。
type IotCardTrafficStat struct {
	CardNo       string `json:"card_no" example:"89860600000000000001"`
	Name         string `json:"name" example:"1号卡"`
	RegionName   string `json:"region_name" example:"东区厂房"`
	DeviceName   string `json:"device_name" example:"东区-01"`
	TrafficTotal int64  `json:"traffic_total" example:"10240"`
	TrafficUsed  int64  `json:"traffic_used" example:"8192"`
	TrafficLeft  int64  `json:"traffic_left" example:"2048"`
	Status       string `json:"status" example:"normal"`
}

// ── 首页看板 ────────────────────────────────────────────────────

// DashboardResp 首页看板数据。
//
// 「今日/本周/本月预警」按风险与告警分别给出发生数（*_risk_count / *_alarm_count）
// 与处理数（*_handled_count / *_risk_handled_count / *_alarm_handled_count），
// 合计口径的处理数为 *_handled_count。
type DashboardResp struct {
	TodayRiskCount    int64 `json:"today_risk_count" example:"6"`
	TodayAlarmCount   int64 `json:"today_alarm_count" example:"2"`
	TodayHandledCount int64 `json:"today_handled_count" example:"5"`
	TodayRiskHandled  int64 `json:"today_risk_handled_count" example:"4"`
	TodayAlarmHandled int64 `json:"today_alarm_handled_count" example:"1"`

	WeekRiskCount    int64 `json:"week_risk_count" example:"38"`
	WeekAlarmCount   int64 `json:"week_alarm_count" example:"11"`
	WeekHandledCount int64 `json:"week_handled_count" example:"40"`
	WeekRiskHandled  int64 `json:"week_risk_handled_count" example:"31"`
	WeekAlarmHandled int64 `json:"week_alarm_handled_count" example:"9"`

	MonthRiskCount    int64 `json:"month_risk_count" example:"152"`
	MonthAlarmCount   int64 `json:"month_alarm_count" example:"44"`
	MonthHandledCount int64 `json:"month_handled_count" example:"168"`
	MonthRiskHandled  int64 `json:"month_risk_handled_count" example:"131"`
	MonthAlarmHandled int64 `json:"month_alarm_handled_count" example:"37"`

	// DeviceStat 设备统计：总数/在线/离线。
	DeviceStat DeviceStat `json:"device_stat"`
	// RiskTypeStats 风险类型分析（近 30 天）。
	RiskTypeStats []NameCount `json:"risk_type_stats"`
	// RiskHandleStats 风险处理统计（近 30 天）。
	RiskHandleStats HandleStat `json:"risk_handle_stats"`
	// RegionRiskStats 区域风险统计（近 30 天）。
	RegionRiskStats []NameCount `json:"region_risk_stats"`
	// RiskTrend 风险趋势（近 7 天）。
	RiskTrend []TrendPoint `json:"risk_trend"`
	// AlarmTrend 告警趋势（近 7 天）。
	AlarmTrend []TrendPoint `json:"alarm_trend"`
}

// RDashboard 首页看板响应。
type RDashboard struct {
	BaseResponse
	Data DashboardResp `json:"data"`
}

// ── 风险统计分析 ────────────────────────────────────────────────

// RiskStatsResp 风险统计分析数据（按 start_dt ~ end_dt 区间）。
type RiskStatsResp struct {
	Total   int64 `json:"total" example:"160"`
	Handled int64 `json:"handled" example:"132"`
	Pending int64 `json:"pending" example:"28"`

	// ByType 按风险类型分组统计。
	ByType []NameCount `json:"by_type"`
	// ByRegion 按所属区域分组统计。
	ByRegion []NameCount `json:"by_region"`
	// ByLevel 按风险等级分组统计，name 取 high/medium/low。
	ByLevel []NameCount `json:"by_level"`
	// ByDevice 按设备分组统计（Top 10）。
	ByDevice []NameCount `json:"by_device"`
	// Trend 区间内逐日风险趋势。
	Trend []TrendPoint `json:"trend"`
}

// RRiskStats 风险统计分析响应。
type RRiskStats struct {
	BaseResponse
	Data RiskStatsResp `json:"data"`
}

// ── 设备告警分析 ────────────────────────────────────────────────

// AlarmStatsResp 设备告警分析数据（按 start_dt ~ end_dt 区间）。
type AlarmStatsResp struct {
	Total   int64 `json:"total" example:"44"`
	Handled int64 `json:"handled" example:"40"`
	Pending int64 `json:"pending" example:"4"`

	// ByType 按告警类型分组统计。
	ByType []NameCount `json:"by_type"`
	// ByRegion 按所属区域分组统计。
	ByRegion []NameCount `json:"by_region"`
	// ByLevel 按告警等级分组统计，name 取 high/medium/low。
	ByLevel []NameCount `json:"by_level"`
	// ByDevice 按设备分组统计（Top 10）。
	ByDevice []NameCount `json:"by_device"`
	// Trend 区间内逐日告警趋势。
	Trend []TrendPoint `json:"trend"`
}

// RAlarmStats 设备告警分析响应。
type RAlarmStats struct {
	BaseResponse
	Data AlarmStatsResp `json:"data"`
}

// ── 可视化大屏 ──────────────────────────────────────────────────

// ScreenResp 可视化大屏数据。
type ScreenResp struct {
	// RiskRealtime 最新 10 条风险（已预载 risk_type / device / region 名称）。
	RiskRealtime []model.RiskRecord `json:"risk_realtime"`
	// AlarmRealtime 最新 10 条设备告警（已预载 alarm_type / device / region 名称）。
	AlarmRealtime []model.AlarmRecord `json:"alarm_realtime"`
	// RiskStats 风险类型统计（近 30 天）。
	RiskStats []NameCount `json:"risk_stats"`
	// AlarmStats 告警类型统计（近 30 天）。
	AlarmStats []NameCount `json:"alarm_stats"`
	// DeviceMonitor 设备监控：总数/在线/离线。
	DeviceMonitor DeviceStat `json:"device_monitor"`
	// TrafficMonitor 流量监控：剩余流量最少的 10 张物联网卡。
	TrafficMonitor []IotCardTrafficStat `json:"traffic_monitor"`

	TodayRiskCount  int64 `json:"today_risk_count" example:"6"`
	TodayAlarmCount int64 `json:"today_alarm_count" example:"2"`
}

// RScreen 可视化大屏响应。
type RScreen struct {
	BaseResponse
	Data ScreenResp `json:"data"`
}

// ── 物联网卡流量监控 ────────────────────────────────────────────

// TrafficStatsResp 物联网卡流量监控数据。
type TrafficStatsResp struct {
	TotalTraffic int64 `json:"total_traffic" example:"1048576"`
	UsedTraffic  int64 `json:"used_traffic" example:"524288"`
	LeftTraffic  int64 `json:"left_traffic" example:"524288"`

	// Items 全部物联网卡流量明细（剩余流量升序）。
	Items []IotCardTrafficStat `json:"items"`
}

// RTrafficStats 物联网卡流量监控响应。
type RTrafficStats struct {
	BaseResponse
	Data TrafficStatsResp `json:"data"`
}

// ── 设备状态统计 ────────────────────────────────────────────────

// DeviceStatusResp 设备状态统计。
//
// LowBattery 为电量低于 20% 的设备数；FullStorage 为剩余存储不足总容量 10% 的设备数。
type DeviceStatusResp struct {
	Total       int64 `json:"total" example:"120"`
	Online      int64 `json:"online" example:"96"`
	Offline     int64 `json:"offline" example:"24"`
	LowBattery  int64 `json:"low_battery" example:"7"`
	FullStorage int64 `json:"full_storage" example:"3"`
}

// RDeviceStatus 设备状态统计响应。
type RDeviceStatus struct {
	BaseResponse
	Data DeviceStatusResp `json:"data"`
}
