package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/stats"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// StatsHandler 统计分析：首页看板 / 风险分析 / 告警分析 / 可视化大屏 / 流量监控 / 设备状态。
type StatsHandler struct{}

// NewStatsHandler 构造统计分析处理器。
func NewStatsHandler() *StatsHandler { return &StatsHandler{} }

// Dashboard 首页看板
// @Summary 首页看板
// @Description 今日/本周/本月的风险与告警预警数、处理数（分别给出，另给合计），设备统计，
// @Description 风险类型分析、风险处理统计、区域风险统计（近 30 天），风险趋势与告警趋势（近 7 天）。
// @ID dashboard
// @Tags Stats
// @Success 200 {object} response.RDashboard
// @Router /web/stats/dashboard [get]
func (h *StatsHandler) Dashboard(c *gin.Context) {
	res, err := stats.NewStatsService().Dashboard(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// RiskStats 风险统计分析
// @Summary 风险统计分析
// @Description 按 start_dt ~ end_dt 统计风险总量与处理情况，并给出风险类型、所属区域、风险等级、
// @Description 设备（Top 10）分组统计与逐日趋势。时间区间缺省为近 30 天。
// @ID riskStats
// @Tags Stats
// @Param start_dt query string false "开始时间 2006-01-02 15:04:05，缺省30天前"
// @Param end_dt query string false "结束时间 2006-01-02 15:04:05，缺省当前时间"
// @Success 200 {object} response.RRiskStats
// @Failure 400 {object} response.BaseResponse
// @Router /web/stats/risk [get]
func (h *StatsHandler) RiskStats(c *gin.Context) {
	var req request.DateRangeReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := stats.NewStatsService().RiskStats(c.Request.Context(), utils.NewDataScope(c), req.StartDT, req.EndDT)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// AlarmStats 设备告警分析
// @Summary 设备告警分析
// @Description 按 start_dt ~ end_dt 统计告警总量与处理情况，并给出告警类型、所属区域、告警等级、
// @Description 设备（Top 10）分组统计与逐日趋势。时间区间缺省为近 30 天。
// @ID alarmStats
// @Tags Stats
// @Param start_dt query string false "开始时间 2006-01-02 15:04:05，缺省30天前"
// @Param end_dt query string false "结束时间 2006-01-02 15:04:05，缺省当前时间"
// @Success 200 {object} response.RAlarmStats
// @Failure 400 {object} response.BaseResponse
// @Router /web/stats/alarm [get]
func (h *StatsHandler) AlarmStats(c *gin.Context) {
	var req request.DateRangeReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := stats.NewStatsService().AlarmStats(c.Request.Context(), utils.NewDataScope(c), req.StartDT, req.EndDT)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// Screen 可视化大屏
// @Summary 可视化大屏
// @Description 风险实时告警与设备状态告警（各最新 10 条）、风险统计与设备告警统计（近 30 天）、
// @Description 设备监控（总数/在线/离线）、流量监控（剩余流量最少的 10 张卡）、今日风险与告警数。
// @ID screen
// @Tags Stats
// @Success 200 {object} response.RScreen
// @Router /web/stats/screen [get]
func (h *StatsHandler) Screen(c *gin.Context) {
	res, err := stats.NewStatsService().Screen(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// TrafficStats 物联网卡流量监控
// @Summary 物联网卡流量监控
// @Description 返回物联网卡流量总额/已用/剩余的汇总值与明细（含卡号、名称、所属区域、所属设备，按剩余流量升序）。
// @ID trafficStats
// @Tags Stats
// @Success 200 {object} response.RTrafficStats
// @Router /web/stats/traffic [get]
func (h *StatsHandler) TrafficStats(c *gin.Context) {
	res, err := stats.NewStatsService().TrafficStats(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// DeviceStatus 设备状态统计
// @Summary 设备状态统计
// @Description 设备总数/在线/离线，以及低电量（电量 < 20%）与存储将满（剩余存储 < 总容量 10%）的设备数。
// @ID deviceStatus
// @Tags Stats
// @Success 200 {object} response.RDeviceStatus
// @Router /web/stats/device-status [get]
func (h *StatsHandler) DeviceStatus(c *gin.Context) {
	res, err := stats.NewStatsService().DeviceStatus(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}
