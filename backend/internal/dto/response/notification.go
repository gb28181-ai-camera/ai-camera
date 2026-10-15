package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PageNotificationResp 站内通知分页载荷。
type PageNotificationResp struct {
	PageResp
	Items []model.Notification `json:"items"`
}

// RPageNotification 站内通知分页响应。
type RPageNotification struct {
	BaseResponse
	Data PageNotificationResp `json:"data"`
}

// RUnreadCount 未读数量响应。
type RUnreadCount struct {
	BaseResponse
	Data int64 `json:"data"`
}
