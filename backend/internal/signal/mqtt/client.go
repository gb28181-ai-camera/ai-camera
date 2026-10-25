package mqtt

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/pkg/utils"
	paho "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// 默认连接参数。
const (
	// DefaultBrokerURL 与 docker compose 中的服务名一致。
	DefaultBrokerURL = "tcp://mqtt:1883"
	// DefaultClientID 后端信令 worker 的 MQTT 客户端 ID。
	DefaultClientID = "ai-camera-signaling"
	// DefaultQoS 信令使用 QoS1：至少一次。App 侧按 request_id 去重，
	// 后端侧也做了去重，重复投递不会产生第二次协商。
	DefaultQoS byte = 1
)

const (
	connectTimeout    = 5 * time.Second
	publishTimeout    = 5 * time.Second
	subscribeTimeout  = 5 * time.Second
	disconnectQuiesce = 250 // ms
)

var (
	// ErrNotConnected MQTT 连接不可用。
	ErrNotConnected = errors.New("MQTT 未连接")
)

// Config 是信令 worker 的启动配置。
type Config struct {
	Enabled   bool
	BrokerURL string
	// Token 既是 broker 的 MQTT_RPC_TOKEN，也是内部账号 __platform__ 的密码。
	Token    string
	ClientID string
}

// ConfigFromEnv 从环境变量读取配置。
//
// 只在 MQTT_SIGNAL_ENABLED=true 时要求 MQTT_RPC_TOKEN：
// 未启用信令的部署不应该因为缺少 token 而启动失败。
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		BrokerURL: envOr("MQTT_BROKER_URL", DefaultBrokerURL),
		Token:     strings.TrimSpace(os.Getenv("MQTT_RPC_TOKEN")),
		ClientID:  envOr("MQTT_CLIENT_ID", DefaultClientID),
	}

	raw := strings.TrimSpace(os.Getenv("MQTT_SIGNAL_ENABLED"))
	if raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("MQTT_SIGNAL_ENABLED=%q 非法，应为 true/false", raw)
		}
		cfg.Enabled = v
	}

	if !cfg.Enabled {
		return cfg, nil
	}
	if cfg.Token == "" {
		return cfg, errors.New("MQTT_SIGNAL_ENABLED=true 时必须设置 MQTT_RPC_TOKEN（内部账号 __platform__ 的密码）")
	}
	if strings.TrimSpace(cfg.BrokerURL) == "" {
		return cfg, errors.New("MQTT_BROKER_URL 不能为空")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return cfg, errors.New("MQTT_CLIENT_ID 不能为空")
	}
	return cfg, nil
}

// Handler 处理一条订阅到的消息。
type Handler func(topic string, payload []byte)

// Client 封装 paho MQTT 客户端。
//
// 职责仅限于连接、自动重连、在 OnConnect 中重建订阅，以及 publish。
// 业务语义全部在 Bridge 中，便于用假 publisher 做单测。
type Client struct {
	cfg     Config
	handler Handler
	client  paho.Client
}

// NewClient 构造 MQTT 客户端（尚未连接）。
func NewClient(cfg Config, handler Handler) (*Client, error) {
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("MQTT token 为空")
	}
	if strings.TrimSpace(cfg.BrokerURL) == "" {
		return nil, errors.New("MQTT broker 地址为空")
	}
	if handler == nil {
		return nil, errors.New("MQTT 消息处理器为空")
	}

	c := &Client{cfg: cfg, handler: handler}

	opts := paho.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetUsername(PlatformUsername).
		SetPassword(cfg.Token).
		// 信令是瞬时消息，断线后重连不补发历史 offer；App 会重试。
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(10 * time.Second).
		SetOrderMatters(true).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			utils.Warn("MQTT 信令连接断开，将自动重连", zap.Error(err))
		}).
		SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
			utils.Info("MQTT 信令正在重连")
		}).
		SetOnConnectHandler(func(pc paho.Client) {
			utils.Info("MQTT 信令已连接",
				zap.String("broker", cfg.BrokerURL),
				zap.String("client_id", cfg.ClientID))
			// 订阅必须在 OnConnect 里重建：clean session + 自动重连后旧订阅已失效。
			// 两个话题：房间/点播信令 + 设备业务上报。
			for _, filter := range []string{BackendTopicFilter, ReportTopicFilter} {
				tok := pc.Subscribe(filter, DefaultQoS, func(_ paho.Client, m paho.Message) {
					c.handler(m.Topic(), m.Payload())
				})
				if !tok.WaitTimeout(subscribeTimeout) {
					utils.Error("MQTT 信令订阅超时", zap.String("topic", filter))
					return
				}
				if err := tok.Error(); err != nil {
					utils.Error("MQTT 信令订阅失败", zap.String("topic", filter), zap.Error(err))
					return
				}
				utils.Info("MQTT 信令订阅成功", zap.String("topic", filter))
			}
		})

	c.client = paho.NewClient(opts)
	return c, nil
}

// Start 发起连接，并在首次失败后持续重试。
//
// 为什么不能只依赖 paho 的 ConnectRetry：后端与 broker 同时启动时，
// 首次 CONNECT 的鉴权回调会打到尚未开始 listen 的 HTTP 端口，
// 被 fail-closed 拒绝；paho 的 ConnectRetry 只覆盖部分失败路径，
// 实测会停在「未连接」状态。这里用显式循环保证最终连上。
//
// 连接成功后的断线由 paho 的 AutoReconnect 处理；OnConnect 会重建订阅。
func (c *Client) Start(ctx context.Context) error {
	if c == nil || c.client == nil {
		return errors.New("MQTT 客户端未初始化")
	}
	go c.connectLoop(ctx)
	return nil
}

func (c *Client) connectLoop(ctx context.Context) {
	for {
		if c.client.IsConnectionOpen() {
			return
		}
		tok := c.client.Connect()
		if !tok.WaitTimeout(connectTimeout) {
			utils.Warn("MQTT 信令连接超时，5 秒后重试",
				zap.String("broker", c.cfg.BrokerURL))
		} else if err := tok.Error(); err != nil {
			utils.Warn("MQTT 信令连接失败，5 秒后重试",
				zap.String("broker", c.cfg.BrokerURL), zap.Error(err))
		} else if c.client.IsConnectionOpen() {
			// OnConnect 已同步完成订阅。
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// Publish 发布一条消息，实现 Publisher。
func (c *Client) Publish(topic string, qos byte, retained bool, payload []byte) error {
	if c == nil || c.client == nil {
		return ErrNotConnected
	}
	if !c.client.IsConnectionOpen() {
		return ErrNotConnected
	}
	tok := c.client.Publish(topic, qos, retained, payload)
	if !tok.WaitTimeout(publishTimeout) {
		return fmt.Errorf("MQTT 发布超时: topic=%s", topic)
	}
	return tok.Error()
}

// Close 断开连接。会先取消订阅，再等待 250ms 让在途 QoS1 消息完成握手。
func (c *Client) Close() {
	if c == nil || c.client == nil {
		return
	}
	if c.client.IsConnectionOpen() {
		tok := c.client.Unsubscribe(BackendTopicFilter, ReportTopicFilter)
		tok.WaitTimeout(time.Second)
	}
	c.client.Disconnect(disconnectQuiesce)
}

// envOr 读取环境变量，空值时返回默认值。
func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
