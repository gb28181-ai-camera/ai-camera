package request

// PagePostReq 岗位分页查询请求。Keyword 模糊匹配岗位名称/编码。
type PagePostReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"运维"`
	Active  *bool  `form:"active" json:"active" example:"true"`
}

// CreatePostReq 新增岗位请求。Code 是 Casbin g3 主体，必须唯一。
type CreatePostReq struct {
	Name        string  `json:"name" binding:"required" example:"巡检员"`
	Code        string  `json:"code" binding:"required" example:"INSPECTOR"`
	Sequence    int     `json:"sequence" example:"10"`
	Description string  `json:"description" example:"负责设备巡检"`
	Active      *bool   `json:"active" example:"true"`
	RoleIDs     []int64 `json:"role_ids" example:"1,2"`
}

// UpdatePostReq 更新岗位请求。为空（未传）的字段不覆盖原值；
// RoleIDs 为 nil 表示不调整岗位角色，传空数组表示清空。
type UpdatePostReq struct {
	Name        string  `json:"name" example:"巡检员"`
	Code        string  `json:"code" example:"INSPECTOR"`
	Sequence    int     `json:"sequence" example:"10"`
	Description string  `json:"description" example:"负责设备巡检"`
	Active      *bool   `json:"active" example:"true"`
	RoleIDs     []int64 `json:"role_ids" example:"1,2"`
}
