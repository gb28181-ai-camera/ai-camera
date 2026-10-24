package rpc

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/turn"
	"github.com/gin-gonic/gin"
)

// TurnHandler 签发 coturn 的短期凭据。
//
// coturn 没有 HTTP 鉴权回调，所以「自定义授权」发生在**签发端**：
// 调用方（IOT OTA、App 网关、其他内部服务）先通过 X-RPC-Token 证明身份，
// 再拿到一份带过期时间的 HMAC 凭据。coturn 只用共享密钥验签。
type TurnHandler struct {
	svc *turn.Service
}

// NewTurnHandler 构造 Handler。
func NewTurnHandler() *TurnHandler {
	return &TurnHandler{svc: turn.New()}
}

// authorizeTurnRPC 校验 TURN_RPC_TOKEN。
func authorizeTurnRPC(c *gin.Context) bool {
	expected := strings.TrimSpace(os.Getenv(turn.EnvRPCToken))
	if expected == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    http.StatusServiceUnavailable,
			"message": "后端未配置 TURN_RPC_TOKEN，拒绝签发 TURN 凭据",
		})
		return false
	}
	got := strings.TrimSpace(c.GetHeader("X-RPC-Token"))
	if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    http.StatusUnauthorized,
			"message": "X-RPC-Token 无效",
		})
		return false
	}
	return true
}

// IssueCredential 处理 POST /rpc/turn/credentials。
func (h *TurnHandler) IssueCredential(c *gin.Context) {
	if !authorizeTurnRPC(c) {
		return
	}
	var req request.TurnCredentialReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "message": "请求体不是合法 JSON"})
		return
	}
	cred, err := h.svc.Issue(req.Subject, req.TTL)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, turn.ErrSecretNotConfigured) {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, gin.H{"code": status, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": http.StatusOK, "message": "success", "data": cred})
}
