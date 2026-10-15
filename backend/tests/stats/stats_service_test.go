package stats

import (
	"context"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	svcalarm "github.com/boticz/ai-camera-backend/internal/service/alarm"
	svcbasic "github.com/boticz/ai-camera-backend/internal/service/basic"
	svcdevice "github.com/boticz/ai-camera-backend/internal/service/device"
	svcrisk "github.com/boticz/ai-camera-backend/internal/service/risk"
	svcstats "github.com/boticz/ai-camera-backend/internal/service/stats"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 统计分析（StatsService）：口径自洽 + 数据权限收敛
// ════════════════════════════════════════════════════════════════

func statsSvc() *svcstats.StatsService { return svcstats.NewStatsService() }

// statsRange 近 3 天的统计区间（覆盖用例新造的数据）。
// 结束时间取当天 23:59:59，保证用例刚创建的数据落在闭区间内且趋势恒为 4 个点。
func statsRange() (string, string) {
	now := time.Now()
	return now.AddDate(0, 0, -3).Format("2006-01-02 00:00:00"),
		now.Format("2006-01-02") + " 23:59:59"
}

// newDevice 创建设备（online 控制在线状态）。
func newDevice(t *testing.T, ctx context.Context, code string, online bool) *model.Device {
	t.Helper()
	region, err := svcbasic.NewRegionService().Create(ctx, &request.CreateRegionReq{
		Name: "ZZQ-统计区域", Code: uniq("ZZQ_SREG"),
	})
	if err != nil {
		t.Fatalf("创建区域失败: %v", err)
	}
	svc := svcdevice.NewDeviceService()
	d, err := svc.Create(ctx, &request.CreateDeviceReq{
		Name: "ZZQ-统计设备", DeviceCode: code, RegionID: &region.ID, Location: "东门",
	})
	if err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	if online {
		status := model.DeviceStatusOnline
		if err := svc.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: status, Battery: 88}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
	}
	return d
}

// newRisk 上报一条风险（handled 控制是否已处理）。
func newRisk(t *testing.T, ctx context.Context, handled bool) *model.RiskRecord {
	t.Helper()
	region, err := svcbasic.NewRegionService().Create(ctx, &request.CreateRegionReq{
		Name: "ZZQ-统计风险区域", Code: uniq("ZZQ_SRREG"),
	})
	if err != nil {
		t.Fatalf("创建区域失败: %v", err)
	}
	dev, err := svcdevice.NewDeviceService().Create(ctx, &request.CreateDeviceReq{
		Name: "ZZQ-统计风险设备", DeviceCode: uniq("ZZQ-SRDEV"), RegionID: &region.ID, Location: "东门",
	})
	if err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	rt, err := svcbasic.NewRiskTypeService().Create(ctx, &request.CreateRiskTypeReq{
		Name: "ZZQ-统计风险类型", Code: uniq("zzq_srt"), Level: model.LevelHigh,
	})
	if err != nil {
		t.Fatalf("创建风险类型失败: %v", err)
	}
	rec, err := svcstatsRiskReport(ctx, dev.DeviceCode, rt.Code)
	if err != nil {
		t.Fatalf("上报风险失败: %v", err)
	}
	if handled {
		if err := svcstatsRiskHandle(ctx, rec.ID); err != nil {
			t.Fatalf("处理风险失败: %v", err)
		}
	}
	return rec
}

// TestStatsService_Dashboard 首页看板口径自洽：合计 = 风险 + 告警。
func TestStatsService_Dashboard(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := statsSvc()

		before, err := svc.Dashboard(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询看板失败: %v", err)
		}

		// 今日新增 2 条风险（1 条已处理）
		newRisk(t, ctx, true)
		newRisk(t, ctx, false)

		after, err := svc.Dashboard(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询看板失败: %v", err)
		}
		if after.TodayRiskCount != before.TodayRiskCount+2 {
			t.Errorf("今日风险数 = %d, 期望 %d", after.TodayRiskCount, before.TodayRiskCount+2)
		}
		if after.TodayRiskHandled != before.TodayRiskHandled+1 {
			t.Errorf("今日风险处理数 = %d, 期望 %d", after.TodayRiskHandled, before.TodayRiskHandled+1)
		}
		if after.TodayHandledCount != after.TodayRiskHandled+after.TodayAlarmHandled {
			t.Errorf("合计处理数 %d != 风险 %d + 告警 %d",
				after.TodayHandledCount, after.TodayRiskHandled, after.TodayAlarmHandled)
		}
		if after.WeekRiskCount < after.TodayRiskCount {
			t.Errorf("本周风险数(%d) 应不小于今日(%d)", after.WeekRiskCount, after.TodayRiskCount)
		}
		if after.MonthRiskCount < after.WeekRiskCount {
			t.Errorf("本月风险数(%d) 应不小于本周(%d)", after.MonthRiskCount, after.WeekRiskCount)
		}
		if after.DeviceStat.Total != after.DeviceStat.Online+after.DeviceStat.Offline {
			t.Errorf("设备统计不自洽: %+v", after.DeviceStat)
		}
		if after.DeviceStat.Total < 0 || after.DeviceStat.Online < 0 || after.DeviceStat.Offline < 0 {
			t.Errorf("设备统计不应为负: %+v", after.DeviceStat)
		}
		if after.RiskTypeStats == nil || after.RegionRiskStats == nil || after.RiskTrend == nil || after.AlarmTrend == nil {
			t.Error("数组类字段不应为 nil")
		}
		if len(after.RiskTrend) != 7 || len(after.AlarmTrend) != 7 {
			t.Errorf("趋势应为近 7 天 7 个点: %d/%d", len(after.RiskTrend), len(after.AlarmTrend))
		}
		if after.RiskHandleStats.Pending+after.RiskHandleStats.Handled < 0 {
			t.Errorf("处理统计不应为负: %+v", after.RiskHandleStats)
		}
	})
}

// TestStatsService_RiskStats 风险统计：分组求和与总数一致，区间为闭区间。
func TestStatsService_RiskStats(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := statsSvc()
		start, end := statsRange()

		before, err := svc.RiskStats(ctx, dsAll(), start, end)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		newRisk(t, ctx, false)
		newRisk(t, ctx, true)
		newRisk(t, ctx, true)

		after, err := svc.RiskStats(ctx, dsAll(), start, end)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if after.Total != before.Total+3 {
			t.Errorf("total = %d, 期望 %d", after.Total, before.Total+3)
		}
		if after.Handled != before.Handled+2 {
			t.Errorf("handled = %d, 期望 %d", after.Handled, before.Handled+2)
		}
		if after.Total != after.Handled+after.Pending {
			t.Errorf("total(%d) != handled(%d) + pending(%d)", after.Total, after.Handled, after.Pending)
		}
		if sum := sumCounts(after.ByType); sum != after.Total {
			t.Errorf("按类型分组之和 %d != total %d", sum, after.Total)
		}
		if sum := sumCounts(after.ByRegion); sum != after.Total {
			t.Errorf("按区域分组之和 %d != total %d", sum, after.Total)
		}
		if sum := sumCounts(after.ByLevel); sum != after.Total {
			t.Errorf("按等级分组之和 %d != total %d", sum, after.Total)
		}
		if sum := sumTrend(after.Trend); sum != after.Total {
			t.Errorf("趋势之和 %d != total %d", sum, after.Total)
		}
		if len(after.Trend) != 4 {
			t.Errorf("趋势点数 = %d, 期望 4（近 3 天 + 今天）", len(after.Trend))
		}
		if after.ByType == nil || after.ByDevice == nil {
			t.Error("数组类字段不应为 nil")
		}

		// 区间反向时按代码实现自动交换起止（仍返回数据而非报错）
		swapped, err := svc.RiskStats(ctx, dsAll(), end, start)
		if err != nil {
			t.Fatalf("反向区间应可处理: %v", err)
		}
		if swapped.Total != after.Total {
			t.Errorf("反向区间应等价于原区间: %d vs %d", swapped.Total, after.Total)
		}

		// 无数据的过去区间：计数为 0，数组非 nil
		empty, err := svc.RiskStats(ctx, dsAll(), "2020-01-01 00:00:00", "2020-01-03 00:00:00")
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if empty.Total != 0 || empty.Handled != 0 || empty.Pending != 0 {
			t.Errorf("无数据区间应全为 0: %+v", empty)
		}
		if empty.ByType == nil || empty.Trend == nil {
			t.Error("无数据时数组类字段不应为 nil")
		}
		if len(empty.Trend) != 3 {
			t.Errorf("无数据区间的趋势点数 = %d, 期望 3", len(empty.Trend))
		}
	})
}

// TestStatsService_AlarmStats 告警统计：分组求和与总数一致。
func TestStatsService_AlarmStats(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := statsSvc()
		start, end := statsRange()

		before, err := svc.AlarmStats(ctx, dsAll(), start, end)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		newAlarm(t, ctx)
		newAlarm(t, ctx)

		after, err := svc.AlarmStats(ctx, dsAll(), start, end)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if after.Total != before.Total+2 {
			t.Errorf("total = %d, 期望 %d", after.Total, before.Total+2)
		}
		if after.Total != after.Handled+after.Pending {
			t.Errorf("total(%d) != handled(%d) + pending(%d)", after.Total, after.Handled, after.Pending)
		}
		if sum := sumCounts(after.ByType); sum != after.Total {
			t.Errorf("按类型分组之和 %d != total %d", sum, after.Total)
		}
		if sum := sumCounts(after.ByLevel); sum != after.Total {
			t.Errorf("按等级分组之和 %d != total %d", sum, after.Total)
		}
		if sum := sumTrend(after.Trend); sum != after.Total {
			t.Errorf("趋势之和 %d != total %d", sum, after.Total)
		}
		if after.ByType == nil || after.ByDevice == nil || after.Trend == nil {
			t.Error("数组类字段不应为 nil")
		}
	})
}

// TestStatsService_Screen 可视化大屏：8 个展示块均有值且自洽。
func TestStatsService_Screen(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := statsSvc()

		newRisk(t, ctx, false)
		newAlarm(t, ctx)

		got, err := svc.Screen(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询大屏失败: %v", err)
		}
		if got.RiskRealtime == nil || got.AlarmRealtime == nil || got.RiskStats == nil ||
			got.AlarmStats == nil || got.TrafficMonitor == nil {
			t.Error("数组类字段不应为 nil")
		}
		if len(got.RiskRealtime) > 10 || len(got.AlarmRealtime) > 10 {
			t.Errorf("实时列表最多 10 条: %d/%d", len(got.RiskRealtime), len(got.AlarmRealtime))
		}
		if len(got.RiskRealtime) > 0 {
			it := got.RiskRealtime[0]
			if it.Device == nil || it.RiskType == nil {
				t.Errorf("实时风险应预载关联名称: %+v", it)
			}
		}
		if len(got.AlarmRealtime) > 0 {
			it := got.AlarmRealtime[0]
			if it.Device == nil || it.AlarmType == nil {
				t.Errorf("实时告警应预载关联名称: %+v", it)
			}
		}
		if got.DeviceMonitor.Total != got.DeviceMonitor.Online+got.DeviceMonitor.Offline {
			t.Errorf("设备监控不自洽: %+v", got.DeviceMonitor)
		}
		if got.TodayRiskCount < 1 {
			t.Errorf("今日风险数 = %d, 期望 >= 1", got.TodayRiskCount)
		}
		if got.TodayAlarmCount < 1 {
			t.Errorf("今日告警数 = %d, 期望 >= 1", got.TodayAlarmCount)
		}
	})
}

// TestStatsService_TrafficStats 流量统计与物联网卡字段求和一致。
func TestStatsService_TrafficStats(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := statsSvc()

		before, err := svc.TrafficStats(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}

		region, err := svcbasic.NewRegionService().Create(ctx, &request.CreateRegionReq{
			Name: "ZZQ-流量区域", Code: uniq("ZZQ_TREG"),
		})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		cards := svcdevice.NewIotCardService()
		specs := []struct {
			total int64
			used  int64
		}{{1000, 200}, {1000, 300}, {2000, 500}}
		for _, s := range specs {
			if _, err := cards.Create(ctx, &request.CreateIotCardReq{
				Name: "ZZQ-流量卡", CardNo: uniq("8986"), RegionID: &region.ID,
				TrafficTotal: s.total, TrafficUsed: s.used,
			}); err != nil {
				t.Fatalf("创建物联网卡失败: %v", err)
			}
		}

		after, err := svc.TrafficStats(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if after.TotalTraffic != before.TotalTraffic+4000 {
			t.Errorf("总流量 = %d, 期望 %d", after.TotalTraffic, before.TotalTraffic+4000)
		}
		if after.UsedTraffic != before.UsedTraffic+1000 {
			t.Errorf("已用流量 = %d, 期望 %d", after.UsedTraffic, before.UsedTraffic+1000)
		}
		if after.LeftTraffic != before.LeftTraffic+3000 {
			t.Errorf("剩余流量 = %d, 期望 %d", after.LeftTraffic, before.LeftTraffic+3000)
		}
		if len(after.Items) != len(before.Items)+3 {
			t.Errorf("明细条数 = %d, 期望 %d", len(after.Items), len(before.Items)+3)
		}
		// 明细按剩余流量升序
		for i := 1; i < len(after.Items); i++ {
			if after.Items[i].TrafficLeft < after.Items[i-1].TrafficLeft {
				t.Error("明细应按剩余流量升序返回")
			}
		}
		if after.Items == nil {
			t.Error("明细不应为 nil")
		}
	})
}

// TestStatsService_DeviceStatus 设备状态统计自洽（online + offline == total）。
func TestStatsService_DeviceStatus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := statsSvc()

		before, err := svc.DeviceStatus(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		newDevice(t, ctx, uniq("ZZQ-SD1"), true)
		newDevice(t, ctx, uniq("ZZQ-SD2"), true)
		newDevice(t, ctx, uniq("ZZQ-SD3"), false)

		after, err := svc.DeviceStatus(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if after.Total != before.Total+3 {
			t.Errorf("total = %d, 期望 %d", after.Total, before.Total+3)
		}
		if after.Online != before.Online+2 {
			t.Errorf("online = %d, 期望 %d", after.Online, before.Online+2)
		}
		if after.Offline != before.Offline+1 {
			t.Errorf("offline = %d, 期望 %d", after.Offline, before.Offline+1)
		}
		if after.Online+after.Offline != after.Total {
			t.Errorf("online + offline (%d) != total (%d)", after.Online+after.Offline, after.Total)
		}
		if after.LowBattery < 0 || after.FullStorage < 0 {
			t.Errorf("统计值不应为负: %+v", after)
		}
		if after.LowBattery > after.Total {
			t.Errorf("低电量设备数 %d 不应超过总数 %d", after.LowBattery, after.Total)
		}
	})
}

// TestStatsService_Dashboard_DataScope 统计接口受数据权限约束（不得绕过）。
func TestStatsService_Dashboard_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := statsSvc()

		deptA, deptB, deptC := int64(9951), int64(9952), int64(9953)
		if err := tx.Unscoped().Where("dept_id IN ?", []int64{deptA, deptB, deptC}).Delete(&model.Device{}).Error; err != nil {
			t.Fatalf("清理失败: %v", err)
		}
		specs := []struct {
			dept    int64
			creator string
			suffix  string
			online  bool
		}{
			{deptA, "ds_user_a", "A1", true}, {deptA, "ds_user_b", "A2", true},
			{deptB, "ds_user_b", "B1", false}, {deptB, "ds_user_b", "B2", false},
			{deptC, "ds_user_c", "C1", true}, {deptC, "ds_user_c", "C2", true},
		}
		for _, s := range specs {
			status := model.DeviceStatusOffline
			if s.online {
				status = model.DeviceStatusOnline
			}
			m := &model.Device{
				Name: "ZZQ-DS-STAT-" + s.suffix, DeviceCode: uniq("ZZQ_DSSTAT_" + s.suffix),
				Status: status, Active: boolPtr(true),
				DeptID:    int64Ptr(s.dept),
				BaseModel: model.BaseModel{CreatedBy: s.creator, UpdatedBy: s.creator},
			}
			if err := tx.Create(m).Error; err != nil {
				t.Fatalf("预置数据失败: %v", err)
			}
		}

		all, err := svc.DeviceStatus(ctx, dsAll())
		if err != nil {
			t.Fatalf("all 查询失败: %v", err)
		}
		cross, _ := svc.DeviceStatus(ctx, dsPerm("cross_dept", "ds_user_b", &deptA, "9952"))
		dept, _ := svc.DeviceStatus(ctx, dsPerm("dept", "ds_user_b", &deptA, ""))
		personal, _ := svc.DeviceStatus(ctx, dsPerm("personal", "ds_user_a", &deptA, ""))

		if cross.Total != 4 {
			t.Errorf("cross_dept total = %d, 期望 4", cross.Total)
		}
		if dept.Total != 2 {
			t.Errorf("dept total = %d, 期望 2", dept.Total)
		}
		if personal.Total != 1 {
			t.Errorf("personal total = %d, 期望 1", personal.Total)
		}
		if personal.Total > dept.Total || dept.Total > cross.Total || cross.Total > all.Total {
			t.Errorf("可见行数应满足 personal <= dept <= cross_dept <= all: %d/%d/%d/%d",
				personal.Total, dept.Total, cross.Total, all.Total)
		}
		if dept.Online+dept.Offline != dept.Total {
			t.Errorf("dept 权限下统计不自洽: %+v", dept)
		}

		// 流量统计同样受限
		tAll, _ := svc.TrafficStats(ctx, dsAll())
		tDept, _ := svc.TrafficStats(ctx, dsPerm("dept", "ds_user_b", &deptA, ""))
		if tDept.TotalTraffic > tAll.TotalTraffic {
			t.Errorf("dept 权限下的流量总额不应超过 all: %d > %d", tDept.TotalTraffic, tAll.TotalTraffic)
		}
		// 看板与屏幕同样受限
		dAll, _ := svc.Dashboard(ctx, dsAll())
		dDept, _ := svc.Dashboard(ctx, dsPerm("dept", "ds_user_b", &deptA, ""))
		if dDept.DeviceStat.Total > dAll.DeviceStat.Total {
			t.Errorf("dept 权限下的设备总数不应超过 all: %d > %d", dDept.DeviceStat.Total, dAll.DeviceStat.Total)
		}
		sAll, _ := svc.Screen(ctx, dsAll())
		sDept, _ := svc.Screen(ctx, dsPerm("dept", "ds_user_b", &deptA, ""))
		if sDept.DeviceMonitor.Total > sAll.DeviceMonitor.Total {
			t.Errorf("dept 权限下的大屏设备总数不应超过 all")
		}
	})
}

// TestStatsService_EmptyResult 无数据区间：计数为 0，数组非 null。
func TestStatsService_EmptyResult(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := statsSvc()

		start, end := "2020-01-01 00:00:00", "2020-01-03 00:00:00"
		risk, err := svc.RiskStats(ctx, dsAll(), start, end)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if risk.Total != 0 || risk.Handled != 0 || risk.Pending != 0 {
			t.Errorf("空区间应全为 0: %+v", risk)
		}
		if risk.ByType == nil || risk.ByRegion == nil || risk.ByLevel == nil ||
			risk.ByDevice == nil || risk.Trend == nil {
			t.Error("空结果时数组类字段不应为 nil")
		}

		alarm, err := svc.AlarmStats(ctx, dsAll(), start, end)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if alarm.Total != 0 {
			t.Errorf("空区间应全为 0: %+v", alarm)
		}
		if alarm.ByType == nil || alarm.Trend == nil {
			t.Error("空结果时数组类字段不应为 nil")
		}

		// 大屏在无数据时也不得返回 nil 数组
		screen, err := svc.Screen(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if screen.RiskRealtime == nil || screen.AlarmRealtime == nil ||
			screen.RiskStats == nil || screen.AlarmStats == nil || screen.TrafficMonitor == nil {
			t.Error("大屏数组类字段不应为 nil")
		}

		dash, err := svc.Dashboard(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if dash.RiskTypeStats == nil || dash.RegionRiskStats == nil || dash.RiskTrend == nil || dash.AlarmTrend == nil {
			t.Error("看板数组类字段不应为 nil")
		}
	})
}

// ── 内部工具 ──────────────────────────────────────────────────

// sumCounts 汇总名称-数量分组的计数。
func sumCounts(items []response.NameCount) int64 {
	var n int64
	for _, it := range items {
		n += it.Count
	}
	return n
}

// sumTrend 汇总逐日趋势的计数。
func sumTrend(items []response.TrendPoint) int64 {
	var n int64
	for _, it := range items {
		n += it.Count
	}
	return n
}

// svcstatsRiskReport 上报一条风险（仅用于构造统计数据）。
func svcstatsRiskReport(ctx context.Context, deviceCode, riskTypeCode string) (*model.RiskRecord, error) {
	return svcrisk.NewRiskService().Report(ctx, &request.ReportRiskReq{
		DeviceCode: deviceCode, RiskTypeCode: riskTypeCode, DetectScore: 0.9,
	})
}

// svcstatsRiskHandle 确认并处理一条风险，使其计入 handled 口径。
func svcstatsRiskHandle(ctx context.Context, id int64) error {
	svc := svcrisk.NewRiskService()
	if err := svc.UpdateAccurate(ctx, id, model.DetectAccurateYes, testUserName); err != nil {
		return err
	}
	return svc.Handle(ctx, id, &request.HandleRiskReq{HandleResult: "统计处置"}, testUserName)
}

// newAlarm 上报一条告警。
func newAlarm(t *testing.T, ctx context.Context) *model.AlarmRecord {
	t.Helper()
	region, err := svcbasic.NewRegionService().Create(ctx, &request.CreateRegionReq{
		Name: "ZZQ-统计告警区域", Code: uniq("ZZQ_SAREG"),
	})
	if err != nil {
		t.Fatalf("创建区域失败: %v", err)
	}
	dev, err := svcdevice.NewDeviceService().Create(ctx, &request.CreateDeviceReq{
		Name: "ZZQ-统计告警设备", DeviceCode: uniq("ZZQ-SADEV"), RegionID: &region.ID, Location: "东门",
	})
	if err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	at, err := svcbasic.NewAlarmTypeService().Create(ctx, &request.CreateAlarmTypeReq{
		Name: "ZZQ-统计告警类型", Code: uniq("zzq_sat"), Level: model.LevelHigh,
	})
	if err != nil {
		t.Fatalf("创建告警类型失败: %v", err)
	}
	rec, err := svcalarm.NewAlarmService().Report(ctx, &request.ReportAlarmReq{
		DeviceCode: dev.DeviceCode, AlarmTypeCode: at.Code, Content: "统计用告警",
	})
	if err != nil {
		t.Fatalf("上报告警失败: %v", err)
	}
	return rec
}
