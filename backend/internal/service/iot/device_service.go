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
			items, err := s.groups.ListItems(ctx, list[i].ID)
			if err != nil {
				return nil, err
			}
			if items == nil {
				items = []model.IotServiceItem{}
			}
			groups = append(groups, response.IotServiceGroupDetail{
				IotServiceGroup: maskGroup(list[i]),
				Items:           items,
			})
		}
	}

	return &response.IotDeviceDetail{
		Device:        maskDevice(*d),
		Accounts:      accounts,
		ServiceGroups: groups,
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

	shareToken := strings.TrimSpace(req.ShareToken)
	if shareToken == "" {
		suffix, err := randomFrom(usernameAlphabet, 24)
		if err != nil {
			return nil, errors.New("生成分享令牌失败")
		}
		shareToken = "share_" + suffix
	}

	account := &model.IotDeviceAccount{
		DeviceID:     deviceID,
		Username:     username,
		PasswordHash: string(hash),
		Role:         model.IotAccountRoleShared,
		Nickname:     strings.TrimSpace(req.Nickname),
		Phone:        strings.TrimSpace(req.Phone),
		Status:       status,
		ShareToken:   &shareToken,
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
	if v := strings.TrimSpace(req.ShareToken); v != "" {
		m.ShareToken = &v
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
