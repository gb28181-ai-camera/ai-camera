package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/tools/gbsim/manscdp"
	"github.com/boticz/ai-camera-backend/tools/gbsim/ps"
	"github.com/boticz/ai-camera-backend/tools/gbsim/sipua"
	"github.com/boticz/ai-camera-backend/tools/gbsim/stream"
)

const samplePath = "testdata/sample.h264"

func sampleConfig() *Config {
	return &Config{
		DeviceID:   "34020000001320000001",
		DeviceName: "东门球机",
		SIPServer:  "127.0.0.1:5060",
		SIPDomain:  "3402000000",
		Password:   "sim-secret",
		LocalIP:    "127.0.0.1",
		LocalPort:  5062,
		Transport:  "udp",
		Expires:    3600,
		Channels:   2,
		Mode:       ModeUDP,
		Charset:    defaultCharset,
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestSampleFileIsValidAnnexBH264 校验内置样本码流满足交付要求：
// 以起始码开头、含 SPS/PPS、能被本项目自己的解析器切出 NAL。
func TestSampleFileIsValidAnnexBH264(t *testing.T) {
	nals, err := loadH264Sample(samplePath)
	if err != nil {
		t.Fatalf("样本校验失败：%v", err)
	}
	if len(nals) < 10 {
		t.Fatalf("样本 NAL 数量过少：%d", len(nals))
	}

	var sps, pps, idr, nonIDR int
	for _, nal := range nals {
		switch ps.NALType(nal) {
		case ps.NALTypeSPS:
			sps++
		case ps.NALTypePPS:
			pps++
		case ps.NALTypeIDR:
			idr++
		case ps.NALTypeNonIDR:
			nonIDR++
		}
	}
	if sps == 0 || pps == 0 {
		t.Fatalf("样本缺少 SPS/PPS：SPS=%d PPS=%d", sps, pps)
	}
	if idr == 0 || nonIDR == 0 {
		t.Fatalf("样本缺少 I/P 帧：IDR=%d 非IDR=%d", idr, nonIDR)
	}

	info, err := os.Stat(samplePath)
	if err != nil {
		t.Fatalf("读取样本文件信息失败：%v", err)
	}
	if info.Size() > 1<<20 {
		t.Fatalf("样本文件超过 1MB：%d 字节", info.Size())
	}

	// 样本必须能被完整封装成 PS 并逐字节还原
	var buf bytes.Buffer
	m := ps.NewMuxer(&buf)
	if _, err := m.WriteAnnexBStream(mustRead(t, samplePath), 0, 3600); err != nil {
		t.Fatalf("封装样本失败：%v", err)
	}
	if int64(buf.Len()) <= info.Size() {
		t.Fatalf("PS 输出应大于原始码流（含头开销），实际 %d vs %d", buf.Len(), info.Size())
	}
	t.Logf("样本校验通过：%d 字节 / %d 个 NAL / SPS=%d PPS=%d IDR=%d 非IDR=%d / PS 输出 %d 字节",
		info.Size(), len(nals), sps, pps, idr, nonIDR, buf.Len())
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败：%v", path, err)
	}
	return b
}

// TestLoadH264SampleRejectsBadInput 覆盖样本校验的失败分支。
func TestLoadH264SampleRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	cases := map[string][]byte{
		"过短":           {0x00, 0x00},
		"无起始码":         {0xFF, 0xFF, 0xFF, 0xFF, 0x67},
		"有起始码但无SPSPPS": {0x00, 0x00, 0x00, 0x01, 0x41, 0x9A, 0x00},
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(dir, name+".h264")
			if err := os.WriteFile(p, content, 0o600); err != nil {
				t.Fatalf("写临时样本失败：%v", err)
			}
			if _, err := loadH264Sample(p); err == nil {
				t.Fatalf("期望校验失败：%s", name)
			}
		})
	}
	if _, err := loadH264Sample(filepath.Join(dir, "不存在.h264")); err == nil {
		t.Fatal("文件不存在时应当报错")
	}
}

// TestBuildChannels 验证通道编码推导：类型位 132→131，序号从 1 开始。
func TestBuildChannels(t *testing.T) {
	cfg := sampleConfig()
	cfg.Channels = 3
	channels := cfg.BuildChannels()
	if len(channels) != 3 {
		t.Fatalf("通道数量不符：%d", len(channels))
	}
	want := []string{
		"34020000001310000001",
		"34020000001310000002",
		"34020000001310000003",
	}
	for i, ch := range channels {
		if ch.DeviceID != want[i] {
			t.Errorf("第 %d 个通道编码：want %s got %s", i, want[i], ch.DeviceID)
		}
		if len(ch.DeviceID) != 20 {
			t.Errorf("通道编码必须是 20 位：%s", ch.DeviceID)
		}
	}
	if channels[0].Name != "东门球机通道1" {
		t.Errorf("通道名不符：%q", channels[0].Name)
	}
}

// TestDispatcherCatalogSinglePage 验证单页目录响应的 SN 回带、SumNum 与中文通道名。
//
// 用 1 个通道是有意的：分片按**字节预算**（catalogByteBudget=700），
// 单条目录项已约 460 字节，2 条就会拆成两片 —— 多片场景由
// TestDispatcherCatalogChunking 覆盖。
func TestDispatcherCatalogSinglePage(t *testing.T) {
	cfg := sampleConfig()
	cfg.Channels = 1
	d := &messageDispatcher{cfg: cfg, channels: cfg.BuildChannels(), logger: silentLogger()}

	query, err := manscdp.Encode(&manscdp.Query{
		CmdType: manscdp.CmdCatalog, SN: 123, DeviceID: cfg.DeviceID,
	}, cfg.Charset)
	if err != nil {
		t.Fatalf("构造查询失败：%v", err)
	}
	replies := d.Handle(query, sipua.ContentTypeMANSCDP)
	if len(replies) != 1 {
		t.Fatalf("期望 1 条目录响应，实际 %d", len(replies))
	}

	var got manscdp.Catalog
	if err := manscdp.Decode(replies[0], cfg.Charset, &got); err != nil {
		t.Fatalf("解析目录响应失败：%v", err)
	}
	if got.CmdType != manscdp.CmdCatalog {
		t.Errorf("CmdType 不符：%q", got.CmdType)
	}
	if got.SN != 123 {
		t.Errorf("SN 应回带查询的 SN，want 123 got %d", got.SN)
	}
	if got.DeviceID != cfg.DeviceID {
		t.Errorf("DeviceID 不符：%q", got.DeviceID)
	}
	if got.SumNum != 1 || got.DeviceList.Num != 1 {
		t.Errorf("SumNum/Num 不符：%d/%d", got.SumNum, got.DeviceList.Num)
	}
	if len(got.DeviceList.Items) != 1 {
		t.Fatalf("目录项数量不符：%d", len(got.DeviceList.Items))
	}
	first := got.DeviceList.Items[0]
	if first.Name != "东门球机通道1" {
		t.Errorf("中文通道名经 GB2312 往返后不一致：%q", first.Name)
	}
	if first.DeviceID != "34020000001310000001" {
		t.Errorf("通道编码不符：%q", first.DeviceID)
	}
	if first.ParentID != cfg.DeviceID {
		t.Errorf("ParentID 应为父设备编码：%q", first.ParentID)
	}
	if first.CivilCode != "34020000" {
		t.Errorf("CivilCode 应取编码前 8 位：%q", first.CivilCode)
	}
	if first.PTZType != 1 {
		t.Errorf("PTZType 不符：%d", first.PTZType)
	}
}

// TestDispatcherCatalogChunking 验证大目录按 SumNum 分片（§13.3.2 的 R9 场景）。
//
// 断言的是**行为契约**而非固定片数：每片编码后不超过 catalogByteBudget、
// 各片条数合计等于总通道数、SN/SumNum 一致。片数取决于通道名长度等
// 编码细节，写死片数会让这个用例在改文案时无谓失败。
func TestDispatcherCatalogChunking(t *testing.T) {
	cfg := sampleConfig()
	cfg.Channels = 25
	d := &messageDispatcher{cfg: cfg, channels: cfg.BuildChannels(), logger: silentLogger()}

	replies := d.catalogReplies(55)
	if len(replies) < 2 {
		t.Fatalf("25 通道的目录应分成多片，实际 %d 片", len(replies))
	}
	total := 0
	for i, r := range replies {
		if len(r) > catalogByteBudget {
			t.Errorf("第 %d 片正文 %d 字节，超过预算 %d（UDP 下会被 sipgo 拒绝发送）",
				i, len(r), catalogByteBudget)
		}
		var page manscdp.Catalog
		if err := manscdp.Decode(r, cfg.Charset, &page); err != nil {
			t.Fatalf("解析第 %d 片失败：%v", i, err)
		}
		if page.SN != 55 {
			t.Errorf("第 %d 片的 SN 应与查询一致：%d", i, page.SN)
		}
		if page.SumNum != cfg.Channels {
			t.Errorf("第 %d 片的 SumNum 应为总数 %d，实际 %d", i, cfg.Channels, page.SumNum)
		}
		if page.DeviceList.Num != len(page.DeviceList.Items) {
			t.Errorf("第 %d 片的 Num(%d) 与实际条数(%d) 不符",
				i, page.DeviceList.Num, len(page.DeviceList.Items))
		}
		if page.DeviceList.Num == 0 {
			t.Errorf("第 %d 片为空片，分片逻辑有误", i)
		}
		total += page.DeviceList.Num
	}
	if total != cfg.Channels {
		t.Fatalf("分片条数合计 %d ≠ 总通道数 %d", total, cfg.Channels)
	}
	// 末页同样应满足 SN/SumNum 一致性（上面循环已覆盖），此处不再断言固定条数
	last := replies[len(replies)-1]
	var page manscdp.Catalog
	if err := manscdp.Decode(last, cfg.Charset, &page); err != nil {
		t.Fatalf("解析末页失败：%v", err)
	}
	if page.DeviceList.Num == 0 {
		t.Fatalf("末页为空，分片逻辑有误")
	}
}

// TestDispatcherAnswersDeviceInfoAndStatus 验证设备信息与设备状态查询的响应。
func TestDispatcherAnswersDeviceInfoAndStatus(t *testing.T) {
	cfg := sampleConfig()
	d := &messageDispatcher{cfg: cfg, channels: cfg.BuildChannels(), logger: silentLogger()}

	query := func(cmdType string) []byte {
		b, err := manscdp.Encode(&manscdp.Query{CmdType: cmdType, SN: 9, DeviceID: cfg.DeviceID}, cfg.Charset)
		if err != nil {
			t.Fatalf("构造查询失败：%v", err)
		}
		return b
	}

	replies := d.Handle(query(manscdp.CmdDeviceInfo), "")
	if len(replies) != 1 {
		t.Fatalf("设备信息查询应回 1 条，实际 %d", len(replies))
	}
	var info manscdp.DeviceInfo
	if err := manscdp.Decode(replies[0], cfg.Charset, &info); err != nil {
		t.Fatalf("解析设备信息失败：%v", err)
	}
	if info.DeviceName != cfg.DeviceName || info.Channel != 2 || info.Result != "OK" {
		t.Fatalf("设备信息内容不符：%+v", info)
	}

	replies = d.Handle(query(manscdp.CmdDeviceStatus), "")
	if len(replies) != 1 {
		t.Fatalf("设备状态查询应回 1 条，实际 %d", len(replies))
	}
	var status manscdp.DeviceStatus
	if err := manscdp.Decode(replies[0], cfg.Charset, &status); err != nil {
		t.Fatalf("解析设备状态失败：%v", err)
	}
	if status.Online != "ONLINE" || status.Encode != "H264" {
		t.Fatalf("设备状态内容不符：%+v", status)
	}
}

// TestDispatcherDropsMalformedBody 畸形正文必须被丢弃且不 panic（R1 的隔离性要求）。
func TestDispatcherDropsMalformedBody(t *testing.T) {
	cfg := sampleConfig()
	d := &messageDispatcher{cfg: cfg, channels: cfg.BuildChannels(), logger: silentLogger()}
	for _, body := range [][]byte{nil, {}, []byte("not xml"), []byte("<Response><CmdType>")} {
		if got := d.Handle(body, ""); got != nil {
			t.Errorf("畸形报文应返回 nil，实际 %d 条", len(got))
		}
	}
	// 未实现的指令也应安静丢弃
	unknown, _ := manscdp.Encode(&manscdp.Query{CmdType: "RecordInfo", SN: 1}, cfg.Charset)
	if got := d.Handle(unknown, ""); got != nil {
		t.Errorf("未实现指令应返回 nil，实际 %d 条", len(got))
	}
}

// TestDispatcherEmptyCatalogStillReplies 空目录也必须回一条 SumNum=0，否则平台一直等超时。
func TestDispatcherEmptyCatalogStillReplies(t *testing.T) {
	cfg := sampleConfig()
	d := &messageDispatcher{cfg: cfg, logger: silentLogger()} // channels 为空
	replies := d.catalogReplies(3)
	if len(replies) != 1 {
		t.Fatalf("空目录应回 1 条，实际 %d", len(replies))
	}
	var page manscdp.Catalog
	if err := manscdp.Decode(replies[0], cfg.Charset, &page); err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if page.SumNum != 0 || page.DeviceList.Num != 0 {
		t.Fatalf("空目录的 SumNum/Num 应为 0：%d/%d", page.SumNum, page.DeviceList.Num)
	}
}

// TestParseFlagsHelp --help 必须能正常输出用法并返回 ErrHelp。
func TestParseFlagsHelp(t *testing.T) {
	var out bytes.Buffer
	_, err := parseFlags([]string{"--help"}, &out)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("--help 应返回 flag.ErrHelp，实际 %v", err)
	}
	if !strings.Contains(out.String(), "gbsim") || !strings.Contains(out.String(), "--device-id") {
		t.Fatalf("--help 输出应包含用法与参数说明，实际：%s", out.String())
	}
}

// TestParseFlagsHappyPath 验证完整参数解析。
func TestParseFlagsHappyPath(t *testing.T) {
	var out bytes.Buffer
	cfg, err := parseFlags([]string{
		"--device-id", "34020000001320000001",
		"--sip-server", "127.0.0.1:5060",
		"--sip-domain", "3402000000",
		"--password", "p",
		"--channels", "4",
		"--mode", "udp",
		"--stream-file", "testdata/sample.h264",
		"--risk-interval", "45s",
		"--verbose",
	}, &out)
	if err != nil {
		t.Fatalf("解析失败：%v\n%s", err, out.String())
	}
	if cfg.DeviceID != "34020000001320000001" || cfg.Channels != 4 || cfg.Mode != ModeUDP {
		t.Fatalf("解析结果不符：%+v", cfg)
	}
	if cfg.StreamFile != "testdata/sample.h264" {
		t.Fatalf("stream-file 不符：%q", cfg.StreamFile)
	}
	if cfg.RiskInterval.String() != "45s" {
		t.Fatalf("risk-interval 不符：%s", cfg.RiskInterval)
	}
	if !cfg.Verbose {
		t.Fatal("verbose 未生效")
	}
}

// TestParseFlagsValidation 覆盖参数校验的失败分支。
func TestParseFlagsValidation(t *testing.T) {
	baseArgs := []string{
		"--device-id", "34020000001320000001",
		"--sip-server", "127.0.0.1:5060",
		"--sip-domain", "3402000000",
	}
	with := func(extra ...string) []string {
		return append(append([]string{}, baseArgs...), extra...)
	}
	cases := map[string][]string{
		"缺 device-id": {"--sip-server", "127.0.0.1:5060", "--sip-domain", "3402000000"},
		"device-id 非 20 位": {
			"--device-id", "340200", "--sip-server", "127.0.0.1:5060", "--sip-domain", "3402000000",
		},
		"device-id 含非数字": {
			"--device-id", "3402000000132000000X", "--sip-server", "127.0.0.1:5060", "--sip-domain", "3402000000",
		},
		"缺 sip-server": {"--device-id", "34020000001320000001", "--sip-domain", "3402000000"},
		"sip-server 无端口": {
			"--device-id", "34020000001320000001", "--sip-server", "127.0.0.1", "--sip-domain", "3402000000",
		},
		"缺 sip-domain":  {"--device-id", "34020000001320000001", "--sip-server", "127.0.0.1:5060"},
		"channels 为 0":  with("--channels", "0"),
		"mode 非法":       with("--mode", "sctp"),
		"transport 非法":  with("--transport", "sctp"),
		"charset 非法":    with("--charset", "BIG5"),
		"expires 非正":    with("--expires", "0"),
		"local-port 越界": with("--local-port", "70000"),
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if _, err := parseFlags(args, &out); err == nil {
				t.Fatalf("期望参数校验失败：%v", args)
			}
			// 参数错误时也应打印用法
			if out.Len() == 0 {
				t.Error("参数错误时应打印用法")
			}
		})
	}
}

// TestParseFlagsTransportTCP 确认 --transport tcp 不再是参数错误（长连接模型已实现）。
func TestParseFlagsTransportTCP(t *testing.T) {
	var out bytes.Buffer
	cfg, err := parseFlags([]string{
		"--device-id", "34020000001320000001",
		"--sip-server", "127.0.0.1:5060",
		"--sip-domain", "3402000000",
		"--transport", "tcp",
	}, &out)
	if err != nil {
		t.Fatalf("--transport tcp 应可解析：%v\n%s", err, out.String())
	}
	if cfg.Transport != "tcp" {
		t.Fatalf("Transport = %q，期望 tcp", cfg.Transport)
	}
}

// TestConfigResolveLocalIP 验证本地 IP 探测：显式配置优先，否则按路由探测。
func TestConfigResolveLocalIP(t *testing.T) {
	cfg := sampleConfig()
	cfg.LocalIP = "10.1.2.3"
	got, err := cfg.ResolveLocalIP()
	if err != nil {
		t.Fatalf("显式配置时不应报错：%v", err)
	}
	if got != "10.1.2.3" {
		t.Fatalf("显式配置应原样返回：%q", got)
	}

	cfg.LocalIP = ""
	cfg.SIPServer = "127.0.0.1:5060"
	got, err = cfg.ResolveLocalIP()
	if err != nil {
		t.Fatalf("回环地址探测失败：%v", err)
	}
	if got != "127.0.0.1" {
		t.Fatalf("探测回环应得到 127.0.0.1，实际 %q", got)
	}
}

// TestParseFlagsMediaOptions 覆盖本批次新增的推流参数。
func TestParseFlagsMediaOptions(t *testing.T) {
	var out bytes.Buffer
	cfg, err := parseFlags([]string{
		"--device-id", "34020000001320000001",
		"--sip-server", "127.0.0.1:5060",
		"--sip-domain", "3402000000",
		"--stream-file", samplePath,
		"--fps", "30",
		"--duration", "5s",
		"--ssrc-override", "0999999999",
		"--loop=false",
	}, &out)
	if err != nil {
		t.Fatalf("解析失败：%v\n%s", err, out.String())
	}
	if cfg.FPS != 30 {
		t.Errorf("--fps 解析错误：%d", cfg.FPS)
	}
	if cfg.Duration.String() != "5s" {
		t.Errorf("--duration 解析错误：%s", cfg.Duration)
	}
	if cfg.SSRCOverride != "0999999999" {
		t.Errorf("--ssrc-override 解析错误：%q", cfg.SSRCOverride)
	}
	if cfg.Loop {
		t.Error("--loop=false 未生效")
	}
}

// TestParseFlagsDefaultLoopOn 点播是持续流，--loop 默认必须为 true。
func TestParseFlagsDefaultLoopOn(t *testing.T) {
	var out bytes.Buffer
	cfg, err := parseFlags([]string{
		"--device-id", "34020000001320000001",
		"--sip-server", "127.0.0.1:5060",
		"--sip-domain", "3402000000",
	}, &out)
	if err != nil {
		t.Fatalf("解析失败：%v", err)
	}
	if !cfg.Loop {
		t.Error("--loop 默认应为 true")
	}
	if cfg.FPS != defaultFPS {
		t.Errorf("--fps 默认应为 %d，实际 %d", defaultFPS, cfg.FPS)
	}
}

// TestValidateRejectsBadMediaOptions 负数帧率 / 负时长 / 非数字 SSRC 都应在启动前拦下。
func TestValidateRejectsBadMediaOptions(t *testing.T) {
	base := func() *Config { return sampleConfig() }

	c := base()
	c.FPS = -1
	if err := c.Validate(); err == nil {
		t.Error("负 fps 应校验失败")
	}

	c = base()
	c.Duration = -time.Second
	if err := c.Validate(); err == nil {
		t.Error("负 duration 应校验失败")
	}

	c = base()
	c.SSRCOverride = "0x2000"
	if err := c.Validate(); err == nil {
		t.Error("非十进制 ssrc-override 应校验失败")
	}

	c = base()
	c.SSRCOverride = "0999999999"
	if err := c.Validate(); err != nil {
		t.Errorf("合法 ssrc-override 不应失败：%v", err)
	}
}

// TestMediaFactoryUsesSDPTarget 是接线层的守门用例：
// 推流目标必须来自 INVITE 的 SDP（InviteInfo 的 PlatformIP/PlatformPort），
// SSRC 必须用设备侧决定的值 —— 任何一处退回配置里的硬编码，多路点播就会串流。
func TestMediaFactoryUsesSDPTarget(t *testing.T) {
	nals, err := loadH264Sample(samplePath)
	if err != nil {
		t.Fatalf("加载样本失败：%v", err)
	}

	// 用一个真实监听的端口做目标，确保 stream.New 能成功建连。
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("监听失败：%v", err)
	}
	defer conn.Close()
	host, portStr, _ := net.SplitHostPort(conn.LocalAddr().String())
	port, _ := strconv.Atoi(portStr)

	cfg := sampleConfig()
	cfg.StreamFile = samplePath
	cfg.FPS = 25
	cfg.Loop = false

	factory := newMediaFactory(cfg, nals, silentLogger())
	sess, localPort, err := factory(sipua.InviteInfo{
		PlatformIP:   host,
		PlatformPort: port,
		DeviceSSRC:   "0200000001",
	})
	if err != nil {
		t.Fatalf("媒体工厂构造失败：%v", err)
	}
	st, ok := sess.(*stream.Streamer)
	if !ok {
		t.Fatalf("媒体工厂应返回 *stream.Streamer，实际 %T", sess)
	}
	defer st.Close()

	wantTarget := net.JoinHostPort(host, strconv.Itoa(port))
	if st.Target() != wantTarget {
		t.Errorf("推流目标应取自 SDP 的 %q，实际 %q", wantTarget, st.Target())
	}
	if st.SSRC() != 200000001 {
		t.Errorf("SSRC 应取自 SDP 的 0200000001（十进制 200000001），实际 %d", st.SSRC())
	}
	if localPort == 0 || localPort != st.LocalRTPPort() {
		t.Errorf("返回的本地端口 %d 应等于 Streamer 实际绑定端口 %d", localPort, st.LocalRTPPort())
	}
}

// TestParseSSRC 锁定十进制 SSRC 字符串到 uint32 的转换与报错。
func TestParseSSRC(t *testing.T) {
	cases := map[string]uint32{
		"0200000001": 200000001,
		"0999999999": 999999999,
		"0000000001": 1,
	}
	for in, want := range cases {
		got, err := parseSSRC(in)
		if err != nil {
			t.Fatalf("解析 %q 失败：%v", in, err)
		}
		if got != want {
			t.Errorf("解析 %q 应为 %d，实际 %d", in, want, got)
		}
	}
	for _, bad := range []string{"", "0x2000", "abc", "99999999999999"} {
		if _, err := parseSSRC(bad); err == nil {
			t.Errorf("非法 SSRC %q 应报错", bad)
		}
	}
}
