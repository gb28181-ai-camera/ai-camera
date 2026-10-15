package response

import "github.com/boticz/ai-camera-backend/internal/model"

// ════════════════════════════════════════════════════════════════
// 基础信息管理：区域 / 风险类型 / 告警类型
// ════════════════════════════════════════════════════════════════

// ── 区域 ────────────────────────────────────────────────────────

// PageRegionResp 区域分页载荷。
type PageRegionResp struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Items    []model.Region `json:"items"`
}

// RPageRegion 区域分页响应。
type RPageRegion struct {
	BaseResponse
	Data PageRegionResp `json:"data"`
}

// RRegion 区域详情响应。
type RRegion struct {
	BaseResponse
	Data model.Region `json:"data"`
}

// ── 风险类型 ────────────────────────────────────────────────────

// PageRiskTypeResp 风险类型分页载荷。
type PageRiskTypeResp struct {
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Items    []model.RiskType `json:"items"`
}

// RPageRiskType 风险类型分页响应。
type RPageRiskType struct {
	BaseResponse
	Data PageRiskTypeResp `json:"data"`
}

// RRiskType 风险类型详情响应。
type RRiskType struct {
	BaseResponse
	Data model.RiskType `json:"data"`
}

// ── 告警类型 ────────────────────────────────────────────────────

// PageAlarmTypeResp 告警类型分页载荷。
type PageAlarmTypeResp struct {
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Items    []model.AlarmType `json:"items"`
}

// RPageAlarmType 告警类型分页响应。
type RPageAlarmType struct {
	BaseResponse
	Data PageAlarmTypeResp `json:"data"`
}

// RAlarmType 告警类型详情响应。
type RAlarmType struct {
	BaseResponse
	Data model.AlarmType `json:"data"`
}
