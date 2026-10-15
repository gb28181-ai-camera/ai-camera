package request

// ════════════════════════════════════════════════════════════════
// 基础信息管理：区域 / 风险类型 / 告警类型
// ════════════════════════════════════════════════════════════════

// ── 区域 ────────────────────────────────────────────────────────

// PageRegionReq 区域分页查询。Keyword 模糊匹配区域名称与区域编码。
type PageRegionReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"园区"`
}

// CreateRegionReq 新增区域。
type CreateRegionReq struct {
	Name     string `json:"name" binding:"required" example:"东区厂房"`
	Code     string `json:"code" example:"REGION_EAST"`
	Sequence int    `json:"sequence" example:"10"`
	Active   *bool  `json:"active" example:"true"`
	Remark   string `json:"remark" example:"一楼生产车间"`
}

// UpdateRegionReq 修改区域。
type UpdateRegionReq struct {
	Name     string `json:"name" binding:"required" example:"东区厂房"`
	Code     string `json:"code" example:"REGION_EAST"`
	Sequence int    `json:"sequence" example:"10"`
	Active   *bool  `json:"active" example:"true"`
	Remark   string `json:"remark" example:"一楼生产车间"`
}

// ── 风险类型 ────────────────────────────────────────────────────

// PageRiskTypeReq 风险类型分页查询。Keyword 模糊匹配名称与编码，Level 精确过滤等级。
type PageRiskTypeReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"安全帽"`
	Level   string `form:"level" json:"level" example:"high"`
}

// CreateRiskTypeReq 新增风险类型。
type CreateRiskTypeReq struct {
	Name        string `json:"name" binding:"required" example:"未佩戴安全帽"`
	Code        string `json:"code" example:"RISK_HELMET"`
	Level       string `json:"level" binding:"omitempty,oneof=high medium low" example:"high"`
	Sequence    int    `json:"sequence" example:"10"`
	Active      *bool  `json:"active" example:"true"`
	Description string `json:"description" example:"检测到人员未佩戴安全帽"`
}

// UpdateRiskTypeReq 修改风险类型。
type UpdateRiskTypeReq struct {
	Name        string `json:"name" binding:"required" example:"未佩戴安全帽"`
	Code        string `json:"code" example:"RISK_HELMET"`
	Level       string `json:"level" binding:"omitempty,oneof=high medium low" example:"high"`
	Sequence    int    `json:"sequence" example:"10"`
	Active      *bool  `json:"active" example:"true"`
	Description string `json:"description" example:"检测到人员未佩戴安全帽"`
}

// ── 告警类型 ────────────────────────────────────────────────────

// PageAlarmTypeReq 告警类型分页查询。Keyword 模糊匹配名称与编码，Level 精确过滤等级。
type PageAlarmTypeReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"离线"`
	Level   string `form:"level" json:"level" example:"high"`
}

// CreateAlarmTypeReq 新增告警类型。
type CreateAlarmTypeReq struct {
	Name        string `json:"name" binding:"required" example:"离线告警"`
	Code        string `json:"code" example:"offline"`
	Level       string `json:"level" binding:"omitempty,oneof=high medium low" example:"high"`
	Sequence    int    `json:"sequence" example:"10"`
	Active      *bool  `json:"active" example:"true"`
	Description string `json:"description" example:"设备超过心跳间隔未上报"`
}

// UpdateAlarmTypeReq 修改告警类型。
type UpdateAlarmTypeReq struct {
	Name        string `json:"name" binding:"required" example:"离线告警"`
	Code        string `json:"code" example:"offline"`
	Level       string `json:"level" binding:"omitempty,oneof=high medium low" example:"high"`
	Sequence    int    `json:"sequence" example:"10"`
	Active      *bool  `json:"active" example:"true"`
	Description string `json:"description" example:"设备超过心跳间隔未上报"`
}
