package response

import "github.com/boticz/ai-camera-backend/internal/model"

// PageRiskRecordResp 风险记录分页载荷。
//
// Items 中的每条记录均预加载 RiskType / Device / Region，
// 前端可直接读取 risk_type.name、device.name、device.device_code、region.name。
type PageRiskRecordResp struct {
	Items    []model.RiskRecord `json:"items"`
	Total    int64              `json:"total" example:"128"`
	Page     int                `json:"page" example:"1"`
	PageSize int                `json:"page_size" example:"20"`
}

// RPageRiskRecord 风险记录分页响应。
type RPageRiskRecord struct {
	BaseResponse
	Data PageRiskRecordResp `json:"data"`
}

// RRiskRecord 风险记录详情响应。
type RRiskRecord struct {
	BaseResponse
	Data model.RiskRecord `json:"data"`
}
