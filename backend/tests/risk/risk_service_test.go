package risk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svcrisk "github.com/boticz/ai-camera-backend/internal/service/risk"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// AI 风险（RiskService）：两条状态分支 + 处理前置条件
// ════════════════════════════════════════════════════════════════

// reportRisk 通过 RiskService.Report 上报一条风险并返回实体。
func reportRisk(t *testing.T, ctx context.Context, f *riskFixture, score float64) *model.RiskRecord {
	t.Helper()
	rec, err := svcrisk.NewRiskService().Report(ctx, &request.ReportRiskReq{
		DeviceCode: f.DeviceCode, RiskTypeCode: f.TypeCode,
		ImageURL: "/uploads/2026/09/risk.jpg", VideoURL: "/uploads/2026/09/risk.mp4",
		DetectScore: score,
	})
	if err != nil {
		t.Fatalf("上报风险失败: %v", err)
	}
	return rec
}

// TestRiskService_Report 风险上报落库初始态。
func TestRiskService_Report(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)

		rec := reportRisk(t, ctx, f, 0.93)
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
		if got.DetectAccurate != "" || got.AccurateBy != "" || got.AccurateDT != nil {
			t.Errorf("检测准确字段应待确认: %q/%q/%v", got.DetectAccurate, got.AccurateBy, got.AccurateDT)
		}
		if got.HandleResult != "" || got.HandleImage != "" || got.Handler != "" || got.HandledDT != nil {
			t.Errorf("处理字段应为零值: %+v", got)
		}
		if got.VoicePlayed == nil || *got.VoicePlayed {
			t.Errorf("voice_played = %v, 期望 false", got.VoicePlayed)
		}
		if got.ImageURL != "/uploads/2026/09/risk.jpg" || got.VideoURL != "/uploads/2026/09/risk.mp4" {
			t.Errorf("图片/录像字段错误: %q/%q", got.ImageURL, got.VideoURL)
		}
		if got.DetectScore != 0.93 {
			t.Errorf("detect_score = %v, 期望 0.93", got.DetectScore)
		}
		if got.OccurredDT.IsZero() {
			t.Error("occurred_dt 不应为零值")
		}
		if got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", got.CreatedBy, got.CreatedDT)
		}
	})
}

// TestRiskService_Report_Redundancy 上报时冗余设备编码/区域/位置与风险等级。
func TestRiskService_Report_Redundancy(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		f := newRiskFixture(t, ctx, model.LevelHigh)

		rec := reportRisk(t, ctx, f, 0.8)
		if rec.RiskTypeID != f.RiskType.ID {
			t.Errorf("risk_type_id = %d, 期望 %d", rec.RiskTypeID, f.RiskType.ID)
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
		if rec.RiskLevel != model.LevelHigh {
			t.Errorf("risk_level = %q, 期望 high（冗余自风险类型）", rec.RiskLevel)
		}
	})
}

// TestRiskService_Report_InvalidDevice 设备或风险类型不存在时不产生孤儿记录。
func TestRiskService_Report_InvalidDevice(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelMedium)

		before := countRows(t, tx, "ac_risk_record", "")

		_, err := svc.Report(ctx, &request.ReportRiskReq{
			DeviceCode: uniq("ZZQ-GHOST"), RiskTypeCode: f.TypeCode,
		})
		if err == nil {
			t.Fatal("设备不存在应返回错误")
		}
		if !errors.Is(err, svcrisk.ErrDeviceNotFound) {
			t.Errorf("错误应为 ErrDeviceNotFound, got %v", err)
		}
		if n := countRows(t, tx, "ac_risk_record", ""); n != before {
			t.Errorf("不得新增风险记录: %d -> %d", before, n)
		}

		_, err = svc.Report(ctx, &request.ReportRiskReq{
			DeviceCode: f.DeviceCode, RiskTypeCode: uniq("zzq_no_such_type"),
		})
		if err == nil {
			t.Fatal("风险类型不存在应返回错误")
		}
		if !errors.Is(err, svcrisk.ErrRiskTypeNotFound) {
			t.Errorf("错误应为 ErrRiskTypeNotFound, got %v", err)
		}
		if n := countRows(t, tx, "ac_risk_record", ""); n != before {
			t.Errorf("不得新增风险记录: %d -> %d", before, n)
		}
	})
}

// TestRiskService_UpdateAccurate_No 检测准确=否（误报）：直接转已处理且置已读。
func TestRiskService_UpdateAccurate_No(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)
		rec := reportRisk(t, ctx, f, 0.9)

		operator := "admin"
		if err := svc.UpdateAccurate(ctx, rec.ID, "no", operator); err != nil {
			t.Fatalf("确认误报失败: %v", err)
		}

		got, _ := svc.Get(ctx, rec.ID)
		if got.DetectAccurate != model.DetectAccurateNo {
			t.Errorf("detect_accurate = %q, 期望 no", got.DetectAccurate)
		}
		if got.AccurateBy != operator {
			t.Errorf("accurate_by = %q, 期望 %q", got.AccurateBy, operator)
		}
		if got.AccurateDT == nil {
			t.Error("accurate_dt 应非空")
		}
		if got.Read == nil || !*got.Read {
			t.Errorf("read = %v, 期望 true", got.Read)
		}
		if got.ReadDT == nil {
			t.Error("read_dt 应非空")
		}
		if got.Status != model.HandleStatusHandled {
			t.Errorf("status = %q, 期望 handled（误报直接办结）", got.Status)
		}
		if got.HandleResult != "误报，无需处理" {
			t.Errorf("handle_result = %q, 期望 误报，无需处理", got.HandleResult)
		}
		if got.Handler != "" || got.HandledDT != nil {
			t.Errorf("误报不应有处理人/处理时间: %q/%v", got.Handler, got.HandledDT)
		}
	})
}

// TestRiskService_UpdateAccurate_Yes 检测准确=是：仅置已读，状态仍为 pending。
func TestRiskService_UpdateAccurate_Yes(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)
		rec := reportRisk(t, ctx, f, 0.9)

		if err := svc.UpdateAccurate(ctx, rec.ID, "yes", "admin"); err != nil {
			t.Fatalf("确认准确失败: %v", err)
		}

		got, _ := svc.Get(ctx, rec.ID)
		if got.DetectAccurate != model.DetectAccurateYes {
			t.Errorf("detect_accurate = %q, 期望 yes", got.DetectAccurate)
		}
		if got.AccurateBy != "admin" || got.AccurateDT == nil {
			t.Errorf("确认人/时间未落库: %q/%v", got.AccurateBy, got.AccurateDT)
		}
		if got.Read == nil || !*got.Read || got.ReadDT == nil {
			t.Errorf("read/read_dt 应已置位: %v/%v", got.Read, got.ReadDT)
		}
		if got.Status != model.HandleStatusPending {
			t.Errorf("status = %q, 期望仍为 pending（需人工处理）", got.Status)
		}
		if got.HandleResult != "" || got.Handler != "" {
			t.Errorf("处理字段应保持零值: %q/%q", got.HandleResult, got.Handler)
		}
	})
}

// TestRiskService_UpdateAccurate_InvalidValue 非法取值被拒绝且字段不变。
func TestRiskService_UpdateAccurate_InvalidValue(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)
		rec := reportRisk(t, ctx, f, 0.9)

		for _, bad := range []string{"", "maybe", "YES", " true "} {
			err := svc.UpdateAccurate(ctx, rec.ID, bad, "admin")
			if err == nil {
				t.Errorf("accurate=%q 应返回校验错误", bad)
				continue
			}
			if !errors.Is(err, svcrisk.ErrInvalidAccurate) {
				t.Errorf("accurate=%q 错误类型应为 ErrInvalidAccurate, got %v", bad, err)
			}
		}

		got, _ := svc.Get(ctx, rec.ID)
		if got.DetectAccurate != "" || got.AccurateBy != "" || got.AccurateDT != nil {
			t.Errorf("非法取值后字段不应被修改: %q/%q/%v", got.DetectAccurate, got.AccurateBy, got.AccurateDT)
		}
		if got.Read == nil || *got.Read {
			t.Errorf("read 不应被修改: %v", got.Read)
		}
	})
}

// TestRiskService_Handle 处理风险：写全部处理字段并办结。
func TestRiskService_Handle(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)
		rec := reportRisk(t, ctx, f, 0.9)
		if err := svc.UpdateAccurate(ctx, rec.ID, "yes", "admin"); err != nil {
			t.Fatalf("确认准确失败: %v", err)
		}

		if err := svc.Handle(ctx, rec.ID, &request.HandleRiskReq{
			HandleResult: "已现场清理", HandleImage: "/uploads/2026/09/handle.png",
		}, "admin"); err != nil {
			t.Fatalf("处理失败: %v", err)
		}

		got, _ := svc.Get(ctx, rec.ID)
		if got.HandleResult != "已现场清理" {
			t.Errorf("handle_result = %q", got.HandleResult)
		}
		if got.HandleImage != "/uploads/2026/09/handle.png" {
			t.Errorf("handle_image = %q", got.HandleImage)
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

// TestRiskService_Handle_AlreadyHandled 已处理的风险不可重复处理。
func TestRiskService_Handle_AlreadyHandled(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)
		rec := reportRisk(t, ctx, f, 0.9)
		if err := svc.UpdateAccurate(ctx, rec.ID, "yes", "admin"); err != nil {
			t.Fatalf("确认准确失败: %v", err)
		}
		if err := svc.Handle(ctx, rec.ID, &request.HandleRiskReq{HandleResult: "首次处理"}, "admin"); err != nil {
			t.Fatalf("首次处理失败: %v", err)
		}
		first, _ := svc.Get(ctx, rec.ID)

		err := svc.Handle(ctx, rec.ID, &request.HandleRiskReq{HandleResult: "二次处理"}, "other")
		if err == nil {
			t.Fatal("已处理的风险再次处理应返回错误")
		}
		if !errors.Is(err, svcrisk.ErrAlreadyHandled) {
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

// TestRiskService_Handle_RequiresAccurate 未确认检测准确前禁止处理（前置条件）。
func TestRiskService_Handle_RequiresAccurate(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)
		rec := reportRisk(t, ctx, f, 0.9)

		// 未确认（detect_accurate 为空）→ 拒绝
		err := svc.Handle(ctx, rec.ID, &request.HandleRiskReq{HandleResult: "直接处理"}, "admin")
		if err == nil {
			t.Fatal("未确认检测准确时应返回错误")
		}
		if !errors.Is(err, svcrisk.ErrAccurateRequired) {
			t.Errorf("错误应为 ErrAccurateRequired, got %v", err)
		}
		got, _ := svc.Get(ctx, rec.ID)
		if got.Status != model.HandleStatusPending || got.HandleResult != "" {
			t.Errorf("拒绝时不应写入处理字段: %q/%q", got.Status, got.HandleResult)
		}

		// 确认误报（已办结）后 → 拒绝
		if err := svc.UpdateAccurate(ctx, rec.ID, "no", "admin"); err != nil {
			t.Fatalf("确认误报失败: %v", err)
		}
		if err := svc.Handle(ctx, rec.ID, &request.HandleRiskReq{HandleResult: "再处理"}, "admin"); err == nil {
			t.Error("误报已办结后再次处理应返回错误")
		}
	})
}

// TestRiskService_PendingCount 待处理数与明细列表口径一致。
func TestRiskService_PendingCount(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)

		baseline, err := svc.PendingCount(ctx, dsAll())
		if err != nil {
			t.Fatalf("统计失败: %v", err)
		}

		ids := make([]int64, 0, 5)
		for i := 0; i < 5; i++ {
			ids = append(ids, reportRisk(t, ctx, f, 0.5).ID)
		}
		got, _ := svc.PendingCount(ctx, dsAll())
		if got != baseline+5 {
			t.Fatalf("待处理数 = %d, 期望 %d", got, baseline+5)
		}

		page, err := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Status: model.HandleStatusPending,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if page.Total != got {
			t.Errorf("待处理数 %d != 列表 total %d", got, page.Total)
		}

		// 处理 1 条 → 减 1
		if err := svc.UpdateAccurate(ctx, ids[0], "yes", "admin"); err != nil {
			t.Fatalf("确认失败: %v", err)
		}
		if err := svc.Handle(ctx, ids[0], &request.HandleRiskReq{HandleResult: "已处理"}, "admin"); err != nil {
			t.Fatalf("处理失败: %v", err)
		}
		afterHandle, _ := svc.PendingCount(ctx, dsAll())
		if afterHandle != got-1 {
			t.Errorf("处理 1 条后待处理数 = %d, 期望 %d", afterHandle, got-1)
		}

		// 误报 1 条 → 再减 1
		if err := svc.UpdateAccurate(ctx, ids[1], "no", "admin"); err != nil {
			t.Fatalf("误报失败: %v", err)
		}
		afterFalse, _ := svc.PendingCount(ctx, dsAll())
		if afterFalse != got-2 {
			t.Errorf("误报 1 条后待处理数 = %d, 期望 %d", afterFalse, got-2)
		}
	})
}

// TestRiskService_MarkRead 标记已读：幂等且不改状态。
func TestRiskService_MarkRead(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelMedium)
		rec := reportRisk(t, ctx, f, 0.7)

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

		// 幂等：重复调用不刷新 read_dt
		time.Sleep(10 * time.Millisecond)
		if err := svc.MarkRead(ctx, rec.ID); err != nil {
			t.Fatalf("重复标记失败: %v", err)
		}
		again, _ := svc.Get(ctx, rec.ID)
		if again.ReadDT == nil || !again.ReadDT.Equal(firstReadDT) {
			t.Errorf("重复标记不应刷新 read_dt: %v -> %v", firstReadDT, again.ReadDT)
		}

		if _, err := svc.Get(ctx, 999999999); err == nil {
			t.Error("不存在的风险应返回错误")
		}
	})
}

// TestRiskService_Page_Filters 风险分页过滤条件。
func TestRiskService_Page_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)

		r1 := reportRisk(t, ctx, f, 0.9)
		r2 := reportRisk(t, ctx, f, 0.8)
		if err := svc.UpdateAccurate(ctx, r2.ID, "no", "admin"); err != nil {
			t.Fatalf("误报失败: %v", err)
		}

		byType, err := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, RiskTypeID: f.RiskType.ID,
		}, dsAll())
		if err != nil {
			t.Fatalf("按风险类型分页失败: %v", err)
		}
		if byType.Total != 2 {
			t.Errorf("按风险类型命中 %d, 期望 2", byType.Total)
		}

		byRegion, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, RegionID: f.Region.ID,
		}, dsAll())
		if byRegion.Total != 2 {
			t.Errorf("按区域命中 %d, 期望 2", byRegion.Total)
		}

		// 状态过滤叠加设备编号，避免受库里既有数据影响
		byStatus, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode, Status: model.HandleStatusPending,
		}, dsAll())
		if byStatus.Total != 1 {
			t.Errorf("按状态命中 %d, 期望 1", byStatus.Total)
		}
		byHandled, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode, Status: model.HandleStatusHandled,
		}, dsAll())
		if byHandled.Total != 1 {
			t.Errorf("按已处理状态命中 %d, 期望 1", byHandled.Total)
		}

		byDevice, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode,
		}, dsAll())
		if byDevice.Total != 2 {
			t.Errorf("按设备编号命中 %d, 期望 2", byDevice.Total)
		}

		byDeviceID, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceID: f.Device.ID,
		}, dsAll())
		if byDeviceID.Total != 2 {
			t.Errorf("按设备 ID 命中 %d, 期望 2", byDeviceID.Total)
		}

		byAccurate, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode, DetectAccurate: model.DetectAccurateNo,
		}, dsAll())
		if byAccurate.Total != 1 {
			t.Errorf("按检测准确命中 %d, 期望 1", byAccurate.Total)
		}

		unread := false
		byUnread, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode, Read: &unread,
		}, dsAll())
		if byUnread.Total != 1 {
			t.Errorf("按未读命中 %d, 期望 1", byUnread.Total)
		}

		// 时间范围为闭区间（按日过滤含当天）
		today := time.Now().Format("2006-01-02")
		byRange, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode,
			DateRangeReq: request.DateRangeReq{StartDT: today, EndDT: today},
		}, dsAll())
		if byRange.Total != 2 {
			t.Errorf("按当天时间范围命中 %d, 期望 2", byRange.Total)
		}

		_ = r1
		none, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: uniq("ZZQ-NO-RISK"),
		}, dsAll())
		if none.Total != 0 || len(none.Items) != 0 {
			t.Errorf("无匹配时 total=%d items=%d", none.Total, len(none.Items))
		}
	})
}

// TestRiskService_Page_Preload 列表回显风险类型/设备/区域名称。
func TestRiskService_Page_Preload(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)
		reportRisk(t, ctx, f, 0.9)

		page, err := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, DeviceCode: f.DeviceCode,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if len(page.Items) != 1 {
			t.Fatalf("items = %d, 期望 1", len(page.Items))
		}
		it := page.Items[0]
		if it.RiskType == nil || it.RiskType.Name != f.RiskType.Name {
			t.Errorf("RiskType.Name 未回显: %+v", it.RiskType)
		}
		if it.Device == nil || it.Device.Name != f.Device.Name || it.Device.DeviceCode != f.DeviceCode {
			t.Errorf("Device 名称/编号未回显: %+v", it.Device)
		}
		if it.Region == nil || it.Region.Name != f.Region.Name {
			t.Errorf("Region.Name 未回显: %+v", it.Region)
		}
		if page.Page != 1 || page.PageSize != 200 {
			t.Errorf("分页回显 page=%d page_size=%d", page.Page, page.PageSize)
		}
	})
}

// TestRiskService_Get 风险详情逐字段核对。
func TestRiskService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelLow)
		rec := reportRisk(t, ctx, f, 0.66)

		got, err := svc.Get(ctx, rec.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if got.ID != rec.ID || got.RiskTypeID != f.RiskType.ID || got.DeviceID != f.Device.ID ||
			got.DeviceCode != f.DeviceCode || got.RiskLevel != model.LevelLow ||
			got.DetectScore != 0.66 || got.ImageURL != "/uploads/2026/09/risk.jpg" ||
			got.VideoURL != "/uploads/2026/09/risk.mp4" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.RegionID == nil || *got.RegionID != f.Region.ID || got.Location != "东门" {
			t.Errorf("区域/位置字段错误: %v/%q", got.RegionID, got.Location)
		}
		if got.DetectAccurate != "" || got.Status != model.HandleStatusPending {
			t.Errorf("状态字段错误: %q/%q", got.DetectAccurate, got.Status)
		}
		if _, err := svc.Get(ctx, 999999999); err == nil {
			t.Error("不存在的风险应返回错误")
		}
	})
}

// TestRiskService_Delete 风险软删除后列表与待处理数同步收敛。
func TestRiskService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelMedium)

		rec := reportRisk(t, ctx, f, 0.5)
		pageBefore, _ := svc.Page(ctx, &request.PageRiskReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}, dsAll())
		pendingBefore, _ := svc.PendingCount(ctx, dsAll())

		if err := svc.Delete(ctx, rec.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_risk_record", rec.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		pageAfter, _ := svc.Page(ctx, &request.PageRiskReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}, dsAll())
		if pageAfter.Total != pageBefore.Total-1 {
			t.Errorf("删除后 total = %d, 期望 %d", pageAfter.Total, pageBefore.Total-1)
		}
		pendingAfter, _ := svc.PendingCount(ctx, dsAll())
		if pendingAfter != pendingBefore-1 {
			t.Errorf("删除后待处理数 = %d, 期望 %d", pendingAfter, pendingBefore-1)
		}
		if _, err := svc.Get(ctx, rec.ID); err == nil {
			t.Error("软删除后不应可读")
		}
		if err := svc.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的风险应返回错误")
		}
	})
}

// TestRiskService_Page_DataScope 风险列表与待处理数均受数据权限约束。
func TestRiskService_Page_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)

		deptA, deptB, deptC := int64(9701), int64(9702), int64(9703)
		if err := tx.Unscoped().Where("dept_id IN ?", []int64{deptA, deptB, deptC}).Delete(&model.RiskRecord{}).Error; err != nil {
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
			m := &model.RiskRecord{
				RiskTypeID: f.RiskType.ID, DeviceID: f.Device.ID, DeviceCode: f.DeviceCode + "-" + s.suffix,
				RegionID: &f.Region.ID, RiskLevel: model.LevelHigh,
				OccurredDT: time.Now(), Status: model.HandleStatusPending, Read: boolPtr(false),
				DeptID:    int64Ptr(s.dept),
				BaseModel: model.BaseModel{CreatedBy: s.creator, UpdatedBy: s.creator},
			}
			if err := tx.Create(m).Error; err != nil {
				t.Fatalf("预置数据失败: %v", err)
			}
		}

		q := request.PageRiskReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}
		all, err := svc.Page(ctx, &q, dsAll())
		if err != nil {
			t.Fatalf("all 分页失败: %v", err)
		}
		cross, _ := svc.Page(ctx, &q, dsPerm("cross_dept", "ds_user_b", &deptA, "9702"))
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

		// 待处理数同样受约束
		pAll, _ := svc.PendingCount(ctx, dsAll())
		pDept, _ := svc.PendingCount(ctx, dsPerm("dept", "ds_user_b", &deptA, ""))
		pPersonal, _ := svc.PendingCount(ctx, dsPerm("personal", "ds_user_a", &deptA, ""))
		if pPersonal > pDept || pDept > pAll {
			t.Errorf("待处理数应满足 personal <= dept <= all: %d/%d/%d", pPersonal, pDept, pAll)
		}
		if pDept != 2 {
			t.Errorf("dept 待处理数 = %d, 期望 2", pDept)
		}
		if pPersonal != 1 {
			t.Errorf("personal 待处理数 = %d, 期望 1", pPersonal)
		}
	})
}

// TestIT_RiskFalsePositive 误报闭环：上报 → 误报办结 → 待处理数归零。
func TestIT_RiskFalsePositive(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)

		baseline, _ := svc.PendingCount(ctx, dsAll())
		rec := reportRisk(t, ctx, f, 0.95)

		afterReport, _ := svc.PendingCount(ctx, dsAll())
		if afterReport != baseline+1 {
			t.Fatalf("上报后待处理数 = %d, 期望 %d", afterReport, baseline+1)
		}
		pendingList, _ := svc.Page(ctx, &request.PageRiskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Status: model.HandleStatusPending, DeviceCode: f.DeviceCode,
		}, dsAll())
		if pendingList.Total != 1 {
			t.Errorf("待处理列表 total = %d, 期望 1", pendingList.Total)
		}

		if err := svc.UpdateAccurate(ctx, rec.ID, "no", "admin"); err != nil {
			t.Fatalf("误报失败: %v", err)
		}

		afterFalse, _ := svc.PendingCount(ctx, dsAll())
		if afterFalse != baseline {
			t.Errorf("误报后待处理数 = %d, 期望回到基线 %d", afterFalse, baseline)
		}
		got, _ := svc.Get(ctx, rec.ID)
		if got.Status != model.HandleStatusHandled {
			t.Errorf("status = %q, 期望 handled", got.Status)
		}
		if got.Read == nil || !*got.Read {
			t.Error("read 应为 true")
		}
		if got.Handler != "" {
			t.Errorf("误报不应记录处理人: %q", got.Handler)
		}
	})
}

// TestRiskService_Summary 总览指标与列表同口径（总数/待处理/高风险/未复核/未读）。
func TestRiskService_Summary(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcrisk.NewRiskService()
		f := newRiskFixture(t, ctx, model.LevelHigh)

		before, err := svc.Summary(ctx, dsAll())
		if err != nil {
			t.Fatalf("读取总览失败: %v", err)
		}
		for i := 0; i < 3; i++ {
			reportRisk(t, ctx, f, 0.8)
		}
		after, err := svc.Summary(ctx, dsAll())
		if err != nil {
			t.Fatalf("读取总览失败: %v", err)
		}
		if after.Total != before.Total+3 {
			t.Errorf("total = %d, 期望 %d", after.Total, before.Total+3)
		}
		if after.Pending != before.Pending+3 {
			t.Errorf("pending = %d, 期望 %d", after.Pending, before.Pending+3)
		}
		if after.High != before.High+3 {
			t.Errorf("high = %d, 期望 %d", after.High, before.High+3)
		}
		if after.Unconfirmed != before.Unconfirmed+3 {
			t.Errorf("unconfirmed = %d, 期望 %d", after.Unconfirmed, before.Unconfirmed+3)
		}
		if after.Unread != before.Unread+3 {
			t.Errorf("unread = %d, 期望 %d", after.Unread, before.Unread+3)
		}
		if after.Today < 3 {
			t.Errorf("today = %d, 期望至少 3", after.Today)
		}
	})
}
