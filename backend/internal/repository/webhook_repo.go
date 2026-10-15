package repository

import (
	"context"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// WebhookRepo WebHook 配置数据访问。
//
// WebHook 属平台级配置（不按部门隔离），因此不施加 DataScope。
type WebhookRepo struct{}

// NewWebhookRepo 构造仓储。
func NewWebhookRepo() *WebhookRepo { return &WebhookRepo{} }

func (r *WebhookRepo) query() *gorm.DB {
	return bootstrap.DB().Model(&model.WebhookConfig{})
}

// Page 分页查询 WebHook 配置。
func (r *WebhookRepo) Page(ctx context.Context, req *request.PageWebhookReq) ([]model.WebhookConfig, int64, error) {
	q := r.query().WithContext(ctx)

	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		q = q.Where("name ILIKE ?", "%"+kw+"%")
	}
	if p := strings.TrimSpace(req.Platform); p != "" {
		q = q.Where("platform = ?", p)
	}
	if req.Active != nil {
		q = q.Where("active = ?", *req.Active)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	req.Normalize()
	var items []model.WebhookConfig
	err := q.Order("sequence ASC, id ASC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error

	return items, total, err
}

// ListActiveByEvent 查询订阅了指定事件且已启用的配置，供消息分发使用。
func (r *WebhookRepo) ListActiveByEvent(ctx context.Context, event string) ([]model.WebhookConfig, error) {
	var items []model.WebhookConfig
	err := r.query().WithContext(ctx).
		Where("active = true").
		Order("sequence ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}

	// EventTypes 是逗号分隔的多值列，用 LIKE 会误匹配（如 risk 命中 risky），
	// 因此取回后在内存里精确过滤。
	out := make([]model.WebhookConfig, 0, len(items))
	for _, it := range items {
		if it.Subscribes(event) {
			out = append(out, it)
		}
	}
	return out, nil
}

// GetByID 按 ID 查询。
func (r *WebhookRepo) GetByID(ctx context.Context, id int64) (*model.WebhookConfig, error) {
	var m model.WebhookConfig
	if err := r.query().WithContext(ctx).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增。
func (r *WebhookRepo) Create(ctx context.Context, m *model.WebhookConfig) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全字段更新（由 service 组装好完整实体）。
func (r *WebhookRepo) Update(ctx context.Context, m *model.WebhookConfig) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.WebhookConfig{}).
		Where("id = ?", m.ID).
		Select("name", "platform", "url", "secret", "msg_type", "event_types",
			"sequence", "active", "remark", "updated_by", "updated_dt").
		Updates(m).Error
}

// Delete 软删除。
func (r *WebhookRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Where("id = ?", id).Delete(&model.WebhookConfig{}).Error
}

// SaveSendResult 记录最近一次推送结果。
func (r *WebhookRepo) SaveSendResult(ctx context.Context, id int64, status, errMsg string) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.WebhookConfig{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_send_dt": gorm.Expr("NOW()"),
			"last_status":  status,
			"last_error":   errMsg,
		}).Error
}
