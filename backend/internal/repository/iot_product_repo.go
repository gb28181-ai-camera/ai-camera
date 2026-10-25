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
// 用显式 JOIN 而不是 Preload：ac_iot_product_service_group 是纯关联表，
// 需要按服务组的 sequence 排序，JOIN 一次取回比两次查询更直接。
func (r *IotProductRepo) ListServiceGroups(ctx context.Context, productID int64) ([]model.IotServiceGroup, error) {
	var items []model.IotServiceGroup
	err := bootstrap.DB().WithContext(ctx).
		Table("ac_iot_service_group AS sg").
		Select("sg.*").
		Joins("JOIN ac_iot_product_service_group AS psg ON psg.service_group_id = sg.id").
		Where("psg.product_id = ? AND psg.deleted_at IS NULL AND sg.deleted_at IS NULL", productID).
		Order("sg.id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ListServiceGroupBriefsByProductIDs 批量查询多个产品绑定的服务组摘要。
//
// 列表页一次查完，避免逐行调用 ListServiceGroups 造成 N+1。
func (r *IotProductRepo) ListServiceGroupBriefsByProductIDs(
	ctx context.Context, productIDs []int64,
) (map[int64][]model.IotServiceGroupBrief, error) {
	out := make(map[int64][]model.IotServiceGroupBrief)
	if len(productIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ProductID int64
		ID        int64
		Code      string
		Name      string
		Type      string
		Active    *bool
	}
	err := bootstrap.DB().WithContext(ctx).
		Table("ac_iot_product_service_group AS psg").
		Select("psg.product_id, sg.id, sg.code, sg.name, sg.type, sg.active").
		Joins("JOIN ac_iot_service_group AS sg ON sg.id = psg.service_group_id").
		Where("psg.product_id IN ?", productIDs).
		Where("psg.deleted_at IS NULL AND sg.deleted_at IS NULL").
		Order("psg.product_id ASC, sg.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProductID] = append(out[row.ProductID], model.IotServiceGroupBrief{
			ID: row.ID, Code: row.Code, Name: row.Name, Type: row.Type, Active: row.Active,
		})
	}
	return out, nil
}

// ReplaceServiceGroups 整体替换产品绑定的服务组。
//
// 采用**硬删除**后重建：关联表上有 (product_id, service_group_id)
// 唯一索引，软删除的行仍会占用索引，导致解绑后无法再次绑定同一服务组。
func (r *IotProductRepo) ReplaceServiceGroups(ctx context.Context, productID int64, groupIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().
			Where("product_id = ?", productID).
			Delete(&model.IotProductServiceGroup{}).Error; err != nil {
			return err
		}
		if len(groupIDs) == 0 {
			return nil
		}
		rows := make([]model.IotProductServiceGroup, 0, len(groupIDs))
		seen := make(map[int64]struct{}, len(groupIDs))
		for _, gid := range groupIDs {
			if _, ok := seen[gid]; ok {
				continue // 请求里重复的 ID 去重，避免撞唯一索引
			}
			seen[gid] = struct{}{}
			rows = append(rows, model.IotProductServiceGroup{ProductID: productID, ServiceGroupID: gid})
		}
		return tx.Create(&rows).Error
	})
}

// CountDevices 统计绑定到某产品的设备数量。
//
// 用于产品类型/配置更新时的兼容策略：已有设备的历史产品只警告不阻止，
// 避免「类型校验把老数据锁死、连改名都保存不了」。
func (r *IotProductRepo) CountDevices(ctx context.Context, productID int64) (int64, error) {
	var cnt int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("product_id = ?", productID).
		Count(&cnt).Error
	return cnt, err
}

// CountByGroup 统计绑定了某服务组的产品数量（删除服务组前的引用检查）。
func (r *IotProductRepo) CountByGroup(ctx context.Context, groupID int64) (int64, error) {
	var cnt int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.IotProductServiceGroup{}).
		Joins("JOIN ac_iot_product p ON p.id = ac_iot_product_service_group.product_id AND p.deleted_at IS NULL").
		Where("service_group_id = ?", groupID).
		Count(&cnt).Error
	return cnt, err
}

// ReferenceCountsByGroup 批量统计每个服务组被多少个产品引用。
//
// 服务组列表需要展示「是否已被产品引用」以决定能否编辑；
// 一次聚合查询替代逐行 Count，避免列表 N+1。
func (r *IotProductRepo) ReferenceCountsByGroup(ctx context.Context) (map[int64]int64, error) {
	var rows []struct {
		ServiceGroupID int64
		Cnt            int64
	}
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.IotProductServiceGroup{}).
		Joins("JOIN ac_iot_product p ON p.id = ac_iot_product_service_group.product_id AND p.deleted_at IS NULL").
		Select("service_group_id, COUNT(*) AS cnt").
		Group("service_group_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(rows))
	for _, row := range rows {
		out[row.ServiceGroupID] = row.Cnt
	}
	return out, nil
}
