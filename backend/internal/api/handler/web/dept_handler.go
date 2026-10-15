package handler

import (
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// DeptHandler 部门（组织）管理。
type DeptHandler struct{}

// NewDeptHandler 构造部门处理器。
func NewDeptHandler() *DeptHandler { return &DeptHandler{} }

// Tree 部门树列表
// @Summary 部门树列表
// @Description 返回全部部门的树结构，children 按 sequence 升序。
// @ID treeDepts
// @Tags System
// @Success 200 {object} response.RDeptTree
// @Router /web/system/depts [get]
func (h *DeptHandler) Tree(c *gin.Context) {
	list, err := system.NewDeptService().Tree(c.Request.Context())
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, list)
}

// Options 部门下拉选项
// @Summary 部门下拉选项
// @ID deptOptions
// @Tags System
// @Success 200 {object} response.ROptionList
// @Router /web/system/depts/options [get]
func (h *DeptHandler) Options(c *gin.Context) {
	list, err := system.NewDeptService().Options(c.Request.Context())
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Success(c, list)
}

// Get 部门详情
// @Summary 部门详情
// @ID getDept
// @Tags System
// @Param id path int true "部门ID"
// @Success 200 {object} response.RDept
// @Router /web/system/depts/{id} [get]
func (h *DeptHandler) Get(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	d, err := system.NewDeptService().Get(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "部门不存在")
		return
	}
	utils.Success(c, d)
}

// Create 新增部门
// @Summary 新增部门
// @Description level 与 path 由后端依据上级部门自动维护。
// @ID createDept
// @Tags System
// @Param body body request.CreateDeptReq true "部门信息"
// @Success 201 {object} response.RDept
// @Router /web/system/depts [post]
func (h *DeptHandler) Create(c *gin.Context) {
	var req request.CreateDeptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	d, err := system.NewDeptService().Create(c.Request.Context(), &req)
	if err != nil {
		systemRespondErr(c, err, "")
		return
	}
	utils.Created(c, d)
}

// Update 更新部门
// @Summary 更新部门
// @Description 变更上级部门或部门编码时，级联重算本部门与全部下级的 level / path。
// @ID updateDept
// @Tags System
// @Param id path int true "部门ID"
// @Param body body request.UpdateDeptReq true "部门信息"
// @Success 200 {object} response.RDept
// @Router /web/system/depts/{id} [put]
func (h *DeptHandler) Update(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}
	var req request.UpdateDeptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	svc := system.NewDeptService()
	if err := svc.Update(c.Request.Context(), id, &req); err != nil {
		systemRespondErr(c, err, "部门不存在")
		return
	}

	d, err := svc.Get(c.Request.Context(), id)
	if err != nil {
		systemRespondErr(c, err, "部门不存在")
		return
	}
	utils.Success(c, d)
}

// Delete 删除部门
// @Summary 删除部门
// @Description 存在下级部门或部门下存在用户时拒绝删除。
// @ID deleteDept
// @Tags System
// @Param id path int true "部门ID"
// @Success 200 {object} response.BaseResponse
// @Router /web/system/depts/{id} [delete]
func (h *DeptHandler) Delete(c *gin.Context) {
	id, ok := systemPathID(c, "id")
	if !ok {
		return
	}

	if err := system.NewDeptService().Delete(c.Request.Context(), id); err != nil {
		systemRespondErr(c, err, "部门不存在")
		return
	}
	utils.Success(c, nil)
}
