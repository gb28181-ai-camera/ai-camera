package response

import "github.com/boticz/ai-camera-backend/internal/model"

// RMenuList 菜单树列表响应（data 为按 parent_id 组装的树，children 嵌套）。
type RMenuList struct {
	BaseResponse
	Data []model.Menu `json:"data"`
}

// RMenu 单条菜单响应（含绑定的接口列表 apis）。
type RMenu struct {
	BaseResponse
	Data model.Menu `json:"data"`
}

// RMenuAPIList 菜单接口列表响应。
type RMenuAPIList struct {
	BaseResponse
	Data []model.MenuAPI `json:"data"`
}

// RMenuAPI 单条菜单接口响应。
type RMenuAPI struct {
	BaseResponse
	Data model.MenuAPI `json:"data"`
}

// MenuRoleBrief 菜单已授权角色的摘要信息。
type MenuRoleBrief struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// RMenuRoleList 菜单已授权角色列表响应。
type RMenuRoleList struct {
	BaseResponse
	Data []MenuRoleBrief `json:"data"`
}
