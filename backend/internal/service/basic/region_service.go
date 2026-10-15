package basic

import (
	"context"
	"fmt"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
)

// RegionService 区域业务逻辑。
type RegionService struct{}

// NewRegionService 构造区域服务。
func NewRegionService() *RegionService { return &RegionService{} }

// Page 分页查询区域。
func (s *RegionService) Page(ctx context.Context, req *request.PageRegionReq, ds *utils.DataScope) (*response.PageRegionResp, error) {
	req.Normalize()
	list, total, err := repository.NewRegionRepo().Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}
	return &response.PageRegionResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    list,
	}, nil
}

// Options 区域下拉选项。
func (s *RegionService) Options(ctx context.Context, ds *utils.DataScope) ([]response.OptionItem, error) {
	list, err := repository.NewRegionRepo().Options(ctx, ds)
	if err != nil {
		return nil, err
	}
	items := make([]response.OptionItem, 0, len(list))
	for i := range list {
		items = append(items, response.OptionItem{ID: list[i].ID, Name: list[i].Name, Code: list[i].Code})
	}
	return items, nil
}

// Get 区域详情。
func (s *RegionService) Get(ctx context.Context, id int64) (*model.Region, error) {
	return repository.NewRegionRepo().GetByID(ctx, id)
}

// Create 新增区域。
func (s *RegionService) Create(ctx context.Context, req *request.CreateRegionReq) (*model.Region, error) {
	m := &model.Region{
		Name:     req.Name,
		Code:     req.Code,
		Sequence: req.Sequence,
		Active:   req.Active,
		Remark:   req.Remark,
	}
	// 显式补默认值，保证返回体与库中落库值一致（GORM 对带 default 标签的零值字段会走库默认值）。
	if m.Sequence == 0 {
		m.Sequence = 10
	}
	if m.Active == nil {
		active := true
		m.Active = &active
	}
	if err := repository.NewRegionRepo().Create(ctx, m); err != nil {
		return nil, fmt.Errorf("新增区域失败: %w", err)
	}
	return m, nil
}

// Update 修改区域。
func (s *RegionService) Update(ctx context.Context, id int64, req *request.UpdateRegionReq) error {
	repo := repository.NewRegionRepo()
	m, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	m.Name = req.Name
	m.Code = req.Code
	if req.Sequence != 0 {
		m.Sequence = req.Sequence
	}
	if req.Active != nil {
		m.Active = req.Active
	}
	m.Remark = req.Remark

	if err := repo.Update(ctx, m); err != nil {
		return fmt.Errorf("修改区域失败: %w", err)
	}
	return nil
}

// regionRefs 区域的引用检查清单。
//
// 与需求文档「02-基础信息模块 §5.2 引用保护流程」逐条对应：区域被下列
// 业务表以外键引用时拒绝删除。新增引用区域的功能时，**必须同步扩充本清单**，
// 否则会产生悬挂在已删除区域上的记录，使按区域筛选失效。
var regionRefs = []struct {
	table  string
	column string
	label  string
}{
	{"ac_device", "region_id", "台设备"},
	{"ac_risk_record", "region_id", "条风险记录"},
	{"ac_alarm_record", "region_id", "条告警记录"},
	{"ac_inspect_task", "region_id", "个巡检任务"},
	{"ac_inspect_record", "region_id", "条巡检记录"},
	{"ac_iot_card", "region_id", "张物联网卡"},
	{"ac_voice_config", "region_id", "条语音配置"},
	{"ac_device_upgrade_plan", "region_id", "条升级计划"},
	{"ac_device_upgrade_record", "region_id", "条升级记录"},
}

// Delete 软删除区域。被任何业务表引用时拒绝删除。
func (s *RegionService) Delete(ctx context.Context, id int64) error {
	repo := repository.NewRegionRepo()
	if _, err := repo.GetByID(ctx, id); err != nil {
		return err
	}

	for _, ref := range regionRefs {
		n, err := repo.CountByColumn(ctx, ref.table, ref.column, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("该区域下存在 %d %s，无法删除", n, ref.label)
		}
	}

	if err := repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除区域失败: %w", err)
	}
	return nil
}
