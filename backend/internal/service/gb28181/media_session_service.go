package gb28181

// 媒体会话视图与拉流代理 / 推流的播放地址。
//
// 与「国标点播」的区别：代理与推流的流已经存在于 ZLM，平台不需要发
// INVITE，只需要确认流真的在节点上，然后把播放地址返回给前端。

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

var (
	// ErrGbStreamNotRunning 流未启动 / 未推送到节点（HTTP 409）。
	ErrGbStreamNotRunning = errors.New("流未启动或尚未推送到媒体节点")
	// ErrGbMediaServerUnreachable 媒体节点不可达（HTTP 409）。
	ErrGbMediaServerUnreachable = errors.New("媒体节点不可达")
)

// GbMediaSessionService 流会话与流播放地址服务。
type GbMediaSessionService struct {
	serverRepo *repository.GbMediaServerRepo
	proxyRepo  *repository.GbStreamProxyRepo
	pushRepo   *repository.GbStreamPushRepo
}

// NewGbMediaSessionService 构造流会话服务。
func NewGbMediaSessionService() *GbMediaSessionService {
	return &GbMediaSessionService{
		serverRepo: repository.NewGbMediaServerRepo(),
		proxyRepo:  repository.NewGbStreamProxyRepo(),
		pushRepo:   repository.NewGbStreamPushRepo(),
	}
}

// ListByMediaServer 返回节点当前全部流会话。
//
// 节点不可达时返回 200 + online=false + message：这属于「节点故障」的
// 正常观测结果，不是接口调用错误，用 5xx 会让前端把整页判为失败。
func (s *GbMediaSessionService) ListByMediaServer(ctx context.Context, id int64) (*response.GbMediaStreamListResp, error) {
	ms, err := s.serverRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMediaServerNotFound
		}
		return nil, err
	}

	resp := &response.GbMediaStreamListResp{
		MediaServerID: ms.ServerID,
		Items:         []response.GbMediaStreamItem{},
	}

	infos, err := mediaserver.FromModel(ms).GetMediaList(ctx, "", "", "", "")
	if err != nil {
		resp.Online = false
		resp.Message = "媒体节点不可达: " + err.Error()
		return resp, nil
	}
	resp.Online = true
	for _, info := range infos {
		resp.Items = append(resp.Items, toGbMediaStreamItem(info))
	}
	resp.Total = len(resp.Items)
	resp.Message = "节点在线"
	return resp, nil
}

// PlayProxy 返回拉流代理的播放地址。
func (s *GbMediaSessionService) PlayProxy(ctx context.Context, id int64) (*response.GbStreamPlayResp, error) {
	m, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbStreamProxyNotFound
		}
		return nil, err
	}
	if !boolValue(m.Active) || !boolValue(m.Pulling) {
		return nil, ErrGbStreamNotRunning
	}
	return s.buildPlayResp(ctx, m.MediaServerID, m.Name, m.App, m.Stream, id)
}

// PlayPush 返回推流的播放地址。
func (s *GbMediaSessionService) PlayPush(ctx context.Context, id int64) (*response.GbStreamPlayResp, error) {
	m, err := s.pushRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbStreamPushNotFound
		}
		return nil, err
	}
	if !boolValue(m.Pushing) || !strings.EqualFold(m.Status, model.GbStatusOff) {
		return nil, ErrGbStreamNotRunning
	}
	return s.buildPlayResp(ctx, m.MediaServerID, m.Name, m.App, m.Stream, id)
}

// buildPlayResp 校验流在节点上存在并拼出各协议地址。
func (s *GbMediaSessionService) buildPlayResp(ctx context.Context, mediaServerKey, name, app, stream string, id int64) (*response.GbStreamPlayResp, error) {
	ms, err := resolveMediaServerByKey(ctx, mediaServerKey)
	if err != nil {
		return nil, err
	}
	infos, err := mediaserver.FromModel(ms).GetMediaList(ctx, app, stream, "", "")
	if err != nil {
		return nil, ErrGbMediaServerUnreachable
	}
	alive := false
	for _, info := range infos {
		if info.App == app && info.Stream == stream {
			alive = true
			break
		}
	}
	if !alive {
		return nil, ErrGbStreamNotRunning
	}

	urls := mediaserver.BuildPlayURLs(ms, app, stream)
	protocols := make([]string, 0, 3)
	for _, p := range []string{"webrtc", "flv", "hls"} {
		if playURLForProtocol(urls, p) != "" {
			protocols = append(protocols, p)
		}
	}
	return &response.GbStreamPlayResp{
		ID:            id,
		Name:          name,
		App:           app,
		Stream:        stream,
		MediaServerID: ms.ServerID,
		WebRTC:        urls.WebRTC,
		FLV:           urls.FLV,
		HLS:           urls.HLS,
		WSFLV:         urls.WSFLV,
		RTSP:          urls.RTSP,
		RTMP:          urls.RTMP,
		Protocols:     protocols,
	}, nil
}

// toGbMediaStreamItem 把 ZLM 的流信息压平成列表项（取首条视频轨）。
func toGbMediaStreamItem(info mediaserver.MediaInfo) response.GbMediaStreamItem {
	item := response.GbMediaStreamItem{
		App:         info.App,
		Stream:      info.Stream,
		Schema:      info.Schema,
		Vhost:       info.Vhost,
		OriginType:  firstNonEmpty(info.OriginTypeStr, intToStr(info.OriginType)),
		OriginURL:   info.OriginURL,
		ReaderCount: info.ReaderCount,
		AliveSecond: info.AliveSecond,
		BytesSpeed:  info.BytesSpeed,
		CreateStamp: info.CreateStamp,
	}
	if info.OriginSock != nil {
		item.OriginPeerIP = info.OriginSock.PeerIP
		item.OriginPeerPort = info.OriginSock.PeerPort
	}
	for _, track := range info.Tracks {
		if track.CodecType == 0 && item.Codec == "" { // 0 = 视频
			item.Codec = firstNonEmpty(track.CodecIDName, intToStr(track.CodecID))
			item.Width, item.Height, item.FPS = track.Width, track.Height, track.FPS
		}
		if track.CodecType == 1 { // 1 = 音频
			item.HasAudio = true
			item.AudioCodec = firstNonEmpty(track.CodecIDName, intToStr(track.CodecID))
		}
	}
	return item
}

// playURLForProtocol 按协议名取地址。
func playURLForProtocol(u mediaserver.PlayURLs, protocol string) string {
	switch protocol {
	case "webrtc":
		return u.WebRTC
	case "flv":
		return u.FLV
	case "hls":
		return u.HLS
	}
	return ""
}

// intToStr 整数转字符串（ZLM 部分字段是 int 枚举，0 视为「未知」）。
func intToStr(v int) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(v)
}
