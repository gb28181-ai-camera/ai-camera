package mqtt

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestMessageRoundTrip 校验 offer/answer/error 三种报文的 JSON 字段契约。
func TestMessageRoundTrip(t *testing.T) {
	offer := &Message{
		Type:      TypeOffer,
		RequestID: "req-1",
		ClientID:  "flutter-1",
		ChannelID: 123,
		Stream:    "0BEBC201",
		SDP:       "v=0\r\n...",
	}
	data, err := offer.Marshal()
	if err != nil {
		t.Fatalf("序列化 offer 失败: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("offer 不是合法 JSON: %v", err)
	}
	for _, key := range []string{"type", "request_id", "client_id", "channel_id", "stream", "sdp"} {
		if _, ok := raw[key]; !ok {
			t.Fatalf("offer 缺少字段 %q: %s", key, data)
		}
	}

	got, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("解析 offer 失败: %v", err)
	}
	if got.Type != TypeOffer || got.RequestID != "req-1" || got.ClientID != "flutter-1" ||
		got.ChannelID != 123 || got.Stream != "0BEBC201" || got.SDP != "v=0\r\n..." {
		t.Fatalf("offer 往返不一致: %+v", got)
	}

	answerData, err := NewAnswer("req-1", "v=0\r\nanswer").Marshal()
	if err != nil {
		t.Fatalf("序列化 answer 失败: %v", err)
	}
	var answer map[string]any
	if err := json.Unmarshal(answerData, &answer); err != nil {
		t.Fatalf("answer 不是合法 JSON: %v", err)
	}
	if answer["type"] != TypeAnswer || answer["sdp"] != "v=0\r\nanswer" {
		t.Fatalf("answer 字段不正确: %s", answerData)
	}
	if _, ok := answer["client_id"]; ok {
		t.Fatalf("answer 不应携带 client_id: %s", answerData)
	}

	errData, err := NewError("req-1", CodeWhepFailed, "媒体协商失败").Marshal()
	if err != nil {
		t.Fatalf("序列化 error 失败: %v", err)
	}
	var em map[string]any
	if err := json.Unmarshal(errData, &em); err != nil {
		t.Fatalf("error 不是合法 JSON: %v", err)
	}
	if em["type"] != TypeError || em["code"] != CodeWhepFailed || em["message"] != "媒体协商失败" {
		t.Fatalf("error 字段不正确: %s", errData)
	}
	if _, ok := em["sdp"]; ok {
		t.Fatalf("error 不应携带 sdp: %s", errData)
	}
}

// TestParseMessageInvalid 空体、坏 JSON 必须返回 ErrInvalidMessage。
func TestParseMessageInvalid(t *testing.T) {
	for _, data := range [][]byte{nil, {}, []byte("{"), []byte(`{"type":`)} {
		if _, err := ParseMessage(data); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("输入 %q 应返回 ErrInvalidMessage，实际 %v", data, err)
		}
	}
}

// TestParseBackendTopic 提取 group_code，并拒绝越界/非法层级。
func TestParseBackendTopic(t *testing.T) {
	cases := []struct {
		topic string
		group string
		ok    bool
	}{
		{"iot/g1/signal/backend", "g1", true},
		{"iot/iot-webrtc-01/signal/backend", "iot-webrtc-01", true},
		{"iot/g1/signal/backend/extra", "", false},
		{"iot/g1/signal/frontend", "", false},
		{"iot//signal/backend", "", false},
		{"iot/g+/signal/backend", "", false},
		{"iot/g1/app/c1/signal", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		group, ok := ParseBackendTopic(c.topic)
		if ok != c.ok || group != c.group {
			t.Fatalf("ParseBackendTopic(%q) = (%q,%v)，期望 (%q,%v)", c.topic, group, ok, c.group, c.ok)
		}
	}
}

// TestAppSignalTopic client_id 不得越出话题层级。
func TestAppSignalTopic(t *testing.T) {
	topic, err := AppSignalTopic("g1", "flutter-1")
	if err != nil || topic != "iot/g1/app/flutter-1/signal" {
		t.Fatalf("构造话题失败: topic=%q err=%v", topic, err)
	}
	if BackendSignalTopic("g1") != "iot/g1/signal/backend" {
		t.Fatalf("后端话题不正确: %q", BackendSignalTopic("g1"))
	}

	for _, c := range []struct{ group, client string }{
		{"", "c1"},
		{"g1", ""},
		{"g/1", "c1"},
		{"g1", "c/1"},
		{"g1", "c+"},
		{"g1", "c#"},
		{"g1", strings.Repeat("x", maxClientIDLen+1)},
	} {
		if _, err := AppSignalTopic(c.group, c.client); !errors.Is(err, ErrInvalidTopic) {
			t.Fatalf("group=%q client=%q 应返回 ErrInvalidTopic，实际 %v", c.group, c.client, err)
		}
	}
}

// TestValidateOffer 必填字段与长度限制。
func TestValidateOffer(t *testing.T) {
	valid := &Message{
		Type: TypeOffer, RequestID: "req", ClientID: "c1", Stream: "0BEBC201", SDP: "v=0",
	}
	if err := valid.ValidateOffer(); err != nil {
		t.Fatalf("合法 offer 不应报错: %v", err)
	}

	byChannel := &Message{Type: TypeOffer, RequestID: "req", ClientID: "c1", ChannelID: 7, SDP: "v=0"}
	if err := byChannel.ValidateOffer(); err != nil {
		t.Fatalf("仅 channel_id 的 offer 应合法: %v", err)
	}

	cases := map[string]*Message{
		"空消息":          nil,
		"类型错误":         {Type: TypeAnswer, RequestID: "r", ClientID: "c", Stream: "s", SDP: "v=0"},
		"缺 request_id": {Type: TypeOffer, ClientID: "c", Stream: "s", SDP: "v=0"},
		"缺 client_id":  {Type: TypeOffer, RequestID: "r", Stream: "s", SDP: "v=0"},
		"缺流":           {Type: TypeOffer, RequestID: "r", ClientID: "c", SDP: "v=0"},
		"缺 sdp":        {Type: TypeOffer, RequestID: "r", ClientID: "c", Stream: "s", SDP: " "},
		"client 含斜杠":   {Type: TypeOffer, RequestID: "r", ClientID: "a/b", Stream: "s", SDP: "v=0"},
		"stream 含控制符":  {Type: TypeOffer, RequestID: "r", ClientID: "c", Stream: "s\x00", SDP: "v=0"},
	}
	for name, msg := range cases {
		if err := msg.ValidateOffer(); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("%s 应返回 ErrInvalidMessage，实际 %v", name, err)
		}
	}
}
