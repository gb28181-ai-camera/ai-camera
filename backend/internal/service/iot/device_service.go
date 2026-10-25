package iot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 设备与设备账号（Web 端）
// ════════════════════════════════════════════════════════════════

// DeviceService IOT 设备业务逻辑。
type DeviceService struct {
	devices  *repository.IotDeviceRepo
	products *repository.IotProductRepo
	groups   *repository.IotServiceGroupRepo
}

// NewDeviceService 构建设备服务。
func NewDeviceService() *DeviceService {
	return &DeviceService{
		devices:  repository.NewIotDeviceRepo(),
		products: repository.NewIotProductRepo(),
		groups:   repository.NewIotServiceGroupRepo(),
	}
}

// Page 设备分页列表（接入密钥脱敏）。
func (s *DeviceService) Page(ctx context.Context, req *request.PageIotDeviceReq) (*response.PageIotDeviceResp, error) {
	req.Normalize()
	items, total, err := s.devices.Page(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make([]model.Device, 0, len(items))
	for i := range items {
		out = append(out, maskDevice(items[i]))
	}
	return &response.PageIotDeviceResp{
		Total: total, Page: req.Page, PageSize: req.PageSize, Items: out,
	}, nil
}

// Get 设备详情：设备 + 账号列表 + 已授权服务组（含服务项）。
func (s *DeviceService) Get(ctx context.Context, id int64) (*response.IotDeviceDetail, error) {
	d, err := s.devices.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	accounts, err := s.devices.ListAccounts(ctx, id)
	if err != nil {
		return nil, err
	}
	if accounts == nil {
		accounts = []model.IotDeviceAccount{}
	}

	groups := []response.IotServiceGroupDetail{}
	// 用 Preload 出来的 Product 判空：产品被删除后绑定行可能仍在，
	// 此时不应再把孤儿服务组展示成「已授权」。
	if d.ProductID != nil && d.Product != nil {
		list, err := s.products.ListServiceGroups(ctx, *d.ProductID)
		if err != nil {
			return nil, err
		}
		for i := range list {
			services, err := s.groups.ListServices(ctx, list[i].ID)
			if err != nil {
				return nil, err
			}
			if services == nil {
				services = []model.IotService{}
			}
			groups = append(groups, response.IotServiceGroupDetail{
				IotServiceGroup: maskGroup(list[i]),
				Services:        services,
			})
		}
	}

	productCode := ""
	if d.Product != nil {
		productCode = strings.TrimSpace(d.Product.Code)
	}
	// 没有产品时不生成二维码内容，避免前端渲染 `qrDevice//<code>`。
	qrContent := ""
	if productCode != "" {
		qrContent = deviceQRContent(productCode, d.DeviceCode)
	}

	return &response.IotDeviceDetail{
		Device:        maskDevice(*d),
		Accounts:      accounts,
		ServiceGroups: groups,
		// 详情直接带二维码内容：前端列表/详情都可以就地渲染，
		// 不必为了一个纯本地可推导的字符串多打一次接口。
		QRContent: qrContent,
	}, nil
}

// CreateAccount 新增设备账号（默认分享账号）。
func (s *DeviceService) CreateAccount(ctx context.Context, deviceID int64, req *request.CreateIotDeviceAccountReq, ip string) (*response.IotDeviceAccountCreated, error) {
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = model.IotAccountRoleShared
	}
	if err := validateAccountRole(role); err != nil {
		return nil, err
	}

	// 主账号统一走「激活」流程生成，保证用户名/密码/事件口径一致。
	if role == model.IotAccountRoleOwner {
		if _, err := s.devices.GetOwner(ctx, deviceID); err == nil {
			return nil, ErrOwnerExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		account, password, err := createOwnerAccount(ctx, s.devices, d, req.Nickname, req.Phone, model.IotEventAccountCreate, ip)
		if err != nil {
			return nil, err
		}
		return &response.IotDeviceAccountCreated{Account: *account, Password: password}, nil
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username, err = generateAccountUsername(ctx, s.devices, "user_")
		if err != nil {
			return nil, err
		}
	} else {
		exists, err := s.devices.ExistsAccountUsername(ctx, username, 0)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("登录名 %s 已存在", username)
		}
	}

	password := strings.TrimSpace(req.Password)
	generated := false
	if password == "" {
		password, err = randomFrom(credentialAlphabet, 16)
		if err != nil {
			return nil, errors.New("生成账号密码失败")
		}
		generated = true
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("密码加密失败")
	}

	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = model.IotAccountStatusActive
	}
	if err := validateAccountStatus(status); err != nil {
		return nil, err
	}

	// 分享令牌只由 POST /devices/:id/share-token 生成：
	// 管理端新增的普通分享账号不带令牌，避免「创建账号即产生一个可被认领的入口」。
	account := &model.IotDeviceAccount{
		DeviceID:     deviceID,
		Username:     username,
		PasswordHash: string(hash),
		Role:         model.IotAccountRoleShared,
		Nickname:     strings.TrimSpace(req.Nickname),
		Phone:        strings.TrimSpace(req.Phone),
		Status:       status,
	}
	if err := s.devices.CreateAccount(ctx, account); err != nil {
		return nil, err
	}
	_ = s.devices.CreateEvent(ctx, &model.IotDeviceEvent{
		DeviceID: deviceID,
		Event:    model.IotEventAccountCreate,
		Detail:   fmt.Sprintf(`{"username":%q,"role":%q}`, username, model.IotAccountRoleShared),
		IP:       ip,
	})

	out := &response.IotDeviceAccountCreated{Account: *account}
	if generated {
		out.Password = password
	}
	return out, nil
}

// UpdateAccount 更新设备账号。密码留空表示不修改。
func (s *DeviceService) UpdateAccount(ctx context.Context, deviceID, accountID int64, req *request.UpdateIotDeviceAccountReq) (*model.IotDeviceAccount, error) {
	m, err := s.devices.GetAccount(ctx, deviceID, accountID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	if v := strings.TrimSpace(req.Username); v != "" && v != m.Username {
		exists, err := s.devices.ExistsAccountUsername(ctx, v, accountID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("登录名 %s 已存在", v)
		}
		m.Username = v
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, errors.New("密码加密失败")
		}
		m.PasswordHash = string(hash)
	}
	if v := strings.TrimSpace(req.Nickname); v != "" {
		m.Nickname = v
	}
	if v := strings.TrimSpace(req.Phone); v != "" {
		m.Phone = v
	}
	if v := strings.TrimSpace(req.Status); v != "" {
		if err := validateAccountStatus(v); err != nil {
			return nil, err
		}
		m.Status = v
	}
	if err := s.devices.UpdateAccount(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// DeleteAccount 删除设备账号（主账号禁止删除）。
func (s *DeviceService) DeleteAccount(ctx context.Context, deviceID, accountID int64) error {
	m, err := s.devices.GetAccount(ctx, deviceID, accountID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAccountNotFound
		}
		return err
	}
	if m.Role == model.IotAccountRoleOwner {
		return ErrOwnerDeleteForbidden
	}
	return s.devices.DeleteAccount(ctx, accountID)
}

// Activate 管理员手动激活设备（幂等）。
//
// 幂等语义：主账号已存在时直接返回既有账号，不重新生成密码。
// 这与设备端 /iot/activate 的「重复激活返回 409」不同 ——
// 管理端页面可能因超时重复提交，重复请求不应被视为错误。
func (s *DeviceService) Activate(ctx context.Context, deviceID int64, ip string) (*response.IotDeviceActivated, error) {
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	owner, err := s.devices.GetOwner(ctx, deviceID)
	if err == nil {
		activatedAt := owner.ActivatedAt
		if activatedAt == nil {
			activatedAt = d.ActivatedAt
		}
		return &response.IotDeviceActivated{
			Username:    owner.Username,
			DeviceCode:  d.DeviceCode,
			ActivatedAt: activatedAt,
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	account, password, err := createOwnerAccount(ctx, s.devices, d, "", "", model.IotEventActivate, ip)
	if err != nil {
		return nil, err
	}
	return &response.IotDeviceActivated{
		Username:    account.Username,
		Password:    password,
		DeviceCode:  d.DeviceCode,
		ActivatedAt: account.ActivatedAt,
	}, nil
}

// Unbind 后台解绑设备：删除全部账号（含主账号）并清除激活状态。
//
// 设计 §9 要求「绑定后可在后台解绑」：二维码/蓝牙绑定一旦发出，
// 管理员必须能在后台收回，否则设备会永久挂在第一个认领用户名下。
// 解绑是幂等的：重复调用返回 accounts=0，仍记一条解绑事件。
// 设备接入配置（产品、接入密钥、国标编码）保持不变，设备无需重新配网，
// 只需重新激活主账号。
func (s *DeviceService) Unbind(ctx context.Context, deviceID int64, ip string) (*response.IotDeviceUnbindResult, error) {
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	removed, err := s.devices.UnbindAll(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	_ = s.devices.CreateEvent(ctx, &model.IotDeviceEvent{
		DeviceID: deviceID,
		Event:    model.IotEventUnbind,
		Detail:   fmt.Sprintf(`{"device_code":%q,"accounts":%d}`, d.DeviceCode, removed),
		IP:       ip,
	})
	return &response.IotDeviceUnbindResult{DeviceID: deviceID, Accounts: removed}, nil
}

// ResetSecret 重置设备接入密钥，返回一次性明文。
func (s *DeviceService) ResetSecret(ctx context.Context, deviceID int64, ip string) (*response.IotDeviceSecret, error) {
	d, err := s.devices.GetByID(ctx, deviceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	secret, err := GenerateDeviceSecret()
	if err != nil {
		return nil, errors.New("生成设备接入密钥失败")
	}
	if err := s.devices.SetSecret(ctx, deviceID, secret); err != nil {
		return nil, err
	}
	// 事件详情只记设备编号，绝不落盘明文密钥。
	_ = s.devices.CreateEvent(ctx, &model.IotDeviceEvent{
		DeviceID: deviceID,
		Event:    model.IotEventSecretReset,
		Detail:   fmt.Sprintf(`{"device_code":%q,"reset_at":%q}`, d.DeviceCode, time.Now().Format(time.RFC3339)),
		IP:       ip,
	})
	return &response.IotDeviceSecret{DeviceCode: d.DeviceCode, DeviceSecret: secret}, nil
}

// BindBySIP 供信令层在 REGISTER 白名单未命中时做一次显式绑定。
//
// 背景：SIP 设备首次注册时平台还没有它的国标编码，只有设备出厂编码
// （device_code）与产品编码。信令层按这两个值找到设备后调用本方法落库，
// 之后该设备就能进入正常的国标注册流程。
//
// 规则：
//   - **未同时携带产品编码与设备编码 → 不做任何绑定**，返回 (nil, nil)：
//     这类 GB28181 设备按独立设备接入（只按 gb_device_id 走国标白名单），
//     不要求也不自动绑定产品；OTA 对无产品设备返回空服务组。
//   - 设备不存在 → ErrDeviceNotFound；
//   - 产品为空/编码不一致 → ErrDeviceProductMismatch（绝不跨产品挂设备）；
//   - gb_device_id 为空 → 写入并置 activated；
//   - 已有且相同 → 幂等返回（设备重启/重复 REGISTER 很常见）；
//   - 已有且不同 → ErrGbDeviceIDConflict，**不静默改绑**：
//     改绑意味着另一个国标编码的历史数据会被错认，必须人工介入。
func (s *DeviceService) BindBySIP(ctx context.Context, productCode, deviceCode, gbDeviceID string) (*model.Device, error) {
	productCode = strings.TrimSpace(productCode)
	deviceCode = strings.TrimSpace(deviceCode)
	gbDeviceID = strings.TrimSpace(gbDeviceID)
	if productCode == "" || deviceCode == "" {
		// 没有产品编码/设备编码 = 独立国标设备，不参与 IOT 产品绑定。
		return nil, nil
	}
	if gbDeviceID == "" {
		return nil, ErrGbDeviceIDRequired
	}

	d, err := s.devices.GetByCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	if d.ProductID == nil {
		return nil, ErrDeviceProductMismatch
	}
	p, err := s.products.GetByID(ctx, *d.ProductID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceProductMismatch
		}
		return nil, err
	}
	if p.Code != productCode {
		return nil, ErrDeviceProductMismatch
	}

	switch {
	case d.GbDeviceID == "":
		if err := s.devices.SetGbDeviceBinding(ctx, d.ID, gbDeviceID, model.GbRegisterStatusActivated); err != nil {
			return nil, err
		}
	case d.GbDeviceID == gbDeviceID:
		// 幂等：重复 REGISTER 不重复写库。
	default:
		return nil, fmt.Errorf("%w（当前 %s，请求 %s）", ErrGbDeviceIDConflict, d.GbDeviceID, gbDeviceID)
	}

	return s.devices.GetByID(ctx, d.ID)
}

// SetProduct 设置设备所属产品。productID 为 nil 表示解绑。
func (s *DeviceService) SetProduct(ctx context.Context, deviceID int64, productID *int64) error {
	if _, err := s.devices.GetByID(ctx, deviceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}
	if productID != nil {
		if _, err := s.products.GetByID(ctx, *productID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrProductNotFound
			}
			return err
		}
	}
	return s.devices.SetProduct(ctx, deviceID, productID)
}
