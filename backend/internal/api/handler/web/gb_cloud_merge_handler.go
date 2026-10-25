package handler

import (
	"errors"
	"net/http"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 云端录像多段合并任务
//
// 创建后异步执行 ffmpeg concat；环境缺少 ffmpeg 时任务立即 failed。
// 单文件直接下载与多选 zip 打包是既有能力（gb_cloud_record_handler.go），
// 本文件只负责「合并成一个 MP4」。
// ════════════════════════════════════════════════════════════════

// GbCloudMergeHandler 合并任务处理器。
type GbCloudMergeHandler struct {
	svc *gbsvc.GbCloudMergeService
}

// NewGbCloudMergeHandler 构造合并任务处理器。
func NewGbCloudMergeHandler() *GbCloudMergeHandler {
	return &GbCloudMergeHandler{svc: gbsvc.NewGbCloudMergeService()}
}

// CreateMergeTask 创建合并任务
//
// @Summary      创建云端录像合并任务
// @Description  选择至少两段云端录像，异步用 ffmpeg concat（-c copy）合并为一个 MP4。未安装 ffmpeg 时任务立即落为 failed 并返回明确原因
// @Tags         GbCloudMerge
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbMergeTaskCreateReq  true  "录像ID与任务名"
// @Success      200  {object}  response.RGbMergeTask
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/merge-tasks [post]
// @ID           createGbCloudMergeTask
func (h *GbCloudMergeHandler) CreateMergeTask(c *gin.Context) {
	var req request.GbMergeTaskCreateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	task, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		respondMergeErr(c, err)
		return
	}
	item := gbsvc.ToGbMergeTaskItem(*task)
	utils.Success(c, item)
}

// PageMergeTasks 合并任务分页
//
// @Summary      合并任务分页
// @Description  支持状态与关键字（任务名/文件名）筛选
// @Tags         GbCloudMerge
// @Accept       json
// @Produce      json
// @Param        page      query     int     false  "页码"     default(1)
// @Param        page_size query     int     false  "每页条数"  default(20)
// @Param        status    query     string  false  "状态 pending/running/success/failed"
// @Param        keyword   query     string  false  "关键字"
// @Success      200  {object}  response.RPageGbMergeTask
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/merge-tasks [get]
// @ID           pageGbCloudMergeTasks
func (h *GbCloudMergeHandler) PageMergeTasks(c *gin.Context) {
	var req request.PageGbMergeTaskReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondMergeErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// GetMergeTask 合并任务详情
//
// @Summary      合并任务详情
// @Description  返回状态、失败原因、输出文件信息
// @Tags         GbCloudMerge
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "任务ID"
// @Success      200  {object}  response.RGbMergeTask
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/merge-tasks/{id} [get]
// @ID           getGbCloudMergeTask
func (h *GbCloudMergeHandler) GetMergeTask(c *gin.Context) {
	id, ok := gbPathID(c, "合并任务")
	if !ok {
		return
	}
	item, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		respondMergeErr(c, err)
		return
	}
	utils.Success(c, item)
}

// DownloadMergeTask 下载合并文件
//
// @Summary      下载合并后的录像
// @Description  任务未完成返回 409；文件已被清理返回 404
// @Tags         GbCloudMerge
// @Produce      application/octet-stream
// @Param        id  path      int  true  "任务ID"
// @Success      200  {file}    file  "MP4 文件"
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Failure      409  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/merge-tasks/{id}/download [get]
// @ID           downloadGbCloudMergeTask
func (h *GbCloudMergeHandler) DownloadMergeTask(c *gin.Context) {
	id, ok := gbPathID(c, "合并任务")
	if !ok {
		return
	}
	path, name, err := h.svc.DownloadPath(c.Request.Context(), id)
	if err != nil {
		respondMergeErr(c, err)
		return
	}
	c.FileAttachment(path, name)
}

// respondMergeErr 合并任务错误映射。
func respondMergeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gbsvc.ErrGbMergeTaskNotFound):
		utils.NotFound(c, err.Error())
	case errors.Is(err, gbsvc.ErrGbMergeTaskNotReady), errors.Is(err, gbsvc.ErrGbMergeTaskRunning):
		c.JSON(http.StatusConflict, utils.Response{Code: http.StatusConflict, Message: err.Error()})
	case errors.Is(err, gbsvc.ErrGbMergeRecordNotFound):
		utils.BadRequest(c, err.Error())
	default:
		utils.BadRequest(c, err.Error())
	}
}
