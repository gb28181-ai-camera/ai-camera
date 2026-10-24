package gb28181_test

// 本文件覆盖三处「设备在线态维护」：
//   1. 心跳超时判离线（严格大于 3×心跳间隔，含端点清理与幂等）
//   2. 订阅自动续订（提前量纯逻辑 + 落库/跳过分支）
//   3. SSRC 从 ZLM 流列表周期重建（不重复分配、不释放本进程持有的流）
//
// 心跳与提前量用可控时钟/纯函数单测，不依赖数据库；涉及 ac_gb_subscribe 的
// 用例走 testutil.WithTx（事务回滚，不清理/重灌测试库）。

import (
	"context"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/device"
	"github.com/boticz/ai-camera-backend/internal/gb28181/ssrc"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 1. 心跳超时
// ════════════════════════════════════════════════════════════════

// heartbeatFakeStore 是 device.Store 的内存实现，并额外实现
// MarkHeartbeatTimeoutOffline（device 包通过类型断言识别该可选能力）。
type heartbeatFakeStore struct {
	dev *model.Device
	// writes 记录真正发生的「置离线」写入次数，用来断言已离线设备不被重复写。
	writes int
}

func (f *heartbeatFakeStore) FindDevice(context.Context, string) (*model.Device, error) {
	return f.dev, nil
}

func (f *heartbeatFakeStore) UpdateDevice(_ context.Context, _ int64, fields map[string]interface{}) error {
	if f.dev == nil {
		return nil
	}
	if v, ok := fields["status"].(string); ok {
		f.dev.Status = v
	}
	if v, ok := fields["last_beat_dt"].(time.Time); ok {
		f.dev.LastBeatDT = &v
	}
	return nil
}

func (f *heartbeatFakeStore) FindChannel(context.Context, string) (*model.GbChannel, error) {
	return nil, nil
}
func (f *heartbeatFakeStore) UpsertChannel(context.Context, *model.GbChannel) error { return nil }
func (f *heartbeatFakeStore) UpdateChannel(context.Context, int64, map[string]interface{}) error {
	return nil
}
func (f *heartbeatFakeStore) CountChannels(context.Context, int64) (int64, error) { return 0, nil }
func (f *heartbeatFakeStore) CreateSipLog(context.Context, *model.GbSipLog) error { return nil }
func (f *heartbeatFakeStore) CreateAlarm(context.Context, *model.GbAlarm) error   { return nil }
func (f *heartbeatFakeStore) CreateMobilePosition(context.Context, *model.GbMobilePosition) error {
	return nil
}
func (f *heartbeatFakeStore) MarkExpiredOffline(context.Context, time.Time) (int64, error) {
	return 0, nil
}

// MarkHeartbeatTimeoutOffline 复刻生产 SQL 的判定：now-last_beat_dt > timeout，
// 且只处理 status='online' 的行。
func (f *heartbeatFakeStore) MarkHeartbeatTimeoutOffline(_ context.Context, now time.Time, timeout time.Duration) (int64, error) {
	d := f.dev
	if d == nil || d.Status != model.DeviceStatusOnline || d.LastBeatDT == nil {
		return 0, nil
	}
	if !now.Add(-timeout).After(*d.LastBeatDT) {
		return 0, nil
	}
	d.Status = model.DeviceStatusOffline
	d.OfflineDT = &now
	f.writes++
	return 1, nil
}

// TestHeartbeatTimeout_Maintenance 覆盖「刚好未超时 / 超时 / 已离线」三种状态。
func TestHeartbeatTimeout_Maintenance(t *testing.T) {
	const gbID = "34020000001320000001"
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	dev := &model.Device{
		BaseModel:  model.BaseModel{ID: 1},
		GbDeviceID: gbID,
		Status:     model.DeviceStatusOnline,
		LastBeatDT: &base,
	}
	store := &heartbeatFakeStore{dev: dev}
	mgr := device.NewManager(store, device.Config{}, nil)

	clock := base
	mgr.SetNow(func() time.Time { return clock })
	mgr.SetEndpoint(gbID, "192.0.2.10:5060", "UDP")

	// ① 恰好 3×60s：now-last_beat == 阈值，严格大于才算超时 → 仍在线。
	clock = base.Add(180 * time.Second)
	n, err := mgr.HeartbeatOnce(context.Background())
	if err != nil {
		t.Fatalf("心跳扫描失败：%v", err)
	}
	if n != 0 || dev.Status != model.DeviceStatusOnline {
		t.Fatalf("边界上不应判离线：count=%d status=%s", n, dev.Status)
	}
	if _, ok := mgr.Endpoint(gbID); !ok {
		t.Fatalf("在线设备的端点不应被清理")
	}
	if store.writes != 0 {
		t.Fatalf("边界上不应写库，实际写入 %d 次", store.writes)
	}

	// ② 再超 1 秒：判定离线，写 offline_dt，并清理内存端点。
	clock = clock.Add(time.Second)
	n, err = mgr.HeartbeatOnce(context.Background())
	if err != nil {
		t.Fatalf("心跳扫描失败：%v", err)
	}
	if n != 1 || dev.Status != model.DeviceStatusOffline {
		t.Fatalf("超过阈值应判离线：count=%d status=%s", n, dev.Status)
	}
	if dev.OfflineDT == nil || !dev.OfflineDT.Equal(clock) {
		t.Fatalf("offline_dt 应写入当前时间，实际 %v", dev.OfflineDT)
	}
	if _, ok := mgr.Endpoint(gbID); ok {
		t.Fatalf("离线后应清理内存端点，避免继续向死地址发指令")
	}

	// ③ 已离线：后续扫描只更新 status='online' 的行，不能重复写。
	before := store.writes
	clock = clock.Add(10 * time.Minute)
	n, err = mgr.HeartbeatOnce(context.Background())
	if err != nil {
		t.Fatalf("心跳扫描失败：%v", err)
	}
	if n != 0 {
		t.Fatalf("已离线设备不应再次计数，实际 %d", n)
	}
	if store.writes != before {
		t.Fatalf("已离线设备不应重复写库：before=%d after=%d", before, store.writes)
	}
}

// TestHeartbeatTimeout_StoreSQL 验证生产 SQL 的三条过滤：
// 只动 online、严格大于阈值、跳过软删除设备。
func TestHeartbeatTimeout_StoreSQL(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		now := time.Now()
		stale := now.Add(-4 * time.Minute)  // 超过 180s
		inside := now.Add(-2 * time.Minute) // 未超过 180s
		online := model.DeviceStatusOnline
		offline := model.DeviceStatusOffline

		nextID := int64(-9100)
		mk := func(gbID string, status string, beat *time.Time) *model.Device {
			// 1) device_code 有唯一索引：同一事务里插入多台设备必须各自给值，
			//    全部留空会直接撞 idx_ac_device_code。
			// 2) 显式使用负数主键：测试库是共享的，种子数据用显式 ID 写入后
			//    自增序列可能落后于 max(id)（见 CLAUDE.md 的 syncSequences 说明），
			//    依赖序列会偶发撞 ac_device_pkey。负数不会与任何业务数据冲突。
			nextID++
			d := &model.Device{
				BaseModel:  model.BaseModel{ID: nextID},
				Name:       gbID,
				DeviceCode: "CODE_" + gbID,
				GbDeviceID: gbID,
				Status:     status,
				LastBeatDT: beat,
			}
			if err := tx.Create(d).Error; err != nil {
				t.Fatalf("创建设备失败：%v", err)
			}
			return d
		}
		expired := mk("34020000001320000101", online, &stale)
		fresh := mk("34020000001320000102", online, &inside)
		already := mk("34020000001320000103", offline, &stale)
		deleted := mk("34020000001320000104", online, &stale)
		if err := tx.Delete(deleted).Error; err != nil {
			t.Fatalf("软删除设备失败：%v", err)
		}

		n, err := device.NewGbStore().MarkHeartbeatTimeoutOffline(ctx, now, 3*time.Minute)
		if err != nil {
			t.Fatalf("心跳超时写库失败：%v", err)
		}
		if n != 1 {
			t.Fatalf("应只置离线 1 台，实际 %d", n)
		}

		reload := func(d *model.Device) model.Device {
			var got model.Device
			if err := tx.Unscoped().First(&got, d.ID).Error; err != nil {
				t.Fatalf("读取设备失败：%v", err)
			}
			return got
		}
		if got := reload(expired); got.Status != model.DeviceStatusOffline || got.OfflineDT == nil {
			t.Fatalf("超时设备应离线并写 offline_dt：status=%s offline_dt=%v", got.Status, got.OfflineDT)
		}
		if got := reload(fresh); got.Status != model.DeviceStatusOnline {
			t.Fatalf("未超时设备应保持在线，实际 %s", got.Status)
		}
		if got := reload(already); got.Status != model.DeviceStatusOffline {
			t.Fatalf("已离线设备不应被改动，实际 %s", got.Status)
		}
		if got := reload(deleted); got.Status != model.DeviceStatusOnline {
			t.Fatalf("软删除设备不应被改动，实际 %s", got.Status)
		}
	})
}

// ════════════════════════════════════════════════════════════════
// 2. 订阅自动续订
// ════════════════════════════════════════════════════════════════

// fakeController 实现 control.SipController + SubscribeWithCallID。
type fakeController struct {
	calls     []fakeSubscribeCall
	failTimes int // 前 N 次返回错误，用于验证失败计数
}

type fakeSubscribeCall struct {
	gbDeviceID string
	subType    string
	interval   int
}

func (f *fakeController) QueryCatalog(context.Context, *model.Device) error { return nil }

func (f *fakeController) Subscribe(_ context.Context, dev *model.Device, subType string, interval int) error {
	_, err := f.SubscribeWithCallID(context.Background(), dev, subType, interval)
	return err
}

func (f *fakeController) SubscribeWithCallID(_ context.Context, dev *model.Device, subType string, interval int) (string, error) {
	f.calls = append(f.calls, fakeSubscribeCall{gbDeviceID: dev.GbDeviceID, subType: subType, interval: interval})
	if f.failTimes > 0 {
		f.failTimes--
		return "failed-call-id", errFakeSubscribe
	}
	return "test-call-id", nil
}

func (f *fakeController) Unsubscribe(context.Context, *model.Device, string) error { return nil }
func (f *fakeController) SendMessage(context.Context, *model.Device, string) error { return nil }

type fakeSubscribeError struct{}

func (fakeSubscribeError) Error() string { return "模拟设备拒绝 SUBSCRIBE" }

var errFakeSubscribe = fakeSubscribeError{}

// TestRenewLead_Maintenance 续订提前量：20% 与 60s 下限、有效期一半上限。
func TestRenewLead_Maintenance(t *testing.T) {
	cases := []struct {
		expires int64
		want    time.Duration
	}{
		{3600, 12 * time.Minute},     // 20% = 720s
		{300, 60 * time.Second},      // 20% = 60s，正好等于下限
		{100, 50 * time.Second},      // 下限 60s 超过一半，按一半截断
		{30, 15 * time.Second},       // 同上
		{86400, 17280 * time.Second}, // 长有效期按 20%
		{0, 12 * time.Minute},        // <=0 按缺省 3600s
		{-5, 12 * time.Minute},       // 负数同样兜底
	}
	for _, c := range cases {
		if got := gbsvc.RenewLead(c.expires); got != c.want {
			t.Errorf("RenewLead(%d) = %v，期望 %v", c.expires, got, c.want)
		}
	}
}

// TestRenewDue_PersistsCallID 到期订阅应重发 SUBSCRIBE 并回写四个字段。
func TestRenewDue_PersistsCallID(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		now := time.Now()
		dev := &model.Device{
			BaseModel:  model.BaseModel{ID: -9201}, // 负数主键，避免共享测试库的序列漂移
			Name:       "维护测试设备",
			DeviceCode: "MAINT_DEV_77",
			GbDeviceID: "34020000001320000077",
			Status:     model.DeviceStatusOnline,
		}
		if err := tx.Create(dev).Error; err != nil {
			t.Fatalf("创建设备失败：%v", err)
		}

		expire := now.Add(30 * time.Second) // 已进入 720s 续订窗口
		sub := &model.GbSubscribe{
			BaseModel:  model.BaseModel{ID: -9301},
			DeviceID:   &dev.ID,
			GbDeviceID: dev.GbDeviceID,
			Type:       model.GbSubscribeCatalog,
			Expires:    3600,
			Status:     "active",
			ExpireDT:   &expire,
		}
		if err := tx.Create(sub).Error; err != nil {
			t.Fatalf("创建订阅失败：%v", err)
		}

		prev := control.Default()
		fake := &fakeController{}
		control.Set(fake)
		defer control.Set(prev)

		n, err := gbsvc.NewGbSubscribeService().RenewDue(ctx)
		if err != nil {
			t.Fatalf("续订失败：%v", err)
		}
		if n != 1 {
			t.Fatalf("应续订 1 条，实际 %d", n)
		}
		if len(fake.calls) != 1 {
			t.Fatalf("应发送 1 次 SUBSCRIBE，实际 %d", len(fake.calls))
		}
		if call := fake.calls[0]; call.gbDeviceID != dev.GbDeviceID ||
			call.subType != model.GbSubscribeCatalog || call.interval != 3600 {
			t.Fatalf("SUBSCRIBE 参数不符：%+v", call)
		}

		var got model.GbSubscribe
		if err := tx.First(&got, sub.ID).Error; err != nil {
			t.Fatalf("读取订阅失败：%v", err)
		}
		if got.Status != "active" {
			t.Errorf("续订成功后 status 应为 active，实际 %q", got.Status)
		}
		if got.CallID != "test-call-id" {
			t.Errorf("call_id 应回写真实 Call-ID，实际 %q", got.CallID)
		}
		if got.SubscribeDT == nil || got.SubscribeDT.Before(now.Add(-time.Minute)) {
			t.Errorf("subscribe_dt 应刷新为本次续订时间，实际 %v", got.SubscribeDT)
		}
		if got.ExpireDT == nil || got.ExpireDT.Before(now.Add(3500*time.Second)) {
			t.Errorf("expire_dt 应按 expires 顺延，实际 %v", got.ExpireDT)
		}
	})
}

// TestRenewDue_SkipsWhenSIPUnavailable SIP 未启动时必须静默跳过，且不能改坏状态。
func TestRenewDue_SkipsWhenSIPUnavailable(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		now := time.Now()
		expire := now.Add(30 * time.Second)
		sub := &model.GbSubscribe{
			BaseModel:  model.BaseModel{ID: -9302},
			GbDeviceID: "34020000001320000078",
			Type:       model.GbSubscribeAlarm,
			Expires:    3600,
			Status:     "active",
			CallID:     "keep-me",
			ExpireDT:   &expire,
		}
		if err := tx.Create(sub).Error; err != nil {
			t.Fatalf("创建订阅失败：%v", err)
		}

		prev := control.Default()
		control.Set(nil) // 模拟信令服务未启动
		defer control.Set(prev)
		n, err := gbsvc.NewGbSubscribeService().RenewDue(ctx)
		if err != nil {
			t.Fatalf("信令未启动时不应报错：%v", err)
		}
		if n != 0 {
			t.Fatalf("信令未启动时不应续订，实际 %d", n)
		}

		var got model.GbSubscribe
		if err := tx.First(&got, sub.ID).Error; err != nil {
			t.Fatalf("读取订阅失败：%v", err)
		}
		if got.Status != "active" || got.CallID != "keep-me" {
			t.Fatalf("信令未启动时不应改动订阅：status=%q call_id=%q", got.Status, got.CallID)
		}
	})
}

// TestRenewDue_MarksFailedAfterRetries 连续 3 次失败后置 failed，且不再重试。
func TestRenewDue_MarksFailedAfterRetries(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		now := time.Now()
		dev := &model.Device{
			BaseModel:  model.BaseModel{ID: -9203}, // 负数主键，避免共享测试库的序列漂移
			Name:       "续订失败测试设备",
			DeviceCode: "MAINT_DEV_79",
			GbDeviceID: "34020000001320000079",
			Status:     model.DeviceStatusOnline,
		}
		if err := tx.Create(dev).Error; err != nil {
			t.Fatalf("创建设备失败：%v", err)
		}
		expire := now.Add(30 * time.Second)
		sub := &model.GbSubscribe{
			BaseModel:  model.BaseModel{ID: -9303},
			DeviceID:   &dev.ID,
			GbDeviceID: dev.GbDeviceID,
			Type:       model.GbSubscribeMobilePosition,
			Expires:    3600,
			Status:     "active",
			ExpireDT:   &expire,
		}
		if err := tx.Create(sub).Error; err != nil {
			t.Fatalf("创建订阅失败：%v", err)
		}

		prev := control.Default()
		fake := &fakeController{failTimes: 3}
		control.Set(fake)
		defer control.Set(prev)

		svc := gbsvc.NewGbSubscribeService()
		for i := 0; i < 2; i++ {
			n, err := svc.RenewDue(ctx)
			if err != nil || n != 0 {
				t.Fatalf("第 %d 次续订应失败且不计数：n=%d err=%v", i+1, n, err)
			}
			var cur model.GbSubscribe
			_ = tx.First(&cur, sub.ID).Error
			if cur.Status != "active" {
				t.Fatalf("前两次失败后应保持 active，实际 %q", cur.Status)
			}
		}

		// 第三次失败 → failed
		if _, err := svc.RenewDue(ctx); err != nil {
			t.Fatalf("续订返回错误：%v", err)
		}
		var got model.GbSubscribe
		if err := tx.First(&got, sub.ID).Error; err != nil {
			t.Fatalf("读取订阅失败：%v", err)
		}
		if got.Status != "failed" {
			t.Fatalf("连续 3 次失败后应置 failed，实际 %q", got.Status)
		}

		// 置 failed 后不再进入续订窗口（查询只扫 active）。
		calls := len(fake.calls)
		if _, err := svc.RenewDue(ctx); err != nil {
			t.Fatalf("续订返回错误：%v", err)
		}
		if len(fake.calls) != calls {
			t.Fatalf("failed 订阅不应继续重发，实际多发了 %d 次", len(fake.calls)-calls)
		}
	})
}

// ════════════════════════════════════════════════════════════════
// 3. SSRC 周期重建
// ════════════════════════════════════════════════════════════════

// TestSSRCRebuild_Maintenance Rebuild 合入 ZLM 流列表后：
// 外部流被标记占用、本进程仍持有的流不释放、后续分配不重复。
func TestSSRCRebuild_Maintenance(t *testing.T) {
	a := ssrc.New("3402000000")

	// 本进程正在使用的两个 SSRC（模拟用户正在观看的流）。
	held1, err := a.Allocate(true)
	if err != nil {
		t.Fatalf("分配失败：%v", err)
	}
	held2, err := a.Allocate(true)
	if err != nil {
		t.Fatalf("分配失败：%v", err)
	}

	// ZLM 里还有一条本进程不认识的外部流（平台重启前的残留）。
	const external = "0200000042"
	externalHex, _ := ssrc.ToHexStream(external)
	a.Rebuild([]string{externalHex, "not-a-stream"})

	if !a.InUse(held1) || !a.InUse(held2) {
		t.Fatalf("Rebuild 不应释放本进程仍持有的流：%s/%s", held1, held2)
	}
	if !a.InUse(external) {
		t.Fatalf("ZLM 中的外部流应被标记占用：%s", external)
	}

	// 继续分配 50 次：不能重复、不能撞上外部流、不能分到已占用的槽位。
	seen := map[string]bool{held1: true, held2: true, external: true}
	for i := 0; i < 50; i++ {
		s, err := a.Allocate(true)
		if err != nil {
			t.Fatalf("第 %d 次分配失败：%v", i+1, err)
		}
		if seen[s] {
			t.Fatalf("分配到了重复/已占用的 SSRC：%s", s)
		}
		if !a.InUse(s) {
			t.Fatalf("分配后应处于占用状态：%s", s)
		}
		seen[s] = true
	}
}
