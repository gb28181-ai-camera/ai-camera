package handler

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标级联（上级平台）配置
//
// ★ 本批次只做配置与映射表 CRUD：不向上级注册、不推目录、
// 不处理上级 INVITE。SIP 交互属于后续批次。
// ════════════════════════════════════════════════════════════════

// GbPlatformHandler 级联平台处理器。
type GbPlatformHandler struct {
	svc *gbsvc.GbPlatformService
}

// NewGbPlatformHandler 构造级联平台处理器。
func NewGbPlatformHandler() *GbPlatformHandler {
	return &GbPlatformHandler{svc: gbsvc.NewGbPlatformService()}
}

// gbChannelPathID 解析 :channelId（级联通道用的是本地通道主键）。
func gbChannelPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("channelId"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的通道ID")
		return 0, false
	}
	return id, true
}

// Page 上级平台分页
//
// @Summary      上级平台分页
// @Description  返回平台配置，password 脱敏为 ****
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "页码"     default(1)
// @Param        page_size  query     int     false  "每页条数"  default(20)
// @Param        keyword    query     string  false  "名称/国标编码关键字"
// @Param        status     query     string  false  "在线状态 ON/OFF"
// @Success      200  {object}  response.RPageGbPlatform
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/platforms [get]
// @ID           pageGbPlatforms
func (h *GbPlatformHandler) Page(c *gin.Context) {
	var req request.PageGbPlatformReq
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

// Get 上级平台详情
//
// @Summary      上级平台详情
// @Description  返回平台配置与分组/区域可见范围，password 脱敏为 ****
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "平台ID"
// @Success      200  {object}  response.RGbPlatform
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id} [get]
// @ID           getGbPlatform
func (h *GbPlatformHandler) Get(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound)
		return
	}
	utils.Success(c, resp)
}

// Create 新增上级平台
//
// @Summary      新增上级平台
// @Description  只保存配置，不发起 REGISTER；server_gb_id 全局唯一
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateGbPlatformReq  true  "平台信息"
// @Success      200  {object}  response.RGbPlatformCreated
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/platforms [post]
// @ID           createGbPlatform
func (h *GbPlatformHandler) Create(c *gin.Context) {
	var req request.CreateGbPlatformReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformGBIDExists)
		return
	}
	utils.Success(c, m)
}

// Update 编辑上级平台
//
// @Summary      编辑上级平台
// @Description  增量更新；password 传空或含 **** 时保持原值
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id    path      int                        true  "平台ID"
// @Param        body  body      request.UpdateGbPlatformReq  true  "平台信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id} [put]
// @ID           updateGbPlatform
func (h *GbPlatformHandler) Update(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	var req request.UpdateGbPlatformReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound, gbsvc.ErrGbPlatformGBIDExists)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除上级平台
//
// @Summary      删除上级平台
// @Description  级联清理通道/分组/区域映射；不做任何 SIP 注销
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "平台ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id} [delete]
// @ID           deleteGbPlatform
func (h *GbPlatformHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound)
		return
	}
	utils.Success(c, nil)
}

// PageChannel 级联通道分页
//
// @Summary      级联通道分页
// @Description  返回该上级平台可见的本地通道及其定制信息
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id         path      int     true   "平台ID"
// @Param        page       query     int     false  "页码"     default(1)
// @Param        page_size  query     int     false  "每页条数"  default(20)
// @Param        keyword    query     string  false  "定制编码/名称关键字"
// @Param        channel_id query     int     false  "本地通道ID"
// @Success      200  {object}  response.RPageGbPlatformChannel
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id}/channels [get]
// @ID           pageGbPlatformChannels
func (h *GbPlatformHandler) PageChannel(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	var req request.PageGbPlatformChannelReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageChannel(c.Request.Context(), id, &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound)
		return
	}
	utils.Success(c, resp)
}

// AddChannel 添加级联通道
//
// @Summary      添加级联通道
// @Description  把本地通道加入上级目录；已存在的通道跳过。默认用本地国标编码作为定制编码
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id    path      int                             true  "平台ID"
// @Param        body  body      request.GbPlatformChannelIDsReq  true  "通道ID集合"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id}/channels [post]
// @ID           addGbPlatformChannels
func (h *GbPlatformHandler) AddChannel(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	var req request.GbPlatformChannelIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.AddChannel(c.Request.Context(), id, req.ChannelIDs); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound, gbsvc.ErrGbPlatformChannelIDExists)
		return
	}
	utils.Success(c, nil)
}

// RemoveChannel 移除级联通道
//
// @Summary      移除级联通道
// @Description  从上级目录移除；幂等，不存在的映射直接忽略
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id    path      int                             true  "平台ID"
// @Param        body  body      request.GbPlatformChannelIDsReq  true  "通道ID集合"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id}/channels [delete]
// @ID           removeGbPlatformChannels
func (h *GbPlatformHandler) RemoveChannel(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	var req request.GbPlatformChannelIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.RemoveChannel(c.Request.Context(), id, req.ChannelIDs); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound)
		return
	}
	utils.Success(c, nil)
}

// UpdateChannelCustom 修改级联通道定制信息
//
// @Summary      修改级联通道定制信息
// @Description  修改该通道在上级平台的名称/编码/厂商等定制值；定制编码全局唯一
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id         path      int                                  true  "平台ID"
// @Param        channelId  path      int                                  true  "本地通道ID"
// @Param        body       body      request.UpdateGbPlatformChannelReq    true  "定制信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id}/channels/{channelId} [put]
// @ID           updateGbPlatformChannelCustom
func (h *GbPlatformHandler) UpdateChannelCustom(c *gin.Context) {
	platformID, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	channelID, ok := gbChannelPathID(c)
	if !ok {
		return
	}
	var req request.UpdateGbPlatformChannelReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.UpdateChannelCustom(c.Request.Context(), platformID, channelID, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformChannelNotFound, gbsvc.ErrGbPlatformChannelIDExists)
		return
	}
	utils.Success(c, nil)
}

// SetGroups 设置分组可见范围
//
// @Summary      设置分组可见范围
// @Description  整体替换：决定「按分组推送目录」时对上级可见的业务分组
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id    path      int                            true  "平台ID"
// @Param        body  body      request.SetGbPlatformGroupsReq  true  "分组ID集合"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id}/groups [put]
// @ID           setGbPlatformGroups
func (h *GbPlatformHandler) SetGroups(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	var req request.SetGbPlatformGroupsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SetGroups(c.Request.Context(), id, req.GroupIDs); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound)
		return
	}
	utils.Success(c, nil)
}

// SetRegions 设置区域可见范围
//
// @Summary      设置区域可见范围
// @Description  整体替换：决定「按区域推送目录」时对上级可见的区域
// @Tags         GbPlatform
// @Accept       json
// @Produce      json
// @Param        id    path      int                             true  "平台ID"
// @Param        body  body      request.SetGbPlatformRegionsReq  true  "区域ID集合"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/platforms/{id}/regions [put]
// @ID           setGbPlatformRegions
func (h *GbPlatformHandler) SetRegions(c *gin.Context) {
	id, ok := gbPathID(c, "上级平台")
	if !ok {
		return
	}
	var req request.SetGbPlatformRegionsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SetRegions(c.Request.Context(), id, req.RegionIDs); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbPlatformNotFound)
		return
	}
	utils.Success(c, nil)
}
