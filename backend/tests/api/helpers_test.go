// Package api 覆盖 HTTP 层：路由注册表（静态段 vs :param 段）与中间件行为。
//
// 设计约束：
//   - **不启动监听端口**：测试进程可能与本机 :8081 的后端服务并存，
//     全部请求经 httptest.NewRecorder + Engine.ServeHTTP 或 gin.CreateTestContext 走内存；
//   - **纯路由/中间件用例不碰库**：路由注册表、AuthMiddleware、NormalizeEmptyQuery、
//     Casbin 策略判定均可在无数据库连接时完成；
//   - 仅 ApiKeyAuth / OperationLog / 「路由命中真实 Handler」三类用例经 testutil.WithTx
//     在事务中访问数据库，结束后回滚。
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/internal/api/middleware"
	"github.com/boticz/ai-camera-backend/internal/api/router"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/pkg/auth"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

const (
	// testJWTSecret 测试用 JWT 密钥（与生产配置无关）。
	testJWTSecret = "api-layer-test-secret"
	// rbacModelRel 生产 Casbin model 文件相对本包目录的路径。
	// go test 的 CWD 是包目录 backend/tests/api。
	rbacModelRel = "../../config/rbac_model.conf"

	testUserID   int64  = 7
	testUsername string = "api-tester"
	testRoleCode string = "tester"
	testDeptID   int64  = 3
)

// TestMain 静音 gin 的访问日志：路由表与中间件用例会产生大量请求记录。
func TestMain(m *testing.M) {
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard
	os.Exit(m.Run())
}

// ────────────────────────────────────────────────────────────────
// 路由构造
// ────────────────────────────────────────────────────────────────

// ensureTestConfig 注入一份最小配置。
//
// bootstrap.init() 依赖 CONFIG_PATH / config/config.yaml，而 go test 的 CWD 是包目录，
// 两者都取不到，于是 bootstrap.Config() 为 nil；router.SetupRouter() 会立刻读取
// cfg.Server.Mode 而空指针崩溃，因此必须显式注入测试配置。
//
// Swagger 关闭：swag 注册的路由不参与业务契约，避免污染路由总数断言。
func ensureTestConfig(t *testing.T) {
	t.Helper()
	if bootstrap.Config() != nil {
		return
	}
	bootstrap.SetConfig(&config.Config{
		Server:  config.ServerConfig{Mode: gin.TestMode},
		Media:   config.MediaConfig{UploadDir: t.TempDir()},
		Swagger: config.SwaggerConfig{Enabled: false},
	})
}

// testJWTService 返回与 buildRouter 注入的实例同密钥的 JWT 服务。
func testJWTService() *auth.JWTService {
	return auth.NewJWTService(&config.JWTConfig{Secret: testJWTSecret, ExpireHours: 1})
}

// buildRouter 构造生产路由表并打通认证链路。
//
// router.SetupRouter() 内部注入的是 bootstrap.JWTService()——测试进程未执行
// bootstrap.Init()，该值为 nil，AuthMiddleware 会以「认证服务未初始化」500 短路。
// 这里用中间件包提供的测试专用入口覆盖，使受保护端点可被真实访问。
func buildRouter(t *testing.T) *gin.Engine {
	t.Helper()
	ensureTestConfig(t)
	r := router.SetupRouter()
	middleware.SetJWTServiceForTest(testJWTService())
	return r
}

// validToken 签发一枚可被 AuthMiddleware 解析的 token，并写入全部 7 个 claim。
func validToken(t *testing.T) string {
	t.Helper()
	token, err := testJWTService().GenerateToken(
		testUserID, testUsername, testRoleCode, testDeptID, "all", 1, "2,3",
	)
	if err != nil {
		t.Fatalf("签发测试 token 失败: %v", err)
	}
	return token
}

// ────────────────────────────────────────────────────────────────
// 请求工具
// ────────────────────────────────────────────────────────────────

// doRequest 经内存引擎发送请求，不绑定端口。
func doRequest(r http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// apiResp 统一响应体 {code, message, data}。
type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// decodeResp 解析响应体为统一响应结构；非 JSON（例如 gin 内建 404 纯文本）时失败。
func decodeResp(t *testing.T, rec *httptest.ResponseRecorder) apiResp {
	t.Helper()
	var r apiResp
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("响应不是统一 JSON 结构（疑似落入 gin NoRoute）: %v, body=%q", err, rec.Body.String())
	}
	return r
}

// ────────────────────────────────────────────────────────────────
// 中间件链工具
// ────────────────────────────────────────────────────────────────

// runChain 在最小 gin 引擎上串起中间件，末端挂一个可观测是否被调用的 handler。
//
// 返回的 reached 指针在末端 handler 执行后为 true：用于断言「中间件放行」
// 与「中间件短路」两种路径。请求体为空，需要请求体时用 runChainWithBody。
func runChain(
	t *testing.T,
	method, target string,
	headers map[string]string,
	terminal gin.HandlerFunc,
	mws ...gin.HandlerFunc,
) (*httptest.ResponseRecorder, *bool) {
	t.Helper()
	return runChainWithBody(t, method, target, "", headers, terminal, mws...)
}

// runChainWithBody 与 runChain 相同，但可携带请求体（用于审计/绑定类用例）。
func runChainWithBody(
	t *testing.T,
	method, target, body string,
	headers map[string]string,
	terminal gin.HandlerFunc,
	mws ...gin.HandlerFunc,
) (*httptest.ResponseRecorder, *bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	e := gin.New()
	e.Use(mws...)

	reached := false
	handler := terminal
	if handler == nil {
		handler = func(c *gin.Context) {
			utils.Success(c, gin.H{"reached": true})
		}
	}
	e.Any("/*path", func(c *gin.Context) {
		reached = true
		handler(c)
	})

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else if method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete {
		reader = strings.NewReader("")
	}

	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec, &reached
}

// bearer 构造 Authorization 头集合。
func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}
