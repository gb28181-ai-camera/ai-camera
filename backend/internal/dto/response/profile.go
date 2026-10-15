package response

import "github.com/boticz/ai-camera-backend/internal/model"

// RUser 单条用户详情响应（个人中心与用户管理共用）。
type RUser struct {
	BaseResponse
	Data model.User `json:"data"`
}

// 说明：个人中心菜单树（GET /web/profile/menus）复用 response.RMenuList
// （定义在 menu.go），与系统菜单树接口保持同一响应类型。

// GenerateApiKeyResp 生成 API Key 的结果。
// FullKey 为完整密钥，**仅在本次响应中返回一次**，服务端只保存其 SHA256 摘要。
type GenerateApiKeyResp struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	KeyPrefix string `json:"key_prefix"`
	FullKey   string `json:"full_key"`
}

// RGenerateApiKey 生成 API Key 响应。
type RGenerateApiKey struct {
	BaseResponse
	Data GenerateApiKeyResp `json:"data"`
}

// RApiKeyList API Key 列表响应（KeyHash 不返回）。
type RApiKeyList struct {
	BaseResponse
	Data []model.ApiKey `json:"data"`
}
