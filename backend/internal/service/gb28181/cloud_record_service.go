package gb28181

// 云端录像：ZLM 落盘 MP4 的索引查询、播放与下载。
//
// 下载能力分两种：
//   - 单文件：返回 ZLM 的直接下载地址（packaging=false）；
//   - 多选打包：服务端用标准库 archive/zip **流式**打包，不依赖外部
//     Assist 服务。接口仍是 GET /web/gb/cloud-records/:id/download，
//     通过 ?ids=1,2,3 触发，因此没有新增路由。
//
// 文件内容从媒体节点的 HTTP 文件服务读取（/record/{app}/{stream}/{file}），
// 平台不落盘、不缓存，避免在大文件上产生额外磁盘占用。

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ErrGbCloudRecordNotFound 云端录像索引不存在。
var ErrGbCloudRecordNotFound = errors.New("云端录像不存在")

// 批量打包的限制与错误。
var (
	// ErrTooManyZipFiles 一次打包的录像数量超限。
	ErrTooManyZipFiles = fmt.Errorf("一次最多打包 %d 个录像文件", MaxZipFiles)
	// ErrNoDownloadableFiles 全部录像都不可下载（文件不存在/缺少路径）。
	ErrNoDownloadableFiles = errors.New("没有可下载的录像文件")
)

// MaxZipFiles 是单次 zip 打包的文件数上限。
//
// 限制的目的不是性能，而是防止用户一次勾选上千条录像把平台与 ZLM 的
// 连接数打满；50 个文件对「按时间段补录」场景已经足够。
const MaxZipFiles = 50

// gbFileHTTPClient 用于访问 ZLM 的 HTTP 文件服务。
//
// 不设置整体 Timeout：打包大文件可能持续几分钟；只限制响应头等待时间，
// 避免节点黑洞时 HTTP 请求永久挂住。真正的取消由请求 ctx 控制。
var gbFileHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	},
}

// GbCloudRecordService 云端录像业务逻辑。
type GbCloudRecordService struct {
	repo *repository.GbCloudRecordRepo
}

// NewGbCloudRecordService 构造云端录像服务。
func NewGbCloudRecordService() *GbCloudRecordService {
	return &GbCloudRecordService{repo: repository.NewGbCloudRecordRepo()}
}

// ListDates 返回指定范围内「有录像的日期」列表。
//
// 契约签名为 (ctx, req)，但 §10.0 要求 /web/gb/* 列表施加数据权限；
// 用变参同时兼容两者：Handler 传 ds，内部调用可省略。
func (s *GbCloudRecordService) ListDates(ctx context.Context, req *request.GbCloudRecordDateReq, ds ...*utils.DataScope) ([]string, error) {
	if req == nil {
		req = &request.GbCloudRecordDateReq{}
	}

	q := bootstrap.DB().WithContext(ctx).
		Table("ac_gb_cloud_record").
		Where("deleted_at IS NULL").
		Where("start_time IS NOT NULL")
	if req.DeviceID != nil {
		q = q.Where("device_id = ?", *req.DeviceID)
	}
	if req.ChannelID != nil {
		q = q.Where("channel_id = ?", *req.ChannelID)
	}
	if v := strings.TrimSpace(req.MediaServerID); v != "" {
		q = q.Where("media_server_id = ?", v)
	}
	if v := strings.TrimSpace(req.StartDT); v != "" {
		q = q.Where("start_time >= ?", v)
	}
	if v := strings.TrimSpace(req.EndDT); v != "" {
		q = q.Where("start_time <= ?", v)
	}
	if len(ds) > 0 && ds[0] != nil {
		q = ds[0].ApplyToQuery(q, "ac_gb_cloud_record.dept_id")
	}

	// DISTINCT + ORDER BY 必须作用在同一个表达式上；用别名 d 保证 PostgreSQL
	// 的 SELECT DISTINCT 规则成立（按原始表达式排序会报「must appear in select list」）。
	type dateRow struct {
		D string `gorm:"column:d"`
	}
	var rows []dateRow
	err := q.Select("DISTINCT to_char(start_time, 'YYYY-MM-DD') AS d").
		Order("d DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	dates := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.D != "" {
			dates = append(dates, r.D)
		}
	}
	return dates, nil
}

// Page 云端录像分页。
func (s *GbCloudRecordService) Page(ctx context.Context, req *request.PageGbCloudRecordReq, ds *utils.DataScope) (*response.PageGbCloudRecordResp, error) {
	if req == nil {
		req = &request.PageGbCloudRecordReq{}
	}
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.GbCloudRecordItem, 0, len(items))
	for _, m := range items {
		out = append(out, toGbCloudRecordItem(m))
	}
	return &response.PageGbCloudRecordResp{
		Items:    out,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// toGbCloudRecordItem 模型 → 列表项，关联名称来自 Preload。
func toGbCloudRecordItem(m model.GbCloudRecord) response.GbCloudRecordItem {
	deviceName, channelName := "", ""
	if m.Device != nil {
		deviceName = m.Device.Name
	}
	if m.Channel != nil {
		channelName = m.Channel.Name
	}
	return response.GbCloudRecordItem{
		ID:            m.ID,
		App:           m.App,
		Stream:        m.Stream,
		CallID:        m.CallID,
		DeviceID:      m.DeviceID,
		DeviceName:    deviceName,
		ChannelID:     m.ChannelID,
		ChannelName:   channelName,
		StartTime:     m.StartTime,
		EndTime:       m.EndTime,
		MediaServerID: m.MediaServerID,
		FileName:      m.FileName,
		Folder:        m.Folder,
		FilePath:      m.FilePath,
		FileSize:      m.FileSize,
		TimeLen:       m.TimeLen,
		Collect:       m.Collect,
		Remark:        m.Remark,
		CreatedDT:     m.CreatedDT,
		UpdatedDT:     m.UpdatedDT,
	}
}

// Delete 批量软删除录像索引。
//
// 只删除索引，不删除磁盘文件：文件由 ZLM 的录像保留策略清理
// （ac_gb_media_server.record_day）。在这里删文件会让 ZLM 的索引与文件不一致。
func (s *GbCloudRecordService) Delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("请选择要删除的录像")
	}
	for _, id := range ids {
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// Collect 批量收藏 / 取消收藏。
//
// 收藏项不会被自动清理（ac_gb_cloud_record.collect 的语义），
// 因此这个接口只改标记，不动文件。
func (s *GbCloudRecordService) Collect(ctx context.Context, ids []int64, collect bool) error {
	if len(ids) == 0 {
		return errors.New("请选择要操作的录像")
	}
	for _, id := range ids {
		if err := s.repo.UpdateFields(ctx, id, map[string]interface{}{"collect": collect}); err != nil {
			return err
		}
	}
	return nil
}

// PlayPath 返回云端录像的播放地址。
//
// app 默认为 record：ZLM 录像落在 record/{app}/{stream}/ 下，
// 播放时 app 必须是录像所在的 app，而不是实时流的 rtp。
func (s *GbCloudRecordService) PlayPath(ctx context.Context, id int64) (*response.GbCloudRecordPlayResp, error) {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	app := firstNonEmpty(m.App, "record")
	if strings.TrimSpace(m.Stream) == "" {
		return nil, errors.New("录像缺少流名，无法生成播放地址")
	}
	ms, err := s.resolveServer(ctx, m.MediaServerID)
	if err != nil {
		return nil, err
	}
	urls := mediaserver.BuildPlayURLs(ms, app, m.Stream)
	return &response.GbCloudRecordPlayResp{
		ID:            m.ID,
		App:           app,
		Stream:        m.Stream,
		HTTPBase:      mediaHTTPBase(ms),
		WebRTC:        urls.WebRTC,
		FLV:           urls.FLV,
		HLS:           urls.HLS,
		WSFLV:         urls.WSFLV,
		RTSP:          urls.RTSP,
		RTMP:          urls.RTMP,
		MediaServerID: firstNonEmpty(m.MediaServerID, ms.ServerID),
		FileName:      m.FileName,
		FilePath:      m.FilePath,
		FileSize:      m.FileSize,
		TimeLen:       m.TimeLen,
		StartTime:     m.StartTime,
		EndTime:       m.EndTime,
	}, nil
}

// Download 返回单文件直接下载地址。
//
// ZLM 的 HTTP 文件服务把录像暴露在 /record/{app}/{stream}/{file}，
// 与播放共用一个 HTTP 端口。Packaging 恒为 false —— 单文件无需打包。
func (s *GbCloudRecordService) Download(ctx context.Context, id int64) (*response.GbCloudRecordDownloadResp, error) {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	u, fileName, err := s.downloadURL(ctx, m)
	if err != nil {
		return nil, err
	}
	return &response.GbCloudRecordDownloadResp{
		ID:          m.ID,
		URL:         u,
		FileName:    fileName,
		FileSize:    m.FileSize,
		ContentType: "video/mp4",
		Packaging:   false,
		Message:     "单文件直接下载",
	}, nil
}

// downloadURL 生成一条录像的可下载 URL 与文件名。
//
// 两条路径：
//   - FilePath 本身是 http(s) URL（对象存储/外部文件服务）→ 直接使用；
//   - 否则按 ZLM 的 /record/{app}/{stream}/{file} 约定拼接。
func (s *GbCloudRecordService) downloadURL(ctx context.Context, m *model.GbCloudRecord) (string, string, error) {
	if m == nil {
		return "", "", ErrGbCloudRecordNotFound
	}

	// 已经是完整 URL 时直接返回（部分 ZLM 部署把录像放在对象存储）。
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(m.FilePath)), "http") {
		name := firstNonEmpty(m.FileName, filepath.Base(strings.TrimSpace(m.FilePath)))
		return strings.TrimSpace(m.FilePath), name, nil
	}

	fileName := strings.TrimSpace(m.FileName)
	if fileName == "" && strings.TrimSpace(m.FilePath) != "" {
		fileName = filepath.Base(strings.ReplaceAll(m.FilePath, "\\", "/"))
	}
	if fileName == "" {
		return "", "", errors.New("录像缺少文件名，无法生成下载地址")
	}
	app := firstNonEmpty(m.App, "record")
	if strings.TrimSpace(m.Stream) == "" {
		return "", "", errors.New("录像缺少流名，无法生成下载地址")
	}
	ms, err := s.resolveServer(ctx, m.MediaServerID)
	if err != nil {
		return "", "", err
	}

	u := fmt.Sprintf("%s/record/%s/%s/%s",
		mediaHTTPBase(ms), urlPathEscape(app), urlPathEscape(m.Stream), urlPathEscape(fileName))
	return u, fileName, nil
}

// ── 多选打包下载（不依赖 Assist 服务）────────────────────────

// ZipEntry 是一个待写入 zip 的录像文件。
type ZipEntry struct {
	ID       int64
	FileName string
	URL      string
}

// PrepareZip 解析待打包的录像并做一次可用性预检。
//
// 为什么先预检（HEAD）再流式写 zip：HTTP 响应头一旦发出就无法回改，
// 「文件不存在」的列表必须在写正文之前确定，才能放进 X-Skipped 头。
// 预检失败的文件同时从 entries 里剔除，保证 zip 内不会出现空条目。
func (s *GbCloudRecordService) PrepareZip(ctx context.Context, ids []int64) (entries []ZipEntry, skipped []string, err error) {
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return nil, nil, errors.New("请选择要下载的录像")
	}
	if len(ids) > MaxZipFiles {
		return nil, nil, ErrTooManyZipFiles
	}

	used := map[string]int{}
	for _, id := range ids {
		m, gerr := s.getByID(ctx, id)
		if gerr != nil {
			skipped = append(skipped, fmt.Sprintf("id-%d", id))
			continue
		}
		u, fileName, derr := s.downloadURL(ctx, m)
		if derr != nil || strings.TrimSpace(u) == "" {
			skipped = append(skipped, firstNonEmpty(m.FileName, fmt.Sprintf("id-%d", id)))
			continue
		}
		if !zipFileExists(ctx, u) {
			skipped = append(skipped, firstNonEmpty(fileName, fmt.Sprintf("id-%d", id)))
			continue
		}

		name := zipEntryName(fileName, m.ID)
		if n, ok := used[name]; ok {
			// 不同目录下可能有同名文件：追加序号避免 zip 内覆盖。
			used[name] = n + 1
			name = fmt.Sprintf("%d_%s", n+2, name)
		} else {
			used[name] = 1
		}
		entries = append(entries, ZipEntry{ID: m.ID, FileName: name, URL: u})
	}

	if len(entries) == 0 {
		return nil, skipped, ErrNoDownloadableFiles
	}
	return entries, skipped, nil
}

// WriteZipStream 把 entries 流式写入 w。
//
// 使用 archive/zip 的流式模式：不构建临时文件、不把整段视频读进内存，
// 边下载边压缩，适合大文件。调用方必须先写出响应头，因此本函数不再
// 返回「跳过」信息（预检阶段已经处理）。
func WriteZipStream(ctx context.Context, w io.Writer, entries []ZipEntry) (int, error) {
	zw := zip.NewWriter(w)
	written := 0
	now := time.Now()

	for _, e := range entries {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.URL, nil)
		if err != nil {
			continue
		}
		resp, err := gbFileHTTPClient.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		// MP4 本身已是压缩格式，再 Deflate 只会浪费 CPU，因此用 Store。
		hdr := &zip.FileHeader{Name: e.FileName, Method: zip.Store}
		hdr.SetModTime(now)
		fw, err := zw.CreateHeader(hdr)
		if err != nil {
			resp.Body.Close()
			_ = zw.Close()
			return written, err
		}
		_, cerr := io.Copy(fw, resp.Body)
		resp.Body.Close()
		if cerr != nil {
			_ = zw.Close()
			return written, fmt.Errorf("写入 %s 失败：%w", e.FileName, cerr)
		}
		written++
	}

	if err := zw.Close(); err != nil {
		return written, err
	}
	return written, nil
}

// zipFileExists 用 HEAD 预检文件是否存在。
//
// 少数文件服务不支持 HEAD（405/501）：此时按「可能存在」处理，
// 真正下载失败会在 WriteZipStream 里跳过该条目。
func zipFileExists(ctx context.Context, u string) bool {
	hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(hctx, http.MethodHead, u, nil)
	if err != nil {
		return false
	}
	resp, err := gbFileHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusMethodNotAllowed, http.StatusNotImplemented:
		return true
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// zipEntryName 取安全的文件名：去掉路径分隔符，避免 zip slip。
func zipEntryName(name string, id int64) string {
	base := filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if base == "" || base == "." || base == "/" {
		base = fmt.Sprintf("record-%d.mp4", id)
	}
	return base
}

// uniqueIDs 去重并过滤非法 ID，保持原有顺序。
func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// SkippedHeaderValue 把跳过文件名编码成可放进 HTTP 头的 ASCII 值。
//
// 文件名可能含中文；HTTP 头按 latin-1 解释，直接放会乱码甚至被中间件拒绝。
// 用 query escape 编码，前端解码后展示。
func SkippedHeaderValue(names []string) string {
	if len(names) == 0 {
		return ""
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, url.QueryEscape(n))
	}
	return strings.Join(out, ",")
}

// getByID 查询录像并统一 NotFound 语义。
func (s *GbCloudRecordService) getByID(ctx context.Context, id int64) (*model.GbCloudRecord, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbCloudRecordNotFound
		}
		return nil, err
	}
	return m, nil
}

// resolveServer 解析录像所属节点；server_id 缺失时回落默认节点。
func (s *GbCloudRecordService) resolveServer(ctx context.Context, serverID string) (*model.GbMediaServer, error) {
	return resolveMediaServerByKey(ctx, serverID)
}

// urlPathEscape 对 URL 路径段做转义（保留可读性，仅转义危险字符）。
func urlPathEscape(s string) string {
	r := strings.NewReplacer(
		"%", "%25", " ", "%20", "/", "%2F", "\\", "%5C",
		"?", "%3F", "#", "%23", "&", "%26", "+", "%2B",
	)
	return r.Replace(s)
}
