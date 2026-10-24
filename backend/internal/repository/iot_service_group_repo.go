package repository

import (
	"context"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 服务组 / 服务项
// ════════════════════════════════════════════════════════════════

// IotServiceGroupRepo IOT 服务组数据访问层。
type IotServiceGroupRepo struct{}

// NewIotServiceGroupRepo 构造 IOT 服务组仓储。
func NewIotServiceGroupRepo() *IotServiceGroupRepo { return &IotServiceGroupRepo{} }

// Page 分页查询服务组。
func (r *IotServiceGroupRepo) Page(ctx context.Context, req *request.PageIotServiceGroupReq) ([]model.IotServiceGroup, int64, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.IotServiceGroup{})
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(name ILIKE ? OR code ILIKE ? OR username ILIKE ?)", like, like, like)
	}
	if typ := strings.TrimSpace(req.Type); typ != "" {
		q = q.Where("type = ?", typ)
	}
	if req.Active != nil {
		q = q.Where("active = ?", *req.Active)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.IotServiceGroup
	if err := q.Session(&gorm.Session{}).
		Order("id ASC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID 按主键查询服务组。
func (r *IotServiceGroupRepo) GetByID(ctx context.Context, id int64) (*model.IotServiceGroup, error) {
	var m model.IotServiceGroup
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByUsername 按组级登录名查询服务组。
//
// MQTT 鉴权只需用户名定位；密码比对在 service 层做（需要 constant-time 比较），
// 避免把口令比较散落到 SQL 里。
func (r *IotServiceGroupRepo) FindByUsername(ctx context.Context, username string) (*model.IotServiceGroup, error) {
	var m model.IotServiceGroup
	if err := bootstrap.DB().WithContext(ctx).
		Where("username = ?", username).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListItems 查询服务组的服务项。
func (r *IotServiceGroupRepo) ListItems(ctx context.Context, groupID int64) ([]model.IotServiceItem, error) {
	var items []model.IotServiceItem
	err := bootstrap.DB().WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("sequence ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ExistsByCode 判断服务组编码是否存在（excludeID>0 时排除自身）。
func (r *IotServiceGroupRepo) ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.IotServiceGroup{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ExistsByUsername 判断组级登录名是否已被占用（自动生成时需要重试）。
func (r *IotServiceGroupRepo) ExistsByUsername(ctx context.Context, username string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.IotServiceGroup{}).Where("username = ?", username)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// Create 新增服务组。
func (r *IotServiceGroupRepo) Create(ctx context.Context, m *model.IotServiceGroup) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 保存服务组。
func (r *IotServiceGroupRepo) Update(ctx context.Context, m *model.IotServiceGroup) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除服务组。
func (r *IotServiceGroupRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.IotServiceGroup{}, id).Error
}

// UpdateCredentials 只更新用户名与密码。
func (r *IotServiceGroupRepo) UpdateCredentials(ctx context.Context, id int64, username, password string) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.IotServiceGroup{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"username": username, "password": password}).Error
}

// ListByIDs 按 ID 批量查询服务组（绑定前校验存在性）。
func (r *IotServiceGroupRepo) ListByIDs(ctx context.Context, ids []int64) ([]model.IotServiceGroup, error) {
	if len(ids) == 0 {
		return []model.IotServiceGroup{}, nil
	}
	var items []model.IotServiceGroup
	if err := bootstrap.DB().WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ReplaceItems 整体替换服务项。
//
// 与产品绑定同理，使用硬删除：若软删除，(group_id, protocol) 唯一索引
// 仍被已删除的行占用，同一协议将永远无法重新加入。
func (r *IotServiceGroupRepo) ReplaceItems(ctx context.Context, groupID int64, items []model.IotServiceItem) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("group_id = ?", groupID).
			Delete(&model.IotServiceItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for i := range items {
			items[i].GroupID = groupID
			items[i].ID = 0
		}
		return tx.Create(&items).Error
	})
}
