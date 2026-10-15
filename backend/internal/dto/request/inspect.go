package request

// ════════════════════════════════════════════════════════════════
// 巡检中心：巡检任务 / 巡检记录
// ════════════════════════════════════════════════════════════════

// PageInspectTaskReq 巡检任务分页查询参数。
// 时间区间按「巡检时间」plan_dt 过滤。
type PageInspectTaskReq struct {
	PageReq
	Keyword     string `form:"keyword" json:"keyword" example:"配电房巡检"`
	RegionID    *int64 `form:"region_id" json:"region_id" example:"1"`
	InspectorID *int64 `form:"inspector_id" json:"inspector_id" example:"3"`
	Status      string `form:"status" json:"status" example:"published"`
	StartDT     string `form:"start_dt" json:"start_dt" example:"2026-09-01"`
	EndDT       string `form:"end_dt" json:"end_dt" example:"2026-09-30"`
}

// CreateInspectTaskReq 创建巡检任务。
type CreateInspectTaskReq struct {
	Name        string `json:"name" binding:"required" example:"配电房巡检"`
	RegionID    *int64 `json:"region_id" example:"1"`
	Location    string `json:"location" example:"1号配电房"`
	InspectorID int64  `json:"inspector_id" binding:"required" example:"3"`
	Content     string `json:"content" example:"检查设备外观、接线端子与运行温度"`
	PlanDT      string `json:"plan_dt" example:"2026-09-22 09:00:00"`
	Status      string `json:"status" example:"published"`
	Remark      string `json:"remark" example:"雨季重点巡检"`
}

// UpdateInspectTaskReq 更新巡检任务。
// 仅 status 为 published / pending 的任务可编辑，finished 的任务由移动端完成后不可修改。
type UpdateInspectTaskReq struct {
	Name        string `json:"name" binding:"required" example:"配电房巡检"`
	RegionID    *int64 `json:"region_id" example:"1"`
	Location    string `json:"location" example:"1号配电房"`
	InspectorID int64  `json:"inspector_id" binding:"required" example:"3"`
	Content     string `json:"content" example:"检查设备外观、接线端子与运行温度"`
	PlanDT      string `json:"plan_dt" example:"2026-09-22 09:00:00"`
	Status      string `json:"status" example:"published"`
	Remark      string `json:"remark" example:"雨季重点巡检"`
}

// PageInspectRecordReq 巡检记录分页查询参数。
// 时间区间按「巡检时间」plan_dt 过滤，与巡检任务列表保持一致。
type PageInspectRecordReq struct {
	PageReq
	Keyword     string `form:"keyword" json:"keyword" example:"配电房"`
	RegionID    *int64 `form:"region_id" json:"region_id" example:"1"`
	InspectorID *int64 `form:"inspector_id" json:"inspector_id" example:"3"`
	StartDT     string `form:"start_dt" json:"start_dt" example:"2026-09-01"`
	EndDT       string `form:"end_dt" json:"end_dt" example:"2026-09-30"`
}

// FinishInspectTaskReq 移动端提交巡检结果。
type FinishInspectTaskReq struct {
	Result      string `json:"result" binding:"required" example:"设备运行正常，无异常发热"`
	ResultImage string `json:"result_image" example:"/uploads/2026/09/inspect.jpg"`
}
