package handler

import (
	"errors"
	"net/http"
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
// @Description  PTZCmd 0x81；preset_id 取值 1-255。name 会持久化到 ac_gb_ptz_preset 并在预置位列表返回
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
	if err := h.svc.AddPreset(c.Request.Context(), channelID, req.PresetID, req.Name); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// RenamePreset 重命名预置位
//
// @Summary      重命名预置位
// @Description  仅更新平台侧名称（ac_gb_ptz_preset），不下发设备指令；无需设备在线
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                     true  "通道ID"
// @Param        presetId   path      int                     true  "预置位编号"
// @Param        body       body      request.GbPresetRenameReq true  "新名称"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/presets/{presetId} [put]
// @ID           renameGbPtzPreset
func (h *GbPtzHandler) RenamePreset(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	presetID, ok := gbPresetIDParam(c)
	if !ok {
		return
	}
	var req request.GbPresetRenameReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.RenamePreset(c.Request.Context(), channelID, presetID, req.Name); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, nil)
}

// AddCruisePoint 加入巡航点
//
// @Summary      加入巡航点
// @Description  PTZCmd 0x84；group_id=巡航组号 1-255，preset_id=预置位号 1-255。设备需先设置该预置位
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                      true  "通道ID"
// @Param        body       body      request.GbCruisePointReq true  "巡航点"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/cruise-points [post]
// @ID           addGbPtzCruisePoint
func (h *GbPtzHandler) AddCruisePoint(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbCruisePointReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.AddCruisePoint(c.Request.Context(), channelID, req.GroupID, req.PresetID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// SetCruisePoint 设置巡航参数
//
// @Summary      设置巡航速度/停留时间
// @Description  PTZCmd 0x86 设置速度、0x87 设置停留时间；speed 与 stay_time 至少传一个（1-4095）
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                         true  "通道ID"
// @Param        body       body      request.GbCruisePointSetReq true  "巡航参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/cruise-points [put]
// @ID           setGbPtzCruisePoint
func (h *GbPtzHandler) SetCruisePoint(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbCruisePointSetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SetCruisePoint(c.Request.Context(), channelID, req.GroupID, req.Speed, req.StayTime); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// DeleteCruisePoint 删除巡航点
//
// @Summary      删除巡航点
// @Description  PTZCmd 0x85；group_id=巡航组号，preset_id=预置位号（传 0 删除整条巡航）
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true   "通道ID"
// @Param        group_id   query     int  true   "巡航组号 1-255"
// @Param        preset_id  query     int  false  "预置位号 0-255（0=删除整条巡航）" default(0)
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/cruise-points [delete]
// @ID           deleteGbPtzCruisePoint
func (h *GbPtzHandler) DeleteCruisePoint(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	groupID, err := strconv.Atoi(c.Query("group_id"))
	if err != nil || groupID < 1 || groupID > 255 {
		utils.BadRequest(c, "无效的巡航组号（1-255）")
		return
	}
	presetID := 0
	if v := c.Query("preset_id"); v != "" {
		presetID, err = strconv.Atoi(v)
		if err != nil || presetID < 0 || presetID > 255 {
			utils.BadRequest(c, "无效的预置位编号（0-255）")
			return
		}
	}
	if err := h.svc.DeleteCruisePoint(c.Request.Context(), channelID, groupID, presetID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// SetScanBounds 设置扫描边界
//
// @Summary      设置扫描边界/速度
// @Description  PTZCmd 0x89 数据2=1 左边界 / 2 右边界（把当前位置记为边界，不带角度值），0x8A 设置速度 1-4095。设置后用 POST /ptz/{channelId}/scan 启动
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                      true  "通道ID"
// @Param        body       body      request.GbScanBoundsReq  true  "扫描参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/scan/bounds [put]
// @ID           setGbPtzScanBounds
func (h *GbPtzHandler) SetScanBounds(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbScanBoundsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SetScanBounds(c.Request.Context(), channelID, &req); err != nil {
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

// ────────────────────────────────────────────────────────────────
// GB/T 28181-2022 云台扩展查询
//
// 与上面的 2016 指令不同：这些接口是「同步等待设备业务应答」的查询，
// 最长阻塞 5 秒（由 sipserver 控制）。因此错误映射单独处理：
//   - 版本不支持 → 400（设备是 2016，改配置/升级固件才能用）；
//   - 设备超时   → 504（设备在线但没回应，查网络/设备）。
// 两者都不走 respondGbErr 的默认 400，避免用户把「版本问题」当成「参数问题」。
// ────────────────────────────────────────────────────────────────

// respondGb2022Err 2022 查询专用错误映射。
func respondGb2022Err(c *gin.Context, err error, notFound ...error) {
	switch {
	case errors.Is(err, gbsvc.ErrGb2022Unsupported):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, gbsvc.ErrGbQueryTimeout):
		c.JSON(http.StatusGatewayTimeout, utils.Response{
			Code:    http.StatusGatewayTimeout,
			Message: err.Error(),
		})
	default:
		respondGbErr(c, err, notFound...)
	}
}

// gbCruiseTrackNumberParam 解析路径中的轨迹编号（:number，从 0 开始）。
func gbCruiseTrackNumberParam(c *gin.Context) (int, bool) {
	n, err := strconv.Atoi(c.Param("number"))
	if err != nil || n < 0 {
		utils.BadRequest(c, "无效的巡航轨迹编号（从 0 开始）")
		return 0, false
	}
	return n, true
}

// PTZPosition 查询 PTZ 精准位置
//
// @Summary      查询 PTZ 精准位置
// @Description  GB28181-2022 扩展（A.2.6.15）。返回 Pan/Tilt/Zoom；设备为 2016 时明确返回「版本不支持」。同步等待最多 5 秒，超时返回 504
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbPtzPosition
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Failure      504  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/position [get]
// @ID           gbPtzPosition
func (h *GbPtzHandler) PTZPosition(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.PTZPosition(c.Request.Context(), channelID)
	if err != nil {
		respondGb2022Err(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// Precise PTZ 精准控制
//
// @Summary      PTZ 精准控制
// @Description  GB28181-2022 扩展（A.2.3.1.11）。pan/tilt/zoom 均可选，但至少传一个；绝对角度 + 倍率定位
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int                      true  "通道ID"
// @Param        body       body      request.GbPtzPreciseReq  true  "精准控制参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/precise [post]
// @ID           preciseGbPtz
func (h *GbPtzHandler) Precise(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	var req request.GbPtzPreciseReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Precise(c.Request.Context(), channelID, &req); err != nil {
		respondGb2022Err(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// CruiseTracks 巡航轨迹列表
//
// @Summary      巡航轨迹列表
// @Description  GB28181-2022 扩展（A.2.6.13）。设备无巡航能力时返回空数组
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbCruiseTrackList
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Failure      504  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/cruise-tracks [get]
// @ID           listGbPtzCruiseTracks
func (h *GbPtzHandler) CruiseTracks(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	items, err := h.svc.CruiseTracks(c.Request.Context(), channelID)
	if err != nil {
		respondGb2022Err(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, items)
}

// CruiseTrack 巡航轨迹明细
//
// @Summary      巡航轨迹明细
// @Description  GB28181-2022 扩展（A.2.6.14）。返回该轨迹的预置点、停留时间与速度
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Param        number     path      int  true  "轨迹编号（从 0 开始）"
// @Success      200  {object}  response.RGbCruiseTrackDetail
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Failure      504  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/cruise-tracks/{number} [get]
// @ID           getGbPtzCruiseTrack
func (h *GbPtzHandler) CruiseTrack(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	number, ok := gbCruiseTrackNumberParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.CruiseTrack(c.Request.Context(), channelID, number)
	if err != nil {
		respondGb2022Err(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// HomePosition 看守位
//
// @Summary      查询看守位
// @Description  GB28181-2022 扩展（A.2.6.12）。supported=false 表示设备未上报 HomePosition 块（不支持），不是「已关闭」
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbHomePosition
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Failure      504  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/home-position [get]
// @ID           getGbPtzHomePosition
func (h *GbPtzHandler) HomePosition(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.HomePosition(c.Request.Context(), channelID)
	if err != nil {
		respondGb2022Err(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// SDCardStatus 存储卡状态
//
// @Summary      查询存储卡状态
// @Description  GB28181-2022 扩展（A.2.6.16）。容量单位 MB；部分设备对 SDCard 列表的标签名实现不一致，未知字段会被忽略
// @Tags         GbPtz
// @Accept       json
// @Produce      json
// @Param        channelId  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbSDCardStatus
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Failure      504  {object}  response.BaseResponse
// @Router       /web/gb/ptz/{channelId}/sdcard [get]
// @ID           getGbPtzSDCard
func (h *GbPtzHandler) SDCardStatus(c *gin.Context) {
	channelID, ok := gbChannelIDParam(c)
	if !ok {
		return
	}
	resp, err := h.svc.SDCardStatus(c.Request.Context(), channelID)
	if err != nil {
		respondGb2022Err(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}
