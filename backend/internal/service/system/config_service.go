package system

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// ConfigService 系统参数业务逻辑。列表为非分页接口，支持 keyword 模糊匹配。
type ConfigService struct{}

// NewConfigService 构造系统参数服务。
func NewConfigService() *ConfigService { return &ConfigService{} }

// List 查询系统参数列表（非分页）。keyword 模糊匹配参数名称与参数键。
func (s *ConfigService) List(ctx context.Context, keyword string) ([]model.SystemConfig, error) {
	return repository.NewConfigRepo().List(ctx, keyword)
}

// Create 新增系统参数。
func (s *ConfigService) Create(ctx context.Context, req *request.CreateConfigReq) (*model.SystemConfig, error) {
	repo := repository.NewConfigRepo()

	exists, err := repo.ExistsCode(ctx, req.Key, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, bizErr("参数键 %s 已存在", req.Key)
	}

	active := true
	c := &model.SystemConfig{
		Name:        req.Name,
		Code:        req.Key,
		Value:       req.Value,
		GroupName:   req.GroupName,
		DataType:    req.DataType,
		Sequence:    req.Sequence,
		Description: req.Description,
		Active:      &active,
	}
	if c.DataType == "" {
		c.DataType = "string"
	}
	if c.GroupName == "" {
		c.GroupName = "system"
	}
	if c.Sequence == 0 {
		c.Sequence = 10
	}

	if err := repo.Create(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Update 更新系统参数。未传（零值）的字段不覆盖原值。
func (s *ConfigService) Update(ctx context.Context, id int64, req *request.UpdateConfigReq) error {
	repo := repository.NewConfigRepo()

	c, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if req.Key != "" && req.Key != c.Code {
		exists, err := repo.ExistsCode(ctx, req.Key, id)
		if err != nil {
			return err
		}
		if exists {
			return bizErr("参数键 %s 已存在", req.Key)
		}
		c.Code = req.Key
	}
	if req.Name != "" {
		c.Name = req.Name
	}
	if req.Value != "" {
		c.Value = req.Value
	}
	if req.DataType != "" {
		c.DataType = req.DataType
	}
	if req.GroupName != "" {
		c.GroupName = req.GroupName
	}
	if req.Sequence != 0 {
		c.Sequence = req.Sequence
	}
	if req.Description != "" {
		c.Description = req.Description
	}

	return repo.Update(ctx, c)
}

// Delete 删除系统参数。
func (s *ConfigService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewConfigRepo()
	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}
	return repo.Delete(ctx, id)
}

// SetActive 启用/停用系统参数，供 /enable 与 /disable 两个端点复用。
func (s *ConfigService) SetActive(ctx context.Context, id int64, active bool) error {
	repo := repository.NewConfigRepo()
	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}
	return repo.SetActive(ctx, id, active)
}
