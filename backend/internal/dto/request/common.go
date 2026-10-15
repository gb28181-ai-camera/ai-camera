package request

// LoginReq 登录请求。注意 JSON 字段为 user_name（与前端表单保持一致）。
type LoginReq struct {
	Username string `json:"user_name" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"admin123"`
}

// PageReq 分页查询基类，被所有 PageXxxReq 嵌入。
type PageReq struct {
	Page     int `form:"page" json:"page" example:"1"`
	PageSize int `form:"page_size" json:"page_size" example:"20"`
}

// Normalize 归一化分页参数：页码至少为 1，每页 1~200 条。
func (p *PageReq) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 200 {
		p.PageSize = 200
	}
}

// Offset 返回 SQL 偏移量。
func (p *PageReq) Offset() int {
	p.Normalize()
	return (p.Page - 1) * p.PageSize
}

// BatchIDReq 批量操作请求。
type BatchIDReq struct {
	IDs []int64 `json:"ids" binding:"required,min=1"`
}

// DateRangeReq 时间范围查询参数，被需要时间筛选的 PageXxxReq 嵌入。
type DateRangeReq struct {
	StartDT string `form:"start_dt" json:"start_dt" example:"2026-01-01 00:00:00"`
	EndDT   string `form:"end_dt" json:"end_dt" example:"2026-12-31 23:59:59"`
}
