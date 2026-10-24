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

// IotDeviceDetail 设备详情：设备 + 账号列表 + 已授权服务组。
type IotDeviceDetail struct {
	Device        model.Device             `json:"device"`
	Accounts      []model.IotDeviceAccount `json:"accounts"`
	ServiceGroups []IotServiceGroupDetail  `json:"service_groups"`
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
