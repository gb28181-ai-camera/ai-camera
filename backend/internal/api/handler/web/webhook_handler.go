package handler

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	_ "github.com/boticz/ai-camera-backend/internal/dto/response"
	svc "github.com/boticz/ai-camera-backend/internal/service/setting"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// WebhookHandler 消息推送 WebHook（飞书 / 钉钉 / 企业微信 / 自定义）处理器。
type WebhookHandler struct {
	service *svc.WebhookService
}

// NewWebhookHandler 构造处理器。
func NewWebhookHandler() *WebhookHandler {
	return &WebhookHandler{service: svc.NewWebhookService()}
}

// Page 分页查询 WebHook 配置
// @Summary      分页查询 WebHook 配置
// @Description  支持按名称关键字、平台、启用状态筛选
// @Tags         Setting
// @ID           pageWebhooks
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page       query     int     false  "页码"      default(1)
// @Param        page_size  query     int     false  "每页条数"   default(20)
// @Param        keyword    query     string  false  "名称关键字"
// @Param        platform   query     string  false  "平台 feishu/dingtalk/wecom/custom"
// @Param        active     query     bool    false  "启用状态"
// @Success      200  {object}  response.RPageWebhook
// @Router       /web/settings/webhooks [get]
func (h *WebhookHandler) Page(c *gin.Context) {
	var req request.PageWebhookReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	resp, err := h.service.Page(c.Request.Context(), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// Platforms 平台选项
// @Summary      查询支持的 WebHook 平台
// @Description  返回飞书/钉钉/企业微信/自定义 WebHook 选项，供前端下拉使用
// @Tags         Setting
// @ID           webhookPlatforms
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.RWebhookPlatformList
// @Router       /web/settings/webhooks/platforms [get]
func (h *WebhookHandler) Platforms(c *gin.Context) {
	utils.Success(c, h.service.Platforms())
}

// Get WebHook 详情
// @Summary      查询 WebHook 配置详情
// @Tags         Setting
// @ID           getWebhook
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "配置ID"
// @Success      200  {object}  response.RWebhook
// @Router       /web/settings/webhooks/{id} [get]
func (h *WebhookHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "非法的配置ID")
		return
	}

	m, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		utils.NotFound(c, err.Error())
		return
	}
	utils.Success(c, m)
}

// Create 新增 WebHook 配置
// @Summary      新增 WebHook 配置
// @Description  支持飞书、钉钉（自动加签）、企业微信与自定义 WebHook
// @Tags         Setting
// @ID           createWebhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.CreateWebhookReq  true  "WebHook 配置"
// @Success      200   {object}  response.RWebhook
// @Router       /web/settings/webhooks [post]
func (h *WebhookHandler) Create(c *gin.Context) {
	var req request.CreateWebhookReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	m, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, m)
}

// Update 编辑 WebHook 配置
// @Summary      编辑 WebHook 配置
// @Description  secret 传空或脱敏串（含 ****）时保持原值
// @Tags         Setting
// @ID           updateWebhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int                        true  "配置ID"
// @Param        body  body      request.UpdateWebhookReq   true  "WebHook 配置"
// @Success      200   {object}  response.RBool
// @Router       /web/settings/webhooks/{id} [put]
func (h *WebhookHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "非法的配置ID")
		return
	}

	var req request.UpdateWebhookReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	if err := h.service.Update(c.Request.Context(), id, &req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// Delete 删除 WebHook 配置
// @Summary      删除 WebHook 配置
// @Tags         Setting
// @ID           deleteWebhook
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "配置ID"
// @Success      200  {object}  response.RBool
// @Router       /web/settings/webhooks/{id} [delete]
func (h *WebhookHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "非法的配置ID")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// Enable 启用 WebHook
// @Summary      启用 WebHook 配置
// @Tags         Setting
// @ID           enableWebhook
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "配置ID"
// @Success      200  {object}  response.RBool
// @Router       /web/settings/webhooks/{id}/enable [put]
func (h *WebhookHandler) Enable(c *gin.Context) {
	h.setActive(c, true)
}

// Disable 停用 WebHook
// @Summary      停用 WebHook 配置
// @Tags         Setting
// @ID           disableWebhook
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "配置ID"
// @Success      200  {object}  response.RBool
// @Router       /web/settings/webhooks/{id}/disable [put]
func (h *WebhookHandler) Disable(c *gin.Context) {
	h.setActive(c, false)
}

func (h *WebhookHandler) setActive(c *gin.Context, active bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "非法的配置ID")
		return
	}
	if err := h.service.SetActive(c.Request.Context(), id, active); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, true)
}

// Test 发送测试消息
// @Summary      向指定 WebHook 发送测试消息
// @Description  实时推送一条测试消息以验证通道连通性；失败原因在 data.message 中返回
// @Tags         Setting
// @ID           testWebhook
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int                     true  "配置ID"
// @Param        body  body      request.TestWebhookReq  false "自定义测试文案"
// @Success      200   {object}  response.RWebhookTest
// @Router       /web/settings/webhooks/{id}/test [post]
func (h *WebhookHandler) Test(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "非法的配置ID")
		return
	}

	var req request.TestWebhookReq
	_ = c.ShouldBindJSON(&req) // 允许空 body

	resp, err := h.service.Test(c.Request.Context(), id, &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// TestInline 即时测试未保存的配置
// @Summary      即时测试 WebHook 配置（不落库）
// @Description  用于新增配置时先验证连通性再保存
// @Tags         Setting
// @ID           testWebhookInline
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      request.TestWebhookInlineReq  true  "WebHook 配置"
// @Success      200   {object}  response.RWebhookTest
// @Router       /web/settings/webhooks/test [post]
func (h *WebhookHandler) TestInline(c *gin.Context) {
	var req request.TestWebhookInlineReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	resp, err := h.service.TestInline(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}
