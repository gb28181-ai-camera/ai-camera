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
	Name string `json:"name" binding:"required" example:"4G 球机"`
	Code string `json:"code" binding:"required" example:"CAM_4G_01"`
	// 配网方式由产品配置：qrcode（无需配网）/ bluetooth / ap。
	Category      string `json:"category" example:"摄像头"`
	Description   string `json:"description"`
	AutoNetwork   *bool  `json:"auto_network" example:"true"`
	NetConfigMode string `json:"net_config_mode" example:"bluetooth"`
	AllowShare    *bool  `json:"allow_share" example:"true"`
	Sequence      int    `json:"sequence" example:"10"`
	Active        *bool  `json:"active" example:"true"`
	// 产品可选的服务组（系统预置三条）。创建时同步绑定，并为组内服务生成凭据。
	ServiceGroupIDs []int64 `json:"service_group_ids"`
}

// UpdateIotProductReq 更新产品。
//
// AutoNetwork / AllowShare / Active 用指针，以便区分「未传」与「显式 false」；
// 文本字段为空表示不修改（与项目内其它 Update 请求保持一致）。
type UpdateIotProductReq struct {
	Name string `json:"name"`
	Code string `json:"code"`
	// 配网方式为空表示不修改。
	Category      string `json:"category"`
	Description   string `json:"description"`
	AutoNetwork   *bool  `json:"auto_network"`
	NetConfigMode string `json:"net_config_mode"`
	AllowShare    *bool  `json:"allow_share"`
	Sequence      int    `json:"sequence"`
	Active        *bool  `json:"active"`
	// nil 表示不修改服务组绑定；空数组表示清空；非空表示整体替换。
	ServiceGroupIDs []int64 `json:"service_group_ids"`
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
// 服务组是系统预置数据：编码与类型不可修改，只允许名称/描述/启用状态；
// 被产品引用时只读。
type UpdateIotServiceGroupReq struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Active      *bool  `json:"active"`
}

// IotServiceReq 单个服务。
type IotServiceReq struct {
	Code     string `json:"code" binding:"required" example:"mqtt"`
	Name     string `json:"name" binding:"required" example:"MQTT 接入"`
	Protocol string `json:"protocol" binding:"required" example:"MQTT"`
	Host     string `json:"host" example:"mqtt.example.com"`
	Port     int    `json:"port" example:"1883"`
	Path     string `json:"path"`
	Extra    string `json:"extra" example:"{\"tls\":true}"`
	Sequence int    `json:"sequence" example:"10"`
	Active   *bool  `json:"active" example:"true"`
}

// ReplaceIotServicesReq 整体替换服务组的服务。
//
// 语义是替换而非增量：未列出的服务会被删除，便于前端用一张表格提交全量配置。
type ReplaceIotServicesReq struct {
	Services []IotServiceReq `json:"services"`
}

// ── 设备与账号 ────────────────────────────────────────────────

// PageIotDeviceReq IOT 设备分页查询（数据源仍是 ac_device）。
type PageIotDeviceReq struct {
	PageReq
	Keyword   string `form:"keyword" json:"keyword" example:"CAM"`
	ProductID *int64 `form:"product_id" json:"product_id"`
	Status    string `form:"status" json:"status" example:"online"`
	// ServiceGroupType 按产品绑定的服务组类型过滤：webrtc / aichat / gb28181。
	// 「实时音视频」「智能语音聊天」页面用它复用同一个设备列表接口。
	ServiceGroupType string `form:"service_group_type" json:"service_group_type" example:"webrtc"`
}

// CreateIotDeviceAccountReq 新增设备账号。
//
// Role 缺省为 shared（分享账号）；owner 只应在设备未激活时使用，
// 常规的主账号创建走 /devices/:id/activate 或设备端 /iot/activate。
// Password 留空时由服务端生成 16 位随机密码并一次性返回。
type CreateIotDeviceAccountReq struct {
	Username string `json:"username" example:"family01"`
	Password string `json:"password"`
	Role     string `json:"role" example:"shared"`
	Nickname string `json:"nickname" example:"家人账号"`
	Phone    string `json:"phone"`
	Status   string `json:"status" example:"active"`
	// 分享令牌不由本接口写入：需要分享码时调用
	// POST /web/iot/devices/:id/share-token，由服务端生成一次性令牌。
}

// UpdateIotDeviceAccountReq 更新设备账号。
//
// Role 不可修改：主账号与分享账号的权限边界不同，静默允许改角色
// 会让「分享」变成「主账号」，属于越权风险。密码为空则保持原值。
type UpdateIotDeviceAccountReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
	Phone    string `json:"phone"`
	Status   string `json:"status"`
	// share_token 不在此接口接受：令牌只能由 share-token 接口生成，
	// 否则管理员可写入任意令牌，绕过「一次性、可撤销」的约束。
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

// ── App 端 IOT ───────────────────────────────────────────────

// ClaimIotDeviceReq App 扫码认领设备。token 来自二维码内容。
type ClaimIotDeviceReq struct {
	Token string `json:"token" binding:"required" example:"share_abcd1234"`
}

// BindIotDeviceReq App 蓝牙配网后绑定设备。
//
// device_secret 是设备出厂/重置时下发的接入密钥，服务端用常量时间比较校验；
// 校验通过且设备尚无主账号时，本次绑定会创建主账号（owner）。
type BindIotDeviceReq struct {
	DeviceCode   string `json:"device_code" binding:"required" example:"CAM_4G_001"`
	DeviceSecret string `json:"device_secret" binding:"required" example:"0123456789abcdef"`
	Nickname     string `json:"nickname" example:"客厅摄像头"`
}

// BindIotDeviceQRReq App 扫码绑定设备。
//
// 二维码内容只含 product_code + device_code，不含密钥：扫码即视为持有设备，
// 因此本接口不做 secret 校验（安全边界见接口契约说明）。
type BindIotDeviceQRReq struct {
	ProductCode string `json:"product_code" binding:"required" example:"CAM_4G_01"`
	DeviceCode  string `json:"device_code" binding:"required" example:"CAM_4G_001"`
}
