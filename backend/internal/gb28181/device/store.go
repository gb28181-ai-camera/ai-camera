package device

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// ErrNoDatabase 表示 bootstrap 未初始化数据库。
//
// 为什么需要它：信令日志由后台协程异步写入，若 DB 为 nil 而直接调用 repository，
// gorm 会在协程里 panic —— 一个日志问题会带崩整个进程。所有 Store 方法都先判空。
var ErrNoDatabase = errors.New("数据库未初始化")

// Store 是 device 包对数据访问的最小依赖，便于单测注入假实现。
//
// 实现全部复用 repository 层的既有方法（不改 repository，避免与并行开发冲突）；
// 唯一例外是「批量把过期设备置离线」，repository 没有对应方法，
// 直接用 bootstrap.DB() 执行一条 UPDATE —— 语义单一，不值得新增仓储方法。
type Store interface {
	FindDevice(ctx context.Context, gbDeviceID string) (*model.Device, error)
	UpdateDevice(ctx context.Context, id int64, fields map[string]interface{}) error

	FindChannel(ctx context.Context, gbChannelID string) (*model.GbChannel, error)
	UpsertChannel(ctx context.Context, ch *model.GbChannel) error
	// UpdateChannel 局部更新通道，供目录事件（ON/OFF/DEL/VLOST）修改状态用。
	UpdateChannel(ctx context.Context, id int64, fields map[string]interface{}) error
	CountChannels(ctx context.Context, deviceID int64) (int64, error)

	CreateSipLog(ctx context.Context, entry *model.GbSipLog) error
	CreateAlarm(ctx context.Context, alarm *model.GbAlarm) error
	CreateMobilePosition(ctx context.Context, pos *model.GbMobilePosition) error

	MarkExpiredOffline(ctx context.Context, now time.Time) (int64, error)
}

// GbStore 是 Store 的生产实现。
type GbStore struct{}

// NewGbStore 构造数据访问实现。
func NewGbStore() *GbStore { return &GbStore{} }

// FindDevice 按国标编码查询设备。
func (s *GbStore) FindDevice(ctx context.Context, gbDeviceID string) (*model.Device, error) {
	return repository.NewGbDeviceRepo().FindByGbDeviceID(ctx, gbDeviceID)
}

// UpdateDevice 局部更新设备字段。
func (s *GbStore) UpdateDevice(ctx context.Context, id int64, fields map[string]interface{}) error {
	return repository.NewGbDeviceRepo().UpdateRegisterState(ctx, id, fields)
}

// FindChannel 按国标编码查询通道（保留用户定制字段用）。
func (s *GbStore) FindChannel(ctx context.Context, gbChannelID string) (*model.GbChannel, error) {
	return repository.NewGbChannelRepo().GetByGbChannelID(ctx, gbChannelID)
}

// UpsertChannel 按国标编码 upsert 通道。
func (s *GbStore) UpsertChannel(ctx context.Context, ch *model.GbChannel) error {
	return repository.NewGbChannelRepo().UpsertByGbChannelID(ctx, ch)
}

// UpdateChannel 局部更新通道字段。
func (s *GbStore) UpdateChannel(ctx context.Context, id int64, fields map[string]interface{}) error {
	return repository.NewGbChannelRepo().UpdateFields(ctx, id, fields)
}

// CountChannels 统计设备下的通道数。
func (s *GbStore) CountChannels(ctx context.Context, deviceID int64) (int64, error) {
	return repository.NewGbChannelRepo().CountByDevice(ctx, deviceID)
}

// CreateSipLog 写一条信令日志。
func (s *GbStore) CreateSipLog(ctx context.Context, entry *model.GbSipLog) error {
	if bootstrap.DB() == nil {
		return ErrNoDatabase
	}
	return repository.NewGbSipLogRepo().Create(ctx, entry)
}

// CreateAlarm 写一条国标报警。
func (s *GbStore) CreateAlarm(ctx context.Context, alarm *model.GbAlarm) error {
	return repository.NewGbAlarmRepo().Create(ctx, alarm)
}

// CreateMobilePosition 写一条移动位置。
func (s *GbStore) CreateMobilePosition(ctx context.Context, pos *model.GbMobilePosition) error {
	return repository.NewGbMobilePositionRepo().Create(ctx, pos)
}

// MarkExpiredOffline 把 sip_expires 已过期但仍标记在线的设备置为 offline。
//
// 只改状态、不删除设备（设计 §4.2）：设备再次 REGISTER 即可恢复，
// 通道数据也一并保留，避免现场掉线一次就要重新同步目录。
func (s *GbStore) MarkExpiredOffline(ctx context.Context, now time.Time) (int64, error) {
	if bootstrap.DB() == nil {
		return 0, ErrNoDatabase
	}
	res := bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("gb_device_id IS NOT NULL AND gb_device_id <> ''").
		Where("status = ?", model.DeviceStatusOnline).
		Where("sip_expires IS NOT NULL AND sip_expires < ?", now).
		Updates(map[string]interface{}{
			"status":     model.DeviceStatusOffline,
			"offline_dt": now,
			"updated_dt": now,
			"updated_by": "sipsvc",
		})
	return res.RowsAffected, res.Error
}

// MarkHeartbeatTimeoutOffline 把「心跳超时」的在线设备置为 offline。
//
// 规则：now - last_beat_dt > timeout（严格大于，见 Manager.HeartbeatOnce）。
// 只更新 status='online' 的行，因此重复扫描不会反复写同一台设备；
// GORM 的软删除条件（deleted_at IS NULL）由模型自动附加，无需手写。
//
// last_beat_dt IS NULL 的在线设备不在这里处理：REGISTER 成功会写 last_beat_dt，
// 正常流程不会出现 NULL；万一出现，交给 sip_expires 过期扫描兜底。
func (s *GbStore) MarkHeartbeatTimeoutOffline(ctx context.Context, now time.Time, timeout time.Duration) (int64, error) {
	if timeout <= 0 {
		return 0, nil
	}
	if bootstrap.DB() == nil {
		return 0, ErrNoDatabase
	}
	deadline := now.Add(-timeout)
	res := bootstrap.DB().WithContext(ctx).
		Model(&model.Device{}).
		Where("gb_device_id IS NOT NULL AND gb_device_id <> ''").
		Where("status = ?", model.DeviceStatusOnline).
		Where("last_beat_dt IS NOT NULL AND last_beat_dt < ?", deadline).
		Updates(map[string]interface{}{
			"status":     model.DeviceStatusOffline,
			"offline_dt": now,
			"updated_dt": now,
			"updated_by": "sipsvc",
		})
	return res.RowsAffected, res.Error
}

// normalizeRegisterWay 把目录项的 RegisterWay 数字转成 ac_gb_channel 的字符串列。
func normalizeRegisterWay(v int) string {
	if v <= 0 {
		return ""
	}
	return strconv.Itoa(v)
}
