package repository

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// IotDevicePositionRepo MQTT 设备位置历史。
type IotDevicePositionRepo struct{}

// NewIotDevicePositionRepo 构造仓储。
func NewIotDevicePositionRepo() *IotDevicePositionRepo { return &IotDevicePositionRepo{} }

// Create 写入一条位置。
func (r *IotDevicePositionRepo) Create(ctx context.Context, m *model.IotDevicePosition) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Latest 返回设备最近一条位置（不存在返回 nil, nil）。
func (r *IotDevicePositionRepo) Latest(ctx context.Context, deviceID int64) (*model.IotDevicePosition, error) {
	var m model.IotDevicePosition
	err := bootstrap.DB().WithContext(ctx).
		Where("device_id = ?", deviceID).
		Order("reported_dt DESC, id DESC").
		First(&m).Error
	if err != nil {
		return nil, nil
	}
	return &m, nil
}
