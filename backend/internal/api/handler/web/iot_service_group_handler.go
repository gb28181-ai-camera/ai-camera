package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"

	// 仅用于 Swagger 注解中的类型解析。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"

	iotsvc "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 服务组 / 服务项
// ════════════════════════════════════════════════════════════════

// IotServiceGroupHandler 服务组处理器。
type IotServiceGroupHandler struct {
	svc *iotsvc.ServiceGroupService
}

// NewIotServiceGroupHandler 构造服务组处理器。
func NewIotServiceGroupHandler() *IotServiceGroupHandler {
	return &IotServiceGroupHandler{svc: iotsvc.NewServiceGroupService()}
}

// Page 服务组分页列表
//
// @Summary      IOT服务组分页列表
// @Description  支持关键字（名称/编码/用户名）与类型筛选；密码脱敏
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "页码"     default(1)
// @Param        page_size  query     int     false  "每页条数"  default(20)
// @Param        keyword    query     string  false  "关键字(名称/编码/用户名)"
// @Param        type       query     string  false  "类型 gb28181/webrtc/aichat"
// @Param        active     query     bool    false  "是否启用"
// @Success      200  {object}  response.RPageIotServiceGroup
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/iot/service-groups [get]
// @ID           pageIotServiceGroups
func (h *IotServiceGroupHandler) Page(c *gin.Context) {
	var req request.PageIotServiceGroupReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req)
	if err != nil {
		respondIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Get 服务组详情
//
// @Summary      IOT服务组详情
// @Description  返回服务组及全部服务项；密码脱敏为 ****
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "服务组ID"
// @Success      200  {object}  response.RIotServiceGroup
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/service-groups/{id} [get]
// @ID           getIotServiceGroup
func (h *IotServiceGroupHandler) Get(c *gin.Context) {
	id, ok := iotServiceGroupPathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrServiceGroupNotFound)
		return
	}
	utils.Success(c, resp)
}

// Create 新增服务组
//
// @Summary      新增IOT服务组
// @Description  自动生成组级用户名与 16 位密码；服务组类型决定允许的协议
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateIotServiceGroupReq  true  "服务组"
// @Success      200  {object}  response.RIotServiceGroup
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/iot/service-groups [post]
// @ID           createIotServiceGroup
func (h *IotServiceGroupHandler) Create(c *gin.Context) {
	var req request.CreateIotServiceGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Update 更新服务组
//
// @Summary      更新IOT服务组
// @Description  password 为空或 **** 时保持原密码；类型变更要求已有服务项兼容
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id    path      int                              true  "服务组ID"
// @Param        body  body      request.UpdateIotServiceGroupReq  true  "服务组"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/service-groups/{id} [put]
// @ID           updateIotServiceGroup
func (h *IotServiceGroupHandler) Update(c *gin.Context) {
	id, ok := iotServiceGroupPathID(c)
	if !ok {
		return
	}
	var req request.UpdateIotServiceGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondIotErr(c, err, iotsvc.ErrServiceGroupNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除服务组
//
// @Summary      删除IOT服务组
// @Description  被产品引用时拒绝删除
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "服务组ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/service-groups/{id} [delete]
// @ID           deleteIotServiceGroup
func (h *IotServiceGroupHandler) Delete(c *gin.Context) {
	id, ok := iotServiceGroupPathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondIotErr(c, err, iotsvc.ErrServiceGroupNotFound)
		return
	}
	utils.Success(c, nil)
}

// ResetCredentials 重置服务组凭据
//
// @Summary      重置服务组用户名与密码
// @Description  返回一次性明文凭据，供管理员同步到设备；下次 OTA 生效
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "服务组ID"
// @Success      200  {object}  response.RIotServiceGroupCredentials
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/service-groups/{id}/reset-credentials [post]
// @ID           resetIotServiceGroupCredentials
func (h *IotServiceGroupHandler) ResetCredentials(c *gin.Context) {
	id, ok := iotServiceGroupPathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.ResetCredentials(c.Request.Context(), id)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrServiceGroupNotFound)
		return
	}
	utils.Success(c, resp)
}

// ReplaceItems 替换服务项
//
// @Summary      整体替换服务组服务项
// @Description  按服务组类型校验协议白名单；未列出的服务项会被删除
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id    path      int                               true  "服务组ID"
// @Param        body  body      request.ReplaceIotServiceItemsReq  true  "服务项列表"
// @Success      200  {object}  response.RIotServiceItemList
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/service-groups/{id}/items [put]
// @ID           replaceIotServiceGroupItems
func (h *IotServiceGroupHandler) ReplaceItems(c *gin.Context) {
	id, ok := iotServiceGroupPathID(c)
	if !ok {
		return
	}
	var req request.ReplaceIotServiceItemsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	items, err := h.svc.ReplaceItems(c.Request.Context(), id, &req)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrServiceGroupNotFound)
		return
	}
	utils.Success(c, gin.H{"items": items})
}
