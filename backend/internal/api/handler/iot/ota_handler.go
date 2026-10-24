package iot

import (
	"errors"
	"net/http"

	"github.com/boticz/ai-camera-backend/internal/dto/request"

	// 仅用于 Swagger 注解中的类型解析。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"

	svciot "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 设备端接口（无 JWT / 无 Casbin）
//
// 鉴权头：
//   X-Device-Code    设备编号（ac_device.device_code）
//   X-Device-Secret  设备接入密钥（ac_device.device_secret，明文仅展示一次）
//
// 这两个接口由设备固件直接调用，因此不做用户登录，也不进入 Casbin 策略。
// ════════════════════════════════════════════════════════════════

const (
	headerDeviceCode   = "X-Device-Code"
	headerDeviceSecret = "X-Device-Secret"
)

// DeviceHandler 设备端处理器。
type DeviceHandler struct {
	svc *svciot.DeviceEndpointService
}

// NewDeviceHandler 构造设备端处理器。
func NewDeviceHandler() *DeviceHandler {
	return &DeviceHandler{svc: svciot.NewDeviceEndpointService()}
}

// respondDeviceErr 设备端错误映射：鉴权失败 401，重复激活 409，其余 400。
func respondDeviceErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, svciot.ErrDeviceCredentialMissing),
		errors.Is(err, svciot.ErrDeviceSecretNotSet),
		errors.Is(err, svciot.ErrDeviceSecretInvalid):
		utils.Unauthorized(c, err.Error())
	case errors.Is(err, svciot.ErrAlreadyActivated):
		c.JSON(http.StatusConflict, utils.Response{Code: http.StatusConflict, Message: err.Error()})
	default:
		utils.BadRequest(c, err.Error())
	}
}

// OTA 设备拉取产品与服务授权配置
//
// @Summary      设备端 OTA
// @Description  返回产品信息、设备信息与产品绑定的服务组（含明文组凭据，供设备登录服务）
// @Tags         IOT-Device
// @Accept       json
// @Produce      json
// @Param        X-Device-Code    header    string            true   "设备编号"
// @Param        X-Device-Secret  header    string            true   "设备接入密钥"
// @Param        body             body      request.IotOTAReq  false  "OTA请求"
// @Success      200  {object}  response.RIotOTA
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Router       /iot/ota [post]
// @ID           deviceOTA
func (h *DeviceHandler) OTA(c *gin.Context) {
	// body 可选：设备可以不传或传空对象。
	var req request.IotOTAReq
	_ = c.ShouldBindJSON(&req)

	resp, err := h.svc.OTA(
		c.Request.Context(),
		c.GetHeader(headerDeviceCode),
		c.GetHeader(headerDeviceSecret),
		req.ClientVersion,
		c.ClientIP(),
	)
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Activate 设备激活主账号
//
// @Summary      设备端激活
// @Description  首次激活创建设备主账号并返回一次性明文密码；重复激活返回 409
// @Tags         IOT-Device
// @Accept       json
// @Produce      json
// @Param        X-Device-Code    header    string                 true   "设备编号"
// @Param        X-Device-Secret  header    string                 true   "设备接入密钥"
// @Param        body             body      request.IotActivateReq  false  "激活请求"
// @Success      200  {object}  response.RIotDeviceActivateResp
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Failure      409  {object}  response.BaseResponse
// @Router       /iot/activate [post]
// @ID           deviceActivate
func (h *DeviceHandler) Activate(c *gin.Context) {
	var req request.IotActivateReq
	_ = c.ShouldBindJSON(&req)

	resp, err := h.svc.Activate(
		c.Request.Context(),
		c.GetHeader(headerDeviceCode),
		c.GetHeader(headerDeviceSecret),
		&req,
		c.ClientIP(),
	)
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, resp)
}
