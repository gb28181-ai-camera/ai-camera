package hook

import (
	"context"
	"fmt"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
)

// MediaServerStore 抽象「更新媒体节点在线状态」，便于单测注入。
//
// 只暴露 hook 层真正需要的写操作，避免让 hook 包直接承担节点 CRUD 的职责。
type MediaServerStore interface {
	// MarkOnline 把节点标记为在线并刷新最近心跳时间。
	MarkOnline(ctx context.Context, serverID string, at time.Time) error
}

// DBMediaServerStore 是 MediaServerStore 的数据库实现（表 ac_gb_media_server）。
//
// 直接用 bootstrap.DB() 而不改 repository 包：hook 是基础设施层，
// 只做一次字段级更新，复用 UpdateFields 会引入不必要的依赖与审计字段变更。
type DBMediaServerStore struct{}

// MarkOnline 更新 online / last_keepalive_dt。
func (DBMediaServerStore) MarkOnline(ctx context.Context, serverID string, at time.Time) error {
	db := bootstrap.DB()
	if db == nil {
		return fmt.Errorf("数据库未初始化，无法更新媒体节点 %s", serverID)
	}
	return db.WithContext(ctx).Model(&model.GbMediaServer{}).
		Where("server_id = ?", serverID).
		Updates(map[string]any{
			"online":            true,
			"last_keepalive_dt": at,
			"updated_dt":        at,
		}).Error
}

// DefaultSink 是本期使用的默认业务实现。
//
// 本期只做「节点在线状态 + 活跃流内存表」两件事：
//   - 播放编排、云端录像落库、语音回推状态维护分别属于 Wave 2 的模块，
//     这里只留下明确的事件入口与 TODO，不提前实现半成品。
type DefaultSink struct {
	store  MediaServerStore
	active *ActiveStreams
	now    func() time.Time
}

// NewDefaultSink 使用数据库存储与新的活跃流表构造 Sink。
func NewDefaultSink() *DefaultSink {
	return &DefaultSink{
		store:  DBMediaServerStore{},
		active: NewActiveStreams(),
		now:    time.Now,
	}
}

// NewDefaultSinkWithStore 供单测注入替代存储。
func NewDefaultSinkWithStore(store MediaServerStore) *DefaultSink {
	s := NewDefaultSink()
	s.store = store
	return s
}

// ActiveStreams 返回活跃流表（编排层只读查询用）。
func (s *DefaultSink) ActiveStreams() *ActiveStreams {
	if s == nil {
		return nil
	}
	return s.active
}

// OnServerKeepalive 处理节点心跳：刷新在线状态与最近心跳时间。
func (s *DefaultSink) OnServerKeepalive(ctx context.Context, event *MediaServerEvent) error {
	if event == nil || event.MediaServerID == "" {
		utils.Warn("[ZLM-HOOK] 心跳缺少 mediaServerId，忽略")
		return nil
	}
	at := s.now()
	if err := s.store.MarkOnline(ctx, event.MediaServerID, at); err != nil {
		utils.Warn("[ZLM-HOOK] 更新节点心跳失败",
			zap.String("mediaServerId", event.MediaServerID), zap.Error(err))
		return err
	}
	utils.Debug("[ZLM-HOOK] 节点心跳",
		zap.String("mediaServerId", event.MediaServerID), zap.Time("at", at))
	return nil
}

// OnServerStarted 处理节点启动：ZLM 重启后立即把节点标记为在线。
//
// 为什么与心跳同处理：ZLM 启动后不一定马上发首包心跳，而编排层需要尽快
// 知道节点恢复；heartbeat 与 started 都是「节点活着」的等价证据。
func (s *DefaultSink) OnServerStarted(ctx context.Context, event *MediaServerEvent) error {
	if event == nil || event.MediaServerID == "" {
		utils.Warn("[ZLM-HOOK] 节点启动回调缺少 mediaServerId，忽略")
		return nil
	}
	at := s.now()
	if err := s.store.MarkOnline(ctx, event.MediaServerID, at); err != nil {
		utils.Warn("[ZLM-HOOK] 节点启动后更新在线状态失败",
			zap.String("mediaServerId", event.MediaServerID), zap.Error(err))
		return err
	}
	utils.Info("[ZLM-HOOK] ZLM 节点已启动",
		zap.String("mediaServerId", event.MediaServerID), zap.String("ip", event.IP))
	return nil
}

// OnPublish 处理推流鉴权。
//
// ⚠️ 本期先放行。但这是 ZLM 侧唯一的兜底鉴权点，Wave 2 必须补齐：
// 只允许 app=rtp 且 stream 为「平台自己分配过的 hex SSRC」的推流；
// 未知 SSRC 一律拒绝，否则任何人都能往 10000 端口打流占用资源。
//
// 判断依据应来自 Redis 的 SSRC 注册表（{deviceId}_{channelId} → hex SSRC），
// 不能只看 stream 的格式 —— 格式正确不代表是平台发起的会话。
func (s *DefaultSink) OnPublish(_ context.Context, event *PublishEvent) (bool, error) {
	if event == nil {
		return false, nil
	}
	utils.Debug("[ZLM-HOOK] 推流鉴权（本期放行）",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("app", event.App),
		zap.String("stream", event.Stream),
		zap.String("schema", event.Schema))
	return true, nil
}

// OnPlay 处理播放鉴权。
//
// 本期放行；Wave 2 需按 Redis 会话表中的 (app, stream, callId) 校验，
// 防止未授权的播放地址被外部分享后长期可用。
func (s *DefaultSink) OnPlay(_ context.Context, event *PlayEvent) (bool, error) {
	if event == nil {
		return false, nil
	}
	utils.Debug("[ZLM-HOOK] 播放鉴权（本期放行）",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("app", event.App),
		zap.String("stream", event.Stream),
		zap.String("schema", event.Schema))
	return true, nil
}

// OnStreamChanged 维护活跃流内存表。
//
// 明确边界：**不在这里写 ac_gb_cloud_record**。ZLM 的 on_stream_changed 只表示
// 流注册/注销；录像索引必须由 on_record_mp4（文件真正落盘后）产生，
// 在流注册时写索引会留下大量 0 字节/不完整的记录。
func (s *DefaultSink) OnStreamChanged(_ context.Context, event *StreamChangedEvent) error {
	if event == nil || event.App == "" || event.Stream == "" {
		return nil
	}
	if event.Regist {
		s.active.Set(ActiveStream{
			App:           event.App,
			Stream:        event.Stream,
			Schema:        event.Schema,
			MediaServerID: event.MediaServerID,
			OriginURL:     event.OriginURL,
			Tracks:        event.Tracks,
			Since:         s.now(),
		})
		utils.Info("[ZLM-HOOK] 流注册",
			zap.String("mediaServerId", event.MediaServerID),
			zap.String("app", event.App),
			zap.String("stream", event.Stream),
			zap.String("schema", event.Schema),
			zap.Int("tracks", len(event.Tracks)))
		return nil
	}

	s.active.Remove(event.App, event.Stream)
	utils.Info("[ZLM-HOOK] 流注销",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("app", event.App),
		zap.String("stream", event.Stream),
		zap.String("schema", event.Schema))
	return nil
}

// OnStreamNoneReader 处理无人观看事件。
//
// 只在内存表里没有播放者时不做事；本期不主动 close 流（响应 close=false）。
// Wave 2 的按需收流策略会在这里查询会话表后再决定。
func (s *DefaultSink) OnStreamNoneReader(_ context.Context, event *StreamEvent) error {
	if event == nil {
		return nil
	}
	utils.Info("[ZLM-HOOK] 流无人观看",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("app", event.App),
		zap.String("stream", event.Stream),
		zap.String("schema", event.Schema))
	return nil
}

// OnStreamNotFound 处理流不存在事件（按需拉流的触发点，本期只记录）。
func (s *DefaultSink) OnStreamNotFound(_ context.Context, event *StreamEvent) error {
	if event == nil {
		return nil
	}
	utils.Info("[ZLM-HOOK] 流未找到",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("app", event.App),
		zap.String("stream", event.Stream),
		zap.String("schema", event.Schema))
	return nil
}

// OnSendRTPStopped 处理 RTP 发送被动结束（语音回推/级联推流）。
//
// Wave 2 需要据此把 ac_gb_stream_push.pushing 置为 false、
// 关闭 ac_voice_session 中对应的会话；本期只记录日志。
func (s *DefaultSink) OnSendRTPStopped(_ context.Context, event *SendRTPStoppedEvent) error {
	if event == nil {
		return nil
	}
	utils.Info("[ZLM-HOOK] RTP 发送已停止",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("app", event.App),
		zap.String("stream", event.Stream))
	return nil
}

// OnRTPServerTimeout 处理 RTP 收流超时（ZLM timeoutSec 到期）。
func (s *DefaultSink) OnRTPServerTimeout(_ context.Context, event *StreamEvent) error {
	if event == nil {
		return nil
	}
	// 单端口模式下没有流级 RTP server，但 ZLM 仍会以 stream_id=SSRC 的形式回调；
	// 用 stream_id 兜底清理活跃流，避免内存表留下「僵尸流」。
	stream := event.Stream
	if stream == "" {
		stream = event.StreamID
	}
	if event.App != "" && stream != "" {
		s.active.Remove(event.App, stream)
	}
	utils.Info("[ZLM-HOOK] RTP 收流超时",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("stream_id", event.StreamID),
		zap.String("ssrc", event.SSRC),
		zap.Int("local_port", event.LocalPort))
	return nil
}

// OnRecordMP4 处理录像落盘完成。
//
// 这里才是写 ac_gb_cloud_record 的正确时机（文件已完整落盘）。
// 但本期缺少 (app,stream) → (device_id,channel_id) 的映射（SSRC 注册表在 Wave 2 落地），
// 强行写索引会得到一堆无归属的记录，因此只记录日志并留下 TODO。
func (s *DefaultSink) OnRecordMP4(_ context.Context, event *RecordMP4Event) error {
	if event == nil {
		return nil
	}
	utils.Info("[ZLM-HOOK] 录像落盘完成",
		zap.String("mediaServerId", event.MediaServerID),
		zap.String("app", event.App),
		zap.String("stream", event.Stream),
		zap.String("file_path", event.FilePath),
		zap.Int64("file_size", event.FileSize),
		zap.Float64("time_len", event.TimeLen))
	// TODO(Wave 2): 按 SSRC 注册表反查 device/channel，写入 ac_gb_cloud_record。
	return nil
}
