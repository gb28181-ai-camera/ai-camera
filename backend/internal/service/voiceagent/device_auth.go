package voiceagent

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/repository"
)

// 设备鉴权错误。
var (
	ErrDeviceUnauthorized = errors.New("设备凭据无效")
	ErrDeviceNoAIChatWS   = errors.New("产品未绑定 AIChat 服务组或缺少 WebSocket 服务")
	ErrDeviceMismatch     = errors.New("设备不属于该产品")
)

// DeviceIdentity 设备端 WS 身份。
type DeviceIdentity struct {
	ProductID  int64
	DeviceID   int64
	DeviceCode string
}

// AuthenticateDevice 校验产品×服务凭据，并要求产品绑定 aichat 组且含 WebSocket。
//
// 凭据来源：ac_iot_product_service.username/password（组级凭据在产品×服务维度）。
// 设备身份不属于凭据本身：设备编号由调用方从 query/header 提供，校验归属产品。
func AuthenticateDevice(ctx context.Context, username, password, deviceCode string) (*DeviceIdentity, error) {
	if strings.TrimSpace(username) == "" || password == "" {
		return nil, ErrDeviceUnauthorized
	}
	row, err := repository.FindProductServiceByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return nil, ErrDeviceUnauthorized
	}
	// 明文密码比较使用常量时间，避免时序侧信道。
	if subtle.ConstantTimeCompare([]byte(row.Password), []byte(password)) != 1 {
		return nil, ErrDeviceUnauthorized
	}
	ok, err := repository.ProductHasAIChatWebSocket(ctx, row.ProductID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrDeviceNoAIChatWS
	}

	ident := &DeviceIdentity{ProductID: row.ProductID}
	code := strings.TrimSpace(deviceCode)
	if code == "" {
		return ident, nil
	}
	dev, err := repository.FindDeviceByCode(ctx, code)
	if err != nil {
		return nil, ErrDeviceMismatch
	}
	if dev.ProductID == nil || *dev.ProductID != row.ProductID {
		return nil, ErrDeviceMismatch
	}
	if dev.Active != nil && !*dev.Active {
		return nil, ErrDeviceUnauthorized
	}
	ident.DeviceID = dev.ID
	ident.DeviceCode = dev.DeviceCode
	return ident, nil
}

// ResolveDeviceID 供 ticket 签发使用：把设备编号解析为设备 ID。
//
// 首期只校验设备存在；是否属于当前用户由 App 设备列表的既有权限约束，
// 语音层不再重复建模设备所有权。
func ResolveDeviceID(ctx context.Context, code string) (int64, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, nil
	}
	dev, err := repository.FindDeviceByCode(ctx, code)
	if err != nil || dev == nil {
		return 0, errors.New("设备不存在")
	}
	return dev.ID, nil
}
