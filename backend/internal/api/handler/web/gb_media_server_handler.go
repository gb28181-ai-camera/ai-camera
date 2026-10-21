package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 流媒体节点（ZLMediaKit）
// ════════════════════════════════════════════════════════════════

// GbMediaServerHandler 流媒体节点处理器。
type GbMediaServerHandler struct {
	svc *gbsvc.GbMediaServerService
}

// NewGbMediaServerHandler 构造流媒体节点处理器。
func NewGbMediaServerHandler() *GbMediaServerHandler {
	return &GbMediaServerHandler{svc: gbsvc.NewGbMediaServerService()}
}

// List 媒体节点列表
//
// @Summary      媒体节点列表
// @Description  简单列表（不分页，节点数量天然很少）；secret 脱敏为 ****
// @Tags         GbMediaServer
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RGbMediaServerList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/media-servers [get]
// @ID           listGbMediaServers
func (h *GbMediaServerHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, items)
}

// Get 媒体节点详情
//
// @Summary      媒体节点详情
// @Description  返回节点全部端口与收流配置；secret 脱敏为 ****
// @Tags         GbMediaServer
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "节点ID"
// @Success      200  {object}  response.RGbMediaServer
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id} [get]
// @ID           getGbMediaServer
func (h *GbMediaServerHandler) Get(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbMediaServerNotFound)
		return
	}
	utils.Success(c, resp)
}

// Create 新增媒体节点
//
// @Summary      新增媒体节点
// @Description  server_id 全局唯一；secret 直接写入（新增时不存在脱敏问题）
// @Tags         GbMediaServer
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateGbMediaServerReq  true  "节点信息"
// @Success      200  {object}  response.RGbMediaServerCreated
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/media-servers [post]
// @ID           createGbMediaServer
func (h *GbMediaServerHandler) Create(c *gin.Context) {
	var req request.CreateGbMediaServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, m)
}

// Update 编辑媒体节点
//
// @Summary      编辑媒体节点
// @Description  全字段增量更新；secret 传空或含 **** 时保持原值
// @Tags         GbMediaServer
// @Accept       json
// @Produce      json
// @Param        id    path      int                           true  "节点ID"
// @Param        body  body      request.UpdateGbMediaServerReq  true  "节点信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id} [put]
// @ID           updateGbMediaServer
func (h *GbMediaServerHandler) Update(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	var req request.UpdateGbMediaServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbMediaServerNotFound, gbsvc.ErrGbMediaServerIDExists)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除媒体节点
//
// @Summary      删除媒体节点
// @Description  软删除节点；已绑定该节点的设备需由管理员重新分配
// @Tags         GbMediaServer
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "节点ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id} [delete]
// @ID           deleteGbMediaServer
func (h *GbMediaServerHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbMediaServerNotFound)
		return
	}
	utils.Success(c, nil)
}

// Check 媒体节点连通性检查
//
// @Summary      媒体节点连通性检查
// @Description  调用 ZLM /index/api/getServerConfig；返回 online、version、rtp_port 等
// @Tags         GbMediaServer
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "节点ID"
// @Success      200  {object}  response.RGbMediaServerCheck
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id}/check [get]
// @ID           checkGbMediaServer
func (h *GbMediaServerHandler) Check(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	resp, err := h.svc.Check(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbMediaServerNotFound)
		return
	}
	utils.Success(c, resp)
}

// ServerInfo 媒体节点运行信息
//
// @Summary      媒体节点运行信息
// @Description  汇总 getMediaList 与 getThreadsLoad：活跃流数、线程数、平均/最大负载
// @Tags         GbMediaServer
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "节点ID"
// @Success      200  {object}  response.RGbMediaServerInfo
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/media-servers/{id}/info [get]
// @ID           getGbMediaServerInfo
func (h *GbMediaServerHandler) ServerInfo(c *gin.Context) {
	id, ok := gbPathID(c, "媒体节点")
	if !ok {
		return
	}
	resp, err := h.svc.ServerInfo(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbMediaServerNotFound)
		return
	}
	utils.Success(c, resp)
}
