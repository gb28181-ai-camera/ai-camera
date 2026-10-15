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
// 尚未覆盖（后续批次）：TCP-PASSIVE / TCP-ACTIVE、INVITE 应答与真实推流、
// 云台与回放应答、AI 风险上报、`--spoof-message` 等异常注入、交互模式。
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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/boticz/ai-camera-backend/tools/gbsim/manscdp"
	"github.com/boticz/ai-camera-backend/tools/gbsim/ps"
	"github.com/boticz/ai-camera-backend/tools/gbsim/sipua"
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
	fs.StringVar(&cfg.Mode, "mode", defaultMode, "媒体传输模式：udp / tcp-passive / tcp-active（本批次仅 udp）")
	fs.StringVar(&cfg.StreamFile, "stream-file", "", "H.264 Annex-B 样本文件路径；给了就加载并校验")
	fs.StringVar(&cfg.Charset, "charset", defaultCharset, "XML 正文编码：GB2312 / UTF-8（对应 ac_device.charset）")
	fs.StringVar(&cfg.LocalIP, "local-ip", "", "本地信令 IP；留空则按到 --sip-server 的路由自动探测")
	fs.IntVar(&cfg.LocalPort, "local-port", defaultLocalPort, "本地信令端口（平台会把下行指令发到这里）")
	fs.StringVar(&cfg.Transport, "transport", defaultTransport, "信令传输（本批次仅 udp；tcp 需长连接模型，未实现）")
	fs.IntVar(&cfg.Expires, "expires", defaultExpires, "注册有效期（秒）")
	fs.DurationVar(&cfg.KeepaliveInterval, "keepalive", defaultKeepalive, "Keepalive 心跳发送周期")
	fs.DurationVar(&riskEvery, "risk-interval", defaultRiskInterval, "AI 风险上报周期（⚠️ 本批次尚未实现，仅接受参数）")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "输出 SIP / MANSCDP 报文级日志")
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

const usageText = `gbsim —— GB28181 智能相机模拟器（v0.2.0 第一批：SIP 注册 + MANSCDP + PS/RTP 封装）

用法:
  go run ./tools/gbsim \
    --device-id 34020000001320000001 \
    --sip-server 127.0.0.1:5060 \
    --sip-domain 3402000000 \
    --password '<设备 SIP 密码>' \
    --channels 2 \
    --mode udp \
    --stream-file testdata/sample.h264 \
    --risk-interval 30s

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

	if cfg.StreamFile != "" {
		nals, err := loadH264Sample(cfg.StreamFile)
		if err != nil {
			return err
		}
		logger.Info("样本码流已加载", "file", cfg.StreamFile, "nals", len(nals),
			"access_units", len(ps.GroupAccessUnits(nals)))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ua, err := sipua.New(sipua.Config{
		DeviceID:  cfg.DeviceID,
		SIPServer: cfg.SIPServer,
		SIPDomain: cfg.SIPDomain,
		Password:  cfg.Password,
		LocalIP:   cfg.LocalIP,
		LocalPort: cfg.LocalPort,
		Transport: cfg.Transport,
		Expires:   cfg.Expires,
		Logger:    logger,
	})
	if err != nil {
		return err
	}
	defer ua.Close()

	dispatcher := &messageDispatcher{cfg: cfg, channels: channels, logger: logger}
	ua.SetMessageHandler(dispatcher.Handle)

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
	fmt.Fprintf(os.Stderr, "  通道 %d 个：\n", len(channels))
	for _, ch := range channels {
		fmt.Fprintf(os.Stderr, "    - %s %s\n", ch.DeviceID, ch.Name)
	}
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

// catalogPageSize 是单条目录响应 MESSAGE 里携带的最大通道数。
//
// 目录响应按 SumNum 跨多条 MESSAGE 分片（§12.1）；取 100 是为了让
// 单条 SIP MESSAGE 稳稳落在 UDP MTU 之内 —— sipgo 的 UDP 传输层在报文超过
// UDPMTUSize-200 时会直接返回 ErrUDPMTUCongestion，分片不足会导致大目录设备同步失败。
const catalogPageSize = 100

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
	default:
		d.logger.Warn("暂未实现的 MANSCDP 指令（后续批次补充）", "cmd_type", env.CmdType)
		return nil
	}
}

// one 把一条正文包成切片，便于统一返回类型。
func (d *messageDispatcher) one(body []byte) [][]byte {
	if body == nil {
		return nil
	}
	return [][]byte{body}
}

// catalogReplies 构造目录响应，按 catalogPageSize 分片。
//
// 分片复用的是**同一个 SN**（平台按 (deviceId, SN) 重组，§6.2 #3），
// SumNum 恒为总通道数，DeviceList 的 Num 是本片条数。
func (d *messageDispatcher) catalogReplies(sn int) [][]byte {
	total := len(d.channels)
	if total == 0 {
		// 空目录也要回一条 SumNum=0，否则平台会一直等到超时。
		return d.one(d.encodeCatalog(sn, nil, 0))
	}
	var out [][]byte
	for start := 0; start < total; start += catalogPageSize {
		end := min(start+catalogPageSize, total)
		out = append(out, d.encodeCatalog(sn, d.channels[start:end], total))
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
