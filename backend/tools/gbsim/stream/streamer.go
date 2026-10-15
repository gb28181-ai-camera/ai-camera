// Package stream 把样本 H.264 码流按 GB28181 的链路打出去：
//
//	Annex-B H.264 → ps.Muxer（NAL → PES → PS）→ rtp.Sender（PS → 分包 → RTP/UDP）
//
// 目标地址与 SSRC **全部来自平台 INVITE 的 SDP**（`c=` / `m=` / `y=`），
// 不是配置里的硬编码值 —— 这是 §7.1 时序图里「设备往 c= 的地址 + m= 的端口发 RTP」那一步。
//
// 本批次只做 UDP。TCP-PASSIVE / TCP-ACTIVE 需要额外的 `$` 分帧与主动连出，留给下一批。
package stream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/tools/gbsim/ps"
	"github.com/boticz/ai-camera-backend/tools/gbsim/rtp"
)

// DefaultFPS 是推流帧率默认值，与样本码流一致（PTS 步进 = 90000/25 = 3600）。
const DefaultFPS = 25

// Config 是一次推流会话的入参。
type Config struct {
	NALs       [][]byte      // 已切好的 Annex-B NAL 列表（不含起始码）
	SSRC       uint32        // RTP SSRC，取自设备侧 SDP 的 y=
	TargetIP   string        // 平台收流地址，取自平台 SDP 的 c=
	TargetPort int           // 平台收流端口，取自平台 SDP 的 m=
	LocalIP    string        // 本地绑定 IP（留空则交给内核选路）
	FPS        int           // 推流帧率；≤0 表示**不限速**（仅用于压测，正常别用）
	Loop       bool          // 样本放完后是否从头再来
	Duration   time.Duration // >0 时推满该时长自动停止
	MTU        int           // RTP 负载上限，0 用 rtp 包默认值
	Logger     *slog.Logger
}

// Streamer 是一个可启停的推流会话。
//
// 每个 INVITE 会话一个 Streamer：各持有独立的 UDP 连接、RTP 序号与统计，
// 多通道并发点播时互不干扰（与 §2.2 单端口模式下的多路复用并不冲突 ——
// 单端口指的是**平台侧 ZLM 的收流端口**，设备侧各自用自己的本地端口发出）。
type Streamer struct {
	cfg    Config
	log    *slog.Logger
	conn   *net.UDPConn
	sender *rtp.Sender
	units  [][][]byte // 按访问单元（一帧）分组后的 NAL

	mu        sync.Mutex
	started   bool
	cancel    context.CancelFunc
	done      chan struct{}
	runErr    error
	closeOnce sync.Once
}

// New 建立推流会话：绑定本地 UDP 端口、连向平台收流地址、按帧切分样本。
//
// 构造时即绑定端口，是为了让上层能在 200 OK 的 SDP 里写出**真实的**设备侧 m= 端口
// （见 LocalRTPPort）。若延到 ACK 之后才绑定，SDP 里就只能瞎填一个端口。
func New(cfg Config) (*Streamer, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if len(cfg.NALs) == 0 {
		return nil, errors.New("样本码流为空，无法推流")
	}
	if cfg.TargetIP == "" || cfg.TargetPort <= 0 {
		return nil, fmt.Errorf("平台收流地址非法：%s:%d（应来自 INVITE 的 SDP c= / m=）", cfg.TargetIP, cfg.TargetPort)
	}

	units := ps.GroupAccessUnits(cfg.NALs)
	if len(units) == 0 {
		return nil, errors.New("样本码流切不出任何访问单元")
	}

	var laddr *net.UDPAddr
	if cfg.LocalIP != "" {
		ip := net.ParseIP(cfg.LocalIP)
		if ip == nil {
			return nil, fmt.Errorf("本地 IP 非法：%q", cfg.LocalIP)
		}
		laddr = &net.UDPAddr{IP: ip}
	}
	raddr := &net.UDPAddr{IP: net.ParseIP(cfg.TargetIP), Port: cfg.TargetPort}
	if raddr.IP == nil {
		return nil, fmt.Errorf("平台收流地址不是合法 IP：%q（SDP 的 c= 必须是 IP，不能是域名）", cfg.TargetIP)
	}

	conn, err := net.DialUDP("udp4", laddr, raddr)
	if err != nil {
		return nil, fmt.Errorf("建立推流 UDP 连接失败（%s → %s）：%w", laddr, raddr, err)
	}

	var opts []rtp.Option
	if cfg.MTU > 0 {
		opts = append(opts, rtp.WithMTU(cfg.MTU))
	}
	return &Streamer{
		cfg:    cfg,
		log:    cfg.Logger,
		conn:   conn,
		sender: rtp.NewSender(conn, cfg.SSRC, opts...),
		units:  units,
	}, nil
}

// LocalRTPPort 返回本地绑定的 UDP 端口，供上层写进设备侧 SDP 的 m= 行。
func (s *Streamer) LocalRTPPort() int {
	if a, ok := s.conn.LocalAddr().(*net.UDPAddr); ok {
		return a.Port
	}
	return 0
}

// SSRC 返回本会话实际使用的 SSRC。
func (s *Streamer) SSRC() uint32 { return s.sender.SSRC() }

// Target 返回推流目标（平台收流地址），形如 "10.0.0.5:10000"。
func (s *Streamer) Target() string { return s.target() }

// Stats 返回累计发出的 RTP 包数与负载字节数。
func (s *Streamer) Stats() (packets, payloadBytes uint64) {
	return s.sender.Stats()
}

// Start 在后台启动推流循环，立即返回。
//
// ctx 的生命周期即会话生命周期：ctx 取消（或 Duration 到期、或调用 Stop）时停止推流。
func (s *Streamer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("推流会话已启动")
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.done = make(chan struct{})
	s.started = true

	go func() {
		defer close(s.done)
		err := s.run(runCtx)
		s.mu.Lock()
		s.runErr = err
		s.mu.Unlock()
		if err != nil && !errors.Is(err, context.Canceled) {
			s.log.Error("推流异常结束", "error", err, "ssrc", fmt.Sprintf("%08X", s.sender.SSRC()))
			return
		}
		packets, bytes := s.Stats()
		s.log.Info("推流结束", "ssrc", fmt.Sprintf("%08X", s.sender.SSRC()),
			"packets", packets, "bytes", bytes, "target", s.target())
	}()
	return nil
}

// Stop 停止推流并释放 UDP 连接。可重复调用。
//
// 每个 Streamer 只服务一次 INVITE 会话、不复用，因此 Stop 直接关掉连接 ——
// 否则每个 BYE 都留下一个占着端口的 socket，多轮点播后端口会被耗尽。
func (s *Streamer) Stop() {
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	s.closeOnce.Do(func() { _ = s.conn.Close() })
}

// Close 等价于 Stop，保留此名是为了让调用方读起来更自然。
func (s *Streamer) Close() error {
	s.Stop()
	return nil
}

// run 是推流主循环：按帧率把每个访问单元封成 PS 再经 RTP 发出。
func (s *Streamer) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if s.cfg.Duration > 0 {
		timer := time.AfterFunc(s.cfg.Duration, func() {
			s.log.Info("推流时长到期，自动停止", "duration", s.cfg.Duration.String())
			cancel()
		})
		defer timer.Stop()
	}

	// 每秒打印一次进度：排障时「设备到底推没推」全靠这几个数字（§13.3.3 配方第 3 步）。
	go s.reportLoop(ctx)

	fps := s.cfg.FPS
	pace := fps > 0
	if !pace {
		// 不限速时仍需要一个合理的 PTS 步进，否则时间戳完全不前进。
		fps = DefaultFPS
	}
	ptsStep := uint64(ps.ClockRate) / uint64(fps)

	var ticker *time.Ticker
	if pace {
		ticker = time.NewTicker(time.Second / time.Duration(fps))
		defer ticker.Stop()
	}

	// Muxer 的写出口直接接到 RTP 发送器：每个访问单元一次 Write，
	// 恰好对应一段 PS 字节流，用该帧的 PTS 作为 RTP 时间戳。
	w := &rtpWriter{sender: s.sender}
	muxer := ps.NewMuxer(w)

	var pts uint64
	s.log.Info("开始推流", "ssrc", fmt.Sprintf("%08X", s.sender.SSRC()),
		"target", s.target(), "fps", fps, "loop", s.cfg.Loop,
		"frames", len(s.units), "local_port", s.LocalRTPPort())

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		// 每轮（含循环回卷）重发一次 PS 头 + PSM：中途接入的解码器靠它识别基本流类型。
		if err := muxer.WritePSHeader(); err != nil {
			return err
		}
		for _, unit := range s.units {
			if err := ctx.Err(); err != nil {
				return err
			}
			w.setTimestamp(rtp.TimestampFromPTS(pts))
			if err := muxer.WriteAccessUnit(pts, unit); err != nil {
				return err
			}
			pts += ptsStep
			if ticker != nil {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
				}
			}
		}
		if !s.cfg.Loop {
			return nil
		}
	}
}

// reportLoop 每秒打印一次已发送的包数与字节数。
func (s *Streamer) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			packets, bytes := s.Stats()
			s.log.Info("推流进度",
				"ssrc", fmt.Sprintf("%08X", s.sender.SSRC()),
				"elapsed", time.Since(start).Round(time.Second).String(),
				"packets", packets, "bytes", bytes, "target", s.target())
		}
	}
}

// target 返回「平台收流地址」的可读表示，用于日志关联。
func (s *Streamer) target() string {
	return fmt.Sprintf("%s:%d", s.cfg.TargetIP, s.cfg.TargetPort)
}

// rtpWriter 把 ps.Muxer 的输出直接转成 RTP 包。
//
// ps.Muxer 的接口是 io.Writer，而 rtp.Sender 需要「一段 PS + 时间戳」；
// 两者之间的桥就是它：Muxer 每次 Write 的内容恰好是一个访问单元的完整 PS 字节流，
// 时间戳由推流循环在写入前 setTimestamp 指定。
type rtpWriter struct {
	sender *rtp.Sender
	mu     sync.Mutex
	ts     uint32
}

func (w *rtpWriter) setTimestamp(ts uint32) {
	w.mu.Lock()
	w.ts = ts
	w.mu.Unlock()
}

func (w *rtpWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	ts := w.ts
	w.mu.Unlock()
	sent, err := w.sender.SendPSBytes(p, ts)
	if err != nil {
		return 0, err
	}
	// 返回 len(p) 而非实际发出的字节数：io.Writer 的约定是「消费了多少输入」，
	// 这里输入已被完整封装进 RTP 包，返回短计数会让 Muxer 误判为写失败。
	_ = sent
	return len(p), nil
}
