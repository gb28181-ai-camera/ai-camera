package request

// ════════════════════════════════════════════════════════════════
// IOT 管理：产品 / 服务组 / 服务项 / 设备账号 / 设备端 OTA & 激活
// ════════════════════════════════════════════════════════════════

// ── 产品 ──────────────────────────────────────────────────────

// PageIotProductReq 产品分页查询。
type PageIotProductReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"摄像头"`
	Active  *bool  `form:"active" json:"active"`
}

// CreateIotProductReq 新增产品。
type CreateIotProductReq struct {
	Name          string `json:"name" binding:"required" example:"4G 球机"`
	Code          string `json:"code" binding:"required" example:"CAM_4G_01"`
	Category      string `json:"category" example:"摄像头"`
	Description   string `json:"description"`
	AutoNetwork   *bool  `json:"auto_network" example:"true"`
	NetConfigMode string `json:"net_config_mode" example:"bluetooth"`
	AllowShare    *bool  `json:"allow_share" example:"true"`
	Sequence      int    `json:"sequence" example:"10"`
	Active        *bool  `json:"active" example:"true"`
}

// UpdateIotProductReq 更新产品。
//
// AutoNetwork / AllowShare / Active 用指针，以便区分「未传」与「显式 false」；
// 文本字段为空表示不修改（与项目内其它 Update 请求保持一致）。
type UpdateIotProductReq struct {
	Name          string `json:"name"`
	Code          string `json:"code"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	AutoNetwork   *bool  `json:"auto_network"`
	NetConfigMode string `json:"net_config_mode"`
	AllowShare    *bool  `json:"allow_share"`
	Sequence      int    `json:"sequence"`
	Active        *bool  `json:"active"`
}

// BindIotProductServiceGroupsReq 绑定产品可用服务组（整体替换）。
type BindIotProductServiceGroupsReq struct {
	ServiceGroupIDs []int64 `json:"service_group_ids"`
}

// ── 服务组 ────────────────────────────────────────────────────

// PageIotServiceGroupReq 服务组分页查询。
type PageIotServiceGroupReq struct {
	PageReq
	Keyword string `form:"keyword" json:"keyword" example:"GB28181"`
	Type    string `form:"type" json:"type" example:"gb28181"`
	Active  *bool  `form:"active" json:"active"`
}

// CreateIotServiceGroupReq 新增服务组。
//
// 不接收 username/password：凭据由平台自动生成（避免人工设置弱口令，
// 也避免前端在创建表单里看到明文密码）。
type CreateIotServiceGroupReq struct {
	Name        string `json:"name" binding:"required" example:"GB28181 服务组"`
	Code        string `json:"code" binding:"required" example:"GB_GROUP_01"`
	Type        string `json:"type" binding:"required" example:"gb28181"`
	Description string `json:"description"`
	Active      *bool  `json:"active" example:"true"`
}

// UpdateIotServiceGroupReq 更新服务组。
//
// Password 为空或含 **** 时保持原密码（复用 utils.ApplyMaskedUpdate）。
// 常规轮换请使用 reset-credentials，它会同时更换用户名与密码。
type UpdateIotServiceGroupReq struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Password    string `json:"password" example:"****"`
	Active      *bool  `json:"active"`
}

// IotServiceItemReq 单个服务项。
type IotServiceItemReq struct {
	Protocol string `json:"protocol" example:"SIP"`
	Host     string `json:"host" example:"sip.example.com"`
	Port     int    `json:"port" example:"5060"`
	Path     string `json:"path"`
	Extra    string `json:"extra" example:"{\"transport\":\"UDP\"}"`
	Sequence int    `json:"sequence" example:"10"`
	Active   *bool  `json:"active" example:"true"`
}

// ReplaceIotServiceItemsReq 整体替换服务组的服务项。
//
// 语义是替换而非增量：未列出的协议会被删除，便于前端用一张表格提交全量配置。
type ReplaceIotServiceItemsReq struct {
	Items []IotServiceItemReq `json:"items"`
}

// ── 设备与账号 ────────────────────────────────────────────────

// PageIotDeviceReq IOT 设备分页查询（数据源仍是 ac_device）。
type PageIotDeviceReq struct {
	PageReq
	Keyword   string `form:"keyword" json:"keyword" example:"CAM"`
	ProductID *int64 `form:"product_id" json:"product_id"`
	Status    string `form:"status" json:"status" example:"online"`
}

// CreateIotDeviceAccountReq 新增设备账号。
//
// Role 缺省为 shared（分享账号）；owner 只应在设备未激活时使用，
// 常规的主账号创建走 /devices/:id/activate 或设备端 /iot/activate。
// Password 留空时由服务端生成 16 位随机密码并一次性返回。
type CreateIotDeviceAccountReq struct {
	Username   string `json:"username" example:"family01"`
	Password   string `json:"password"`
	Role       string `json:"role" example:"shared"`
	Nickname   string `json:"nickname" example:"家人账号"`
	Phone      string `json:"phone"`
	Status     string `json:"status" example:"active"`
	ShareToken string `json:"share_token"`
}

// UpdateIotDeviceAccountReq 更新设备账号。
//
// Role 不可修改：主账号与分享账号的权限边界不同，静默允许改角色
// 会让「分享」变成「主账号」，属于越权风险。密码为空则保持原值。
type UpdateIotDeviceAccountReq struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	Nickname   string `json:"nickname"`
	Phone      string `json:"phone"`
	Status     string `json:"status"`
	ShareToken string `json:"share_token"`
}

// UpdateIotDeviceProductReq 设置设备所属产品（product_id 为空表示解绑）。
type UpdateIotDeviceProductReq struct {
	ProductID *int64 `json:"product_id"`
}

// ── 设备端（无 JWT）──────────────────────────────────────────

// IotOTAReq 设备端 OTA 请求。client_version 可选，仅用于留痕。
type IotOTAReq struct {
	ClientVersion string `json:"client_version" example:"1.2.3"`
}

// IotActivateReq 设备端激活请求。全部字段可选，用于生成主账号资料。
type IotActivateReq struct {
	OwnerName  string `json:"owner_name" example:"张三"`
	OwnerPhone string `json:"owner_phone" example:"13800000000"`
	Nickname   string `json:"nickname" example:"主人账号"`
}
