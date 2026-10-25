package repository

import (
	"context"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 云端录像多段合并任务数据访问
// ════════════════════════════════════════════════════════════════

// GbCloudMergeRepo 合并任务数据访问。
type GbCloudMergeRepo struct{}

// NewGbCloudMergeRepo 构造合并任务仓储。
func NewGbCloudMergeRepo() *GbCloudMergeRepo { return &GbCloudMergeRepo{} }

// GetRecordsByIDs 按主键批量读取云端录像。
func (r *GbCloudMergeRepo) GetRecordsByIDs(ctx context.Context, ids []int64) ([]model.GbCloudRecord, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var items []model.GbCloudRecord
	err := bootstrap.DB().WithContext(ctx).
		Where("id IN ?", ids).
		Find(&items).Error
	return items, err
}

// CreateTask 新增合并任务。
func (r *GbCloudMergeRepo) CreateTask(ctx context.Context, t *model.GbRecordMergeTask) error {
	return bootstrap.DB().WithContext(ctx).Create(t).Error
}

// GetTask 按主键查询任务。
func (r *GbCloudMergeRepo) GetTask(ctx context.Context, id int64) (*model.GbRecordMergeTask, error) {
	var t model.GbRecordMergeTask
	if err := bootstrap.DB().WithContext(ctx).First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateTaskFields 增量更新任务字段。
func (r *GbCloudMergeRepo) UpdateTaskFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	return bootstrap.DB().WithContext(ctx).Model(&model.GbRecordMergeTask{}).
		Where("id = ?", id).Updates(fields).Error
}

// PageTasks 分页查询合并任务。
func (r *GbCloudMergeRepo) PageTasks(ctx context.Context, req *request.PageGbMergeTaskReq, ds *utils.DataScope) ([]model.GbRecordMergeTask, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbRecordMergeTask{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR file_name ILIKE ?)", like, like)
		}
		if st := strings.TrimSpace(req.Status); st != "" {
			q = q.Where("status = ?", st)
		}
		if ds != nil {
			q = ds.ApplyToQuery(q, "ac_gb_record_merge_task.dept_id")
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.GbRecordMergeTask
	err := build().Order("id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error
	return items, total, err
}
