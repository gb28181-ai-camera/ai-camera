package setting

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/pkg/webhook"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WebhookService 消息推送 WebHook 配置与分发。
type WebhookService struct {
	repo *repository.WebhookRepo
}

// NewWebhookService 构造服务。
func NewWebhookService() *WebhookService {
	return &WebhookService{repo: repository.NewWebhookRepo()}
}

// ── 配置管理 ──────────────────────────────────────────────────

// Page 分页查询。
func (s *WebhookService) Page(ctx context.Context, req *request.PageWebhookReq) (*response.PageWebhookResp, error) {
	items, total, err := s.repo.Page(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("查询 WebHook 配置失败: %w", err)
	}
	req.Normalize()
	return &response.PageWebhookResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Get 按 ID 查询。
func (s *WebhookService) Get(ctx context.Context, id int64) (*model.WebhookConfig, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("WebHook 配置不存在")
		}
		return nil, fmt.Errorf("查询 WebHook 配置失败: %w", err)
	}
	return m, nil
}

// Create 新增配置。
func (s *WebhookService) Create(ctx context.Context, req *request.CreateWebhookReq) (*model.WebhookConfig, error) {
	if err := validateWebhookReq(&req.WebhookItemReq); err != nil {
		return nil, err
	}

	m := &model.WebhookConfig{
		Name:       strings.TrimSpace(req.Name),
		Platform:   req.Platform,
		URL:        strings.TrimSpace(req.URL),
		Secret:     normalizeSecret(req.Secret),
		MsgType:    defaultMsgType(req.MsgType),
		EventTypes: joinEvents(req.EventTypes),
		Sequence:   req.Sequence,
		Active:     boolOr(req.Active, true),
		Remark:     req.Remark,
	}
	if m.Sequence == 0 {
		m.Sequence = 10
	}

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("创建 WebHook 配置失败: %w", err)
	}
	return m, nil
}

// Update 编辑配置。
func (s *WebhookService) Update(ctx context.Context, id int64, req *request.UpdateWebhookReq) error {
	if err := validateWebhookReq(&req.WebhookItemReq); err != nil {
		return err
	}

	cur, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	cur.Name = strings.TrimSpace(req.Name)
	cur.Platform = req.Platform
	cur.URL = strings.TrimSpace(req.URL)
	cur.MsgType = defaultMsgType(req.MsgType)
	cur.EventTypes = joinEvents(req.EventTypes)
	cur.Sequence = req.Sequence
	cur.Remark = req.Remark

	if req.Sequence == 0 {
		cur.Sequence = 10
	}
	if req.Active != nil {
		cur.Active = req.Active
	}

	// secret 为空或为脱敏串时保持原值，避免把 ****** 写进库
	if sec := normalizeSecret(req.Secret); sec != nil && !isMasked(*sec) {
		cur.Secret = sec
	}

	if err := s.repo.Update(ctx, cur); err != nil {
		return fmt.Errorf("更新 WebHook 配置失败: %w", err)
	}
	return nil
}

// Delete 删除配置。
func (s *WebhookService) Delete(ctx context.Context, id int64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除 WebHook 配置失败: %w", err)
	}
	return nil
}

// SetActive 启用/停用。
func (s *WebhookService) SetActive(ctx context.Context, id int64, active bool) error {
	cur, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	cur.Active = &active
	return s.repo.Update(ctx, cur)
}

// ── 测试与分发 ────────────────────────────────────────────────

// Test 向已保存的配置发送一条测试消息。
func (s *WebhookService) Test(ctx context.Context, id int64, req *request.TestWebhookReq) (*response.WebhookTestResp, error) {
	cfg, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	msg := &webhook.Message{
		Event:   model.WebhookEventRisk,
		Title:   firstNonEmpty(req.Title, "WebHook 连通性测试"),
		Content: firstNonEmpty(req.Content, "这是一条来自 AI Camera 智能监控平台的测试消息，收到即表示该通道配置正确。"),
		Fields: map[string]string{
			"配置名称": cfg.Name,
			"平台":   platformLabel(cfg.Platform),
		},
	}

	sendErr := webhook.Send(ctx, cfg, msg)
	s.recordResult(ctx, cfg.ID, sendErr)

	if sendErr != nil {
		return &response.WebhookTestResp{Success: false, Message: sendErr.Error()}, nil
	}
	return &response.WebhookTestResp{Success: true, Message: "测试消息已发送，请到对应群聊确认"}, nil
}

// TestInline 未保存配置时的即时测试（不落库）。
func (s *WebhookService) TestInline(ctx context.Context, req *request.TestWebhookInlineReq) (*response.WebhookTestResp, error) {
	if err := validateWebhookReq(&req.WebhookItemReq); err != nil {
		return nil, err
	}

	cfg := &model.WebhookConfig{
		Name:     req.Name,
		Platform: req.Platform,
		URL:      strings.TrimSpace(req.URL),
		Secret:   normalizeSecret(req.Secret),
		MsgType:  defaultMsgType(req.MsgType),
	}

	msg := &webhook.Message{
		Event:   model.WebhookEventRisk,
		Title:   firstNonEmpty(req.Title, "WebHook 连通性测试"),
		Content: firstNonEmpty(req.Content, "这是一条来自 AI Camera 智能监控平台的测试消息，收到即表示该通道配置正确。"),
	}

	if err := webhook.Send(ctx, cfg, msg); err != nil {
		return &response.WebhookTestResp{Success: false, Message: err.Error()}, nil
	}
	return &response.WebhookTestResp{Success: true, Message: "测试消息已发送，请到对应群聊确认"}, nil
}

// Dispatch 按事件类型向所有订阅的通道分发消息。
//
// 该方法被风险上报、告警上报、巡检任务创建等业务路径调用。
// 分发失败只记录到通道的 last_status/last_error，**不影响主业务流程**。
func (s *WebhookService) Dispatch(ctx context.Context, msg *webhook.Message) {
	if msg == nil || msg.Event == "" {
		return
	}

	cfgs, err := s.repo.ListActiveByEvent(ctx, msg.Event)
	if err != nil {
		utils.Warn("查询 WebHook 通道失败", zap.Error(err))
		return
	}
	if len(cfgs) == 0 {
		return
	}

	for i := range cfgs {
		cfg := cfgs[i]
		sendErr := webhook.Send(ctx, &cfg, msg)
		s.recordResult(ctx, cfg.ID, sendErr)
	}
}

// recordResult 记录推送结果（best-effort，失败仅写日志）。
func (s *WebhookService) recordResult(ctx context.Context, id int64, sendErr error) {
	status := "success"
	errMsg := ""
	if sendErr != nil {
		status = "failed"
		errMsg = sendErr.Error()
		utils.Warn("WebHook 推送失败", zap.Int64("id", id), zap.Error(sendErr))
	}
	if err := s.repo.SaveSendResult(ctx, id, status, errMsg); err != nil {
		utils.Warn("记录 WebHook 推送结果失败", zap.Error(err))
	}
}

// Platforms 返回支持的平台选项。
func (s *WebhookService) Platforms() []response.WebhookPlatformItem {
	return []response.WebhookPlatformItem{
		{Value: model.WebhookPlatformFeishu, Label: "飞书"},
		{Value: model.WebhookPlatformDingtalk, Label: "钉钉"},
		{Value: model.WebhookPlatformWecom, Label: "企业微信"},
		{Value: model.WebhookPlatformCustom, Label: "自定义 WebHook"},
	}
}

// ── 辅助 ──────────────────────────────────────────────────────

func validateWebhookReq(req *request.WebhookItemReq) error {
	if strings.TrimSpace(req.Name) == "" {
		return errors.New("配置名称不能为空")
	}
	switch req.Platform {
	case model.WebhookPlatformFeishu, model.WebhookPlatformDingtalk,
		model.WebhookPlatformWecom, model.WebhookPlatformCustom:
	default:
		return errors.New("平台类型不合法")
	}
	u := strings.TrimSpace(req.URL)
	if u == "" {
		return errors.New("WebHook 地址不能为空")
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return errors.New("WebHook 地址必须以 http:// 或 https:// 开头")
	}
	return nil
}

func defaultMsgType(t string) string {
	if t == model.WebhookMsgText {
		return model.WebhookMsgText
	}
	return model.WebhookMsgMarkdown
}

// normalizeSecret 空串归一为 nil，避免把空密钥写库。
func normalizeSecret(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func isMasked(s string) bool {
	return strings.Contains(s, "****")
}

// joinEvents 把事件数组拼成逗号分隔串；空数组表示订阅全部。
func joinEvents(events []string) string {
	if len(events) == 0 {
		return ""
	}
	valid := make([]string, 0, len(events))
	for _, e := range events {
		switch strings.TrimSpace(e) {
		case model.WebhookEventRisk, model.WebhookEventAlarm,
			model.WebhookEventInspect, model.WebhookEventDevice:
			valid = append(valid, strings.TrimSpace(e))
		}
	}
	return strings.Join(valid, ",")
}

func boolOr(v *bool, def bool) *bool {
	if v != nil {
		return v
	}
	return &def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func platformLabel(p string) string {
	switch p {
	case model.WebhookPlatformFeishu:
		return "飞书"
	case model.WebhookPlatformDingtalk:
		return "钉钉"
	case model.WebhookPlatformWecom:
		return "企业微信"
	default:
		return "自定义 WebHook"
	}
}
