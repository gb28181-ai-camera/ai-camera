package device

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 设备模式（DeviceModeService）/ 物联网卡（IotCardService）/
// 语音配置（VoiceConfigService）
// ════════════════════════════════════════════════════════════════

// ── DeviceModeService ─────────────────────────────────────────

// TestDeviceModeService_Create 创建设备模式：全字段落库 + 名称唯一。
func TestDeviceModeService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		modes := newDeviceModeService()

		name := uniq("ZZQ-模式")
		got, err := modes.Create(ctx, &request.CreateDeviceModeReq{
			Name: name, Code: "zzq_mode", Sequence: 6, Remark: "测试模式", Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建设备模式失败: %v", err)
		}
		if got.Name != name || got.Code != "zzq_mode" || got.Sequence != 6 || got.Remark != "测试模式" {
			t.Errorf("字段不一致: %+v", got)
		}
		if got.Active == nil || !*got.Active || got.CreatedBy != testUserName || got.CreatedDT.IsZero() {
			t.Errorf("默认值/审计字段异常: %+v", got)
		}
		if got.DeptID == nil || *got.DeptID != testDeptID {
			t.Errorf("dept_id = %v", got.DeptID)
		}

		// 重复名称被拒绝
		if _, err := modes.Create(ctx, &request.CreateDeviceModeReq{Name: name}); err == nil {
			t.Error("重复模式名称应返回错误")
		}
		// 名称为空白被拒绝
		if _, err := modes.Create(ctx, &request.CreateDeviceModeReq{Name: "   "}); err == nil {
			t.Error("名称为空白应返回错误")
		}
		// 缺省 sequence=10
		def, _ := modes.Create(ctx, &request.CreateDeviceModeReq{Name: uniq("ZZQ-默认模式")})
		if def.Sequence != 10 {
			t.Errorf("默认 sequence = %d, 期望 10", def.Sequence)
		}
	})
}

// TestDeviceModeService_Options_Builtin 内置三种模式（充电/全开/省电）必须存在。
func TestDeviceModeService_Options_Builtin(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		modes := newDeviceModeService()

		items, err := modes.Options(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}
		want := map[string]bool{"充电模式": false, "全开模式": false, "省电模式": false}
		for _, it := range items {
			if _, ok := want[it.Name]; ok {
				want[it.Name] = true
			}
		}
		for name, found := range want {
			if !found {
				t.Errorf("内置模式 %s 未出现在选项中", name)
			}
		}

		// SwitchMode 的入参与模式名称一致
		d := mustDevice(t, ctx, uniq("ZZQ-BUILTIN-MODE"), nil)
		if err := newDeviceService().SwitchMode(ctx, d.ID, "全开模式"); err != nil {
			t.Errorf("以内置模式名切换应成功: %v", err)
		}
	})
}

// TestDeviceModeService_Page 设备模式分页与关键字过滤。
func TestDeviceModeService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		modes := newDeviceModeService()

		keyword := uniq("ZZQ-PAGE-MODE")
		if _, err := modes.Create(ctx, &request.CreateDeviceModeReq{Name: keyword + "-A", Code: "a", Remark: "r"}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if _, err := modes.Create(ctx, &request.CreateDeviceModeReq{Name: keyword + "-B", Code: "b"}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		resp, err := modes.Page(ctx, &request.PageDeviceModeReq{
			PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: keyword,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 2 || len(resp.Items) != 2 {
			t.Fatalf("total=%d items=%d, 期望 2/2", resp.Total, len(resp.Items))
		}
		for _, it := range resp.Items {
			if it.Name == "" || it.Code == "" || it.Sequence == 0 || it.CreatedDT.IsZero() {
				t.Errorf("items 字段不完整: %+v", it)
			}
		}
	})
}

// TestDeviceModeService_Update 设备模式更新：字段全部生效。
func TestDeviceModeService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		modes := newDeviceModeService()

		created, err := modes.Create(ctx, &request.CreateDeviceModeReq{Name: uniq("ZZQ-旧模式"), Code: "old"})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		newName := uniq("ZZQ-新模式")
		if err := modes.Update(ctx, created.ID, &request.UpdateDeviceModeReq{
			Name: newName, Code: "new", Sequence: 21, Remark: "新备注", Active: boolPtr(false),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		var got model.DeviceMode
		if err := tx.Where("id = ?", created.ID).First(&got).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if got.Name != newName || got.Code != "new" || got.Sequence != 21 || got.Remark != "新备注" {
			t.Errorf("字段未全部更新: %+v", got)
		}
		if got.Active == nil || *got.Active {
			t.Errorf("active = %v, 期望 false", got.Active)
		}
		if got.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", got.UpdatedBy)
		}
	})
}

// TestDeviceModeService_Delete 设备模式软删除；已使用该模式的设备值保留。
func TestDeviceModeService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		modes := newDeviceModeService()
		devices := newDeviceService()

		name := uniq("ZZQ-待删模式")
		created, err := modes.Create(ctx, &request.CreateDeviceModeReq{Name: name})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		d := mustDevice(t, ctx, uniq("ZZQ-MODE-DEV"), nil)
		if err := devices.SwitchMode(ctx, d.ID, name); err != nil {
			t.Fatalf("切换模式失败: %v", err)
		}

		if err := modes.Delete(ctx, created.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_device_mode", created.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		opts, _ := modes.Options(ctx, dsAll())
		for _, it := range opts {
			if it.ID == created.ID {
				t.Error("软删除后不应出现在选项中")
			}
		}
		page, _ := modes.Page(ctx, &request.PageDeviceModeReq{
			PageReq: request.PageReq{Page: 1, PageSize: 10}, Keyword: name,
		}, dsAll())
		if page.Total != 0 {
			t.Errorf("软删除后分页 total = %d, 期望 0", page.Total)
		}
		// 已使用该模式的设备保留历史值
		m, _ := devices.Get(ctx, d.ID)
		if m.DeviceMode != name {
			t.Errorf("设备 device_mode = %q, 期望保留 %q", m.DeviceMode, name)
		}
		if err := modes.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的模式应返回错误")
		}
	})
}

// TestDeviceModeService_Options 下拉选项：简单列表且只含启用项。
func TestDeviceModeService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		modes := newDeviceModeService()

		on, _ := modes.Create(ctx, &request.CreateDeviceModeReq{Name: uniq("ZZQ-OPT-ON"), Code: "on", Active: boolPtr(true)})
		off, _ := modes.Create(ctx, &request.CreateDeviceModeReq{Name: uniq("ZZQ-OPT-OFF"), Code: "off", Active: boolPtr(false)})

		items, err := modes.Options(ctx, dsAll())
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
			t.Error("启用模式应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用模式不应出现在选项中")
		}
	})
}

// ── IotCardService ────────────────────────────────────────────

// TestIotCardService_Create 创建物联网卡：全字段落库 + 默认状态。
func TestIotCardService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		regionID := mustRegion(t, ctx, "ZZQ-卡区域")
		cardNo := uniq("8986001")
		got, err := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "移动卡A", CardNo: cardNo, RegionID: &regionID,
			TrafficTotal: 10240, TrafficUsed: 2048, TrafficLeft: 8000,
			StatDT: "2026-09-20 10:00:00", ExpireDT: "2027-09-20 10:00:00",
			Remark: "备注", Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建物联网卡失败: %v", err)
		}
		var m model.IotCard
		if err := tx.Where("id = ?", got.ID).First(&m).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.Name != "移动卡A" || m.CardNo != cardNo || m.TrafficTotal != 10240 ||
			m.TrafficUsed != 2048 || m.TrafficLeft != 8000 || m.Remark != "备注" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.RegionID == nil || *m.RegionID != regionID {
			t.Errorf("region_id = %v", m.RegionID)
		}
		if m.Status != model.IotCardStatusNormal {
			t.Errorf("status = %q, 期望 normal", m.Status)
		}
		if m.StatDT == nil || m.ExpireDT == nil {
			t.Error("stat_dt / expire_dt 应已解析")
		}
		if m.Active == nil || !*m.Active || m.CreatedBy != testUserName || m.CreatedDT.IsZero() {
			t.Errorf("默认值/审计字段异常: %+v", m)
		}
		if m.DeptID == nil || *m.DeptID != testDeptID {
			t.Errorf("dept_id = %v", m.DeptID)
		}
	})
}

// TestIotCardService_Create_DuplicateCardNo 卡号唯一。
func TestIotCardService_Create_DuplicateCardNo(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		cardNo := uniq("8986002")
		if _, err := cards.Create(ctx, &request.CreateIotCardReq{Name: "ZZQ-1", CardNo: cardNo}); err != nil {
			t.Fatalf("首次创建失败: %v", err)
		}
		if _, err := cards.Create(ctx, &request.CreateIotCardReq{Name: "ZZQ-2", CardNo: cardNo}); err == nil {
			t.Fatal("重复卡号应返回错误")
		}
		if n := countRows(t, tx, "ac_iot_card", "card_no = ?", cardNo); n != 1 {
			t.Errorf("同卡号记录数 = %d, 期望 1", n)
		}
		if _, err := cards.Create(ctx, &request.CreateIotCardReq{Name: "ZZQ-3", CardNo: "  "}); err == nil {
			t.Error("空卡号应返回错误")
		}
	})
}

// TestIotCardService_Create_Traffic 流量字段一致性：剩余流量按总额减已用补算。
func TestIotCardService_Create_Traffic(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		got, err := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-流量卡", CardNo: uniq("8986003"), TrafficTotal: 10240, TrafficUsed: 2048,
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if got.TrafficLeft != 8192 {
			t.Errorf("traffic_left = %d, 期望 8192（10240-2048）", got.TrafficLeft)
		}
		if got.TrafficTotal < 0 || got.TrafficUsed < 0 || got.TrafficLeft < 0 {
			t.Error("流量字段不应为负")
		}
	})
}

// TestIotCardService_Create_Status 卡状态按到期时间与剩余流量推导。
func TestIotCardService_Create_Status(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		normal, err := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-正常卡", CardNo: uniq("8986010"), TrafficTotal: 1000, TrafficUsed: 100,
			ExpireDT: time.Now().AddDate(1, 0, 0).Format("2006-01-02 15:04:05"),
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if normal.Status != model.IotCardStatusNormal {
			t.Errorf("未到期且有余额时 status = %q, 期望 normal", normal.Status)
		}

		expired, err := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-到期卡", CardNo: uniq("8986011"), TrafficTotal: 1000, TrafficUsed: 100,
			ExpireDT: time.Now().AddDate(-1, 0, 0).Format("2006-01-02 15:04:05"),
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if expired.Status != model.IotCardStatusExpired {
			t.Errorf("已到期时 status = %q, 期望 expired", expired.Status)
		}

		arrears, err := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-欠费卡", CardNo: uniq("8986012"), TrafficTotal: 1000, TrafficUsed: 1000,
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if arrears.Status != model.IotCardStatusArrears {
			t.Errorf("剩余流量为 0 时 status = %q, 期望 arrears", arrears.Status)
		}

		disabled, err := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-停用卡", CardNo: uniq("8986013"), TrafficTotal: 1000, TrafficUsed: 100,
			Status: model.IotCardStatusDisabled,
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if disabled.Status != model.IotCardStatusDisabled {
			t.Errorf("手动停用时 status = %q, 期望 disabled", disabled.Status)
		}
	})
}

// TestIotCardService_Page 物联网卡分页：过滤 + 关联名称回显。
func TestIotCardService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		regionID := mustRegion(t, ctx, "ZZQ-卡分页区域")
		device := mustDevice(t, ctx, uniq("ZZQ-CARD-DEV"), &regionID)
		card := mustIotCard(t, ctx, uniq("ZZQ-PAGE-CARD"), &regionID)
		if err := newDeviceService().Update(ctx, device.ID, &request.UpdateDeviceReq{Name: device.Name, IotCardID: &card.ID}); err != nil {
			t.Fatalf("绑定卡失败: %v", err)
		}

		page, err := cards.Page(ctx, &request.PageIotCardReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceID: &device.ID,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("total=%d items=%d, 期望 1/1", page.Total, len(page.Items))
		}
		it := page.Items[0]
		if it.CardNo != card.CardNo {
			t.Errorf("card_no = %q", it.CardNo)
		}
		if it.Region == nil || it.Region.Name != "ZZQ-卡分页区域" {
			t.Errorf("Region.Name 未回显: %+v", it.Region)
		}
		if it.Device == nil || it.Device.Name != device.Name || it.Device.DeviceCode != device.DeviceCode {
			t.Errorf("Device 名称/编号未回显: %+v", it.Device)
		}

		byRegion, _ := cards.Page(ctx, &request.PageIotCardReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, RegionID: &regionID,
		}, dsAll())
		if byRegion.Total != 1 {
			t.Errorf("按区域过滤命中 %d, 期望 1", byRegion.Total)
		}
		byStatus, _ := cards.Page(ctx, &request.PageIotCardReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Status: model.IotCardStatusNormal,
		}, dsAll())
		if byStatus.Total == 0 {
			t.Error("按状态过滤应命中至少 1 条")
		}
		byKeyword, _ := cards.Page(ctx, &request.PageIotCardReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: card.CardNo,
		}, dsAll())
		if byKeyword.Total != 1 {
			t.Errorf("按卡号过滤命中 %d, 期望 1", byKeyword.Total)
		}
	})
}

// TestIotCardService_Update 物联网卡更新：字段生效，剩余流量与状态重算。
func TestIotCardService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		regionA := mustRegion(t, ctx, "ZZQ-卡更新区域A")
		regionB := mustRegion(t, ctx, "ZZQ-卡更新区域B")
		device := mustDevice(t, ctx, uniq("ZZQ-CARD-UPD-DEV"), &regionA)
		// 初始 flow 为 0，便于验证「剩余流量按总额-已用补算」的分支
		created, err := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-旧卡名", CardNo: uniq("ZZQ-UPD-CARD"), RegionID: &regionA,
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if created.TrafficLeft != 0 {
			t.Fatalf("前置条件：初始 traffic_left 应为 0, got %d", created.TrafficLeft)
		}

		if err := cards.Update(ctx, created.ID, &request.UpdateIotCardReq{
			Name: "ZZQ-新卡名", RegionID: &regionB, DeviceID: &device.ID,
			TrafficTotal: 2000, TrafficUsed: 500, Remark: "已更新",
			ExpireDT: time.Now().AddDate(1, 0, 0).Format("2006-01-02 15:04:05"),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		var m model.IotCard
		if err := tx.Where("id = ?", created.ID).First(&m).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.Name != "ZZQ-新卡名" || m.TrafficTotal != 2000 || m.TrafficUsed != 500 || m.Remark != "已更新" {
			t.Errorf("字段未全部更新: %+v", m)
		}
		if m.RegionID == nil || *m.RegionID != regionB {
			t.Errorf("region_id = %v, 期望 %d", m.RegionID, regionB)
		}
		if m.DeviceID == nil || *m.DeviceID != device.ID {
			t.Errorf("device_id = %v, 期望 %d", m.DeviceID, device.ID)
		}
		if m.TrafficLeft != 1500 {
			t.Errorf("traffic_left = %d, 期望 1500（原值为 0 时按总额-已用补算）", m.TrafficLeft)
		}
		if m.ExpireDT == nil {
			t.Error("expire_dt 应已更新")
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}
		if m.CardNo != created.CardNo {
			t.Errorf("card_no 不应被改动: %q", m.CardNo)
		}
	})
}

// TestIotCardService_Delete 物联网卡软删除后不可见。
func TestIotCardService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		created := mustIotCard(t, ctx, uniq("ZZQ-DEL-CARD"), nil)
		if err := cards.Delete(ctx, created.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_iot_card", created.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		opts, _ := cards.Options(ctx, dsAll())
		for _, it := range opts {
			if it.ID == created.ID {
				t.Error("软删除后不应出现在选项中")
			}
		}
		page, _ := cards.Page(ctx, &request.PageIotCardReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: created.CardNo,
		}, dsAll())
		if page.Total != 0 {
			t.Errorf("软删除后分页 total = %d, 期望 0", page.Total)
		}
		if err := cards.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的卡应返回错误")
		}
	})
}

// TestIotCardService_ReplaceCard 换卡事务：写日志 + 设备当前卡 + 双向绑定。
func TestIotCardService_ReplaceCard(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-换卡区域")
		c1 := mustIotCard(t, ctx, uniq("ZZQ-C1"), &regionID)
		c2 := mustIotCard(t, ctx, uniq("ZZQ-C2"), &regionID)
		d := mustDevice(t, ctx, uniq("ZZQ-REPLACE"), &regionID)

		// 预置：设备当前卡为 C1
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{Name: d.Name, IotCardID: &c1.ID}); err != nil {
			t.Fatalf("绑定原卡失败: %v", err)
		}

		before := countRows(t, tx, "ac_iot_card_replace_log", "device_id = ?", d.ID)
		logRow, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{
			DeviceID: d.ID, NewIotCardID: c2.ID, Remark: "卡损坏更换",
		}, testUserName)
		if err != nil {
			t.Fatalf("换卡失败: %v", err)
		}

		if n := countRows(t, tx, "ac_iot_card_replace_log", "device_id = ?", d.ID); n != before+1 {
			t.Fatalf("换卡记录新增 %d 条, 期望 1", n-before)
		}
		if logRow.DeviceID != d.ID || logRow.DeviceCode != d.DeviceCode {
			t.Errorf("日志冗余设备字段错误: %+v", logRow)
		}
		if logRow.RegionID == nil || *logRow.RegionID != regionID || logRow.Location != d.Location {
			t.Errorf("日志冗余区域/位置错误: %v/%q", logRow.RegionID, logRow.Location)
		}
		if logRow.OldIotCardID == nil || *logRow.OldIotCardID != c1.ID {
			t.Errorf("old_iot_card_id = %v, 期望 %d", logRow.OldIotCardID, c1.ID)
		}
		if logRow.NewIotCardID == nil || *logRow.NewIotCardID != c2.ID {
			t.Errorf("new_iot_card_id = %v, 期望 %d", logRow.NewIotCardID, c2.ID)
		}
		if logRow.OldCardNo != c1.CardNo || logRow.NewCardNo != c2.CardNo {
			t.Errorf("卡号冗余错误: %q -> %q", logRow.OldCardNo, logRow.NewCardNo)
		}
		if logRow.ReplaceDT.IsZero() {
			t.Error("replace_dt 不应为零值")
		}
		if logRow.Operator != testUserName {
			t.Errorf("operator = %q, 期望 %q", logRow.Operator, testUserName)
		}
		if logRow.Remark != "卡损坏更换" {
			t.Errorf("remark = %q", logRow.Remark)
		}

		// 设备当前卡已切换
		dev, _ := devices.Get(ctx, d.ID)
		if dev.IotCardID == nil || *dev.IotCardID != c2.ID {
			t.Errorf("device.iot_card_id = %v, 期望 %d", dev.IotCardID, c2.ID)
		}
		// 旧卡解绑、新卡绑定
		var oldCard, newCard model.IotCard
		_ = tx.Where("id = ?", c1.ID).First(&oldCard).Error
		_ = tx.Where("id = ?", c2.ID).First(&newCard).Error
		if oldCard.DeviceID != nil {
			t.Errorf("旧卡 device_id 应被清空, got %v", oldCard.DeviceID)
		}
		if newCard.DeviceID == nil || *newCard.DeviceID != d.ID {
			t.Errorf("新卡 device_id = %v, 期望 %d", newCard.DeviceID, d.ID)
		}

		// 入参校验
		if _, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{DeviceID: 0, NewIotCardID: c2.ID}, ""); err == nil {
			t.Error("设备为空应返回错误")
		}
		if _, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{DeviceID: d.ID, NewIotCardID: 0}, ""); err == nil {
			t.Error("新卡为空应返回错误")
		}
		if _, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{
			DeviceID: d.ID, OldIotCardID: &c2.ID, NewIotCardID: c2.ID,
		}, ""); err == nil {
			t.Error("新卡与原卡相同应返回错误")
		}
		// 新卡已被其它设备占用时拒绝
		other := mustDevice(t, ctx, uniq("ZZQ-OTHER-DEV"), &regionID)
		if err := devices.Update(ctx, other.ID, &request.UpdateDeviceReq{Name: other.Name, IotCardID: &c2.ID}); err != nil {
			t.Fatalf("占用卡失败: %v", err)
		}
		if _, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{DeviceID: d.ID, NewIotCardID: c2.ID}, ""); err == nil {
			t.Error("新卡已被其它设备占用应返回错误")
		}
	})
}

// TestIotCardService_PageReplaceLogs 换卡记录为追加型：连续换卡产生多条且不覆盖。
func TestIotCardService_PageReplaceLogs(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-日志区域")
		c1 := mustIotCard(t, ctx, uniq("ZZQ-L1"), &regionID)
		c2 := mustIotCard(t, ctx, uniq("ZZQ-L2"), &regionID)
		c3 := mustIotCard(t, ctx, uniq("ZZQ-L3"), &regionID)
		d := mustDevice(t, ctx, uniq("ZZQ-LOGS"), &regionID)
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{Name: d.Name, IotCardID: &c1.ID}); err != nil {
			t.Fatalf("绑定卡失败: %v", err)
		}

		operator := uniq("op")
		if _, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{DeviceID: d.ID, NewIotCardID: c2.ID}, operator); err != nil {
			t.Fatalf("换卡失败: %v", err)
		}
		if _, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{DeviceID: d.ID, NewIotCardID: c3.ID}, operator); err != nil {
			t.Fatalf("换卡失败: %v", err)
		}

		deviceID := d.ID
		resp, err := cards.PageReplaceLogs(ctx, &request.PageIotCardReplaceLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceID: &deviceID,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 2 || len(resp.Items) != 2 {
			t.Fatalf("total=%d items=%d, 期望 2/2（追加型，不覆盖）", resp.Total, len(resp.Items))
		}
		if resp.Items[0].ID < resp.Items[1].ID {
			t.Error("换卡记录应按 replace_dt 倒序返回")
		}
		for _, it := range resp.Items {
			if it.Operator != operator {
				t.Errorf("operator = %q, 期望 %q", it.Operator, operator)
			}
			if it.Device == nil || it.Device.Name == "" {
				t.Errorf("Preload Device 名称未回显: %+v", it.Device)
			}
		}
		// 无 UPDATE/DELETE 端点：再次查询条数不变
		again, _ := cards.PageReplaceLogs(ctx, &request.PageIotCardReplaceLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceID: &deviceID,
		}, dsAll())
		if again.Total != 2 {
			t.Errorf("重复查询 total = %d, 期望 2", again.Total)
		}
	})
}

// TestIotCardService_PageReplaceLogs_Filters 换卡记录过滤条件。
func TestIotCardService_PageReplaceLogs_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-日志过滤区域")
		c1 := mustIotCard(t, ctx, uniq("ZZQ-F1"), &regionID)
		c2 := mustIotCard(t, ctx, uniq("ZZQ-F2"), &regionID)
		d := mustDevice(t, ctx, uniq("ZZQ-FLOG"), &regionID)
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{Name: d.Name, IotCardID: &c1.ID}); err != nil {
			t.Fatalf("绑定卡失败: %v", err)
		}
		if _, err := cards.ReplaceCard(ctx, &request.ReplaceIotCardReq{DeviceID: d.ID, NewIotCardID: c2.ID}, testUserName); err != nil {
			t.Fatalf("换卡失败: %v", err)
		}

		deviceID := d.ID
		byDevice, err := cards.PageReplaceLogs(ctx, &request.PageIotCardReplaceLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceID: &deviceID,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if byDevice.Total != 1 {
			t.Errorf("按设备过滤命中 %d, 期望 1", byDevice.Total)
		}

		byCode, _ := cards.PageReplaceLogs(ctx, &request.PageIotCardReplaceLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceCode: d.DeviceCode,
		}, dsAll())
		if byCode.Total != 1 {
			t.Errorf("按设备编号过滤命中 %d, 期望 1", byCode.Total)
		}

		byCard, _ := cards.PageReplaceLogs(ctx, &request.PageIotCardReplaceLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, CardNo: c1.CardNo,
		}, dsAll())
		if byCard.Total != 1 {
			t.Errorf("按原卡号过滤命中 %d, 期望 1", byCard.Total)
		}

		byRegion, _ := cards.PageReplaceLogs(ctx, &request.PageIotCardReplaceLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, RegionID: &regionID,
		}, dsAll())
		if byRegion.Total != 1 {
			t.Errorf("按区域过滤命中 %d, 期望 1", byRegion.Total)
		}

		none, _ := cards.PageReplaceLogs(ctx, &request.PageIotCardReplaceLogReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceCode: uniq("ZZQ-NO-DEV"),
		}, dsAll())
		if none.Total != 0 {
			t.Errorf("无匹配时 total = %d, 期望 0", none.Total)
		}
	})
}

// TestIotCardService_Options 物联网卡下拉选项：只含启用卡且受数据权限约束。
func TestIotCardService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		cards := newIotCardService()

		on, _ := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-选项卡", CardNo: uniq("8986020"), TrafficTotal: 1000, TrafficUsed: 10, Active: boolPtr(true),
		})
		off, _ := cards.Create(ctx, &request.CreateIotCardReq{
			Name: "ZZQ-停用卡", CardNo: uniq("8986021"), TrafficTotal: 1000, TrafficUsed: 10, Active: boolPtr(false),
		})

		items, err := cards.Options(ctx, dsAll())
		if err != nil {
			t.Fatalf("查询选项失败: %v", err)
		}
		seen := map[int64]bool{}
		for _, it := range items {
			seen[it.ID] = true
			if it.ID <= 0 || it.Name == "" || it.Code == "" {
				t.Errorf("选项字段不完整: %+v", it)
			}
		}
		if !seen[on.ID] {
			t.Error("启用卡应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用卡不应出现在选项中")
		}

		// 数据权限过滤：无匹配部门时不可见
		deptA := int64(9601)
		scoped := dsPerm("dept", "nobody", &deptA, "")
		narrow, _ := cards.Options(ctx, scoped)
		for _, it := range narrow {
			if it.ID == on.ID {
				t.Error("数据权限外的卡不应出现在选项中")
			}
		}
	})
}

// ── VoiceConfigService ────────────────────────────────────────

// TestVoiceConfigService_Create 创建语音配置：全字段落库。
func TestVoiceConfigService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		voices := newVoiceConfigService()

		riskTypeID := pickID(t, tx, "ac_risk_type", "")
		got, err := voices.Create(ctx, &request.CreateVoiceConfigReq{
			Name: "闯入告警播报", VoiceType: "intrusion", RiskTypeID: &riskTypeID,
			AudioURL: "/uploads/audio/intrusion.mp3", AudioName: "intrusion.mp3",
			Remark: "备注", Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建语音配置失败: %v", err)
		}
		var m model.VoiceConfig
		if err := tx.Where("id = ?", got.ID).First(&m).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.Name != "闯入告警播报" || m.VoiceType != "intrusion" || m.AudioURL != "/uploads/audio/intrusion.mp3" ||
			m.AudioName != "intrusion.mp3" || m.Remark != "备注" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.RiskTypeID == nil || *m.RiskTypeID != riskTypeID {
			t.Errorf("risk_type_id = %v", m.RiskTypeID)
		}
		if m.Active == nil || !*m.Active || m.CreatedBy != testUserName || m.CreatedDT.IsZero() {
			t.Errorf("默认值/审计字段异常: %+v", m)
		}
		if m.DeptID == nil || *m.DeptID != testDeptID {
			t.Errorf("dept_id = %v", m.DeptID)
		}
		if _, err := voices.Create(ctx, &request.CreateVoiceConfigReq{Name: "  "}); err == nil {
			t.Error("名称为空白应返回错误")
		}
	})
}

// TestVoiceConfigService_Create_Relations 语音配置与风险类型/区域/设备关联并回显名称。
func TestVoiceConfigService_Create_Relations(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		voices := newVoiceConfigService()

		regionName := "ZZQ-语音区域"
		regionID := mustRegion(t, ctx, regionName)
		device := mustDevice(t, ctx, uniq("ZZQ-VOICE-DEV"), &regionID)

		var rt model.RiskType
		if err := tx.Where("id = ?", pickID(t, tx, "ac_risk_type", "")).First(&rt).Error; err != nil {
			t.Fatalf("读取风险类型失败: %v", err)
		}

		if _, err := voices.Create(ctx, &request.CreateVoiceConfigReq{
			Name: "ZZQ-关联语音", VoiceType: rt.Code, RiskTypeID: &rt.ID, RegionID: &regionID, DeviceID: &device.ID,
		}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		page, err := voices.Page(ctx, &request.PageVoiceConfigReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-关联语音",
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if page.Total != 1 || len(page.Items) != 1 {
			t.Fatalf("total=%d items=%d", page.Total, len(page.Items))
		}
		it := page.Items[0]
		if it.RiskType == nil || it.RiskType.Name != rt.Name {
			t.Errorf("RiskType.Name 未回显: %+v", it.RiskType)
		}
		if it.Region == nil || it.Region.Name != regionName {
			t.Errorf("Region.Name 未回显: %+v", it.Region)
		}
		if it.Device == nil || it.Device.Name != device.Name {
			t.Errorf("Device.Name 未回显: %+v", it.Device)
		}
	})
}

// TestVoiceConfigService_Create_InvalidRelation 关联不存在时被数据库外键拒绝（不得产生脏数据）。
func TestVoiceConfigService_Create_InvalidRelation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		voices := newVoiceConfigService()

		// 该用例触发外键错误，事务进入 aborted 状态，故只做单次断言。
		bad := int64(999999)
		_, err := voices.Create(ctx, &request.CreateVoiceConfigReq{
			Name: "ZZQ-非法关联", RiskTypeID: &bad,
		})
		if err == nil {
			t.Fatal("关联不存在的风险类型应返回错误")
		}
	})
}

// TestVoiceConfigService_Page 语音配置分页与过滤，音频字段完整返回。
func TestVoiceConfigService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		voices := newVoiceConfigService()

		regionID := mustRegion(t, ctx, "ZZQ-语音分页区域")
		device := mustDevice(t, ctx, uniq("ZZQ-VP-DEV"), &regionID)
		vt := uniq("vt")
		if _, err := voices.Create(ctx, &request.CreateVoiceConfigReq{
			Name: "ZZQ-分页语音", VoiceType: vt, RegionID: &regionID, DeviceID: &device.ID,
			AudioURL: "/uploads/audio/a.mp3", AudioName: "a.mp3",
		}); err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		page, err := voices.Page(ctx, &request.PageVoiceConfigReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, VoiceType: vt,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if page.Total != 1 {
			t.Fatalf("按 voice_type 过滤命中 %d, 期望 1", page.Total)
		}
		it := page.Items[0]
		if it.AudioURL != "/uploads/audio/a.mp3" || it.AudioName != "a.mp3" {
			t.Errorf("音频字段不完整: %+v", it)
		}

		byDevice, _ := voices.Page(ctx, &request.PageVoiceConfigReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceID: &device.ID,
		}, dsAll())
		if byDevice.Total != 1 {
			t.Errorf("按设备过滤命中 %d, 期望 1", byDevice.Total)
		}
		byRegion, _ := voices.Page(ctx, &request.PageVoiceConfigReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, RegionID: &regionID,
		}, dsAll())
		if byRegion.Total != 1 {
			t.Errorf("按区域过滤命中 %d, 期望 1", byRegion.Total)
		}
		byName, _ := voices.Page(ctx, &request.PageVoiceConfigReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-分页语音",
		}, dsAll())
		if byName.Total != 1 {
			t.Errorf("按名称过滤命中 %d, 期望 1", byName.Total)
		}
	})
}

// TestVoiceConfigService_Update 语音配置更新：9 字段全部生效。
func TestVoiceConfigService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		voices := newVoiceConfigService()

		regionA := mustRegion(t, ctx, "ZZQ-VU-A")
		regionB := mustRegion(t, ctx, "ZZQ-VU-B")
		devA := mustDevice(t, ctx, uniq("ZZQ-VU-DA"), &regionA)
		devB := mustDevice(t, ctx, uniq("ZZQ-VU-DB"), &regionB)
		rtA := pickID(t, tx, "ac_risk_type", "")
		rtB := pickID(t, tx, "ac_risk_type", "id <> ?", rtA)

		created, err := voices.Create(ctx, &request.CreateVoiceConfigReq{
			Name: "ZZQ-旧语音", VoiceType: "old", RiskTypeID: &rtA, RegionID: &regionA, DeviceID: &devA.ID,
			AudioURL: "/old.mp3", AudioName: "old.mp3", Remark: "旧",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		if err := voices.Update(ctx, created.ID, &request.UpdateVoiceConfigReq{
			Name: "ZZQ-新语音", VoiceType: "new", RiskTypeID: &rtB, RegionID: &regionB, DeviceID: &devB.ID,
			AudioURL: "/new.mp3", AudioName: "new.mp3", Remark: "新", Active: boolPtr(false),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		var m model.VoiceConfig
		if err := tx.Where("id = ?", created.ID).First(&m).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.Name != "ZZQ-新语音" || m.VoiceType != "new" || m.AudioURL != "/new.mp3" ||
			m.AudioName != "new.mp3" || m.Remark != "新" {
			t.Errorf("字段未全部更新: %+v", m)
		}
		if m.RiskTypeID == nil || *m.RiskTypeID != rtB {
			t.Errorf("risk_type_id = %v, 期望 %d", m.RiskTypeID, rtB)
		}
		if m.RegionID == nil || *m.RegionID != regionB {
			t.Errorf("region_id = %v, 期望 %d", m.RegionID, regionB)
		}
		if m.DeviceID == nil || *m.DeviceID != devB.ID {
			t.Errorf("device_id = %v, 期望 %d", m.DeviceID, devB.ID)
		}
		if m.Active == nil || *m.Active {
			t.Errorf("active = %v, 期望 false", m.Active)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}
	})
}

// TestVoiceConfigService_Delete 语音配置软删除后不再出现在列表中。
func TestVoiceConfigService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		voices := newVoiceConfigService()

		created, err := voices.Create(ctx, &request.CreateVoiceConfigReq{Name: "ZZQ-待删语音", VoiceType: "x"})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		if err := voices.Delete(ctx, created.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_voice_config", created.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		page, _ := voices.Page(ctx, &request.PageVoiceConfigReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-待删语音",
		}, dsAll())
		if page.Total != 0 {
			t.Errorf("软删除后 total = %d, 期望 0", page.Total)
		}
		if err := voices.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的配置应返回错误")
		}
	})
}

// 保留 fmt 引用，避免用例调整后 import 失效。
var _ = fmt.Sprintf
