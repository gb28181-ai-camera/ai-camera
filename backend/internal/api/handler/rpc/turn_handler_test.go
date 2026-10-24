package rpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/service/turn"
	"github.com/gin-gonic/gin"
)

// newTurnEngine 只挂 TURN handler，避免测试依赖数据库与完整路由。
func newTurnEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewTurnHandler()
	r.POST("/rpc/turn/credentials", h.IssueCredential)
	return r
}

func doRequest(t *testing.T, r *gin.Engine, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rpc/turn/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-RPC-Token", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestTurnHandlerToken token 缺失/错误必须拒绝，正确时签发凭据。
func TestTurnHandlerToken(t *testing.T) {
	t.Setenv(turn.EnvRPCToken, "rpc-token")
	t.Setenv(turn.EnvSecret, "test-secret")
	r := newTurnEngine()

	if w := doRequest(t, r, "", `{"subject":"u1"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401，实际 %d", w.Code)
	}
	if w := doRequest(t, r, "wrong", `{"subject":"u1"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("错误 token 应 401，实际 %d", w.Code)
	}

	w := doRequest(t, r, "rpc-token", `{"subject":"u1","ttl":600}`)
	if w.Code != http.StatusOK {
		t.Fatalf("正确 token 应 200，实际 %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		Code int `json:"code"`
		Data struct {
			Username string `json:"username"`
			Password string `json:"password"`
			TTL      int    `json:"ttl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if out.Data.Username == "" || out.Data.Password == "" || out.Data.TTL != 600 {
		t.Fatalf("凭据字段不完整: %+v", out.Data)
	}
}

// TestTurnHandlerNoRPCToken 后端未配置 token 时必须 503（fail-closed）。
func TestTurnHandlerNoRPCToken(t *testing.T) {
	t.Setenv(turn.EnvRPCToken, "")
	t.Setenv(turn.EnvSecret, "test-secret")
	r := newTurnEngine()
	if w := doRequest(t, r, "anything", `{"subject":"u1"}`); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置 token 应 503，实际 %d", w.Code)
	}
}

// TestTurnHandlerNoSecret 配了 token 但没配 TURN 密钥时也必须失败。
func TestTurnHandlerNoSecret(t *testing.T) {
	t.Setenv(turn.EnvRPCToken, "rpc-token")
	t.Setenv(turn.EnvSecret, "")
	r := newTurnEngine()
	if w := doRequest(t, r, "rpc-token", `{"subject":"u1"}`); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("未配置密钥应 503，实际 %d", w.Code)
	}
}
