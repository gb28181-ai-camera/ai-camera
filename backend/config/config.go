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
