package response

import "github.com/boticz/ai-camera-backend/internal/model"

// RSystemConfigList 系统参数列表响应（非分页）。
type RSystemConfigList struct {
	BaseResponse
	Data []model.SystemConfig `json:"data"`
}

// RSystemConfig 单条系统参数响应。
type RSystemConfig struct {
	BaseResponse
	Data model.SystemConfig `json:"data"`
}
