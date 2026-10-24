package iot

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	turnsvc "github.com/boticz/ai-camera-backend/internal/service/turn"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 设备端接口（OTA / 激活）
//
// 为什么设备端不走 JWT 与 Casbin：
//   设备是无人值守进程，没有用户会话，也没有角色/部门概念；
//   用 JWT 意味着要在设备上分发可长期使用的令牌，泄露后无法按设备吊销。
//   这里改用「设备编号 + 设备接入密钥」这对固有凭据：
//   - 密钥由平台在重置时生成，仅展示一次；
//   - 每台设备一份，泄露只影响该设备，可单独轮换；
//   - 请求头 X-Device-Code / X-Device-Secret 直接映射到鉴权逻辑，
//     不经过任何用户体系。
// ════════════════════════════════════════════════════════════════

// DeviceEndpointService 设备端业务逻辑。
type DeviceEndpointService struct {
	devices  *repository.IotDeviceRepo
	products *repository.IotProductRepo
	groups   *repository.IotServiceGroupRepo
}

// NewDeviceEndpointService 构造设备端服务。
func NewDeviceEndpointService() *DeviceEndpointService {
	return &DeviceEndpointService{
		devices:  repository.NewIotDeviceRepo(),
		products: repository.NewIotProductRepo(),
		groups:   repository.NewIotServiceGroupRepo(),
	}
}

// authenticate 校验设备身份。
//
// 错误分三类是为了给现场排障提供可执行的提示（未配置密钥 / 密钥错误 / 缺头），
// 但都统一映射为 HTTP 401。
func (s *DeviceEndpointService) authenticate(ctx context.Context, deviceCode, deviceSecret string) (*model.Device, error) {
	deviceCode = strings.TrimSpace(deviceCode)
	deviceSecret = strings.TrimSpace(deviceSecret)
	if deviceCode == "" || deviceSecret == "" {
		return nil, ErrDeviceCredentialMissing
	}

	d, err := s.devices.GetByCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceSecretInvalid
		}
		return nil, err
	}
	if d.DeviceSecret == "" {
		return nil, ErrDeviceSecretNotSet
	}
	if !secureEqual(d.DeviceSecret, deviceSecret) {
		return nil, ErrDeviceSecretInvalid
	}
	return d, nil
}

// OTA 返回产品信息、设备信息与服务授权配置。
func (s *DeviceEndpointService) OTA(ctx context.Context, deviceCode, deviceSecret, clientVersion, ip string) (*response.IotOTAResp, error) {
	d, err := s.authenticate(ctx, deviceCode, deviceSecret)
	if err != nil {
		return nil, err
	}

	var product *model.IotProduct
	if d.ProductID != nil {
		p, err := s.products.GetByID(ctx, *d.ProductID)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			// 产品被删除但设备仍指向它：按「未绑定产品」处理，
			// 返回空服务组而不是让设备 OTA 整体失败。
		} else {
			product = p
		}
	}

	groups := make([]response.IotOTAServiceGroup, 0)
	if product != nil {
		list, err := s.products.ListServiceGroups(ctx, product.ID)
		if err != nil {
			return nil, err
		}
		for i := range list {
			if list[i].Active != nil && !*list[i].Active {
				continue
			}
			items, err := s.groups.ListItems(ctx, list[i].ID)
			if err != nil {
				return nil, err
			}
			activeItems := make([]model.IotServiceItem, 0, len(items))
			for _, it := range items {
				if it.Active != nil && !*it.Active {
					continue
				}
				activeItems = append(activeItems, it)
			}
			groups = append(groups, response.IotOTAServiceGroup{
				Code:     list[i].Code,
				Name:     list[i].Name,
				Type:     list[i].Type,
				Username: list[i].Username,
				// 设备端必须拿到明文才能登录服务；Web 读接口仍然脱敏。
				Password: list[i].Password,
				Items:    activeItems,
			})
		}
	}

	// OTA 响应不回传 device_secret：设备已经持有它，回传只会扩大泄露面。
	outDevice := *d
	outDevice.DeviceSecret = ""

	detail, _ := json.Marshal(map[string]interface{}{
		"client_version": strings.TrimSpace(clientVersion),
		"product_id":     outDevice.ProductID,
	})
	_ = s.devices.CreateEvent(ctx, &model.IotDeviceEvent{
		DeviceID: d.ID,
		Event:    model.IotEventOTA,
		Detail:   string(detail),
		IP:       ip,
	})

	// TURN/STUN：设备做 WebRTC 时经常需要穿透，随 OTA 一并下发短期凭据。
	// 未配置 TURN 时省略该字段，不影响 OTA 其它内容。
	var turnCred *response.IotOTATurn
	tsvc := turnsvc.New()
	if tsvc.Configured() {
		if c, err := tsvc.Issue(d.DeviceCode, 0); err == nil {
			turnCred = &response.IotOTATurn{
				URLs:       c.URLs,
				Username:   c.Username,
				Credential: c.Password,
				TTL:        c.TTL,
				ExpiresAt:  c.ExpiresAt,
				Realm:      c.Realm,
			}
		}
		// 签发失败只省略字段：TURN 不可用不应阻断设备拿服务组配置。
	}

	return &response.IotOTAResp{
		Product:       product,
		Device:        outDevice,
		ServiceGroups: groups,
		Turn:          turnCred,
		ServerTime:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// Activate 激活设备主账号。重复激活返回 ErrAlreadyActivated（HTTP 409）。
func (s *DeviceEndpointService) Activate(ctx context.Context, deviceCode, deviceSecret string, req *request.IotActivateReq, ip string) (*response.IotDeviceActivated, error) {
	d, err := s.authenticate(ctx, deviceCode, deviceSecret)
	if err != nil {
		return nil, err
	}

	if _, err := s.devices.GetOwner(ctx, d.ID); err == nil {
		return nil, ErrAlreadyActivated
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	nickname, phone := "", ""
	if req != nil {
		nickname = strings.TrimSpace(req.Nickname)
		if nickname == "" {
			nickname = strings.TrimSpace(req.OwnerName)
		}
		phone = strings.TrimSpace(req.OwnerPhone)
	}

	account, password, err := createOwnerAccount(ctx, s.devices, d, nickname, phone, model.IotEventActivate, ip)
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
