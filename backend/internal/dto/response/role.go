package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PageRoleResp 角色分页载荷。
type PageRoleResp struct {
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Items    []model.Role `json:"items" filter:"-"`
}

// RPageRole 角色分页响应。
type RPageRole struct {
	BaseResponse
	Data PageRoleResp `json:"data"`
}

// RRoleList 角色简单列表响应（无分页）。
type RRoleList struct {
	BaseResponse
	Data []model.Role `json:"data"`
}

// RRole 单条角色详情响应。
type RRole struct {
	BaseResponse
	Data model.Role `json:"data"`
}

// RIDList ID 集合响应，用于角色已授权的菜单/部门/岗位/用户 ID 列表。
type RIDList struct {
	BaseResponse
	Data []int64 `json:"data"`
}
