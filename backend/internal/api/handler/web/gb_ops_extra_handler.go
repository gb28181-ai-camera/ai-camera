package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标运维增强：媒体节点指标 / 流会话 / 平台信息 / 日志 / 报警流
//
// 与 WVP 对齐的「运维」页面能力。SSE 路由（/logs/stream、/alarms/stream）
// 都挂在带 JWT 的 wAuth 组上：EventSource 无法自定义 Header，前端需用
// fetch + ReadableStream 携带 Bearer token（前端实现不在本批次范围）。
// ════════════════════════════════════════════════════════════════

// GbOpsExtraHandler 国标运维增强处理器。
type GbOpsExtraHandler struct {
	metricsSvc *gbsvc.GbMediaMetricsService
	sessionSvc *gbsvc.GbMediaSessionService
	infoSvc    *gbsvc.GbPlatformInfoService
	logSvc     *gbsvc.GbLogStreamService
	alarmSvc   *gbsvc.GbAlarmStreamService
}

// NewGbOpsExtraHandler 构造运维增强处理器。
func NewGbOpsExtraHandler() *GbOpsExtraHandler {
	return &GbOpsExtraHandler{
		metricsSvc: gbsvc.NewGbMediaMetricsService(),
		sessionSvc: gbsvc.NewGbMediaSessionService(),
		infoSvc:    gbsvc.NewGbPlatformInfoService(),
		logSvc:     gbsvc.NewGbLogStreamService(),
		alarmSvc:   gbsvc.NewGbAlarmStreamService(),
	}
}

// respondGbOpsErr 在通用国标错误映射之上补 409（流未启动 / 节点不可达）。
func respondGbOpsErr(c *gin.Context, err error, notFound ...error) {
	if errors.Is(err, gbsvc.ErrGbStreamNotRunning) || errors.Is(err, gbsvc.ErrGbMediaServerUnreachable) {
		c.JSON(http.StatusConflict, utils.Response{Code: http.StatusConflict, Message: err.Error()})
		return
	}
	if errors.Is(err, gbsvc.ErrGbMediaMetricsUnavailable) {
		utils.ServerError(c, err.Error())
		return
	}
	respondGbErr(c, err, notFound...)
}

// ── 媒体节点指标 ──────────────────────────────────────────────

// Metrics 媒体节点当前指标
//
// @Summary      媒体节点当前指标
// @Description  采集后端所在主机的 CPU/内存/磁盘/网络/负载，并返回节点在线状态与当前流数。多副本部署时各副本只返回自身节点指标
// @Tags         GbOps
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "媒体节点ID"
// @Success      200  {object}  response.RGbMediaMetrics
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      500  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id}/metrics [get]
// @ID           getGbMediaServerMetrics
func (h *GbOpsExtraHandler) Metrics(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	resp, err := h.metricsSvc.Metrics(c.Request.Context(), id)
	if err != nil {
		respondGbOpsErr(c, err, gbsvc.ErrGbMediaServerNotFound)
		return
	}
	utils.Success(c, resp)
}

// MetricsHistory 媒体节点指标历史
//
// @Summary      媒体节点指标历史
// @Description  返回进程内环缓冲的时间序列（5 秒采样，保留 24 小时）。多副本部署时只有单节点数据
// @Tags         GbOps
// @Accept       json
// @Produce      json
// @Param        id     path      int     true   "媒体节点ID"
// @Param        range  query     string  false  "时间范围 1h/24h"  default(1h)
// @Success      200  {object}  response.RGbMediaMetricsHistory
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id}/metrics/history [get]
// @ID           getGbMediaServerMetricsHistory
func (h *GbOpsExtraHandler) MetricsHistory(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	resp, err := h.metricsSvc.History(c.Request.Context(), id, c.Query("range"))
	if err != nil {
		respondGbOpsErr(c, err, gbsvc.ErrGbMediaServerNotFound)
		return
	}
	utils.Success(c, resp)
}

// Streams 媒体节点流会话
//
// @Summary      媒体节点流会话
// @Description  调用 ZLM getMediaList 返回当前全部流（app/stream/schema/编码/分辨率/帧率/观看数/码率/存活时长）
// @Tags         GbOps
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "媒体节点ID"
// @Success      200  {object}  response.RGbMediaStreamList
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id}/streams [get]
// @ID           listGbMediaServerStreams
func (h *GbOpsExtraHandler) Streams(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	resp, err := h.sessionSvc.ListByMediaServer(c.Request.Context(), id)
	if err != nil {
		respondGbOpsErr(c, err, gbsvc.ErrGbMediaServerNotFound)
		return
	}
	utils.Success(c, resp)
}

// ── 平台信息 ──────────────────────────────────────────────────

// PlatformInfo 平台运行信息
//
// @Summary      平台运行信息
// @Description  版本/构建信息/运行时长/SIP 服务状态/默认媒体节点摘要/DB 与 Redis 状态
// @Tags         GbOps
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RGbPlatformInfo
// @Router       /web/gb/platform-info [get]
// @ID           getGbPlatformInfo
func (h *GbOpsExtraHandler) PlatformInfo(c *gin.Context) {
	utils.Success(c, h.infoSvc.Info(c.Request.Context()))
}

// ── 日志 ──────────────────────────────────────────────────────

// Logs 日志检索
//
// @Summary      日志检索
// @Description  基于进程内 ring buffer（最近 5000 条）的历史日志检索；level 为最低级别
// @Tags         GbOps
// @Accept       json
// @Produce      json
// @Param        page      query     int     false  "页码"     default(1)
// @Param        page_size query     int     false  "每页条数"  default(50)
// @Param        level     query     string  false  "最低级别 debug/info/warn/error"
// @Param        keyword   query     string  false  "关键字（消息与结构化字段）"
// @Success      200  {object}  response.RPageGbLog
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/logs [get]
// @ID           pageGbLogs
func (h *GbOpsExtraHandler) Logs(c *gin.Context) {
	var req request.GbLogQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.logSvc.Page(&req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// LogsStream 实时日志
//
// @Summary      实时日志（SSE）
// @Description  text/event-stream；事件名 log，data 为 GbLogItem JSON。建连先回放最近 50 条匹配日志；每 15 秒发送 ping
// @Tags         GbOps
// @Produce      text/event-stream
// @Param        level    query     string  false  "最低级别"
// @Param        keyword  query     string  false  "关键字"
// @Success      200  {string}  string  "SSE 流"
// @Router       /web/gb/logs/stream [get]
// @ID           streamGbLogs
func (h *GbOpsExtraHandler) LogsStream(c *gin.Context) {
	level := c.Query("level")
	keyword := c.Query("keyword")

	prepareSSE(c)

	// 回放最近日志，避免前端建连后空白等待。
	for _, item := range h.logSvc.Recent(50) {
		if !utils.LogEntryMatches(toLogEntry(item), level, keyword) {
			continue
		}
		c.SSEvent("log", item)
	}
	c.Writer.Flush()

	ch, unsubscribe := h.logSvc.Subscribe(256)
	defer unsubscribe()

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case entry := <-ch:
			if !h.logSvc.Match(entry, level, keyword) {
				continue
			}
			c.SSEvent("log", gbsvc.ToGbLogItem(entry))
			c.Writer.Flush()
		case <-ping.C:
			c.SSEvent("ping", gin.H{"ts": time.Now().Unix()})
			c.Writer.Flush()
		}
	}
}

// ── 报警实时流 ────────────────────────────────────────────────

// AlarmsStream 报警实时推送
//
// @Summary      报警实时推送（SSE）
// @Description  text/event-stream；事件名 alarm，data 为 GbAlarmItem JSON。服务端按 id 游标每秒轮询 ac_gb_alarm，只推送建连后的新报警
// @Tags         GbOps
// @Produce      text/event-stream
// @Success      200  {string}  string  "SSE 流"
// @Router       /web/gb/alarms/stream [get]
// @ID           streamGbAlarms
func (h *GbOpsExtraHandler) AlarmsStream(c *gin.Context) {
	prepareSSE(c)

	lastID := h.alarmSvc.LatestID(c.Request.Context())
	c.SSEvent("ready", gin.H{"last_id": lastID})
	c.Writer.Flush()

	_ = h.alarmSvc.Stream(c.Request.Context(), lastID, func(item response.GbAlarmItem) error {
		c.SSEvent("alarm", item)
		c.Writer.Flush()
		// 写入失败不终止流：连接断开由 ctx.Done() 感知，gin 会忽略后续写入。
		return nil
	})
}

// ── 拉流代理 / 推流播放地址 ───────────────────────────────────

// PlayStreamProxy 拉流代理播放地址
//
// @Summary      拉流代理播放地址
// @Description  返回 WebRTC/FLV/HLS（含 WS-FLV/RTSP/RTMP）地址；代理未启动或流不在节点上返回 409
// @Tags         GbOps
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "拉流代理ID"
// @Success      200  {object}  response.RGbStreamPlay
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      409  {object}  response.BaseResponse
// @Router       /web/gb/stream-proxies/{id}/play [get]
// @ID           playGbStreamProxy
func (h *GbOpsExtraHandler) PlayStreamProxy(c *gin.Context) {
	id, ok := gbPathID(c, "拉流代理")
	if !ok {
		return
	}
	resp, err := h.sessionSvc.PlayProxy(c.Request.Context(), id)
	if err != nil {
		respondGbOpsErr(c, err, gbsvc.ErrGbStreamProxyNotFound)
		return
	}
	utils.Success(c, resp)
}

// PlayStreamPush 推流播放地址
//
// @Summary      推流播放地址
// @Description  返回 WebRTC/FLV/HLS（含 WS-FLV/RTSP/RTMP）地址；推流未启用或流不在节点上返回 409
// @Tags         GbOps
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "推流ID"
// @Success      200  {object}  response.RGbStreamPlay
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      409  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes/{id}/play [get]
// @ID           playGbStreamPush
func (h *GbOpsExtraHandler) PlayStreamPush(c *gin.Context) {
	id, ok := gbPathID(c, "推流")
	if !ok {
		return
	}
	resp, err := h.sessionSvc.PlayPush(c.Request.Context(), id)
	if err != nil {
		respondGbOpsErr(c, err, gbsvc.ErrGbStreamPushNotFound)
		return
	}
	utils.Success(c, resp)
}

// ── SSE 公共 ──────────────────────────────────────────────────

// prepareSSE 设置 SSE 响应头，禁用中间层缓冲。
func prepareSSE(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
}

// toLogEntry 响应 DTO 转回内部日志条目，用于复用过滤逻辑。
func toLogEntry(item response.GbLogItem) utils.LogEntry {
	return utils.LogEntry{
		ID:      item.ID,
		Time:    item.Time,
		Level:   item.Level,
		Logger:  item.Logger,
		Message: item.Message,
		Caller:  item.Caller,
		Fields:  item.Fields,
	}
}

// parsePathID 解析任意命名的路径参数。
func parsePathID(c *gin.Context, name, label string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的"+label+"ID")
		return 0, false
	}
	return id, true
}
