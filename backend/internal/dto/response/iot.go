package response

import (
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理：产品 / 服务组 / 设备账号 / 设备端 OTA & 激活
// ════════════════════════════════════════════════════════════════

// ── 产品 ──────────────────────────────────────────────────────

// PageIotProductResp 产品分页载荷。
type PageIotProductResp struct {
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Items    []model.IotProduct `json:"items"`
}

// RPageIotProduct 产品分页响应。
type RPageIotProduct struct {
	BaseResponse
	Data PageIotProductResp `json:"data"`
}

// RIotProduct 产品详情响应。
type RIotProduct struct {
	BaseResponse
	Data model.IotProduct `json:"data"`
}

// IotProductUpdateResult 产品更新结果。
//
// Warning 为兼容字段：早期版本曾按「设备类型」做联动校验并返回警告；
// 现在配网方式与服务组都由产品直接配置，正常情况下为空。
type IotProductUpdateResult struct {
	Warning string `json:"warning,omitempty"`
}

// RIotProductUpdated 产品更新响应。
type RIotProductUpdated struct {
	BaseResponse
	Data IotProductUpdateResult `json:"data"`
}

// IotShareQRResp 分享二维码内容。
//
// content 是可直接编码成二维码的 JSON 字符串；设备扫码后据此完成绑定/分享。
type IotShareQRResp struct {
	ProductID   int64  `json:"product_id"`
	ProductCode string `json:"product_code"`
	Content     string `json:"content"`
}

// RIotShareQR 生成分享二维码响应。
type RIotShareQR struct {
	BaseResponse
	Data IotShareQRResp `json:"data"`
}

// ── 服务组 ────────────────────────────────────────────────────

// IotServiceGroupDetail 服务组详情（含服务项）。
type IotServiceGroupDetail struct {
	model.IotServiceGroup
	Items []model.IotServiceItem `json:"items"`
}

// PageIotServiceGroupResp 服务组分页载荷。
type PageIotServiceGroupResp struct {
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
	Items    []model.IotServiceGroup `json:"items"`
}

// RPageIotServiceGroup 服务组分页响应。
type RPageIotServiceGroup struct {
	BaseResponse
	Data PageIotServiceGroupResp `json:"data"`
}

// RIotServiceGroup 服务组详情响应。
type RIotServiceGroup struct {
	BaseResponse
	Data IotServiceGroupDetail `json:"data"`
}

// RIotServiceGroupList 服务组简单列表响应（产品已绑定服务组）。
type RIotServiceGroupList struct {
	BaseResponse
	Data []model.IotServiceGroup `json:"data"`
}

// IotServiceItemListResp 服务项列表载荷。
type IotServiceItemListResp struct {
	Items []model.IotServiceItem `json:"items"`
}

// RIotServiceItemList 服务项列表响应。
type RIotServiceItemList struct {
	BaseResponse
	Data IotServiceItemListResp `json:"data"`
}

// IotServiceGroupCredentials 服务组凭据（重置后一次性返回明文）。
type IotServiceGroupCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RIotServiceGroupCredentials 重置凭据响应。
type RIotServiceGroupCredentials struct {
	BaseResponse
	Data IotServiceGroupCredentials `json:"data"`
}

// ── 设备与账号 ────────────────────────────────────────────────

// PageIotDeviceResp IOT 设备分页载荷。
type PageIotDeviceResp struct {
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Items    []model.Device `json:"items"`
}

// RPageIotDevice IOT 设备分页响应。
type RPageIotDevice struct {
	BaseResponse
	Data PageIotDeviceResp `json:"data"`
}

// IotDeviceDetail 设备详情：设备 + 账号列表 + 已授权服务组 + 设备二维码。
type IotDeviceDetail struct {
	Device        model.Device             `json:"device"`
	Accounts      []model.IotDeviceAccount `json:"accounts"`
	ServiceGroups []IotServiceGroupDetail  `json:"service_groups"`
	// QRContent 设备二维码内容，前端可直接渲染，无需再调 /qr。
	QRContent string `json:"qr_content"`
}

// IotDeviceQR 设备二维码。
//
// qr_content 固定为 {"type":"device","product_code":"...","device_code":"..."}：
// App 扫到 type=device 即视为持有该设备，用 bind-qr 完成绑定。
type IotDeviceQR struct {
	DeviceID    int64  `json:"device_id"`
	ProductCode string `json:"product_code"`
	DeviceCode  string `json:"device_code"`
	QRContent   string `json:"qr_content"`
}

// RIotDeviceQR 设备二维码响应。
type RIotDeviceQR struct {
	BaseResponse
	Data IotDeviceQR `json:"data"`
}

// RIotDeviceDetail 设备详情响应。
type RIotDeviceDetail struct {
	BaseResponse
	Data IotDeviceDetail `json:"data"`
}

// IotDeviceAccountCreated 新增账号结果。
//
// Password 仅在服务端自动生成时返回一次；管理员显式设置密码时为空。
type IotDeviceAccountCreated struct {
	Account  model.IotDeviceAccount `json:"account"`
	Password string                 `json:"password,omitempty"`
}

// RIotDeviceAccountCreated 新增账号响应。
type RIotDeviceAccountCreated struct {
	BaseResponse
	Data IotDeviceAccountCreated `json:"data"`
}

// RIotDeviceAccount 设备账号详情响应。
type RIotDeviceAccount struct {
	BaseResponse
	Data model.IotDeviceAccount `json:"data"`
}

// IotDeviceActivated 激活结果。
//
// Password 仅在**首次**创建主账号时返回；重复激活（幂等）时为空。
type IotDeviceActivated struct {
	Username    string     `json:"username"`
	Password    string     `json:"password,omitempty"`
	DeviceCode  string     `json:"device_code"`
	ActivatedAt *time.Time `json:"activated_at"`
}

// RIotDeviceActivated Web 端手动激活响应。
type RIotDeviceActivated struct {
	BaseResponse
	Data IotDeviceActivated `json:"data"`
}

// IotDeviceSecret 重置后的设备接入密钥（一次性明文）。
type IotDeviceSecret struct {
	DeviceCode   string `json:"device_code"`
	DeviceSecret string `json:"device_secret"`
}

// RIotDeviceSecret 重置密钥响应。
type RIotDeviceSecret struct {
	BaseResponse
	Data IotDeviceSecret `json:"data"`
}

// ── 设备端 OTA / 激活 ────────────────────────────────────────

// IotOTAServiceGroup 下发给设备的服务组配置。
//
// username/password 是**明文**：设备已用 device_secret 完成身份确认，
// 若此处再脱敏，设备将无法登录服务。Web 读接口仍然脱敏。
type IotOTAServiceGroup struct {
	Code     string                 `json:"code"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Username string                 `json:"username"`
	Password string                 `json:"password"`
	Items    []model.IotServiceItem `json:"items"`
}

// IotOTATurn 是随 OTA 下发的 TURN/STUN 接入信息。
//
// coturn 没有鉴权回调，凭据由后端用共享密钥签发（TURN REST API）：
// Username 是 "<过期时间戳>:<device_code>"，Credential 是 HMAC-SHA1 的 Base64。
// 未配置 TURN 时整个字段省略，设备侧按「无 TURN」降级。
type IotOTATurn struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
	TTL        int      `json:"ttl"`
	ExpiresAt  int64    `json:"expires_at"`
	Realm      string   `json:"realm"`
}

// IotOTAResp 设备端 OTA 响应。
type IotOTAResp struct {
	Product       *model.IotProduct    `json:"product"`
	Device        model.Device         `json:"device"`
	ServiceGroups []IotOTAServiceGroup `json:"service_groups"`
	Turn          *IotOTATurn          `json:"turn,omitempty"`
	ServerTime    string               `json:"server_time"`
}

// RIotOTA 设备端 OTA 响应包装。
type RIotOTA struct {
	BaseResponse
	Data IotOTAResp `json:"data"`
}

// RIotDeviceActivateResp 设备端激活响应。
type RIotDeviceActivateResp struct {
	BaseResponse
	Data IotDeviceActivated `json:"data"`
}

// ── 明文鉴权信息（Web「复制鉴权信息」/ App 服务组）────────────
//
// 为什么单独接口而不是在列表里返回明文：
//  1. 明文密码是最高敏感数据，不能让任何带 iot:list 权限的读接口顺带泄露；
//  2. 「复制」是低频、显式的人为动作，单独端点可以要求更强的 update 权限，
//     并在网关/审计层按 URL 单独打点；
//  3. 其余读接口继续返回 ****，前端也不存在「误把明文渲染到列表」的可能。

// IotCredentialsProduct 凭据所属产品（仅摘要）。
type IotCredentialsProduct struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// IotCredentialsGroup 单个服务组的明文凭据与服务项。
type IotCredentialsGroup struct {
	Code     string                 `json:"code"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Username string                 `json:"username"`
	Password string                 `json:"password"`
	Items    []model.IotServiceItem `json:"items"`
}

// IotMqttInfo MQTT 连接信息。
//
// topic_prefix 约定为 iot/<group_code>：同一产品下不同服务组的设备
// 必须落在不同的主题前缀下，否则 ACL 无法按组隔离。
type IotMqttInfo struct {
	URL         string `json:"url"`
	TopicPrefix string `json:"topic_prefix"`
}

// IotTurnInfo TURN/STUN 接入点（仅地址与 Realm，不含短期凭据）。
type IotTurnInfo struct {
	URLs  []string `json:"urls"`
	Realm string   `json:"realm"`
}

// IotCredentials 产品级明文鉴权信息（前端「复制」按钮的数据源）。
type IotCredentials struct {
	Product IotCredentialsProduct `json:"product"`
	Groups  []IotCredentialsGroup `json:"groups"`
	MQTT    *IotMqttInfo          `json:"mqtt"`
	Turn    *IotTurnInfo          `json:"turn"`
}

// RIotCredentials 产品明文鉴权信息响应。
type RIotCredentials struct {
	BaseResponse
	Data IotCredentials `json:"data"`
}

// IotGroupCredentials 单个服务组的明文鉴权信息。
type IotGroupCredentials struct {
	Group IotCredentialsGroup `json:"group"`
	MQTT  *IotMqttInfo        `json:"mqtt"`
}

// RIotGroupCredentials 单个服务组明文鉴权信息响应。
type RIotGroupCredentials struct {
	BaseResponse
	Data IotGroupCredentials `json:"data"`
}

// ── 设备分享令牌 ──────────────────────────────────────────────

// IotDeviceShareToken 设备分享令牌（一次性）。
//
// QRContent 固定为 {"type":"device_share","token":"..."}：
// App 扫码后先按 type 判断这是设备分享而不是产品分享，再取 token 认领。
type IotDeviceShareToken struct {
	DeviceID  int64  `json:"device_id"`
	AccountID int64  `json:"account_id"`
	Token     string `json:"token"`
	QRContent string `json:"qr_content"`
}

// RIotDeviceShareToken 生成分享令牌响应。
type RIotDeviceShareToken struct {
	BaseResponse
	Data IotDeviceShareToken `json:"data"`
}

// IotShareTokenRevoked 撤销结果。
type IotShareTokenRevoked struct {
	Revoked int64 `json:"revoked"`
}

// RIotShareTokenRevoked 撤销分享令牌响应。
type RIotShareTokenRevoked struct {
	BaseResponse
	Data IotShareTokenRevoked `json:"data"`
}

// ── App 端 IOT 与 RTC ────────────────────────────────────────

// IotProductBrief 产品摘要。
type IotProductBrief struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
	// AllowShare 决定 App 是否展示「分享设备」入口；
	// 配网方式由产品配置（net_config_mode）表达，不在这里重复下发。
	AllowShare bool `json:"allow_share"`
}

// AppIotDeviceAccount 设备账号摘要（不含密码哈希与分享令牌）。
type AppIotDeviceAccount struct {
	ID             int64      `json:"id"`
	DeviceID       int64      `json:"device_id"`
	Username       string     `json:"username"`
	Role           string     `json:"role"`
	Nickname       string     `json:"nickname"`
	Phone          string     `json:"phone"`
	Status         string     `json:"status"`
	PlatformUserID *int64     `json:"platform_user_id,omitempty"`
	ActivatedAt    *time.Time `json:"activated_at"`
}

// AppIotDevice App 设备列表项。
type AppIotDevice struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	DeviceCode string           `json:"device_code"`
	Location   string           `json:"location"`
	Status     string           `json:"status"`
	Product    *IotProductBrief `json:"product,omitempty"`
	Role       string           `json:"role"`
	Nickname   string           `json:"nickname"`
}

// IotServiceGroupBrief 服务组摘要（不含凭据）。
type IotServiceGroupBrief struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Active bool   `json:"active"`
}

// AppIotDeviceDetail App 设备详情。
type AppIotDeviceDetail struct {
	Device        AppIotDevice           `json:"device"`
	Product       *model.IotProduct      `json:"product"`
	Account       AppIotDeviceAccount    `json:"account"`
	ServiceGroups []IotServiceGroupBrief `json:"service_groups"`
}

// AppIotDeviceBinding 认领/绑定结果。Created 表示本次是否新建了账号。
type AppIotDeviceBinding struct {
	Device  AppIotDevice        `json:"device"`
	Account AppIotDeviceAccount `json:"account"`
	Role    string              `json:"role"`
	Created bool                `json:"created"`
}

// RIotAppDeviceList App 设备列表响应。
type RIotAppDeviceList struct {
	BaseResponse
	Data []AppIotDevice `json:"data"`
}

// RIotAppDeviceDetail App 设备详情响应。
type RIotAppDeviceDetail struct {
	BaseResponse
	Data AppIotDeviceDetail `json:"data"`
}

// RIotAppDeviceBinding App 认领/绑定响应。
type RIotAppDeviceBinding struct {
	BaseResponse
	Data AppIotDeviceBinding `json:"data"`
}

// RIotAppServices App 设备服务组（含明文凭据）响应。
type RIotAppServices struct {
	BaseResponse
	Data IotCredentials `json:"data"`
}

// IceServer WebRTC ICE 服务器条目（TURN REST API 短期凭据）。
type IceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
	Realm      string   `json:"realm"`
	TTL        int      `json:"ttl"`
	ExpiresAt  int64    `json:"expires_at"`
}

// RIceServerList ICE 服务器列表响应（未配置 TURN 时 data 为空数组）。
type RIceServerList struct {
	BaseResponse
	Data []IceServer `json:"data"`
}
