package repository

import (
	"context"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// NotificationRepo 站内通知（ac_notification）数据访问。
//
// 所有读写方法都携带 userID 作为过滤条件，保证用户只能操作自己的通知；
// 跨用户访问表现为「记录不存在」。
type NotificationRepo struct{}

// NewNotificationRepo 构造通知仓储。
func NewNotificationRepo() *NotificationRepo { return &NotificationRepo{} }

// notifQuery 组装通知查询条件（含 user_id 隔离）。
func notifQuery(db *gorm.DB, userID int64, req *request.PageNotificationReq) *gorm.DB {
	q := db.Model(&model.Notification{}).Where("user_id = ?", userID)
	if req.Category != "" {
		q = q.Where("category = ?", req.Category)
	}
	if req.Read != nil {
		q = q.Where("read = ?", *req.Read)
	}
	return q
}

// Page 分页查询当前用户的通知（按创建时间倒序）。
func (r *NotificationRepo) Page(ctx context.Context, userID int64, req *request.PageNotificationReq) ([]model.Notification, int64, error) {
	db := bootstrap.DB().WithContext(ctx)

	var total int64
	if err := notifQuery(db, userID, req).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.Notification
	if err := notifQuery(db, userID, req).
		Order("created_dt DESC, id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UnreadCount 统计当前用户未读通知数量。
func (r *NotificationRepo) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).Count(&n).Error
	return n, err
}

// GetByID 查询当前用户的单条通知。
func (r *NotificationRepo) GetByID(ctx context.Context, userID, id int64) (*model.Notification, error) {
	var n model.Notification
	if err := bootstrap.DB().WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).First(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// Create 新增通知（供风险/告警/巡检等服务推送）。
func (r *NotificationRepo) Create(ctx context.Context, n *model.Notification) error {
	return bootstrap.DB().WithContext(ctx).Create(n).Error
}

// MarkRead 标记当前用户的单条通知为已读；返回受影响行数（0 表示通知不存在）。
func (r *NotificationRepo) MarkRead(ctx context.Context, userID, id int64) (int64, error) {
	now := time.Now()
	tx := bootstrap.DB().WithContext(ctx).Model(&model.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"read": true, "read_dt": now, "updated_dt": now})
	return tx.RowsAffected, tx.Error
}

// MarkAllRead 标记当前用户全部未读通知为已读，返回受影响行数。
func (r *NotificationRepo) MarkAllRead(ctx context.Context, userID int64) (int64, error) {
	now := time.Now()
	tx := bootstrap.DB().WithContext(ctx).Model(&model.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).
		Updates(map[string]interface{}{"read": true, "read_dt": now, "updated_dt": now})
	return tx.RowsAffected, tx.Error
}

// Delete 删除当前用户的单条通知（软删除），返回受影响行数。
func (r *NotificationRepo) Delete(ctx context.Context, userID, id int64) (int64, error) {
	tx := bootstrap.DB().WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Notification{})
	return tx.RowsAffected, tx.Error
}
