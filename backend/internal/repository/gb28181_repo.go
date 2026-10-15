package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标视频数据访问（一）：国标设备 / 通道 / 业务分组
//
// 数据权限：凡带 dept_id 的表，列表查询均以 ds.ApplyToQuery 施加行级过滤
// （见设计文档 §10.1）。唯一的真例外是 ac_gb_sip_log（另见 gb28181_alarm_repo.go）。
// ════════════════════════════════════════════════════════════════

// ── 国标设备（白名单查询）──────────────────────────────────────

// GbDeviceRepo 国标设备数据访问。表 ac_device（国标接入相关列）。
//
// 与 DeviceRepo 的分工：DeviceRepo 面向设备管理的通用 CRUD；
// GbDeviceRepo 只关心 SIP 白名单所需的国标接入列，供信令层（cmd/sipsvc）调用。
// 二者操作同一张表，不引入额外的表结构。
type GbDeviceRepo struct{}

// NewGbDeviceRepo 构造国标设备仓储。
func NewGbDeviceRepo() *GbDeviceRepo { return &GbDeviceRepo{} }

// FindByGbDeviceID 按国标 20 位编码查询设备。
//
// ★ 这是需求 2「SIP 设备必须是设备管理里有的设备」的核心查询：
// SIP 信令层收到 REGISTER / MESSAGE 时调用它做白名单校验，
// **查不到即回 403 Forbidden**（未登记设备拒绝接入）。
//
// 只匹配 gb_device_id 精确相等；gorm.ErrRecordNotFound 返回时 caller 应视为「未命中」。
// 注意：gb_device_id 与 device_code 是两个不同的 ID 命名空间，此处绝不可退化用 device_code。
func (r *GbDeviceRepo) FindByGbDeviceID(ctx context.Context, gbDeviceID string) (*model.Device, error) {
	if strings.TrimSpace(gbDeviceID) == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var m model.Device
	err := bootstrap.DB().WithContext(ctx).
		Where("gb_device_id = ?", gbDeviceID).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// FindByGbDeviceIDs 批量按国标编码查询（订阅/级联入向校验用）。
func (r *GbDeviceRepo) FindByGbDeviceIDs(ctx context.Context, gbDeviceIDs []string) ([]model.Device, error) {
	if len(gbDeviceIDs) == 0 {
		return nil, nil
	}
	var items []model.Device
	err := bootstrap.DB().WithContext(ctx).
		Where("gb_device_id IN ?", gbDeviceIDs).
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ExistsByGbDeviceID 判断国标编码是否已被占用（excludeID>0 时排除自身）。
func (r *GbDeviceRepo) ExistsByGbDeviceID(ctx context.Context, gbDeviceID string, excludeID int64) (bool, error) {
	if strings.TrimSpace(gbDeviceID) == "" {
		return false, nil
	}
	q := bootstrap.DB().WithContext(ctx).Model(&model.Device{}).
		Where("gb_device_id = ?", gbDeviceID)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// UpdateRegisterState 更新设备的注册结果（信令层在 REGISTER 处理后调用）。
//
// fields 由调用方给定（如 register_status / register_dt / sip_expires / status /
// local_ip / channel_count），此处只补审计字段，避免 Save 全字段回写误伤。
func (r *GbDeviceRepo) UpdateRegisterState(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// ── 国标通道 ──────────────────────────────────────────────────

// GbChannelRepo 国标通道数据访问。表 ac_gb_channel（含 dept_id）。
type GbChannelRepo struct{}

// NewGbChannelRepo 构造国标通道仓储。
func NewGbChannelRepo() *GbChannelRepo { return &GbChannelRepo{} }

// channelFilters 施加通道列表筛选条件。
func channelFilters(q *gorm.DB, req *request.PageGbChannelReq) *gorm.DB {
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(name ILIKE ? OR gb_channel_id ILIKE ? OR device_code ILIKE ?)", like, like, like)
	}
	if req.DeviceID != nil {
		q = q.Where("device_id = ?", *req.DeviceID)
	}
	if req.RegionID != nil {
		q = q.Where("region_id = ?", *req.RegionID)
	}
	if req.GroupID != nil {
		q = q.Where("group_id = ?", *req.GroupID)
	}
	if s := strings.TrimSpace(req.Status); s != "" {
		q = q.Where("status = ?", s)
	}
	if s := strings.TrimSpace(req.GbDeviceID); s != "" {
		q = q.Where("gb_device_id = ?", s)
	}
	return q
}

// Page 分页查询国标通道。Preload 关联设备/区域/分组/录像计划，列表直接展示名称。
func (r *GbChannelRepo) Page(ctx context.Context, req *request.PageGbChannelReq, ds *utils.DataScope) ([]model.GbChannel, int64, error) {
	build := func() *gorm.DB {
		q := channelFilters(bootstrap.DB().WithContext(ctx).Model(&model.GbChannel{}), req)
		return ds.ApplyToQuery(q, "ac_gb_channel.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbChannel
	err := build().
		Preload("Device").
		Preload("Region").
		Preload("Group").
		Preload("RecordPlan").
		Order("id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Options 通道下拉选项（仅启用状态）。
func (r *GbChannelRepo) Options(ctx context.Context, ds *utils.DataScope) ([]model.GbChannel, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbChannel{}).
		Select("id, name, gb_channel_id, device_id, device_code, status").
		Where("(active IS NULL OR active = true)")
	q = ds.ApplyToQuery(q, "ac_gb_channel.dept_id")

	var items []model.GbChannel
	if err := q.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListByDeviceID 查询某设备下的全部通道（目录同步后展示用）。
func (r *GbChannelRepo) ListByDeviceID(ctx context.Context, deviceID int64) ([]model.GbChannel, error) {
	var items []model.GbChannel
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbChannel{}).
		Where("device_id = ?", deviceID).
		Order("id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询通道详情（含关联对象）。
func (r *GbChannelRepo) GetByID(ctx context.Context, id int64) (*model.GbChannel, error) {
	var m model.GbChannel
	err := bootstrap.DB().WithContext(ctx).
		Preload("Device").
		Preload("Region").
		Preload("Group").
		Preload("RecordPlan").
		First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByGbChannelID 按国标编码查询通道（点播/云台入向校验用）。
func (r *GbChannelRepo) GetByGbChannelID(ctx context.Context, gbChannelID string) (*model.GbChannel, error) {
	var m model.GbChannel
	err := bootstrap.DB().WithContext(ctx).
		Where("gb_channel_id = ?", gbChannelID).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CountByDevice 统计设备下的通道数（目录同步后回写 ac_device.channel_count）。
func (r *GbChannelRepo) CountByDevice(ctx context.Context, deviceID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbChannel{}).
		Where("device_id = ?", deviceID).Count(&n).Error
	return n, err
}

// Create 新增通道。
func (r *GbChannelRepo) Create(ctx context.Context, m *model.GbChannel) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存通道。
func (r *GbChannelRepo) Update(ctx context.Context, m *model.GbChannel) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新通道字段。fields 为空时不执行任何操作。
func (r *GbChannelRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbChannel{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete 软删除通道。
func (r *GbChannelRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbChannel{}, id).Error
}

// DeleteByDeviceID 软删除某设备下的全部通道。
// 设备被拒绝接入时其通道一并失效（见设计文档 §4.1 说明）。
func (r *GbChannelRepo) DeleteByDeviceID(ctx context.Context, deviceID int64) error {
	return bootstrap.DB().WithContext(ctx).
		Where("device_id = ?", deviceID).
		Delete(&model.GbChannel{}).Error
}

// UpsertByGbChannelID 按国标编码 upsert 通道（目录同步写入用）。
//
// 目录同步是「设备上报 → 平台落库」，同一编码可能反复上报；
// 以 gb_channel_id 为冲突键更新，避免重复插入。
func (r *GbChannelRepo) UpsertByGbChannelID(ctx context.Context, m *model.GbChannel) error {
	var existing model.GbChannel
	err := bootstrap.DB().WithContext(ctx).
		Where("gb_channel_id = ?", m.GbChannelID).
		First(&existing).Error
	switch {
	case err == nil:
		m.ID = existing.ID
		m.CreatedBy = existing.CreatedBy
		m.CreatedDT = existing.CreatedDT
		m.BaseModel.DeletedAt = gorm.DeletedAt{}
		return bootstrap.DB().WithContext(ctx).Save(m).Error
	case err == gorm.ErrRecordNotFound:
		return bootstrap.DB().WithContext(ctx).Create(m).Error
	default:
		return err
	}
}

// ── 国标业务分组 ──────────────────────────────────────────────

// GbGroupRepo 国标业务分组数据访问。表 ac_gb_group（含 dept_id，树形）。
type GbGroupRepo struct{}

// NewGbGroupRepo 构造业务分组仓储。
func NewGbGroupRepo() *GbGroupRepo { return &GbGroupRepo{} }

// Page 分页查询业务分组。
func (r *GbGroupRepo) Page(ctx context.Context, req *request.PageGbGroupReq, ds *utils.DataScope) ([]model.GbGroup, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbGroup{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR alias ILIKE ? OR device_id ILIKE ?)", like, like, like)
		}
		if req.ParentID != nil {
			q = q.Where("parent_id = ?", *req.ParentID)
		}
		return ds.ApplyToQuery(q, "ac_gb_group.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbGroup
	err := build().Order("sequence ASC, id ASC").
		Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// ListAll 查询全部启用分组（构建分组树用，不分页）。
func (r *GbGroupRepo) ListAll(ctx context.Context, ds *utils.DataScope) ([]model.GbGroup, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbGroup{}).
		Where("(active IS NULL OR active = true)")
	q = ds.ApplyToQuery(q, "ac_gb_group.dept_id")

	var items []model.GbGroup
	if err := q.Order("sequence ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询分组。
func (r *GbGroupRepo) GetByID(ctx context.Context, id int64) (*model.GbGroup, error) {
	var m model.GbGroup
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增分组。
func (r *GbGroupRepo) Create(ctx context.Context, m *model.GbGroup) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 保存分组（整行更新）。
func (r *GbGroupRepo) Update(ctx context.Context, m *model.GbGroup) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除分组。
func (r *GbGroupRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbGroup{}, id).Error
}

// CountChildren 统计直接子分组数（删除前引用校验）。
func (r *GbGroupRepo) CountChildren(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbGroup{}).
		Where("parent_id = ?", id).Count(&n).Error
	return n, err
}

// CountChannels 统计归属该分组的通道数（删除前引用校验）。
func (r *GbGroupRepo) CountChannels(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbChannel{}).
		Where("group_id = ?", id).Count(&n).Error
	return n, err
}
