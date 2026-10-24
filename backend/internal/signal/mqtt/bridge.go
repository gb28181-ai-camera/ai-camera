package mqtt

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/gb28181/webrtc"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
)

// 默认参数。
const (
	// DefaultMaxInflight 同时在途的 offer 上限。
	//
	// 每个在途请求只占一个 goroutine 与一个 HTTP 连接；32 足够覆盖单机
	// App 的并发开窗峰值，又能防止 broker 突然灌入大量消息把 ZLM 打满。
	DefaultMaxInflight = 32
	// DefaultAnswerTimeout 单次 WHEP 协商的超时。
	DefaultAnswerTimeout = 15 * time.Second
	// defaultDedupTTL 去重窗口。App 重发同一 request_id 通常是网络抖动，
	// 5 分钟足以覆盖；过期后允许同一 ID 再次使用，避免 map 无限增长。
	defaultDedupTTL = 5 * time.Minute
	// dedupSweepThreshold 触发惰性清理的条目数。
	dedupSweepThreshold = 4096
)

// 错误码（与 App 约定）。
const (
	CodeInvalidRequest  = "invalid_request"
	CodeUnsupportedType = "unsupported_type"
	CodeStreamNotFound  = "stream_not_found"
	CodeBusy            = "busy"
	CodeTimeout         = "timeout"
	CodeWhepFailed      = "whep_failed"
	CodeInternal        = "internal_error"
)

// Publisher 是信令下发所需的最小发布接口（*Client 实现，测试可用假实现）。
type Publisher interface {
	Publish(topic string, qos byte, retained bool, payload []byte) error
}

// Answerer 是 SDP 协商能力（*webrtc.Client 实现，测试可用假实现）。
type Answerer interface {
	Answer(ctx context.Context, stream, offer string) (string, error)
}

// StreamRegistry 是播放会话查询能力（*play.Registry 实现）。
type StreamRegistry interface {
	GetByChannel(channelID int64) (*play.Stream, bool)
	GetByStream(stream string) (*play.Stream, bool)
}

// BridgeOptions 是 Bridge 的可调参数。
type BridgeOptions struct {
	MaxInflight   int
	AnswerTimeout time.Duration
	DedupTTL      time.Duration
}

// Bridge 把 MQTT 上的 offer 转成 ZLM 的 WHEP 协商结果。
//
// 并发模型：
//   - Handle 由 MQTT 回调调用，必须快速返回（不能阻塞后续消息）；
//   - 每个 offer 起独立 goroutine，最多 MaxInflight 个在途；
//   - 去重表保证同一 request_id 只处理/回复一次。
type Bridge struct {
	publisher     Publisher
	answerer      Answerer
	registry      StreamRegistry
	maxInflight   int
	answerTimeout time.Duration
	dedupTTL      time.Duration

	sem chan struct{}

	mu     sync.Mutex
	seen   map[string]time.Time
	closed bool
	wg     sync.WaitGroup

	now func() time.Time
}

// NewBridge 使用默认参数构造 Bridge。
func NewBridge(pub Publisher, ans Answerer, reg StreamRegistry) *Bridge {
	return NewBridgeWithOptions(pub, ans, reg, BridgeOptions{})
}

// NewBridgeWithOptions 使用显式参数构造 Bridge（测试可缩短超时）。
func NewBridgeWithOptions(pub Publisher, ans Answerer, reg StreamRegistry, opts BridgeOptions) *Bridge {
	if opts.MaxInflight <= 0 {
		opts.MaxInflight = DefaultMaxInflight
	}
	if opts.AnswerTimeout <= 0 {
		opts.AnswerTimeout = DefaultAnswerTimeout
	}
	if opts.DedupTTL <= 0 {
		opts.DedupTTL = defaultDedupTTL
	}
	return &Bridge{
		publisher:     pub,
		answerer:      ans,
		registry:      reg,
		maxInflight:   opts.MaxInflight,
		answerTimeout: opts.AnswerTimeout,
		dedupTTL:      opts.DedupTTL,
		sem:           make(chan struct{}, opts.MaxInflight),
		seen:          make(map[string]time.Time),
		now:           time.Now,
	}
}

// Handle 处理一条来自 iot/+/signal/backend 的消息。
//
// 该方法不做任何阻塞操作：校验、去重、取信号量后立即返回，
// WHEP 协商在独立 goroutine 中完成。
func (b *Bridge) Handle(topic string, payload []byte) {
	if b == nil {
		return
	}
	group, ok := ParseBackendTopic(topic)
	if !ok {
		utils.Warn("MQTT 信令：忽略非法话题", zap.String("topic", topic))
		return
	}

	msg, err := ParseMessage(payload)
	if err != nil {
		// 消息体不合法时通常拿不到 request_id/client_id，无法定向回 error。
		utils.Warn("MQTT 信令：消息解析失败",
			zap.String("topic", topic), zap.Error(err))
		return
	}

	// 无论后续成功与否，同一 request_id 只处理一次、只回一次。
	if msg.RequestID != "" {
		if !b.markSeen(msg.RequestID) {
			utils.Debug("MQTT 信令：重复 request_id，忽略",
				zap.String("request_id", msg.RequestID),
				zap.String("client_id", msg.ClientID))
			return
		}
	}

	switch msg.Type {
	case TypeCandidate:
		// ZLM 返回的是非 trickle answer，ICE 候选已包含在 SDP 中。
		// App 若仍发送 candidate，直接忽略并记 debug，避免误判为协议错误。
		utils.Debug("MQTT 信令：忽略 candidate（非 trickle ICE）",
			zap.String("request_id", msg.RequestID),
			zap.String("client_id", msg.ClientID))
		return
	case TypeOffer:
		// 继续处理。
	default:
		b.replyError(group, msg.ClientID, msg.RequestID,
			CodeUnsupportedType, "仅支持 offer 消息")
		return
	}

	if err := msg.ValidateOffer(); err != nil {
		utils.Warn("MQTT 信令：offer 校验失败",
			zap.String("request_id", msg.RequestID),
			zap.String("client_id", msg.ClientID),
			zap.Error(err))
		b.replyError(group, msg.ClientID, msg.RequestID,
			CodeInvalidRequest, "offer 参数不完整或格式非法")
		return
	}

	stream, code := b.resolveStream(msg)
	if code != "" {
		b.replyError(group, msg.ClientID, msg.RequestID,
			code, "流不存在或已结束，请先调用播放接口")
		return
	}
	if b.answerer == nil {
		utils.Error("MQTT 信令：未配置 WHEP 协商器")
		b.replyError(group, msg.ClientID, msg.RequestID,
			CodeInternal, "信令服务未就绪，请稍后重试")
		return
	}

	// 先占坑再起 goroutine：超限时立即回 error，不让 App 干等。
	select {
	case b.sem <- struct{}{}:
	default:
		utils.Warn("MQTT 信令：在途请求已达上限",
			zap.Int("max_inflight", b.maxInflight),
			zap.String("request_id", msg.RequestID))
		b.replyError(group, msg.ClientID, msg.RequestID,
			CodeBusy, "信令服务繁忙，请稍后重试")
		return
	}

	if !b.start() {
		<-b.sem
		// 已进入关闭流程：不再回消息，broker 连接也即将断开。
		return
	}

	go func() {
		defer func() {
			<-b.sem
			b.wg.Done()
		}()
		b.process(group, msg, stream)
	}()
}

// resolveStream 解析本次协商使用的 ZLM 流名。
//
// 规则：
//   - App 带了 stream：以它为准；若注册表里有同一路，校验 channel_id 一致。
//     （App 的 stream 来自现有播放接口，注册表可能因进程重启而暂时为空，
//     所以「注册表查不到」不直接判失败，最终由 ZLM 判定。）
//   - App 只有 channel_id：查 play.DefaultRegistry 兜底。
//   - 两者都拿不到：stream_not_found。
func (b *Bridge) resolveStream(msg *Message) (string, string) {
	stream := strings.TrimSpace(msg.Stream)
	if stream != "" {
		if b.registry != nil {
			if st, ok := b.registry.GetByStream(stream); ok {
				if msg.ChannelID > 0 && st.ChannelID != msg.ChannelID {
					return "", CodeStreamNotFound
				}
				return st.Stream, ""
			}
			// stream 不在注册表里，但该通道正在播另一路流：说明 App 拿的是
			// 过期/串台的 stream，必须拒绝，不能拿它去问 ZLM。
			if msg.ChannelID > 0 {
				if st, ok := b.registry.GetByChannel(msg.ChannelID); ok && st.Stream != stream {
					return "", CodeStreamNotFound
				}
			}
		}
		return stream, ""
	}

	if msg.ChannelID > 0 && b.registry != nil {
		if st, ok := b.registry.GetByChannel(msg.ChannelID); ok {
			return st.Stream, ""
		}
	}
	return "", CodeStreamNotFound
}

// process 执行一次完整的 WHEP 协商并下发结果。
func (b *Bridge) process(group string, msg *Message, stream string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), b.answerTimeout)
	defer cancel()

	answer, err := b.answerer.Answer(ctx, stream, msg.SDP)
	if err != nil {
		code := CodeWhepFailed
		publicMsg := "媒体协商失败，请稍后重试"
		switch {
		case errors.Is(err, webrtc.ErrStreamNotFound):
			code, publicMsg = CodeStreamNotFound, "流不存在或已结束，请重新发起播放"
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			code, publicMsg = CodeTimeout, "媒体节点响应超时，请稍后重试"
		}
		// 错误详情只进日志，不回传给 App，避免泄露媒体节点内网地址。
		utils.Warn("MQTT 信令：WHEP 协商失败",
			zap.String("request_id", msg.RequestID),
			zap.String("client_id", msg.ClientID),
			zap.Int64("channel_id", msg.ChannelID),
			zap.String("stream", stream),
			zap.String("code", code),
			zap.Duration("elapsed", time.Since(start)),
			zap.Error(err))
		b.replyError(group, msg.ClientID, msg.RequestID, code, publicMsg)
		return
	}

	if err := b.publish(group, msg.ClientID, NewAnswer(msg.RequestID, answer)); err != nil {
		utils.Error("MQTT 信令：answer 下发失败",
			zap.String("request_id", msg.RequestID),
			zap.String("client_id", msg.ClientID),
			zap.String("stream", stream),
			zap.Error(err))
		return
	}

	utils.Info("MQTT 信令：answer 已下发",
		zap.String("request_id", msg.RequestID),
		zap.String("client_id", msg.ClientID),
		zap.Int64("channel_id", msg.ChannelID),
		zap.String("stream", stream),
		zap.Duration("elapsed", time.Since(start)))
}

// replyError 向 App 下发 error 消息。缺少 client_id/request_id 时只能记日志。
func (b *Bridge) replyError(group, clientID, requestID, code, message string) {
	if strings.TrimSpace(requestID) == "" {
		utils.Warn("MQTT 信令：无法回 error（缺少 request_id）",
			zap.String("client_id", clientID), zap.String("code", code))
		return
	}
	if err := b.publish(group, clientID, NewError(requestID, code, message)); err != nil {
		utils.Warn("MQTT 信令：error 下发失败",
			zap.String("request_id", requestID),
			zap.String("client_id", clientID),
			zap.String("code", code),
			zap.Error(err))
	}
}

// publish 序列化并发布到 App 话题。
func (b *Bridge) publish(group, clientID string, msg *Message) error {
	if b.publisher == nil {
		return errors.New("未配置 MQTT publisher")
	}
	topic, err := AppSignalTopic(group, clientID)
	if err != nil {
		return err
	}
	payload, err := msg.Marshal()
	if err != nil {
		return err
	}
	return b.publisher.Publish(topic, DefaultQoS, false, payload)
}

// start 在关闭流程开始前登记一个在途请求。
func (b *Bridge) start() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return false
	}
	b.wg.Add(1)
	return true
}

// markSeen 记录 request_id；已存在且未过期时返回 false。
func (b *Bridge) markSeen(id string) bool {
	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()

	if t, ok := b.seen[id]; ok && now.Sub(t) < b.dedupTTL {
		return false
	}
	b.seen[id] = now
	if len(b.seen) > dedupSweepThreshold {
		b.sweepLocked(now)
	}
	return true
}

// sweepLocked 清理过期去重记录。调用方必须持有 b.mu。
func (b *Bridge) sweepLocked(now time.Time) {
	for id, t := range b.seen {
		if now.Sub(t) >= b.dedupTTL {
			delete(b.seen, id)
		}
	}
}

// Close 停止接收新请求，并等待在途请求完成（受 ctx 限制）。
//
// 注意：Close 只负责「不再受理 + 等在途」，broker 连接由 Client.Close 关闭；
// 顺序应为 Bridge.Close → Client.Close，保证 answer 仍能发出去。
func (b *Bridge) Close(ctx context.Context) {
	if b == nil {
		return
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.mu.Unlock()

	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		utils.Info("MQTT 信令桥已停止")
	case <-ctx.Done():
		utils.Warn("MQTT 信令桥停止超时，仍有在途请求")
	}
}

// Inflight 返回当前在途请求数（测试与排障用）。
func (b *Bridge) Inflight() int { return len(b.sem) }
