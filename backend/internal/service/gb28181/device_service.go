package gb28181

import (
	"context"
	"errors"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标设备（复用 ac_device 表，只处理 gb_device_id 非空的记录）
// ════════════════════════════════════════════════════════════════

// GbDeviceService 国标设备业务逻辑。
type GbDeviceService struct {
	repo        *repository.GbDeviceRepo
	channelRepo *repository.GbChannelRepo
}

// NewGbDeviceService 构造国标设备服务。
func NewGbDeviceService() *GbDeviceService {
	return &GbDeviceService{
		repo:        repository.NewGbDeviceRepo(),
		channelRepo: repository.NewGbChannelRepo(),
	}
}

// Page 国标设备分页列表。
//
// ★ 只返回 gb_device_id 非空的设备 —— 设备管理里普通 AI 相机不会出现在国标页面。
// ★ 必须施加 DataScope：ac_device 带 dept_id，行级权限与设备管理保持一致。
func (s *GbDeviceService) Page(ctx context.Context, req *request.PageGbDeviceReq, ds *utils.DataScope) (*response.PageGbDeviceResp, error) {
	req.Normalize()

	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.Device{}).
			Where("gb_device_id IS NOT NULL AND gb_device_id <> ''")

		if kw := trim(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR device_code ILIKE ? OR gb_device_id ILIKE ?)", like, like, like)
		}
		if v := trim(req.RegisterStatus); v != "" {
			q = q.Where("register_status = ?", v)
		}
		if v := trim(req.Status); v != "" {
			q = q.Where("status = ?", v)
		}
		if req.RegionID != nil {
			q = q.Where("region_id = ?", *req.RegionID)
		}
		if ds != nil {
			q = ds.ApplyToQuery(q, "ac_device.dept_id")
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, err
	}

	var devices []model.Device
	if err := build().Preload("Region").
		Order("id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&devices).Error; err != nil {
		return nil, err
	}

	items := make([]response.GbDeviceItem, 0, len(devices))
	for _, d := range devices {
		items = append(items, toGbDeviceItem(d))
	}

	return &response.PageGbDeviceResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Get 国标设备详情。
//
// 详情额外返回媒体节点名称，前端不必再查一次节点列表。
func (s *GbDeviceService) Get(ctx context.Context, id int64) (*response.GbDeviceDetailResp, error) {
	d, err := s.getDevice(ctx, id, true)
	if err != nil {
		return nil, err
	}

	mediaServerName := ""
	if d.MediaServerID != "" && d.MediaServerID != "auto" {
		var name string
		_ = bootstrap.DB().WithContext(ctx).Model(&model.GbMediaServer{}).
			Select("name").Where("server_id = ?", d.MediaServerID).
			Limit(1).Scan(&name).Error
		mediaServerName = name
	}

	item := toGbDeviceItem(*d)
	return &response.GbDeviceDetailResp{
		GbDeviceItem:    item,
		MediaServerName: mediaServerName,
		Region:          d.Region,
	}, nil
}

// SyncChannel 触发设备目录（Catalog）同步。
//
// 只负责把查询报文发出去；设备返回的 Catalog 由信令层异步重组落库。
func (s *GbDeviceService) SyncChannel(ctx context.Context, id int64) error {
	d, err := s.getDevice(ctx, id, true)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	if err := ctl.QueryCatalog(ctx, d); err != nil {
		return err
	}
	utils.Info("国标设备目录同步已下发 gb_device_id=" + d.GbDeviceID)
	return nil
}

// Subscribe 订阅设备目录/报警/位置。
func (s *GbDeviceService) Subscribe(ctx context.Context, id int64, subType string) error {
	subType = trim(subType)
	if !validSubscribeType(subType) {
		return ErrGbSubscribeTypeInvalid
	}
	d, err := s.getDevice(ctx, id, true)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	return ctl.Subscribe(ctx, d, subType, 0)
}

// Unsubscribe 取消订阅。
func (s *GbDeviceService) Unsubscribe(ctx context.Context, id int64, subType string) error {
	subType = trim(subType)
	if !validSubscribeType(subType) {
		return ErrGbSubscribeTypeInvalid
	}
	d, err := s.getDevice(ctx, id, true)
	if err != nil {
		return err
	}
	ctl := control.Default()
	if ctl == nil {
		return ErrSipServiceUnavailable
	}
	return ctl.Unsubscribe(ctx, d, subType)
}

// GenerateActivation 生成（或读取已有）设备激活凭证。
//
// 幂等语义：
//   - 已有 gb_device_id 不再重新生成；
//   - 已有 sip_password 不再重新生成（否则已配置好的设备会立刻注册失败）。
//
// 仅补齐缺失字段，并把新设备置为 pending 等待首次 REGISTER。
func (s *GbDeviceService) GenerateActivation(ctx context.Context, id int64) (*response.GbActivationResp, error) {
	d, err := s.getDevice(ctx, id, false)
	if err != nil {
		return nil, err
	}

	cfg := loadGbSipConfig()
	fields := map[string]interface{}{}

	gbID := trim(d.GbDeviceID)
	if gbID == "" {
		gbID, err = generateGbDeviceID(ctx, cfg)
		if err != nil {
			return nil, err
		}
		fields["gb_device_id"] = gbID
	}

	pwd := d.SipPassword
	if pwd == "" {
		pwd, err = generateSipPassword(16)
		if err != nil {
			return nil, err
		}
		fields["sip_password"] = pwd
	}

	if trim(d.SipRealm) == "" {
		fields["sip_realm"] = cfg.Domain
	}
	if trim(d.SipTransport) == "" {
		fields["sip_transport"] = cfg.Transport
	}
	if trim(d.StreamMode) == "" {
		fields["stream_mode"] = model.GbStreamModeUDP
	}
	if trim(d.Charset) == "" {
		fields["charset"] = model.GbCharsetGB2312
	}
	if trim(d.RegisterStatus) == "" {
		fields["register_status"] = model.GbRegisterStatusPending
	}

	if len(fields) > 0 {
		if err := s.repo.UpdateRegisterState(ctx, id, fields); err != nil {
			return nil, err
		}
	}

	registerStatus := d.RegisterStatus
	if v, ok := fields["register_status"].(string); ok {
		registerStatus = v
	}

	resp := response.GbActivationResp{
		DeviceID:       d.ID,
		GbDeviceID:     gbID,
		SipPassword:    pwd,
		SipServer:      cfg.sipServerAddr(),
		SipPort:        cfg.Port,
		SipDomain:      cfg.Domain,
		Transport:      firstNonEmpty(d.SipTransport, cfg.Transport),
		RegisterStatus: registerStatus,
	}
	resp.QrContent = buildActivationQrContent(resp)
	return &resp, nil
}

// Delete 删除国标设备（软删除），并一并软删除其通道。
//
// 放在同一事务里：设备不在了但通道还挂在列表上会形成孤儿数据。
func (s *GbDeviceService) Delete(ctx context.Context, id int64) error {
	if _, err := s.getDevice(ctx, id, false); err != nil {
		return err
	}
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", id).Delete(&model.GbChannel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Device{}, id).Error
	})
}

// ── 内部工具 ──────────────────────────────────────────────────

// getDevice 按主键读取设备。
//
// requireGbID=true 时额外要求 gb_device_id 非空：信令类操作没有国标编码无法寻址，
// 提前给出明确错误比让 SIP 层返回 403 更容易定位。
func (s *GbDeviceService) getDevice(ctx context.Context, id int64, requireGbID bool) (*model.Device, error) {
	var d model.Device
	err := bootstrap.DB().WithContext(ctx).
		Preload("Region").
		First(&d, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbDeviceNotFound
		}
		return nil, err
	}
	if requireGbID && trim(d.GbDeviceID) == "" {
		return nil, ErrGbDeviceNotConfigured
	}
	return &d, nil
}

// validSubscribeType 校验订阅类型。
func validSubscribeType(t string) bool {
	switch strings.ToLower(trim(t)) {
	case model.GbSubscribeCatalog, model.GbSubscribeAlarm, model.GbSubscribeMobilePosition:
		return true
	default:
		return false
	}
}
