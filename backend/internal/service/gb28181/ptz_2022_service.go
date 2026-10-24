package gb28181

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/version"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// ════════════════════════════════════════════════════════════════
// GB/T 28181-2022 云台扩展查询（精准位置 / 巡航轨迹 / 看守位 / 存储卡）
//
// 与 2016 云台控制的区别：
//   - 2016 的 PTZCmd 是「方向 + 速度」的相对运动，只需 200 OK；
//   - 2022 的查询是「发 MESSAGE → 设备再回一条 MESSAGE 承载业务结果」，
//     sipserver 已按 (设备编码, SN) 做同步等待（最多 5 秒），HTTP 层
//     不再叠加超时，否则会出现「HTTP 先超时、SIP 结果随后丢弃」的假失败。
//
// ★ 为什么 2016 设备必须在服务层提前拦截：
//   2016 设备收到不认识的 CmdType（PTZPosition / CruiseTrackQuery 等）时，
//   通常按标准直接丢弃，连错误应答都不回。若不提前判断版本，用户点击后只能
//   干等 5 秒拿到「设备超时」，从而把「版本不支持」误判成「网络/设备故障」，
//   现场排障方向完全跑偏。因此这里在发报文之前就用 version.Supports 拒绝，
//   并明确告知「该设备为 GB28181-2016，不支持 2022 扩展」。
// ════════════════════════════════════════════════════════════════

var (
	// ErrGb2022Unsupported 设备版本不支持该 2022 能力。
	ErrGb2022Unsupported = errors.New("设备协议版本不支持 2022 扩展能力")
	// ErrGbQueryTimeout 设备未在 sipserver 的 5 秒窗口内返回业务应答。
	ErrGbQueryTimeout = errors.New("设备未在规定时间内返回 2022 查询结果")
	// ErrGbPtzPreciseEmpty 精准控制未提供任何参数。
	ErrGbPtzPreciseEmpty = errors.New("pan/tilt/zoom 至少传一个")
	// ErrGbPtzPreciseInvalid 精准控制参数非法（NaN/Inf）。
	ErrGbPtzPreciseInvalid = errors.New("pan/tilt/zoom 必须是有限数值")
	// ErrGbCruiseTrackNumberInvalid 巡航轨迹编号非法。
	ErrGbCruiseTrackNumberInvalid = errors.New("巡航轨迹编号不能为负数（标准从 0 开始）")
)

// ptz2022Controller 是 sipserver 提供的 2022 查询能力子集。
//
// 为什么不直接往 control.SipController 里加方法：该接口被多个进程共用，
// 扩展它会迫使所有实现同步修改。这里按「能力接口」做类型断言，取不到时
// 说明当前进程没有内嵌信令服务，返回 503 而不是静默成功。
type ptz2022Controller interface {
	QueryPTZPosition(context.Context, *model.Device, string) (*manscdp.PTZPositionResponse, error)
	QueryCruiseTrackList(context.Context, *model.Device, string) (*manscdp.CruiseTrackListResponse, error)
	QueryCruiseTrack(context.Context, *model.Device, string, int) (*manscdp.CruiseTrackResponse, error)
	QueryHomePosition(context.Context, *model.Device, string) (*manscdp.HomePositionResponse, error)
	QuerySDCardStatus(context.Context, *model.Device, string) (*manscdp.SDCardStatusResponse, error)
	ControlPTZPrecise(context.Context, *model.Device, string, *float64, *float64, *float64) error
}

// featureLabel 给能力项起一个可直接展示给用户的中文名。
func featureLabel(f version.Feature) string {
	switch f {
	case version.FeaturePTZPosition:
		return "实时位置"
	case version.FeaturePTZPreciseCtrl:
		return "精准控制"
	case version.FeatureCruiseTrackList, version.FeatureCruiseTrack:
		return "巡航轨迹"
	case version.FeatureHomePosition:
		return "看守位"
	case version.FeatureSDCardStatus:
		return "存储卡状态"
	default:
		return string(f)
	}
}

// EnsurePTZ2022 校验设备是否具备某项 2022 云台能力。
//
// 导出供 tests/gb28181 直接断言（与 BuildPtzCommand 的导出理由一致：
// 让「版本拦截」这条最容易回归的规则可以脱离 SIP/数据库单测）。
func EnsurePTZ2022(dev *model.Device, feature version.Feature) error {
	if dev == nil {
		return ErrGbDeviceNotFound
	}
	if version.Supports(version.Normalize(dev.GbVersion), feature) {
		return nil
	}
	// 明确写出当前版本与所需版本：前端据此提示「该设备为 GB28181-2016」。
	ver := version.Normalize(dev.GbVersion)
	return fmt.Errorf("%w：该设备为 GB28181-%s，不支持「%s」（需要 GB28181-2022，设备编码 %s）",
		ErrGb2022Unsupported, ver, featureLabel(feature), dev.GbDeviceID)
}

// ValidatePTZPreciseReq 校验精准控制请求。
//
// 三个字段都是「可选、但至少一个」：空请求会让设备无动作，
// 前端却收到 200，属于最典型的「按钮点了没反应也不报错」。
func ValidatePTZPreciseReq(req *request.GbPtzPreciseReq) error {
	if req == nil || (req.Pan == nil && req.Tilt == nil && req.Zoom == nil) {
		return ErrGbPtzPreciseEmpty
	}
	for _, v := range []*float64{req.Pan, req.Tilt, req.Zoom} {
		if v == nil {
			continue
		}
		if math.IsNaN(*v) || math.IsInf(*v, 0) {
			return ErrGbPtzPreciseInvalid
		}
	}
	return nil
}

// resolvePTZ2022 解析通道/设备、校验版本并取出 2022 能力控制器。
func (s *GbPtzService) resolvePTZ2022(ctx context.Context, channelID int64, feature version.Feature) (*model.GbChannel, *model.Device, ptz2022Controller, error) {
	ch, dev, err := loadLiveChannelAndDevice(ctx, s.channelRepo, channelID)
	if err != nil {
		return nil, nil, nil, err
	}
	// 先判版本再判信令：2016 设备无论信令是否启动都不可能成功，
	// 优先返回「版本不支持」更贴近真实原因。
	if err := EnsurePTZ2022(dev, feature); err != nil {
		return nil, nil, nil, err
	}
	raw := control.Default()
	if raw == nil {
		return nil, nil, nil, ErrSipServiceUnavailable
	}
	ctl, ok := raw.(ptz2022Controller)
	if !ok {
		// 实现不完整（例如测试替身只实现了 2016 接口）：显式失败，避免空指针。
		return nil, nil, nil, ErrSipServiceUnavailable
	}
	return ch, dev, ctl, nil
}

// mapPTZ2022Err 把 sipserver 的底层错误映射成业务错误。
//
// 必须区分「版本不支持」与「设备超时」：
//   - 版本不支持 → 400，用户需要改设备配置/升级固件；
//   - 设备超时   → 504，用户需要检查设备在线与网络。
//
// 两者混成一个错误会让现场排障无从下手。
func mapPTZ2022Err(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sipserver.ErrVersionUnsupported):
		return fmt.Errorf("%w：%v", ErrGb2022Unsupported, err)
	case errors.Is(err, sipserver.ErrQueryTimeout):
		return fmt.Errorf("%w：%v", ErrGbQueryTimeout, err)
	default:
		return err
	}
}

// PTZPosition 查询 PTZ 精准位置（Pan/Tilt/Zoom）。
func (s *GbPtzService) PTZPosition(ctx context.Context, channelID int64) (*response.GbPtzPositionResp, error) {
	ch, dev, ctl, err := s.resolvePTZ2022(ctx, channelID, version.FeaturePTZPosition)
	if err != nil {
		return nil, err
	}
	resp, err := ctl.QueryPTZPosition(ctx, dev, ch.GbChannelID)
	if err != nil {
		return nil, mapPTZ2022Err(err)
	}
	out := &response.GbPtzPositionResp{
		Pan:                  resp.Pan,
		Tilt:                 resp.Tilt,
		Zoom:                 resp.Zoom,
		HorizontalFieldAngle: resp.HorizontalFieldAngle,
		VerticalFieldAngle:   resp.VerticalFieldAngle,
		MaxViewDistance:      resp.MaxViewDistance,
	}
	return out, nil
}

// Precise 下发 PTZ 精准控制（绝对角度 + 倍率）。
func (s *GbPtzService) Precise(ctx context.Context, channelID int64, req *request.GbPtzPreciseReq) error {
	if err := ValidatePTZPreciseReq(req); err != nil {
		return err
	}
	ch, dev, ctl, err := s.resolvePTZ2022(ctx, channelID, version.FeaturePTZPreciseCtrl)
	if err != nil {
		return err
	}
	return mapPTZ2022Err(ctl.ControlPTZPrecise(ctx, dev, ch.GbChannelID, req.Pan, req.Tilt, req.Zoom))
}

// CruiseTracks 查询巡航轨迹列表。
//
// 设备无巡航能力时标准允许省略 CruiseTrackList 块，这里统一返回空切片，
// 前端可以直接 length 判断，不用区分 nil/空数组。
func (s *GbPtzService) CruiseTracks(ctx context.Context, channelID int64) ([]response.GbCruiseTrackItem, error) {
	ch, dev, ctl, err := s.resolvePTZ2022(ctx, channelID, version.FeatureCruiseTrackList)
	if err != nil {
		return nil, err
	}
	resp, err := ctl.QueryCruiseTrackList(ctx, dev, ch.GbChannelID)
	if err != nil {
		return nil, mapPTZ2022Err(err)
	}
	out := make([]response.GbCruiseTrackItem, 0)
	if resp.CruiseTrackList != nil {
		for _, t := range resp.CruiseTrackList.Tracks {
			out = append(out, response.GbCruiseTrackItem{Number: t.Number, Name: t.Name})
		}
	}
	return out, nil
}

// CruiseTrack 查询单条巡航轨迹的预置点明细。
func (s *GbPtzService) CruiseTrack(ctx context.Context, channelID int64, number int) (*response.GbCruiseTrackDetailResp, error) {
	if number < 0 {
		return nil, ErrGbCruiseTrackNumberInvalid
	}
	ch, dev, ctl, err := s.resolvePTZ2022(ctx, channelID, version.FeatureCruiseTrack)
	if err != nil {
		return nil, err
	}
	resp, err := ctl.QueryCruiseTrack(ctx, dev, ch.GbChannelID, number)
	if err != nil {
		return nil, mapPTZ2022Err(err)
	}
	out := &response.GbCruiseTrackDetailResp{
		Number: resp.Number,
		Name:   resp.Name,
		Points: make([]response.GbCruisePointItem, 0),
	}
	if resp.CruisePointList != nil {
		for _, p := range resp.CruisePointList.Points {
			out.Points = append(out.Points, response.GbCruisePointItem{
				PresetIndex: p.PresetIndex,
				StayTime:    p.StayTime,
				Speed:       p.Speed,
			})
		}
	}
	return out, nil
}

// HomePosition 查询看守位。
func (s *GbPtzService) HomePosition(ctx context.Context, channelID int64) (*response.GbHomePositionResp, error) {
	ch, dev, ctl, err := s.resolvePTZ2022(ctx, channelID, version.FeatureHomePosition)
	if err != nil {
		return nil, err
	}
	resp, err := ctl.QueryHomePosition(ctx, dev, ch.GbChannelID)
	if err != nil {
		return nil, mapPTZ2022Err(err)
	}
	out := &response.GbHomePositionResp{}
	// HomePosition 块缺失 = 设备不支持，不能当成 Enabled=0（已关闭）。
	if resp.HomePosition != nil {
		out.Supported = true
		out.Enabled = resp.HomePosition.Enabled
		out.ResetTime = resp.HomePosition.ResetTime
		out.PresetIndex = resp.HomePosition.PresetIndex
	}
	return out, nil
}

// SDCardStatus 查询存储卡状态。
func (s *GbPtzService) SDCardStatus(ctx context.Context, channelID int64) (*response.GbSDCardStatusResp, error) {
	ch, dev, ctl, err := s.resolvePTZ2022(ctx, channelID, version.FeatureSDCardStatus)
	if err != nil {
		return nil, err
	}
	resp, err := ctl.QuerySDCardStatus(ctx, dev, ch.GbChannelID)
	if err != nil {
		return nil, mapPTZ2022Err(err)
	}
	out := &response.GbSDCardStatusResp{Cards: make([]response.GbSDCardItem, 0)}
	if resp.SDCardStatusInfo != nil {
		out.Num = resp.SDCardStatusInfo.Num
		out.Status = resp.SDCardStatusInfo.Status
		out.FormatProgress = resp.SDCardStatusInfo.FormatProgress
		for _, c := range resp.SDCardStatusInfo.Cards {
			out.Cards = append(out.Cards, response.GbSDCardItem{
				Capacity:  c.Capacity,
				FreeSpace: c.FreeSpace,
				Status:    c.Status,
			})
		}
	}
	return out, nil
}
