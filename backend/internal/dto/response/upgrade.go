package response

import "github.com/boticz/ai-camera-backend/internal/model"

// ════════════════════════════════════════════════════════════════
// 升级管理
// ════════════════════════════════════════════════════════════════

// ── 固件类型 ──

// PageFirmwareTypeResp 固件类型分页载荷。
type PageFirmwareTypeResp struct {
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Items    []model.FirmwareType `json:"items"`
}

// RPageFirmwareType 固件类型分页响应。
type RPageFirmwareType struct {
	BaseResponse
	Data PageFirmwareTypeResp `json:"data"`
}

// RFirmwareType 固件类型详情响应。
type RFirmwareType struct {
	BaseResponse
	Data model.FirmwareType `json:"data"`
}

// ── 固件 ──

// PageFirmwareResp 固件分页载荷。Items 中携带固件类型名称（firmware_type.name）。
type PageFirmwareResp struct {
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Items    []model.Firmware `json:"items"`
}

// RPageFirmware 固件分页响应。
type RPageFirmware struct {
	BaseResponse
	Data PageFirmwareResp `json:"data"`
}

// RFirmware 固件详情响应。
type RFirmware struct {
	BaseResponse
	Data model.Firmware `json:"data"`
}

// ── AI 模型 ──

// PageAIModelResp AI模型分页载荷。Items 中携带适用固件名称（firmware.name）。
type PageAIModelResp struct {
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Items    []model.AIModel `json:"items"`
}

// RPageAIModel AI模型分页响应。
type RPageAIModel struct {
	BaseResponse
	Data PageAIModelResp `json:"data"`
}

// RAIModel AI模型详情响应。
type RAIModel struct {
	BaseResponse
	Data model.AIModel `json:"data"`
}

// ── 升级计划 ──

// PageUpgradePlanResp 升级计划分页载荷。Items 中携带设备信息（device）。
type PageUpgradePlanResp struct {
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Items    []model.DeviceUpgradePlan `json:"items"`
}

// RPageUpgradePlan 升级计划分页响应。
type RPageUpgradePlan struct {
	BaseResponse
	Data PageUpgradePlanResp `json:"data"`
}

// ── 升级记录 ──

// PageUpgradeRecordResp 升级记录分页载荷。Items 中携带设备信息（device）。
type PageUpgradeRecordResp struct {
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
	Items    []model.DeviceUpgradeRecord `json:"items"`
}

// RPageUpgradeRecord 升级记录分页响应。
type RPageUpgradeRecord struct {
	BaseResponse
	Data PageUpgradeRecordResp `json:"data"`
}

// ── 升级下发结果 ──

// UpgradeDevicesResp 设备升级下发结果：立即升级台数与定时计划台数。
type UpgradeDevicesResp struct {
	NowCount  int64 `json:"now_count" example:"3"`
	PlanCount int64 `json:"plan_count" example:"2"`
}

// RUpgradeDevices 设备升级下发响应。
type RUpgradeDevices struct {
	BaseResponse
	Data UpgradeDevicesResp `json:"data"`
}
