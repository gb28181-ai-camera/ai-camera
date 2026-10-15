package handler

import (
	"errors"
	"strconv"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	devicesvc "github.com/boticz/ai-camera-backend/internal/service/device"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 资源管理 - 设备
// ════════════════════════════════════════════════════════════════

// DeviceHandler 设备处理器。
type DeviceHandler struct {
	svc *devicesvc.DeviceService
}

// NewDeviceHandler 构建设备处理器。
func NewDeviceHandler() *DeviceHandler {
	return &DeviceHandler{svc: devicesvc.NewDeviceService()}
}

// respondDeviceErr 把服务层错误映射为统一响应：命中 notFound 集合返回 404，其余返回 400。
func respondDeviceErr(c *gin.Context, err error, notFound ...error) {
	for _, target := range notFound {
		if target != nil && errors.Is(err, target) {
			utils.NotFound(c, err.Error())
			return
		}
	}
	utils.BadRequest(c, err.Error())
}

// devicePathID 解析路径中的设备 ID。
func devicePathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的设备ID")
		return 0, false
	}
	return id, true
}

// Page 设备分页列表
//
// @Summary      设备分页列表
// @Description  支持关键字（设备名称/编号/位置）、区域、运行状态、设备模式、工作模式筛选；返回关联区域、物联网卡、固件、AI模型信息
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"     default(1)
// @Param        page_size   query     int     false  "每页条数"  default(20)
// @Param        keyword     query     string  false  "关键字(设备名称/编号/位置)"
// @Param        region_id   query     int     false  "区域ID"
// @Param        status      query     string  false  "运行状态 online/offline"
// @Param        device_mode query     string  false  "设备模式"
// @Param        work_mode   query     string  false  "工作模式"
// @Success      200  {object}  response.RPageDevice
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices [get]
// @ID           pageDevices
func (h *DeviceHandler) Page(c *gin.Context) {
	var req request.PageDeviceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Options 设备下拉选项
//
// @Summary      设备下拉选项
// @Description  仅返回启用状态的设备；name 为设备名称，code 为设备编号
// @Tags         Device
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.ROptionList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/options [get]
// @ID           deviceOptions
func (h *DeviceHandler) Options(c *gin.Context) {
	items, err := h.svc.Options(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, items)
}

// Get 设备详情
//
// @Summary      设备详情
// @Description  按 ID 查询设备，含关联区域、物联网卡、固件、AI模型信息
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RDevice
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/{id} [get]
// @ID           getDevice
func (h *DeviceHandler) Get(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, m)
}

// Create 新增设备
//
// @Summary      新增设备
// @Description  设备编号全局唯一；新建后运行状态固定为 offline，等待设备心跳刷新
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateDeviceReq  true  "设备信息"
// @Success      200  {object}  response.RDevice
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices [post]
// @ID           createDevice
func (h *DeviceHandler) Create(c *gin.Context) {
	var req request.CreateDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, m)
}

// Update 更新设备
//
// @Summary      更新设备
// @Description  增量覆盖：文本非空才覆盖，数值大于 0 才覆盖，指针字段非 nil 才覆盖；iot_card_id 传 0 表示解绑物联网卡
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true  "设备ID"
// @Param        body  body      request.UpdateDeviceReq  true  "设备信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/{id} [put]
// @ID           updateDevice
func (h *DeviceHandler) Update(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	var req request.UpdateDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除设备
//
// @Summary      删除设备
// @Description  软删除设备，同时解除其物联网卡绑定
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/{id} [delete]
// @ID           deleteDevice
func (h *DeviceHandler) Delete(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// SwitchMode 切换设备模式
//
// @Summary      切换设备模式
// @Description  模式名称必须存在于设备模式表中，否则报「设备模式不存在」
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int                    true  "设备ID"
// @Param        body  body      request.SwitchModeReq  true  "设备模式"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/{id}/mode [put]
// @ID           switchDeviceMode
func (h *DeviceHandler) SwitchMode(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	var req request.SwitchModeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SwitchMode(c.Request.Context(), id, req.Mode); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound, devicesvc.ErrModeNotFound)
		return
	}
	utils.Success(c, nil)
}

// SyncTime 同步设备时间
//
// @Summary      同步设备时间
// @Description  把设备的最近心跳时间置为服务端当前时间，并返回同步后的时间字符串
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RSyncTime
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/{id}/sync-time [put]
// @ID           syncDeviceTime
func (h *DeviceHandler) SyncTime(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	now := time.Now()
	if err := h.svc.SyncTime(c.Request.Context(), id); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, response.DeviceSyncTimeResp{SyncTime: now.Format("2006-01-02 15:04:05")})
}

// ToggleMonitor 加入/移出实时监控
//
// @Summary      加入/移出实时监控
// @Description  设置设备是否加入实时监控
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "设备ID"
// @Param        body  body      request.ToggleMonitorReq  true  "监控开关"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/{id}/monitor [put]
// @ID           toggleDeviceMonitor
func (h *DeviceHandler) ToggleMonitor(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	var req request.ToggleMonitorReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.ToggleMonitor(c.Request.Context(), id, req.Enabled); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// Heartbeat 设备心跳上报
//
// @Summary      设备心跳上报
// @Description  ApiKey 鉴权；设备上报运行状态，更新设备快照并追加一条运行历史，状态翻转时补记上线/离线时间
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int                          true  "设备ID"
// @Param        body  body      request.DeviceHeartbeatReq  true  "心跳数据"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/{id}/heartbeat [post]
// @ID           deviceHeartbeat
func (h *DeviceHandler) Heartbeat(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	var req request.DeviceHeartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Heartbeat(c.Request.Context(), id, &req); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}

// PageHistory 设备历史记录
//
// @Summary      设备历史记录分页列表
// @Description  支持按设备、区域、运行状态、设备编号与时间区间筛选；返回关联设备及所属区域
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"        default(1)
// @Param        page_size   query     int     false  "每页条数"     default(20)
// @Param        device_id   query     int     false  "设备ID"
// @Param        region_id   query     int     false  "区域ID"
// @Param        status      query     string  false  "运行状态 online/offline"
// @Param        device_code query     string  false  "设备编号(模糊)"
// @Param        start_dt    query     string  false  "采样开始时间"
// @Param        end_dt      query     string  false  "采样结束时间"
// @Success      200  {object}  response.RPageDeviceHistory
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/history [get]
// @ID           pageDeviceHistory
func (h *DeviceHandler) PageHistory(c *gin.Context) {
	var req request.PageDeviceHistoryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageHistory(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// HistoryChart 设备历史曲线
//
// @Summary      设备历史曲线
// @Description  默认返回近 3 天数据；days<=7 按小时聚合，否则按天聚合。status_points 中 online=1、offline=0；status_summary 为在线/离线时长（分钟）
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int  true   "设备ID"
// @Param        days  query     int  false  "统计天数，默认3"
// @Success      200  {object}  response.RDeviceHistoryChart
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/{id}/history-chart [get]
// @ID           deviceHistoryChart
func (h *DeviceHandler) HistoryChart(c *gin.Context) {
	id, ok := devicePathID(c)
	if !ok {
		return
	}
	days := 3
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			days = n
		}
	}
	resp, err := h.svc.HistoryChart(c.Request.Context(), id, days)
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, resp)
}
