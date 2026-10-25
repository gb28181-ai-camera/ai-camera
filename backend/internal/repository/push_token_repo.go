package repository

import (
	"context"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// PushTokenRepo App 推送令牌。
type PushTokenRepo struct{}

// NewPushTokenRepo 构造仓储。
func NewPushTokenRepo() *PushTokenRepo { return &PushTokenRepo{} }

// Register 幂等注册：同一 CID 换用户时迁移到新用户。
func (r *PushTokenRepo) Register(ctx context.Context, userID int64, cid, platform, appVersion string) error {
	now := time.Now()
	var existing model.UserPushToken
	err := bootstrap.DB().WithContext(ctx).Unscoped().Where("cid = ?", cid).First(&existing).Error
	if err == nil {
		return bootstrap.DB().WithContext(ctx).Model(&model.UserPushToken{}).
			Where("id = ?", existing.ID).
			Updates(map[string]interface{}{
				"user_id": userID, "platform": platform, "app_version": appVersion,
				"last_seen_dt": now, "deleted_at": nil,
			}).Error
	}
	return bootstrap.DB().WithContext(ctx).Create(&model.UserPushToken{
		UserID: userID, CID: cid, Platform: platform, AppVersion: appVersion, LastSeenDT: now,
	}).Error
}

// ListByUserIDs 返回用户集合的有效 CID。
func (r *PushTokenRepo) ListByUserIDs(ctx context.Context, userIDs []int64) ([]model.UserPushToken, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var items []model.UserPushToken
	err := bootstrap.DB().WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Order("last_seen_dt DESC").
		Find(&items).Error
	return items, err
}

// Delete 删除一个 CID（App 退出登录/关闭推送时调用）。
func (r *PushTokenRepo) Delete(ctx context.Context, userID int64, cid string) error {
	return bootstrap.DB().WithContext(ctx).
		Where("user_id = ? AND cid = ?", userID, cid).
		Delete(&model.UserPushToken{}).Error
}
