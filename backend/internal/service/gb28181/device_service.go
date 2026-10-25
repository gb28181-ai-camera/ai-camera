package gb28181

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/device"
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
	extraRepo   *repository.GbDeviceExtraRepo
}

// NewGbDeviceService 构造国标设备服务。
func NewGbDeviceService() *GbDeviceService {
	return &GbDeviceService{
		repo:        repository.NewGbDeviceRepo(),
		channelRepo: repository.NewGbChannelRepo(),
		extraRepo:   repository.NewGbDeviceExtraRepo(),
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
		if req.ProductID != nil {
			q = q.Where("product_id = ?", *req.ProductID)
		}
		if t := trim(req.ServiceGroupType); t != "" {
			q = q.Where(`EXISTS (
				SELECT 1
				FROM ac_iot_product p
				JOIN ac_iot_product_service_group psg ON psg.product_id = p.id
				JOIN ac_iot_service_group sg ON sg.id = psg.service_group_id
				WHERE p.id = ac_device.product_id
				  AND p.deleted_at IS NULL
				  AND psg.deleted_at IS NULL
				  AND sg.deleted_at IS NULL
				  AND sg.active = TRUE
				  AND sg.type = ?
			)`, t)
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
	if err := build().Preload("Region").Preload("Product").
		Order("id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&devices).Error; err != nil {
		return nil, err
	}

	items := make([]response.GbDeviceItem, 0, len(devices))
	productIDs := make([]int64, 0, len(devices))
	seen := make(map[int64]struct{}, len(devices))
	for _, d := range devices {
		item := toGbDeviceItem(d)
		applyGbDeviceExtra(&item, d)
		items = append(items, item)
		if d.ProductID != nil {
			if _, ok := seen[*d.ProductID]; !ok {
				seen[*d.ProductID] = struct{}{}
				productIDs = append(productIDs, *d.ProductID)
			}
		}
	}
	if groupsByProduct, err := repository.NewIotProductRepo().
		ListServiceGroupBriefsByProductIDs(ctx, productIDs); err == nil {
		for i := range items {
			if items[i].ProductID != nil {
				items[i].ServiceGroups = groupsByProduct[*items[i].ProductID]
			}
		}
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
	applyGbDeviceExtra(&item, *d)
	if d.ProductID != nil {
		if groups, err := repository.NewIotProductRepo().
			ListServiceGroupBriefsByProductIDs(ctx, []int64{*d.ProductID}); err == nil {
			item.ServiceGroups = groups[*d.ProductID]
		}
	}
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
	device.MarkCatalogSyncRequested(d.GbDeviceID, time.Now())
	if err := ctl.QueryCatalog(ctx, d); err != nil {
		device.MarkCatalogSyncFailed(d.GbDeviceID, "下发 Catalog 查询失败："+err.Error(), time.Now())
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

// Update 编辑国标设备档案（WVP 对齐的设备编辑）。
//
// 增量语义：DTO 字段为指针，非 nil 才覆盖；字符串指针允许传空串清空
// （name 除外，设备名称不允许为空）。所有格式校验都在写库前完成，
// 避免「部分字段写进去、另一个字段报错」的半更新状态。
func (s *GbDeviceService) Update(ctx context.Context, id int64, req *request.UpdateGbDeviceReq) error {
	if req == nil {
		return errors.New("参数不能为空")
	}
	d, err := s.extraRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbDeviceNotFound
		}
		return err
	}

	fields := map[string]interface{}{}

	if req.Name != nil {
		name := trim(*req.Name)
		if name == "" {
			return errors.New("设备名称不能为空")
		}
		if len([]rune(name)) > 255 {
			return errors.New("设备名称不能超过 255 个字符")
		}
		fields["name"] = name
	}
	if req.Location != nil {
		fields["location"] = trim(*req.Location)
	}
	if req.Remark != nil {
		fields["remark"] = trim(*req.Remark)
	}
	if req.Active != nil {
		fields["active"] = *req.Active
	}

	if req.GbDeviceID != nil {
		gbID := trim(*req.GbDeviceID)
		if !isGBDeviceID(gbID) {
			return errors.New("国标编码必须是 20 位数字")
		}
		if gbID != trim(d.GbDeviceID) {
			// 在线设备换编码会立即失去 SIP 身份，设备侧仍在用旧编码注册，
			// 表现为「改完就离线且再也注册不上」。先下线再改才是安全路径。
			if strings.EqualFold(trim(d.Status), model.DeviceStatusOnline) {
				return errors.New("设备在线时不能修改国标编码，请先让设备离线")
			}
			exists, err := s.repo.ExistsByGbDeviceID(ctx, gbID, id)
			if err != nil {
				return err
			}
			if exists {
				return ErrGbDeviceIDExists
			}
			fields["gb_device_id"] = gbID
		}
	}

	if req.SipRealm != nil {
		realm := trim(*req.SipRealm)
		if !isDigits(realm) || len(realm) != 10 {
			return errors.New("SIP 域必须是 10 位数字")
		}
		fields["sip_realm"] = realm
	}

	if req.SipTransport != nil {
		v := strings.ToUpper(trim(*req.SipTransport))
		switch v {
		case model.GbSipTransportUDP, model.GbSipTransportTCP:
			fields["sip_transport"] = v
		default:
			return errors.New("SIP 传输方式仅支持 UDP 或 TCP")
		}
	}

	if req.StreamMode != nil {
		v := strings.ToUpper(trim(*req.StreamMode))
		switch v {
		case model.GbStreamModeUDP, model.GbStreamModeTCPPassive, model.GbStreamModeTCPActive:
			fields["stream_mode"] = v
		default:
			return errors.New("媒体传输方式仅支持 UDP / TCP-PASSIVE / TCP-ACTIVE")
		}
	}

	if req.Charset != nil {
		v, ok := normalizeGbCharset(*req.Charset)
		if !ok {
			return errors.New("字符集仅支持 GB2312 或 UTF-8")
		}
		fields["charset"] = v
	}

	if req.GbVersion != nil {
		v := trim(*req.GbVersion)
		switch v {
		case model.GbVersion2016, model.GbVersion2022:
			fields["gb_version"] = v
		default:
			return errors.New("国标协议版本仅支持 2016 或 2022")
		}
	}

	if req.MediaServerID != nil {
		v := trim(*req.MediaServerID)
		if v == "" || strings.EqualFold(v, "auto") {
			v = "auto"
		} else {
			ok, err := s.extraRepo.MediaServerExists(ctx, v)
			if err != nil {
				return err
			}
			if !ok {
				return ErrGbMediaServerNotFound
			}
		}
		fields["media_server_id"] = v
	}

	if req.LocalIP != nil {
		v := trim(*req.LocalIP)
		if v != "" && net.ParseIP(v) == nil {
			return errors.New("local_ip 不是合法的 IP 地址")
		}
		fields["local_ip"] = v
	}
	if req.SDPIP != nil {
		v := trim(*req.SDPIP)
		if v != "" && net.ParseIP(v) == nil {
			return errors.New("sdp_ip 不是合法的 IP 地址")
		}
		fields["sdp_ip"] = v
	}

	if req.Latitude != nil {
		v := trim(*req.Latitude)
		if v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f < -90 || f > 90 {
				return errors.New("纬度必须在 -90 到 90 之间")
			}
		}
		fields["latitude"] = v
	}
	if req.Longitude != nil {
		v := trim(*req.Longitude)
		if v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil || f < -180 || f > 180 {
				return errors.New("经度必须在 -180 到 180 之间")
			}
		}
		fields["longitude"] = v
	}

	return s.extraRepo.UpdateProfile(ctx, id, fields)
}

// applyGbDeviceExtra 补充 common.go 的 toGbDeviceItem 未覆盖的字段。
//
// common.go 由其他模块维护，不便在这里改映射；扩展字段在此统一补齐，
// 避免列表与详情的字段集不一致。
func applyGbDeviceExtra(item *response.GbDeviceItem, d model.Device) {
	item.GbVersion = d.GbVersion
}

// isGBDeviceID 判断是否为 20 位数字国标编码。
func isGBDeviceID(s string) bool {
	return len(s) == 20 && isDigits(s)
}

// isDigits 判断字符串是否全为 ASCII 数字。
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// normalizeGbCharset 归一化字符集取值。
func normalizeGbCharset(s string) (string, bool) {
	switch strings.ToUpper(strings.ReplaceAll(trim(s), "_", "-")) {
	case "UTF-8", "UTF8":
		return model.GbCharsetUTF8, true
	case "GB2312", "GB-2312", "GBK", "GB18030":
		return model.GbCharsetGB2312, true
	default:
		return "", false
	}
}

// ── 内部工具 ──────────────────────────────────────────────────

// getDevice 按主键读取设备。
//
// requireGbID=true 时额外要求 gb_device_id 非空：信令类操作没有国标编码无法寻址，
// 提前给出明确错误比让 SIP 层返回 403 更容易定位。
func (s *GbDeviceService) getDevice(ctx context.Context, id int64, requireGbID bool) (*model.Device, error) {
	var d model.Device
	err := bootstrap.DB().WithContext(ctx).
		Preload("Region").Preload("Product").
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
	case model.GbSubscribeCatalog, model.GbSubscribeAlarm, model.GbSubscribeMobilePosition, model.GbSubscribePTZPosition:
		return true
	default:
		return false
	}
}
