package iot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 设备分享令牌（Web 生成 / App 认领）
//
// 分享令牌只有一个目的：把「某台设备的一个共享账号」通过二维码转交给
// 另一个平台用户。因此它必须满足两点：
//   1. 一次性：二维码会被截图/转发，若可重复使用，任何看到图片的人都能
//      永久把设备加入自己的账号；认领成功即清空 token。
//   2. 可撤销：管理员在二维码泄漏或发错人时，能立即让未认领的令牌失效。
// ════════════════════════════════════════════════════════════════

// ErrShareTokenNotFound 分享令牌不存在、已被认领或已撤销。
//
// 对外统一为同一句中文提示，避免攻击者通过错误差异枚举有效令牌。
var ErrShareTokenNotFound = errors.New("分享令牌无效或已被认领")

// ErrProductShareDisabled 产品未开启分享，不允许生成设备分享码。
//
// 产品上的 allow_share 是运营开关（安防相机可关闭分享，适老化相机必须开启）；
// 只在生成分享码时校验，已存在的令牌与已认领账号不受影响：
// 关闭开关是「不再新增分享」，不是「追溯作废」。
var ErrProductShareDisabled = errors.New("该产品未开启分享")

// ShareService 设备分享令牌业务逻辑。
type ShareService struct {
	devices *repository.IotDeviceRepo
}

// NewShareService 构造分享服务。
func NewShareService() *ShareService {
	return &ShareService{devices: repository.NewIotDeviceRepo()}
}

// CreateShareToken 为设备生成一次性分享令牌，并创建一个尚未绑定用户的共享账号。
//
// 为什么在这里就建账号而不是等认领时再建：令牌与账号必须一一对应，
// 认领只是「写 platform_user_id + 清 token」的一次更新，事务边界最小，
// 也不会出现「认领成功但账号创建失败」的半成品状态。
func (s *ShareService) CreateShareToken(ctx context.Context, deviceID int64, ip string) (*response.IotDeviceShareToken, error) {
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	// 产品级分享开关：设备已归属产品且产品关闭分享时拒绝。
	// 设备未绑定产品（历史设备）不阻断：它们没有产品配置可依据，
	// 若一律拒绝会让存量设备永远无法分享。
	if d.Product != nil && !d.Product.AllowShare {
		return nil, ErrProductShareDisabled
	}

	token, err := s.uniqueShareToken(ctx)
	if err != nil {
		return nil, err
	}
	username, err := generateAccountUsername(ctx, s.devices, "share_")
	if err != nil {
		return nil, err
	}
	password, err := randomFrom(credentialAlphabet, 16)
	if err != nil {
		return nil, errors.New("生成分享账号密码失败")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}

	account := &model.IotDeviceAccount{
		DeviceID:     d.ID,
		Username:     username,
		PasswordHash: string(hash),
		Role:         model.IotAccountRoleShared,
		Nickname:     "分享令牌",
		Status:       model.IotAccountStatusActive,
		ShareToken:   &token,
		// platform_user_id 保持 NULL：此刻令牌尚未被任何用户认领。
	}
	if err := s.devices.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	_ = s.devices.CreateEvent(ctx, &model.IotDeviceEvent{
		DeviceID: d.ID,
		Event:    model.IotEventAccountCreate,
		Detail:   fmt.Sprintf(`{"username":%q,"role":%q,"source":"share-token"}`, username, model.IotAccountRoleShared),
		IP:       ip,
	})

	return &response.IotDeviceShareToken{
		DeviceID:  d.ID,
		AccountID: account.ID,
		Token:     token,
		// 与参考 App（neme-ai-app）的分享码格式保持一致：
		//   boticz.cn/deviceShareToken/{token}
		// App 侧按 URL 前缀分流；JSON 格式仍作为兼容输入保留。
		QRContent: "https://boticz.cn/deviceShareToken/" + token,
	}, nil
}

// RevokeShareToken 撤销设备上所有尚未被认领的分享令牌。
func (s *ShareService) RevokeShareToken(ctx context.Context, deviceID int64) (int64, error) {
	if _, err := s.devices.GetByID(ctx, deviceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrDeviceNotFound
		}
		return 0, err
	}
	return s.devices.RevokeUnusedShareTokens(ctx, deviceID)
}

// DeviceQR 生成设备二维码内容（产品编码 + 设备编码）。
//
// 二维码不含任何密钥：它表达的是「持有此码即视为持有设备」，App 扫码后
// 通过 /app/iot/devices/bind-qr 完成绑定。产品编码参与校验，避免把设备
// 错绑到同名/相似编码的其它产品下。
func (s *ShareService) DeviceQR(ctx context.Context, deviceID int64) (*response.IotDeviceQR, error) {
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	productCode := ""
	if d.Product != nil {
		productCode = d.Product.Code
	}
	return &response.IotDeviceQR{
		DeviceID:    d.ID,
		ProductCode: productCode,
		DeviceCode:  d.DeviceCode,
		QRContent:   deviceQRContent(productCode, d.DeviceCode),
	}, nil
}

// deviceQRContent 序列化设备二维码内容。
//
// 用结构体而不是 map：map 的 JSON 键按字典序输出，字段顺序不稳定，
// 二维码内容一旦被客户端缓存/比对，顺序变化会造成无意义的差异。
// deviceQRContent 生成设备二维码内容。
//
// 格式与参考 App（neme-ai-app）一致：
//
//	https://boticz.cn/qrDevice/{product_code}/{device_code}
//
// App 侧按前缀识别；JSON 形式（{"type":"device",...}）仅作为兼容输入保留，
// 平台生成时不再使用。产品编码参与路径，避免把设备错绑到相似产品下。
func deviceQRContent(productCode, deviceCode string) string {
	return "https://boticz.cn/qrDevice/" + url.PathEscape(productCode) + "/" + url.PathEscape(deviceCode)
}

// uniqueShareToken 生成全局唯一的分享令牌，冲突时重试。
func (s *ShareService) uniqueShareToken(ctx context.Context) (string, error) {
	for i := 0; i < 10; i++ {
		suffix, err := randomFrom(usernameAlphabet, 24)
		if err != nil {
			return "", errors.New("生成分享令牌失败")
		}
		token := "share_" + suffix
		exists, err := s.devices.ExistsShareToken(ctx, token)
		if err != nil {
			return "", err
		}
		if !exists {
			return token, nil
		}
	}
	return "", errors.New("生成分享令牌失败，请重试")
}

// normalizeShareToken 去掉首尾空白；空串直接视为无效。
func normalizeShareToken(token string) string {
	return strings.TrimSpace(token)
}
