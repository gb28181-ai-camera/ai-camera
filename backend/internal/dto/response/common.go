package response

// OptionItem 下拉选项通用结构。所有 /options 类接口统一返回该结构。
type OptionItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Code string `json:"code,omitempty"`
}

// ROptionList 选项列表响应。
type ROptionList struct {
	BaseResponse
	Data []OptionItem `json:"data"`
}

// RCount 计数响应（待处理数、未读数等）。
type RCount struct {
	BaseResponse
	Data int64 `json:"data"`
}

// RBool 布尔结果响应。
type RBool struct {
	BaseResponse
	Data bool `json:"data"`
}
