package handler

import (
	"errors"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/upgrade"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// UpgradeHandler 升级管理处理器（固件类型 / 固件 / AI模型 / 升级下发 / 升级计划 / 升级记录）。
type UpgradeHandler struct {
	svc *upgrade.UpgradeService
}

// NewUpgradeHandler 构造升级管理处理器。
func NewUpgradeHandler() *UpgradeHandler {
	return &UpgradeHandler{svc: upgrade.NewUpgradeService()}
}

// upgradeFail 统一升级域错误响应：记录不存在返回 404，其余业务/参数错误返回 400。
func upgradeFail(c *gin.Context, err error, notFoundMsg string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.NotFound(c, notFoundMsg)
		return
	}
	utils.BadRequest(c, err.Error())
}

// upgradePathID 解析路径中的 ID，非法返回 0。
func upgradePathID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// ════════════════════════════════════════════════════════════════
// 固件类型
// ════════════════════════════════════════════════════════════════

// PageFirmwareTypes 固件类型分页列表
// @Summary 固件类型分页列表
// @ID pageFirmwareTypes
// @Tags Upgrade
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（类型名称/编码）"
// @Success 200 {object} response.RPageFirmwareType
// @Router /web/upgrades/firmware-types [get]
func (h *UpgradeHandler) PageFirmwareTypes(c *gin.Context) {
	var req request.PageFirmwareTypeReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageFirmwareTypes(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "固件类型不存在")
		return
	}
	utils.Success(c, resp)
}

// FirmwareTypeOptions 固件类型下拉选项
// @Summary 固件类型下拉选项
// @ID firmwareTypeOptions
// @Tags Upgrade
// @Success 200 {object} response.ROptionList
// @Router /web/upgrades/firmware-types/options [get]
func (h *UpgradeHandler) FirmwareTypeOptions(c *gin.Context) {
	opts, err := h.svc.FirmwareTypeOptions(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "固件类型不存在")
		return
	}
	utils.Success(c, opts)
}

// CreateFirmwareType 创建固件类型
// @Summary 创建固件类型
// @ID createFirmwareType
// @Tags Upgrade
// @Param body body request.CreateFirmwareTypeReq true "固件类型信息"
// @Success 200 {object} response.RFirmwareType
// @Router /web/upgrades/firmware-types [post]
func (h *UpgradeHandler) CreateFirmwareType(c *gin.Context) {
	var req request.CreateFirmwareTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.CreateFirmwareType(c.Request.Context(), &req)
	if err != nil {
		upgradeFail(c, err, "固件类型不存在")
		return
	}
	utils.Success(c, m)
}

// UpdateFirmwareType 更新固件类型
// @Summary 更新固件类型
// @ID updateFirmwareType
// @Tags Upgrade
// @Param id path int true "固件类型ID"
// @Param body body request.UpdateFirmwareTypeReq true "固件类型信息"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/firmware-types/{id} [put]
func (h *UpgradeHandler) UpdateFirmwareType(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的固件类型ID")
		return
	}
	var req request.UpdateFirmwareTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateFirmwareType(c.Request.Context(), id, &req); err != nil {
		upgradeFail(c, err, "固件类型不存在")
		return
	}
	utils.Success(c, nil)
}

// DeleteFirmwareType 删除固件类型
// @Summary 删除固件类型
// @ID deleteFirmwareType
// @Tags Upgrade
// @Param id path int true "固件类型ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/firmware-types/{id} [delete]
func (h *UpgradeHandler) DeleteFirmwareType(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的固件类型ID")
		return
	}
	if err := h.svc.DeleteFirmwareType(c.Request.Context(), id); err != nil {
		upgradeFail(c, err, "固件类型不存在")
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 固件
// ════════════════════════════════════════════════════════════════

// PageFirmwares 固件分页列表
// @Summary 固件分页列表
// @ID pageFirmwares
// @Tags Upgrade
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（固件名称/版本号）"
// @Param firmware_type_id query int false "固件类型ID"
// @Success 200 {object} response.RPageFirmware
// @Router /web/upgrades/firmwares [get]
func (h *UpgradeHandler) PageFirmwares(c *gin.Context) {
	var req request.PageFirmwareReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageFirmwares(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "固件不存在")
		return
	}
	utils.Success(c, resp)
}

// FirmwareOptions 固件下拉选项
// @Summary 固件下拉选项
// @ID firmwareOptions
// @Tags Upgrade
// @Success 200 {object} response.ROptionList
// @Router /web/upgrades/firmwares/options [get]
func (h *UpgradeHandler) FirmwareOptions(c *gin.Context) {
	opts, err := h.svc.FirmwareOptions(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "固件不存在")
		return
	}
	utils.Success(c, opts)
}

// GetFirmware 固件详情
// @Summary 固件详情
// @ID getFirmware
// @Tags Upgrade
// @Param id path int true "固件ID"
// @Success 200 {object} response.RFirmware
// @Router /web/upgrades/firmwares/{id} [get]
func (h *UpgradeHandler) GetFirmware(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的固件ID")
		return
	}
	m, err := h.svc.GetFirmware(c.Request.Context(), id)
	if err != nil {
		upgradeFail(c, err, "固件不存在")
		return
	}
	utils.Success(c, m)
}

// CreateFirmware 创建固件
// @Summary 创建固件
// @ID createFirmware
// @Tags Upgrade
// @Param body body request.CreateFirmwareReq true "固件信息"
// @Success 200 {object} response.RFirmware
// @Router /web/upgrades/firmwares [post]
func (h *UpgradeHandler) CreateFirmware(c *gin.Context) {
	var req request.CreateFirmwareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.CreateFirmware(c.Request.Context(), &req)
	if err != nil {
		upgradeFail(c, err, "固件不存在")
		return
	}
	utils.Success(c, m)
}

// UpdateFirmware 更新固件
// @Summary 更新固件
// @ID updateFirmware
// @Tags Upgrade
// @Param id path int true "固件ID"
// @Param body body request.UpdateFirmwareReq true "固件信息"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/firmwares/{id} [put]
func (h *UpgradeHandler) UpdateFirmware(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的固件ID")
		return
	}
	var req request.UpdateFirmwareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateFirmware(c.Request.Context(), id, &req); err != nil {
		upgradeFail(c, err, "固件不存在")
		return
	}
	utils.Success(c, nil)
}

// DeleteFirmware 删除固件
// @Summary 删除固件
// @ID deleteFirmware
// @Tags Upgrade
// @Param id path int true "固件ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/firmwares/{id} [delete]
func (h *UpgradeHandler) DeleteFirmware(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的固件ID")
		return
	}
	if err := h.svc.DeleteFirmware(c.Request.Context(), id); err != nil {
		upgradeFail(c, err, "固件不存在")
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// AI 模型
// ════════════════════════════════════════════════════════════════

// PageModels AI模型分页列表
// @Summary AI模型分页列表
// @ID pageModels
// @Tags Upgrade
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（模型名称/版本号）"
// @Param firmware_id query int false "适用固件ID"
// @Success 200 {object} response.RPageAIModel
// @Router /web/upgrades/models [get]
func (h *UpgradeHandler) PageModels(c *gin.Context) {
	var req request.PageAIModelReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageModels(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "AI模型不存在")
		return
	}
	utils.Success(c, resp)
}

// ModelOptions AI模型下拉选项
// @Summary AI模型下拉选项
// @ID modelOptions
// @Tags Upgrade
// @Success 200 {object} response.ROptionList
// @Router /web/upgrades/models/options [get]
func (h *UpgradeHandler) ModelOptions(c *gin.Context) {
	opts, err := h.svc.ModelOptions(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "AI模型不存在")
		return
	}
	utils.Success(c, opts)
}

// GetModel AI模型详情
// @Summary AI模型详情
// @ID getAIModel
// @Tags Upgrade
// @Param id path int true "AI模型ID"
// @Success 200 {object} response.RAIModel
// @Router /web/upgrades/models/{id} [get]
func (h *UpgradeHandler) GetModel(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的AI模型ID")
		return
	}
	m, err := h.svc.GetModel(c.Request.Context(), id)
	if err != nil {
		upgradeFail(c, err, "模型不存在")
		return
	}
	utils.Success(c, m)
}

// CreateModel 创建AI模型
// @Summary 创建AI模型
// @ID createAIModel
// @Tags Upgrade
// @Param body body request.CreateAIModelReq true "AI模型信息"
// @Success 200 {object} response.RAIModel
// @Router /web/upgrades/models [post]
func (h *UpgradeHandler) CreateModel(c *gin.Context) {
	var req request.CreateAIModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.CreateModel(c.Request.Context(), &req)
	if err != nil {
		upgradeFail(c, err, "适用固件不存在")
		return
	}
	utils.Success(c, m)
}

// UpdateModel 更新AI模型
// @Summary 更新AI模型
// @ID updateAIModel
// @Tags Upgrade
// @Param id path int true "AI模型ID"
// @Param body body request.UpdateAIModelReq true "AI模型信息"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/models/{id} [put]
func (h *UpgradeHandler) UpdateModel(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的AI模型ID")
		return
	}
	var req request.UpdateAIModelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateModel(c.Request.Context(), id, &req); err != nil {
		upgradeFail(c, err, "AI模型不存在")
		return
	}
	utils.Success(c, nil)
}

// DeleteModel 删除AI模型
// @Summary 删除AI模型
// @ID deleteAIModel
// @Tags Upgrade
// @Param id path int true "AI模型ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/models/{id} [delete]
func (h *UpgradeHandler) DeleteModel(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的AI模型ID")
		return
	}
	if err := h.svc.DeleteModel(c.Request.Context(), id); err != nil {
		upgradeFail(c, err, "AI模型不存在")
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 升级下发 / 计划 / 记录
// ════════════════════════════════════════════════════════════════

// UpgradeDevices 设备升级下发
// @Summary 设备升级下发（现在升级 / 定时升级）
// @ID upgradeDevices
// @Tags Upgrade
// @Param body body request.UpgradeDevicesReq true "升级下发信息"
// @Success 200 {object} response.RUpgradeDevices
// @Router /web/upgrades/devices [post]
func (h *UpgradeHandler) UpgradeDevices(c *gin.Context) {
	var req request.UpgradeDevicesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.UpgradeDevices(c.Request.Context(), &req, c.GetString("user_name"))
	if err != nil {
		upgradeFail(c, err, "设备不存在")
		return
	}
	utils.Success(c, resp)
}

// PagePlans 升级计划分页列表
// @Summary 升级计划分页列表
// @ID pageUpgradePlans
// @Tags Upgrade
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（设备编号/设备名称）"
// @Param region_id query int false "所属区域ID"
// @Param device_id query int false "设备ID"
// @Param status query string false "状态 pending/running/done/failed/canceled"
// @Success 200 {object} response.RPageUpgradePlan
// @Router /web/upgrades/plans [get]
func (h *UpgradeHandler) PagePlans(c *gin.Context) {
	var req request.PageUpgradePlanReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PagePlans(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "升级计划不存在")
		return
	}
	utils.Success(c, resp)
}

// CancelPlan 取消升级计划
// @Summary 取消升级计划
// @ID cancelUpgradePlan
// @Tags Upgrade
// @Param id path int true "升级计划ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/plans/{id}/cancel [put]
func (h *UpgradeHandler) CancelPlan(c *gin.Context) {
	id := upgradePathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的升级计划ID")
		return
	}
	if err := h.svc.CancelPlan(c.Request.Context(), id); err != nil {
		upgradeFail(c, err, "升级计划不存在")
		return
	}
	utils.Success(c, nil)
}

// BatchCancelPlans 批量取消升级计划
// @Summary 批量取消升级计划
// @ID batchCancelUpgradePlans
// @Tags Upgrade
// @Param body body request.BatchIDReq true "升级计划ID列表"
// @Success 200 {object} response.BaseResponse
// @Router /web/upgrades/plans/cancel [put]
func (h *UpgradeHandler) BatchCancelPlans(c *gin.Context) {
	var req request.BatchIDReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.BatchCancelPlans(c.Request.Context(), req.IDs); err != nil {
		upgradeFail(c, err, "升级计划不存在")
		return
	}
	utils.Success(c, nil)
}

// PageRecords 升级记录分页列表
// @Summary 升级记录分页列表
// @ID pageUpgradeRecords
// @Tags Upgrade
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（设备编号/设备名称）"
// @Param region_id query int false "所属区域ID"
// @Param device_id query int false "设备ID"
// @Param start_dt query string false "升级时间起（YYYY-MM-DD）"
// @Param end_dt query string false "升级时间止（YYYY-MM-DD）"
// @Success 200 {object} response.RPageUpgradeRecord
// @Router /web/upgrades/records [get]
func (h *UpgradeHandler) PageRecords(c *gin.Context) {
	var req request.PageUpgradeRecordReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageRecords(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		upgradeFail(c, err, "升级记录不存在")
		return
	}
	utils.Success(c, resp)
}
