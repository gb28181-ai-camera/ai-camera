package request

// CreateConfigReq 新增系统参数请求。Key 对应 model.SystemConfig.Code（JSON 字段名 key）。
type CreateConfigReq struct {
	Name        string `json:"name" binding:"required" example:"系统名称"`
	Key         string `json:"key" binding:"required" example:"system.name"`
	Value       string `json:"value" example:"AI Camera"`
	GroupName   string `json:"group_name" example:"system"`
	DataType    string `json:"data_type" binding:"omitempty,oneof=string int bool json" example:"string"`
	Sequence    int    `json:"sequence" example:"10"`
	Description string `json:"description" example:"登录页与页头展示名称"`
}

// UpdateConfigReq 更新系统参数请求。为空（未传）的字段不覆盖原值。
type UpdateConfigReq struct {
	Name        string `json:"name" example:"系统名称"`
	Key         string `json:"key" example:"system.name"`
	Value       string `json:"value" example:"AI Camera"`
	GroupName   string `json:"group_name" example:"system"`
	DataType    string `json:"data_type" binding:"omitempty,oneof=string int bool json" example:"string"`
	Sequence    int    `json:"sequence" example:"10"`
	Description string `json:"description" example:"登录页与页头展示名称"`
}
