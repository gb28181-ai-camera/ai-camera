package handler

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"

	// 仅用于 Swagger 注解中的类型解析（response 包在本文件正文中不出现）。
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"

	devicesvc "github.com/boticz/ai-camera-backend/internal/service/device"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 资源管理 - 物联网卡 / 换卡记录
// ════════════════════════════════════════════════════════════════

// IotCardHandler 物联网卡处理器。
type IotCardHandler struct {
	svc *devicesvc.IotCardService
}

// NewIotCardHandler 构造物联网卡处理器。
func NewIotCardHandler() *IotCardHandler {
	return &IotCardHandler{svc: devicesvc.NewIotCardService()}
}

// iotCardPathID 解析路径中的物联网卡 ID。
func iotCardPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的物联网卡ID")
		return 0, false
	}
	return id, true
}

// Page 物联网卡分页列表
//
// @Summary      物联网卡分页列表
// @Description  支持关键字（卡名称/卡号）、区域、所属设备、状态筛选；返回关联区域与设备
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        page       query     int     false  "页码"     default(1)
// @Param        page_size  query     int     false  "每页条数"  default(20)
// @Param        keyword    query     string  false  "关键字(卡名称/卡号)"
// @Param        region_id  query     int     false  "区域ID"
// @Param        device_id  query     int     false  "所属设备ID"
// @Param        status     query     string  false  "状态 normal/arrears/expired/disabled"
// @Success      200  {object}  response.RPageIotCard
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/iot-cards [get]
// @ID           pageIotCards
func (h *IotCardHandler) Page(c *gin.Context) {
	var req request.PageIotCardReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Options 物联网卡下拉选项
//
// @Summary      物联网卡下拉选项
// @Description  仅返回启用状态的物联网卡；name 为卡名称，code 为卡号
// @Tags         Device
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.ROptionList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/iot-cards/options [get]
// @ID           iotCardOptions
func (h *IotCardHandler) Options(c *gin.Context) {
	items, err := h.svc.Options(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, items)
}

// Create 新增物联网卡
//
// @Summary      新增物联网卡
// @Description  卡号全局唯一；status 由到期时间与剩余流量自动推导（已到期 expired、剩余流量<=0 arrears）
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateIotCardReq  true  "物联网卡"
// @Success      200  {object}  response.RIotCard
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/iot-cards [post]
// @ID           createIotCard
func (h *IotCardHandler) Create(c *gin.Context) {
	var req request.CreateIotCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, m)
}

// Update 更新物联网卡
//
// @Summary      更新物联网卡
// @Description  增量覆盖：文本非空才覆盖，数值大于 0 才覆盖，指针字段非 nil 才覆盖（传 0 表示清空）；保存后重新推导 status
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "物联网卡ID"
// @Param        body  body      request.UpdateIotCardReq  true  "物联网卡"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/iot-cards/{id} [put]
// @ID           updateIotCard
func (h *IotCardHandler) Update(c *gin.Context) {
	id, ok := iotCardPathID(c)
	if !ok {
		return
	}
	var req request.UpdateIotCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrIotCardNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除物联网卡
//
// @Summary      删除物联网卡
// @Description  软删除物联网卡
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "物联网卡ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/iot-cards/{id} [delete]
// @ID           deleteIotCard
func (h *IotCardHandler) Delete(c *gin.Context) {
	id, ok := iotCardPathID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondDeviceErr(c, err, devicesvc.ErrIotCardNotFound)
		return
	}
	utils.Success(c, nil)
}

// PageReplaceLogs 换卡记录分页列表
//
// @Summary      换卡记录分页列表
// @Description  支持按区域、设备、设备编号、卡号与更换时间区间筛选；返回关联设备与区域
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        page        query     int     false  "页码"        default(1)
// @Param        page_size   query     int     false  "每页条数"     default(20)
// @Param        region_id   query     int     false  "区域ID"
// @Param        device_id   query     int     false  "设备ID"
// @Param        device_code query     string  false  "设备编号(模糊)"
// @Param        card_no     query     string  false  "卡号(模糊，匹配原卡或新卡)"
// @Param        start_dt    query     string  false  "更换开始时间"
// @Param        end_dt      query     string  false  "更换结束时间"
// @Success      200  {object}  response.RPageIotCardReplaceLog
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/devices/iot-card-replace-logs [get]
// @ID           pageIotCardReplaceLogs
func (h *IotCardHandler) PageReplaceLogs(c *gin.Context) {
	var req request.PageIotCardReplaceLogReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.PageReplaceLogs(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondDeviceErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// ReplaceCard 设备换卡
//
// @Summary      设备换卡
// @Description  单事务内解绑旧卡、绑定新卡、更新设备当前物联网卡并写入换卡记录；old_iot_card_id 缺省时取设备当前绑定卡
// @Tags         Device
// @Accept       json
// @Produce      json
// @Param        body  body      request.ReplaceIotCardReq  true  "换卡信息"
// @Success      200  {object}  response.RIotCardReplaceLog
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/devices/iot-card-replace-logs [post]
// @ID           replaceIotCard
func (h *IotCardHandler) ReplaceCard(c *gin.Context) {
	var req request.ReplaceIotCardReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	// 操作人取自登录态，供换卡记录留痕
	operator := c.GetString("user_name")
	row, err := h.svc.ReplaceCard(c.Request.Context(), &req, operator)
	if err != nil {
		respondDeviceErr(c, err, devicesvc.ErrDeviceNotFound, devicesvc.ErrIotCardNotFound)
		return
	}
	utils.Success(c, row)
}
