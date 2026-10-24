package gb28181

// 录像计划（周计划）CRUD + 调度接线。
//
// 计划配置由本文件维护，到点录制由 internal/gb28181/record 的 Scheduler 执行：
//   - 构造函数里调用 record.EnsureStarted() 懒启动进程级调度器（路由注册即触发，
//     因此不需要修改 cmd/server）；
//   - 计划新增/编辑/删除/换绑通道后调用 record.NotifyChanged()，让调度器
//     立即重新对齐，而不是等下一个 30 秒 ticker；
//   - 跨天时间段（如 22:00–06:00）用 start > end 表示，调度器按「前一天」
//     的 WeekDay 匹配凌晨部分。
//
// 数据关系：
//   - ac_gb_record_plan_item 存「一周内的时间段」，无 channel_id；
//   - 计划与通道的绑定关系存在 ac_gb_channel.record_plan_id（DDL 09-国标视频.sql）。
// 因此 LinkChannels 操作的是通道表，而不是明细表。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/record"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

var (
	// ErrGbRecordPlanNotFound 录像计划不存在。
	ErrGbRecordPlanNotFound = errors.New("录像计划不存在")
	// ErrGbRecordPlanInUse 计划仍被通道引用，不能删除。
	ErrGbRecordPlanInUse = errors.New("该计划仍被通道引用，请先解除绑定再删除")
)

// GbRecordPlanService 录像计划业务逻辑。
type GbRecordPlanService struct {
	planRepo *repository.GbRecordPlanRepo
	itemRepo *repository.GbRecordPlanItemRepo
}

// NewGbRecordPlanService 构造录像计划服务。
//
// 调度器接线点：处理器在路由注册阶段被构造 → 这里被调用 → EnsureStarted
// 用 sync.Once 保证全进程只启动一个 Scheduler。放在这里的原因是不能修改
// cmd/server，而 handler 的构造时机恰好等价于「服务启动」。
func NewGbRecordPlanService() *GbRecordPlanService {
	record.EnsureStarted()
	return &GbRecordPlanService{
		planRepo: repository.NewGbRecordPlanRepo(),
		itemRepo: repository.NewGbRecordPlanItemRepo(),
	}
}

// Page 录像计划分页。
func (s *GbRecordPlanService) Page(ctx context.Context, req *request.PageGbRecordPlanReq, ds *utils.DataScope) (*response.PageGbRecordPlanResp, error) {
	if req == nil {
		req = &request.PageGbRecordPlanReq{}
	}
	req.Normalize()
	plans, total, err := s.planRepo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(plans))
	for _, p := range plans {
		ids = append(ids, p.ID)
	}
	itemCounts, channelCounts, err := s.countRelations(ctx, ids)
	if err != nil {
		return nil, err
	}

	items := make([]response.GbRecordPlanItem, 0, len(plans))
	for _, p := range plans {
		items = append(items, response.GbRecordPlanItem{
			ID:           p.ID,
			Active:       p.Active,
			Name:         p.Name,
			Snap:         p.Snap,
			Remark:       p.Remark,
			ItemCount:    int(itemCounts[p.ID]),
			ChannelCount: channelCounts[p.ID],
			CreatedDT:    p.CreatedDT,
			UpdatedDT:    p.UpdatedDT,
		})
	}
	return &response.PageGbRecordPlanResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// countRelations 批量统计计划的时间段数与绑定通道数，避免逐行 N+1。
func (s *GbRecordPlanService) countRelations(ctx context.Context, planIDs []int64) (map[int64]int64, map[int64]int64, error) {
	itemCounts := map[int64]int64{}
	channelCounts := map[int64]int64{}
	if len(planIDs) == 0 {
		return itemCounts, channelCounts, nil
	}

	type cntRow struct {
		Key   int64 `gorm:"column:k"`
		Count int64 `gorm:"column:c"`
	}

	var itemRows []cntRow
	if err := bootstrap.DB().WithContext(ctx).
		Model(&model.GbRecordPlanItem{}).
		Select("plan_id AS k, COUNT(*) AS c").
		Where("plan_id IN ?", planIDs).
		Group("plan_id").
		Scan(&itemRows).Error; err != nil {
		return nil, nil, err
	}
	for _, r := range itemRows {
		itemCounts[r.Key] = r.Count
	}

	var channelRows []cntRow
	if err := bootstrap.DB().WithContext(ctx).
		Model(&model.GbChannel{}).
		Select("record_plan_id AS k, COUNT(*) AS c").
		Where("record_plan_id IN ?", planIDs).
		Group("record_plan_id").
		Scan(&channelRows).Error; err != nil {
		return nil, nil, err
	}
	for _, r := range channelRows {
		channelCounts[r.Key] = r.Count
	}
	return itemCounts, channelCounts, nil
}

// Get 录像计划详情（时间段 + 已绑定通道）。
func (s *GbRecordPlanService) Get(ctx context.Context, id int64) (*response.GbRecordPlanDetailResp, error) {
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGbRecordPlanNotFound
		}
		return nil, err
	}

	items, err := s.itemRepo.ListByPlanID(ctx, id)
	if err != nil {
		return nil, err
	}
	channels, err := s.listPlanChannels(ctx, id)
	if err != nil {
		return nil, err
	}

	itemResp := make([]response.GbRecordPlanItemResp, 0, len(items))
	for _, it := range items {
		itemResp = append(itemResp, response.GbRecordPlanItemResp{
			ID:        it.ID,
			WeekDay:   it.WeekDay,
			StartTime: it.StartTime,
			EndTime:   it.EndTime,
		})
	}
	channelIDs := make([]int64, 0, len(channels))
	for _, c := range channels {
		channelIDs = append(channelIDs, c.ID)
	}

	return &response.GbRecordPlanDetailResp{
		GbRecordPlanItem: response.GbRecordPlanItem{
			ID:           plan.ID,
			Active:       plan.Active,
			Name:         plan.Name,
			Snap:         plan.Snap,
			Remark:       plan.Remark,
			ItemCount:    len(items),
			ChannelCount: int64(len(channels)),
			CreatedDT:    plan.CreatedDT,
			UpdatedDT:    plan.UpdatedDT,
		},
		Items:      itemResp,
		ChannelIDs: channelIDs,
		Channels:   channels,
	}, nil
}

// listPlanChannels 查询绑定到计划的通道。
func (s *GbRecordPlanService) listPlanChannels(ctx context.Context, planID int64) ([]response.GbRecordPlanChannelItem, error) {
	var rows []model.GbChannel
	if err := bootstrap.DB().WithContext(ctx).
		Preload("Device").
		Where("record_plan_id = ?", planID).
		Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]response.GbRecordPlanChannelItem, 0, len(rows))
	for _, c := range rows {
		deviceName := ""
		if c.Device != nil {
			deviceName = c.Device.Name
		}
		out = append(out, response.GbRecordPlanChannelItem{
			ID:          c.ID,
			Name:        c.Name,
			GbChannelID: c.GbChannelID,
			DeviceCode:  c.DeviceCode,
			DeviceName:  deviceName,
			Status:      c.Status,
		})
	}
	return out, nil
}

// Create 新增录像计划及其时间段（同一事务）。
func (s *GbRecordPlanService) Create(ctx context.Context, req *request.CreateGbRecordPlanReq) (*model.GbRecordPlan, error) {
	if req == nil {
		return nil, errors.New("请求不能为空")
	}
	name := trim(req.Name)
	if name == "" {
		return nil, errors.New("计划名称不能为空")
	}
	items, err := normalizePlanItems(req.Items)
	if err != nil {
		return nil, err
	}

	plan := &model.GbRecordPlan{
		Name:   name,
		Snap:   boolOr(req.Snap, false),
		Remark: trim(req.Remark),
		Active: boolOr(req.Active, true),
	}
	err = bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(plan).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].PlanID = plan.ID
		}
		if len(items) > 0 {
			if err := tx.Create(&items).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 新计划可能已经覆盖当前时刻：立刻让调度器对齐，避免等 30 秒。
	record.NotifyChanged()
	return plan, nil
}

// Update 编辑录像计划。
//
// Items 为 nil 时不修改时间段；显式传空数组时清空全部时间段。
func (s *GbRecordPlanService) Update(ctx context.Context, id int64, req *request.UpdateGbRecordPlanReq) error {
	if req == nil {
		return errors.New("请求不能为空")
	}
	plan, err := s.planRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbRecordPlanNotFound
		}
		return err
	}

	var items []model.GbRecordPlanItem
	if req.Items != nil {
		items, err = normalizePlanItems(req.Items)
		if err != nil {
			return err
		}
	}

	if req.Name != "" {
		v := trim(req.Name)
		if v == "" {
			return errors.New("计划名称不能为空")
		}
		plan.Name = v
	}
	if req.Active != nil {
		plan.Active = req.Active
	}
	if req.Snap != nil {
		plan.Snap = req.Snap
	}
	if req.Remark != nil {
		plan.Remark = trim(*req.Remark)
	}

	err = bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(plan).Error; err != nil {
			return err
		}
		if req.Items == nil {
			return nil
		}
		// 明细是整体提交的集合（无独立身份），先软删后批量插入，
		// 与 GbRecordPlanItemRepo.ReplaceForPlan 的语义一致。
		if err := tx.Where("plan_id = ?", id).Delete(&model.GbRecordPlanItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for i := range items {
			items[i].PlanID = id
		}
		return tx.Create(&items).Error
	})
	if err != nil {
		return err
	}
	// 时间段被整体重建（item id 变化），调度器会据此停止旧任务并启动新任务。
	record.NotifyChanged()
	return nil
}

// Delete 删除录像计划（有通道引用时拒绝）。
func (s *GbRecordPlanService) Delete(ctx context.Context, id int64) error {
	if _, err := s.planRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbRecordPlanNotFound
		}
		return err
	}
	inUse, err := s.planRepo.CountChannels(ctx, id)
	if err != nil {
		return err
	}
	if inUse > 0 {
		return ErrGbRecordPlanInUse
	}
	err = bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", id).Delete(&model.GbRecordPlanItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.GbRecordPlan{}, id).Error
	})
	if err == nil {
		record.NotifyChanged()
	}
	return err
}

// LinkChannels 整体替换计划绑定的通道集合。
//
// 关系列是 ac_gb_channel.record_plan_id：传空数组等于解除该计划的全部通道。
// 不在任何计划里的通道保持不动。
func (s *GbRecordPlanService) LinkChannels(ctx context.Context, id int64, channelIDs []int64) error {
	if _, err := s.planRepo.GetByID(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrGbRecordPlanNotFound
		}
		return err
	}

	ids := dedupIDs(channelIDs)

	err := bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(ids) > 0 {
			var found int64
			if err := tx.Model(&model.GbChannel{}).Where("id IN ?", ids).Count(&found).Error; err != nil {
				return err
			}
			if found != int64(len(ids)) {
				return fmt.Errorf("部分通道不存在：提交 %d 个，实际找到 %d 个", len(ids), found)
			}
			// 先把原先绑定本计划、但本次未提交的通道解绑。
			if err := tx.Model(&model.GbChannel{}).
				Where("record_plan_id = ? AND id NOT IN ?", id, ids).
				Update("record_plan_id", nil).Error; err != nil {
				return err
			}
			// 再绑定本次提交的通道（可能是从其他计划迁移过来）。
			return tx.Model(&model.GbChannel{}).
				Where("id IN ?", ids).
				Update("record_plan_id", id).Error
		}
		return tx.Model(&model.GbChannel{}).
			Where("record_plan_id = ?", id).
			Update("record_plan_id", nil).Error
	})
	if err != nil {
		return err
	}
	// 解绑/换绑会改变「哪些通道该录」，立即对齐。
	record.NotifyChanged()
	return nil
}

// normalizePlanItems 校验并归一化时间段。
//
// 时间统一存 HH:mm:ss（模型注释如此约定），但前端常见 HH:mm，
// 这里一并接受并补秒，避免「08:00」被设备侧当成非法时间。
func normalizePlanItems(in []request.GbRecordPlanItemReq) ([]model.GbRecordPlanItem, error) {
	out := make([]model.GbRecordPlanItem, 0, len(in))
	for idx, it := range in {
		if it.WeekDay < 0 || it.WeekDay > 6 {
			return nil, fmt.Errorf("第 %d 个时间段：week_day 必须在 0（周日）~ 6（周六）之间", idx+1)
		}
		start, err := normalizeClock(it.StartTime)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个时间段开始时间非法: %w", idx+1, err)
		}
		end, err := normalizeClock(it.EndTime)
		if err != nil {
			return nil, fmt.Errorf("第 %d 个时间段结束时间非法: %w", idx+1, err)
		}
		// 允许 end < start：这表示**跨天时段**（如 22:00–06:00），
		// 由调度器按「开始那天的 WeekDay」匹配，次日凌晨仍属于同一窗口。
		// 只拒绝起止完全相同（零长度窗口没有意义）。
		if end == start {
			return nil, fmt.Errorf("第 %d 个时间段的开始与结束时间不能相同", idx+1)
		}
		out = append(out, model.GbRecordPlanItem{
			WeekDay:   it.WeekDay,
			StartTime: start,
			EndTime:   end,
		})
	}
	return out, nil
}

// normalizeClock 归一化 HH:mm / HH:mm:ss 为 HH:mm:ss。
func normalizeClock(s string) (string, error) {
	v := strings.TrimSpace(s)
	if v == "" {
		return "", errors.New("时间不能为空")
	}
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.Format("15:04:05"), nil
		}
	}
	return "", errors.New("格式应为 HH:mm:ss")
}

// dedupIDs 去重并剔除非法 ID。
func dedupIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
