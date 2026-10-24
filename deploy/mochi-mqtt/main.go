// mochi-mqtt broker：鉴权与 ACL 全部委托给 AI Camera 后端。
//
// 为什么单独一个可执行文件而不是把 broker 嵌进后端：
//   - MQTT 的连接数、会话状态与 HTTP API 的扩缩容诉求不同；
//   - broker 重启不应该带走 API；
//   - 鉴权通过 RPC 回后端，broker 保持无状态，可水平扩容。
//
// 启动配置全部走环境变量，便于 compose / K8s 注入：
//
//	MQTT_LISTEN        监听地址，默认 :1883
//	MQTT_WS_LISTEN     WebSocket 监听地址，默认 :8083，留空关闭
//	BACKEND_RPC_URL    后端基址，默认 http://backend:8080
//	MQTT_RPC_TOKEN     与后端 MQTT_RPC_TOKEN 一致；为空时**拒绝启动**
//	MQTT_DEBUG         设为 1 打开 mochi 的调试日志
package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/boticz/ai-camera-mochi-mqtt/hook"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/listeners"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level(),
	}))
	slog.SetDefault(logger)

	token := strings.TrimSpace(os.Getenv("MQTT_RPC_TOKEN"))
	if token == "" {
		// fail-fast：没有 token 时后端会拒绝所有 RPC，broker 起来也连不上任何客户端，
		// 与其让现场排查「为什么永远是 401」，不如直接拒绝启动并打清楚原因。
		logger.Error("MQTT_RPC_TOKEN 未配置，拒绝启动",
			"hint", "该值必须与后端环境变量 MQTT_RPC_TOKEN 完全一致")
		os.Exit(1)
	}

	backend := envOr("BACKEND_RPC_URL", "http://backend:8080")
	listen := envOr("MQTT_LISTEN", ":1883")
	wsListen := envOr("MQTT_WS_LISTEN", ":8083")

	server := mqtt.New(&mqtt.Options{InlineClient: false})

	// 先挂鉴权 hook：必须在任何 listener 开始接收连接之前生效，
	// 否则并发启动时可能有连接绕过 RPC 鉴权。
	if err := server.AddHook(hook.New(hook.Config{
		BackendURL: backend,
		Token:      token,
		Logger:     logger,
	}, nil), nil); err != nil {
		logger.Error("挂载 RPC 鉴权 hook 失败", "error", err)
		os.Exit(1)
	}

	if err := server.AddListener(listeners.NewTCP(listeners.Config{
		ID:      "tcp",
		Address: listen,
	})); err != nil {
		logger.Error("启动 TCP listener 失败", "addr", listen, "error", err)
		os.Exit(1)
	}

	if wsListen != "" {
		if err := server.AddListener(listeners.NewWebsocket(listeners.Config{
			ID:      "ws",
			Address: wsListen,
		})); err != nil {
			logger.Error("启动 WebSocket listener 失败", "addr", wsListen, "error", err)
			os.Exit(1)
		}
	}

	logger.Info("mochi-mqtt 启动", "tcp", listen, "ws", wsListen, "backend", backend)
	if err := server.Serve(); err != nil {
		logger.Error("mochi-mqtt 运行失败", "error", err)
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func level() slog.Level {
	if strings.TrimSpace(os.Getenv("MQTT_DEBUG")) == "1" {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}
