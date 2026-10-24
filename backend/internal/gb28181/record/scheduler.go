package record

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
)

// DefaultInterval 是扫描周期。
//
// 30 秒是精度与开销的折中：录像计划的最小粒度是分钟，30 秒最多带来
// 半分钟的起录延迟；同时每轮只是一次轻量 SQL，不会给数据库造成压力。
const DefaultInterval = 30 * time.Second

// TargetLoader 加载「所有绑定到启用计划的通道 × 时间段」。
// 抽成函数类型是为了单测能注入假数据，不依赖数据库。
type TargetLoader func(ctx context.Context) ([]Target, error)

// JobRef 是一路已启动的录制任务。
//
// 保存到 Scheduler 的 jobs 里，用于下一轮判断「是否还在时段内」。
type JobRef struct {
	ChannelID     int64
	Key           string
	Stream        string
	App           string
	SSRC          string
	MediaServerID string
	DeviceID      int64
	EndsAt        time.Time
	// OwnStream 表示这条流是调度器为了录制主动发起的点播会话。
	// 复用用户正在观看的流时为 false：停止录制时只 stopRecord，
	// 不能把用户正在看的流也关掉。
	OwnStream bool
}

// JobRunner 执行真正的启动/停止动作，便于单测替换。
type JobRunner interface {
	Start(ctx context.Context, t Target, now time.Time) (JobRef, error)
	Stop(ctx context.Context, job JobRef)
}

// Option 是 Scheduler 的可选配置（测试注入用）。
type Option func(*Scheduler)

// WithInterval 自定义扫描周期。
func WithInterval(d time.Duration) Option {
	return func(s *Scheduler) {
		if d > 0 {
			s.interval = d
		}
	}
}

// WithNow 注入时钟。
func WithNow(fn func() time.Time) Option {
	return func(s *Scheduler) {
		if fn != nil {
			s.now = fn
		}
	}
}

// WithLoader 注入目标加载器。
func WithLoader(l TargetLoader) Option {
	return func(s *Scheduler) {
		if l != nil {
			s.loader = l
		}
	}
}

// WithRunner 注入任务执行器。
func WithRunner(r JobRunner) Option {
	return func(s *Scheduler) {
		if r != nil {
			s.runner = r
		}
	}
}

// Scheduler 是录像计划调度器。
//
// 并发模型：
//   - jobs 只由 ReconcileOnce 的持有锁路径读写；
//   - 启动动作可能阻塞（点播要等设备推流），因此放到 goroutine，
//     用 starting 标记防止同一通道被重复启动；
//   - 停止动作很快（HTTP/SIP），同步执行。
type Scheduler struct {
	mu       sync.Mutex
	jobs     map[int64]JobRef
	starting map[int64]bool
	// running 防止 Notify 与 ticker 同时进入 ReconcileOnce。
	running bool

	interval time.Duration
	now      func() time.Time
	loader   TargetLoader
	runner   JobRunner
	notify   chan struct{}
}

// NewScheduler 构造调度器；未显式注入时使用数据库加载器与默认执行器。
func NewScheduler(opts ...Option) *Scheduler {
	s := &Scheduler{
		jobs:     map[int64]JobRef{},
		starting: map[int64]bool{},
		interval: DefaultInterval,
		now:      time.Now,
		notify:   make(chan struct{}, 1),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.loader == nil {
		s.loader = loadTargetsFromDB
	}
	if s.runner == nil {
		s.runner = &defaultRunner{}
	}
	return s
}

var (
	ensureOnce       sync.Once
	defaultScheduler atomic.Pointer[Scheduler]
)

// EnsureStarted 懒启动进程级调度器。
//
// 接线点：GbRecordPlanService 的构造函数。路由注册时 handler 会被构造，
// 因此进程一启动就会走到这里，不需要修改 cmd/server。重复调用安全。
func EnsureStarted() {
	ensureOnce.Do(func() {
		if bootstrap.DB() == nil {
			// 单元测试/只读工具可能没有数据库：不启动，避免后台 goroutine 空转。
			return
		}
		s := NewScheduler()
		defaultScheduler.Store(s)
		go s.Start(context.Background())
	})
}

// NotifyChanged 通知调度器「计划/绑定关系变了」，立即重新对齐一次。
//
// 不做同步等待：HTTP 接口不应该因为某台设备点播慢而被拖住。
// 通道容量为 1，短时间内的多次修改会合并成一次扫描。
func NotifyChanged() {
	if s := defaultScheduler.Load(); s != nil {
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
}

// Start 启动扫描循环，直到 ctx 被取消。
//
// 首轮在 2 秒后执行：给 bootstrap（配置/数据库）留出完成初始化的时间，
// 同时保证「进程重启后从当前时刻重新判断」几乎立即生效。
func (s *Scheduler) Start(ctx context.Context) {
	first := time.NewTimer(2 * time.Second)
	defer first.Stop()
	select {
	case <-ctx.Done():
		return
	case <-first.C:
	}

	s.ReconcileOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.stopAll()
			return
		case <-ticker.C:
			s.ReconcileOnce(ctx)
		case <-s.notify:
			s.ReconcileOnce(ctx)
		}
	}
}

// ReconcileOnce 对齐「期望的录制集合」与「正在运行的录制集合」。
//
// 这是调度器的核心，且被设计成同步、可重入调用的纯编排：
//  1. 加载全部「通道 × 时间段」；
//  2. 按当前时刻挑出每个通道生效的时间段；
//  3. 不再生效的任务 → 停止；
//  4. 新任务 → 启动（已有同 Key 任务则跳过，保证幂等）。
func (s *Scheduler) ReconcileOnce(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		// 上一轮还没结束（点播启动可能较慢）：放弃本轮，下一轮 ticker 会再来。
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	targets, err := s.loader(ctx)
	if err != nil {
		// 数据库瞬时故障不应该把正在录的任务全部停掉：保持现状，下轮重试。
		utils.Warn("[GB-RECORD] 加载录像计划失败，保留现有任务", zap.Error(err))
		return
	}

	now := s.now()
	// 明确离线的设备不参与本轮对齐：避免每 30 秒刷一次「设备离线」告警。
	// 状态为空（测试/历史数据）时仍按在线处理，由 runner 做最终校验。
	online := make([]Target, 0, len(targets))
	for _, t := range targets {
		if strings.EqualFold(strings.TrimSpace(t.DeviceStatus), model.DeviceStatusOffline) {
			continue
		}
		online = append(online, t)
	}
	grouped := GroupByChannel(online)
	desired := make(map[int64]Target, len(grouped))
	for chID, list := range grouped {
		if t, ok := PickActive(list, now); ok {
			desired[chID] = t
		}
	}

	// ── 停止已经不在时段内的任务 ──────────────────────────────
	s.mu.Lock()
	var toStop []JobRef
	for chID, job := range s.jobs {
		want, ok := desired[chID]
		if ok && want.Window.Key() == job.Key {
			continue
		}
		delete(s.jobs, chID)
		toStop = append(toStop, job)
	}
	s.mu.Unlock()

	for _, job := range toStop {
		utils.Info("[GB-RECORD] 时段结束，停止录像",
			zap.Int64("channel_id", job.ChannelID), zap.String("stream", job.Stream))
		s.runner.Stop(ctx, job)
	}

	// ── 启动新任务 ────────────────────────────────────────────
	for chID, target := range desired {
		s.mu.Lock()
		_, exists := s.jobs[chID]
		if exists || s.starting[chID] {
			s.mu.Unlock()
			continue
		}
		s.starting[chID] = true
		s.mu.Unlock()

		go s.startAsync(ctx, target, now)
	}
}

// startAsync 在后台启动一路录制。
//
// 启动失败只记日志，不写入任何「失败任务」：下一轮扫描会重试。
// 这是有意为之 —— 设备刚上线、流还没建好都属于瞬时失败。
func (s *Scheduler) startAsync(ctx context.Context, target Target, now time.Time) {
	defer func() {
		s.mu.Lock()
		delete(s.starting, target.ChannelID)
		s.mu.Unlock()
	}()

	job, err := s.runner.Start(ctx, target, now)
	if err != nil {
		utils.Warn("[GB-RECORD] 启动录像失败，将在下一轮重试",
			zap.Int64("channel_id", target.ChannelID),
			zap.String("channel", target.ChannelName),
			zap.String("window", target.Window.Key()),
			zap.Error(err))
		return
	}

	// 启动期间时段可能已经结束（或计划被改掉）：立刻停掉，避免留下无人管理的录制。
	if !target.Window.ActiveAt(s.now()) {
		s.runner.Stop(ctx, job)
		return
	}

	s.mu.Lock()
	// 期间可能有同 Key 的并发启动结果先写入，保留先到者（幂等）。
	if _, ok := s.jobs[target.ChannelID]; !ok {
		s.jobs[target.ChannelID] = job
	} else {
		// 产生了重复任务：停掉刚启动的这一路。
		s.mu.Unlock()
		s.runner.Stop(ctx, job)
		return
	}
	s.mu.Unlock()

	utils.Info("[GB-RECORD] 录像已启动",
		zap.Int64("channel_id", target.ChannelID),
		zap.String("channel", target.ChannelName),
		zap.String("stream", job.Stream),
		zap.Time("ends_at", job.EndsAt))
}

// stopAll 进程退出时尽力停止全部任务。
//
// 使用独立超时上下文：ctx 已经取消，不能拿它去发 StopRecord/BYE。
func (s *Scheduler) stopAll() {
	s.mu.Lock()
	jobs := make([]JobRef, 0, len(s.jobs))
	for _, j := range s.jobs {
		jobs = append(jobs, j)
	}
	s.jobs = map[int64]JobRef{}
	s.mu.Unlock()

	for _, job := range jobs {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		s.runner.Stop(ctx, job)
		cancel()
	}
}

// ── 数据库加载器 ──────────────────────────────────────────────

// planTargetRow 是联表查询的扁平结果。
type planTargetRow struct {
	ChannelID    int64  `gorm:"column:channel_id"`
	ChannelName  string `gorm:"column:channel_name"`
	DeviceID     int64  `gorm:"column:device_id"`
	DeviceName   string `gorm:"column:device_name"`
	DeviceCode   string `gorm:"column:device_code"`
	GbDeviceID   string `gorm:"column:gb_device_id"`
	GbChannelID  string `gorm:"column:gb_channel_id"`
	DeviceStatus string `gorm:"column:device_status"`
	PlanID       int64  `gorm:"column:plan_id"`
	ItemID       int64  `gorm:"column:item_id"`
	WeekDay      int64  `gorm:"column:week_day"`
	StartTime    string `gorm:"column:start_time"`
	EndTime      string `gorm:"column:end_time"`
	Snap         *bool  `gorm:"column:snap"`
}

// loadTargetsFromDB 查询「启用计划 × 时间段 × 启用通道」。
//
// 关联关系在 ac_gb_channel.record_plan_id，明细表不含通道，因此必须联表。
// 只取启用计划：计划被停用后调度器会在下一轮停止对应任务（desired 为空）。
func loadTargetsFromDB(ctx context.Context) ([]Target, error) {
	db := bootstrap.DB()
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	var rows []planTargetRow
	err := db.WithContext(ctx).
		Table("ac_gb_channel AS ch").
		Select(`ch.id AS channel_id, ch.name AS channel_name,
			ch.device_id AS device_id, ch.device_code AS device_code,
			ch.gb_device_id AS gb_device_id, ch.gb_channel_id AS gb_channel_id,
			d.name AS device_name, d.status AS device_status,
			p.id AS plan_id, p.snap AS snap,
			i.id AS item_id, i.week_day AS week_day,
			i.start_time AS start_time, i.end_time AS end_time`).
		Joins("JOIN ac_gb_record_plan p ON p.id = ch.record_plan_id AND p.deleted_at IS NULL").
		Joins("JOIN ac_gb_record_plan_item i ON i.plan_id = p.id AND i.deleted_at IS NULL").
		Joins("JOIN ac_device d ON d.id = ch.device_id AND d.deleted_at IS NULL").
		Where("ch.deleted_at IS NULL").
		Where("(ch.active IS NULL OR ch.active = TRUE)").
		Where("(p.active IS NULL OR p.active = TRUE)").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make([]Target, 0, len(rows))
	for _, r := range rows {
		out = append(out, Target{
			ChannelID:    r.ChannelID,
			ChannelName:  r.ChannelName,
			DeviceID:     r.DeviceID,
			DeviceName:   r.DeviceName,
			DeviceCode:   r.DeviceCode,
			GbDeviceID:   r.GbDeviceID,
			GbChannelID:  r.GbChannelID,
			DeviceStatus: r.DeviceStatus,
			Window: Window{
				PlanID:    r.PlanID,
				ItemID:    r.ItemID,
				WeekDay:   r.WeekDay,
				StartTime: r.StartTime,
				EndTime:   r.EndTime,
				Snap:      r.Snap != nil && *r.Snap,
			},
		})
	}
	return out, nil
}

// ── 默认执行器：实时点播 + ZLM 录制 + 设备 RecordCmd ─────────

// defaultRunner 是生产实现。
type defaultRunner struct{}

// Start 启动一路云端录像与设备录像。
//
// 关键点：ZLM 的 startRecord 只能录制**已经存在的流**，因此必须先保证
// 设备在推流。复用实时点播编排（play.Orchestrator）有两个好处：
//  1. 已有用户在看时直接复用会话，不再重复 INVITE；
//  2. INVITE/等待建流/BYE/close_streams 的清理逻辑已经过验证。
func (r *defaultRunner) Start(ctx context.Context, t Target, now time.Time) (JobRef, error) {
	ch, err := repository.NewGbChannelRepo().GetByID(ctx, t.ChannelID)
	if err != nil {
		return JobRef{}, fmt.Errorf("读取通道失败：%w", err)
	}
	dev := ch.Device
	if dev == nil {
		dev, err = repository.NewDeviceRepo().GetByID(ctx, ch.DeviceID)
		if err != nil {
			return JobRef{}, fmt.Errorf("读取设备失败：%w", err)
		}
	}
	if strings.TrimSpace(dev.GbDeviceID) == "" {
		return JobRef{}, errors.New("设备未配置国标编码")
	}
	if !strings.EqualFold(strings.TrimSpace(dev.Status), model.DeviceStatusOnline) {
		return JobRef{}, fmt.Errorf("设备离线（status=%s）", dev.Status)
	}

	client, ms, err := play.ResolveMediaServer(ctx, dev)
	if err != nil {
		return JobRef{}, err
	}

	orch := play.DefaultOrchestrator()
	ownStream := false
	stream := ""
	ssrcStr := ""

	// 先尝试复用已有在线流。
	if st, ok := orch.Registry().GetByChannel(ch.ID); ok {
		if ready, rerr := client.IsStreamReady(ctx, st.App, st.Stream, "", ""); rerr == nil && ready {
			stream, ssrcStr = st.Stream, st.SSRC
		} else if rerr != nil {
			return JobRef{}, fmt.Errorf("查询媒体流状态失败：%w", rerr)
		}
	}
	if stream == "" {
		res, perr := orch.Play(ctx, dev, ch, "flv")
		if perr != nil {
			return JobRef{}, fmt.Errorf("建立录制流失败：%w", perr)
		}
		stream, ssrcStr = res.Stream, res.SSRC
		ownStream = true
	}

	remaining := t.Window.Remaining(now)
	maxSecond := int(remaining.Seconds())
	if maxSecond < 1 {
		maxSecond = 1
	}

	// StartRecord 偶发「流刚注册但还没就绪」，做几次短重试。
	var recErr error
	for i := 0; i < 3; i++ {
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		recErr = client.StartRecord(rctx, "rtp", stream, "", maxSecond)
		cancel()
		if recErr == nil {
			break
		}
		select {
		case <-ctx.Done():
			recErr = ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		if recErr != nil && ctx.Err() != nil {
			break
		}
	}
	if recErr != nil {
		if ownStream {
			_ = orch.Stop(context.Background(), stream)
		}
		return JobRef{}, fmt.Errorf("ZLM 开始录制失败：%w", recErr)
	}

	// 设备本地录像：best-effort。不同厂商对 RecordCmd 的支持差异很大
	// （有的只接受通道编码、有的要求设备编码），失败不能影响云端录像。
	if err := sendRecordCmd(ctx, dev, ch, true); err != nil {
		utils.Warn("[GB-RECORD] 设备 RecordCmd(开始) 下发失败，云端录像不受影响",
			zap.Int64("channel_id", ch.ID), zap.Error(err))
	}

	RegisterStreamOwner(StreamOwner{
		App:           "rtp",
		Stream:        stream,
		MediaServerID: ms.ServerID,
		DeviceID:      dev.ID,
		ChannelID:     ch.ID,
	})

	return JobRef{
		ChannelID:     ch.ID,
		Key:           t.Window.Key(),
		Stream:        stream,
		App:           "rtp",
		SSRC:          ssrcStr,
		MediaServerID: ms.ServerID,
		DeviceID:      dev.ID,
		EndsAt:        t.Window.EndAt(now),
		OwnStream:     ownStream,
	}, nil
}

// Stop 停止录制。
//
// 顺序很重要：先 stopRecord 再 BYE/close_streams。反过来流会立即消失，
// ZLM 找不到流，最后一个 MP4 片段可能没有正常收尾。
func (r *defaultRunner) Stop(ctx context.Context, job JobRef) {
	client, _, err := resolveClientByID(ctx, job.MediaServerID)
	if err == nil && client != nil && job.Stream != "" {
		if serr := client.StopRecord(ctx, job.App, job.Stream, ""); serr != nil {
			utils.Warn("[GB-RECORD] 停止 ZLM 录制失败",
				zap.String("stream", job.Stream), zap.Error(serr))
		}
	}

	// 设备录像同样 best-effort。
	if dev, derr := repository.NewDeviceRepo().GetByID(ctx, job.DeviceID); derr == nil {
		if ch, cerr := loadChannelForJob(ctx, job); cerr == nil && ch != nil {
			if rerr := sendRecordCmd(ctx, dev, ch, false); rerr != nil {
				utils.Warn("[GB-RECORD] 设备 RecordCmd(停止) 下发失败",
					zap.Int64("channel_id", job.ChannelID), zap.Error(rerr))
			}
		}
	}

	// 延迟移除：等 ZLM 最后一个 on_record_mp4 回调写完再清理映射。
	UnregisterStreamOwnerAfter(job.Stream, 15*time.Second)

	if job.OwnStream {
		// 只关闭调度器自己建立的流；用户正在观看的流保持不动。
		if oerr := play.DefaultOrchestrator().Stop(ctx, job.Stream); oerr != nil &&
			!errors.Is(oerr, play.ErrStreamNotFound) {
			utils.Warn("[GB-RECORD] 关闭录制流失败",
				zap.String("stream", job.Stream), zap.Error(oerr))
		}
	}

}

// loadChannelForJob 停止设备录像时回查通道（需要 GbChannelID 组装 RecordCmd）。
//
// JobRef 不保存设备侧编码，避免结构体里出现两套 ID；停止时按通道主键
// 回查一次，成本可忽略。
func loadChannelForJob(ctx context.Context, job JobRef) (*model.GbChannel, error) {
	return repository.NewGbChannelRepo().GetByID(ctx, job.ChannelID)
}

// resolveClientByID 按 mediaServerID 解析 ZLM 客户端；为空时回落默认节点。
func resolveClientByID(ctx context.Context, serverID string) (*mediaserver.Client, *model.GbMediaServer, error) {
	key := strings.TrimSpace(serverID)
	if key != "" {
		if c, ms, err := mediaserver.FromServerID(ctx, key); err == nil && ms != nil {
			return c, ms, nil
		}
	}
	return mediaserver.FromDefaultServer(ctx)
}

// recSN 是 RecordCmd 的 MANSCDP 流水号。
var recSN atomic.Int32

// sendRecordCmd 下发设备录像开始/停止（MANSCDP DeviceControl）。
//
// 用 UTF-8 文本构造：SendMessage 会按设备 charset 再转码；
// 与 PTZ 的二进制指令不同，RecordCmd 是纯文本，可以安全转码。
func sendRecordCmd(ctx context.Context, dev *model.Device, ch *model.GbChannel, start bool) error {
	ctl := control.Default()
	if ctl == nil {
		return errors.New("SIP 信令服务未启动")
	}
	cmd := manscdp.NewRecordControl(ch.GbChannelID, int(recSN.Add(1)), start)
	body, err := manscdp.MarshalString(cmd, charset.UTF8)
	if err != nil {
		return fmt.Errorf("RecordCmd 编码失败：%w", err)
	}
	sctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctl.SendMessage(sctx, dev, body); err != nil {
		return fmt.Errorf("RecordCmd 下发失败：%w", err)
	}
	return nil
}
