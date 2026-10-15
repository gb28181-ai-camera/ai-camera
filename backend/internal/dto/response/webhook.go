package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PageWebhookResp WebHook 配置分页载荷。
type PageWebhookResp struct {
	Items    []model.WebhookConfig `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// RPageWebhook WebHook 配置分页响应。
type RPageWebhook struct {
	BaseResponse
	Data PageWebhookResp `json:"data"`
}

// RWebhook WebHook 单条响应。
type RWebhook struct {
	BaseResponse
	Data model.WebhookConfig `json:"data"`
}

// WebhookTestResp 测试推送结果。
type WebhookTestResp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// RWebhookTest 测试推送响应。
type RWebhookTest struct {
	BaseResponse
	Data WebhookTestResp `json:"data"`
}

// WebhookPlatformItem 平台选项。
type WebhookPlatformItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// RWebhookPlatformList 平台选项列表响应。
type RWebhookPlatformList struct {
	BaseResponse
	Data []WebhookPlatformItem `json:"data"`
}
