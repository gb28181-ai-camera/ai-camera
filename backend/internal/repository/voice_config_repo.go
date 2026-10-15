package repository

import (
	"context"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 语音配置数据访问
// ════════════════════════════════════════════════════════════════

// VoiceConfigRepo 语音配置数据访问层。
type VoiceConfigRepo struct{}

// NewVoiceConfigRepo 构造语音配置仓储。
func NewVoiceConfigRepo() *VoiceConfigRepo { return &VoiceConfigRepo{} }

// voiceConfigQuery 构造带表别名 v 的语音配置查询基座。
func voiceConfigQuery() *gorm.DB {
	return bootstrap.DB().Model(&model.VoiceConfig{}).Table("ac_voice_config AS v")
}

// Page 分页查询语音配置。
//
// Preload 区域/设备/风险类型，使列表直接展示关联名称而非 ID。
func (r *VoiceConfigRepo) Page(ctx context.Context, req *request.PageVoiceConfigReq, ds *utils.DataScope) ([]model.VoiceConfig, int64, error) {
	q := voiceConfigQuery().WithContext(ctx)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("(v.name ILIKE ? OR v.voice_type ILIKE ? OR v.audio_name ILIKE ?)", like, like, like)
	}
	if s := strings.TrimSpace(req.VoiceType); s != "" {
		q = q.Where("v.voice_type = ?", s)
	}
	if req.RegionID != nil {
		q = q.Where("v.region_id = ?", *req.RegionID)
	}
	if req.DeviceID != nil {
		q = q.Where("v.device_id = ?", *req.DeviceID)
	}
	q = ds.ApplyToQuery(q, "v.dept_id")

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.VoiceConfig
	err := q.Session(&gorm.Session{}).
		Preload("Region").
		Preload("Device").
		Preload("RiskType").
		Order("v.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID 按主键查询语音配置。
func (r *VoiceConfigRepo) GetByID(ctx context.Context, id int64) (*model.VoiceConfig, error) {
	var m model.VoiceConfig
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增语音配置。
func (r *VoiceConfigRepo) Create(ctx context.Context, m *model.VoiceConfig) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存语音配置。
func (r *VoiceConfigRepo) Update(ctx context.Context, m *model.VoiceConfig) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除语音配置。
func (r *VoiceConfigRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.VoiceConfig{}, id).Error
}
