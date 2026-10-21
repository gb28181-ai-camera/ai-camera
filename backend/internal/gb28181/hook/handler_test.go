package hook

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ── 测试替身 ──────────────────────────────────────────────────

type fakeSecretStore struct {
	valid string
	err   error
}

func (f fakeSecretStore) IsValidHookSecret(_ context.Context, secret string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return secret == f.valid, nil
}

type fakeSink struct {
	keepalive   *MediaServerEvent
	started     *MediaServerEvent
	publish     *PublishEvent
	play        *PlayEvent
	changed     *StreamChangedEvent
	noneReader  *StreamEvent
	notFound    *StreamEvent
	sendStopped *SendRTPStoppedEvent
	rtpTimeout  *StreamEvent
	record      *RecordMP4Event

	publishAllowed bool
	playAllowed    bool
	publishErr     error
}

func (f *fakeSink) OnServerKeepalive(_ context.Context, e *MediaServerEvent) error {
	f.keepalive = e
	return nil
}

func (f *fakeSink) OnServerStarted(_ context.Context, e *MediaServerEvent) error {
	f.started = e
	return nil
}

func (f *fakeSink) OnPublish(_ context.Context, e *PublishEvent) (bool, error) {
	f.publish = e
	return f.publishAllowed, f.publishErr
}

func (f *fakeSink) OnPlay(_ context.Context, e *PlayEvent) (bool, error) {
	f.play = e
	return f.playAllowed, nil
}

func (f *fakeSink) OnStreamChanged(_ context.Context, e *StreamChangedEvent) error {
	f.changed = e
	return nil
}

func (f *fakeSink) OnStreamNoneReader(_ context.Context, e *StreamEvent) error {
	f.noneReader = e
	return nil
}

func (f *fakeSink) OnStreamNotFound(_ context.Context, e *StreamEvent) error {
	f.notFound = e
	return nil
}

func (f *fakeSink) OnSendRTPStopped(_ context.Context, e *SendRTPStoppedEvent) error {
	f.sendStopped = e
	return nil
}

func (f *fakeSink) OnRTPServerTimeout(_ context.Context, e *StreamEvent) error {
	f.rtpTimeout = e
	return nil
}

func (f *fakeSink) OnRecordMP4(_ context.Context, e *RecordMP4Event) error {
	f.record = e
	return nil
}

// newTestRouter 构造与生产一致的回调路由（不带任何 JWT/Casbin 中间件）。
func newTestRouter(sink Sink, secrets SecretStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(sink, secrets)
	r := gin.New()
	g := r.Group("/media/hook")
	g.POST("/on_server_keepalive", h.OnServerKeepalive)
	g.POST("/on_server_started", h.OnServerStarted)
	g.POST("/on_publish", h.OnPublish)
	g.POST("/on_play", h.OnPlay)
	g.POST("/on_stream_changed", h.OnStreamChanged)
	g.POST("/on_stream_none_reader", h.OnStreamNoneReader)
	g.POST("/on_stream_not_found", h.OnStreamNotFound)
	g.POST("/on_send_rtp_stopped", h.OnSendRTPStopped)
	g.POST("/on_rtp_server_timeout", h.OnRTPServerTimeout)
	g.POST("/on_record_mp4", h.OnRecordMP4)
	return r
}

func doPost(t *testing.T, r *gin.Engine, path, secret, body string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path+"?secret="+secret, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func decodeResult(t *testing.T, body []byte) Result {
	t.Helper()
	var res Result
	if err := json.Unmarshal(body, &res); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, body)
	}
	return res
}

// ── 鉴权 ──────────────────────────────────────────────────────

func TestSecretRejectedOnEveryEndpoint(t *testing.T) {
	paths := []string{
		"/media/hook/on_server_keepalive",
		"/media/hook/on_server_started",
		"/media/hook/on_publish",
		"/media/hook/on_play",
		"/media/hook/on_stream_changed",
		"/media/hook/on_stream_none_reader",
		"/media/hook/on_stream_not_found",
		"/media/hook/on_send_rtp_stopped",
		"/media/hook/on_rtp_server_timeout",
		"/media/hook/on_record_mp4",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			sink := &fakeSink{publishAllowed: true, playAllowed: true}
			r := newTestRouter(sink, fakeSecretStore{valid: "good"})

			code, body := doPost(t, r, path, "bad", `{}`)
			if code != http.StatusOK {
				t.Errorf("HTTP status = %d, want 200 (ZLM 只认 200)", code)
			}
			res := decodeResult(t, body)
			if res.Code != -1 || res.Msg != "invalid secret" {
				t.Errorf("response = %+v, want code=-1 msg=invalid secret", res)
			}
			// 鉴权失败不能触达业务。
			if sink.keepalive != nil || sink.publish != nil || sink.changed != nil {
				t.Error("sink should not be called when secret is invalid")
			}
		})
	}
}

func TestSecretStoreErrorReturns200(t *testing.T) {
	r := newTestRouter(&fakeSink{}, fakeSecretStore{err: errors.New("db down")})
	code, body := doPost(t, r, "/media/hook/on_server_keepalive", "x", `{}`)
	if code != http.StatusOK {
		t.Errorf("HTTP status = %d, want 200", code)
	}
	res := decodeResult(t, body)
	if res.Code != -1 {
		t.Errorf("response = %+v, want code=-1", res)
	}
}

func TestMalformedJSON(t *testing.T) {
	r := newTestRouter(&fakeSink{}, fakeSecretStore{valid: "good"})
	code, body := doPost(t, r, "/media/hook/on_stream_changed", "good", `{`)
	if code != http.StatusOK {
		t.Errorf("HTTP status = %d, want 200", code)
	}
	res := decodeResult(t, body)
	if res.Code != -1 || res.Msg != "invalid json" {
		t.Errorf("response = %+v", res)
	}
}

// ── 每个端点的 JSON 解析 ─────────────────────────────────────

func TestAllEndpointsParse(t *testing.T) {
	sink := &fakeSink{publishAllowed: true, playAllowed: true}
	r := newTestRouter(sink, fakeSecretStore{valid: "good"})

	t.Run("on_server_keepalive", func(t *testing.T) {
		_, body := doPost(t, r, "/media/hook/on_server_keepalive", "good",
			`{"mediaServerId":"zlm-1","data":{"general.mediaServerId":"zlm-1"}}`)
		if res := decodeResult(t, body); res.Code != 0 {
			t.Fatalf("response = %+v", res)
		}
		if sink.keepalive == nil || sink.keepalive.MediaServerID != "zlm-1" {
			t.Fatalf("keepalive event = %+v", sink.keepalive)
		}
		if sink.keepalive.Data["general.mediaServerId"] != "zlm-1" {
			t.Errorf("keepalive data not parsed: %+v", sink.keepalive.Data)
		}
		if sink.keepalive.ReceivedAt.IsZero() {
			t.Error("ReceivedAt should be set")
		}
	})

	t.Run("on_server_started", func(t *testing.T) {
		_, body := doPost(t, r, "/media/hook/on_server_started", "good",
			`{"mediaServerId":"zlm-1","general.mediaServerId":"zlm-1","http.port":"80"}`)
		if res := decodeResult(t, body); res.Code != 0 {
			t.Fatalf("response = %+v", res)
		}
		if sink.started == nil || sink.started.MediaServerID != "zlm-1" {
			t.Fatalf("started event = %+v", sink.started)
		}
		if sink.started.Config["http.port"] != "80" {
			t.Errorf("config not parsed: %+v", sink.started.Config)
		}
	})

	t.Run("on_server_started_general_id_fallback", func(t *testing.T) {
		sink.started = nil
		_, _ = doPost(t, r, "/media/hook/on_server_started", "good",
			`{"general.mediaServerId":"zlm-2"}`)
		if sink.started == nil || sink.started.MediaServerID != "zlm-2" {
			t.Fatalf("general.mediaServerId fallback failed: %+v", sink.started)
		}
	})

	t.Run("on_publish", func(t *testing.T) {
		_, body := doPost(t, r, "/media/hook/on_publish", "good",
			`{"mediaServerId":"zlm-1","app":"rtp","stream":"3B9ACA01","schema":"rtsp","port":10000}`)
		if res := decodeResult(t, body); res.Code != 0 {
			t.Fatalf("response = %+v", res)
		}
		if sink.publish == nil || sink.publish.App != "rtp" || sink.publish.Stream != "3B9ACA01" || sink.publish.Port != 10000 {
			t.Fatalf("publish event = %+v", sink.publish)
		}
	})

	t.Run("on_play", func(t *testing.T) {
		_, body := doPost(t, r, "/media/hook/on_play", "good",
			`{"mediaServerId":"zlm-1","app":"rtp","stream":"3B9ACA01","schema":"http-flv"}`)
		if res := decodeResult(t, body); res.Code != 0 {
			t.Fatalf("response = %+v", res)
		}
		if sink.play == nil || sink.play.Schema != "http-flv" {
			t.Fatalf("play event = %+v", sink.play)
		}
	})

	t.Run("on_stream_changed", func(t *testing.T) {
		_, body := doPost(t, r, "/media/hook/on_stream_changed", "good",
			`{"mediaServerId":"zlm-1","regist":true,"app":"rtp","stream":"3B9ACA01","schema":"rtsp",
			  "callId":"call-1","originSock":{"identifier":"3B9ACA01","local_port":10000},
			  "tracks":[{"codec_id_name":"H264","width":512,"height":384}]}`)
		if res := decodeResult(t, body); res.Code != 0 {
			t.Fatalf("response = %+v", res)
		}
		e := sink.changed
		if e == nil || !e.Regist || e.CallID != "call-1" || e.OriginSock == nil || e.OriginSock.LocalPort != 10000 {
			t.Fatalf("changed event = %+v", e)
		}
		if len(e.Tracks) != 1 || e.Tracks[0].Width != 512 {
			t.Errorf("tracks not parsed: %+v", e.Tracks)
		}
	})

	t.Run("on_stream_none_reader", func(t *testing.T) {
		_, body := doPost(t, r, "/media/hook/on_stream_none_reader", "good",
			`{"mediaServerId":"zlm-1","app":"rtp","stream":"3B9ACA01","schema":"rtsp"}`)
		var res NoneReaderResult
		if err := json.Unmarshal(body, &res); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		if res.Code != 0 || res.Close {
			t.Errorf("response = %+v, want code=0 close=false", res)
		}
		if sink.noneReader == nil || sink.noneReader.Stream != "3B9ACA01" {
			t.Fatalf("noneReader event = %+v", sink.noneReader)
		}
	})

	t.Run("on_stream_not_found", func(t *testing.T) {
		_, _ = doPost(t, r, "/media/hook/on_stream_not_found", "good",
			`{"mediaServerId":"zlm-1","app":"rtp","stream":"S","schema":"rtsp"}`)
		if sink.notFound == nil || sink.notFound.Stream != "S" {
			t.Fatalf("notFound event = %+v", sink.notFound)
		}
	})

	t.Run("on_send_rtp_stopped", func(t *testing.T) {
		_, _ = doPost(t, r, "/media/hook/on_send_rtp_stopped", "good",
			`{"mediaServerId":"zlm-1","app":"rtp","stream":"S"}`)
		if sink.sendStopped == nil || sink.sendStopped.Stream != "S" {
			t.Fatalf("sendStopped event = %+v", sink.sendStopped)
		}
	})

	t.Run("on_rtp_server_timeout", func(t *testing.T) {
		_, _ = doPost(t, r, "/media/hook/on_rtp_server_timeout", "good",
			`{"mediaServerId":"zlm-1","stream_id":"3B9ACA01","local_port":10000,"tcpMode":0,"ssrc":"1000000001"}`)
		e := sink.rtpTimeout
		if e == nil || e.StreamID != "3B9ACA01" || e.SSRC != "1000000001" || e.LocalPort != 10000 {
			t.Fatalf("rtpTimeout event = %+v", e)
		}
		// app 由 handler 补齐为 rtp（单端口收流）。
		if e.App != "rtp" || e.Stream != "3B9ACA01" {
			t.Errorf("app/stream not normalized: %+v", e)
		}
	})

	t.Run("on_record_mp4", func(t *testing.T) {
		_, _ = doPost(t, r, "/media/hook/on_record_mp4", "good",
			`{"mediaServerId":"zlm-1","app":"rtp","stream":"S","file_name":"a.mp4","file_path":"/rec/a.mp4",
			  "file_size":12345,"time_len":10.5,"start_time":1700000000}`)
		e := sink.record
		if e == nil || e.FilePath != "/rec/a.mp4" || e.FileSize != 12345 || e.TimeLen != 10.5 {
			t.Fatalf("record event = %+v", e)
		}
	})
}

// ── 推流/播放拒绝语义 ────────────────────────────────────────

func TestPublishRejectedReturnsCodeMinusOne(t *testing.T) {
	sink := &fakeSink{publishAllowed: false}
	r := newTestRouter(sink, fakeSecretStore{valid: "good"})

	code, body := doPost(t, r, "/media/hook/on_publish", "good",
		`{"mediaServerId":"zlm-1","app":"rtp","stream":"UNKNOWN"}`)
	if code != http.StatusOK {
		t.Errorf("HTTP status = %d, want 200", code)
	}
	res := decodeResult(t, body)
	if res.Code != -1 {
		t.Errorf("response = %+v, want code=-1", res)
	}
}

func TestPublishSinkErrorFailsClosed(t *testing.T) {
	sink := &fakeSink{publishAllowed: true, publishErr: errors.New("redis down")}
	r := newTestRouter(sink, fakeSecretStore{valid: "good"})

	_, body := doPost(t, r, "/media/hook/on_publish", "good",
		`{"mediaServerId":"zlm-1","app":"rtp","stream":"S"}`)
	if res := decodeResult(t, body); res.Code != -1 {
		t.Errorf("response = %+v, want code=-1 (fail-closed)", res)
	}
}

func TestPlayRejectedReturnsCodeMinusOne(t *testing.T) {
	r := newTestRouter(&fakeSink{playAllowed: false}, fakeSecretStore{valid: "good"})
	_, body := doPost(t, r, "/media/hook/on_play", "good",
		`{"mediaServerId":"zlm-1","app":"rtp","stream":"S"}`)
	if res := decodeResult(t, body); res.Code != -1 {
		t.Errorf("response = %+v, want code=-1", res)
	}
}

// ── DefaultSink ──────────────────────────────────────────────

type fakeMediaServerStore struct {
	calls    int
	serverID string
	at       time.Time
	err      error
}

func (f *fakeMediaServerStore) MarkOnline(_ context.Context, serverID string, at time.Time) error {
	f.calls++
	f.serverID = serverID
	f.at = at
	return f.err
}

func TestDefaultSinkUpdatesMediaServer(t *testing.T) {
	store := &fakeMediaServerStore{}
	sink := NewDefaultSinkWithStore(store)
	fixed := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	sink.now = func() time.Time { return fixed }

	if err := sink.OnServerKeepalive(context.Background(), &MediaServerEvent{MediaServerID: "zlm-1"}); err != nil {
		t.Fatalf("OnServerKeepalive: %v", err)
	}
	if store.calls != 1 || store.serverID != "zlm-1" || !store.at.Equal(fixed) {
		t.Fatalf("store = %+v", store)
	}

	if err := sink.OnServerStarted(context.Background(), &MediaServerEvent{MediaServerID: "zlm-2"}); err != nil {
		t.Fatalf("OnServerStarted: %v", err)
	}
	if store.calls != 2 || store.serverID != "zlm-2" {
		t.Fatalf("store = %+v", store)
	}

	// 缺少 mediaServerId 时不应写库。
	if err := sink.OnServerKeepalive(context.Background(), &MediaServerEvent{}); err != nil {
		t.Fatalf("OnServerKeepalive(empty): %v", err)
	}
	if store.calls != 2 {
		t.Errorf("store should not be called without mediaServerId, calls = %d", store.calls)
	}
}

func TestDefaultSinkActiveStreams(t *testing.T) {
	sink := NewDefaultSinkWithStore(&fakeMediaServerStore{})

	if err := sink.OnStreamChanged(context.Background(), &StreamChangedEvent{
		MediaServerID: "zlm-1",
		Regist:        true,
		App:           "rtp",
		Stream:        "3B9ACA01",
		Schema:        "rtsp",
		Tracks:        []Track{{CodecIDName: "H264"}},
	}); err != nil {
		t.Fatalf("OnStreamChanged(regist): %v", err)
	}
	if sink.ActiveStreams().Len() != 1 {
		t.Fatalf("active streams = %d, want 1", sink.ActiveStreams().Len())
	}
	got, ok := sink.ActiveStreams().Get("rtp", "3B9ACA01")
	if !ok || len(got.Tracks) != 1 || got.Tracks[0].CodecIDName != "H264" {
		t.Fatalf("active stream = %+v, ok=%v", got, ok)
	}

	if err := sink.OnStreamChanged(context.Background(), &StreamChangedEvent{
		MediaServerID: "zlm-1", Regist: false, App: "rtp", Stream: "3B9ACA01",
	}); err != nil {
		t.Fatalf("OnStreamChanged(unregist): %v", err)
	}
	if sink.ActiveStreams().Len() != 0 {
		t.Fatalf("active streams = %d, want 0", sink.ActiveStreams().Len())
	}
}

func TestDefaultSinkPublishAndPlayAllowByDefault(t *testing.T) {
	sink := NewDefaultSinkWithStore(&fakeMediaServerStore{})

	allowed, err := sink.OnPublish(context.Background(), &PublishEvent{App: "rtp", Stream: "S"})
	if err != nil || !allowed {
		t.Errorf("OnPublish = (%v, %v), want (true, nil)", allowed, err)
	}
	allowed, err = sink.OnPlay(context.Background(), &PlayEvent{App: "rtp", Stream: "S"})
	if err != nil || !allowed {
		t.Errorf("OnPlay = (%v, %v), want (true, nil)", allowed, err)
	}
}

func TestDefaultSinkRTPTimeoutCleansActiveStream(t *testing.T) {
	sink := NewDefaultSinkWithStore(&fakeMediaServerStore{})
	sink.ActiveStreams().Set(ActiveStream{App: "rtp", Stream: "3B9ACA01"})

	if err := sink.OnRTPServerTimeout(context.Background(), &StreamEvent{
		MediaServerID: "zlm-1", App: "rtp", StreamID: "3B9ACA01", SSRC: "1000000001",
	}); err != nil {
		t.Fatalf("OnRTPServerTimeout: %v", err)
	}
	if sink.ActiveStreams().Len() != 0 {
		t.Errorf("active streams = %d, want 0 after timeout", sink.ActiveStreams().Len())
	}
}

func TestDefaultSinkHandlesNilEvents(t *testing.T) {
	sink := NewDefaultSinkWithStore(&fakeMediaServerStore{})
	ctx := context.Background()

	if err := sink.OnStreamNoneReader(ctx, nil); err != nil {
		t.Errorf("OnStreamNoneReader(nil): %v", err)
	}
	if err := sink.OnStreamNotFound(ctx, nil); err != nil {
		t.Errorf("OnStreamNotFound(nil): %v", err)
	}
	if err := sink.OnSendRTPStopped(ctx, nil); err != nil {
		t.Errorf("OnSendRTPStopped(nil): %v", err)
	}
	if err := sink.OnRTPServerTimeout(ctx, nil); err != nil {
		t.Errorf("OnRTPServerTimeout(nil): %v", err)
	}
	if err := sink.OnRecordMP4(ctx, nil); err != nil {
		t.Errorf("OnRecordMP4(nil): %v", err)
	}
}
