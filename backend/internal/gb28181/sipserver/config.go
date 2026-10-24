package sipserver

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/sdp"
)

// Config 是 SIP 信令服务的运行参数。
type Config struct {
	ListenIP        string // 监听地址，0.0.0.0 表示全部网卡
	Port            int
	Transport       string // UDP / TCP / BOTH
	AdvertiseIP     string // 对外通告的本机 IP（Via/Contact/SDP 兜底）
	Domain          string // SIP 域，10 位
	Realm           string // 缺省 Digest 域
	ServerID        string // 平台 20 位国标编码
	ServerName      string // User-Agent
	RegisterExpires int    // 缺省注册有效期（秒）
	NonceSecret     string // 无状态 nonce 的 HMAC 密钥
	DefaultCharset  string // 设备未配置 charset 时的缺省值
	MediaPublicIP   string // SDP 通告的收流 IP
	RTPProxyPort    int    // ZLM 单端口收流端口
	// VideoCodecs / AudioCodecs 是 SDP 通告的编码列表，顺序即优先级。
	// 来自 gb28181.media.video_codecs / audio_codecs；留空时在 applyDefaults 中
	// 填入 GB28181-2022 的默认列表。
	VideoCodecs []string
	AudioCodecs []string
	Logger      *slog.Logger
}

// TransportUDP/TCP/Both 是 transport 配置的三个合法取值。
const (
	TransportUDP  = "UDP"
	TransportTCP  = "TCP"
	TransportBoth = "BOTH"
)

// FromAppConfig 从全局配置推导 sipserver.Config，并补齐缺省值。
func FromAppConfig(c *config.Config, logger *slog.Logger) (Config, error) {
	if c == nil {
		return Config{}, fmt.Errorf("配置为 nil")
	}
	g := c.GB28181
	cfg := Config{
		ListenIP:        strings.TrimSpace(g.SIP.ListenIP),
		Port:            g.SIP.Port,
		Transport:       strings.ToUpper(strings.TrimSpace(g.SIP.Transport)),
		AdvertiseIP:     strings.TrimSpace(g.SIP.AdvertiseIP),
		Domain:          strings.TrimSpace(g.SIP.Domain),
		Realm:           strings.TrimSpace(g.SIP.Realm),
		ServerID:        strings.TrimSpace(g.SIP.ServerID),
		ServerName:      strings.TrimSpace(g.SIP.ServerName),
		RegisterExpires: g.SIP.RegisterExpires,
		NonceSecret:     g.SIP.NonceSecret,
		DefaultCharset:  charset.Normalize(g.Charset),
		MediaPublicIP:   strings.TrimSpace(g.Media.PublicIP),
		RTPProxyPort:    g.Media.RTPProxyPort,
		VideoCodecs:     trimAll(g.Media.VideoCodecs),
		AudioCodecs:     trimAll(g.Media.AudioCodecs),
		Logger:          logger,
	}
	if err := cfg.applyDefaults(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyDefaults 补齐缺省值并校验必要项。
func (c *Config) applyDefaults() error {
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	if c.Port == 0 {
		c.Port = 5060
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("gb28181.sip.port 非法：%d", c.Port)
	}
	switch c.Transport {
	case "", TransportUDP:
		c.Transport = TransportUDP
	case TransportTCP, TransportBoth:
	default:
		return fmt.Errorf("gb28181.sip.transport 仅支持 UDP / TCP / BOTH，当前 %q", c.Transport)
	}
	if c.Domain == "" {
		c.Domain = "3402000000"
	}
	if len(c.Domain) != 10 {
		return fmt.Errorf("gb28181.sip.domain 应为 10 位，当前 %q", c.Domain)
	}
	if c.Realm == "" {
		c.Realm = c.Domain
	}
	if c.ServerID == "" {
		// 20 位平台编码：10 位域 + 类型(20) + 序号，与 WVP 的生成规则一致。
		c.ServerID = c.Domain + "2000000001"
	}
	if c.ServerName == "" {
		c.ServerName = "AI Camera"
	}
	if c.RegisterExpires <= 0 {
		c.RegisterExpires = 3600
	}
	if c.DefaultCharset == "" || !charset.IsGB(c.DefaultCharset) {
		if strings.EqualFold(c.DefaultCharset, charset.UTF8) {
			c.DefaultCharset = charset.UTF8
		} else {
			c.DefaultCharset = charset.GB2312
		}
	}
	if c.AdvertiseIP == "" {
		c.AdvertiseIP = resolveAdvertiseIP(c.ListenIP)
	}
	if c.MediaPublicIP == "" {
		// SDP 里的地址必须是设备可达的；没配就用本机通告地址。
		c.MediaPublicIP = c.AdvertiseIP
	}
	if c.RTPProxyPort == 0 {
		c.RTPProxyPort = 10000
	}
	// 编码协商：配置留空时用 GB28181-2022 默认列表（PS/H264/H265 + PCMA/AAC…）。
	// 放在这里而不是 BuildPlaySDP，是为了让启动日志与「实际下发」永远一致。
	if len(c.VideoCodecs) == 0 {
		c.VideoCodecs = sdp.DefaultVideoCodecs()
	}
	if len(c.AudioCodecs) == 0 {
		c.AudioCodecs = sdp.DefaultAudioCodecs()
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

// trimAll 去掉配置项的空白并丢弃空串，避免生成非法的 rtpmap 行。
func trimAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if v := strings.TrimSpace(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// resolveAdvertiseIP 选出设备可达的本机地址。
//
// listen_ip 是具体地址时直接用它；0.0.0.0 时通过「拨号探测」让内核选默认网卡，
// 这是 WVP `SipLayer.getLocalIp` 的等价做法。探测失败回落到 127.0.0.1，
// 至少保证本地联调可用。
func resolveAdvertiseIP(listenIP string) string {
	if listenIP != "" && listenIP != "0.0.0.0" && listenIP != "::" {
		return listenIP
	}
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 500*time.Millisecond)
	if err == nil {
		defer conn.Close()
		if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok && addr.IP != nil && !addr.IP.IsUnspecified() {
			return addr.IP.String()
		}
	}
	return "127.0.0.1"
}
