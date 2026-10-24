// Package rpc 提供**内部服务间**调用的接口（不是给浏览器/App 用的）。
//
// 与 /web、/app 的区别：
//   - 不走用户 JWT，也不走 Casbin —— 调用方是 MQTT broker 之类的内部进程；
//   - 用共享 token（X-RPC-Token）鉴权，token 来自环境变量，不落库；
//   - 响应体直接是调用方约定的结构（如 {"allow":true}），不套 {code,message,data}。
//
// 这些端点不应暴露到公网：K8s 用 NetworkPolicy，compose 用内网网络。
package rpc

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/gin-gonic/gin"
)

// MqttAuthHandler 处理 broker 的鉴权与 ACL RPC。
type MqttAuthHandler struct {
	svc *iot.MqttAuthService
}

// NewMqttAuthHandler 构造 Handler。
func NewMqttAuthHandler() *MqttAuthHandler {
	return &MqttAuthHandler{svc: iot.NewMqttAuthService()}
}

// RpcTokenEnv 是共享 token 的环境变量名。
const RpcTokenEnv = "MQTT_RPC_TOKEN"

// authorizeRPC 校验 X-RPC-Token。
//
// 未配置 token 时返回 503 并拒绝 —— 不能因为「没配」就放行，
// 那等于把 MQTT 鉴权变成匿名可写。
func authorizeRPC(c *gin.Context) bool {
	expected := strings.TrimSpace(os.Getenv(RpcTokenEnv))
	if expected == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"allow":  false,
			"reason": "后端未配置 MQTT_RPC_TOKEN，拒绝所有 MQTT 鉴权请求",
		})
		return false
	}
	got := strings.TrimSpace(c.GetHeader("X-RPC-Token"))
	if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"allow":  false,
			"reason": "X-RPC-Token 无效",
		})
		return false
	}
	return true
}

// Authenticate 处理 POST /rpc/mqtt/auth。
func (h *MqttAuthHandler) Authenticate(c *gin.Context) {
	if !authorizeRPC(c) {
		return
	}
	var req request.MqttAuthReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.MqttAuthResp{Allow: false, Reason: "请求体不是合法 JSON"})
		return
	}
	resp, err := h.svc.Authenticate(c.Request.Context(), &req)
	if err != nil {
		// 服务层目前不返回 error（一切异常都转成 allow=false），
		// 这里是防御性分支：真出异常也绝不能放行。
		c.JSON(http.StatusOK, response.MqttAuthResp{Allow: false, Reason: "内部错误"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CheckACL 处理 POST /rpc/mqtt/acl。
func (h *MqttAuthHandler) CheckACL(c *gin.Context) {
	if !authorizeRPC(c) {
		return
	}
	var req request.MqttACLReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.MqttACLResp{Allow: false, Reason: "请求体不是合法 JSON"})
		return
	}
	resp, err := h.svc.CheckACL(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusOK, response.MqttACLResp{Allow: false, Reason: "内部错误"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
