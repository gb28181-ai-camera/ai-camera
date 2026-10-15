package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// NotificationHandler 站内通知（仅限当前登录用户自己的数据）。
type NotificationHandler struct{}

// NewNotificationHandler 构造通知处理器。
func NewNotificationHandler() *NotificationHandler { return &NotificationHandler{} }

// currentUserID 读取 JWT 中的用户 ID，缺失时已写入 401 响应并返回 false。
func currentUserID(c *gin.Context) (int64, bool) {
	userID := c.GetInt64("user_id")
	if userID <= 0 {
		utils.Unauthorized(c, "未提供认证Token")
		return 0, false
	}
	return userID, true
}

// Page 站内通知分页列表
// @Summary 站内通知分页列表
// @Description 仅返回当前登录用户的通知，按创建时间倒序。
// @ID pageNotifications
// @Tags System
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param category query string false "通知分类 alarm/risk/inspect"
// @Param read query bool false "是否已读"
// @Success 200 {object} response.RPageNotification
// @Router /web/system/notifications [get]
func (h *NotificationHandler) Page(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var req request.PageNotificationReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	resp, err := system.NewNotificationService().Page(c.Request.Context(), userID, &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, resp)
}

// UnreadCount 未读通知数量
// @Summary 未读通知数量
// @ID unreadNotificationCount
// @Tags System
// @Success 200 {object} response.RUnreadCount
// @Router /web/system/notifications/unread-count [get]
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	n, err := system.NewNotificationService().UnreadCount(c.Request.Context(), userID)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, n)
}

// MarkRead 标记通知已读
// @Summary 标记通知已读
// @ID markNotificationRead
// @Tags System
// @Param id path int true "通知ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/notifications/{id}/read [put]
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewNotificationService().MarkRead(c.Request.Context(), userID, id); err != nil {
		systemRespondErr(c, err, "通知不存在")
		return
	}
	utils.Success(c, nil)
}

// MarkAllRead 全部标记已读
// @Summary 全部标记已读
// @Description 将当前登录用户的全部未读通知标记为已读。该路由必须在 /notifications/{id} 之前注册。
// @ID markAllNotificationsRead
// @Tags System
// @Success 200 {object} response.BaseResponse
// @Router /web/system/notifications/read-all [put]
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	if err := system.NewNotificationService().MarkAllRead(c.Request.Context(), userID); err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, nil)
}

// Delete 删除通知
// @Summary 删除通知
// @ID deleteNotification
// @Tags System
// @Param id path int true "通知ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/notifications/{id} [delete]
func (h *NotificationHandler) Delete(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewNotificationService().Delete(c.Request.Context(), userID, id); err != nil {
		systemRespondErr(c, err, "通知不存在")
		return
	}
	utils.Success(c, nil)
}
