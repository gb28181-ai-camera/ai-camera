package gb28181

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// 订阅记录状态（ac_gb_subscribe.status）。
const (
	subscribeStatusActive = "active"
	subscribeStatusFailed = "failed"
	// defaultSubscribeExpires 与 Create 的缺省一致：一小时。
	defaultSubscribeExpires int64 = 3600
)

// 自动续订参数。
const (
	// renewLeadRatio 续订提前量 = 有效期 × 20%。
	//
	// 为什么按比例而不是固定值：有效期是设备给的（常见 3600s，也有 300s/86400s）。
	// 固定 60s 对 12 小时的订阅只提前 0.14%，一轮网络抖动就可能错过；
	// 20% 让提前量随有效期自适应 —— 3600s 提前 12 分钟，有 24 个扫描周期可重试。
	renewLeadRatio = 0.2
	// renewLeadMin 续订提前量下限 60s。
	//
	// 扫描周期是 30s，提前量低于 60s 会落在两次扫描之间必然错过；
	// 同时 300s 这类短订阅按 20% 只有 60s，下限正好覆盖。
	renewLeadMin = 60 * time.Second
	// renewMaxFailures 连续失败多少次后把订阅置 failed。
	//
	// 选「3 次置 failed」而不是「永远保持 active」：设备确实不可达时，
	// 每 30s 重发会持续占用事务与日志；3 次（约 1.5 分钟）足以过滤瞬时故障。
	// 失败计数只存内存 —— ac_gb_subscribe 没有失败列，且新增列会牵动并行开发中的
	// 模型与 DDL；进程重启后计数归零，最坏是多试 3 次。
	renewMaxFailures = 3
)

// renewMu 串行化续订扫描。
//
// sipserver 的定时器本身是串行的，但 RenewDue 也可能被运维脚本/测试调用；
// 并发执行会对同一条订阅重复发 SUBSCRIBE。锁是包级的，因为每次回调都新建
// 一个 GbSubscribeService（无状态）。
var renewMu sync.Mutex

// renewFailures 记录各订阅的连续续订失败次数，key 为 ac_gb_subscribe.id。
var renewFailures = map[int64]int{}

// init 把续订入口注册给信令服务。
//
// 为什么是回调：service/gb28181 已经依赖 sipserver（播放/语音直接调用信令），
// 反向 import 会形成 import cycle；而定时器必须跟随 SIP 服务的启停，所以由
// sipserver 持有计时器、业务层注册实现。
// cmd/sipsvc 用空导入确保本包被链接进独立信令进程。
func init() {
	sipserver.SetSubscribeRenewer(func(ctx context.Context) (int, error) {
		return NewGbSubscribeService().RenewDue(ctx)
	})
}

// callIDSubscriber 是 sipserver.Service 的可选能力：返回本次 SUBSCRIBE 的真实 Call-ID。
// 用本地接口断言而不是扩 control.SipController，避免把维护细节塞进业务接口，
// 也让「信令未启动时的 nil 控制器」继续走普通的 Subscribe 分支。
type callIDSubscriber interface {
	SubscribeWithCallID(ctx context.Context, dev *model.Device, subType string, interval int) (string, error)
}

// RenewLead 返回给定订阅有效期（秒）下的续订提前量。
//
// 规则：max(有效期 × 20%, 60s)，但不超过有效期的一半。
// 上限的意义：有效期只有 30s 时，60s 的提前量比有效期还大，会导致「刚续订完
// 下一轮扫描又判定到期」，变成每 30s 发一次 SUBSCRIBE 的死循环；取一半保证
// 至少隔一个完整周期。
// 有效期 <= 0 时按缺省 3600s 处理（Create 也做同样的兜底）。
func RenewLead(expires int64) time.Duration {
	if expires <= 0 {
		expires = defaultSubscribeExpires
	}
	ttl := time.Duration(expires) * time.Second
	lead := time.Duration(float64(ttl) * renewLeadRatio)
	if lead < renewLeadMin {
		lead = renewLeadMin
	}
	if half := ttl / 2; lead > half {
		lead = half
	}
	return lead
}

// RenewDue 续订已进入「续订窗口」的订阅，返回成功续订的条数。
//
// 窗口：status='active' 且 expire_dt <= now + RenewLead(expires)。
// 为什么不用 SQL 直接算窗口：提前量依赖每行的 expires，SQL 里要拼
// GREATEST(expires*0.2, 60) 与区间运算，可读性差且难测；订阅表是低基数表
// （每设备最多 4 条），整表扫 active 行在 Go 里过滤更清晰。
//
// SIP 服务未启动时直接返回 (0, nil)：既不能报错打断调用方，也绝不能把
// 状态改成 failed——那会在「信令暂时没起」时把所有订阅判死。
func (s *GbSubscribeService) RenewDue(ctx context.Context) (int, error) {
	db := bootstrap.DB()
	if db == nil {
		return 0, nil
	}
	ctl := control.Default()
	if ctl == nil {
		return 0, nil
	}

	renewMu.Lock()
	defer renewMu.Unlock()

	now := time.Now()
	var rows []model.GbSubscribe
	if err := db.WithContext(ctx).
		Where("status = ?", subscribeStatusActive).
		Where("expire_dt IS NOT NULL").
		Order("id ASC").Find(&rows).Error; err != nil {
		return 0, err
	}

	// 清理已消失订阅的失败计数，避免 map 随删除操作无限增长。
	seen := make(map[int64]struct{}, len(rows))
	for i := range rows {
		seen[rows[i].ID] = struct{}{}
	}
	for id := range renewFailures {
		if _, ok := seen[id]; !ok {
			delete(renewFailures, id)
		}
	}

	renewed := 0
	var firstErr error
	for i := range rows {
		sub := &rows[i]
		if sub.ExpireDT == nil || sub.ExpireDT.After(now.Add(RenewLead(sub.Expires))) {
			continue // 尚未进入续订窗口
		}

		// 本轮维护预算用尽（例如前面有一台设备不可达，SIP 事务吃满了超时）：
		// 剩下的留到下一轮，**不能计为续订失败**——它们根本没发出去。
		if err := ctx.Err(); err != nil {
			return renewed, err
		}

		dev, err := s.loadDevice(ctx, sub)
		if err != nil {
			s.recordRenewFailure(ctx, db, sub, err, now)
			continue
		}
		if dev == nil {
			s.recordRenewFailure(ctx, db, sub, errors.New("订阅关联的设备不存在"), now)
			continue
		}
		if dev.Status != model.DeviceStatusOnline {
			// 设备离线时不发：SIP 事务必然超时，还会消耗失败次数把订阅判死。
			// 保持 active，等设备重新 REGISTER/心跳上线后的下一轮扫描再续订。
			slog.Debug("订阅续订跳过：设备离线",
				"subscribe_id", sub.ID, "gb_device_id", sub.GbDeviceID)
			continue
		}

		expires := sub.Expires
		if expires <= 0 {
			expires = defaultSubscribeExpires
		}
		var callID string
		if withID, ok := ctl.(callIDSubscriber); ok {
			callID, err = withID.SubscribeWithCallID(ctx, dev, sub.Type, int(expires))
		} else {
			err = ctl.Subscribe(ctx, dev, sub.Type, int(expires))
		}
		if err != nil {
			s.recordRenewFailure(ctx, db, sub, err, now)
			continue
		}

		delete(renewFailures, sub.ID)
		expire := now.Add(time.Duration(expires) * time.Second)
		updates := map[string]interface{}{
			"status":       subscribeStatusActive,
			"subscribe_dt": now,
			"expire_dt":    expire,
			"call_id":      callID, // 控制器不支持返回 Call-ID 时写空串，清掉过期的旧值
			"updated_dt":   now,
			"updated_by":   "sipsvc",
		}
		if err := db.WithContext(ctx).Model(&model.GbSubscribe{}).
			Where("id = ?", sub.ID).Updates(updates).Error; err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		renewed++
		slog.Info("订阅已续订",
			"subscribe_id", sub.ID, "gb_device_id", sub.GbDeviceID,
			"type", sub.Type, "expires", expires, "call_id", callID)
	}
	return renewed, firstErr
}

// recordRenewFailure 累计连续失败次数，达到阈值时把订阅置为 failed。
//
// 未达阈值时**保持 active**：下一轮扫描会继续尝试，expire_dt 不变，
// 前端能看出「快到期但还在重试」。达到阈值后置 failed 并停止重试，
// 由用户重新订阅或设备恢复后人工处理。
func (s *GbSubscribeService) recordRenewFailure(ctx context.Context, db *gorm.DB, sub *model.GbSubscribe, cause error, now time.Time) {
	n := renewFailures[sub.ID] + 1
	renewFailures[sub.ID] = n
	if n < renewMaxFailures {
		slog.Warn("订阅续订失败，保持 active 等待下一轮",
			"subscribe_id", sub.ID, "gb_device_id", sub.GbDeviceID,
			"type", sub.Type, "failures", n, "error", cause)
		return
	}

	delete(renewFailures, sub.ID)
	if err := db.WithContext(ctx).Model(&model.GbSubscribe{}).
		Where("id = ?", sub.ID).
		Updates(map[string]interface{}{
			"status":     subscribeStatusFailed,
			"updated_dt": now,
			"updated_by": "sipsvc",
		}).Error; err != nil {
		slog.Warn("订阅连续续订失败，但置 failed 写库失败",
			"subscribe_id", sub.ID, "error", err)
		return
	}
	slog.Warn("订阅连续续订失败，已置为 failed",
		"subscribe_id", sub.ID, "gb_device_id", sub.GbDeviceID,
		"type", sub.Type, "failures", n, "error", cause)
}

// loadDevice 取订阅对应的设备对象。
//
// 优先 device_id（本地主键，最稳），失效时回落到国标编码 —— 订阅记录允许
// 只登记 gb_device_id（设备尚未入库或 ID 被重建）。
func (s *GbSubscribeService) loadDevice(ctx context.Context, sub *model.GbSubscribe) (*model.Device, error) {
	if sub.DeviceID != nil && *sub.DeviceID > 0 {
		var d model.Device
		err := bootstrap.DB().WithContext(ctx).First(&d, *sub.DeviceID).Error
		if err == nil {
			return &d, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	gbID := strings.TrimSpace(sub.GbDeviceID)
	if gbID == "" {
		return nil, nil
	}
	d, err := s.deviceRepo.FindByGbDeviceID(ctx, gbID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

// ════════════════════════════════════════════════════════════════
// 国标订阅记录（ac_gb_subscribe，RFC 3265）
// ════════════════════════════════════════════════════════════════

// GbSubscribeService 订阅记录业务逻辑。
type GbSubscribeService struct {
	deviceRepo *repository.GbDeviceRepo
}

// NewGbSubscribeService 构造订阅记录服务。
func NewGbSubscribeService() *GbSubscribeService {
	return &GbSubscribeService{deviceRepo: repository.NewGbDeviceRepo()}
}

// Page 订阅记录分页列表。
//
// ds 为可变参数（契约签名为 Page(ctx, req)）；Handler 传入 DataScope 时会施加行级过滤。
func (s *GbSubscribeService) Page(ctx context.Context, req *request.PageGbSubscribeReq, ds ...*utils.DataScope) (*response.PageGbSubscribeResp, error) {
	req.Normalize()

	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbSubscribe{})
		if v := trim(req.GbDeviceID); v != "" {
			q = q.Where("gb_device_id = ?", v)
		}
		if v := trim(req.Type); v != "" {
			q = q.Where("type = ?", v)
		}
		if v := trim(req.Status); v != "" {
			q = q.Where("status = ?", v)
		}
		if len(ds) > 0 && ds[0] != nil {
			q = ds[0].ApplyToQuery(q, "ac_gb_subscribe.dept_id")
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, err
	}

	var records []model.GbSubscribe
	if err := build().Order("id DESC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&records).Error; err != nil {
		return nil, err
	}

	// 批量取设备名称：只查 id/name 两列，避免把 sip_password 等敏感列读进内存。
	deviceIDs := make([]int64, 0, len(records))
	for _, r := range records {
		if r.DeviceID != nil && *r.DeviceID > 0 {
			deviceIDs = append(deviceIDs, *r.DeviceID)
		}
	}
	names := map[int64]string{}
	if len(deviceIDs) > 0 {
		var devices []model.Device
		if err := bootstrap.DB().WithContext(ctx).Model(&model.Device{}).
			Select("id, name").Where("id IN ?", deviceIDs).Find(&devices).Error; err != nil {
			return nil, err
		}
		for _, d := range devices {
			names[d.ID] = d.Name
		}
	}

	items := make([]response.GbSubscribeItem, 0, len(records))
	for _, r := range records {
		name := ""
		if r.DeviceID != nil {
			name = names[*r.DeviceID]
		}
		items = append(items, response.GbSubscribeItem{GbSubscribe: r, DeviceName: name})
	}

	return &response.PageGbSubscribeResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Create 新增订阅记录。
//
// 该接口只登记订阅关系（例如平台重启后恢复的订阅），不发送 SIP SUBSCRIBE；
// 真正的订阅动作走 POST /web/gb/devices/:id/subscribe。
func (s *GbSubscribeService) Create(ctx context.Context, req *request.CreateGbSubscribeReq) (*model.GbSubscribe, error) {
	subType := trim(req.Type)
	if !validSubscribeType(subType) {
		return nil, ErrGbSubscribeTypeInvalid
	}

	m := &model.GbSubscribe{
		Type:    subType,
		CallID:  trim(req.CallID),
		Expires: req.Expires,
		Status:  "active",
		Remark:  trim(req.Remark),
		DeptID:  model.DeptIDFromCtx(ctx),
	}
	if m.Expires <= 0 {
		m.Expires = 3600
	}

	var device *model.Device
	if req.DeviceID != nil && *req.DeviceID > 0 {
		var row model.Device
		if err := bootstrap.DB().WithContext(ctx).First(&row, *req.DeviceID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrGbDeviceNotFound
			}
			return nil, err
		}
		device = &row
		m.DeviceID = &row.ID
		m.GbDeviceID = row.GbDeviceID
	}
	if gbID := trim(req.GbDeviceID); gbID != "" {
		m.GbDeviceID = gbID
		if device == nil {
			// 只有国标编码时尝试反查本地设备；查不到也允许登记（设备可能尚未入库）。
			if row, err := s.deviceRepo.FindByGbDeviceID(ctx, gbID); err == nil {
				device = row
				m.DeviceID = &row.ID
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
	}
	if m.GbDeviceID == "" {
		return nil, errors.New("gb_device_id 不能为空（可直接传国标编码，或传 device_id 由服务端回填）")
	}

	if device != nil && m.DeptID == nil {
		m.DeptID = device.DeptID
	}

	now := time.Now()
	expire := now.Add(time.Duration(m.Expires) * time.Second)
	m.SubscribeDT = &now
	m.ExpireDT = &expire

	if err := bootstrap.DB().WithContext(ctx).Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// Delete 删除订阅记录（软删除）。
func (s *GbSubscribeService) Delete(ctx context.Context, id int64) error {
	var m model.GbSubscribe
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbSubscribeNotFound
		}
		return err
	}
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbSubscribe{}, id).Error
}
