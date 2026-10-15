package response

import "github.com/boticz/ai-camera-backend/internal/model"

// RDeptTree 部门树响应（data 为按 parent_id 组装的树，children 嵌套）。
type RDeptTree struct {
	BaseResponse
	Data []model.Dept `json:"data"`
}

// RDept 单条部门响应。
type RDept struct {
	BaseResponse
	Data model.Dept `json:"data"`
}
