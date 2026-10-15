package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// RoleHandler 角色与授权管理接口。
type RoleHandler struct{}

// NewRoleHandler 构造角色处理器。
func NewRoleHandler() *RoleHandler { return &RoleHandler{} }

// Page 角色分页列表
// @Summary 角色分页列表
// @Description 支持按关键字（角色名称/权限字符）与状态筛选
// @ID pageRoles
// @Tags System
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键字(角色名称/权限字符)"
// @Param active query bool false "是否启用"
// @Success 200 {object} response.RPageRole
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles [get]
func (h *RoleHandler) Page(c *gin.Context) {
	var req request.PageRoleReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	roleSvc := system.NewRoleService()
	resp, err := roleSvc.Page(c.Request.Context(), &req)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, resp)
}

// Options 角色下拉选项
// @Summary 角色下拉选项
// @Description 返回启用角色的 id/名称/权限字符，供选择器使用
// @ID roleOptions
// @Tags System
// @Produce json
// @Success 200 {object} response.ROptionList
// @Router /web/system/roles/options [get]
func (h *RoleHandler) Options(c *gin.Context) {
	roleSvc := system.NewRoleService()
	items, err := roleSvc.Options(c.Request.Context())
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, items)
}

// Get 角色详情
// @Summary 角色详情
// @Description 返回角色信息及已授权菜单
// @ID getRole
// @Tags System
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.RRole
// @Failure 404 {object} response.BaseResponse
// @Router /web/system/roles/{id} [get]
func (h *RoleHandler) Get(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	roleSvc := system.NewRoleService()
	role, err := roleSvc.Get(c.Request.Context(), id)
	if err != nil {
		utils.NotFound(c, "角色不存在")
		return
	}

	utils.Success(c, role)
}

// Create 创建角色
// @Summary 创建角色
// @Description 权限字符（code）全局唯一，是 Casbin 的角色主体；可同时授权菜单
// @ID createRole
// @Tags System
// @Accept json
// @Produce json
// @Param body body request.CreateRoleReq true "角色信息"
// @Success 200 {object} response.RRole
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles [post]
func (h *RoleHandler) Create(c *gin.Context) {
	var req request.CreateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	roleSvc := system.NewRoleService()
	role, err := roleSvc.Create(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, role)
}

// Update 更新角色
// @Summary 更新角色
// @Description 字符串字段为空表示不修改；menu_ids 为 null 表示不调整菜单授权，传空数组表示清空
// @ID updateRole
// @Tags System
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param body body request.UpdateRoleReq true "角色信息"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id} [put]
func (h *RoleHandler) Update(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	var req request.UpdateRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	roleSvc := system.NewRoleService()
	if err := roleSvc.Update(c.Request.Context(), id, &req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// Delete 删除角色
// @Summary 删除角色
// @Description 软删除角色，同时清理菜单/部门/岗位/用户关联与 Casbin 策略
// @ID deleteRole
// @Tags System
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id} [delete]
func (h *RoleHandler) Delete(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	roleSvc := system.NewRoleService()
	if err := roleSvc.Delete(c.Request.Context(), id); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// GetMenus 获取角色已授权的菜单
// @Summary 获取角色已授权的菜单
// @ID getRoleMenus
// @Tags System
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.RIDList
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/menus [get]
func (h *RoleHandler) GetMenus(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	roleSvc := system.NewRoleService()
	menuIDs, err := roleSvc.GetMenus(c.Request.Context(), id)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, menuIDs)
}

// AssignMenus 分配角色菜单
// @Summary 分配角色菜单
// @Description 覆盖式授权，传空数组表示清空；保存后同步重建 Casbin p 策略
// @ID assignRoleMenus
// @Tags System
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param body body request.AssignIDsReq true "菜单ID列表"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/menus [put]
func (h *RoleHandler) AssignMenus(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	var req request.AssignIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	roleSvc := system.NewRoleService()
	if err := roleSvc.AssignMenus(c.Request.Context(), id, req.IDs); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// GetDepts 获取角色关联的部门
// @Summary 获取角色关联的部门
// @ID getRoleDepts
// @Tags System
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.RIDList
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/depts [get]
func (h *RoleHandler) GetDepts(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	roleSvc := system.NewRoleService()
	deptIDs, err := roleSvc.GetDepts(c.Request.Context(), id)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, deptIDs)
}

// AssignDepts 分配角色部门
// @Summary 分配角色部门
// @Description 覆盖式关联，传空数组表示清空；保存后同步重建 Casbin g2 策略
// @ID assignRoleDepts
// @Tags System
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param body body request.AssignIDsReq true "部门ID列表"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/depts [put]
func (h *RoleHandler) AssignDepts(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	var req request.AssignIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	roleSvc := system.NewRoleService()
	if err := roleSvc.AssignDepts(c.Request.Context(), id, req.IDs); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// GetPosts 获取角色关联的岗位
// @Summary 获取角色关联的岗位
// @ID getRolePosts
// @Tags System
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.RIDList
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/posts [get]
func (h *RoleHandler) GetPosts(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	roleSvc := system.NewRoleService()
	postIDs, err := roleSvc.GetPosts(c.Request.Context(), id)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, postIDs)
}

// AssignPosts 分配角色岗位
// @Summary 分配角色岗位
// @Description 覆盖式关联，传空数组表示清空；保存后同步重建 Casbin g3 策略
// @ID assignRolePosts
// @Tags System
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param body body request.AssignIDsReq true "岗位ID列表"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/posts [put]
func (h *RoleHandler) AssignPosts(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	var req request.AssignIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	roleSvc := system.NewRoleService()
	if err := roleSvc.AssignPosts(c.Request.Context(), id, req.IDs); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// GetUsers 获取角色关联的用户
// @Summary 获取角色关联的用户
// @ID getRoleUsers
// @Tags System
// @Produce json
// @Param id path int true "角色ID"
// @Success 200 {object} response.RIDList
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/users [get]
func (h *RoleHandler) GetUsers(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	roleSvc := system.NewRoleService()
	userIDs, err := roleSvc.GetUsers(c.Request.Context(), id)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, userIDs)
}

// AssignUsers 分配角色用户
// @Summary 分配角色用户
// @Description 覆盖式关联，传空数组表示清空；保存后同步重建 Casbin g 策略
// @ID assignRoleUsers
// @Tags System
// @Accept json
// @Produce json
// @Param id path int true "角色ID"
// @Param body body request.AssignIDsReq true "用户ID列表"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/system/roles/{id}/users [put]
func (h *RoleHandler) AssignUsers(c *gin.Context) {
	id, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	var req request.AssignIDsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	roleSvc := system.NewRoleService()
	if err := roleSvc.AssignUsers(c.Request.Context(), id, req.IDs); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}
