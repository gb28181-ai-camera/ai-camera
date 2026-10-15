package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

// 上传体积上限。
const (
	maxMediaFileSize = 50 << 20 // 50MB：图片与录像
	maxAudioFileSize = 20 << 20 // 20MB：语音播报音频
)

// 允许的扩展名白名单（小写，含点）。
var (
	mediaFileExts = map[string]struct{}{
		".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {},
		".webp": {}, ".bmp": {}, ".mp4": {}, ".avi": {}, ".mov": {},
	}
	audioFileExts = map[string]struct{}{
		".mp3": {}, ".wav": {}, ".ogg": {}, ".m4a": {}, ".aac": {},
	}
)

// UploadHandler 文件上传处理器（无状态）。
//
// 存储约定：
//   - 物理路径：<cfg.Media.UploadDir>/<yyyyMM>/<uuid><ext>
//   - 访问地址：<cfg.Media.BaseURL>/<yyyyMM>/<uuid><ext>
//
// 静态目录 `/uploads` 由 router 统一注册并指向 cfg.Media.UploadDir，
// 本处理器只负责落盘与返回可访问 URL，不注册路由。
type UploadHandler struct{}

// NewUploadHandler 构造上传处理器。
func NewUploadHandler() *UploadHandler { return &UploadHandler{} }

// Upload 上传图片/录像文件。
// @Summary 上传图片或录像
// @Description 通用文件上传：图片（jpg/jpeg/png/gif/webp/bmp）与录像（mp4/avi/mov），单文件上限 50MB；文件按 uploads/yyyyMM 分目录存储，返回可直接访问的 URL
// @Tags Common
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "文件（字段名 file）"
// @Success 200 {object} response.UploadResp "上传成功，data 含 url/file_name/file_size"
// @Failure 400 {object} response.BaseResponse "未选择文件、类型不支持或超出大小限制"
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /web/upload [post]
func (h *UploadHandler) Upload(c *gin.Context) {
	h.save(c, mediaFileExts, maxMediaFileSize, "文件")
}

// UploadAudio 上传语音播报音频。
// @Summary 上传语音播报音频
// @Description 音频上传：mp3/wav/ogg/m4a/aac，单文件上限 20MB；文件按 uploads/yyyyMM 分目录存储，返回可直接访问的 URL
// @Tags Common
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "音频文件（字段名 file）"
// @Success 200 {object} response.UploadResp "上传成功，data 含 url/file_name/file_size"
// @Failure 400 {object} response.BaseResponse "未选择文件、类型不支持或超出大小限制"
// @Failure 401 {object} response.BaseResponse
// @Failure 500 {object} response.BaseResponse
// @Router /web/upload/audio [post]
func (h *UploadHandler) UploadAudio(c *gin.Context) {
	h.save(c, audioFileExts, maxAudioFileSize, "音频文件")
}

// save 校验并保存 multipart 字段 file。
//
// 校验顺序：体积（先看 Content-Length 再核对解析后的文件大小）→ 扩展名白名单 → 落盘。
func (h *UploadHandler) save(c *gin.Context, allowed map[string]struct{}, maxSize int64, label string) {
	cfg := bootstrap.Config()
	if cfg == nil || cfg.Media.UploadDir == "" {
		utils.ServerError(c, "上传目录未配置")
		return
	}

	// 提前按请求体大小拒绝超大请求，避免整段 multipart 被解析落盘
	if c.Request.ContentLength > maxSize+4096 {
		utils.BadRequest(c, fmt.Sprintf("%s大小不能超过 %dMB", label, maxSize>>20))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请选择要上传的文件")
		return
	}
	if fileHeader.Size <= 0 {
		utils.BadRequest(c, label+"内容为空")
		return
	}
	if fileHeader.Size > maxSize {
		utils.BadRequest(c, fmt.Sprintf("%s大小不能超过 %dMB", label, maxSize>>20))
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if _, ok := allowed[ext]; !ok {
		utils.BadRequest(c, fmt.Sprintf("不支持的文件类型: %s", ext))
		return
	}

	name, err := randomFileName(ext)
	if err != nil {
		utils.ServerError(c, "生成文件名失败")
		return
	}

	// 相对路径用于拼接访问 URL，物理目录挂在 cfg.Media.UploadDir 之下
	subDir := time.Now().Format("200601")
	absDir := filepath.Join(cfg.Media.UploadDir, subDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		utils.ServerError(c, "创建上传目录失败")
		return
	}

	if err := c.SaveUploadedFile(fileHeader, filepath.Join(absDir, name)); err != nil {
		utils.ServerError(c, "保存文件失败")
		return
	}

	relPath := filepath.ToSlash(filepath.Join(subDir, name))
	url := strings.TrimRight(cfg.Media.BaseURL, "/") + "/" + relPath

	utils.Success(c, response.UploadResp{
		URL:      url,
		FileName: fileHeader.Filename,
		FileSize: fileHeader.Size,
	})
}

// randomFileName 生成 <uuid><ext> 形式的文件名。
// 使用 crypto/rand 生成 16 字节随机数并格式化为 UUID v4 字形，避免引入新依赖。
func randomFileName(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
	buf[8] = (buf[8] & 0x3f) | 0x80 // variant RFC 4122

	h := hex.EncodeToString(buf)
	return fmt.Sprintf("%s-%s-%s-%s-%s%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32], ext), nil
}
