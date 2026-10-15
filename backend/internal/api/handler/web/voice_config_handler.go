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
// 资源管理 - 语音配置
// ════════════════════════════════════════════════════════════════

// VoiceConfigHandler 语音配置处理器。
type VoiceConfigHandler struct {
	svc *devicesvc.VoiceConfigService
}

// NewVoiceConfigHandler 构造语音配置处理器。
func NewVoiceConfigHandler() *VoiceConfigHandler {
	return &VoiceConfigHandler{svc: devicesvc.NewVoiceConfigService()}
}

// Page 语音配置分页列表
//
// @Summary      语音配置分页列表
// @Description  支持关键字（语音名称/类型/文件名）、语音类型、区域、设备筛选；返回关联区域、设备与风险类型
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"     default(1)
// @Param        page_size   query     int     false  "每页条数"  default(20)
// @Param        keyword     query     string  false  "关键字(语音名称/类型/文件名)"
// @Param        voice_type  query     string  false  "语音类型"
// @Param        region_id   query     int     false  "区域ID"
// @Param        device_id   query     int     false  "设备ID"
// @Success      200  {object}  response.RPageVoiceConfig
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/voice-configs [get]
// @ID           pageVoiceConfigs
func (h *VoiceConfigHandler) Page(c *gin.Context) {
	var req request.PageVoiceConfigReq
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

// Create 新增语音配置
//
// @Summary      新增语音配置
// @Description  为风险类型配置播报音频
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateVoiceConfigReq  true  "语音配置"
// @Success      200  {object}  response.RVoiceConfig
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/voice-configs [post]
// @ID           createVoiceConfig
func (h *VoiceConfigHandler) Create(c *gin.Context) {
	var req request.CreateVoiceConfigReq
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

// Update 更新语音配置
//
// @Summary      更新语音配置
// @Description  增量覆盖：文本非空才覆盖，指针字段非 nil 才覆盖（传 0 表示清空）
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int                           true  "语音配置ID"
// @Param        body  body      request.UpdateVoiceConfigReq  true  "语音配置"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/voice-configs/{id} [put]
// @ID           updateVoiceConfig
func (h *VoiceConfigHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的语音配置ID")
		return
	}
	var req request.UpdateVoiceConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrVoiceConfigNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除语音配置
//
// @Summary      删除语音配置
// @Description  软删除语音配置
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "语音配置ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/voice-configs/{id} [delete]
// @ID           deleteVoiceConfig
func (h *VoiceConfigHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的语音配置ID")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrVoiceConfigNotFound)
		return
	}
	utils.Success(c, nil)
}
