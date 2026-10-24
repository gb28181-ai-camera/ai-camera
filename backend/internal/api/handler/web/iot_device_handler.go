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
// IOT 管理 - 设备与设备账号
// ════════════════════════════════════════════════════════════════

// IotDeviceHandler IOT 设备处理器。
type IotDeviceHandler struct {
	svc *iotsvc.DeviceService
}

// NewIotDeviceHandler 构造 IOT 设备处理器。
func NewIotDeviceHandler() *IotDeviceHandler {
	return &IotDeviceHandler{svc: iotsvc.NewDeviceService()}
}

// Page IOT设备分页列表
//
// @Summary      IOT设备分页列表
// @Description  数据源为 ac_device；支持关键字、产品、运行状态筛选；device_secret 脱敏
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"     default(1)
// @Param        page_size   query     int     false  "每页条数"  default(20)
// @Param        keyword     query     string  false  "关键字(设备名称/编号/位置)"
// @Param        product_id  query     int     false  "产品ID"
// @Param        status      query     string  false  "运行状态 online/offline"
// @Success      200  {object}  response.RPageIotDevice
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/iot/devices [get]
// @ID           pageIotDevices
func (h *IotDeviceHandler) Page(c *gin.Context) {
	var req request.PageIotDeviceReq
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

// Get IOT设备详情
//
// @Summary      IOT设备详情
// @Description  返回设备、账号列表与已授权服务组（含服务项）
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RIotDeviceDetail
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/devices/{id} [get]
// @ID           getIotDevice
func (h *IotDeviceHandler) Get(c *gin.Context) {
	id, ok := iotDevicePathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// CreateAccount 新增设备账号
//
// @Summary      新增设备账号
// @Description  缺省创建分享账号；password 留空时服务端生成并一次性返回
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id    path      int                                  true  "设备ID"
// @Param        body  body      request.CreateIotDeviceAccountReq    true  "设备账号"
// @Success      200  {object}  response.RIotDeviceAccountCreated
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/devices/{id}/accounts [post]
// @ID           createIotDeviceAccount
func (h *IotDeviceHandler) CreateAccount(c *gin.Context) {
	deviceID, ok := iotDevicePathID(c)
	if !ok {
		return
	}
	var req request.CreateIotDeviceAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.CreateAccount(c.Request.Context(), deviceID, &req, c.ClientIP())
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// UpdateAccount 更新设备账号
//
// @Summary      更新设备账号
// @Description  role 不可修改；password 留空保持原值
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id         path      int                                  true  "设备ID"
// @Param        accountId  path      int                                  true  "账号ID"
// @Param        body       body      request.UpdateIotDeviceAccountReq    true  "设备账号"
// @Success      200  {object}  response.RIotDeviceAccount
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/devices/{id}/accounts/{accountId} [put]
// @ID           updateIotDeviceAccount
func (h *IotDeviceHandler) UpdateAccount(c *gin.Context) {
	deviceID, ok := iotDevicePathID(c)
	if !ok {
		return
	}
	accountID, ok := iotAccountPathID(c)
	if !ok {
		return
	}
	var req request.UpdateIotDeviceAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.UpdateAccount(c.Request.Context(), deviceID, accountID, &req)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrAccountNotFound)
		return
	}
	utils.Success(c, m)
}

// DeleteAccount 删除设备账号
//
// @Summary      删除设备账号
// @Description  主账号（owner）不允许删除
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id         path      int  true  "设备ID"
// @Param        accountId  path      int  true  "账号ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/devices/{id}/accounts/{accountId} [delete]
// @ID           deleteIotDeviceAccount
func (h *IotDeviceHandler) DeleteAccount(c *gin.Context) {
	deviceID, ok := iotDevicePathID(c)
	if !ok {
		return
	}
	accountID, ok := iotAccountPathID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteAccount(c.Request.Context(), deviceID, accountID); err != nil {
		respondIotErr(c, err, iotsvc.ErrAccountNotFound)
		return
	}
	utils.Success(c, nil)
}

// Activate 手动激活设备
//
// @Summary      手动激活设备主账号
// @Description  幂等：主账号已存在时返回既有账号，密码只在首次创建时返回
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RIotDeviceActivated
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/devices/{id}/activate [post]
// @ID           activateIotDevice
func (h *IotDeviceHandler) Activate(c *gin.Context) {
	id, ok := iotDevicePathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.Activate(c.Request.Context(), id, c.ClientIP())
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// ResetSecret 重置设备接入密钥
//
// @Summary      重置设备接入密钥
// @Description  返回一次性明文 device_secret；设备需同步更新后才能继续 OTA
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RIotDeviceSecret
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/devices/{id}/reset-secret [post]
// @ID           resetIotDeviceSecret
func (h *IotDeviceHandler) ResetSecret(c *gin.Context) {
	id, ok := iotDevicePathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.ResetSecret(c.Request.Context(), id, c.ClientIP())
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrDeviceNotFound)
		return
	}
	utils.Success(c, resp)
}

// SetProduct 设置设备所属产品
//
// @Summary      设置设备所属产品
// @Description  product_id 为空表示解绑
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id    path      int                               true  "设备ID"
// @Param        body  body      request.UpdateIotDeviceProductReq  true  "产品ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/devices/{id}/product [put]
// @ID           updateIotDeviceProduct
func (h *IotDeviceHandler) SetProduct(c *gin.Context) {
	id, ok := iotDevicePathID(c)
	if !ok {
		return
	}
	var req request.UpdateIotDeviceProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SetProduct(c.Request.Context(), id, req.ProductID); err != nil {
		respondIotErr(c, err, iotsvc.ErrDeviceNotFound, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, nil)
}
