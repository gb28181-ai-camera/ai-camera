package device

import (
	"context"
	"errors"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 资源管理 - 语音配置
// ════════════════════════════════════════════════════════════════

// VoiceConfigService 语音配置业务逻辑。
type VoiceConfigService struct {
	repo *repository.VoiceConfigRepo
}

// NewVoiceConfigService 构造语音配置服务。
func NewVoiceConfigService() *VoiceConfigService {
	return &VoiceConfigService{repo: repository.NewVoiceConfigRepo()}
}

// Page 语音配置分页列表。
func (s *VoiceConfigService) Page(ctx context.Context, req *request.PageVoiceConfigReq, ds *utils.DataScope) (*response.PageVoiceConfigResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.VoiceConfig{}
	}
	return &response.PageVoiceConfigResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// Create 新增语音配置。
func (s *VoiceConfigService) Create(ctx context.Context, req *request.CreateVoiceConfigReq) (*model.VoiceConfig, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("语音名称不能为空")
	}

	m := &model.VoiceConfig{
		Name:       name,
		VoiceType:  strings.TrimSpace(req.VoiceType),
		RiskTypeID: req.RiskTypeID,
		RegionID:   req.RegionID,
		DeviceID:   req.DeviceID,
		AudioURL:   req.AudioURL,
		AudioName:  req.AudioName,
		Remark:     req.Remark,
	}
	m.Active = boolOr(req.Active, true)

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 更新语音配置。文本非空才覆盖，指针非 nil 才覆盖。
func (s *VoiceConfigService) Update(ctx context.Context, id int64, req *request.UpdateVoiceConfigReq) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrVoiceConfigNotFound
		}
		return err
	}

	if v := strings.TrimSpace(req.Name); v != "" {
		m.Name = v
	}
	if v := strings.TrimSpace(req.VoiceType); v != "" {
		m.VoiceType = v
	}
	if req.RiskTypeID != nil {
		m.RiskTypeID = nullableID(*req.RiskTypeID)
	}
	if req.RegionID != nil {
		m.RegionID = nullableID(*req.RegionID)
	}
	if req.DeviceID != nil {
		m.DeviceID = nullableID(*req.DeviceID)
	}
	if v := strings.TrimSpace(req.AudioURL); v != "" {
		m.AudioURL = v
	}
	if v := strings.TrimSpace(req.AudioName); v != "" {
		m.AudioName = v
	}
	if v := strings.TrimSpace(req.Remark); v != "" {
		m.Remark = v
	}
	if req.Active != nil {
		m.Active = req.Active
	}

	return s.repo.Update(ctx, m)
}

// Delete 删除语音配置（软删除）。
func (s *VoiceConfigService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrVoiceConfigNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}
