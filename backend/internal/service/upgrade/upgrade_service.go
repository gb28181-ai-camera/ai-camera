package upgrade

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// 升级方式常量（UpgradeDevicesReq.UpgradeType）。
const (
	UpgradeTypeNow  = "now"  // 现在升级：立即写入升级记录并更新设备版本
	UpgradeTypePlan = "plan" // 定时升级：写入待升级计划
)

// UpgradeService 升级管理业务逻辑：固件类型 / 固件 / AI模型 / 升级下发 / 升级计划 / 升级记录。
type UpgradeService struct {
	repo *repository.FirmwareRepo
}

// NewUpgradeService 构造升级管理服务。
func NewUpgradeService() *UpgradeService {
	return &UpgradeService{repo: repository.NewFirmwareRepo()}
}

// ────────────────────────────────────────────────────────────────
// 固件类型
// ────────────────────────────────────────────────────────────────

// PageFirmwareTypes 固件类型分页查询。
func (s *UpgradeService) PageFirmwareTypes(ctx context.Context, req *request.PageFirmwareTypeReq, ds *utils.DataScope) (*response.PageFirmwareTypeResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageFirmwareTypes(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageFirmwareTypeResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// FirmwareTypeOptions 固件类型下拉选项。
func (s *UpgradeService) FirmwareTypeOptions(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	items, err := s.repo.FirmwareTypeOptions(ctx, ds)
	if err != nil {
		return nil, err
	}
	opts := make([]response.OptionItem, 0, len(items))
	for _, it := range items {
		opts = append(opts, response.OptionItem{ID: it.ID, Name: it.Name, Code: it.Code})
	}
	return opts, nil
}

// CreateFirmwareType 新增固件类型。
func (s *UpgradeService) CreateFirmwareType(ctx context.Context, req *request.CreateFirmwareTypeReq) (*model.FirmwareType, error) {
	m := &model.FirmwareType{
		Active:      req.Active,
		Sequence:    req.Sequence,
		Name:        strings.TrimSpace(req.Name),
		Code:        strings.TrimSpace(req.Code),
		Description: req.Description,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	if m.Sequence <= 0 {
		m.Sequence = 10
	}
	if err := s.repo.CreateFirmwareType(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateFirmwareType 更新固件类型。
func (s *UpgradeService) UpdateFirmwareType(ctx context.Context, id int64, req *request.UpdateFirmwareTypeReq) error {
	m := &model.FirmwareType{
		BaseModel:   model.BaseModel{ID: id},
		Active:      req.Active,
		Sequence:    req.Sequence,
		Name:        strings.TrimSpace(req.Name),
		Code:        strings.TrimSpace(req.Code),
		Description: req.Description,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	if m.Sequence <= 0 {
		m.Sequence = 10
	}
	return s.repo.UpdateFirmwareType(ctx, m)
}

// DeleteFirmwareType 删除固件类型（软删除）。
func (s *UpgradeService) DeleteFirmwareType(ctx context.Context, id int64) error {
	return s.repo.DeleteFirmwareType(ctx, id)
}

// ────────────────────────────────────────────────────────────────
// 固件
// ────────────────────────────────────────────────────────────────

// PageFirmwares 固件分页查询。
func (s *UpgradeService) PageFirmwares(ctx context.Context, req *request.PageFirmwareReq, ds *utils.DataScope) (*response.PageFirmwareResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageFirmwares(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageFirmwareResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// FirmwareOptions 固件下拉选项（Code 为版本号，供前端展示「名称 版本」）。
func (s *UpgradeService) FirmwareOptions(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	items, err := s.repo.FirmwareOptions(ctx, ds)
	if err != nil {
		return nil, err
	}
	opts := make([]response.OptionItem, 0, len(items))
	for _, it := range items {
		opts = append(opts, response.OptionItem{ID: it.ID, Name: it.Name, Code: it.Version})
	}
	return opts, nil
}

// GetFirmware 固件详情。
// 查不到时原样返回 gorm.ErrRecordNotFound，由 handler 统一转为 404「固件不存在」。
func (s *UpgradeService) GetFirmware(ctx context.Context, id int64) (*model.Firmware, error) {
	return s.repo.GetFirmware(ctx, id)
}

// CreateFirmware 新增固件。
func (s *UpgradeService) CreateFirmware(ctx context.Context, req *request.CreateFirmwareReq) (*model.Firmware, error) {
	releaseDT, err := parseOptionalDT(req.ReleaseDT)
	if err != nil {
		return nil, err
	}
	m := &model.Firmware{
		Active:         req.Active,
		Name:           strings.TrimSpace(req.Name),
		Version:        strings.TrimSpace(req.Version),
		FirmwareTypeID: req.FirmwareTypeID,
		FileURL:        req.FileURL,
		FileSize:       req.FileSize,
		Md5:            req.Md5,
		ReleaseDT:      releaseDT,
		Description:    req.Description,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	if err := s.repo.CreateFirmware(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateFirmware 更新固件。
func (s *UpgradeService) UpdateFirmware(ctx context.Context, id int64, req *request.UpdateFirmwareReq) error {
	releaseDT, err := parseOptionalDT(req.ReleaseDT)
	if err != nil {
		return err
	}
	m := &model.Firmware{
		BaseModel:      model.BaseModel{ID: id},
		Active:         req.Active,
		Name:           strings.TrimSpace(req.Name),
		Version:        strings.TrimSpace(req.Version),
		FirmwareTypeID: req.FirmwareTypeID,
		FileURL:        req.FileURL,
		FileSize:       req.FileSize,
		Md5:            req.Md5,
		ReleaseDT:      releaseDT,
		Description:    req.Description,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	return s.repo.UpdateFirmware(ctx, m)
}

// DeleteFirmware 删除固件（软删除）。
func (s *UpgradeService) DeleteFirmware(ctx context.Context, id int64) error {
	return s.repo.DeleteFirmware(ctx, id)
}

// ────────────────────────────────────────────────────────────────
// AI 模型
// ────────────────────────────────────────────────────────────────

// PageModels AI模型分页查询。
func (s *UpgradeService) PageModels(ctx context.Context, req *request.PageAIModelReq, ds *utils.DataScope) (*response.PageAIModelResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageModels(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageAIModelResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// ModelOptions AI模型下拉选项（Code 为版本号）。
func (s *UpgradeService) ModelOptions(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	items, err := s.repo.ModelOptions(ctx, ds)
	if err != nil {
		return nil, err
	}
	opts := make([]response.OptionItem, 0, len(items))
	for _, it := range items {
		opts = append(opts, response.OptionItem{ID: it.ID, Name: it.Name, Code: it.Version})
	}
	return opts, nil
}

// GetModel AI模型详情。
// 查不到时原样返回 gorm.ErrRecordNotFound，由 handler 统一转为 404「模型不存在」。
func (s *UpgradeService) GetModel(ctx context.Context, id int64) (*model.AIModel, error) {
	return s.repo.GetModel(ctx, id)
}

// CreateModel 新增AI模型。模型必须关联已存在的适用固件。
func (s *UpgradeService) CreateModel(ctx context.Context, req *request.CreateAIModelReq) (*model.AIModel, error) {
	if err := s.checkFirmwareExists(ctx, req.FirmwareID); err != nil {
		return nil, err
	}
	releaseDT, err := parseOptionalDT(req.ReleaseDT)
	if err != nil {
		return nil, err
	}
	m := &model.AIModel{
		Active:      req.Active,
		Name:        strings.TrimSpace(req.Name),
		Version:     strings.TrimSpace(req.Version),
		FirmwareID:  req.FirmwareID,
		FileURL:     req.FileURL,
		FileSize:    req.FileSize,
		Md5:         req.Md5,
		ReleaseDT:   releaseDT,
		Description: req.Description,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	if err := s.repo.CreateModel(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateModel 更新AI模型。模型必须关联已存在的适用固件。
func (s *UpgradeService) UpdateModel(ctx context.Context, id int64, req *request.UpdateAIModelReq) error {
	if err := s.checkFirmwareExists(ctx, req.FirmwareID); err != nil {
		return err
	}
	releaseDT, err := parseOptionalDT(req.ReleaseDT)
	if err != nil {
		return err
	}
	m := &model.AIModel{
		BaseModel:   model.BaseModel{ID: id},
		Active:      req.Active,
		Name:        strings.TrimSpace(req.Name),
		Version:     strings.TrimSpace(req.Version),
		FirmwareID:  req.FirmwareID,
		FileURL:     req.FileURL,
		FileSize:    req.FileSize,
		Md5:         req.Md5,
		ReleaseDT:   releaseDT,
		Description: req.Description,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	return s.repo.UpdateModel(ctx, m)
}

// DeleteModel 删除AI模型（软删除）。
func (s *UpgradeService) DeleteModel(ctx context.Context, id int64) error {
	return s.repo.DeleteModel(ctx, id)
}

// ────────────────────────────────────────────────────────────────
// 升级下发
// ────────────────────────────────────────────────────────────────

// UpgradeDevices 设备升级下发。
//
//	upgrade_type=now  — 立即为每台设备写入升级记录并更新设备当前固件/模型版本；
//	upgrade_type=plan — 校验 plan_dt 晚于当前时间后，为每台设备写入待升级计划。
func (s *UpgradeService) UpgradeDevices(ctx context.Context, req *request.UpgradeDevicesReq, operator string) (*response.UpgradeDevicesResp, error) {
	if len(req.DeviceIDs) == 0 {
		return nil, errors.New("请选择要升级的设备")
	}

	target := &repository.UpgradeTarget{}
	if req.FirmwareID != nil && *req.FirmwareID > 0 {
		fw, err := s.repo.GetFirmware(ctx, *req.FirmwareID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("固件不存在")
			}
			return nil, err
		}
		fwID := fw.ID
		target.FirmwareID = &fwID
		target.FirmwareName = fw.Name
		target.FirmwareVer = fw.Version
	}
	if req.AIModelID != nil && *req.AIModelID > 0 {
		md, err := s.repo.GetModel(ctx, *req.AIModelID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("AI模型不存在")
			}
			return nil, err
		}
		mdID := md.ID
		target.AIModelID = &mdID
		target.AIModelName = md.Name
		target.AIModelVer = md.Version
	}
	if target.FirmwareID == nil && target.AIModelID == nil {
		return nil, errors.New("请选择升级固件或AI模型")
	}

	switch req.UpgradeType {
	case UpgradeTypeNow:
		count, err := s.repo.UpgradeNow(ctx, req.DeviceIDs, target, operator)
		if err != nil {
			return nil, err
		}
		return &response.UpgradeDevicesResp{NowCount: count}, nil

	case UpgradeTypePlan:
		planDT, err := parseOptionalDT(req.PlanDT)
		if err != nil {
			return nil, err
		}
		if planDT == nil {
			return nil, errors.New("请选择计划升级时间")
		}
		if !planDT.After(time.Now()) {
			return nil, errors.New("计划升级时间必须晚于当前时间")
		}
		count, err := s.repo.CreatePlans(ctx, req.DeviceIDs, target, *planDT)
		if err != nil {
			return nil, err
		}
		return &response.UpgradeDevicesResp{PlanCount: count}, nil
	}

	return nil, errors.New("升级方式不正确")
}

// ────────────────────────────────────────────────────────────────
// 升级计划 / 升级记录
// ────────────────────────────────────────────────────────────────

// PagePlans 升级计划分页查询。
func (s *UpgradeService) PagePlans(ctx context.Context, req *request.PageUpgradePlanReq, ds *utils.DataScope) (*response.PageUpgradePlanResp, error) {
	req.Normalize()
	items, total, err := s.repo.PagePlans(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageUpgradePlanResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// CancelPlan 取消升级计划（仅 pending 可取消）。
func (s *UpgradeService) CancelPlan(ctx context.Context, id int64) error {
	return s.repo.CancelPlan(ctx, id)
}

// BatchCancelPlans 批量取消升级计划（仅 pending 受影响）。
func (s *UpgradeService) BatchCancelPlans(ctx context.Context, ids []int64) error {
	_, err := s.repo.BatchCancelPlans(ctx, ids)
	return err
}

// PageRecords 升级记录分页查询。
func (s *UpgradeService) PageRecords(ctx context.Context, req *request.PageUpgradeRecordReq, ds *utils.DataScope) (*response.PageUpgradeRecordResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageRecords(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageUpgradeRecordResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// ────────────────────────────────────────────────────────────────
// 内部工具
// ────────────────────────────────────────────────────────────────

// checkFirmwareExists 校验模型绑定的固件存在。
func (s *UpgradeService) checkFirmwareExists(ctx context.Context, firmwareID *int64) error {
	if firmwareID == nil || *firmwareID <= 0 {
		return errors.New("请选择适用固件")
	}
	if _, err := s.repo.GetFirmware(ctx, *firmwareID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("适用固件不存在")
		}
		return err
	}
	return nil
}

// parseOptionalDT 解析可空时间入参，支持 YYYY-MM-DD / YYYY-MM-DD HH:mm / YYYY-MM-DD HH:mm:ss / RFC3339。
func parseOptionalDT(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, errors.New("时间格式不正确")
}

// boolPtr 返回布尔值的指针，用于 *bool 字段显式赋值。
func boolPtr(b bool) *bool { return &b }
