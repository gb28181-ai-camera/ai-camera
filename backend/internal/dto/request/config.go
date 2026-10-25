package request

// CreateConfigReq 新增系统参数请求。Key 对应 model.SystemConfig.Code（JSON 字段名 key）。
//
// DataType 取值为 string/int/integer/float/bool/boolean/json/select；
// integer/boolean 分别归一为 int/bool 后落库，前端按归一后的类型渲染。
// Schema 为参数值的 JSON Schema 描述，前端据此渲染表单。
type CreateConfigReq struct {
	Name        string `json:"name" binding:"required" example:"系统名称"`
	Key         string `json:"key" binding:"required" example:"system.name"`
	Value       string `json:"value" example:"AI Camera"`
	GroupName   string `json:"group_name" example:"system"`
	DataType    string `json:"data_type" binding:"omitempty,oneof=string int integer float bool boolean json select" example:"string"`
	Schema      string `json:"schema" example:"{\"type\":\"string\",\"maxLength\":64}"`
	Active      *bool  `json:"active" example:"true"`
	Sequence    int    `json:"sequence" example:"10"`
	Description string `json:"description" example:"登录页与页头展示名称"`
}

// UpdateConfigReq 更新系统参数请求。为空（未传）的字段不覆盖原值；Active 为 nil 时不修改启用状态。
type UpdateConfigReq struct {
	Name        string `json:"name" example:"系统名称"`
	Key         string `json:"key" example:"system.name"`
	Value       string `json:"value" example:"AI Camera"`
	GroupName   string `json:"group_name" example:"system"`
	DataType    string `json:"data_type" binding:"omitempty,oneof=string int integer float bool boolean json select" example:"string"`
	Schema      string `json:"schema" example:"{\"type\":\"string\",\"maxLength\":64}"`
	Active      *bool  `json:"active" example:"true"`
	Sequence    int    `json:"sequence" example:"10"`
	Description string `json:"description" example:"登录页与页头展示名称"`
}
