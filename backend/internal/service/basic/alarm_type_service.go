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

// AlarmTypeService 告警类型业务逻辑。
type AlarmTypeService struct{}

// NewAlarmTypeService 构造告警类型服务。
func NewAlarmTypeService() *AlarmTypeService { return &AlarmTypeService{} }

// Page 分页查询告警类型。
func (s *AlarmTypeService) Page(ctx context.Context, req *request.PageAlarmTypeReq, ds *utils.DataScope) (*response.PageAlarmTypeResp, error) {
	req.Normalize()
	list, total, err := repository.NewAlarmTypeRepo().Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageAlarmTypeResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    list,
	}, nil
}

// Options 告警类型下拉选项。
func (s *AlarmTypeService) Options(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	list, err := repository.NewAlarmTypeRepo().Options(ctx, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.OptionItem, 0, len(list))
	for i := range list {
		items = append(items, response.OptionItem{ID: list[i].ID, Name: list[i].Name, Code: list[i].Code})
	}
	return items, nil
}

// Get 告警类型详情。
func (s *AlarmTypeService) Get(ctx context.Context, id int64) (*model.AlarmType, error) {
	return repository.NewAlarmTypeRepo().GetByID(ctx, id)
}

// Create 新增告警类型。
func (s *AlarmTypeService) Create(ctx context.Context, req *request.CreateAlarmTypeReq) (*model.AlarmType, error) {
	m := &model.AlarmType{
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
	if err := repository.NewAlarmTypeRepo().Create(ctx, m); err != nil {
		return nil, fmt.Errorf("新增告警类型失败: %w", err)
	}
	return m, nil
}

// Update 修改告警类型。
func (s *AlarmTypeService) Update(ctx context.Context, id int64, req *request.UpdateAlarmTypeReq) error {
	repo := repository.NewAlarmTypeRepo()
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
		return fmt.Errorf("修改告警类型失败: %w", err)
	}
	return nil
}

// Delete 软删除告警类型。被告警记录或告警设置引用时拒绝删除。
func (s *AlarmTypeService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewAlarmTypeRepo()
	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}

	if n, err := repo.CountAlarms(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return fmt.Errorf("该告警类型下存在 %d 条告警记录，无法删除", n)
	}
	if n, err := repo.CountAlarmSettings(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return fmt.Errorf("该告警类型已被 %d 条告警设置引用，无法删除", n)
	}

	if err := repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除告警类型失败: %w", err)
	}
	return nil
}
