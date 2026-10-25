// Package device 负责 GB28181 设备的注册、心跳、目录同步与信令日志。
//
// 状态模型（设计 §4.2）：
//
//	REGISTER 成功 → register_status='activated'、status='online'、sip_expires=now+expires
//	Keepalive     → 刷新 last_beat_dt/status
//	sip_expires 到期 → status='offline'（不删除设备，也不删通道）
//	心跳超时       → status='offline'（now-last_beat_dt > 3×心跳间隔）
//
// 两条离线判定（sip_expires 过期 / 心跳超时）取**先到者**：注册有效期通常 3600s，
// 而心跳 60s 一跳，设备掉电/断网时心跳超时会先命中，离线状态因此能在三分钟内
// 反映出来，而不是等注册有效期走完。
//
// 目录同步按 (deviceID, SN) 重组多条 MESSAGE（设计 §6.2 #3）：
// 设备把 SumNum 条通道拆成若干片，每片复用同一个 SN，收齐后才落库，
// 避免半份目录覆盖出错误的 channel_count。
package device

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/version"
	"github.com/boticz/ai-camera-backend/internal/model"
)

// Config 是设备管理器的参数。
type Config struct {
	// Domain 是 SIP 域（10 位），用于校验目录项是否落在本域前缀内。
	Domain string
	// DefaultExpires 是设备未声明 Expires 时的缺省注册有效期（秒）。
	DefaultExpires int
	// CatalogTimeout 是目录分片重组的最长等待时间，超时丢弃半份目录。
	CatalogTimeout time.Duration
	// SweepInterval 是过期扫描与半份目录清理的周期。
	SweepInterval time.Duration
	// HeartbeatInterval 是设备心跳间隔的缺省值（秒级时长）。
	//
	// GB28181 设备的 Keepalive 报文本身**不带间隔字段**（manscdp.Keepalive 只有
	// SN/DeviceID/Status），因此无法从报文里读到真实间隔，统一按行业惯例的 60s 处理。
	HeartbeatInterval time.Duration
	// HeartbeatTimeoutFactor 是心跳超时倍数，缺省 3。
	//
	// 为什么是 3 倍而不是 1 倍：一次心跳丢失可能只是 UDP 丢包、设备 GC 抖动或
	// 平台短暂过载，1 倍阈值会把这类瞬时异常误判为离线；2 倍在 60s 心跳下只有
	// 60s 余量，仍偏紧。3 倍（180s）既能容忍连续丢两跳，又能让掉电设备在 3 分钟
	// 内变为离线，是 WVP 等实现的常用取值。
	HeartbeatTimeoutFactor int
	// LogBuffer 是信令日志写入队列长度；满时丢弃并计数，绝不阻塞主流程。
	LogBuffer int
}

const (
	// DefaultHeartbeatInterval 缺省心跳间隔：60 秒。
	DefaultHeartbeatInterval = 60 * time.Second
	// DefaultHeartbeatTimeoutFactor 缺省心跳超时倍数：3 倍。
	DefaultHeartbeatTimeoutFactor = 3
)

// heartbeatTimeoutStore 是 Store 的**可选能力**。
//
// 为什么用类型断言而不是直接扩 Store：Store 是给单测注入的窄接口，已有的
// fake 实现只关心注册/目录/日志；把「按心跳批量置离线」塞进必需方法会迫使
// 所有测试桩跟着改，且它只服务于一个后台扫描。生产实现（GbStore）实现了本接口。
type heartbeatTimeoutStore interface {
	MarkHeartbeatTimeoutOffline(ctx context.Context, now time.Time, timeout time.Duration) (int64, error)
}

// Endpoint 是设备最近一次有效报文的源地址。
//
// 为什么放内存而不落库：ac_device 没有 host_address 列，且一期 SIP 固定单节点
// （设计 §2.4）。进程重启后设备的下一条 REGISTER/Keepalive 会自动补齐。
type Endpoint struct {
	Addr      string
	Transport string
	UpdatedAt time.Time
}

// Manager 管理设备运行时状态。
type Manager struct {
	store Store
	cfg   Config
	log   *slog.Logger
	now   func() time.Time

	mu        sync.Mutex
	endpoints map[string]Endpoint
	catalogs  map[string]*catalogAccum

	logCh   chan model.GbSipLog
	dropped int64

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup
	started  bool
}

// catalogAccum 是一份尚未收齐的目录。
type catalogAccum struct {
	sum   int
	items []manscdp.CatalogItem
	last  time.Time
}

// CatalogSyncState 是「设备目录同步」的进程内快照。
//
// 为什么是进程级而不是 Manager 字段：HTTP 业务进程与信令进程可能不在同一进程，
// 但 WVP 对齐接口 GET /web/gb/devices/:id/sync-status 需要读取它。放包级状态后，
// 单进程（embedded）部署天然可用；分进程部署时 HTTP 进程返回 idle —— 这是
// 明确接受的行为（同步进度本来就是运维辅助信息，不落库）。
type CatalogSyncState struct {
	Status     string    // syncing / success / failed
	Total      int       // 设备声明的总条数（SumNum；未知时为当前已收条数）
	Current    int       // 已接收条数
	Error      string    // 失败原因
	StartedAt  time.Time // 本轮开始时间
	FinishedAt time.Time // 本轮结束时间（未结束为零值）
}

// 目录同步状态取值。
const (
	CatalogSyncStatusSyncing = "syncing"
	CatalogSyncStatusSuccess = "success"
	CatalogSyncStatusFailed  = "failed"
)

type catalogSyncStore struct {
	mu    sync.RWMutex
	items map[string]CatalogSyncState
}

var catalogSyncStates = &catalogSyncStore{items: map[string]CatalogSyncState{}}

// CatalogSyncStatusOf 返回设备最近一次目录同步的状态快照。
// 第二个返回值为 false 表示本进程还没有任何同步记录。
func CatalogSyncStatusOf(gbDeviceID string) (CatalogSyncState, bool) {
	if gbDeviceID == "" {
		return CatalogSyncState{}, false
	}
	catalogSyncStates.mu.RLock()
	defer catalogSyncStates.mu.RUnlock()
	st, ok := catalogSyncStates.items[gbDeviceID]
	return st, ok
}

// MarkCatalogSyncRequested 标记「平台已下发 Catalog 查询、等待设备响应」。
//
// HTTP 层触发同步时调用：如果不先标记，前端在设备响应第一个分片之前
// 只会看到上一轮的 success/idle，误以为本次同步没有开始。
func MarkCatalogSyncRequested(gbDeviceID string, now time.Time) {
	if gbDeviceID == "" {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	catalogSyncStates.mu.Lock()
	defer catalogSyncStates.mu.Unlock()
	catalogSyncStates.items[gbDeviceID] = CatalogSyncState{
		Status:    CatalogSyncStatusSyncing,
		StartedAt: now,
	}
}

// MarkCatalogSyncFailed 记录一次在「下发阶段」就失败的同步。
func MarkCatalogSyncFailed(gbDeviceID, reason string, now time.Time) {
	if gbDeviceID == "" {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	catalogSyncStates.mu.Lock()
	defer catalogSyncStates.mu.Unlock()
	st := catalogSyncStates.items[gbDeviceID]
	if st.StartedAt.IsZero() {
		st.StartedAt = now
	}
	st.Status = CatalogSyncStatusFailed
	st.Error = reason
	st.FinishedAt = now
	catalogSyncStates.items[gbDeviceID] = st
}

// markCatalogSyncing 标记同步开始/进度。首次调用或上一轮已结束时重置开始时间。
func markCatalogSyncing(gbDeviceID string, total, current int, now time.Time) {
	catalogSyncStates.mu.Lock()
	defer catalogSyncStates.mu.Unlock()
	st, ok := catalogSyncStates.items[gbDeviceID]
	if !ok || st.Status != CatalogSyncStatusSyncing {
		st = CatalogSyncState{StartedAt: now}
	}
	st.Status = CatalogSyncStatusSyncing
	st.Total = total
	st.Current = current
	st.Error = ""
	st.FinishedAt = time.Time{}
	catalogSyncStates.items[gbDeviceID] = st
}

// markCatalogFinished 标记同步结束（成功或失败）。
func markCatalogFinished(gbDeviceID, status, reason string, total, current int, now time.Time) {
	catalogSyncStates.mu.Lock()
	defer catalogSyncStates.mu.Unlock()
	st := catalogSyncStates.items[gbDeviceID]
	if st.StartedAt.IsZero() {
		st.StartedAt = now
	}
	st.Status = status
	st.Total = total
	st.Current = current
	st.Error = reason
	st.FinishedAt = now
	catalogSyncStates.items[gbDeviceID] = st
}

// NewManager 构造设备管理器。
func NewManager(store Store, cfg Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.DefaultExpires <= 0 {
		cfg.DefaultExpires = 3600
	}
	if cfg.CatalogTimeout <= 0 {
		cfg.CatalogTimeout = 60 * time.Second
	}
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = 30 * time.Second
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = DefaultHeartbeatInterval
	}
	if cfg.HeartbeatTimeoutFactor <= 0 {
		cfg.HeartbeatTimeoutFactor = DefaultHeartbeatTimeoutFactor
	}
	if cfg.LogBuffer <= 0 {
		cfg.LogBuffer = 1024
	}
	return &Manager{
		store:     store,
		cfg:       cfg,
		log:       logger,
		now:       time.Now,
		endpoints: map[string]Endpoint{},
		catalogs:  map[string]*catalogAccum{},
		logCh:     make(chan model.GbSipLog, cfg.LogBuffer),
		stopCh:    make(chan struct{}),
	}
}

// SetNow 覆盖时钟（单测用）。
func (m *Manager) SetNow(f func() time.Time) {
	if f != nil {
		m.now = f
	}
}

// Start 启动日志写入与过期扫描的后台协程。可重复调用。
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return
	}
	m.started = true
	m.mu.Unlock()

	m.wg.Add(2)
	go m.logWorker(ctx)
	go m.sweepLoop(ctx)
}

// Stop 停止后台协程并尽量把队列里的日志写完。
func (m *Manager) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
	m.wg.Wait()
}

// Endpoint 返回设备最近的源地址。
func (m *Manager) Endpoint(gbDeviceID string) (Endpoint, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ep, ok := m.endpoints[gbDeviceID]
	return ep, ok
}

// SetEndpoint 记录设备的源地址（REGISTER / Keepalive / 任意入向报文）。
func (m *Manager) SetEndpoint(gbDeviceID, addr, transport string) {
	if gbDeviceID == "" || addr == "" {
		return
	}
	m.mu.Lock()
	m.endpoints[gbDeviceID] = Endpoint{
		Addr:      addr,
		Transport: strings.ToUpper(transport),
		UpdatedAt: m.now(),
	}
	m.mu.Unlock()
}

// dropEndpoint 设备注销时清掉源地址。
func (m *Manager) dropEndpoint(gbDeviceID string) {
	m.mu.Lock()
	delete(m.endpoints, gbDeviceID)
	m.mu.Unlock()
}

// OnRegister 处理注册成功。
//
// 首次注册写 register_dt（只写一次）；每次成功注册都刷新 sip_expires 与在线状态。
func (m *Manager) OnRegister(ctx context.Context, dev *model.Device, remote, transport string, expires int) error {
	if dev == nil {
		return nil
	}
	if expires <= 0 {
		return m.OnUnregister(ctx, dev)
	}
	now := m.now()
	fields := map[string]interface{}{
		"register_status": model.GbRegisterStatusActivated,
		"sip_expires":     now.Add(time.Duration(expires) * time.Second),
		"status":          model.DeviceStatusOnline,
		"last_beat_dt":    now,
	}
	if transport != "" {
		fields["sip_transport"] = strings.ToUpper(transport)
	}
	if dev.RegisterDT == nil {
		// 首次成功注册即「激活」时刻，后续重注册不覆盖。
		fields["register_dt"] = now
	}
	if dev.Status != model.DeviceStatusOnline {
		fields["online_dt"] = now
	}
	if err := m.store.UpdateDevice(ctx, dev.ID, fields); err != nil {
		return err
	}

	// 同步内存副本，避免同一次请求后续逻辑读到旧值。
	dev.RegisterStatus = model.GbRegisterStatusActivated
	dev.Status = model.DeviceStatusOnline
	exp := now.Add(time.Duration(expires) * time.Second)
	dev.SipExpires = &exp
	dev.LastBeatDT = &now
	if dev.RegisterDT == nil {
		dev.RegisterDT = &now
	}
	if transport != "" {
		dev.SipTransport = strings.ToUpper(transport)
	}
	m.SetEndpoint(dev.GbDeviceID, remote, transport)
	return nil
}

// OnUnregister 处理 Expires: 0 的注销。
func (m *Manager) OnUnregister(ctx context.Context, dev *model.Device) error {
	if dev == nil {
		return nil
	}
	now := m.now()
	fields := map[string]interface{}{
		"status":      model.DeviceStatusOffline,
		"sip_expires": nil,
		"offline_dt":  now,
	}
	if err := m.store.UpdateDevice(ctx, dev.ID, fields); err != nil {
		return err
	}
	dev.Status = model.DeviceStatusOffline
	dev.SipExpires = nil
	m.dropEndpoint(dev.GbDeviceID)
	return nil
}

// MarkRejected 记录一次鉴权失败（设备已建档但 Digest 不通过）。
//
// 只写 register_status，不改 online/offline：设备可能仍在发心跳，
// 直接判离线会与心跳逻辑打架；register_status=rejected 已足以让管理员看见问题。
func (m *Manager) MarkRejected(ctx context.Context, dev *model.Device) error {
	if dev == nil {
		return nil
	}
	if err := m.store.UpdateDevice(ctx, dev.ID, map[string]interface{}{
		"register_status": model.GbRegisterStatusRejected,
	}); err != nil {
		return err
	}
	dev.RegisterStatus = model.GbRegisterStatusRejected
	return nil
}

// OnKeepalive 处理心跳：刷新 last_beat_dt 与在线状态。
//
// 心跳不延长 sip_expires —— 注册有效期只由 REGISTER 决定（设计 §4.2）。
func (m *Manager) OnKeepalive(ctx context.Context, dev *model.Device, remote, transport string) error {
	if dev == nil {
		return nil
	}
	now := m.now()
	fields := map[string]interface{}{
		"last_beat_dt": now,
		"status":       model.DeviceStatusOnline,
	}
	if dev.Status != model.DeviceStatusOnline {
		fields["online_dt"] = now
	}
	if err := m.store.UpdateDevice(ctx, dev.ID, fields); err != nil {
		return err
	}
	dev.Status = model.DeviceStatusOnline
	dev.LastBeatDT = &now
	m.SetEndpoint(dev.GbDeviceID, remote, transport)
	return nil
}

// OnDeviceStatus 处理 DeviceStatus 查询响应。
//
// 只有设备明确给了 Online 取值时才改状态，避免把「响应缺字段」误判为离线。
func (m *Manager) OnDeviceStatus(ctx context.Context, dev *model.Device, resp *manscdp.DeviceStatusResponse) error {
	if dev == nil || resp == nil || resp.Online == "" {
		return nil
	}
	now := m.now()
	online := strings.EqualFold(resp.Online, "ONLINE")
	fields := map[string]interface{}{"last_beat_dt": now}
	if online {
		fields["status"] = model.DeviceStatusOnline
		if dev.Status != model.DeviceStatusOnline {
			fields["online_dt"] = now
		}
	} else {
		fields["status"] = model.DeviceStatusOffline
		fields["offline_dt"] = now
	}
	if err := m.store.UpdateDevice(ctx, dev.ID, fields); err != nil {
		return err
	}
	if online {
		dev.Status = model.DeviceStatusOnline
	} else {
		dev.Status = model.DeviceStatusOffline
	}
	dev.LastBeatDT = &now
	return nil
}

// OnCatalogResponse 接收一片目录响应；返回是否已收齐并落库。
func (m *Manager) OnCatalogResponse(ctx context.Context, dev *model.Device, resp *manscdp.CatalogResponse) (bool, error) {
	if dev == nil || resp == nil {
		return false, nil
	}
	key := dev.GbDeviceID + "#" + strconv.Itoa(resp.SN)
	now := m.now()

	m.mu.Lock()
	acc := m.catalogs[key]
	if acc == nil {
		acc = &catalogAccum{}
		m.catalogs[key] = acc
	}
	if resp.SumNum > 0 {
		acc.sum = resp.SumNum
	}
	acc.items = append(acc.items, resp.DeviceList.Items...)
	acc.last = now

	// 同步进度：每个分片都刷新，前端轮询 sync-status 能看到「20/32」。
	syncTotal := acc.sum
	if syncTotal <= 0 {
		syncTotal = len(acc.items)
	}
	markCatalogSyncing(dev.GbDeviceID, syncTotal, len(acc.items), now)

	complete := false
	switch {
	case acc.sum > 0:
		complete = len(acc.items) >= acc.sum
	case resp.DeviceList.Num == 0 && len(resp.DeviceList.Items) == 0:
		// 空目录也要落一次，否则 channel_count 会保留上一次的旧值。
		complete = true
	case resp.DeviceList.Num > 0:
		complete = len(resp.DeviceList.Items) >= resp.DeviceList.Num
	}
	if !complete {
		m.mu.Unlock()
		return false, nil
	}
	items := acc.items
	total := acc.sum
	if total <= 0 {
		total = len(items)
	}
	delete(m.catalogs, key)
	m.mu.Unlock()

	if err := m.persistCatalog(ctx, dev, items); err != nil {
		markCatalogFinished(dev.GbDeviceID, CatalogSyncStatusFailed, err.Error(), total, len(items), m.now())
		return true, err
	}
	markCatalogFinished(dev.GbDeviceID, CatalogSyncStatusSuccess, "", total, len(items), m.now())
	return true, nil
}

// persistCatalog 把收齐的目录项写入 ac_gb_channel，并回写 channel_count。
func (m *Manager) persistCatalog(ctx context.Context, dev *model.Device, items []manscdp.CatalogItem) error {
	now := m.now()
	seen := make(map[string]struct{}, len(items))
	saved := 0
	skipped := 0

	for _, item := range items {
		id := strings.TrimSpace(item.DeviceID)
		if id == "" {
			skipped++
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}

		// §4.4 第 3 条：目录项必须落在本设备的域前缀内，否则跳过并记日志，
		// 不整体拒绝 —— 个别设备会混入上级平台/分组的编码，整体拒绝会丢整份目录。
		if !m.acceptChannelID(dev, id) {
			skipped++
			m.log.Warn("目录项不在设备域前缀内，已跳过",
				"gb_device_id", dev.GbDeviceID, "channel_id", id, "sn_source", "catalog")
			continue
		}

		ch := &model.GbChannel{
			DeviceID:     dev.ID,
			DeviceCode:   dev.DeviceCode,
			GbDeviceID:   dev.GbDeviceID,
			GbChannelID:  id,
			Name:         item.Name,
			Manufacturer: item.Manufacturer,
			Model:        item.Model,
			Owner:        item.Owner,
			CivilCode:    item.CivilCode,
			Address:      item.Address,
			Parental:     int64(item.Parental),
			ParentID:     item.ParentID,
			ChannelType:  int64(item.ChannelType),
			PtzType:      int64(item.PTZType),
			RegisterWay:  normalizeRegisterWay(item.RegisterWay),
			Status:       normalizeChannelStatus(item.Status),
			Longitude:    item.Longitude,
			Latitude:     item.Latitude,
			SyncDT:       &now,
			DeptID:       dev.DeptID,
		}
		active := true
		ch.Active = &active

		// 设备上报只覆盖设备侧字段；用户在本平台改过的区域/分组/备注等必须保留，
		// 否则每次目录同步都会把用户的整理工作清空。
		if existing, err := m.store.FindChannel(ctx, id); err == nil && existing != nil {
			ch.RegionID = existing.RegionID
			ch.GroupID = existing.GroupID
			ch.RecordPlanID = existing.RecordPlanID
			ch.EnableBroadcast = existing.EnableBroadcast
			ch.IsCustom = existing.IsCustom
			ch.Remark = existing.Remark
			if existing.Active != nil {
				ch.Active = existing.Active
			}
		}

		if err := m.store.UpsertChannel(ctx, ch); err != nil {
			m.log.Error("写入目录通道失败", "channel_id", id, "error", err)
			continue
		}
		saved++
	}

	count, err := m.store.CountChannels(ctx, dev.ID)
	if err != nil {
		return err
	}
	if err := m.store.UpdateDevice(ctx, dev.ID, map[string]interface{}{"channel_count": count}); err != nil {
		return err
	}
	dev.ChannelCount = count

	m.log.Info("目录同步完成",
		"gb_device_id", dev.GbDeviceID, "items", len(items),
		"saved", saved, "skipped", skipped, "channel_count", count)
	return nil
}

// acceptChannelID 校验目录项归属。
//
// GB28181 的 20 位编码前 10 位是 SIP 域；同一设备的通道必须落在同一域下。
// 设备编码本身可能是「设备」也可能是「通道」，两种都接受。
func (m *Manager) acceptChannelID(dev *model.Device, channelID string) bool {
	if len(channelID) != 20 {
		return false
	}
	if channelID == dev.GbDeviceID {
		return true
	}
	prefix := strings.TrimSpace(m.cfg.Domain)
	if len(dev.GbDeviceID) >= 10 {
		prefix = dev.GbDeviceID[:10]
	}
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(channelID, prefix)
}

// OnAlarm 处理设备主动上报的报警，落 ac_gb_alarm。
func (m *Manager) OnAlarm(ctx context.Context, dev *model.Device, msg *manscdp.Alarm) error {
	if dev == nil || msg == nil {
		return nil
	}
	gbChannelID := strings.TrimSpace(msg.DeviceID)
	var channelID *int64
	if gbChannelID != "" && gbChannelID != dev.GbDeviceID {
		if ch, err := m.store.FindChannel(ctx, gbChannelID); err == nil && ch != nil {
			id := ch.ID
			channelID = &id
		}
	}
	method, _ := strconv.ParseInt(strings.TrimSpace(msg.AlarmMethod), 10, 64)
	alarmTime := parseDeviceTime(msg.AlarmTime, m.now())
	deviceID := dev.ID
	alarm := &model.GbAlarm{
		DeviceID:         &deviceID,
		ChannelID:        channelID,
		GbDeviceID:       dev.GbDeviceID,
		GbChannelID:      gbChannelID,
		AlarmPriority:    msg.AlarmPriority,
		AlarmMethod:      method,
		AlarmType:        msg.AlarmType,
		AlarmTime:        &alarmTime,
		AlarmDescription: msg.AlarmDescription,
		Longitude:        msg.Longitude,
		Latitude:         msg.Latitude,
		DeptID:           dev.DeptID,
	}
	if err := m.store.CreateAlarm(ctx, alarm); err != nil {
		return err
	}
	m.log.Info("收到国标报警", "gb_device_id", dev.GbDeviceID,
		"channel_id", gbChannelID, "priority", msg.AlarmPriority, "type", msg.AlarmType)
	return nil
}

// OnMobilePosition 处理移动位置上报，落 ac_gb_mobile_position。
func (m *Manager) OnMobilePosition(ctx context.Context, dev *model.Device, msg *manscdp.MobilePosition) error {
	if dev == nil || msg == nil {
		return nil
	}
	gbChannelID := strings.TrimSpace(msg.DeviceID)
	var channelID *int64
	if gbChannelID != "" && gbChannelID != dev.GbDeviceID {
		if ch, err := m.store.FindChannel(ctx, gbChannelID); err == nil && ch != nil {
			id := ch.ID
			channelID = &id
		}
	}
	reportTime := parseDeviceTime(msg.Time, m.now())
	deviceID := dev.ID
	pos := &model.GbMobilePosition{
		DeviceID:    &deviceID,
		ChannelID:   channelID,
		GbChannelID: gbChannelID,
		ReportTime:  &reportTime,
		Longitude:   msg.Longitude,
		Latitude:    msg.Latitude,
		Altitude:    msg.Altitude,
		Speed:       msg.Speed,
		Direction:   msg.Direction,
		DeptID:      dev.DeptID,
	}
	return m.store.CreateMobilePosition(ctx, pos)
}

// LogSip 把一条信令日志投入异步队列。
//
// 设计约束：信令日志写入失败**不能影响主流程**。因此这里只做非阻塞投递，
// 队列满时丢弃并累加计数（由调用方在退出前读 DroppedLogs 记录一条告警）。
func (m *Manager) LogSip(entry model.GbSipLog) {
	if entry.CreatedDT.IsZero() {
		entry.CreatedDT = m.now()
	}
	if entry.Direction == "" {
		entry.Direction = "in"
	}
	select {
	case m.logCh <- entry:
	default:
		m.mu.Lock()
		m.dropped++
		m.mu.Unlock()
	}
}

// DroppedLogs 返回因队列满被丢弃的日志条数。
func (m *Manager) DroppedLogs() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dropped
}

// ExpireOnce 执行一次过期扫描，返回置为离线的设备数。
func (m *Manager) ExpireOnce(ctx context.Context) (int64, error) {
	return m.store.MarkExpiredOffline(ctx, m.now())
}

// HeartbeatTimeout 返回当前配置下的心跳超时时长（间隔 × 倍数）。
func (m *Manager) HeartbeatTimeout() time.Duration {
	interval := m.cfg.HeartbeatInterval
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}
	factor := m.cfg.HeartbeatTimeoutFactor
	if factor <= 0 {
		factor = DefaultHeartbeatTimeoutFactor
	}
	return interval * time.Duration(factor)
}

// HeartbeatOnce 执行一次心跳超时扫描，返回置为离线的设备数。
//
// 判定条件：now - last_beat_dt > timeout（**严格大于**）。
// 边界上（恰好等于 3×间隔）仍视为在线：扫描本身 30s 一轮，卡在边界的设备
// 下一轮必然命中，没必要在等号上抢跑 —— 宁可晚 30s 判离线，也不要因为时钟
// 精度/取整把一条刚发来的心跳判成超时。
//
// 同时清理内存 Endpoint：设备已判离线却还留着源地址，PTZ/查询等指令会继续
// 往一个死地址发，每次都要白等一个 SIP 超时。
func (m *Manager) HeartbeatOnce(ctx context.Context) (int64, error) {
	timeout := m.HeartbeatTimeout()
	now := m.now()

	store, ok := m.store.(heartbeatTimeoutStore)
	if !ok {
		// 测试桩未实现该能力时静默跳过，不影响其他状态维护。
		return 0, nil
	}
	n, err := store.MarkHeartbeatTimeoutOffline(ctx, now, timeout)
	if err != nil {
		return 0, err
	}
	m.dropStaleEndpoints(now.Add(-timeout))
	return n, nil
}

// dropStaleEndpoints 清掉超过心跳超时窗口没有更新的源地址。
//
// 用内存里的 UpdatedAt 而不是 liveness 状态：任何入向报文（REGISTER/Keepalive/
// 目录响应）都会刷新它，正好等价于「这条地址还活着」。
func (m *Manager) dropStaleEndpoints(deadline time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, ep := range m.endpoints {
		if ep.UpdatedAt.Before(deadline) {
			delete(m.endpoints, id)
		}
	}
}

// logWorker 串行写日志：单写者既避免并发写库压力，也保证日志顺序与报文顺序一致。
func (m *Manager) logWorker(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			m.drainLogs()
			return
		case <-m.stopCh:
			m.drainLogs()
			return
		case entry := <-m.logCh:
			m.writeLog(entry)
		}
	}
}

// drainLogs 退出前尽量写完剩余日志，最多等 3 秒。
func (m *Manager) drainLogs() {
	deadline := time.After(3 * time.Second)
	for {
		select {
		case entry := <-m.logCh:
			m.writeLog(entry)
		case <-deadline:
			if n := m.DroppedLogs(); n > 0 {
				m.log.Warn("信令日志队列有丢弃", "dropped", n)
			}
			return
		default:
			return
		}
	}
}

func (m *Manager) writeLog(entry model.GbSipLog) {
	// 兜底 recover：这是后台协程，任何 panic 都会终止整个信令进程。
	// 日志是排障手段，不能反过来成为故障源（设计要求「失败不能影响主流程」）。
	defer func() {
		if r := recover(); r != nil {
			m.log.Error("写入信令日志时发生 panic，已忽略", "panic", r, "method", entry.Method)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := m.store.CreateSipLog(ctx, &entry); err != nil {
		// 日志表写失败只告警，不重试、不影响信令。
		m.log.Warn("写入 ac_gb_sip_log 失败", "error", err, "method", entry.Method, "call_id", entry.CallID)
	}
}

// sweepLoop 周期执行过期扫描与半份目录清理。
func (m *Manager) sweepLoop(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(m.cfg.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if n, err := m.ExpireOnce(ctx); err != nil {
				m.log.Warn("注册过期扫描失败", "error", err)
			} else if n > 0 {
				m.log.Info("注册过期，设备置为离线", "count", n)
			}
			if n, err := m.HeartbeatOnce(ctx); err != nil {
				m.log.Warn("心跳超时扫描失败", "error", err)
			} else if n > 0 {
				m.log.Info("心跳超时，设备置为离线",
					"count", n, "timeout", m.HeartbeatTimeout().String())
			}
			m.dropStaleCatalogs()
		}
	}
}

// dropStaleCatalogs 丢弃超时未收齐的目录分片，避免内存无限增长。
func (m *Manager) dropStaleCatalogs() {
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, acc := range m.catalogs {
		if now.Sub(acc.last) > m.cfg.CatalogTimeout {
			m.log.Warn("目录分片超时未收齐，已丢弃",
				"key", key, "received", len(acc.items), "expected", acc.sum)
			// key 形如 "<gbDeviceID>#<SN>"；超时同样要落到 sync-status，
			// 否则前端会一直显示 syncing。
			if idx := strings.LastIndex(key, "#"); idx > 0 {
				total := acc.sum
				if total <= 0 {
					total = len(acc.items)
				}
				markCatalogFinished(key[:idx], CatalogSyncStatusFailed,
					"目录分片超时未收齐", total, len(acc.items), now)
			}
			delete(m.catalogs, key)
		}
	}
}

// normalizeChannelStatus 把设备上报的通道状态统一成 ON/OFF。
func normalizeChannelStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "OFF", "OFFLINE", "0":
		return model.GbStatusOff
	default:
		return model.GbStatusOn
	}
}

// parseDeviceTime 解析设备时间；常见格式有带 T 与带空格两种，解析失败用兜底时间。
func parseDeviceTime(s string, fallback time.Time) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return fallback
}

// OnCommandSeen 在收到任何 MANSCDP 命令后，按命令能力升级设备协议版本。
//
// 为什么需要自动升级：设备的 REGISTER 头里没有可靠的版本声明，而 2022 独有命令
// （PTZPosition、CruiseTrackListQuery…）本身就证明了设备至少支持 2022。
// 版本一旦升到 2022 就不再降级 —— 设备固件降级的场景极少，反复升降会让
// 「支持哪些能力」变得不可预测。
func (m *Manager) OnCommandSeen(ctx context.Context, dev *model.Device, cmdType string) error {
	if dev == nil {
		return nil
	}
	current := version.Normalize(dev.GbVersion)
	next := version.UpgradeOnCommand(current, cmdType)
	if next == current {
		return nil
	}
	if err := m.store.UpdateDevice(ctx, dev.ID, map[string]interface{}{
		"gb_version": string(next),
	}); err != nil {
		return err
	}
	dev.GbVersion = string(next)
	m.log.Info("设备协议版本已升级",
		"gb_device_id", dev.GbDeviceID, "from", string(current), "to", string(next),
		"trigger_cmd", cmdType)
	return nil
}

// OnCatalogNotify 处理目录订阅通知（Notify/Catalog）的增量变更。
//
// 与全量 Catalog 响应的区别：Notify 只带变化的条目，且通常带 Event。
// 不能走 OnCatalogResponse 的「按 SumNum 等齐」逻辑，否则永远等不到。
//
// 删除语义：DEL 只把通道置为 inactive + OFF，**不做物理删除** ——
// 历史报警/录像/位置仍引用该通道，物理删除会让这些记录失去归属。
func (m *Manager) OnCatalogNotify(ctx context.Context, dev *model.Device, notify *manscdp.CatalogNotify) error {
	if dev == nil || notify == nil {
		return nil
	}
	now := m.now()
	var upserts []manscdp.CatalogItem

	for _, item := range notify.DeviceList.Items {
		id := strings.TrimSpace(item.DeviceID)
		if id == "" {
			continue
		}
		effect := manscdp.ResolveCatalogItemState(item)
		if !effect.Known {
			m.log.Warn("目录通知含未知事件，已忽略",
				"gb_device_id", dev.GbDeviceID, "channel_id", id, "event", item.Event)
			continue
		}

		// ADD/UPDATE 需要完整字段，交给 upsert 路径；其余事件只改状态。
		if strings.EqualFold(item.Event, manscdp.CatalogEventAdd) ||
			strings.EqualFold(item.Event, manscdp.CatalogEventUpdate) {
			upserts = append(upserts, item)
		}

		ch, err := m.store.FindChannel(ctx, id)
		if err != nil || ch == nil {
			// 本地还没有该通道时，ON/OFF 无法更新；ADD/UPDATE 由上面对 upsert 处理。
			if effect.Online != nil && !*effect.Online {
				continue
			}
			continue
		}
		fields := map[string]interface{}{}
		if effect.Online != nil {
			if *effect.Online {
				fields["status"] = model.GbStatusOn
			} else {
				fields["status"] = model.GbStatusOff
			}
		}
		if effect.Delete {
			fields["status"] = model.GbStatusOff
			fields["active"] = false
		}
		if len(fields) > 0 {
			if err := m.store.UpdateChannel(ctx, ch.ID, fields); err != nil {
				m.log.Warn("更新通道状态失败",
					"gb_device_id", dev.GbDeviceID, "channel_id", id, "error", err)
			}
		}
	}

	if len(upserts) > 0 {
		// 复用全量目录的落库逻辑：它负责域前缀校验、保留用户自定义字段、回写 channel_count。
		if err := m.persistCatalog(ctx, dev, upserts); err != nil {
			return err
		}
	}
	m.log.Debug("目录通知处理完成",
		"gb_device_id", dev.GbDeviceID, "items", len(notify.DeviceList.Items), "at", now)
	return nil
}
