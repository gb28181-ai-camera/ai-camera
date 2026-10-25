package model

import "time"

// UserPushToken 是 App 上报的推送客户端标识（个推 CID）。
//
// 为什么需要它：设备呼叫 App 时，App 可能已被系统回收，WebSocket 不存在；
// 服务端只能通过厂商推送通道唤醒。CID 由个推 SDK 在 App 启动后生成，
// 通过 POST /app/push-tokens 上报，按 user_id 关联。
type UserPushToken struct {
	BaseModel
	UserID     int64     `gorm:"column:user_id;not null;index:idx_ac_user_push_token_user;comment:平台用户ID" json:"user_id"`
	CID        string    `gorm:"column:cid;size:128;not null;uniqueIndex:idx_ac_user_push_token_cid;comment:个推客户端ID" json:"cid"`
	Platform   string    `gorm:"column:platform;size:20;comment:ios/android" json:"platform"`
	AppVersion string    `gorm:"column:app_version;size:50;comment:App版本" json:"app_version"`
	LastSeenDT time.Time `gorm:"column:last_seen_dt;comment:最近上报时间" json:"last_seen_dt"`
}

// TableName 表名。
func (UserPushToken) TableName() string { return "ac_user_push_token" }

func init() {
	RegisterModel(&UserPushToken{})
}
