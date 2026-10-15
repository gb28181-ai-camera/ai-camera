package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// StatsRepo 统计分析数据访问。
//
// 实现约定：
//  1. 全部统计查询集中在本文件，使用 GORM 聚合（Select + Group）或原生 SQL 片段；
//  2. 凡表带 dept_id 的一律经 ds.ApplyToQuery 施加行级过滤，别名必须与查询内别名一致；
//  3. 时间边界由 time.Now() 派生的常量格式化为 SQL 字面量后内联（非用户输入，无注入风险），
//     避免多条件聚合时 GORM 占位符顺序带来的歧义；
//  4. 无数据的自然日由 fillTrend 补零，保证前端图表 X 轴连续。
type StatsRepo struct{}

// NewStatsRepo 构造统计分析仓储。
func NewStatsRepo() *StatsRepo { return &StatsRepo{} }

// ────────────────────────────────────────────────────────────────
// 内部工具
// ────────────────────────────────────────────────────────────────

// statsDTLayout 统计查询时间格式。
const statsDTLayout = "2006-01-02 15:04:05"

// statsLit 把时间格式化为 SQL 字面量（单引号包裹）。
func statsLit(t time.Time) string {
	return "'" + t.Format(statsDTLayout) + "'"
}

// parseStatsDT 解析时间字符串，支持 "2006-01-02 15:04:05"、"2006-01-02T15:04:05"、"2006-01-02"。
func parseStatsDT(s string) (time.Time, bool) {
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

// resolveStatsRange 归一化统计区间：start 缺省 30 天前 00:00:00，end 缺省当前时间。
// 仅给出日期（yyyy-MM-dd）的 end_dt 视为当天 23:59:59。
func resolveStatsRange(startDT, endDT string) (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -30)
	end := now

	if t, ok := parseStatsDT(startDT); ok {
		start = t
	}
	if t, ok := parseStatsDT(endDT); ok {
		if len(strings.TrimSpace(endDT)) == len("2006-01-02") {
			t = t.Add(24*time.Hour - time.Second)
		}
		end = t
	}
	if end.Before(start) {
		start, end = end, start
	}
	return start, end
}

// todayStart 返回今日 00:00:00。
func todayStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// weekStart 返回本周一 00:00:00。
func weekStart() time.Time {
	t := todayStart()
	return t.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
}

// monthStart 返回本月 1 日 00:00:00。
func monthStart() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

// periodCount 周期计数：今日/本周/本月 发生数与处理数。
type periodCount struct {
	TodayCount   int64 `gorm:"column:today_count"`
	WeekCount    int64 `gorm:"column:week_count"`
	MonthCount   int64 `gorm:"column:month_count"`
	TodayHandled int64 `gorm:"column:today_handled"`
	WeekHandled  int64 `gorm:"column:week_handled"`
	MonthHandled int64 `gorm:"column:month_handled"`
}

// periodCounts 一次查询取回 今日/本周/本月 的发生数与处理数。
// table 形如 "ac_risk_record AS rr"，alias 为 "rr"，timeCol 为发生时间列（含别名）。
func (r *StatsRepo) periodCounts(ctx context.Context, ds *utils.DataScope, table, alias, timeCol string) (*periodCount, error) {
	today, week, month := todayStart(), weekStart(), monthStart()

	sel := fmt.Sprintf(`COUNT(*) FILTER (WHERE %[1]s >= %[2]s) AS today_count,
COUNT(*) FILTER (WHERE %[1]s >= %[3]s) AS week_count,
COUNT(*) FILTER (WHERE %[1]s >= %[4]s) AS month_count,
COUNT(*) FILTER (WHERE status = 'handled' AND handled_dt >= %[2]s) AS today_handled,
COUNT(*) FILTER (WHERE status = 'handled' AND handled_dt >= %[3]s) AS week_handled,
COUNT(*) FILTER (WHERE status = 'handled' AND handled_dt >= %[4]s) AS month_handled`,
		timeCol, statsLit(today), statsLit(week), statsLit(month))

	q := bootstrap.DB().WithContext(ctx).Table(table).
		Where(alias + ".deleted_at IS NULL")
	q = ds.ApplyToQuery(q, alias+".dept_id")

	var row periodCount
	if err := q.Select(sel).Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// groupSpec 分组统计描述。
type groupSpec struct {
	table      string // FROM 片段（含别名），如 "ac_risk_record AS rr"
	alias      string // 主表别名，用于软删除与数据权限限定
	join       string // JOIN 片段，可空
	nameExpr   string // 分组名称表达式（含别名限定）
	timeCol    string // 时间过滤列（含别名限定），空则不按时间过滤
	extraWhere string // 附加过滤条件，可空
	limit      int    // 0 表示不限
}

// groupByName 按 nameExpr 分组计数，结果按数量倒序。
func (r *StatsRepo) groupByName(ctx context.Context, ds *utils.DataScope, g groupSpec, from, to time.Time) ([]response.NameCount, error) {
	q := bootstrap.DB().WithContext(ctx).Table(g.table).
		Select(g.nameExpr + " AS name, COUNT(*) AS count")
	if g.join != "" {
		q = q.Joins(g.join)
	}
	q = q.Where(g.alias + ".deleted_at IS NULL")
	if g.timeCol != "" {
		q = q.Where(g.timeCol+" >= ? AND "+g.timeCol+" <= ?", from, to)
	}
	if g.extraWhere != "" {
		q = q.Where(g.extraWhere)
	}
	q = ds.ApplyToQuery(q, g.alias+".dept_id")
	q = q.Group(g.nameExpr).Order("count DESC")
	if g.limit > 0 {
		q = q.Limit(g.limit)
	}

	list := make([]response.NameCount, 0, 8)
	if err := q.Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// handleStat 按状态计数 pending / handled。
func (r *StatsRepo) handleStat(ctx context.Context, ds *utils.DataScope, table, alias, timeCol string, from, to time.Time) (*response.HandleStat, error) {
	sel := fmt.Sprintf(`COUNT(*) FILTER (WHERE status = 'pending') AS pending,
COUNT(*) FILTER (WHERE status = 'handled') AS handled`)

	q := bootstrap.DB().WithContext(ctx).Table(table).
		Where(alias+".deleted_at IS NULL").
		Where(timeCol+" >= ? AND "+timeCol+" <= ?", from, to)
	q = ds.ApplyToQuery(q, alias+".dept_id")

	var row response.HandleStat
	if err := q.Select(sel).Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// totalsRow 按状态的汇总计数。
type totalsRow struct {
	Total   int64 `gorm:"column:total"`
	Handled int64 `gorm:"column:handled"`
	Pending int64 `gorm:"column:pending"`
}

// trafficTotalsRow 物联网卡流量汇总。
type trafficTotalsRow struct {
	TotalTraffic int64 `gorm:"column:total_traffic"`
	UsedTraffic  int64 `gorm:"column:used_traffic"`
	LeftTraffic  int64 `gorm:"column:left_traffic"`
}

// totals 按状态的汇总计数（total / handled / pending）。
func (r *StatsRepo) totals(ctx context.Context, ds *utils.DataScope, table, alias, timeCol string, from, to time.Time) (*totalsRow, error) {
	sel := `COUNT(*) AS total,
COUNT(*) FILTER (WHERE status = 'handled') AS handled,
COUNT(*) FILTER (WHERE status = 'pending') AS pending`

	q := bootstrap.DB().WithContext(ctx).Table(table).
		Where(alias+".deleted_at IS NULL").
		Where(timeCol+" >= ? AND "+timeCol+" <= ?", from, to)
	q = ds.ApplyToQuery(q, alias+".dept_id")

	var row totalsRow
	if err := q.Select(sel).Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// trendSeries 逐日趋势（按日期分组计数），未补零。
func (r *StatsRepo) trendSeries(ctx context.Context, ds *utils.DataScope, table, alias, timeCol string, from, to time.Time) ([]response.TrendPoint, error) {
	dateExpr := fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD')", timeCol)

	q := bootstrap.DB().WithContext(ctx).Table(table).
		Where(alias+".deleted_at IS NULL").
		Where(timeCol+" >= ? AND "+timeCol+" <= ?", from, to)
	q = ds.ApplyToQuery(q, alias+".dept_id")
	q = q.Select(dateExpr + " AS date, COUNT(*) AS count").
		Group(dateExpr).Order(dateExpr + " ASC")

	list := make([]response.TrendPoint, 0, 16)
	if err := q.Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// fillTrend 把逐日趋势补零为 [from, to] 的连续日序列。
func fillTrend(rows []response.TrendPoint, from, to time.Time) []response.TrendPoint {
	byDate := make(map[string]int64, len(rows))
	for _, row := range rows {
		byDate[row.Date] = row.Count
	}

	start := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	end := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())

	list := make([]response.TrendPoint, 0, 16)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		list = append(list, response.TrendPoint{Date: key, Count: byDate[key]})
	}
	return list
}

// deviceStatusCounts 汇总设备状态（总数/在线/离线/低电量/存储将满）。
func (r *StatsRepo) deviceStatusCounts(ctx context.Context, ds *utils.DataScope) (*response.DeviceStatusResp, error) {
	// 低电量：battery < 20；存储将满：storage_free < storage_total 的 10%（用整数乘法避免浮点误差）。
	sel := `COUNT(*) AS total,
COUNT(*) FILTER (WHERE status = 'online') AS online,
COUNT(*) FILTER (WHERE status = 'offline') AS offline,
COUNT(*) FILTER (WHERE battery < 20) AS low_battery,
COUNT(*) FILTER (WHERE storage_total > 0 AND storage_free * 10 < storage_total) AS full_storage`

	q := bootstrap.DB().WithContext(ctx).Table("ac_device AS d").
		Where("d.deleted_at IS NULL")
	q = ds.ApplyToQuery(q, "d.dept_id")

	var row response.DeviceStatusResp
	if err := q.Select(sel).Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// trafficItems 物联网卡流量明细。
func (r *StatsRepo) trafficItems(ctx context.Context, ds *utils.DataScope, limit int) ([]response.IotCardTrafficStat, error) {
	sel := `c.card_no AS card_no,
c.name AS name,
COALESCE(rg.name, '') AS region_name,
COALESCE(dv.name, '') AS device_name,
COALESCE(c.traffic_total, 0) AS traffic_total,
COALESCE(c.traffic_used, 0) AS traffic_used,
COALESCE(c.traffic_left, 0) AS traffic_left,
c.status AS status`

	q := bootstrap.DB().WithContext(ctx).Table("ac_iot_card AS c").
		Joins("LEFT JOIN ac_region AS rg ON rg.id = c.region_id").
		Joins("LEFT JOIN ac_device AS dv ON dv.id = c.device_id").
		Where("c.deleted_at IS NULL")
	q = ds.ApplyToQuery(q, "c.dept_id")
	q = q.Select(sel).Order("c.traffic_left ASC, c.id ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}

	list := make([]response.IotCardTrafficStat, 0, 8)
	if err := q.Scan(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// 风险/告警两张业务表的分组维度描述，供 Dashboard / Screen / 统计分析复用。
var (
	riskTypeGroupSpec = groupSpec{
		table:    "ac_risk_record AS rr",
		alias:    "rr",
		join:     "LEFT JOIN ac_risk_type AS rt ON rt.id = rr.risk_type_id",
		nameExpr: "COALESCE(rt.name, '未分类')",
		timeCol:  "rr.occurred_dt",
	}
	riskRegionGroupSpec = groupSpec{
		table:    "ac_risk_record AS rr",
		alias:    "rr",
		join:     "LEFT JOIN ac_region AS rg ON rg.id = rr.region_id",
		nameExpr: "COALESCE(rg.name, '未分配区域')",
		timeCol:  "rr.occurred_dt",
	}
	riskLevelGroupSpec = groupSpec{
		table:    "ac_risk_record AS rr",
		alias:    "rr",
		nameExpr: "COALESCE(NULLIF(rr.risk_level, ''), 'unknown')",
		timeCol:  "rr.occurred_dt",
	}
	riskDeviceGroupSpec = groupSpec{
		table:    "ac_risk_record AS rr",
		alias:    "rr",
		join:     "LEFT JOIN ac_device AS dv ON dv.id = rr.device_id",
		nameExpr: "COALESCE(dv.name, '未知设备')",
		timeCol:  "rr.occurred_dt",
		limit:    10,
	}

	alarmTypeGroupSpec = groupSpec{
		table:    "ac_alarm_record AS ar",
		alias:    "ar",
		join:     "LEFT JOIN ac_alarm_type AS at ON at.id = ar.alarm_type_id",
		nameExpr: "COALESCE(at.name, '未分类')",
		timeCol:  "ar.occurred_dt",
	}
	alarmRegionGroupSpec = groupSpec{
		table:    "ac_alarm_record AS ar",
		alias:    "ar",
		join:     "LEFT JOIN ac_region AS rg ON rg.id = ar.region_id",
		nameExpr: "COALESCE(rg.name, '未分配区域')",
		timeCol:  "ar.occurred_dt",
	}
	alarmLevelGroupSpec = groupSpec{
		table:    "ac_alarm_record AS ar",
		alias:    "ar",
		nameExpr: "COALESCE(NULLIF(ar.alarm_level, ''), 'unknown')",
		timeCol:  "ar.occurred_dt",
	}
	alarmDeviceGroupSpec = groupSpec{
		table:    "ac_alarm_record AS ar",
		alias:    "ar",
		join:     "LEFT JOIN ac_device AS dv ON dv.id = ar.device_id",
		nameExpr: "COALESCE(dv.name, '未知设备')",
		timeCol:  "ar.occurred_dt",
		limit:    10,
	}
)

// ────────────────────────────────────────────────────────────────
// 首页看板
// ────────────────────────────────────────────────────────────────

// Dashboard 汇总首页看板数据。近 30 天口径用于分组统计与处理统计，近 7 天用于趋势。
func (r *StatsRepo) Dashboard(ctx context.Context, ds *utils.DataScope) (*response.DashboardResp, error) {
	out := &response.DashboardResp{}

	// 今日/本周/本月：风险与告警分别取发生数、处理数。
	risk, err := r.periodCounts(ctx, ds, "ac_risk_record AS rr", "rr", "rr.occurred_dt")
	if err != nil {
		return nil, err
	}
	alarm, err := r.periodCounts(ctx, ds, "ac_alarm_record AS ar", "ar", "ar.occurred_dt")
	if err != nil {
		return nil, err
	}

	out.TodayRiskCount = risk.TodayCount
	out.TodayAlarmCount = alarm.TodayCount
	out.TodayRiskHandled = risk.TodayHandled
	out.TodayAlarmHandled = alarm.TodayHandled
	out.TodayHandledCount = risk.TodayHandled + alarm.TodayHandled

	out.WeekRiskCount = risk.WeekCount
	out.WeekAlarmCount = alarm.WeekCount
	out.WeekRiskHandled = risk.WeekHandled
	out.WeekAlarmHandled = alarm.WeekHandled
	out.WeekHandledCount = risk.WeekHandled + alarm.WeekHandled

	out.MonthRiskCount = risk.MonthCount
	out.MonthAlarmCount = alarm.MonthCount
	out.MonthRiskHandled = risk.MonthHandled
	out.MonthAlarmHandled = alarm.MonthHandled
	out.MonthHandledCount = risk.MonthHandled + alarm.MonthHandled

	// 设备统计。
	devStat, err := r.deviceStatusCounts(ctx, ds)
	if err != nil {
		return nil, err
	}
	out.DeviceStat = response.DeviceStat{
		Total:   devStat.Total,
		Online:  devStat.Online,
		Offline: devStat.Offline,
	}

	today := todayStart()
	from30 := today.AddDate(0, 0, -29)
	now := time.Now()
	from7 := today.AddDate(0, 0, -6)

	// 风险类型分析 / 区域风险统计（近 30 天）。
	if out.RiskTypeStats, err = r.groupByName(ctx, ds, riskTypeGroupSpec, from30, now); err != nil {
		return nil, err
	}
	if out.RegionRiskStats, err = r.groupByName(ctx, ds, riskRegionGroupSpec, from30, now); err != nil {
		return nil, err
	}

	// 风险处理统计（近 30 天）。
	handle, err := r.handleStat(ctx, ds, "ac_risk_record AS rr", "rr", "rr.occurred_dt", from30, now)
	if err != nil {
		return nil, err
	}
	out.RiskHandleStats = *handle

	// 风险/告警趋势（近 7 天，补零保证 7 个点）。
	riskTrend, err := r.trendSeries(ctx, ds, "ac_risk_record AS rr", "rr", "rr.occurred_dt", from7, now)
	if err != nil {
		return nil, err
	}
	out.RiskTrend = fillTrend(riskTrend, from7, now)

	alarmTrend, err := r.trendSeries(ctx, ds, "ac_alarm_record AS ar", "ar", "ar.occurred_dt", from7, now)
	if err != nil {
		return nil, err
	}
	out.AlarmTrend = fillTrend(alarmTrend, from7, now)

	return out, nil
}

// ────────────────────────────────────────────────────────────────
// 风险统计分析 / 设备告警分析
// ────────────────────────────────────────────────────────────────

// RiskStats 风险统计分析。start_dt / end_dt 缺省为近 30 天。
func (r *StatsRepo) RiskStats(ctx context.Context, ds *utils.DataScope, startDT, endDT string) (*response.RiskStatsResp, error) {
	from, to := resolveStatsRange(startDT, endDT)

	base, err := r.totals(ctx, ds, "ac_risk_record AS rr", "rr", "rr.occurred_dt", from, to)
	if err != nil {
		return nil, err
	}
	out := &response.RiskStatsResp{
		Total:   base.Total,
		Handled: base.Handled,
		Pending: base.Pending,
	}

	if out.ByType, err = r.groupByName(ctx, ds, riskTypeGroupSpec, from, to); err != nil {
		return nil, err
	}
	if out.ByRegion, err = r.groupByName(ctx, ds, riskRegionGroupSpec, from, to); err != nil {
		return nil, err
	}
	if out.ByLevel, err = r.groupByName(ctx, ds, riskLevelGroupSpec, from, to); err != nil {
		return nil, err
	}
	if out.ByDevice, err = r.groupByName(ctx, ds, riskDeviceGroupSpec, from, to); err != nil {
		return nil, err
	}

	rows, err := r.trendSeries(ctx, ds, "ac_risk_record AS rr", "rr", "rr.occurred_dt", from, to)
	if err != nil {
		return nil, err
	}
	out.Trend = fillTrend(rows, from, to)

	return out, nil
}

// AlarmStats 设备告警分析。start_dt / end_dt 缺省为近 30 天。
func (r *StatsRepo) AlarmStats(ctx context.Context, ds *utils.DataScope, startDT, endDT string) (*response.AlarmStatsResp, error) {
	from, to := resolveStatsRange(startDT, endDT)

	base, err := r.totals(ctx, ds, "ac_alarm_record AS ar", "ar", "ar.occurred_dt", from, to)
	if err != nil {
		return nil, err
	}
	out := &response.AlarmStatsResp{
		Total:   base.Total,
		Handled: base.Handled,
		Pending: base.Pending,
	}

	if out.ByType, err = r.groupByName(ctx, ds, alarmTypeGroupSpec, from, to); err != nil {
		return nil, err
	}
	if out.ByRegion, err = r.groupByName(ctx, ds, alarmRegionGroupSpec, from, to); err != nil {
		return nil, err
	}
	if out.ByLevel, err = r.groupByName(ctx, ds, alarmLevelGroupSpec, from, to); err != nil {
		return nil, err
	}
	if out.ByDevice, err = r.groupByName(ctx, ds, alarmDeviceGroupSpec, from, to); err != nil {
		return nil, err
	}

	rows, err := r.trendSeries(ctx, ds, "ac_alarm_record AS ar", "ar", "ar.occurred_dt", from, to)
	if err != nil {
		return nil, err
	}
	out.Trend = fillTrend(rows, from, to)

	return out, nil
}

// ────────────────────────────────────────────────────────────────
// 可视化大屏
// ────────────────────────────────────────────────────────────────

// Screen 汇总可视化大屏数据。实时列表取最新 10 条，分组统计取近 30 天。
func (r *StatsRepo) Screen(ctx context.Context, ds *utils.DataScope) (*response.ScreenResp, error) {
	out := &response.ScreenResp{}

	// 风险实时告警：最新 10 条，预载类型/设备/区域名称。
	riskList := make([]model.RiskRecord, 0, 10)
	riskQ := bootstrap.DB().WithContext(ctx).Model(&model.RiskRecord{}).
		Preload("RiskType").Preload("Device").Preload("Region")
	riskQ = ds.ApplyToQuery(riskQ, "dept_id")
	if err := riskQ.Order("occurred_dt DESC, id DESC").Limit(10).Find(&riskList).Error; err != nil {
		return nil, err
	}
	out.RiskRealtime = riskList

	// 设备状态告警：最新 10 条。
	alarmList := make([]model.AlarmRecord, 0, 10)
	alarmQ := bootstrap.DB().WithContext(ctx).Model(&model.AlarmRecord{}).
		Preload("AlarmType").Preload("Device").Preload("Region")
	alarmQ = ds.ApplyToQuery(alarmQ, "dept_id")
	if err := alarmQ.Order("occurred_dt DESC, id DESC").Limit(10).Find(&alarmList).Error; err != nil {
		return nil, err
	}
	out.AlarmRealtime = alarmList

	today := todayStart()
	from30 := today.AddDate(0, 0, -29)
	now := time.Now()

	// 风险统计 / 设备告警统计（近 30 天）。
	var err error
	if out.RiskStats, err = r.groupByName(ctx, ds, riskTypeGroupSpec, from30, now); err != nil {
		return nil, err
	}
	if out.AlarmStats, err = r.groupByName(ctx, ds, alarmTypeGroupSpec, from30, now); err != nil {
		return nil, err
	}

	// 设备监控：总数/在线/离线。
	devStat, err := r.deviceStatusCounts(ctx, ds)
	if err != nil {
		return nil, err
	}
	out.DeviceMonitor = response.DeviceStat{
		Total:   devStat.Total,
		Online:  devStat.Online,
		Offline: devStat.Offline,
	}

	// 流量监控：剩余流量最少的 10 张物联网卡。
	if out.TrafficMonitor, err = r.trafficItems(ctx, ds, 10); err != nil {
		return nil, err
	}

	// 今日风险/告警数。
	riskPeriod, err := r.periodCounts(ctx, ds, "ac_risk_record AS rr", "rr", "rr.occurred_dt")
	if err != nil {
		return nil, err
	}
	alarmPeriod, err := r.periodCounts(ctx, ds, "ac_alarm_record AS ar", "ar", "ar.occurred_dt")
	if err != nil {
		return nil, err
	}
	out.TodayRiskCount = riskPeriod.TodayCount
	out.TodayAlarmCount = alarmPeriod.TodayCount

	return out, nil
}

// ────────────────────────────────────────────────────────────────
// 物联网卡流量监控 / 设备状态统计
// ────────────────────────────────────────────────────────────────

// TrafficStats 物联网卡流量监控：汇总额度与全部卡片明细（剩余流量升序）。
func (r *StatsRepo) TrafficStats(ctx context.Context, ds *utils.DataScope) (*response.TrafficStatsResp, error) {
	sel := `COALESCE(SUM(c.traffic_total), 0) AS total_traffic,
COALESCE(SUM(c.traffic_used), 0) AS used_traffic,
COALESCE(SUM(c.traffic_left), 0) AS left_traffic`

	q := bootstrap.DB().WithContext(ctx).Table("ac_iot_card AS c").
		Where("c.deleted_at IS NULL")
	q = ds.ApplyToQuery(q, "c.dept_id")

	var sum trafficTotalsRow
	if err := q.Select(sel).Scan(&sum).Error; err != nil {
		return nil, err
	}

	items, err := r.trafficItems(ctx, ds, 0)
	if err != nil {
		return nil, err
	}
	return &response.TrafficStatsResp{
		TotalTraffic: sum.TotalTraffic,
		UsedTraffic:  sum.UsedTraffic,
		LeftTraffic:  sum.LeftTraffic,
		Items:        items,
	}, nil
}

// DeviceStatus 设备状态统计：总数/在线/离线/低电量/存储将满。
func (r *StatsRepo) DeviceStatus(ctx context.Context, ds *utils.DataScope) (*response.DeviceStatusResp, error) {
	return r.deviceStatusCounts(ctx, ds)
}
