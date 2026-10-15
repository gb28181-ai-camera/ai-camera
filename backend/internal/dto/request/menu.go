package request

// CreateMenuReq 新增菜单请求。Type 取值 directory/menu/button。
type CreateMenuReq struct {
	Name           string `json:"name" binding:"required" example:"用户管理"`
	ParentID       *int64 `json:"parent_id" example:"1"`
	Sequence       int    `json:"sequence" example:"10"`
	Path           string `json:"path" example:"/system/user"`
	Icon           string `json:"icon" example:"user"`
	Component      string `json:"component" example:"system/user/index"`
	Type           string `json:"type" binding:"required,oneof=directory menu button" example:"menu"`
	Platform       string `json:"platform" binding:"omitempty,oneof=pc mobile" example:"pc"`
	PermissionCode string `json:"permission_code" example:"system:user:list"`
	Description    string `json:"description" example:"用户管理菜单"`
	Active         *bool  `json:"active" example:"true"`
	Hidden         *bool  `json:"hidden" example:"false"`
}

// UpdateMenuReq 更新菜单请求。字段与新增一致，为空（未传）的字段不覆盖原值。
type UpdateMenuReq struct {
	Name           string `json:"name" example:"用户管理"`
	ParentID       *int64 `json:"parent_id" example:"1"`
	Sequence       int    `json:"sequence" example:"10"`
	Path           string `json:"path" example:"/system/user"`
	Icon           string `json:"icon" example:"user"`
	Component      string `json:"component" example:"system/user/index"`
	Type           string `json:"type" binding:"omitempty,oneof=directory menu button" example:"menu"`
	Platform       string `json:"platform" binding:"omitempty,oneof=pc mobile" example:"pc"`
	PermissionCode string `json:"permission_code" example:"system:user:list"`
	Description    string `json:"description" example:"用户管理菜单"`
	Active         *bool  `json:"active" example:"true"`
	Hidden         *bool  `json:"hidden" example:"false"`
}

// CreateMenuAPIReq 新增菜单接口请求。菜单接口是 Casbin p 策略的唯一来源，
// 新增后会自动为拥有该菜单的角色重建策略。
type CreateMenuAPIReq struct {
	Method      string `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH" example:"GET"`
	Path        string `json:"path" binding:"required" example:"/web/system/users"`
	Description string `json:"description" example:"用户列表"`
}
