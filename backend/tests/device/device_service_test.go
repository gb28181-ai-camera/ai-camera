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
// 设备管理（DeviceService）
// ════════════════════════════════════════════════════════════════

// TestDeviceService_Create 创建设备：全字段落库 + 默认值 + 卡双向绑定。
func TestDeviceService_Create(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-区域")
		card := mustIotCard(t, ctx, uniq("ZZQ-CARD-C"), &regionID)
		fw := mustFirmware(t, ctx, uniq("1.0"))
		md := mustAIModel(t, ctx, uniq("1.0"), fw)

		code := uniq("ZZQ-CAM-C")
		got, err := devices.Create(ctx, &request.CreateDeviceReq{
			Name: "1号摄像头", DeviceCode: code, RegionID: &regionID,
			Location: "东门入口", Latitude: "30.123456", Longitude: "120.123456",
			FirmwareID: &fw.ID, FirmwareVer: fw.Version, ModelID: &md.ID, ModelVer: md.Version,
			SystemVer: "sys-1.0", WorkMode: "正常", DeviceMode: "全开模式", InfoInterval: 120,
			IotCardID: &card.ID, StorageTotal: 65536, MonitorEnabled: boolPtr(true),
			StreamURL: "rtsp://192.168.1.10/live", Remark: "备注", Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建设备失败: %v", err)
		}

		m, err := devices.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("回读设备失败: %v", err)
		}
		if m.Name != "1号摄像头" || m.DeviceCode != code || m.Location != "东门入口" ||
			m.Latitude != "30.123456" || m.Longitude != "120.123456" ||
			m.FirmwareVer != fw.Version || m.ModelVer != md.Version || m.SystemVer != "sys-1.0" ||
			m.WorkMode != "正常" || m.DeviceMode != "全开模式" || m.InfoInterval != 120 ||
			m.StorageTotal != 65536 || m.StreamURL != "rtsp://192.168.1.10/live" || m.Remark != "备注" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.RegionID == nil || *m.RegionID != regionID {
			t.Errorf("region_id = %v", m.RegionID)
		}
		if m.FirmwareID == nil || *m.FirmwareID != fw.ID {
			t.Errorf("firmware_id = %v", m.FirmwareID)
		}
		if m.ModelID == nil || *m.ModelID != md.ID {
			t.Errorf("model_id = %v", m.ModelID)
		}
		if m.IotCardID == nil || *m.IotCardID != card.ID {
			t.Errorf("iot_card_id = %v", m.IotCardID)
		}
		if m.MonitorEnabled == nil || !*m.MonitorEnabled {
			t.Errorf("monitor_enabled = %v, 期望 true", m.MonitorEnabled)
		}
		if m.Active == nil || !*m.Active {
			t.Errorf("active = %v, 期望 true", m.Active)
		}
		if m.DeptID == nil || *m.DeptID != testDeptID {
			t.Errorf("dept_id = %v, 期望 %d", m.DeptID, testDeptID)
		}
		if m.CreatedBy != testUserName || m.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", m.CreatedBy, m.CreatedDT)
		}

		// 双向绑定：卡侧的 device_id 指向该设备
		var bound model.IotCard
		if err := tx.Where("id = ?", card.ID).First(&bound).Error; err != nil {
			t.Fatalf("回读物联网卡失败: %v", err)
		}
		if bound.DeviceID == nil || *bound.DeviceID != m.ID {
			t.Errorf("卡侧 device_id = %v, 期望 %d", bound.DeviceID, m.ID)
		}

		// InfoInterval 缺省为 60，monitor_enabled 缺省 false
		def, err := devices.Create(ctx, &request.CreateDeviceReq{Name: "ZZQ-默认设备", DeviceCode: uniq("ZZQ-DEF")})
		if err != nil {
			t.Fatalf("创建默认设备失败: %v", err)
		}
		if def.InfoInterval != 60 {
			t.Errorf("默认 info_interval = %d, 期望 60", def.InfoInterval)
		}
		if def.MonitorEnabled == nil || *def.MonitorEnabled {
			t.Errorf("默认 monitor_enabled = %v, 期望 false", def.MonitorEnabled)
		}
	})
}

// TestDeviceService_Create_DefaultStatus 新建时状态固定为离线，运行时字段全零。
func TestDeviceService_Create_DefaultStatus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		code := uniq("ZZQ-OFFLINE")
		got := mustDevice(t, ctx, code, nil)
		m, _ := devices.Get(ctx, got.ID)

		if m.Status != model.DeviceStatusOffline {
			t.Errorf("status = %q, 期望 offline", m.Status)
		}
		if m.Battery != 0 || m.Voltage != "" || m.StorageTotal != 0 || m.StorageFree != 0 {
			t.Errorf("运行时字段应为零值: battery=%d voltage=%q total=%d free=%d",
				m.Battery, m.Voltage, m.StorageTotal, m.StorageFree)
		}
		if m.OnlineDT != nil || m.OfflineDT != nil || m.LastBeatDT != nil {
			t.Errorf("跃迁时间应均为 nil: online=%v offline=%v beat=%v", m.OnlineDT, m.OfflineDT, m.LastBeatDT)
		}
	})
}

// TestDeviceService_Create_DuplicateCode 设备编号唯一。
func TestDeviceService_Create_DuplicateCode(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		code := uniq("ZZQ-DUP-DEV")
		if _, err := devices.Create(ctx, &request.CreateDeviceReq{Name: "ZZQ-1", DeviceCode: code}); err != nil {
			t.Fatalf("首次创建失败: %v", err)
		}
		if _, err := devices.Create(ctx, &request.CreateDeviceReq{Name: "ZZQ-2", DeviceCode: code}); err == nil {
			t.Fatal("重复设备编号应返回错误")
		}
		if n := countRows(t, tx, "ac_device", "device_code = ?", code); n != 1 {
			t.Errorf("同编号设备数 = %d, 期望 1", n)
		}

		// 名称与编号为空白时拒绝
		if _, err := devices.Create(ctx, &request.CreateDeviceReq{Name: "  ", DeviceCode: uniq("ZZQ-BLANK")}); err == nil {
			t.Error("名称为空白应返回错误")
		}
		if _, err := devices.Create(ctx, &request.CreateDeviceReq{Name: "ZZQ-3", DeviceCode: "  "}); err == nil {
			t.Error("编号为空白应返回错误")
		}
	})
}

// TestDeviceService_Get 设备详情逐字段核对。
func TestDeviceService_Get(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-详情区域")
		got := mustDevice(t, ctx, uniq("ZZQ-GET-DEV"), &regionID)

		m, err := devices.Get(ctx, got.ID)
		if err != nil {
			t.Fatalf("获取失败: %v", err)
		}
		if m.Name != got.Name || m.DeviceCode != got.DeviceCode || m.Location != got.Location {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.RegionID == nil || *m.RegionID != regionID {
			t.Errorf("region_id = %v", m.RegionID)
		}
		if m.Region == nil || m.Region.Name != "ZZQ-详情区域" {
			t.Errorf("Region 关联未回显: %+v", m.Region)
		}
		if _, err := devices.Get(ctx, 999999999); err == nil {
			t.Error("不存在的设备应返回错误")
		}
	})
}

// TestDeviceService_Page 设备分页元信息与 items 长度。
func TestDeviceService_Page(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		keyword := uniq("ZZQ-PAGE-DEV")
		for i := 0; i < 3; i++ {
			mustDevice(t, ctx, fmt.Sprintf("%s-%d", keyword, i), nil)
		}

		resp, err := devices.Page(ctx, &request.PageDeviceReq{
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

		// 软删除的设备不出现在列表中
		target := resp.Items[0].ID
		if err := devices.Delete(ctx, target); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		after, _ := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword,
		}, dsAll())
		if after.Total != 2 {
			t.Errorf("删除后 total = %d, 期望 2", after.Total)
		}
		for _, it := range after.Items {
			if it.ID == target {
				t.Error("软删除的设备不应出现在列表中")
			}
		}
	})
}

// TestDeviceService_Page_Preload 列表回显区域/物联网卡/固件/模型名称。
func TestDeviceService_Page_Preload(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-预载区域")
		card := mustIotCard(t, ctx, uniq("ZZQ-CARD-P"), &regionID)
		fw := mustFirmware(t, ctx, uniq("9.9"))
		md := mustAIModel(t, ctx, uniq("8.8"), fw)
		keyword := uniq("ZZQ-PRELOAD")

		if _, err := devices.Create(ctx, &request.CreateDeviceReq{
			Name: "ZZQ-预载设备", DeviceCode: keyword, RegionID: &regionID,
			IotCardID: &card.ID, FirmwareID: &fw.ID, FirmwareVer: fw.Version,
			ModelID: &md.ID, ModelVer: md.Version,
		}); err != nil {
			t.Fatalf("创建设备失败: %v", err)
		}

		resp, err := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("items = %d, 期望 1", len(resp.Items))
		}
		it := resp.Items[0]
		if it.Region == nil || it.Region.Name != "ZZQ-预载区域" {
			t.Errorf("Region.Name 未回显: %+v", it.Region)
		}
		if it.IotCard == nil || it.IotCard.CardNo != card.CardNo {
			t.Errorf("IotCard.CardNo 未回显: %+v", it.IotCard)
		}
		if it.Firmware == nil || it.Firmware.Version != fw.Version {
			t.Errorf("Firmware.Version 未回显: %+v", it.Firmware)
		}
		if it.AIModel == nil || it.AIModel.Version != md.Version {
			t.Errorf("AIModel.Version 未回显: %+v", it.AIModel)
		}
	})
}

// TestDeviceService_Page_Filters 按 status / region_id / name / device_code / device_mode 过滤。
func TestDeviceService_Page_Filters(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionA := mustRegion(t, ctx, "ZZQ-过滤区域A")
		regionB := mustRegion(t, ctx, "ZZQ-过滤区域B")
		keyword := uniq("ZZQ-FILTER")

		d1 := mustDevice(t, ctx, keyword+"-A", &regionA)
		mustDevice(t, ctx, keyword+"-B", &regionB)
		// 名称不含编号，用于验证 keyword 命中的是「设备编号」列
		if _, err := devices.Create(ctx, &request.CreateDeviceReq{
			Name: "别名设备", DeviceCode: keyword + "-C", RegionID: &regionB,
		}); err != nil {
			t.Fatalf("创建设备失败: %v", err)
		}
		if err := devices.SwitchMode(ctx, d1.ID, "省电模式"); err != nil {
			t.Fatalf("切换模式失败: %v", err)
		}
		// 让 A 在线
		if err := devices.Heartbeat(ctx, d1.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOnline, Battery: 90}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}

		byRegion, err := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword, RegionID: &regionA,
		}, dsAll())
		if err != nil {
			t.Fatalf("按区域分页失败: %v", err)
		}
		if byRegion.Total != 1 {
			t.Errorf("按区域命中 %d, 期望 1", byRegion.Total)
		}

		statusOnline := model.DeviceStatusOnline
		byStatus, _ := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword, Status: statusOnline,
		}, dsAll())
		if byStatus.Total != 1 {
			t.Errorf("按状态命中 %d, 期望 1", byStatus.Total)
		}

		// keyword 同时模糊匹配设备名称 / 设备编号 / 设备位置
		byCode, _ := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword + "-C",
		}, dsAll())
		if byCode.Total != 1 {
			t.Errorf("按设备编号命中 %d, 期望 1（别名设备的名称不含编号）", byCode.Total)
		}

		byName, _ := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "ZZQ-" + keyword + "-A",
		}, dsAll())
		if byName.Total != 1 {
			t.Errorf("按名称命中 %d, 期望 1", byName.Total)
		}

		byLocation, _ := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: "东门入口",
		}, dsAll())
		if byLocation.Total < 2 {
			t.Errorf("按位置命中 %d, 期望 >= 2", byLocation.Total)
		}

		byMode, _ := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: keyword, DeviceMode: "省电模式",
		}, dsAll())
		if byMode.Total != 1 {
			t.Errorf("按设备模式命中 %d, 期望 1", byMode.Total)
		}

		none, _ := devices.Page(ctx, &request.PageDeviceReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, Keyword: uniq("ZZQ-NONE"),
		}, dsAll())
		if none.Total != 0 || len(none.Items) != 0 {
			t.Errorf("无匹配时 total=%d items=%d", none.Total, len(none.Items))
		}
	})
}

// TestDeviceService_Update 设备更新：可改字段全部生效，运行时字段不受影响。
func TestDeviceService_Update(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionA := mustRegion(t, ctx, "ZZQ-更新区域A")
		regionB := mustRegion(t, ctx, "ZZQ-更新区域B")
		card := mustIotCard(t, ctx, uniq("ZZQ-CARD-U"), &regionA)
		otherCard := mustIotCard(t, ctx, uniq("ZZQ-CARD-U2"), &regionA)

		d := mustDevice(t, ctx, uniq("ZZQ-UPD"), &regionA)
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{
			Status: model.DeviceStatusOnline, Battery: 66, Voltage: "12.1", StorageFree: 4096,
		}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}

		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{
			Name: "ZZQ-新名称", RegionID: &regionB, Location: "西门", Latitude: "31.1", Longitude: "121.2",
			WorkMode: "节能", InfoInterval: 300, IotCardID: &card.ID,
			StreamURL: "rtsp://new/live", Remark: "已更新", MonitorEnabled: boolPtr(true),
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		m, _ := devices.Get(ctx, d.ID)
		if m.Name != "ZZQ-新名称" || m.Location != "西门" || m.Latitude != "31.1" || m.Longitude != "121.2" ||
			m.WorkMode != "节能" || m.InfoInterval != 300 || m.StreamURL != "rtsp://new/live" || m.Remark != "已更新" {
			t.Errorf("字段未全部更新: %+v", m)
		}
		if m.RegionID == nil || *m.RegionID != regionB {
			t.Errorf("region_id = %v, 期望 %d", m.RegionID, regionB)
		}
		if m.IotCardID == nil || *m.IotCardID != card.ID {
			t.Errorf("iot_card_id = %v, 期望 %d", m.IotCardID, card.ID)
		}
		if m.MonitorEnabled == nil || !*m.MonitorEnabled {
			t.Errorf("monitor_enabled = %v, 期望 true", m.MonitorEnabled)
		}
		if m.DeviceCode != d.DeviceCode {
			t.Errorf("device_code 不应被改动: %q", m.DeviceCode)
		}
		if m.Status != model.DeviceStatusOnline || m.Battery != 66 || m.Voltage != "12.1" {
			t.Errorf("运行时字段不应被覆盖: status=%q battery=%d voltage=%q", m.Status, m.Battery, m.Voltage)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}

		// iot_card_id 传 0 表示解绑，并同步卡侧
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{Name: m.Name, IotCardID: int64Ptr(0)}); err != nil {
			t.Fatalf("解绑失败: %v", err)
		}
		unbound, _ := devices.Get(ctx, d.ID)
		if unbound.IotCardID != nil {
			t.Errorf("解绑后 iot_card_id 应为 nil, got %v", unbound.IotCardID)
		}
		var c model.IotCard
		_ = tx.Where("id = ?", card.ID).First(&c).Error
		if c.DeviceID != nil {
			t.Errorf("旧卡 device_id 应被清空, got %v", c.DeviceID)
		}

		// 更新为其它设备已占用的编号被拒绝
		other := mustDevice(t, ctx, uniq("ZZQ-OTHER-UPD"), &regionA)
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{DeviceCode: other.DeviceCode}); err == nil {
			t.Error("设备编号与其它设备冲突时应返回错误")
		}
		// 改成自己的原编号不算冲突
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{DeviceCode: d.DeviceCode}); err != nil {
			t.Errorf("保留原设备编号应允许: %v", err)
		}
		_ = otherCard
	})
}

// TestDeviceService_Delete 设备软删除并解除卡绑定；历史记录保留。
func TestDeviceService_Delete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-删除区域")
		card := mustIotCard(t, ctx, uniq("ZZQ-CARD-D"), &regionID)
		d := mustDevice(t, ctx, uniq("ZZQ-DEL-DEV"), &regionID)
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{Name: d.Name, IotCardID: &card.ID}); err != nil {
			t.Fatalf("绑定卡失败: %v", err)
		}
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOnline}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}

		if err := devices.Delete(ctx, d.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_device", d.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		if _, err := devices.Get(ctx, d.ID); err == nil {
			t.Error("软删除后不应可读")
		}
		if !activeCardUnbound(t, tx, card.ID) {
			t.Error("删除设备应解除其物联网卡绑定")
		}

		// 历史记录保留（不随设备删除而丢失）
		if n := countRows(t, tx, "ac_device_history", "device_id = ?", d.ID); n == 0 {
			t.Error("设备历史记录应保留")
		}
		opts, _ := devices.Options(ctx, dsAll())
		for _, it := range opts {
			if it.ID == d.ID {
				t.Error("软删除后不应出现在选项中")
			}
		}
		if err := devices.Delete(ctx, 999999999); err == nil {
			t.Error("不存在的设备应返回错误")
		}
	})
}

// TestDeviceService_SwitchMode 切换设备模式：仅改写 device_mode 并刷新审计字段。
func TestDeviceService_SwitchMode(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		d := mustDevice(t, ctx, uniq("ZZQ-MODE"), nil)
		before, _ := devices.Get(ctx, d.ID)

		if err := devices.SwitchMode(ctx, d.ID, "省电模式"); err != nil {
			t.Fatalf("切换模式失败: %v", err)
		}
		m, _ := devices.Get(ctx, d.ID)
		if m.DeviceMode != "省电模式" {
			t.Errorf("device_mode = %q, 期望 省电模式", m.DeviceMode)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}
		if m.Name != before.Name || m.DeviceCode != before.DeviceCode || m.Status != before.Status {
			t.Errorf("其它字段不应变化: %+v", m)
		}

		// 未在设备模式表中登记的模式被拒绝
		if err := devices.SwitchMode(ctx, d.ID, uniq("ZZQ-NO-SUCH-MODE")); err == nil {
			t.Error("未登记的模式应返回错误")
		}
		if err := devices.SwitchMode(ctx, d.ID, ""); err == nil {
			t.Error("空模式应返回错误")
		}
	})
}

// TestDeviceService_SyncTime 同步时间：刷新 last_beat_dt 与审计字段。
func TestDeviceService_SyncTime(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		d := mustDevice(t, ctx, uniq("ZZQ-SYNC"), nil)
		before, _ := devices.Get(ctx, d.ID)
		if before.LastBeatDT != nil {
			t.Fatalf("新建设备 last_beat_dt 应为 nil")
		}

		if err := devices.SyncTime(ctx, d.ID); err != nil {
			t.Fatalf("同步时间失败: %v", err)
		}
		m, _ := devices.Get(ctx, d.ID)
		if m.LastBeatDT == nil {
			t.Fatal("last_beat_dt 应被置为当前时间")
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}
		if m.Name != before.Name || m.DeviceCode != before.DeviceCode || m.Status != before.Status ||
			m.Battery != before.Battery {
			t.Errorf("不应修改业务字段: %+v", m)
		}
		if err := devices.SyncTime(ctx, 999999999); err == nil {
			t.Error("不存在的设备应返回错误")
		}
	})
}

// TestDeviceService_ToggleMonitor 加入/移出实时监控。
func TestDeviceService_ToggleMonitor(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		d := mustDevice(t, ctx, uniq("ZZQ-MON"), nil)
		if err := devices.ToggleMonitor(ctx, d.ID, true); err != nil {
			t.Fatalf("开启监控失败: %v", err)
		}
		on, _ := devices.Get(ctx, d.ID)
		if on.MonitorEnabled == nil || !*on.MonitorEnabled {
			t.Errorf("monitor_enabled = %v, 期望 true", on.MonitorEnabled)
		}
		if on.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", on.UpdatedBy)
		}

		if err := devices.ToggleMonitor(ctx, d.ID, false); err != nil {
			t.Fatalf("关闭监控失败: %v", err)
		}
		off, _ := devices.Get(ctx, d.ID)
		if off.MonitorEnabled == nil || *off.MonitorEnabled {
			t.Errorf("monitor_enabled = %v, 期望 false", off.MonitorEnabled)
		}
		if err := devices.ToggleMonitor(ctx, 999999999, true); err == nil {
			t.Error("不存在的设备应返回错误")
		}
	})
}

// TestDeviceService_Heartbeat 心跳：写设备快照 + 追加历史 + 冗余字段回填。
func TestDeviceService_Heartbeat(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-心跳区域")
		card := mustIotCard(t, ctx, uniq("ZZQ-CARD-H"), &regionID)
		d := mustDevice(t, ctx, uniq("ZZQ-BEAT"), &regionID)
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{Name: d.Name, IotCardID: &card.ID, Location: "东门入口"}); err != nil {
			t.Fatalf("绑定卡失败: %v", err)
		}

		histBefore := countRows(t, tx, "ac_device_history", "device_id = ?", d.ID)
		start := time.Now()
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{
			Status: model.DeviceStatusOnline, Battery: 85, Voltage: "12.6", StorageFree: 8192,
		}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}

		m, _ := devices.Get(ctx, d.ID)
		if m.Status != model.DeviceStatusOnline || m.Battery != 85 || m.Voltage != "12.6" || m.StorageFree != 8192 {
			t.Errorf("设备快照未更新: status=%q battery=%d voltage=%q free=%d", m.Status, m.Battery, m.Voltage, m.StorageFree)
		}
		if m.LastBeatDT == nil || m.LastBeatDT.Before(start.Add(-time.Minute)) {
			t.Errorf("last_beat_dt = %v, 应接近当前时间", m.LastBeatDT)
		}

		if n := countRows(t, tx, "ac_device_history", "device_id = ?", d.ID); n != histBefore+1 {
			t.Fatalf("历史记录新增 %d 条, 期望 1", n-histBefore)
		}
		var h model.DeviceHistory
		if err := tx.Where("device_id = ?", d.ID).Order("id DESC").First(&h).Error; err != nil {
			t.Fatalf("回读历史失败: %v", err)
		}
		if h.DeviceID != d.ID || h.DeviceCode != d.DeviceCode || h.IotCardNo != card.CardNo {
			t.Errorf("历史冗余字段错误: device_code=%q iot_card_no=%q", h.DeviceCode, h.IotCardNo)
		}
		if h.RegionID == nil || *h.RegionID != regionID || h.Location != "东门入口" {
			t.Errorf("历史冗余区域/位置错误: %v/%q", h.RegionID, h.Location)
		}
		if h.Status != model.DeviceStatusOnline || h.Battery != 85 || h.Voltage != "12.6" || h.StorageFree != 8192 {
			t.Errorf("历史采样值错误: %+v", h)
		}
		if h.SampledDT.IsZero() {
			t.Error("sampled_dt 不应为零值")
		}
	})
}

// TestDeviceService_Heartbeat_StatusTransition 状态跃迁时补记 online_dt / offline_dt。
func TestDeviceService_Heartbeat_StatusTransition(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		d := mustDevice(t, ctx, uniq("ZZQ-TRANS"), nil)

		// offline -> online：online_dt 由 nil 变非空
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOnline, Battery: 70}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
		on, _ := devices.Get(ctx, d.ID)
		if on.OnlineDT == nil {
			t.Fatal("状态跃迁到 online 时 online_dt 应非空")
		}
		if on.OfflineDT != nil {
			t.Errorf("offline_dt 应保持 nil, got %v", on.OfflineDT)
		}
		onlineDT := *on.OnlineDT

		time.Sleep(5 * time.Millisecond)

		// online -> offline：offline_dt 由 nil 变非空，online_dt 保持不变
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOffline}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
		off, _ := devices.Get(ctx, d.ID)
		if off.OfflineDT == nil {
			t.Fatal("状态跃迁到 offline 时 offline_dt 应非空")
		}
		if off.OnlineDT == nil || !off.OnlineDT.Equal(onlineDT) {
			t.Errorf("online_dt 不应被刷新: %v -> %v", onlineDT, off.OnlineDT)
		}
		if off.Status != model.DeviceStatusOffline {
			t.Errorf("status = %q, 期望 offline", off.Status)
		}
	})
}

// TestDeviceService_Heartbeat_SameStatus 同状态连续心跳不刷新跃迁时间。
func TestDeviceService_Heartbeat_SameStatus(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		d := mustDevice(t, ctx, uniq("ZZQ-SAME"), nil)
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOnline}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
		first, _ := devices.Get(ctx, d.ID)
		if first.OnlineDT == nil {
			t.Fatal("首次心跳应写 online_dt")
		}
		firstBeat := *first.LastBeatDT
		onlineDT := *first.OnlineDT

		time.Sleep(5 * time.Millisecond)
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOnline}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
		second, _ := devices.Get(ctx, d.ID)
		if !second.OnlineDT.Equal(onlineDT) {
			t.Errorf("同状态心跳不应刷新 online_dt: %v -> %v", onlineDT, second.OnlineDT)
		}
		if !second.LastBeatDT.After(firstBeat) {
			t.Errorf("last_beat_dt 应每次刷新: %v -> %v", firstBeat, second.LastBeatDT)
		}
		if n := countRows(t, tx, "ac_device_history", "device_id = ?", d.ID); n != 2 {
			t.Errorf("两次心跳应产生 2 条历史, got %d", n)
		}
	})
}

// TestDeviceService_Heartbeat_IotCardNo 历史记录的流量卡号为当时快照（换卡后随之变化）。
func TestDeviceService_Heartbeat_IotCardNo(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		regionID := mustRegion(t, ctx, "ZZQ-卡号区域")
		c1 := mustIotCard(t, ctx, uniq("ZZQ-C1"), &regionID)
		c2 := mustIotCard(t, ctx, uniq("ZZQ-C2"), &regionID)

		d := mustDevice(t, ctx, uniq("ZZQ-CARDNO"), &regionID)
		if err := devices.Update(ctx, d.ID, &request.UpdateDeviceReq{Name: d.Name, IotCardID: &c1.ID}); err != nil {
			t.Fatalf("绑定卡失败: %v", err)
		}
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOnline}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
		var h1 model.DeviceHistory
		if err := tx.Where("device_id = ?", d.ID).Order("id DESC").First(&h1).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if h1.IotCardNo != c1.CardNo {
			t.Errorf("历史 iot_card_no = %q, 期望 %q", h1.IotCardNo, c1.CardNo)
		}

		// 换卡后再次心跳，历史记录冗余新卡号
		if _, err := newIotCardService().ReplaceCard(ctx, &request.ReplaceIotCardReq{
			DeviceID: d.ID, NewIotCardID: c2.ID,
		}, testUserName); err != nil {
			t.Fatalf("换卡失败: %v", err)
		}
		if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{Status: model.DeviceStatusOnline}); err != nil {
			t.Fatalf("心跳失败: %v", err)
		}
		var h2 model.DeviceHistory
		if err := tx.Where("device_id = ?", d.ID).Order("id DESC").First(&h2).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if h2.IotCardNo != c2.CardNo {
			t.Errorf("换卡后历史 iot_card_no = %q, 期望 %q", h2.IotCardNo, c2.CardNo)
		}
	})
}

// TestDeviceService_PageHistory 设备历史分页：按 sampled_dt 倒序、支持过滤。
func TestDeviceService_PageHistory(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		d := mustDevice(t, ctx, uniq("ZZQ-HIST"), nil)
		for i := 0; i < 3; i++ {
			if err := devices.Heartbeat(ctx, d.ID, &request.DeviceHeartbeatReq{
				Status: model.DeviceStatusOnline, Battery: 50 + i, StorageFree: 100 + int64(i),
			}); err != nil {
				t.Fatalf("心跳失败: %v", err)
			}
			time.Sleep(2 * time.Millisecond)
		}

		deviceID := d.ID
		resp, err := devices.PageHistory(ctx, &request.PageDeviceHistoryReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceID: &deviceID,
		}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 3 || len(resp.Items) != 3 {
			t.Fatalf("total=%d items=%d, 期望 3/3", resp.Total, len(resp.Items))
		}
		for i := 1; i < len(resp.Items); i++ {
			if resp.Items[i].SampledDT.After(resp.Items[i-1].SampledDT) {
				t.Error("历史应按 sampled_dt 倒序返回")
			}
		}
		it := resp.Items[0]
		if it.DeviceID != d.ID || it.DeviceCode != d.DeviceCode || it.Status != model.DeviceStatusOnline {
			t.Errorf("冗余字段不完整: %+v", it)
		}

		byStatus, _ := devices.PageHistory(ctx, &request.PageDeviceHistoryReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceID: &deviceID, Status: model.DeviceStatusOffline,
		}, dsAll())
		if byStatus.Total != 0 {
			t.Errorf("按状态过滤命中 %d, 期望 0", byStatus.Total)
		}

		byCode, _ := devices.PageHistory(ctx, &request.PageDeviceHistoryReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, DeviceCode: d.DeviceCode,
		}, dsAll())
		if byCode.Total != 3 {
			t.Errorf("按设备编号过滤命中 %d, 期望 3", byCode.Total)
		}
	})
}

// TestDeviceService_HistoryChart 历史曲线：仅统计近 days 天并按小时聚合。
func TestDeviceService_HistoryChart(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		d := mustDevice(t, ctx, uniq("ZZQ-CHART"), nil)
		now := time.Now()
		// 近 3 天内 5 个不同小时 + 5 天前 2 条
		for i := 1; i <= 5; i++ {
			writeHistory(t, tx, d, now.Add(-time.Duration(i)*time.Hour), model.DeviceStatusOnline, 80-i)
		}
		for i := 0; i < 2; i++ {
			writeHistory(t, tx, d, now.AddDate(0, 0, -5).Add(-time.Duration(i)*time.Hour), model.DeviceStatusOffline, 10)
		}

		resp, err := devices.HistoryChart(ctx, d.ID, 3)
		if err != nil {
			t.Fatalf("查询曲线失败: %v", err)
		}
		if resp.DeviceID != d.ID || resp.Days != 3 || resp.Granularity != "hour" {
			t.Errorf("曲线元信息错误: %+v", resp)
		}
		if len(resp.BatteryPoints) != 5 {
			t.Errorf("电量点数 = %d, 期望 5（仅近 3 天）", len(resp.BatteryPoints))
		}
		if len(resp.VoltagePoints) != 5 || len(resp.StoragePoints) != 5 || len(resp.StatusPoints) != 5 {
			t.Errorf("四条曲线点数应一致: %d/%d/%d",
				len(resp.VoltagePoints), len(resp.StoragePoints), len(resp.StatusPoints))
		}
		for i := 0; i < len(resp.BatteryPoints); i++ {
			if resp.BatteryPoints[i].Time == "" {
				t.Error("曲线点时间标签不应为空")
			}
		}
		if resp.StatusSummary.OnlineMinutes < 0 || resp.StatusSummary.OfflineMinutes < 0 {
			t.Error("在线/离线时长不应为负")
		}

		// days<=0 走默认 3 天；days>7 按天聚合
		def, err := devices.HistoryChart(ctx, d.ID, 0)
		if err != nil {
			t.Fatalf("查询曲线失败: %v", err)
		}
		if def.Days != 3 {
			t.Errorf("days=0 时 Days = %d, 期望 3", def.Days)
		}
		big, _ := devices.HistoryChart(ctx, d.ID, 30)
		if big.Granularity != "day" {
			t.Errorf("days=30 时粒度 = %q, 期望 day", big.Granularity)
		}
		if big.Days != 30 {
			t.Errorf("days=30 时 Days = %d", big.Days)
		}
	})
}

// TestDeviceService_Options 设备下拉选项只返回启用设备。
func TestDeviceService_Options(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		devices := newDeviceService()

		on, err := devices.Create(ctx, &request.CreateDeviceReq{
			Name: "ZZQ-选项设备", DeviceCode: uniq("ZZQ-OPT-ON"), Active: boolPtr(true),
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		off, err := devices.Create(ctx, &request.CreateDeviceReq{
			Name: "ZZQ-停用设备", DeviceCode: uniq("ZZQ-OPT-OFF"), Active: boolPtr(false),
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		items, err := devices.Options(ctx, dsAll())
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
			t.Error("启用设备应出现在选项中")
		}
		if seen[off.ID] {
			t.Error("停用设备不应出现在选项中")
		}
	})
}

// TestDeviceService_Page_DataScope 设备列表数据权限四级。
func TestDeviceService_Page_DataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		devices := newDeviceService()

		// ac_device.dept_id 无外键约束，可直接构造隔离数据
		deptA, deptB, deptC := int64(9501), int64(9502), int64(9503)
		if err := tx.Unscoped().Where("dept_id IN ?", []int64{deptA, deptB, deptC}).Delete(&model.Device{}).Error; err != nil {
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
			m := &model.Device{
				Name: "ZZQ-DS-" + s.suffix, DeviceCode: uniq("ZZQ_DSDEV_" + s.suffix),
				Status: model.DeviceStatusOffline, Active: boolPtr(true),
				DeptID:    int64Ptr(s.dept),
				BaseModel: model.BaseModel{CreatedBy: s.creator, UpdatedBy: s.creator},
			}
			if err := tx.Create(m).Error; err != nil {
				t.Fatalf("预置数据失败: %v", err)
			}
		}

		q := request.PageDeviceReq{PageReq: request.PageReq{Page: 1, PageSize: 200}}
		cross, err := devices.Page(ctx, &q, dsPerm("cross_dept", "ds_user_b", &deptA, fmt.Sprintf("%d", deptB)))
		if err != nil {
			t.Fatalf("cross_dept 分页失败: %v", err)
		}
		dept, _ := devices.Page(ctx, &q, dsPerm("dept", "ds_user_b", &deptA, ""))
		personal, _ := devices.Page(ctx, &q, dsPerm("personal", "ds_user_a", &deptA, ""))
		all, _ := devices.Page(ctx, &q, dsAll())

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

// ── 内部工具 ──────────────────────────────────────────────────

// writeHistory 直接写入一条设备历史（曲线用例需要构造特定时间分布）。
func writeHistory(t *testing.T, tx *gorm.DB, d *model.Device, at time.Time, status string, battery int) {
	t.Helper()
	h := &model.DeviceHistory{
		DeviceID: d.ID, DeviceCode: d.DeviceCode, RegionID: d.RegionID, Location: d.Location,
		Status: status, Battery: battery, Voltage: "12.0", StorageFree: 1024, SampledDT: at,
	}
	if err := tx.Create(h).Error; err != nil {
		t.Fatalf("写入设备历史失败: %v", err)
	}
}

// activeCardUnbound 判断物联网卡是否已解绑（device_id 为空）。
func activeCardUnbound(t *testing.T, tx *gorm.DB, cardID int64) bool {
	t.Helper()
	var c model.IotCard
	if err := tx.Where("id = ?", cardID).First(&c).Error; err != nil {
		t.Fatalf("回读物联网卡失败: %v", err)
	}
	return c.DeviceID == nil
}
