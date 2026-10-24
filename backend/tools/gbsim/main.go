// Command gbsim 是一个 GB28181 智能相机模拟器，用作 AI Camera 平台的测试夹具与回归门禁
// （见 docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md §12）。
//
// 它的战略价值在于：自研 SIP 服务一行代码都没写时，先让模拟器对着已知正确的平台
// （WVP-Pro / 本项目 sipsvc）跑通，之后它就是一把可信的「尺子」——
// 排障时错误只可能在平台侧，调试空间立刻收敛一半。
//
// 本文件所在的是**第一批**交付内容，覆盖：
//
//	✅ SIP REGISTER（401 挑战 → Digest → 200）、周期 Keepalive、re-REGISTER
//	✅ 入向 MESSAGE 的 MANSCDP 解析与目录 / 设备信息 / 设备状态响应（含 GB2312）
//	✅ PS 封装器（Annex-B NAL → PES → PS）与 RTP 分包发送
//
// 本批新增：
//
//	✅ SIP over TCP（客户端长连接模型，不监听本地端口）
//	✅ 媒体 TCP-PASSIVE（主动连出 + RFC 4571 `$` 分帧）
//	✅ TCP-ACTIVE 的明确拒绝路径（平台单端口模式互斥）
//
// 尚未覆盖（后续批次）：云台与回放应答、AI 风险上报、
// `--spoof-message` 等异常注入、交互模式。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/boticz/ai-camera-backend/tools/gbsim/manscdp"
	"github.com/boticz/ai-camera-backend/tools/gbsim/ps"
	"github.com/boticz/ai-camera-backend/tools/gbsim/sipua"
	"github.com/boticz/ai-camera-backend/tools/gbsim/stream"
)

// exitUsage 是「参数用法错误」的退出码，与 flag 包的约定保持一致。
const exitUsage = 2

func main() {
	cfg, err := parseFlags(os.Args[1:], os.Stdout)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return // --help：用法已打印，正常退出
		}
		fmt.Fprintln(os.Stderr, "参数错误："+err.Error())
		os.Exit(exitUsage)
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "启动失败："+err.Error())
		os.Exit(1)
	}
}

// parseFlags 解析命令行参数。参数错误返回 error；--help 返回 flag.ErrHelp。
func parseFlags(args []string, out io.Writer) (*Config, error) {
	fs := flag.NewFlagSet("gbsim", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.Usage = func() {
		fmt.Fprint(out, usageText)
		fs.PrintDefaults()
	}

	var (
		cfg       Config
		showHelp  bool
		riskEvery = defaultRiskInterval
	)
	fs.StringVar(&cfg.DeviceID, "device-id", "", "20 位国标设备编码（必填，SIP From 与目录 DeviceID 都用它）")
	fs.StringVar(&cfg.DeviceName, "device-name", "模拟相机", "设备名称（中文名按 --charset 编码）")
	fs.StringVar(&cfg.SIPServer, "sip-server", "", "平台 SIP 地址 host:port（必填，如 127.0.0.1:5060）")
	fs.StringVar(&cfg.SIPDomain, "sip-domain", "", "SIP 域 / Digest realm（必填，如 3402000000）")
	fs.StringVar(&cfg.Password, "password", "", "设备 SIP 密码（对应 ac_device.sip_password）")
	fs.IntVar(&cfg.Channels, "channels", 1, "模拟的通道数量，≥1")
	fs.StringVar(&cfg.Mode, "mode", defaultMode, "媒体传输模式：udp / tcp-passive / tcp-active（tcp-active 与平台单端口收流互斥，仅用于验证拒绝路径）")
	fs.StringVar(&cfg.StreamFile, "stream-file", "", "H.264 Annex-B 样本文件路径；给了就加载并校验")
	fs.BoolVar(&cfg.Loop, "loop", true, "样本放完后从头再推（点播是持续流，默认开）")
	fs.IntVar(&cfg.FPS, "fps", defaultFPS, "推流帧率；0 表示不限速（仅压测用，正常别关）")
	fs.DurationVar(&cfg.Duration, "duration", 0, "每路推流推满该时长后自动停（0 = 不限制）")
	fs.StringVar(&cfg.SSRCOverride, "ssrc-override", "", "设备在 200 OK 里回一个与平台不同的 y=（验证平台的 SSRC 校正路径）")
	fs.StringVar(&cfg.Charset, "charset", defaultCharset, "XML 正文编码：GB2312 / UTF-8（对应 ac_device.charset）")
	fs.StringVar(&cfg.LocalIP, "local-ip", "", "本地信令 IP；留空则按到 --sip-server 的路由自动探测")
	fs.IntVar(&cfg.LocalPort, "local-port", defaultLocalPort, "本地信令端口（平台会把下行指令发到这里）")
	fs.StringVar(&cfg.Transport, "transport", defaultTransport, "信令传输：udp / tcp（tcp 只由 client 长连接 dial 出去，不监听本地端口）")
	fs.IntVar(&cfg.Expires, "expires", defaultExpires, "注册有效期（秒）")
	fs.DurationVar(&cfg.KeepaliveInterval, "keepalive", defaultKeepalive, "Keepalive 心跳发送周期")
	fs.DurationVar(&riskEvery, "risk-interval", defaultRiskInterval, "AI 风险上报周期（⚠️ 本批次尚未实现，仅接受参数）")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "输出 SIP / MANSCDP 报文级日志")
	fs.BoolVar(&cfg.Enable2022, "2022", true, "应答 GB28181-2022 查询（精准位置/巡航轨迹/看守位/存储卡）；false 模拟 2016 设备不应答")
	fs.BoolVar(&showHelp, "help", false, "显示帮助")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if showHelp {
		fs.Usage()
		return nil, flag.ErrHelp
	}
	cfg.RiskInterval = riskEvery
	if err := cfg.Validate(); err != nil {
		fs.Usage()
		return nil, err
	}
	return &cfg, nil
}

const usageText = `gbsim —— GB28181 智能相机模拟器（v0.2.0 第二批：SIP 注册 + MANSCDP + INVITE 应答 + 真实 RTP 推流）

用法:
  go run ./tools/gbsim \
    --device-id 34020000001320000001 \
    --sip-server 127.0.0.1:5060 \
    --sip-domain 3402000000 \
    --password '<设备 SIP 密码>' \
    --channels 2 \
    --mode udp \
    --stream-file testdata/sample.h264 \
    --fps 25 --loop --duration 0

参数:
`

// defaultRiskInterval 是 --risk-interval 的默认值（本批次仅为占位）。
const defaultRiskInterval = 30 * time.Second

// registerTimeout 是首次注册的整体超时。
// 若不设，SIP 的 UDP 事务会等满 Timer F（32 秒）才放弃，地址填错时体验很差。
const registerTimeout = 15 * time.Second

// run 是模拟器的主流程。
func run(cfg *Config) error {
	logger := newLogger(cfg.Verbose)

	localIP, err := cfg.ResolveLocalIP()
	if err != nil {
		return err
	}
	cfg.LocalIP = localIP

	channels := cfg.BuildChannels()
	printBanner(cfg, channels)

	// 样本码流在启动时加载一次，之后所有点播会话共用（只读，可并发）。
	var sampleNALs [][]byte
	if cfg.StreamFile != "" {
		nals, err := loadH264Sample(cfg.StreamFile)
		if err != nil {
			return err
		}
		sampleNALs = nals
		logger.Info("样本码流已加载", "file", cfg.StreamFile, "nals", len(nals),
			"access_units", len(ps.GroupAccessUnits(nals)))
	} else {
		logger.Warn("未提供 --stream-file，收到点播 INVITE 会回 488（没有可推的媒体源）")
	}
	switch strings.ToLower(cfg.Mode) {
	case ModeTCPActive:
		// 参数可解析，但当前平台架构下走不通：单端口模式不创建流级 RTP server，
		// ZLM 无法主动拨号设备。明确告警，收到点播时也会回 488。
		logger.Warn("--mode tcp-active：与平台单端口收流模式互斥，所有点播 INVITE 都会被回 488",
			"mode", cfg.Mode)
	case ModeTCPPassive:
		logger.Info("--mode tcp-passive：收到 a=setup:passive 的 INVITE 时主动连出，并按 RFC 4571 分帧",
			"mode", cfg.Mode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ua, err := sipua.New(sipua.Config{
		DeviceID:     cfg.DeviceID,
		SIPServer:    cfg.SIPServer,
		SIPDomain:    cfg.SIPDomain,
		Password:     cfg.Password,
		LocalIP:      cfg.LocalIP,
		LocalPort:    cfg.LocalPort,
		Transport:    cfg.Transport,
		Expires:      cfg.Expires,
		SSRCOverride: cfg.SSRCOverride,
		MediaMode:    cfg.Mode,
		Logger:       logger,
	})
	if err != nil {
		return err
	}
	defer ua.Close()

	dispatcher := &messageDispatcher{cfg: cfg, channels: channels, logger: logger}
	ua.SetMessageHandler(dispatcher.Handle)
	if len(sampleNALs) > 0 {
		ua.SetMediaFactory(newMediaFactory(cfg, sampleNALs, logger))
	}

	if err := ua.Start(ctx); err != nil {
		return err
	}

	// 首次注册给一个显式超时：SIP 的 UDP 事务默认要等满 Timer F（32 秒）才放弃，
	// 地址写错时让人干等半分钟。15 秒足够覆盖正常的 401 往返 + 慢平台。
	regCtx, cancelReg := context.WithTimeout(ctx, registerTimeout)
	err = ua.Register(regCtx)
	cancelReg()
	if err != nil {
		return fmt.Errorf("注册失败（%s 内未完成或平台拒绝）：%w", registerTimeout, err)
	}

	go ua.ReRegisterLoop(ctx)
	if cfg.KeepaliveInterval > 0 {
		go keepaliveLoop(ctx, ua, cfg, logger)
	}

	if cfg.RiskInterval > 0 {
		logger.Warn("--risk-interval 已接受但本批次尚未实现 AI 风险上报，参数暂不生效",
			"risk_interval", cfg.RiskInterval.String())
	}

	logger.Info("gbsim 已就绪，Ctrl+C 退出",
		"device_id", cfg.DeviceID, "aor", ua.AOR(), "local", ua.LocalAddr(),
		"channels", len(channels), "mode", cfg.Mode, "charset", cfg.Charset)

	<-ctx.Done()
	logger.Info("收到退出信号，发送注销 REGISTER")

	// 注销用一个短超时的独立上下文：主 ctx 已经取消了。
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := ua.Unregister(shutdownCtx); err != nil {
		logger.Warn("注销失败（不影响退出）", "error", err)
	}
	return nil
}

// newLogger 构造项目风格的日志器：verbose 时降到 Debug 级别。
func newLogger(verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// printBanner 打印启动摘要，便于在集中调试时一眼确认这次起的是哪台设备。
func printBanner(cfg *Config, channels []Channel) {
	fmt.Fprintf(os.Stderr, "gbsim 启动\n")
	fmt.Fprintf(os.Stderr, "  设备编码  %s（%s）\n", cfg.DeviceID, cfg.DeviceName)
	fmt.Fprintf(os.Stderr, "  平台地址  %s（域 %s）\n", cfg.SIPServer, cfg.SIPDomain)
	fmt.Fprintf(os.Stderr, "  本地信令  %s:%d（%s）\n", cfg.LocalIP, cfg.LocalPort, cfg.Transport)
	fmt.Fprintf(os.Stderr, "  注册有效期 %ds，心跳间隔 %s\n", cfg.Expires, cfg.KeepaliveInterval)
	fmt.Fprintf(os.Stderr, "  媒体模式  %s，编码 %s\n", cfg.Mode, cfg.Charset)
	if cfg.StreamFile != "" {
		fmt.Fprintf(os.Stderr, "  推流参数  %s，fps=%d，loop=%v，duration=%s\n",
			cfg.StreamFile, cfg.FPS, cfg.Loop, durationText(cfg.Duration))
	}
	if cfg.SSRCOverride != "" {
		fmt.Fprintf(os.Stderr, "  ⚠ SSRC 覆盖 %s（故意与平台不同，用于验证平台的校正路径）\n", cfg.SSRCOverride)
	}
	fmt.Fprintf(os.Stderr, "  通道 %d 个：\n", len(channels))
	for _, ch := range channels {
		fmt.Fprintf(os.Stderr, "    - %s %s\n", ch.DeviceID, ch.Name)
	}
}

// durationText 把推流时长渲染成可读文本（0 显示为「不限」）。
func durationText(d time.Duration) string {
	if d <= 0 {
		return "不限"
	}
	return d.String()
}

// loadH264Sample 读取并校验 Annex-B H.264 样本。
//
// 校验三条（对应交付要求）：以起始码开头、含 SPS/PPS、能被切出 NAL。
func loadH264Sample(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取样本码流 %s 失败：%w", path, err)
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("样本码流 %s 过短（%d 字节）", path, len(data))
	}
	hasStartCode := data[0] == 0x00 && data[1] == 0x00 &&
		(data[2] == 0x01 || (data[2] == 0x00 && data[3] == 0x01))
	if !hasStartCode {
		return nil, fmt.Errorf("样本码流 %s 不是 Annex-B 格式（应以 00 00 01 或 00 00 00 01 开头）", path)
	}

	nals := ps.SplitAnnexB(data)
	if len(nals) == 0 {
		return nil, fmt.Errorf("样本码流 %s 切不出任何 NAL", path)
	}
	var hasSPS, hasPPS bool
	for _, nal := range nals {
		switch ps.NALType(nal) {
		case ps.NALTypeSPS:
			hasSPS = true
		case ps.NALTypePPS:
			hasPPS = true
		}
	}
	if !hasSPS || !hasPPS {
		return nil, fmt.Errorf("样本码流 %s 缺少 SPS/PPS（SPS=%v PPS=%v），无法用于推流",
			path, hasSPS, hasPPS)
	}
	return nals, nil
}

// newMediaFactory 构造 sipua 需要的媒体工厂：每次点播 INVITE 都新建一个推流会话。
//
// 关键点：**推流目标与 SSRC 全部取自 INVITE 的 SDP**（info 里），
// 而不是 cfg 里的任何字段 —— 平台每次点播都可能分配不同的端口与 SSRC。
// cfg 只提供那些与单次点播无关的参数：样本、帧率、循环、时长。
func newMediaFactory(cfg *Config, nals [][]byte, logger *slog.Logger) sipua.MediaFactory {
	return func(info sipua.InviteInfo) (sipua.MediaSession, int, error) {
		ssrc, err := parseSSRC(info.DeviceSSRC)
		if err != nil {
			return nil, 0, err
		}
		st, err := stream.New(stream.Config{
			NALs:       nals,
			SSRC:       ssrc,
			TargetIP:   info.PlatformIP,   // ← 平台 SDP 的 c=
			TargetPort: info.PlatformPort, // ← 平台 SDP 的 m=
			LocalIP:    cfg.LocalIP,
			FPS:        cfg.FPS,
			Loop:       cfg.Loop,
			Duration:   cfg.Duration,
			// 传输模式以 INVITE 的 SDP 为准，不用 --mode：
			// 平台才是协商发起方，flag 只是设备的「配置声明」。
			Transport: info.Transport,
			Logger:    logger,
		})
		if err != nil {
			return nil, 0, err
		}
		return st, st.LocalRTPPort(), nil
	}
}

// parseSSRC 把国标 SSRC 的十进制字符串（10 位，如 "0200000001"）转成 uint32。
func parseSSRC(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("SSRC %q 不是合法的十进制 32 位无符号整数：%w", s, err)
	}
	return uint32(v), nil
}

// keepaliveLoop 周期性发送 MANSCDP Keepalive，维持平台侧的在线状态。
func keepaliveLoop(ctx context.Context, ua *sipua.UA, cfg *Config, logger *slog.Logger) {
	ticker := time.NewTicker(cfg.KeepaliveInterval)
	defer ticker.Stop()
	var sn atomic.Int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			body, err := manscdp.Encode(manscdp.NewKeepalive(cfg.DeviceID, nextSN(&sn)), cfg.Charset)
			if err != nil {
				logger.Error("构造 Keepalive 报文失败", "error", err)
				continue
			}
			if _, err := ua.SendMessage(ctx, body); err != nil {
				logger.Error("发送 Keepalive 失败", "error", err)
			}
		}
	}
}

// nextSN 生成 MANSCDP 的 SN（0-65535 循环递增）。
func nextSN(counter *atomic.Int64) int {
	return int(counter.Add(1) % 65536)
}

// catalogByteBudget 是单条目录响应 MESSAGE 的正文预算（字节）。
//
// 为什么按**字节**而不是按条数分片：早先按 100 条/页，实测 --channels 2 时
// 单条 GB2312 目录响应已约 950 字节，2 条就能顶到上限 —— 100 条纯属想当然。
// sipgo 的 UDP 传输层在报文超过 UDPMTUSize-200（默认 1300）时直接返回
// ErrUDPMTUCongestion，表现是「`--channels 2` 时目录一条都发不出去」。
//
// 取 700 是实测出来的，不是拍的：2 个通道的 GB2312 目录正文 950 字节，
// 加上 SIP 头（From/To/Via/Call-ID/Content-Type 等约 350 字节）后整包约 1300，
// 正好顶到 sipgo 的 UDPMTUSize-200 = 1300 上限并报错。
// 正文预算必须按「整包」算，只盯正文大小会重蹈覆辙。
// 注意：这是**模拟器**的稳健性措施；真实设备的目录分片策略由厂商决定，
// 平台侧必须能处理任意分片，见 protocol/manscdp 的 (deviceID, SN) 重组。
const catalogByteBudget = 700

// messageDispatcher 把入向 MANSCDP 报文路由成对应的响应报文。
//
// 它只负责「构造正文」，实际发送由 sipua.UA 在回完 200 OK 后完成。
type messageDispatcher struct {
	cfg      *Config
	channels []Channel
	logger   *slog.Logger
}

// Handle 实现 sipua.MessageHandler。
func (d *messageDispatcher) Handle(body []byte, contentType string) [][]byte {
	env, err := manscdp.CmdTypeOf(body, d.cfg.Charset)
	if err != nil {
		// 解析失败必须显式记日志：这正是「设备注册上来了但目录一条都没有」的现场
		// （CHARSET 不匹配 / 报文畸形），静默丢弃会让排障从零开始。
		d.logger.Warn("丢弃无法解析的 MANSCDP 报文",
			"error", err, "content_type", contentType, "bytes", len(body))
		return nil
	}
	d.logger.Debug("收到 MANSCDP", "cmd_type", env.CmdType, "sn", env.SN, "device_id", env.DeviceID)

	switch env.CmdType {
	case manscdp.CmdCatalog:
		return d.catalogReplies(env.SN)
	case manscdp.CmdDeviceInfo:
		return d.one(d.encodeDeviceInfo(env.SN))
	case manscdp.CmdDeviceStatus:
		return d.one(d.encodeDeviceStatus(env.SN))

	// ── GB/T 28181-2022 查询 ──
	// 默认应答；--2022=false 时可用来模拟 2016 设备（平台应在发报文前拦截，
	// 因此正常情况下这里根本收不到这些 CmdType）。
	case manscdp.CmdPTZPosition:
		if !d.allow2022(env.CmdType) {
			return nil
		}
		return d.one(d.encodePTZPosition(env.SN, env.DeviceID))
	case manscdp.CmdCruiseTrackListQuery:
		if !d.allow2022(env.CmdType) {
			return nil
		}
		return d.one(d.encodeCruiseTrackList(env.SN, env.DeviceID))
	case manscdp.CmdCruiseTrackQuery:
		if !d.allow2022(env.CmdType) {
			return nil
		}
		// 单条轨迹查询要读 Number；解码失败时按 0（第一条）处理并告警，
		// 避免因为一个可选字段让整条应答消失。
		var q manscdp.CruiseTrackQuery
		if err := manscdp.Decode(body, d.cfg.Charset, &q); err != nil {
			d.logger.Warn("解析 CruiseTrackQuery 失败，按轨迹 0 处理", "error", err)
		}
		return d.one(d.encodeCruiseTrack(env.SN, env.DeviceID, q.Number))
	case manscdp.CmdHomePositionQuery:
		if !d.allow2022(env.CmdType) {
			return nil
		}
		return d.one(d.encodeHomePosition(env.SN, env.DeviceID))
	case manscdp.CmdSDCardStatus:
		if !d.allow2022(env.CmdType) {
			return nil
		}
		return d.one(d.encodeSDCardStatus(env.SN, env.DeviceID))

	default:
		d.logger.Warn("暂未实现的 MANSCDP 指令（后续批次补充）", "cmd_type", env.CmdType)
		return nil
	}
}

// allow2022 判断是否应答 2022 查询。
func (d *messageDispatcher) allow2022(cmdType string) bool {
	if d.cfg != nil && d.cfg.Enable2022 {
		return true
	}
	d.logger.Info("收到 2022 查询但 --2022=false，模拟 2016 设备不应答", "cmd_type", cmdType)
	return false
}

// encodePTZPosition 编码 PTZ 精准位置应答（固定值，仅用于回归）。
func (d *messageDispatcher) encodePTZPosition(sn int, deviceID string) []byte {
	body, err := manscdp.Encode(&manscdp.PTZPositionResponse{
		CmdType:  manscdp.CmdPTZPosition,
		SN:       sn,
		DeviceID: deviceID,
		Pan:      12.5,
		Tilt:     30,
		Zoom:     2,
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码 PTZPosition 响应失败", "error", err)
		return nil
	}
	return body
}

// encodeCruiseTrackList 编码巡航轨迹列表应答（固定 2 条）。
func (d *messageDispatcher) encodeCruiseTrackList(sn int, deviceID string) []byte {
	tracks := []manscdp.CruiseTrack{
		{Number: 0, Name: "东门巡航"},
		{Number: 1, Name: "仓库巡航"},
	}
	body, err := manscdp.Encode(&manscdp.CruiseTrackListResponse{
		CmdType:  manscdp.CmdCruiseTrackListQuery,
		SN:       sn,
		DeviceID: deviceID,
		SumNum:   len(tracks),
		CruiseTrackList: &manscdp.CruiseTrackList{
			Num:    len(tracks),
			Tracks: tracks,
		},
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码 CruiseTrackList 响应失败", "error", err)
		return nil
	}
	return body
}

// encodeCruiseTrack 编码单条轨迹明细（固定 2 个巡航点）。
func (d *messageDispatcher) encodeCruiseTrack(sn int, deviceID string, number int) []byte {
	points := []manscdp.CruisePoint{
		{PresetIndex: 1, StayTime: 10, Speed: 5},
		{PresetIndex: 2, StayTime: 20, Speed: 6},
	}
	name := "东门巡航"
	if number != 0 {
		name = fmt.Sprintf("巡航轨迹%d", number)
	}
	body, err := manscdp.Encode(&manscdp.CruiseTrackResponse{
		CmdType:  manscdp.CmdCruiseTrackQuery,
		SN:       sn,
		DeviceID: deviceID,
		Number:   number,
		Name:     name,
		CruisePointList: &manscdp.CruisePointList{
			Num:    len(points),
			Points: points,
		},
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码 CruiseTrack 响应失败", "error", err)
		return nil
	}
	return body
}

// encodeHomePosition 编码看守位应答（启用、预置位 1、间隔 300 秒）。
func (d *messageDispatcher) encodeHomePosition(sn int, deviceID string) []byte {
	body, err := manscdp.Encode(&manscdp.HomePositionResponse{
		CmdType:  manscdp.CmdHomePositionQuery,
		SN:       sn,
		DeviceID: deviceID,
		HomePosition: &manscdp.HomePosition{
			Enabled:     1,
			ResetTime:   300,
			PresetIndex: 1,
		},
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码 HomePosition 响应失败", "error", err)
		return nil
	}
	return body
}

// encodeSDCardStatus 编码存储卡状态应答（1 张卡，32GB，剩余 16GB）。
func (d *messageDispatcher) encodeSDCardStatus(sn int, deviceID string) []byte {
	cards := []manscdp.SDCardInfo{
		{Capacity: 32768, FreeSpace: 16384, Status: 0},
	}
	body, err := manscdp.Encode(&manscdp.SDCardStatusResponse{
		CmdType:  manscdp.CmdSDCardStatus,
		SN:       sn,
		DeviceID: deviceID,
		SDCardStatusInfo: &manscdp.SDCardStatusInfo{
			Num:            len(cards),
			Status:         0,
			FormatProgress: 0,
			Cards:          cards,
		},
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码 SDCardStatus 响应失败", "error", err)
		return nil
	}
	return body
}

// one 把一条正文包成切片，便于统一返回类型。
func (d *messageDispatcher) one(body []byte) [][]byte {
	if body == nil {
		return nil
	}
	return [][]byte{body}
}

// catalogReplies 构造目录响应，按编码后**字节数**分片。
//
// 分片复用的是**同一个 SN**（平台按 (deviceId, SN) 重组，§6.2 #3），
// SumNum 恒为总通道数，DeviceList 的 Num 是本片条数。
//
// 每片至少放 1 条 —— 单条就超预算时宁可发超（让平台侧暴露问题），
// 也不能因为「永远放不下」而丢掉通道、制造「设备少报通道」的假象。
func (d *messageDispatcher) catalogReplies(sn int) [][]byte {
	total := len(d.channels)
	if total == 0 {
		// 空目录也要回一条 SumNum=0，否则平台会一直等到超时。
		return d.one(d.encodeCatalog(sn, nil, 0))
	}
	var out [][]byte
	for start := 0; start < total; {
		end := start + 1
		body := d.encodeCatalog(sn, d.channels[start:end], total)
		for end < total {
			next := d.encodeCatalog(sn, d.channels[start:end+1], total)
			if len(next) > catalogByteBudget {
				break
			}
			end++
			body = next
		}
		out = append(out, body)
		start = end
	}
	return out
}

// encodeCatalog 编码一条目录响应；channels 为本片通道，total 为总通道数。
func (d *messageDispatcher) encodeCatalog(sn int, channels []Channel, total int) []byte {
	items := make([]manscdp.CatalogItem, 0, len(channels))
	for _, ch := range channels {
		items = append(items, manscdp.CatalogItem{
			DeviceID:     ch.DeviceID,
			Name:         ch.Name,
			Manufacturer: defaultManufacturer,
			Model:        defaultDeviceModel,
			Owner:        "Owner",
			CivilCode:    ch.DeviceID[:8],
			Address:      d.cfg.DeviceName,
			Parental:     0,
			ParentID:     d.cfg.DeviceID,
			RegisterWay:  1,
			Secrecy:      0,
			Status:       "ON",
			PTZType:      ch.PTZType,
		})
	}
	body, err := manscdp.Encode(&manscdp.Catalog{
		CmdType:  manscdp.CmdCatalog,
		SN:       sn,
		DeviceID: d.cfg.DeviceID,
		SumNum:   total,
		DeviceList: manscdp.DeviceList{
			Num:   len(items),
			Items: items,
		},
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码目录响应失败", "error", err)
		return nil
	}
	return body
}

// encodeDeviceInfo 编码设备信息响应。
func (d *messageDispatcher) encodeDeviceInfo(sn int) []byte {
	body, err := manscdp.Encode(&manscdp.DeviceInfo{
		CmdType:      manscdp.CmdDeviceInfo,
		SN:           sn,
		DeviceID:     d.cfg.DeviceID,
		Result:       "OK",
		DeviceName:   d.cfg.DeviceName,
		Manufacturer: defaultManufacturer,
		Model:        defaultDeviceModel,
		Firmware:     defaultFirmware,
		Channel:      len(d.channels),
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码设备信息响应失败", "error", err)
		return nil
	}
	return body
}

// encodeDeviceStatus 编码设备状态响应。
func (d *messageDispatcher) encodeDeviceStatus(sn int) []byte {
	body, err := manscdp.Encode(&manscdp.DeviceStatus{
		CmdType:    manscdp.CmdDeviceStatus,
		SN:         sn,
		DeviceID:   d.cfg.DeviceID,
		Result:     "OK",
		Online:     "ONLINE",
		Status:     "OK",
		Encode:     "H264",
		DeviceTime: time.Now().Format("2006-01-02T15:04:05"),
	}, d.cfg.Charset)
	if err != nil {
		d.logger.Error("编码设备状态响应失败", "error", err)
		return nil
	}
	return body
}
