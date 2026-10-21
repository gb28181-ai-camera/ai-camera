package hook

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SecretStore 校验 hook URL 上的 secret。
//
// 抽成接口而不是直接查库：单测可以在不连数据库的前提下覆盖全部鉴权分支。
type SecretStore interface {
	// IsValidHookSecret 判断 secret 是否与任一媒体节点配置匹配。
	IsValidHookSecret(ctx context.Context, secret string) (bool, error)
}

// DBSecretStore 用 ac_gb_media_server.secret 做校验。
//
// 为什么是「任一匹配」而不是按 mediaServerId 精确匹配：
// ZLM 的 hook URL 是每个节点各自配置的静态字符串，回调体里的 mediaServerId
// 可能被篡改；只要 secret 命中任意一个已登记节点，就说明调用方持有平台密钥。
// 节点级身份由后续业务逻辑（event.MediaServerID）再校验。
type DBSecretStore struct{}

// IsValidHookSecret 查询数据库判断密钥是否有效。
func (DBSecretStore) IsValidHookSecret(ctx context.Context, secret string) (bool, error) {
	if secret == "" {
		return false, nil
	}
	db := bootstrap.DB()
	if db == nil {
		return false, errors.New("数据库未初始化，无法校验 hook secret")
	}
	var n int64
	err := db.WithContext(ctx).Model(&model.GbMediaServer{}).
		Where("secret = ?", secret).Count(&n).Error
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Handler 把 Gin 请求解析成事件并分发给 Sink。
type Handler struct {
	sink    Sink
	secrets SecretStore
	now     func() time.Time
}

// NewHandler 构造回调处理器。
func NewHandler(sink Sink, secrets SecretStore) *Handler {
	return &Handler{sink: sink, secrets: secrets, now: time.Now}
}

// ── 鉴权与响应小工具 ──────────────────────────────────────────

// authorize 校验查询参数 secret。
//
// 失败时返回 HTTP 200 + {"code":-1,"msg":"invalid secret"}：
// ZLM 的 hook 协议只认 HTTP 200，非 200 会被视为网络错误反复重试。
func (h *Handler) authorize(c *gin.Context) bool {
	secret := c.Query("secret")
	ok, err := h.secrets.IsValidHookSecret(c.Request.Context(), secret)
	if err != nil {
		utils.Warn("[ZLM-HOOK] 校验 secret 失败", zap.Error(err))
		c.JSON(http.StatusOK, FailResult("secret validation failed"))
		return false
	}
	if !ok {
		utils.Warn("[ZLM-HOOK] 非法的 hook secret", zap.String("remote", c.ClientIP()))
		c.JSON(http.StatusOK, FailResult("invalid secret"))
		return false
	}
	return true
}

// bindJSON 解析请求体；失败时返回与鉴权失败同形的 200 响应。
func bindJSON(c *gin.Context, dst any) bool {
	// 用 ShouldBindJSON 而不是 ShouldBind：hook 负载固定为 JSON，
	// 避免 ZLM 偶尔发 Content-Type 不标准时被表单解析器抢走。
	if err := c.ShouldBindJSON(dst); err != nil {
		utils.Warn("[ZLM-HOOK] 解析回调负载失败",
			zap.String("path", c.FullPath()), zap.Error(err))
		c.JSON(http.StatusOK, FailResult("invalid json"))
		return false
	}
	return true
}

// ok 返回标准成功响应。
func ok(c *gin.Context) { c.JSON(http.StatusOK, SuccessResult()) }

// ── on_server_keepalive ───────────────────────────────────────

type keepalivePayload struct {
	MediaServerID string         `json:"mediaServerId"`
	Data          map[string]any `json:"data"`
}

// OnServerKeepalive 节点心跳。
func (h *Handler) OnServerKeepalive(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var payload keepalivePayload
	if !bindJSON(c, &payload) {
		return
	}
	event := &MediaServerEvent{
		MediaServerID: payload.MediaServerID,
		IP:            c.ClientIP(),
		Data:          payload.Data,
		ReceivedAt:    h.now(),
	}
	if err := h.sink.OnServerKeepalive(c.Request.Context(), event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理心跳失败", zap.Error(err))
	}
	ok(c)
}

// ── on_server_started ─────────────────────────────────────────

// OnServerStarted 节点启动。
//
// on_server_started 的负载是 ZLM 的完整配置表（扁平 key），
// 不强行映射成结构体，避免 ZLM 版本升级时因新增字段解析失败。
func (h *Handler) OnServerStarted(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var payload map[string]any
	if !bindJSON(c, &payload) {
		return
	}
	id, _ := payload["mediaServerId"].(string)
	if id == "" {
		// 部分版本只在 general.mediaServerId 里给出节点标识。
		id, _ = payload["general.mediaServerId"].(string)
	}
	event := &MediaServerEvent{
		MediaServerID: id,
		IP:            c.ClientIP(),
		Config:        payload,
		ReceivedAt:    h.now(),
	}
	if err := h.sink.OnServerStarted(c.Request.Context(), event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理节点启动失败", zap.Error(err))
	}
	ok(c)
}

// ── on_publish ────────────────────────────────────────────────

// OnPublish 推流鉴权。
//
// 拒绝时必须返回 code=-1（HTTP 仍为 200），ZLM 才会真正断开推流。
// Sink 返回 error 时按拒绝处理（fail-closed），避免鉴权逻辑异常时放行。
func (h *Handler) OnPublish(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var event PublishEvent
	if !bindJSON(c, &event) {
		return
	}
	allowed, err := h.sink.OnPublish(c.Request.Context(), &event)
	if err != nil {
		utils.Warn("[ZLM-HOOK] 推流鉴权处理失败，按拒绝处理",
			zap.String("app", event.App), zap.String("stream", event.Stream), zap.Error(err))
		c.JSON(http.StatusOK, FailResult("publish check failed"))
		return
	}
	if !allowed {
		utils.Info("[ZLM-HOOK] 拒绝推流",
			zap.String("mediaServerId", event.MediaServerID),
			zap.String("app", event.App), zap.String("stream", event.Stream))
		c.JSON(http.StatusOK, FailResult("publish rejected"))
		return
	}
	ok(c)
}

// ── on_play ───────────────────────────────────────────────────

// OnPlay 播放鉴权。
func (h *Handler) OnPlay(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var event PlayEvent
	if !bindJSON(c, &event) {
		return
	}
	allowed, err := h.sink.OnPlay(c.Request.Context(), &event)
	if err != nil {
		utils.Warn("[ZLM-HOOK] 播放鉴权处理失败，按拒绝处理",
			zap.String("app", event.App), zap.String("stream", event.Stream), zap.Error(err))
		c.JSON(http.StatusOK, FailResult("play check failed"))
		return
	}
	if !allowed {
		utils.Info("[ZLM-HOOK] 拒绝播放",
			zap.String("mediaServerId", event.MediaServerID),
			zap.String("app", event.App), zap.String("stream", event.Stream))
		c.JSON(http.StatusOK, FailResult("play rejected"))
		return
	}
	ok(c)
}

// ── on_stream_changed ─────────────────────────────────────────

// OnStreamChanged 流注册/注销。
func (h *Handler) OnStreamChanged(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var event StreamChangedEvent
	if !bindJSON(c, &event) {
		return
	}
	if err := h.sink.OnStreamChanged(c.Request.Context(), &event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理流变化失败", zap.Error(err))
	}
	ok(c)
}

// ── on_stream_none_reader ─────────────────────────────────────

// OnStreamNoneReader 流无人观看。本期固定 close=false。
func (h *Handler) OnStreamNoneReader(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var event StreamEvent
	if !bindJSON(c, &event) {
		return
	}
	if err := h.sink.OnStreamNoneReader(c.Request.Context(), &event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理无人观看失败", zap.Error(err))
	}
	c.JSON(http.StatusOK, NoneReaderResult{Code: 0, Msg: "success", Close: false})
}

// ── on_stream_not_found ───────────────────────────────────────

// OnStreamNotFound 流未找到。
func (h *Handler) OnStreamNotFound(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var event StreamEvent
	if !bindJSON(c, &event) {
		return
	}
	if err := h.sink.OnStreamNotFound(c.Request.Context(), &event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理流未找到失败", zap.Error(err))
	}
	ok(c)
}

// ── on_send_rtp_stopped ───────────────────────────────────────

// OnSendRTPStopped RTP 发送被动结束。
func (h *Handler) OnSendRTPStopped(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var event SendRTPStoppedEvent
	if !bindJSON(c, &event) {
		return
	}
	if err := h.sink.OnSendRTPStopped(c.Request.Context(), &event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理 RTP 发送停止失败", zap.Error(err))
	}
	ok(c)
}

// ── on_rtp_server_timeout ─────────────────────────────────────

type rtpServerTimeoutPayload struct {
	MediaServerID string `json:"mediaServerId"`
	LocalPort     int    `json:"local_port"`
	StreamID      string `json:"stream_id"`
	TCPMode       int    `json:"tcpMode"`
	ReUsePort     bool   `json:"re_use_port"`
	SSRC          string `json:"ssrc"`
}

// OnRTPServerTimeout RTP 收流超时。
//
// 该回调只发生在 RTP server 语境下，app 恒为 rtp（与 WVP 的处理一致），
// 因此在转换事件时补齐 App，便于下游按统一键清理活跃流。
func (h *Handler) OnRTPServerTimeout(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var payload rtpServerTimeoutPayload
	if !bindJSON(c, &payload) {
		return
	}
	event := &StreamEvent{
		MediaServerID: payload.MediaServerID,
		App:           "rtp",
		Stream:        payload.StreamID,
		StreamID:      payload.StreamID,
		SSRC:          payload.SSRC,
		LocalPort:     payload.LocalPort,
		TCPMode:       payload.TCPMode,
		ReUsePort:     payload.ReUsePort,
	}
	if err := h.sink.OnRTPServerTimeout(c.Request.Context(), event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理 RTP 收流超时失败", zap.Error(err))
	}
	ok(c)
}

// ── on_record_mp4 ─────────────────────────────────────────────

// OnRecordMP4 录像落盘完成。
func (h *Handler) OnRecordMP4(c *gin.Context) {
	if !h.authorize(c) {
		return
	}
	var event RecordMP4Event
	if !bindJSON(c, &event) {
		return
	}
	if err := h.sink.OnRecordMP4(c.Request.Context(), &event); err != nil {
		utils.Warn("[ZLM-HOOK] 处理录像完成失败", zap.Error(err))
	}
	ok(c)
}
