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
// IOT 管理 - 服务组 / 服务
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
		q = q.Where("(name ILIKE ? OR code ILIKE ?)", like, like)
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

// ListServices 查询服务组下的服务。
func (r *IotServiceGroupRepo) ListServices(ctx context.Context, groupID int64) ([]model.IotService, error) {
	var items []model.IotService
	err := bootstrap.DB().WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("sequence ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ListServicesByGroupIDs 批量查询多个服务组下的服务。
func (r *IotServiceGroupRepo) ListServicesByGroupIDs(ctx context.Context, groupIDs []int64) ([]model.IotService, error) {
	if len(groupIDs) == 0 {
		return []model.IotService{}, nil
	}
	var items []model.IotService
	err := bootstrap.DB().WithContext(ctx).
		Where("group_id IN ?", groupIDs).
		Order("group_id ASC, sequence ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetService 按主键查询服务。
func (r *IotServiceGroupRepo) GetService(ctx context.Context, id int64) (*model.IotService, error) {
	var m model.IotService
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateService 新增单个服务（供测试/初始化使用）。
func (r *IotServiceGroupRepo) CreateService(ctx context.Context, m *model.IotService) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
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

// Create 新增服务组（仅供测试/初始化使用；业务接口不开放新增）。
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

// ReplaceServices 整体替换服务组下的服务。
//
// 使用硬删除：软删除行会继续占用 (group_id, code) 唯一索引，
// 导致同一服务编码无法重新加入。
func (r *IotServiceGroupRepo) ReplaceServices(ctx context.Context, groupID int64, services []model.IotService) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("group_id = ?", groupID).
			Delete(&model.IotService{}).Error; err != nil {
			return err
		}
		if len(services) == 0 {
			return nil
		}
		for i := range services {
			services[i].GroupID = groupID
			services[i].ID = 0
		}
		return tx.Create(&services).Error
	})
}
