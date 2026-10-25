package gb28181

// 平台运行信息：版本 / 构建 / 运行时长 / SIP / 默认媒体节点 / 依赖状态。
//
// 该接口替代 WVP 的「系统信息」页面。所有探测都带超时，任何一项失败
// 只影响对应字段，不让整个接口失败 —— 运维页在系统半故障时最有价值。

import (
	"context"
	"net"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"gorm.io/gorm"
)

// PlatformVersion 平台版本号。
//
// 与 CHANGE_LOG 保持一致；构建信息里的 module version 在本地构建时
// 通常是 "(devel)"，因此以常量为主、构建信息为辅。
const PlatformVersion = "v0.5.0"

// platformStartedAt 进程启动时间（包初始化即记录）。
var platformStartedAt = time.Now()

// GbPlatformInfoService 平台信息服务。
type GbPlatformInfoService struct {
	mediaRepo *repository.GbMediaServerRepo
}

// NewGbPlatformInfoService 构造平台信息服务。
func NewGbPlatformInfoService() *GbPlatformInfoService {
	return &GbPlatformInfoService{mediaRepo: repository.NewGbMediaServerRepo()}
}

// Info 汇总平台运行信息。
func (s *GbPlatformInfoService) Info(ctx context.Context) *response.GbPlatformInfoResp {
	now := time.Now()
	hostname, _ := os.Hostname()

	resp := &response.GbPlatformInfoResp{
		Version:       PlatformVersion,
		GoVersion:     runtime.Version(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Hostname:      hostname,
		StartedDT:     platformStartedAt,
		UptimeSeconds: int64(now.Sub(platformStartedAt).Seconds()),
		SIP:           s.sipStatus(),
		Database:      s.databaseStatus(ctx),
		Redis:         s.redisStatus(ctx),
	}
	resp.BuildVersion, resp.BuildTime = buildInfo()

	if ms, err := s.mediaRepo.GetDefault(ctx); err == nil {
		resp.DefaultMediaServer = toDefaultMediaServerSummary(ms)
	} else if !isNotFound(err) {
		resp.DefaultMediaServer = nil
	}

	return resp
}

// sipStatus 读取 SIP 配置与运行时是否已注入控制器。
func (s *GbPlatformInfoService) sipStatus() response.GbSipStatus {
	st := response.GbSipStatus{}
	cfg := bootstrap.Config()
	if cfg != nil {
		g := cfg.GB28181
		st.Enabled = g.Enabled
		st.Embedded = g.Embedded
		st.Domain = g.SIP.Domain
		st.Transport = firstNonEmpty(g.SIP.Transport, model.GbSipTransportUDP)
		st.ListenAddr = net.JoinHostPort(firstNonEmpty(g.SIP.ListenIP, "0.0.0.0"), strconv.Itoa(firstInt(g.SIP.Port, 5060)))
		advertiseIP := firstNonEmpty(g.SIP.AdvertiseIP, g.SIP.ListenIP, "127.0.0.1")
		st.AdvertiseAddr = net.JoinHostPort(advertiseIP, strconv.Itoa(firstInt(g.SIP.Port, 5060)))
	}
	// control.Default() 非 nil 才说明当前进程真的能发 SIP 报文。
	st.Running = control.Default() != nil
	return st
}

// databaseStatus 探测数据库连通性。
func (s *GbPlatformInfoService) databaseStatus(ctx context.Context) response.GbDependencyStatus {
	st := response.GbDependencyStatus{Configured: true}
	db := bootstrap.DB()
	if db == nil {
		st.Configured = false
		st.Message = "数据库未初始化"
		return st
	}
	start := time.Now()
	sqlDB, err := db.DB()
	if err != nil {
		st.Message = "获取数据库连接失败: " + err.Error()
		return st
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err = sqlDB.PingContext(pingCtx)
	st.LatencyMS = round2(float64(time.Since(start).Microseconds()) / 1000)
	if err != nil {
		st.Message = "数据库不可达: " + err.Error()
		return st
	}
	st.Online = true
	st.Message = "连接正常"
	return st
}

// redisStatus 探测 Redis 连通性。
//
// 本期工程没有引入 Redis 客户端（验证码/令牌黑名单未落地），因此只做
// TCP 可达性探测：报告「配置了但不可达」比谎报在线更有运维价值。
func (s *GbPlatformInfoService) redisStatus(ctx context.Context) response.GbDependencyStatus {
	st := response.GbDependencyStatus{}
	cfg := bootstrap.Config()
	if cfg == nil || strings.TrimSpace(cfg.Redis.Host) == "" {
		st.Message = "未配置 Redis"
		return st
	}
	st.Configured = true
	st.Address = net.JoinHostPort(cfg.Redis.Host, strconv.Itoa(firstInt(cfg.Redis.Port, 6379)))

	start := time.Now()
	dialer := &net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := dialer.DialContext(ctx, "tcp", st.Address)
	st.LatencyMS = round2(float64(time.Since(start).Microseconds()) / 1000)
	if err != nil {
		st.Message = "Redis 不可达: " + err.Error()
		return st
	}
	_ = conn.Close()
	st.Online = true
	st.Message = "连接正常"
	return st
}

// buildInfo 从 Go 构建信息中读取版本与 VCS 时间。
func buildInfo() (version, buildTime string) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	version = bi.Main.Version
	if version == "(devel)" || version == "" {
		version = ""
	}
	for _, setting := range bi.Settings {
		if setting.Key == "vcs.time" {
			buildTime = setting.Value
		}
	}
	return version, buildTime
}

// toDefaultMediaServerSummary 默认节点摘要（不返回 secret）。
func toDefaultMediaServerSummary(ms *model.GbMediaServer) *response.GbDefaultMediaServerSummary {
	if ms == nil {
		return nil
	}
	return &response.GbDefaultMediaServerSummary{
		ID:              ms.ID,
		Name:            ms.Name,
		ServerID:        ms.ServerID,
		IP:              ms.IP,
		HTTPPort:        ms.HTTPPort,
		StreamIP:        firstNonEmpty(ms.StreamIP, ms.IP),
		Online:          boolValue(ms.Online),
		LastKeepaliveDT: ms.LastKeepaliveDT,
	}
}

func isNotFound(err error) bool { return err == gorm.ErrRecordNotFound }

// firstInt 返回第一个非零整数。
func firstInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}
