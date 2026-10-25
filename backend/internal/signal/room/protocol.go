// Package room 实现「设备 ↔ 查看方」的 MQTT WebRTC 房间信令中继。
//
// 与 internal/signal/mqtt（App → 后端 → ZLM WHEP）是两条独立链路：
//
//	设备 ──subscribe iot/<group>/device/<device_code>/signal──▶ 收到 joinRoom
//	设备 ──publish  iot/<group>/signal/backend──────────────▶ 本进程
//	查看方（浏览器）──WebSocket──▶ 本进程 ──publish──▶ 设备话题
//
// 媒体面完全不经过后端：浏览器与设备直接交换 DTLS/SRTP（必要时经 TURN）。
//
// 握手顺序（与设备端约定）：
//
//  1. 查看方进入房间 → 发 joinRoom；
//  2. 设备收到 joinRoom → 回 joinRoom 并创建 offer；
//  3. 查看方收到设备的 joinRoom → 再发一次 joinRoom（确认双方都在线）；
//  4. 设备收到第二次 joinRoom → 发 offer；
//  5. 查看方 answer；双方 trickle iceCandidate；
//  6. 中途 candidate 变化、设备重新 offer（ICE restart）都按同一条通道转发。
package room

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// 消息类型。
const (
	TypeJoinRoom     = "joinRoom"
	TypeOffer        = "offer"
	TypeAnswer       = "answer"
	TypeICECandidate = "iceCandidate"
	TypeLeaveRoom    = "leaveRoom"
	TypeReady        = "ready"
	TypeError        = "error"

	// 设备 ↔ App 通话控制（v0.6.0 适老化相机）。
	TypeInvite = "invite"
	TypeAgree  = "agree"
	TypeReject = "reject"
	TypeHangup = "hangup"
)

// 角色。
const (
	RoleViewer = "viewer"
	RoleDevice = "device"
	RoleApp    = "app"
)

// 字段长度上限（MQTT 单包上限由 broker 决定，这里只做防呆）。
const (
	maxRoomLen      = 128
	maxDeviceLen    = 128
	maxClientLen    = 128
	maxRequestIDLen = 128
	maxCallIDLen    = 128
	maxSDPLen       = 1 << 20 // 1 MiB
	maxCandidateLen = 8192
)

// 错误码（回给浏览器的 error 消息）。
const (
	CodeInvalidMessage = "invalid_message"
	CodeInvalidType    = "invalid_type"
	CodeRoomNotFound   = "room_not_found"
	CodeDeviceMismatch = "device_mismatch"
	CodePublishFailed  = "publish_failed"
	CodeRoomFull       = "room_full"
)

var (
	// ErrInvalidMessage 消息体非法。
	ErrInvalidMessage = errors.New("非法信令消息")
	// ErrInvalidTopic 非法设备话题。
	ErrInvalidTopic = errors.New("非法设备信令话题")
	// ErrRoomNotFound 查看方未加入任何房间。
	ErrRoomNotFound = errors.New("房间不存在")
)

// ICECandidate 是浏览器 RTCIceCandidate.toJSON() 的结构。
type ICECandidate struct {
	Candidate        string  `json:"candidate"`
	SDPMid           *string `json:"sdpMid,omitempty"`
	SDPMLineIndex    *uint16 `json:"sdpMLineIndex,omitempty"`
	UsernameFragment *string `json:"usernameFragment,omitempty"`
}

// Message 是房间信令的统一 JSON 结构。
//
// 同一结构承载全部类型，用 type 区分；浏览器与设备共用。
// `client_id` 恒为发送方标识；`to` 是可选的目标接收方。
type Message struct {
	Type       string        `json:"type"`
	RoomID     string        `json:"room_id,omitempty"`
	DeviceCode string        `json:"device_code,omitempty"`
	ClientID   string        `json:"client_id,omitempty"`
	To         string        `json:"to,omitempty"`
	Role       string        `json:"role,omitempty"`
	SDP        string        `json:"sdp,omitempty"`
	SDPType    string        `json:"sdp_type,omitempty"`
	Candidate  *ICECandidate `json:"candidate,omitempty"`
	RequestID  string        `json:"request_id,omitempty"`
	// CallID 标识一通「设备 ↔ App」呼叫；媒体协商仍复用 offer/answer。
	CallID       string `json:"call_id,omitempty"`
	MediaType    string `json:"media_type,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Code         string `json:"code,omitempty"`
	ErrorMessage string `json:"message,omitempty"`
	Timestamp    int64  `json:"ts,omitempty"`
	// DeviceOnline 只在 ready 消息里出现，供页面提示「设备未在房间」。
	DeviceOnline bool `json:"device_online,omitempty"`
}

// NewError 构造 error 消息。
func NewError(code, message string) *Message {
	return &Message{Type: TypeError, Code: code, ErrorMessage: message}
}

// Marshal 序列化。
func (m *Message) Marshal() ([]byte, error) {
	if m == nil {
		return nil, errors.New("消息为空")
	}
	return json.Marshal(m)
}

// ParseMessage 解析并规范化消息类型。
func ParseMessage(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: 空消息体", ErrInvalidMessage)
	}
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	m.Type = NormalizeType(m.Type)
	m.RoomID = strings.TrimSpace(m.RoomID)
	m.DeviceCode = strings.TrimSpace(m.DeviceCode)
	m.ClientID = strings.TrimSpace(m.ClientID)
	m.To = strings.TrimSpace(m.To)
	m.Role = strings.ToLower(strings.TrimSpace(m.Role))
	m.SDPType = strings.ToLower(strings.TrimSpace(m.SDPType))
	m.RequestID = strings.TrimSpace(m.RequestID)
	m.Code = strings.TrimSpace(m.Code)
	return &m, nil
}

// NormalizeType 兼容小写/别名写法。
func NormalizeType(raw string) string {
	t := strings.TrimSpace(raw)
	switch strings.ToLower(t) {
	case "joinroom", "join_room":
		return TypeJoinRoom
	case "leaveroom", "leave_room":
		return TypeLeaveRoom
	case "icecandidate", "ice_candidate", "candidate":
		return TypeICECandidate
	case "offer":
		return TypeOffer
	case "answer":
		return TypeAnswer
	case "invite":
		return TypeInvite
	case "agree":
		return TypeAgree
	case "reject":
		return TypeReject
	case "hangup":
		return TypeHangup
	case "ready":
		return TypeReady
	case "error":
		return TypeError
	default:
		return t
	}
}

// IsRoomMessage 判断一条 MQTT 消息是否属于房间信令。
//
// 旧 WHEP 链路的 offer 没有 room_id，因此只按 RoomID 区分；
// 这样同一 broker 话题上两条链路可以并存。
func IsRoomMessage(payload []byte) bool {
	var probe struct {
		RoomID string `json:"room_id"`
	}
	if err := json.Unmarshal(payload, &probe); err != nil {
		return false
	}
	return strings.TrimSpace(probe.RoomID) != ""
}

// DeviceTopic 返回设备订阅的话题：iot/<group>/device/<device_code>/signal。
func DeviceTopic(group, deviceCode string) (string, error) {
	group = strings.TrimSpace(group)
	deviceCode = strings.TrimSpace(deviceCode)
	if group == "" || len(group) > maxRoomLen || !isSafeTopicLevel(group) {
		return "", fmt.Errorf("%w: group=%q", ErrInvalidTopic, group)
	}
	if deviceCode == "" || len(deviceCode) > maxDeviceLen || !isSafeTopicLevel(deviceCode) {
		return "", fmt.Errorf("%w: device_code=%q", ErrInvalidTopic, deviceCode)
	}
	return "iot/" + group + "/device/" + deviceCode + "/signal", nil
}

// ValidateFromViewer 校验浏览器发来的消息。
func (m *Message) ValidateFromViewer() error {
	if m == nil {
		return fmt.Errorf("%w: 消息为空", ErrInvalidMessage)
	}
	switch m.Type {
	case TypeJoinRoom, TypeAnswer, TypeICECandidate, TypeLeaveRoom,
		TypeInvite, TypeAgree, TypeReject, TypeHangup:
	default:
		return fmt.Errorf("%w: 查看方不允许发送 %q", ErrInvalidMessage, m.Type)
	}
	if err := m.validateCommon(); err != nil {
		return err
	}
	switch m.Type {
	case TypeAnswer:
		if strings.TrimSpace(m.SDP) == "" {
			return fmt.Errorf("%w: answer 缺少 sdp", ErrInvalidMessage)
		}
		if len(m.SDP) > maxSDPLen {
			return fmt.Errorf("%w: sdp 过大", ErrInvalidMessage)
		}
		m.SDPType = TypeAnswer
	case TypeICECandidate:
		if m.Candidate == nil {
			return fmt.Errorf("%w: iceCandidate 缺少 candidate", ErrInvalidMessage)
		}
		if len(m.Candidate.Candidate) > maxCandidateLen {
			return fmt.Errorf("%w: candidate 过长", ErrInvalidMessage)
		}
	case TypeInvite, TypeAgree, TypeReject, TypeHangup:
		if err := m.validateCall(); err != nil {
			return err
		}
	}
	return nil
}

// ValidateFromDevice 校验设备发来的消息。
func (m *Message) ValidateFromDevice() error {
	if m == nil {
		return fmt.Errorf("%w: 消息为空", ErrInvalidMessage)
	}
	switch m.Type {
	case TypeJoinRoom, TypeOffer, TypeAnswer, TypeICECandidate, TypeLeaveRoom, TypeError,
		TypeInvite, TypeAgree, TypeReject, TypeHangup:
	default:
		return fmt.Errorf("%w: 设备不允许发送 %q", ErrInvalidMessage, m.Type)
	}
	if err := m.validateCommon(); err != nil {
		return err
	}
	switch m.Type {
	case TypeOffer, TypeAnswer:
		if strings.TrimSpace(m.SDP) == "" {
			return fmt.Errorf("%w: %s 缺少 sdp", ErrInvalidMessage, m.Type)
		}
		if len(m.SDP) > maxSDPLen {
			return fmt.Errorf("%w: sdp 过大", ErrInvalidMessage)
		}
		if m.SDPType == "" {
			m.SDPType = m.Type
		}
	case TypeICECandidate:
		if m.Candidate == nil {
			return fmt.Errorf("%w: iceCandidate 缺少 candidate", ErrInvalidMessage)
		}
		if len(m.Candidate.Candidate) > maxCandidateLen {
			return fmt.Errorf("%w: candidate 过长", ErrInvalidMessage)
		}
	case TypeInvite, TypeAgree, TypeReject, TypeHangup:
		if err := m.validateCall(); err != nil {
			return err
		}
	}
	return nil
}

// validateCall 校验通话控制消息。
func (m *Message) validateCall() error {
	callID := strings.TrimSpace(m.CallID)
	if callID == "" {
		return fmt.Errorf("%w: %s 缺少 call_id", ErrInvalidMessage, m.Type)
	}
	if len(callID) > maxCallIDLen || !isSafeTopicLevel(callID) {
		return fmt.Errorf("%w: call_id 含非法字符", ErrInvalidMessage)
	}
	switch strings.ToLower(strings.TrimSpace(m.MediaType)) {
	case "", "video", "audio":
	default:
		return fmt.Errorf("%w: media_type 仅支持 video/audio", ErrInvalidMessage)
	}
	return nil
}

func (m *Message) validateCommon() error {
	if m.RoomID == "" {
		return fmt.Errorf("%w: room_id 为空", ErrInvalidMessage)
	}
	if len(m.RoomID) > maxRoomLen || !isSafeTopicLevel(m.RoomID) {
		return fmt.Errorf("%w: room_id 含非法字符", ErrInvalidMessage)
	}
	if m.ClientID != "" && (len(m.ClientID) > maxClientLen || !isSafeTopicLevel(m.ClientID)) {
		return fmt.Errorf("%w: client_id 含非法字符", ErrInvalidMessage)
	}
	if m.To != "" && (len(m.To) > maxClientLen || !isSafeTopicLevel(m.To)) {
		return fmt.Errorf("%w: to 含非法字符", ErrInvalidMessage)
	}
	if m.DeviceCode != "" && (len(m.DeviceCode) > maxDeviceLen || !isSafeTopicLevel(m.DeviceCode)) {
		return fmt.Errorf("%w: device_code 含非法字符", ErrInvalidMessage)
	}
	if m.RequestID != "" && len(m.RequestID) > maxRequestIDLen {
		return fmt.Errorf("%w: request_id 过长", ErrInvalidMessage)
	}
	return nil
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
