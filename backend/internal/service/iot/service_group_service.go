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
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 服务组 / 服务项
// ════════════════════════════════════════════════════════════════

// ServiceGroupService IOT 服务组业务逻辑。
type ServiceGroupService struct {
	repo    *repository.IotServiceGroupRepo
	product *repository.IotProductRepo
}

// NewServiceGroupService 构造服务组服务。
func NewServiceGroupService() *ServiceGroupService {
	return &ServiceGroupService{
		repo:    repository.NewIotServiceGroupRepo(),
		product: repository.NewIotProductRepo(),
	}
}

// Page 服务组分页列表（密码脱敏）。
func (s *ServiceGroupService) Page(ctx context.Context, req *request.PageIotServiceGroupReq) (*response.PageIotServiceGroupResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req)
	if err != nil {
		return nil, err
	}
	return &response.PageIotServiceGroupResp{
		Total: total, Page: req.Page, PageSize: req.PageSize, Items: maskGroups(items),
	}, nil
}

// Get 服务组详情（含服务项，密码脱敏）。
func (s *ServiceGroupService) Get(ctx context.Context, id int64) (*response.IotServiceGroupDetail, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}
	items, err := s.repo.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.IotServiceItem{}
	}
	return &response.IotServiceGroupDetail{
		IotServiceGroup: maskGroup(*m),
		Items:           items,
	}, nil
}

// Create 新增服务组，自动生成组级用户名与密码。
//
// 为什么不在创建接口里接收密码：组凭据会被下发给大量设备，
// 人工设置容易出现弱口令与跨组复用；统一由 crypto/rand 生成更安全，
// 且密码只在设备端 OTA 时下发，管理端始终只见 ****。
func (s *ServiceGroupService) Create(ctx context.Context, req *request.CreateIotServiceGroupReq) (*response.IotServiceGroupDetail, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("服务组名称不能为空")
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, errors.New("服务组编码不能为空")
	}
	groupType := strings.TrimSpace(req.Type)
	if err := validateGroupType(groupType); err != nil {
		return nil, err
	}

	exists, err := s.repo.ExistsByCode(ctx, code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrServiceGroupCodeExists
	}

	username, err := generateUsername(ctx, s.repo, "iot_", 0)
	if err != nil {
		return nil, err
	}
	password, err := GenerateServiceGroupPassword()
	if err != nil {
		return nil, errors.New("生成服务组密码失败")
	}

	m := &model.IotServiceGroup{
		Name:        name,
		Code:        code,
		Type:        groupType,
		Username:    username,
		Password:    password,
		Description: req.Description,
		Active:      boolOr(req.Active, true),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return &response.IotServiceGroupDetail{
		IotServiceGroup: maskGroup(*m),
		Items:           []model.IotServiceItem{},
	}, nil
}

// Update 更新服务组。
func (s *ServiceGroupService) Update(ctx context.Context, id int64, req *request.UpdateIotServiceGroupReq) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceGroupNotFound
		}
		return err
	}

	if v := strings.TrimSpace(req.Name); v != "" {
		m.Name = v
	}
	if v := strings.TrimSpace(req.Code); v != "" && v != m.Code {
		exists, err := s.repo.ExistsByCode(ctx, v, id)
		if err != nil {
			return err
		}
		if exists {
			return ErrServiceGroupCodeExists
		}
		m.Code = v
	}
	if v := strings.TrimSpace(req.Type); v != "" && v != m.Type {
		if err := validateGroupType(v); err != nil {
			return err
		}
		// 类型决定协议白名单。已有服务项若与新类型不兼容，必须先调整服务项，
		// 否则会留下「类型是 aichat、服务项却是 SIP」的脏数据。
		items, err := s.repo.ListItems(ctx, id)
		if err != nil {
			return err
		}
		for _, it := range items {
			if _, err := canonicalProtocol(v, it.Protocol); err != nil {
				return fmt.Errorf("服务组下存在与新类型不兼容的服务项（%s），请先调整服务项", it.Protocol)
			}
		}
		m.Type = v
	}
	if req.Description != "" {
		m.Description = req.Description
	}
	if req.Password != "" {
		m.Password = utils.ApplyMaskedUpdate(req.Password, m.Password)
	}
	if req.Active != nil {
		m.Active = req.Active
	}
	return s.repo.Update(ctx, m)
}

// Delete 删除服务组。有产品引用时拒绝。
func (s *ServiceGroupService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceGroupNotFound
		}
		return err
	}
	cnt, err := s.product.CountByGroup(ctx, id)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return fmt.Errorf("%w（被 %d 个产品引用）", ErrServiceGroupInUse, cnt)
	}
	// 服务项随组一起清理（硬删除，避免占用协议唯一索引）。
	if err := s.repo.ReplaceItems(ctx, id, nil); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// ResetCredentials 轮换服务组的用户名与密码，返回一次性明文。
//
// 这是唯一会改变凭据的入口：设备端已保存旧凭据时，
// 管理员需要在设备下次 OTA 前完成轮换，因此响应直接返回新明文便于抄录。
func (s *ServiceGroupService) ResetCredentials(ctx context.Context, id int64) (*response.IotServiceGroupCredentials, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}
	username, err := generateUsername(ctx, s.repo, "iot_", id)
	if err != nil {
		return nil, err
	}
	password, err := GenerateServiceGroupPassword()
	if err != nil {
		return nil, errors.New("生成服务组密码失败")
	}
	if err := s.repo.UpdateCredentials(ctx, id, username, password); err != nil {
		return nil, err
	}
	return &response.IotServiceGroupCredentials{Username: username, Password: password}, nil
}

// GetItems 查询服务项。
func (s *ServiceGroupService) GetItems(ctx context.Context, id int64) ([]model.IotServiceItem, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}
	items, err := s.repo.ListItems(ctx, id)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.IotServiceItem{}
	}
	return items, nil
}

// ReplaceItems 整体替换服务项，并校验协议白名单。
func (s *ServiceGroupService) ReplaceItems(ctx context.Context, id int64, req *request.ReplaceIotServiceItemsReq) ([]model.IotServiceItem, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}

	items := make([]model.IotServiceItem, 0, len(req.Items))
	seen := make(map[string]struct{}, len(req.Items))
	for _, in := range req.Items {
		protocol, err := canonicalProtocol(m.Type, in.Protocol)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[protocol]; ok {
			return nil, fmt.Errorf("协议 %s 重复，同一服务组内每种协议只能配置一次", protocol)
		}
		seen[protocol] = struct{}{}

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
		items = append(items, model.IotServiceItem{
			GroupID:  id,
			Protocol: protocol,
			Host:     strings.TrimSpace(in.Host),
			Port:     port,
			Path:     strings.TrimSpace(in.Path),
			Extra:    in.Extra,
			Sequence: seq,
			Active:   boolOr(in.Active, true),
		})
	}

	if err := s.repo.ReplaceItems(ctx, id, items); err != nil {
		return nil, err
	}
	return s.GetItems(ctx, id)
}
