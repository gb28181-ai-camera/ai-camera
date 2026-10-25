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

// RiskSummaryResp 风险总览指标（AI风险台账顶部）。
type RiskSummaryResp struct {
	Total       int64 `json:"total" example:"1280"`
	Pending     int64 `json:"pending" example:"36"`
	Handled     int64 `json:"handled" example:"1244"`
	High        int64 `json:"high" example:"218"`
	Unread      int64 `json:"unread" example:"12"`
	AccurateYes int64 `json:"accurate_yes" example:"920"`
	AccurateNo  int64 `json:"accurate_no" example:"40"`
	Unconfirmed int64 `json:"unconfirmed" example:"320"`
	Today       int64 `json:"today" example:"18"`
}

// RRiskSummary 风险总览响应。
type RRiskSummary struct {
	BaseResponse
	Data RiskSummaryResp `json:"data"`
}

// RRiskRecord 风险记录详情响应。
type RRiskRecord struct {
	BaseResponse
	Data model.RiskRecord `json:"data"`
}
