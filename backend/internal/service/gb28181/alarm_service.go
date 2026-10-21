package gb28181

// 国标报警（设备主动上报的 MANSCDP Alarm）。
//
// 与平台自研的 ac_alarm_record 区分：这里只处理国标设备上报的报警，
// 不参与 AI 风险识别链路。抓图复用 ZLM 的 getSnap。

import (
	"context"
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
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ErrGbAlarmNotFound 国标报警不存在。
var ErrGbAlarmNotFound = errors.New("国标报警不存在")

// GbAlarmService 国标报警业务逻辑。
type GbAlarmService struct {
	repo     *repository.GbAlarmRepo
	chanRepo *repository.GbChannelRepo
	devRepo  *repository.DeviceRepo
}

// NewGbAlarmService 构造国标报警服务。
func NewGbAlarmService() *GbAlarmService {
	return &GbAlarmService{
		repo:     repository.NewGbAlarmRepo(),
		chanRepo: repository.NewGbChannelRepo(),
		devRepo:  repository.NewDeviceRepo(),
	}
}

// Page 国标报警分页。
func (s *GbAlarmService) Page(ctx context.Context, req *request.PageGbAlarmReq, ds *utils.DataScope) (*response.PageGbAlarmResp, error) {
	if req == nil {
		req = &request.PageGbAlarmReq{}
	}
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.GbAlarmItem, 0, len(items))
	for _, m := range items {
		out = append(out, toGbAlarmItem(m))
	}
	return &response.PageGbAlarmResp{
		Items:    out,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// toGbAlarmItem 模型 → 列表项。
//
// 显式取字段而不是内嵌 model.GbAlarm：后者 Preload 了 Channel，
// 会连带序列化 ac_gb_channel.password。
func toGbAlarmItem(m model.GbAlarm) response.GbAlarmItem {
	deviceName, channelName := "", ""
	if m.Device != nil {
		deviceName = m.Device.Name
	}
	if m.Channel != nil {
		channelName = m.Channel.Name
	}
	return response.GbAlarmItem{
		ID:               m.ID,
		DeviceID:         m.DeviceID,
		ChannelID:        m.ChannelID,
		DeviceName:       deviceName,
		ChannelName:      channelName,
		GbDeviceID:       m.GbDeviceID,
		GbChannelID:      m.GbChannelID,
		AlarmPriority:    m.AlarmPriority,
		AlarmMethod:      m.AlarmMethod,
		AlarmType:        m.AlarmType,
		AlarmTime:        m.AlarmTime,
		AlarmDescription: m.AlarmDescription,
		Longitude:        m.Longitude,
		Latitude:         m.Latitude,
		SnapPath:         m.SnapPath,
		RecordPath:       m.RecordPath,
		Handled:          m.Handled,
		Remark:           m.Remark,
		CreatedDT:        m.CreatedDT,
		UpdatedDT:        m.UpdatedDT,
		UpdatedBy:        m.UpdatedBy,
	}
}

// Get 国标报警详情。
func (s *GbAlarmService) Get(ctx context.Context, id int64) (*response.GbAlarmDetailResp, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbAlarmNotFound
		}
		return nil, err
	}
	return &response.GbAlarmDetailResp{GbAlarmItem: toGbAlarmItem(*m)}, nil
}

// Snapshot 对报警关联通道抓图。
//
// 取流顺序：
//  1. ac_gb_channel.stream_id（播放编排写入的当前流）；
//  2. 进程内 SIP 会话（用户正在看该通道，但还没回写 stream_id）。
//
// 两者都没有时明确报错，而不是抓一张黑图。
func (s *GbAlarmService) Snapshot(ctx context.Context, id int64) (*response.GbSnapshotResp, error) {
	alarm, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbAlarmNotFound
		}
		return nil, err
	}

	var ch *model.GbChannel
	if alarm.ChannelID != nil {
		ch, err = s.chanRepo.GetByID(ctx, *alarm.ChannelID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrGbChannelNotFound
			}
			return nil, err
		}
	} else if alarm.GbChannelID != "" {
		ch, err = s.chanRepo.GetByGbChannelID(ctx, alarm.GbChannelID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrGbChannelNotFound
			}
			return nil, err
		}
		if alarm.ChannelID == nil {
			alarm.ChannelID = &ch.ID
		}
	} else {
		return nil, errors.New("报警未关联通道，无法抓图")
	}

	app, stream := "rtp", strings.TrimSpace(ch.StreamID)
	if sipSvc, err := activeSipService(); err == nil {
		if dlg, ok := sipSvc.Sessions().GetByChannel(ch.GbChannelID); ok && dlg != nil {
			app, stream = dlg.App, dlg.Stream
		}
	}
	if stream == "" {
		return nil, errors.New("通道当前没有视频流，请先播放该通道后再抓图")
	}

	dev := ch.Device
	if dev == nil && ch.DeviceID > 0 {
		dev, _ = s.devRepo.GetByID(ctx, ch.DeviceID)
	}
	ms, err := resolveMediaServerForDevice(ctx, dev)
	if err != nil {
		return nil, err
	}

	urls := mediaserver.BuildPlayURLs(ms, app, stream)
	snapURL := firstNonEmpty(urls.RTSP, urls.FLV, urls.HLS)
	if snapURL == "" {
		return nil, errors.New("媒体节点未配置可用的抓图地址")
	}

	client := mediaserver.FromModel(ms)
	jpeg, err := client.GetSnap(ctx, snapURL, 5, 10, false)
	if err != nil {
		return nil, fmt.Errorf("ZLM 抓图失败: %w", err)
	}
	if len(jpeg) == 0 {
		return nil, errors.New("ZLM 返回了空图片")
	}

	rel, url, err := saveSnapshotFile(id, jpeg)
	if err != nil {
		return nil, err
	}

	// 回写 snap_path，便于前端在报警列表中直接展示缩略图。
	if err := s.repo.UpdateFields(ctx, id, map[string]interface{}{"snap_path": url}); err != nil {
		return nil, err
	}

	// 响应类型复用契约 §12 的 GbSnapshotResp（与实时播放抓图一致），
	// 前端可以直接复用同一个图片展示组件。
	return &response.GbSnapshotResp{
		URL:           url,
		FileName:      filepath.Base(rel),
		FileSize:      int64(len(jpeg)),
		Stream:        stream,
		MediaServerID: ms.ServerID,
		CaptureDT:     time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// saveSnapshotFile 把图片写入 <upload_dir>/snapshots，返回相对路径与访问 URL。
func saveSnapshotFile(alarmID int64, data []byte) (rel, url string, err error) {
	cfg := bootstrap.Config()
	if cfg == nil || strings.TrimSpace(cfg.Media.UploadDir) == "" {
		return "", "", errors.New("上传目录未配置，无法保存抓图")
	}
	dir := filepath.Join(cfg.Media.UploadDir, "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("创建抓图目录失败: %w", err)
	}
	name := fmt.Sprintf("gb_alarm_%d_%d.jpg", alarmID, time.Now().UnixNano())
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", "", fmt.Errorf("保存抓图失败: %w", err)
	}
	rel = filepath.ToSlash(filepath.Join("snapshots", name))
	url = strings.TrimRight(cfg.Media.BaseURL, "/") + "/" + rel
	return rel, url, nil
}

// Handle 处理报警。
func (s *GbAlarmService) Handle(ctx context.Context, id int64, req *request.GbAlarmHandleReq, operator string) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbAlarmNotFound
		}
		return err
	}
	handled := true
	if req != nil && req.Handled != nil {
		handled = *req.Handled
	}
	fields := map[string]interface{}{
		"handled":    handled,
		"updated_by": operator,
		"updated_dt": time.Now(),
	}
	if req != nil && strings.TrimSpace(req.Remark) != "" {
		fields["remark"] = trim(req.Remark)
	}
	return s.repo.UpdateFields(ctx, id, fields)
}

// Clear 批量清除（软删除）报警。
func (s *GbAlarmService) Clear(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("请选择要清除的报警")
	}
	for _, id := range ids {
		if err := s.repo.Delete(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
