package gb28181

// 拉流代理：平台让 ZLM 主动从 rtsp/rtmp/http 源拉流。
//
// 与「国标设备推流」的区别：拉流代理的源在平台之外，ZLM 是客户端。
// Start/Stop 才真正调用 ZLM；Create/Update 只落配置，避免编辑时误触重连。

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

var (
	// ErrGbStreamProxyNotFound 拉流代理不存在。
	ErrGbStreamProxyNotFound = errors.New("拉流代理不存在")
	// ErrGbStreamProxyExists 同一节点的 app+stream 已存在。
	ErrGbStreamProxyExists = errors.New("该 app+stream 已存在拉流代理")
)

// GbStreamProxyService 拉流代理业务逻辑。
type GbStreamProxyService struct {
	repo *repository.GbStreamProxyRepo
}

// NewGbStreamProxyService 构造拉流代理服务。
func NewGbStreamProxyService() *GbStreamProxyService {
	return &GbStreamProxyService{repo: repository.NewGbStreamProxyRepo()}
}

// Page 拉流代理分页。
func (s *GbStreamProxyService) Page(ctx context.Context, req *request.PageGbStreamProxyReq, ds *utils.DataScope) (*response.PageGbStreamProxyResp, error) {
	if req == nil {
		req = &request.PageGbStreamProxyReq{}
	}
	req.Normalize()
	rows, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.GbStreamProxyItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, response.GbStreamProxyItem{GbStreamProxy: m})
	}
	return &response.PageGbStreamProxyResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Create 新增拉流代理（只落配置，不拉起）。
func (s *GbStreamProxyService) Create(ctx context.Context, req *request.CreateGbStreamProxyReq) (*model.GbStreamProxy, error) {
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	name, app, stream, src := trim(req.Name), trim(req.App), trim(req.Stream), trim(req.SrcURL)
	if name == "" || app == "" || stream == "" || src == "" {
		return nil, errors.New("名称、app、stream、源地址均不能为空")
	}
	if _, err := s.repo.GetByAppStream(ctx, app, stream); err == nil {
		return nil, ErrGbStreamProxyExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	m := &model.GbStreamProxy{
		Name:                    name,
		Type:                    firstNonEmpty(req.Type, "default"),
		App:                     app,
		Stream:                  stream,
		SrcURL:                  src,
		TimeoutMS:               defaultInt64(req.TimeoutMS, 10000),
		FFmpegCmdKey:            trim(req.FFmpegCmdKey),
		RTSPType:                req.RTSPType,
		MediaServerID:           trim(req.MediaServerID),
		EnableAudio:             boolOr(req.EnableAudio, true),
		EnableMP4:               boolOr(req.EnableMP4, false),
		Enable:                  boolOr(req.Enable, true),
		EnableDisableNoneReader: boolOr(req.EnableDisableNoneReader, false),
		EnableRemoveNoneReader:  boolOr(req.EnableRemoveNoneReader, false),
		Pulling:                 boolOr(nil, false),
		Remark:                  trim(req.Remark),
		Active:                  boolOr(req.Active, true),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 编辑拉流代理（增量）。
func (s *GbStreamProxyService) Update(ctx context.Context, id int64, req *request.UpdateGbStreamProxyReq) error {
	if req == nil {
		return errors.New("请求不能为空")
	}
	m, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	fields := map[string]interface{}{}
	setStr := func(key string, v *string) {
		if v != nil {
			fields[key] = trim(*v)
		}
	}
	setInt := func(key string, v *int64) {
		if v != nil {
			fields[key] = *v
		}
	}
	setBool := func(key string, v *bool) {
		if v != nil {
			fields[key] = *v
		}
	}

	setStr("name", req.Name)
	setStr("type", req.Type)
	setStr("src_url", req.SrcURL)
	setInt("timeout_ms", req.TimeoutMS)
	setStr("ffmpeg_cmd_key", req.FFmpegCmdKey)
	setInt("rtsp_type", req.RTSPType)
	setStr("media_server_id", req.MediaServerID)
	setBool("enable_audio", req.EnableAudio)
	setBool("enable_mp4", req.EnableMP4)
	setBool("enable", req.Enable)
	setBool("enable_disable_none_reader", req.EnableDisableNoneReader)
	setBool("enable_remove_none_reader", req.EnableRemoveNoneReader)
	setStr("remark", req.Remark)
	setBool("active", req.Active)

	app, stream := m.App, m.Stream
	if req.App != nil {
		app = trim(*req.App)
		fields["app"] = app
	}
	if req.Stream != nil {
		stream = trim(*req.Stream)
		fields["stream"] = stream
	}
	if (req.App != nil || req.Stream != nil) && (app == "" || stream == "") {
		return errors.New("app 与 stream 不能为空")
	}
	if app != m.App || stream != m.Stream {
		if other, err := s.repo.GetByAppStream(ctx, app, stream); err == nil && other.ID != id {
			return ErrGbStreamProxyExists
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if v, ok := fields["name"]; ok && v == "" {
		return errors.New("名称不能为空")
	}
	if v, ok := fields["src_url"]; ok && v == "" {
		return errors.New("源地址不能为空")
	}
	return s.repo.UpdateFields(ctx, id, fields)
}

// Delete 删除拉流代理；正在拉流时先尽力停止。
func (s *GbStreamProxyService) Delete(ctx context.Context, id int64) error {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	if boolValue(m.Pulling) {
		_ = s.stopOnServer(ctx, m)
	}
	return s.repo.Delete(ctx, id)
}

// Start 让 ZLM 开始拉流。
func (s *GbStreamProxyService) Start(ctx context.Context, id int64) error {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	if boolValue(m.Pulling) {
		return nil // 幂等
	}
	ms, err := resolveMediaServerByKey(ctx, m.MediaServerID)
	if err != nil {
		return err
	}
	client := mediaserver.FromModel(ms)
	result, err := client.AddStreamProxy(ctx, mediaserver.AddStreamProxyParam{
		App:         m.App,
		Stream:      m.Stream,
		URL:         m.SrcURL,
		EnableAudio: boolValue(m.EnableAudio),
		EnableMP4:   boolValue(m.EnableMP4),
		RTPType:     strconv.FormatInt(m.RTSPType, 10),
		TimeoutSec:  int(firstInt64(m.TimeoutMS, 10000) / 1000),
		RetryCount:  3,
	})
	if err != nil {
		return err
	}
	fields := map[string]interface{}{
		"pulling": true,
		"active":  true,
		"enable":  true,
	}
	if result != nil && result.Key != "" {
		fields["stream_key"] = result.Key
	}
	return s.repo.UpdateFields(ctx, id, fields)
}

// Stop 停止拉流。
func (s *GbStreamProxyService) Stop(ctx context.Context, id int64) error {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	if !boolValue(m.Pulling) {
		return nil // 幂等
	}
	if err := s.stopOnServer(ctx, m); err != nil {
		return err
	}
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{"pulling": false})
}

// stopOnServer 调用 ZLM 停止拉流。
//
// addStreamProxy 返回的 key 是 delStreamProxy 的唯一入参；历史数据没有 key
// 时退回 close_streams（单端口模式下按 app+stream 关流同样有效）。
func (s *GbStreamProxyService) stopOnServer(ctx context.Context, m *model.GbStreamProxy) error {
	ms, err := resolveMediaServerByKey(ctx, m.MediaServerID)
	if err != nil {
		return err
	}
	client := mediaserver.FromModel(ms)
	if strings.TrimSpace(m.StreamKey) != "" {
		return client.DelStreamProxy(ctx, m.StreamKey)
	}
	return client.CloseStreams(ctx, m.App, m.Stream)
}

func (s *GbStreamProxyService) getByID(ctx context.Context, id int64) (*model.GbStreamProxy, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbStreamProxyNotFound
		}
		return nil, err
	}
	return m, nil
}

// defaultInt64 空值取默认。
func defaultInt64(v, def int64) int64 {
	if v > 0 {
		return v
	}
	return def
}

// firstInt64 返回第一个非零值。
func firstInt64(values ...int64) int64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}
