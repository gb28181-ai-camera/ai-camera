package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ProfileHandler 个人中心接口（个人信息 / 密码 / 菜单 / API Key）。
type ProfileHandler struct{}

// NewProfileHandler 构造个人中心处理器。
func NewProfileHandler() *ProfileHandler { return &ProfileHandler{} }

// GetProfile 获取当前用户信息
// @Summary 获取当前用户信息
// @Description 返回当前登录用户的资料（含部门/岗位/角色）
// @ID getProfile
// @Tags Profile
// @Produce json
// @Success 200 {object} response.RUser
// @Failure 401 {object} response.BaseResponse
// @Router /web/profile [get]
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	profileSvc := system.NewProfileService()
	user, err := profileSvc.Get(c.Request.Context(), userID)
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	utils.Success(c, user)
}

// UpdateProfile 更新当前用户信息
// @Summary 更新当前用户信息
// @Description 仅可更新姓名、邮箱、手机号、头像与备注
// @ID updateProfile
// @Tags Profile
// @Accept json
// @Produce json
// @Param body body request.UpdateProfileReq true "个人信息"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/profile [put]
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	var req request.UpdateProfileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID := c.GetInt64("user_id")
	profileSvc := system.NewProfileService()
	if err := profileSvc.Update(c.Request.Context(), userID, &req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}

// ChangePassword 修改当前用户密码
// @Summary 修改当前用户密码
// @Description 需提供原密码，新密码至少 6 位
// @ID changePassword
// @Tags Profile
// @Accept json
// @Produce json
// @Param body body request.ChangePasswordReq true "密码信息"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/profile/password [put]
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	var req request.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID := c.GetInt64("user_id")
	profileSvc := system.NewProfileService()
	if err := profileSvc.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, "密码修改成功")
}

// GetMenus 获取当前用户菜单树
// @Summary 获取当前用户菜单树
// @Description 按当前用户角色（直接/岗位/部门）返回 PC 端菜单树，含按钮节点的权限标识
// @ID getProfileMenus
// @Tags Profile
// @Produce json
// @Success 200 {object} response.RMenuList
// @Failure 401 {object} response.BaseResponse
// @Router /web/profile/menus [get]
func (h *ProfileHandler) GetMenus(c *gin.Context) {
	username := c.GetString("user_name")

	profileSvc := system.NewProfileService()
	menus, err := profileSvc.Menus(c.Request.Context(), username)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, menus)
}

// ListApiKeys 获取当前用户 API Key 列表
// @Summary 获取当前用户 API Key 列表
// @Description 只返回密钥前缀与名称，完整密钥仅在生成时返回一次
// @ID listApiKeys
// @Tags Profile
// @Produce json
// @Success 200 {object} response.RApiKeyList
// @Failure 401 {object} response.BaseResponse
// @Router /web/profile/api-keys [get]
func (h *ProfileHandler) ListApiKeys(c *gin.Context) {
	userID := c.GetInt64("user_id")

	profileSvc := system.NewProfileService()
	keys, err := profileSvc.ListApiKeys(c.Request.Context(), userID)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, keys)
}

// GenerateApiKey 生成 API Key
// @Summary 生成 API Key
// @Description 生成 64 位随机密钥，服务端仅保存 SHA256 摘要；完整密钥仅在本次响应返回
// @ID generateApiKey
// @Tags Profile
// @Accept json
// @Produce json
// @Param body body request.GenerateApiKeyReq true "密钥名称"
// @Success 200 {object} response.RGenerateApiKey
// @Failure 400 {object} response.BaseResponse
// @Router /web/profile/api-keys [post]
func (h *ProfileHandler) GenerateApiKey(c *gin.Context) {
	var req request.GenerateApiKeyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	userID := c.GetInt64("user_id")
	profileSvc := system.NewProfileService()
	resp, err := profileSvc.GenerateApiKey(c.Request.Context(), userID, req.Name)
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}

	utils.Success(c, resp)
}

// DeleteApiKey 删除 API Key
// @Summary 删除 API Key
// @Description 仅可删除当前用户自己创建的密钥
// @ID deleteApiKey
// @Tags Profile
// @Produce json
// @Param id path int true "密钥ID"
// @Success 200 {object} response.BaseResponse
// @Failure 400 {object} response.BaseResponse
// @Router /web/profile/api-keys/{id} [delete]
func (h *ProfileHandler) DeleteApiKey(c *gin.Context) {
	keyID, ok := parseSystemPathID(c)
	if !ok {
		return
	}

	userID := c.GetInt64("user_id")
	profileSvc := system.NewProfileService()
	if err := profileSvc.DeleteApiKey(c.Request.Context(), userID, keyID); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.Success(c, nil)
}
