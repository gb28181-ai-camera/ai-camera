package request

// PageRiskReq AI 风险记录分页查询请求。
//
// 过滤维度与前端「AI风险」列表一致：
// 关键字（设备编号 / 设备名称模糊匹配）、风险类型、区域、设备、
// 处理状态、检测准确性、已读状态与发生时间范围。
// Read 使用指针以区分「未传」与「显式传 false（只看未读）」。
type PageRiskReq struct {
	PageReq
	DateRangeReq

	Keyword        string `form:"keyword" json:"keyword" example:"CAM001"`
	RiskTypeID     int64  `form:"risk_type_id" json:"risk_type_id" example:"1"`
	RegionID       int64  `form:"region_id" json:"region_id" example:"1"`
	DeviceID       int64  `form:"device_id" json:"device_id" example:"1"`
	DeviceCode     string `form:"device_code" json:"device_code" example:"CAM001"`
	Status         string `form:"status" json:"status" example:"pending"`
	DetectAccurate string `form:"detect_accurate" json:"detect_accurate" example:"yes"`
	Read           *bool  `form:"read" json:"read" example:"false"`
}

// HandleRiskReq 风险处理请求：处理结果与处理图片。
type HandleRiskReq struct {
	HandleResult string `json:"handle_result" binding:"required" example:"已现场驱离并复位警戒线"`
	HandleImage  string `json:"handle_image" example:"/uploads/2026/09/handle.jpg"`
}

// UpdateAccurateReq 检测准确性确认请求。accurate 取值 yes（确认风险）/ no（误报）。
type UpdateAccurateReq struct {
	Accurate string `json:"accurate" binding:"required,oneof=yes no" example:"yes"`
}

// ReportRiskReq 设备上报风险请求（ApiKey 认证，无用户上下文）。
//
// OccurredDT 为字符串，支持 "2006-01-02 15:04:05"、RFC3339、"2006-01-02" 等格式；
// 缺省或无法解析时以服务端当前时间作为发生时间。
type ReportRiskReq struct {
	DeviceCode   string  `json:"device_code" binding:"required" example:"CAM001"`
	RiskTypeCode string  `json:"risk_type_code" binding:"required" example:"intrusion"`
	ImageURL     string  `json:"image_url" example:"/uploads/2026/09/risk.jpg"`
	VideoURL     string  `json:"video_url" example:"/uploads/2026/09/risk.mp4"`
	DetectScore  float64 `json:"detect_score" example:"0.93"`
	OccurredDT   string  `json:"occurred_dt" example:"2026-09-21 10:30:00"`
}
