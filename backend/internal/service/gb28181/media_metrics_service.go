package gb28181

// 媒体节点主机指标采集与历史环缓冲。
//
// 为什么采集「后端所在主机」而不是 ZLM 接口：
//   - ZLM 没有暴露 CPU / 内存 / 磁盘 / 负载的 REST API；
//   - WVP 的媒体节点监控同样用 oshi 采集平台所在主机，语义对齐；
//   - 单机部署（本项目的默认形态）下，后端与 ZLM 同机，指标完全等价。
// 多副本部署时各副本只能看到自身节点，属已知限制（设计文档注明）。

import (
	"context"
	"errors"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	cpu "github.com/shirou/gopsutil/v4/cpu"
	disk "github.com/shirou/gopsutil/v4/disk"
	load "github.com/shirou/gopsutil/v4/load"
	mem "github.com/shirou/gopsutil/v4/mem"
	netio "github.com/shirou/gopsutil/v4/net"
	"gorm.io/gorm"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

const (
	// metricsSampleInterval 历史采样间隔。
	metricsSampleInterval = 5 * time.Second
	// metricsRetention 历史保留时长（与 24h 查询范围一致）。
	metricsRetention = 24 * time.Hour
	// metricsCPUWindow 单次 CPU 百分比采样的最小窗口。
	//
	// 200ms 是「不显著拖慢 HTTP 请求」与「读数有意义」的折中：
	// gopsutil 的无间隔调用返回的是开机以来的平均值，无法反映当前负载。
	metricsCPUWindow = 200 * time.Millisecond
)

// ErrGbMediaMetricsUnavailable 指标采集失败（不常见，通常为宿主限制）。
var ErrGbMediaMetricsUnavailable = errors.New("主机指标采集失败")

// metricsBuffer 单节点的时间序列环缓冲。
type metricsBuffer struct {
	mu     sync.Mutex
	points []response.GbMediaMetricPoint
}

const metricsMaxPoints = int(metricsRetention / metricsSampleInterval)

// append 写入一个采样点并淘汰超过保留期的数据。
func (b *metricsBuffer) append(p response.GbMediaMetricPoint) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.points = append(b.points, p)
	cutoff := p.TS.Add(-metricsRetention)
	idx := 0
	for idx < len(b.points) && b.points[idx].TS.Before(cutoff) {
		idx++
	}
	if idx > 0 {
		b.points = append([]response.GbMediaMetricPoint(nil), b.points[idx:]...)
	}
	if len(b.points) > metricsMaxPoints {
		b.points = append([]response.GbMediaMetricPoint(nil), b.points[len(b.points)-metricsMaxPoints:]...)
	}
}

// since 返回指定时间之后的采样点（升序）。
func (b *metricsBuffer) since(from time.Time) []response.GbMediaMetricPoint {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]response.GbMediaMetricPoint, 0, len(b.points))
	for _, p := range b.points {
		if !p.TS.Before(from) {
			out = append(out, p)
		}
	}
	return out
}

var (
	metricsBuffers   sync.Map // server_id -> *metricsBuffer
	metricsSamplerON = true
	metricsSampler   sync.Once
	lastNetSample    struct {
		sync.Mutex
		rx, tx uint64
		ts     time.Time
	}
)

// SetMetricsSamplerEnabled 开关后台采样器（测试中关闭，避免后台 goroutine 干扰事务库）。
func SetMetricsSamplerEnabled(enabled bool) {
	metricsSamplerON = enabled
}

// ClearMetricsBuffers 清空全部节点的指标历史（测试用）。
func ClearMetricsBuffers() {
	metricsBuffers.Range(func(key, _ interface{}) bool {
		metricsBuffers.Delete(key)
		return true
	})
}

func metricsBufferFor(serverID string) *metricsBuffer {
	v, _ := metricsBuffers.LoadOrStore(serverID, &metricsBuffer{})
	return v.(*metricsBuffer)
}

// GbMediaMetricsService 媒体节点指标服务。
type GbMediaMetricsService struct {
	serverRepo *repository.GbMediaServerRepo
	extraRepo  *repository.GbOpsExtraRepo
}

// NewGbMediaMetricsService 构造指标服务。
func NewGbMediaMetricsService() *GbMediaMetricsService {
	return &GbMediaMetricsService{
		serverRepo: repository.NewGbMediaServerRepo(),
		extraRepo:  repository.NewGbOpsExtraRepo(),
	}
}

// Metrics 返回节点当前指标（主机资源 + 节点在线 + 当前流数）。
func (s *GbMediaMetricsService) Metrics(ctx context.Context, id int64) (*response.GbMediaMetricsResp, error) {
	ms, err := s.serverRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMediaServerNotFound
		}
		return nil, err
	}

	host, err := CollectHostMetrics(ctx)
	if err != nil {
		return nil, err
	}

	resp := &response.GbMediaMetricsResp{
		MediaServerID:   ms.ServerID,
		MediaServerName: ms.Name,
		Host:            host,
		SampledDT:       time.Now(),
	}

	// 当前流数：查一次 getMediaList；节点不可达时 online=false，但不让整个
	// 指标接口失败 —— 主机指标仍然有诊断价值。
	mediaCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	infos, mediaErr := mediaserver.FromModel(ms).GetMediaList(mediaCtx, "", "", "", "")
	if mediaErr != nil {
		resp.Online = false
		resp.Message = "媒体节点不可达: " + mediaErr.Error()
	} else {
		resp.Online = true
		resp.StreamCount = len(infos)
		resp.Message = "节点在线"
	}

	s.record(ms.ServerID, resp)
	s.startSampler()
	return resp, nil
}

// History 返回指定范围的历史采样点。range 仅支持 1h / 24h。
func (s *GbMediaMetricsService) History(ctx context.Context, id int64, rangeStr string) (*response.GbMediaMetricsHistoryResp, error) {
	ms, err := s.serverRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbMediaServerNotFound
		}
		return nil, err
	}

	dur := time.Hour
	switch rangeStr {
	case "", "1h":
		dur = time.Hour
	case "24h":
		dur = 24 * time.Hour
	default:
		return nil, errors.New("range 仅支持 1h 或 24h")
	}

	// 没有历史时补一个当前点，避免前端拿到空数组无法渲染。
	buf := metricsBufferFor(ms.ServerID)
	if len(buf.since(time.Now().Add(-dur))) == 0 {
		if host, err := CollectHostMetrics(ctx); err == nil {
			s.record(ms.ServerID, &response.GbMediaMetricsResp{
				MediaServerID:   ms.ServerID,
				MediaServerName: ms.Name,
				Host:            host,
				SampledDT:       time.Now(),
				Online:          boolValue(ms.Online),
			})
		}
	}

	points := buf.since(time.Now().Add(-dur))
	if points == nil {
		points = []response.GbMediaMetricPoint{}
	}
	// 只看历史页的用户也应触发后台采样，否则时间序列永远只有访问瞬间的点。
	s.startSampler()
	return &response.GbMediaMetricsHistoryResp{
		MediaServerID: ms.ServerID,
		Range:         firstNonEmpty(rangeStr, "1h"),
		IntervalSec:   int(metricsSampleInterval.Seconds()),
		Items:         points,
	}, nil
}

// RecordSample 写入一个采样点。导出以便测试与将来的外部采集器复用。
func (s *GbMediaMetricsService) RecordSample(serverID string, p response.GbMediaMetricPoint) {
	metricsBufferFor(serverID).append(p)
}

// HistoryPoints 读取指定节点自 from 起的采样点（测试与内部复用）。
func (s *GbMediaMetricsService) HistoryPoints(serverID string, from time.Time) []response.GbMediaMetricPoint {
	return metricsBufferFor(serverID).since(from)
}

// record 把一次指标响应转为采样点。
func (s *GbMediaMetricsService) record(serverID string, resp *response.GbMediaMetricsResp) {
	s.RecordSample(serverID, response.GbMediaMetricPoint{
		TS:          resp.SampledDT,
		CPUPercent:  resp.Host.CPUPercent,
		MemPercent:  resp.Host.MemPercent,
		DiskPercent: resp.Host.DiskPercent,
		NetRxRate:   resp.Host.NetRxRate,
		NetTxRate:   resp.Host.NetTxRate,
		Load1:       resp.Host.Load1,
		StreamCount: resp.StreamCount,
		Online:      resp.Online,
	})
}

// startSampler 懒启动后台采样器：只在真正有人访问指标接口后才开始采样。
func (s *GbMediaMetricsService) startSampler() {
	if !metricsSamplerON {
		return
	}
	metricsSampler.Do(func() {
		go s.samplerLoop()
	})
}

func (s *GbMediaMetricsService) samplerLoop() {
	ticker := time.NewTicker(metricsSampleInterval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		servers, err := s.extraRepo.ListMediaServers(ctx)
		cancel()
		if err != nil {
			continue
		}
		for i := range servers {
			s.sampleOnce(&servers[i])
		}
	}
}

func (s *GbMediaMetricsService) sampleOnce(ms *model.GbMediaServer) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	host, err := CollectHostMetrics(ctx)
	if err != nil {
		return
	}
	p := response.GbMediaMetricPoint{
		TS:          time.Now(),
		CPUPercent:  host.CPUPercent,
		MemPercent:  host.MemPercent,
		DiskPercent: host.DiskPercent,
		NetRxRate:   host.NetRxRate,
		NetTxRate:   host.NetTxRate,
		Load1:       host.Load1,
	}
	mediaCtx, mcancel := context.WithTimeout(ctx, 2*time.Second)
	defer mcancel()
	if infos, err := mediaserver.FromModel(ms).GetMediaList(mediaCtx, "", "", "", ""); err == nil {
		p.Online = true
		p.StreamCount = len(infos)
	}
	s.RecordSample(ms.ServerID, p)
}

// CollectHostMetrics 采集后端所在主机的资源快照。
func CollectHostMetrics(ctx context.Context) (response.GbHostMetricsPayload, error) {
	out := response.GbHostMetricsPayload{
		OSInfo: runtime.GOOS + "/" + runtime.GOARCH,
	}

	percents, err := cpu.PercentWithContext(ctx, metricsCPUWindow, false)
	if err != nil || len(percents) == 0 {
		return out, errors.Join(ErrGbMediaMetricsUnavailable, err)
	}
	out.CPUPercent = round2(percents[0])
	if n, err := cpu.CountsWithContext(ctx, true); err == nil {
		out.CPUCores = n
	}

	// 负载均值在 Windows 不可用；失败时保留 0，不影响其他指标。
	if avg, err := load.AvgWithContext(ctx); err == nil && avg != nil {
		out.Load1, out.Load5, out.Load15 = round2(avg.Load1), round2(avg.Load5), round2(avg.Load15)
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil && vm != nil {
		out.MemUsed, out.MemTotal = vm.Used, vm.Total
		out.MemPercent = round2(vm.UsedPercent)
	}

	root := diskRootPath()
	if du, err := disk.UsageWithContext(ctx, root); err == nil && du != nil {
		out.DiskUsed, out.DiskTotal = du.Used, du.Total
		out.DiskPercent = round2(du.UsedPercent)
	}

	if counters, err := netio.IOCountersWithContext(ctx, false); err == nil {
		var rx, tx uint64
		for _, c := range counters {
			rx += c.BytesRecv
			tx += c.BytesSent
		}
		out.NetRxBytes, out.NetTxBytes = rx, tx
		out.NetRxRate, out.NetTxRate = netRates(rx, tx)
	}

	return out, nil
}

// diskRootPath 选择用于统计的磁盘：Windows 取系统盘，其余取根分区。
func diskRootPath() string {
	if runtime.GOOS == "windows" {
		if drive := getenvDefault("SystemDrive", "C:"); drive != "" {
			return drive + `\`
		}
		return `C:\`
	}
	return "/"
}

// netRates 依据相邻两次累计字节数计算速率（字节/秒）。
func netRates(rx, tx uint64) (float64, float64) {
	now := time.Now()
	lastNetSample.Lock()
	defer lastNetSample.Unlock()
	prevRx, prevTx, prevTS := lastNetSample.rx, lastNetSample.tx, lastNetSample.ts
	lastNetSample.rx, lastNetSample.tx, lastNetSample.ts = rx, tx, now
	if prevTS.IsZero() || now.Sub(prevTS) <= 0 || rx < prevRx || tx < prevTx {
		return 0, 0
	}
	elapsed := now.Sub(prevTS).Seconds()
	return round2(float64(rx-prevRx) / elapsed), round2(float64(tx-prevTx) / elapsed)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
