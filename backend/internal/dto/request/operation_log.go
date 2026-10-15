package request

// PageOperationLogReq 操作日志分页查询请求。
// Module 精确匹配系统模块，Operator 模糊匹配操作人员，时间范围匹配操作时间。
type PageOperationLogReq struct {
	PageReq
	Module   string `form:"module" json:"module" example:"system"`
	Operator string `form:"operator" json:"operator" example:"admin"`
	Action   string `form:"action" json:"action" example:"新增"`
	StartDT  string `form:"start_dt" json:"start_dt" example:"2026-01-01 00:00:00"`
	EndDT    string `form:"end_dt" json:"end_dt" example:"2026-12-31 23:59:59"`
}
