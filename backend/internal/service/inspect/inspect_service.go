package inspect

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// InspectService 巡检中心业务逻辑：
// PC 端负责巡检任务的配置与巡检记录的查询，移动端负责完成巡检任务。
type InspectService struct {
	repo *repository.InspectRepo
}

// NewInspectService 构造巡检服务。
func NewInspectService() *InspectService {
	return &InspectService{repo: repository.NewInspectRepo()}
}

// ────────────────────────────────────────────────────────────────
// 巡检任务
// ────────────────────────────────────────────────────────────────

// PageTasks 巡检任务分页查询。
func (s *InspectService) PageTasks(ctx context.Context, req *request.PageInspectTaskReq, ds *utils.DataScope) (*response.PageInspectTaskResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageTasks(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageInspectTaskResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// GetTask 巡检任务详情。
func (s *InspectService) GetTask(ctx context.Context, id int64) (*model.InspectTask, error) {
	return s.repo.GetTask(ctx, id)
}

// CreateTask 创建巡检任务。status 缺省为 published，消息已读标记缺省为 false。
func (s *InspectService) CreateTask(ctx context.Context, req *request.CreateInspectTaskReq) (*model.InspectTask, error) {
	if _, err := s.repo.UserNameByID(ctx, req.InspectorID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("巡检人不存在")
		}
		return nil, err
	}

	planDT, err := parseDT(req.PlanDT)
	if err != nil {
		return nil, err
	}

	status := req.Status
	if status != model.InspectStatusPending && status != model.InspectStatusPublished {
		status = model.InspectStatusPublished
	}

	m := &model.InspectTask{
		Active:      boolPtr(true),
		Name:        strings.TrimSpace(req.Name),
		RegionID:    req.RegionID,
		Location:    req.Location,
		InspectorID: req.InspectorID,
		Content:     req.Content,
		PlanDT:      planDT,
		Status:      status,
		Read:        boolPtr(false),
		Remark:      req.Remark,
	}
	if err := s.repo.CreateTask(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateTask 更新巡检任务。仅 published / pending 状态可编辑；已完成任务不可修改。
func (s *InspectService) UpdateTask(ctx context.Context, id int64, req *request.UpdateInspectTaskReq) error {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if task.Status != model.InspectStatusPublished && task.Status != model.InspectStatusPending {
		if task.Status == model.InspectStatusFinished {
			return errors.New("已完成的巡检任务不可修改")
		}
		return errors.New("该巡检任务不可修改")
	}

	if _, err := s.repo.UserNameByID(ctx, req.InspectorID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("巡检人不存在")
		}
		return err
	}

	planDT, err := parseDT(req.PlanDT)
	if err != nil {
		return err
	}

	status := req.Status
	switch status {
	case model.InspectStatusPending, model.InspectStatusPublished, model.InspectStatusCanceled:
	default:
		status = task.Status
	}

	m := &model.InspectTask{
		BaseModel:   model.BaseModel{ID: id},
		Name:        strings.TrimSpace(req.Name),
		RegionID:    req.RegionID,
		Location:    req.Location,
		InspectorID: req.InspectorID,
		Content:     req.Content,
		PlanDT:      planDT,
		Status:      status,
		Remark:      req.Remark,
	}
	return s.repo.UpdateTask(ctx, m)
}

// MarkTaskRead 标记巡检任务的消息通知为已读。已读时直接返回，保证幂等；
// 任务不存在时返回 gorm.ErrRecordNotFound（Handler 转为 404「巡检任务不存在」）。
func (s *InspectService) MarkTaskRead(ctx context.Context, id int64) error {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if task.Read != nil && *task.Read {
		return nil
	}
	return s.repo.MarkTaskRead(ctx, id)
}

// DeleteTask 删除巡检任务（软删除）。已完成任务不可删除。
func (s *InspectService) DeleteTask(ctx context.Context, id int64) error {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if task.Status == model.InspectStatusFinished {
		return errors.New("已完成的巡检任务不可删除")
	}
	return s.repo.DeleteTask(ctx, id)
}

// ────────────────────────────────────────────────────────────────
// 巡检记录
// ────────────────────────────────────────────────────────────────

// PageRecords 巡检记录分页查询。
func (s *InspectService) PageRecords(ctx context.Context, req *request.PageInspectRecordReq, ds *utils.DataScope) (*response.PageInspectRecordResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageRecords(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageInspectRecordResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// GetRecord 巡检记录详情。
func (s *InspectService) GetRecord(ctx context.Context, id int64) (*model.InspectRecord, error) {
	return s.repo.GetRecord(ctx, id)
}

// FinishTask 移动端完成巡检任务：
// 事务内校验任务归属与状态，生成巡检记录（保留任务快照）并把任务置为 finished。
func (s *InspectService) FinishTask(ctx context.Context, taskID, userID int64, req *request.FinishInspectTaskReq) (*model.InspectRecord, error) {
	userName, err := s.repo.UserNameByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("巡检人不存在")
		}
		return nil, err
	}

	now := time.Now()
	rec := &model.InspectRecord{
		Inspector:   userName,
		Result:      req.Result,
		ResultImage: req.ResultImage,
		HandledDT:   &now,
		Status:      model.InspectStatusFinished,
	}
	return s.repo.FinishTask(ctx, taskID, userID, rec)
}

// ────────────────────────────────────────────────────────────────
// 内部工具
// ────────────────────────────────────────────────────────────────

// parseDT 解析时间入参，支持 YYYY-MM-DD / YYYY-MM-DD HH:mm / YYYY-MM-DD HH:mm:ss / RFC3339；
// 空串取当前时间。
func parseDT(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now(), nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("时间格式不正确")
}

// boolPtr 返回布尔值的指针，用于 *bool 字段显式赋值。
func boolPtr(b bool) *bool { return &b }
