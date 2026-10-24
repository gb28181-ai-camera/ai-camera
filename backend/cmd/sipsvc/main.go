// Command sipsvc 是 GB28181 SIP 信令服务（设计 §14.4 模块 C）。
//
// 它是与 cmd/server 同属一个 Go module 的第二个入口，共用 internal/model、
// repository 与 bootstrap，但**副本固定为 1**：SIP 事务、TCP 连接与 INVITE 会话
// 状态都是节点本地的（设计 §2.4），横向扩容需要 Redis RPC，本期不做。
//
// 与 HTTP 服务的关系：HTTP 进程通过 internal/gb28181/control 的接口调用本服务，
// 本服务启动时注入实现。开发期也可用 gb28181.embedded=true 让 cmd/server 内嵌启动。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	// 空导入：订阅自动续订的实现注册在 service/gb28181 的 init() 里。
	// 内嵌模式（cmd/server）会经 handler 链接该包；独立信令进程必须显式引入，
	// 否则 sipserver 里的续订定时器取不到回调，订阅到期后不会续订。
	_ "github.com/boticz/ai-camera-backend/internal/service/gb28181"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sipsvc 启动失败："+err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config/config.yaml"
	}

	// bootstrap 的 init() 会在包导入时按 CONFIG_PATH 自动加载；
	// 配置缺失时它静默跳过（保证 go test 不受影响），这里再显式初始化以给出明确错误。
	if bootstrap.Config() == nil {
		bootstrap.Init(cfgPath)
	}
	cfg := bootstrap.Config()
	if cfg == nil {
		return fmt.Errorf("无法加载配置 %s", cfgPath)
	}
	if bootstrap.DB() == nil {
		return errors.New("数据库未初始化：SIP 白名单与注册状态都依赖 ac_device，不能空跑")
	}

	logger := newLogger(cfg.Log.Level)
	if !cfg.GB28181.Enabled {
		logger.Warn("配置中 gb28181.enabled=false，但命令行已显式启动 sipsvc —— 继续运行")
	}

	svcCfg, err := sipserver.FromAppConfig(cfg, logger)
	if err != nil {
		return err
	}
	svc, err := sipserver.New(svcCfg)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := svc.Start(ctx); err != nil {
		return err
	}

	logger.Info("sipsvc 就绪",
		"config", cfgPath,
		"sip", fmt.Sprintf("%s:%d/%s", svcCfg.ListenIP, svcCfg.Port, svcCfg.Transport),
		"advertise_ip", svcCfg.AdvertiseIP,
		"domain", svcCfg.Domain,
		"server_id", svcCfg.ServerID,
		"sdp_ip", svcCfg.MediaPublicIP,
		"rtp_proxy_port", svcCfg.RTPProxyPort,
		"charset", svcCfg.DefaultCharset)
	logger.Info("等待设备注册；Ctrl+C 退出（退出时会把在线会话全部 BYE）")

	<-ctx.Done()
	logger.Info("收到退出信号，正在停止信令服务")

	// 给 BYE 与日志落库留出时间；Stop 内部还有 3 秒的会话超时。
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		logger.Warn("信令服务停止超时，强制退出")
	}
	logger.Info("sipsvc 已退出")
	return nil
}

// newLogger 构造项目风格的日志器。
func newLogger(level string) *slog.Logger {
	lv := slog.LevelInfo
	if strings.EqualFold(strings.TrimSpace(level), "debug") {
		lv = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lv}))
}
