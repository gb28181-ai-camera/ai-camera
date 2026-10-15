package request

// ════════════════════════════════════════════════════════════════
// 告警设置：设备告警设置 / 风险通知设置 / 第三方配置（个推、短信）
// ════════════════════════════════════════════════════════════════

// ── 设备告警设置 ──

// PageAlarmSettingReq 设备告警设置分页查询参数。
// 仅返回当前登录用户的配置，无需传 user_id。
type PageAlarmSettingReq struct {
	PageReq
	Keyword     string `form:"keyword" json:"keyword" example:"电量告警"`
	AlarmTypeID *int64 `form:"alarm_type_id" json:"alarm_type_id" example:"1"`
}

// CreateAlarmSettingReq 创建设备告警设置。
type CreateAlarmSettingReq struct {
	AlarmTypeID   int64  `json:"alarm_type_id" binding:"required" example:"1"`
	Active        *bool  `json:"active" example:"true"`
	NotifyType    string `json:"notify_type" example:"app"`
	NotifyFreq    int    `json:"notify_freq" example:"60"`
	NotifyUserIDs string `json:"notify_user_ids" example:"3,4"`
	Threshold     string `json:"threshold" example:"20"`
	Remark        string `json:"remark" example:"电量低于阈值时提醒"`
}

// UpdateAlarmSettingReq 更新设备告警设置。
type UpdateAlarmSettingReq struct {
	AlarmTypeID   int64  `json:"alarm_type_id" binding:"required" example:"1"`
	Active        *bool  `json:"active" example:"true"`
	NotifyType    string `json:"notify_type" example:"app"`
	NotifyFreq    int    `json:"notify_freq" example:"60"`
	NotifyUserIDs string `json:"notify_user_ids" example:"3,4"`
	Threshold     string `json:"threshold" example:"20"`
	Remark        string `json:"remark" example:"电量低于阈值时提醒"`
}

// ── 风险通知设置 ──

// PageRiskNotifySettingReq 风险通知设置分页查询参数。仅返回当前登录用户的配置。
type PageRiskNotifySettingReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"未佩戴安全帽"`
}

// CreateRiskNotifySettingReq 创建风险通知设置。
type CreateRiskNotifySettingReq struct {
	RiskTypeID    int64  `json:"risk_type_id" binding:"required" example:"2"`
	Active        *bool  `json:"active" example:"true"`
	NotifyType    string `json:"notify_type" example:"app"`
	NotifyFreq    int    `json:"notify_freq" example:"0"`
	NotifyUserIDs string `json:"notify_user_ids" example:"3,4"`
	Remark        string `json:"remark" example:"高风险实时通知"`
}

// UpdateRiskNotifySettingReq 更新风险通知设置。
type UpdateRiskNotifySettingReq struct {
	RiskTypeID    int64  `json:"risk_type_id" binding:"required" example:"2"`
	Active        *bool  `json:"active" example:"true"`
	NotifyType    string `json:"notify_type" example:"app"`
	NotifyFreq    int    `json:"notify_freq" example:"0"`
	NotifyUserIDs string `json:"notify_user_ids" example:"3,4"`
	Remark        string `json:"remark" example:"高风险实时通知"`
}

// ── 第三方配置 ──

// UpdateGetuiReq 更新个推配置。敏感项为空或为脱敏串时跳过，不覆盖原值。
type UpdateGetuiReq struct {
	AppID        string `json:"app_id" example:"abc123"`
	AppKey       string `json:"app_key" example:"key123"`
	AppSecret    string `json:"app_secret" example:"secr****"`
	MasterSecret string `json:"master_secret" example:"mast****"`
}

// UpdateSmsReq 更新腾讯云短信配置。敏感项为空或为脱敏串时跳过，不覆盖原值。
type UpdateSmsReq struct {
	SecretID  string `json:"secret_id" example:"AKIDxxxx"`
	SecretKey string `json:"secret_key" example:"secr****"`
	SdkAppID  string `json:"sdk_app_id" example:"1400000000"`
	SignName  string `json:"sign_name" example:"AI摄像头"`
}
