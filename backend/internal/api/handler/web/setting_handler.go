package handler

import (
	"errors"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/setting"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SettingHandler 告警设置处理器（设备告警设置 / 风险通知设置 / 第三方配置）。
//
// 设备告警设置与风险通知设置一律以当前登录用户为界，用户 ID 取自 JWT claims。
type SettingHandler struct {
	svc *setting.SettingService
}

// NewSettingHandler 构造告警设置处理器。
func NewSettingHandler() *SettingHandler {
	return &SettingHandler{svc: setting.NewSettingService()}
}

// settingFail 统一告警设置域错误响应：记录不存在返回 404，其余业务/参数错误返回 400。
func settingFail(c *gin.Context, err error, notFoundMsg string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.NotFound(c, notFoundMsg)
		return
	}
	utils.BadRequest(c, err.Error())
}

// settingPathID 解析路径中的 ID，非法返回 0。
func settingPathID(c *gin.Context) int64 {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

// ════════════════════════════════════════════════════════════════
// 设备告警设置
// ════════════════════════════════════════════════════════════════

// PageAlarmSettings 设备告警设置分页列表
// @Summary 设备告警设置分页列表（仅当前登录用户）
// @ID pageAlarmSettings
// @Tags Setting
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（告警类型名称）"
// @Param alarm_type_id query int false "告警类型ID"
// @Success 200 {object} response.RPageAlarmSetting
// @Router /web/settings/alarm-settings [get]
func (h *SettingHandler) PageAlarmSettings(c *gin.Context) {
	var req request.PageAlarmSettingReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageAlarmSettings(c.Request.Context(), c.GetInt64("user_id"), &req)
	if err != nil {
		settingFail(c, err, "告警设置不存在")
		return
	}
	utils.Success(c, resp)
}

// CreateAlarmSetting 创建设备告警设置
// @Summary 创建设备告警设置（归属当前登录用户）
// @ID createAlarmSetting
// @Tags Setting
// @Param body body request.CreateAlarmSettingReq true "告警设置信息"
// @Success 200 {object} response.RAlarmSetting
// @Router /web/settings/alarm-settings [post]
func (h *SettingHandler) CreateAlarmSetting(c *gin.Context) {
	var req request.CreateAlarmSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.CreateAlarmSetting(c.Request.Context(), c.GetInt64("user_id"), &req)
	if err != nil {
		settingFail(c, err, "告警设置不存在")
		return
	}
	utils.Success(c, m)
}

// UpdateAlarmSetting 更新设备告警设置
// @Summary 更新设备告警设置（仅本人配置）
// @ID updateAlarmSetting
// @Tags Setting
// @Param id path int true "告警设置ID"
// @Param body body request.UpdateAlarmSettingReq true "告警设置信息"
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/alarm-settings/{id} [put]
func (h *SettingHandler) UpdateAlarmSetting(c *gin.Context) {
	id := settingPathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的告警设置ID")
		return
	}
	var req request.UpdateAlarmSettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateAlarmSetting(c.Request.Context(), c.GetInt64("user_id"), id, &req); err != nil {
		settingFail(c, err, "告警设置不存在")
		return
	}
	utils.Success(c, nil)
}

// DeleteAlarmSetting 删除设备告警设置
// @Summary 删除设备告警设置（仅本人配置）
// @ID deleteAlarmSetting
// @Tags Setting
// @Param id path int true "告警设置ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/alarm-settings/{id} [delete]
func (h *SettingHandler) DeleteAlarmSetting(c *gin.Context) {
	id := settingPathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的告警设置ID")
		return
	}
	if err := h.svc.DeleteAlarmSetting(c.Request.Context(), c.GetInt64("user_id"), id); err != nil {
		settingFail(c, err, "告警设置不存在")
		return
	}
	utils.Success(c, nil)
}

// DisableAllAlarm 关闭全部设备告警通知
// @Summary 关闭全部设备告警通知（仅当前登录用户）
// @ID disableAllAlarm
// @Tags Setting
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/alarm-settings/disable-all [put]
func (h *SettingHandler) DisableAllAlarm(c *gin.Context) {
	if err := h.svc.DisableAllAlarm(c.Request.Context(), c.GetInt64("user_id")); err != nil {
		settingFail(c, err, "告警设置不存在")
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 风险通知设置
// ════════════════════════════════════════════════════════════════

// PageRiskNotifySettings 风险通知设置分页列表
// @Summary 风险通知设置分页列表（仅当前登录用户）
// @ID pageRiskNotifySettings
// @Tags Setting
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "关键字（风险类型名称）"
// @Success 200 {object} response.RPageRiskNotifySetting
// @Router /web/settings/risk-notify-settings [get]
func (h *SettingHandler) PageRiskNotifySettings(c *gin.Context) {
	var req request.PageRiskNotifySettingReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageRiskNotifySettings(c.Request.Context(), c.GetInt64("user_id"), &req)
	if err != nil {
		settingFail(c, err, "风险通知设置不存在")
		return
	}
	utils.Success(c, resp)
}

// CreateRiskNotifySetting 创建风险通知设置
// @Summary 创建风险通知设置（归属当前登录用户）
// @ID createRiskNotifySetting
// @Tags Setting
// @Param body body request.CreateRiskNotifySettingReq true "风险通知设置信息"
// @Success 200 {object} response.RRiskNotifySetting
// @Router /web/settings/risk-notify-settings [post]
func (h *SettingHandler) CreateRiskNotifySetting(c *gin.Context) {
	var req request.CreateRiskNotifySettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.CreateRiskNotifySetting(c.Request.Context(), c.GetInt64("user_id"), &req)
	if err != nil {
		settingFail(c, err, "风险通知设置不存在")
		return
	}
	utils.Success(c, m)
}

// UpdateRiskNotifySetting 更新风险通知设置
// @Summary 更新风险通知设置（仅本人配置）
// @ID updateRiskNotifySetting
// @Tags Setting
// @Param id path int true "风险通知设置ID"
// @Param body body request.UpdateRiskNotifySettingReq true "风险通知设置信息"
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/risk-notify-settings/{id} [put]
func (h *SettingHandler) UpdateRiskNotifySetting(c *gin.Context) {
	id := settingPathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的风险通知设置ID")
		return
	}
	var req request.UpdateRiskNotifySettingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateRiskNotifySetting(c.Request.Context(), c.GetInt64("user_id"), id, &req); err != nil {
		settingFail(c, err, "风险通知设置不存在")
		return
	}
	utils.Success(c, nil)
}

// DeleteRiskNotifySetting 删除风险通知设置
// @Summary 删除风险通知设置（仅本人配置）
// @ID deleteRiskNotifySetting
// @Tags Setting
// @Param id path int true "风险通知设置ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/risk-notify-settings/{id} [delete]
func (h *SettingHandler) DeleteRiskNotifySetting(c *gin.Context) {
	id := settingPathID(c)
	if id == 0 {
		utils.BadRequest(c, "无效的风险通知设置ID")
		return
	}
	if err := h.svc.DeleteRiskNotifySetting(c.Request.Context(), c.GetInt64("user_id"), id); err != nil {
		settingFail(c, err, "风险通知设置不存在")
		return
	}
	utils.Success(c, nil)
}

// DisableAllRiskNotify 关闭全部风险通知
// @Summary 关闭全部风险通知（仅当前登录用户）
// @ID disableAllRiskNotify
// @Tags Setting
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/risk-notify-settings/disable-all [put]
func (h *SettingHandler) DisableAllRiskNotify(c *gin.Context) {
	if err := h.svc.DisableAllRiskNotify(c.Request.Context(), c.GetInt64("user_id")); err != nil {
		settingFail(c, err, "风险通知设置不存在")
		return
	}
	utils.Success(c, nil)
}

// ════════════════════════════════════════════════════════════════
// 第三方配置（个推 / 短信）
// ════════════════════════════════════════════════════════════════

// GetGetui 获取个推配置
// @Summary 获取个推配置（敏感项脱敏）
// @ID getGetui
// @Tags Setting
// @Success 200 {object} response.RGetuiConfig
// @Router /web/settings/getui [get]
func (h *SettingHandler) GetGetui(c *gin.Context) {
	resp, err := h.svc.GetGetui(c.Request.Context())
	if err != nil {
		settingFail(c, err, "个推配置不存在")
		return
	}
	utils.Success(c, resp)
}

// UpdateGetui 更新个推配置
// @Summary 更新个推配置（空值与脱敏串不覆盖原值）
// @ID updateGetui
// @Tags Setting
// @Param body body request.UpdateGetuiReq true "个推配置"
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/getui [put]
func (h *SettingHandler) UpdateGetui(c *gin.Context) {
	var req request.UpdateGetuiReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateGetui(c.Request.Context(), &req); err != nil {
		settingFail(c, err, "个推配置不存在")
		return
	}
	utils.Success(c, nil)
}

// GetSms 获取短信配置
// @Summary 获取短信配置（敏感项脱敏）
// @ID getSms
// @Tags Setting
// @Success 200 {object} response.RSmsConfig
// @Router /web/settings/sms [get]
func (h *SettingHandler) GetSms(c *gin.Context) {
	resp, err := h.svc.GetSms(c.Request.Context())
	if err != nil {
		settingFail(c, err, "短信配置不存在")
		return
	}
	utils.Success(c, resp)
}

// UpdateSms 更新短信配置
// @Summary 更新短信配置（空值与脱敏串不覆盖原值）
// @ID updateSms
// @Tags Setting
// @Param body body request.UpdateSmsReq true "短信配置"
// @Success 200 {object} response.BaseResponse
// @Router /web/settings/sms [put]
func (h *SettingHandler) UpdateSms(c *gin.Context) {
	var req request.UpdateSmsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateSms(c.Request.Context(), &req); err != nil {
		settingFail(c, err, "短信配置不存在")
		return
	}
	utils.Success(c, nil)
}
