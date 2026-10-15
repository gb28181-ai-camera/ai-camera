package repository

import (
	"context"
	"errors"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// InspectRepo 巡检中心（巡检任务 / 巡检记录）数据访问层。
type InspectRepo struct{}

// NewInspectRepo 构造巡检仓储。
func NewInspectRepo() *InspectRepo { return &InspectRepo{} }

func (r *InspectRepo) db(ctx context.Context) *gorm.DB {
	return bootstrap.DB().WithContext(ctx)
}

// ────────────────────────────────────────────────────────────────
// 时间区间参数归一化（包内共用，供各领域列表按时间过滤）
// ────────────────────────────────────────────────────────────────

// rangeStartDT 归一化区间起始值：仅日期时补足为当日 00:00:00。
func rangeStartDT(s string) string {
	if len(s) == 10 {
		return s + " 00:00:00"
	}
	return s
}

// rangeEndDT 归一化区间结束值：仅日期时补足为当日 23:59:59（含当日）。
func rangeEndDT(s string) string {
	if len(s) == 10 {
		return s + " 23:59:59"
	}
	return s
}

// ────────────────────────────────────────────────────────────────
// 巡检任务
// ────────────────────────────────────────────────────────────────

// PageTasks 巡检任务分页查询。
// 返回项在任务本体之外冗余巡检人姓名（JOIN ac_user），并按 dept_id 施加数据权限。
func (r *InspectRepo) PageTasks(ctx context.Context, req *request.PageInspectTaskReq, ds *utils.DataScope) ([]response.InspectTaskItem, int64, error) {
	// 分页与统计使用同一组条件，故以闭包重复构建查询，避免 Count 与 Select 相互干扰。
	build := func() *gorm.DB {
		q := r.db(ctx).
			Model(&response.InspectTaskItem{}).
			Joins("LEFT JOIN ac_user AS u ON u.id = ac_inspect_task.inspector_id AND u.deleted_at IS NULL")

		q = ds.ApplyToQuery(q, "ac_inspect_task.dept_id")

		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			q = q.Where("(ac_inspect_task.name ILIKE ? OR ac_inspect_task.location ILIKE ?)", like, like)
		}
		if req.RegionID != nil && *req.RegionID > 0 {
			q = q.Where("ac_inspect_task.region_id = ?", *req.RegionID)
		}
		if req.InspectorID != nil && *req.InspectorID > 0 {
			q = q.Where("ac_inspect_task.inspector_id = ?", *req.InspectorID)
		}
		if req.Status != "" {
			q = q.Where("ac_inspect_task.status = ?", req.Status)
		}
		if req.StartDT != "" {
			q = q.Where("ac_inspect_task.plan_dt >= ?", rangeStartDT(req.StartDT))
		}
		if req.EndDT != "" {
			q = q.Where("ac_inspect_task.plan_dt <= ?", rangeEndDT(req.EndDT))
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]response.InspectTaskItem, 0)
	if err := build().
		Select("ac_inspect_task.*, u.name AS inspector_name").
		Preload("Region").
		Order("ac_inspect_task.plan_dt DESC").
		Order("ac_inspect_task.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetTask 按主键读取巡检任务（含区域）。
func (r *InspectRepo) GetTask(ctx context.Context, id int64) (*model.InspectTask, error) {
	var m model.InspectTask
	if err := r.db(ctx).Preload("Region").Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateTask 新增巡检任务。
func (r *InspectRepo) CreateTask(ctx context.Context, m *model.InspectTask) error {
	return r.db(ctx).Create(m).Error
}

// UpdateTask 全量更新巡检任务的可编辑字段（含零值）。
func (r *InspectRepo) UpdateTask(ctx context.Context, m *model.InspectTask) error {
	return r.db(ctx).
		Model(m).
		Select("name", "region_id", "location", "inspector_id", "content", "plan_dt", "status", "remark").
		Updates(m).Error
}

// DeleteTask 软删除巡检任务。
func (r *InspectRepo) DeleteTask(ctx context.Context, id int64) error {
	return r.db(ctx).Delete(&model.InspectTask{}, id).Error
}

// MarkTaskRead 将巡检任务的消息通知置为已读（只更新 read 列，不动其他字段）。
func (r *InspectRepo) MarkTaskRead(ctx context.Context, id int64) error {
	return r.db(ctx).
		Model(&model.InspectTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{"read": true}).Error
}

// UserNameByID 读取用户姓名，用于校验巡检人是否存在并冗余到列表/快照。
func (r *InspectRepo) UserNameByID(ctx context.Context, id int64) (string, error) {
	var name string
	if err := r.db(ctx).
		Table("ac_user").
		Select("name").
		Where("id = ? AND deleted_at IS NULL", id).
		Scan(&name).Error; err != nil {
		return "", err
	}
	if name == "" {
		return "", gorm.ErrRecordNotFound
	}
	return name, nil
}

// ────────────────────────────────────────────────────────────────
// 巡检记录
// ────────────────────────────────────────────────────────────────

// PageRecords 巡检记录分页查询（含区域）。
func (r *InspectRepo) PageRecords(ctx context.Context, req *request.PageInspectRecordReq, ds *utils.DataScope) ([]model.InspectRecord, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.InspectRecord{})
		q = ds.ApplyToQuery(q, "ac_inspect_record.dept_id")

		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			q = q.Where("(ac_inspect_record.task_name ILIKE ? OR ac_inspect_record.location ILIKE ?)", like, like)
		}
		if req.RegionID != nil && *req.RegionID > 0 {
			q = q.Where("ac_inspect_record.region_id = ?", *req.RegionID)
		}
		if req.InspectorID != nil && *req.InspectorID > 0 {
			q = q.Where("ac_inspect_record.inspector_id = ?", *req.InspectorID)
		}
		if req.StartDT != "" {
			q = q.Where("ac_inspect_record.plan_dt >= ?", rangeStartDT(req.StartDT))
		}
		if req.EndDT != "" {
			q = q.Where("ac_inspect_record.plan_dt <= ?", rangeEndDT(req.EndDT))
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	items := make([]model.InspectRecord, 0)
	if err := build().
		Preload("Region").
		Order("ac_inspect_record.handled_dt DESC").
		Order("ac_inspect_record.id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetRecord 按主键读取巡检记录（含区域）。
func (r *InspectRepo) GetRecord(ctx context.Context, id int64) (*model.InspectRecord, error) {
	var m model.InspectRecord
	if err := r.db(ctx).Preload("Region").Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateRecord 新增巡检记录。
func (r *InspectRepo) CreateRecord(ctx context.Context, m *model.InspectRecord) error {
	return r.db(ctx).Create(m).Error
}

// FinishTask 在事务内完成巡检任务：
//  1. 校验任务存在、归属当前巡检人（否则「无权处理该巡检任务」）、状态为 published（否则「该任务已处理」）；
//  2. 依据任务快照生成巡检记录（任务名称/区域/位置/巡检人/内容/巡检时间），并补齐结果、结果图与处理时间；
//  3. 任务状态置为 finished。
//
// rec 由 Service 预填 result / result_image / inspector / handled_dt / status。
func (r *InspectRepo) FinishTask(ctx context.Context, taskID, userID int64, rec *model.InspectRecord) (*model.InspectRecord, error) {
	err := r.db(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.InspectTask
		if err := tx.Where("id = ?", taskID).First(&task).Error; err != nil {
			return err
		}
		if task.InspectorID != userID {
			return errors.New("无权处理该巡检任务")
		}
		if task.Status != model.InspectStatusPublished {
			return errors.New("该任务已处理")
		}

		rec.TaskID = task.ID
		rec.TaskName = task.Name
		rec.RegionID = task.RegionID
		rec.Location = task.Location
		rec.InspectorID = task.InspectorID
		rec.Content = task.Content
		rec.PlanDT = task.PlanDT
		rec.DeptID = task.DeptID

		if err := tx.Create(rec).Error; err != nil {
			return err
		}

		return tx.Model(&model.InspectTask{}).
			Where("id = ?", taskID).
			Select("status").
			Updates(&model.InspectTask{Status: model.InspectStatusFinished}).Error
	})
	if err != nil {
		return nil, err
	}
	return rec, nil
}
