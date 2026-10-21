package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 推流配置
//
// 推流地址由平台按媒体节点生成（rtmp://…），在列表的 push_url 字段返回。
// 本期没有 ZLM「向外部持续推流」的 REST API，start/stop 只维护启停状态，
// 实际推流由外部编码器按该地址推入 ZLM。
// ════════════════════════════════════════════════════════════════

// GbStreamPushHandler 推流处理器。
type GbStreamPushHandler struct {
	svc *gbsvc.GbStreamPushService
}

// NewGbStreamPushHandler 构造推流处理器。
func NewGbStreamPushHandler() *GbStreamPushHandler {
	return &GbStreamPushHandler{svc: gbsvc.NewGbStreamPushService()}
}

// Page 推流分页
//
// @Summary      推流分页
// @Description  返回配置与平台生成的 push_url（rtmp://<stream_ip>:<rtmp_port>/<app>/<stream>）
// @Tags         GbStreamPush
// @Accept       json
// @Produce      json
// @Param        page             query     int     false  "页码"     default(1)
// @Param        page_size        query     int     false  "每页条数"  default(20)
// @Param        keyword          query     string  false  "关键字(名称/app/stream)"
// @Param        pushing          query     bool    false  "是否正在推流"
// @Param        media_server_id  query     string  false  "媒体节点"
// @Param        status           query     string  false  "状态 ON/OFF"
// @Success      200  {object}  response.RPageGbStreamPush
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes [get]
// @ID           pageGbStreamPushes
func (h *GbStreamPushHandler) Page(c *gin.Context) {
	var req request.PageGbStreamPushReq
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

// Create 新增推流配置
//
// @Summary      新增推流配置
// @Description  app+stream 唯一；默认状态 OFF，调用 start 后置 ON
// @Tags         GbStreamPush
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateGbStreamPushReq  true  "推流信息"
// @Success      200  {object}  response.RGbStreamPush
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes [post]
// @ID           createGbStreamPush
func (h *GbStreamPushHandler) Create(c *gin.Context) {
	var req request.CreateGbStreamPushReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamPushExists)
		return
	}
	utils.Success(c, m)
}

// Update 编辑推流配置
//
// @Summary      编辑推流配置
// @Description  增量更新；修改 app/stream 时重新校验唯一性
// @Tags         GbStreamPush
// @Accept       json
// @Produce      json
// @Param        id    path      int                         true  "推流ID"
// @Param        body  body      request.UpdateGbStreamPushReq  true  "推流信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes/{id} [put]
// @ID           updateGbStreamPush
func (h *GbStreamPushHandler) Update(c *gin.Context) {
	id, ok := gbPathID(c, "推流")
	if !ok {
		return
	}
	var req request.UpdateGbStreamPushReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamPushNotFound, gbsvc.ErrGbStreamPushExists)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除推流配置
//
// @Summary      删除推流配置
// @Description  软删除；不会影响已经推入 ZLM 的外部流
// @Tags         GbStreamPush
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "推流ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes/{id} [delete]
// @ID           deleteGbStreamPush
func (h *GbStreamPushHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "推流")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamPushNotFound)
		return
	}
	utils.Success(c, nil)
}

// Start 启用推流
//
// @Summary      启用推流
// @Description  校验媒体节点可生成推流地址后置为 ON；实际推流由外部编码器按 push_url 推入
// @Tags         GbStreamPush
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "推流ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes/{id}/start [post]
// @ID           startGbStreamPush
func (h *GbStreamPushHandler) Start(c *gin.Context) {
	id, ok := gbPathID(c, "推流")
	if !ok {
		return
	}
	if err := h.svc.Start(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamPushNotFound)
		return
	}
	utils.Success(c, nil)
}

// Stop 停止推流
//
// @Summary      停止推流
// @Description  置为 OFF；不会主动切断外部编码器与 ZLM 的连接
// @Tags         GbStreamPush
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "推流ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes/{id}/stop [post]
// @ID           stopGbStreamPush
func (h *GbStreamPushHandler) Stop(c *gin.Context) {
	id, ok := gbPathID(c, "推流")
	if !ok {
		return
	}
	if err := h.svc.Stop(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbStreamPushNotFound)
		return
	}
	utils.Success(c, nil)
}
