package repository

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// PostRepo 岗位数据访问。岗位 Code 是 Casbin g3 主体。
type PostRepo struct{}

// NewPostRepo 构造岗位仓储。
func NewPostRepo() *PostRepo { return &PostRepo{} }

// postQuery 组装岗位列表的过滤条件（分页与计数共用，避免语句复用问题）。
func postQuery(db *gorm.DB, req *request.PagePostReq) *gorm.DB {
	q := db.Model(&model.Post{})
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		q = q.Where("name ILIKE ? OR code ILIKE ?", kw, kw)
	}
	if req.Active != nil {
		q = q.Where("active = ?", *req.Active)
	}
	return q
}

// Page 分页查询岗位。
func (r *PostRepo) Page(ctx context.Context, req *request.PagePostReq) ([]model.Post, int64, error) {
	db := bootstrap.DB().WithContext(ctx)

	var total int64
	if err := postQuery(db, req).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.Post
	if err := postQuery(db, req).
		Order("sequence ASC, id ASC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ListActive 查询启用岗位（供下拉选项使用）。
func (r *PostRepo) ListActive(ctx context.Context) ([]model.Post, error) {
	var list []model.Post
	err := bootstrap.DB().WithContext(ctx).
		Where("active = ?", true).
		Order("sequence ASC, id ASC").
		Find(&list).Error
	return list, err
}

// GetByID 查询单条岗位。
func (r *PostRepo) GetByID(ctx context.Context, id int64) (*model.Post, error) {
	var p model.Post
	if err := bootstrap.DB().WithContext(ctx).First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ExistsCode 判断岗位编码是否已被占用（excludeID 用于更新时排除自身）。
func (r *PostRepo) ExistsCode(ctx context.Context, code string, excludeID int64) (bool, error) {
	if code == "" {
		return false, nil
	}
	q := bootstrap.DB().WithContext(ctx).Model(&model.Post{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// CountUsers 统计岗位下的用户数量（判断岗位是否可删除）。
func (r *PostRepo) CountUsers(ctx context.Context, postID int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.User{}).
		Where("post_id = ?", postID).Count(&n).Error
	return n, err
}

// Create 新增岗位。
func (r *PostRepo) Create(ctx context.Context, p *model.Post) error {
	return bootstrap.DB().WithContext(ctx).Omit("RoleIDs").Create(p).Error
}

// Update 全字段保存岗位（角色关联由 ReplaceRoles 维护）。
func (r *PostRepo) Update(ctx context.Context, p *model.Post) error {
	return bootstrap.DB().WithContext(ctx).Omit("RoleIDs").Save(p).Error
}

// DeleteCascade 在事务内删除岗位及其角色关联（ac_post_role_rel）。
func (r *PostRepo) DeleteCascade(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id = ?", id).Delete(&model.PostRoleRel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Post{}, id).Error
	})
}

// ─── 岗位-角色关联 ───────────────────────────────────────────

// ListRoleIDs 查询岗位关联的角色 ID。
func (r *PostRepo) ListRoleIDs(ctx context.Context, postID int64) ([]int64, error) {
	var ids []int64
	err := bootstrap.DB().WithContext(ctx).
		Model(&model.PostRoleRel{}).
		Where("post_id = ?", postID).
		Order("role_id ASC").
		Pluck("role_id", &ids).Error
	return ids, err
}

// ReplaceRoles 覆盖式重建岗位的角色关联（先删后插，同一事务内完成）。
func (r *PostRepo) ReplaceRoles(ctx context.Context, postID int64, roleIDs []int64) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id = ?", postID).Delete(&model.PostRoleRel{}).Error; err != nil {
			return err
		}
		if len(roleIDs) == 0 {
			return nil
		}
		rels := make([]model.PostRoleRel, 0, len(roleIDs))
		seen := make(map[int64]struct{}, len(roleIDs))
		for _, rid := range roleIDs {
			if rid <= 0 {
				continue
			}
			if _, ok := seen[rid]; ok {
				continue
			}
			seen[rid] = struct{}{}
			rels = append(rels, model.PostRoleRel{PostID: postID, RoleID: rid})
		}
		if len(rels) == 0 {
			return nil
		}
		return tx.Create(&rels).Error
	})
}
