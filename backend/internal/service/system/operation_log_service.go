package system

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// OperationLogService 操作日志业务逻辑。日志由中间件自动写入，本服务只提供查询与清理。
type OperationLogService struct{}

// NewOperationLogService 构造操作日志服务。
func NewOperationLogService() *OperationLogService { return &OperationLogService{} }

// Page 分页查询操作日志。
func (s *OperationLogService) Page(ctx context.Context, req *request.PageOperationLogReq) (*response.PageOperationLogResp, error) {
	req.Normalize()

	list, total, err := repository.NewOperationLogRepo().Page(ctx, req)
	if err != nil {
		return nil, err
	}

	resp := &response.PageOperationLogResp{Items: list}
	resp.Total = total
	resp.Page = req.Page
	resp.PageSize = req.PageSize
	return resp, nil
}

// Get 查询单条操作日志。
func (s *OperationLogService) Get(ctx context.Context, id int64) (*model.OperationLog, error) {
	return repository.NewOperationLogRepo().GetByID(ctx, id)
}

// Delete 删除单条操作日志。
func (s *OperationLogService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewOperationLogRepo()
	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}
	return repo.Delete(ctx, id)
}

// Clear 清空全部操作日志。
func (s *OperationLogService) Clear(ctx context.Context) error {
	return repository.NewOperationLogRepo().Clear(ctx)
}

// Export 按筛选条件导出操作日志（不分页，最多 5 万条）。
func (s *OperationLogService) Export(ctx context.Context, req *request.PageOperationLogReq) ([]model.OperationLog, error) {
	return repository.NewOperationLogRepo().ListForExport(ctx, req)
}
