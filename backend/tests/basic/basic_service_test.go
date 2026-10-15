package basic

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/basic"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 基础信息：区域 / 风险类型 / 告警类型
// ════════════════════════════════════════════════════════════════

// ── RegionService ──────────────────────────────────────────────

// TestRegionService_Create 创建区域：全字段落库 + 默认值 + 审计字段。
func TestRegionService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		got, err := svc.Create(ctx, &request.CreateRegionReq{
			Name:     "ZZQ-创建区域",
			Code:     "ZZQ_R1",
			Sequence: 30,
			Active:   boolPtr(true),
			Remark:   "测试备注",
		})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}

		m, err := svc.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("回读区域失败: %v", err)
		}
		if m.Name != "ZZQ-创建区域" {
			t.Errorf("name = %q", m.Name)
		}
		if m.Code != "ZZQ_R1" {
			t.Errorf("code = %q", m.Code)
		}
		if m.Sequence != 30 {
			t.Errorf("sequence = %d", m.Sequence)
		}
		if m.Active == nil || !*m.Active {
			t.Errorf("active 应为 true, got %v", m.Active)
		}
		if m.Remark != "测试备注" {
			t.Errorf("remark = %q", m.Remark)
		}
		if m.CreatedBy != testUserName {
			t.Errorf("created_by = %q, 期望 %q", m.CreatedBy, testUserName)
		}
		if m.CreatedDT.IsZero() {
			t.Error("created_dt 不应为零值")
		}
		if m.DeptID == nil || *m.DeptID != testDeptID {
			t.Errorf("dept_id = %v, 期望 %d", m.DeptID, testDeptID)
		}

		// 未传 sequence / active 时走默认值
		def, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-默认值"})
		if err != nil {
			t.Fatalf("创建默认区域失败: %v", err)
		}
		if def.Sequence != 10 {
			t.Errorf("默认 sequence = %d, 期望 10", def.Sequence)
		}
		if def.Active == nil || !*def.Active {
			t.Errorf("默认 active 应为 true, got %v", def.Active)
		}
	})
}

// TestRegionService_Get 区域详情逐字段核对，不存在时返回错误。
func TestRegionService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		created, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-详情", Code: "ZZQ_G", Sequence: 7, Remark: "r"})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		m, err := svc.Get(ctx, created.ID)
		if err != nil {
			t.Fatalf("获取区域失败: %v", err)
		}
		if m.ID != created.ID || m.Name != "ZZQ-详情" || m.Code != "ZZQ_G" || m.Sequence != 7 || m.Remark != "r" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.UpdatedDT.IsZero() {
			t.Error("updated_dt 不应为零值")
		}

		if _, err := svc.Get(ctx, 999999999); err == nil {
			t.Error("查询不存在的区域应返回错误")
		}
	})
}

// TestRegionService_Update 区域更新逐字段回读，且 created_* 不被覆盖。
func TestRegionService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		created, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-旧", Code: "ZZQ_OLD", Sequence: 5})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		origCreatedBy, origCreatedDT := created.CreatedBy, created.CreatedDT

		if err := svc.Update(ctx, created.ID, &request.UpdateRegionReq{
			Name:     "厂区B",
			Code:     "R-B",
			Sequence: 20,
			Active:   boolPtr(false),
			Remark:   "已调整",
		}); err != nil {
			t.Fatalf("更新区域失败: %v", err)
		}

		m, err := svc.Get(ctx, created.ID)
		if err != nil {
			t.Fatalf("回读区域失败: %v", err)
		}
		if m.Name != "厂区B" {
			t.Errorf("name = %q, 期望 厂区B", m.Name)
		}
		if m.Code != "R-B" {
			t.Errorf("code = %q, 期望 R-B", m.Code)
		}
		if m.Sequence != 20 {
			t.Errorf("sequence = %d, 期望 20", m.Sequence)
		}
		if m.Remark != "已调整" {
			t.Errorf("remark = %q, 期望 已调整", m.Remark)
		}
		if m.Active == nil || *m.Active {
			t.Errorf("active 应为 false, got %v", m.Active)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q, 期望 %q", m.UpdatedBy, testUserName)
		}
		// ⚠️ 这两个时间戳来自**不同的时钟**，不能做微秒级先后比较：
		//   created_dt 由 PostgreSQL 的 DEFAULT CURRENT_TIMESTAMP 赋值（库的时钟）
		//   updated_dt 由 Go 的 autoUpdateTime 赋值（应用的时钟）
		// 容器化环境下二者分处不同机器（PG 在 Docker 的 Linux VM、测试在 Windows 宿主机），
		// 实测出现过 updated_dt 比 created_dt 早 81µs 的抖动，约 1/3 概率误报。
		// 这里给 1 秒容差 —— 断言的**本意**是「更新确实刷新了 updated_dt」，
		// 而不是断言两个时钟的微秒级先后。
		const clockSkewTolerance = time.Second
		if m.UpdatedDT.Before(m.CreatedDT.Add(-clockSkewTolerance)) {
			t.Errorf("updated_dt(%v) 比 created_dt(%v) 早了超过 %v，更新未刷新时间戳",
				m.UpdatedDT, m.CreatedDT, clockSkewTolerance)
		}
		if m.CreatedBy != origCreatedBy {
			t.Errorf("created_by 被改动: %q -> %q", origCreatedBy, m.CreatedBy)
		}
		if !m.CreatedDT.Equal(origCreatedDT) {
			t.Errorf("created_dt 被改动: %v -> %v", origCreatedDT, m.CreatedDT)
		}

		// sequence 传 0 表示不修改
		if err := svc.Update(ctx, created.ID, &request.UpdateRegionReq{Name: "厂区C", Code: "R-C"}); err != nil {
			t.Fatalf("二次更新失败: %v", err)
		}
		m2, _ := svc.Get(ctx, created.ID)
		if m2.Sequence != 20 {
			t.Errorf("sequence 传 0 时不应被覆盖, got %d", m2.Sequence)
		}
		if m2.Active == nil || *m2.Active {
			t.Error("active 传 nil 时不应被覆盖")
		}
	})
}

// TestRegionService_Delete 区域软删除后列表与详情均不可见。
func TestRegionService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		created, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-待删", Code: "ZZQ_DEL"})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}

		before, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}, dsAll())
		if err != nil {
			t.Fatalf("删除前分页失败: %v", err)
		}

		if err := svc.Delete(ctx, created.ID); err != nil {
			t.Fatalf("删除区域失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_region", created.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		if _, err := svc.Get(ctx, created.ID); err == nil {
			t.Error("软删除后 Get 应返回错误")
		}
		after, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}, dsAll())
		if err != nil {
			t.Fatalf("删除后分页失败: %v", err)
		}
		if after.Total != before.Total-1 {
			t.Errorf("删除后 total = %d, 期望 %d", after.Total, before.Total-1)
		}
		for _, it := range after.Items {
			if it.ID == created.ID {
				t.Error("软删除的区域不应出现在列表中")
			}
		}
	})
}

// TestRegionService_Delete_Referenced 被设备引用的区域删除被拒绝（按实际实现：阻断）。
func TestRegionService_Delete_Referenced(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		region, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-被引用", Code: "ZZQ_REF"})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		dev := &model.Device{
			Name: "ZZQ-设备", DeviceCode: "ZZQ-REF-DEV", RegionID: int64Ptr(region.ID), Status: model.DeviceStatusOffline,
		}
		if err := tx.Create(dev).Error; err != nil {
			t.Fatalf("创建设备失败: %v", err)
		}

		err = svc.Delete(ctx, region.ID)
		if err == nil {
			t.Fatal("区域被设备引用时应拒绝删除")
		}
		if !strings.Contains(err.Error(), "设备") {
			t.Errorf("错误信息应说明设备引用: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_region", region.ID); dt != nil {
			t.Error("拒绝删除时 deleted_at 应仍为空")
		}
		if countRows(t, tx, "ac_device", "id = ?", dev.ID) != 1 {
			t.Error("设备记录应保留且 region_id 不变")
		}

		// 被风险记录引用同样拒绝
		region2, _ := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-被风险引用", Code: "ZZQ_REF2"})
		risk := &model.RiskRecord{
			RiskTypeID: pickID(t, tx, "ac_risk_type", ""), DeviceID: dev.ID, DeviceCode: dev.DeviceCode,
			RegionID: int64Ptr(region2.ID), Status: model.HandleStatusPending,
		}
		if err := tx.Create(risk).Error; err != nil {
			t.Fatalf("创建风险记录失败: %v", err)
		}
		if err := svc.Delete(ctx, region2.ID); err == nil {
			t.Error("区域被风险记录引用时应拒绝删除")
		}
	})
}

// TestRegionService_Delete_Referenced_AllTables 回归测试：
// 需求文档 02-基础信息模块 §5.2 列出了区域被引用的 **9 张表**，
// 实现曾只校验 ac_device 与 ac_risk_record（2/9），导致仅被低频表引用的区域可被删除、
// 留下悬挂引用。本用例逐表构造引用并断言删除被拒绝。
func TestRegionService_Delete_Referenced_AllTables(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		deviceID := pickID(t, tx, "ac_device", "")
		riskTypeID := pickID(t, tx, "ac_risk_type", "")
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")

		// 每张引用表一个独立区域，逐个验证「有引用即拒绝」
		cases := []struct {
			name    string
			fixture func(regionID int64) error
		}{
			{"ac_alarm_record", func(rid int64) error {
				return tx.Create(&model.AlarmRecord{
					AlarmTypeID: alarmTypeID, DeviceID: deviceID, DeviceCode: "ZZQ-REF",
					RegionID: int64Ptr(rid), Status: model.HandleStatusPending,
				}).Error
			}},
			{"ac_inspect_task", func(rid int64) error {
				return tx.Create(&model.InspectTask{
					Name: "ZZQ-巡检任务", RegionID: int64Ptr(rid), InspectorID: 1,
					Status: model.InspectStatusPublished, PlanDT: time.Now(),
				}).Error
			}},
			{"ac_inspect_record", func(rid int64) error {
				return tx.Create(&model.InspectRecord{
					TaskName: "ZZQ-巡检记录", RegionID: int64Ptr(rid), InspectorID: 1,
					Status: model.InspectStatusFinished, PlanDT: time.Now(),
				}).Error
			}},
			{"ac_iot_card", func(rid int64) error {
				return tx.Create(&model.IotCard{
					Name: "ZZQ-物联网卡", CardNo: "ZZQ-REF-CARD-" + strconv.FormatInt(rid, 10),
					RegionID: int64Ptr(rid), Status: model.IotCardStatusNormal,
				}).Error
			}},
			{"ac_voice_config", func(rid int64) error {
				return tx.Create(&model.VoiceConfig{
					Name: "ZZQ-语音", RegionID: int64Ptr(rid), RiskTypeID: int64Ptr(riskTypeID),
				}).Error
			}},
			{"ac_device_upgrade_plan", func(rid int64) error {
				return tx.Create(&model.DeviceUpgradePlan{
					DeviceID: deviceID, DeviceCode: "ZZQ-REF", RegionID: int64Ptr(rid),
					PlanDT: time.Now().Add(time.Hour), Status: model.UpgradePlanPending,
				}).Error
			}},
			{"ac_device_upgrade_record", func(rid int64) error {
				return tx.Create(&model.DeviceUpgradeRecord{
					DeviceID: deviceID, DeviceCode: "ZZQ-REF", RegionID: int64Ptr(rid),
					UpgradeDT: time.Now(), Status: model.UpgradePlanDone,
				}).Error
			}},
		}

		for _, c := range cases {
			region, err := svc.Create(ctx, &request.CreateRegionReq{
				Name: "ZZQ-" + c.name, Code: "ZZQ_REF_" + c.name,
			})
			if err != nil {
				t.Fatalf("[%s] 创建区域失败: %v", c.name, err)
			}
			if err := c.fixture(region.ID); err != nil {
				t.Fatalf("[%s] 构造引用失败: %v", c.name, err)
			}

			if err := svc.Delete(ctx, region.ID); err == nil {
				t.Errorf("[%s] 区域被引用时应拒绝删除（%s 未纳入引用校验）", c.name, c.name)
			}
			if dt := softDeletedAt(t, tx, "ac_region", region.ID); dt != nil {
				t.Errorf("[%s] 拒绝删除时 deleted_at 应仍为空", c.name)
			}
		}
	})
}

// TestRegionService_Page 分页元信息回显，items 长度 = min(page_size, total)。
func TestRegionService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		for i := 0; i < 3; i++ {
			if _, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-分页", Code: "ZZQ_PAGE"}); err != nil {
				t.Fatalf("创建区域失败: %v", err)
			}
		}

		resp, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 20}}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Page != 1 || resp.PageSize != 20 {
			t.Errorf("分页回显 page=%d page_size=%d", resp.Page, resp.PageSize)
		}
		if resp.Total < 3 {
			t.Errorf("total = %d, 期望 >= 3", resp.Total)
		}
		want := int(resp.Total)
		if want > 20 {
			want = 20
		}
		if len(resp.Items) != want {
			t.Errorf("items 长度 = %d, 期望 %d", len(resp.Items), want)
		}
	})
}

// TestRegionService_Page_Filters keyword 同时模糊匹配名称与编码；无匹配时 total=0 且 items 非 nil。
func TestRegionService_Page_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		if _, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-东区厂房", Code: "ZZQ_FILTER"}); err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}

		byName, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-东区"}, dsAll())
		if err != nil {
			t.Fatalf("按名称分页失败: %v", err)
		}
		if byName.Total != 1 {
			t.Errorf("按名称过滤 total = %d, 期望 1", byName.Total)
		}

		byCode, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ_FILTER"}, dsAll())
		if err != nil {
			t.Fatalf("按编码分页失败: %v", err)
		}
		if byCode.Total != 1 {
			t.Errorf("按编码过滤 total = %d, 期望 1", byCode.Total)
		}

		none, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-NOT-EXIST"}, dsAll())
		if err != nil {
			t.Fatalf("无匹配分页失败: %v", err)
		}
		if none.Total != 0 || len(none.Items) != 0 {
			t.Errorf("无匹配时 total=%d items=%d", none.Total, len(none.Items))
		}
	})
}

// TestRegionService_Page_Default 分页参数归一化：page<=0 → 1，page_size<=0 → 20，超上限 → 200。
func TestRegionService_Page_Default(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		p, err := svc.Page(ctx, &request.PageRegionReq{}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if p.Page != 1 || p.PageSize != 20 {
			t.Errorf("缺省分页 page=%d page_size=%d, 期望 1/20", p.Page, p.PageSize)
		}

		for _, in := range []int{0, -1} {
			p, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: in, PageSize: 0}}, dsAll())
			if err != nil {
				t.Fatalf("分页失败: %v", err)
			}
			if p.Page != 1 {
				t.Errorf("page=%d 应归一为 1", in)
			}
			if p.PageSize != 20 {
				t.Errorf("page_size=0 应归一为 20, got %d", p.PageSize)
			}
		}

		big, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 1000}}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if big.PageSize != 200 {
			t.Errorf("page_size=1000 应截断为 200, got %d", big.PageSize)
		}

		// page 超出总页数：items 空但 total 不变
		over, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 9999, PageSize: 20}}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if len(over.Items) != 0 {
			t.Errorf("超出总页数时 items 应为空, got %d", len(over.Items))
		}
		if over.Total != big.Total {
			t.Errorf("超出总页数时 total 不应变化: %d vs %d", over.Total, big.Total)
		}
	})
}

// TestRegionService_Page_Order 排序按 sequence 升序，sequence 相同时按 id 升序稳定。
func TestRegionService_Page_Order(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		// 先清理同前缀的残留数据，保证本用例独立可重复
		if err := tx.Unscoped().Where("name LIKE ?", "ZZQ-ORD-%").Delete(&model.Region{}).Error; err != nil {
			t.Fatalf("清理残留失败: %v", err)
		}
		a, _ := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-ORD-A", Sequence: 50})
		b, _ := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-ORD-B", Sequence: 50})
		c, _ := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-ORD-C", Sequence: 50})
		d, _ := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-ORD-D", Sequence: 5})

		resp, err := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-ORD-"}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 4 {
			t.Fatalf("total = %d, 期望 4", resp.Total)
		}
		want := []int64{d.ID, a.ID, b.ID, c.ID}
		for i, id := range want {
			if resp.Items[i].ID != id {
				t.Errorf("第 %d 项 id = %d, 期望 %d（sequence 升序 + id 升序）", i, resp.Items[i].ID, id)
			}
		}
		// 重复执行顺序一致
		resp2, _ := svc.Page(ctx, &request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-ORD-"}, dsAll())
		for i := range want {
			if resp2.Items[i].ID != resp.Items[i].ID {
				t.Errorf("重复分页顺序不一致: 第 %d 项 %d vs %d", i, resp2.Items[i].ID, resp.Items[i].ID)
			}
		}
	})
}

// TestRegionService_Options 下拉选项为简单数组，只返回启用区域。
func TestRegionService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		if _, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-启用", Code: "ZZQ_OPT_ON", Active: boolPtr(true)}); err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		off, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-停用", Code: "ZZQ_OPT_OFF", Active: boolPtr(false)})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}

		items, err := svc.Options(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}
		if len(items) == 0 {
			t.Fatal("选项不应为空")
		}
		var foundOn, foundOff bool
		for _, it := range items {
			if it.ID <= 0 || it.Name == "" {
				t.Errorf("选项字段不完整: %+v", it)
			}
			if it.ID == off.ID {
				foundOff = true
			}
			if it.Code == "ZZQ_OPT_ON" {
				foundOn = true
				if it.Name != "ZZQ-启用" {
					t.Errorf("选项 name = %q", it.Name)
				}
			}
		}
		if !foundOn {
			t.Error("启用区域应出现在选项中")
		}
		if foundOff {
			t.Error("停用区域不应出现在选项中")
		}
	})
}

// TestRegionService_Options_Active 停用区域在 Options 中不可见（独立用例，对应文档「只返回 active=true」）。
func TestRegionService_Options_Active(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		off, err := svc.Create(ctx, &request.CreateRegionReq{Name: "ZZQ-仅停用", Code: "ZZQ_OFF_ONLY", Active: boolPtr(false)})
		if err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}
		items, err := svc.Options(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}
		for _, it := range items {
			if it.ID == off.ID {
				t.Error("active=false 的区域不应出现在选项中")
			}
		}
	})
}

// TestRegionService_Create_CodeTooLong 超长 code 被数据库拒绝（列宽 50）。
func TestRegionService_Create_CodeTooLong(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRegionService()

		// 该用例触发数据库层约束错误，事务会进入 aborted 状态，故只做单次断言。
		_, err := svc.Create(ctx, &request.CreateRegionReq{
			Name: "ZZQ-超长编码",
			Code: strings.Repeat("C", 51),
		})
		if err == nil {
			t.Fatal("code 超长（51 > 50）应被拒绝")
		}
	})
}

// TestRegionService_Page_DataScope 数据权限四级：可见行数 6/4/2/1，且集合逐级包含。
func TestRegionService_Page_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := basic.NewRegionService()
		seedDataScopeRegions(t, ctx, tx)

		deptA := int64(9001)
		q := request.PageRegionReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}

		all, err := svc.Page(ctx, &q, dsAll())
		if err != nil {
			t.Fatalf("all 分页失败: %v", err)
		}
		cross, err := svc.Page(ctx, &q, dsPerm("cross_dept", "ds_user_b", &deptA, "9002"))
		if err != nil {
			t.Fatalf("cross_dept 分页失败: %v", err)
		}
		dept, err := svc.Page(ctx, &q, dsPerm("dept", "ds_user_b", &deptA, ""))
		if err != nil {
			t.Fatalf("dept 分页失败: %v", err)
		}
		personal, err := svc.Page(ctx, &q, dsPerm("personal", "ds_user_a", &deptA, ""))
		if err != nil {
			t.Fatalf("personal 分页失败: %v", err)
		}

		if cross.Total != 4 {
			t.Errorf("cross_dept(9001,9002) total = %d, 期望 4", cross.Total)
		}
		if dept.Total != 2 {
			t.Errorf("dept(9001) total = %d, 期望 2", dept.Total)
		}
		if personal.Total != 1 {
			t.Errorf("personal(9001 + ds_user_a) total = %d, 期望 1", personal.Total)
		}
		if all.Total < cross.Total || cross.Total < dept.Total || dept.Total < personal.Total {
			t.Errorf("可见行数应满足 personal <= dept <= cross_dept <= all: %d/%d/%d/%d",
				personal.Total, dept.Total, cross.Total, all.Total)
		}

		allIDs := regionIDs(all.Items)
		crossIDs := regionIDs(cross.Items)
		deptIDs := regionIDs(dept.Items)
		personalIDs := regionIDs(personal.Items)
		for id := range personalIDs {
			if !deptIDs[id] {
				t.Errorf("personal 的 %d 不在 dept 集合内", id)
			}
		}
		for id := range deptIDs {
			if !crossIDs[id] {
				t.Errorf("dept 的 %d 不在 cross_dept 集合内", id)
			}
		}
		for id := range crossIDs {
			if !allIDs[id] {
				t.Errorf("cross_dept 的 %d 不在 all 集合内", id)
			}
		}

		// 其它部门（9003）在 dept 权限下不可见
		for _, it := range dept.Items {
			if it.DeptID == nil || *it.DeptID != deptA {
				t.Errorf("dept 权限下出现非本部门行: dept_id=%v", it.DeptID)
			}
		}
	})
}

// ── RiskTypeService ───────────────────────────────────────────

// TestRiskTypeService_Create 创建风险类型：全字段落库。
func TestRiskTypeService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRiskTypeService()

		got, err := svc.Create(ctx, &request.CreateRiskTypeReq{
			Name: "ZZQ-风险类型", Code: "ZZQ_RT", Level: model.LevelHigh, Sequence: 20,
			Active: boolPtr(true), Description: "识别未佩戴安全帽行为",
		})
		if err != nil {
			t.Fatalf("创建风险类型失败: %v", err)
		}
		m, err := svc.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.Name != "ZZQ-风险类型" || m.Code != "ZZQ_RT" || m.Level != model.LevelHigh ||
			m.Sequence != 20 || m.Description != "识别未佩戴安全帽行为" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.Active == nil || !*m.Active {
			t.Errorf("active = %v", m.Active)
		}
		if m.CreatedBy != testUserName || m.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", m.CreatedBy, m.CreatedDT)
		}
		if m.DeptID == nil || *m.DeptID != testDeptID {
			t.Errorf("dept_id = %v", m.DeptID)
		}
	})
}

// TestRiskTypeService_Create_Level 等级枚举：high/medium/low 原样落库，缺省 medium。
// 注：Service 层不做枚举校验（由 Handler 的 binding 标签承担），非法值会被原样保存。
func TestRiskTypeService_Create_Level(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRiskTypeService()

		for _, lv := range []string{model.LevelHigh, model.LevelMedium, model.LevelLow} {
			got, err := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-等级" + lv, Level: lv})
			if err != nil {
				t.Fatalf("创建 level=%s 失败: %v", lv, err)
			}
			m, _ := svc.Get(ctx, got.ID)
			if m.Level != lv {
				t.Errorf("level = %q, 期望 %q", m.Level, lv)
			}
		}

		def, err := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-默认等级"})
		if err != nil {
			t.Fatalf("创建缺省等级失败: %v", err)
		}
		if def.Level != model.LevelMedium {
			t.Errorf("缺省 level = %q, 期望 medium", def.Level)
		}
	})
}

// TestRiskTypeService_Get 详情逐字段核对。
func TestRiskTypeService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRiskTypeService()

		got, _ := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-RT详情", Code: "ZZQ_RTG", Level: model.LevelLow, Sequence: 3, Description: "d"})
		m, err := svc.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if m.ID != got.ID || m.Name != "ZZQ-RT详情" || m.Code != "ZZQ_RTG" || m.Level != model.LevelLow ||
			m.Sequence != 3 || m.Description != "d" || m.Active == nil || !*m.Active {
			t.Errorf("字段不一致: %+v", m)
		}
		if _, err := svc.Get(ctx, 999999999); err == nil {
			t.Error("不存在的风险类型应返回错误")
		}
	})
}

// TestRiskTypeService_Update 6 字段全部更新且审计字段刷新。
func TestRiskTypeService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRiskTypeService()

		got, _ := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-RT旧", Code: "ZZQ_RTOLD", Level: model.LevelLow})
		if err := svc.Update(ctx, got.ID, &request.UpdateRiskTypeReq{
			Name: "ZZQ-RT新", Code: "ZZQ_RTNEW", Level: model.LevelHigh, Sequence: 9,
			Active: boolPtr(false), Description: "更新描述",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}
		m, _ := svc.Get(ctx, got.ID)
		if m.Name != "ZZQ-RT新" || m.Code != "ZZQ_RTNEW" || m.Level != model.LevelHigh ||
			m.Sequence != 9 || m.Description != "更新描述" {
			t.Errorf("字段未全部更新: %+v", m)
		}
		if m.Active == nil || *m.Active {
			t.Errorf("active 应为 false, got %v", m.Active)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}
		if m.CreatedBy != testUserName {
			t.Errorf("created_by 被改动: %q", m.CreatedBy)
		}
	})
}

// TestRiskTypeService_Delete 软删除：Page/Options 不再返回，历史记录引用保留。
func TestRiskTypeService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRiskTypeService()

		got, _ := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-RT待删", Code: "ZZQ_RTDEL"})
		dev := &model.Device{Name: "ZZQ-RT设备", DeviceCode: "ZZQ-RT-DEV", Status: model.DeviceStatusOffline}
		if err := tx.Create(dev).Error; err != nil {
			t.Fatalf("创建设备失败: %v", err)
		}
		risk := &model.RiskRecord{RiskTypeID: got.ID, DeviceID: dev.ID, DeviceCode: dev.DeviceCode, Status: model.HandleStatusPending}
		if err := tx.Create(risk).Error; err != nil {
			t.Fatalf("创建风险失败: %v", err)
		}

		// 被风险记录引用时拒绝删除
		if err := svc.Delete(ctx, got.ID); err == nil {
			t.Fatal("被风险记录引用的风险类型应拒绝删除")
		}

		// 清理引用后可删除
		if err := tx.Unscoped().Where("id = ?", risk.ID).Delete(&model.RiskRecord{}).Error; err != nil {
			t.Fatalf("清理风险记录失败: %v", err)
		}
		if err := svc.Delete(ctx, got.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_risk_type", got.ID); dt == nil {
			t.Error("deleted_at 应非空")
		}
		if _, err := svc.Get(ctx, got.ID); err == nil {
			t.Error("软删除后不应可读")
		}
		opts, _ := svc.Options(ctx, dsAll())
		for _, it := range opts {
			if it.ID == got.ID {
				t.Error("软删除后不应出现在选项中")
			}
		}
	})
}

// TestRiskTypeService_Page 分页与 level 过滤。
func TestRiskTypeService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRiskTypeService()

		if _, err := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-分页风险", Code: "ZZQ_RTPAGE", Level: model.LevelLow, Description: "完整描述"}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		resp, err := svc.Page(ctx, &request.PageRiskTypeReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-分页风险"}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 1 || len(resp.Items) != 1 {
			t.Fatalf("total=%d items=%d", resp.Total, len(resp.Items))
		}
		if resp.Items[0].Level != model.LevelLow || resp.Items[0].Description != "完整描述" {
			t.Errorf("items 字段不完整: %+v", resp.Items[0])
		}

		byLevel, err := svc.Page(ctx, &request.PageRiskTypeReq{PageReq: request.PageReq{Page: 1, PageSize: 200}, Level: model.LevelLow}, dsAll())
		if err != nil {
			t.Fatalf("按等级分页失败: %v", err)
		}
		for _, it := range byLevel.Items {
			if it.Level != model.LevelLow {
				t.Errorf("level 过滤失效: %+v", it)
			}
		}
	})
}

// TestRiskTypeService_Options 下拉选项只返回启用项且受数据权限过滤。
func TestRiskTypeService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewRiskTypeService()

		on, _ := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-RT启用", Code: "ZZQ_RTON", Active: boolPtr(true)})
		off, _ := svc.Create(ctx, &request.CreateRiskTypeReq{Name: "ZZQ-RT停用", Code: "ZZQ_RTOFF", Active: boolPtr(false)})

		items, err := svc.Options(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}
		seen := map[int64]bool{}
		for _, it := range items {
			seen[it.ID] = true
			if it.ID <= 0 || it.Name == "" {
				t.Errorf("选项字段不完整: %+v", it)
			}
		}
		if !seen[on.ID] {
			t.Error("启用项应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用项不应出现在选项中")
		}
	})
}

// TestRiskTypeService_Page_DataScope 风险类型同样受四级数据权限约束。
func TestRiskTypeService_Page_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := basic.NewRiskTypeService()
		seedDataScopeRiskTypes(t, ctx, tx)

		deptA := int64(9001)
		q := request.PageRiskTypeReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}

		all, _ := svc.Page(ctx, &q, dsAll())
		cross, _ := svc.Page(ctx, &q, dsPerm("cross_dept", "ds_user_b", &deptA, "9002"))
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
	})
}

// ── AlarmTypeService ──────────────────────────────────────────

// TestAlarmTypeService_Create 创建告警类型：全字段落库。
func TestAlarmTypeService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewAlarmTypeService()

		got, err := svc.Create(ctx, &request.CreateAlarmTypeReq{
			Name: "ZZQ-自定义告警", Code: "ZZQ_AT", Level: model.LevelLow, Sequence: 12,
			Active: boolPtr(true), Description: "自定义告警说明",
		})
		if err != nil {
			t.Fatalf("创建告警类型失败: %v", err)
		}
		m, err := svc.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.Name != "ZZQ-自定义告警" || m.Code != "ZZQ_AT" || m.Level != model.LevelLow ||
			m.Sequence != 12 || m.Description != "自定义告警说明" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.Active == nil || !*m.Active || m.CreatedBy != testUserName || m.CreatedDT.IsZero() {
			t.Errorf("默认值/审计字段异常: %+v", m)
		}
		if m.DeptID == nil || *m.DeptID != testDeptID {
			t.Errorf("dept_id = %v", m.DeptID)
		}
	})
}

// TestAlarmTypeService_Create_Level 等级枚举：三值落库，缺省 medium。
func TestAlarmTypeService_Create_Level(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewAlarmTypeService()

		for _, lv := range []string{model.LevelHigh, model.LevelMedium, model.LevelLow} {
			got, err := svc.Create(ctx, &request.CreateAlarmTypeReq{Name: "ZZQ-告警等级" + lv, Level: lv})
			if err != nil {
				t.Fatalf("创建 level=%s 失败: %v", lv, err)
			}
			m, _ := svc.Get(ctx, got.ID)
			if m.Level != lv {
				t.Errorf("level = %q, 期望 %q", m.Level, lv)
			}
		}
		def, _ := svc.Create(ctx, &request.CreateAlarmTypeReq{Name: "ZZQ-告警默认等级"})
		if def.Level != model.LevelMedium {
			t.Errorf("缺省 level = %q, 期望 medium", def.Level)
		}
	})
}

// TestAlarmTypeService_Options_Builtin 内置三条告警类型（offline/battery/traffic）必须存在且字段完整。
func TestAlarmTypeService_Options_Builtin(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := basic.NewAlarmTypeService()
		items, err := svc.Options(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}

		want := map[string]bool{
			model.AlarmCodeOffline: false,
			model.AlarmCodeBattery: false,
			model.AlarmCodeTraffic: false,
		}
		for _, it := range items {
			if _, ok := want[it.Code]; ok {
				want[it.Code] = true
				if it.Name == "" {
					t.Errorf("内置告警 %s 的 name 为空", it.Code)
				}
			}
		}
		for code, found := range want {
			if !found {
				t.Errorf("内置告警类型 %s 未出现在选项中", code)
			}
		}

		// 内置项 level / active 有值
		var at model.AlarmType
		if err := tx.Where("code = ?", model.AlarmCodeOffline).First(&at).Error; err != nil {
			t.Fatalf("读取内置告警失败: %v", err)
		}
		if at.Level == "" {
			t.Error("内置告警 level 不应为空")
		}
		if at.Active == nil {
			t.Error("内置告警 active 不应为 nil")
		}
	})
}

// TestAlarmTypeService_Delete_Builtin 内置告警类型被告警设置引用时不可删除；无引用时可软删除。
func TestAlarmTypeService_Delete_Builtin(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewAlarmTypeService()

		offlineID := pickID(t, tx, "ac_alarm_type", "code = ?", model.AlarmCodeOffline)

		// 被告警设置引用 -> 拒绝
		setting := &model.AlarmSetting{UserID: testUserID, AlarmTypeID: offlineID, Active: boolPtr(true), NotifyType: model.NotifyTypeApp}
		if err := tx.Create(setting).Error; err != nil {
			t.Fatalf("创建告警设置失败: %v", err)
		}
		if err := svc.Delete(ctx, offlineID); err == nil {
			t.Fatal("被告警设置引用的告警类型应拒绝删除")
		}
		if dt := softDeletedAt(t, tx, "ac_alarm_type", offlineID); dt != nil {
			t.Error("拒绝删除时 deleted_at 应仍为空")
		}

		// 清除告警设置引用后，仍受「告警记录引用」保护
		if err := tx.Unscoped().Where("id = ?", setting.ID).Delete(&model.AlarmSetting{}).Error; err != nil {
			t.Fatalf("清理告警设置失败: %v", err)
		}
		dev := &model.Device{Name: "ZZQ-AT设备", DeviceCode: "ZZQ-BUILTIN-DEV", Status: model.DeviceStatusOffline}
		if err := tx.Create(dev).Error; err != nil {
			t.Fatalf("创建设备失败: %v", err)
		}
		alarm := &model.AlarmRecord{
			AlarmTypeID: offlineID, DeviceID: dev.ID, DeviceCode: dev.DeviceCode,
			Status: model.HandleStatusPending, OccurredDT: time.Now(),
		}
		if err := tx.Create(alarm).Error; err != nil {
			t.Fatalf("创建告警记录失败: %v", err)
		}
		err := svc.Delete(ctx, offlineID)
		if err == nil {
			t.Fatal("被告警记录引用的内置告警类型应拒绝删除（历史记录不因基础数据删除而丢失）")
		}
		if !strings.Contains(err.Error(), "告警记录") {
			t.Errorf("错误信息应说明告警记录引用: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_alarm_type", offlineID); dt != nil {
			t.Error("拒绝删除时 deleted_at 应仍为空")
		}
	})
}

// TestAlarmTypeService_Get 详情逐字段核对。
func TestAlarmTypeService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewAlarmTypeService()

		got, _ := svc.Create(ctx, &request.CreateAlarmTypeReq{Name: "ZZQ-AT详情", Code: "ZZQ_ATG", Level: model.LevelLow, Sequence: 4, Description: "x"})
		m, err := svc.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if m.ID != got.ID || m.Name != "ZZQ-AT详情" || m.Code != "ZZQ_ATG" || m.Level != model.LevelLow ||
			m.Sequence != 4 || m.Description != "x" {
			t.Errorf("字段不一致: %+v", m)
		}
		if _, err := svc.Get(ctx, 999999999); err == nil {
			t.Error("不存在的告警类型应返回错误")
		}
	})
}

// TestAlarmTypeService_Update 6 字段全部更新且审计字段刷新。
func TestAlarmTypeService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewAlarmTypeService()

		got, _ := svc.Create(ctx, &request.CreateAlarmTypeReq{Name: "ZZQ-AT旧", Code: "ZZQ_ATOLD"})
		if err := svc.Update(ctx, got.ID, &request.UpdateAlarmTypeReq{
			Name: "ZZQ-AT新", Code: "ZZQ_ATNEW", Level: model.LevelHigh, Sequence: 8,
			Active: boolPtr(false), Description: "新描述",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}
		m, _ := svc.Get(ctx, got.ID)
		if m.Name != "ZZQ-AT新" || m.Code != "ZZQ_ATNEW" || m.Level != model.LevelHigh ||
			m.Sequence != 8 || m.Description != "新描述" || m.Active == nil || *m.Active {
			t.Errorf("字段未全部更新: %+v", m)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}
	})
}

// TestAlarmTypeService_Page 分页与过滤，items 字段完整。
func TestAlarmTypeService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewAlarmTypeService()

		if _, err := svc.Create(ctx, &request.CreateAlarmTypeReq{Name: "ZZQ-分页告警", Code: "ZZQ_ATPAGE", Level: model.LevelLow, Description: "告警描述"}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		resp, err := svc.Page(ctx, &request.PageAlarmTypeReq{PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-分页告警"}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 1 || len(resp.Items) != 1 {
			t.Fatalf("total=%d items=%d", resp.Total, len(resp.Items))
		}
		if resp.Items[0].Level != model.LevelLow || resp.Items[0].Description != "告警描述" {
			t.Errorf("字段不完整: %+v", resp.Items[0])
		}

		byLevel, err := svc.Page(ctx, &request.PageAlarmTypeReq{PageReq: request.PageReq{Page: 1, PageSize: 200}, Level: model.LevelHigh}, dsAll())
		if err != nil {
			t.Fatalf("按等级分页失败: %v", err)
		}
		for _, it := range byLevel.Items {
			if it.Level != model.LevelHigh {
				t.Errorf("level 过滤失效: %+v", it)
			}
		}
	})
}

// TestAlarmTypeService_Delete 自定义告警类型软删除后不可见，历史告警记录引用保留。
func TestAlarmTypeService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := basic.NewAlarmTypeService()

		got, _ := svc.Create(ctx, &request.CreateAlarmTypeReq{Name: "ZZQ-AT待删", Code: "ZZQ_ATDEL"})
		dev := &model.Device{Name: "ZZQ-AT设备", DeviceCode: "ZZQ-AT-DEV", Status: model.DeviceStatusOffline}
		if err := tx.Create(dev).Error; err != nil {
			t.Fatalf("创建设备失败: %v", err)
		}
		alarm := &model.AlarmRecord{AlarmTypeID: got.ID, DeviceID: dev.ID, DeviceCode: dev.DeviceCode, Status: model.HandleStatusPending}
		if err := tx.Create(alarm).Error; err != nil {
			t.Fatalf("创建告警失败: %v", err)
		}

		if err := svc.Delete(ctx, got.ID); err == nil {
			t.Fatal("被告警记录引用的告警类型应拒绝删除")
		}
		if err := tx.Unscoped().Where("id = ?", alarm.ID).Delete(&model.AlarmRecord{}).Error; err != nil {
			t.Fatalf("清理告警失败: %v", err)
		}
		if err := svc.Delete(ctx, got.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_alarm_type", got.ID); dt == nil {
			t.Error("deleted_at 应非空")
		}
		// 历史告警记录的 alarm_type_id 保留（此处已清理，改校验其他表不级联）
		if countRows(t, tx, "ac_device", "id = ?", dev.ID) != 1 {
			t.Error("设备记录不应受影响")
		}
	})
}

// ── DataScope（utils） ────────────────────────────────────────

// TestDataScope_ApplyToQuery_All all 权限不附加任何过滤。
func TestDataScope_ApplyToQuery_All(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		seedDataScopeRegions(t, ctx, tx)

		var total, filtered int64
		if err := bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL").Count(&total).Error; err != nil {
			t.Fatalf("统计全表失败: %v", err)
		}
		ds := &utils.DataScope{DataPerm: "all", UserName: "ds_user_a"}
		q := ds.ApplyToQuery(bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL"), "ac_region.dept_id")
		if err := q.Count(&filtered).Error; err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		if filtered != total {
			t.Errorf("all 权限应返回全表: %d vs %d", filtered, total)
		}
	})
}

// TestDataScope_ApplyToQuery_NoDept dept_id 为空时退化为 created_by 过滤（不得返回全表）。
func TestDataScope_ApplyToQuery_NoDept(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		seedDataScopeRegions(t, ctx, tx)

		var total int64
		if err := bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL").Count(&total).Error; err != nil {
			t.Fatalf("统计全表失败: %v", err)
		}
		var mine int64
		if err := bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL AND created_by = ?", "ds_user_a").Count(&mine).Error; err != nil {
			t.Fatalf("统计本人数据失败: %v", err)
		}

		for _, perm := range []string{"dept", "personal"} {
			ds := &utils.DataScope{DataPerm: perm, UserName: "ds_user_a"}
			var n int64
			q := ds.ApplyToQuery(bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL"), "ac_region.dept_id")
			if err := q.Count(&n).Error; err != nil {
				t.Fatalf("%s 统计失败: %v", perm, err)
			}
			if n != mine {
				t.Errorf("%s 且无 dept_id 时应退化为 created_by 过滤: %d, 期望 %d", perm, n, mine)
			}
			if n == total {
				t.Errorf("%s 且无 dept_id 时不得返回全表", perm)
			}
		}
	})
}

// TestDataScope_ApplyToQuery_EmptyCrossDept cross_dept 且无任何部门 ID 时退化为 created_by。
func TestDataScope_ApplyToQuery_EmptyCrossDept(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		seedDataScopeRegions(t, ctx, tx)

		var total, mine int64
		_ = bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL").Count(&total).Error
		_ = bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL AND created_by = ?", "ds_user_a").Count(&mine).Error

		ds := &utils.DataScope{DataPerm: "cross_dept", UserName: "ds_user_a", CrossDeptIDs: ""}
		var n int64
		q := ds.ApplyToQuery(bootstrap.DB().Table("ac_region").Where("deleted_at IS NULL"), "ac_region.dept_id")
		if err := q.Count(&n).Error; err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		if n != mine {
			t.Errorf("cross_dept 无跨部门ID时应退化为 created_by: %d, 期望 %d", n, mine)
		}
		if n == total {
			t.Error("cross_dept 无跨部门ID时不得返回全表（最危险的越权路径）")
		}
	})
}

// TestDataScope_ParseIDs 逗号分隔 ID 解析：忽略空项与非法项。
func TestDataScope_ParseIDs(t *testing.T) {
	got := utils.ParseIDs("2, 3,,abc,5")
	want := []int64{2, 3, 5}
	if len(got) != len(want) {
		t.Fatalf("ParseIDs = %v, 期望 %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ParseIDs[%d] = %d, 期望 %d", i, got[i], want[i])
		}
	}
	if len(utils.ParseIDs("")) != 0 {
		t.Error("空串应返回空集合")
	}
	if len(utils.ParseIDs(" , ")) != 0 {
		t.Error("全空白应返回空集合")
	}
}

// TestDataScope_ApplyToQuery_Alias 带表别名时 created_by 列名应正确推导，不产生歧义错误。
func TestDataScope_ApplyToQuery_Alias(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		seedDataScopeRegions(t, ctx, tx)

		ds := &utils.DataScope{DataPerm: "personal", UserName: "ds_user_a", DeptID: int64Ptr(9001)}
		var n int64
		q := ds.ApplyToQuery(
			bootstrap.DB().Table("ac_region AS rg").Where("rg.deleted_at IS NULL"),
			"rg.dept_id",
		)
		if err := q.Count(&n).Error; err != nil {
			t.Fatalf("带别名的数据权限查询应成功（列名前缀需正确推导）: %v", err)
		}
		if n != 1 {
			t.Errorf("personal 可见行数 = %d, 期望 1", n)
		}
	})
}

// ── 用例内部工具 ──────────────────────────────────────────────

// seedDataScopeRegions 预置 3 个部门各 2 条区域（其中仅 1 条由 ds_user_a 创建）。
// 使用事务内物理删除保证可重复执行。
func seedDataScopeRegions(t *testing.T, ctx context.Context, tx *gorm.DB) {
	t.Helper()
	depts := []int64{9001, 9002, 9003}
	if err := tx.Unscoped().Where("dept_id IN ?", depts).Delete(&model.Region{}).Error; err != nil {
		t.Fatalf("清理数据权限用例数据失败: %v", err)
	}
	type row struct {
		dept    int64
		creator string
		suffix  string
	}
	rows := []row{
		{9001, "ds_user_a", "A1"},
		{9001, "ds_user_b", "A2"},
		{9002, "ds_user_b", "B1"},
		{9002, "ds_user_b", "B2"},
		{9003, "ds_user_c", "C1"},
		{9003, "ds_user_c", "C2"},
	}
	for _, r := range rows {
		m := &model.Region{
			Name:     "ZZQ-DS-" + r.suffix,
			Code:     "ZZQ_DS_" + r.suffix,
			Sequence: 10,
			Active:   boolPtr(true),
			DeptID:   int64Ptr(r.dept),
			BaseModel: model.BaseModel{
				CreatedBy: r.creator,
				UpdatedBy: r.creator,
			},
		}
		if err := tx.Create(m).Error; err != nil {
			t.Fatalf("预置数据权限数据失败: %v", err)
		}
	}
}

// seedDataScopeRiskTypes 与 seedDataScopeRegions 同构，作用于 ac_risk_type。
func seedDataScopeRiskTypes(t *testing.T, ctx context.Context, tx *gorm.DB) {
	t.Helper()
	depts := []int64{9001, 9002, 9003}
	if err := tx.Unscoped().Where("dept_id IN ?", depts).Delete(&model.RiskType{}).Error; err != nil {
		t.Fatalf("清理数据权限用例数据失败: %v", err)
	}
	specs := []struct {
		dept    int64
		creator string
		suffix  string
	}{
		{9001, "ds_user_a", "A1"},
		{9001, "ds_user_b", "A2"},
		{9002, "ds_user_b", "B1"},
		{9002, "ds_user_b", "B2"},
		{9003, "ds_user_c", "C1"},
		{9003, "ds_user_c", "C2"},
	}
	for _, s := range specs {
		m := &model.RiskType{
			Name: "ZZQ-DSRT-" + s.suffix, Code: "ZZQ_DSRT_" + s.suffix,
			Level: model.LevelMedium, Sequence: 10, Active: boolPtr(true),
			DeptID:    int64Ptr(s.dept),
			BaseModel: model.BaseModel{CreatedBy: s.creator, UpdatedBy: s.creator},
		}
		if err := tx.Create(m).Error; err != nil {
			t.Fatalf("预置数据权限数据失败: %v", err)
		}
	}
}

// regionIDs 把区域列表转成 ID 集合。
func regionIDs(items []model.Region) map[int64]bool {
	out := make(map[int64]bool, len(items))
	for _, it := range items {
		out[it.ID] = true
	}
	return out
}
