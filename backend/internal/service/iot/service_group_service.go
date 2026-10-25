package iot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 服务组 / 服务
// ════════════════════════════════════════════════════════════════

// ServiceGroupService IOT 服务组业务逻辑。
type ServiceGroupService struct {
	repo           *repository.IotServiceGroupRepo
	product        *repository.IotProductRepo
	productService *repository.IotProductServiceRepo
}

// NewServiceGroupService 构造服务组服务。
func NewServiceGroupService() *ServiceGroupService {
	return &ServiceGroupService{
		repo:           repository.NewIotServiceGroupRepo(),
		product:        repository.NewIotProductRepo(),
		productService: repository.NewIotProductServiceRepo(),
	}
}

// Page 服务组分页列表。
func (s *ServiceGroupService) Page(ctx context.Context, req *request.PageIotServiceGroupReq) (*response.PageIotServiceGroupResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req)
	if err != nil {
		return nil, err
	}
	counts, err := s.product.ReferenceCountsByGroup(ctx)
	if err != nil {
		return nil, err
	}
	out := maskGroups(items)
	for i := range out {
		out[i].ReferencedByProducts = counts[out[i].ID]
	}
	return &response.PageIotServiceGroupResp{
		Total: total, Page: req.Page, PageSize: req.PageSize, Items: out,
	}, nil
}

// Get 服务组详情（含服务）。
func (s *ServiceGroupService) Get(ctx context.Context, id int64) (*response.IotServiceGroupDetail, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}
	services, err := s.repo.ListServices(ctx, id)
	if err != nil {
		return nil, err
	}
	if services == nil {
		services = []model.IotService{}
	}
	group := maskGroup(*m)
	cnt, err := s.product.CountByGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	group.ReferencedByProducts = cnt
	return &response.IotServiceGroupDetail{
		IotServiceGroup: group,
		Services:        services,
	}, nil
}

// Create 服务组为系统预置数据，不开放新增。
func (s *ServiceGroupService) Create(ctx context.Context, req *request.CreateIotServiceGroupReq) (*response.IotServiceGroupDetail, error) {
	return nil, ErrServiceGroupCreateForbidden
}

// ensureMutable 校验服务组未被任何产品引用。
func (s *ServiceGroupService) ensureMutable(ctx context.Context, id int64) error {
	cnt, err := s.product.CountByGroup(ctx, id)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return fmt.Errorf("%w（被 %d 个产品引用，请先在产品上解绑）", ErrServiceGroupInUse, cnt)
	}
	return nil
}

// Update 更新服务组（编码/类型不可修改）。
func (s *ServiceGroupService) Update(ctx context.Context, id int64, req *request.UpdateIotServiceGroupReq) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceGroupNotFound
		}
		return err
	}
	if err := s.ensureMutable(ctx, id); err != nil {
		return err
	}
	if v := strings.TrimSpace(req.Code); v != "" && v != m.Code {
		return ErrServiceGroupCodeImmutable
	}
	if v := strings.TrimSpace(req.Type); v != "" && v != m.Type {
		return ErrServiceGroupTypeImmutable
	}
	if v := strings.TrimSpace(req.Name); v != "" {
		m.Name = v
	}
	if req.Description != "" {
		m.Description = req.Description
	}
	if req.Active != nil {
		m.Active = req.Active
	}
	return s.repo.Update(ctx, m)
}

// Delete 删除服务组。固定三条不允许删除；其他历史数据在未被引用时可删。
func (s *ServiceGroupService) Delete(ctx context.Context, id int64) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceGroupNotFound
		}
		return err
	}
	if model.IsFixedServiceGroupCode(m.Code) {
		return ErrServiceGroupDeleteForbidden
	}
	if err := s.ensureMutable(ctx, id); err != nil {
		return err
	}
	services, err := s.repo.ListServices(ctx, id)
	if err != nil {
		return err
	}
	for _, svc := range services {
		if err := s.productService.DeleteByService(ctx, svc.ID); err != nil {
			return err
		}
	}
	if err := s.repo.ReplaceServices(ctx, id, nil); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// GetServices 查询服务组下的服务。
func (s *ServiceGroupService) GetServices(ctx context.Context, id int64) ([]model.IotService, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}
	services, err := s.repo.ListServices(ctx, id)
	if err != nil {
		return nil, err
	}
	if services == nil {
		services = []model.IotService{}
	}
	return services, nil
}

// ReplaceServices 整体替换服务组的服务，并校验协议白名单。
func (s *ServiceGroupService) ReplaceServices(ctx context.Context, id int64, req *request.ReplaceIotServicesReq) ([]model.IotService, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}
	if err := s.ensureMutable(ctx, id); err != nil {
		return nil, err
	}

	services := make([]model.IotService, 0, len(req.Services))
	seen := make(map[string]struct{}, len(req.Services))
	for _, in := range req.Services {
		code := strings.TrimSpace(in.Code)
		if code == "" {
			return nil, errors.New("服务编码不能为空")
		}
		if _, ok := seen[code]; ok {
			return nil, fmt.Errorf("服务编码 %s 重复，同一服务组内编码必须唯一", code)
		}
		seen[code] = struct{}{}

		protocol, err := canonicalProtocol(m.Type, in.Protocol)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(in.Name)
		if name == "" {
			name = protocol
		}
		port := in.Port
		if port == 0 {
			port = protocolDefaultPorts[protocol]
		}
		if port < 0 || port > 65535 {
			return nil, fmt.Errorf("协议 %s 的端口 %d 不合法", protocol, port)
		}
		seq := in.Sequence
		if seq <= 0 {
			seq = 10
		}
		services = append(services, model.IotService{
			GroupID:  id,
			Code:     code,
			Name:     name,
			Protocol: protocol,
			Host:     strings.TrimSpace(in.Host),
			Port:     port,
			Path:     strings.TrimSpace(in.Path),
			Extra:    in.Extra,
			Sequence: seq,
			Active:   boolOr(in.Active, true),
		})
	}

	if err := s.repo.ReplaceServices(ctx, id, services); err != nil {
		return nil, err
	}
	return s.GetServices(ctx, id)
}
