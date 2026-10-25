package room

import (
	"errors"
	"strings"
	"sync"
	"time"

	mqttsig "github.com/boticz/ai-camera-backend/internal/signal/mqtt"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
)

// 房间参数。
const (
	// DefaultMaxViewersPerRoom 单个设备最多同时查看数。
	// 每个查看方在设备侧对应一条独立的 PeerConnection；8 路足够覆盖
	// 「多人同时查看」又不至于让设备编码器过载。
	DefaultMaxViewersPerRoom = 8
	// DefaultQoS 房间信令与 MQTT 信令一致，使用 QoS1。
	DefaultQoS byte = 1
	// deviceOfflineGrace 设备最后一次信令后多久视为离线（仅用于展示）。
	deviceOfflineGrace = 90 * time.Second
)

// Publisher 是最小 MQTT 发布接口（*mqttsig.Client 实现，测试可用假实现）。
type Publisher interface {
	Publish(topic string, qos byte, retained bool, payload []byte) error
}

// SendFunc 把消息推给查看方。返回 false 表示连接已关闭。
type SendFunc func(msg *Message) bool

// Viewer 是一路浏览器查看会话。
type Viewer struct {
	ID         string
	DeviceID   int64
	RoomID     string
	DeviceCode string
	GroupCode  string
	UserID     int64
	Send       SendFunc
}

// Room 是一台设备的信令房间。
type Room struct {
	ID         string
	DeviceCode string
	GroupCode  string

	mu             sync.RWMutex
	viewers        map[string]*Viewer
	deviceOnline   bool
	deviceClientID string
	lastDeviceSeen time.Time
}

// Hub 维护全部房间与查看方。
type Hub struct {
	pub        Publisher
	maxViewers int

	mu      sync.RWMutex
	rooms   map[string]*Room
	viewers map[string]*Room // viewer id → room
	now     func() time.Time
}

// NewHub 构造房间中继。
func NewHub(pub Publisher) *Hub {
	return &Hub{
		pub:        pub,
		maxViewers: DefaultMaxViewersPerRoom,
		rooms:      make(map[string]*Room),
		viewers:    make(map[string]*Room),
		now:        time.Now,
	}
}

// Join 把查看方加入房间。
//
// ready 消息由调用方（WS handler）发送，这里只登记并返回房间状态，
// 避免 Hub 依赖具体的 WebSocket 实现。
func (h *Hub) Join(v *Viewer) (*Room, error) {
	if h == nil || v == nil {
		return nil, errors.New("房间未初始化")
	}
	if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.RoomID) == "" {
		return nil, errors.New("viewer_id / room_id 不能为空")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	room := h.rooms[v.RoomID]
	if room == nil {
		room = &Room{
			ID:         v.RoomID,
			DeviceCode: v.DeviceCode,
			GroupCode:  v.GroupCode,
			viewers:    make(map[string]*Viewer),
		}
		h.rooms[v.RoomID] = room
	}
	if room.DeviceCode != "" && v.DeviceCode != "" && room.DeviceCode != v.DeviceCode {
		return nil, ErrDeviceMismatch
	}

	room.mu.Lock()
	defer room.mu.Unlock()
	if _, exists := room.viewers[v.ID]; !exists && len(room.viewers) >= h.maxViewers {
		return nil, ErrRoomFull
	}
	room.viewers[v.ID] = v
	h.viewers[v.ID] = room
	return room, nil
}

// ErrDeviceMismatch 房间与设备不匹配。
var ErrDeviceMismatch = errors.New("设备与房间不匹配")

// ErrRoomFull 房间已满。
var ErrRoomFull = errors.New("房间查看人数已满")

// ForwardFromViewer 把浏览器消息转发给设备。
func (h *Hub) ForwardFromViewer(viewerID string, msg *Message) error {
	if h == nil || msg == nil {
		return ErrInvalidMessage
	}
	h.mu.RLock()
	room := h.viewers[viewerID]
	h.mu.RUnlock()
	if room == nil {
		return ErrRoomNotFound
	}

	// 服务端覆盖身份字段：不信任浏览器自行填写的 device_code/room_id/client_id。
	out := *msg
	out.RoomID = room.ID
	out.DeviceCode = room.DeviceCode
	out.ClientID = viewerID
	out.Role = RoleViewer
	if err := out.ValidateFromViewer(); err != nil {
		return err
	}

	// answer / iceCandidate 明确发给设备客户端；joinRoom / leaveRoom 也带上，
	// 设备端无需再猜发送者。
	out.To = deviceClientID(room.DeviceCode)
	if out.Type == TypeLeaveRoom && out.Reason == "" {
		out.Reason = "viewer_left"
	}
	return h.publishToDevice(room, &out)
}

// HandleDevice 处理设备发到 iot/+/signal/backend 的消息。
//
// 返回 handled=false 表示这不是房间信令（可能是旧 App → ZLM WHEP 链路的 offer）。
func (h *Hub) HandleDevice(topic string, payload []byte) (handled bool) {
	if h == nil || !IsRoomMessage(payload) {
		return false
	}
	group, ok := mqttsig.ParseBackendTopic(topic)
	if !ok {
		utils.Warn("WebRTC 房间：忽略非法话题", zap.String("topic", topic))
		return true
	}
	msg, err := ParseMessage(payload)
	if err != nil {
		utils.Warn("WebRTC 房间：消息解析失败", zap.String("topic", topic), zap.Error(err))
		return true
	}
	if err := msg.ValidateFromDevice(); err != nil {
		utils.Warn("WebRTC 房间：设备消息校验失败",
			zap.String("room_id", msg.RoomID), zap.Error(err))
		return true
	}

	h.mu.Lock()
	room := h.rooms[msg.RoomID]
	if room == nil {
		room = &Room{
			ID:         msg.RoomID,
			DeviceCode: msg.DeviceCode,
			GroupCode:  group,
			viewers:    make(map[string]*Viewer),
		}
		h.rooms[msg.RoomID] = room
	}
	h.mu.Unlock()

	room.mu.Lock()
	if room.GroupCode != "" && room.GroupCode != group {
		room.mu.Unlock()
		utils.Warn("WebRTC 房间：服务组不匹配",
			zap.String("room_id", msg.RoomID),
			zap.String("expect", room.GroupCode),
			zap.String("got", group))
		return true
	}
	if msg.DeviceCode != "" && room.DeviceCode != "" && msg.DeviceCode != room.DeviceCode {
		room.mu.Unlock()
		utils.Warn("WebRTC 房间：设备编码不匹配",
			zap.String("room_id", msg.RoomID),
			zap.String("device_code", msg.DeviceCode))
		return true
	}
	room.deviceOnline = msg.Type != TypeLeaveRoom
	room.deviceClientID = msg.ClientID
	room.lastDeviceSeen = h.now()
	targets := make([]*Viewer, 0, len(room.viewers))
	for _, v := range room.viewers {
		if msg.To != "" && v.ID != msg.To {
			continue
		}
		targets = append(targets, v)
	}
	room.mu.Unlock()

	if msg.Type == TypeLeaveRoom {
		room.mu.Lock()
		room.deviceOnline = false
		room.mu.Unlock()
	}

	for _, v := range targets {
		if v.Send == nil {
			continue
		}
		if !v.Send(msg) {
			h.LeaveViewer(v.ID, "send_failed")
		}
	}
	utils.Debug("WebRTC 房间：设备消息已转发",
		zap.String("room_id", msg.RoomID),
		zap.String("type", msg.Type),
		zap.Int("viewers", len(targets)))
	return true
}

// LeaveViewer 移除查看方并通知设备关闭对应的 PeerConnection。
func (h *Hub) LeaveViewer(viewerID, reason string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	room := h.viewers[viewerID]
	delete(h.viewers, viewerID)
	h.mu.Unlock()
	if room == nil {
		return
	}

	room.mu.Lock()
	delete(room.viewers, viewerID)
	empty := len(room.viewers) == 0 && !room.deviceOnline
	room.mu.Unlock()

	msg := &Message{
		Type:       TypeLeaveRoom,
		RoomID:     room.ID,
		DeviceCode: room.DeviceCode,
		ClientID:   viewerID,
		Role:       RoleViewer,
		To:         deviceClientID(room.DeviceCode),
		Reason:     strings.TrimSpace(reason),
	}
	if msg.Reason == "" {
		msg.Reason = "viewer_left"
	}
	if err := h.publishToDevice(room, msg); err != nil {
		utils.Warn("WebRTC 房间：离开通知发送失败",
			zap.String("room_id", room.ID), zap.Error(err))
	}

	if empty {
		h.mu.Lock()
		if current := h.rooms[room.ID]; current == room {
			room.mu.RLock()
			stillEmpty := len(room.viewers) == 0 && !room.deviceOnline
			room.mu.RUnlock()
			if stillEmpty {
				delete(h.rooms, room.ID)
			}
		}
		h.mu.Unlock()
	}
}

// DeviceOnline 返回房间内设备是否在线（最近有信令且未 leaveRoom）。
func (h *Hub) DeviceOnline(roomID string) bool {
	h.mu.RLock()
	room := h.rooms[roomID]
	h.mu.RUnlock()
	if room == nil {
		return false
	}
	room.mu.RLock()
	defer room.mu.RUnlock()
	return room.deviceOnline && h.now().Sub(room.lastDeviceSeen) <= deviceOfflineGrace
}

// Stats 返回房间/查看方数量（健康检查与排障）。
func (h *Hub) Stats() (rooms int, viewers int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms), len(h.viewers)
}

func (h *Hub) publishToDevice(room *Room, msg *Message) error {
	if h.pub == nil {
		return errors.New("MQTT publisher 未配置")
	}
	topic, err := DeviceTopic(room.GroupCode, room.DeviceCode)
	if err != nil {
		return err
	}
	payload, err := msg.Marshal()
	if err != nil {
		return err
	}
	if err := h.pub.Publish(topic, DefaultQoS, false, payload); err != nil {
		return err
	}
	utils.Debug("WebRTC 房间：已下发设备",
		zap.String("topic", topic),
		zap.String("type", msg.Type),
		zap.String("to", msg.To))
	return nil
}

func deviceClientID(deviceCode string) string {
	return "device-" + strings.TrimSpace(deviceCode)
}

// 默认 Hub 供 HTTP/WS handler 使用。
//
// 与 voiceagent.DefaultGateway 同样的做法：main 在启动时注入 broker
// publisher，handler 通过 DefaultHub() 取用；未启用 MQTT 时为 nil，
// 接口会明确返回「信令服务未就绪」而不是静默失败。
var (
	defaultHub   *Hub
	defaultHubMu sync.RWMutex
)

// SetDefaultHub 注入默认 Hub。
func SetDefaultHub(h *Hub) {
	defaultHubMu.Lock()
	defaultHub = h
	defaultHubMu.Unlock()
}

// DefaultHub 返回默认 Hub（可能为 nil）。
func DefaultHub() *Hub {
	defaultHubMu.RLock()
	defer defaultHubMu.RUnlock()
	return defaultHub
}
