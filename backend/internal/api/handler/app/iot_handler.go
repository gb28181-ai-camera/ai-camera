package app

import (
	"errors"

	"github.com/boticz/ai-camera-backend/internal/dto/request"

	// 仅用于 Swagger 注解中的类型解析。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"

	iotsvc "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 移动端 IOT：设备认领 / 绑定 / 查询 / ICE
//
// 路由归属 router/app.go 的 aAuth 组（authMW → casbinMW），
// 因此这里不需要解析 JWT，user_id 直接取自中间件写入的上下文。
// ════════════════════════════════════════════════════════════════

// AppIotHandler 移动端 IOT 处理器（无状态）。
type AppIotHandler struct {
	svc *iotsvc.AppDeviceService
}

// NewAppIotHandler 构造移动端 IOT 处理器。
func NewAppIotHandler() *AppIotHandler {
	return &AppIotHandler{svc: iotsvc.NewAppDeviceService()}
}

// respondAppIotErr 把 IOT 业务错误映射为移动端 HTTP 状态。
//
// 认领/绑定的输入错误（令牌失效、密钥错误）统一 400：这是用户可纠正的操作，
// 不是鉴权失败；设备/绑定关系不存在用 404；其余按 500 暴露原始中文信息。
func respondAppIotErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, iotsvc.ErrShareTokenNotFound),
		errors.Is(err, iotsvc.ErrDeviceCredentialMissing),
		errors.Is(err, iotsvc.ErrDeviceSecretNotSet),
		errors.Is(err, iotsvc.ErrDeviceSecretInvalid),
		errors.Is(err, iotsvc.ErrBindQRInvalid),
		errors.Is(err, iotsvc.ErrProductShareDisabled):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, iotsvc.ErrDeviceNotOwner):
		utils.Forbidden(c, err.Error())
	case errors.Is(err, iotsvc.ErrDeviceNotFound),
		errors.Is(err, iotsvc.ErrDeviceNotBound):
		utils.NotFound(c, err.Error())
	default:
		utils.ServerError(c, err.Error())
	}
}

// appUserID 读取当前登录用户；中间件保证存在，但显式判空避免 0 号用户越权。
func appUserID(c *gin.Context) (int64, bool) {
	id := c.GetInt64("user_id")
	if id <= 0 {
		utils.Unauthorized(c, "未提供认证Token")
		return 0, false
	}
	return id, true
}

// Claim 扫码认领设备
//
// @Summary      扫码认领设备
// @Description  使用分享二维码中的一次性 token 把共享账号绑定到当前用户；同一 token 只能成功一次
// @Tags         App-IOT
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.ClaimIotDeviceReq  true  "分享令牌"
// @Success      200   {object}  response.RIotAppDeviceBinding
// @Failure      400   {object}  response.BaseResponse
// @Failure      401   {object}  response.BaseResponse
// @Router       /app/iot/devices/claim [post]
// @ID           claimAppIotDevice
func (h *AppIotHandler) Claim(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	var req request.ClaimIotDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Claim(c.Request.Context(), userID, &req)
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Bind 蓝牙配网后绑定设备
//
// @Summary      绑定设备
// @Description  校验 device_code + device_secret 后把设备绑定到当前用户；设备无主账号时创建 owner，否则创建 shared
// @Tags         App-IOT
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.BindIotDeviceReq  true  "设备编号与接入密钥"
// @Success      200   {object}  response.RIotAppDeviceBinding
// @Failure      400   {object}  response.BaseResponse
// @Failure      401   {object}  response.BaseResponse
// @Router       /app/iot/devices/bind [post]
// @ID           bindAppIotDevice
func (h *AppIotHandler) Bind(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	var req request.BindIotDeviceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Bind(c.Request.Context(), userID, &req, c.ClientIP())
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// BindByQR 扫码绑定设备
//
// @Summary      扫码绑定设备
// @Description  使用设备二维码中的 product_code + device_code 绑定；扫码即视为持有设备，无需密钥
// @Tags         App-IOT
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.BindIotDeviceQRReq  true  "产品编码与设备编码"
// @Success      200   {object}  response.RIotAppDeviceBinding
// @Failure      400   {object}  response.BaseResponse
// @Failure      401   {object}  response.BaseResponse
// @Router       /app/iot/devices/bind-qr [post]
// @ID           bindAppIotDeviceByQR
func (h *AppIotHandler) BindByQR(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	var req request.BindIotDeviceQRReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.BindByQR(c.Request.Context(), userID, &req, c.ClientIP())
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// List 我的设备列表
//
// @Summary      我的设备列表
// @Description  仅返回当前用户已认领/绑定的未删除设备
// @Tags         App-IOT
// @Produce      json
// @Security     BearerAuth
// @Success      200   {object}  response.RIotAppDeviceList
// @Failure      401   {object}  response.BaseResponse
// @Router       /app/iot/devices [get]
// @ID           listAppIotDevices
func (h *AppIotHandler) List(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	items, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, items)
}

// Get 我的设备详情
//
// @Summary      我的设备详情
// @Description  返回设备、产品、当前用户在该设备上的账号角色与服务组摘要
// @Tags         App-IOT
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      int  true  "设备ID"
// @Success      200   {object}  response.RIotAppDeviceDetail
// @Failure      401   {object}  response.BaseResponse
// @Failure      404   {object}  response.BaseResponse
// @Router       /app/iot/devices/{id} [get]
// @ID           getAppIotDevice
func (h *AppIotHandler) Get(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), userID, id)
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// CreateShareToken 生成设备分享码
//
// @Summary      生成设备分享码
// @Description  仅设备主账号(owner)可调用；生成一次性分享令牌与 shared 占位账号，返回结构与 Web 端一致
// @Tags         App-IOT
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      int  true  "设备ID"
// @Success      200   {object}  response.RIotDeviceShareToken
// @Failure      401   {object}  response.BaseResponse
// @Failure      403   {object}  response.BaseResponse
// @Failure      404   {object}  response.BaseResponse
// @Router       /app/iot/devices/{id}/share-token [post]
// @ID           createAppIotDeviceShareToken
func (h *AppIotHandler) CreateShareToken(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	resp, err := h.svc.CreateShareToken(c.Request.Context(), userID, id, c.ClientIP())
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Services 设备服务组（含明文凭据）
//
// @Summary      设备服务组
// @Description  返回设备产品的服务组明文凭据、服务项、MQTT URL 与 topic 前缀；仅绑定用户可访问
// @Tags         App-IOT
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      int  true  "设备ID"
// @Success      200   {object}  response.RIotAppServices
// @Failure      401   {object}  response.BaseResponse
// @Failure      404   {object}  response.BaseResponse
// @Router       /app/iot/devices/{id}/services [get]
// @ID           getAppIotDeviceServices
func (h *AppIotHandler) Services(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Services(c.Request.Context(), userID, id)
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// IceServers ICE 服务器
//
// @Summary      ICE 服务器
// @Description  返回 TURN/STUN 地址与短期凭据（subject=当前用户ID）；未配置 TURN 时 data 为空数组
// @Tags         App-RTC
// @Produce      json
// @Security     BearerAuth
// @Success      200   {object}  response.RIceServerList
// @Failure      401   {object}  response.BaseResponse
// @Router       /app/rtc/ice-servers [get]
// @ID           listAppIceServers
func (h *AppIotHandler) IceServers(c *gin.Context) {
	userID, ok := appUserID(c)
	if !ok {
		return
	}
	items, err := h.svc.IceServers(c.Request.Context(), userID)
	if err != nil {
		respondAppIotErr(c, err)
		return
	}
	utils.Success(c, items)
}
