// 本文件是移动端国标视频（GB/T 28181）的**薄封装**服务。
//
// 为什么 App 要单独薄封装，而不是让移动端直接调用 /web/gb 的 Handler：
//   - /web/gb 的 Handler 绑定的是管理后台的鉴权与分页请求，移动端需要自己的
//     JWT 上下文（c.GetInt64("user_id")）与 DataScope 构造入口；
//   - 管理面响应带着 sip_password、password、sdp_ip 等配置/排障字段，移动端
//     只做看视频、控云台、查报警，必须裁剪；
//   - 移动端契约（§10.12）是独立冻结面，不能因为 Web 结构演进被动改变。
//
// 但**业务逻辑一行都不重复**：本文件只做「参数校验 → 委托 gb28181 service →
// 字段裁剪」。播放/云台/回放/语音的 SIP 编排、错误语义、幂等性全部由
// internal/service/gb28181 负责，与 /web/gb 共用同一份实现。
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// 移动端参数校验错误。
//
// 与 gb28181 包的 ErrGbPtzSpeedInvalid 等保持同义；这里额外定义是为了让 App
// 在**访问数据库之前**就拒绝非法方向（例如 "diagonal"），避免「通道不存在」
// 掩盖了「参数写错」这一更直接的原因。
var (
	// ErrAppGbPtzDirectionInvalid 方向不在契约枚举内。
	ErrAppGbPtzDirectionInvalid = errors.New("云台方向非法，仅支持 up/down/left/right/up-left/up-right/down-left/down-right/stop")
	// ErrAppGbPtzSpeedInvalid 速度越界（契约钉死 1–8，0 表示使用默认值 5）。
	ErrAppGbPtzSpeedInvalid = errors.New("云台速度必须在 1-8 之间")
)

// appPtzDirections 契约 §15.2 冻结的九种方向。
var appPtzDirections = map[string]struct{}{
	"up": {}, "down": {}, "left": {}, "right": {},
	"up-left": {}, "up-right": {}, "down-left": {}, "down-right": {},
	"stop": {},
}

// AppGbService 移动端国标视频服务（无状态，可安全复用）。
type AppGbService struct{}

// NewAppGbService 构造移动端国标视频服务。
func NewAppGbService() *AppGbService { return &AppGbService{} }

// ── 委托的领域服务（每次调用新建，与其它模块保持一致的构造方式）──

func (s *AppGbService) deviceSvc() *gbsvc.GbDeviceService     { return gbsvc.NewGbDeviceService() }
func (s *AppGbService) channelSvc() *gbsvc.GbChannelService   { return gbsvc.NewGbChannelService() }
func (s *AppGbService) playSvc() *gbsvc.GbPlayService         { return gbsvc.NewGbPlayService() }
func (s *AppGbService) ptzSvc() *gbsvc.GbPtzService           { return gbsvc.NewGbPtzService() }
func (s *AppGbService) playbackSvc() *gbsvc.GbPlaybackService { return gbsvc.NewGbPlaybackService() }
func (s *AppGbService) voiceSvc() *gbsvc.GbVoiceService       { return gbsvc.NewGbVoiceService() }
func (s *AppGbService) alarmSvc() *gbsvc.GbAlarmService       { return gbsvc.NewGbAlarmService() }

// ════════════════════════════════════════════════════════════════
// 设备 / 通道
// ════════════════════════════════════════════════════════════════

// PageDevices 分页查询已建档国标设备。
//
// 复用 GbDeviceService.Page：它已经限定 gb_device_id 非空并施加 DataScope；
// 本方法只把管理面结构裁剪成移动端结构（不含任何 SIP 接入配置）。
func (s *AppGbService) PageDevices(ctx context.Context, req *request.PageGbDeviceReq, ds *utils.DataScope) (*response.AppPageGbDeviceResp, error) {
	if req == nil {
		req = &request.PageGbDeviceReq{}
	}
	page, err := s.deviceSvc().Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}

	items := make([]response.AppGbDeviceItem, 0, len(page.Items))
	for _, d := range page.Items {
		items = append(items, response.AppGbDeviceItem{
			ID:             d.ID,
			Name:           d.Name,
			DeviceCode:     d.DeviceCode,
			GbDeviceID:     d.GbDeviceID,
			RegionID:       d.RegionID,
			RegionName:     d.RegionName,
			Location:       d.Location,
			Status:         d.Status,
			RegisterStatus: d.RegisterStatus,
			ChannelCount:   d.ChannelCount,
			Active:         d.Active,
		})
	}
	return &response.AppPageGbDeviceResp{
		Items:    items,
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
	}, nil
}

// PageChannels 分页查询国标通道（可按设备过滤）。
//
// 复用 GbChannelService.Page：DataScope 施加在 ac_gb_channel.dept_id 上；
// 本方法裁剪掉设备侧 password 等管理字段。
func (s *AppGbService) PageChannels(ctx context.Context, req *request.PageGbChannelReq, ds *utils.DataScope) (*response.AppPageGbChannelResp, error) {
	if req == nil {
		req = &request.PageGbChannelReq{}
	}
	page, err := s.channelSvc().Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}

	items := make([]response.AppGbChannelItem, 0, len(page.Items))
	for _, c := range page.Items {
		items = append(items, response.AppGbChannelItem{
			ID:              c.ID,
			DeviceID:        c.DeviceID,
			DeviceName:      c.DeviceName,
			GbDeviceID:      c.GbDeviceID,
			GbChannelID:     c.GbChannelID,
			Name:            c.Name,
			Status:          c.Status,
			ChannelType:     c.ChannelType,
			PtzType:         c.PtzType,
			HasAudio:        c.HasAudio,
			EnableBroadcast: c.EnableBroadcast,
			RegionName:      c.RegionName,
			GroupName:       c.GroupName,
		})
	}
	return &response.AppPageGbChannelResp{
		Items:    items,
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
	}, nil
}

// ════════════════════════════════════════════════════════════════
// 实时播放 / 抓图
// ════════════════════════════════════════════════════════════════

// Play 发起实时点播。
//
// 复用 GbPlayService.Play：SSRC 分配、INVITE、等待 ZLM 建流、同通道复用
// 全部由领域层处理；移动端只拿地址与 stream_id。
func (s *AppGbService) Play(ctx context.Context, channelID int64, req *request.GbPlayReq) (*response.AppGbPlayResp, error) {
	out, err := s.playSvc().Play(ctx, channelID, req)
	if err != nil {
		return nil, err
	}
	return &response.AppGbPlayResp{
		App:      out.App,
		Stream:   out.Stream,
		HTTPBase: out.HTTPBase,
		WebRTC:   out.WebRTC,
		FLV:      out.FLV,
		HLS:      out.HLS,
		StreamID: out.StreamID,
	}, nil
}

// StopPlay 停止实时点播。
//
// 复用 GbPlayService.StopPlay：stream_id 为空时按通道停止，且会校验
// stream_id 确实属于该通道，避免误停其它通道。
func (s *AppGbService) StopPlay(ctx context.Context, channelID int64, streamID string) error {
	return s.playSvc().StopPlay(ctx, channelID, streamID)
}

// Snapshot 抓图。
//
// 复用 GbPlayService.Snapshot：未开播时域服务会自动先点播再抓图。
func (s *AppGbService) Snapshot(ctx context.Context, channelID int64) (*response.AppGbSnapshotResp, error) {
	out, err := s.playSvc().Snapshot(ctx, channelID)
	if err != nil {
		return nil, err
	}
	return &response.AppGbSnapshotResp{
		URL:       out.URL,
		FileName:  out.FileName,
		FileSize:  out.FileSize,
		CaptureDT: out.CaptureDT,
	}, nil
}

// ════════════════════════════════════════════════════════════════
// 云台
// ════════════════════════════════════════════════════════════════

// Ptz 云台方向/动作控制。
//
// 先做纯参数校验（方向枚举、速度 1–8、direction/action 互斥），再委托
// GbPtzService.Control 走带外 SIP MESSAGE 下发指令。
// 校验前置的意义：参数错误应当稳定返回 400，而不是先查库、查 SIP 后
// 才以 404/503 的形式暴露，掩盖真正的原因。
func (s *AppGbService) Ptz(ctx context.Context, channelID int64, req *request.GbPtzReq) error {
	if req == nil {
		return gbsvc.ErrGbPtzCommandMissing
	}
	direction := strings.ToLower(strings.TrimSpace(req.Direction))
	action := strings.ToLower(strings.TrimSpace(req.Action))

	if direction != "" && action != "" {
		return gbsvc.ErrGbPtzDirectionConflict
	}
	if direction == "" && action == "" {
		return gbsvc.ErrGbPtzCommandMissing
	}
	if direction != "" {
		if _, ok := appPtzDirections[direction]; !ok {
			return fmt.Errorf("%w（收到 %q）", ErrAppGbPtzDirectionInvalid, req.Direction)
		}
	}
	// 0 表示未填写，由领域层按契约缺省为 5；显式非法值必须拒绝，
	// 不能静默夹到 1 或 8。
	if req.Speed != 0 && (req.Speed < 1 || req.Speed > 8) {
		return fmt.Errorf("%w（收到 %d）", ErrAppGbPtzSpeedInvalid, req.Speed)
	}
	if direction != "" {
		req.Direction = direction
	}
	if action != "" {
		req.Action = action
	}
	return s.ptzSvc().Control(ctx, channelID, req)
}

// ════════════════════════════════════════════════════════════════
// 录像查询 / 回放
// ════════════════════════════════════════════════════════════════

// QueryRecord 检索设备侧录像。
//
// 复用 GbPlaybackService.QueryRecord：设备无响应时返回 Timeout=true 而非
// HTTP 错误 —— 这是业务结果，移动端需要区分「没有录像」与「设备没回」。
func (s *AppGbService) QueryRecord(ctx context.Context, channelID int64, req *request.GbRecordQueryReq) (*response.AppGbRecordQueryResp, error) {
	out, err := s.playbackSvc().QueryRecord(ctx, channelID, req)
	if err != nil {
		return nil, err
	}
	items := make([]response.AppGbRecordItem, 0, len(out.Items))
	for _, it := range out.Items {
		items = append(items, response.AppGbRecordItem{
			ChannelID: it.ChannelID,
			Name:      it.Name,
			StartTime: it.StartTime,
			EndTime:   it.EndTime,
			TimeLen:   it.TimeLen,
		})
	}
	return &response.AppGbRecordQueryResp{
		ChannelID: out.ChannelID,
		Total:     out.Total,
		Timeout:   out.Timeout,
		Items:     items,
	}, nil
}

// StartPlayback 发起回放。
//
// 复用 GbPlaybackService.StartPlayback：时间窗校验、INVITE、媒体节点解析
// 都在领域层完成。
func (s *AppGbService) StartPlayback(ctx context.Context, req *request.GbPlaybackReq) (*response.AppGbPlaybackResp, error) {
	out, err := s.playbackSvc().StartPlayback(ctx, req)
	if err != nil {
		return nil, err
	}
	return &response.AppGbPlaybackResp{
		StreamID:  out.StreamID,
		App:       out.App,
		Stream:    out.Stream,
		HTTPBase:  out.HTTPBase,
		WebRTC:    out.WebRTC,
		FLV:       out.FLV,
		HLS:       out.HLS,
		ChannelID: out.ChannelID,
		StartTime: out.StartTime,
		EndTime:   out.EndTime,
	}, nil
}

// ControlPlayback 回放控制（pause/resume/seek/speed）。
//
// 复用 GbPlaybackService.Control：走**带内 SIP INFO + MANSRTSP**，
// 与云台的带外 MESSAGE 不是同一条路径。
func (s *AppGbService) ControlPlayback(ctx context.Context, req *request.GbPlaybackControlReq) error {
	return s.playbackSvc().Control(ctx, req)
}

// StopPlayback 停止回放。复用 GbPlaybackService.StopPlayback（幂等）。
func (s *AppGbService) StopPlayback(ctx context.Context, streamID string) error {
	return s.playbackSvc().StopPlayback(ctx, streamID)
}

// ════════════════════════════════════════════════════════════════
// 语音对讲
// ════════════════════════════════════════════════════════════════

// StartTalk 开始语音对讲。
//
// userID 来自 App JWT，由领域层写入 ac_voice_session 供追溯。
// agentEnabled 为 App 的「希望接入语音 Agent」意愿；控制面不可达时领域层
// 会自动降级为纯对讲并通过响应回显 agent_enabled=false。
func (s *AppGbService) StartTalk(ctx context.Context, channelID, userID int64, agentEnabled bool) (*response.AppGbVoiceSessionResp, error) {
	out, err := s.voiceSvc().StartTalk(ctx, channelID, userID, agentEnabled)
	if err != nil {
		return nil, err
	}
	return &response.AppGbVoiceSessionResp{
		SessionID:    out.SessionID,
		Mode:         out.Mode,
		AgentEnabled: out.AgentEnabled,
		SSRC:         out.SSRC,
		AudioCodec:   out.AudioCodec,
		StartDT:      out.StartDT,
	}, nil
}

// StopTalk 结束语音对讲。
//
// session_id 可省略：App 的「松手即停」通常只带 channelId，此时按通道
// 查找进行中的对讲会话（StopTalkByChannel），避免前端为了停止再存一次状态。
func (s *AppGbService) StopTalk(ctx context.Context, channelID int64, sessionID string) error {
	if v := strings.TrimSpace(sessionID); v != "" {
		return s.voiceSvc().StopTalk(ctx, v)
	}
	return s.voiceSvc().StopTalkByChannel(ctx, channelID)
}

// ════════════════════════════════════════════════════════════════
// 国标报警
// ════════════════════════════════════════════════════════════════

// PageAlarms 分页查询国标报警（设备主动上报的 MANSCDP Alarm）。
//
// 复用 GbAlarmService.Page：DataScope 施加在 ac_gb_alarm.dept_id 上；
// 本方法只裁剪为移动端列表所需字段。
func (s *AppGbService) PageAlarms(ctx context.Context, req *request.PageGbAlarmReq, ds *utils.DataScope) (*response.AppPageGbAlarmResp, error) {
	if req == nil {
		req = &request.PageGbAlarmReq{}
	}
	page, err := s.alarmSvc().Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.AppGbAlarmItem, 0, len(page.Items))
	for _, a := range page.Items {
		items = append(items, response.AppGbAlarmItem{
			ID:               a.ID,
			DeviceID:         a.DeviceID,
			ChannelID:        a.ChannelID,
			DeviceName:       a.DeviceName,
			ChannelName:      a.ChannelName,
			AlarmPriority:    a.AlarmPriority,
			AlarmType:        a.AlarmType,
			AlarmTime:        a.AlarmTime,
			AlarmDescription: a.AlarmDescription,
			SnapPath:         a.SnapPath,
			Handled:          a.Handled,
		})
	}
	return &response.AppPageGbAlarmResp{
		Items:    items,
		Total:    page.Total,
		Page:     page.Page,
		PageSize: page.PageSize,
	}, nil
}
