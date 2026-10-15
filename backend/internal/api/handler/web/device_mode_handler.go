package handler

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"

	// 仅用于 Swagger 注解中的类型解析（response 包在本文件正文中不出现）。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"

	devicesvc "github.com/boticz/ai-camera-backend/internal/service/device"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 资源管理 - 设备模式
// ════════════════════════════════════════════════════════════════

// DeviceModeHandler 设备模式处理器。
type DeviceModeHandler struct {
	svc *devicesvc.DeviceModeService
}

// NewDeviceModeHandler 构建设备模式处理器。
func NewDeviceModeHandler() *DeviceModeHandler {
	return &DeviceModeHandler{svc: devicesvc.NewDeviceModeService()}
}

// deviceModePathID 解析路径中的设备模式 ID。
func deviceModePathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的设备模式ID")
		return 0, false
	}
	return id, true
}

// Page 设备模式分页列表
//
// @Summary      设备模式分页列表
// @Description  支持按模式名称/编码关键字筛选
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "页码"     default(1)
// @Param        page_size  query     int     false  "每页条数"  default(20)
// @Param        keyword    query     string  false  "关键字(模式名称/编码)"
// @Success      200  {object}  response.RPageDeviceMode
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/modes [get]
// @ID           pageDeviceModes
func (h *DeviceModeHandler) Page(c *gin.Context) {
	var req request.PageDeviceModeReq
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

// Options 设备模式下拉选项
//
// @Summary      设备模式下拉选项
// @Description  仅返回启用状态的设备模式；name 为模式名称，code 为模式编码
// @Tags         Device
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.ROptionList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/modes/options [get]
// @ID           deviceModeOptions
func (h *DeviceModeHandler) Options(c *gin.Context) {
	items, err := h.svc.Options(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, items)
}

// Create 新增设备模式
//
// @Summary      新增设备模式
// @Description  模式名称不可重复
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateDeviceModeReq  true  "设备模式"
// @Success      200  {object}  response.RDeviceMode
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/modes [post]
// @ID           createDeviceMode
func (h *DeviceModeHandler) Create(c *gin.Context) {
	var req request.CreateDeviceModeReq
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

// Update 更新设备模式
//
// @Summary      更新设备模式
// @Description  增量覆盖：文本非空才覆盖，数值大于 0 才覆盖，指针字段非 nil 才覆盖
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int                          true  "设备模式ID"
// @Param        body  body      request.UpdateDeviceModeReq  true  "设备模式"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/modes/{id} [put]
// @ID           updateDeviceMode
func (h *DeviceModeHandler) Update(c *gin.Context) {
	id, ok := deviceModePathID(c)
	if !ok {
		return
	}
	var req request.UpdateDeviceModeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrModeNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除设备模式
//
// @Summary      删除设备模式
// @Description  软删除设备模式
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备模式ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/modes/{id} [delete]
// @ID           deleteDeviceMode
func (h *DeviceModeHandler) Delete(c *gin.Context) {
	id, ok := deviceModePathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrModeNotFound)
		return
	}
	utils.Success(c, nil)
}
