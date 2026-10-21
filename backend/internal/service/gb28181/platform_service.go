package gb28181

// 国标级联：本平台作为下级，向上级平台注册/推送目录。
//
// ★ 本批次边界：**只做配置与映射表的 CRUD**。
//   - 不向上级平台发起 REGISTER / SUBSCRIBE；
//   - 不推送 Catalog；
//   - 不处理上级的 INVITE。
//
// 这些 SIP 交互依赖级联专用的对话框与目录组装，属于后续批次。本文件把
// 「上级平台 + 可见通道 + 可见分组/区域」的配置维护完整，保证 SIP 层接入时
// 数据已经就绪；不做任何「看起来已级联」的假动作。

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

var (
	// ErrGbPlatformNotFound 上级平台不存在。
	ErrGbPlatformNotFound = errors.New("上级平台不存在")
	// ErrGbPlatformGBIDExists 上级平台国标编码已被占用。
	ErrGbPlatformGBIDExists = errors.New("上级平台国标编码已被占用")
	// ErrGbPlatformChannelNotFound 级联通道映射不存在。
	ErrGbPlatformChannelNotFound = errors.New("级联通道映射不存在")
	// ErrGbPlatformChannelIDExists 定制国标编码已被占用。
	ErrGbPlatformChannelIDExists = errors.New("该定制国标编码已被其他通道占用")
)

// GbPlatformService 级联平台业务逻辑。
type GbPlatformService struct {
	repo      *repository.GbPlatformRepo
	chanRepo  *repository.GbPlatformChannelRepo
	groupRepo *repository.GbPlatformGroupRepo
	regRepo   *repository.GbPlatformRegionRepo
}

// NewGbPlatformService 构造级联平台服务。
func NewGbPlatformService() *GbPlatformService {
	return &GbPlatformService{
		repo:      repository.NewGbPlatformRepo(),
		chanRepo:  repository.NewGbPlatformChannelRepo(),
		groupRepo: repository.NewGbPlatformGroupRepo(),
		regRepo:   repository.NewGbPlatformRegionRepo(),
	}
}

// Page 上级平台分页。
func (s *GbPlatformService) Page(ctx context.Context, req *request.PageGbPlatformReq, ds *utils.DataScope) (*response.PageGbPlatformResp, error) {
	if req == nil {
		req = &request.PageGbPlatformReq{}
	}
	req.Normalize()
	rows, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.GbPlatformItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, toGbPlatformItem(m))
	}
	return &response.PageGbPlatformResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// toGbPlatformItem 脱敏后回显。
func toGbPlatformItem(m model.GbPlatform) response.GbPlatformItem {
	m.Password = utils.MaskSecret(m.Password)
	return response.GbPlatformItem{GbPlatform: m}
}

// Get 上级平台详情（含分组/区域可见范围）。
func (s *GbPlatformService) Get(ctx context.Context, id int64) (*response.GbPlatformDetailResp, error) {
	m, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	groupIDs, err := s.groupRepo.ListGroupIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	regionIDs, err := s.regRepo.ListRegionIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	return &response.GbPlatformDetailResp{
		GbPlatformItem: toGbPlatformItem(*m),
		GroupIDs:       groupIDs,
		RegionIDs:      regionIDs,
	}, nil
}

// Create 新增上级平台（只落配置，不发起注册）。
func (s *GbPlatformService) Create(ctx context.Context, req *request.CreateGbPlatformReq) (*model.GbPlatform, error) {
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	name, gbID := trim(req.Name), trim(req.ServerGBID)
	if name == "" || gbID == "" {
		return nil, errors.New("平台名称与国标编码不能为空")
	}
	exists, err := s.repo.ExistsByServerGBID(ctx, gbID, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrGbPlatformGBIDExists
	}

	m := &model.GbPlatform{
		Name:           name,
		ServerGBID:     gbID,
		ServerGBDomain: trim(req.ServerGBDomain),
		ServerIP:       trim(req.ServerIP),
		ServerPort:     req.ServerPort,
		DeviceGBID:     trim(req.DeviceGBID),
		DeviceIP:       trim(req.DeviceIP),
		DevicePort:     req.DevicePort,
		Username:       trim(req.Username),
		Password:       req.Password,
		Expires:        defaultInt64(req.Expires, 3600),
		KeepTimeout:    defaultInt64(req.KeepTimeout, 60),
		Transport:      normalizeTransport(req.Transport),
		CivilCode:      trim(req.CivilCode),
		Status:         model.GbStatusOff,
		RegisterWay:    firstNonEmpty(req.RegisterWay, "local"),
		CharacterSet:   firstNonEmpty(req.CharacterSet, model.GbCharsetGB2312),

		AsMessageChannel:    boolOr(req.AsMessageChannel, false),
		PTZ:                 boolOr(req.PTZ, true),
		RTCP:                boolOr(req.RTCP, false),
		CatalogWithPlatform: boolOr(req.CatalogWithPlatform, true),
		CatalogWithGroup:    req.CatalogWithGroup,
		CatalogWithRegion:   req.CatalogWithRegion,
		AutoPushChannel:     boolOr(req.AutoPushChannel, false),

		ServerID:     trim(req.ServerID),
		Manufacturer: trim(req.Manufacturer),
		Model:        trim(req.Model),
		Address:      trim(req.Address),
		Secrecy:      req.Secrecy,
		SendStreamIP: trim(req.SendStreamIP),
		Remark:       trim(req.Remark),
		Active:       boolOr(req.Active, true),
		Enable:       boolOr(req.Enable, true),
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, err
	}
	// 返回值用于前端回显：密码不能以明文回传（契约 §10.0-2）。
	m.Password = utils.MaskSecret(m.Password)
	return m, nil
}

// Update 编辑上级平台。password 空值/**** 保持原值。
func (s *GbPlatformService) Update(ctx context.Context, id int64, req *request.UpdateGbPlatformReq) error {
	if req == nil {
		return errors.New("请求不能为空")
	}
	m, err := s.getByID(ctx, id)
	if err != nil {
		return err
	}
	fields := map[string]interface{}{}
	setStr := func(key string, v *string) {
		if v != nil {
			fields[key] = trim(*v)
		}
	}
	setInt := func(key string, v *int64) {
		if v != nil {
			fields[key] = *v
		}
	}
	setBool := func(key string, v *bool) {
		if v != nil {
			fields[key] = *v
		}
	}

	if req.ServerGBID != nil {
		v := trim(*req.ServerGBID)
		if v == "" {
			return errors.New("国标编码不能为空")
		}
		if v != m.ServerGBID {
			exists, err := s.repo.ExistsByServerGBID(ctx, v, id)
			if err != nil {
				return err
			}
			if exists {
				return ErrGbPlatformGBIDExists
			}
			fields["server_gb_id"] = v
		}
	}
	if req.Name != nil {
		v := trim(*req.Name)
		if v == "" {
			return errors.New("平台名称不能为空")
		}
		fields["name"] = v
	}
	if req.Transport != nil {
		fields["transport"] = normalizeTransport(*req.Transport)
	}
	if req.Password != nil {
		fields["password"] = utils.ApplyMaskedUpdate(*req.Password, m.Password)
	}

	setStr("server_gb_domain", req.ServerGBDomain)
	setStr("server_ip", req.ServerIP)
	setInt("server_port", req.ServerPort)
	setStr("device_gb_id", req.DeviceGBID)
	setStr("device_ip", req.DeviceIP)
	setInt("device_port", req.DevicePort)
	setStr("username", req.Username)
	setInt("expires", req.Expires)
	setInt("keep_timeout", req.KeepTimeout)
	setStr("civil_code", req.CivilCode)
	setStr("register_way", req.RegisterWay)
	setBool("as_message_channel", req.AsMessageChannel)
	setStr("character_set", req.CharacterSet)
	setBool("ptz", req.PTZ)
	setBool("rtcp", req.RTCP)
	setStr("server_id", req.ServerID)
	setStr("manufacturer", req.Manufacturer)
	setStr("model", req.Model)
	setStr("address", req.Address)
	setInt("secrecy", req.Secrecy)
	setBool("catalog_with_platform", req.CatalogWithPlatform)
	setInt("catalog_with_group", req.CatalogWithGroup)
	setInt("catalog_with_region", req.CatalogWithRegion)
	setBool("auto_push_channel", req.AutoPushChannel)
	setStr("send_stream_ip", req.SendStreamIP)
	setStr("remark", req.Remark)
	setBool("active", req.Active)
	setBool("enable", req.Enable)

	return s.repo.UpdateFields(ctx, id, fields)
}

// Delete 删除上级平台，并级联清理通道/分组/区域映射。
//
// 用事务保证「平台已删、映射还在」的脏数据不会出现 —— 映射表无 dept_id，
// 遗留行不会被列表过滤掉，会一直占用 custom_device_id 唯一索引。
func (s *GbPlatformService) Delete(ctx context.Context, id int64) error {
	if _, err := s.getByID(ctx, id); err != nil {
		return err
	}
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 映射表是纯关联行，没有独立生命周期；物理删除可避免
		// (platform_id, group_id) 这类**非 partial** 唯一索引被软删除行长期占用。
		if err := tx.Unscoped().Where("platform_id = ?", id).Delete(&model.GbPlatformChannel{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("platform_id = ?", id).Delete(&model.GbPlatformGroup{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("platform_id = ?", id).Delete(&model.GbPlatformRegion{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.GbPlatform{}, id).Error
	})
}

// PageChannel 某上级平台下的级联通道分页。
func (s *GbPlatformService) PageChannel(ctx context.Context, platformID int64, req *request.PageGbPlatformChannelReq) (*response.PageGbPlatformChannelResp, error) {
	if _, err := s.getByID(ctx, platformID); err != nil {
		return nil, err
	}
	if req == nil {
		req = &request.PageGbPlatformChannelReq{}
	}
	req.PlatformID = &platformID
	req.Normalize()

	// 映射表无独立的数据权限语义（契约 §14-9：随主表过滤）；
	// 这里显式传 all，避免 repository 对 nil DataScope 解引用。
	rows, total, err := s.chanRepo.Page(ctx, req, &utils.DataScope{DataPerm: "all"})
	if err != nil {
		return nil, err
	}
	items := make([]response.GbPlatformChannelItem, 0, len(rows))
	for _, m := range rows {
		items = append(items, toGbPlatformChannelItem(m))
	}
	// repository 只 Preload 了 Channel，没有 Device；批量补设备名，
	// 避免列表里「通道有名字、设备一栏空白」。
	if err := s.fillDeviceNames(ctx, items); err != nil {
		return nil, err
	}
	return &response.PageGbPlatformChannelResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// fillDeviceNames 批量补齐设备名称。
func (s *GbPlatformService) fillDeviceNames(ctx context.Context, items []response.GbPlatformChannelItem) error {
	if len(items) == 0 {
		return nil
	}
	channelIDs := make([]int64, 0, len(items))
	for _, it := range items {
		if it.ChannelID > 0 {
			channelIDs = append(channelIDs, it.ChannelID)
		}
	}
	if len(channelIDs) == 0 {
		return nil
	}
	var channels []model.GbChannel
	if err := bootstrap.DB().WithContext(ctx).Preload("Device").
		Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
		return err
	}
	nameByChannel := make(map[int64]string, len(channels))
	for _, c := range channels {
		if c.Device != nil {
			nameByChannel[c.ID] = c.Device.Name
		}
	}
	for i := range items {
		if items[i].DeviceName == "" {
			items[i].DeviceName = nameByChannel[items[i].ChannelID]
		}
	}
	return nil
}

// toGbPlatformChannelItem 显式映射，避免序列化 Preload 出来的通道密码。
func toGbPlatformChannelItem(m model.GbPlatformChannel) response.GbPlatformChannelItem {
	out := response.GbPlatformChannelItem{
		ID:                 m.ID,
		PlatformID:         m.PlatformID,
		ChannelID:          m.ChannelID,
		CustomDeviceID:     m.CustomDeviceID,
		CustomName:         m.CustomName,
		CustomManufacturer: m.CustomManufacturer,
		CustomModel:        m.CustomModel,
		CustomOwner:        m.CustomOwner,
		CustomCivilCode:    m.CustomCivilCode,
		CustomAddress:      m.CustomAddress,
		CustomParental:     m.CustomParental,
		CustomParentID:     m.CustomParentID,
		CustomStatus:       m.CustomStatus,
		CustomPTZType:      m.CustomPTZType,
		CustomChannelType:  m.CustomChannelType,
		CreatedDT:          m.CreatedDT,
		UpdatedDT:          m.UpdatedDT,
	}
	if m.Channel != nil {
		out.ChannelName = m.Channel.Name
		out.GbChannelID = m.Channel.GbChannelID
		out.DeviceCode = m.Channel.DeviceCode
		if m.Channel.Device != nil {
			out.DeviceName = m.Channel.Device.Name
		}
	}
	return out
}

// AddChannel 把本地通道加入上级平台的目录（幂等：已存在则跳过）。
func (s *GbPlatformService) AddChannel(ctx context.Context, platformID int64, channelIDs []int64) error {
	if _, err := s.getByID(ctx, platformID); err != nil {
		return err
	}
	ids := dedupIDs(channelIDs)
	if len(ids) == 0 {
		return errors.New("请选择要添加的通道")
	}

	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, channelID := range ids {
			var ch model.GbChannel
			if err := tx.First(&ch, channelID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("通道 %d 不存在", channelID)
				}
				return err
			}
			var existing int64
			if err := tx.Model(&model.GbPlatformChannel{}).
				Where("platform_id = ? AND channel_id = ?", platformID, channelID).
				Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				continue
			}

			// 默认用本地通道国标编码作为上级视角的编码；该列全局唯一，
			// 同一通道推给两个上级平台时需要人工改其中一个的定制编码。
			customID := ch.GbChannelID
			if customID == "" {
				return fmt.Errorf("通道 %d 尚未同步国标编码，无法加入上级目录", channelID)
			}
			var used int64
			if err := tx.Model(&model.GbPlatformChannel{}).
				Where("custom_device_id = ?", customID).Count(&used).Error; err != nil {
				return err
			}
			if used > 0 {
				return ErrGbPlatformChannelIDExists
			}

			row := &model.GbPlatformChannel{
				PlatformID:     platformID,
				ChannelID:      channelID,
				CustomDeviceID: customID,
				CustomName:     ch.Name,
				CustomStatus:   ch.Status,
			}
			if err := tx.Create(row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveChannel 从上级平台目录中移除通道（幂等）。
func (s *GbPlatformService) RemoveChannel(ctx context.Context, platformID int64, channelIDs []int64) error {
	if _, err := s.getByID(ctx, platformID); err != nil {
		return err
	}
	ids := dedupIDs(channelIDs)
	if len(ids) == 0 {
		return errors.New("请选择要移除的通道")
	}
	return bootstrap.DB().WithContext(ctx).
		Where("platform_id = ? AND channel_id IN ?", platformID, ids).
		Delete(&model.GbPlatformChannel{}).Error
}

// UpdateChannelCustom 修改通道在上级平台的定制信息。
func (s *GbPlatformService) UpdateChannelCustom(ctx context.Context, platformID, channelID int64, req *request.UpdateGbPlatformChannelReq) error {
	if req == nil {
		return errors.New("请求不能为空")
	}
	var row model.GbPlatformChannel
	if err := bootstrap.DB().WithContext(ctx).
		Where("platform_id = ? AND channel_id = ?", platformID, channelID).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbPlatformChannelNotFound
		}
		return err
	}

	fields := map[string]interface{}{}
	setStr := func(key string, v *string) {
		if v != nil {
			fields[key] = trim(*v)
		}
	}
	setInt := func(key string, v *int64) {
		if v != nil {
			fields[key] = *v
		}
	}

	if req.CustomDeviceID != nil {
		v := trim(*req.CustomDeviceID)
		if v == "" {
			return errors.New("定制国标编码不能为空")
		}
		if v != row.CustomDeviceID {
			var used int64
			if err := bootstrap.DB().WithContext(ctx).Model(&model.GbPlatformChannel{}).
				Where("custom_device_id = ? AND id <> ?", v, row.ID).Count(&used).Error; err != nil {
				return err
			}
			if used > 0 {
				return ErrGbPlatformChannelIDExists
			}
			fields["custom_device_id"] = v
		}
	}
	setStr("custom_name", req.CustomName)
	setStr("custom_manufacturer", req.CustomManufacturer)
	setStr("custom_model", req.CustomModel)
	setStr("custom_owner", req.CustomOwner)
	setStr("custom_civil_code", req.CustomCivilCode)
	setStr("custom_address", req.CustomAddress)
	setInt("custom_parental", req.CustomParental)
	setStr("custom_parent_id", req.CustomParentID)
	setStr("custom_status", req.CustomStatus)
	setInt("custom_ptz_type", req.CustomPTZType)
	setInt("custom_channel_type", req.CustomChannelType)

	if len(fields) == 0 {
		return nil
	}
	return bootstrap.DB().WithContext(ctx).Model(&model.GbPlatformChannel{}).
		Where("id = ?", row.ID).Updates(fields).Error
}

// SetGroups 设置「按分组推送目录」的可见范围。
//
// 不直接调用 GbPlatformGroupRepo.ReplaceForPlatform：该方法「先软删后插入」，
// 而 uk_ac_gb_platform_group_platform_group 是**非 partial** 唯一索引，
// 同一 (platform_id, group_id) 第二次保存会撞索引。这里改为事务内物理删除后插入。
func (s *GbPlatformService) SetGroups(ctx context.Context, platformID int64, groupIDs []int64) error {
	if _, err := s.getByID(ctx, platformID); err != nil {
		return err
	}
	ids := dedupIDs(groupIDs)
	if err := validateIDsExist(ctx, &model.GbGroup{}, ids); err != nil {
		return err
	}
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("platform_id = ?", platformID).
			Delete(&model.GbPlatformGroup{}).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		rows := make([]model.GbPlatformGroup, 0, len(ids))
		for _, gid := range ids {
			rows = append(rows, model.GbPlatformGroup{PlatformID: platformID, GroupID: gid})
		}
		return tx.Create(&rows).Error
	})
}

// SetRegions 设置「按区域推送目录」的可见范围（同 SetGroups，绕开非 partial 唯一索引）。
func (s *GbPlatformService) SetRegions(ctx context.Context, platformID int64, regionIDs []int64) error {
	if _, err := s.getByID(ctx, platformID); err != nil {
		return err
	}
	ids := dedupIDs(regionIDs)
	if err := validateIDsExist(ctx, &model.Region{}, ids); err != nil {
		return err
	}
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("platform_id = ?", platformID).
			Delete(&model.GbPlatformRegion{}).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		rows := make([]model.GbPlatformRegion, 0, len(ids))
		for _, rid := range ids {
			rows = append(rows, model.GbPlatformRegion{PlatformID: platformID, RegionID: rid})
		}
		return tx.Create(&rows).Error
	})
}

// validateIDsExist 校验一组主键是否都存在，避免写入指向不存在实体的映射。
func validateIDsExist(ctx context.Context, model any, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	var found int64
	if err := bootstrap.DB().WithContext(ctx).Model(model).Where("id IN ?", ids).Count(&found).Error; err != nil {
		return err
	}
	if found != int64(len(ids)) {
		return fmt.Errorf("部分 ID 不存在：提交 %d 个，实际找到 %d 个", len(ids), found)
	}
	return nil
}

func (s *GbPlatformService) getByID(ctx context.Context, id int64) (*model.GbPlatform, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbPlatformNotFound
		}
		return nil, err
	}
	return m, nil
}

// normalizeTransport SIP 传输只接受 UDP/TCP。
func normalizeTransport(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), model.GbSipTransportTCP) {
		return model.GbSipTransportTCP
	}
	return model.GbSipTransportUDP
}
