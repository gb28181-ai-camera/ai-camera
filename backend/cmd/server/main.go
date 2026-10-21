package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/boticz/ai-camera-backend/internal/api/router"
	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
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

	cfg := bootstrap.Config()
	addr := fmt.Sprintf(":%d", cfg.Server.Port)

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
		if err := sipSvc.Start(context.Background()); err != nil {
			log.Fatalf("GB28181 信令服务启动失败: %v", err)
		}
		log.Printf("GB28181 信令已内嵌启动: %s:%d/%s (domain=%s, server=%s)",
			sipCfg.ListenIP, sipCfg.Port, sipCfg.Transport, sipCfg.Domain, sipCfg.ServerID)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if sipSvc != nil {
		sipSvc.Stop()
	}
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务关闭异常: %v", err)
	}
	log.Println("服务已退出")
}
