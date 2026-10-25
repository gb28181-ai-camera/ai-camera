package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/tools/gbsim/manscdp"
)

// 默认值。这些取值对齐 WVP-Pro / GB28181 常见实现，改动前请先确认平台侧配置。
const (
	defaultLocalPort    = 5062
	defaultExpires      = 3600
	defaultKeepalive    = 60 * time.Second
	defaultCharset      = "GB2312"
	defaultTransport    = "udp"
	defaultMode         = "udp"
	defaultDeviceModel  = "AICam-SIM"
	defaultManufacturer = "Boticz-Simulator"
	defaultFirmware     = "v0.2.0-sim"
	// defaultFPS 与内置样本码流的帧率一致（PTS 步进 = 90000/25 = 3600）。
	defaultFPS = 25
)

// 支持的媒体传输模式（§7.2）。
const (
	ModeUDP        = "udp"
	ModeTCPPassive = "tcp-passive"
	ModeTCPActive  = "tcp-active"
)

// Config 汇总 gbsim 的全部运行参数，由命令行 flag 填充。
type Config struct {
	// —— 设备身份 ——
	DeviceID   string // 20 位国标设备编码，SIP From 头与目录 DeviceID 都用它
	DeviceName string // 设备名称（中文名受 Charset 影响）
	// DeviceCode / ProductCode 是 IOT 的业务设备编码与产品编码。
	//
	// 非空时随 REGISTER 发送 X-Device-Code / X-Product-Code 头，
	// 用于「国标编码尚未建档但设备档案已存在」时的首次绑定。
	DeviceCode  string
	ProductCode string
	Channels    int    // 模拟的通道数量，≥1
	Charset     string // 通道名/设备名的 XML 编码：GB2312 或 UTF-8（§2.6 结论六）

	// —— SIP 接入 ——
	SIPServer string // 平台 SIP 地址，host:port
	SIPDomain string // SIP 域，同时作为 Digest realm
	Password  string // SIP 注册密码
	LocalIP   string // 本地信令 IP，留空则按到 SIP 服务器的路由自动探测
	LocalPort int    // 本地信令端口
	Transport string // 信令传输：udp / tcp
	Expires   int    // 注册有效期（秒），决定 re-REGISTER 周期

	// —— 媒体 ——
	Mode         string        // 媒体传输模式：udp / tcp-passive / tcp-active
	StreamFile   string        // H.264 Annex-B 样本文件路径，空则不加载（无样本时 INVITE 会被回 488）
	Loop         bool          // 样本放完后是否从头再推
	FPS          int           // 推流帧率；≤0 表示不限速（仅压测用）
	Duration     time.Duration // >0 时每路推流推满该时长自动停止
	SSRCOverride string        // 非空时设备在 200 OK 里回一个与平台不同的 y=（验证平台校正路径）

	// —— 行为开关 ——
	KeepaliveInterval time.Duration // Keepalive 心跳周期
	RiskInterval      time.Duration // 预留：AI 风险上报周期（本批次未实现）
	// AlarmInterval 国标报警上报周期（Notify/Alarm），0 表示不报。
	AlarmInterval time.Duration
	// PositionInterval 国标移动位置上报告周期（Notify/MobilePosition），0 表示不报。
	PositionInterval time.Duration
	// AlarmType 报警类型码：1 视频报警 / 2 设备报警 / 5 移动侦测等（GB/T 28181 附录）。
	AlarmType string
	Verbose   bool // 打印 SIP / MANSCDP 报文级日志

	// Enable2022 是否应答 GB/T 28181-2022 查询。
	// 默认 true（--2022=true）；设为 false 可模拟一台 2016 设备 ——
	// 平台对 2016 设备的 2022 查询应在发报文前就被拦截，这个开关用来
	// 验证「平台不会误发、误等」。模拟器自身不区分协议版本，只决定是否应答。
	Enable2022 bool
}

// Validate 校验配置的基本合法性。它只做「本地即可判定」的检查，
// 不尝试连接平台 —— 连接失败应在 Register 阶段暴露。
func (c *Config) Validate() error {
	if len(c.DeviceID) != 20 {
		return fmt.Errorf("--device-id 必须是 20 位国标编码，当前 %d 位：%q", len(c.DeviceID), c.DeviceID)
	}
	for i := 0; i < len(c.DeviceID); i++ {
		if c.DeviceID[i] < '0' || c.DeviceID[i] > '9' {
			return fmt.Errorf("--device-id 必须是纯数字：%q", c.DeviceID)
		}
	}
	if c.SIPServer == "" {
		return fmt.Errorf("--sip-server 不能为空")
	}
	if _, _, err := net.SplitHostPort(c.SIPServer); err != nil {
		return fmt.Errorf("--sip-server 必须是 host:port 形式：%w", err)
	}
	if c.SIPDomain == "" {
		return fmt.Errorf("--sip-domain 不能为空")
	}
	if c.Channels < 1 {
		return fmt.Errorf("--channels 必须 ≥ 1，当前 %d", c.Channels)
	}
	if c.LocalPort <= 0 || c.LocalPort > 65535 {
		return fmt.Errorf("--local-port 越界：%d", c.LocalPort)
	}
	if c.Expires <= 0 {
		return fmt.Errorf("--expires 必须为正数秒：%d", c.Expires)
	}
	if c.AlarmInterval < 0 || c.PositionInterval < 0 {
		return fmt.Errorf("--alarm-interval / --position-interval 不能为负")
	}
	switch strings.ToLower(strings.TrimSpace(c.Transport)) {
	case "udp":
	case "tcp":
		// TCP 信令采用「只由 client 长连接 dial 出去、不另开监听」的模型（见 sipua.startTCP）。
		// 早期照搬 UDP 的 ListenAndServe 会失败：UDP 的监听 socket 会进 sipgo 的连接池，
		// 出向请求按 laddr 命中它；TCP 的监听 socket 不在同一个池里，client 再 bind
		// 同一端口就会 `bind: Only one usage of each socket address`。
	case "":
		c.Transport = defaultTransport
	default:
		return fmt.Errorf("--transport 只支持 udp / tcp，当前 %q", c.Transport)
	}
	// tcp-active 保留可解析：它对应「平台主动连设备」，与平台单端口收流互斥。
	// 运行期会在 INVITE 阶段明确回 488（见 sipua.handleInvite），而不是在参数层拒绝 ——
	// 这样测试仍能启动模拟器、复现「平台拒绝 TCP-ACTIVE」的完整链路。
	switch strings.ToLower(c.Mode) {
	case ModeUDP, ModeTCPPassive, ModeTCPActive:
	default:
		return fmt.Errorf("--mode 只支持 %s / %s / %s，当前 %q",
			ModeUDP, ModeTCPPassive, ModeTCPActive, c.Mode)
	}
	if !isGBCharset(c.Charset) && !strings.EqualFold(c.Charset, "UTF-8") {
		return fmt.Errorf("--charset 只支持 GB2312 / UTF-8，当前 %q", c.Charset)
	}
	if c.FPS < 0 {
		return fmt.Errorf("--fps 不能为负：%d", c.FPS)
	}
	if c.Duration < 0 {
		return fmt.Errorf("--duration 不能为负：%s", c.Duration)
	}
	if c.SSRCOverride != "" && !isDigits(c.SSRCOverride) {
		return fmt.Errorf("--ssrc-override 必须是十进制数字串（如 0200000001）：%q", c.SSRCOverride)
	}
	return nil
}

// isDigits 判断字符串是否全为 ASCII 数字。
func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// ResolveLocalIP 返回本地信令 IP。
// 显式配置优先；否则对 SIP 服务器做一次 UDP dial，
// 由内核选路反推本机出口 IP —— 避免在多网卡机器上挑错网卡（§6.2 #1）。
func (c *Config) ResolveLocalIP() (string, error) {
	if c.LocalIP != "" {
		return c.LocalIP, nil
	}
	conn, err := net.Dial("udp", c.SIPServer)
	if err != nil {
		return "", fmt.Errorf("探测本地 IP 失败（无法到 %s 建连）：%w", c.SIPServer, err)
	}
	defer conn.Close()
	host, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return "", fmt.Errorf("解析本地地址失败：%w", err)
	}
	return host, nil
}

// Channel 描述一个模拟的 GB28181 通道。
type Channel struct {
	DeviceID string // 20 位通道编码
	Name     string // 通道名称，中文名会按 Charset 编码
	PTZType  int    // 摄像机结构类型：1 球机 / 2 半球 / 3 固定枪机
}

// BuildChannels 由设备编码推导通道列表。
//
// 编码规则（GB/T 28181 附录 D / GA/T 380）：20 位码 = 8 位行政区划 + 2 位行业 + 3 位类型 + 7 位序号。
// 设备码的类型位为 132（网络摄像机），通道码的类型位为 131（摄像机通道），
// 因此通道码 = 设备码前 10 位 + "131" + 7 位序号。
func (c *Config) BuildChannels() []Channel {
	prefix := c.DeviceID[:10]
	out := make([]Channel, 0, c.Channels)
	for i := 0; i < c.Channels; i++ {
		out = append(out, Channel{
			DeviceID: fmt.Sprintf("%s131%07d", prefix, i+1),
			Name:     fmt.Sprintf("%s通道%d", c.DeviceName, i+1),
			PTZType:  1, // 默认球机：可响应云台，便于后续批次验证 PTZ 应答
		})
	}
	return out
}

// isGBCharset 判断 charset 是否走 GB 系。
// 直接复用 manscdp 的判定，避免「编码族」在两处各写一份而慢慢跑偏。
func isGBCharset(charset string) bool {
	return manscdp.IsGBCharset(charset)
}
