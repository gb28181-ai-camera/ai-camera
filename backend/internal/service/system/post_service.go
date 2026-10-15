package system

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// PostService 岗位业务逻辑。岗位 Code 是 Casbin g3 主体。
//
// ac_post 无 dept_id，不做行级数据权限过滤。
type PostService struct{}

// NewPostService 构造岗位服务。
func NewPostService() *PostService { return &PostService{} }

// Page 分页查询岗位。
func (s *PostService) Page(ctx context.Context, req *request.PagePostReq) (*response.PagePostResp, error) {
	req.Normalize()

	list, total, err := repository.NewPostRepo().Page(ctx, req)
	if err != nil {
		return nil, err
	}

	s.fillRoleIDs(ctx, list)

	resp := &response.PagePostResp{Items: list}
	resp.Total = total
	resp.Page = req.Page
	resp.PageSize = req.PageSize
	return resp, nil
}

// Options 返回启用岗位的下拉选项。
func (s *PostService) Options(ctx context.Context) ([]response.OptionItem, error) {
	list, err := repository.NewPostRepo().ListActive(ctx)
	if err != nil {
		return nil, err
	}
	opts := make([]response.OptionItem, 0, len(list))
	for _, p := range list {
		opts = append(opts, response.OptionItem{ID: p.ID, Name: p.Name, Code: p.Code})
	}
	return opts, nil
}

// Get 查询岗位详情（含关联角色 ID）。
func (s *PostService) Get(ctx context.Context, id int64) (*model.Post, error) {
	repo := repository.NewPostRepo()
	p, err := repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	roleIDs, err := repo.ListRoleIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	p.RoleIDs = roleIDs
	return p, nil
}

// Create 新增岗位，并按需写入岗位-角色关联。
func (s *PostService) Create(ctx context.Context, req *request.CreatePostReq) (*model.Post, error) {
	repo := repository.NewPostRepo()

	exists, err := repo.ExistsCode(ctx, req.Code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, bizErr("岗位编码 %s 已存在", req.Code)
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	p := &model.Post{
		Name:        req.Name,
		Code:        req.Code,
		Sequence:    req.Sequence,
		Description: req.Description,
		Active:      &active,
	}
	if p.Sequence == 0 {
		p.Sequence = 10
	}

	if err := repo.Create(ctx, p); err != nil {
		return nil, err
	}

	if len(req.RoleIDs) > 0 {
		if err := repo.ReplaceRoles(ctx, p.ID, req.RoleIDs); err != nil {
			return nil, err
		}
		p.RoleIDs = req.RoleIDs
	}
	return p, nil
}

// Update 更新岗位。未传（零值）的字段不覆盖原值；
// RoleIDs 为 nil 表示不调整岗位角色，传空数组表示清空角色。
func (s *PostService) Update(ctx context.Context, id int64, req *request.UpdatePostReq) error {
	repo := repository.NewPostRepo()

	p, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if req.Code != "" && req.Code != p.Code {
		exists, err := repo.ExistsCode(ctx, req.Code, id)
		if err != nil {
			return err
		}
		if exists {
			return bizErr("岗位编码 %s 已存在", req.Code)
		}
		p.Code = req.Code
	}
	if req.Name != "" {
		p.Name = req.Name
	}
	if req.Sequence != 0 {
		p.Sequence = req.Sequence
	}
	if req.Description != "" {
		p.Description = req.Description
	}
	if req.Active != nil {
		p.Active = req.Active
	}

	if err := repo.Update(ctx, p); err != nil {
		return err
	}

	if req.RoleIDs != nil {
		return repo.ReplaceRoles(ctx, id, req.RoleIDs)
	}
	return nil
}

// Delete 删除岗位。存在关联用户时拒绝删除；删除成功后清理岗位-角色关联（Casbin g3）。
func (s *PostService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewPostRepo()

	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}
	users, err := repo.CountUsers(ctx, id)
	if err != nil {
		return err
	}
	if users > 0 {
		return bizErr("该岗位下存在 %d 个用户，请先调整用户岗位", users)
	}

	return repo.DeleteCascade(ctx, id)
}

// fillRoleIDs 批量填充列表项的关联角色 ID（避免逐行查询）。
func (s *PostService) fillRoleIDs(ctx context.Context, list []model.Post) {
	if len(list) == 0 {
		return
	}
	repo := repository.NewPostRepo()
	for i := range list {
		roleIDs, err := repo.ListRoleIDs(ctx, list[i].ID)
		if err != nil {
			continue
		}
		list[i].RoleIDs = roleIDs
	}
}
