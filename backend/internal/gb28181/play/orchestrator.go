package play

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/sdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/session"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
)

// 编排参数。与 WVP 的经验值对齐：
//   - 5s 等待 ZLM 建流：设备收到 INVITE 后通常 1–3s 内开始推流；
//     无限等待会让 HTTP 请求挂死，前端表现为「转圈到超时」。
//   - 250ms 轮询：足够及时，且不会给 ZLM 的 REST 接口造成压力。
const (
	defaultReadyTimeout = 5 * time.Second
	defaultPollInterval = 250 * time.Millisecond
	cleanupTimeout      = 5 * time.Second
)

// Result 是一次成功点播的编排结果。
type Result struct {
	App           string
	Stream        string
	SSRC          string
	HTTPBase      string
	URLs          mediaserver.PlayURLs
	MediaServerID string
	StreamID      string
}

// Status 是某通道当前的点播状态。
//
// Playing 以注册表为准（「平台认为在播」）；Ready 是 ZLM 的实时判断
// （「媒体真的在线」）。两者不一致通常说明设备停了推流但 BYE 没发出来，
// 或 ZLM 不可达 —— 这两个字段一起返回，排障时一眼能看出来。
type Status struct {
	ChannelID     int64
	Playing       bool
	Ready         bool
	App           string
	Stream        string
	SSRC          string
	HTTPBase      string
	URLs          mediaserver.PlayURLs
	MediaServerID string
	StreamID      string
	StartDT       *time.Time
}

// Orchestrator 负责实时点播的全链路编排。
//
// mu 把同一进程内的 Play/Stop 串行化：
//   - SSRC 分配虽然是线程安全的，但「先查注册表再 INVITE」不是原子的，
//     并发点播同一通道会发出两个 INVITE，设备侧后一个会顶掉前一个；
//   - 单写者语义也让注册表的索引维护更简单（设计 §2.2 的 SSRC 唯一性前提）。
type Orchestrator struct {
	registry *Registry
	mu       sync.Mutex

	readyTimeout time.Duration
	pollInterval time.Duration
}

// NewOrchestrator 构造使用进程级注册表的编排器。
func NewOrchestrator() *Orchestrator {
	return NewOrchestratorWithRegistry(DefaultRegistry())
}

// NewOrchestratorWithRegistry 使用指定注册表构造编排器（测试隔离用）。
func NewOrchestratorWithRegistry(r *Registry) *Orchestrator {
	if r == nil {
		r = NewRegistry()
	}
	return &Orchestrator{
		registry:     r,
		readyTimeout: defaultReadyTimeout,
		pollInterval: defaultPollInterval,
	}
}

var (
	defaultOrchestratorOnce sync.Once
	defaultOrchestrator     *Orchestrator
)

// DefaultOrchestrator 返回进程级编排器。
func DefaultOrchestrator() *Orchestrator {
	defaultOrchestratorOnce.Do(func() { defaultOrchestrator = NewOrchestrator() })
	return defaultOrchestrator
}

// Registry 暴露注册表（业务层按通道查当前会话、停止播放时取 stream_id）。
func (o *Orchestrator) Registry() *Registry { return o.registry }

// SIPService 取出已注入的 SIP 服务实现。
//
// control.Default() 只暴露了 Query/Subscribe/SendMessage 四个方法（给 CRUD 层用），
// 点播还需要 SendInvite/AllocateSSRC/Sessions 等完整能力，因此这里做一次
// 具体类型断言。断言失败等价于「信令服务未启动」—— 必须显式报错，不能静默成功。
func SIPService() (*sipserver.Service, error) {
	c := control.Default()
	if c == nil {
		return nil, ErrSipUnavailable
	}
	svc, ok := c.(*sipserver.Service)
	if !ok || svc == nil {
		return nil, ErrSipUnavailable
	}
	return svc, nil
}

// Play 发起一路实时点播，返回 ZLM 播放地址。
//
// 流程（设计 §7.1）：
//  1. 选媒体节点（设备绑定 → 默认节点）；
//  2. 若该通道已有会话且 ZLM 确认流在线，直接复用（前端重复点击/刷新不重复 INVITE）；
//  3. 分配 SSRC → 构造 SDP → INVITE → ACK；
//  4. 短轮询 IsStreamReady，最多 readyTimeout；
//  5. 返回 WebRTC / FLV / HLS 地址。
//
// 任何一步失败都会回滚：BYE + close_streams + ReleaseSSRC，避免泄漏 SSRC 与僵尸流。
func (o *Orchestrator) Play(ctx context.Context, dev *model.Device, ch *model.GbChannel, protocol string) (*Result, error) {
	if dev == nil {
		return nil, errors.New("设备信息为空")
	}
	if ch == nil {
		return nil, errors.New("通道信息为空")
	}
	if strings.TrimSpace(ch.GbChannelID) == "" {
		return nil, errors.New("通道缺少国标编码，无法点播")
	}
	// TCP-ACTIVE 在单端口模式下不可用：必须在分配 SSRC / 发 INVITE **之前**拒绝，
	// 否则会白占一个 SSRC 并让设备进入「已应答但平台不来连」的半死状态。
	if sipserver.StreamModeOf(dev) == sdp.TransportTCPActive {
		return nil, ErrTCPActiveUnsupported
	}

	svc, err := SIPService()
	if err != nil {
		return nil, err
	}
	client, ms, err := ResolveMediaServer(ctx, dev)
	if err != nil {
		return nil, err
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// 复用已有会话：只有 ZLM 明确说在线才复用。
	// ZLM 不可达时不拆已有会话 —— 拆了也建不回来，只会让本可播放的流中断。
	if st, ok := o.registry.GetByChannel(ch.ID); ok {
		ready, rerr := client.IsStreamReady(ctx, st.App, st.Stream, "", "")
		if rerr == nil && ready {
			return resultOf(st), nil
		}
		if rerr != nil {
			return nil, fmt.Errorf("%w：%v", ErrMediaServerUnavailable, rerr)
		}
		// 注册表有记录但 ZLM 没有流：上一路已经死了，先清理再重新 INVITE。
		o.stopStream(context.Background(), svc, client, st)
	}

	ssrcStr, stream, err := svc.AllocateSSRC()
	if err != nil {
		return nil, fmt.Errorf("分配 SSRC 失败：%w", err)
	}

	sdpBody := svc.BuildPlaySDP(dev, ssrcStr, sdp.SessionNamePlay)
	target := sipserver.CallTarget{
		Device:      dev,
		ChannelID:   ch.GbChannelID,
		SessionName: sdp.SessionNamePlay,
		Subject:     FormatSubject(ch.GbChannelID, ssrcStr, svc.Domain()),
		ContentType: sipserver.ContentTypeSDP,
		SSRC:        ssrcStr,
	}

	dlg, err := svc.SendInvite(ctx, target, []byte(sdpBody), "")
	if err != nil {
		svc.ReleaseSSRC(ssrcStr)
		return nil, fmt.Errorf("发起实时点播失败：%w", err)
	}

	if err := o.waitReady(ctx, client, stream); err != nil {
		o.abort(svc, client, dlg, ssrcStr, stream)
		return nil, err
	}

	urls := mediaserver.BuildPlayURLs(ms, "rtp", stream)
	st := &Stream{
		ChannelID:     ch.ID,
		GbChannelID:   ch.GbChannelID,
		DeviceID:      dev.ID,
		GbDeviceID:    dev.GbDeviceID,
		SessionID:     dlg.CallID,
		App:           "rtp",
		Stream:        stream,
		SSRC:          ssrcStr,
		Protocol:      protocol,
		MediaServerID: ms.ServerID,
		MediaServer:   ms,
		HTTPBase:      HTTPBase(ms),
		URLs:          urls,
		CreatedAt:     time.Now(),
	}
	if err := o.registry.Register(st); err != nil {
		o.abort(svc, client, dlg, ssrcStr, stream)
		return nil, err
	}
	return resultOf(st), nil
}

// Stop 停止一路播放（幂等：重复调用第二次返回 ErrStreamNotFound）。
//
// streamID 可以是 ZLM 流名（%08X(ssrc)）或 SIP Call-ID：
// 契约 §15.1 的 stream_id 由后端生成，两种都能定位到同一会话，这里都接受。
func (o *Orchestrator) Stop(ctx context.Context, streamID string) error {
	streamID = strings.TrimSpace(streamID)
	if streamID == "" {
		return ErrStreamNotFound
	}
	st, ok := o.lookup(streamID)
	if !ok {
		return ErrStreamNotFound
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	// 二次确认：拿到锁后会话可能已被并发 Stop 清掉。
	if cur, ok := o.lookup(streamID); ok {
		st = cur
	} else {
		return ErrStreamNotFound
	}

	var errs []error
	cctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	svc, serr := SIPService()
	if serr != nil {
		errs = append(errs, serr)
	} else {
		// 优先按 Call-ID 找对话；老会话没有 Call-ID 时按通道兜底。
		if dlg, ok := svc.Sessions().Get(st.SessionID); ok {
			if err := dlg.Bye(cctx); err != nil {
				errs = append(errs, fmt.Errorf("发送 BYE 失败：%w", err))
			}
		} else if dlg, ok := svc.Sessions().GetByChannel(st.GbChannelID); ok {
			if err := dlg.Bye(cctx); err != nil {
				errs = append(errs, fmt.Errorf("发送 BYE 失败：%w", err))
			}
		}
		svc.ReleaseSSRC(st.SSRC)
	}

	// 单端口模式没有流级 RTP server，停止流的唯一入口是 close_streams。
	// 注意不能直接 FromModel(nil)：它会返回指向 127.0.0.1:80 的兜底客户端，
	// 在我们没有节点信息时会去关一个不存在的流，制造伪错误。
	var client *mediaserver.Client
	if st.MediaServer != nil {
		client = mediaserver.FromModel(st.MediaServer)
	} else if c, _, err := ResolveMediaServer(ctx, &model.Device{MediaServerID: st.MediaServerID}); err == nil {
		client = c
	}
	if client != nil && st.Stream != "" {
		if err := client.CloseStreams(cctx, st.App, st.Stream); err != nil {
			errs = append(errs, fmt.Errorf("关闭媒体流失败：%w", err))
		}
	}

	o.registry.Unregister(st.Stream)
	return errors.Join(errs...)
}

// Status 查询某通道的点播状态。没有会话时返回 Playing=false 的空状态，
// 不视为错误 —— 前端轮询状态时不该因为「还没开播」拿到 400。
func (o *Orchestrator) Status(ctx context.Context, channelID int64) (*Status, error) {
	st, ok := o.registry.GetByChannel(channelID)
	if !ok {
		return &Status{ChannelID: channelID}, nil
	}

	status := &Status{
		ChannelID:     channelID,
		Playing:       true,
		App:           st.App,
		Stream:        st.Stream,
		SSRC:          st.SSRC,
		HTTPBase:      st.HTTPBase,
		URLs:          st.URLs,
		MediaServerID: st.MediaServerID,
		StreamID:      st.SessionID,
	}
	if !st.CreatedAt.IsZero() {
		t := st.CreatedAt
		status.StartDT = &t
	}

	if st.MediaServer != nil {
		client := mediaserver.FromModel(st.MediaServer)
		qctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
		defer cancel()
		ready, err := client.IsStreamReady(qctx, st.App, st.Stream, "", "")
		status.Ready = err == nil && ready
	}
	return status, nil
}

// waitReady 轮询 ZLM 直到流在线或超时。
func (o *Orchestrator) waitReady(ctx context.Context, client *mediaserver.Client, stream string) error {
	timeout := o.readyTimeout
	if timeout <= 0 {
		timeout = defaultReadyTimeout
	}
	interval := o.pollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	deadline := time.Now().Add(timeout)

	var lastErr error
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if lastErr != nil {
				return fmt.Errorf("%w（流名 %s）：%v", ErrStreamNotReady, stream, lastErr)
			}
			return fmt.Errorf("%w（流名 %s）", ErrStreamNotReady, stream)
		}
		// 每次探测都限制在剩余时间内，避免 ZLM 地址被黑洞时
		// 单次 HTTP 调用（默认 10s）把总超时拖长。
		checkTimeout := remaining
		if checkTimeout > 2*time.Second {
			checkTimeout = 2 * time.Second
		}
		qctx, cancel := context.WithTimeout(ctx, checkTimeout)
		ready, err := client.IsStreamReady(qctx, "rtp", stream, "", "")
		cancel()
		if err == nil && ready {
			return nil
		}
		if err != nil {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// lookup 按流名或 Call-ID 查找会话。
func (o *Orchestrator) lookup(id string) (*Stream, bool) {
	if st, ok := o.registry.GetByStream(id); ok {
		return st, true
	}
	return o.registry.GetBySessionID(id)
}

// abort 回滚一次失败的点播。使用独立超时上下文：请求 ctx 此时可能已取消，
// 用它发 BYE 会直接失败，导致设备侧流一直挂着。
func (o *Orchestrator) abort(svc *sipserver.Service, client *mediaserver.Client, dlg *session.Dialog, ssrc, stream string) {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	if dlg != nil {
		if err := dlg.Bye(ctx); err != nil {
			utils.Warn("点播回滚：发送 BYE 失败", zap.String("stream", stream), zap.Error(err))
		}
	}
	if client != nil {
		if err := client.CloseStreams(ctx, "rtp", stream); err != nil {
			utils.Warn("点播回滚：关闭媒体流失败", zap.String("stream", stream), zap.Error(err))
		}
	}
	if svc != nil {
		svc.ReleaseSSRC(ssrc)
	}
	o.registry.Unregister(stream)
}

// stopStream 清理一条已失效的注册记录（复用前发现 ZLM 没有流）。
func (o *Orchestrator) stopStream(ctx context.Context, svc *sipserver.Service, client *mediaserver.Client, st *Stream) {
	cctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	if svc != nil {
		if dlg, ok := svc.Sessions().Get(st.SessionID); ok {
			_ = dlg.Bye(cctx)
		}
		svc.ReleaseSSRC(st.SSRC)
	}
	if client != nil && st.Stream != "" {
		_ = client.CloseStreams(cctx, st.App, st.Stream)
	}
	o.registry.Unregister(st.Stream)
}

// resultOf 把注册项转成编排结果。
func resultOf(st *Stream) *Result {
	if st == nil {
		return nil
	}
	return &Result{
		App:           st.App,
		Stream:        st.Stream,
		SSRC:          st.SSRC,
		HTTPBase:      st.HTTPBase,
		URLs:          st.URLs,
		MediaServerID: st.MediaServerID,
		StreamID:      st.SessionID,
	}
}

// SSRCToUint32 把 10 位十进制 SSRC 转成 ZLM startSendRtp 需要的 uint32。
func SSRCToUint32(ssrc string) (uint32, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(ssrc), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("SSRC %q 不是合法的 10 位十进制数：%w", ssrc, err)
	}
	return uint32(v), nil
}
