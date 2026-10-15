package app

import (
	"context"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svcalarm "github.com/boticz/ai-camera-backend/internal/service/alarm"
	svcapp "github.com/boticz/ai-camera-backend/internal/service/app"
	svcbasic "github.com/boticz/ai-camera-backend/internal/service/basic"
	svcdevice "github.com/boticz/ai-camera-backend/internal/service/device"
	svcinspect "github.com/boticz/ai-camera-backend/internal/service/inspect"
	svcrisk "github.com/boticz/ai-camera-backend/internal/service/risk"
	svcsystem "github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 移动端聚合服务（AppService）：仅编排，不重复实现领域逻辑
// ════════════════════════════════════════════════════════════════

func appSvc() *svcapp.AppService { return svcapp.NewAppService() }

// appFixture 移动端用例共用的基础数据。
type appFixture struct {
	User       *model.User
	Device     *model.Device
	RiskType   *model.RiskType
	AlarmType  *model.AlarmType
	RiskTypeID int64
}

// newAppFixture 创建移动端用例所需的用户/设备/类型。
func newAppFixture(t *testing.T, ctx context.Context, username string) *appFixture {
	t.Helper()
	u, err := svcsystem.NewUserService().Create(ctx, &request.CreateUserReq{
		Name: username, Username: username, Password: "P@ss123456",
		Active: boolPtr(true), DeptID: int64Ptr(testDeptID),
	})
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	region, err := svcbasic.NewRegionService().Create(ctx, &request.CreateRegionReq{
		Name: "ZZQ-移动区域", Code: uniq("ZZQ_AREG"),
	})
	if err != nil {
		t.Fatalf("创建区域失败: %v", err)
	}
	dev, err := svcdevice.NewDeviceService().Create(ctx, &request.CreateDeviceReq{
		Name: "ZZQ-移动设备", DeviceCode: uniq("ZZQ-ADEV"), RegionID: &region.ID, Location: "东门",
	})
	if err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	rt, err := svcbasic.NewRiskTypeService().Create(ctx, &request.CreateRiskTypeReq{
		Name: "ZZQ-移动风险类型", Code: uniq("zzq_art"), Level: model.LevelHigh,
	})
	if err != nil {
		t.Fatalf("创建风险类型失败: %v", err)
	}
	at, err := svcbasic.NewAlarmTypeService().Create(ctx, &request.CreateAlarmTypeReq{
		Name: "ZZQ-移动告警类型", Code: uniq("zzq_aat"), Level: model.LevelHigh,
	})
	if err != nil {
		t.Fatalf("创建告警类型失败: %v", err)
	}
	return &appFixture{User: u, Device: dev, RiskType: rt, AlarmType: at, RiskTypeID: rt.ID}
}

// reportRisk 上报一条风险。
func (f *appFixture) reportRisk(t *testing.T, ctx context.Context) *model.RiskRecord {
	t.Helper()
	rec, err := svcrisk.NewRiskService().Report(ctx, &request.ReportRiskReq{
		DeviceCode: f.Device.DeviceCode, RiskTypeCode: f.RiskType.Code, DetectScore: 0.9,
		ImageURL: "/uploads/2026/09/risk.jpg", VideoURL: "/uploads/2026/09/risk.mp4",
	})
	if err != nil {
		t.Fatalf("上报风险失败: %v", err)
	}
	return rec
}

// reportAlarm 上报一条告警。
func (f *appFixture) reportAlarm(t *testing.T, ctx context.Context) *model.AlarmRecord {
	t.Helper()
	rec, err := svcalarm.NewAlarmService().Report(ctx, &request.ReportAlarmReq{
		DeviceCode: f.Device.DeviceCode, AlarmTypeCode: f.AlarmType.Code, Content: "移动端告警",
	})
	if err != nil {
		t.Fatalf("上报告警失败: %v", err)
	}
	return rec
}

// TestAppService_Menus 移动端菜单只返回 platform=mobile 的已授权菜单。
func TestAppService_Menus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()

		// 超级管理员：返回全部 mobile 菜单
		menus, err := svc.Menus(ctx, testUserName)
		if err != nil {
			t.Fatalf("查询菜单失败: %v", err)
		}
		if len(menus) == 0 {
			t.Fatal("超级管理员应能看到移动端菜单")
		}
		count := 0
		walk(menus, func(m model.Menu) {
			count++
			if m.Platform != "mobile" {
				t.Errorf("菜单 %d(%s) 平台为 %q, 期望 mobile", m.ID, m.Name, m.Platform)
			}
			if m.Active != nil && !*m.Active {
				t.Errorf("菜单 %d 已停用却出现在结果中", m.ID)
			}
			if m.Hidden != nil && *m.Hidden {
				t.Errorf("菜单 %d 已隐藏却出现在结果中", m.ID)
			}
		})
		if count != len(menus) && count < len(menus) {
			t.Errorf("树节点数 %d 少于根节点数 %d", count, len(menus))
		}

		// 普通用户（无角色）返回空数组
		f := newAppFixture(t, ctx, uniq("zzq_app_no_role"))
		empty, err := svc.Menus(ctx, f.User.Username)
		if err != nil {
			t.Fatalf("查询菜单失败: %v", err)
		}
		if empty == nil || len(empty) != 0 {
			t.Errorf("无角色用户应返回空数组, got %v", empty)
		}

		// 授予移动端菜单后可见
		var mobileMenu model.Menu
		if err := tx.Where("platform = ? AND parent_id IS NULL", "mobile").Order("id ASC").First(&mobileMenu).Error; err != nil {
			t.Fatalf("读取移动端菜单失败: %v", err)
		}
		role, err := svcsystem.NewRoleService().Create(ctx, &request.CreateRoleReq{
			Name: "ZZQ-移动角色", Code: uniq("zzq_app_role"), DataPermission: "all",
		})
		if err != nil {
			t.Fatalf("创建角色失败: %v", err)
		}
		if err := svcsystem.NewRoleService().AssignMenus(ctx, role.ID, []int64{mobileMenu.ID}); err != nil {
			t.Fatalf("授权菜单失败: %v", err)
		}
		if err := tx.Create(&model.UserRoleRel{UserID: f.User.ID, RoleID: role.ID}).Error; err != nil {
			t.Fatalf("关联角色失败: %v", err)
		}

		got, err := svc.Menus(ctx, f.User.Username)
		if err != nil {
			t.Fatalf("查询菜单失败: %v", err)
		}
		if len(got) != 1 || got[0].ID != mobileMenu.ID {
			t.Errorf("应只返回已授权的移动端菜单: %+v", got)
		}
	})
}

// TestAppService_Home 移动端首页计数与领域接口口径一致。
func TestAppService_Home(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_home"))

		// 待巡检任务：指派给该用户
		if _, err := svcinspect.NewInspectService().CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: uniq("ZZQ-移动巡检"), InspectorID: f.User.ID,
			PlanDT: time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
		}); err != nil {
			t.Fatalf("创建巡检任务失败: %v", err)
		}
		f.reportRisk(t, ctx)
		f.reportAlarm(t, ctx)
		if err := svcsystem.NewNotificationService().Push(ctx, f.User.ID, "risk", "标题", "内容", "risk_record", 1); err != nil {
			t.Fatalf("推送通知失败: %v", err)
		}

		got, err := svc.Home(ctx, f.User.ID, dsAll())
		if err != nil {
			t.Fatalf("查询首页失败: %v", err)
		}

		riskPending, _ := svcrisk.NewRiskService().PendingCount(ctx, dsAll())
		if got.PendingRiskCount != riskPending {
			t.Errorf("待处理风险 %d != 领域接口 %d", got.PendingRiskCount, riskPending)
		}
		alarmPending, _ := svcalarm.NewAlarmService().PendingCount(ctx, dsAll())
		if got.PendingAlarmCount != alarmPending {
			t.Errorf("待处理告警 %d != 领域接口 %d", got.PendingAlarmCount, alarmPending)
		}
		if got.PendingInspectCount != 1 {
			t.Errorf("我的待巡检任务 = %d, 期望 1", got.PendingInspectCount)
		}
		unread, _ := svcsystem.NewNotificationService().UnreadCount(ctx, f.User.ID)
		if got.UnreadCount != unread {
			t.Errorf("未读通知 %d != 领域接口 %d", got.UnreadCount, unread)
		}
		if got.TodayRiskCount < 1 || got.TodayAlarmCount < 1 {
			t.Errorf("今日新增应为正数: %d/%d", got.TodayRiskCount, got.TodayAlarmCount)
		}
		if got.DeviceStat.Total != got.DeviceStat.Online+got.DeviceStat.Offline {
			t.Errorf("设备统计不自洽: %+v", got.DeviceStat)
		}
		if got.RecentRisks == nil {
			t.Error("RecentRisks 不应为 nil")
		}
		if len(got.RecentRisks) > 5 {
			t.Errorf("最近风险最多 5 条, got %d", len(got.RecentRisks))
		}
		if len(got.RecentRisks) > 0 && got.RecentRisks[0].Device == nil {
			t.Error("最近风险应预载设备名称")
		}
	})
}

// TestAppService_Todos 移动端待办固定四项且计数与明细一致。
func TestAppService_Todos(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_todo"))

		f.reportRisk(t, ctx)
		f.reportRisk(t, ctx)
		f.reportAlarm(t, ctx)
		if _, err := svcinspect.NewInspectService().CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: uniq("ZZQ-待办巡检"), InspectorID: f.User.ID,
			PlanDT: time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
		}); err != nil {
			t.Fatalf("创建巡检任务失败: %v", err)
		}
		if err := svcsystem.NewNotificationService().Push(ctx, f.User.ID, "alarm", "标题", "内容", "", 0); err != nil {
			t.Fatalf("推送通知失败: %v", err)
		}

		items, err := svc.Todos(ctx, f.User.ID, dsAll())
		if err != nil {
			t.Fatalf("查询待办失败: %v", err)
		}
		if len(items) != 4 {
			t.Fatalf("待办项数 = %d, 期望 4", len(items))
		}
		byCategory := map[string]int64{}
		for _, it := range items {
			if it.Title == "" {
				t.Errorf("待办标题不应为空: %+v", it)
			}
			byCategory[it.Category] = it.Count
		}
		for _, c := range []string{"risk", "alarm", "inspect", "notification"} {
			if _, ok := byCategory[c]; !ok {
				t.Errorf("缺少待办类别 %s", c)
			}
		}
		riskPending, _ := svcrisk.NewRiskService().PendingCount(ctx, dsAll())
		if byCategory["risk"] != riskPending {
			t.Errorf("风险待办 %d != PendingCount %d", byCategory["risk"], riskPending)
		}
		alarmPending, _ := svcalarm.NewAlarmService().PendingCount(ctx, dsAll())
		if byCategory["alarm"] != alarmPending {
			t.Errorf("告警待办 %d != PendingCount %d", byCategory["alarm"], alarmPending)
		}
		if byCategory["inspect"] != 1 {
			t.Errorf("巡检待办 = %d, 期望 1", byCategory["inspect"])
		}
		if byCategory["notification"] != 1 {
			t.Errorf("未读通知待办 = %d, 期望 1", byCategory["notification"])
		}
	})
}

// TestAppService_PageRisks 移动端风险列表与 Web 端口径一致。
func TestAppService_PageRisks(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_risks"))
		f.reportRisk(t, ctx)

		req := &request.PageRiskReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceCode: f.Device.DeviceCode}
		appPage, err := svc.PageRisks(ctx, req, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		webPage, err := svcrisk.NewRiskService().Page(ctx, req, dsAll())
		if err != nil {
			t.Fatalf("Web 查询失败: %v", err)
		}
		if appPage.Total != webPage.Total || appPage.Total != 1 {
			t.Errorf("两端口径不一致: %d vs %d", appPage.Total, webPage.Total)
		}
		if len(appPage.Items) != 1 {
			t.Fatalf("items = %d, 期望 1", len(appPage.Items))
		}
		if appPage.Items[0].RiskType == nil || appPage.Items[0].Device == nil {
			t.Error("列表应预载关联名称")
		}
	})
}

// TestAppService_GetRisk 移动端风险详情字段完整。
func TestAppService_GetRisk(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_getrisk"))
		rec := f.reportRisk(t, ctx)

		got, err := svc.GetRisk(ctx, rec.ID)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if got.ID != rec.ID || got.ImageURL == "" || got.VideoURL == "" {
			t.Errorf("详情字段不完整: %+v", got)
		}
		if got.DetectAccurate != "" || got.Status != model.HandleStatusPending {
			t.Errorf("初始状态错误: %q/%q", got.DetectAccurate, got.Status)
		}
	})
}

// TestAppService_HandleRisk 移动端处理风险与领域实现一致。
func TestAppService_HandleRisk(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_handle"))
		rec := f.reportRisk(t, ctx)
		if err := svc.UpdateRiskAccurate(ctx, rec.ID, model.DetectAccurateYes, "appuser"); err != nil {
			t.Fatalf("确认准确失败: %v", err)
		}

		if err := svc.HandleRisk(ctx, rec.ID, &request.HandleRiskReq{
			HandleResult: "已处理", HandleImage: "/uploads/a.png",
		}, "appuser"); err != nil {
			t.Fatalf("处理失败: %v", err)
		}

		got, _ := svc.GetRisk(ctx, rec.ID)
		if got.HandleResult != "已处理" || got.HandleImage != "/uploads/a.png" || got.Handler != "appuser" {
			t.Errorf("处理字段错误: %+v", got)
		}
		if got.Status != model.HandleStatusHandled || got.HandledDT == nil {
			t.Errorf("状态字段错误: %q/%v", got.Status, got.HandledDT)
		}
	})
}

// TestAppService_UpdateRiskAccurate 移动端确认检测准确与领域实现一致。
func TestAppService_UpdateRiskAccurate(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_accurate"))

		falsePositive := f.reportRisk(t, ctx)
		confirmed := f.reportRisk(t, ctx)

		if err := svc.UpdateRiskAccurate(ctx, falsePositive.ID, model.DetectAccurateNo, "appuser"); err != nil {
			t.Fatalf("误报确认失败: %v", err)
		}
		fp, _ := svc.GetRisk(ctx, falsePositive.ID)
		if fp.DetectAccurate != model.DetectAccurateNo || fp.Status != model.HandleStatusHandled {
			t.Errorf("误报分支错误: %q/%q", fp.DetectAccurate, fp.Status)
		}
		if fp.Read == nil || !*fp.Read {
			t.Error("误报分支应置已读")
		}
		if fp.AccurateBy != "appuser" || fp.AccurateDT == nil {
			t.Errorf("确认人/时间未落库: %q/%v", fp.AccurateBy, fp.AccurateDT)
		}

		if err := svc.UpdateRiskAccurate(ctx, confirmed.ID, model.DetectAccurateYes, "appuser"); err != nil {
			t.Fatalf("确认准确失败: %v", err)
		}
		cf, _ := svc.GetRisk(ctx, confirmed.ID)
		if cf.Status != model.HandleStatusPending {
			t.Errorf("确认风险后状态应仍为 pending, got %q", cf.Status)
		}
	})
}

// TestAppService_PageAlarms 移动端告警列表与 Web 端口径一致。
func TestAppService_PageAlarms(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_alarms"))
		f.reportAlarm(t, ctx)

		req := &request.PageAlarmReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceCode: f.Device.DeviceCode}
		appPage, err := svc.PageAlarms(ctx, req, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		webPage, _ := svcalarm.NewAlarmService().Page(ctx, req, dsAll())
		if appPage.Total != webPage.Total || appPage.Total != 1 {
			t.Errorf("两端口径不一致: %d vs %d", appPage.Total, webPage.Total)
		}
		if len(appPage.Items) != 1 || appPage.Items[0].AlarmType == nil {
			t.Error("列表应预载关联名称")
		}
	})
}

// TestAppService_HandleAlarm 移动端处理告警写全部处理字段。
func TestAppService_HandleAlarm(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_handlealarm"))
		rec := f.reportAlarm(t, ctx)

		if err := svc.HandleAlarm(ctx, rec.ID, &request.HandleAlarmReq{
			HandleResult: "已重启", HandleImage: "/uploads/a.png",
		}, "appuser"); err != nil {
			t.Fatalf("处理失败: %v", err)
		}

		got, err := svcalarm.NewAlarmService().Get(ctx, rec.ID)
		if err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if got.HandleResult != "已重启" || got.HandleImage != "/uploads/a.png" || got.Handler != "appuser" {
			t.Errorf("处理字段错误: %+v", got)
		}
		if got.Status != model.HandleStatusHandled || got.HandledDT == nil {
			t.Errorf("状态字段错误: %q/%v", got.Status, got.HandledDT)
		}
	})
}

// TestAppService_PageInspectTasks 移动端只返回指派给本人的巡检任务。
func TestAppService_PageInspectTasks(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		inspector := newAppFixture(t, ctx, uniq("zzq_app_insp_a"))
		other := newAppFixture(t, ctx, uniq("zzq_app_insp_b"))
		inspectSvc := svcinspect.NewInspectService()

		keyword := uniq("ZZQ-APP-TASK")
		for i := 0; i < 2; i++ {
			if _, err := inspectSvc.CreateTask(ctx, &request.CreateInspectTaskReq{
				Name: keyword, InspectorID: inspector.User.ID, Location: "1号配电房", Content: "内容",
				PlanDT: time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
			}); err != nil {
				t.Fatalf("创建任务失败: %v", err)
			}
		}
		for i := 0; i < 3; i++ {
			if _, err := inspectSvc.CreateTask(ctx, &request.CreateInspectTaskReq{
				Name: keyword, InspectorID: other.User.ID,
				PlanDT: time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
			}); err != nil {
				t.Fatalf("创建任务失败: %v", err)
			}
		}

		resp, err := svc.PageInspectTasks(ctx, inspector.User.ID, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20},
		})
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if resp.Total != 2 {
			t.Fatalf("我的任务数 = %d, 期望 2（仅 inspector_id = 当前用户）", resp.Total)
		}
		for _, it := range resp.Items {
			if it.InspectorID != inspector.User.ID {
				t.Errorf("返回了他人任务: %+v", it)
			}
			if it.Name == "" || it.Location == "" || it.Content == "" || it.PlanDT.IsZero() ||
				it.Status != model.InspectStatusPublished {
				t.Errorf("items 字段不完整: %+v", it)
			}
		}

		// 时间过滤按 plan_dt（任务巡检时间为 1 小时后，仍属当天）
		today := time.Now().Format("2006-01-02")
		byRange, _ := svc.PageInspectTasks(ctx, inspector.User.ID, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, StartDT: today, EndDT: today,
		})
		if byRange.Total != 2 {
			t.Errorf("按当天过滤命中 %d, 期望 2", byRange.Total)
		}
		tomorrow := time.Now().Add(48 * time.Hour).Format("2006-01-02")
		byTomorrow, _ := svc.PageInspectTasks(ctx, inspector.User.ID, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, StartDT: tomorrow, EndDT: tomorrow,
		})
		if byTomorrow.Total != 0 {
			t.Errorf("按后天过滤命中 %d, 期望 0", byTomorrow.Total)
		}
	})
}

// TestAppService_GetInspectTask 移动端任务详情含区域名称。
func TestAppService_GetInspectTask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_gettask"))

		region, err := svcbasic.NewRegionService().Create(ctx, &request.CreateRegionReq{
			Name: "ZZQ-移动详情区域", Code: uniq("ZZQ_ATREG"),
		})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		task, err := svcinspect.NewInspectService().CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: uniq("ZZQ-移动详情任务"), RegionID: &region.ID, Location: "1号配电房",
			InspectorID: f.User.ID, Content: "检查设备",
			PlanDT: time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
		})
		if err != nil {
			t.Fatalf("创建任务失败: %v", err)
		}

		got, err := svc.GetInspectTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if got.Name != task.Name || got.InspectorID != f.User.ID || got.Content != "检查设备" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.Region == nil || got.Region.Name != "ZZQ-移动详情区域" {
			t.Errorf("Region.Name 未回显: %+v", got.Region)
		}
	})
}

// TestAppService_FinishInspectTask 移动端完成任务：生成记录快照 + 越权拦截。
func TestAppService_FinishInspectTask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		inspector := newAppFixture(t, ctx, uniq("zzq_app_finish"))
		other := newAppFixture(t, ctx, uniq("zzq_app_other"))

		task, err := svcinspect.NewInspectService().CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: uniq("ZZQ-移动完成"), InspectorID: inspector.User.ID, Content: "检查设备",
			PlanDT: time.Now().Add(time.Hour).Format("2006-01-02 15:04:05"),
		})
		if err != nil {
			t.Fatalf("创建任务失败: %v", err)
		}

		// 越权：非指派人
		if _, err := svc.FinishInspectTask(ctx, task.ID, other.User.ID, &request.FinishInspectTaskReq{Result: "越权"}); err == nil {
			t.Error("非指派人完成应被拒绝")
		}
		after, _ := svc.GetInspectTask(ctx, task.ID)
		if after.Status != model.InspectStatusPublished {
			t.Errorf("拒绝后任务状态应保持 published, got %q", after.Status)
		}

		rec, err := svc.FinishInspectTask(ctx, task.ID, inspector.User.ID, &request.FinishInspectTaskReq{
			Result: "已处理", ResultImage: "/uploads/i.png",
		})
		if err != nil {
			t.Fatalf("完成任务失败: %v", err)
		}
		if rec.TaskID != task.ID || rec.TaskName != task.Name || rec.Content != "检查设备" {
			t.Errorf("快照字段错误: %+v", rec)
		}
		if rec.Result != "已处理" || rec.ResultImage != "/uploads/i.png" || rec.HandledDT == nil {
			t.Errorf("结果字段错误: %+v", rec)
		}
		if rec.Status != model.InspectStatusFinished {
			t.Errorf("记录 status = %q, 期望 finished", rec.Status)
		}
		done, _ := svc.GetInspectTask(ctx, task.ID)
		if done.Status != model.InspectStatusFinished {
			t.Errorf("任务 status = %q, 期望 finished", done.Status)
		}
		// Web 端记录列表可见同一结果
		records, _ := svcinspect.NewInspectService().PageRecords(ctx, &request.PageInspectRecordReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: task.Name,
		}, dsAll())
		if records.Total != 1 || records.Items[0].Result != "已处理" {
			t.Errorf("Web 端记录列表未同步: %+v", records.Items)
		}
	})
}

// TestAppService_PageDevices 移动端设备列表与详情。
func TestAppService_PageDevices(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_dev"))

		page, err := svc.PageDevices(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: f.Device.DeviceCode,
		}, dsAll())
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("total=%d items=%d, 期望 1/1", page.Total, len(page.Items))
		}
		if page.Items[0].Region == nil || page.Items[0].Region.Name == "" {
			t.Error("列表应预载区域名称")
		}

		got, err := svc.GetDevice(ctx, f.Device.ID)
		if err != nil {
			t.Fatalf("查询详情失败: %v", err)
		}
		if got.DeviceCode != f.Device.DeviceCode || got.Location != "东门" || got.Status != model.DeviceStatusOffline {
			t.Errorf("详情字段不完整: %+v", got)
		}
	})
}

// TestAppService_PageNotifications 移动端通知只读本人且可标记已读。
func TestAppService_PageNotifications(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		notifySvc := svcsystem.NewNotificationService()
		a := newAppFixture(t, ctx, uniq("zzq_app_notify_a"))
		b := newAppFixture(t, ctx, uniq("zzq_app_notify_b"))

		for i := 0; i < 2; i++ {
			if err := notifySvc.Push(ctx, a.User.ID, "risk", "标题", "内容", "risk_record", int64(i)); err != nil {
				t.Fatalf("推送失败: %v", err)
			}
		}
		if err := notifySvc.Push(ctx, b.User.ID, "risk", "他人", "内容", "", 0); err != nil {
			t.Fatalf("推送失败: %v", err)
		}

		page, err := svc.PageNotifications(ctx, a.User.ID, &request.PageNotificationReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20},
		})
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if page.Total != 2 {
			t.Fatalf("A 的通知数 = %d, 期望 2", page.Total)
		}
		for _, it := range page.Items {
			if it.UserID != a.User.ID {
				t.Errorf("出现他人通知: %+v", it)
			}
		}

		target := page.Items[0].ID
		if err := svc.MarkNotificationRead(ctx, a.User.ID, target); err != nil {
			t.Fatalf("标记已读失败: %v", err)
		}
		unread, _ := notifySvc.UnreadCount(ctx, a.User.ID)
		if unread != 1 {
			t.Errorf("标记一条后未读数 = %d, 期望 1", unread)
		}
		// 越权标记他人通知
		if err := svc.MarkNotificationRead(ctx, b.User.ID, page.Items[0].ID); err == nil {
			t.Error("标记他人通知应返回错误")
		}
	})
}

// TestAppService_GetProfile 移动端个人资料。
func TestAppService_GetProfile(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_profile"))

		got, err := svc.GetProfile(ctx, f.User.ID)
		if err != nil {
			t.Fatalf("查询失败: %v", err)
		}
		if got.ID != f.User.ID || got.Username != f.User.Username || got.Name != f.User.Name {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.DeptID == nil || *got.DeptID != testDeptID {
			t.Errorf("dept_id = %v", got.DeptID)
		}
	})
}

// TestAppService_UpdateProfile 移动端资料更新字段与 Web 端一致且不含提权字段。
func TestAppService_UpdateProfile(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		f := newAppFixture(t, ctx, uniq("zzq_app_updprof"))

		if err := svc.UpdateProfile(ctx, f.User.ID, &request.UpdateProfileReq{
			Name: "移动端姓名", Email: "app@x.com", Phone: "13800000002", Avatar: "/app.png", Remark: "移动端更新",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, _ := svc.GetProfile(ctx, f.User.ID)
		if got.Name != "移动端姓名" || got.Email != "app@x.com" || got.Phone != "13800000002" ||
			got.Avatar != "/app.png" || got.Remark != "移动端更新" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.IsAdmin == nil || *got.IsAdmin {
			t.Error("移动端资料更新不得改变 is_admin")
		}
		if got.Active == nil || !*got.Active {
			t.Error("移动端资料更新不得改变 active")
		}
		if got.Username != f.User.Username {
			t.Error("user_name 不可修改")
		}
	})
}

// TestAppService_ChangePassword 移动端改密后旧密码失效、新密码可登录。
func TestAppService_ChangePassword(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := appSvc()
		login := svcsystem.NewAuthService()
		f := newAppFixture(t, ctx, uniq("zzq_app_pwd"))

		const newPwd = "app@2026"
		if err := svc.ChangePassword(ctx, f.User.ID, &request.ChangePasswordReq{
			OldPassword: "P@ss123456", NewPassword: newPwd,
		}); err != nil {
			t.Fatalf("改密失败: %v", err)
		}

		var hash string
		if err := tx.Table("ac_user").Where("id = ?", f.User.ID).Pluck("password_hash", &hash).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(newPwd)); err != nil {
			t.Errorf("新密码应校验通过: %v", err)
		}
		if _, err := login.Login(ctx, f.User.Username, "P@ss123456"); err == nil {
			t.Error("旧密码不应可登录")
		}
		if _, err := login.Login(ctx, f.User.Username, newPwd); err != nil {
			t.Errorf("新密码应可登录: %v", err)
		}
		// 原密码错误时拒绝
		if err := svc.ChangePassword(ctx, f.User.ID, &request.ChangePasswordReq{
			OldPassword: "wrong", NewPassword: "another@2026",
		}); err == nil {
			t.Error("原密码错误应返回错误")
		}
	})
}

// walk 深度优先遍历菜单树。
func walk(nodes []model.Menu, fn func(model.Menu)) {
	for _, n := range nodes {
		fn(n)
		walk(n.Children, fn)
	}
}
