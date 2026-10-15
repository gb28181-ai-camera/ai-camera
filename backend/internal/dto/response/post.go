package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PagePostResp 岗位分页载荷。
type PagePostResp struct {
	PageResp
	Items []model.Post `json:"items"`
}

// RPagePost 岗位分页响应。
type RPagePost struct {
	BaseResponse
	Data PagePostResp `json:"data"`
}

// RPostList 岗位简单列表响应（下拉选项以外的全量列表）。
type RPostList struct {
	BaseResponse
	Data []model.Post `json:"data"`
}

// RPost 单条岗位响应（含关联角色 ID 列表 role_ids）。
type RPost struct {
	BaseResponse
	Data model.Post `json:"data"`
}
