package gb28181

import (
	"context"
	"errors"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标业务分组（树形，与区域正交）
// ════════════════════════════════════════════════════════════════

// GbGroupService 业务分组业务逻辑。
type GbGroupService struct {
	repo *repository.GbGroupRepo
}

// NewGbGroupService 构造业务分组服务。
func NewGbGroupService() *GbGroupService {
	return &GbGroupService{repo: repository.NewGbGroupRepo()}
}

// Tree 业务分组树。
//
// 数据权限过滤后，父节点可能不可见（例如跨部门树）。此时把该节点提升为根，
// 否则整棵子树会从页面上消失 —— 表现为「明明有数据却查不到」。
func (s *GbGroupService) Tree(ctx context.Context, ds *utils.DataScope) ([]*response.GbGroupTreeNode, error) {
	items, err := s.repo.ListAll(ctx, ds)
	if err != nil {
		return nil, err
	}

	nodes := make(map[int64]*response.GbGroupTreeNode, len(items))
	for _, g := range items {
		nodes[g.ID] = &response.GbGroupTreeNode{
			GbGroup:  g,
			Children: []*response.GbGroupTreeNode{},
		}
	}

	roots := make([]*response.GbGroupTreeNode, 0, len(items))
	for _, g := range items {
		node := nodes[g.ID]
		if g.ParentID != nil && *g.ParentID > 0 {
			if parent, ok := nodes[*g.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}
	return roots, nil
}

// List 业务分组简单列表（不分页，供下拉与树表使用）。
func (s *GbGroupService) List(ctx context.Context, ds *utils.DataScope) ([]model.GbGroup, error) {
	items, err := s.repo.ListAll(ctx, ds)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.GbGroup{}
	}
	return items, nil
}

// Create 新增业务分组。
func (s *GbGroupService) Create(ctx context.Context, req *request.CreateGbGroupReq) (*model.GbGroup, error) {
	name := trim(req.Name)
	if name == "" {
		return nil, errors.New("分组名称不能为空")
	}

	var parentID *int64
	if req.ParentID != nil && *req.ParentID > 0 {
		if _, err := s.repo.GetByID(ctx, *req.ParentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("父分组不存在")
			}
			return nil, err
		}
		parentID = req.ParentID
	}

	m := &model.GbGroup{
		Name:           name,
		ParentID:       parentID,
		DeviceID:       trim(req.DeviceID),
		ParentDeviceID: trim(req.ParentDeviceID),
		BusinessGroup:  trim(req.BusinessGroup),
		CivilCode:      trim(req.CivilCode),
		Alias:          trim(req.Alias),
		Remark:         trim(req.Remark),
		Active:         boolOr(req.Active, true),
		DeptID:         model.DeptIDFromCtx(ctx),
	}
	m.Sequence = req.Sequence
	if m.Sequence <= 0 {
		m.Sequence = 10
	}

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 编辑业务分组。
//
// ParentID 传 0 表示提升为根分组；传正整数前会做存在性与成环校验。
func (s *GbGroupService) Update(ctx context.Context, id int64, req *request.UpdateGbGroupReq) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbGroupNotFound
		}
		return err
	}

	fields := map[string]interface{}{}
	if v := trim(req.Name); v != "" {
		fields["name"] = v
	}
	if v := trim(req.DeviceID); v != "" {
		fields["device_id"] = v
	}
	if v := trim(req.ParentDeviceID); v != "" {
		fields["parent_device_id"] = v
	}
	if v := trim(req.BusinessGroup); v != "" {
		fields["business_group"] = v
	}
	if v := trim(req.CivilCode); v != "" {
		fields["civil_code"] = v
	}
	if v := trim(req.Alias); v != "" {
		fields["alias"] = v
	}
	if v := trim(req.Remark); v != "" {
		fields["remark"] = v
	}
	if req.Sequence != nil {
		fields["sequence"] = *req.Sequence
	}
	if req.Active != nil {
		fields["active"] = *req.Active
	}

	if req.ParentID != nil {
		if *req.ParentID <= 0 {
			fields["parent_id"] = nil
		} else {
			parentID := *req.ParentID
			if _, err := s.repo.GetByID(ctx, parentID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errors.New("父分组不存在")
				}
				return err
			}
			cycle, err := s.wouldCycle(ctx, id, parentID)
			if err != nil {
				return err
			}
			if cycle {
				return ErrGbGroupCycle
			}
			fields["parent_id"] = parentID
		}
	}

	return updateGroupFields(ctx, id, fields)
}

// Delete 删除业务分组。
//
// 先校验「无子分组、无通道引用」：级联删除会让用户误删整棵组织树，
// 这里选择显式报错让管理员先处理引用关系。
func (s *GbGroupService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbGroupNotFound
		}
		return err
	}

	children, err := s.repo.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if children > 0 {
		return ErrGbGroupHasChildren
	}

	channels, err := s.repo.CountChannels(ctx, id)
	if err != nil {
		return err
	}
	if channels > 0 {
		return ErrGbGroupInUse
	}

	return s.repo.Delete(ctx, id)
}

// ── 内部工具 ──────────────────────────────────────────────────

// wouldCycle 判断把 id 的父节点改为 parentID 是否会成环。
//
// 从 parentID 向上回溯，若遇到 id 则说明 parentID 是 id 的后代。
// 限制 100 层，防止库中脏数据（历史成环）导致死循环。
func (s *GbGroupService) wouldCycle(ctx context.Context, id, parentID int64) (bool, error) {
	cur := parentID
	for i := 0; i < 100 && cur > 0; i++ {
		if cur == id {
			return true, nil
		}
		p, err := s.repo.GetByID(ctx, cur)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, err
		}
		if p.ParentID == nil || *p.ParentID <= 0 {
			return false, nil
		}
		cur = *p.ParentID
	}
	return false, nil
}

// updateGroupFields 局部更新分组字段。
//
// repository.GbGroupRepo 只有整行 Save，没有 UpdateFields；本批次不修改 repository
// （由其他 Agent 维护），因此用 bootstrap.DB() 写一个等价的轻量更新，
// 并补上与 repository 一致的 updated_by / updated_dt 审计语义。
func updateGroupFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbGroup{}).
		Where("id = ?", id).
		Updates(fields).Error
}
