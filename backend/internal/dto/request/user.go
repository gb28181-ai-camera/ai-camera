package request

// PageUserReq 用户分页查询条件。
type PageUserReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"admin"`
	Active  *bool  `form:"active" json:"active" example:"true"`
	DeptID  *int64 `form:"dept_id" json:"dept_id" example:"1"`
	PostID  *int64 `form:"post_id" json:"post_id" example:"1"`
}

// CreateUserReq 创建用户请求。
type CreateUserReq struct {
	Name     string  `json:"name" binding:"required,max=255" example:"张三"`
	Username string  `json:"user_name" binding:"required,min=2,max=100" example:"zhangsan"`
	Password string  `json:"password" binding:"required,min=6,max=64" example:"admin123"`
	Email    string  `json:"email" binding:"omitempty,max=255" example:"zhangsan@example.com"`
	Phone    string  `json:"phone" binding:"omitempty,max=50" example:"13800138000"`
	Avatar   string  `json:"avatar" example:"/uploads/avatar/1.png"`
	DeptID   *int64  `json:"dept_id" example:"1"`
	PostID   *int64  `json:"post_id" example:"1"`
	Active   *bool   `json:"active" example:"true"`
	IsAdmin  *bool   `json:"is_admin" example:"false"`
	Remark   string  `json:"remark" binding:"omitempty,max=500" example:"新入职"`
	RoleIDs  []int64 `json:"role_ids" example:"1,2"`
}

// UpdateUserReq 更新用户请求。
//
// 字符串字段为空表示不修改；DeptID / PostID / Active / IsAdmin 为 nil 表示不修改。
// Password 为空表示不修改密码（重置密码请使用独立的 reset-password 接口）。
// UserName（登录名）是 Casbin 的 sub 主体，创建后不可修改，传入将被忽略。
type UpdateUserReq struct {
	Name     string  `json:"name" binding:"omitempty,max=255" example:"张三"`
	Username string  `json:"user_name" binding:"omitempty,min=2,max=100" example:"zhangsan"`
	Password string  `json:"password" binding:"omitempty,min=6,max=64" example:"admin123"`
	Email    string  `json:"email" binding:"omitempty,max=255" example:"zhangsan@example.com"`
	Phone    string  `json:"phone" binding:"omitempty,max=50" example:"13800138000"`
	Avatar   string  `json:"avatar" example:"/uploads/avatar/1.png"`
	DeptID   *int64  `json:"dept_id" example:"1"`
	PostID   *int64  `json:"post_id" example:"1"`
	Active   *bool   `json:"active" example:"true"`
	IsAdmin  *bool   `json:"is_admin" example:"false"`
	Remark   string  `json:"remark" binding:"omitempty,max=500" example:"调岗"`
	RoleIDs  []int64 `json:"role_ids" example:"1,2"`
}

// ResetPasswordReq 重置用户密码请求（管理员操作，无需原密码）。
type ResetPasswordReq struct {
	NewPassword string `json:"new_password" binding:"required,min=6,max=64" example:"admin456"`
}
