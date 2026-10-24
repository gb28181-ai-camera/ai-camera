package iot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 明文鉴权信息（Web 复制 / App 服务组）
//
// 为什么明文凭据要单独接口：
//   - 列表/详情接口面向「浏览」，返回 **** 即可；把明文塞进去会让任何
//     拥有 iot:*:list 的角色顺带拿到全部设备服务口令。
//   - 「复制鉴权信息」是显式、低频的人为动作，单独端点可要求
//     iot:product:update / iot:service-group:update 这类更高权限，
//     也便于审计与网关按 URL 单独限流。
//   - 明文密码只在本文件构建的响应里出现；调用方不得复用这些结构体
//     到普通读接口。
// ════════════════════════════════════════════════════════════════

// TURN 配置读取的环境变量名。
//
// 与 internal/service/turn 使用同一组变量；这里只读取「地址与 Realm」，
// 不签发凭据（Web 复制内容里不应带短期口令）。
const (
	turnEnvURLs  = "TURN_URLS"
	turnEnvRealm = "TURN_REALM"

	defaultTurnRealm = "ai-camera"
	// 与 turn 包默认值保持一致：客户端（手机/浏览器）不在 compose 网络里，
	// 因此默认用回环地址；生产必须通过 TURN_URLS 配成可达地址。
	defaultTurnURLs = "stun:127.0.0.1:3478,turn:127.0.0.1:3478?transport=udp,turn:127.0.0.1:3478?transport=tcp"
)

// CredentialsService 构建明文鉴权信息。
type CredentialsService struct {
	products *repository.IotProductRepo
	groups   *repository.IotServiceGroupRepo
}

// NewCredentialsService 构造凭据服务。
func NewCredentialsService() *CredentialsService {
	return &CredentialsService{
		products: repository.NewIotProductRepo(),
		groups:   repository.NewIotServiceGroupRepo(),
	}
}

// ProductCredentials 返回产品及其全部服务组的明文鉴权信息（Web 复制用）。
func (s *CredentialsService) ProductCredentials(ctx context.Context, productID int64) (*response.IotCredentials, error) {
	return s.productCredentials(ctx, productID, true)
}

// ProductCredentialsForApp 返回 App 端所需的服务组信息。
//
// App 使用方已经通过 JWT + 设备绑定校验了身份，因此需要明文凭据才能连接
// MQTT/WebRTC 服务；但 TURN 凭据由独立的 /app/rtc/ice-servers 端点签发，
// 这里不重复下发，避免同一份响应里出现两套不同 TTL 的临时口令。
func (s *CredentialsService) ProductCredentialsForApp(ctx context.Context, productID int64) (*response.IotCredentials, error) {
	return s.productCredentials(ctx, productID, false)
}

func (s *CredentialsService) productCredentials(ctx context.Context, productID int64, withTurn bool) (*response.IotCredentials, error) {
	p, err := s.products.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		return nil, err
	}

	groups, err := s.products.ListServiceGroups(ctx, productID)
	if err != nil {
		return nil, err
	}

	out := &response.IotCredentials{
		Product: response.IotCredentialsProduct{Code: p.Code, Name: p.Name},
		Groups:  make([]response.IotCredentialsGroup, 0, len(groups)),
	}
	for i := range groups {
		items, err := s.groups.ListItems(ctx, groups[i].ID)
		if err != nil {
			return nil, err
		}
		if items == nil {
			items = []model.IotServiceItem{}
		}
		out.Groups = append(out.Groups, response.IotCredentialsGroup{
			Code:     groups[i].Code,
			Name:     groups[i].Name,
			Type:     groups[i].Type,
			Username: groups[i].Username,
			// 明文仅此一处；普通读接口仍走 maskGroup。
			Password: groups[i].Password,
			Items:    items,
		})
		if out.MQTT == nil {
			out.MQTT = mqttInfo(groups[i].Code, items)
		}
	}
	if withTurn {
		out.Turn = turnEndpointInfo()
	}
	return out, nil
}

// GroupCredentials 返回单个服务组的明文鉴权信息（Web 复制用）。
func (s *CredentialsService) GroupCredentials(ctx context.Context, groupID int64) (*response.IotGroupCredentials, error) {
	g, err := s.groups.GetByID(ctx, groupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrServiceGroupNotFound
		}
		return nil, err
	}
	items, err := s.groups.ListItems(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.IotServiceItem{}
	}
	return &response.IotGroupCredentials{
		Group: response.IotCredentialsGroup{
			Code:     g.Code,
			Name:     g.Name,
			Type:     g.Type,
			Username: g.Username,
			Password: g.Password,
			Items:    items,
		},
		MQTT: mqttInfo(g.Code, items),
	}, nil
}

// mqttInfo 从服务项里提取 MQTT 连接地址。
//
// 没有 MQTT 服务项（或 host 未配置）时返回 nil：宁可让调用方看到「没有 MQTT」，
// 也不要拼出 mqtt://:1883 这种看似可用、实际连不上的地址。
func mqttInfo(groupCode string, items []model.IotServiceItem) *response.IotMqttInfo {
	for i := range items {
		it := items[i]
		if !strings.EqualFold(it.Protocol, model.IotProtocolMQTT) {
			continue
		}
		host := strings.TrimSpace(it.Host)
		if host == "" {
			continue
		}
		port := it.Port
		if port <= 0 {
			port = protocolDefaultPorts[model.IotProtocolMQTT]
		}
		return &response.IotMqttInfo{
			URL: fmt.Sprintf("mqtt://%s:%d", host, port),
			// 主题前缀按服务组隔离，MQTT ACL 依赖该约定做组间隔离。
			TopicPrefix: "iot/" + groupCode,
		}
	}
	return nil
}

// turnEndpointInfo 返回 TURN/STUN 的地址与 Realm，不签发凭据。
//
// 未配置 TURN 时返回 nil，避免把回环默认地址误当成生产可用地址展示给用户。
func turnEndpointInfo() *response.IotTurnInfo {
	urlsRaw := strings.TrimSpace(os.Getenv(turnEnvURLs))
	secretSet := strings.TrimSpace(os.Getenv("TURN_STATIC_AUTH_SECRET")) != ""
	if urlsRaw == "" && !secretSet {
		return nil
	}
	if urlsRaw == "" {
		urlsRaw = defaultTurnURLs
	}
	urls := make([]string, 0, 3)
	for _, p := range strings.Split(urlsRaw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			urls = append(urls, p)
		}
	}
	realm := strings.TrimSpace(os.Getenv(turnEnvRealm))
	if realm == "" {
		realm = defaultTurnRealm
	}
	return &response.IotTurnInfo{URLs: urls, Realm: realm}
}
