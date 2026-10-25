package handler

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标设备控制与运维（WVP 对齐）
//
// 路由参数 :id 一律是 ac_device 主键；SIP 指令里的 DeviceID 由 service
// 层替换为 20 位国标编码，两者不可混用。
// ════════════════════════════════════════════════════════════════

// GbDeviceControlHandler 设备控制 / 同步进度 / 时间统计处理器。
type GbDeviceControlHandler struct {
	control *gbsvc.GbDeviceControlService
	sync    *gbsvc.GbDeviceSyncStatusService
	stats   *gbsvc.GbTimeStatisticsService
}

// NewGbDeviceControlHandler 构造设备控制处理器。
func NewGbDeviceControlHandler() *GbDeviceControlHandler {
	return &GbDeviceControlHandler{
		control: gbsvc.NewGbDeviceControlService(),
		sync:    gbsvc.NewGbDeviceSyncStatusService(),
		stats:   gbsvc.NewGbTimeStatisticsService(),
	}
}

// SyncStatus 目录同步进度
//
// @Summary      目录同步进度
// @Description  返回设备最近一次 Catalog 同步的状态（idle/syncing/success/failed）、总数、已接收数、错误与起止时间。状态是进程内的：分进程部署或重启后返回 idle
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RGbDeviceSyncStatus
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/sync-status [get]
// @ID           getGbDeviceSyncStatus
func (h *GbDeviceControlHandler) SyncStatus(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	resp, err := h.sync.SyncStatus(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// TimeStatistics 心跳/注册间隔统计
//
// @Summary      心跳/注册间隔统计
// @Description  聚合 ac_gb_sip_log 中设备入向报文的相邻间隔，返回时间序列与平均/最大/最小（秒）。type=keepalive|register，count=采样条数（2-500，默认 50）
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id     path      int     true   "设备ID"
// @Param        type   query     string  false  "统计类型 keepalive/register"  default(keepalive)
// @Param        count  query     int     false  "采样条数 2-500"                default(50)
// @Success      200  {object}  response.RGbTimeStatistics
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/time-statistics [get]
// @ID           getGbDeviceTimeStatistics
func (h *GbDeviceControlHandler) TimeStatistics(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	count := 50
	if v := c.Query("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			count = n
		}
	}
	resp, err := h.stats.TimeStatistics(c.Request.Context(), id, c.Query("type"), count)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// Reboot 重启设备
//
// @Summary      重启设备
// @Description  发送 MANSCDP DeviceControl / TeleBoot=Boot。SIP 服务未启动时返回 503
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/reboot [post]
// @ID           rebootGbDevice
func (h *GbDeviceControlHandler) Reboot(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	if err := h.control.Reboot(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound, gbsvc.ErrGbDeviceNotConfigured)
		return
	}
	utils.Success(c, nil)
}

// Guard 布防/撤防
//
// @Summary      设备布防/撤防
// @Description  cmd=SetGuard（布防）| ResetGuard（撤防），复用 MANSCDP DeviceControl / GuardCmd。SIP 服务未启动时返回 503
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "设备ID"
// @Param        body  body      request.GbDeviceGuardReq  true  "布防/撤防指令"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/guard [post]
// @ID           guardGbDevice
func (h *GbDeviceControlHandler) Guard(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	var req request.GbDeviceGuardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.control.Guard(c.Request.Context(), id, req.Cmd); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound, gbsvc.ErrGbDeviceNotConfigured)
		return
	}
	utils.Success(c, nil)
}

// AlarmReset 报警复位
//
// @Summary      报警复位
// @Description  发送 MANSCDP DeviceControl / AlarmCmd=ResetAlarm；可选 channel_id 指定复位通道（必须是该设备下的通道）。SIP 服务未启动时返回 503
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true  "设备ID"
// @Param        body  body      request.GbAlarmResetReq  false "报警复位参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/alarm-reset [post]
// @ID           alarmResetGbDevice
func (h *GbDeviceControlHandler) AlarmReset(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	var req request.GbAlarmResetReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	}
	if err := h.control.AlarmReset(c.Request.Context(), id, req.ChannelID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, nil)
}
