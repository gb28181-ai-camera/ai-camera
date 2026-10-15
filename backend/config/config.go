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

// DSN 构造 PostgreSQL 连接串。
func (c *DatabaseConfig) DSN() string {
	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Shanghai",
		c.Host, c.Port, c.User, c.Password, c.DBName, sslmode,
	)
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

// LoadConfig 使用 viper 读取 yaml 配置文件并反序列化为 Config。
func LoadConfig(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}
