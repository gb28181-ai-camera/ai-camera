package auth

import (
	"context"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// DeviceStore 是 DeviceLookup 的生产实现，直接复用 repository 层的白名单查询。
//
// 单独包一层而不是让 auth 直接 import repository：单测可以用假实现替换，
// 不需要数据库，也不会意外依赖 bootstrap 的全局 DB 初始化。
type DeviceStore struct{}

// NewDeviceStore 构造白名单查询实现。
func NewDeviceStore() *DeviceStore { return &DeviceStore{} }

// FindByGbDeviceID 按国标 20 位编码查询设备。
func (s *DeviceStore) FindByGbDeviceID(ctx context.Context, gbDeviceID string) (*model.Device, error) {
	return repository.NewGbDeviceRepo().FindByGbDeviceID(ctx, gbDeviceID)
}
