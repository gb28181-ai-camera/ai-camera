package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PageOperationLogResp 操作日志分页载荷。
type PageOperationLogResp struct {
	PageResp
	Items []model.OperationLog `json:"items"`
}

// RPageOperationLog 操作日志分页响应。
type RPageOperationLog struct {
	BaseResponse
	Data PageOperationLogResp `json:"data"`
}

// ROperationLog 单条操作日志响应。
type ROperationLog struct {
	BaseResponse
	Data model.OperationLog `json:"data"`
}
