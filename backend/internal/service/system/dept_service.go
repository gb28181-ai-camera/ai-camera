package system

import (
	"context"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// DeptService 部门（组织）业务逻辑。
//
// level 与 path 由后端自动维护：level = 父 level + 1（根为 1），
// path = 父 path + "/" + code（根为 "/" + code）。变更上级或编码时会级联重算全部下级。
type DeptService struct{}

// NewDeptService 构造部门服务。
func NewDeptService() *DeptService { return &DeptService{} }

// Tree 返回部门树（含停用部门，供管理页面展示）。
func (s *DeptService) Tree(ctx context.Context) ([]model.Dept, error) {
	list, err := repository.NewDeptRepo().ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return buildDeptTree(list), nil
}

// Options 返回启用部门的下拉选项。
func (s *DeptService) Options(ctx context.Context) ([]response.OptionItem, error) {
	list, err := repository.NewDeptRepo().ListActive(ctx)
	if err != nil {
		return nil, err
	}
	opts := make([]response.OptionItem, 0, len(list))
	for _, d := range list {
		opts = append(opts, response.OptionItem{ID: d.ID, Name: d.Name, Code: d.Code})
	}
	return opts, nil
}

// Get 查询部门详情。
func (s *DeptService) Get(ctx context.Context, id int64) (*model.Dept, error) {
	return repository.NewDeptRepo().GetByID(ctx, id)
}

// Create 新增部门，自动推导 level 与 path。
func (s *DeptService) Create(ctx context.Context, req *request.CreateDeptReq) (*model.Dept, error) {
	repo := repository.NewDeptRepo()

	if req.Code != "" {
		exists, err := repo.ExistsCode(ctx, req.Code, 0)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, bizErr("部门编码 %s 已存在", req.Code)
		}
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	d := &model.Dept{
		Name:        req.Name,
		Code:        req.Code,
		ParentID:    req.ParentID,
		Sequence:    req.Sequence,
		LeaderID:    req.LeaderID,
		Description: req.Description,
		Active:      &active,
	}
	if d.Sequence == 0 {
		d.Sequence = 10
	}

	level, path, err := s.resolveHierarchy(ctx, req.ParentID, req.Code)
	if err != nil {
		return nil, err
	}
	d.Level = level
	d.Path = path

	if err := repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// Update 更新部门。未传（零值）的字段不覆盖原值；
// 变更上级部门或部门编码时，级联重算本部门与全部下级的 level / path。
func (s *DeptService) Update(ctx context.Context, id int64, req *request.UpdateDeptReq) error {
	repo := repository.NewDeptRepo()

	d, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	oldLevel, oldPath := d.Level, d.Path
	parentChanged := false
	codeChanged := false

	if req.Code != "" && req.Code != d.Code {
		exists, err := repo.ExistsCode(ctx, req.Code, id)
		if err != nil {
			return err
		}
		if exists {
			return bizErr("部门编码 %s 已存在", req.Code)
		}
		d.Code = req.Code
		codeChanged = true
	}
	if req.ParentID != nil && (d.ParentID == nil || *d.ParentID != *req.ParentID) {
		if *req.ParentID == id {
			return bizErr("上级部门不能是部门自身")
		}
		if err := s.ensureNotDescendant(ctx, d, *req.ParentID); err != nil {
			return err
		}
		d.ParentID = req.ParentID
		parentChanged = true
	}
	if req.Name != "" {
		d.Name = req.Name
	}
	if req.Sequence != 0 {
		d.Sequence = req.Sequence
	}
	if req.LeaderID != nil {
		d.LeaderID = req.LeaderID
	}
	if req.Description != "" {
		d.Description = req.Description
	}
	if req.Active != nil {
		d.Active = req.Active
	}

	if parentChanged || codeChanged {
		level, path, err := s.resolveHierarchy(ctx, d.ParentID, d.Code)
		if err != nil {
			return err
		}
		d.Level = level
		d.Path = path
	}

	if err := repo.Update(ctx, d); err != nil {
		return err
	}

	if d.Path != oldPath {
		if err := repo.UpdateSubtree(ctx, oldPath, d.Path, d.Level-oldLevel); err != nil {
			return err
		}
	}
	return nil
}

// Delete 删除部门。存在下级部门或在职用户时拒绝删除；
// 删除成功后清理部门-角色关联（Casbin g2）。
func (s *DeptService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewDeptRepo()

	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}
	n, err := repo.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return bizErr("该部门下存在 %d 个子部门，请先删除子部门", n)
	}
	users, err := repo.CountUsers(ctx, id)
	if err != nil {
		return err
	}
	if users > 0 {
		return bizErr("该部门下存在 %d 个用户，请先调整用户所属部门", users)
	}

	return repo.DeleteCascade(ctx, id)
}

// ─── 内部方法 ────────────────────────────────────────────────

// resolveHierarchy 依据父部门推导 level 与 path。
func (s *DeptService) resolveHierarchy(ctx context.Context, parentID *int64, code string) (int, string, error) {
	if parentID == nil || *parentID <= 0 {
		return 1, "/" + code, nil
	}
	parent, err := repository.NewDeptRepo().GetByID(ctx, *parentID)
	if err != nil {
		if IsNotFound(err) {
			return 0, "", bizErr("上级部门不存在")
		}
		return 0, "", err
	}
	level := parent.Level + 1
	if level < 2 {
		level = 2
	}
	return level, parent.Path + "/" + code, nil
}

// ensureNotDescendant 校验 newParentID 不是当前部门的后代，防止部门树成环。
func (s *DeptService) ensureNotDescendant(ctx context.Context, cur *model.Dept, newParentID int64) error {
	parent, err := repository.NewDeptRepo().GetByID(ctx, newParentID)
	if err != nil {
		if IsNotFound(err) {
			return bizErr("上级部门不存在")
		}
		return err
	}
	if parent.ID == cur.ID {
		return bizErr("上级部门不能是部门自身")
	}
	// path 前缀天然表达祖先关系：newParent.Path 以 cur.Path 开头即为后代
	if cur.Path != "" && strings.HasPrefix(parent.Path, cur.Path+"/") {
		return bizErr("上级部门不能是当前部门的下级")
	}
	return nil
}

// buildDeptTree 将扁平部门列表组装为树（children 按输入顺序，即 sequence 升序）。
func buildDeptTree(list []model.Dept) []model.Dept {
	exists := make(map[int64]struct{}, len(list))
	for _, d := range list {
		exists[d.ID] = struct{}{}
	}

	childrenOf := make(map[int64][]model.Dept, len(list))
	roots := make([]model.Dept, 0, len(list))
	for _, d := range list {
		if d.ParentID != nil && *d.ParentID > 0 && *d.ParentID != d.ID {
			if _, ok := exists[*d.ParentID]; ok {
				childrenOf[*d.ParentID] = append(childrenOf[*d.ParentID], d)
				continue
			}
		}
		roots = append(roots, d)
	}

	var fill func(n model.Dept, depth int) model.Dept
	fill = func(n model.Dept, depth int) model.Dept {
		kids := childrenOf[n.ID]
		if len(kids) == 0 || depth >= maxTreeDepth {
			n.Children = nil
			return n
		}
		out := make([]model.Dept, 0, len(kids))
		for _, k := range kids {
			if k.ID == n.ID {
				continue
			}
			out = append(out, fill(k, depth+1))
		}
		n.Children = out
		return n
	}

	for i := range roots {
		roots[i] = fill(roots[i], 1)
	}
	return roots
}
