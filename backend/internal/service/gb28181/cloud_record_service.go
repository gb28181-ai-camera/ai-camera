package gb28181

// 云端录像：ZLM 落盘 MP4 的索引查询、播放与下载。
//
// 范围限定（设计 §14.5 选项 A）：本期**只做单文件直接下载**。
// 合并/切片/打包依赖独立 Assist 服务（record_assist_port），
// 未启用时不得在响应里假装支持。

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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
// 与播放共用一个 HTTP 端口。只有需要「多段合并/打包」时才需要 Assist 服务，
// 本批次一律走直链，Packaging 恒为 false。
func (s *GbCloudRecordService) Download(ctx context.Context, id int64) (*response.GbCloudRecordDownloadResp, error) {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 已经是完整 URL 时直接返回（部分 ZLM 部署把录像放在对象存储）。
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(m.FilePath)), "http") {
		return &response.GbCloudRecordDownloadResp{
			ID:          m.ID,
			URL:         strings.TrimSpace(m.FilePath),
			FileName:    firstNonEmpty(m.FileName, filepath.Base(m.FilePath)),
			FileSize:    m.FileSize,
			ContentType: "video/mp4",
			Packaging:   false,
			Message:     "录像文件直链",
		}, nil
	}

	fileName := strings.TrimSpace(m.FileName)
	if fileName == "" && strings.TrimSpace(m.FilePath) != "" {
		fileName = filepath.Base(strings.ReplaceAll(m.FilePath, "\\", "/"))
	}
	if fileName == "" {
		return nil, errors.New("录像缺少文件名，无法生成下载地址")
	}
	app := firstNonEmpty(m.App, "record")
	if strings.TrimSpace(m.Stream) == "" {
		return nil, errors.New("录像缺少流名，无法生成下载地址")
	}
	ms, err := s.resolveServer(ctx, m.MediaServerID)
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/record/%s/%s/%s",
		mediaHTTPBase(ms), urlPathEscape(app), urlPathEscape(m.Stream), urlPathEscape(fileName))

	msg := "单文件直接下载"
	if ms.RecordAssistPort > 0 {
		// Assist 服务存在但本批次未接入：明确告知只提供单文件直链，
		// 避免前端误以为已经打包。
		msg = "单文件直接下载（Assist 打包能力未接入，多段录像需逐段下载）"
	}
	return &response.GbCloudRecordDownloadResp{
		ID:          m.ID,
		URL:         u,
		FileName:    fileName,
		FileSize:    m.FileSize,
		ContentType: "video/mp4",
		Packaging:   false,
		Message:     msg,
	}, nil
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
