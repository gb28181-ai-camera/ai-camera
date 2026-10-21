package gb28181

// 移动位置（GIS 轨迹）。
//
// 数据来源是设备的 MANSCDP MobilePosition 上报（由信令层落库）；
// 本服务只做读取与「发起位置订阅」。订阅记录本身由
// subscribe_service.go 维护，这里不重复写 ac_gb_subscribe。

import (
	"context"
	"errors"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ErrGbPositionNotFound 通道尚无位置上报。
var ErrGbPositionNotFound = errors.New("暂无该通道的位置上报")

// GbPositionService 移动位置业务逻辑。
type GbPositionService struct {
	repo     *repository.GbMobilePositionRepo
	chanRepo *repository.GbChannelRepo
}

// NewGbPositionService 构造移动位置服务。
func NewGbPositionService() *GbPositionService {
	return &GbPositionService{
		repo:     repository.NewGbMobilePositionRepo(),
		chanRepo: repository.NewGbChannelRepo(),
	}
}

// Latest 某通道的最新位置。
func (s *GbPositionService) Latest(ctx context.Context, channelID int64) (*response.GbPositionResp, error) {
	m, err := s.repo.GetLatestByChannel(ctx, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbPositionNotFound
		}
		return nil, err
	}
	out := toGbPositionItem(*m)
	// GetLatestByChannel 不带 Preload（仓库由其他模块维护，不在本批次改动），
	// 这里补一次通道/设备名称，保证列表展示字段完整。
	if out.ChannelName == "" && m.ChannelID != nil {
		if ch, err := s.chanRepo.GetByID(ctx, *m.ChannelID); err == nil {
			out.ChannelName = ch.Name
			if ch.Device != nil {
				out.DeviceName = ch.Device.Name
			}
		}
	}
	return &out, nil
}

// Realtime 每个通道的最新位置（GIS 实时图层）。
//
// 用 PostgreSQL 的 DISTINCT ON 一次取回，避免「先查通道再逐通道查最新位置」
// 的 N+1；通道数量在国标场景下可能上百，逐条查会明显变慢。
func (s *GbPositionService) Realtime(ctx context.Context, ds ...*utils.DataScope) ([]response.GbPositionResp, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbMobilePosition{}).
		Select("DISTINCT ON (channel_id) *").
		Order("channel_id ASC, report_time DESC")
	if len(ds) > 0 && ds[0] != nil {
		q = ds[0].ApplyToQuery(q, "ac_gb_mobile_position.dept_id")
	}

	var rows []model.GbMobilePosition
	if err := q.Preload("Device").Preload("Channel").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]response.GbPositionResp, 0, len(rows))
	for _, m := range rows {
		out = append(out, toGbPositionItem(m))
	}
	return out, nil
}

// History 位置历史分页（轨迹回放）。
func (s *GbPositionService) History(ctx context.Context, req *request.GbPositionHistoryReq, ds *utils.DataScope) (*response.GbPositionHistoryResp, error) {
	if req == nil {
		req = &request.GbPositionHistoryReq{}
	}
	req.Normalize()

	pageReq := &request.PageGbMobilePositionReq{
		PageReq:      req.PageReq,
		DateRangeReq: req.DateRangeReq,
		DeviceID:     req.DeviceID,
		ChannelID:    req.ChannelID,
		GbChannelID:  req.GbChannelID,
	}
	rows, total, err := s.repo.Page(ctx, pageReq, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.GbPositionResp, 0, len(rows))
	for _, m := range rows {
		items = append(items, toGbPositionItem(m))
	}
	return &response.GbPositionHistoryResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Subscribe 向设备发起移动位置订阅（SUBSCRIBE presence）。
//
// interval<=0 时按 60 秒；订阅有效期与续订由信令层的订阅管理负责，
// 这里只负责把指令发出去。
func (s *GbPositionService) Subscribe(ctx context.Context, channelID int64, interval int) error {
	_, dev, err := loadChannelAndDevice(ctx, channelID)
	if err != nil {
		return err
	}
	sipSvc, err := activeSipService()
	if err != nil {
		return err
	}
	if interval <= 0 {
		interval = 60
	}
	return sipSvc.Subscribe(ctx, dev, model.GbSubscribeMobilePosition, interval)
}

// toGbPositionItem 模型 → 响应，关联名称来自 Preload。
func toGbPositionItem(m model.GbMobilePosition) response.GbPositionResp {
	deviceName, channelName := "", ""
	if m.Device != nil {
		deviceName = m.Device.Name
	}
	if m.Channel != nil {
		channelName = m.Channel.Name
	}
	return response.GbPositionResp{
		ID:          m.ID,
		DeviceID:    m.DeviceID,
		DeviceName:  deviceName,
		ChannelID:   m.ChannelID,
		ChannelName: channelName,
		GbChannelID: m.GbChannelID,
		ReportTime:  m.ReportTime,
		Longitude:   m.Longitude,
		Latitude:    m.Latitude,
		Altitude:    m.Altitude,
		Speed:       m.Speed,
		Direction:   m.Direction,
	}
}
