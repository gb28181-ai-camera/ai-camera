package webrtc

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// mediaServerFromURL 把 httptest 的地址转成媒体节点模型。
func mediaServerFromURL(t *testing.T, raw string) *model.GbMediaServer {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("解析测试地址失败: %v", err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("解析测试地址端口失败: %v", err)
	}
	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil {
		t.Fatalf("解析测试端口失败: %v", err)
	}
	return &model.GbMediaServer{IP: host, HTTPPort: port}
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	ms := mediaServerFromURL(t, srv.URL)
	c := NewClient(func(context.Context) (*model.GbMediaServer, error) { return ms, nil })
	c.HTTPClient = srv.Client()
	return c
}

// TestAnswerSuccess 校验请求方法/头/查询参数，并原样返回 answer SDP。
func TestAnswerSuccess(t *testing.T) {
	const wantAnswer = "v=0\r\no=zlm 1 1 IN IP4 127.0.0.1\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\n"

	var got *http.Request
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/sdp")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(wantAnswer))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	answer, err := c.Answer(context.Background(), "0BEBC201", "v=0\r\no=app")
	if err != nil {
		t.Fatalf("协商失败: %v", err)
	}
	if answer != wantAnswer {
		t.Fatalf("answer 不一致: %q", answer)
	}
	if got.Method != http.MethodPost {
		t.Fatalf("应使用 POST，实际 %s", got.Method)
	}
	if ct := got.Header.Get("Content-Type"); ct != "application/sdp" {
		t.Fatalf("Content-Type 应为 application/sdp，实际 %q", ct)
	}
	q := got.URL.Query()
	if q.Get("app") != "rtp" || q.Get("stream") != "0BEBC201" || q.Get("type") != "play" {
		t.Fatalf("WHEP 查询参数不正确: %s", got.URL.RawQuery)
	}
	if got.URL.Path != "/index/api/webrtc" {
		t.Fatalf("WHEP 路径不正确: %s", got.URL.Path)
	}
	if gotBody != "v=0\r\no=app" {
		t.Fatalf("请求体应为原始 offer SDP，实际 %q", gotBody)
	}
}

// TestAnswerZLMError ZLM 返回 HTTP 200 + JSON 业务错误。
func TestAnswerZLMError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":-1,"msg":"can not find the stream"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv).Answer(context.Background(), "missing", "v=0")
	if err == nil {
		t.Fatal("业务错误应返回 error")
	}
	if !errors.Is(err, ErrStreamNotFound) {
		t.Fatalf("流不存在应可被 errors.Is 识别，实际 %v", err)
	}
	var zerr *ZLMError
	if !errors.As(err, &zerr) || zerr.Code != -1 {
		t.Fatalf("应返回 ZLMError，实际 %v", err)
	}
}

// TestAnswerHTTPErrors 非 2xx 必须失败；404 映射为 flow not found。
func TestAnswerHTTPErrors(t *testing.T) {
	cases := []struct {
		status   int
		notFound bool
	}{
		{http.StatusInternalServerError, false},
		{http.StatusNotFound, true},
		{http.StatusBadGateway, false},
	}
	for _, c := range cases {
		t.Run(strconv.Itoa(c.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte("upstream error"))
			}))
			defer srv.Close()

			_, err := newTestClient(t, srv).Answer(context.Background(), "s1", "v=0")
			if err == nil {
				t.Fatal("非 2xx 应返回 error")
			}
			var herr *HTTPError
			if !errors.As(err, &herr) || herr.StatusCode != c.status {
				t.Fatalf("应返回 HTTPError(%d)，实际 %v", c.status, err)
			}
			if errors.Is(err, ErrStreamNotFound) != c.notFound {
				t.Fatalf("ErrStreamNotFound 映射不正确: %v", err)
			}
		})
	}
}

// TestAnswerBadAnswer 2xx 但响应不是 SDP。
func TestAnswerBadAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv).Answer(context.Background(), "s1", "v=0")
	if !errors.Is(err, ErrBadAnswer) {
		t.Fatalf("应返回 ErrBadAnswer，实际 %v", err)
	}
}

// TestAnswerValidatesInput 空 stream / 空 offer 在发请求前就被拒绝。
func TestAnswerValidatesInput(t *testing.T) {
	called := false
	c := NewClient(func(context.Context) (*model.GbMediaServer, error) {
		called = true
		return &model.GbMediaServer{IP: "127.0.0.1", HTTPPort: 80}, nil
	})

	if _, err := c.Answer(context.Background(), " ", "v=0"); !errors.Is(err, ErrStreamNotFound) {
		t.Fatalf("空 stream 应返回 ErrStreamNotFound，实际 %v", err)
	}
	if _, err := c.Answer(context.Background(), "s1", "  "); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("空 offer 应返回 ErrBadRequest，实际 %v", err)
	}
	if called {
		t.Fatal("参数非法时不应解析媒体节点")
	}

	// 未配置解析器时必须 fail-closed。
	var nilClient *Client
	if _, err := nilClient.Answer(context.Background(), "s1", "v=0"); err == nil {
		t.Fatal("nil 客户端应返回 error")
	}
	if _, err := (&Client{}).Answer(context.Background(), "s1", "v=0"); err == nil {
		t.Fatal("未配置 Resolve 应返回 error")
	}
}

// TestWHEPURL 地址构造：stream_ip 优先、端口回退、查询参数转义。
func TestWHEPURL(t *testing.T) {
	ms := &model.GbMediaServer{IP: "10.0.0.5", StreamIP: "202.1.1.1", HTTPPort: 8082}
	got := WHEPURL(ms, "rtp", "0BEBC201")
	want := "http://202.1.1.1:8082/index/api/webrtc?app=rtp&stream=0BEBC201&type=play"
	if got != want {
		t.Fatalf("WHEPURL = %q，期望 %q", got, want)
	}

	// 没有 stream_ip 时回退 ip；端口未配置回退 80。
	ms2 := &model.GbMediaServer{IP: "10.0.0.6"}
	got2 := WHEPURL(ms2, "rtp", "s1")
	if !strings.HasPrefix(got2, "http://10.0.0.6:80/index/api/webrtc?") {
		t.Fatalf("回退地址不正确: %q", got2)
	}

	// 配置里带协议不应拼出 http://http://。
	ms3 := &model.GbMediaServer{IP: "http://10.0.0.7/", HTTPPort: 80}
	if strings.Contains(WHEPURL(ms3, "rtp", "s1"), "http://http") {
		t.Fatalf("协议前缀未清理: %q", WHEPURL(ms3, "rtp", "s1"))
	}

	// 空节点也要给出可解析的地址（便于错误信息定位）。
	if got := WHEPURL(nil, "rtp", "s1"); !strings.HasPrefix(got, "http://127.0.0.1:80/") {
		t.Fatalf("nil 节点地址不正确: %q", got)
	}
}
