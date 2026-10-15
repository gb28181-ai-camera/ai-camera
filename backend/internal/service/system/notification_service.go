package system

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// 通知分类常量（与 model.Notification.Category 取值一致）。
const (
	NotifyCategoryAlarm   = "alarm"   // 设备告警
	NotifyCategoryRisk    = "risk"    // 风险通知
	NotifyCategoryInspect = "inspect" // 巡检通知
)

// NotificationService 站内通知业务逻辑。
//
// 所有读写方法都要求传入 userID：用户只能查看与操作自己的通知，
// 跨用户访问按「记录不存在」处理。
type NotificationService struct{}

// NewNotificationService 构造通知服务。
func NewNotificationService() *NotificationService { return &NotificationService{} }

// Page 分页查询当前用户的通知。
func (s *NotificationService) Page(ctx context.Context, userID int64, req *request.PageNotificationReq) (*response.PageNotificationResp, error) {
	req.Normalize()

	list, total, err := repository.NewNotificationRepo().Page(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	resp := &response.PageNotificationResp{Items: list}
	resp.Total = total
	resp.Page = req.Page
	resp.PageSize = req.PageSize
	return resp, nil
}

// UnreadCount 返回当前用户的未读通知数量。
func (s *NotificationService) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	return repository.NewNotificationRepo().UnreadCount(ctx, userID)
}

// MarkRead 标记当前用户的单条通知为已读。
func (s *NotificationService) MarkRead(ctx context.Context, userID, id int64) error {
	rows, err := repository.NewNotificationRepo().MarkRead(ctx, userID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return bizErr("通知不存在")
	}
	return nil
}

// MarkAllRead 标记当前用户全部未读通知为已读。
func (s *NotificationService) MarkAllRead(ctx context.Context, userID int64) error {
	_, err := repository.NewNotificationRepo().MarkAllRead(ctx, userID)
	return err
}

// Delete 删除当前用户的单条通知。
func (s *NotificationService) Delete(ctx context.Context, userID, id int64) error {
	rows, err := repository.NewNotificationRepo().Delete(ctx, userID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return bizErr("通知不存在")
	}
	return nil
}

// Push 推送一条站内通知，供风险 / 告警 / 巡检等服务调用。
//
// category 取值 alarm / risk / inspect；refType + refID 指向来源业务记录。
func (s *NotificationService) Push(ctx context.Context, userID int64, category, title, content, refType string, refID int64) error {
	if userID <= 0 {
		return bizErr("通知接收人不能为空")
	}
	if category == "" {
		category = NotifyCategoryAlarm
	}

	unread := false
	n := &model.Notification{
		UserID:   userID,
		Category: category,
		Title:    title,
		Content:  content,
		RefType:  refType,
		RefID:    refID,
		Read:     &unread,
	}
	return repository.NewNotificationRepo().Create(ctx, n)
}
