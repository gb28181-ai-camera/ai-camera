package response

import "github.com/boticz/ai-camera-backend/internal/model"

// ════════════════════════════════════════════════════════════════
// 资源管理：设备 / 设备模式 / 物联网卡 / 换卡记录 / 语音配置 / 设备历史
// ════════════════════════════════════════════════════════════════

// ── 设备 ──────────────────────────────────────────────────────

// PageDeviceResp 设备分页载荷。
type PageDeviceResp struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Items    []model.Device `json:"items"`
}

// RPageDevice 设备分页响应。
type RPageDevice struct {
	BaseResponse
	Data PageDeviceResp `json:"data"`
}

// RDevice 设备详情响应。
type RDevice struct {
	BaseResponse
	Data model.Device `json:"data"`
}

// DeviceSyncTimeResp 设备时间同步结果。
type DeviceSyncTimeResp struct {
	SyncTime string `json:"sync_time" example:"2026-09-21 10:00:00"`
}

// RSyncTime 时间同步响应。
type RSyncTime struct {
	BaseResponse
	Data DeviceSyncTimeResp `json:"data"`
}

// ── 设备模式 ──────────────────────────────────────────────────

// PageDeviceModeResp 设备模式分页载荷。
type PageDeviceModeResp struct {
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Items    []model.DeviceMode `json:"items"`
}

// RPageDeviceMode 设备模式分页响应。
type RPageDeviceMode struct {
	BaseResponse
	Data PageDeviceModeResp `json:"data"`
}

// RDeviceModeList 设备模式简单列表响应。
type RDeviceModeList struct {
	BaseResponse
	Data []model.DeviceMode `json:"data"`
}

// RDeviceMode 设备模式详情响应。
type RDeviceMode struct {
	BaseResponse
	Data model.DeviceMode `json:"data"`
}

// ── 物联网卡 ──────────────────────────────────────────────────

// PageIotCardResp 物联网卡分页载荷。
type PageIotCardResp struct {
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Items    []model.IotCard `json:"items"`
}

// RPageIotCard 物联网卡分页响应。
type RPageIotCard struct {
	BaseResponse
	Data PageIotCardResp `json:"data"`
}

// RIotCardList 物联网卡简单列表响应。
type RIotCardList struct {
	BaseResponse
	Data []model.IotCard `json:"data"`
}

// RIotCard 物联网卡详情响应。
type RIotCard struct {
	BaseResponse
	Data model.IotCard `json:"data"`
}

// ── 换卡记录 ──────────────────────────────────────────────────

// PageIotCardReplaceLogResp 换卡记录分页载荷。
type PageIotCardReplaceLogResp struct {
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Items    []model.IotCardReplaceLog `json:"items"`
}

// RPageIotCardReplaceLog 换卡记录分页响应。
type RPageIotCardReplaceLog struct {
	BaseResponse
	Data PageIotCardReplaceLogResp `json:"data"`
}

// RIotCardReplaceLog 换卡记录详情响应（换卡结果）。
type RIotCardReplaceLog struct {
	BaseResponse
	Data model.IotCardReplaceLog `json:"data"`
}

// ── 语音配置 ──────────────────────────────────────────────────

// PageVoiceConfigResp 语音配置分页载荷。
type PageVoiceConfigResp struct {
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Items    []model.VoiceConfig `json:"items"`
}

// RPageVoiceConfig 语音配置分页响应。
type RPageVoiceConfig struct {
	BaseResponse
	Data PageVoiceConfigResp `json:"data"`
}

// RVoiceConfigList 语音配置简单列表响应。
type RVoiceConfigList struct {
	BaseResponse
	Data []model.VoiceConfig `json:"data"`
}

// RVoiceConfig 语音配置详情响应。
type RVoiceConfig struct {
	BaseResponse
	Data model.VoiceConfig `json:"data"`
}

// ── 设备历史 ──────────────────────────────────────────────────

// PageDeviceHistoryResp 设备历史分页载荷。
type PageDeviceHistoryResp struct {
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
	Items    []model.DeviceHistory `json:"items"`
}

// RPageDeviceHistory 设备历史分页响应。
type RPageDeviceHistory struct {
	BaseResponse
	Data PageDeviceHistoryResp `json:"data"`
}

// DeviceChartPoint 曲线数据点。
//
// Status 曲线取值：online=1、offline=0；
// Voltage 曲线为区间内电压均值（单位 V）。
type DeviceChartPoint struct {
	Time  string  `json:"time" example:"2026-09-21 10:00"`
	Value float64 `json:"value" example:"88"`
}

// DeviceStatusSummary 运行状态时长汇总（单位：分钟）。
type DeviceStatusSummary struct {
	OnlineMinutes  int64 `json:"online_minutes" example:"1440"`
	OfflineMinutes int64 `json:"offline_minutes" example:"0"`
}

// DeviceHistoryChartResp 设备历史曲线聚合结果。
//
// granularity=hour 时按小时聚合（days<=7），否则按天聚合。
type DeviceHistoryChartResp struct {
	DeviceID      int64               `json:"device_id"`
	Days          int                 `json:"days"`
	Granularity   string              `json:"granularity" example:"hour"`
	BatteryPoints []DeviceChartPoint  `json:"battery_points"`
	VoltagePoints []DeviceChartPoint  `json:"voltage_points"`
	StoragePoints []DeviceChartPoint  `json:"storage_points"`
	StatusPoints  []DeviceChartPoint  `json:"status_points"`
	StatusSummary DeviceStatusSummary `json:"status_summary"`
}

// RDeviceHistoryChart 设备历史曲线响应。
type RDeviceHistoryChart struct {
	BaseResponse
	Data DeviceHistoryChartResp `json:"data"`
}
