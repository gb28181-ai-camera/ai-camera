package repository

import (
	"context"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// exportLimit 导出单次最多返回的记录数，避免大表导出打爆内存。
const exportLimit = 50000

// OperationLogRepo 操作日志（ac_operation_log）数据访问。
// 该表无 dept_id，不做行级数据权限过滤。
type OperationLogRepo struct{}

// NewOperationLogRepo 构造操作日志仓储。
func NewOperationLogRepo() *OperationLogRepo { return &OperationLogRepo{} }

// logQuery 组装日志查询条件（分页、导出共用）。
func logQuery(db *gorm.DB, req *request.PageOperationLogReq) *gorm.DB {
	q := db.Model(&model.OperationLog{})
	if req.Module != "" {
		q = q.Where("module = ?", req.Module)
	}
	if req.Action != "" {
		q = q.Where("action = ?", req.Action)
	}
	if req.Operator != "" {
		q = q.Where("operator ILIKE ?", "%"+req.Operator+"%")
	}
	if req.StartDT != "" {
		if t, err := parseLogTime(req.StartDT); err == nil {
			q = q.Where("created_dt >= ?", t)
		}
	}
	if req.EndDT != "" {
		if t, err := parseLogTime(req.EndDT); err == nil {
			q = q.Where("created_dt <= ?", t)
		}
	}
	return q
}

// parseLogTime 解析 "2006-01-02 15:04:05" 或 "2006-01-02" 两种时间格式。
func parseLogTime(s string) (time.Time, error) {
	layouts := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00", "2006-01-02"}
	var lastErr error
	for _, layout := range layouts {
		var t time.Time
		t, lastErr = time.ParseInLocation(layout, s, time.Local)
		if lastErr == nil {
			return t, nil
		}
	}
	return time.Time{}, lastErr
}

// Page 分页查询操作日志（按操作时间倒序）。
func (r *OperationLogRepo) Page(ctx context.Context, req *request.PageOperationLogReq) ([]model.OperationLog, int64, error) {
	db := bootstrap.DB().WithContext(ctx)

	var total int64
	if err := logQuery(db, req).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var list []model.OperationLog
	if err := logQuery(db, req).
		Order("created_dt DESC, id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ListForExport 查询符合筛选条件的日志（按操作时间倒序，最多 exportLimit 条）。
func (r *OperationLogRepo) ListForExport(ctx context.Context, req *request.PageOperationLogReq) ([]model.OperationLog, error) {
	var list []model.OperationLog
	err := logQuery(bootstrap.DB().WithContext(ctx), req).
		Order("created_dt DESC, id DESC").
		Limit(exportLimit).
		Find(&list).Error
	return list, err
}

// GetByID 查询单条日志。
func (r *OperationLogRepo) GetByID(ctx context.Context, id int64) (*model.OperationLog, error) {
	var l model.OperationLog
	if err := bootstrap.DB().WithContext(ctx).First(&l, id).Error; err != nil {
		return nil, err
	}
	return &l, nil
}

// Delete 删除单条日志（软删除）。
func (r *OperationLogRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.OperationLog{}, id).Error
}

// Clear 清空全部日志（物理删除，日志表不应保留墓碑记录）。
func (r *OperationLogRepo) Clear(ctx context.Context) error {
	return bootstrap.DB().WithContext(ctx).Unscoped().
		Where("1 = 1").Delete(&model.OperationLog{}).Error
}
