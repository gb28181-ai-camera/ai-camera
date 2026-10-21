package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标回放与下载
//
// ★ 回放控制走带内 SIP INFO + MANSRTSP，云台走带外 MESSAGE，
// 两者的 HTTP 层看似都是「发个动作」，底层发送路径完全不同。
// ════════════════════════════════════════════════════════════════

// GbPlaybackHandler 回放与下载处理器。
type GbPlaybackHandler struct {
	svc *gbsvc.GbPlaybackService
}

// NewGbPlaybackHandler 构造回放处理器。
func NewGbPlaybackHandler() *GbPlaybackHandler {
	return &GbPlaybackHandler{svc: gbsvc.NewGbPlaybackService()}
}

// QueryRecord 检索设备侧录像
//
// @Summary      检索设备侧录像
// @Description  向设备发送 RecordInfo 查询，等待回调收齐；设备超时返回空列表且 timeout=true（不是 HTTP 错误）。时间格式 2006-01-02T15:04:05
// @Tags         GbPlayback
// @Accept       json
// @Produce      json
// @Param        channel_id  query     int     true   "通道ID（数据库主键）"
// @Param        start_time  query     string  true   "开始时间"
// @Param        end_time    query     string  true   "结束时间"
// @Param        type        query     string  false  "录像类型（透传设备）"
// @Success      200  {object}  response.RGbRecordQuery
// @Failure      400  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/playback/records [get]
// @ID           queryGbPlaybackRecords
func (h *GbPlaybackHandler) QueryRecord(c *gin.Context) {
	var req request.GbRecordQueryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.QueryRecord(c.Request.Context(), req.ChannelID, &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// StartPlayback 发起回放
//
// @Summary      发起回放
// @Description  向设备发送 Playback INVITE，返回 ZLM 播放地址；stream_id 为 SIP Call-ID，控制/停止时回传
// @Tags         GbPlayback
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbPlaybackReq  true  "回放参数"
// @Success      200  {object}  response.RGbPlayback
// @Failure      400  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/playback [post]
// @ID           startGbPlayback
func (h *GbPlaybackHandler) StartPlayback(c *gin.Context) {
	var req request.GbPlaybackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.StartPlayback(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound,
			gbsvc.ErrGbDeviceNotConfigured, gbsvc.ErrPlaybackTimeInvalid)
		return
	}
	utils.Success(c, resp)
}

// Control 回放控制
//
// @Summary      回放控制
// @Description  pause | resume | seek | speed。走带内 SIP INFO + MANSRTSP，不是带外 MESSAGE
// @Tags         GbPlayback
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbPlaybackControlReq  true  "控制参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/playback/control [post]
// @ID           controlGbPlayback
func (h *GbPlaybackHandler) Control(c *gin.Context) {
	var req request.GbPlaybackControlReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Control(c.Request.Context(), &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrPlaybackSessionNotFound, gbsvc.ErrPlaybackActionInvalid)
		return
	}
	utils.Success(c, nil)
}

// StopPlayback 停止回放
//
// @Summary      停止回放
// @Description  发送 BYE 并清理 ZLM 残留流；会话不存在时幂等返回成功
// @Tags         GbPlayback
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbPlaybackStopReq  true  "会话标识"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/playback/stop [post]
// @ID           stopGbPlayback
func (h *GbPlaybackHandler) StopPlayback(c *gin.Context) {
	var req request.GbPlaybackStopReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.StopPlayback(c.Request.Context(), req.StreamID); err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// StartDownload 发起录像下载
//
// @Summary      发起录像下载
// @Description  向设备发送 Download INVITE；任务表仅存在于当前进程内存（无对应数据库表）
// @Tags         GbPlayback
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbDownloadReq  true  "下载参数"
// @Success      200  {object}  response.RGbDownload
// @Failure      400  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/playback/download [post]
// @ID           startGbDownload
func (h *GbPlaybackHandler) StartDownload(c *gin.Context) {
	var req request.GbDownloadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.StartDownload(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound,
			gbsvc.ErrGbDeviceNotConfigured, gbsvc.ErrPlaybackTimeInvalid)
		return
	}
	utils.Success(c, resp)
}

// PageDownload 下载任务分页
//
// @Summary      下载任务分页
// @Description  查询当前进程内的下载任务；服务重启后任务列表清空
// @Tags         GbPlayback
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"     default(1)
// @Param        page_size   query     int     false  "每页条数"  default(20)
// @Param        keyword     query     string  false  "关键字(通道/设备/流名)"
// @Param        channel_id  query     int     false  "通道ID"
// @Param        status      query     string  false  "状态 downloading/stopped"
// @Success      200  {object}  response.RPageGbDownload
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/playback/download [get]
// @ID           pageGbDownloads
func (h *GbPlaybackHandler) PageDownload(c *gin.Context) {
	var req request.PageGbDownloadReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageDownload(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// StopDownload 停止下载任务
//
// @Summary      停止下载任务
// @Description  发送 BYE 结束下载会话；任务记录保留并置为 stopped
// @Tags         GbPlayback
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "任务ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/playback/download/{id} [delete]
// @ID           stopGbDownload
func (h *GbPlaybackHandler) StopDownload(c *gin.Context) {
	id, ok := gbPathID(c, "下载任务")
	if !ok {
		return
	}
	if err := h.svc.StopDownload(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDownloadNotFound)
		return
	}
	utils.Success(c, nil)
}
