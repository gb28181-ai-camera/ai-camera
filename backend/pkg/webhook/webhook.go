// Package webhook 实现多平台消息推送（飞书 / 钉钉 / 企业微信 / 自定义 WebHook）。
//
// 各平台的差异被收敛在本包内：调用方只需构造 Message，由 Send 按平台组装报文。
// 钉钉需要 HMAC-SHA256 加签（timestamp + secret），其余平台直接 POST。
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// Message 一条待推送的消息。各平台按自身格式取用其中的字段。
type Message struct {
	// Event 事件类型，取值见 model.WebhookEvent*。
	Event string `json:"event"`
	// Title 标题（钉钉/企业微信 markdown 使用）。
	Title string `json:"title"`
	// Content 正文。
	Content string `json:"content"`
	// Fields 结构化补充字段，自定义平台会原样带上，便于对接方解析。
	Fields map[string]string `json:"fields,omitempty"`
	// Link 关联跳转地址（若有）。
	Link string `json:"link,omitempty"`
	// AtMobiles 需要 @ 的手机号（钉钉 / 企业微信支持）。
	AtMobiles []string `json:"at_mobiles,omitempty"`
	// AtAll 是否 @ 所有人。
	AtAll bool `json:"at_all,omitempty"`
	// OccurredDT 事件发生时间，零值则取发送时刻。
	OccurredDT time.Time `json:"occurred_dt"`
}

// EventLabel 返回事件类型的中文名，用于消息标题。
func EventLabel(event string) string {
	switch event {
	case model.WebhookEventRisk:
		return "AI风险"
	case model.WebhookEventAlarm:
		return "设备告警"
	case model.WebhookEventInspect:
		return "巡检通知"
	case model.WebhookEventDevice:
		return "设备状态"
	default:
		return "系统通知"
	}
}

// client 复用连接，避免每次推送都新建 Transport。
var client = &http.Client{Timeout: 10 * time.Second}

// Send 按配置的平台类型下发消息。
//
// 返回 error 表示推送失败（网络错误、非 2xx、或平台返回了业务错误码）。
func Send(ctx context.Context, cfg *model.WebhookConfig, msg *Message) error {
	if cfg == nil {
		return fmt.Errorf("webhook 配置为空")
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return fmt.Errorf("webhook 地址为空")
	}

	payload, targetURL, err := buildPayload(cfg, msg)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook 返回 HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 300))
	}

	// 三个平台都习惯用 HTTP 200 + 业务错误码表达失败，需要额外判断
	if err := checkPlatformError(cfg.Platform, respBody); err != nil {
		return err
	}

	return nil
}

// buildPayload 按平台组装报文与最终请求地址。
func buildPayload(cfg *model.WebhookConfig, msg *Message) (map[string]interface{}, string, error) {
	targetURL := cfg.URL
	title := msg.Title
	if title == "" {
		title = EventLabel(msg.Event)
	}

	switch cfg.Platform {
	case model.WebhookPlatformFeishu:
		return feishuPayload(cfg, msg, title), targetURL, nil

	case model.WebhookPlatformDingtalk:
		signed, err := signDingtalk(targetURL, secretOf(cfg))
		if err != nil {
			return nil, "", err
		}
		return dingtalkPayload(cfg, msg, title), signed, nil

	case model.WebhookPlatformWecom:
		return wecomPayload(cfg, msg, title), targetURL, nil

	default: // custom
		return customPayload(cfg, msg, title), targetURL, nil
	}
}

// ── 飞书 ──────────────────────────────────────────────────────
// 自定义机器人支持 text / post / interactive。这里用 text（最通用），
// 需要富文本时用 post 类型。
func feishuPayload(cfg *model.WebhookConfig, msg *Message, title string) map[string]interface{} {
	text := composeText(title, msg)
	return map[string]interface{}{
		"msg_type": "text",
		"content":  map[string]interface{}{"text": text},
	}
}

// ── 钉钉 ──────────────────────────────────────────────────────
// markdown 类型：{"msgtype":"markdown","markdown":{"title":..,"text":..}}
func dingtalkPayload(cfg *model.WebhookConfig, msg *Message, title string) map[string]interface{} {
	if cfg.MsgType == model.WebhookMsgText {
		at := map[string]interface{}{"isAtAll": msg.AtAll}
		if len(msg.AtMobiles) > 0 {
			at["atMobiles"] = msg.AtMobiles
		}
		return map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]interface{}{"content": composeText(title, msg)},
			"at":      at,
		}
	}

	text := composeMarkdown(title, msg)
	if msg.AtAll {
		text += "\n\n@所有人"
	}
	for _, m := range msg.AtMobiles {
		text += "\n\n@" + m
	}
	return map[string]interface{}{
		"msgtype":  "markdown",
		"markdown": map[string]interface{}{"title": title, "text": text},
	}
}

// signDingtalk 计算钉钉加签地址：timestamp + "\n" + secret 做 HMAC-SHA256 后 base64 + urlencode。
// secret 为空时原样返回。
func signDingtalk(rawURL, secret string) (string, error) {
	if secret == "" {
		return rawURL, nil
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	stringToSign := timestamp + "\n" + secret

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("webhook 地址非法: %w", err)
	}
	q := u.Query()
	q.Set("timestamp", timestamp)
	q.Set("sign", sign)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// ── 企业微信 ──────────────────────────────────────────────────
// markdown 类型：{"msgtype":"markdown","markdown":{"content":..}}
func wecomPayload(cfg *model.WebhookConfig, msg *Message, title string) map[string]interface{} {
	if cfg.MsgType == model.WebhookMsgText {
		content := composeText(title, msg)
		if msg.AtAll {
			content += "\n@所有人"
		}
		for _, m := range msg.AtMobiles {
			content += "\n@" + m
		}
		return map[string]interface{}{
			"msgtype": "text",
			"text":    map[string]interface{}{"content": content},
		}
	}

	content := composeMarkdown(title, msg)
	if msg.AtAll {
		content += "\n> @所有人"
	}
	return map[string]interface{}{
		"msgtype":  "markdown",
		"markdown": map[string]interface{}{"content": content},
	}
}

// ── 自定义 WebHook ────────────────────────────────────────────
// 通用 JSON 结构，便于第三方系统直接解析。
func customPayload(cfg *model.WebhookConfig, msg *Message, title string) map[string]interface{} {
	occurred := msg.OccurredDT
	if occurred.IsZero() {
		occurred = time.Now()
	}
	return map[string]interface{}{
		"event":       msg.Event,
		"event_label": EventLabel(msg.Event),
		"title":       title,
		"content":     msg.Content,
		"fields":      msg.Fields,
		"link":        msg.Link,
		"occurred_dt": occurred.Format("2006-01-02 15:04:05"),
	}
}

// ── 文本组装 ──────────────────────────────────────────────────

// composeText 组装纯文本消息。
func composeText(title string, msg *Message) string {
	var b strings.Builder
	b.WriteString("【" + EventLabel(msg.Event) + "】" + title)

	if msg.Content != "" {
		b.WriteString("\n" + msg.Content)
	}

	for _, k := range orderedKeys(msg.Fields) {
		b.WriteString("\n" + k + "：" + msg.Fields[k])
	}

	if msg.Link != "" {
		b.WriteString("\n详情：" + msg.Link)
	}

	return b.String()
}

// composeMarkdown 组装 markdown 消息（钉钉 / 企业微信）。
func composeMarkdown(title string, msg *Message) string {
	var b strings.Builder
	b.WriteString("### 【" + EventLabel(msg.Event) + "】" + title)

	if msg.Content != "" {
		b.WriteString("\n\n" + msg.Content)
	}

	if len(msg.Fields) > 0 {
		b.WriteString("\n")
		for _, k := range orderedKeys(msg.Fields) {
			b.WriteString("\n> **" + k + "**：" + msg.Fields[k])
		}
	}

	if msg.Link != "" {
		b.WriteString("\n\n[查看详情](" + msg.Link + ")")
	}

	return b.String()
}

// orderedKeys 让字段输出顺序稳定（Go map 遍历无序，会导致消息每次都不同）。
// 采用「常见字段优先 + 其余字典序」的固定顺序。
func orderedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}

	preferred := []string{"设备名称", "设备编号", "所属区域", "设备位置", "风险类型", "风险等级",
		"告警类型", "告警等级", "任务名称", "巡检人", "发生时间", "处理状态", "检测准确", "置信度"}

	seen := make(map[string]bool, len(m))
	out := make([]string, 0, len(m))

	for _, k := range preferred {
		if _, ok := m[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}

	rest := make([]string, 0, len(m))
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sortStrings(rest)
	out = append(out, rest...)

	return out
}

// sortStrings 简单插入排序，避免为几行代码引入 sort 依赖之外的复杂度。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// checkPlatformError 检查平台业务错误码。
func checkPlatformError(platform string, body []byte) error {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil // 非 JSON 响应（自定义平台常见），交由 HTTP 状态码判断
	}

	switch platform {
	case model.WebhookPlatformFeishu:
		// {"code":0} 或 {"StatusCode":0} 为成功
		if code, ok := numOf(resp["code"]); ok && code != 0 {
			return fmt.Errorf("飞书返回错误 code=%v msg=%v", code, resp["msg"])
		}
		if code, ok := numOf(resp["StatusCode"]); ok && code != 0 {
			return fmt.Errorf("飞书返回错误 StatusCode=%v msg=%v", code, resp["StatusMessage"])
		}

	case model.WebhookPlatformDingtalk:
		if code, ok := numOf(resp["errcode"]); ok && code != 0 {
			return fmt.Errorf("钉钉返回错误 errcode=%v errmsg=%v", code, resp["errmsg"])
		}

	case model.WebhookPlatformWecom:
		if code, ok := numOf(resp["errcode"]); ok && code != 0 {
			return fmt.Errorf("企业微信返回错误 errcode=%v errmsg=%v", code, resp["errmsg"])
		}
	}

	return nil
}

// numOf 把 JSON 数字（float64）或字符串数字转成 int。
func numOf(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case string:
		n := 0
		if t == "" {
			return 0, false
		}
		for _, ch := range t {
			if ch < '0' || ch > '9' {
				return 0, false
			}
			n = n*10 + int(ch-'0')
		}
		return n, true
	default:
		return 0, false
	}
}

func secretOf(cfg *model.WebhookConfig) string {
	if cfg.Secret == nil {
		return ""
	}
	return *cfg.Secret
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
