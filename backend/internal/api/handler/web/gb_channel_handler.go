package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标通道
// ════════════════════════════════════════════════════════════════

// GbChannelHandler 国标通道处理器。
type GbChannelHandler struct {
	svc *gbsvc.GbChannelService
}

// NewGbChannelHandler 构造国标通道处理器。
func NewGbChannelHandler() *GbChannelHandler {
	return &GbChannelHandler{svc: gbsvc.NewGbChannelService()}
}

// Page 国标通道分页列表
//
// @Summary      国标通道分页列表
// @Description  支持关键字、设备、区域、业务分组、状态筛选；返回设备名称、区域名称、分组名称、录像计划名称
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Param        page          query     int     false  "页码"        default(1)
// @Param        page_size     query     int     false  "每页条数"     default(20)
// @Param        keyword       query     string  false  "关键字(名称/通道编码/设备编号)"
// @Param        device_id     query     int     false  "设备ID"
// @Param        region_id     query     int     false  "区域ID"
// @Param        group_id      query     int     false  "业务分组ID"
// @Param        status        query     string  false  "通道状态 ON/OFF"
// @Param        gb_device_id  query     string  false  "设备国标编码"
// @Success      200  {object}  response.RPageGbChannel
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/channels [get]
// @ID           pageGbChannels
func (h *GbChannelHandler) Page(c *gin.Context) {
	var req request.PageGbChannelReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Options 通道下拉选项
//
// @Summary      通道下拉选项
// @Description  仅返回启用状态的通道；code 为通道国标编码
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.ROptionList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/channels/options [get]
// @ID           gbChannelOptions
func (h *GbChannelHandler) Options(c *gin.Context) {
	items, err := h.svc.Options(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, items)
}

// Get 国标通道详情
//
// @Summary      国标通道详情
// @Description  返回通道属性与关联名称；password 脱敏为 ****
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "通道ID"
// @Success      200  {object}  response.RGbChannel
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id} [get]
// @ID           getGbChannel
func (h *GbChannelHandler) Get(c *gin.Context) {
	id, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, resp)
}

// Update 编辑国标通道
//
// @Summary      编辑国标通道
// @Description  增量更新：字符串非空才覆盖；region_id/group_id/record_plan_id 传 0 表示解绑；password 传空或 **** 保持原值
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Param        id    path      int                        true  "通道ID"
// @Param        body  body      request.UpdateGbChannelReq  true  "通道信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id} [put]
// @ID           updateGbChannel
func (h *GbChannelHandler) Update(c *gin.Context) {
	id, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	var req request.UpdateGbChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbGroupNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除国标通道
//
// @Summary      删除国标通道
// @Description  软删除通道；下次目录同步时若设备仍上报该通道会重新出现
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "通道ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id} [delete]
// @ID           deleteGbChannel
func (h *GbChannelHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, nil)
}

// UpdateRegion 设置通道区域
//
// @Summary      设置通道区域
// @Description  region_id 传 null 或 0 表示清空归属区域
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "通道ID"
// @Param        body  body      request.GbChannelRegionReq  true  "区域"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id}/region [put]
// @ID           updateGbChannelRegion
func (h *GbChannelHandler) UpdateRegion(c *gin.Context) {
	id, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	var req request.GbChannelRegionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateRegion(c.Request.Context(), id, req.RegionID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, nil)
}

// UpdateGroup 设置通道业务分组
//
// @Summary      设置通道业务分组
// @Description  group_id 传 null 或 0 表示清空归属分组
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true  "通道ID"
// @Param        body  body      request.GbChannelGroupReq  true  "业务分组"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id}/group [put]
// @ID           updateGbChannelGroup
func (h *GbChannelHandler) UpdateGroup(c *gin.Context) {
	id, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	var req request.GbChannelGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateGroup(c.Request.Context(), id, req.GroupID); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound, gbsvc.ErrGbGroupNotFound)
		return
	}
	utils.Success(c, nil)
}

// Reset 重置通道
//
// @Summary      重置通道
// @Description  清空名称、区域、分组、流/录像计划绑定、备注等平台侧字段，等待下次目录同步重新填充；不物理删除通道
// @Tags         GbChannel
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "通道ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/channels/{id}/reset [post]
// @ID           resetGbChannel
func (h *GbChannelHandler) Reset(c *gin.Context) {
	id, ok := gbPathID(c, "通道")
	if !ok {
		return
	}
	if err := h.svc.Reset(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbChannelNotFound)
		return
	}
	utils.Success(c, nil)
}
