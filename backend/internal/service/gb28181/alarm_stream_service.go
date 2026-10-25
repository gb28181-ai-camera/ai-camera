package gb28181

// 国标报警实时推送（SSE）。
//
// 实现方式：轮询 ac_gb_alarm 的 id 游标，而不是改 SIP manager 的写入路径。
// 理由：
//   - manager 属于协议栈（另一模块所有），改写入路径会扩大耦合面；
//   - 报警是低频事件（秒级），每秒一次 `id > lastID` 的索引查询开销可忽略；
//   - id 单调递增，天然避免设备时钟漂移导致的时间戳漏报/重复。
//
// 通知渠道（站内通知 / Webhook）仍由既有报警入库逻辑负责，本服务只做
// 「把新报警推给运维大屏」，不改 NotificationService / WebhookService 的签名。

import (
	"context"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
)

const defaultAlarmStreamInterval = time.Second

// GbAlarmStreamService 报警实时推送服务。
type GbAlarmStreamService struct {
	repo     *repository.GbOpsExtraRepo
	interval time.Duration
}

// NewGbAlarmStreamService 构造报警流服务（默认 1s 轮询）。
func NewGbAlarmStreamService() *GbAlarmStreamService {
	return NewGbAlarmStreamServiceWithInterval(defaultAlarmStreamInterval)
}

// NewGbAlarmStreamServiceWithInterval 指定轮询间隔（测试用）。
func NewGbAlarmStreamServiceWithInterval(d time.Duration) *GbAlarmStreamService {
	if d <= 0 {
		d = defaultAlarmStreamInterval
	}
	return &GbAlarmStreamService{repo: repository.NewGbOpsExtraRepo(), interval: d}
}

// LatestID 返回当前最大报警 ID，作为建连游标（避免重放历史）。
func (s *GbAlarmStreamService) LatestID(ctx context.Context) int64 {
	id, err := s.repo.LatestAlarmID(ctx)
	if err != nil {
		return 0
	}
	return id
}

// PollOnce 拉取一批新报警，返回条目与新的游标。
func (s *GbAlarmStreamService) PollOnce(ctx context.Context, lastID int64) ([]response.GbAlarmItem, int64, error) {
	rows, err := s.repo.AlarmsAfterID(ctx, lastID, 100)
	if err != nil {
		return nil, lastID, err
	}
	items := make([]response.GbAlarmItem, 0, len(rows))
	cursor := lastID
	for i := range rows {
		items = append(items, toGbAlarmStreamItem(rows[i]))
		if rows[i].ID > cursor {
			cursor = rows[i].ID
		}
	}
	return items, cursor, nil
}

// Stream 持续推送新报警，直到 ctx 取消或 emit 返回错误。
func (s *GbAlarmStreamService) Stream(ctx context.Context, lastID int64, emit func(response.GbAlarmItem) error) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		items, cursor, err := s.PollOnce(ctx, lastID)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			utils.Warn("国标报警实时流：查询新报警失败", zap.Error(err))
		} else {
			lastID = cursor
			for _, item := range items {
				if err := emit(item); err != nil {
					return err
				}
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// toGbAlarmStreamItem 把报警模型转为 SSE 事件载荷（不返回通道密码等敏感列）。
func toGbAlarmStreamItem(a model.GbAlarm) response.GbAlarmItem {
	deviceName, channelName := "", ""
	if a.Device != nil {
		deviceName = a.Device.Name
	}
	if a.Channel != nil {
		channelName = a.Channel.Name
	}
	return response.GbAlarmItem{
		ID:               a.ID,
		DeviceID:         a.DeviceID,
		ChannelID:        a.ChannelID,
		DeviceName:       deviceName,
		ChannelName:      channelName,
		GbDeviceID:       a.GbDeviceID,
		GbChannelID:      a.GbChannelID,
		AlarmPriority:    a.AlarmPriority,
		AlarmMethod:      a.AlarmMethod,
		AlarmType:        a.AlarmType,
		AlarmTime:        a.AlarmTime,
		AlarmDescription: a.AlarmDescription,
		Longitude:        a.Longitude,
		Latitude:         a.Latitude,
		SnapPath:         a.SnapPath,
		RecordPath:       a.RecordPath,
		Handled:          a.Handled,
		Remark:           a.Remark,
		CreatedDT:        a.CreatedDT,
		UpdatedDT:        a.UpdatedDT,
		UpdatedBy:        a.UpdatedBy,
	}
}
