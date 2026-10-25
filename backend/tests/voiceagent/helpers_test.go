package voiceagent_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// staticProvider 固定运行期配置（测试不依赖 ac_thirdparty_config）。
type staticProvider struct {
	cfg *voiceagent.RuntimeConfig
	err error
}

func (p *staticProvider) Runtime(ctx context.Context) (*voiceagent.RuntimeConfig, error) {
	return p.cfg, p.err
}

func testConfig() *voiceagent.RuntimeConfig {
	return &voiceagent.RuntimeConfig{
		WSEnabled:        true,
		WSPath:           "/ws/voice",
		FastAckText:      "我在",
		MCPEnabled:       true,
		InternalMCPToken: "mcp-secret",
		MemoryEnabled:    true,
		MemoryTopK:       6,
		MemoryTTLDays:    30,
	}
}

var uniqSeq int64

func uniq(prefix string) string {
	n := atomic.AddInt64(&uniqSeq, 1)
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), n)
}

// seedAIChatProduct 创建 aichat 产品（WebSocket 服务 + 产品×服务凭据）。
func seedAIChatProduct(t *testing.T, ctx context.Context) (productID, serviceID int64, username, password string) {
	t.Helper()
	db := dbFromCtx()
	p := &model.IotProduct{Name: uniq("p"), Code: uniq("code")}
	if err := db.WithContext(ctx).Create(p).Error; err != nil {
		t.Fatalf("创建产品失败: %v", err)
	}
	g := &model.IotServiceGroup{Name: uniq("g"), Code: uniq("gc"), Type: model.IotServiceGroupAIChat}
	if err := db.WithContext(ctx).Create(g).Error; err != nil {
		t.Fatalf("创建服务组失败: %v", err)
	}
	s := &model.IotService{
		GroupID: g.ID, Code: uniq("ws"), Name: "WebSocket",
		Protocol: model.IotProtocolWebSocket, Host: "127.0.0.1", Port: 8090, Path: "/ws/voice",
	}
	if err := db.WithContext(ctx).Create(s).Error; err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	if err := db.WithContext(ctx).Create(&model.IotProductServiceGroup{
		ProductID: p.ID, ServiceGroupID: g.ID,
	}).Error; err != nil {
		t.Fatalf("绑定服务组失败: %v", err)
	}
	username = uniq("user")
	password = uniq("pass")
	if err := db.WithContext(ctx).Create(&model.IotProductService{
		ProductID: p.ID, ServiceID: s.ID, Username: username, Password: password,
	}).Error; err != nil {
		t.Fatalf("创建产品服务凭据失败: %v", err)
	}
	return p.ID, s.ID, username, password
}

func seedDevice(t *testing.T, ctx context.Context, productID int64, code string) *model.Device {
	t.Helper()
	d := &model.Device{Name: uniq("dev"), DeviceCode: code, ProductID: &productID}
	if err := dbFromCtx().WithContext(ctx).Create(d).Error; err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	return d
}

func dbFromCtx() *gorm.DB {
	return testutil.InitTestDB()
}

// echoPipecat 启动一个模拟 Pipecat 的 WS 服务：记录请求头并回显消息。
type echoPipecat struct {
	server     *httptest.Server
	authHeader atomic.Value
	ctxHeader  atomic.Value
}

func newEchoPipecat(t *testing.T) *echoPipecat {
	t.Helper()
	e := &echoPipecat{}
	up := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	e.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.authHeader.Store(r.Header.Get("Authorization"))
		e.ctxHeader.Store(r.Header.Get("X-Voice-Context"))
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, data); err != nil {
				return
			}
		}
	}))
	t.Cleanup(e.server.Close)
	return e
}

func (e *echoPipecat) URL() string { return e.server.URL }

// newWSRouter 构造仅包含 WS 网关的 gin 路由。
func newWSRouter(gw *voiceagent.Gateway, path string) *httptest.Server {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET(path, func(c *gin.Context) {
		kind := "web"
		if strings.HasPrefix(path, "/app/") {
			kind = "app"
		}
		gw.HandleWeb(c, kind)
	})
	r.GET("/device/voice/ws", gw.HandleDevice)
	return httptest.NewServer(r)
}

func wsURL(base, path, query string) string {
	u := strings.Replace(base, "http://", "ws://", 1)
	if query != "" {
		u += path + "?" + query
	}
	return u
}

func dialWS(t *testing.T, url string, headers http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	return d.Dial(url, headers)
}

func signContext(t *testing.T, ctx request.ChatContext) string {
	t.Helper()
	signer := voiceagent.NewContextSigner("ctx-secret")
	v, err := signer.Sign(ctx)
	if err != nil {
		t.Fatalf("签名上下文失败: %v", err)
	}
	return v
}

// makeVector 构造 1024 维测试向量：idx 位置为 1，其余为 0.01。
func makeVector(idx int) []float32 {
	v := make([]float32, voiceagent.EmbeddingDim)
	for i := range v {
		v[i] = 0.01
	}
	if idx >= 0 && idx < len(v) {
		v[idx] = 1
	}
	return v
}
