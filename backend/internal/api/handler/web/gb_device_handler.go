package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标设备
// ════════════════════════════════════════════════════════════════

// GbDeviceHandler 国标设备处理器。
type GbDeviceHandler struct {
	svc *gbsvc.GbDeviceService
}

// NewGbDeviceHandler 构造国标设备处理器。
func NewGbDeviceHandler() *GbDeviceHandler {
	return &GbDeviceHandler{svc: gbsvc.NewGbDeviceService()}
}

// gbPathID 解析路径中的主键 ID（国标接口路由参数一律是数据库主键，不是国标编码）。
func gbPathID(c *gin.Context, label string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的"+label+"ID")
		return 0, false
	}
	return id, true
}

// respondGbErr 统一国标业务错误映射。
//
// SIP 信令服务未启动返回 503 而不是 400：这不是参数错误，而是服务端能力缺失，
// 前端/运维看到 503 才能判断该去启动 sipsvc。
func respondGbErr(c *gin.Context, err error, notFound ...error) {
	for _, target := range notFound {
		if target != nil && errors.Is(err, target) {
			utils.NotFound(c, err.Error())
			return
		}
	}
	if errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
		c.JSON(http.StatusServiceUnavailable, utils.Response{Code: http.StatusServiceUnavailable, Message: err.Error()})
		return
	}
	utils.BadRequest(c, err.Error())
}

// Page 国标设备分页列表
//
// @Summary      国标设备分页列表
// @Description  只返回已配置国标接入（gb_device_id 非空）的设备；支持关键字、注册状态、运行状态、区域筛选；返回关联区域名称
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        page             query     int     false  "页码"        default(1)
// @Param        page_size        query     int     false  "每页条数"     default(20)
// @Param        keyword          query     string  false  "关键字(名称/编号/国标编码)"
// @Param        register_status  query     string  false  "注册状态 pending/activated/rejected"
// @Param        status           query     string  false  "运行状态 online/offline"
// @Param        region_id        query     int     false  "区域ID"
// @Success      200  {object}  response.RPageGbDevice
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/devices [get]
// @ID           pageGbDevices
func (h *GbDeviceHandler) Page(c *gin.Context) {
	var req request.PageGbDeviceReq
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

// Get 国标设备详情
//
// @Summary      国标设备详情
// @Description  返回 13 个国标接入列（sip_password 脱敏为 ****）、关联区域与媒体节点名称
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RGbDevice
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id} [get]
// @ID           getGbDevice
func (h *GbDeviceHandler) Get(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// SyncChannel 同步设备目录
//
// @Summary      同步设备目录
// @Description  向设备下发 Catalog 查询；设备异步返回的目录由信令层落库。SIP 服务未启动时返回 503
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/sync-channel [post]
// @ID           syncGbDeviceChannel
func (h *GbDeviceHandler) SyncChannel(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	if err := h.svc.SyncChannel(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound, gbsvc.ErrGbDeviceNotConfigured)
		return
	}
	utils.Success(c, nil)
}

// Subscribe 订阅设备目录/报警/位置
//
// @Summary      订阅设备
// @Description  发送 SIP SUBSCRIBE；type 取值 catalog/alarm/mobile_position。SIP 服务未启动时返回 503
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id    path      int                         true  "设备ID"
// @Param        body  body      request.GbSubscribeActionReq  true  "订阅类型"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/subscribe [post]
// @ID           subscribeGbDevice
func (h *GbDeviceHandler) Subscribe(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	var req request.GbSubscribeActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Subscribe(c.Request.Context(), id, req.Type); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound, gbsvc.ErrGbDeviceNotConfigured)
		return
	}
	utils.Success(c, nil)
}

// Unsubscribe 取消订阅
//
// @Summary      取消订阅设备
// @Description  发送 SIP 取消订阅；type 取值 catalog/alarm/mobile_position
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id    path      int     true  "设备ID"
// @Param        type  query     string  true  "订阅类型 catalog/alarm/mobile_position"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      503  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/subscribe [delete]
// @ID           unsubscribeGbDevice
func (h *GbDeviceHandler) Unsubscribe(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	if err := h.svc.Unsubscribe(c.Request.Context(), id, c.Query("type")); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound, gbsvc.ErrGbDeviceNotConfigured)
		return
	}
	utils.Success(c, nil)
}

// GenerateActivation 生成激活凭证
//
// @Summary      生成设备激活凭证
// @Description  国标编码为空时按 <domain 10 位><132><序号 7 位> 生成；已存在时保持原值（幂等）。返回明文 SIP 密码与二维码 JSON 内容
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RGbActivation
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id}/activation [post]
// @ID           generateGbActivation
func (h *GbDeviceHandler) GenerateActivation(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	resp, err := h.svc.GenerateActivation(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// Delete 删除国标设备
//
// @Summary      删除国标设备
// @Description  软删除设备，并同事务软删除其全部通道
// @Tags         GbDevice
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/devices/{id} [delete]
// @ID           deleteGbDevice
func (h *GbDeviceHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "设备")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbDeviceNotFound)
		return
	}
	utils.Success(c, nil)
}
