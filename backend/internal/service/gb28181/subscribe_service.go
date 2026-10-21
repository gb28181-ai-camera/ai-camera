package gb28181

import (
	"context"
	"errors"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

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
