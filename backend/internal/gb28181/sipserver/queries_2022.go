package sipserver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/version"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// GB28181-2022 查询类命令的请求/应答关联。
//
// 为什么需要等待表：国标 MESSAGE 是「先回 200 OK（无正文），再单独发一条
// MESSAGE 承载业务应答」。HTTP 侧要同步拿到结果，只能发完查询后按
// (设备编码, SN) 等待后续应答。SN 是本进程生成的，设备会原样回带。
//
// 超时统一 5 秒：与录像查询保持一致，慢于这个值的设备即使最终回了，
// 前端也已经拿到超时结果，继续等待没有意义。

// ErrQueryTimeout 表示设备未在超时时间内返回业务应答。
var ErrQueryTimeout = errors.New("设备未在规定时间内返回查询结果")

// ErrVersionUnsupported 表示设备版本不支持该 2022 能力。
var ErrVersionUnsupported = errors.New("设备协议版本不支持该能力（需要 GB28181-2022）")

const query2022Timeout = 5 * time.Second

func queryWaiterKey(dev *model.Device, sn int) string {
	return dev.GbDeviceID + "#" + strconv.Itoa(sn)
}

// waitQueryResponse 注册等待并阻塞直到应答到达、超时或 ctx 取消。
func (s *Service) waitQueryResponse(ctx context.Context, dev *model.Device, sn int) ([]byte, error) {
	ch := make(chan []byte, 1)
	key := queryWaiterKey(dev, sn)
	s.queryWaiters.Store(key, ch)
	defer s.queryWaiters.Delete(key)

	timer := time.NewTimer(query2022Timeout)
	defer timer.Stop()
	select {
	case body := <-ch:
		return body, nil
	case <-timer.C:
		return nil, fmt.Errorf("%w（%s，SN=%d）", ErrQueryTimeout, dev.GbDeviceID, sn)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// DeliverQueryResponse 把设备应答交给等待中的查询方。
//
// 由 handlers.go 的 Response 分支调用；返回 true 表示有等待者接收。
// 没有等待者时返回 false，调用方决定是否只记日志。
func (s *Service) DeliverQueryResponse(dev *model.Device, sn int, body []byte) bool {
	if dev == nil {
		return false
	}
	v, ok := s.queryWaiters.LoadAndDelete(queryWaiterKey(dev, sn))
	if !ok {
		return false
	}
	ch, ok := v.(chan []byte)
	if !ok {
		return false
	}
	select {
	case ch <- body:
	default:
	}
	return true
}

// ensure2022 在发送 2022 独有命令前做能力校验。
func ensure2022(dev *model.Device, feature version.Feature) error {
	if dev == nil {
		return errors.New("设备为 nil")
	}
	if !version.Supports(version.Normalize(dev.GbVersion), feature) {
		return fmt.Errorf("%w（设备 %s 版本 %s）", ErrVersionUnsupported, dev.GbDeviceID, dev.GbVersion)
	}
	return nil
}

// QueryPTZPosition 查询 PTZ 精准状态（2022）。
func (s *Service) QueryPTZPosition(ctx context.Context, dev *model.Device, channelID string) (*manscdp.PTZPositionResponse, error) {
	if err := ensure2022(dev, version.FeaturePTZPosition); err != nil {
		return nil, err
	}
	sn := s.nextSN()
	go func() {
		// 发送失败要让等待方立即返回，而不是干等 5 秒。
		if err := s.query(ctx, dev, manscdp.CmdPTZPosition, &manscdp.PTZPositionQuery{
			CmdType: manscdp.CmdPTZPosition, SN: sn, DeviceID: channelDeviceID(channelID, dev),
		}); err != nil {
			s.queryWaiters.Delete(queryWaiterKey(dev, sn))
		}
	}()
	body, err := s.waitQueryResponse(ctx, dev, sn)
	if err != nil {
		return nil, err
	}
	_, resp, err := manscdp.DecodePTZPosition(body, s.DeviceCharset(dev))
	return resp, err
}

// QueryCruiseTrackList 查询巡航轨迹列表（2022）。
func (s *Service) QueryCruiseTrackList(ctx context.Context, dev *model.Device, channelID string) (*manscdp.CruiseTrackListResponse, error) {
	if err := ensure2022(dev, version.FeatureCruiseTrackList); err != nil {
		return nil, err
	}
	sn := s.nextSN()
	go func() {
		if err := s.query(ctx, dev, manscdp.CmdCruiseTrackListQuery, &manscdp.CruiseTrackListQuery{
			CmdType: manscdp.CmdCruiseTrackListQuery, SN: sn, DeviceID: channelDeviceID(channelID, dev),
		}); err != nil {
			s.queryWaiters.Delete(queryWaiterKey(dev, sn))
		}
	}()
	body, err := s.waitQueryResponse(ctx, dev, sn)
	if err != nil {
		return nil, err
	}
	_, resp, err := manscdp.DecodeCruiseTrackList(body, s.DeviceCharset(dev))
	return resp, err
}

// QueryCruiseTrack 查询单条巡航轨迹的预置点明细（2022）。
func (s *Service) QueryCruiseTrack(ctx context.Context, dev *model.Device, channelID string, number int) (*manscdp.CruiseTrackResponse, error) {
	if err := ensure2022(dev, version.FeatureCruiseTrack); err != nil {
		return nil, err
	}
	sn := s.nextSN()
	go func() {
		if err := s.query(ctx, dev, manscdp.CmdCruiseTrackQuery, &manscdp.CruiseTrackQuery{
			CmdType: manscdp.CmdCruiseTrackQuery, SN: sn,
			DeviceID: channelDeviceID(channelID, dev), Number: number,
		}); err != nil {
			s.queryWaiters.Delete(queryWaiterKey(dev, sn))
		}
	}()
	body, err := s.waitQueryResponse(ctx, dev, sn)
	if err != nil {
		return nil, err
	}
	_, resp, err := manscdp.DecodeCruiseTrack(body, s.DeviceCharset(dev))
	return resp, err
}

// QueryHomePosition 查询看守位（2022）。
func (s *Service) QueryHomePosition(ctx context.Context, dev *model.Device, channelID string) (*manscdp.HomePositionResponse, error) {
	if err := ensure2022(dev, version.FeatureHomePosition); err != nil {
		return nil, err
	}
	sn := s.nextSN()
	go func() {
		if err := s.query(ctx, dev, manscdp.CmdHomePositionQuery, &manscdp.HomePositionQuery{
			CmdType: manscdp.CmdHomePositionQuery, SN: sn, DeviceID: channelDeviceID(channelID, dev),
		}); err != nil {
			s.queryWaiters.Delete(queryWaiterKey(dev, sn))
		}
	}()
	body, err := s.waitQueryResponse(ctx, dev, sn)
	if err != nil {
		return nil, err
	}
	_, resp, err := manscdp.DecodeHomePosition(body, s.DeviceCharset(dev))
	return resp, err
}

// QuerySDCardStatus 查询存储卡状态（2022）。
func (s *Service) QuerySDCardStatus(ctx context.Context, dev *model.Device, channelID string) (*manscdp.SDCardStatusResponse, error) {
	if err := ensure2022(dev, version.FeatureSDCardStatus); err != nil {
		return nil, err
	}
	sn := s.nextSN()
	go func() {
		if err := s.query(ctx, dev, manscdp.CmdSDCardStatus, &manscdp.SDCardStatusQuery{
			CmdType: manscdp.CmdSDCardStatus, SN: sn, DeviceID: channelDeviceID(channelID, dev),
		}); err != nil {
			s.queryWaiters.Delete(queryWaiterKey(dev, sn))
		}
	}()
	body, err := s.waitQueryResponse(ctx, dev, sn)
	if err != nil {
		return nil, err
	}
	_, resp, err := manscdp.DecodeSDCardStatus(body, s.DeviceCharset(dev))
	return resp, err
}

// ControlPTZPrecise 下发 PTZ 精准控制（2022）。
//
// 与 Preset 一样走带外 MESSAGE；没有业务应答，只需 200 OK。
func (s *Service) ControlPTZPrecise(ctx context.Context, dev *model.Device, channelID string, pan, tilt, zoom *float64) error {
	if err := ensure2022(dev, version.FeaturePTZPreciseCtrl); err != nil {
		return err
	}
	ctl := &manscdp.Control{
		CmdType:    manscdp.CmdControl,
		SN:         s.nextSN(),
		DeviceID:   channelDeviceID(channelID, dev),
		PTZPrecise: &manscdp.PTZPreciseCtrl{Pan: pan, Tilt: tilt, Zoom: zoom},
	}
	return s.query(ctx, dev, manscdp.CmdControl, ctl)
}

// channelDeviceID 未指定通道时回落到设备编码（部分设备要求把命令发给设备本身）。
func channelDeviceID(channelID string, dev *model.Device) string {
	if channelID != "" {
		return channelID
	}
	if dev == nil {
		return ""
	}
	return dev.GbDeviceID
}
