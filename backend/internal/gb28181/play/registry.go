// Package play 实现 GB28181 实时点播的编排与会话注册表（设计 §7.1）。
//
// 职责边界：
//   - 本包只做「信令 + 媒体节点编排」：分配 SSRC、发 INVITE、等待 ZLM 建流、
//     拼装播放地址、停止时 BYE + close_streams。
//   - 不接收 RTP、不解析 PS（设计 §2.1 的架构不变量）。
//   - SIP 能力从 control.Default() 注入的 *sipserver.Service 取用，不重写信令层。
//
// 为什么注册表放在这里而不是 SIP 的 session.Manager：
// session.Manager 以 Call-ID / 通道国标编码为索引，服务的是 SIP 会话生命周期；
// 业务侧还需要按「数据库通道主键」和「ZLM 流名」查找，并携带媒体节点与播放地址。
// 两者语义不同，混用会让停止播放时不得不再查一次数据库。
package play

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// 编排层哨兵错误。业务层负责把它们映射成项目统一的业务错误与 HTTP 状态码。
var (
	// ErrSipUnavailable SIP 信令服务未启动（control.Default() 为 nil，或实现类型不符）。
	ErrSipUnavailable = errors.New("SIP 信令服务未启动，无法发起点播/语音指令")
	// ErrMediaServerUnavailable 数据库中没有可用的流媒体节点。
	ErrMediaServerUnavailable = errors.New("流媒体节点不可用，请检查 ac_gb_media_server 的默认节点配置")
	// ErrStreamNotReady 已发出 INVITE，但 ZLM 在超时窗口内没有收到媒体流。
	ErrStreamNotReady = errors.New("设备未在限定时间内推送媒体流")
	// ErrStreamNotFound 注册表中没有对应的播放会话。
	ErrStreamNotFound = errors.New("播放会话不存在或已结束")
	// ErrDeviceOffline 设备不在线，不能发起点播。
	ErrDeviceOffline = errors.New("设备离线，无法发起实时点播")
)

// Stream 是一路进行中的实时播放会话。
//
// 同时保存 MediaServer 实体：停止播放与状态查询时不必再查数据库，
// 也避免「默认节点在播放期间被改掉」导致关错流。
type Stream struct {
	ChannelID   int64  // ac_gb_channel.id（路由参数）
	GbChannelID string // 通道国标编码（INVITE 的 Request-URI 用户部分）
	DeviceID    int64  // ac_device.id
	GbDeviceID  string // 设备国标编码

	SessionID string // SIP Call-ID，返回给前端作为 stream_id
	App       string // ZLM app，恒为 rtp
	Stream    string // ZLM 流名 = %08X(ssrc) 大写
	SSRC      string // 平台分配的 10 位 SSRC
	Protocol  string // 前端请求的协议（webrtc/flv/hls），仅作记录

	MediaServerID string
	MediaServer   *model.GbMediaServer
	HTTPBase      string
	URLs          mediaserver.PlayURLs

	CreatedAt time.Time
}

// Registry 是进程内的播放会话注册表，并发安全。
//
// 「单写者」指同一通道的 Play/Stop 由 Orchestrator 的互斥锁串行化；
// 这里的读写锁只保护索引本身。返回给调用方的都是副本，避免外部修改内部状态。
type Registry struct {
	mu        sync.RWMutex
	byChannel map[int64]*Stream
	byStream  map[string]*Stream
	bySession map[string]*Stream
}

// NewRegistry 构造一个空注册表。
func NewRegistry() *Registry {
	return &Registry{
		byChannel: map[int64]*Stream{},
		byStream:  map[string]*Stream{},
		bySession: map[string]*Stream{},
	}
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

// DefaultRegistry 返回进程级注册表。
//
// 业务 Service 的构造函数按契约无参，因此共享一个进程级实例；
// 测试可以显式构造 NewRegistry() 注入 Orchestrator 以避免互相污染。
func DefaultRegistry() *Registry {
	defaultRegistryOnce.Do(func() { defaultRegistry = NewRegistry() })
	return defaultRegistry
}

// Register 登记一路播放会话。同一通道再次登记时会覆盖旧会话索引。
//
// 覆盖而不是报错：点播重入（前端刷新、用户重复点击）时，旧会话应由编排层
// 先停止；这里做兜底，避免旧索引残留导致「通道永远显示在播」。
func (r *Registry) Register(s *Stream) error {
	if s == nil {
		return errors.New("播放会话不能为空")
	}
	if strings.TrimSpace(s.Stream) == "" {
		return errors.New("播放会话缺少 ZLM 流名")
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	cp := *s

	r.mu.Lock()
	defer r.mu.Unlock()

	// 同一通道只保留一路；清理旧会话的全部索引。
	if old, ok := r.byChannel[cp.ChannelID]; ok && old.Stream != cp.Stream {
		r.removeLocked(old)
	}
	item := &cp
	r.byChannel[item.ChannelID] = item
	r.byStream[item.Stream] = item
	if item.SessionID != "" {
		r.bySession[item.SessionID] = item
	}
	return nil
}

// Unregister 按 ZLM 流名移除会话及其全部索引。
func (r *Registry) Unregister(stream string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if item, ok := r.byStream[stream]; ok {
		r.removeLocked(item)
	}
}

// removeLocked 删除一个会话的所有索引。调用方必须持有写锁。
func (r *Registry) removeLocked(s *Stream) {
	delete(r.byStream, s.Stream)
	if cur, ok := r.byChannel[s.ChannelID]; ok && cur == s {
		delete(r.byChannel, s.ChannelID)
	}
	if s.SessionID != "" {
		if cur, ok := r.bySession[s.SessionID]; ok && cur == s {
			delete(r.bySession, s.SessionID)
		}
	}
}

// GetByChannel 按通道主键查找。
func (r *Registry) GetByChannel(channelID int64) (*Stream, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byChannel[channelID]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

// GetByStream 按 ZLM 流名查找。
func (r *Registry) GetByStream(stream string) (*Stream, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byStream[stream]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

// GetBySessionID 按 SIP Call-ID 查找（停止播放时前端回传的 stream_id）。
func (r *Registry) GetBySessionID(sessionID string) (*Stream, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.bySession[sessionID]
	if !ok {
		return nil, false
	}
	cp := *s
	return &cp, true
}

// List 返回全部会话的快照（顺序不保证）。
func (r *Registry) List() []*Stream {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Stream, 0, len(r.byStream))
	for _, s := range r.byStream {
		cp := *s
		out = append(out, &cp)
	}
	return out
}
