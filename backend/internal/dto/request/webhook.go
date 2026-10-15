package request

// PageWebhookReq WebHook 配置分页查询。
type PageWebhookReq struct {
	PageReq
	Keyword  string `form:"keyword" json:"keyword"`   // 名称模糊
	Platform string `form:"platform" json:"platform"` // feishu/dingtalk/wecom/custom
	Active   *bool  `form:"active" json:"active"`
}

// WebhookItemReq WebHook 配置的新增/编辑公共字段。
type WebhookItemReq struct {
	Name       string   `json:"name" binding:"required" example:"运维告警群"`
	Platform   string   `json:"platform" binding:"required,oneof=feishu dingtalk wecom custom" example:"feishu"`
	URL        string   `json:"url" binding:"required,url" example:"https://open.feishu.cn/open-apis/bot/v2/hook/xxx"`
	Secret     *string  `json:"secret" example:"SECxxx"`
	MsgType    string   `json:"msg_type" binding:"omitempty,oneof=text markdown" example:"markdown"`
	EventTypes []string `json:"event_types" example:"risk,alarm"`
	Sequence   int      `json:"sequence" example:"10"`
	Active     *bool    `json:"active" example:"true"`
	Remark     string   `json:"remark" example:"风险与告警推送到运维群"`
}

// CreateWebhookReq 新增 WebHook 配置。
type CreateWebhookReq struct {
	WebhookItemReq
}

// UpdateWebhookReq 编辑 WebHook 配置。
type UpdateWebhookReq struct {
	WebhookItemReq
}

// TestWebhookReq 发送测试消息。
type TestWebhookReq struct {
	// Title / Content 为空时使用默认的测试文案。
	Title   string `json:"title"`
	Content string `json:"content"`
}

// TestWebhookInlineReq 未保存配置时的即时连通性测试（仅校验，不落库）。
type TestWebhookInlineReq struct {
	WebhookItemReq
	Title   string `json:"title"`
	Content string `json:"content"`
}
