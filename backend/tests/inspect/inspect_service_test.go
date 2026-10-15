package inspect

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svcinspect "github.com/boticz/ai-camera-backend/internal/service/inspect"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 巡检中心（InspectService）：发布即生效 + 完成转记录快照 + 越权拦截
// ════════════════════════════════════════════════════════════════

// createTask 通过 InspectService 创建巡检任务。
func createTask(t *testing.T, ctx context.Context, name string, regionID *int64, inspectorID int64) *model.InspectTask {
	t.Helper()
	task, err := svcinspect.NewInspectService().CreateTask(ctx, &request.CreateInspectTaskReq{
		Name: name, RegionID: regionID, Location: "1号配电房", InspectorID: inspectorID,
		Content: "检查设备外观、接线端子与运行温度", PlanDT: time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04:05"),
		Remark: "雨季重点巡检",
	})
	if err != nil {
		t.Fatalf("创建巡检任务失败: %v", err)
	}
	return task
}

// TestInspectService_CreateTask 创建任务后状态直接为 published（发布即生效）。
func TestInspectService_CreateTask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		task, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: "东门日巡", Location: "东门", InspectorID: mustUser(t, ctx, uniq("zzq_insp")),
			Content: "检查摄像头角度与遮挡",
			PlanDT:  time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04:05"), Remark: "重点区域",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if task.Status != model.InspectStatusPublished {
			t.Errorf("status = %q, 期望 published（发布即生效，不是 pending）", task.Status)
		}
		if task.Read == nil || *task.Read {
			t.Errorf("read = %v, 期望 false", task.Read)
		}
		if task.Active == nil || !*task.Active {
			t.Errorf("active = %v, 期望 true", task.Active)
		}
	})
}

// TestInspectService_CreateTask_DefaultStatus 不传 status 时默认 published，传 pending 则保留。
func TestInspectService_CreateTask_DefaultStatus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()
		inspector := mustUser(t, ctx, uniq("zzq_insp"))

		def := createTask(t, ctx, "ZZQ-默认状态", nil, inspector)
		got, _ := svc.GetTask(ctx, def.ID)
		if got.Status != model.InspectStatusPublished {
			t.Errorf("默认 status = %q, 期望 published", got.Status)
		}
		if got.Read == nil || *got.Read {
			t.Errorf("默认 read = %v, 期望 false", got.Read)
		}

		pending, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: "ZZQ-待发布", InspectorID: inspector, Status: model.InspectStatusPending,
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if pending.Status != model.InspectStatusPending {
			t.Errorf("显式 pending 应保留, got %q", pending.Status)
		}

		// 非法状态回落为 published
		bad, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: "ZZQ-非法状态", InspectorID: inspector, Status: "finished",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if bad.Status != model.InspectStatusPublished {
			t.Errorf("非法状态应回落为 published, got %q", bad.Status)
		}
	})
}

// TestInspectService_CreateTask_Fields 创建任务全字段落库。
func TestInspectService_CreateTask_Fields(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionID := mustRegion(t, ctx, "ZZQ-巡检区域")
		inspector := mustUser(t, ctx, uniq("zzq_insp_field"))
		planDT := time.Now().Add(48 * time.Hour).Truncate(time.Second)

		task, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: "东门日巡", RegionID: &regionID, Location: "东门", InspectorID: inspector,
			Content: "检查摄像头角度与遮挡", PlanDT: planDT.Format("2006-01-02 15:04:05"), Remark: "重点区域",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		got, err := svc.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if got.Name != "东门日巡" || got.Location != "东门" || got.Content != "检查摄像头角度与遮挡" ||
			got.Remark != "重点区域" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.RegionID == nil || *got.RegionID != regionID {
			t.Errorf("region_id = %v, 期望 %d", got.RegionID, regionID)
		}
		if got.InspectorID != inspector {
			t.Errorf("inspector_id = %d, 期望 %d", got.InspectorID, inspector)
		}
		if !got.PlanDT.Truncate(time.Second).Equal(planDT) {
			t.Errorf("plan_dt = %v, 期望 %v", got.PlanDT, planDT)
		}
		if got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", got.CreatedBy, got.CreatedDT)
		}
		if got.DeptID == nil || *got.DeptID != testDeptID {
			t.Errorf("dept_id = %v, 期望 %d", got.DeptID, testDeptID)
		}
	})
}

// TestInspectService_CreateTask_Validation 巡检人不存在时拒绝，不产生孤儿任务。
func TestInspectService_CreateTask_Validation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()
		before := countRows(t, tx, "ac_inspect_task", "")

		_, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{Name: "ZZQ-孤儿任务", InspectorID: 999999999})
		if err == nil {
			t.Fatal("巡检人不存在应返回错误")
		}
		if !strings.Contains(err.Error(), "巡检人不存在") {
			t.Errorf("错误信息 = %v, 期望提示巡检人不存在", err)
		}
		if n := countRows(t, tx, "ac_inspect_task", ""); n != before {
			t.Errorf("不得新增任务: %d -> %d", before, n)
		}

		// plan_dt 格式非法时拒绝
		inspector := mustUser(t, ctx, uniq("zzq_insp_bad"))
		if _, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: "ZZQ-非法时间", InspectorID: inspector, PlanDT: "not-a-date",
		}); err == nil {
			t.Error("非法 plan_dt 应返回错误")
		}
	})
}

// TestInspectService_GetTask 任务详情逐字段核对，含区域名称。
func TestInspectService_GetTask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionID := mustRegion(t, ctx, "ZZQ-详情区域")
		task := createTask(t, ctx, "ZZQ-详情任务", &regionID, mustUser(t, ctx, uniq("zzq_insp_get")))

		got, err := svc.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if got.Name != "ZZQ-详情任务" || got.Location != "1号配电房" || got.Content == "" || got.Remark == "" {
			t.Errorf("字段不完整: %+v", got)
		}
		if got.RegionID == nil || *got.RegionID != regionID {
			t.Errorf("region_id = %v", got.RegionID)
		}
		if got.Status != model.InspectStatusPublished || got.Read == nil || *got.Read {
			t.Errorf("状态字段错误: %q/%v", got.Status, got.Read)
		}
		if got.Region == nil || got.Region.Name != "ZZQ-详情区域" {
			t.Errorf("Region.Name 未回显: %+v", got.Region)
		}
		if _, err := svc.GetTask(ctx, 999999999); err == nil {
			t.Error("不存在的任务应返回错误")
		}
	})
}

// TestInspectService_GetTask_MarkRead 查看任务不改变 read（按实际实现：读操作无副作用）。
func TestInspectService_GetTask_MarkRead(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()
		task := createTask(t, ctx, "ZZQ-查看任务", nil, mustUser(t, ctx, uniq("zzq_insp_read")))

		got, err := svc.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		// GetTask 为纯查询，不写库；「查看转已读」由显式接口 MarkTaskRead 承担。
		if got.Read == nil || *got.Read {
			t.Errorf("GetTask 不应产生副作用: read = %v", got.Read)
		}
		again, _ := svc.GetTask(ctx, task.ID)
		if again.Status != model.InspectStatusPublished || again.Read == nil || *again.Read {
			t.Errorf("重复查看不应改变任务: status=%q read=%v", again.Status, again.Read)
		}
	})
}

// TestInspectService_MarkTaskRead 任务级已读接口：首次转已读、重复调用幂等、任务不存在报错。
func TestInspectService_MarkTaskRead(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()
		inspector := mustUser(t, ctx, uniq("zzq_mark_insp"))
		task := createTask(t, ctx, uniq("ZZQ-MARK-READ"), nil, inspector)

		before, err := svc.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if before.Read == nil || *before.Read {
			t.Fatalf("前置条件：read 应为 false, got %v", before.Read)
		}

		if err := svc.MarkTaskRead(ctx, task.ID); err != nil {
			t.Fatalf("标记已读失败: %v", err)
		}
		got, err := svc.GetTask(ctx, task.ID)
		if err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if got.Read == nil || !*got.Read {
			t.Errorf("read = %v, 期望 true", got.Read)
		}
		// 标记已读不得改动任务的其他字段
		if got.Status != before.Status || got.Name != before.Name ||
			got.InspectorID != before.InspectorID || got.Content != before.Content ||
			!got.PlanDT.Equal(before.PlanDT) {
			t.Errorf("标记已读改动了其他字段: %+v -> %+v", before, got)
		}

		// 幂等：重复调用不报错，且保持已读
		if err := svc.MarkTaskRead(ctx, task.ID); err != nil {
			t.Errorf("重复标记已读应幂等: %v", err)
		}
		again, _ := svc.GetTask(ctx, task.ID)
		if again.Read == nil || !*again.Read {
			t.Errorf("重复调用后 read = %v, 期望 true", again.Read)
		}

		// 任务不存在：返回错误（Handler 依据 not found 转为 404「巡检任务不存在」）
		err = svc.MarkTaskRead(ctx, 999999999)
		if err == nil {
			t.Fatal("不存在的任务应返回错误")
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("错误应为 not found: %v", err)
		}
	})
}

// TestInspectService_PageTasks 任务分页元信息与软删除不可见。
func TestInspectService_PageTasks(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()
		inspector := mustUser(t, ctx, uniq("zzq_insp_page"))

		keyword := uniq("ZZQ-PAGE-TASK")
		for i := 0; i < 3; i++ {
			if _, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{
				Name: keyword, InspectorID: inspector, Content: "c",
			}); err != nil {
				t.Fatalf("创建失败: %v", err)
			}
		}
		resp, err := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 3 || len(resp.Items) != 3 {
			t.Fatalf("total=%d items=%d, 期望 3/3", resp.Total, len(resp.Items))
		}
		if resp.Page != 1 || resp.PageSize != 20 {
			t.Errorf("分页回显 page=%d page_size=%d", resp.Page, resp.PageSize)
		}

		if err := svc.DeleteTask(ctx, resp.Items[0].ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		after, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword,
		}, dsAll())
		if after.Total != 2 {
			t.Errorf("软删除后 total = %d, 期望 2", after.Total)
		}
	})
}

// TestInspectService_PageTasks_Preload 任务列表回显区域名称与巡检人姓名。
func TestInspectService_PageTasks_Preload(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionID := mustRegion(t, ctx, "ZZQ-预载区域")
		inspectorName := uniq("zzq_insp_preload")
		inspector := mustUser(t, ctx, inspectorName)
		task := createTask(t, ctx, uniq("ZZQ-PRELOAD-TASK"), &regionID, inspector)

		resp, err := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, InspectorID: &inspector,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 1 || len(resp.Items) != 1 {
			t.Fatalf("total=%d items=%d, 期望 1/1", resp.Total, len(resp.Items))
		}
		it := resp.Items[0]
		if it.ID != task.ID {
			t.Errorf("任务 ID = %d, 期望 %d", it.ID, task.ID)
		}
		if it.Region == nil || it.Region.Name != "ZZQ-预载区域" {
			t.Errorf("Region.Name 未回显: %+v", it.Region)
		}
		if it.InspectorName != inspectorName {
			t.Errorf("inspector_name = %q, 期望 %q", it.InspectorName, inspectorName)
		}
	})
}

// TestInspectService_PageTasks_Filters 任务列表过滤条件。
func TestInspectService_PageTasks_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionA := mustRegion(t, ctx, "ZZQ-过滤区域A")
		regionB := mustRegion(t, ctx, "ZZQ-过滤区域B")
		inspectorA := mustUser(t, ctx, uniq("zzq_ia"))
		inspectorB := mustUser(t, ctx, uniq("zzq_ib"))

		keyword := uniq("ZZQ-FILTER-TASK")
		createTask(t, ctx, keyword, &regionA, inspectorA)
		createTask(t, ctx, keyword, &regionB, inspectorB)
		// 待发布任务，巡检时间为当天
		today := time.Now().Format("2006-01-02")
		pending, err := svc.CreateTask(ctx, &request.CreateInspectTaskReq{
			Name: keyword, RegionID: &regionA, InspectorID: inspectorA,
			Status: model.InspectStatusPending, PlanDT: today,
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		byRegion, err := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, RegionID: &regionA,
		}, dsAll())
		if err != nil {
			t.Fatalf("按区域分页失败: %v", err)
		}
		if byRegion.Total != 2 {
			t.Errorf("按区域命中 %d, 期望 2", byRegion.Total)
		}

		byInspector, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, InspectorID: &inspectorB,
		}, dsAll())
		if byInspector.Total != 1 {
			t.Errorf("按巡检人命中 %d, 期望 1", byInspector.Total)
		}

		byStatus, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, Status: model.InspectStatusPending,
		}, dsAll())
		if byStatus.Total != 1 || byStatus.Items[0].ID != pending.ID {
			t.Errorf("按状态命中 %d, 期望 1（且为指定任务）", byStatus.Total)
		}

		// 时间范围（按 plan_dt）为闭区间：当天 1 条、明天 2 条
		byToday, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, StartDT: today, EndDT: today,
		}, dsAll())
		if byToday.Total != 1 {
			t.Errorf("按当天巡检时间命中 %d, 期望 1", byToday.Total)
		}
		tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
		byRange, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, StartDT: tomorrow, EndDT: tomorrow,
		}, dsAll())
		if byRange.Total != 2 {
			t.Errorf("按明天巡检时间命中 %d, 期望 2", byRange.Total)
		}

		// 组合条件为 AND + 名称模糊
		combo, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, RegionID: &regionA, InspectorID: &inspectorB,
		}, dsAll())
		if combo.Total != 0 {
			t.Errorf("组合条件命中 %d, 期望 0", combo.Total)
		}

		none, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: uniq("ZZQ-NO-TASK"),
		}, dsAll())
		if none.Total != 0 || len(none.Items) != 0 {
			t.Errorf("无匹配时 total=%d items=%d", none.Total, len(none.Items))
		}
	})
}

// TestInspectService_UpdateTask 更新任务：7 字段生效且状态/已读不变。
func TestInspectService_UpdateTask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionA := mustRegion(t, ctx, "ZZQ-更新区域A")
		regionB := mustRegion(t, ctx, "ZZQ-更新区域B")
		inspectorA := mustUser(t, ctx, uniq("zzq_upa"))
		inspectorB := mustUser(t, ctx, uniq("zzq_upb"))
		task := createTask(t, ctx, "ZZQ-待更新任务", &regionA, inspectorA)

		planDT := time.Now().Add(72 * time.Hour).Truncate(time.Second)
		if err := svc.UpdateTask(ctx, task.ID, &request.UpdateInspectTaskReq{
			Name: "ZZQ-更新后任务", RegionID: &regionB, Location: "西门", InspectorID: inspectorB,
			Content: "新内容", PlanDT: planDT.Format("2006-01-02 15:04:05"), Remark: "新备注",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, _ := svc.GetTask(ctx, task.ID)
		if got.Name != "ZZQ-更新后任务" || got.Location != "西门" || got.Content != "新内容" || got.Remark != "新备注" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.RegionID == nil || *got.RegionID != regionB {
			t.Errorf("region_id = %v, 期望 %d", got.RegionID, regionB)
		}
		if got.InspectorID != inspectorB {
			t.Errorf("inspector_id = %d, 期望 %d", got.InspectorID, inspectorB)
		}
		if !got.PlanDT.Truncate(time.Second).Equal(planDT) {
			t.Errorf("plan_dt = %v, 期望 %v", got.PlanDT, planDT)
		}
		if got.Status != model.InspectStatusPublished {
			t.Errorf("status 不应变化: %q", got.Status)
		}
		if got.Read == nil || *got.Read {
			t.Errorf("read 不应变化: %v", got.Read)
		}
		if got.CreatedBy != testUserName {
			t.Errorf("created_by 不应变化: %q", got.CreatedBy)
		}

		// 巡检人不存在时拒绝
		if err := svc.UpdateTask(ctx, task.ID, &request.UpdateInspectTaskReq{
			Name: got.Name, InspectorID: 999999999,
		}); err == nil {
			t.Error("巡检人不存在应返回错误")
		}
	})
}

// TestInspectService_UpdateTask_Finished 已完成任务不可编辑。
func TestInspectService_UpdateTask_Finished(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()
		inspector := mustUser(t, ctx, uniq("zzq_fin"))
		task := createTask(t, ctx, "ZZQ-已完成任务", nil, inspector)
		if _, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{Result: "已完成"}); err != nil {
			t.Fatalf("完成任务失败: %v", err)
		}
		before, _ := svc.GetTask(ctx, task.ID)

		err := svc.UpdateTask(ctx, task.ID, &request.UpdateInspectTaskReq{
			Name: "ZZQ-试图修改", InspectorID: inspector,
		})
		if err == nil {
			t.Fatal("已完成任务不可修改")
		}
		if !strings.Contains(err.Error(), "不可修改") {
			t.Errorf("错误信息 = %v", err)
		}
		after, _ := svc.GetTask(ctx, task.ID)
		if after.Name != before.Name || after.Content != before.Content || after.Status != before.Status {
			t.Errorf("拒绝后字段不应被修改: %+v", after)
		}
	})
}

// TestInspectService_DeleteTask 删除任务为软删除，且已生成记录仍保留。
func TestInspectService_DeleteTask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()
		inspector := mustUser(t, ctx, uniq("zzq_del_insp"))
		task := createTask(t, ctx, uniq("ZZQ-DEL-TASK"), nil, inspector)

		if err := svc.DeleteTask(ctx, task.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_inspect_task", task.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		if _, err := svc.GetTask(ctx, task.ID); err == nil {
			t.Error("软删除后不应可读")
		}

		// 已完成任务不可删除
		done := createTask(t, ctx, uniq("ZZQ-DONE-TASK"), nil, inspector)
		if _, err := svc.FinishTask(ctx, done.ID, inspector, &request.FinishInspectTaskReq{Result: "已完成"}); err != nil {
			t.Fatalf("完成任务失败: %v", err)
		}
		if err := svc.DeleteTask(ctx, done.ID); err == nil {
			t.Error("已完成任务不可删除")
		}
		if n := countRows(t, tx, "ac_inspect_record", "task_id = ?", done.ID); n != 1 {
			t.Errorf("巡检记录应保留, got %d", n)
		}
	})
}

// TestInspectService_FinishTask 完成任务生成巡检记录（事务 + 快照 + 状态流转）。
func TestInspectService_FinishTask(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionID := mustRegion(t, ctx, "ZZQ-完成区域")
		inspector := mustUser(t, ctx, uniq("zzq_finish"))
		task := createTask(t, ctx, "ZZQ-完成巡检", &regionID, inspector)

		before := countRows(t, tx, "ac_inspect_record", "")
		rec, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{
			Result: "角度已调整", ResultImage: "/uploads/2026/09/inspect.png",
		})
		if err != nil {
			t.Fatalf("完成任务失败: %v", err)
		}

		if n := countRows(t, tx, "ac_inspect_record", ""); n != before+1 {
			t.Fatalf("巡检记录新增 %d 条, 期望 1", n-before)
		}
		if rec.TaskID != task.ID || rec.TaskName != task.Name {
			t.Errorf("task_id/task_name 错误: %d/%q", rec.TaskID, rec.TaskName)
		}
		if rec.RegionID == nil || *rec.RegionID != regionID || rec.Location != task.Location {
			t.Errorf("快照区域/位置错误: %v/%q", rec.RegionID, rec.Location)
		}
		if rec.InspectorID != inspector || rec.Inspector == "" {
			t.Errorf("巡检人字段错误: %d/%q", rec.InspectorID, rec.Inspector)
		}
		if rec.Content != task.Content {
			t.Errorf("content 快照错误: %q", rec.Content)
		}
		if !rec.PlanDT.Truncate(time.Second).Equal(task.PlanDT.Truncate(time.Second)) {
			t.Errorf("plan_dt 快照错误: %v vs %v", rec.PlanDT, task.PlanDT)
		}
		if rec.Result != "角度已调整" || rec.ResultImage != "/uploads/2026/09/inspect.png" {
			t.Errorf("结果字段错误: %q/%q", rec.Result, rec.ResultImage)
		}
		if rec.HandledDT == nil {
			t.Error("handled_dt 应非空")
		}
		if rec.Status != model.InspectStatusFinished {
			t.Errorf("记录 status = %q, 期望 finished", rec.Status)
		}
		if rec.CreatedBy != testUserName {
			t.Errorf("created_by = %q", rec.CreatedBy)
		}

		after, _ := svc.GetTask(ctx, task.ID)
		if after.Status != model.InspectStatusFinished {
			t.Errorf("任务 status = %q, 期望 finished", after.Status)
		}
	})
}

// TestInspectService_FinishTask_Snapshot 记录快照独立于任务后续改动。
func TestInspectService_FinishTask_Snapshot(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionID := mustRegion(t, ctx, "ZZQ-快照区域")
		inspector := mustUser(t, ctx, uniq("zzq_snap"))
		task := createTask(t, ctx, "ZZQ-快照任务", &regionID, inspector)

		rec, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{Result: "完成"})
		if err != nil {
			t.Fatalf("完成任务失败: %v", err)
		}
		taskName, taskContent, taskLocation := rec.TaskName, rec.Content, rec.Location

		// 直接改写任务（模拟任务被改动）
		if err := tx.Model(&model.InspectTask{}).Where("id = ?", task.ID).
			Updates(map[string]interface{}{
				"name": "ZZQ-改名后", "content": "改写后的内容", "location": "改写后的位置",
			}).Error; err != nil {
			t.Fatalf("改写任务失败: %v", err)
		}

		got, err := svc.GetRecord(ctx, rec.ID)
		if err != nil {
			t.Fatalf("读取记录失败: %v", err)
		}
		if got.TaskName != taskName || got.Content != taskContent || got.Location != taskLocation {
			t.Errorf("快照应保持生成时的值: %q/%q/%q", got.TaskName, got.Content, got.Location)
		}
	})
}

// TestInspectService_FinishTask_StatusFlow 状态流转 published → finished，列表同步收敛。
func TestInspectService_FinishTask_StatusFlow(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		inspector := mustUser(t, ctx, uniq("zzq_flow"))
		task := createTask(t, ctx, uniq("ZZQ-FLOW-TASK"), nil, inspector)

		published, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: task.Name, Status: model.InspectStatusPublished,
		}, dsAll())
		if published.Total != 1 {
			t.Fatalf("完成前 published 列表 total = %d, 期望 1", published.Total)
		}

		if _, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{Result: "完成"}); err != nil {
			t.Fatalf("完成任务失败: %v", err)
		}

		publishedAfter, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: task.Name, Status: model.InspectStatusPublished,
		}, dsAll())
		if publishedAfter.Total != 0 {
			t.Errorf("完成后 published 列表应不含该任务, total = %d", publishedAfter.Total)
		}
		finished, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: task.Name, Status: model.InspectStatusFinished,
		}, dsAll())
		if finished.Total != 1 {
			t.Errorf("完成后 finished 列表应包含该任务, total = %d", finished.Total)
		}
	})
}

// TestInspectService_FinishTask_Forbidden 越权完成他人任务被拒绝。
func TestInspectService_FinishTask_Forbidden(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		inspectorA := mustUser(t, ctx, uniq("zzq_owner"))
		inspectorB := mustUser(t, ctx, uniq("zzq_other"))
		task := createTask(t, ctx, uniq("ZZQ-FORBID"), nil, inspectorA)

		before := countRows(t, tx, "ac_inspect_record", "task_id = ?", task.ID)
		_, err := svc.FinishTask(ctx, task.ID, inspectorB, &request.FinishInspectTaskReq{Result: "越权完成"})
		if err == nil {
			t.Fatal("非指派人完成应被拒绝")
		}
		if !strings.Contains(err.Error(), "无权") {
			t.Errorf("错误信息 = %v, 期望提示无权处理", err)
		}
		if n := countRows(t, tx, "ac_inspect_record", "task_id = ?", task.ID); n != before {
			t.Errorf("拒绝时不得生成记录: %d -> %d", before, n)
		}
		got, _ := svc.GetTask(ctx, task.ID)
		if got.Status != model.InspectStatusPublished {
			t.Errorf("任务状态应保持 published, got %q", got.Status)
		}
		if got.Read == nil || *got.Read {
			t.Errorf("read 不应变化: %v", got.Read)
		}
	})
}

// TestInspectService_FinishTask_Idempotent 重复完成不产生重复记录。
func TestInspectService_FinishTask_Idempotent(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		inspector := mustUser(t, ctx, uniq("zzq_idem"))
		task := createTask(t, ctx, uniq("ZZQ-IDEM"), nil, inspector)

		if _, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{Result: "首次"}); err != nil {
			t.Fatalf("首次完成失败: %v", err)
		}
		_, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{Result: "二次"})
		if err == nil {
			t.Fatal("重复完成应返回错误")
		}
		if !strings.Contains(err.Error(), "已处理") {
			t.Errorf("错误信息 = %v, 期望提示任务已处理", err)
		}
		if n := countRows(t, tx, "ac_inspect_record", "task_id = ?", task.ID); n != 1 {
			t.Errorf("巡检记录数 = %d, 期望 1（不产生重复）", n)
		}
	})
}

// TestInspectService_PageRecords_Filters 巡检记录分页与过滤。
func TestInspectService_PageRecords_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionA := mustRegion(t, ctx, "ZZQ-记录区域A")
		regionB := mustRegion(t, ctx, "ZZQ-记录区域B")
		inspectorA := mustUser(t, ctx, uniq("zzq_ra"))
		inspectorB := mustUser(t, ctx, uniq("zzq_rb"))

		keyword := uniq("ZZQ-RECORD")
		t1 := createTask(t, ctx, keyword, &regionA, inspectorA)
		t2 := createTask(t, ctx, keyword, &regionB, inspectorB)
		for _, tk := range []*model.InspectTask{t1, t2} {
			if _, err := svc.FinishTask(ctx, tk.ID, tk.InspectorID, &request.FinishInspectTaskReq{
				Result: "已处理", ResultImage: "/uploads/r.png",
			}); err != nil {
				t.Fatalf("完成任务失败: %v", err)
			}
		}

		byRegion, err := svc.PageRecords(ctx, &request.PageInspectRecordReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, RegionID: &regionA,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if byRegion.Total != 1 {
			t.Errorf("按区域命中 %d, 期望 1", byRegion.Total)
		}

		byInspector, _ := svc.PageRecords(ctx, &request.PageInspectRecordReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, InspectorID: &inspectorB,
		}, dsAll())
		if byInspector.Total != 1 {
			t.Errorf("按巡检人命中 %d, 期望 1", byInspector.Total)
		}

		// 记录的快照 plan_dt 取自任务（createTask 设为次日）
		tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
		byRange, _ := svc.PageRecords(ctx, &request.PageInspectRecordReq{
			PageReq: request.PageReq{Page: 1, PageSize: 200}, Keyword: keyword, StartDT: tomorrow, EndDT: tomorrow,
		}, dsAll())
		if byRange.Total != 2 {
			t.Errorf("按巡检时间范围命中 %d, 期望 2", byRange.Total)
		}
		for _, it := range byRange.Items {
			if it.Result != "已处理" || it.ResultImage != "/uploads/r.png" || it.Status != model.InspectStatusFinished {
				t.Errorf("记录字段不完整: %+v", it)
			}
			if it.HandledDT == nil {
				t.Error("handled_dt 应非空")
			}
		}
	})
}

// TestInspectService_GetRecord 记录详情逐字段核对，含区域名称。
func TestInspectService_GetRecord(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionID := mustRegion(t, ctx, "ZZQ-记录详情区域")
		inspector := mustUser(t, ctx, uniq("zzq_rec_get"))
		task := createTask(t, ctx, uniq("ZZQ-REC-GET"), &regionID, inspector)
		rec, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{
			Result: "设备运行正常，无异常发热", ResultImage: "/uploads/2026/09/i.png",
		})
		if err != nil {
			t.Fatalf("完成任务失败: %v", err)
		}

		got, err := svc.GetRecord(ctx, rec.ID)
		if err != nil {
			t.Fatalf("获取记录失败: %v", err)
		}
		if got.TaskID != task.ID || got.TaskName != task.Name || got.InspectorID != inspector {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.RegionID == nil || *got.RegionID != regionID {
			t.Errorf("region_id = %v", got.RegionID)
		}
		if got.Inspector == "" || got.Content != task.Content {
			t.Errorf("快照字段不完整: inspector=%q content=%q", got.Inspector, got.Content)
		}
		if got.Result != "设备运行正常，无异常发热" || got.ResultImage != "/uploads/2026/09/i.png" {
			t.Errorf("结果字段错误: %q/%q", got.Result, got.ResultImage)
		}
		if got.HandledDT == nil || got.Status != model.InspectStatusFinished {
			t.Errorf("状态字段错误: %v/%q", got.HandledDT, got.Status)
		}
		if got.Region == nil || got.Region.Name != "ZZQ-记录详情区域" {
			t.Errorf("Region.Name 未回显: %+v", got.Region)
		}
		if _, err := svc.GetRecord(ctx, 999999999); err == nil {
			t.Error("不存在的记录应返回错误")
		}
	})
}

// TestInspectService_PageTasks_DataScope 任务与记录的可见性均随数据权限收敛。
func TestInspectService_PageTasks_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := svcinspect.NewInspectService()

		inspector := mustUser(t, ctx, uniq("zzq_ds_insp"))
		deptA, deptB, deptC := int64(9811), int64(9812), int64(9813)
		if err := tx.Unscoped().Where("dept_id IN ?", []int64{deptA, deptB, deptC}).Delete(&model.InspectTask{}).Error; err != nil {
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
			m := &model.InspectTask{
				Name: "ZZQ-DS-TASK-" + s.suffix, InspectorID: inspector, Status: model.InspectStatusPublished,
				PlanDT: time.Now(), Active: boolPtr(true),
				DeptID:    int64Ptr(s.dept),
				BaseModel: model.BaseModel{CreatedBy: s.creator, UpdatedBy: s.creator},
			}
			if err := tx.Create(m).Error; err != nil {
				t.Fatalf("预置数据失败: %v", err)
			}
		}

		q := request.PageInspectTaskReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}
		all, err := svc.PageTasks(ctx, &q, dsAll())
		if err != nil {
			t.Fatalf("all 分页失败: %v", err)
		}
		cross, _ := svc.PageTasks(ctx, &q, dsPerm("cross_dept", "ds_user_b", &deptA, "9812"))
		dept, _ := svc.PageTasks(ctx, &q, dsPerm("dept", "ds_user_b", &deptA, ""))
		personal, _ := svc.PageTasks(ctx, &q, dsPerm("personal", "ds_user_a", &deptA, ""))

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
	})
}

// TestIT_InspectTaskClosure 跨端闭环：Web 创建 → 移动端完成 → Web 记录可见。
func TestIT_InspectTaskClosure(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := svcinspect.NewInspectService()

		regionID := mustRegion(t, ctx, "ZZQ-闭环区域")
		inspector := mustUser(t, ctx, uniq("zzq_closure"))
		inspectorID := inspector
		task := createTask(t, ctx, uniq("ZZQ-CLOSURE"), &regionID, inspector)

		// 移动端视角：仅能查询自己的任务
		appList, err := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, InspectorID: &inspectorID,
		}, dsAll())
		if err != nil {
			t.Fatalf("移动端任务列表失败: %v", err)
		}
		if appList.Total != 1 {
			t.Fatalf("移动端任务数 = %d, 期望 1", appList.Total)
		}

		rec, err := svc.FinishTask(ctx, task.ID, inspector, &request.FinishInspectTaskReq{
			Result: "移动端提交结果", ResultImage: "/uploads/app.png",
		})
		if err != nil {
			t.Fatalf("移动端完成失败: %v", err)
		}

		// Web 端记录列表出现对应记录，字段与移动端提交一致
		webRecords, err := svc.PageRecords(ctx, &request.PageInspectRecordReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, InspectorID: &inspectorID,
		}, dsAll())
		if err != nil {
			t.Fatalf("Web 记录列表失败: %v", err)
		}
		if webRecords.Total != 1 {
			t.Fatalf("Web 记录数 = %d, 期望 1", webRecords.Total)
		}
		got := webRecords.Items[0]
		if got.ID != rec.ID || got.Result != "移动端提交结果" || got.ResultImage != "/uploads/app.png" {
			t.Errorf("两端结果不一致: %+v", got)
		}

		// Web 端任务列表不再包含该任务
		webTasks, _ := svc.PageTasks(ctx, &request.PageInspectTaskReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, InspectorID: &inspectorID, Status: model.InspectStatusPublished,
		}, dsAll())
		if webTasks.Total != 0 {
			t.Errorf("完成后任务不应出现在 published 列表, total = %d", webTasks.Total)
		}
		// 服务端已置 finished
		after, _ := svc.GetTask(ctx, task.ID)
		if after.Status != model.InspectStatusFinished {
			t.Errorf("任务 status = %q, 期望 finished", after.Status)
		}
	})
}
