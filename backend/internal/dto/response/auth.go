package response

// LoginData 登录响应载荷。
//
// DataPerm / CrossDeptIDs 直接写入 JWT claims，供后续请求做行级数据过滤；
// Permissions 为 "METHOD:路径" 形式的接口权限集合，供前端按钮级鉴权使用。
type LoginData struct {
	Token        string   `json:"token"`
	UserID       int64    `json:"user_id"`
	Name         string   `json:"name"`
	DeptID       *int64   `json:"dept_id"`
	DataPerm     string   `json:"data_perm"`
	CrossDeptIDs string   `json:"cross_dept_ids"`
	Permissions  []string `json:"permissions"`
}

// RLogin 登录响应。
type RLogin struct {
	BaseResponse
	Data LoginData `json:"data"`
}
