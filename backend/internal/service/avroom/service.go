// Package avroom 提供「实时音视频 → 查看设备画面」的业务能力：
// 设备越权校验、产品/服务组解析、WS ticket 与 TURN 凭据签发。
//
// 真正的信令中继在 internal/signal/room；本包不碰 WebSocket 与 MQTT，
// 只回答「这个用户能不能看这台设备、该往哪个服务组发信令」。
package avroom

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/internal/service/turn"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	signalroom "github.com/boticz/ai-camera-backend/internal/signal/room"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// TicketKind 查看方 ticket 的类型（与语音 ticket 区分，不能混用）。
const TicketKind = "av"

// 错误。
var (
	ErrDeviceNotBoundProduct = errors.New("设备未绑定 IOT 产品")
	ErrNoWebRTCGroup         = errors.New("设备产品未绑定启用的 WebRTC 服务组")
	ErrNoMQTTService         = errors.New("WebRTC 服务组未启用 MQTT 信令服务")
	ErrDeviceInactive        = errors.New("设备已停用")
	ErrDeviceCodeEmpty       = errors.New("设备编号为空，无法作为房间 ID")
	ErrSignalNotReady        = errors.New("MQTT 房间信令未启用（检查 MQTT_SIGNAL_ENABLED）")
)

// RoomInfo 是一台设备可被查看所需的信息。
type RoomInfo struct {
	Device    *model.Device
	Group     *model.IotServiceGroup
	MQTT      *model.IotService
	GroupCode string
	RoomID    string
}

// Service 实时音视频业务服务。
type Service struct {
	devices  *repository.DeviceRepo
	products *repository.IotProductRepo
	groups   *repository.IotServiceGroupRepo
	turn     *turn.Service
}

// NewService 构造服务。
func NewService() *Service {
	return &Service{
		devices:  repository.NewDeviceRepo(),
		products: repository.NewIotProductRepo(),
		groups:   repository.NewIotServiceGroupRepo(),
		turn:     turn.New(),
	}
}

// Resolve 校验设备并解析出可用的 WebRTC 服务组与 MQTT 服务。
//
// 校验顺序：设备存在且属于当前数据范围 → 设备启用 → 绑定产品 →
// 产品启用 → 产品绑定启用的 webrtc 服务组 → 组内有启用的 MQTT 服务。
func (s *Service) Resolve(ctx context.Context, deviceID int64, ds *utils.DataScope) (*RoomInfo, error) {
	if deviceID <= 0 {
		return nil, errors.New("设备ID无效")
	}
	dev, err := s.devices.GetByIDScoped(ctx, deviceID, ds)
	if err != nil {
		return nil, errors.New("设备不存在或无权访问")
	}
	if dev.Active != nil && !*dev.Active {
		return nil, ErrDeviceInactive
	}
	if strings.TrimSpace(dev.DeviceCode) == "" {
		return nil, ErrDeviceCodeEmpty
	}
	if dev.ProductID == nil || *dev.ProductID <= 0 {
		return nil, ErrDeviceNotBoundProduct
	}
	product, err := s.products.GetByID(ctx, *dev.ProductID)
	if err != nil {
		return nil, errors.New("设备所属产品不存在")
	}
	if product.Active != nil && !*product.Active {
		return nil, errors.New("设备所属产品已停用")
	}

	groups, err := s.products.ListServiceGroups(ctx, *dev.ProductID)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		group := groups[i]
		if group.Type != model.IotServiceGroupWebRTC || !active(group.Active) {
			continue
		}
		services, err := s.groups.ListServices(ctx, group.ID)
		if err != nil {
			return nil, err
		}
		for j := range services {
			svc := services[j]
			if !strings.EqualFold(svc.Protocol, model.IotProtocolMQTT) || !active(svc.Active) {
				continue
			}
			return &RoomInfo{
				Device:    dev,
				Group:     &group,
				MQTT:      &svc,
				GroupCode: group.Code,
				RoomID:    dev.DeviceCode,
			}, nil
		}
		return nil, ErrNoMQTTService
	}
	return nil, ErrNoWebRTCGroup
}

// IssueTicket 为当前用户签发查看方 ticket。
func (s *Service) IssueTicket(ctx context.Context, userID int64, userName, roleCode string, deptID int64, dataPerm, crossDeptIDs string, deviceID int64) (*response.AvViewerTicketResp, error) {
	if signalroom.DefaultHub() == nil {
		return nil, ErrSignalNotReady
	}
	ds := &utils.DataScope{
		UserID: userID, UserName: userName, DataPerm: dataPerm,
		CrossDeptIDs: crossDeptIDs,
	}
	if deptID > 0 {
		ds.DeptID = &deptID
	}
	info, err := s.Resolve(ctx, deviceID, ds)
	if err != nil {
		return nil, err
	}

	clientID, err := randomID("viewer-", 8)
	if err != nil {
		return nil, err
	}
	tickets := voiceagent.DefaultTicketService()
	ticket, claims, err := tickets.Issue(voiceagent.TicketClaims{
		UserID:       userID,
		UserName:     userName,
		RoleCode:     roleCode,
		DeptID:       deptID,
		DataPerm:     dataPerm,
		CrossDeptIDs: crossDeptIDs,
		DeviceID:     info.Device.ID,
		ClientID:     clientID,
		Kind:         TicketKind,
	})
	if err != nil {
		return nil, err
	}

	return &response.AvViewerTicketResp{
		Ticket:       ticket,
		ExpiresIn:    int(time.Until(time.Unix(claims.ExpiresAt, 0)).Seconds()),
		WsURL:        "/web/av/ws",
		RoomID:       info.RoomID,
		DeviceID:     info.Device.ID,
		DeviceCode:   info.Device.DeviceCode,
		DeviceName:   info.Device.Name,
		ClientID:     clientID,
		GroupCode:    info.GroupCode,
		DeviceOnline: signalroom.DefaultHub().DeviceOnline(info.RoomID),
		IceServers:   s.iceServers(userID),
	}, nil
}

// VerifyTicket 校验一次性 ticket，并重新解析设备（防止签发后设备被解绑）。
func (s *Service) VerifyTicket(ctx context.Context, ticket string) (*voiceagent.TicketClaims, *RoomInfo, error) {
	claims, err := voiceagent.DefaultTicketService().Verify(TicketKind, ticket)
	if err != nil {
		return nil, nil, err
	}
	ds := &utils.DataScope{
		UserID: claims.UserID, UserName: claims.UserName, DataPerm: claims.DataPerm,
		CrossDeptIDs: claims.CrossDeptIDs,
	}
	if claims.DeptID > 0 {
		ds.DeptID = &claims.DeptID
	}
	info, err := s.Resolve(ctx, claims.DeviceID, ds)
	if err != nil {
		return nil, nil, err
	}
	return claims, info, nil
}

// iceServers 签发 TURN REST 短期凭据；未配置 TURN 时返回空数组。
func (s *Service) iceServers(userID int64) []response.IceServer {
	cred, err := s.turn.Issue("web-"+strconv.FormatInt(userID, 10), 0)
	if err != nil || cred == nil {
		return []response.IceServer{}
	}
	return []response.IceServer{{
		URLs:       cred.URLs,
		Username:   cred.Username,
		Credential: cred.Password,
		Realm:      cred.Realm,
		TTL:        cred.TTL,
		ExpiresAt:  cred.ExpiresAt,
	}}
}

// active 空值按启用处理（与项目其他 Active 字段语义一致）。
func active(v *bool) bool { return v == nil || *v }

func randomID(prefix string, n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}
