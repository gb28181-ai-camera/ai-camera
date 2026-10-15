package request

// CreateDeptReq 新增部门请求。Code 是 Casbin g2 主体，建议全局唯一。
// Level / Path 由后端依据父部门自动维护，请求中无需传入。
type CreateDeptReq struct {
	Name        string `json:"name" binding:"required" example:"生产部"`
	Code        string `json:"code" example:"PROD"`
	ParentID    *int64 `json:"parent_id" example:"1"`
	Sequence    int    `json:"sequence" example:"10"`
	LeaderID    *int64 `json:"leader_id" example:"1"`
	Description string `json:"description" example:"生产管理部门"`
	Active      *bool  `json:"active" example:"true"`
}

// UpdateDeptReq 更新部门请求。为空（未传）的字段不覆盖原值；
// 变更上级部门时后端会同步重算本部门与全部下级的 level / path。
type UpdateDeptReq struct {
	Name        string `json:"name" example:"生产部"`
	Code        string `json:"code" example:"PROD"`
	ParentID    *int64 `json:"parent_id" example:"1"`
	Sequence    int    `json:"sequence" example:"10"`
	LeaderID    *int64 `json:"leader_id" example:"1"`
	Description string `json:"description" example:"生产管理部门"`
	Active      *bool  `json:"active" example:"true"`
}
