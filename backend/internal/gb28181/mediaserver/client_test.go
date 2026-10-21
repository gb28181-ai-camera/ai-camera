package mediaserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// newTestClient 启动一个 httptest.Server 并返回客户端。
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewWithHTTPClient(srv.URL, "test-secret", srv.Client()), srv
}

func TestGetMediaListSuccess(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index/api/getMediaList" {
			t.Errorf("path = %s, want /index/api/getMediaList", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.PostFormValue("secret"); got != "test-secret" {
			t.Errorf("secret = %q", got)
		}
		if got := r.PostFormValue("app"); got != "rtp" {
			t.Errorf("app = %q", got)
		}
		if got := r.PostFormValue("stream"); got != "3B9ACA01" {
			t.Errorf("stream = %q", got)
		}
		if got := r.PostFormValue("schema"); got != "rtsp" {
			t.Errorf("schema = %q", got)
		}
		if got := r.PostFormValue("vhost"); got != DefaultVhost {
			t.Errorf("vhost = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":[
			{"app":"rtp","stream":"3B9ACA01","schema":"rtsp","originSock":{"identifier":"3B9ACA01","local_port":10000},
			 "tracks":[{"codec_id_name":"H264","width":512,"height":384,"fps":25}]}
		]}`))
	})

	items, err := client.GetMediaList(context.Background(), "rtp", "3B9ACA01", "rtsp", "")
	if err != nil {
		t.Fatalf("GetMediaList: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	got := items[0]
	if got.Stream != "3B9ACA01" || got.App != "rtp" {
		t.Errorf("unexpected media item: %+v", got)
	}
	if got.OriginSock == nil || got.OriginSock.LocalPort != 10000 {
		t.Errorf("originSock not parsed: %+v", got.OriginSock)
	}
	if len(got.Tracks) != 1 || got.Tracks[0].CodecIDName != "H264" || got.Tracks[0].Width != 512 {
		t.Errorf("tracks not parsed: %+v", got.Tracks)
	}
}

func TestAPIError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":-1,"msg":"stream not found"}`))
	})

	_, err := client.GetRtpInfo(context.Background(), "abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := IsAPIError(err)
	if !ok {
		t.Fatalf("IsAPIError = false, err = %v", err)
	}
	if apiErr.Code != -1 || apiErr.Msg != "stream not found" || apiErr.API != "getRtpInfo" {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
}

func TestHTTPError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	})

	_, err := client.GetServerConfig(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "HTTP 502") {
		t.Errorf("error should mention HTTP status: %v", err)
	}
	if _, ok := IsAPIError(err); ok {
		t.Errorf("HTTP error should not be APIError: %v", err)
	}
}

func TestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
	}))
	defer srv.Close()

	client := NewWithHTTPClient(srv.URL, "test-secret", &http.Client{Timeout: 20 * time.Millisecond})
	start := time.Now()
	_, err := client.GetMediaList(context.Background(), "rtp", "x", "", "")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Errorf("timeout not enforced, elapsed = %v", elapsed)
	}
}

func TestContextCanceled(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetServerConfig(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should wrap context.Canceled: %v", err)
	}
}

func TestFromModel(t *testing.T) {
	ms := &model.GbMediaServer{IP: "10.0.0.5", HTTPPort: 8080, Secret: "abc"}
	c := FromModel(ms)
	if c.BaseURL != "http://10.0.0.5:8080" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
	if c.Secret != "abc" {
		t.Errorf("Secret = %q", c.Secret)
	}

	// 端口为 0 回退 80；IP 为空回退 127.0.0.1。
	c2 := FromModel(&model.GbMediaServer{})
	if c2.BaseURL != "http://127.0.0.1:80" {
		t.Errorf("fallback BaseURL = %q", c2.BaseURL)
	}

	c3 := FromModel(nil)
	if c3.BaseURL != "http://127.0.0.1:80" {
		t.Errorf("nil model BaseURL = %q", c3.BaseURL)
	}

	// 已带协议的地址不重复拼协议。
	c4 := FromModel(&model.GbMediaServer{IP: "https://zlm.example.com", HTTPPort: 443})
	if c4.BaseURL != "https://zlm.example.com:443" {
		t.Errorf("scheme-aware BaseURL = %q", c4.BaseURL)
	}
}

func TestBuildPlayURLs(t *testing.T) {
	ms := &model.GbMediaServer{
		IP:       "10.0.0.5",
		StreamIP: "1.2.3.4",
		HTTPPort: 80,
		RTSPPort: 554,
		RTMPPort: 1935,
	}
	urls := BuildPlayURLs(ms, "rtp", "3B9ACA01")

	want := map[string]string{
		"webrtc": "http://1.2.3.4:80/index/api/webrtc?app=rtp&stream=3B9ACA01&type=play",
		"flv":    "http://1.2.3.4:80/rtp/3B9ACA01.live.flv",
		"hls":    "http://1.2.3.4:80/rtp/3B9ACA01/hls.m3u8",
		"ws_flv": "ws://1.2.3.4:80/rtp/3B9ACA01.live.flv",
		"rtsp":   "rtsp://1.2.3.4:554/rtp/3B9ACA01",
		"rtmp":   "rtmp://1.2.3.4:1935/rtp/3B9ACA01",
	}
	got := map[string]string{
		"webrtc": urls.WebRTC, "flv": urls.FLV, "hls": urls.HLS,
		"ws_flv": urls.WSFLV, "rtsp": urls.RTSP, "rtmp": urls.RTMP,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestBuildPlayURLsPortFallback(t *testing.T) {
	// FLV/WS-FLV 独立端口为 0 时回退 http_port；http_port 也为 0 时回退 80。
	urls := BuildPlayURLs(&model.GbMediaServer{IP: "10.0.0.5", FLVPort: 0, WSFLVPort: 0, HTTPPort: 0}, "rtp", "AB")
	if !strings.Contains(urls.FLV, ":80/") {
		t.Errorf("FLV should fall back to http_port(80): %q", urls.FLV)
	}
	if !strings.Contains(urls.WSFLV, ":80/") {
		t.Errorf("WSFLV should fall back to http_port(80): %q", urls.WSFLV)
	}
	// RTSP/RTMP 未配置端口时留空，不能回退到 http 端口。
	if urls.RTSP != "" || urls.RTMP != "" {
		t.Errorf("RTSP/RTMP should be empty when ports are 0: %q %q", urls.RTSP, urls.RTMP)
	}

	// 独立 FLV 端口优先。
	urls2 := BuildPlayURLs(&model.GbMediaServer{IP: "10.0.0.5", HTTPPort: 80, FLVPort: 8080, WSFLVPort: 8081}, "rtp", "AB")
	if !strings.Contains(urls2.FLV, ":8080/") {
		t.Errorf("FLV should use flv_port: %q", urls2.FLV)
	}
	if !strings.Contains(urls2.WSFLV, ":8081/") {
		t.Errorf("WSFLV should use ws_flv_port: %q", urls2.WSFLV)
	}
	if urls2.RTSP != "" || urls2.RTMP != "" {
		t.Errorf("RTSP/RTMP should be empty: %q %q", urls2.RTSP, urls2.RTMP)
	}
}

func TestGetSnap(t *testing.T) {
	img := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index/api/getSnap" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("secret") != "test-secret" || q.Get("url") != "rtsp://x/y" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		if q.Get("timeout_sec") != "10" || q.Get("expire_sec") != "30" || q.Get("async") != "0" {
			t.Errorf("unexpected snap params: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(img)
	})

	got, err := client.GetSnap(context.Background(), "rtsp://x/y", 10, 30, false)
	if err != nil {
		t.Fatalf("GetSnap: %v", err)
	}
	if string(got) != string(img) {
		t.Errorf("image bytes mismatch: %v", got)
	}
}

func TestGetSnapErrorJSON(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":-1,"msg":"snap failed"}`))
	})

	_, err := client.GetSnap(context.Background(), "rtsp://x/y", 10, 30, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := IsAPIError(err)
	if !ok || apiErr.Msg != "snap failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCloseRtpServer(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostFormValue("stream_id") != "3B9ACA01" {
			t.Errorf("stream_id = %q", r.PostFormValue("stream_id"))
		}
		_, _ = w.Write([]byte(`{"code":0,"data":{"hit":1}}`))
	})

	hit, err := client.CloseRtpServer(context.Background(), "3B9ACA01")
	if err != nil {
		t.Fatalf("CloseRtpServer: %v", err)
	}
	if !hit {
		t.Error("hit = false, want true")
	}
}

func TestIsStreamReady(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostFormValue("schema") != "rtsp" {
			t.Errorf("schema = %q", r.PostFormValue("schema"))
		}
		_, _ = w.Write([]byte(`{"code":0,"data":true}`))
	})

	online, err := client.IsStreamReady(context.Background(), "rtp", "3B9ACA01", "rtsp", "")
	if err != nil {
		t.Fatalf("IsStreamReady: %v", err)
	}
	if !online {
		t.Error("online = false, want true")
	}
}

func TestCloseRtpServerRootHit(t *testing.T) {
	// ZLM 实际把 hit 放在根层级。
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"hit":1}`))
	})
	hit, err := client.CloseRtpServer(context.Background(), "x")
	if err != nil || !hit {
		t.Fatalf("CloseRtpServer = (%v, %v), want (true, nil)", hit, err)
	}
}

func TestOpenRtpServerRootPort(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostFormValue("stream_id") != "3B9ACA01" {
			t.Errorf("stream_id = %q", r.PostFormValue("stream_id"))
		}
		_, _ = w.Write([]byte(`{"code":0,"port":10000}`))
	})

	res, err := client.OpenRtpServer(context.Background(), OpenRtpServerParam{StreamID: "3B9ACA01", Port: 0})
	if err != nil {
		t.Fatalf("OpenRtpServer: %v", err)
	}
	if res.Port != 10000 {
		t.Errorf("port = %d, want 10000", res.Port)
	}
}

func TestGetRtpInfoRootFields(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"exist":true,"peer_ip":"10.0.0.9","peer_port":40000,"local_ip":"10.0.0.5","local_port":10000}`))
	})

	info, err := client.GetRtpInfo(context.Background(), "3B9ACA01")
	if err != nil {
		t.Fatalf("GetRtpInfo: %v", err)
	}
	if !info.Exist || info.PeerIP != "10.0.0.9" || info.PeerPort != 40000 || info.LocalPort != 10000 {
		t.Errorf("info = %+v", info)
	}
}

func TestIsStreamReadyRootOnline(t *testing.T) {
	// ZLM 实际返回根层级 online。
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"online":true}`))
	})
	online, err := client.IsStreamReady(context.Background(), "rtp", "S", "rtsp", "")
	if err != nil || !online {
		t.Fatalf("IsStreamReady = (%v, %v), want (true, nil)", online, err)
	}
}

func TestGetServerConfigParsesFlatKeys(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":[{"general.mediaServerId":"ai-camera-zlm-01","http.port":"80"}]}`))
	})

	items, err := client.GetServerConfig(context.Background())
	if err != nil {
		t.Fatalf("GetServerConfig: %v", err)
	}
	if len(items) != 1 || items[0]["general.mediaServerId"] != "ai-camera-zlm-01" {
		t.Errorf("unexpected config: %+v", items)
	}
}
