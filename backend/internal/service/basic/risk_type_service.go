package basic

import (
	"context"
	"fmt"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// RiskTypeService 风险类型业务逻辑。
type RiskTypeService struct{}

// NewRiskTypeService 构造风险类型服务。
func NewRiskTypeService() *RiskTypeService { return &RiskTypeService{} }

// Page 分页查询风险类型。
func (s *RiskTypeService) Page(ctx context.Context, req *request.PageRiskTypeReq, ds *utils.DataScope) (*response.PageRiskTypeResp, error) {
	req.Normalize()
	list, total, err := repository.NewRiskTypeRepo().Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageRiskTypeResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    list,
	}, nil
}

// Options 风险类型下拉选项。
func (s *RiskTypeService) Options(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	list, err := repository.NewRiskTypeRepo().Options(ctx, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.OptionItem, 0, len(list))
	for i := range list {
		items = append(items, response.OptionItem{ID: list[i].ID, Name: list[i].Name, Code: list[i].Code})
	}
	return items, nil
}

// Get 风险类型详情。
func (s *RiskTypeService) Get(ctx context.Context, id int64) (*model.RiskType, error) {
	return repository.NewRiskTypeRepo().GetByID(ctx, id)
}

// Create 新增风险类型。
func (s *RiskTypeService) Create(ctx context.Context, req *request.CreateRiskTypeReq) (*model.RiskType, error) {
	m := &model.RiskType{
		Name:        req.Name,
		Code:        req.Code,
		Level:       req.Level,
		Sequence:    req.Sequence,
		Active:      req.Active,
		Description: req.Description,
	}
	if m.Sequence == 0 {
		m.Sequence = 10
	}
	if m.Level == "" {
		m.Level = model.LevelMedium
	}
	if m.Active == nil {
		active := true
		m.Active = &active
	}
	if err := repository.NewRiskTypeRepo().Create(ctx, m); err != nil {
		return nil, fmt.Errorf("新增风险类型失败: %w", err)
	}
	return m, nil
}

// Update 修改风险类型。
func (s *RiskTypeService) Update(ctx context.Context, id int64, req *request.UpdateRiskTypeReq) error {
	repo := repository.NewRiskTypeRepo()
	m, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	m.Name = req.Name
	m.Code = req.Code
	if req.Level != "" {
		m.Level = req.Level
	}
	if req.Sequence != 0 {
		m.Sequence = req.Sequence
	}
	if req.Active != nil {
		m.Active = req.Active
	}
	m.Description = req.Description

	if err := repo.Update(ctx, m); err != nil {
		return fmt.Errorf("修改风险类型失败: %w", err)
	}
	return nil
}

// Delete 软删除风险类型。被风险记录或语音配置引用时拒绝删除。
func (s *RiskTypeService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewRiskTypeRepo()
	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}

	if n, err := repo.CountRisks(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return fmt.Errorf("该风险类型下存在 %d 条风险记录，无法删除", n)
	}
	if n, err := repo.CountVoiceConfigs(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return fmt.Errorf("该风险类型已被 %d 条语音配置引用，无法删除", n)
	}
	// 风险通知设置（需求文档 02-基础信息模块 §5.2 列出的第三张引用表）
	if n, err := repo.CountByColumn(ctx, "ac_risk_notify_setting", "risk_type_id", id); err != nil {
		return err
	} else if n > 0 {
		return fmt.Errorf("该风险类型已被 %d 条风险通知设置引用，无法删除", n)
	}

	if err := repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除风险类型失败: %w", err)
	}
	return nil
}
