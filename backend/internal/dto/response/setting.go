package response

import "github.com/boticz/ai-camera-backend/internal/model"

// ════════════════════════════════════════════════════════════════
// 告警设置
// ════════════════════════════════════════════════════════════════

// ── 设备告警设置 ──

// PageAlarmSettingResp 设备告警设置分页载荷。
// Items 中携带告警类型名称（alarm_type.name）。
type PageAlarmSettingResp struct {
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	Items    []model.AlarmSetting `json:"items"`
}

// RPageAlarmSetting 设备告警设置分页响应。
type RPageAlarmSetting struct {
	BaseResponse
	Data PageAlarmSettingResp `json:"data"`
}

// RAlarmSetting 设备告警设置详情响应。
type RAlarmSetting struct {
	BaseResponse
	Data model.AlarmSetting `json:"data"`
}

// ── 风险通知设置 ──

// PageRiskNotifySettingResp 风险通知设置分页载荷。
// Items 中携带风险类型名称（risk_type.name）。
type PageRiskNotifySettingResp struct {
	Total    int64                     `json:"total"`
	Page     int                       `json:"page"`
	PageSize int                       `json:"page_size"`
	Items    []model.RiskNotifySetting `json:"items"`
}

// RPageRiskNotifySetting 风险通知设置分页响应。
type RPageRiskNotifySetting struct {
	BaseResponse
	Data PageRiskNotifySettingResp `json:"data"`
}

// RRiskNotifySetting 风险通知设置详情响应。
type RRiskNotifySetting struct {
	BaseResponse
	Data model.RiskNotifySetting `json:"data"`
}

// ── 第三方配置 ──

// GetuiConfigResp 个推配置。敏感项（app_secret / master_secret）返回脱敏值。
type GetuiConfigResp struct {
	AppID        string `json:"app_id" example:"abc123"`
	AppKey       string `json:"app_key" example:"key123"`
	AppSecret    string `json:"app_secret" example:"secr****"`
	MasterSecret string `json:"master_secret" example:"mast****"`
}

// RGetuiConfig 个推配置响应。
type RGetuiConfig struct {
	BaseResponse
	Data GetuiConfigResp `json:"data"`
}

// SmsConfigResp 腾讯云短信配置。敏感项（secret_key）返回脱敏值。
type SmsConfigResp struct {
	SecretID  string `json:"secret_id" example:"AKIDxxxx"`
	SecretKey string `json:"secret_key" example:"secr****"`
	SdkAppID  string `json:"sdk_app_id" example:"1400000000"`
	SignName  string `json:"sign_name" example:"AI摄像头"`
}

// RSmsConfig 腾讯云短信配置响应。
type RSmsConfig struct {
	BaseResponse
	Data SmsConfigResp `json:"data"`
}
