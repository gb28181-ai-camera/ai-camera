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
// IOT 管理 - 产品 / 产品-服务组绑定
// ════════════════════════════════════════════════════════════════

// IotProductRepo IOT 产品数据访问层。
type IotProductRepo struct{}

// NewIotProductRepo 构造 IOT 产品仓储。
func NewIotProductRepo() *IotProductRepo { return &IotProductRepo{} }

// Page 分页查询产品。
func (r *IotProductRepo) Page(ctx context.Context, req *request.PageIotProductReq) ([]model.IotProduct, int64, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.IotProduct{})
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(name ILIKE ? OR code ILIKE ? OR category ILIKE ?)", like, like, like)
	}
	if req.Active != nil {
		q = q.Where("active = ?", *req.Active)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.IotProduct
	if err := q.Session(&gorm.Session{}).
		Order("sequence ASC, id ASC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID 按主键查询产品。
func (r *IotProductRepo) GetByID(ctx context.Context, id int64) (*model.IotProduct, error) {
	var m model.IotProduct
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsByCode 判断产品编码是否存在（excludeID>0 时排除自身）。
func (r *IotProductRepo) ExistsByCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.IotProduct{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// Create 新增产品。
func (r *IotProductRepo) Create(ctx context.Context, m *model.IotProduct) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 保存产品。
func (r *IotProductRepo) Update(ctx context.Context, m *model.IotProduct) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除产品。
func (r *IotProductRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.IotProduct{}, id).Error
}

// UpdateShareQR 只更新分享二维码内容，避免 Save 覆盖并发修改的其它字段。
func (r *IotProductRepo) UpdateShareQR(ctx context.Context, id int64, content string) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.IotProduct{}).
		Where("id = ?", id).
		Update("share_qr_content", content).Error
}

// ListServiceGroups 查询产品已绑定的服务组。
//
// 用显式 JOIN 而不是 Preload：ac_iot_product_service 是纯关联表，
// 需要按服务组的 sequence 排序，JOIN 一次取回比两次查询更直接。
func (r *IotProductRepo) ListServiceGroups(ctx context.Context, productID int64) ([]model.IotServiceGroup, error) {
	var items []model.IotServiceGroup
	err := bootstrap.DB().WithContext(ctx).
		Table("ac_iot_service_group AS sg").
		Select("sg.*").
		Joins("JOIN ac_iot_product_service AS ps ON ps.service_group_id = sg.id").
		Where("ps.product_id = ? AND ps.deleted_at IS NULL AND sg.deleted_at IS NULL", productID).
		Order("sg.id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ReplaceServiceGroups 整体替换产品绑定的服务组。
//
// 采用**硬删除**后重建：ac_iot_product_service 上有 (product_id, service_group_id)
// 唯一索引，软删除的行仍会占用索引，导致解绑后无法再次绑定同一服务组。
func (r *IotProductRepo) ReplaceServiceGroups(ctx context.Context, productID int64, groupIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("product_id = ?", productID).
			Delete(&model.IotProductService{}).Error; err != nil {
			return err
		}
		if len(groupIDs) == 0 {
			return nil
		}
		rows := make([]model.IotProductService, 0, len(groupIDs))
		seen := make(map[int64]struct{}, len(groupIDs))
		for _, gid := range groupIDs {
			if _, ok := seen[gid]; ok {
				continue // 请求里重复的 ID 去重，避免撞唯一索引
			}
			seen[gid] = struct{}{}
			rows = append(rows, model.IotProductService{ProductID: productID, ServiceGroupID: gid})
		}
		return tx.Create(&rows).Error
	})
}

// CountByGroup 统计绑定了某服务组的产品数量（删除服务组前的引用检查）。
func (r *IotProductRepo) CountByGroup(ctx context.Context, groupID int64) (int64, error) {
	var cnt int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.IotProductService{}).
		Where("service_group_id = ?", groupID).
		Count(&cnt).Error
	return cnt, err
}
