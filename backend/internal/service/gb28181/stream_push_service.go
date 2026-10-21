package gb28181

// 推流配置。
//
// ★ 实现选择（本批次明确）：「直接给出推流地址」而不是调用 ZLM 的
// addFFmpegSource。理由：
//   - addFFmpegSource 是「让 ZLM 去拉一个 URL」，语义是拉流，不是推流；
//   - 现有 mediaserver 客户端没有「向外部目标持续推流」的 REST API
//     （startSendRtp 属于语音回推/级联，且需要接收端 SIP 协商）；
//   - 本期级联 SIP 交互不在范围内（见 platform_service.go）。
//
// 因此 Start/Stop 只维护启停状态；地址由平台按媒体节点生成并在列表返回，
// 真正的推流由外部编码器推到该地址。这不是「假装已推流」—— status 字段
// 表达的是「允许/启用推流配置」，UI 文案需与此一致。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

var (
	// ErrGbStreamPushNotFound 推流配置不存在。
	ErrGbStreamPushNotFound = errors.New("推流配置不存在")
	// ErrGbStreamPushExists 同一节点的 app+stream 已存在推流配置。
	ErrGbStreamPushExists = errors.New("该 app+stream 已存在推流配置")
)

// GbStreamPushService 推流业务逻辑。
type GbStreamPushService struct {
	repo *repository.GbStreamPushRepo
}

// NewGbStreamPushService 构造推流服务。
func NewGbStreamPushService() *GbStreamPushService {
	return &GbStreamPushService{repo: repository.NewGbStreamPushRepo()}
}

// Page 推流分页（附带平台生成的推流地址）。
func (s *GbStreamPushService) Page(ctx context.Context, req *request.PageGbStreamPushReq, ds *utils.DataScope) (*response.PageGbStreamPushResp, error) {
	if req == nil {
		req = &request.PageGbStreamPushReq{}
	}
	req.Normalize()
	rows, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}

	cache := map[string]*model.GbMediaServer{}
	items := make([]response.GbStreamPushItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, response.GbStreamPushItem{
			GbStreamPush: m,
			PushURL:      s.pushURL(ctx, m, cache),
		})
	}
	return &response.PageGbStreamPushResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Create 新增推流配置（状态默认 OFF）。
func (s *GbStreamPushService) Create(ctx context.Context, req *request.CreateGbStreamPushReq) (*model.GbStreamPush, error) {
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	name, app, stream := trim(req.Name), trim(req.App), trim(req.Stream)
	if name == "" || app == "" || stream == "" {
		return nil, errors.New("名称、app、stream 均不能为空")
	}
	if _, err := s.repo.GetByAppStream(ctx, app, stream); err == nil {
		return nil, ErrGbStreamPushExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	m := &model.GbStreamPush{
		Name:             name,
		App:              app,
		Stream:           stream,
		MediaServerID:    trim(req.MediaServerID),
		Status:           model.GbStatusOff,
		Pushing:          boolOr(nil, false),
		SelfPush:         boolOr(req.SelfPush, false),
		StartOfflinePush: boolOr(req.StartOfflinePush, false),
		Remark:           trim(req.Remark),
		Active:           boolOr(req.Active, true),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 编辑推流配置。
func (s *GbStreamPushService) Update(ctx context.Context, id int64, req *request.UpdateGbStreamPushReq) error {
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
	setBool := func(key string, v *bool) {
		if v != nil {
			fields[key] = *v
		}
	}
	setStr("name", req.Name)
	setStr("media_server_id", req.MediaServerID)
	setStr("remark", req.Remark)
	setBool("self_push", req.SelfPush)
	setBool("start_offline_push", req.StartOfflinePush)
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
			return ErrGbStreamPushExists
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return s.repo.UpdateFields(ctx, id, fields)
}

// Delete 删除推流配置。
func (s *GbStreamPushService) Delete(ctx context.Context, id int64) error {
	if _, err := s.getByID(ctx, id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// Start 启用推流并返回（在列表中可见的）推流地址。
func (s *GbStreamPushService) Start(ctx context.Context, id int64) error {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	if _, err := s.pushURLRequired(ctx, *m); err != nil {
		return err
	}
	now := time.Now()
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{
		"status":    model.GbStatusOn,
		"pushing":   true,
		"push_time": now,
		"active":    true,
	})
}

// Stop 停止推流配置。
func (s *GbStreamPushService) Stop(ctx context.Context, id int64) error {
	if _, err := s.getByID(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{
		"status":  model.GbStatusOff,
		"pushing": false,
	})
}

// pushURL 计算推流地址；节点解析失败时返回空串（列表不因一个坏配置整体失败）。
func (s *GbStreamPushService) pushURL(ctx context.Context, m model.GbStreamPush, cache map[string]*model.GbMediaServer) string {
	key := strings.TrimSpace(m.MediaServerID)
	if ms, ok := cache[key]; ok {
		return formatPushURL(ms, m.App, m.Stream)
	}
	ms, err := resolveMediaServerByKey(ctx, key)
	if err != nil {
		cache[key] = nil
		return ""
	}
	cache[key] = ms
	return formatPushURL(ms, m.App, m.Stream)
}

// pushURLRequired 计算推流地址，节点/端口不可用时返回明确错误。
func (s *GbStreamPushService) pushURLRequired(ctx context.Context, m model.GbStreamPush) (string, error) {
	ms, err := resolveMediaServerByKey(ctx, m.MediaServerID)
	if err != nil {
		return "", err
	}
	u := formatPushURL(ms, m.App, m.Stream)
	if u == "" {
		return "", errors.New("媒体节点未配置可用的推流地址")
	}
	return u, nil
}

// formatPushURL 按节点配置拼 rtmp:// 地址。
//
// 推流端口必须取 RTMPPort：RTMP 不能回退到 HTTP 端口，
// 否则会生成一个「看起来有地址但连不上」的 URL（与 BuildPlayURLs 同理）。
func formatPushURL(ms *model.GbMediaServer, app, stream string) string {
	if ms == nil || strings.TrimSpace(app) == "" || strings.TrimSpace(stream) == "" {
		return ""
	}
	host := firstNonEmpty(ms.StreamIP, ms.IP, "127.0.0.1")
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://"), "/")
	port := ms.RTMPPort
	if port <= 0 {
		port = 1935
	}
	return fmt.Sprintf("rtmp://%s:%d/%s/%s", host, port, urlPathEscape(app), urlPathEscape(stream))
}

func (s *GbStreamPushService) getByID(ctx context.Context, id int64) (*model.GbStreamPush, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbStreamPushNotFound
		}
		return nil, err
	}
	return m, nil
}
