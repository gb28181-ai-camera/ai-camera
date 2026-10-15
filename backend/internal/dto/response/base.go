package response

// BaseResponse 所有响应包装类型的基类。
// Swagger 注解中以具体子类型（如 RPageUser）作为返回结构，前端据此生成强类型。
type BaseResponse struct {
	Code    int         `json:"code" example:"200"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResp 分页响应基类。
//
// 业务代码请使用类型化的 PageXxxResp（如 PageUserResp），
// 直接使用本类型会使前端生成 any 类型，丧失类型安全。
type PageResp struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    interface{} `json:"items"`
}

// Fill 填充分页元信息。
func (p *PageResp) Fill(page, pageSize int, total int64, items interface{}) {
	p.Page = page
	p.PageSize = pageSize
	p.Total = total
	p.Items = items
}

// UploadResp 文件上传响应。
type UploadResp struct {
	URL      string `json:"url"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
}

// RUpload 文件上传响应包装。
type RUpload struct {
	BaseResponse
	Data UploadResp `json:"data"`
}
