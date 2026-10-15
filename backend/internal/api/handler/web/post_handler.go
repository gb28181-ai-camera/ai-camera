package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// PostHandler 岗位管理。
type PostHandler struct{}

// NewPostHandler 构造岗位处理器。
func NewPostHandler() *PostHandler { return &PostHandler{} }

// Page 岗位分页列表
// @Summary 岗位分页列表
// @ID pagePosts
// @Tags System
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页条数" default(20)
// @Param keyword query string false "关键词(名称/编码)"
// @Param active query bool false "是否启用"
// @Success 200 {object} response.RPagePost
// @Router /web/system/posts [get]
func (h *PostHandler) Page(c *gin.Context) {
	var req request.PagePostReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	resp, err := system.NewPostService().Page(c.Request.Context(), &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, resp)
}

// Options 岗位下拉选项
// @Summary 岗位下拉选项
// @ID postOptions
// @Tags System
// @Success 200 {object} response.ROptionList
// @Router /web/system/posts/options [get]
func (h *PostHandler) Options(c *gin.Context) {
	list, err := system.NewPostService().Options(c.Request.Context())
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, list)
}

// Get 岗位详情
// @Summary 岗位详情
// @Description 返回岗位信息及其关联的角色 ID 列表 role_ids。
// @ID getPost
// @Tags System
// @Param id path int true "岗位ID"
// @Success 200 {object} response.RPost
// @Router /web/system/posts/{id} [get]
func (h *PostHandler) Get(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	p, err := system.NewPostService().Get(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "岗位不存在")
		return
	}
	utils.Success(c, p)
}

// Create 新增岗位
// @Summary 新增岗位
// @ID createPost
// @Tags System
// @Param body body request.CreatePostReq true "岗位信息"
// @Success 201 {object} response.RPost
// @Router /web/system/posts [post]
func (h *PostHandler) Create(c *gin.Context) {
	var req request.CreatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	p, err := system.NewPostService().Create(c.Request.Context(), &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Created(c, p)
}

// Update 更新岗位
// @Summary 更新岗位
// @Description RoleIDs 为 null 表示不调整岗位角色，传空数组表示清空角色。
// @ID updatePost
// @Tags System
// @Param id path int true "岗位ID"
// @Param body body request.UpdatePostReq true "岗位信息"
// @Success 200 {object} response.RPost
// @Router /web/system/posts/{id} [put]
func (h *PostHandler) Update(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}
	var req request.UpdatePostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	svc := system.NewPostService()
	if err := svc.Update(c.Request.Context(), id, &req); err != nil {
		systemRespondErr(c, err, "岗位不存在")
		return
	}

	p, err := svc.Get(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "岗位不存在")
		return
	}
	utils.Success(c, p)
}

// Delete 删除岗位
// @Summary 删除岗位
// @Description 存在关联用户时拒绝删除；删除后清理岗位-角色关联。
// @ID deletePost
// @Tags System
// @Param id path int true "岗位ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/posts/{id} [delete]
func (h *PostHandler) Delete(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewPostService().Delete(c.Request.Context(), id); err != nil {
		systemRespondErr(c, err, "岗位不存在")
		return
	}
	utils.Success(c, nil)
}
