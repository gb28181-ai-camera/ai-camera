package repository

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 产品-服务凭据
// ════════════════════════════════════════════════════════════════

// IotProductServiceRepo 产品-服务凭据数据访问层。
type IotProductServiceRepo struct{}

// NewIotProductServiceRepo 构造产品-服务凭据仓储。
func NewIotProductServiceRepo() *IotProductServiceRepo { return &IotProductServiceRepo{} }

// ListByProduct 查询产品下全部服务凭据。
func (r *IotProductServiceRepo) ListByProduct(ctx context.Context, productID int64) ([]model.IotProductService, error) {
	var items []model.IotProductService
	err := bootstrap.DB().WithContext(ctx).
		Where("product_id = ?", productID).
		Order("service_id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Get 按产品与服务查询凭据。
func (r *IotProductServiceRepo) Get(ctx context.Context, productID, serviceID int64) (*model.IotProductService, error) {
	var m model.IotProductService
	if err := bootstrap.DB().WithContext(ctx).
		Where("product_id = ? AND service_id = ?", productID, serviceID).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByUsername 按服务登录名查询凭据（MQTT 鉴权入口）。
func (r *IotProductServiceRepo) FindByUsername(ctx context.Context, username string) (*model.IotProductService, error) {
	var m model.IotProductService
	if err := bootstrap.DB().WithContext(ctx).
		Where("username = ?", username).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsUsername 判断服务登录名是否已被占用（自动生成时重试）。
func (r *IotProductServiceRepo) ExistsUsername(ctx context.Context, username string) (bool, error) {
	var cnt int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.IotProductService{}).
		Where("username = ?", username).
		Count(&cnt).Error
	return cnt > 0, err
}

// Create 新增产品-服务凭据。
func (r *IotProductServiceRepo) Create(ctx context.Context, m *model.IotProductService) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 保存产品-服务凭据。
func (r *IotProductServiceRepo) Update(ctx context.Context, m *model.IotProductService) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 删除单条产品-服务凭据。
func (r *IotProductServiceRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.IotProductService{}, id).Error
}

// DeleteByProduct 删除产品的全部服务凭据。
func (r *IotProductServiceRepo) DeleteByProduct(ctx context.Context, productID int64) error {
	return bootstrap.DB().WithContext(ctx).
		Where("product_id = ?", productID).
		Delete(&model.IotProductService{}).Error
}

// DeleteByProductAndServices 删除产品在指定服务上的凭据。
func (r *IotProductServiceRepo) DeleteByProductAndServices(ctx context.Context, productID int64, serviceIDs []int64) error {
	if len(serviceIDs) == 0 {
		return nil
	}
	return bootstrap.DB().WithContext(ctx).
		Where("product_id = ? AND service_id IN ?", productID, serviceIDs).
		Delete(&model.IotProductService{}).Error
}

// DeleteByService 删除某个服务下的全部产品凭据（服务被移除时）。
func (r *IotProductServiceRepo) DeleteByService(ctx context.Context, serviceID int64) error {
	return bootstrap.DB().WithContext(ctx).
		Where("service_id = ?", serviceID).
		Delete(&model.IotProductService{}).Error
}
