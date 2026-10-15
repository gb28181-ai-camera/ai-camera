package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PageUserResp 用户分页载荷。
type PageUserResp struct {
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Items    []model.User `json:"items" filter:"-"`
}

// RPageUser 用户分页响应。
type RPageUser struct {
	BaseResponse
	Data PageUserResp `json:"data"`
}

// RUserList 用户简单列表响应（无分页）。
type RUserList struct {
	BaseResponse
	Data []model.User `json:"data"`
}
