package request

// PageAlarmReq 设备告警分页查询请求。
//
// 与 PageRiskReq 同构，告警维度由风险类型换为告警类型；
// 告警来自设备自身状态（离线/电量/流量），没有「检测准确」维度。
type PageAlarmReq struct {
	PageReq
	DateRangeReq

	Keyword     string `form:"keyword" json:"keyword" example:"CAM001"`
	AlarmTypeID int64  `form:"alarm_type_id" json:"alarm_type_id" example:"1"`
	RegionID    int64  `form:"region_id" json:"region_id" example:"1"`
	DeviceID    int64  `form:"device_id" json:"device_id" example:"1"`
	DeviceCode  string `form:"device_code" json:"device_code" example:"CAM001"`
	Status      string `form:"status" json:"status" example:"pending"`
	Read        *bool  `form:"read" json:"read" example:"false"`
}

// HandleAlarmReq 告警处理请求：处理结果与处理图片。
type HandleAlarmReq struct {
	HandleResult string `json:"handle_result" binding:"required" example:"已更换电池并重启设备"`
	HandleImage  string `json:"handle_image" example:"/uploads/2026/09/alarm.jpg"`
}

// ReportAlarmReq 设备上报告警请求（ApiKey 认证，无用户上下文）。
//
// AlarmLevel 缺省时取告警类型配置的 level。
type ReportAlarmReq struct {
	DeviceCode    string `json:"device_code" binding:"required" example:"CAM001"`
	AlarmTypeCode string `json:"alarm_type_code" binding:"required" example:"battery"`
	Content       string `json:"content" example:"设备电量低于10%"`
	AlarmLevel    string `json:"alarm_level" example:"high"`
	OccurredDT    string `json:"occurred_dt" example:"2026-09-21 10:30:00"`
}
