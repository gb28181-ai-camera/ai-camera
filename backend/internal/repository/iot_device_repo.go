package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	// 按产品绑定的服务组类型过滤：设备 → 产品 → 产品-服务组 → 服务组。
	// 未绑定产品/未绑定服务组的设备不会出现在 WebRTC / AIChat 专用页。
	if t := strings.TrimSpace(req.ServiceGroupType); t != "" {
		q = q.Where(`EXISTS (
			SELECT 1
			FROM ac_iot_product_service_group psg
			JOIN ac_iot_service_group sg ON sg.id = psg.service_group_id
			WHERE psg.product_id = d.product_id
			  AND psg.deleted_at IS NULL
			  AND sg.deleted_at IS NULL
			  AND sg.type = ?
		)`, t)
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

// SetGbDeviceBinding 写入设备的国标编码并置为已激活。
//
// SIP 侧绑定只在这一处落库：国标编码是唯一标识，必须与设备当前归属产品
// 一并确认后再写，避免出现「编码写进去了、产品却不匹配」的半绑定状态。
func (r *IotDeviceRepo) SetGbDeviceBinding(ctx context.Context, id int64, gbDeviceID, registerStatus string) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"gb_device_id":    gbDeviceID,
			"register_status": registerStatus,
		}).Error
}

// SetActivatedAt 记录设备主账号首次激活时间。
func (r *IotDeviceRepo) SetActivatedAt(ctx context.Context, id int64, t time.Time) error {
	return bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", id).
		Update("activated_at", t).Error
}

// UnbindAll 解绑设备：删除全部设备账号并清空激活时间。
//
// 账号使用**硬删除**：软删除的行仍会占用 username / share_token 唯一索引，
// 使解绑后无法重新使用同一登录名或生成同值令牌。解绑本身有
// ac_iot_device_event 留痕，不依赖账号行的软删除状态做审计。
// 只清理「平台用户 ↔ 设备」的绑定关系；product_id / device_secret /
// gb_device_id 属于设备接入配置与国标身份，保持不变。
func (r *IotDeviceRepo) UnbindAll(ctx context.Context, deviceID int64) (int64, error) {
	var removed int64
	err := bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Unscoped().
			Where("device_id = ?", deviceID).
			Delete(&model.IotDeviceAccount{})
		if res.Error != nil {
			return res.Error
		}
		removed = res.RowsAffected
		return tx.Model(&model.Device{}).
			Where("id = ?", deviceID).
			Update("activated_at", nil).Error
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
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

// GetAccountByPlatformUser 查询某平台用户在指定设备上的账号。
//
// 不按角色过滤：App 绑定后同一用户在同一设备上只应有一个账号，
// 但历史上可能存在 owner/shared 并存的数据，取最早一条即可。
func (r *IotDeviceRepo) GetAccountByPlatformUser(ctx context.Context, deviceID, userID int64) (*model.IotDeviceAccount, error) {
	var m model.IotDeviceAccount
	if err := bootstrap.DB().WithContext(ctx).
		Where("device_id = ? AND platform_user_id = ?", deviceID, userID).
		Order("id ASC").
		First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsShareToken 判断分享令牌是否已被占用（生成时需要重试）。
func (r *IotDeviceRepo) ExistsShareToken(ctx context.Context, token string) (bool, error) {
	var cnt int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.IotDeviceAccount{}).
		Where("share_token = ?", token).
		Count(&cnt).Error
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ClaimByShareToken 把未认领的分享账号原子地绑定到平台用户，并清空令牌。
//
// 为什么令牌必须一次性：分享二维码可能被转发/截图传播，若令牌可重复使用，
// 任何拿到图片的人都能永久把设备加入自己的账号。认领成功后立即清空 token，
// 使二维码在第一次扫码后即失效，把「分享」限制为一次可控的授权动作。
//
// 并发安全：事务内 SELECT ... FOR UPDATE 锁住账号行；两个并发认领请求会串行化，
// 后者在锁释放后读到的 platform_user_id 已非 NULL，条件不再成立而返回 NotFound。
func (r *IotDeviceRepo) ClaimByShareToken(ctx context.Context, token string, userID int64) (*model.IotDeviceAccount, error) {
	var claimed *model.IotDeviceAccount
	err := bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var acc model.IotDeviceAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("share_token = ? AND platform_user_id IS NULL", token).
			First(&acc).Error; err != nil {
			return err
		}
		// 设备已被删除时令牌一并失效，避免把分享账号挂到墓碑设备上。
		var dev model.Device
		if err := tx.Where("id = ?", acc.DeviceID).First(&dev).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.IotDeviceAccount{}).
			Where("id = ?", acc.ID).
			Updates(map[string]interface{}{"platform_user_id": userID, "share_token": nil}).Error; err != nil {
			return err
		}
		acc.PlatformUserID = &userID
		acc.ShareToken = nil
		claimed = &acc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

// RevokeUnusedShareTokens 清空设备上所有「尚未被认领」的分享令牌。
//
// 只清空 token、不删除账号：这些占位账号的密码哈希从未对外展示，
// 保留行可以避免与唯一索引/审计记录产生额外耦合；token 为空后二维码立即失效。
func (r *IotDeviceRepo) RevokeUnusedShareTokens(ctx context.Context, deviceID int64) (int64, error) {
	res := bootstrap.DB().WithContext(ctx).
		Model(&model.IotDeviceAccount{}).
		Where("device_id = ? AND platform_user_id IS NULL AND share_token IS NOT NULL", deviceID).
		Update("share_token", nil)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// ListAccountsByPlatformUser 查询平台用户在所有设备上的账号。
//
// App 设备列表需要展示「我在该设备上是什么角色」，用一次查询建 map，
// 避免对每台设备做一次 N+1 查询。
func (r *IotDeviceRepo) ListAccountsByPlatformUser(ctx context.Context, userID int64) ([]model.IotDeviceAccount, error) {
	var items []model.IotDeviceAccount
	if err := bootstrap.DB().WithContext(ctx).
		Where("platform_user_id = ?", userID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListDevicesByPlatformUser 查询平台用户已绑定的设备（未删除）。
//
// 先取 device_id 再查设备：ac_device 带软删除与 Preload("Product")，
// 用子查询可以继续复用 GORM 的软删除语义，也不会因为 JOIN 产生重复行。
func (r *IotDeviceRepo) ListDevicesByPlatformUser(ctx context.Context, userID int64) ([]model.Device, error) {
	var ids []int64
	if err := bootstrap.DB().WithContext(ctx).
		Model(&model.IotDeviceAccount{}).
		Where("platform_user_id = ?", userID).
		Distinct().
		Pluck("device_id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []model.Device{}, nil
	}
	var items []model.Device
	if err := bootstrap.DB().WithContext(ctx).
		Preload("Product").
		Where("id IN ?", ids).
		Order("id DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
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
