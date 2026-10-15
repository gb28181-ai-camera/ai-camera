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
// 国标视频数据访问（四）：级联平台 / 语音 / JT1078 车载终端
//
// 说明：ac_gb_platform_group、ac_gb_platform_region 为纯关联表（无 dept_id），
// 随主表 ac_gb_platform 过滤，不单独施加 DataScope（设计文档 §10.1）。
// ════════════════════════════════════════════════════════════════

// ── 上级平台 ──────────────────────────────────────────────────

// GbPlatformRepo 上级平台数据访问。表 ac_gb_platform（含 dept_id）。
type GbPlatformRepo struct{}

// NewGbPlatformRepo 构造上级平台仓储。
func NewGbPlatformRepo() *GbPlatformRepo { return &GbPlatformRepo{} }

// Page 分页查询上级平台。
func (r *GbPlatformRepo) Page(ctx context.Context, req *request.PageGbPlatformReq, ds *utils.DataScope) ([]model.GbPlatform, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatform{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR server_gb_id ILIKE ? OR server_ip ILIKE ?)", like, like, like)
		}
		if s := strings.TrimSpace(req.Status); s != "" {
			q = q.Where("status = ?", s)
		}
		return ds.ApplyToQuery(q, "ac_gb_platform.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbPlatform
	err := build().Order("id DESC").Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// ListAll 查询全部启用平台（目录推送选择器用）。
func (r *GbPlatformRepo) ListAll(ctx context.Context, ds *utils.DataScope) ([]model.GbPlatform, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatform{}).
		Where("(active IS NULL OR active = true)")
	q = ds.ApplyToQuery(q, "ac_gb_platform.dept_id")

	var items []model.GbPlatform
	if err := q.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询平台。
func (r *GbPlatformRepo) GetByID(ctx context.Context, id int64) (*model.GbPlatform, error) {
	var m model.GbPlatform
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByServerGBID 按上级平台国标编码查询（级联入向校验用，对齐 WVP 的入向白名单）。
func (r *GbPlatformRepo) GetByServerGBID(ctx context.Context, serverGBID string) (*model.GbPlatform, error) {
	var m model.GbPlatform
	if err := bootstrap.DB().WithContext(ctx).
		Where("server_gb_id = ?", serverGBID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ExistsByServerGBID 判断上级平台国标编码是否已占用（excludeID>0 排除自身）。
func (r *GbPlatformRepo) ExistsByServerGBID(ctx context.Context, serverGBID string, excludeID int64) (bool, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatform{}).
		Where("server_gb_id = ?", serverGBID)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// Create 新增上级平台。
func (r *GbPlatformRepo) Create(ctx context.Context, m *model.GbPlatform) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存上级平台。
func (r *GbPlatformRepo) Update(ctx context.Context, m *model.GbPlatform) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新（在线状态 / 注册续期）。
func (r *GbPlatformRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
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
		Model(&model.GbPlatform{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除上级平台。
func (r *GbPlatformRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbPlatform{}, id).Error
}

// ── 级联通道 ──────────────────────────────────────────────────

// GbPlatformChannelRepo 级联通道数据访问。表 ac_gb_platform_channel（含 dept_id）。
type GbPlatformChannelRepo struct{}

// NewGbPlatformChannelRepo 构造级联通道仓储。
func NewGbPlatformChannelRepo() *GbPlatformChannelRepo { return &GbPlatformChannelRepo{} }

// Page 分页查询级联通道。Preload 关联平台/本地通道。
func (r *GbPlatformChannelRepo) Page(ctx context.Context, req *request.PageGbPlatformChannelReq, ds *utils.DataScope) ([]model.GbPlatformChannel, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatformChannel{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(custom_device_id ILIKE ? OR custom_name ILIKE ?)", like, like)
		}
		if req.PlatformID != nil {
			q = q.Where("platform_id = ?", *req.PlatformID)
		}
		if req.ChannelID != nil {
			q = q.Where("channel_id = ?", *req.ChannelID)
		}
		return ds.ApplyToQuery(q, "ac_gb_platform_channel.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbPlatformChannel
	err := build().
		Preload("Platform").
		Preload("Channel").
		Order("id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListByPlatformID 查询某上级平台下的全部级联通道（目录推送用）。
func (r *GbPlatformChannelRepo) ListByPlatformID(ctx context.Context, platformID int64) ([]model.GbPlatformChannel, error) {
	var items []model.GbPlatformChannel
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatformChannel{}).
		Where("platform_id = ?", platformID).
		Order("id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询级联通道。
func (r *GbPlatformChannelRepo) GetByID(ctx context.Context, id int64) (*model.GbPlatformChannel, error) {
	var m model.GbPlatformChannel
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增级联通道。
func (r *GbPlatformChannelRepo) Create(ctx context.Context, m *model.GbPlatformChannel) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存级联通道。
func (r *GbPlatformChannelRepo) Update(ctx context.Context, m *model.GbPlatformChannel) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除级联通道。
func (r *GbPlatformChannelRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbPlatformChannel{}, id).Error
}

// DeleteByPlatformID 软删除某上级平台下的全部级联通道（平台删除时级联）。
func (r *GbPlatformChannelRepo) DeleteByPlatformID(ctx context.Context, platformID int64) error {
	return bootstrap.DB().WithContext(ctx).
		Where("platform_id = ?", platformID).
		Delete(&model.GbPlatformChannel{}).Error
}

// CountByChannel 统计某本地通道被多少个上级平台共享（删除通道前引用校验）。
func (r *GbPlatformChannelRepo) CountByChannel(ctx context.Context, channelID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatformChannel{}).
		Where("channel_id = ?", channelID).Count(&n).Error
	return n, err
}

// ── 级联平台 ↔ 分组 / 区域（纯关联表）─────────────────────────

// GbPlatformGroupRepo 级联平台↔业务分组映射数据访问。表 ac_gb_platform_group。
// 纯关联表：无 dept_id，随主表 ac_gb_platform 过滤，不单独施加 DataScope。
type GbPlatformGroupRepo struct{}

// NewGbPlatformGroupRepo 构造级联分组映射仓储。
func NewGbPlatformGroupRepo() *GbPlatformGroupRepo { return &GbPlatformGroupRepo{} }

// ListGroupIDs 查询某平台可见的业务分组 ID 列表。
func (r *GbPlatformGroupRepo) ListGroupIDs(ctx context.Context, platformID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatformGroup{}).
		Where("platform_id = ?", platformID).
		Pluck("group_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// ReplaceForPlatform 以「先删后插」方式整体替换某平台的可见分组集合。
func (r *GbPlatformGroupRepo) ReplaceForPlatform(ctx context.Context, platformID int64, groupIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("platform_id = ?", platformID).Delete(&model.GbPlatformGroup{}).Error; err != nil {
			return err
		}
		if len(groupIDs) == 0 {
			return nil
		}
		rows := make([]model.GbPlatformGroup, 0, len(groupIDs))
		for _, gid := range groupIDs {
			rows = append(rows, model.GbPlatformGroup{PlatformID: platformID, GroupID: gid})
		}
		return tx.Create(&rows).Error
	})
}

// DeleteByPlatformID 软删除某平台的全部分组映射。
func (r *GbPlatformGroupRepo) DeleteByPlatformID(ctx context.Context, platformID int64) error {
	return bootstrap.DB().WithContext(ctx).
		Where("platform_id = ?", platformID).
		Delete(&model.GbPlatformGroup{}).Error
}

// GbPlatformRegionRepo 级联平台↔区域映射数据访问。表 ac_gb_platform_region。
// 纯关联表：无 dept_id，随主表 ac_gb_platform 过滤，不单独施加 DataScope。
type GbPlatformRegionRepo struct{}

// NewGbPlatformRegionRepo 构造级联区域映射仓储。
func NewGbPlatformRegionRepo() *GbPlatformRegionRepo { return &GbPlatformRegionRepo{} }

// ListRegionIDs 查询某平台可见的区域 ID 列表。
func (r *GbPlatformRegionRepo) ListRegionIDs(ctx context.Context, platformID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatformRegion{}).
		Where("platform_id = ?", platformID).
		Pluck("region_id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// ReplaceForPlatform 以「先删后插」方式整体替换某平台的可见区域集合。
func (r *GbPlatformRegionRepo) ReplaceForPlatform(ctx context.Context, platformID int64, regionIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("platform_id = ?", platformID).Delete(&model.GbPlatformRegion{}).Error; err != nil {
			return err
		}
		if len(regionIDs) == 0 {
			return nil
		}
		rows := make([]model.GbPlatformRegion, 0, len(regionIDs))
		for _, rid := range regionIDs {
			rows = append(rows, model.GbPlatformRegion{PlatformID: platformID, RegionID: rid})
		}
		return tx.Create(&rows).Error
	})
}

// DeleteByPlatformID 软删除某平台的全部区域映射。
func (r *GbPlatformRegionRepo) DeleteByPlatformID(ctx context.Context, platformID int64) error {
	return bootstrap.DB().WithContext(ctx).
		Where("platform_id = ?", platformID).
		Delete(&model.GbPlatformRegion{}).Error
}

// ── 语音会话 ──────────────────────────────────────────────────

// VoiceSessionRepo 语音会话数据访问。表 ac_voice_session（含 dept_id）。
type VoiceSessionRepo struct{}

// NewVoiceSessionRepo 构造语音会话仓储。
func NewVoiceSessionRepo() *VoiceSessionRepo { return &VoiceSessionRepo{} }

// Page 分页查询语音会话。Preload 关联设备/通道。
func (r *VoiceSessionRepo) Page(ctx context.Context, req *request.PageVoiceSessionReq, ds *utils.DataScope) ([]model.VoiceSession, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceSession{})
		if s := strings.TrimSpace(req.Type); s != "" {
			q = q.Where("type = ?", s)
		}
		if s := strings.TrimSpace(req.Status); s != "" {
			q = q.Where("status = ?", s)
		}
		if req.DeviceID != nil {
			q = q.Where("device_id = ?", *req.DeviceID)
		}
		if req.ChannelID != nil {
			q = q.Where("channel_id = ?", *req.ChannelID)
		}
		return ds.ApplyToQuery(q, "ac_voice_session.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.VoiceSession
	err := build().
		Preload("Device").
		Preload("Channel").
		Order("start_dt DESC, id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetBySessionID 按会话标识查询（会话停止时定位）。
func (r *VoiceSessionRepo) GetBySessionID(ctx context.Context, sessionID string) (*model.VoiceSession, error) {
	var m model.VoiceSession
	if err := bootstrap.DB().WithContext(ctx).
		Where("session_id = ?", sessionID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// ListActiveByChannel 查询某通道进行中的会话（同通道并发控制）。
func (r *VoiceSessionRepo) ListActiveByChannel(ctx context.Context, channelID int64) ([]model.VoiceSession, error) {
	var items []model.VoiceSession
	err := bootstrap.DB().WithContext(ctx).Model(&model.VoiceSession{}).
		Where("channel_id = ? AND status = ?", channelID, "active").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Create 新增会话。
func (r *VoiceSessionRepo) Create(ctx context.Context, m *model.VoiceSession) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存会话。
func (r *VoiceSessionRepo) Update(ctx context.Context, m *model.VoiceSession) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新（结束会话：status / end_dt / end_reason）。
func (r *VoiceSessionRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
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
		Model(&model.VoiceSession{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除会话。
func (r *VoiceSessionRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.VoiceSession{}, id).Error
}

// ── 语音 provider 字典 ────────────────────────────────────────

// VoiceProviderRepo 语音 provider 字典数据访问。表 ac_voice_provider（含 dept_id）。
type VoiceProviderRepo struct{}

// NewVoiceProviderRepo 构造语音 provider 字典仓储。
func NewVoiceProviderRepo() *VoiceProviderRepo { return &VoiceProviderRepo{} }

// Page 分页查询 provider 字典。
func (r *VoiceProviderRepo) Page(ctx context.Context, req *request.PageVoiceProviderReq, ds *utils.DataScope) ([]model.VoiceProvider, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceProvider{})
		if s := strings.TrimSpace(req.Category); s != "" {
			q = q.Where("category = ?", s)
		}
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR code ILIKE ?)", like, like)
		}
		return ds.ApplyToQuery(q, "ac_voice_provider.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.VoiceProvider
	err := build().Order("category ASC, sequence ASC, id ASC").
		Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// ListByCategory 查询某类别的全部启用 provider（驱动前端动态表单）。
func (r *VoiceProviderRepo) ListByCategory(ctx context.Context, category string) ([]model.VoiceProvider, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.VoiceProvider{}).
		Where("(active IS NULL OR active = true)")
	if s := strings.TrimSpace(category); s != "" {
		q = q.Where("category = ?", s)
	}
	var items []model.VoiceProvider
	if err := q.Order("sequence ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询 provider。
func (r *VoiceProviderRepo) GetByID(ctx context.Context, id int64) (*model.VoiceProvider, error) {
	var m model.VoiceProvider
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByCategoryCode 按 category+code 查询（唯一键，provider 是否可用校验）。
func (r *VoiceProviderRepo) GetByCategoryCode(ctx context.Context, category, code string) (*model.VoiceProvider, error) {
	var m model.VoiceProvider
	if err := bootstrap.DB().WithContext(ctx).
		Where("category = ? AND code = ?", category, code).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增 provider。
func (r *VoiceProviderRepo) Create(ctx context.Context, m *model.VoiceProvider) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存 provider。
func (r *VoiceProviderRepo) Update(ctx context.Context, m *model.VoiceProvider) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除 provider。
func (r *VoiceProviderRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.VoiceProvider{}, id).Error
}

// ── JT/T 1078 车载终端（三期）─────────────────────────────────

// JtTerminalRepo JT1078 终端数据访问。表 ac_jt_terminal（含 dept_id）。
type JtTerminalRepo struct{}

// NewJtTerminalRepo 构造 JT1078 终端仓储。
func NewJtTerminalRepo() *JtTerminalRepo { return &JtTerminalRepo{} }

// Page 分页查询 JT1078 终端。
func (r *JtTerminalRepo) Page(ctx context.Context, req *request.PageJtTerminalReq, ds *utils.DataScope) ([]model.JtTerminal, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.JtTerminal{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(phone_number ILIKE ? OR terminal_id ILIKE ? OR plate_no ILIKE ?)", like, like, like)
		}
		if s := strings.TrimSpace(req.Status); s != "" {
			q = q.Where("status = ?", s)
		}
		if s := strings.TrimSpace(req.PlateNo); s != "" {
			q = q.Where("plate_no ILIKE ?", "%"+s+"%")
		}
		if s := strings.TrimSpace(req.TerminalID); s != "" {
			q = q.Where("terminal_id = ?", s)
		}
		return ds.ApplyToQuery(q, "ac_jt_terminal.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.JtTerminal
	err := build().Order("id DESC").Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// GetByID 按主键查询终端。
func (r *JtTerminalRepo) GetByID(ctx context.Context, id int64) (*model.JtTerminal, error) {
	var m model.JtTerminal
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByPhone 按终端手机号查询（JT1078 入向校验，唯一键）。
func (r *JtTerminalRepo) GetByPhone(ctx context.Context, phone string) (*model.JtTerminal, error) {
	var m model.JtTerminal
	if err := bootstrap.DB().WithContext(ctx).
		Where("phone_number = ?", phone).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增终端。
func (r *JtTerminalRepo) Create(ctx context.Context, m *model.JtTerminal) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存终端。
func (r *JtTerminalRepo) Update(ctx context.Context, m *model.JtTerminal) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新（在线状态 / 最近位置）。
func (r *JtTerminalRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
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
		Model(&model.JtTerminal{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除终端。
func (r *JtTerminalRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.JtTerminal{}, id).Error
}

// JtChannelRepo JT1078 通道数据访问。表 ac_jt_channel（含 dept_id）。
type JtChannelRepo struct{}

// NewJtChannelRepo 构造 JT1078 通道仓储。
func NewJtChannelRepo() *JtChannelRepo { return &JtChannelRepo{} }

// Page 分页查询 JT1078 通道。
func (r *JtChannelRepo) Page(ctx context.Context, req *request.PageJtChannelReq, ds *utils.DataScope) ([]model.JtChannel, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.JtChannel{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			q = q.Where("name ILIKE ?", "%"+kw+"%")
		}
		if req.TerminalDBID != nil {
			q = q.Where("terminal_db_id = ?", *req.TerminalDBID)
		}
		if s := strings.TrimSpace(req.Status); s != "" {
			q = q.Where("status = ?", s)
		}
		return ds.ApplyToQuery(q, "ac_jt_channel.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.JtChannel
	err := build().
		Preload("Terminal").
		Order("terminal_db_id ASC, channel_id ASC").
		Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ListByTerminal 查询某终端的全部通道（终端目录同步用）。
func (r *JtChannelRepo) ListByTerminal(ctx context.Context, terminalDBID int64) ([]model.JtChannel, error) {
	var items []model.JtChannel
	err := bootstrap.DB().WithContext(ctx).Model(&model.JtChannel{}).
		Where("terminal_db_id = ?", terminalDBID).
		Order("channel_id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Create 新增通道。
func (r *JtChannelRepo) Create(ctx context.Context, m *model.JtChannel) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存通道。
func (r *JtChannelRepo) Update(ctx context.Context, m *model.JtChannel) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// ReplaceForTerminal 以「先删后插」方式整体替换某终端的通道集合（目录同步用）。
func (r *JtChannelRepo) ReplaceForTerminal(ctx context.Context, terminalDBID int64, items []model.JtChannel) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("terminal_db_id = ?", terminalDBID).Delete(&model.JtChannel{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

// Delete 软删除通道。
func (r *JtChannelRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.JtChannel{}, id).Error
}
