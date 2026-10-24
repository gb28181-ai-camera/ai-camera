package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置总结构，对应 config/config.yaml 的顶层键。
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	JWT        JWTConfig        `mapstructure:"jwt"`
	Casbin     CasbinConfig     `mapstructure:"casbin"`
	Media      MediaConfig      `mapstructure:"media"`
	ThirdParty ThirdPartyConfig `mapstructure:"thirdparty"`
	Log        LogConfig        `mapstructure:"log"`
	Swagger    SwaggerConfig    `mapstructure:"swagger"`
	GB28181    GB28181Config    `mapstructure:"gb28181"`
}

// GB28181Config GB28181 信令服务（cmd/sipsvc）配置。
//
// 设计依据：docs/敏捷开发/v0.2.0 设备接入与国标视频/00-设计与工作量评估.md §3.3。
// 分为 SIP 信令与媒体通告两段：前者决定本平台在 SIP 域里的身份与监听地址，
// 后者只影响下发给设备的 SDP（ZLM 的收流地址与端口）。
type GB28181Config struct {
	Enabled  bool                 `mapstructure:"enabled"`
	Embedded bool                 `mapstructure:"embedded"` // true 时由 cmd/server 内嵌启动
	Charset  string               `mapstructure:"charset"`  // 设备未配置 charset 时的缺省值
	SIP      GB28181SIPConfig     `mapstructure:"sip"`
	Media    GB28181MediaConfig   `mapstructure:"media"`
	Cascade  GB28181CascadeConfig `mapstructure:"cascade"`
}

// GB28181SIPConfig SIP 信令层配置。
//
// ListenIP 为 0.0.0.0 时监听全部网卡，但出向报文的 Via/Contact 需要一个
// 设备可达的地址：优先用 advertise_ip，未配置则回落到 listen_ip。
// 设备级的多网卡源地址覆盖见 ac_device.local_ip（SDP/源地址选择）。
type GB28181SIPConfig struct {
	ListenIP    string `mapstructure:"listen_ip"`
	Port        int    `mapstructure:"port"`
	Transport   string `mapstructure:"transport"` // UDP / TCP / BOTH
	AdvertiseIP string `mapstructure:"advertise_ip"`
	Domain      string `mapstructure:"domain"`
	Realm       string `mapstructure:"realm"`
	ServerID    string `mapstructure:"server_id"`
	ServerName  string `mapstructure:"server_name"`
	// RegisterExpires 是设备未在 REGISTER 里声明有效期时使用的缺省值（秒）。
	RegisterExpires int `mapstructure:"register_expires"`
	// NonceSecret 用于无状态 Digest nonce 的 HMAC 密钥。为空时退化为固定串，
	// 仅适用于开发环境（重启后旧 nonce 失效是预期行为，设备会自动重注册）。
	NonceSecret string `mapstructure:"nonce_secret"`
}

// GB28181MediaConfig 媒体通告配置（写进 INVITE 的 SDP，交给设备推流）。
type GB28181MediaConfig struct {
	PublicIP     string `mapstructure:"public_ip"`      // SDP c=/o= 通告给设备的 IP
	RTPProxyPort int    `mapstructure:"rtp_proxy_port"` // ZLM 单端口模式的共享收流端口
	// VideoCodecs / AudioCodecs 是 SDP 中通告的编码列表（GB28181-2022 §）。
	// 顺序即 SDP m= 行中的优先级。留空使用内置默认值。
	// 默认视频：PS/90000, H264/90000, H265/90000
	// 默认音频：PCMA/8000, MPEG4-GENERIC/44100, AAC/44100
	VideoCodecs []string `mapstructure:"video_codecs"`
	AudioCodecs []string `mapstructure:"audio_codecs"`
}

// GB28181CascadeConfig 国标级联（作为下级平台向上级注册）的本地信令配置。
//
// 级联使用**独立端口**：5060 已被设备接入占用，且级联的上/下级身份需要
// 独立的 From/Contact 与事务状态，混在同一 listener 上会让排障无从下手。
type GB28181CascadeConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	ListenIP    string `mapstructure:"listen_ip"`
	Port        int    `mapstructure:"port"`
	AdvertiseIP string `mapstructure:"advertise_ip"`
}

// ServerConfig HTTP 服务配置。
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"` // debug / release / test
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig PostgreSQL 连接配置。
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"dbname"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

// DSN 构造 libpq 关键字/值格式的 PostgreSQL 连接串。
//
// ⚠️ 密码为空时必须**整个省略** password 键，不能输出 `password=`。
// pgx 解析关键字/值格式时，空值会让其吞掉其后的键 —— 实测表现：
// 报错里 `database=` 为空、实际连不上目标库，且错误信息指向 database
// 而非 password，极难定位。配置模板里 password 留空是常见情形
// （由环境变量注入），故必须在此处兜住。
func (c *DatabaseConfig) DSN() string {
	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%d user=%s", c.Host, c.Port, c.User)
	if c.Password != "" {
		dsn += " password=" + c.Password
	}
	return fmt.Sprintf("%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		dsn, c.DBName, sslmode)
}

// RedisConfig Redis 连接配置（预留：验证码、令牌黑名单、实时推送）。
type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

// Addr 返回 host:port。
func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// JWTConfig JWT 签发配置。
type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

// CasbinConfig Casbin 配置。
type CasbinConfig struct {
	ModelPath        string        `mapstructure:"model_path"`
	AutoLoadInterval time.Duration `mapstructure:"auto_load_interval"`
}

// MediaConfig 媒体（录像/图片/音频）存储与访问配置。
type MediaConfig struct {
	UploadDir string `mapstructure:"upload_dir"`
	BaseURL   string `mapstructure:"base_url"`
}

// ThirdPartyConfig 第三方平台配置（个推推送、腾讯云短信）。
type ThirdPartyConfig struct {
	Getui GetuiConfig `mapstructure:"getui"`
	SMS   SMSConfig   `mapstructure:"sms"`
}

// GetuiConfig 个推推送配置。
type GetuiConfig struct {
	AppID        string `mapstructure:"app_id"`
	AppKey       string `mapstructure:"app_key"`
	AppSecret    string `mapstructure:"app_secret"`
	MasterSecret string `mapstructure:"master_secret"`
}

// SMSConfig 腾讯云短信配置。
type SMSConfig struct {
	SecretID  string `mapstructure:"secret_id"`
	SecretKey string `mapstructure:"secret_key"`
	SdkAppID  string `mapstructure:"sdk_app_id"`
	SignName  string `mapstructure:"sign_name"`
}

// LogConfig 日志配置。
type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"` // json / console
	Output     string `mapstructure:"output"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// SwaggerConfig 接口文档开关。
type SwaggerConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// envBindings 允许用环境变量覆盖 YAML 中的配置项。
//
// 用途：容器化部署（docker compose / K8s）时，数据库地址与密码随环境变化，
// 不应写进镜像或提交进仓库。YAML 仍是配置的基线，环境变量存在时覆盖之。
//
// 命名规则 `<段>_<键>` 全大写，如 database.password → DATABASE_PASSWORD。
// 显式绑定而非 AutomaticEnv：后者对嵌套键与 Unmarshal 的行为在不同 viper 版本
// 间不一致，显式绑定可预期。
var envBindings = map[string]string{
	"server.port":       "SERVER_PORT",
	"server.mode":       "SERVER_MODE",
	"database.host":     "DATABASE_HOST",
	"database.port":     "DATABASE_PORT",
	"database.user":     "DATABASE_USER",
	"database.password": "DATABASE_PASSWORD",
	"database.dbname":   "DATABASE_DBNAME",
	"redis.host":        "REDIS_HOST",
	"redis.port":        "REDIS_PORT",
	"redis.password":    "REDIS_PASSWORD",
	"jwt.secret":        "JWT_SECRET",
	"media.upload_dir":  "MEDIA_UPLOAD_DIR",
	"media.base_url":    "MEDIA_BASE_URL",
	"log.level":         "LOG_LEVEL",
	"swagger.enabled":   "SWAGGER_ENABLED",

	// GB28181 信令（cmd/sipsvc / 内嵌模式）
	"gb28181.enabled":              "GB28181_ENABLED",
	"gb28181.embedded":             "GB28181_EMBEDDED",
	"gb28181.charset":              "GB28181_CHARSET",
	"gb28181.sip.listen_ip":        "GB28181_SIP_LISTEN_IP",
	"gb28181.sip.port":             "GB28181_SIP_PORT",
	"gb28181.sip.transport":        "GB28181_SIP_TRANSPORT",
	"gb28181.sip.advertise_ip":     "GB28181_SIP_ADVERTISE_IP",
	"gb28181.sip.domain":           "GB28181_SIP_DOMAIN",
	"gb28181.sip.realm":            "GB28181_SIP_REALM",
	"gb28181.sip.server_id":        "GB28181_SIP_SERVER_ID",
	"gb28181.sip.server_name":      "GB28181_SIP_SERVER_NAME",
	"gb28181.sip.register_expires": "GB28181_SIP_REGISTER_EXPIRES",
	"gb28181.sip.nonce_secret":     "GB28181_SIP_NONCE_SECRET",
	"gb28181.media.public_ip":      "GB28181_MEDIA_PUBLIC_IP",
	"gb28181.media.rtp_proxy_port": "GB28181_MEDIA_RTP_PROXY_PORT",
	// 编码列表只走 YAML：viper 的 env 覆盖对 []string 的逗号拆分不可靠，
	// 与其留一个「看起来能配、实际解析成单个元素」的坑，不如明确只支持配置文件。
	// 级联的标量配置走环境变量没有问题。
	"gb28181.cascade.enabled":      "GB28181_CASCADE_ENABLED",
	"gb28181.cascade.listen_ip":    "GB28181_CASCADE_LISTEN_IP",
	"gb28181.cascade.port":         "GB28181_CASCADE_PORT",
	"gb28181.cascade.advertise_ip": "GB28181_CASCADE_ADVERTISE_IP",
}

// LoadConfig 使用 viper 读取 yaml 配置文件并反序列化为 Config。
//
// 读取顺序：YAML 为基线，envBindings 中列出的键可由环境变量覆盖。
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 环境变量覆盖
	for key, env := range envBindings {
		if err := v.BindEnv(key, env); err != nil {
			return nil, fmt.Errorf("绑定环境变量 %s 失败: %w", env, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}
