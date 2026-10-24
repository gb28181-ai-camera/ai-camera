package iot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	turnsvc "github.com/boticz/ai-camera-backend/internal/service/turn"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - App 端设备认领 / 绑定 / 查询 / ICE
//
// 与 Web 端最大的区别：App 的唯一身份来源是 JWT 里的 user_id。
// 所有读写都以 platform_user_id = 当前用户 为边界，绝不接受客户端传入的
// user_id —— 否则任何登录用户都能把别人的设备加到自己名下。
// ════════════════════════════════════════════════════════════════

// ErrDeviceNotBound 当前用户在目标设备上没有账号。
var ErrDeviceNotBound = errors.New("设备未绑定到当前用户")

// ErrDeviceNotOwner 当前用户在该设备上不是主账号。
//
// 生成分享码等于把设备使用权转交给第三方，只有 owner 能决定；shared 账号
// 本身已经是「被分享者」，若允许其继续分享，权限链会无限扩散且无法追溯。
var ErrDeviceNotOwner = errors.New("仅设备主账号可以生成分享码")

// AppDeviceService App 端 IOT 业务逻辑。
type AppDeviceService struct {
	devices *repository.IotDeviceRepo
	creds   *CredentialsService
	share   *ShareService
}

// NewAppDeviceService 构造 App 端 IOT 服务。
func NewAppDeviceService() *AppDeviceService {
	return &AppDeviceService{
		devices: repository.NewIotDeviceRepo(),
		creds:   NewCredentialsService(),
		share:   NewShareService(),
	}
}

// Claim 扫码认领分享设备：把一次性令牌对应的共享账号绑定到当前用户。
func (s *AppDeviceService) Claim(ctx context.Context, userID int64, req *request.ClaimIotDeviceReq) (*response.AppIotDeviceBinding, error) {
	token := normalizeShareToken(req.Token)
	if token == "" {
		return nil, ErrShareTokenNotFound
	}
	account, err := s.devices.ClaimByShareToken(ctx, token, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShareTokenNotFound
		}
		return nil, err
	}
	d, err := s.devices.GetByID(ctx, account.DeviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	return &response.AppIotDeviceBinding{
		Device:  appDevice(*d, *account),
		Account: appAccount(*account),
		Role:    account.Role,
		Created: false,
	}, nil
}

// Bind 蓝牙配网后绑定设备。
//
// 规则：
//   - 同一用户重复绑定 → 幂等返回原账号（网络重试不应产生第二条账号）；
//   - 设备没有 owner → 本次绑定创建 owner（主账号）；
//   - 已有 owner（可能是管理员 Web 激活或别的用户先绑定）→ 创建 shared 账号。
//
// 注意：owner 是否已绑定平台用户不作为判定条件 —— owner 是设备的「主账号」
// 这一事实本身决定新用户只能拿到 shared，否则会出现两个 owner 并存。
func (s *AppDeviceService) Bind(ctx context.Context, userID int64, req *request.BindIotDeviceReq, ip string) (*response.AppIotDeviceBinding, error) {
	deviceCode := strings.TrimSpace(req.DeviceCode)
	secret := strings.TrimSpace(req.DeviceSecret)
	if deviceCode == "" || secret == "" {
		return nil, ErrDeviceCredentialMissing
	}

	d, err := s.devices.GetByCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 与设备端鉴权一致：不区分「设备不存在」与「密钥错误」，
			// 避免用错误文案枚举设备编号。
			return nil, ErrDeviceSecretInvalid
		}
		return nil, err
	}
	if d.DeviceSecret == "" {
		return nil, ErrDeviceSecretNotSet
	}
	if !secureEqual(d.DeviceSecret, secret) {
		return nil, ErrDeviceSecretInvalid
	}

	return s.bindToUser(ctx, d, userID, req.Nickname, ip, "app-bind")
}

// BindByQR 扫码绑定设备。
//
// 二维码只含 product_code + device_code，不含密钥 —— 扫码即视为持有设备。
// 产品编码必须与设备当前 product_id 一致，否则拒绝：仅凭 device_code
// 就绑定会让「猜编号」变成越权入口。
func (s *AppDeviceService) BindByQR(ctx context.Context, userID int64, req *request.BindIotDeviceQRReq, ip string) (*response.AppIotDeviceBinding, error) {
	productCode := strings.TrimSpace(req.ProductCode)
	deviceCode := strings.TrimSpace(req.DeviceCode)
	if productCode == "" || deviceCode == "" {
		return nil, ErrBindQRInvalid
	}

	d, err := s.devices.GetByCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBindQRInvalid
		}
		return nil, err
	}
	matched, err := s.productCodeMatches(ctx, d, productCode)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, ErrBindQRInvalid
	}
	return s.bindToUser(ctx, d, userID, "", ip, "app-bind-qr")
}

// productCodeMatches 校验设备当前归属产品与二维码中的产品编码一致。
func (s *AppDeviceService) productCodeMatches(ctx context.Context, d *model.Device, productCode string) (bool, error) {
	if d.ProductID == nil {
		return false, nil
	}
	p, err := s.creds.products.GetByID(ctx, *d.ProductID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return p.Code == productCode, nil
}

// bindToUser 把设备绑定到用户（Bind 与 BindByQR 共用）。
//
// 幂等优先：同一用户重复绑定返回原账号，不新增行，也不会把 owner 降级为 shared。
func (s *AppDeviceService) bindToUser(ctx context.Context, d *model.Device, userID int64, nickname, ip, source string) (*response.AppIotDeviceBinding, error) {
	if existing, err := s.devices.GetAccountByPlatformUser(ctx, d.ID, userID); err == nil {
		full, err := s.devices.GetByID(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		return &response.AppIotDeviceBinding{
			Device:  appDevice(*full, *existing),
			Account: appAccount(*existing),
			Role:    existing.Role,
			Created: false,
		}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hasOwner := true
	if _, err := s.devices.GetOwner(ctx, d.ID); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		hasOwner = false
	}

	role := model.IotAccountRoleShared
	prefix := "user_"
	nickname = strings.TrimSpace(nickname)
	if !hasOwner {
		role = model.IotAccountRoleOwner
		prefix = "owner_"
		if nickname == "" {
			nickname = "主账号"
		}
	} else if nickname == "" {
		nickname = "分享账号"
	}

	username, err := generateAccountUsername(ctx, s.devices, prefix)
	if err != nil {
		return nil, err
	}
	password, err := randomFrom(credentialAlphabet, 16)
	if err != nil {
		return nil, errors.New("生成账号密码失败")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}

	now := time.Now()
	account := &model.IotDeviceAccount{
		DeviceID:       d.ID,
		Username:       username,
		PasswordHash:   string(hash),
		Role:           role,
		Nickname:       nickname,
		Status:         model.IotAccountStatusActive,
		PlatformUserID: &userID,
	}
	if role == model.IotAccountRoleOwner {
		account.ActivatedAt = &now
	}
	if err := s.devices.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	if role == model.IotAccountRoleOwner {
		if err := s.devices.SetActivatedAt(ctx, d.ID, now); err != nil {
			return nil, err
		}
	}
	_ = s.devices.CreateEvent(ctx, &model.IotDeviceEvent{
		DeviceID: d.ID,
		Event:    model.IotEventAccountCreate,
		Detail:   fmt.Sprintf(`{"username":%q,"role":%q,"source":%q,"platform_user_id":%d}`, username, role, source, userID),
		IP:       ip,
	})

	full, err := s.devices.GetByID(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	return &response.AppIotDeviceBinding{
		Device:  appDevice(*full, *account),
		Account: appAccount(*account),
		Role:    role,
		Created: true,
	}, nil
}

// CreateShareToken App 端生成设备分享码（仅 owner）。
//
// 权限边界与 Web 端一致（Web 由 Casbin 的 iot:device:account:create 控制），
// App 端没有后台角色体系，只能以设备内的账号角色判定：
//   - 当前用户在该设备上没有账号 → ErrDeviceNotBound（404，避免暴露设备是否存在）；
//   - 有账号但角色是 shared → ErrDeviceNotOwner（403）；
//   - owner → 复用 ShareService，返回结构与 Web 完全一致。
//
// 复用而不是复制一份实现：分享令牌的一次性、唯一性与二维码格式
// （https://boticz.cn/deviceShareToken/<token>）必须两端保持同源。
func (s *AppDeviceService) CreateShareToken(ctx context.Context, userID, deviceID int64, ip string) (*response.IotDeviceShareToken, error) {
	account, err := s.devices.GetAccountByPlatformUser(ctx, deviceID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotBound
		}
		return nil, err
	}
	if account.Role != model.IotAccountRoleOwner {
		return nil, ErrDeviceNotOwner
	}
	return s.share.CreateShareToken(ctx, deviceID, ip)
}

// List 当前用户已绑定的设备列表。
func (s *AppDeviceService) List(ctx context.Context, userID int64) ([]response.AppIotDevice, error) {
	devices, err := s.devices.ListDevicesByPlatformUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.devices.ListAccountsByPlatformUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	byDevice := make(map[int64]model.IotDeviceAccount, len(accounts))
	for _, a := range accounts {
		if _, ok := byDevice[a.DeviceID]; !ok {
			byDevice[a.DeviceID] = a
		}
	}

	out := make([]response.AppIotDevice, 0, len(devices))
	for i := range devices {
		acc := byDevice[devices[i].ID]
		out = append(out, appDevice(devices[i], acc))
	}
	return out, nil
}

// Get 设备详情（含产品、账号角色与服务组摘要）。
func (s *AppDeviceService) Get(ctx context.Context, userID, deviceID int64) (*response.AppIotDeviceDetail, error) {
	account, err := s.devices.GetAccountByPlatformUser(ctx, deviceID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotBound
		}
		return nil, err
	}
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	groups := make([]response.IotServiceGroupBrief, 0)
	if d.ProductID != nil && d.Product != nil {
		list, err := s.creds.products.ListServiceGroups(ctx, *d.ProductID)
		if err != nil {
			return nil, err
		}
		for i := range list {
			groups = append(groups, response.IotServiceGroupBrief{
				Code:   list[i].Code,
				Name:   list[i].Name,
				Type:   list[i].Type,
				Active: list[i].Active == nil || *list[i].Active,
			})
		}
	}

	return &response.AppIotDeviceDetail{
		Device:        appDevice(*d, *account),
		Product:       d.Product,
		Account:       appAccount(*account),
		ServiceGroups: groups,
	}, nil
}

// Services 返回设备产品的服务组（含明文凭据、服务项与 MQTT 连接信息）。
//
// 为什么 App 可以拿明文：调用方已通过 JWT 登录且被校验为该设备的绑定用户，
// 与 Web 的 iot:*:list 浏览场景不同，这里的目的是让 App 真正连上服务。
// 未绑定用户会在上面的 GetAccountByPlatformUser 处被拒绝。
func (s *AppDeviceService) Services(ctx context.Context, userID, deviceID int64) (*response.IotCredentials, error) {
	if _, err := s.devices.GetAccountByPlatformUser(ctx, deviceID, userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotBound
		}
		return nil, err
	}
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	if d.ProductID == nil || d.Product == nil {
		return &response.IotCredentials{
			Product: response.IotCredentialsProduct{},
			Groups:  []response.IotCredentialsGroup{},
			MQTT:    nil,
		}, nil
	}
	return s.creds.ProductCredentialsForApp(ctx, *d.ProductID)
}

// IceServers 返回当前用户的 TURN/STUN 凭据。
//
// subject 使用 user_id 而不是用户名：用户可能改名，ID 才是稳定标识，
// 也便于在 coturn 日志里按用户排查流量。未配置 TURN 时返回空列表，
// 客户端据此降级为仅 STUN/直连，而不是把整个请求判为失败。
func (s *AppDeviceService) IceServers(ctx context.Context, userID int64) ([]response.IceServer, error) {
	out := []response.IceServer{}
	svc := turnsvc.New()
	if !svc.Configured() {
		return out, nil
	}
	cred, err := svc.Issue(strconv.FormatInt(userID, 10), 0)
	if err != nil {
		if errors.Is(err, turnsvc.ErrSecretNotConfigured) {
			return out, nil
		}
		return nil, err
	}
	return []response.IceServer{{
		URLs:       cred.URLs,
		Username:   cred.Username,
		Credential: cred.Password,
		Realm:      cred.Realm,
		TTL:        cred.TTL,
		ExpiresAt:  cred.ExpiresAt,
	}}, nil
}

// appDevice 把设备模型转换为 App 列表/详情项。
func appDevice(d model.Device, acc model.IotDeviceAccount) response.AppIotDevice {
	out := response.AppIotDevice{
		ID:         d.ID,
		Name:       d.Name,
		DeviceCode: d.DeviceCode,
		Location:   d.Location,
		Status:     d.Status,
		Role:       acc.Role,
		Nickname:   acc.Nickname,
	}
	if d.Product != nil {
		out.Product = &response.IotProductBrief{
			ID:         d.Product.ID,
			Code:       d.Product.Code,
			Name:       d.Product.Name,
			AllowShare: d.Product.AllowShare,
		}
	}
	return out
}

// appAccount 把账号模型裁剪为 App 可安全展示的摘要。
//
// 必须裁剪：PasswordHash 虽然 json:"-"，但 ShareToken 等字段不应出现在
// 任何 App 响应里；显式结构体可以防止以后给模型加字段时意外泄露。
func appAccount(acc model.IotDeviceAccount) response.AppIotDeviceAccount {
	return response.AppIotDeviceAccount{
		ID:             acc.ID,
		DeviceID:       acc.DeviceID,
		Username:       acc.Username,
		Role:           acc.Role,
		Nickname:       acc.Nickname,
		Phone:          acc.Phone,
		Status:         acc.Status,
		PlatformUserID: acc.PlatformUserID,
		ActivatedAt:    acc.ActivatedAt,
	}
}
