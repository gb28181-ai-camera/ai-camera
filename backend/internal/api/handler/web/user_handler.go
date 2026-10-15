package handler

import (
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// parseSystemPathID 解析路径参数 :id。
//
// 用户/角色/个人中心等系统管理处理器同属本包，共用该解析逻辑；
// 解析失败时直接写出 400 响应并返回 false，调用方应立即 return。
func parseSystemPathID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的ID")
		return 0, false
	}
	return id, true
}

// UserHandler 用户管理接口。
type UserHandler struct{}

// NewUserHandler 构造用户处理器。
func NewUserHandler() *UserHandler { return &UserHandler{} }

// Page 用户分页列表
// @Summary 用户分页列表
// @Description 支持按关键字（姓名/登录名/手机号）、状态、部门、岗位筛选；返回关联的部门/岗位/角色名称
// @ID pageUsers
// @Tags System
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字(姓名/登录名/手机号)"
// @Param active query bool false "是否启用"
// @Param dept_id query int false "部门ID"
// @Param post_id query int false "岗位ID"
// @Success 200 {object} response.RPageUser
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/users [get]
func (h *UserHandler) Page(c *gin.Context) {
	var req request.PageUserReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userSvc := system.NewUserService()
	resp, err := userSvc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, resp)
}

// Options 用户下拉选项
// @Summary 用户下拉选项
// @Description 返回启用用户的 id/姓名/登录名，供选择器使用
// @ID userOptions
// @Tags System
// @Produce json
// @Success 200 {object} response.ROptionList
// @Router /web/system/users/options [get]
func (h *UserHandler) Options(c *gin.Context) {
	userSvc := system.NewUserService()
	items, err := userSvc.Options(c.Request.Context())
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, items)
}

// Get 用户详情
// @Summary 用户详情
// @ID getUser
// @Tags System
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} response.RUser
// @Failure 404 {object} response.BaseResponse
// @Router /web/system/users/{id} [get]
func (h *UserHandler) Get(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	userSvc := system.NewUserService()
	user, err := userSvc.Get(c.Request.Context(), id)
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	utils.Success(c, user)
}

// Create 创建用户
// @Summary 创建用户
// @Description 登录名唯一；可选同时分配角色（role_ids）与岗位
// @ID createUser
// @Tags System
// @Accept json
// @Produce json
// @Param body body request.CreateUserReq true "用户信息"
// @Success 200 {object} response.RUser
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/users [post]
func (h *UserHandler) Create(c *gin.Context) {
	var req request.CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userSvc := system.NewUserService()
	user, err := userSvc.Create(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, user)
}

// Update 更新用户
// @Summary 更新用户
// @Description 字符串字段为空表示不修改；dept_id/post_id/active/is_admin 传 null 表示不修改
// @ID updateUser
// @Tags System
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param body body request.UpdateUserReq true "用户信息"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/users/{id} [put]
func (h *UserHandler) Update(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	var req request.UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userSvc := system.NewUserService()
	if err := userSvc.Update(c.Request.Context(), id, &req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// Delete 删除用户
// @Summary 删除用户
// @Description 软删除用户，同时清理角色/岗位关联与 Casbin 授权
// @ID deleteUser
// @Tags System
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/users/{id} [delete]
func (h *UserHandler) Delete(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	userSvc := system.NewUserService()
	if err := userSvc.Delete(c.Request.Context(), id); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// Enable 启用用户
// @Summary 启用用户
// @ID enableUser
// @Tags System
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/users/{id}/enable [put]
func (h *UserHandler) Enable(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	userSvc := system.NewUserService()
	if err := userSvc.SetActive(c.Request.Context(), id, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// Disable 禁用用户
// @Summary 禁用用户
// @Description 禁用后该用户无法登录
// @ID disableUser
// @Tags System
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/users/{id}/disable [put]
func (h *UserHandler) Disable(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	userSvc := system.NewUserService()
	if err := userSvc.SetActive(c.Request.Context(), id, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// ResetPassword 重置用户密码
// @Summary 重置用户密码
// @Description 管理员重置指定用户密码，无需提供原密码
// @ID resetUserPassword
// @Tags System
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Param body body request.ResetPasswordReq true "新密码"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/users/{id}/reset-password [put]
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	var req request.ResetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userSvc := system.NewUserService()
	if err := userSvc.ResetPassword(c.Request.Context(), id, req.NewPassword); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}
