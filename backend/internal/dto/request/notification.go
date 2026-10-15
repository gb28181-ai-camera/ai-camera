package request

// PageNotificationReq 站内通知分页查询请求。
// 通知只能查询当前登录用户自己的数据，user_id 由后端从 JWT 注入。
type PageNotificationReq struct {
	PageReq
	Category string `form:"category" json:"category" binding:"omitempty,oneof=alarm risk inspect" example:"alarm"`
	Read     *bool  `form:"read" json:"read" example:"false"`
}
