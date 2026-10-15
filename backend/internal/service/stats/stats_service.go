package stats

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// StatsService 统计分析业务逻辑。
//
// 统计口径（详见接口契约 5.9 与需求「首页 / 统计分析 / 可视化大屏」章节）：
//   - 首页看板：今日/本周/本月 预警与处理（风险、告警分别给出，另给合计），
//     设备统计、风险类型分析、风险处理统计、区域风险统计（近 30 天），风险/告警趋势（近 7 天）；
//   - 风险/告警分析：按查询区间给出总量、处理情况与类型/区域/等级/设备（Top 10）/逐日趋势；
//   - 可视化大屏：实时最新 10 条风险与告警、近 30 天类型统计、设备与流量监控；
//   - 流量监控：物联网卡额度汇总与明细（剩余流量升序）。
//
// 本层负责时间区间缺省归一化（start_dt 缺省 30 天前 00:00:00，end_dt 缺省当前时间），
// 聚合查询本身集中在 repository.StatsRepo。
type StatsService struct{}

// NewStatsService 构造统计分析服务。
func NewStatsService() *StatsService { return &StatsService{} }

// statsDTLayout 统计接口时间参数格式。
const statsDTLayout = "2006-01-02 15:04:05"

// normalizeRange 归一化 start_dt / end_dt：
// 为空或缺省时 start 取 30 天前 00:00:00、end 取当前时间；
// 仅给日期（yyyy-MM-dd）的 end 补足到当天 23:59:59，避免漏掉当天数据。
func normalizeRange(startDT, endDT string) (string, string) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -30)
	end := now

	if t, ok := parseDT(startDT); ok {
		start = t
	}
	if t, ok := parseDT(endDT); ok {
		if len(strings.TrimSpace(endDT)) == len("2006-01-02") {
			t = t.Add(24*time.Hour - time.Second)
		}
		end = t
	}
	if end.Before(start) {
		start, end = end, start
	}
	return start.Format(statsDTLayout), end.Format(statsDTLayout)
}

// parseDT 解析时间字符串，支持带/不带秒的时间以及纯日期。
func parseDT(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{statsDTLayout, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Dashboard 首页看板。
func (s *StatsService) Dashboard(ctx context.Context, ds *utils.DataScope) (*response.DashboardResp, error) {
	return repository.NewStatsRepo().Dashboard(ctx, ds)
}

// RiskStats 风险统计分析。startDT / endDT 为空时取近 30 天。
func (s *StatsService) RiskStats(ctx context.Context, ds *utils.DataScope, startDT, endDT string) (*response.RiskStatsResp, error) {
	start, end := normalizeRange(startDT, endDT)
	return repository.NewStatsRepo().RiskStats(ctx, ds, start, end)
}

// AlarmStats 设备告警分析。startDT / endDT 为空时取近 30 天。
func (s *StatsService) AlarmStats(ctx context.Context, ds *utils.DataScope, startDT, endDT string) (*response.AlarmStatsResp, error) {
	start, end := normalizeRange(startDT, endDT)
	return repository.NewStatsRepo().AlarmStats(ctx, ds, start, end)
}

// Screen 可视化大屏。
func (s *StatsService) Screen(ctx context.Context, ds *utils.DataScope) (*response.ScreenResp, error) {
	return repository.NewStatsRepo().Screen(ctx, ds)
}

// TrafficStats 物联网卡流量监控。
func (s *StatsService) TrafficStats(ctx context.Context, ds *utils.DataScope) (*response.TrafficStatsResp, error) {
	return repository.NewStatsRepo().TrafficStats(ctx, ds)
}

// DeviceStatus 设备状态统计。
func (s *StatsService) DeviceStatus(ctx context.Context, ds *utils.DataScope) (*response.DeviceStatusResp, error) {
	return repository.NewStatsRepo().DeviceStatus(ctx, ds)
}
