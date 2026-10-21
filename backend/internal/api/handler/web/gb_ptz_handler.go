package handler

import (
	"strconv"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标云台控制
//
// 所有指令走带外 SIP MESSAGE（MANSCDP DeviceControl），不是带内 INFO。
// ════════════════════════════════════════════════════════════════

// GbPtzHandler 云台处理器。
type GbPtzHandler struct {
	svc *gbsvc.GbPtzService
}

// NewGbPtzHandler 构造云台处理器。
func NewGbPtzHandler() *GbPtzHandler {
	return &GbPtzHandler{svc: gbsvc.NewGbPtzService()}
}

// gbPresetIDParam 解析路径中的预置位编号（:presetId）。
func gbPresetIDParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("presetId"))
	if err != nil || id < 1 || id > 255 {
		utils.BadRequest(c, "无效的预置位编号")
		return 0, false
	}
	return id, true
}

// Control 云台控制
//
// @Summary      云台控制
// @Description  八方向/停止 + 速度(1-8)，或变焦/光圈/聚焦动作；direction 与 action 互斥。指令经带外 MESSAGE 下发
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int               true  "通道ID"
// @Param        body       body      request.GbPtzReq  true  "云台参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId} [post]
// @ID           controlGbPtz
func (h *GbPtzHandler) Control(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPtzReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Control(c.Request.Context(), channelID, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// ListPresets 预置位列表
//
// @Summary      预置位列表
// @Description  向设备发 PresetQuery 并返回本进程缓存的预置位（本期设备异步响应未回传，重启后缓存为空）
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbPresetList
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/presets [get]
// @ID           listGbPtzPresets
func (h *GbPtzHandler) ListPresets(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	items, err := h.svc.ListPresets(c.Request.Context(), channelID)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, items)
}

// AddPreset 设置预置位
//
// @Summary      设置预置位
// @Description  PTZCmd 0x81；preset_id 取值 1-255。name 本期仅前端展示，设备侧名称以设备为准
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                  true  "通道ID"
// @Param        body       body      request.GbPresetReq  true  "预置位参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/presets [post]
// @ID           addGbPtzPreset
func (h *GbPtzHandler) AddPreset(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPresetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.AddPreset(c.Request.Context(), channelID, req.PresetID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// DeletePreset 删除预置位
//
// @Summary      删除预置位
// @Description  PTZCmd 0x83；preset_id 取值 1-255
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Param        presetId   path      int  true  "预置位编号"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/presets/{presetId} [delete]
// @ID           deleteGbPtzPreset
func (h *GbPtzHandler) DeletePreset(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	presetID, ok := gbPresetIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.DeletePreset(c.Request.Context(), channelID, presetID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// GotoPreset 调用预置位
//
// @Summary      调用预置位
// @Description  PTZCmd 0x82；preset_id 取值 1-255。设备转到该预置位。
// @Description  没有这个端点时「预置位」只有设置与删除、无法调用，功能是不完整的。
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Param        presetId   path      int  true  "预置位编号"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/presets/{presetId}/goto [post]
// @ID           gotoGbPtzPreset
func (h *GbPtzHandler) GotoPreset(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	presetID, ok := gbPresetIDParam(c)
	if !ok {
		return
	}
	if err := h.svc.GotoPreset(c.Request.Context(), channelID, presetID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// Cruise 巡航控制
//
// @Summary      巡航控制
// @Description  action=start|stop，group_id=1-255；指令经带外 MESSAGE 下发
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                     true  "通道ID"
// @Param        body       body      request.GbPtzCruiseReq  true  "巡航参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/cruise [post]
// @ID           cruiseGbPtz
func (h *GbPtzHandler) Cruise(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPtzCruiseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Cruise(c.Request.Context(), channelID, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// Scan 自动扫描控制
//
// @Summary      自动扫描控制
// @Description  action=start|stop，group_id 可省略（缺省 1）
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                   true  "通道ID"
// @Param        body       body      request.GbPtzScanReq  true  "扫描参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/scan [post]
// @ID           scanGbPtz
func (h *GbPtzHandler) Scan(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPtzScanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Scan(c.Request.Context(), channelID, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// Wiper 雨刷控制
//
// @Summary      雨刷控制
// @Description  on=true 打开、false 关闭；参数可放在请求体或 query（on=true）
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                    true  "通道ID"
// @Param        body       body      request.GbPtzWiperReq  false "雨刷参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/wiper [post]
// @ID           wiperGbPtz
func (h *GbPtzHandler) Wiper(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}

	var req request.GbPtzWiperReq
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.BadRequest(c, "参数错误: "+err.Error())
			return
		}
	} else {
		switch strings.ToLower(strings.TrimSpace(c.Query("on"))) {
		case "true", "1", "yes":
			req.On = true
		}
	}

	if err := h.svc.Wiper(c.Request.Context(), channelID, req.On); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}
