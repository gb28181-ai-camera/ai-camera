package model

import "time"

// ════════════════════════════════════════════════════════════════
// IOT 管理：产品 / 服务组 / 服务项 / 产品-服务组绑定 / 设备账号 / 设备事件
//
// 设计要点（为什么这样建模）：
//   1. 设备**复用 ac_device**，不新建 ac_iot_device。
//      设备是全平台唯一的资产（已被风险、告警、巡检、升级、国标等模块引用），
//      再建一张 IOT 设备表会产生两套主键与两套生命周期，数据同步必然出错。
//      IOT 视角只是在 ac_device 上追加 product_id / device_secret / activated_at。
//   2. 服务组凭据是**组级**而不是设备级。
//      同一产品下可能有成百上千台设备，若每台设备一套凭据，服务端要维护
//      N 倍的账号与轮换流程；而设备登录服务只需要「读到一组可用凭据」。
//      因此 username/password 落在 ac_iot_service_group，设备端 OTA 时下发。
//   3. 设备端 OTA / 激活**不走 JWT**。设备是无人值守进程，没有用户会话，
//      也无法完成 Casbin 的角色授权。它用设备自带的 device_secret 做双向确认：
//      平台下发 secret（仅一次）→ 设备持有 → 每次 OTA 携带 X-Device-* 头。
// ════════════════════════════════════════════════════════════════

// IotGroupTypeLabels 服务组类型的中文名（错误提示用）。
var IotGroupTypeLabels = map[string]string{
	IotServiceGroupGb28181: "GB28181",
	IotServiceGroupWebRTC:  "WebRTC",
	IotServiceGroupAIChat:  "AIChat",
}

// IOT 产品配网方式。
const (
	IotNetConfigBluetooth = "bluetooth" // 蓝牙配网
	IotNetConfigAP        = "ap"        // AP 配网
	IotNetConfigQRCode    = "qrcode"    // 二维码配网
)

// IOT 服务组类型。
const (
	IotServiceGroupGb28181 = "gb28181"
	IotServiceGroupWebRTC  = "webrtc"
	IotServiceGroupAIChat  = "aichat"
)

// IotFixedServiceGroupCodes 系统预置的服务组编码。
//
// 服务组不是自由字典，而是三条固定数据：gb28181 / webrtc / aichat。
// 编码同时决定类型与协议白名单；不开放新增/删除，编码与类型不可修改。
var IotFixedServiceGroupCodes = []string{
	IotServiceGroupGb28181,
	IotServiceGroupWebRTC,
	IotServiceGroupAIChat,
}

// IsFixedServiceGroupCode 判断编码是否为系统预置的服务组。
func IsFixedServiceGroupCode(code string) bool {
	for _, c := range IotFixedServiceGroupCodes {
		if c == code {
			return true
		}
	}
	return false
}

// IOT 服务协议。
const (
	IotProtocolSIP       = "SIP"
	IotProtocolRTSP      = "RTSP"
	IotProtocolWebRTC    = "WebRTC"
	IotProtocolMQTT      = "MQTT"
	IotProtocolWebSocket = "WebSocket"
	IotProtocolTURN      = "TURN"
)

// IotGroupAllowedProtocols 服务组类型 → 允许的协议白名单。
//
// 白名单是**业务约束**而不是技术约束：例如 GB28181 服务组只应暴露 SIP 与 RTSP，
// 混入 WebSocket 会让设备拿到与自己协议栈无关的配置，排查成本极高。
// 创建/更新服务项时校验；读取时不再校验（历史数据保持可读）。
var IotGroupAllowedProtocols = map[string][]string{
	IotServiceGroupGb28181: {IotProtocolSIP, IotProtocolRTSP},
	IotServiceGroupWebRTC:  {IotProtocolWebRTC, IotProtocolMQTT, IotProtocolRTSP, IotProtocolTURN},
	IotServiceGroupAIChat:  {IotProtocolWebSocket},
}

// IOT 设备账号角色。
const (
	IotAccountRoleOwner  = "owner"  // 主账号（激活时创建）
	IotAccountRoleShared = "shared" // 分享账号
)

// IOT 设备账号状态。
const (
	IotAccountStatusActive   = "active"
	IotAccountStatusDisabled = "disabled"
)

// IOT 设备事件类型。
const (
	IotEventOTA           = "ota"
	IotEventActivate      = "activate"
	IotEventAccountCreate = "account-create"
	IotEventSecretReset   = "secret-reset"
	IotEventUnbind        = "unbind"
)

// IotProduct IOT 产品。表 ac_iot_product。
//
// 产品是设备接入配置的聚合根：联网方式、配网方式、分享策略以及可用的服务组，
// 都挂在产品上；设备只需要指向产品，OTA 时由产品推导出全部服务配置。
type IotProduct struct {
	BaseModel
	Active         *bool  `gorm:"column:active;default:true" json:"active"`
	Sequence       int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name           string `gorm:"column:name;size:255;not null;comment:产品名称" json:"name"`
	Code           string `gorm:"column:code;size:100;not null;uniqueIndex:idx_ac_iot_product_code;comment:产品编码" json:"code"`
	Category       string `gorm:"column:category;size:100;comment:产品分类" json:"category"`
	Description    string `gorm:"column:description;type:text;comment:产品描述" json:"description"`
	AutoNetwork    bool   `gorm:"column:auto_network;default:false;comment:是否自动联网(4G设备)" json:"auto_network"`
	NetConfigMode  string `gorm:"column:net_config_mode;size:20;default:bluetooth;comment:配网方式 bluetooth/ap/qrcode" json:"net_config_mode"`
	AllowShare     bool   `gorm:"column:allow_share;default:false;comment:是否允许分享" json:"allow_share"`
	ShareQRContent string `gorm:"column:share_qr_content;type:text;comment:分享二维码内容(JSON字符串)" json:"share_qr_content"`
}

func (IotProduct) TableName() string { return "ac_iot_product" }

// IotServiceGroup IOT 服务组。表 ac_iot_service_group。
//
// 服务组是**固定三条分类数据**（gb28181 / webrtc / aichat），只描述分类，
// 不持有凭据；具体接入服务与凭据分别落在 ac_iot_service / ac_iot_product_service。
type IotServiceGroup struct {
	BaseModel
	Active      *bool  `gorm:"column:active;default:true" json:"active"`
	Name        string `gorm:"column:name;size:255;not null;comment:服务组名称" json:"name"`
	Code        string `gorm:"column:code;size:100;not null;uniqueIndex:idx_ac_iot_service_group_code;comment:服务组编码" json:"code"`
	Type        string `gorm:"column:type;size:20;not null;comment:服务组类型 gb28181/webrtc/aichat" json:"type"`
	Description string `gorm:"column:description;type:text;comment:描述" json:"description"`

	// ReferencedByProducts 引用该服务组的产品数量（非数据库列）。
	//
	// 服务组一旦被产品绑定就进入**只读**状态：设备 OTA 正在使用它下面的服务，
	// 修改服务或地址会让已联网设备拿到不一致的配置。必须先在产品上解绑。
	ReferencedByProducts int64 `gorm:"-" json:"referenced_by_products"`
}

func (IotServiceGroup) TableName() string { return "ac_iot_service_group" }

// IotService IOT 服务。表 ac_iot_service。
//
// 服务组 → 服务是 1:N；服务是具体接入项（SIP / RTSP / WebRTC / MQTT / TURN / WebSocket）。
// 同一组内按 code 唯一；整体替换时硬删除，避免软删除行占用唯一索引。
type IotService struct {
	BaseModel
	Active   *bool  `gorm:"column:active;default:true" json:"active"`
	GroupID  int64  `gorm:"column:group_id;not null;uniqueIndex:idx_ac_iot_service_grp_code;comment:服务组ID" json:"group_id"`
	Code     string `gorm:"column:code;size:50;not null;uniqueIndex:idx_ac_iot_service_grp_code;comment:服务编码(组内唯一)" json:"code"`
	Name     string `gorm:"column:name;size:100;not null;comment:服务名称" json:"name"`
	Protocol string `gorm:"column:protocol;size:20;not null;comment:协议 SIP/RTSP/WebRTC/MQTT/WebSocket/TURN" json:"protocol"`
	Host     string `gorm:"column:host;size:255;comment:服务地址" json:"host"`
	Port     int    `gorm:"column:port;comment:服务端口" json:"port"`
	Path     string `gorm:"column:path;size:500;comment:服务路径" json:"path"`
	Extra    string `gorm:"column:extra;type:text;comment:扩展配置(JSON字符串)" json:"extra"`
	Sequence int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
}

func (IotService) TableName() string { return "ac_iot_service" }

// IotProductServiceGroup 产品-服务组绑定。表 ac_iot_product_service_group。
//
// 纯关联表：无 active / 无审计语义，整体替换时硬删除后重建。
type IotProductServiceGroup struct {
	BaseModel
	ProductID      int64 `gorm:"column:product_id;not null;uniqueIndex:idx_ac_iot_product_service_group;comment:产品ID" json:"product_id"`
	ServiceGroupID int64 `gorm:"column:service_group_id;not null;uniqueIndex:idx_ac_iot_product_service_group;comment:服务组ID" json:"service_group_id"`
}

func (IotProductServiceGroup) TableName() string { return "ac_iot_product_service_group" }

// IotProductService 产品-服务绑定（含该产品在该服务上的鉴权信息）。
//
// 凭据是**产品 × 服务**级：同一服务组下不同产品、同一产品的不同服务都有独立账号。
// 设备端 OTA 按产品读取本表下发；MQTT/TURN 鉴权也以这里的 username/password 为准。
// 密码为明文入库（设备端必须拿到可直接登录的凭据），Web 读接口统一脱敏。
type IotProductService struct {
	BaseModel
	Active    *bool  `gorm:"column:active;default:true" json:"active"`
	ProductID int64  `gorm:"column:product_id;not null;uniqueIndex:idx_ac_iot_product_service;comment:产品ID" json:"product_id"`
	ServiceID int64  `gorm:"column:service_id;not null;uniqueIndex:idx_ac_iot_product_service;comment:服务ID" json:"service_id"`
	Username  string `gorm:"column:username;size:100;not null;uniqueIndex:idx_ac_iot_product_service_username;comment:服务登录用户名(自动生成,全局唯一)" json:"username"`
	Password  string `gorm:"column:password;size:255;not null;comment:服务登录密码(自动生成,敏感)" json:"password"`
	Extra     string `gorm:"column:extra;type:text;comment:扩展配置(JSON字符串)" json:"extra"`
}

func (IotProductService) TableName() string { return "ac_iot_product_service" }

// TurnUserLT coturn 长期凭据用户表（与 coturn SQL userdb 共用）。
//
// coturn 没有 HTTP/RPC 认证回调；使用 --userdb 指向本库后，
// 它直接查询 turnusers_lt。hmackey = MD5(username:realm:password)。
// 该表不由业务接口直接暴露，只由产品×TURN 服务凭据同步写入。
type TurnUserLT struct {
	Realm   string `gorm:"column:realm;size:127;primaryKey" json:"realm"`
	Name    string `gorm:"column:name;size:512;primaryKey" json:"name"`
	Hmackey string `gorm:"column:hmackey;size:128;not null" json:"hmackey"`
}

func (TurnUserLT) TableName() string { return "turnusers_lt" }

// TurnSecret coturn SQL userdb 的 REST 密钥表（结构兼容，业务不写入）。
type TurnSecret struct {
	Realm string `gorm:"column:realm;size:127;primaryKey" json:"realm"`
	Value string `gorm:"column:value;size:256;primaryKey" json:"value"`
}

func (TurnSecret) TableName() string { return "turn_secret" }

// IotDeviceAccount 设备账号。表 ac_iot_device_account。
//
// 主账号（owner）由「激活」创建，一台设备最多一个；
// 分享账号（shared）由管理端按需添加。密码只存 bcrypt 哈希，
// 明文仅在创建/激活的响应里返回一次。
type IotDeviceAccount struct {
	BaseModel
	DeviceID     int64      `gorm:"column:device_id;not null;index:idx_ac_iot_device_account_device;comment:设备ID" json:"device_id"`
	Username     string     `gorm:"column:username;size:100;not null;uniqueIndex:idx_ac_iot_device_account_username;comment:登录名" json:"username"`
	PasswordHash string     `gorm:"column:password_hash;size:255;not null;comment:密码哈希(bcrypt)" json:"-"`
	Role         string     `gorm:"column:role;size:20;not null;default:shared;comment:角色 owner/shared" json:"role"`
	Nickname     string     `gorm:"column:nickname;size:100;comment:昵称" json:"nickname"`
	Phone        string     `gorm:"column:phone;size:50;comment:手机号" json:"phone"`
	Status       string     `gorm:"column:status;size:20;not null;default:active;comment:状态 active/disabled" json:"status"`
	ShareToken   *string    `gorm:"column:share_token;size:100;uniqueIndex:idx_ac_iot_device_account_share_token;comment:分享令牌(可空)" json:"share_token,omitempty"`
	ActivatedAt  *time.Time `gorm:"column:activated_at;comment:激活时间" json:"activated_at"`

	// PlatformUserID 账号所绑定的平台用户（App 认领/绑定后写入）。
	//
	// 为什么单独建列而不是复用 username：设备账号的 username 是**给设备端服务**用的
	// 登录名，平台用户则是 App 的 JWT 主体，两者命名空间与生命周期都不同。
	// 分享令牌尚未被认领时该列为 NULL；App 设备列表按此列过滤，
	// 从而保证「一个用户只能看到自己认领/绑定的设备」。
	PlatformUserID *int64 `gorm:"column:platform_user_id;index:idx_ac_iot_device_account_platform_user;comment:绑定的平台用户ID" json:"platform_user_id,omitempty"`
}

func (IotDeviceAccount) TableName() string { return "ac_iot_device_account" }

// IotDeviceEvent 设备端接入事件（追加写）。表 ac_iot_device_event。
//
// OTA / 激活 / 新增账号 / 重置密钥都会留痕，用于回答
// 「这台设备什么时候被谁接入过、密钥什么时候换的」。不做更新与删除。
type IotDeviceEvent struct {
	BaseModel
	DeviceID int64  `gorm:"column:device_id;not null;index:idx_ac_iot_device_event_device;comment:设备ID" json:"device_id"`
	Event    string `gorm:"column:event;size:30;not null;comment:ota/activate/account-create/secret-reset/unbind" json:"event"`
	Detail   string `gorm:"column:detail;type:text;comment:事件详情(JSON字符串)" json:"detail"`
	IP       string `gorm:"column:ip;size:64;comment:来源IP" json:"ip"`
}

func (IotDeviceEvent) TableName() string { return "ac_iot_device_event" }
