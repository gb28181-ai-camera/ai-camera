package mqtt

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/gb28181/webrtc"
)

// ── 测试替身 ──────────────────────────────────────────────────

type recordedPublish struct {
	topic    string
	qos      byte
	retained bool
	payload  []byte
}

type fakePublisher struct {
	mu    sync.Mutex
	items []recordedPublish
	ch    chan recordedPublish
	err   error
}

func newFakePublisher() *fakePublisher {
	return &fakePublisher{ch: make(chan recordedPublish, 16)}
}

func (f *fakePublisher) Publish(topic string, qos byte, retained bool, payload []byte) error {
	cp := make([]byte, len(payload))
	copy(cp, payload)
	rec := recordedPublish{topic: topic, qos: qos, retained: retained, payload: cp}

	f.mu.Lock()
	f.items = append(f.items, rec)
	f.mu.Unlock()

	if f.err != nil {
		return f.err
	}
	select {
	case f.ch <- rec:
	default:
	}
	return nil
}

func (f *fakePublisher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.items)
}

func (f *fakePublisher) all() []recordedPublish {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedPublish(nil), f.items...)
}

type fakeAnswerer struct {
	mu     sync.Mutex
	answer string
	err    error
	gate   <-chan struct{}
	calls  int
	stream string
	offer  string
}

func (f *fakeAnswerer) Answer(ctx context.Context, stream, offer string) (string, error) {
	f.mu.Lock()
	f.calls++
	f.stream = stream
	f.offer = offer
	f.mu.Unlock()

	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return f.answer, f.err
}

func (f *fakeAnswerer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeAnswerer) lastStream() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stream
}

type fakeRegistry struct {
	byChannel map[int64]*play.Stream
	byStream  map[string]*play.Stream
}

func (r *fakeRegistry) GetByChannel(id int64) (*play.Stream, bool) {
	if r == nil {
		return nil, false
	}
	v, ok := r.byChannel[id]
	return v, ok
}

func (r *fakeRegistry) GetByStream(stream string) (*play.Stream, bool) {
	if r == nil {
		return nil, false
	}
	v, ok := r.byStream[stream]
	return v, ok
}

// ── 辅助函数 ──────────────────────────────────────────────────

func mustPayload(t *testing.T, m *Message) []byte {
	t.Helper()
	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("序列化消息失败: %v", err)
	}
	return data
}

func waitPublish(t *testing.T, pub *fakePublisher) recordedPublish {
	t.Helper()
	select {
	case rec := <-pub.ch:
		return rec
	case <-time.After(3 * time.Second):
		t.Fatal("等待 MQTT 发布超时")
		return recordedPublish{}
	}
}

func decodePublish(t *testing.T, rec recordedPublish) *Message {
	t.Helper()
	msg, err := ParseMessage(rec.payload)
	if err != nil {
		t.Fatalf("发布的报文不是合法 JSON: %v (%s)", err, rec.payload)
	}
	return msg
}

func newBridgeForTest(pub *fakePublisher, ans *fakeAnswerer, reg StreamRegistry) *Bridge {
	return NewBridgeWithOptions(pub, ans, reg, BridgeOptions{
		MaxInflight:   32,
		AnswerTimeout: 2 * time.Second,
		DedupTTL:      time.Minute,
	})
}

// ── 用例 ──────────────────────────────────────────────────────

// TestBridgeOfferPublishesAnswer offer → WHEP answer → App 话题。
func TestBridgeOfferPublishesAnswer(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0\r\no=zlm"}
	b := newBridgeForTest(pub, ans, nil)

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "flutter-1",
		ChannelID: 123, Stream: "0BEBC201", SDP: "v=0\r\no=app",
	}))

	rec := waitPublish(t, pub)
	if rec.topic != "iot/g1/app/flutter-1/signal" {
		t.Fatalf("answer 话题不正确: %q", rec.topic)
	}
	if rec.qos != DefaultQoS || rec.retained {
		t.Fatalf("answer QoS/retained 不正确: qos=%d retained=%v", rec.qos, rec.retained)
	}
	msg := decodePublish(t, rec)
	if msg.Type != TypeAnswer || msg.RequestID != "req-1" || msg.SDP != "v=0\r\no=zlm" {
		t.Fatalf("answer 内容不正确: %+v", msg)
	}
	if ans.lastStream() != "0BEBC201" {
		t.Fatalf("应把 stream 传给 WHEP，实际 %q", ans.lastStream())
	}
}

// TestBridgeResolvesStreamFromRegistry 只有 channel_id 时用注册表兜底。
func TestBridgeResolvesStreamFromRegistry(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0"}
	reg := &fakeRegistry{
		byChannel: map[int64]*play.Stream{123: {ChannelID: 123, Stream: "0BEBC201"}},
	}
	b := newBridgeForTest(pub, ans, reg)

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "c1", ChannelID: 123, SDP: "v=0",
	}))

	rec := waitPublish(t, pub)
	if decodePublish(t, rec).Type != TypeAnswer {
		t.Fatal("应返回 answer")
	}
	if ans.lastStream() != "0BEBC201" {
		t.Fatalf("应从注册表解析 stream，实际 %q", ans.lastStream())
	}
}

// TestBridgeStreamChannelMismatch 注册表存在但 channel 不符时必须拒绝。
func TestBridgeStreamChannelMismatch(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0"}
	reg := &fakeRegistry{
		byStream: map[string]*play.Stream{"0BEBC201": {ChannelID: 1, Stream: "0BEBC201"}},
	}
	b := newBridgeForTest(pub, ans, reg)

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "c1",
		ChannelID: 2, Stream: "0BEBC201", SDP: "v=0",
	}))

	msg := decodePublish(t, waitPublish(t, pub))
	if msg.Type != TypeError || msg.Code != CodeStreamNotFound {
		t.Fatalf("应返回 stream_not_found，实际 %+v", msg)
	}
	if ans.callCount() != 0 {
		t.Fatal("校验失败不应调用 WHEP")
	}
}

// TestBridgeStaleStreamForChannel 通道在播另一路流时，App 传来的旧 stream 必须拒绝。
func TestBridgeStaleStreamForChannel(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0"}
	reg := &fakeRegistry{
		byChannel: map[int64]*play.Stream{1: {ChannelID: 1, Stream: "CURRENT1"}},
	}
	b := newBridgeForTest(pub, ans, reg)

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "c1",
		ChannelID: 1, Stream: "STALE001", SDP: "v=0",
	}))

	msg := decodePublish(t, waitPublish(t, pub))
	if msg.Type != TypeError || msg.Code != CodeStreamNotFound {
		t.Fatalf("过期 stream 应返回 stream_not_found，实际 %+v", msg)
	}
	if ans.callCount() != 0 {
		t.Fatal("过期 stream 不应调用 WHEP")
	}
}

// TestBridgeStreamNotFound 既无 stream 也无法从 channel 解析。
func TestBridgeStreamNotFound(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0"}
	b := newBridgeForTest(pub, ans, &fakeRegistry{})

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "c1", ChannelID: 999, SDP: "v=0",
	}))

	msg := decodePublish(t, waitPublish(t, pub))
	if msg.Type != TypeError || msg.Code != CodeStreamNotFound || msg.RequestID != "req-1" {
		t.Fatalf("应返回 stream_not_found，实际 %+v", msg)
	}
}

// TestBridgeAnswererErrors WHEP 错误映射到约定错误码。
func TestBridgeAnswererErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code string
	}{
		{"流不存在", webrtc.ErrStreamNotFound, CodeStreamNotFound},
		{"协商失败", errors.New("connection refused"), CodeWhepFailed},
		{"超时", context.DeadlineExceeded, CodeTimeout},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pub := newFakePublisher()
			ans := &fakeAnswerer{err: c.err}
			b := newBridgeForTest(pub, ans, nil)

			b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
				Type: TypeOffer, RequestID: "req-1", ClientID: "c1", Stream: "s1", SDP: "v=0",
			}))

			msg := decodePublish(t, waitPublish(t, pub))
			if msg.Type != TypeError || msg.Code != c.code {
				t.Fatalf("错误码应为 %s，实际 %+v", c.code, msg)
			}
			// 不把内部错误细节（可能含内网地址）回传给 App。
			if len(msg.Message) == 0 {
				t.Fatal("error 应带可读提示")
			}
		})
	}
}

// TestBridgeDuplicateRequestID 同一 request_id 只回一次。
func TestBridgeDuplicateRequestID(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0"}
	b := newBridgeForTest(pub, ans, nil)

	payload := mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "same-id", ClientID: "c1", Stream: "s1", SDP: "v=0",
	})
	b.Handle(BackendSignalTopic("g1"), payload)
	waitPublish(t, pub)

	b.Handle(BackendSignalTopic("g1"), payload)
	time.Sleep(100 * time.Millisecond)

	if got := pub.count(); got != 1 {
		t.Fatalf("重复 request_id 只应发布一次，实际 %d 次", got)
	}
	if got := ans.callCount(); got != 1 {
		t.Fatalf("重复 request_id 只应协商一次，实际 %d 次", got)
	}
}

// TestBridgeCandidateIgnored 非 trickle 模式下候选消息直接忽略。
func TestBridgeCandidateIgnored(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0"}
	b := newBridgeForTest(pub, ans, nil)

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeCandidate, RequestID: "req-1", ClientID: "c1", SDP: "candidate:...",
	}))
	time.Sleep(100 * time.Millisecond)

	if pub.count() != 0 || ans.callCount() != 0 {
		t.Fatalf("candidate 应被忽略: publish=%d answer=%d", pub.count(), ans.callCount())
	}
}

// TestBridgeUnsupportedType 非 offer 消息回 unsupported_type。
func TestBridgeUnsupportedType(t *testing.T) {
	pub := newFakePublisher()
	b := newBridgeForTest(pub, &fakeAnswerer{}, nil)

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: "bye", RequestID: "req-1", ClientID: "c1",
	}))
	msg := decodePublish(t, waitPublish(t, pub))
	if msg.Type != TypeError || msg.Code != CodeUnsupportedType {
		t.Fatalf("应返回 unsupported_type，实际 %+v", msg)
	}
}

// TestBridgeInvalidOffer 缺字段时回 invalid_request。
func TestBridgeInvalidOffer(t *testing.T) {
	pub := newFakePublisher()
	b := newBridgeForTest(pub, &fakeAnswerer{}, nil)

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "c1", Stream: "s1", // 缺 SDP
	}))
	msg := decodePublish(t, waitPublish(t, pub))
	if msg.Type != TypeError || msg.Code != CodeInvalidRequest {
		t.Fatalf("应返回 invalid_request，实际 %+v", msg)
	}
}

// TestBridgeBusy 在途请求超限时立即回 busy。
func TestBridgeBusy(t *testing.T) {
	pub := newFakePublisher()
	gate := make(chan struct{})
	ans := &fakeAnswerer{answer: "v=0", gate: gate}
	b := NewBridgeWithOptions(pub, ans, nil, BridgeOptions{
		MaxInflight: 1, AnswerTimeout: 2 * time.Second, DedupTTL: time.Minute,
	})

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "c1", Stream: "s1", SDP: "v=0",
	}))
	// 等第一个请求真正进入 Answer，确保信号量已被占用。
	deadline := time.Now().Add(time.Second)
	for ans.callCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if ans.callCount() == 0 {
		t.Fatal("第一个请求未进入 WHEP 协商")
	}

	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-2", ClientID: "c2", Stream: "s2", SDP: "v=0",
	}))
	msg := decodePublish(t, waitPublish(t, pub))
	if msg.Type != TypeError || msg.Code != CodeBusy || msg.RequestID != "req-2" {
		t.Fatalf("应返回 busy，实际 %+v", msg)
	}

	close(gate)
	// 等待第一个请求收尾，避免 Close 在测试结束后仍等待。
	waitPublish(t, pub)
}

// TestBridgeCloseStopsNewRequests Close 后不再受理新 offer。
func TestBridgeCloseStopsNewRequests(t *testing.T) {
	pub := newFakePublisher()
	ans := &fakeAnswerer{answer: "v=0"}
	b := newBridgeForTest(pub, ans, nil)

	b.Close(context.Background())
	b.Handle(BackendSignalTopic("g1"), mustPayload(t, &Message{
		Type: TypeOffer, RequestID: "req-1", ClientID: "c1", Stream: "s1", SDP: "v=0",
	}))
	time.Sleep(100 * time.Millisecond)

	if pub.count() != 0 || ans.callCount() != 0 {
		t.Fatalf("Close 后不应再处理请求: publish=%d answer=%d", pub.count(), ans.callCount())
	}
}

// TestBridgeInvalidTopic 非法话题不应 panic，也不应发布。
func TestBridgeInvalidTopic(t *testing.T) {
	pub := newFakePublisher()
	b := newBridgeForTest(pub, &fakeAnswerer{}, nil)

	b.Handle("iot/g1/other/topic", mustPayload(t, &Message{Type: TypeOffer, RequestID: "r"}))
	b.Handle("", []byte("not-json"))
	time.Sleep(50 * time.Millisecond)

	if pub.count() != 0 {
		t.Fatalf("非法话题不应发布任何消息，实际 %d 条", pub.count())
	}
}
