package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/boticz/ai-camera-backend/internal/api/router"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/gb28181/play"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	whep "github.com/boticz/ai-camera-backend/internal/gb28181/webrtc"
	"github.com/boticz/ai-camera-backend/internal/mqttrpc"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/internal/service/iot"
	mqttsignal "github.com/boticz/ai-camera-backend/internal/signal/mqtt"
	signalroom "github.com/boticz/ai-camera-backend/internal/signal/room"
)

// @title           AI Camera 智能监控平台 API
// @version         1.0.0
// @description     AI Camera 后端系统接口文档
// @termsOfService  http://swagger.io/terms/

// @contact.name   技术支持
// @contact.email  support@boticz.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
func main() {
	// 配置、数据库、JWT、Casbin 均在 bootstrap 包 init() 时完成装配
	r := router.SetupRouter()

	// 进程级上下文：用于停止后台 janitor。与 HTTP 请求上下文无关，
	// 收到 SIGTERM 时统一 cancel。
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	cfg := bootstrap.Config()
	addr := fmt.Sprintf(":%d", cfg.Server.Port)

	// 信令日志 janitor：默认保留 30 天，每天执行一次。
	// 放在 HTTP 服务启动前注册，保证即使 HTTP 端口被占用也能在日志里看到清理任务。
	if bootstrap.DB() != nil {
		go startSipLogJanitor(appCtx, gbsvc.NewGbSipLogService(), sipLogRetentionDays())
	} else {
		log.Println("数据库未初始化，跳过信令日志清理任务")
	}

	// 开发期便利（设计 §3.2）：gb28181.embedded=true 时把 SIP 信令内嵌进本进程，
	// 一条命令即可与 tools/gbsim 端到端联调。生产环境应改用独立的 cmd/sipsvc
	// （副本固定 1，与可水平扩展的 API 进程解耦）。
	var sipSvc *sipserver.Service
	if cfg.GB28181.Enabled && cfg.GB28181.Embedded {
		sipCfg, err := sipserver.FromAppConfig(cfg, nil)
		if err != nil {
			log.Fatalf("GB28181 配置非法: %v", err)
		}
		sipSvc, err = sipserver.New(sipCfg)
		if err != nil {
			log.Fatalf("GB28181 信令服务初始化失败: %v", err)
		}
		// 首次绑定：REGISTER 带 X-Device-Code/X-Product-Code 且国标编码未建档时，
		// 由 IOT 设备档案完成绑定。注入接口而不是让 sipserver 直接依赖 IOT 包。
		sipSvc.SetDeviceBinder(iot.NewDeviceService())
		if err := sipSvc.Start(context.Background()); err != nil {
			log.Fatalf("GB28181 信令服务启动失败: %v", err)
		}
		log.Printf("GB28181 信令已内嵌启动: %s:%d/%s (domain=%s, server=%s)",
			sipCfg.ListenIP, sipCfg.Port, sipCfg.Transport, sipCfg.Domain, sipCfg.ServerID)
	}

	// MQTT 信令 → ZLM WHEP 桥（默认关闭）。
	//
	// 只在 MQTT_SIGNAL_ENABLED=true 时启动：信令 worker 会以内部账号
	// __platform__ 常连 broker，不需要信令的部署不应无端多一条长连接。
	// 初次连接失败不阻塞 HTTP 服务，paho 会在后台自动重连。
	var (
		signalClient *mqttsignal.Client
		signalBridge *mqttsignal.Bridge
		signalHub    *signalroom.Hub
	)
	if sigCfg, err := mqttsignal.ConfigFromEnv(); err != nil {
		log.Fatalf("MQTT 信令配置非法: %v", err)
	} else if sigCfg.Enabled {
		signalClient, err = mqttsignal.NewClient(sigCfg, func(topic string, payload []byte) {
			// 同一个 broker 话题承载两条链路：房间信令优先，其余交给 WHEP 桥。
			// 消息在 Start 之后才可能到达；判空防止初始化顺序导致 nil panic。
			if signalHub != nil && signalHub.HandleDevice(topic, payload) {
				return
			}
			if signalBridge != nil {
				signalBridge.Handle(topic, payload)
			}
		})
		if err != nil {
			log.Fatalf("MQTT 信令客户端初始化失败: %v", err)
		}
		signalBridge = mqttsignal.NewBridge(signalClient, whep.NewDefaultClient(), play.DefaultRegistry())
		signalHub = signalroom.NewHub(signalClient)
		signalroom.SetDefaultHub(signalHub)
		if err := signalClient.Start(appCtx); err != nil {
			log.Printf("MQTT 信令启动异常（HTTP 服务继续运行）: %v", err)
		}
		log.Printf("MQTT 信令 worker 已启用: broker=%s client_id=%s", sigCfg.BrokerURL, sigCfg.ClientID)
	}

	// MQTT gRPC 鉴权服务：供内部 fork 的 mochi-mqtt 调用。
	//
	// 与上面的 HTTP 信令 worker 是两件独立的事：
	//   - gRPC：broker → 后端，做 CONNECT 鉴权；
	//   - 信令 worker：后端 → broker，做 WebRTC SDP 中转。
	// 两者可以单独启用。
	var mqttGRPC *mqttrpc.Server
	if os.Getenv("MQTT_GRPC_ENABLED") == "true" {
		grpcListen := os.Getenv("MQTT_GRPC_LISTEN")
		if grpcListen == "" {
			grpcListen = ":50051"
		}
		mqttGRPC = mqttrpc.New(nil)
		if err := mqttGRPC.Start(grpcListen); err != nil {
			log.Fatalf("MQTT gRPC 鉴权服务启动失败: %v", err)
		}
	}

	srv := &http.Server{
		Handler:      r,
		Addr:         addr,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		log.Printf("AI Camera 服务启动于 %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 优雅退出：等待信号后给在途请求 10 秒完成时间
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务...")

	// 先停后台任务，再停 HTTP：避免 janitor 在连接池关闭后还在执行清理。
	appCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 信令先停：Bridge 拒绝新 offer 并等待在途协商，Client 仍保持连接，
	// 保证最后几条 answer 能发出去；之后再断开 broker 连接。
	if signalBridge != nil {
		signalBridge.Close(ctx)
	}
	if signalClient != nil {
		signalClient.Close()
	}
	if mqttGRPC != nil {
		mqttGRPC.Stop()
	}
	if sipSvc != nil {
		sipSvc.Stop()
	}
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务关闭异常: %v", err)
	}
	log.Println("服务已退出")
}

// sipLogRetentionDays 读取 SIP_LOG_RETENTION_DAYS，默认 30 天。
//
// 为什么不放进 config 结构：这是一个纯运维参数，且只被 janitor 使用；
// 为一个整数扩展 Config + 模板 + 文档的收益很低。非法值必须显式回退并打印，
// 不能静默使用 0（0 会被 Cleanup 当成默认值，看似正常，实则掩盖了配置错误）。
func sipLogRetentionDays() int {
	raw := strings.TrimSpace(os.Getenv("SIP_LOG_RETENTION_DAYS"))
	if raw == "" {
		return gbsvc.DefaultSipLogRetentionDays
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		log.Printf("SIP_LOG_RETENTION_DAYS=%q 非法，回退到默认 %d 天", raw, gbsvc.DefaultSipLogRetentionDays)
		return gbsvc.DefaultSipLogRetentionDays
	}
	return n
}

// startSipLogJanitor 启动信令日志清理循环。
//
// 首次延迟 10 分钟：进程刚启动时正在做 AutoMigrate、Casbin 装载与设备重注册，
// 此时再压一个大表 DELETE 会放大启动抖动。10 分钟后系统已稳定，再开始清理。
// 之后按 24 小时周期执行；单次清理有 30 分钟超时，避免慢查询把 janitor 永久卡住。
func startSipLogJanitor(ctx context.Context, svc *gbsvc.GbSipLogService, retentionDays int) {
	const (
		interval   = 24 * time.Hour
		firstDelay = 10 * time.Minute
		runTimeout = 30 * time.Minute
	)

	log.Printf("信令日志清理任务已启动：保留 %d 天，周期 %s，首次延迟 %s",
		retentionDays, interval, firstDelay)

	first := time.NewTimer(firstDelay)
	defer first.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	run := func() {
		runCtx, cancel := context.WithTimeout(ctx, runTimeout)
		defer cancel()
		start := time.Now()
		n, err := svc.Cleanup(runCtx, retentionDays)
		if err != nil {
			log.Printf("信令日志清理未完成（保留 %d 天，已删除 %d 行）: %v", retentionDays, n, err)
			return
		}
		log.Printf("信令日志清理完成：保留 %d 天，删除 %d 行，耗时 %s",
			retentionDays, n, time.Since(start).Round(time.Millisecond))
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("信令日志清理任务已停止")
			return
		case <-first.C:
			run()
		case <-ticker.C:
			run()
		}
	}
}
