package handler

import (
	"net/http"

	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// ════════════════════════════════════════════════════════════════
// 推流配置批量导入（CSV / TSV）
//
// 同步返回逐行结果，不建任务表；文件大小与行数均有限制。
// ════════════════════════════════════════════════════════════════

// streamImportMaxUpload 上传文件大小上限（与 service 的限制保持一致）。
const streamImportMaxUpload = 4 << 20

// GbStreamImportHandler 推流导入处理器。
type GbStreamImportHandler struct {
	svc *gbsvc.GbStreamImportService
}

// NewGbStreamImportHandler 构造推流导入处理器。
func NewGbStreamImportHandler() *GbStreamImportHandler {
	return &GbStreamImportHandler{svc: gbsvc.NewGbStreamImportService()}
}

// ImportStreamPushes 导入推流配置
//
// @Summary      推流配置批量导入
// @Description  支持 CSV（逗号）与 TSV（制表符），表头可选。列：name/名称、app/应用名、stream/流ID、media_server_id/媒体节点、push_url/推流地址、gb_channel_id/通道编码、remark/备注。无表头时按 name,app,stream,media_server_id,remark 顺序。单次最多 1000 行、4MB；不支持 xlsx
// @Tags         GbStreamImport
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "CSV/TSV 文件"
// @Success      200  {object}  response.RGbStreamImportResult
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/gb/stream-pushes/import [post]
// @ID           importGbStreamPushes
func (h *GbStreamImportHandler) ImportStreamPushes(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, streamImportMaxUpload+1024)

	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请上传导入文件（表单字段名 file）: "+err.Error())
		return
	}
	if fh.Size > streamImportMaxUpload {
		utils.BadRequest(c, "导入文件不能超过 4MB")
		return
	}
	f, err := fh.Open()
	if err != nil {
		utils.BadRequest(c, "读取上传文件失败: "+err.Error())
		return
	}
	defer f.Close()

	resp, err := h.svc.Import(c.Request.Context(), f)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}
