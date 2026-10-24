package device

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/model"
)

const (
	testDeviceID  = "34020000001320000001"
	testChannel1  = "34020000001320000001"
	testChannel2  = "34020000001320000002"
	testChannel3  = "34020000001320000003"
	testBadChanID = "99999999999999999999"
)

// fakeStore 内存实现，用于验证管理器写入的字段与调用顺序。
type fakeStore struct {
	mu        sync.Mutex
	device    *model.Device
	channels  map[string]*model.GbChannel
	logs      []model.GbSipLog
	alarms    []model.GbAlarm
	positions []model.GbMobilePosition
	expired   int64
	lookupErr error
}

func newFakeStore(dev *model.Device) *fakeStore {
	return &fakeStore{device: dev, channels: map[string]*model.GbChannel{}}
}

func (f *fakeStore) FindDevice(_ context.Context, gbDeviceID string) (*model.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if f.device == nil || f.device.GbDeviceID != gbDeviceID {
		return nil, errors.New("not found")
	}
	return f.device, nil
}

func (f *fakeStore) UpdateDevice(_ context.Context, id int64, fields map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.device == nil || f.device.ID != id {
		return errors.New("device not found")
	}
	for k, v := range fields {
		switch k {
		case "register_status":
			f.device.RegisterStatus = v.(string)
		case "status":
			f.device.Status = v.(string)
		case "sip_transport":
			f.device.SipTransport = v.(string)
		case "register_dt":
			t := v.(time.Time)
			f.device.RegisterDT = &t
		case "last_beat_dt":
			t := v.(time.Time)
			f.device.LastBeatDT = &t
		case "online_dt":
			t := v.(time.Time)
			f.device.OnlineDT = &t
		case "offline_dt":
			t := v.(time.Time)
			f.device.OfflineDT = &t
		case "sip_expires":
			if v == nil {
				f.device.SipExpires = nil
			} else {
				t := v.(time.Time)
				f.device.SipExpires = &t
			}
		case "channel_count":
			f.device.ChannelCount = v.(int64)
		case "gb_version":
			f.device.GbVersion = v.(string)
		}
	}
	return nil
}

func (f *fakeStore) FindChannel(_ context.Context, gbChannelID string) (*model.GbChannel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ok := f.channels[gbChannelID]
	if !ok {
		return nil, errors.New("channel not found")
	}
	return ch, nil
}

func (f *fakeStore) UpsertChannel(_ context.Context, ch *model.GbChannel) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channels[ch.GbChannelID] = ch
	return nil
}

func (f *fakeStore) UpdateChannel(_ context.Context, id int64, fields map[string]interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range f.channels {
		if ch.ID != id {
			continue
		}
		for k, v := range fields {
			switch k {
			case "status":
				ch.Status = v.(string)
			case "active":
				if b, ok := v.(bool); ok {
					ch.Active = &b
				}
			}
		}
		return nil
	}
	return errors.New("channel not found")
}

func (f *fakeStore) CountChannels(_ context.Context, deviceID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, ch := range f.channels {
		if ch.DeviceID == deviceID {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) CreateSipLog(_ context.Context, entry *model.GbSipLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, *entry)
	return nil
}

func (f *fakeStore) CreateAlarm(_ context.Context, alarm *model.GbAlarm) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alarms = append(f.alarms, *alarm)
	return nil
}

func (f *fakeStore) CreateMobilePosition(_ context.Context, pos *model.GbMobilePosition) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.positions = append(f.positions, *pos)
	return nil
}

func (f *fakeStore) MarkExpiredOffline(_ context.Context, _ time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.expired, nil
}

func newTestManager(store Store) *Manager {
	m := NewManager(store, Config{Domain: "3402000000", DefaultExpires: 3600}, nil)
	fixed := time.Unix(1_700_000_000, 0)
	m.SetNow(func() time.Time { return fixed })
	return m
}

func testDevice() *model.Device {
	return &model.Device{
		BaseModel:      model.BaseModel{ID: 7},
		DeviceCode:     "CAM_001",
		GbDeviceID:     testDeviceID,
		RegisterStatus: model.GbRegisterStatusPending,
		Status:         model.DeviceStatusOffline,
		Charset:        model.GbCharsetGB2312,
	}
}

// 注册：首次写 register_dt；重复注册不覆盖 register_dt，但刷新 sip_expires。
func TestOnRegisterAndUnregister(t *testing.T) {
	dev := testDevice()
	store := newFakeStore(dev)
	m := newTestManager(store)
	ctx := context.Background()

	if err := m.OnRegister(ctx, dev, "10.0.0.9:5060", "udp", 3600); err != nil {
		t.Fatalf("OnRegister 失败：%v", err)
	}
	if dev.RegisterStatus != model.GbRegisterStatusActivated {
		t.Fatalf("register_status = %q", dev.RegisterStatus)
	}
	if dev.Status != model.DeviceStatusOnline {
		t.Fatalf("status = %q", dev.Status)
	}
	if dev.RegisterDT == nil || dev.SipExpires == nil || dev.LastBeatDT == nil {
		t.Fatalf("register_dt / sip_expires / last_beat_dt 未写入：%+v", dev)
	}
	if dev.SipTransport != "UDP" {
		t.Fatalf("sip_transport = %q", dev.SipTransport)
	}
	firstRegister := *dev.RegisterDT
	if ep, ok := m.Endpoint(testDeviceID); !ok || ep.Addr != "10.0.0.9:5060" || ep.Transport != "UDP" {
		t.Fatalf("端点未记录：%+v ok=%v", ep, ok)
	}

	// 重注册：register_dt 保持不变，expires 前移。
	m.SetNow(func() time.Time { return time.Unix(1_700_001_000, 0) })
	if err := m.OnRegister(ctx, dev, "10.0.0.9:5061", "udp", 1800); err != nil {
		t.Fatalf("重注册失败：%v", err)
	}
	if !dev.RegisterDT.Equal(firstRegister) {
		t.Fatalf("register_dt 被重复注册覆盖：%v → %v", firstRegister, *dev.RegisterDT)
	}
	if want := time.Unix(1_700_001_000+1800, 0); !dev.SipExpires.Equal(want) {
		t.Fatalf("sip_expires = %v，期望 %v", dev.SipExpires, want)
	}

	// 注销：offline + sip_expires 清空 + 端点移除。
	if err := m.OnRegister(ctx, dev, "10.0.0.9:5061", "udp", 0); err != nil {
		t.Fatalf("注销失败：%v", err)
	}
	if dev.Status != model.DeviceStatusOffline || dev.SipExpires != nil {
		t.Fatalf("注销后状态错误：status=%q expires=%v", dev.Status, dev.SipExpires)
	}
	if _, ok := m.Endpoint(testDeviceID); ok {
		t.Fatalf("注销后端点应被移除")
	}
}

func TestOnKeepalive(t *testing.T) {
	dev := testDevice()
	store := newFakeStore(dev)
	m := newTestManager(store)

	if err := m.OnKeepalive(context.Background(), dev, "10.0.0.9:5060", "tcp"); err != nil {
		t.Fatalf("OnKeepalive 失败：%v", err)
	}
	if dev.Status != model.DeviceStatusOnline || dev.LastBeatDT == nil {
		t.Fatalf("心跳后状态未刷新：%+v", dev)
	}
	if ep, ok := m.Endpoint(testDeviceID); !ok || ep.Transport != "TCP" {
		t.Fatalf("心跳端点未记录：%+v", ep)
	}
}

func TestMarkRejected(t *testing.T) {
	dev := testDevice()
	m := newTestManager(newFakeStore(dev))
	if err := m.MarkRejected(context.Background(), dev); err != nil {
		t.Fatalf("MarkRejected 失败：%v", err)
	}
	if dev.RegisterStatus != model.GbRegisterStatusRejected {
		t.Fatalf("register_status = %q，期望 rejected", dev.RegisterStatus)
	}
}

// 目录分片重组：第一片不落库，收齐后一次落库并回写 channel_count。
func TestCatalogReassembly(t *testing.T) {
	dev := testDevice()
	store := newFakeStore(dev)
	m := newTestManager(store)
	ctx := context.Background()

	chunk1 := &manscdp.CatalogResponse{
		CmdType: manscdp.CmdCatalog, SN: 11, DeviceID: testDeviceID, SumNum: 3,
		DeviceList: manscdp.DeviceList{Num: 2, Items: []manscdp.CatalogItem{
			{DeviceID: testChannel1, Name: "东门球机", Manufacturer: "AI Camera", Model: "IPC-1", Status: "ON", RegisterWay: 1, PTZType: 1},
			{DeviceID: testChannel2, Name: "仓库枪机", Status: "OFF"},
		}},
	}
	done, err := m.OnCatalogResponse(ctx, dev, chunk1)
	if err != nil {
		t.Fatalf("第一片处理失败：%v", err)
	}
	if done {
		t.Fatalf("第一片未收齐，不应完成")
	}
	if len(store.channels) != 0 {
		t.Fatalf("未收齐时不应落库，实际 %d 条", len(store.channels))
	}

	chunk2 := &manscdp.CatalogResponse{
		CmdType: manscdp.CmdCatalog, SN: 11, DeviceID: testDeviceID, SumNum: 3,
		DeviceList: manscdp.DeviceList{Num: 3, Items: []manscdp.CatalogItem{
			// 重复项：应被去重
			{DeviceID: testChannel1, Name: "东门球机", Status: "ON"},
			{DeviceID: testChannel3, Name: "北门半球", Status: "ON"},
			// 域前缀不符：应跳过
			{DeviceID: testBadChanID, Name: "伪造通道", Status: "ON"},
		}},
	}
	done, err = m.OnCatalogResponse(ctx, dev, chunk2)
	if err != nil {
		t.Fatalf("第二片处理失败：%v", err)
	}
	if !done {
		t.Fatalf("收齐后应返回完成")
	}
	if len(store.channels) != 3 {
		t.Fatalf("落库通道数 = %d，期望 3（重复去重、伪造跳过）", len(store.channels))
	}
	if _, bad := store.channels[testBadChanID]; bad {
		t.Fatalf("域前缀不符的通道不应落库")
	}
	ch := store.channels[testChannel1]
	if ch.Name != "东门球机" || ch.Manufacturer != "AI Camera" || ch.PtzType != 1 {
		t.Fatalf("通道字段写入错误：%+v", ch)
	}
	if ch.Status != model.GbStatusOn {
		t.Fatalf("通道状态 = %q", ch.Status)
	}
	if store.channels[testChannel2].Status != model.GbStatusOff {
		t.Fatalf("OFF 通道状态未保留：%q", store.channels[testChannel2].Status)
	}
	if dev.ChannelCount != 3 {
		t.Fatalf("channel_count = %d，期望 3", dev.ChannelCount)
	}
	if ch.DeviceCode != dev.DeviceCode || ch.GbDeviceID != dev.GbDeviceID {
		t.Fatalf("冗余列未写入：%+v", ch)
	}
}

// 目录同步不能清空用户在本平台维护的区域/分组/备注。
func TestCatalogPreservesUserFields(t *testing.T) {
	dev := testDevice()
	store := newFakeStore(dev)
	regionID := int64(9)
	groupID := int64(5)
	store.channels[testChannel1] = &model.GbChannel{
		BaseModel:   model.BaseModel{ID: 88},
		DeviceID:    dev.ID,
		GbChannelID: testChannel1,
		RegionID:    &regionID,
		GroupID:     &groupID,
		Remark:      "用户备注",
	}
	m := newTestManager(store)

	resp := &manscdp.CatalogResponse{
		CmdType: manscdp.CmdCatalog, SN: 12, DeviceID: testDeviceID, SumNum: 1,
		DeviceList: manscdp.DeviceList{Num: 1, Items: []manscdp.CatalogItem{
			{DeviceID: testChannel1, Name: "改名后的通道", Status: "ON"},
		}},
	}
	if _, err := m.OnCatalogResponse(context.Background(), dev, resp); err != nil {
		t.Fatalf("目录处理失败：%v", err)
	}
	got := store.channels[testChannel1]
	if got.RegionID == nil || *got.RegionID != regionID {
		t.Fatalf("region_id 被目录同步清空：%v", got.RegionID)
	}
	if got.GroupID == nil || *got.GroupID != groupID {
		t.Fatalf("group_id 被目录同步清空：%v", got.GroupID)
	}
	if got.Remark != "用户备注" {
		t.Fatalf("备注被清空：%q", got.Remark)
	}
	if got.Name != "改名后的通道" {
		t.Fatalf("设备名未更新：%q", got.Name)
	}
}

// 空目录也要落一次 channel_count=0。
func TestEmptyCatalog(t *testing.T) {
	dev := testDevice()
	dev.ChannelCount = 9
	store := newFakeStore(dev)
	m := newTestManager(store)

	resp := &manscdp.CatalogResponse{
		CmdType: manscdp.CmdCatalog, SN: 13, DeviceID: testDeviceID, SumNum: 0,
		DeviceList: manscdp.DeviceList{Num: 0},
	}
	done, err := m.OnCatalogResponse(context.Background(), dev, resp)
	if err != nil || !done {
		t.Fatalf("空目录应立即完成：done=%v err=%v", done, err)
	}
	if len(store.channels) != 0 || dev.ChannelCount != 0 {
		t.Fatalf("空目录应把 channel_count 归零：channels=%d count=%d", len(store.channels), dev.ChannelCount)
	}
}

// 报警与移动位置落库。
func TestOnAlarmAndMobilePosition(t *testing.T) {
	dev := testDevice()
	store := newFakeStore(dev)
	m := newTestManager(store)
	ctx := context.Background()

	alarm := &manscdp.Alarm{
		DeviceID: testChannel1, AlarmPriority: "1", AlarmMethod: "5",
		AlarmTime: "2026-10-21T10:00:00", AlarmDescription: "移动侦测", AlarmType: "1",
		Longitude: 120.1, Latitude: 30.2,
	}
	if err := m.OnAlarm(ctx, dev, alarm); err != nil {
		t.Fatalf("OnAlarm 失败：%v", err)
	}
	if len(store.alarms) != 1 {
		t.Fatalf("报警未落库")
	}
	saved := store.alarms[0]
	if saved.AlarmMethod != 5 || saved.AlarmDescription != "移动侦测" || saved.AlarmPriority != "1" {
		t.Fatalf("报警字段错误：%+v", saved)
	}
	if saved.AlarmTime == nil || saved.AlarmTime.Format("2006-01-02 15:04:05") != "2026-10-21 10:00:00" {
		t.Fatalf("报警时间解析错误：%v", saved.AlarmTime)
	}
	if saved.DeviceID == nil || *saved.DeviceID != dev.ID {
		t.Fatalf("报警归属设备错误：%v", saved.DeviceID)
	}

	pos := &manscdp.MobilePosition{
		DeviceID: testChannel2, Time: "2026-10-21 10:00:01",
		Longitude: 120.3, Latitude: 30.4, Speed: 12.5, Direction: 90, Altitude: 10,
	}
	if err := m.OnMobilePosition(ctx, dev, pos); err != nil {
		t.Fatalf("OnMobilePosition 失败：%v", err)
	}
	if len(store.positions) != 1 {
		t.Fatalf("移动位置未落库")
	}
	if store.positions[0].Speed != 12.5 || store.positions[0].Longitude != 120.3 {
		t.Fatalf("移动位置字段错误：%+v", store.positions[0])
	}
}

// 信令日志异步写入；队列投递不阻塞主流程。
func TestLogSipAsync(t *testing.T) {
	dev := testDevice()
	store := newFakeStore(dev)
	m := newTestManager(store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	m.LogSip(model.GbSipLog{GbDeviceID: testDeviceID, Method: "REGISTER", Result: "success"})
	m.LogSip(model.GbSipLog{GbDeviceID: testDeviceID, Method: "MESSAGE", Result: "success"})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		store.mu.Lock()
		n := len(store.logs)
		store.mu.Unlock()
		if n == 2 {
			m.Stop()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.Stop()
	t.Fatalf("信令日志未在 2 秒内写入：%d 条", len(store.logs))
}

func TestEndpointIgnoreEmpty(t *testing.T) {
	m := newTestManager(newFakeStore(testDevice()))
	m.SetEndpoint("", "1.2.3.4:5060", "UDP")
	m.SetEndpoint(testDeviceID, "", "UDP")
	if _, ok := m.Endpoint(testDeviceID); ok {
		t.Fatalf("空地址不应记录端点")
	}
}

// TestOnCommandSeenUpgradesVersion 2022 独有命令应把设备版本升到 2022，且不回退。
func TestOnCommandSeenUpgradesVersion(t *testing.T) {
	dev := testDevice()
	dev.GbVersion = "2016"
	store := newFakeStore(dev)
	m := newTestManager(store)
	ctx := context.Background()

	// 2016 命令不升级
	if err := m.OnCommandSeen(ctx, dev, "Catalog"); err != nil {
		t.Fatalf("OnCommandSeen 失败：%v", err)
	}
	if dev.GbVersion != "2016" {
		t.Fatalf("普通命令不应升级版本，实际 %q", dev.GbVersion)
	}

	// 2022 独有命令升级
	if err := m.OnCommandSeen(ctx, dev, "CruiseTrackListQuery"); err != nil {
		t.Fatalf("OnCommandSeen 失败：%v", err)
	}
	if dev.GbVersion != "2022" {
		t.Fatalf("2022 命令应升级版本，实际 %q", dev.GbVersion)
	}
	if store.device.GbVersion != "2022" {
		t.Fatalf("版本应落库，实际 %q", store.device.GbVersion)
	}

	// 不再回退
	if err := m.OnCommandSeen(ctx, dev, "Catalog"); err != nil {
		t.Fatalf("OnCommandSeen 失败：%v", err)
	}
	if dev.GbVersion != "2022" {
		t.Fatalf("版本不应回退，实际 %q", dev.GbVersion)
	}
}

// TestOnCatalogNotifyEvents 目录通知的 ON/OFF/DEL 应更新通道状态，未知事件被忽略。
func TestOnCatalogNotifyEvents(t *testing.T) {
	dev := testDevice()
	store := newFakeStore(dev)
	active := true
	ch := &model.GbChannel{
		BaseModel:   model.BaseModel{ID: 11},
		DeviceID:    dev.ID,
		GbDeviceID:  dev.GbDeviceID,
		GbChannelID: testChannel1,
		Status:      model.GbStatusOn,
		Active:      &active,
	}
	store.channels[testChannel1] = ch
	m := newTestManager(store)
	ctx := context.Background()

	notify := &manscdp.CatalogNotify{
		CmdType: manscdp.CmdCatalog,
		DeviceList: manscdp.DeviceList{
			Num: 1,
			Items: []manscdp.CatalogItem{{
				DeviceID: testChannel1,
				Event:    manscdp.CatalogEventOff,
			}},
		},
	}
	if err := m.OnCatalogNotify(ctx, dev, notify); err != nil {
		t.Fatalf("OnCatalogNotify 失败：%v", err)
	}
	if ch.Status != model.GbStatusOff {
		t.Fatalf("OFF 事件应把状态置为 OFF，实际 %q", ch.Status)
	}

	notify.DeviceList.Items[0].Event = "UNKNOWN_EVENT"
	if err := m.OnCatalogNotify(ctx, dev, notify); err != nil {
		t.Fatalf("未知事件不应返回错误：%v", err)
	}
	if ch.Status != model.GbStatusOff {
		t.Fatalf("未知事件不应改变状态，实际 %q", ch.Status)
	}

	notify.DeviceList.Items[0].Event = manscdp.CatalogEventDel
	if err := m.OnCatalogNotify(ctx, dev, notify); err != nil {
		t.Fatalf("OnCatalogNotify 失败：%v", err)
	}
	if ch.Status != model.GbStatusOff || ch.Active == nil || *ch.Active {
		t.Fatalf("DEL 应置 OFF 且 active=false，实际 status=%q active=%v", ch.Status, ch.Active)
	}
}
