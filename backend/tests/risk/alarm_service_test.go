package risk

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svcalarm "github.com/boticz/ai-camera-backend/internal/service/alarm"
	"github.com/boticz/ai-camera-backend/internal/service/basic"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 设备告警（AlarmService）：无「检测准确」维度的直接处理流程
// ════════════════════════════════════════════════════════════════

// reportAlarm 通过 AlarmService.Report 上报一条告警。
func reportAlarm(t *testing.T, ctx context.Context, f *alarmFixture, content, level string) *model.AlarmRecord {
	t.Helper()
	rec, err := svcalarm.NewAlarmService().Report(ctx, &request.ReportAlarmReq{
		DeviceCode: f.DeviceCode, AlarmTypeCode: f.TypeCode,
		Content: content, AlarmLevel: level,
	})
	if err != nil {
		t.Fatalf("上报告警失败: %v", err)
	}
	return rec
}

// TestAlarmService_Report 告警上报落库初始态。
func TestAlarmService_Report(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelHigh)

		rec := reportAlarm(t, ctx, f, "设备离线超过10分钟", model.LevelHigh)
		got, err := svc.Get(ctx, rec.ID)
		if err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if got.Status != model.HandleStatusPending {
			t.Errorf("status = %q, 期望 pending", got.Status)
		}
		if got.Read == nil || *got.Read {
			t.Errorf("read = %v, 期望 false", got.Read)
		}
		if got.ReadDT != nil {
			t.Errorf("read_dt 应为 nil, got %v", got.ReadDT)
		}
		if got.HandleResult != "" || got.HandleImage != "" || got.Handler != "" || got.HandledDT != nil {
			t.Errorf("处理字段应为零值: %+v", got)
		}
		if got.Content != "设备离线超过10分钟" {
			t.Errorf("content = %q", got.Content)
		}
		if got.OccurredDT.IsZero() {
			t.Error("occurred_dt 不应为零值")
		}
		if got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", got.CreatedBy, got.CreatedDT)
		}
	})
}

// TestAlarmService_Report_Redundancy 上报冗余字段回填：设备编码/区域/位置/等级。
func TestAlarmService_Report_Redundancy(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		f := newAlarmFixture(t, ctx, model.LevelHigh)

		rec := reportAlarm(t, ctx, f, "设备离线超过10分钟", "")
		if rec.AlarmTypeID != f.AlarmType.ID {
			t.Errorf("alarm_type_id = %d, 期望 %d", rec.AlarmTypeID, f.AlarmType.ID)
		}
		if rec.DeviceID != f.Device.ID {
			t.Errorf("device_id = %d, 期望 %d", rec.DeviceID, f.Device.ID)
		}
		if rec.DeviceCode != f.DeviceCode {
			t.Errorf("device_code = %q, 期望 %q", rec.DeviceCode, f.DeviceCode)
		}
		if rec.RegionID == nil || *rec.RegionID != f.Region.ID {
			t.Errorf("region_id = %v, 期望 %d", rec.RegionID, f.Region.ID)
		}
		if rec.Location != "东门" {
			t.Errorf("location = %q, 期望 东门", rec.Location)
		}
		if rec.AlarmLevel != model.LevelHigh {
			t.Errorf("alarm_level = %q, 期望 high（缺省回落至告警类型配置）", rec.AlarmLevel)
		}
		if rec.Content != "设备离线超过10分钟" {
			t.Errorf("content = %q", rec.Content)
		}

		// 显式传入的等级优先于类型配置
		override := reportAlarm(t, ctx, f, "紧急", model.LevelLow)
		if override.AlarmLevel != model.LevelLow {
			t.Errorf("显式等级 = %q, 期望 low", override.AlarmLevel)
		}

		// 设备不存在时不产生孤儿记录
		before := countRows(t, tx, "ac_alarm_record", "")
		if _, err := svcalarm.NewAlarmService().Report(ctx, &request.ReportAlarmReq{
			DeviceCode: uniq("ZZQ-GHOST-ALARM"), AlarmTypeCode: f.TypeCode,
		}); err == nil {
			t.Error("设备不存在应返回错误")
		}
		if n := countRows(t, tx, "ac_alarm_record", ""); n != before {
			t.Errorf("不得新增告警记录: %d -> %d", before, n)
		}
	})
}

// TestAlarmService_Report_BuiltinTypes 内置三类告警（offline/battery/traffic）均可上报。
func TestAlarmService_Report_BuiltinTypes(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelMedium)

		for _, code := range []string{
			model.AlarmCodeOffline, model.AlarmCodeBattery, model.AlarmCodeTraffic,
		} {
			var at model.AlarmType
			if err := tx.Where("code = ?", code).First(&at).Error; err != nil {
				t.Fatalf("读取内置告警类型 %s 失败: %v", code, err)
			}
			rec, err := svc.Report(ctx, &request.ReportAlarmReq{
				DeviceCode: f.DeviceCode, AlarmTypeCode: code, Content: "内置类型上报",
			})
			if err != nil {
				t.Fatalf("上报 %s 失败: %v", code, err)
			}
			if rec.AlarmTypeID != at.ID {
				t.Errorf("alarm_type_id = %d, 期望 %d", rec.AlarmTypeID, at.ID)
			}
			if rec.AlarmLevel != at.Level {
				t.Errorf("%s 的 alarm_level = %q, 期望 %q", code, rec.AlarmLevel, at.Level)
			}
		}

		// 非法编码
		if _, err := svc.Report(ctx, &request.ReportAlarmReq{
			DeviceCode: f.DeviceCode, AlarmTypeCode: uniq("no_such_alarm"),
		}); err == nil {
			t.Error("不存在的告警编码应返回错误")
		}
	})
}

// TestAlarmService_Handle 处理告警：写全部处理字段并办结（无前置确认）。
func TestAlarmService_Handle(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelHigh)
		rec := reportAlarm(t, ctx, f, "设备离线", "")

		// 未标记已读时处理，应同时置已读
		if err := svc.Handle(ctx, rec.ID, &request.HandleAlarmReq{
			HandleResult: "已重启设备", HandleImage: "/uploads/2026/09/alarm.png",
		}, "admin"); err != nil {
			t.Fatalf("处理失败: %v", err)
		}

		got, _ := svc.Get(ctx, rec.ID)
		if got.HandleResult != "已重启设备" || got.HandleImage != "/uploads/2026/09/alarm.png" {
			t.Errorf("处理字段错误: %q/%q", got.HandleResult, got.HandleImage)
		}
		if got.Handler != "admin" {
			t.Errorf("handler = %q, 期望 admin", got.Handler)
		}
		if got.HandledDT == nil {
			t.Error("handled_dt 应非空")
		}
		if got.Status != model.HandleStatusHandled {
			t.Errorf("status = %q, 期望 handled", got.Status)
		}
		if got.Read == nil || !*got.Read || got.ReadDT == nil {
			t.Errorf("处理动作应隐含已读: %v/%v", got.Read, got.ReadDT)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", got.UpdatedBy)
		}
	})
}

// TestAlarmService_Handle_AlreadyHandled 已处理的告警不可重复处理。
func TestAlarmService_Handle_AlreadyHandled(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelHigh)
		rec := reportAlarm(t, ctx, f, "设备离线", "")
		if err := svc.Handle(ctx, rec.ID, &request.HandleAlarmReq{HandleResult: "首次处理"}, "admin"); err != nil {
			t.Fatalf("首次处理失败: %v", err)
		}
		first, _ := svc.Get(ctx, rec.ID)

		err := svc.Handle(ctx, rec.ID, &request.HandleAlarmReq{HandleResult: "二次处理"}, "other")
		if err == nil {
			t.Fatal("已处理的告警再次处理应返回错误")
		}
		if !errors.Is(err, svcalarm.ErrAlreadyHandled) {
			t.Errorf("错误应为 ErrAlreadyHandled, got %v", err)
		}
		got, _ := svc.Get(ctx, rec.ID)
		if got.Status != model.HandleStatusHandled {
			t.Errorf("status 不得回退: %q", got.Status)
		}
		if got.HandleResult != first.HandleResult || got.Handler != first.Handler {
			t.Errorf("处理字段不应被覆盖: %q/%q", got.HandleResult, got.Handler)
		}
	})
}

// TestAlarmService_MarkRead 标记已读：幂等且不改状态。
func TestAlarmService_MarkRead(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelMedium)
		rec := reportAlarm(t, ctx, f, "电量低", "")

		before, _ := svc.PendingCount(ctx, dsAll())
		if err := svc.MarkRead(ctx, rec.ID); err != nil {
			t.Fatalf("标记已读失败: %v", err)
		}
		got, _ := svc.Get(ctx, rec.ID)
		if got.Read == nil || !*got.Read || got.ReadDT == nil {
			t.Fatalf("read/read_dt 未置位: %v/%v", got.Read, got.ReadDT)
		}
		if got.Status != model.HandleStatusPending {
			t.Errorf("status 不应变化: %q", got.Status)
		}
		after, _ := svc.PendingCount(ctx, dsAll())
		if after != before {
			t.Errorf("待处理数不应变化: %d -> %d", before, after)
		}
		firstReadDT := *got.ReadDT

		time.Sleep(10 * time.Millisecond)
		if err := svc.MarkRead(ctx, rec.ID); err != nil {
			t.Fatalf("重复标记失败: %v", err)
		}
		again, _ := svc.Get(ctx, rec.ID)
		if again.ReadDT == nil || !again.ReadDT.Equal(firstReadDT) {
			t.Errorf("重复标记不应刷新 read_dt: %v -> %v", firstReadDT, again.ReadDT)
		}
	})
}

// TestAlarmService_PendingCount 待处理数与列表口径一致，且受数据权限约束。
func TestAlarmService_PendingCount(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelHigh)

		baseline, err := svc.PendingCount(ctx, dsAll())
		if err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		ids := make([]int64, 0, 5)
		for i := 0; i < 4; i++ {
			ids = append(ids, reportAlarm(t, ctx, f, "离线", "").ID)
		}
		handled := reportAlarm(t, ctx, f, "离线", "")
		if err := svc.Handle(ctx, handled.ID, &request.HandleAlarmReq{HandleResult: "已处理"}, "admin"); err != nil {
			t.Fatalf("处理失败: %v", err)
		}

		got, _ := svc.PendingCount(ctx, dsAll())
		if got != baseline+4 {
			t.Errorf("待处理数 = %d, 期望 %d（已处理的 1 条不计入）", got, baseline+4)
		}
		page, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Status: model.HandleStatusPending,
		}, dsAll())
		if page.Total != got {
			t.Errorf("待处理数 %d != 列表 total %d", got, page.Total)
		}
		_ = ids
	})
}

// TestAlarmService_Page_Filters 告警分页过滤条件。
func TestAlarmService_Page_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelHigh)

		r1 := reportAlarm(t, ctx, f, "离线", "")
		if err := svc.Handle(ctx, r1.ID, &request.HandleAlarmReq{HandleResult: "已处理"}, "admin"); err != nil {
			t.Fatalf("处理失败: %v", err)
		}
		if got, _ := svc.Get(ctx, r1.ID); got.Status != model.HandleStatusHandled {
			t.Fatalf("前置条件：首条告警应已处理, got %q", got.Status)
		}
		r2 := reportAlarm(t, ctx, f, "电量低", "")
		if got, _ := svc.Get(ctx, r2.ID); got.Status != model.HandleStatusPending {
			t.Fatalf("前置条件：第二条告警应待处理, got %q", got.Status)
		}

		byType, err := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, AlarmTypeID: f.AlarmType.ID,
		}, dsAll())
		if err != nil {
			t.Fatalf("按告警类型分页失败: %v", err)
		}
		if byType.Total != 2 {
			t.Errorf("按告警类型命中 %d, 期望 2", byType.Total)
		}

		byRegion, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, RegionID: f.Region.ID,
		}, dsAll())
		if byRegion.Total != 2 {
			t.Errorf("按区域命中 %d, 期望 2", byRegion.Total)
		}

		// 状态过滤叠加设备编号，避免受库里既有数据影响
		byStatus, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode, Status: model.HandleStatusPending,
		}, dsAll())
		if byStatus.Total != 1 {
			t.Errorf("按状态命中 %d, 期望 1", byStatus.Total)
		}
		byHandled, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode, Status: model.HandleStatusHandled,
		}, dsAll())
		if byHandled.Total != 1 {
			t.Errorf("按已处理状态命中 %d, 期望 1", byHandled.Total)
		}

		byDevice, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode,
		}, dsAll())
		if byDevice.Total != 2 {
			t.Errorf("按设备编号命中 %d, 期望 2", byDevice.Total)
		}

		byKeyword, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: f.Device.Name,
		}, dsAll())
		if byKeyword.Total != 2 {
			t.Errorf("按设备名称命中 %d, 期望 2", byKeyword.Total)
		}

		unread := false
		byUnread, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode, Read: &unread,
		}, dsAll())
		if byUnread.Total != 1 {
			t.Errorf("按未读命中 %d, 期望 1", byUnread.Total)
		}

		today := time.Now().Format("2006-01-02")
		byRange, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode,
			DateRangeReq: request.DateRangeReq{StartDT: today, EndDT: today},
		}, dsAll())
		if byRange.Total != 2 {
			t.Errorf("按当天时间范围命中 %d, 期望 2", byRange.Total)
		}

		none, _ := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: uniq("ZZQ-NO-ALARM"),
		}, dsAll())
		if none.Total != 0 || len(none.Items) != 0 {
			t.Errorf("无匹配时 total=%d items=%d", none.Total, len(none.Items))
		}
	})
}

// TestAlarmService_Page_Preload 列表回显告警类型/设备/区域名称。
func TestAlarmService_Page_Preload(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelHigh)
		reportAlarm(t, ctx, f, "离线", "")

		page, err := svc.Page(ctx, &request.PageAlarmReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("items = %d, 期望 1", len(page.Items))
		}
		it := page.Items[0]
		if it.AlarmType == nil || it.AlarmType.Name != f.AlarmType.Name {
			t.Errorf("AlarmType.Name 未回显: %+v", it.AlarmType)
		}
		if it.Device == nil || it.Device.Name != f.Device.Name || it.Device.DeviceCode != f.DeviceCode {
			t.Errorf("Device 名称/编号未回显: %+v", it.Device)
		}
		if it.Region == nil || it.Region.Name != f.Region.Name {
			t.Errorf("Region.Name 未回显: %+v", it.Region)
		}
	})
}

// TestAlarmService_Get 告警详情逐字段核对。
func TestAlarmService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelLow)
		rec := reportAlarm(t, ctx, f, "流量超限", "")

		got, err := svc.Get(ctx, rec.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if got.ID != rec.ID || got.AlarmTypeID != f.AlarmType.ID || got.DeviceID != f.Device.ID ||
			got.DeviceCode != f.DeviceCode || got.Content != "流量超限" || got.AlarmLevel != model.LevelLow {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.RegionID == nil || *got.RegionID != f.Region.ID || got.Location != "东门" {
			t.Errorf("区域/位置字段错误: %v/%q", got.RegionID, got.Location)
		}
		if got.Status != model.HandleStatusPending || got.Read == nil || *got.Read {
			t.Errorf("状态字段错误: %q/%v", got.Status, got.Read)
		}
		if _, err := svc.Get(ctx, 999999999); err == nil {
			t.Error("不存在的告警应返回错误")
		}
	})
}

// TestAlarmService_Delete 告警软删除后列表与待处理数同步收敛。
func TestAlarmService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelMedium)
		rec := reportAlarm(t, ctx, f, "离线", "")

		pageBefore, _ := svc.Page(ctx, &request.PageAlarmReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}, dsAll())
		pendingBefore, _ := svc.PendingCount(ctx, dsAll())

		if err := svc.Delete(ctx, rec.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_alarm_record", rec.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		pageAfter, _ := svc.Page(ctx, &request.PageAlarmReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}, dsAll())
		if pageAfter.Total != pageBefore.Total-1 {
			t.Errorf("删除后 total = %d, 期望 %d", pageAfter.Total, pageBefore.Total-1)
		}
		pendingAfter, _ := svc.PendingCount(ctx, dsAll())
		if pendingAfter != pendingBefore-1 {
			t.Errorf("删除后待处理数 = %d, 期望 %d", pendingAfter, pendingBefore-1)
		}
		if err := svc.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的告警应返回错误")
		}
	})
}

// TestAlarmService_NoAccurateEndpoint 告警无「检测准确」维度：服务与模型层面均不提供该能力。
func TestAlarmService_NoAccurateEndpoint(t *testing.T) {
	// 1) Service 不提供 UpdateAccurate 方法（对应路由 PUT /web/alarms/:id/accurate 不存在）
	typ := reflect.TypeOf(svcalarm.NewAlarmService())
	for i := 0; i < typ.NumMethod(); i++ {
		if typ.Method(i).Name == "UpdateAccurate" {
			t.Fatal("AlarmService 不应提供 UpdateAccurate（告警来自设备状态，非 AI 识别）")
		}
	}

	// 2) 模型不含 DetectAccurate / AccurateBy / AccurateDT 字段
	recType := reflect.TypeOf(model.AlarmRecord{})
	for _, name := range []string{"DetectAccurate", "AccurateBy", "AccurateDT"} {
		if _, ok := recType.FieldByName(name); ok {
			t.Errorf("AlarmRecord 不应包含 %s 字段", name)
		}
	}

	// 3) 对照：风险记录具备该维度（说明该能力确实只在风险侧）
	if _, ok := reflect.TypeOf(model.RiskRecord{}).FieldByName("DetectAccurate"); !ok {
		t.Error("RiskRecord 应包含 DetectAccurate 字段")
	}

	// 4) 告警处理不需要前置确认，可直接办结
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		f := newAlarmFixture(t, ctx, model.LevelLow)
		rec := reportAlarm(t, ctx, f, "直接处理", "")
		if err := svcalarm.NewAlarmService().Handle(ctx, rec.ID, &request.HandleAlarmReq{HandleResult: "直接办结"}, "admin"); err != nil {
			t.Errorf("告警应可在无确认环节下直接处理: %v", err)
		}
	})
}

// 保留 basic 包的引用说明：告警类型由基础信息模块维护，测试中通过内置编码使用。
var _ = basic.NewAlarmTypeService

// TestAlarmService_Page_DataScope 告警列表与待处理数均受数据权限约束。
func TestAlarmService_Page_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := svcalarm.NewAlarmService()
		f := newAlarmFixture(t, ctx, model.LevelHigh)

		deptA, deptB, deptC := int64(9801), int64(9802), int64(9803)
		if err := tx.Unscoped().Where("dept_id IN ?", []int64{deptA, deptB, deptC}).Delete(&model.AlarmRecord{}).Error; err != nil {
			t.Fatalf("清理失败: %v", err)
		}
		specs := []struct {
			dept    int64
			creator string
			suffix  string
		}{
			{deptA, "ds_user_a", "A1"}, {deptA, "ds_user_b", "A2"},
			{deptB, "ds_user_b", "B1"}, {deptB, "ds_user_b", "B2"},
			{deptC, "ds_user_c", "C1"}, {deptC, "ds_user_c", "C2"},
		}
		for _, s := range specs {
			m := &model.AlarmRecord{
				AlarmTypeID: f.AlarmType.ID, DeviceID: f.Device.ID, DeviceCode: f.DeviceCode + "-" + s.suffix,
				RegionID: &f.Region.ID, AlarmLevel: model.LevelHigh, Content: "c",
				OccurredDT: time.Now(), Status: model.HandleStatusPending, Read: boolPtr(false),
				DeptID:    int64Ptr(s.dept),
				BaseModel: model.BaseModel{CreatedBy: s.creator, UpdatedBy: s.creator},
			}
			if err := tx.Create(m).Error; err != nil {
				t.Fatalf("预置数据失败: %v", err)
			}
		}

		q := request.PageAlarmReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}
		all, err := svc.Page(ctx, &q, dsAll())
		if err != nil {
			t.Fatalf("all 分页失败: %v", err)
		}
		cross, _ := svc.Page(ctx, &q, dsPerm("cross_dept", "ds_user_b", &deptA, "9802"))
		dept, _ := svc.Page(ctx, &q, dsPerm("dept", "ds_user_b", &deptA, ""))
		personal, _ := svc.Page(ctx, &q, dsPerm("personal", "ds_user_a", &deptA, ""))

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

		pAll, _ := svc.PendingCount(ctx, dsAll())
		pDept, _ := svc.PendingCount(ctx, dsPerm("dept", "ds_user_b", &deptA, ""))
		pPersonal, _ := svc.PendingCount(ctx, dsPerm("personal", "ds_user_a", &deptA, ""))
		if pPersonal > pDept || pDept > pAll {
			t.Errorf("待处理数应满足 personal <= dept <= all: %d/%d/%d", pPersonal, pDept, pAll)
		}
		if pDept != 2 || pPersonal != 1 {
			t.Errorf("dept/personal 待处理数 = %d/%d, 期望 2/1", pDept, pPersonal)
		}
	})
}
