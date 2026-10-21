package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 拉流代理（ZLM 主动从 rtsp/rtmp/http 源拉流）
// ════════════════════════════════════════════════════════════════

// GbStreamProxyHandler 拉流代理处理器。
type GbStreamProxyHandler struct {
	svc *gbsvc.GbStreamProxyService
}

// NewGbStreamProxyHandler 构造拉流代理处理器。
func NewGbStreamProxyHandler() *GbStreamProxyHandler {
	return &GbStreamProxyHandler{svc: gbsvc.NewGbStreamProxyService()}
}

// Page 拉流代理分页
//
// @Summary      拉流代理分页
// @Description  支持关键字、拉流状态与媒体节点筛选
// @Tags         GbStreamProxy
// @Accept       json
// @Produce      json
// @Param        page             query     int     false  "页码"     default(1)
// @Param        page_size        query     int     false  "每页条数"  default(20)
// @Param        keyword          query     string  false  "关键字(名称/app/stream/源地址)"
// @Param        pulling          query     bool    false  "是否正在拉流"
// @Param        media_server_id  query     string  false  "媒体节点"
// @Success      200  {object}  response.RPageGbStreamProxy
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/stream-proxies [get]
// @ID           pageGbStreamProxies
func (h *GbStreamProxyHandler) Page(c *gin.Context) {
	var req request.PageGbStreamProxyReq
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

// Create 新增拉流代理
//
// @Summary      新增拉流代理
// @Description  只落配置，不立即拉起；app+stream 必须唯一，调用 start 才让 ZLM 拉流
// @Tags         GbStreamProxy
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateGbStreamProxyReq  true  "代理信息"
// @Success      200  {object}  response.RGbStreamProxy
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/stream-proxies [post]
// @ID           createGbStreamProxy
func (h *GbStreamProxyHandler) Create(c *gin.Context) {
	var req request.CreateGbStreamProxyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamProxyExists)
		return
	}
	utils.Success(c, m)
}

// Update 编辑拉流代理
//
// @Summary      编辑拉流代理
// @Description  增量更新；修改 app/stream 时重新校验唯一性
// @Tags         GbStreamProxy
// @Accept       json
// @Produce      json
// @Param        id    path      int                           true  "代理ID"
// @Param        body  body      request.UpdateGbStreamProxyReq  true  "代理信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-proxies/{id} [put]
// @ID           updateGbStreamProxy
func (h *GbStreamProxyHandler) Update(c *gin.Context) {
	id, ok := gbPathID(c, "拉流代理")
	if !ok {
		return
	}
	var req request.UpdateGbStreamProxyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamProxyNotFound, gbsvc.ErrGbStreamProxyExists)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除拉流代理
//
// @Summary      删除拉流代理
// @Description  正在拉流时先尽力停止 ZLM 拉流，再软删除配置
// @Tags         GbStreamProxy
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "代理ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-proxies/{id} [delete]
// @ID           deleteGbStreamProxy
func (h *GbStreamProxyHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "拉流代理")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamProxyNotFound)
		return
	}
	utils.Success(c, nil)
}

// Start 开始拉流
//
// @Summary      开始拉流
// @Description  调用 ZLM addStreamProxy；已拉起时幂等返回成功
// @Tags         GbStreamProxy
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "代理ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-proxies/{id}/start [post]
// @ID           startGbStreamProxy
func (h *GbStreamProxyHandler) Start(c *gin.Context) {
	id, ok := gbPathID(c, "拉流代理")
	if !ok {
		return
	}
	if err := h.svc.Start(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamProxyNotFound)
		return
	}
	utils.Success(c, nil)
}

// Stop 停止拉流
//
// @Summary      停止拉流
// @Description  调用 ZLM delStreamProxy（无 key 时回退 close_streams）；已停止时幂等返回成功
// @Tags         GbStreamProxy
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "代理ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-proxies/{id}/stop [post]
// @ID           stopGbStreamProxy
func (h *GbStreamProxyHandler) Stop(c *gin.Context) {
	id, ok := gbPathID(c, "拉流代理")
	if !ok {
		return
	}
	if err := h.svc.Stop(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamProxyNotFound)
		return
	}
	utils.Success(c, nil)
}
