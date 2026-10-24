package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"

	iotsvc "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// IOT 管理 - 产品
// ════════════════════════════════════════════════════════════════

// IotProductHandler IOT 产品处理器。
type IotProductHandler struct {
	svc  *iotsvc.ProductService
	cred *iotsvc.CredentialsService
}

// NewIotProductHandler 构造产品处理器。
func NewIotProductHandler() *IotProductHandler {
	return &IotProductHandler{
		svc:  iotsvc.NewProductService(),
		cred: iotsvc.NewCredentialsService(),
	}
}

// Page 产品分页列表
//
// @Summary      IOT产品分页列表
// @Description  支持关键字（名称/编码/分类）与启用状态筛选
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "页码"     default(1)
// @Param        page_size  query     int     false  "每页条数"  default(20)
// @Param        keyword    query     string  false  "关键字(名称/编码/分类)"
// @Param        active     query     bool    false  "是否启用"
// @Success      200  {object}  response.RPageIotProduct
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/iot/products [get]
// @ID           pageIotProducts
func (h *IotProductHandler) Page(c *gin.Context) {
	var req request.PageIotProductReq
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

// Get 产品详情
//
// @Summary      IOT产品详情
// @Description  返回产品全部配置字段
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "产品ID"
// @Success      200  {object}  response.RIotProduct
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/products/{id} [get]
// @ID           getIotProduct
func (h *IotProductHandler) Get(c *gin.Context) {
	id, ok := iotProductPathID(c)
	if !ok {
		return
	}
	m, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, m)
}

// Create 新增产品
//
// @Summary      新增IOT产品
// @Description  产品编码全局唯一；配网方式（qrcode/bluetooth/ap）、是否允许分享、
// @Description  服务组绑定都由产品自身配置决定，平台不按设备类型做联动约束。
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateIotProductReq  true  "产品"
// @Success      200  {object}  response.RIotProduct
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/iot/products [post]
// @ID           createIotProduct
func (h *IotProductHandler) Create(c *gin.Context) {
	var req request.CreateIotProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondIotErr(c, err)
		return
	}
	utils.Success(c, m)
}

// Update 更新产品
//
// @Summary      更新IOT产品
// @Description  增量覆盖：文本非空才覆盖，指针字段非 nil 才覆盖。产品已有设备绑定时配置冲突只返回 warning 不阻止
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id    path      int                          true  "产品ID"
// @Param        body  body      request.UpdateIotProductReq  true  "产品"
// @Success      200  {object}  response.RIotProductUpdated
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/products/{id} [put]
// @ID           updateIotProduct
func (h *IotProductHandler) Update(c *gin.Context) {
	id, ok := iotProductPathID(c)
	if !ok {
		return
	}
	var req request.UpdateIotProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	warning, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, response.IotProductUpdateResult{Warning: warning})
}

// Delete 删除产品
//
// @Summary      删除IOT产品
// @Description  软删除产品
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "产品ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/products/{id} [delete]
// @ID           deleteIotProduct
func (h *IotProductHandler) Delete(c *gin.Context) {
	id, ok := iotProductPathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondIotErr(c, err, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, nil)
}

// GenerateShareQR 生成/刷新分享二维码
//
// @Summary      生成产品分享二维码内容
// @Description  返回可直接编码为二维码的 JSON 字符串，并持久化到产品 share_qr_content
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "产品ID"
// @Success      200  {object}  response.RIotShareQR
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/products/{id}/share-qr [post]
// @ID           generateIotProductShareQR
func (h *IotProductHandler) GenerateShareQR(c *gin.Context) {
	id, ok := iotProductPathID(c)
	if !ok {
		return
	}
	resp, err := h.svc.GenerateShareQR(c.Request.Context(), id)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, resp)
}

// GetServiceGroups 查询产品已绑定服务组
//
// @Summary      查询产品已绑定服务组
// @Description  服务组密码脱敏返回
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "产品ID"
// @Success      200  {object}  response.RIotServiceGroupList
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/products/{id}/service-groups [get]
// @ID           getIotProductServiceGroups
func (h *IotProductHandler) GetServiceGroups(c *gin.Context) {
	id, ok := iotProductPathID(c)
	if !ok {
		return
	}
	items, err := h.svc.GetServiceGroups(c.Request.Context(), id)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, items)
}

// BindServiceGroups 绑定产品服务组（整体替换）
//
// @Summary      绑定产品服务组
// @Description  整体替换：未列出的服务组将被解绑
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id    path      int                                   true  "产品ID"
// @Param        body  body      request.BindIotProductServiceGroupsReq  true  "服务组ID列表"
// @Success      200  {object}  response.RIotServiceGroupList
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/products/{id}/service-groups [put]
// @ID           bindIotProductServiceGroups
func (h *IotProductHandler) BindServiceGroups(c *gin.Context) {
	id, ok := iotProductPathID(c)
	if !ok {
		return
	}
	var req request.BindIotProductServiceGroupsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	items, err := h.svc.BindServiceGroups(c.Request.Context(), id, req.ServiceGroupIDs)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, items)
}

// Credentials 产品明文鉴权信息（复制用）
//
// @Summary      产品明文鉴权信息
// @Description  返回产品及其全部服务组的明文用户名/密码、服务项与 MQTT 连接地址；仅本接口返回明文
// @Tags         IOT
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "产品ID"
// @Success      200  {object}  response.RIotCredentials
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/iot/products/{id}/credentials [get]
// @ID           getIotProductCredentials
func (h *IotProductHandler) Credentials(c *gin.Context) {
	id, ok := iotProductPathID(c)
	if !ok {
		return
	}
	resp, err := h.cred.ProductCredentials(c.Request.Context(), id)
	if err != nil {
		respondIotErr(c, err, iotsvc.ErrProductNotFound)
		return
	}
	utils.Success(c, resp)
}
