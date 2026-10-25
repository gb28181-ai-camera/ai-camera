// Package mqtt 实现 App ↔ 后端的 MQTT 信令桥。
//
// 拓扑：
//
//	App ──publish iot/<group>/signal/backend──▶ broker ──▶ 本进程
//	本进程 ──POST SDP offer──▶ ZLM /index/api/webrtc
//	本进程 ──publish iot/<group>/app/<client_id>/signal──▶ broker ──▶ App
//
// 媒体面不经过后端：App 与 ZLM 直接交换 RTP/DTLS（必要时经 TURN）。
// 后端只转发 SDP，不解析媒体，也不承担 NAT 穿透。
package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// 消息类型。
const (
	TypeOffer     = "offer"
	TypeAnswer    = "answer"
	TypeError     = "error"
	TypeCandidate = "candidate"
)

// 话题约定。
const (
	// BackendTopicFilter 是后端订阅的通配符：iot/<group_code>/signal/backend。
	BackendTopicFilter = "iot/+/signal/backend"
	// backendTopicLevels 是 iot/<group>/signal/backend 的层级数。
	backendTopicLevels = 4
)

// 内部账号。密码来自环境变量 MQTT_RPC_TOKEN（见 service/iot 的鉴权分支）。
const (
	// PlatformUsername 是后端信令 worker 连接 broker 时使用的固定用户名。
	PlatformUsername = "__platform__"
	// PlatformGroupCode 是鉴权成功后返回的组编码；ACL 只认这个特殊值。
	PlatformGroupCode = "__platform__"
)

// 字段长度上限。MQTT 单包上限由 broker 决定，这里只做防呆，
// 避免异常客户端把超长字符串写进日志或 ZLM URL。
const (
	maxRequestIDLen = 128
	maxClientIDLen  = 128
	maxGroupLen     = 64
	maxStreamLen    = 128
	maxSDPLen       = 1 << 20 // 1 MiB
)

var (
	// ErrInvalidTopic 话题不符合 iot/<group>/signal/backend。
	ErrInvalidTopic = errors.New("非法信令话题")
	// ErrInvalidMessage 消息体不是合法 JSON 或缺少必填字段。
	ErrInvalidMessage = errors.New("非法信令消息")
)

// Message 是信令话题上的统一 JSON 结构。
//
// 同一结构承载 offer / answer / error / candidate；用 type 区分。
// omitempty 让 answer 不携带 stream/channel，error 不携带 sdp，保持报文精简。
type Message struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
	ClientID  string `json:"client_id,omitempty"`
	ChannelID int64  `json:"channel_id,omitempty"`
	Stream    string `json:"stream,omitempty"`
	SDP       string `json:"sdp,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	// RoomID 非空表示这是「设备 ↔ 查看方」房间信令，由 internal/signal/room
	// 处理；WHEP 桥必须忽略，不能把它当成 App 的点播 offer。
	RoomID string `json:"room_id,omitempty"`
}

// NewAnswer 构造发给 App 的 answer 消息。
func NewAnswer(requestID, sdp string) *Message {
	return &Message{Type: TypeAnswer, RequestID: requestID, SDP: sdp}
}

// NewError 构造发给 App 的 error 消息。
func NewError(requestID, code, message string) *Message {
	return &Message{Type: TypeError, RequestID: requestID, Code: code, Message: message}
}

// ParseMessage 解析 JSON 消息。
func ParseMessage(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: 空消息体", ErrInvalidMessage)
	}
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	m.Type = strings.ToLower(strings.TrimSpace(m.Type))
	m.RequestID = strings.TrimSpace(m.RequestID)
	m.ClientID = strings.TrimSpace(m.ClientID)
	m.Stream = strings.TrimSpace(m.Stream)
	m.Code = strings.TrimSpace(m.Code)
	return &m, nil
}

// Marshal 序列化消息。
func (m *Message) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("消息为空")
	}
	return json.Marshal(m)
}

// ValidateOffer 校验一条 offer 的必填字段。
//
// 只做「能不能安全地转发」的检查，不解析 SDP 内容 —— 后端不是 SDP 的语义所有者，
// 真正的协商结果由 ZLM 决定。
func (m *Message) ValidateOffer() error {
	if m == nil {
		return fmt.Errorf("%w: 消息为空", ErrInvalidMessage)
	}
	if m.Type != TypeOffer {
		return fmt.Errorf("%w: type=%q", ErrInvalidMessage, m.Type)
	}
	if m.RequestID == "" {
		return fmt.Errorf("%w: request_id 为空", ErrInvalidMessage)
	}
	if len(m.RequestID) > maxRequestIDLen {
		return fmt.Errorf("%w: request_id 过长", ErrInvalidMessage)
	}
	if m.ClientID == "" {
		return fmt.Errorf("%w: client_id 为空", ErrInvalidMessage)
	}
	if len(m.ClientID) > maxClientIDLen || !isSafeTopicLevel(m.ClientID) {
		return fmt.Errorf("%w: client_id 含非法字符", ErrInvalidMessage)
	}
	if m.ChannelID <= 0 && m.Stream == "" {
		return fmt.Errorf("%w: channel_id 与 stream 至少提供一个", ErrInvalidMessage)
	}
	if len(m.Stream) > maxStreamLen || (m.Stream != "" && !isSafeQueryValue(m.Stream)) {
		return fmt.Errorf("%w: stream 含非法字符", ErrInvalidMessage)
	}
	if strings.TrimSpace(m.SDP) == "" {
		return fmt.Errorf("%w: sdp 为空", ErrInvalidMessage)
	}
	if len(m.SDP) > maxSDPLen {
		return fmt.Errorf("%w: sdp 过大", ErrInvalidMessage)
	}
	return nil
}

// ParseBackendTopic 从 iot/<group>/signal/backend 提取 group_code。
func ParseBackendTopic(topic string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(topic), "/")
	if len(parts) != backendTopicLevels {
		return "", false
	}
	if parts[0] != "iot" || parts[2] != "signal" || parts[3] != "backend" {
		return "", false
	}
	group := strings.TrimSpace(parts[1])
	if group == "" || len(group) > maxGroupLen || !isSafeTopicLevel(group) {
		return "", false
	}
	return group, true
}

// BackendSignalTopic 返回某个组的后端订阅话题。
func BackendSignalTopic(group string) string {
	return "iot/" + group + "/signal/backend"
}

// AppSignalTopic 构造发给指定 App 的话题：iot/<group>/app/<client_id>/signal。
//
// client_id 直接进入话题层级，必须拒绝 / + #，否则会越出本客户端命名空间。
func AppSignalTopic(group, clientID string) (string, error) {
	group = strings.TrimSpace(group)
	clientID = strings.TrimSpace(clientID)
	if group == "" || len(group) > maxGroupLen || !isSafeTopicLevel(group) {
		return "", fmt.Errorf("%w: group_code=%q", ErrInvalidTopic, group)
	}
	if clientID == "" || len(clientID) > maxClientIDLen || !isSafeTopicLevel(clientID) {
		return "", fmt.Errorf("%w: client_id=%q", ErrInvalidTopic, clientID)
	}
	return "iot/" + group + "/app/" + clientID + "/signal", nil
}

// isSafeTopicLevel 判断字符串能否安全地作为一个 MQTT topic level。
func isSafeTopicLevel(v string) bool {
	if v == "" || strings.ContainsAny(v, "/+#") {
		return false
	}
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// isSafeQueryValue 判断字符串能否放进 ZLM 的查询参数。
//
// 这里不禁止空格等字符（url.Values 会转义），只拒绝控制字符；
// 真正的转义由 webrtc.WHEPURL 完成。
func isSafeQueryValue(v string) bool {
	for _, r := range v {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}
