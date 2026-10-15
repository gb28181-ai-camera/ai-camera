package response

import "github.com/boticz/ai-camera-backend/internal/model"

// ════════════════════════════════════════════════════════════════
// 移动端（App）响应结构
// ════════════════════════════════════════════════════════════════

// AppDeviceStat App 首页设备状态统计。
type AppDeviceStat struct {
	Total   int64 `json:"total" example:"120"`  // 设备总数
	Online  int64 `json:"online" example:"98"`  // 在线设备数
	Offline int64 `json:"offline" example:"22"` // 离线设备数
}

// AppHomeResp App 首页仪表盘数据。
//
// 聚合「待处理风险/告警、我的待巡检任务、今日新增、设备在线状态、
// 最新风险、未读通知」六类信息，供移动端首页一屏展示。
type AppHomeResp struct {
	PendingRiskCount    int64              `json:"pending_risk_count" example:"12"`   // 待处理风险数
	PendingAlarmCount   int64              `json:"pending_alarm_count" example:"5"`   // 待处理告警数
	PendingInspectCount int64              `json:"pending_inspect_count" example:"3"` // 我的待巡检任务数
	TodayRiskCount      int64              `json:"today_risk_count" example:"7"`      // 今日新增风险数
	TodayAlarmCount     int64              `json:"today_alarm_count" example:"2"`     // 今日新增告警数
	DeviceStat          AppDeviceStat      `json:"device_stat"`                       // 设备状态统计
	RecentRisks         []model.RiskRecord `json:"recent_risks"`                      // 最新 5 条风险
	UnreadCount         int64              `json:"unread_count" example:"9"`          // 未读通知数
}

// RAppHome App 首页仪表盘响应。
type RAppHome struct {
	BaseResponse
	Data AppHomeResp `json:"data"`
}

// AppTodoItem App 待办项。
//
// Category 取值：risk（待处理风险）/ alarm（待处理告警）/
// inspect（我的待巡检任务）/ notification（未读通知）。
type AppTodoItem struct {
	Category string `json:"category" example:"risk"` // 待办类别
	Title    string `json:"title" example:"待处理风险"`   // 待办标题
	Count    int64  `json:"count" example:"12"`      // 待办数量
	RefID    int64  `json:"ref_id" example:"0"`      // 关联业务 ID（列表类为 0）
}

// RAppTodoList App 待办列表响应。
type RAppTodoList struct {
	BaseResponse
	Data []AppTodoItem `json:"data"`
}

// RAppMenuList 移动端菜单树响应（仅返回 platform=mobile 的菜单）。
type RAppMenuList struct {
	BaseResponse
	Data []model.Menu `json:"data"`
}
