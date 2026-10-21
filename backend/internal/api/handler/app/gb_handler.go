package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	// 空导入：本文件仅在 Swagger 注解中引用 response 包类型
	// （@Success {object} response.RAppPageGbDevice 等），代码中不直接构造响应体，
	// 显式导入以便 swag 解析注解中的类型引用。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"
	appsvc "github.com/boticz/ai-camera-backend/internal/service/app"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 移动端国标视频（GB/T 28181）处理器
//
// 本层是**薄封装**：解析路径/请求体 → 委托 service/app.AppGbService →
// 统一错误映射到 HTTP 状态。所有 SIP/媒体编排都在
// internal/service/gb28181 中，与 /web/gb 共用同一份实现。
//
// 路由归属：router/app.go 的 aAuth 组（authMW → casbinMW），
// 因此这里的 handler 不需要再做 JWT 解析；user_id 从 gin.Context 读取。
// ════════════════════════════════════════════════════════════════

// AppGbHandler 移动端国标视频处理器（无状态）。
type AppGbHandler struct {
	svc *appsvc.AppGbService
}

// NewAppGbHandler 构造移动端国标视频处理器。
func NewAppGbHandler() *AppGbHandler {
	return &AppGbHandler{svc: appsvc.NewAppGbService()}
}

// appGbChannelIDParam 解析路径中的通道主键（:channelId）。
//
// 契约 §10.0：路径参数是数据库主键，不是国标编码。
func appGbChannelIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("channelId"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的通道ID")
		return 0, false
	}
	return id, true
}

// respondAppGbErr 把国标业务错误映射为移动端可理解的 HTTP 状态。
//
// 映射原则：
//   - 404：目标资源不存在（通道/设备/会话）；
//   - 503：依赖服务不可用（SIP 未启动、ZLM 节点不可达/未就绪）——
//     必须显式失败，不能静默返回成功让 App 以为操作已生效；
//   - 400：调用方参数或业务前置条件不满足（离线、未配置国标编码、速度越界等）；
//   - 其它：500，保留原始中文错误信息便于定位。
func respondAppGbErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gbsvc.ErrGbChannelNotFound),
		errors.Is(err, gbsvc.ErrGbDeviceNotFound),
		errors.Is(err, gbsvc.ErrGbPlaySessionNotFound),
		errors.Is(err, gbsvc.ErrPlaybackSessionNotFound),
		errors.Is(err, gbsvc.ErrGbAlarmNotFound),
		errors.Is(err, gbsvc.ErrGbVoiceSessionNotFound):
		utils.NotFound(c, err.Error())

	case errors.Is(err, gbsvc.ErrSipServiceUnavailable),
		errors.Is(err, gbsvc.ErrGbMediaServerUnavailable),
		errors.Is(err, gbsvc.ErrGbPlayStreamNotReady):
		c.JSON(http.StatusServiceUnavailable, utils.Response{
			Code:    http.StatusServiceUnavailable,
			Message: err.Error(),
		})

	case errors.Is(err, appsvc.ErrAppGbPtzDirectionInvalid),
		errors.Is(err, appsvc.ErrAppGbPtzSpeedInvalid),
		errors.Is(err, gbsvc.ErrGbPtzSpeedInvalid),
		errors.Is(err, gbsvc.ErrGbPtzDirectionConflict),
		errors.Is(err, gbsvc.ErrGbPtzCommandMissing),
		errors.Is(err, gbsvc.ErrGbPlayProtocolInvalid),
		errors.Is(err, gbsvc.ErrGbDeviceOffline),
		errors.Is(err, gbsvc.ErrGbDeviceNotConfigured),
		errors.Is(err, gbsvc.ErrPlaybackTimeInvalid),
		errors.Is(err, gbsvc.ErrPlaybackActionInvalid),
		errors.Is(err, gbsvc.ErrGbVoiceSessionBusy),
		errors.Is(err, gbsvc.ErrGbBroadcastDisabled),
		errors.Is(err, gbsvc.ErrGbVoiceSessionTypeMismatch),
		errors.Is(err, gbsvc.ErrGbSnapshotEmpty):
		utils.BadRequest(c, err.Error())

	default:
		utils.ServerError(c, err.Error())
	}
}

// ════════════════════════════════════════════════════════════════
// 设备 / 通道
// ════════════════════════════════════════════════════════════════

// PageDevices 分页查询国标设备。
// @Summary 分页查询国标设备
// @Description 只返回已配置国标接入（gb_device_id 非空）的设备；支持关键字、注册状态、运行状态、区域筛选；不含 SIP 密码等接入配置
// @Tags AppGb
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（设备名称/编号/国标编码）"
// @Param register_status query string false "注册状态 pending/activated/rejected"
// @Param status query string false "运行状态 online/offline"
// @Param region_id query int false "区域ID"
// @Success 200 {object} response.RAppPageGbDevice "国标设备分页列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/gb/devices [get]
// @ID appPageGbDevices
func (h *AppGbHandler) PageDevices(c *gin.Context) {
	var req request.PageGbDeviceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageDevices(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// PageChannels 分页查询国标通道。
// @Summary 分页查询国标通道
// @Description 支持按设备、区域、业务分组、状态与关键字过滤；返回设备/区域/分组名称，不含通道密码
// @Tags AppGb
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（通道名称/国标编码）"
// @Param device_id query int false "设备ID"
// @Param region_id query int false "区域ID"
// @Param group_id query int false "业务分组ID"
// @Param status query string false "通道状态 ON/OFF"
// @Param gb_device_id query string false "设备国标编码"
// @Success 200 {object} response.RAppPageGbChannel "国标通道分页列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/gb/channels [get]
// @ID appPageGbChannels
func (h *AppGbHandler) PageChannels(c *gin.Context) {
	var req request.PageGbChannelReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageChannels(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// ════════════════════════════════════════════════════════════════
// 实时播放 / 抓图
// ════════════════════════════════════════════════════════════════

// Play 发起实时点播。
// @Summary 发起实时点播
// @Description 分配 SSRC 并向设备发 INVITE，等待 ZLM 建流后返回 WebRTC/HTTP-FLV/HLS 地址。同一通道已有在线流时直接复用
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param channelId path int true "通道ID"
// @Param body body request.GbPlayReq false "播放参数（protocol 缺省 webrtc）"
// @Success 200 {object} response.RAppGbPlay "播放地址"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/play/{channelId} [post]
// @ID appPlayGbChannel
func (h *AppGbHandler) Play(c *gin.Context) {
	channelID, ok := appGbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPlayReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	resp, err := h.svc.Play(c.Request.Context(), channelID, &req)
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// StopPlay 停止实时点播。
// @Summary 停止实时点播
// @Description 向设备发 BYE、关闭 ZLM 媒体流并归还 SSRC；stream_id 可省略（按通道停止）
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param channelId path int true "通道ID"
// @Param body body request.GbPlayReq false "stream_id（可选）"
// @Success 200 {object} response.BaseResponse "停止成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/play/{channelId}/stop [post]
// @ID appStopGbPlay
func (h *AppGbHandler) StopPlay(c *gin.Context) {
	channelID, ok := appGbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPlayReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	streamID := strings.TrimSpace(req.StreamID)
	if streamID == "" {
		streamID = strings.TrimSpace(c.Query("stream_id"))
	}
	if err := h.svc.StopPlay(c.Request.Context(), channelID, streamID); err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// Snapshot 通道抓图。
// @Summary 通道抓图
// @Description 调用 ZLM getSnap 抓取当前画面并保存到上传目录，返回可直接访问的图片 URL；未开播时先自动点播
// @Tags AppGb
// @Produce json
// @Security BearerAuth
// @Param channelId path int true "通道ID"
// @Success 200 {object} response.RAppGbSnapshot "抓图结果"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/play/{channelId}/snapshot [post]
// @ID appSnapshotGbChannel
func (h *AppGbHandler) Snapshot(c *gin.Context) {
	channelID, ok := appGbChannelIDParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.Snapshot(c.Request.Context(), channelID)
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// ════════════════════════════════════════════════════════════════
// 云台
// ════════════════════════════════════════════════════════════════

// Ptz 云台控制。
// @Summary 云台控制
// @Description 八方向/停止 + 速度(1-8)，或变焦/光圈/聚焦动作；direction 与 action 互斥。指令经带外 SIP MESSAGE 下发
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param channelId path int true "通道ID"
// @Param body body request.GbPtzReq true "云台参数"
// @Success 200 {object} response.BaseResponse "控制指令已下发"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/ptz/{channelId} [post]
// @ID appControlGbPtz
func (h *AppGbHandler) Ptz(c *gin.Context) {
	channelID, ok := appGbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPtzReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Ptz(c.Request.Context(), channelID, &req); err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 录像查询 / 回放
// ════════════════════════════════════════════════════════════════

// QueryRecord 录像查询。
// @Summary 查询设备侧录像
// @Description 向设备下发 RecordInfo 查询并等待响应；设备无响应时返回 timeout=true 与已收到的条目，不返回 HTTP 错误
// @Tags AppGb
// @Produce json
// @Security BearerAuth
// @Param channel_id query int true "通道ID"
// @Param start_time query string true "开始时间，格式 2006-01-02T15:04:05"
// @Param end_time query string true "结束时间，格式 2006-01-02T15:04:05"
// @Param type query string false "录像类型（透传设备）"
// @Success 200 {object} response.RAppGbRecordQuery "录像列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/playback/records [get]
// @ID appQueryGbRecords
func (h *AppGbHandler) QueryRecord(c *gin.Context) {
	var req request.GbRecordQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.QueryRecord(c.Request.Context(), req.ChannelID, &req)
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// StartPlayback 开始回放。
// @Summary 开始录像回放
// @Description 按时间窗向设备发回放 INVITE，返回 ZLM 播放地址；时间格式 2006-01-02T15:04:05
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body request.GbPlaybackReq true "回放参数"
// @Success 200 {object} response.RAppGbPlayback "回放会话"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/playback [post]
// @ID appStartGbPlayback
func (h *AppGbHandler) StartPlayback(c *gin.Context) {
	var req request.GbPlaybackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.StartPlayback(c.Request.Context(), &req)
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// ControlPlayback 回放控制。
// @Summary 回放控制
// @Description pause/resume/seek/speed，走带内 SIP INFO + MANSRTSP（与云台的带外 MESSAGE 不同）
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body request.GbPlaybackControlReq true "控制参数"
// @Success 200 {object} response.BaseResponse "控制成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/playback/control [post]
// @ID appControlGbPlayback
func (h *AppGbHandler) ControlPlayback(c *gin.Context) {
	var req request.GbPlaybackControlReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.ControlPlayback(c.Request.Context(), &req); err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// StopPlayback 停止回放。
// @Summary 停止录像回放
// @Description 向设备发 BYE 并释放媒体流；会话已结束时幂等返回成功
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body request.GbPlaybackStopReq true "回放会话（stream_id）"
// @Success 200 {object} response.BaseResponse "停止成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/playback/stop [post]
// @ID appStopGbPlayback
func (h *AppGbHandler) StopPlayback(c *gin.Context) {
	var req request.GbPlaybackStopReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.StopPlayback(c.Request.Context(), req.StreamID); err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 语音对讲
// ════════════════════════════════════════════════════════════════

// StartTalk 开始语音对讲。
// @Summary 开始语音对讲
// @Description 向通道发起 Talk INVITE 并由 ZLM 回推音频；agent_enabled 可选，控制面不可达时自动降级为纯对讲
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param channelId path int true "通道ID"
// @Param body body request.GbVoiceStartReq false "agent_enabled（可选）"
// @Success 200 {object} response.RAppGbVoiceSession "语音会话"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/talk/{channelId}/start [post]
// @ID appStartGbTalk
func (h *AppGbHandler) StartTalk(c *gin.Context) {
	channelID, ok := appGbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbVoiceStartReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	resp, err := h.svc.StartTalk(c.Request.Context(), channelID, c.GetInt64("user_id"), req.AgentEnabled)
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// StopTalk 结束语音对讲。
// @Summary 结束语音对讲
// @Description session_id 可省略；省略时按通道结束进行中的对讲会话
// @Tags AppGb
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param channelId path int true "通道ID"
// @Param body body request.GbVoiceStopReq false "session_id（可选）"
// @Success 200 {object} response.BaseResponse "停止成功"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Failure 503 {object} response.BaseResponse
// @Router /app/gb/talk/{channelId}/stop [post]
// @ID appStopGbTalk
func (h *AppGbHandler) StopTalk(c *gin.Context) {
	channelID, ok := appGbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbVoiceStopReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(c.Query("session_id"))
	}
	if err := h.svc.StopTalk(c.Request.Context(), channelID, sessionID); err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 国标报警
// ════════════════════════════════════════════════════════════════

// PageAlarms 分页查询国标报警。
// @Summary 分页查询国标报警
// @Description 只返回国标设备主动上报的报警；支持设备/通道、报警类型、级别、处理状态、时间范围与关键字过滤
// @Tags AppGb
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字（报警描述/设备或通道国标编码/报警类型）"
// @Param device_id query int false "设备ID"
// @Param channel_id query int false "通道ID"
// @Param alarm_type query string false "报警类型编码"
// @Param alarm_priority query string false "报警级别 1/2/3/4"
// @Param handled query bool false "是否已处理"
// @Param start_dt query string false "报警时间起"
// @Param end_dt query string false "报警时间止"
// @Success 200 {object} response.RAppPageGbAlarm "国标报警分页列表"
// @Failure 400 {object} response.BaseResponse
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /app/gb/alarms [get]
// @ID appPageGbAlarms
func (h *AppGbHandler) PageAlarms(c *gin.Context) {
	var req request.PageGbAlarmReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageAlarms(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondAppGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}
