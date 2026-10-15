package handler

import (
	"errors"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/basic"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BasicHandler 基础信息管理：区域 / 风险类型 / 告警类型。
type BasicHandler struct{}

// NewBasicHandler 构造基础信息处理器。
func NewBasicHandler() *BasicHandler { return &BasicHandler{} }

// basicPathID 解析路径参数 :id。解析失败时已写出 400，调用方直接返回即可。
func basicPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的ID")
		return 0, false
	}
	return id, true
}

// ════════════════════════════════════════════════════════════════
// 区域管理
// ════════════════════════════════════════════════════════════════

// PageRegions 区域分页列表
// @Summary 区域分页列表
// @Description 支持 keyword 模糊匹配区域名称/区域编码，按 sequence 升序返回。
// @ID pageRegions
// @Tags Basic
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20"
// @Param keyword query string false "区域名称/编码关键字"
// @Success 200 {object} response.RPageRegion
// @Failure 400 {object} response.BaseResponse
// @Router /web/basic/regions [get]
func (h *BasicHandler) PageRegions(c *gin.Context) {
	var req request.PageRegionReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := basic.NewRegionService().Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// RegionOptions 区域下拉选项
// @Summary 区域下拉选项
// @Description 返回启用状态的区域，按 sequence 升序。
// @ID regionOptions
// @Tags Basic
// @Success 200 {object} response.ROptionList
// @Router /web/basic/regions/options [get]
func (h *BasicHandler) RegionOptions(c *gin.Context) {
	items, err := basic.NewRegionService().Options(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, items)
}

// GetRegion 区域详情
// @Summary 区域详情
// @ID getRegion
// @Tags Basic
// @Param id path int true "区域ID"
// @Success 200 {object} response.RRegion
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/regions/{id} [get]
func (h *BasicHandler) GetRegion(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	m, err := basic.NewRegionService().Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "区域不存在")
			return
		}
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, m)
}

// CreateRegion 新增区域
// @Summary 新增区域
// @ID createRegion
// @Tags Basic
// @Param body body request.CreateRegionReq true "区域信息"
// @Success 200 {object} response.RRegion
// @Failure 400 {object} response.BaseResponse
// @Router /web/basic/regions [post]
func (h *BasicHandler) CreateRegion(c *gin.Context) {
	var req request.CreateRegionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	m, err := basic.NewRegionService().Create(c.Request.Context(), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Created(c, m)
}

// UpdateRegion 修改区域
// @Summary 修改区域
// @ID updateRegion
// @Tags Basic
// @Param id path int true "区域ID"
// @Param body body request.UpdateRegionReq true "区域信息"
// @Success 200 {object} response.RBool
// @Failure 400 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/regions/{id} [put]
func (h *BasicHandler) UpdateRegion(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	var req request.UpdateRegionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := basic.NewRegionService().Update(c.Request.Context(), id, &req); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "区域不存在")
			return
		}
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// DeleteRegion 删除区域
// @Summary 删除区域
// @Description 软删除。区域下存在设备或风险记录时返回 400 并提示引用数量。
// @ID deleteRegion
// @Tags Basic
// @Param id path int true "区域ID"
// @Success 200 {object} response.RBool
// @Failure 400 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/regions/{id} [delete]
func (h *BasicHandler) DeleteRegion(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	if err := basic.NewRegionService().Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "区域不存在")
			return
		}
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// ════════════════════════════════════════════════════════════════
// 风险类型管理
// ════════════════════════════════════════════════════════════════

// PageRiskTypes 风险类型分页列表
// @Summary 风险类型分页列表
// @Description 支持 keyword 模糊匹配名称/编码、level 精确过滤等级，按 sequence 升序返回。
// @ID pageRiskTypes
// @Tags Basic
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20"
// @Param keyword query string false "风险类型名称/编码关键字"
// @Param level query string false "风险等级 high/medium/low"
// @Success 200 {object} response.RPageRiskType
// @Failure 400 {object} response.BaseResponse
// @Router /web/basic/risk-types [get]
func (h *BasicHandler) PageRiskTypes(c *gin.Context) {
	var req request.PageRiskTypeReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := basic.NewRiskTypeService().Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// RiskTypeOptions 风险类型下拉选项
// @Summary 风险类型下拉选项
// @ID riskTypeOptions
// @Tags Basic
// @Success 200 {object} response.ROptionList
// @Router /web/basic/risk-types/options [get]
func (h *BasicHandler) RiskTypeOptions(c *gin.Context) {
	items, err := basic.NewRiskTypeService().Options(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, items)
}

// GetRiskType 风险类型详情
// @Summary 风险类型详情
// @ID getRiskType
// @Tags Basic
// @Param id path int true "风险类型ID"
// @Success 200 {object} response.RRiskType
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/risk-types/{id} [get]
func (h *BasicHandler) GetRiskType(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	m, err := basic.NewRiskTypeService().Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "风险类型不存在")
			return
		}
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, m)
}

// CreateRiskType 新增风险类型
// @Summary 新增风险类型
// @ID createRiskType
// @Tags Basic
// @Param body body request.CreateRiskTypeReq true "风险类型信息"
// @Success 200 {object} response.RRiskType
// @Failure 400 {object} response.BaseResponse
// @Router /web/basic/risk-types [post]
func (h *BasicHandler) CreateRiskType(c *gin.Context) {
	var req request.CreateRiskTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	m, err := basic.NewRiskTypeService().Create(c.Request.Context(), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Created(c, m)
}

// UpdateRiskType 修改风险类型
// @Summary 修改风险类型
// @ID updateRiskType
// @Tags Basic
// @Param id path int true "风险类型ID"
// @Param body body request.UpdateRiskTypeReq true "风险类型信息"
// @Success 200 {object} response.RBool
// @Failure 400 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/risk-types/{id} [put]
func (h *BasicHandler) UpdateRiskType(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	var req request.UpdateRiskTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := basic.NewRiskTypeService().Update(c.Request.Context(), id, &req); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "风险类型不存在")
			return
		}
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// DeleteRiskType 删除风险类型
// @Summary 删除风险类型
// @Description 软删除。被风险记录或语音配置引用时返回 400 并提示引用数量。
// @ID deleteRiskType
// @Tags Basic
// @Param id path int true "风险类型ID"
// @Success 200 {object} response.RBool
// @Failure 400 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/risk-types/{id} [delete]
func (h *BasicHandler) DeleteRiskType(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	if err := basic.NewRiskTypeService().Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "风险类型不存在")
			return
		}
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// ════════════════════════════════════════════════════════════════
// 告警类型管理
// ════════════════════════════════════════════════════════════════

// PageAlarmTypes 告警类型分页列表
// @Summary 告警类型分页列表
// @Description 支持 keyword 模糊匹配名称/编码、level 精确过滤等级，按 sequence 升序返回。
// @ID pageAlarmTypes
// @Tags Basic
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20"
// @Param keyword query string false "告警名称/编码关键字"
// @Param level query string false "告警等级 high/medium/low"
// @Success 200 {object} response.RPageAlarmType
// @Failure 400 {object} response.BaseResponse
// @Router /web/basic/alarm-types [get]
func (h *BasicHandler) PageAlarmTypes(c *gin.Context) {
	var req request.PageAlarmTypeReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res, err := basic.NewAlarmTypeService().Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, res)
}

// AlarmTypeOptions 告警类型下拉选项
// @Summary 告警类型下拉选项
// @ID alarmTypeOptions
// @Tags Basic
// @Success 200 {object} response.ROptionList
// @Router /web/basic/alarm-types/options [get]
func (h *BasicHandler) AlarmTypeOptions(c *gin.Context) {
	items, err := basic.NewAlarmTypeService().Options(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, items)
}

// GetAlarmType 告警类型详情
// @Summary 告警类型详情
// @ID getAlarmType
// @Tags Basic
// @Param id path int true "告警类型ID"
// @Success 200 {object} response.RAlarmType
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/alarm-types/{id} [get]
func (h *BasicHandler) GetAlarmType(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	m, err := basic.NewAlarmTypeService().Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "告警类型不存在")
			return
		}
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, m)
}

// CreateAlarmType 新增告警类型
// @Summary 新增告警类型
// @ID createAlarmType
// @Tags Basic
// @Param body body request.CreateAlarmTypeReq true "告警类型信息"
// @Success 200 {object} response.RAlarmType
// @Failure 400 {object} response.BaseResponse
// @Router /web/basic/alarm-types [post]
func (h *BasicHandler) CreateAlarmType(c *gin.Context) {
	var req request.CreateAlarmTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	m, err := basic.NewAlarmTypeService().Create(c.Request.Context(), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Created(c, m)
}

// UpdateAlarmType 修改告警类型
// @Summary 修改告警类型
// @ID updateAlarmType
// @Tags Basic
// @Param id path int true "告警类型ID"
// @Param body body request.UpdateAlarmTypeReq true "告警类型信息"
// @Success 200 {object} response.RBool
// @Failure 400 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/alarm-types/{id} [put]
func (h *BasicHandler) UpdateAlarmType(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	var req request.UpdateAlarmTypeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := basic.NewAlarmTypeService().Update(c.Request.Context(), id, &req); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "告警类型不存在")
			return
		}
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// DeleteAlarmType 删除告警类型
// @Summary 删除告警类型
// @Description 软删除。被告警记录或告警设置引用时返回 400 并提示引用数量。
// @ID deleteAlarmType
// @Tags Basic
// @Param id path int true "告警类型ID"
// @Success 200 {object} response.RBool
// @Failure 400 {object} response.BaseResponse
// @Failure 404 {object} response.BaseResponse
// @Router /web/basic/alarm-types/{id} [delete]
func (h *BasicHandler) DeleteAlarmType(c *gin.Context) {
	id, ok := basicPathID(c)
	if !ok {
		return
	}
	if err := basic.NewAlarmTypeService().Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "告警类型不存在")
			return
		}
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}
