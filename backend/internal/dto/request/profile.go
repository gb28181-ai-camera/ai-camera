package request

// UpdateProfileReq 更新个人信息请求（仅本人可维护的字段）。
//
// 字符串字段为空表示「不修改」，避免前端部分提交时误清空已有资料。
type UpdateProfileReq struct {
	Name   string `json:"name" binding:"omitempty,max=255" example:"张三"`
	Email  string `json:"email" binding:"omitempty,max=255" example:"zhangsan@example.com"`
	Phone  string `json:"phone" binding:"omitempty,max=50" example:"13800138000"`
	Avatar string `json:"avatar" example:"/uploads/avatar/1.png"`
	Remark string `json:"remark" binding:"omitempty,max=500" example:"系统管理员"`
}

// ChangePasswordReq 修改密码请求。需同时提供原密码与新密码。
type ChangePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required" example:"admin123"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64" example:"admin456"`
}

// GenerateApiKeyReq 生成 API Key 请求。Name 为便于识别的用途说明。
type GenerateApiKeyReq struct {
	Name string `json:"name" binding:"required,max=100" example:"设备上报密钥"`
}
