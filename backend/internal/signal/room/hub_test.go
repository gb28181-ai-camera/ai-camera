package room

import (
	"encoding/json"
	"sync"
	"testing"
)

type fakePublisher struct {
	mu       sync.Mutex
	topic    string
	payloads [][]byte
}

func (p *fakePublisher) Publish(topic string, _ byte, _ bool, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.topic = topic
	p.payloads = append(p.payloads, append([]byte(nil), payload...))
	return nil
}

func (p *fakePublisher) last() (string, Message, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.payloads) == 0 {
		return "", Message{}, false
	}
	var m Message
	if err := json.Unmarshal(p.payloads[len(p.payloads)-1], &m); err != nil {
		return "", Message{}, false
	}
	return p.topic, m, true
}

type fakeViewer struct {
	mu   sync.Mutex
	msgs []Message
}

func (v *fakeViewer) send(m *Message) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.msgs = append(v.msgs, *m)
	return true
}

func (v *fakeViewer) count() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.msgs)
}

func newViewer(id, deviceCode string) (*Viewer, *fakeViewer) {
	fv := &fakeViewer{}
	return &Viewer{
		ID:         id,
		RoomID:     deviceCode,
		DeviceCode: deviceCode,
		GroupCode:  "webrtc",
		Send:       fv.send,
	}, fv
}

func deviceMessage(t *testing.T, payload string) []byte {
	t.Helper()
	if !json.Valid([]byte(payload)) {
		t.Fatalf("测试消息不是合法 JSON: %s", payload)
	}
	return []byte(payload)
}

func TestViewerJoinPublishesToDevice(t *testing.T) {
	pub := &fakePublisher{}
	hub := NewHub(pub)
	v, _ := newViewer("viewer-1", "CAM_001")
	if _, err := hub.Join(v); err != nil {
		t.Fatalf("Join: %v", err)
	}
	if err := hub.ForwardFromViewer("viewer-1", &Message{Type: TypeJoinRoom}); err != nil {
		t.Fatalf("ForwardFromViewer: %v", err)
	}
	topic, msg, ok := pub.last()
	if !ok {
		t.Fatal("未发布设备消息")
	}
	if topic != "iot/webrtc/device/CAM_001/signal" {
		t.Fatalf("topic=%q", topic)
	}
	if msg.Type != TypeJoinRoom || msg.ClientID != "viewer-1" || msg.To != "device-CAM_001" {
		t.Fatalf("消息身份字段不对: %+v", msg)
	}
	if msg.Role != RoleViewer || msg.DeviceCode != "CAM_001" || msg.RoomID != "CAM_001" {
		t.Fatalf("消息路由字段不对: %+v", msg)
	}
}

func TestDeviceJoinBroadcastAndOfferTarget(t *testing.T) {
	hub := NewHub(&fakePublisher{})
	v1, f1 := newViewer("viewer-1", "CAM_001")
	v2, f2 := newViewer("viewer-2", "CAM_001")
	if _, err := hub.Join(v1); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.Join(v2); err != nil {
		t.Fatal(err)
	}

	// 设备进入房间：广播给所有查看方。
	join := deviceMessage(t, `{"type":"joinRoom","room_id":"CAM_001","device_code":"CAM_001","client_id":"device-CAM_001","role":"device"}`)
	if !hub.HandleDevice("iot/webrtc/signal/backend", join) {

		t.Fatal("设备 joinRoom 未被识别")
	}
	if f1.count() != 1 || f2.count() != 1 {
		t.Fatalf("joinRoom 应广播，实际 v1=%d v2=%d", f1.count(), f2.count())
	}

	// 设备发 offer，to 指定 viewer-2：只应到达 viewer-2。
	offer := deviceMessage(t, `{"type":"offer","room_id":"CAM_001","device_code":"CAM_001","client_id":"device-CAM_001","to":"viewer-2","sdp":"v=0","sdp_type":"offer"}`)
	if !hub.HandleDevice("iot/webrtc/signal/backend", offer) {
		t.Fatal("offer 未被识别")
	}
	if f1.count() != 1 {
		t.Fatalf("viewer-1 不应收到定向 offer，实际 %d", f1.count())
	}
	if f2.count() != 2 {
		t.Fatalf("viewer-2 应收到 offer，实际 %d", f2.count())
	}
	f2.mu.Lock()
	got := f2.msgs[len(f2.msgs)-1]
	f2.mu.Unlock()
	if got.Type != TypeOffer || got.SDP != "v=0" {
		t.Fatalf("offer 内容不对: %+v", got)
	}
}

func TestReOfferAndCandidateRelay(t *testing.T) {
	pub := &fakePublisher{}
	hub := NewHub(pub)
	v, fv := newViewer("viewer-1", "CAM_001")
	if _, err := hub.Join(v); err != nil {
		t.Fatal(err)
	}
	// 设备首次 offer
	_ = hub.HandleDevice("iot/webrtc/signal/backend",
		deviceMessage(t, `{"type":"offer","room_id":"CAM_001","device_code":"CAM_001","to":"viewer-1","sdp":"offer-1"}`))
	// 设备重新 offer（ICE restart）
	_ = hub.HandleDevice("iot/webrtc/signal/backend",
		deviceMessage(t, `{"type":"offer","room_id":"CAM_001","device_code":"CAM_001","to":"viewer-1","sdp":"offer-2"}`))
	if fv.count() != 2 {
		t.Fatalf("重新 offer 应转发，实际 %d", fv.count())
	}

	// 查看方 answer + candidate 都应发到设备话题。
	if err := hub.ForwardFromViewer("viewer-1", &Message{Type: TypeAnswer, SDP: "answer-1"}); err != nil {
		t.Fatal(err)
	}
	if err := hub.ForwardFromViewer("viewer-1", &Message{
		Type:      TypeICECandidate,
		Candidate: &ICECandidate{Candidate: "candidate:1 1 udp ..."},
	}); err != nil {
		t.Fatal(err)
	}
	if len(pub.payloads) != 2 {
		t.Fatalf("应发布 2 条（answer + candidate），实际 %d", len(pub.payloads))
	}
	if pub.topic != "iot/webrtc/device/CAM_001/signal" {
		t.Fatalf("topic=%q", pub.topic)
	}
}

func TestNonRoomMessageIgnored(t *testing.T) {
	hub := NewHub(&fakePublisher{})
	whepOffer := []byte(`{"type":"offer","request_id":"r1","client_id":"app-1","stream":"ABC","sdp":"v=0"}`)
	if hub.HandleDevice("iot/webrtc/signal/backend", whepOffer) {

		t.Fatal("无 room_id 的 WHEP offer 不应被房间 hub 处理")
	}
}

func TestLeaveViewerNotifiesDevice(t *testing.T) {
	pub := &fakePublisher{}
	hub := NewHub(pub)
	v, _ := newViewer("viewer-1", "CAM_001")
	if _, err := hub.Join(v); err != nil {
		t.Fatal(err)
	}
	hub.LeaveViewer("viewer-1", "user_closed")
	_, msg, ok := pub.last()
	if !ok {
		t.Fatal("离开房间应通知设备")
	}
	if msg.Type != TypeLeaveRoom || msg.ClientID != "viewer-1" || msg.Reason != "user_closed" {
		t.Fatalf("leaveRoom 内容不对: %+v", msg)
	}
	rooms, viewers := hub.Stats()
	if rooms != 0 || viewers != 0 {
		t.Fatalf("空房间应回收，实际 rooms=%d viewers=%d", rooms, viewers)
	}
}
