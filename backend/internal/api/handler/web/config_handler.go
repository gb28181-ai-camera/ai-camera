package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ConfigHandler 系统参数管理。
type ConfigHandler struct{}

// NewConfigHandler 构造系统参数处理器。
func NewConfigHandler() *ConfigHandler { return &ConfigHandler{} }

// List 系统参数列表
// @Summary 系统参数列表
// @Description 非分页列表，keyword 模糊匹配参数名称与参数键。
// @ID listConfigs
// @Tags System
// @Param keyword query string false "关键词(名称/参数键)"
// @Success 200 {object} response.RSystemConfigList
// @Router /web/system/configs [get]
func (h *ConfigHandler) List(c *gin.Context) {
	keyword := c.Query("keyword")

	list, err := system.NewConfigService().List(c.Request.Context(), keyword)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, list)
}

// Create 新增系统参数
// @Summary 新增系统参数
// @Description 支持 active（默认 true）与 schema（JSON Schema）。data_type 支持 string/int/integer/float/bool/boolean/json/select，integer/boolean 归一为 int/bool。
// @ID createConfig
// @Tags System
// @Param body body request.CreateConfigReq true "参数信息"
// @Success 201 {object} response.RSystemConfig
// @Router /web/system/configs [post]
func (h *ConfigHandler) Create(c *gin.Context) {
	var req request.CreateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	cfg, err := system.NewConfigService().Create(c.Request.Context(), &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Created(c, cfg)
}

// Update 更新系统参数
// @Summary 更新系统参数
// @Description active 为 null/未传时不修改启用状态；其他为空（未传）的字段同样不覆盖原值。
// @ID updateConfig
// @Tags System
// @Param id path int true "参数ID"
// @Param body body request.UpdateConfigReq true "参数信息"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/configs/{id} [put]
func (h *ConfigHandler) Update(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}
	var req request.UpdateConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if err := system.NewConfigService().Update(c.Request.Context(), id, &req); err != nil {
		systemRespondErr(c, err, "参数不存在")
		return
	}
	utils.Success(c, nil)
}

// Delete 删除系统参数
// @Summary 删除系统参数
// @ID deleteConfig
// @Tags System
// @Param id path int true "参数ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/configs/{id} [delete]
func (h *ConfigHandler) Delete(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewConfigService().Delete(c.Request.Context(), id); err != nil {
		systemRespondErr(c, err, "参数不存在")
		return
	}
	utils.Success(c, nil)
}

// Enable 启用系统参数
// @Summary 启用系统参数
// @ID enableConfig
// @Tags System
// @Param id path int true "参数ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/configs/{id}/enable [put]
func (h *ConfigHandler) Enable(c *gin.Context) {
	h.setActive(c, true)
}

// Disable 停用系统参数
// @Summary 停用系统参数
// @ID disableConfig
// @Tags System
// @Param id path int true "参数ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/configs/{id}/disable [put]
func (h *ConfigHandler) Disable(c *gin.Context) {
	h.setActive(c, false)
}

// setActive 启用/停用的公共实现。
func (h *ConfigHandler) setActive(c *gin.Context, active bool) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewConfigService().SetActive(c.Request.Context(), id, active); err != nil {
		systemRespondErr(c, err, "参数不存在")
		return
	}
	utils.Success(c, nil)
}
