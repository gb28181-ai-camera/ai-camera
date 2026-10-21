package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 国标业务分组
// ════════════════════════════════════════════════════════════════

// GbGroupHandler 业务分组处理器。
type GbGroupHandler struct {
	svc *gbsvc.GbGroupService
}

// NewGbGroupHandler 构造业务分组处理器。
func NewGbGroupHandler() *GbGroupHandler {
	return &GbGroupHandler{svc: gbsvc.NewGbGroupService()}
}

// Tree 业务分组树
//
// @Summary      业务分组树
// @Description  按 parent_id 组装树；父节点因数据权限不可见时，子节点提升为根，避免整棵子树消失
// @Tags         GbGroup
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RGbGroupTree
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/groups/tree [get]
// @ID           gbGroupTree
func (h *GbGroupHandler) Tree(c *gin.Context) {
	items, err := h.svc.Tree(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, items)
}

// List 业务分组列表
//
// @Summary      业务分组列表
// @Description  简单列表（不分页），按 sequence、id 排序，供下拉与树表使用
// @Tags         GbGroup
// @Accept       json
// @Produce      json
// @Success      200  {object}  response.RGbGroupList
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/groups [get]
// @ID           listGbGroups
func (h *GbGroupHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, items)
}

// Create 新增业务分组
//
// @Summary      新增业务分组
// @Description  名称必填；parent_id 为空表示根分组
// @Tags         GbGroup
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateGbGroupReq  true  "分组信息"
// @Success      200  {object}  response.RGbGroup
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/groups [post]
// @ID           createGbGroup
func (h *GbGroupHandler) Create(c *gin.Context) {
	var req request.CreateGbGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, m)
}

// Update 编辑业务分组
//
// @Summary      编辑业务分组
// @Description  增量更新；parent_id 传 0 表示提升为根；会校验父子成环
// @Tags         GbGroup
// @Accept       json
// @Produce      json
// @Param        id    path      int                      true  "分组ID"
// @Param        body  body      request.UpdateGbGroupReq  true  "分组信息"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/groups/{id} [put]
// @ID           updateGbGroup
func (h *GbGroupHandler) Update(c *gin.Context) {
	id, ok := gbPathID(c, "分组")
	if !ok {
		return
	}
	var req request.UpdateGbGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Update(c.Request.Context(), id, &req); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbGroupNotFound)
		return
	}
	utils.Success(c, nil)
}

// Delete 删除业务分组
//
// @Summary      删除业务分组
// @Description  存在子分组或仍被通道引用时拒绝删除
// @Tags         GbGroup
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "分组ID"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/groups/{id} [delete]
// @ID           deleteGbGroup
func (h *GbGroupHandler) Delete(c *gin.Context) {
	id, ok := gbPathID(c, "分组")
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		respondGbErr(c, err, gbsvc.ErrGbGroupNotFound)
		return
	}
	utils.Success(c, nil)
}
