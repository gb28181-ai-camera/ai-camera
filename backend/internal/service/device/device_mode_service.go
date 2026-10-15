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
// 资源管理 - 设备模式
// ════════════════════════════════════════════════════════════════

// DeviceModeService 设备模式业务逻辑。
type DeviceModeService struct {
	repo *repository.DeviceModeRepo
}

// NewDeviceModeService 构建设备模式服务。
func NewDeviceModeService() *DeviceModeService {
	return &DeviceModeService{repo: repository.NewDeviceModeRepo()}
}

// Page 设备模式分页列表。
func (s *DeviceModeService) Page(ctx context.Context, req *request.PageDeviceModeReq, ds *utils.DataScope) (*response.PageDeviceModeResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.DeviceMode{}
	}
	return &response.PageDeviceModeResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// Options 设备模式下拉选项。Name 取模式名称，Code 取模式编码。
func (s *DeviceModeService) Options(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	items, err := s.repo.Options(ctx, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.OptionItem, 0, len(items))
	for i := range items {
		out = append(out, response.OptionItem{
			ID:   items[i].ID,
			Name: items[i].Name,
			Code: items[i].Code,
		})
	}
	return out, nil
}

// Create 新增设备模式。
func (s *DeviceModeService) Create(ctx context.Context, req *request.CreateDeviceModeReq) (*model.DeviceMode, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("模式名称不能为空")
	}

	exists, err := s.repo.ExistsByName(ctx, name, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("模式名称已存在")
	}

	m := &model.DeviceMode{
		Name:     name,
		Code:     strings.TrimSpace(req.Code),
		Sequence: req.Sequence,
		Remark:   req.Remark,
	}
	if m.Sequence <= 0 {
		m.Sequence = 10
	}
	m.Active = boolOr(req.Active, true)

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 更新设备模式。文本非空才覆盖，数值大于 0 才覆盖，指针非 nil 才覆盖。
func (s *DeviceModeService) Update(ctx context.Context, id int64, req *request.UpdateDeviceModeReq) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrModeNotFound
		}
		return err
	}

	if v := strings.TrimSpace(req.Name); v != "" && v != m.Name {
		exists, err := s.repo.ExistsByName(ctx, v, id)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("模式名称已存在")
		}
		m.Name = v
	}
	if v := strings.TrimSpace(req.Code); v != "" {
		m.Code = v
	}
	if req.Sequence > 0 {
		m.Sequence = req.Sequence
	}
	if v := strings.TrimSpace(req.Remark); v != "" {
		m.Remark = v
	}
	if req.Active != nil {
		m.Active = req.Active
	}

	return s.repo.Update(ctx, m)
}

// Delete 删除设备模式（软删除）。
func (s *DeviceModeService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrModeNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}
