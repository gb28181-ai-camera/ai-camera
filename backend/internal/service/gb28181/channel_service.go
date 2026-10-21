package gb28181

import (
	"context"
	"errors"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标通道
// ════════════════════════════════════════════════════════════════

// GbChannelService 国标通道业务逻辑。
type GbChannelService struct {
	repo       *repository.GbChannelRepo
	groupRepo  *repository.GbGroupRepo
	planRepo   *repository.GbRecordPlanRepo
	regionRepo *repository.RegionRepo
}

// NewGbChannelService 构造国标通道服务。
func NewGbChannelService() *GbChannelService {
	return &GbChannelService{
		repo:       repository.NewGbChannelRepo(),
		groupRepo:  repository.NewGbGroupRepo(),
		planRepo:   repository.NewGbRecordPlanRepo(),
		regionRepo: repository.NewRegionRepo(),
	}
}

// Page 国标通道分页列表。
func (s *GbChannelService) Page(ctx context.Context, req *request.PageGbChannelReq, ds *utils.DataScope) (*response.PageGbChannelResp, error) {
	req.Normalize()
	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.GbChannelItem, 0, len(items))
	for _, c := range items {
		out = append(out, toGbChannelItem(c))
	}
	return &response.PageGbChannelResp{
		Items:    out,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Get 国标通道详情。
func (s *GbChannelService) Get(ctx context.Context, id int64) (*response.GbChannelDetailResp, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbChannelNotFound
		}
		return nil, err
	}
	return &response.GbChannelDetailResp{
		GbChannelItem: toGbChannelItem(*c),
		Password:      utils.MaskSecret(c.Password),
	}, nil
}

// Update 编辑国标通道。
//
// 增量语义：字符串非空才覆盖，指针字段非 nil 才覆盖；
// RegionID / GroupID / RecordPlanID 传 0 表示解绑（写 NULL）。
func (s *GbChannelService) Update(ctx context.Context, id int64, req *request.UpdateGbChannelReq) error {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbChannelNotFound
		}
		return err
	}

	fields := map[string]interface{}{}

	if v := trim(req.Name); v != "" {
		fields["name"] = v
	}
	if v := trim(req.Manufacturer); v != "" {
		fields["manufacturer"] = v
	}
	if v := trim(req.Model); v != "" {
		fields["model"] = v
	}
	if v := trim(req.Owner); v != "" {
		fields["owner"] = v
	}
	if v := trim(req.CivilCode); v != "" {
		fields["civil_code"] = v
	}
	if v := trim(req.Address); v != "" {
		fields["address"] = v
	}
	if v := trim(req.ParentID); v != "" {
		fields["parent_id"] = v
	}
	if v := trim(req.Status); v != "" {
		fields["status"] = v
	}
	if v := trim(req.StreamID); v != "" {
		fields["stream_id"] = v
	}
	if v := trim(req.Remark); v != "" {
		fields["remark"] = v
	}

	if req.ChannelType != nil {
		fields["channel_type"] = *req.ChannelType
	}
	if req.PtzType != nil {
		fields["ptz_type"] = *req.PtzType
	}
	if req.HasAudio != nil {
		fields["has_audio"] = *req.HasAudio
	}
	if req.EnableBroadcast != nil {
		fields["enable_broadcast"] = *req.EnableBroadcast
	}
	if req.Active != nil {
		fields["active"] = *req.Active
	}

	if req.RegionID != nil {
		regionID, err := s.normalizeRegion(ctx, req.RegionID)
		if err != nil {
			return err
		}
		fields["region_id"] = regionID
	}
	if req.GroupID != nil {
		groupID, err := s.normalizeGroup(ctx, req.GroupID)
		if err != nil {
			return err
		}
		fields["group_id"] = groupID
	}
	if req.RecordPlanID != nil {
		planID, err := s.normalizePlan(ctx, req.RecordPlanID)
		if err != nil {
			return err
		}
		fields["record_plan_id"] = planID
	}

	// 敏感字段：空值 / **** 保持原值，避免前端回显值覆盖真实密码。
	if trim(req.Password) != "" {
		fields["password"] = utils.ApplyMaskedUpdate(req.Password, c.Password)
	}

	return s.repo.UpdateFields(ctx, id, fields)
}

// Delete 软删除通道。
func (s *GbChannelService) Delete(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbChannelNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}

// UpdateRegion 设置通道所属区域；regionID 为 nil/0 表示清空。
func (s *GbChannelService) UpdateRegion(ctx context.Context, id int64, regionID *int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbChannelNotFound
		}
		return err
	}
	value, err := s.normalizeRegion(ctx, regionID)
	if err != nil {
		return err
	}
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{"region_id": value})
}

// UpdateGroup 设置通道所属业务分组；groupID 为 nil/0 表示清空。
func (s *GbChannelService) UpdateGroup(ctx context.Context, id int64, groupID *int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbChannelNotFound
		}
		return err
	}
	value, err := s.normalizeGroup(ctx, groupID)
	if err != nil {
		return err
	}
	return s.repo.UpdateFields(ctx, id, map[string]interface{}{"group_id": value})
}

// Reset 重置通道的平台侧自定义字段，等待下一次 Catalog 同步重新填充。
//
// 只清空「平台可编辑、且可由设备重新上报」的字段，**不物理删除通道**：
//   - name：设备 Catalog 会重新下发真实名称；
//   - region_id / group_id：纯平台侧组织关系，清空后由管理员重新分配；
//   - stream_id / record_plan_id / enable_broadcast：平台侧配置；
//   - is_custom / remark / sync_dt：重置同步标记与备注。
//
// 保留 gb_channel_id、device_id、status、设备上报的属性（厂商/型号/地址等）：
// 这些来自设备，删掉反而会造成「重置后信息丢失直到下次同步」。
func (s *GbChannelService) Reset(ctx context.Context, id int64) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbChannelNotFound
		}
		return err
	}
	fields := map[string]interface{}{
		"name":             "",
		"region_id":        nil,
		"group_id":         nil,
		"stream_id":        "",
		"record_plan_id":   nil,
		"enable_broadcast": false,
		"is_custom":        false,
		"remark":           "",
		"sync_dt":          nil,
	}
	return s.repo.UpdateFields(ctx, id, fields)
}

// Options 通道下拉选项（仅启用状态）。
func (s *GbChannelService) Options(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	items, err := s.repo.Options(ctx, ds)
	if err != nil {
		return nil, err
	}
	out := make([]response.OptionItem, 0, len(items))
	for _, c := range items {
		out = append(out, response.OptionItem{ID: c.ID, Name: c.Name, Code: c.GbChannelID})
	}
	return out, nil
}

// ── 内部工具 ──────────────────────────────────────────────────

// normalizeRegion 校验区域并返回可写库的值（nil 表示清空）。
func (s *GbChannelService) normalizeRegion(ctx context.Context, regionID *int64) (interface{}, error) {
	if regionID == nil || *regionID <= 0 {
		return nil, nil
	}
	if _, err := s.regionRepo.GetByID(ctx, *regionID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("区域不存在")
		}
		return nil, err
	}
	return *regionID, nil
}

// normalizeGroup 校验业务分组并返回可写库的值。
func (s *GbChannelService) normalizeGroup(ctx context.Context, groupID *int64) (interface{}, error) {
	if groupID == nil || *groupID <= 0 {
		return nil, nil
	}
	if _, err := s.groupRepo.GetByID(ctx, *groupID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbGroupNotFound
		}
		return nil, err
	}
	return *groupID, nil
}

// normalizePlan 校验录像计划并返回可写库的值。
func (s *GbChannelService) normalizePlan(ctx context.Context, planID *int64) (interface{}, error) {
	if planID == nil || *planID <= 0 {
		return nil, nil
	}
	if _, err := s.planRepo.GetByID(ctx, *planID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("录像计划不存在")
		}
		return nil, err
	}
	return *planID, nil
}
