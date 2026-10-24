package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 云端录像（ZLM 落盘 MP4 的索引）
//
// 下载支持两种形态（设计 §14.5 的「单文件 vs 打包」）：
//   - 单文件：返回 JSON 直链，packaging=false；
//   - 多选：同一路由上通过 ?ids=1,2,3 返回 application/zip 流，
//     由 Go 标准库 archive/zip 在服务端流式打包，不依赖外部 Assist 服务。
// ════════════════════════════════════════════════════════════════

// GbCloudRecordHandler 云端录像处理器。
type GbCloudRecordHandler struct {
	svc *gbsvc.GbCloudRecordService
}

// NewGbCloudRecordHandler 构造云端录像处理器。
func NewGbCloudRecordHandler() *GbCloudRecordHandler {
	return &GbCloudRecordHandler{svc: gbsvc.NewGbCloudRecordService()}
}

// ListDates 有录像的日期列表
//
// @Summary      有录像的日期列表
// @Description  返回指定范围内存在录像的日期（YYYY-MM-DD），用于日历控件
// @Tags         GbCloudRecord
// @Accept       json
// @Produce      json
// @Param        device_id        query     int     false  "设备ID"
// @Param        channel_id       query     int     false  "通道ID"
// @Param        media_server_id  query     string  false  "媒体节点"
// @Param        start_dt         query     string  false  "开始时间"
// @Param        end_dt           query     string  false  "结束时间"
// @Success      200  {object}  response.RGbCloudRecordDates
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/dates [get]
// @ID           listGbCloudRecordDates
func (h *GbCloudRecordHandler) ListDates(c *gin.Context) {
	var req request.GbCloudRecordDateReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	dates, err := h.svc.ListDates(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, dates)
}

// Page 云端录像分页
//
// @Summary      云端录像分页
// @Description  支持关键字、app/stream、设备/通道、节点、收藏与时间范围筛选；返回关联设备/通道名称
// @Tags         GbCloudRecord
// @Accept       json
// @Produce      json
// @Param        page             query     int     false  "页码"     default(1)
// @Param        page_size        query     int     false  "每页条数"  default(20)
// @Param        keyword          query     string  false  "关键字(文件名/流名/Call-ID)"
// @Param        app              query     string  false  "ZLM app"
// @Param        stream           query     string  false  "ZLM stream"
// @Param        device_id        query     int     false  "设备ID"
// @Param        channel_id       query     int     false  "通道ID"
// @Param        media_server_id  query     string  false  "媒体节点"
// @Param        collect          query     bool    false  "是否收藏"
// @Param        start_dt         query     string  false  "开始时间"
// @Param        end_dt           query     string  false  "结束时间"
// @Success      200  {object}  response.RPageGbCloudRecord
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records [get]
// @ID           pageGbCloudRecords
func (h *GbCloudRecordHandler) Page(c *gin.Context) {
	var req request.PageGbCloudRecordReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	resp, err := h.svc.Page(c.Request.Context(), &req, utils.NewDataScope(c))
	if err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, resp)
}

// Delete 批量删除录像索引
//
// @Summary      批量删除录像索引
// @Description  只删除数据库索引，不删除磁盘文件（文件由 ZLM 保留策略清理）
// @Tags         GbCloudRecord
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbCloudRecordBatchReq  true  "录像ID集合"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records [delete]
// @ID           deleteGbCloudRecords
func (h *GbCloudRecordHandler) Delete(c *gin.Context) {
	var req request.GbCloudRecordBatchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Delete(c.Request.Context(), req.IDs); err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// Collect 批量收藏/取消收藏
//
// @Summary      批量收藏/取消收藏
// @Description  收藏的录像不会被自动清理
// @Tags         GbCloudRecord
// @Accept       json
// @Produce      json
// @Param        body  body      request.GbCloudRecordCollectReq  true  "收藏参数"
// @Success      200  {object}  response.BaseResponse
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/collect [put]
// @ID           collectGbCloudRecords
func (h *GbCloudRecordHandler) Collect(c *gin.Context) {
	var req request.GbCloudRecordCollectReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Collect(c.Request.Context(), req.IDs, req.Collect); err != nil {
		respondGbErr(c, err)
		return
	}
	utils.Success(c, nil)
}

// PlayPath 云端录像播放地址
//
// @Summary      云端录像播放地址
// @Description  按录像所在 app（默认 record）生成 WebRTC/FLV/HLS 地址
// @Tags         GbCloudRecord
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "录像ID"
// @Success      200  {object}  response.RGbCloudRecordPlay
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/{id}/play [get]
// @ID           getGbCloudRecordPlay
func (h *GbCloudRecordHandler) PlayPath(c *gin.Context) {
	id, ok := gbPathID(c, "录像")
	if !ok {
		return
	}
	resp, err := h.svc.PlayPath(c.Request.Context(), id)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbCloudRecordNotFound)
		return
	}
	utils.Success(c, resp)
}

// Download 云端录像下载
//
// @Summary      云端录像下载
// @Description  单文件返回 JSON 下载地址；传 ?ids=1,2,3（>=2 个）时返回 application/zip 流。单次最多 50 个，文件不存在会跳过并通过响应头 X-Skipped 返回（URL 编码）。
// @Tags         GbCloudRecord
// @Accept       json
// @Produce      json
// @Param        id   path      int     true   "录像ID（未传 ids 时下载该条）"
// @Param        ids  query     string  false  "多选录像ID，逗号分隔；传 2 个及以上时返回 zip"
// @Success      200  {object}  response.RGbCloudRecordDownload
// @Failure      400  {object}  response.BaseResponse
// @Failure      404  {object}  response.BaseResponse
// @Router       /web/gb/cloud-records/{id}/download [get]
// @ID           getGbCloudRecordDownload
func (h *GbCloudRecordHandler) Download(c *gin.Context) {
	id, ok := gbPathID(c, "录像")
	if !ok {
		return
	}

	ids, parseErr := parseDownloadIDs(c.Query("ids"))
	if parseErr != nil {
		utils.BadRequest(c, parseErr.Error())
		return
	}
	if len(ids) == 0 {
		ids = []int64{id}
	}

	// 多选：流式 zip。单条仍保持原有 JSON 直链行为，兼容现有前端。
	if len(ids) > 1 {
		h.downloadZip(c, ids)
		return
	}

	resp, err := h.svc.Download(c.Request.Context(), ids[0])
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbCloudRecordNotFound)
		return
	}
	utils.Success(c, resp)
}

// downloadZip 打包下载。
//
// 响应头必须在校验完所有文件之后、写正文之前设置：X-Skipped 要列出
// 预检失败的文件，而 HTTP 头一旦写出就不能再改。
func (h *GbCloudRecordHandler) downloadZip(c *gin.Context, ids []int64) {
	ctx := c.Request.Context()
	entries, skipped, err := h.svc.PrepareZip(ctx, ids)
	if err != nil {
		respondGbErr(c, err, gbsvc.ErrGbCloudRecordNotFound, gbsvc.ErrNoDownloadableFiles)
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition",
		fmt.Sprintf(`attachment; filename="gb-cloud-records-%s.zip"`, time.Now().Format("20060102-150405")))
	if v := gbsvc.SkippedHeaderValue(skipped); v != "" {
		// 中文文件名按 URL 编码后放入头，前端 decodeURIComponent 还原。
		c.Header("X-Skipped", v)
	}
	c.Status(http.StatusOK)

	if _, werr := gbsvc.WriteZipStream(ctx, c.Writer, entries); werr != nil {
		// 响应头已发出，只能记录错误（客户端会收到一个不完整的 zip）。
		_ = c.Error(werr)
	}
}

// parseDownloadIDs 解析逗号分隔的 ids。
//
// 允许重复（服务层会去重）；空串表示调用方只传了路径参数。
func parseDownloadIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("ids 参数非法：%q 不是有效的录像ID", p)
		}
		out = append(out, v)
	}
	return out, nil
}
