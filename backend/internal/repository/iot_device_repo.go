package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 设备（复用 ac_device）/ 设备账号 / 设备事件
//
// 设备表不另建：IOT 只是给既有设备增加产品归属与接入密钥。
// 本仓储只做 IOT 视角需要的读写，设备本身的 CRUD 仍在 DeviceRepo。
// ════════════════════════════════════════════════════════════════

// IotDeviceRepo IOT 设备数据访问层。
type IotDeviceRepo struct{}

// NewIotDeviceRepo 构造 IOT 设备仓储。
func NewIotDeviceRepo() *IotDeviceRepo { return &IotDeviceRepo{} }

// Page 分页查询设备（未删除），支持产品与关键字筛选。
//
// 刻意不施加 DataScope：IOT 管理页的诉求是「列举所有设备」，设备按产品组织，
// 与区域/部门的数据权限无关。真实的数据隔离由 Casbin 接口权限码
// iot:device:list 控制。
func (r *IotDeviceRepo) Page(ctx context.Context, req *request.PageIotDeviceReq) ([]model.Device, int64, error) {
	q := bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Table("ac_device AS d")

	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(d.name ILIKE ? OR d.device_code ILIKE ? OR d.location ILIKE ?)", like, like, like)
	}
	if req.ProductID != nil {
		q = q.Where("d.product_id = ?", *req.ProductID)
	}
	if s := strings.TrimSpace(req.Status); s != "" {
		q = q.Where("d.status = ?", s)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.Device
	if err := q.Session(&gorm.Session{}).
		Preload("Product").
		Order("d.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID 按主键查询设备（含产品）。
func (r *IotDeviceRepo) GetByID(ctx context.Context, id int64) (*model.Device, error) {
	var m model.Device
	if err := bootstrap.DB().WithContext(ctx).
		Preload("Product").
		First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByCode 按设备编号查询（设备端 OTA/激活鉴权用）。
func (r *IotDeviceRepo) GetByCode(ctx context.Context, code string) (*model.Device, error) {
	var m model.Device
	if err := bootstrap.DB().WithContext(ctx).
		Where("device_code = ?", code).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// SetProduct 设置设备所属产品（nil 表示解绑）。
func (r *IotDeviceRepo) SetProduct(ctx context.Context, id int64, productID *int64) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", id).
		Update("product_id", productID).Error
}

// SetSecret 写入/轮换设备接入密钥。
func (r *IotDeviceRepo) SetSecret(ctx context.Context, id int64, secret string) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", id).
		Update("device_secret", secret).Error
}

// SetActivatedAt 记录设备主账号首次激活时间。
func (r *IotDeviceRepo) SetActivatedAt(ctx context.Context, id int64, t time.Time) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", id).
		Update("activated_at", t).Error
}

// ── 设备账号 ──────────────────────────────────────────────────

// ListAccounts 查询设备账号（按角色、ID 排序，owner 在前）。
func (r *IotDeviceRepo) ListAccounts(ctx context.Context, deviceID int64) ([]model.IotDeviceAccount, error) {
	var items []model.IotDeviceAccount
	err := bootstrap.DB().WithContext(ctx).
		Where("device_id = ?", deviceID).
		Order("CASE WHEN role = 'owner' THEN 0 ELSE 1 END, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetAccount 按设备与账号 ID 查询（防止跨设备越权修改）。
func (r *IotDeviceRepo) GetAccount(ctx context.Context, deviceID, accountID int64) (*model.IotDeviceAccount, error) {
	var m model.IotDeviceAccount
	if err := bootstrap.DB().WithContext(ctx).
		Where("id = ? AND device_id = ?", accountID, deviceID).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetOwner 查询设备的主账号（激活幂等判断）。
func (r *IotDeviceRepo) GetOwner(ctx context.Context, deviceID int64) (*model.IotDeviceAccount, error) {
	var m model.IotDeviceAccount
	if err := bootstrap.DB().WithContext(ctx).
		Where("device_id = ? AND role = ?", deviceID, model.IotAccountRoleOwner).
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsAccountUsername 判断设备账号登录名是否已被占用。
func (r *IotDeviceRepo) ExistsAccountUsername(ctx context.Context, username string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.IotDeviceAccount{}).Where("username = ?", username)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// CreateAccount 新增设备账号。
func (r *IotDeviceRepo) CreateAccount(ctx context.Context, m *model.IotDeviceAccount) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// UpdateAccount 保存设备账号。
func (r *IotDeviceRepo) UpdateAccount(ctx context.Context, m *model.IotDeviceAccount) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// DeleteAccount 软删除设备账号。
func (r *IotDeviceRepo) DeleteAccount(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.IotDeviceAccount{}, id).Error
}

// ── 设备事件 ──────────────────────────────────────────────────

// CreateEvent 追加一条设备接入事件。
func (r *IotDeviceRepo) CreateEvent(ctx context.Context, m *model.IotDeviceEvent) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}
