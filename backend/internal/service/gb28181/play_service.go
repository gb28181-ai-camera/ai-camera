package gb28181

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 播放 / 云台 / 语音共用的业务错误与通道解析
//
// 本文件只新增类型与函数，不修改 common.go / device_service.go，避免与
// 同批次其他模块的改动冲突。
// ════════════════════════════════════════════════════════════════

var (
	// ErrGbDeviceOffline 设备离线（ac_device.status != online）。
	ErrGbDeviceOffline = errors.New("设备离线，请确认设备已注册并在线")
	// ErrGbPlayProtocolInvalid 播放协议非法。
	ErrGbPlayProtocolInvalid = errors.New("播放协议非法，仅支持 webrtc/flv/hls")
	// ErrGbPlaySessionNotFound 播放会话不存在或已结束。
	ErrGbPlaySessionNotFound = errors.New("播放会话不存在或已结束")
	// ErrGbPlayStreamNotReady INVITE 成功但 ZLM 未在超时窗口内收到流。
	ErrGbPlayStreamNotReady = errors.New("设备未在限定时间内推送媒体流，请检查设备与流媒体节点的连通性")
	// ErrGbMediaServerUnavailable 没有可用的流媒体节点。
	ErrGbMediaServerUnavailable = errors.New("流媒体节点不可用，请检查默认节点配置")
	// ErrGbSnapshotDirNotConfigured 抓图无法落盘（上传目录未配置）。
	ErrGbSnapshotDirNotConfigured = errors.New("上传目录未配置，无法保存抓图")
	// ErrGbSnapshotEmpty 抓图接口返回空数据。
	ErrGbSnapshotEmpty = errors.New("抓图失败：流媒体节点返回空图片")
)

// loadLiveChannelAndDevice 解析通道与其归属设备，并做在线/国标编码校验。
//
// 为什么所有实时类接口都要先过这一关：
//   - 通道不存在 → 404，而不是把国标编码空字符串丢给 INVITE；
//   - 设备未配置国标编码 → 400（先去生成激活凭证）；
//   - 设备离线 → 400 并给出明确中文提示。设备离线时 SendInvite 也会失败，
//     但错误是「设备尚未注册（无信令地址）」，对前端用户不够直观。
func loadLiveChannelAndDevice(ctx context.Context, repo *repository.GbChannelRepo, channelID int64) (*model.GbChannel, *model.Device, error) {
	ch, err := repo.GetByID(ctx, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrGbChannelNotFound
		}
		return nil, nil, err
	}

	dev := ch.Device
	if dev == nil || dev.ID == 0 {
		var d model.Device
		if err := bootstrap.DB().WithContext(ctx).First(&d, ch.DeviceID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil, ErrGbDeviceNotFound
			}
			return nil, nil, err
		}
		dev = &d
	}

	if strings.TrimSpace(dev.GbDeviceID) == "" {
		return nil, nil, ErrGbDeviceNotConfigured
	}
	if !strings.EqualFold(strings.TrimSpace(dev.Status), model.DeviceStatusOnline) {
		return nil, nil, ErrGbDeviceOffline
	}
	return ch, dev, nil
}

// mapLivePlayErr 把编排层错误映射成业务层错误。
func mapLivePlayErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, play.ErrSipUnavailable):
		return ErrSipServiceUnavailable
	case errors.Is(err, play.ErrMediaServerUnavailable):
		return fmt.Errorf("%w：%v", ErrGbMediaServerUnavailable, err)
	case errors.Is(err, play.ErrStreamNotReady):
		return fmt.Errorf("%w：%v", ErrGbPlayStreamNotReady, err)
	case errors.Is(err, play.ErrStreamNotFound):
		return ErrGbPlaySessionNotFound
	case errors.Is(err, play.ErrDeviceOffline):
		return ErrGbDeviceOffline
	default:
		return err
	}
}

// ════════════════════════════════════════════════════════════════
// GbPlayService 实时点播
// ════════════════════════════════════════════════════════════════

// GbPlayService 实时播放业务逻辑。
type GbPlayService struct {
	channelRepo *repository.GbChannelRepo
	orch        *play.Orchestrator
}

// NewGbPlayService 构造实时播放服务。
func NewGbPlayService() *GbPlayService {
	return &GbPlayService{
		channelRepo: repository.NewGbChannelRepo(),
		orch:        play.DefaultOrchestrator(),
	}
}

// Play 发起实时点播。
//
// 幂等语义：同一通道已有在线流时直接复用（前端刷新/重复点击不会重复 INVITE）。
func (s *GbPlayService) Play(ctx context.Context, channelID int64, req *request.GbPlayReq) (*response.GbPlayResp, error) {
	if req == nil {
		req = &request.GbPlayReq{}
	}
	protocol := strings.ToLower(strings.TrimSpace(req.Protocol))
	if protocol == "" {
		protocol = "webrtc"
	}
	switch protocol {
	case "webrtc", "flv", "hls":
	default:
		return nil, ErrGbPlayProtocolInvalid
	}

	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return nil, err
	}

	res, err := s.orch.Play(ctx, dev, ch, protocol)
	if err != nil {
		return nil, mapLivePlayErr(err)
	}

	resp := toGbPlayResp(res)
	// 流已就绪，向 ZLM 读一次实际的视频编码（GB28181-2022 的 H.264/H.265 协商结果）。
	// 失败只留空并告警：编码是「锦上添花」的展示字段，不能让它把已经播通的流判成失败。
	resp.Codec = probePlayCodec(ctx, dev, res)
	return resp, nil
}

// codecProbeTimeout 是 getMediaInfo 探测的上限。
//
// 取 2s 而不是沿用 ZLM 客户端的 10s：此时流已经就绪并即将返回前端，
// 为了一个展示字段把 HTTP 请求再挂 10 秒不可接受。
const codecProbeTimeout = 2 * time.Second

// probePlayCodec 读取 ZLM 流信息里的视频编码名（如 H264 / H265）。
//
// 为什么用 getMediaInfo 而不是解析设备 200 OK 的 SDP：
// SDP 里通告的是**能力列表**（PS/H264/H265 可能全在），实际推流始终是 PS 封装；
// 只有 ZLM 解封装后的 tracks.codec_id_name 才是真实的视频编码。
func probePlayCodec(ctx context.Context, dev *model.Device, res *play.Result) string {
	if res == nil || strings.TrimSpace(res.Stream) == "" {
		return ""
	}
	client, _, err := play.ResolveMediaServer(ctx, dev)
	if err != nil {
		utils.Warn("编码探测：解析流媒体节点失败",
			zap.String("stream", res.Stream), zap.Error(err))
		return ""
	}

	pctx, cancel := context.WithTimeout(ctx, codecProbeTimeout)
	defer cancel()
	info, err := client.GetMediaInfo(pctx, res.App, res.Stream, "", "")
	if err != nil {
		utils.Warn("编码探测：getMediaInfo 失败（不影响播放）",
			zap.String("stream", res.Stream), zap.Error(err))
		return ""
	}
	if info == nil {
		utils.Warn("编码探测：ZLM 未返回流信息（不影响播放）",
			zap.String("stream", res.Stream))
		return ""
	}
	return codecFromTracks(info.Tracks)
}

// codecFromTracks 从轨道列表里挑出视频编码；没有视频轨时回落到音频。
//
// ZLM 的 codec_type：0 = 视频、1 = 音频。codec_id_name 通常是 H264 / H265 / AAC，
// 个别版本会返回 AVC / HEVC 这类别名，统一归一成国标常用的写法。
func codecFromTracks(tracks []mediaserver.MediaTrack) string {
	audio := ""
	for _, tr := range tracks {
		name := normalizeCodecName(tr.CodecIDName)
		if name == "" {
			continue
		}
		if tr.CodecType == 0 {
			return name
		}
		if audio == "" {
			audio = name
		}
	}
	return audio
}

// normalizeCodecName 把编码别名归一成大写标准名。
func normalizeCodecName(name string) string {
	v := strings.ToUpper(strings.TrimSpace(name))
	switch v {
	case "AVC":
		return "H264"
	case "HEVC":
		return "H265"
	default:
		return v
	}
}

// StopPlay 停止实时点播。
//
// ★ 不校验设备在线：停流恰恰是设备离线后的清理手段，若要求在线，
// 离线设备会永远停留在「播放中」，直到进程重启。
func (s *GbPlayService) StopPlay(ctx context.Context, channelID int64, streamID string) error {
	if _, err := s.channelRepo.GetByID(ctx, channelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbChannelNotFound
		}
		return err
	}

	st, ok := s.orch.Registry().GetByChannel(channelID)
	if !ok {
		return ErrGbPlaySessionNotFound
	}

	// stream_id 来自前端，必须校验它确实属于该通道，避免「拿着 A 通道的
	// stream_id 停掉 B 通道」这类越权/误操作。
	if v := strings.TrimSpace(streamID); v != "" && v != st.SessionID && v != st.Stream {
		return ErrGbPlaySessionNotFound
	}

	if err := s.orch.Stop(ctx, st.Stream); err != nil {
		return mapLivePlayErr(err)
	}
	return nil
}

// PlayStatus 查询通道的点播状态。
func (s *GbPlayService) PlayStatus(ctx context.Context, channelID int64) (*response.GbPlayStatusResp, error) {
	if _, err := s.channelRepo.GetByID(ctx, channelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbChannelNotFound
		}
		return nil, err
	}

	st, err := s.orch.Status(ctx, channelID)
	if err != nil {
		return nil, err
	}
	resp := &response.GbPlayStatusResp{
		ChannelID:     st.ChannelID,
		Playing:       st.Playing,
		Ready:         st.Ready,
		App:           st.App,
		Stream:        st.Stream,
		SSRC:          st.SSRC,
		HTTPBase:      st.HTTPBase,
		WebRTC:        st.URLs.WebRTC,
		FLV:           st.URLs.FLV,
		HLS:           st.URLs.HLS,
		MediaServerID: st.MediaServerID,
		StreamID:      st.StreamID,
	}
	if st.StartDT != nil {
		resp.StartDT = st.StartDT.Format("2006-01-02 15:04:05")
	}
	return resp, nil
}

// Snapshot 对通道抓图并保存到本服务的上传目录。
//
// 没有进行中的播放会话时会先点播（webrtc）再抓图：ZLM 的 getSnap 需要一个
// 可拉取的流地址，对「从未点播过的通道」直接抓图必然失败。
func (s *GbPlayService) Snapshot(ctx context.Context, channelID int64) (*response.GbSnapshotResp, error) {
	_, _, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return nil, err
	}

	st, ok := s.orch.Registry().GetByChannel(channelID)
	if !ok {
		if _, err := s.Play(ctx, channelID, &request.GbPlayReq{Protocol: "webrtc"}); err != nil {
			return nil, err
		}
		st, ok = s.orch.Registry().GetByChannel(channelID)
		if !ok {
			return nil, ErrGbPlaySessionNotFound
		}
	}

	var client *mediaserver.Client
	if st.MediaServer != nil {
		client = mediaserver.FromModel(st.MediaServer)
	} else {
		c, _, err := play.ResolveMediaServer(ctx, &model.Device{MediaServerID: st.MediaServerID})
		if err != nil {
			return nil, mapLivePlayErr(err)
		}
		client = c
	}

	// RTSP 最稳（ZLM 内部协议），未配置 RTSP 端口时退回 HTTP-FLV。
	streamURL := st.URLs.RTSP
	if strings.TrimSpace(streamURL) == "" {
		streamURL = st.URLs.FLV
	}
	if strings.TrimSpace(streamURL) == "" {
		return nil, errors.New("播放会话没有可用的拉流地址，无法抓图")
	}

	snapCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	data, err := client.GetSnap(snapCtx, streamURL, 5, 10, false)
	if err != nil {
		return nil, fmt.Errorf("抓图失败：%w", err)
	}
	if len(data) == 0 {
		return nil, ErrGbSnapshotEmpty
	}

	url, name, err := saveLiveSnapshot(data)
	if err != nil {
		return nil, err
	}
	return &response.GbSnapshotResp{
		URL:           url,
		FileName:      name,
		FileSize:      int64(len(data)),
		Stream:        st.Stream,
		MediaServerID: st.MediaServerID,
		CaptureDT:     time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// toGbPlayResp 把编排结果转成契约响应。
func toGbPlayResp(res *play.Result) *response.GbPlayResp {
	if res == nil {
		return nil
	}
	return &response.GbPlayResp{
		App:           res.App,
		Stream:        res.Stream,
		SSRC:          res.SSRC,
		HTTPBase:      res.HTTPBase,
		WebRTC:        res.URLs.WebRTC,
		FLV:           res.URLs.FLV,
		HLS:           res.URLs.HLS,
		MediaServerID: res.MediaServerID,
		StreamID:      res.StreamID,
	}
}

// saveLiveSnapshot 把 JPEG 写入与 /web/upload 相同约定的目录。
//
// 存储约定（与 upload_handler.go 保持一致）：
//   - 物理路径：<cfg.Media.UploadDir>/<yyyyMM>/<随机名>.jpg
//   - 访问地址：<cfg.Media.BaseURL>/<yyyyMM>/<随机名>.jpg
func saveLiveSnapshot(data []byte) (url, name string, err error) {
	cfg := bootstrap.Config()
	if cfg == nil || strings.TrimSpace(cfg.Media.UploadDir) == "" {
		return "", "", ErrGbSnapshotDirNotConfigured
	}

	name, err = randomSnapshotName()
	if err != nil {
		return "", "", err
	}

	subDir := time.Now().Format("200601")
	absDir := filepath.Join(cfg.Media.UploadDir, subDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", "", fmt.Errorf("创建抓图目录失败：%w", err)
	}
	if err := os.WriteFile(filepath.Join(absDir, name), data, 0o644); err != nil {
		return "", "", fmt.Errorf("保存抓图失败：%w", err)
	}

	relPath := filepath.ToSlash(filepath.Join(subDir, name))
	base := strings.TrimRight(strings.TrimSpace(cfg.Media.BaseURL), "/")
	if base == "" {
		// BaseURL 未配置时给出站内相对地址：静态目录挂在 /uploads。
		base = "/uploads"
	}
	return base + "/" + relPath, name, nil
}

// randomSnapshotName 生成 <32 位随机 hex>.jpg。
func randomSnapshotName() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("生成抓图文件名失败：%w", err)
	}
	return hex.EncodeToString(buf[:]) + ".jpg", nil
}
