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
// IOT 管理 - 明文鉴权信息（Web 复制 / App 服务）
//
// 明文密码只在「产品×服务」维度存在：组本身不持有凭据。
// 普通读接口返回 ****，只有本文件的 ProductCredentials 返回明文。
// ════════════════════════════════════════════════════════════════

const (
	turnEnvURLs  = "TURN_URLS"
	turnEnvRealm = "TURN_REALM"

	defaultTurnRealm = "ai-camera"
	defaultTurnURLs  = "stun:127.0.0.1:3478,turn:127.0.0.1:3478?transport=udp,turn:127.0.0.1:3478?transport=tcp"
)

// CredentialsService 构建明文鉴权信息。
type CredentialsService struct {
	products       *repository.IotProductRepo
	groups         *repository.IotServiceGroupRepo
	productService *repository.IotProductServiceRepo
}

// NewCredentialsService 构造凭据服务。
func NewCredentialsService() *CredentialsService {
	return &CredentialsService{
		products:       repository.NewIotProductRepo(),
		groups:         repository.NewIotServiceGroupRepo(),
		productService: repository.NewIotProductServiceRepo(),
	}
}

// ProductCredentials 返回产品及其全部服务的明文鉴权信息（Web 复制用）。
func (s *CredentialsService) ProductCredentials(ctx context.Context, productID int64) (*response.IotCredentials, error) {
	return s.productCredentials(ctx, productID)
}

// ProductCredentialsForApp 返回 App 端所需的服务信息（同样含明文，App 已完成 JWT + 绑定校验）。
func (s *CredentialsService) ProductCredentialsForApp(ctx context.Context, productID int64) (*response.IotCredentials, error) {
	return s.productCredentials(ctx, productID)
}

func (s *CredentialsService) productCredentials(ctx context.Context, productID int64) (*response.IotCredentials, error) {
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
	rows, err := s.productService.ListByProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	rowByService := make(map[int64]model.IotProductService, len(rows))
	for _, row := range rows {
		rowByService[row.ServiceID] = row
	}

	out := &response.IotCredentials{
		Product: response.IotCredentialsProduct{Code: p.Code, Name: p.Name},
		Groups:  make([]response.IotCredentialsGroup, 0, len(groups)),
	}
	for i := range groups {
		services, err := s.groups.ListServices(ctx, groups[i].ID)
		if err != nil {
			return nil, err
		}
		creds := make([]response.IotCredentialsService, 0, len(services))
		for _, svc := range services {
			row, ok := rowByService[svc.ID]
			if !ok {
				continue // 产品尚未生成该服务的凭据（历史数据/事务中）
			}
			creds = append(creds, response.IotCredentialsService{
				Code:     svc.Code,
				Name:     svc.Name,
				Protocol: svc.Protocol,
				Host:     svc.Host,
				Port:     svc.Port,
				Path:     svc.Path,
				Extra:    svc.Extra,
				Username: row.Username,
				// 明文仅此一处；普通读接口走 maskProductServices。
				Password: row.Password,
			})
		}
		out.Groups = append(out.Groups, response.IotCredentialsGroup{
			Code:     groups[i].Code,
			Name:     groups[i].Name,
			Type:     groups[i].Type,
			Services: creds,
		})
		if out.MQTT == nil {
			out.MQTT = mqttInfo(groups[i].Code, creds)
		}
	}
	out.Turn = turnEndpointInfo()
	return out, nil
}

// mqttInfo 从产品服务里提取 MQTT 连接地址。
func mqttInfo(groupCode string, services []response.IotCredentialsService) *response.IotMqttInfo {
	for i := range services {
		svc := services[i]
		if !strings.EqualFold(svc.Protocol, model.IotProtocolMQTT) {
			continue
		}
		host := strings.TrimSpace(svc.Host)
		if host == "" {
			continue
		}
		port := svc.Port
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
