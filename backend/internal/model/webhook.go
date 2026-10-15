package model

import "time"

// ════════════════════════════════════════════════════════════════
// 消息推送：WebHook 配置
// ════════════════════════════════════════════════════════════════

// WebHook 平台类型。
const (
	WebhookPlatformFeishu   = "feishu"   // 飞书自定义机器人
	WebhookPlatformDingtalk = "dingtalk" // 钉钉自定义机器人
	WebhookPlatformWecom    = "wecom"    // 企业微信机器人
	WebhookPlatformCustom   = "custom"   // 自定义 WebHook（通用 JSON）
)

// WebHook 订阅事件类型。与 ac_notification.category 取值保持一致，便于统一分发。
const (
	WebhookEventRisk    = "risk"    // AI 风险
	WebhookEventAlarm   = "alarm"   // 设备告警
	WebhookEventInspect = "inspect" // 巡检通知
	WebhookEventDevice  = "device"  // 设备上下线
)

// WebHook 消息格式。
const (
	WebhookMsgText     = "text"
	WebhookMsgMarkdown = "markdown"
)

// WebhookConfig 消息推送 WebHook 配置。表 ac_webhook_config。
//
// 平台级配置（不按部门隔离），由系统管理员维护。
// 每条配置可订阅一个或多个事件类型（EventTypes 逗号分隔），
// 事件发生时由 WebhookService.Dispatch 逐个下发。
type WebhookConfig struct {
	BaseModel
	Active   *bool  `gorm:"column:active;default:true;comment:是否启用" json:"active"`
	Sequence int    `gorm:"column:sequence;default:10;comment:排序" json:"sequence"`
	Name     string `gorm:"column:name;size:255;not null;comment:配置名称" json:"name"`
	Platform string `gorm:"column:platform;size:20;not null;default:custom;comment:feishu/dingtalk/wecom/custom" json:"platform"`
	URL      string `gorm:"column:url;size:1000;not null;comment:WebHook 地址" json:"url"`

	// Secret 用于钉钉加签（HMAC-SHA256）或自定义平台的签名校验；飞书/企业微信可留空。
	Secret *string `gorm:"column:secret;size:255;comment:签名密钥(钉钉加签/自定义)" json:"secret"`

	MsgType string `gorm:"column:msg_type;size:20;default:text;comment:text/markdown" json:"msg_type"`
	// EventTypes 订阅的事件类型，逗号分隔，如 "risk,alarm"。为空表示订阅全部事件。
	EventTypes string `gorm:"column:event_types;size:255;comment:订阅事件(逗号分隔,空=全部)" json:"event_types"`

	// 发送结果快照，便于在列表页直接观察通道健康度
	LastSendDT *time.Time `gorm:"column:last_send_dt;comment:最近发送时间" json:"last_send_dt"`
	LastStatus string     `gorm:"column:last_status;size:20;comment:最近发送结果 success/failed" json:"last_status"`
	LastError  string     `gorm:"column:last_error;type:text;comment:最近失败原因" json:"last_error"`

	Remark string `gorm:"column:remark;size:500;comment:备注" json:"remark"`
}

func (WebhookConfig) TableName() string { return "ac_webhook_config" }

// Subscribes 判断该配置是否订阅了指定事件。
// EventTypes 为空表示订阅全部事件。
func (w *WebhookConfig) Subscribes(event string) bool {
	if w.EventTypes == "" {
		return true
	}
	for _, e := range splitCSV(w.EventTypes) {
		if e == event {
			return true
		}
	}
	return false
}

// splitCSV 按逗号切分并去除空白项。
func splitCSV(s string) []string {
	out := []string{}
	cur := ""
	for _, ch := range s {
		if ch == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		if ch == ' ' || ch == '\t' {
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
