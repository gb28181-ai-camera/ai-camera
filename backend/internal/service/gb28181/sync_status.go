package gb28181

import (
	"context"
	"errors"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/device"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 目录同步进度（WVP 对齐 GET /web/gb/devices/:id/sync-status）
//
// 状态来源是 gb28181/device 包的进程级快照，不落库：
//   - 单进程（embedded）部署可直接读取；
//   - 分进程部署 / 进程重启后读不到，返回 idle —— 同步是异步且可重发的，
//     把「读不到」显示成失败反而会误导运维。
// ════════════════════════════════════════════════════════════════

// GbDeviceSyncStatusService 目录同步进度业务逻辑。
type GbDeviceSyncStatusService struct {
	deviceRepo *repository.GbDeviceExtraRepo
}

// NewGbDeviceSyncStatusService 构造目录同步进度服务。
func NewGbDeviceSyncStatusService() *GbDeviceSyncStatusService {
	return &GbDeviceSyncStatusService{deviceRepo: repository.NewGbDeviceExtraRepo()}
}

// SyncStatus 查询设备最近一次目录同步的进度。
func (s *GbDeviceSyncStatusService) SyncStatus(ctx context.Context, id int64) (*response.GbDeviceSyncStatusResp, error) {
	d, err := s.deviceRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbDeviceNotFound
		}
		return nil, err
	}

	resp := &response.GbDeviceSyncStatusResp{
		DeviceID:   d.ID,
		GbDeviceID: d.GbDeviceID,
		// 默认 idle：本进程没有同步记录（未触发过、或分进程/重启后丢失）。
		Status: "idle",
	}
	st, ok := device.CatalogSyncStatusOf(d.GbDeviceID)
	if !ok {
		return resp, nil
	}

	resp.Status = st.Status
	resp.Total = st.Total
	resp.Current = st.Current
	resp.Error = st.Error
	if !st.StartedAt.IsZero() {
		t := st.StartedAt
		resp.StartedDT = &t
	}
	if !st.FinishedAt.IsZero() {
		t := st.FinishedAt
		resp.FinishedDT = &t
	}
	return resp, nil
}
