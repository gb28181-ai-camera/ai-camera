package request

// PageRoleReq 角色分页查询条件。
type PageRoleReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"管理"`
	Active  *bool  `form:"active" json:"active" example:"true"`
}

// CreateRoleReq 创建角色请求。
type CreateRoleReq struct {
	Name           string  `json:"name" binding:"required,max=255" example:"系统管理员"`
	Code           string  `json:"code" binding:"required,max=50" example:"admin"`
	Sequence       int     `json:"sequence" example:"1"`
	Description    string  `json:"description" example:"拥有全部权限"`
	DataPermission string  `json:"data_permission" example:"all"`
	CrossDeptIDs   string  `json:"cross_dept_ids" example:"2,3"`
	Active         *bool   `json:"active" example:"true"`
	MenuIDs        []int64 `json:"menu_ids" example:"1,2,3"`
}

// UpdateRoleReq 更新角色请求。字符串为空表示不修改；MenuIDs 为 nil 表示不调整菜单授权。
type UpdateRoleReq struct {
	Name           string  `json:"name" binding:"omitempty,max=255" example:"系统管理员"`
	Code           string  `json:"code" binding:"omitempty,max=50" example:"admin"`
	Sequence       int     `json:"sequence" example:"1"`
	Description    string  `json:"description" example:"拥有全部权限"`
	DataPermission string  `json:"data_permission" example:"all"`
	CrossDeptIDs   string  `json:"cross_dept_ids" example:"2,3"`
	Active         *bool   `json:"active" example:"true"`
	MenuIDs        []int64 `json:"menu_ids" example:"1,2,3"`
}

// AssignIDsReq 角色关联分配请求，用于菜单/部门/岗位/用户的授权。
// 传空数组表示清空该角色的全部授权。
type AssignIDsReq struct {
	IDs []int64 `json:"ids" example:"1,2,3"`
}
