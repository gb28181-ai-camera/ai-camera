package handler

import (
	"errors"
	"strconv"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// MenuHandler 菜单与菜单接口管理。
type MenuHandler struct{}

// NewMenuHandler 构造菜单处理器。
func NewMenuHandler() *MenuHandler { return &MenuHandler{} }

// ────────────────────────────────────────────────────────────────
// 系统管理 Handler 共用工具（本文件为 package handler 内系统模块的公共实现）
// ────────────────────────────────────────────────────────────────

// systemRespondErr 统一错误映射：
// 记录不存在 → 404，业务校验错误（BizError）→ 400，其余 → 500。
func systemRespondErr(c *gin.Context, err error, notFoundMsg string) {
	if err == nil {
		return
	}
	if system.IsNotFound(err) {
		if notFoundMsg == "" {
			notFoundMsg = "记录不存在"
		}
		utils.NotFound(c, notFoundMsg)
		return
	}
	var be *system.BizError
	if errors.As(err, &be) {
		utils.BadRequest(c, be.Msg)
		return
	}
	utils.ServerError(c, err.Error())
}

// systemPathID 解析路径参数为 int64，失败时已写入 400 响应并返回 false。
func systemPathID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		utils.BadRequest(c, "无效的ID参数")
		return 0, false
	}
	return id, true
}

// ────────────────────────────────────────────────────────────────
// 菜单
// ────────────────────────────────────────────────────────────────

// Tree 菜单树列表
// @Summary 菜单树列表
// @Description 返回指定平台的菜单树，platform 缺省为 pc；children 按 sequence 升序。
// @Description 默认仅返回启用菜单（角色授权树行为）；管理页传 include_inactive=true&platform=all 可看到停用菜单与移动端菜单。
// @ID treeMenus
// @Tags System
// @Param platform query string false "平台 pc/mobile/all" default(pc)
// @Param include_inactive query bool false "是否包含停用菜单" default(false)
// @Success 200 {object} response.RMenuList
// @Router /web/system/menus [get]
func (h *MenuHandler) Tree(c *gin.Context) {
	platform := c.DefaultQuery("platform", "pc")
	includeInactive, _ := strconv.ParseBool(c.DefaultQuery("include_inactive", "false"))

	list, err := system.NewMenuService().TreeWithOptions(c.Request.Context(), platform, includeInactive)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, list)
}

// Get 菜单详情
// @Summary 菜单详情
// @ID getMenu
// @Tags System
// @Param id path int true "菜单ID"
// @Success 200 {object} response.RMenu
// @Router /web/system/menus/{id} [get]
func (h *MenuHandler) Get(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	m, err := system.NewMenuService().Get(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}
	utils.Success(c, m)
}

// Create 新增菜单
// @Summary 新增菜单
// @ID createMenu
// @Tags System
// @Param body body request.CreateMenuReq true "菜单信息"
// @Success 201 {object} response.RMenu
// @Router /web/system/menus [post]
func (h *MenuHandler) Create(c *gin.Context) {
	var req request.CreateMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	m, err := system.NewMenuService().Create(c.Request.Context(), &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Created(c, m)
}

// Update 更新菜单
// @Summary 更新菜单
// @ID updateMenu
// @Tags System
// @Param id path int true "菜单ID"
// @Param body body request.UpdateMenuReq true "菜单信息"
// @Success 200 {object} response.RMenu
// @Router /web/system/menus/{id} [put]
func (h *MenuHandler) Update(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}
	var req request.UpdateMenuReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	svc := system.NewMenuService()
	if err := svc.Update(c.Request.Context(), id, &req); err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}

	m, err := svc.Get(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}
	utils.Success(c, m)
}

// Delete 删除菜单
// @Summary 删除菜单
// @Description 存在子菜单时拒绝删除；删除后清理绑定接口与角色关系并重建 Casbin 策略。
// @ID deleteMenu
// @Tags System
// @Param id path int true "菜单ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/menus/{id} [delete]
func (h *MenuHandler) Delete(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewMenuService().Delete(c.Request.Context(), id); err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}
	utils.Success(c, nil)
}

// GetRoles 菜单角色列表
// @Summary 菜单角色列表
// @Description 返回已授权该菜单的角色（id/code/name）。需要权限码 system:menu:role。
// @ID getMenuRoles
// @Tags System
// @Param id path int true "菜单ID"
// @Success 200 {object} response.RMenuRoleList
// @Router /web/system/menus/{id}/roles [get]
func (h *MenuHandler) GetRoles(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	roles, err := system.NewMenuService().GetRoles(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}

	list := make([]response.MenuRoleBrief, 0, len(roles))
	for _, r := range roles {
		list = append(list, response.MenuRoleBrief{ID: r.ID, Code: r.Code, Name: r.Name})
	}
	utils.Success(c, list)
}

// AssignRoles 分配菜单角色
// @Summary 分配菜单角色
// @Description 覆盖式写入 ac_role_menu_rel，并为受影响角色重建 Casbin p 策略。需要权限码 system:menu:role。
// @ID assignMenuRoles
// @Tags System
// @Param id path int true "菜单ID"
// @Param body body request.AssignMenuRolesReq true "角色ID列表（为空表示清空授权）"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/menus/{id}/roles [put]
func (h *MenuHandler) AssignRoles(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}
	var req request.AssignMenuRolesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	if err := system.NewMenuService().AssignRoles(c.Request.Context(), id, req.IDs); err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}
	utils.Success(c, nil)
}

// ────────────────────────────────────────────────────────────────
// 菜单接口
// ────────────────────────────────────────────────────────────────

// ListAPIs 菜单接口列表
// @Summary 菜单接口列表
// @ID listMenuApis
// @Tags System
// @Param id path int true "菜单ID"
// @Success 200 {object} response.RMenuAPIList
// @Router /web/system/menus/{id}/apis [get]
func (h *MenuHandler) ListAPIs(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	list, err := system.NewMenuService().ListAPIs(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}
	utils.Success(c, list)
}

// CreateAPI 新增菜单接口
// @Summary 新增菜单接口
// @Description 菜单接口是 Casbin p 策略的唯一来源；新增后自动为拥有该菜单的角色重建策略。
// @ID createMenuApi
// @Tags System
// @Param id path int true "菜单ID"
// @Param body body request.CreateMenuAPIReq true "接口信息"
// @Success 201 {object} response.RMenuAPI
// @Router /web/system/menus/{id}/apis [post]
func (h *MenuHandler) CreateAPI(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}
	var req request.CreateMenuAPIReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	api, err := system.NewMenuService().CreateAPI(c.Request.Context(), id, &req)
	if err != nil {
		systemRespondErr(c, err, "菜单不存在")
		return
	}
	utils.Created(c, api)
}

// DeleteAPI 删除菜单接口
// @Summary 删除菜单接口
// @Description 删除后自动为拥有该菜单的角色重建 Casbin 策略。
// @ID deleteMenuApi
// @Tags System
// @Param apiId path int true "接口ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/menus/apis/{apiId} [delete]
func (h *MenuHandler) DeleteAPI(c *gin.Context) {
	apiID, ok := systemPathID(c, "apiId")
	if !ok {
		return
	}

	// 路由不含菜单 ID（DELETE /web/system/menus/apis/:apiId），
	// 传 0 表示由服务按接口自身归属的菜单处理。
	var menuID int64
	if v := c.Param("id"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			menuID = n
		}
	}

	if err := system.NewMenuService().DeleteAPI(c.Request.Context(), menuID, apiID); err != nil {
		systemRespondErr(c, err, "接口不存在")
		return
	}
	utils.Success(c, nil)
}
