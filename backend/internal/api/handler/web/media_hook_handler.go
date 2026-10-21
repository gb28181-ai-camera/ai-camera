package handler

import (
	"github.com/boticz/ai-camera-backend/internal/gb28181/hook"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// ZLMediaKit 回调（/media/hook）
//
// 这一组接口是 ZLM → 平台的**入向**通知，调用方是外部进程，没有 JWT。
// 因此：
//   - 路由由 internal/api/router/media.go 单独注册，**不挂 authMW / casbinMW**；
//   - 鉴权只依赖 URL 查询参数 secret（与 ac_gb_media_server.secret 匹配）。
//
// Handler 只做薄封装：真正的解析/分发在 internal/gb28181/hook 包，
// 这里保留独立方法是为了给每个端点写 Swagger 注释（生成器按方法注释工作）。
// ════════════════════════════════════════════════════════════════

// MediaHookHandler ZLM 回调处理器。
type MediaHookHandler struct {
	h *hook.Handler
}

// NewMediaHookHandler 使用默认 Sink 与数据库 secret 校验构造处理器。
func NewMediaHookHandler() *MediaHookHandler {
	return NewMediaHookHandlerWith(hook.NewDefaultSink(), hook.DBSecretStore{})
}

// NewMediaHookHandlerWith 允许 Wave 2 注入自定义 Sink（播放编排、录像落库、语音回推）。
func NewMediaHookHandlerWith(sink hook.Sink, secrets hook.SecretStore) *MediaHookHandler {
	return &MediaHookHandler{h: hook.NewHandler(sink, secrets)}
}

// OnServerKeepalive ZLM 节点心跳
//
// @Summary      ZLM 节点心跳回调
// @Description  ZLM 按 hook_alive_interval（默认 15s）上报。校验 secret 后刷新节点 online / last_keepalive_dt。此路由不挂 JWT 鉴权。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string  true  "ZLM API 密钥（ac_gb_media_server.secret）"
// @Success      200     {object}  hook.Result  "code=0 成功；code=-1 鉴权失败"
// @Router       /media/hook/on_server_keepalive [post]
// @ID           mediaHookOnServerKeepalive
func (h *MediaHookHandler) OnServerKeepalive(c *gin.Context) { h.h.OnServerKeepalive(c) }

// OnServerStarted ZLM 节点启动
//
// @Summary      ZLM 节点启动回调
// @Description  ZLM 启动时上报完整配置表。校验 secret 后把节点标记为在线。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string  true  "ZLM API 密钥"
// @Param        body    body    object  true  "ZLM 配置表（扁平 key，含 mediaServerId）"
// @Success      200     {object}  hook.Result
// @Router       /media/hook/on_server_started [post]
// @ID           mediaHookOnServerStarted
func (h *MediaHookHandler) OnServerStarted(c *gin.Context) { h.h.OnServerStarted(c) }

// OnPublish 推流鉴权
//
// @Summary      ZLM 推流鉴权回调
// @Description  rtsp/rtmp/rtp 推流前触发。**拒绝时返回 HTTP 200 + {"code":-1}**（ZLM 约定，非 HTTP 401）。本期放行，Wave 2 按 SSRC 注册表校验。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string              true  "ZLM API 密钥"
// @Param        body    body    hook.PublishEvent  true  "推流信息"
// @Success      200     {object}  hook.Result  "code=0 放行；code=-1 拒绝"
// @Router       /media/hook/on_publish [post]
// @ID           mediaHookOnPublish
func (h *MediaHookHandler) OnPublish(c *gin.Context) { h.h.OnPublish(c) }

// OnPlay 播放鉴权
//
// @Summary      ZLM 播放鉴权回调
// @Description  rtsp/rtmp/http-flv/ws-flv/hls 播放前触发。拒绝时返回 HTTP 200 + {"code":-1}。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string           true  "ZLM API 密钥"
// @Param        body    body    hook.PlayEvent  true  "播放信息"
// @Success      200     {object}  hook.Result  "code=0 放行；code=-1 拒绝"
// @Router       /media/hook/on_play [post]
// @ID           mediaHookOnPlay
func (h *MediaHookHandler) OnPlay(c *gin.Context) { h.h.OnPlay(c) }

// OnStreamChanged 流注册/注销
//
// @Summary      ZLM 流注册/注销回调
// @Description  regist=true 记录活跃流，regist=false 清理内存表。**不写 ac_gb_cloud_record**（录像索引由 on_record_mp4 产生）。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string                   true  "ZLM API 密钥"
// @Param        body    body    hook.StreamChangedEvent true  "流变化信息"
// @Success      200     {object}  hook.Result
// @Router       /media/hook/on_stream_changed [post]
// @ID           mediaHookOnStreamChanged
func (h *MediaHookHandler) OnStreamChanged(c *gin.Context) { h.h.OnStreamChanged(c) }

// OnStreamNoneReader 流无人观看
//
// @Summary      ZLM 流无人观看回调
// @Description  ZLM 检测到流没有播放者时触发。本期固定返回 close=false，不主动关流。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string            true  "ZLM API 密钥"
// @Param        body    body    hook.StreamEvent true  "流信息"
// @Success      200     {object}  hook.NoneReaderResult
// @Router       /media/hook/on_stream_none_reader [post]
// @ID           mediaHookOnStreamNoneReader
func (h *MediaHookHandler) OnStreamNoneReader(c *gin.Context) { h.h.OnStreamNoneReader(c) }

// OnStreamNotFound 流未找到
//
// @Summary      ZLM 流未找到回调
// @Description  按需拉流的触发点；本期只记录日志。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string            true  "ZLM API 密钥"
// @Param        body    body    hook.StreamEvent true  "流信息"
// @Success      200     {object}  hook.Result
// @Router       /media/hook/on_stream_not_found [post]
// @ID           mediaHookOnStreamNotFound
func (h *MediaHookHandler) OnStreamNotFound(c *gin.Context) { h.h.OnStreamNotFound(c) }

// OnSendRTPStopped RTP 发送结束
//
// @Summary      ZLM RTP 发送停止回调
// @Description  语音回推/级联推流被动结束。Wave 2 据此更新推流状态与语音会话。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string                     true  "ZLM API 密钥"
// @Param        body    body    hook.SendRTPStoppedEvent  true  "发送信息"
// @Success      200     {object}  hook.Result
// @Router       /media/hook/on_send_rtp_stopped [post]
// @ID           mediaHookOnSendRTPStopped
func (h *MediaHookHandler) OnSendRTPStopped(c *gin.Context) { h.h.OnSendRTPStopped(c) }

// OnRTPServerTimeout RTP 收流超时
//
// @Summary      ZLM RTP 收流超时回调
// @Description  ZLM rtp_proxy.timeoutSec 到期后触发；单端口模式下 stream_id 即 hex SSRC。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string            true  "ZLM API 密钥"
// @Param        body    body    object           true  "超时信息（stream_id/local_port/ssrc）"
// @Success      200     {object}  hook.Result
// @Router       /media/hook/on_rtp_server_timeout [post]
// @ID           mediaHookOnRTPServerTimeout
func (h *MediaHookHandler) OnRTPServerTimeout(c *gin.Context) { h.h.OnRTPServerTimeout(c) }

// OnRecordMP4 录像落盘完成
//
// @Summary      ZLM 录像落盘完成回调
// @Description  文件完整落盘后触发，是写入 ac_gb_cloud_record 的正确时机（Wave 2 实现）。
// @Tags         MediaHook
// @Accept       json
// @Produce      json
// @Param        secret  query  string                  true  "ZLM API 密钥"
// @Param        body    body    hook.RecordMP4Event    true  "录像文件信息"
// @Success      200     {object}  hook.Result
// @Router       /media/hook/on_record_mp4 [post]
// @ID           mediaHookOnRecordMP4
func (h *MediaHookHandler) OnRecordMP4(c *gin.Context) { h.h.OnRecordMP4(c) }
