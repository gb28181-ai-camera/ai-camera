package response

import "github.com/boticz/ai-camera-backend/internal/model"

// ════════════════════════════════════════════════════════════════
// 巡检中心
// ════════════════════════════════════════════════════════════════

// InspectTaskItem 巡检任务列表项。
//
// 在任务本体之外冗余携带巡检人姓名（ac_user.name，由列表查询 JOIN 得到），
// 避免前端为每一行再查一次用户。
type InspectTaskItem struct {
	model.InspectTask
	// InspectorName 巡检人姓名。
	InspectorName string `gorm:"column:inspector_name" json:"inspector_name"`
}

// PageInspectTaskResp 巡检任务分页载荷。
type PageInspectTaskResp struct {
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Items    []InspectTaskItem `json:"items"`
}

// RPageInspectTask 巡检任务分页响应。
type RPageInspectTask struct {
	BaseResponse
	Data PageInspectTaskResp `json:"data"`
}

// PageInspectRecordResp 巡检记录分页载荷。
type PageInspectRecordResp struct {
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Items    []model.InspectRecord `json:"items"`
}

// RPageInspectRecord 巡检记录分页响应。
type RPageInspectRecord struct {
	BaseResponse
	Data PageInspectRecordResp `json:"data"`
}

// RInspectTask 巡检任务详情响应。
type RInspectTask struct {
	BaseResponse
	Data model.InspectTask `json:"data"`
}

// RInspectRecord 巡检记录详情响应。
type RInspectRecord struct {
	BaseResponse
	Data model.InspectRecord `json:"data"`
}
