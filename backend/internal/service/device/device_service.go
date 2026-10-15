package device

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 资源管理 - 设备
// ════════════════════════════════════════════════════════════════

// 业务错误：Handler 依据 errors.Is 分类返回 404 / 400。
var (
	// ErrDeviceNotFound 设备不存在。
	ErrDeviceNotFound = errors.New("设备不存在")
	// ErrDeviceCodeExists 设备编号已存在。
	ErrDeviceCodeExists = errors.New("设备编号已存在")
	// ErrModeNotFound 设备模式不存在。
	ErrModeNotFound = errors.New("设备模式不存在")
	// ErrIotCardNotFound 物联网卡不存在。
	ErrIotCardNotFound = errors.New("物联网卡不存在")
	// ErrIotCardInUse 物联网卡已被其它设备占用。
	ErrIotCardInUse = errors.New("物联网卡已被其它设备使用")
	// ErrVoiceConfigNotFound 语音配置不存在。
	ErrVoiceConfigNotFound = errors.New("语音配置不存在")
)

// 曲线时间标签格式：按小时聚合精确到小时，按天聚合精确到日。
const (
	chartHourLayout = "2006-01-02 15:00"
	chartDayLayout  = "2006-01-02"
)

// DeviceService 设备业务逻辑。
type DeviceService struct {
	repo     *repository.DeviceRepo
	cardRepo *repository.IotCardRepo
	modeRepo *repository.DeviceModeRepo
}

// NewDeviceService 构建设备服务。
func NewDeviceService() *DeviceService {
	return &DeviceService{
		repo:     repository.NewDeviceRepo(),
		cardRepo: repository.NewIotCardRepo(),
		modeRepo: repository.NewDeviceModeRepo(),
	}
}

// Page 设备分页列表。
func (s *DeviceService) Page(ctx context.Context, req *request.PageDeviceReq, ds *utils.DataScope) (*response.PageDeviceResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.Device{}
	}
	return &response.PageDeviceResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// Options 设备下拉选项。Name 取设备名称，Code 取设备编号。
func (s *DeviceService) Options(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	items, err := s.repo.Options(ctx, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.OptionItem, 0, len(items))
	for i := range items {
		out = append(out, response.OptionItem{
			ID:   items[i].ID,
			Name: items[i].Name,
			Code: items[i].DeviceCode,
		})
	}
	return out, nil
}

// Get 设备详情。
func (s *DeviceService) Get(ctx context.Context, id int64) (*model.Device, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}
	return m, nil
}

// Create 新增设备。
func (s *DeviceService) Create(ctx context.Context, req *request.CreateDeviceReq) (*model.Device, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("设备名称不能为空")
	}
	code := strings.TrimSpace(req.DeviceCode)
	if code == "" {
		return nil, errors.New("设备编号不能为空")
	}

	exists, err := s.repo.ExistsByCode(ctx, code, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrDeviceCodeExists
	}

	m := &model.Device{
		Name:         name,
		DeviceCode:   code,
		RegionID:     req.RegionID,
		Location:     req.Location,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		FirmwareID:   req.FirmwareID,
		FirmwareVer:  req.FirmwareVer,
		ModelID:      req.ModelID,
		ModelVer:     req.ModelVer,
		SystemVer:    req.SystemVer,
		WorkMode:     req.WorkMode,
		DeviceMode:   req.DeviceMode,
		InfoInterval: req.InfoInterval,
		IotCardID:    req.IotCardID,
		StorageTotal: req.StorageTotal,
		StreamURL:    req.StreamURL,
		Remark:       req.Remark,
		// 新建设备尚未上报心跳，固定为离线，等待首次心跳刷新
		Status: model.DeviceStatusOffline,
	}
	if m.InfoInterval <= 0 {
		m.InfoInterval = 60
	}
	m.Active = boolOr(req.Active, true)
	m.MonitorEnabled = boolOr(req.MonitorEnabled, false)

	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}

	// 双向绑定：设备侧 iot_card_id 已写入，同步卡侧的 device_id
	s.bindCard(ctx, m.IotCardID, m.ID)
	return m, nil
}

// Update 更新设备。
//
// 采用「增量覆盖」语义：文本字段非空才覆盖（便于清空可选字段时传空串被忽略），
// 数值字段大于 0 才覆盖，指针字段非 nil 才覆盖；
// iot_card_id 传 0 表示解绑当前物联网卡。
func (s *DeviceService) Update(ctx context.Context, id int64, req *request.UpdateDeviceReq) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	fields := map[string]interface{}{}

	if v := strings.TrimSpace(req.Name); v != "" {
		fields["name"] = v
	}
	if v := strings.TrimSpace(req.DeviceCode); v != "" && v != m.DeviceCode {
		exists, err := s.repo.ExistsByCode(ctx, v, id)
		if err != nil {
			return err
		}
		if exists {
			return ErrDeviceCodeExists
		}
		fields["device_code"] = v
	}
	if req.RegionID != nil {
		fields["region_id"] = *req.RegionID
	}
	if v := strings.TrimSpace(req.Location); v != "" {
		fields["location"] = v
	}
	if v := strings.TrimSpace(req.Latitude); v != "" {
		fields["latitude"] = v
	}
	if v := strings.TrimSpace(req.Longitude); v != "" {
		fields["longitude"] = v
	}
	if req.FirmwareID != nil {
		fields["firmware_id"] = *req.FirmwareID
	}
	if v := strings.TrimSpace(req.FirmwareVer); v != "" {
		fields["firmware_ver"] = v
	}
	if req.ModelID != nil {
		fields["model_id"] = *req.ModelID
	}
	if v := strings.TrimSpace(req.ModelVer); v != "" {
		fields["model_ver"] = v
	}
	if v := strings.TrimSpace(req.SystemVer); v != "" {
		fields["system_ver"] = v
	}
	if v := strings.TrimSpace(req.WorkMode); v != "" {
		fields["work_mode"] = v
	}
	if v := strings.TrimSpace(req.DeviceMode); v != "" {
		fields["device_mode"] = v
	}
	if req.InfoInterval > 0 {
		fields["info_interval"] = req.InfoInterval
	}
	if req.StorageTotal > 0 {
		fields["storage_total"] = req.StorageTotal
	}
	if v := strings.TrimSpace(req.StreamURL); v != "" {
		fields["stream_url"] = v
	}
	if v := strings.TrimSpace(req.Remark); v != "" {
		fields["remark"] = v
	}
	if req.Active != nil {
		fields["active"] = *req.Active
	}
	if req.MonitorEnabled != nil {
		fields["monitor_enabled"] = *req.MonitorEnabled
	}

	oldCardID := m.IotCardID
	newCardID := m.IotCardID
	if req.IotCardID != nil {
		if *req.IotCardID > 0 {
			newCardID = req.IotCardID
		} else {
			// 传 0 视为解绑
			newCardID = nil
		}
		fields["iot_card_id"] = newCardID
	}

	if err := s.repo.UpdateFields(ctx, id, fields); err != nil {
		return err
	}

	// 卡绑定变更后维护双向一致性
	if oldCardID != nil && *oldCardID > 0 && (newCardID == nil || *newCardID != *oldCardID) {
		s.unbindCard(ctx, *oldCardID)
	}
	if newCardID != nil && (oldCardID == nil || *newCardID != *oldCardID) {
		s.bindCard(ctx, newCardID, id)
	}
	return nil
}

// Delete 删除设备（软删除），并解除其物联网卡绑定。
func (s *DeviceService) Delete(ctx context.Context, id int64) error {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	if m.IotCardID != nil && *m.IotCardID > 0 {
		s.unbindCard(ctx, *m.IotCardID)
	}
	return nil
}

// SwitchMode 切换设备模式。
//
// mode 必须命中设备模式表中的名称，否则返回 ErrModeNotFound。
// 该操作仅改写设备模式并留痕日志，不涉及物联网卡。
func (s *DeviceService) SwitchMode(ctx context.Context, id int64, mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return ErrModeNotFound
	}

	ok, err := s.modeRepo.ExistsByName(ctx, mode, 0)
	if err != nil {
		return err
	}
	if !ok {
		return ErrModeNotFound
	}

	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	if err := s.repo.UpdateFields(ctx, id, map[string]interface{}{"device_mode": mode}); err != nil {
		return err
	}
	utils.Info(fmt.Sprintf("设备模式切换 device=%s(%d) %s -> %s operator=%s",
		m.DeviceCode, m.ID, m.DeviceMode, mode, model.UsernameFromCtx(ctx)))
	return nil
}

// SyncTime 同步设备时间：把 last_beat_dt 置为当前时间。
func (s *DeviceService) SyncTime(ctx context.Context, id int64) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{"last_beat_dt": time.Now()})
}

// ToggleMonitor 加入/移出实时监控。
func (s *DeviceService) ToggleMonitor(ctx context.Context, id int64, enabled bool) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{"monitor_enabled": enabled})
}

// Heartbeat 设备心跳上报。
//
// 更新设备的运行状态快照（status/battery/voltage/storage_free/last_beat_dt），
// 状态发生翻转时补记 online_dt / offline_dt，并追加一条运行历史采样。
// 入参未携带的运行时字段（电量/电压/剩余存储）沿用设备当前值，避免被 0 值覆盖。
func (s *DeviceService) Heartbeat(ctx context.Context, id int64, req *request.DeviceHeartbeatReq) error {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDeviceNotFound
		}
		return err
	}

	now := time.Now()
	status := strings.TrimSpace(req.Status)
	if status == "" {
		status = d.Status
	}
	if status == "" {
		status = model.DeviceStatusOffline
	}

	battery := req.Battery
	if battery <= 0 {
		battery = d.Battery
	}
	voltage := strings.TrimSpace(req.Voltage)
	if voltage == "" {
		voltage = d.Voltage
	}
	storageFree := req.StorageFree
	if storageFree <= 0 {
		storageFree = d.StorageFree
	}

	fields := map[string]interface{}{
		"status":       status,
		"battery":      battery,
		"voltage":      voltage,
		"storage_free": storageFree,
		"last_beat_dt": now,
	}
	if d.Status != status {
		if status == model.DeviceStatusOnline {
			fields["online_dt"] = now
		} else {
			fields["offline_dt"] = now
		}
	}
	if err := s.repo.UpdateFields(ctx, id, fields); err != nil {
		return err
	}

	// 冗余字段从设备与物联网卡快照，保证历史记录可独立回溯
	cardNo := ""
	if d.IotCard != nil {
		cardNo = d.IotCard.CardNo
	}
	h := &model.DeviceHistory{
		DeviceID:    d.ID,
		DeviceCode:  d.DeviceCode,
		IotCardNo:   cardNo,
		RegionID:    d.RegionID,
		Location:    d.Location,
		Status:      status,
		Battery:     battery,
		Voltage:     voltage,
		StorageFree: storageFree,
		SampledDT:   now,
		DeptID:      d.DeptID,
	}
	return s.repo.CreateHistory(ctx, h)
}

// PageHistory 设备运行历史分页列表。
func (s *DeviceService) PageHistory(ctx context.Context, req *request.PageDeviceHistoryReq, ds *utils.DataScope) (*response.PageDeviceHistoryResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageHistory(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.DeviceHistory{}
	}
	return &response.PageDeviceHistoryResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// HistoryChart 设备历史曲线（默认近 3 天）。
//
// days<=7 时按小时聚合，否则按天聚合；返回电量/电压/剩余存储/状态四条曲线，
// 以及在线、离线时长汇总（单位分钟）。
func (s *DeviceService) HistoryChart(ctx context.Context, deviceID int64, days int) (*response.DeviceHistoryChartResp, error) {
	if days <= 0 {
		days = 3
	}
	if days > 90 {
		days = 90
	}

	granularity := "hour"
	layout := chartHourLayout
	if days > 7 {
		granularity = "day"
		layout = chartDayLayout
	}
	since := time.Now().AddDate(0, 0, -days)

	points, err := s.repo.AggregateHistory(ctx, deviceID, since, granularity)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.StatusSummary(ctx, deviceID, since)
	if err != nil {
		return nil, err
	}

	resp := &response.DeviceHistoryChartResp{
		DeviceID:      deviceID,
		Days:          days,
		Granularity:   granularity,
		BatteryPoints: make([]response.DeviceChartPoint, 0, len(points)),
		VoltagePoints: make([]response.DeviceChartPoint, 0, len(points)),
		StoragePoints: make([]response.DeviceChartPoint, 0, len(points)),
		StatusPoints:  make([]response.DeviceChartPoint, 0, len(points)),
	}
	for _, p := range points {
		label := p.Bucket.Format(layout)
		resp.BatteryPoints = append(resp.BatteryPoints, response.DeviceChartPoint{Time: label, Value: round(p.Battery, 1)})
		resp.VoltagePoints = append(resp.VoltagePoints, response.DeviceChartPoint{Time: label, Value: round(p.Voltage, 2)})
		resp.StoragePoints = append(resp.StoragePoints, response.DeviceChartPoint{Time: label, Value: round(p.StorageFree, 1)})
		resp.StatusPoints = append(resp.StatusPoints, response.DeviceChartPoint{Time: label, Value: round(p.StatusValue, 1)})
	}
	resp.StatusSummary = response.DeviceStatusSummary{
		OnlineMinutes:  int64(math.Round(summary.OnlineSeconds / 60)),
		OfflineMinutes: int64(math.Round(summary.OfflineSeconds / 60)),
	}
	return resp, nil
}

// ── 内部工具 ──────────────────────────────────────────────────

// bindCard 把物联网卡绑定到设备（失败仅告警，不阻断主流程）。
func (s *DeviceService) bindCard(ctx context.Context, cardID *int64, deviceID int64) {
	if cardID == nil || *cardID <= 0 {
		return
	}
	if err := s.cardRepo.BindDevice(ctx, *cardID, &deviceID); err != nil {
		utils.Warn(fmt.Sprintf("同步物联网卡绑定失败 card_id=%d device_id=%d: %v", *cardID, deviceID, err))
	}
}

// unbindCard 解除物联网卡的设备绑定（失败仅告警）。
func (s *DeviceService) unbindCard(ctx context.Context, cardID int64) {
	if cardID <= 0 {
		return
	}
	if err := s.cardRepo.BindDevice(ctx, cardID, nil); err != nil {
		utils.Warn(fmt.Sprintf("解除物联网卡绑定失败 card_id=%d: %v", cardID, err))
	}
}

// boolOr 返回指针指向的布尔值，为空时取默认值。
func boolOr(p *bool, def bool) *bool {
	if p != nil {
		return p
	}
	v := def
	return &v
}

// round 四舍五入到指定小数位。
func round(v float64, digits int) float64 {
	f := math.Pow10(digits)
	return math.Round(v*f) / f
}
