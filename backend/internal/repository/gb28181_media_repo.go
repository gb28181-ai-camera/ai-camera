package repository

import (
	"context"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 国标视频数据访问（二）：媒体节点 / 拉流代理 / 推流 / 云端录像 / 录像计划
// ════════════════════════════════════════════════════════════════

// ── 流媒体节点 ────────────────────────────────────────────────

// GbMediaServerRepo 流媒体节点数据访问。表 ac_gb_media_server（含 dept_id）。
type GbMediaServerRepo struct{}

// NewGbMediaServerRepo 构造媒体节点仓储。
func NewGbMediaServerRepo() *GbMediaServerRepo { return &GbMediaServerRepo{} }

// Page 分页查询媒体节点。
func (r *GbMediaServerRepo) Page(ctx context.Context, req *request.PageGbMediaServerReq, ds *utils.DataScope) ([]model.GbMediaServer, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbMediaServer{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR server_id ILIKE ? OR ip ILIKE ?)", like, like, like)
		}
		if req.Online != nil {
			q = q.Where("online = ?", *req.Online)
		}
		return ds.ApplyToQuery(q, "ac_gb_media_server.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbMediaServer
	err := build().Order("id ASC").Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// ListAll 查询全部启用节点（自动选节点时用）。
func (r *GbMediaServerRepo) ListAll(ctx context.Context) ([]model.GbMediaServer, error) {
	var items []model.GbMediaServer
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbMediaServer{}).
		Where("(active IS NULL OR active = true)").
		Order("default_server DESC, id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询节点详情。
func (r *GbMediaServerRepo) GetByID(ctx context.Context, id int64) (*model.GbMediaServer, error) {
	var m model.GbMediaServer
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByServerID 按 ZLM 节点唯一标识查询（ZLM 回调入向用）。
func (r *GbMediaServerRepo) GetByServerID(ctx context.Context, serverID string) (*model.GbMediaServer, error) {
	var m model.GbMediaServer
	if err := bootstrap.DB().WithContext(ctx).
		Where("server_id = ?", serverID).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetDefault 查询默认节点（default_server=true，取最新一条）。
func (r *GbMediaServerRepo) GetDefault(ctx context.Context) (*model.GbMediaServer, error) {
	var m model.GbMediaServer
	if err := bootstrap.DB().WithContext(ctx).
		Where("default_server = true").
		Order("id ASC").First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增节点。
func (r *GbMediaServerRepo) Create(ctx context.Context, m *model.GbMediaServer) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存节点。
func (r *GbMediaServerRepo) Update(ctx context.Context, m *model.GbMediaServer) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新节点字段（心跳探测更新 online / last_keepalive_dt 等）。
func (r *GbMediaServerRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbMediaServer{}).Where("id = ?", id).Updates(fields).Error
}

// ClearDefaultExcept 清除除 keepID 外的默认节点标记（保证全局唯一默认节点）。
func (r *GbMediaServerRepo) ClearDefaultExcept(ctx context.Context, keepID int64) error {
	return bootstrap.DB().WithContext(ctx).Model(&model.GbMediaServer{}).
		Where("id <> ?", keepID).
		Update("default_server", false).Error
}

// Delete 软删除节点。
func (r *GbMediaServerRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbMediaServer{}, id).Error
}

// ── 拉流代理 ──────────────────────────────────────────────────

// GbStreamProxyRepo 拉流代理数据访问。表 ac_gb_stream_proxy（含 dept_id）。
type GbStreamProxyRepo struct{}

// NewGbStreamProxyRepo 构造拉流代理仓储。
func NewGbStreamProxyRepo() *GbStreamProxyRepo { return &GbStreamProxyRepo{} }

// Page 分页查询拉流代理。
func (r *GbStreamProxyRepo) Page(ctx context.Context, req *request.PageGbStreamProxyReq, ds *utils.DataScope) ([]model.GbStreamProxy, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbStreamProxy{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR app ILIKE ? OR stream ILIKE ? OR src_url ILIKE ?)", like, like, like, like)
		}
		if req.Pulling != nil {
			q = q.Where("pulling = ?", *req.Pulling)
		}
		if s := strings.TrimSpace(req.MediaServerID); s != "" {
			q = q.Where("media_server_id = ?", s)
		}
		return ds.ApplyToQuery(q, "ac_gb_stream_proxy.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbStreamProxy
	err := build().Order("id DESC").Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// GetByID 按主键查询拉流代理。
func (r *GbStreamProxyRepo) GetByID(ctx context.Context, id int64) (*model.GbStreamProxy, error) {
	var m model.GbStreamProxy
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByAppStream 按 app+stream 查询（唯一键）。
func (r *GbStreamProxyRepo) GetByAppStream(ctx context.Context, app, stream string) (*model.GbStreamProxy, error) {
	var m model.GbStreamProxy
	if err := bootstrap.DB().WithContext(ctx).
		Where("app = ? AND stream = ?", app, stream).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增拉流代理。
func (r *GbStreamProxyRepo) Create(ctx context.Context, m *model.GbStreamProxy) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存拉流代理。
func (r *GbStreamProxyRepo) Update(ctx context.Context, m *model.GbStreamProxy) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新（启停 / pulling 状态）。
func (r *GbStreamProxyRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbStreamProxy{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除拉流代理。
func (r *GbStreamProxyRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbStreamProxy{}, id).Error
}

// ── 推流 ──────────────────────────────────────────────────────

// GbStreamPushRepo 推流数据访问。表 ac_gb_stream_push（含 dept_id）。
type GbStreamPushRepo struct{}

// NewGbStreamPushRepo 构造推流仓储。
func NewGbStreamPushRepo() *GbStreamPushRepo { return &GbStreamPushRepo{} }

// Page 分页查询推流。
func (r *GbStreamPushRepo) Page(ctx context.Context, req *request.PageGbStreamPushReq, ds *utils.DataScope) ([]model.GbStreamPush, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbStreamPush{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(name ILIKE ? OR app ILIKE ? OR stream ILIKE ?)", like, like, like)
		}
		if req.Pushing != nil {
			q = q.Where("pushing = ?", *req.Pushing)
		}
		if s := strings.TrimSpace(req.MediaServerID); s != "" {
			q = q.Where("media_server_id = ?", s)
		}
		if s := strings.TrimSpace(req.Status); s != "" {
			q = q.Where("status = ?", s)
		}
		return ds.ApplyToQuery(q, "ac_gb_stream_push.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbStreamPush
	err := build().Order("id DESC").Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// GetByID 按主键查询推流。
func (r *GbStreamPushRepo) GetByID(ctx context.Context, id int64) (*model.GbStreamPush, error) {
	var m model.GbStreamPush
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByAppStream 按 app+stream 查询（唯一键）。
func (r *GbStreamPushRepo) GetByAppStream(ctx context.Context, app, stream string) (*model.GbStreamPush, error) {
	var m model.GbStreamPush
	if err := bootstrap.DB().WithContext(ctx).
		Where("app = ? AND stream = ?", app, stream).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增推流。
func (r *GbStreamPushRepo) Create(ctx context.Context, m *model.GbStreamPush) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存推流。
func (r *GbStreamPushRepo) Update(ctx context.Context, m *model.GbStreamPush) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// UpdateFields 局部更新（启停 / pushing 状态）。
func (r *GbStreamPushRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbStreamPush{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除推流。
func (r *GbStreamPushRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbStreamPush{}, id).Error
}

// ── 云端录像 ──────────────────────────────────────────────────

// GbCloudRecordRepo 云端录像数据访问。表 ac_gb_cloud_record（含 dept_id）。
type GbCloudRecordRepo struct{}

// NewGbCloudRecordRepo 构造云端录像仓储。
func NewGbCloudRecordRepo() *GbCloudRecordRepo { return &GbCloudRecordRepo{} }

// Page 分页查询云端录像。Preload 关联设备/通道，列表直接展示名称。
func (r *GbCloudRecordRepo) Page(ctx context.Context, req *request.PageGbCloudRecordReq, ds *utils.DataScope) ([]model.GbCloudRecord, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbCloudRecord{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			like := "%" + kw + "%"
			q = q.Where("(file_name ILIKE ? OR stream ILIKE ? OR call_id ILIKE ?)", like, like, like)
		}
		if s := strings.TrimSpace(req.App); s != "" {
			q = q.Where("app = ?", s)
		}
		if s := strings.TrimSpace(req.Stream); s != "" {
			q = q.Where("stream = ?", s)
		}
		if req.DeviceID != nil {
			q = q.Where("device_id = ?", *req.DeviceID)
		}
		if req.ChannelID != nil {
			q = q.Where("channel_id = ?", *req.ChannelID)
		}
		if s := strings.TrimSpace(req.MediaServerID); s != "" {
			q = q.Where("media_server_id = ?", s)
		}
		if req.Collect != nil {
			q = q.Where("collect = ?", *req.Collect)
		}
		if s := strings.TrimSpace(req.StartDT); s != "" {
			q = q.Where("start_time >= ?", s)
		}
		if s := strings.TrimSpace(req.EndDT); s != "" {
			q = q.Where("start_time <= ?", s)
		}
		return ds.ApplyToQuery(q, "ac_gb_cloud_record.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbCloudRecord
	err := build().
		Preload("Device").
		Preload("Channel").
		Order("start_time DESC, id DESC").
		Offset(req.Offset()).
		Limit(req.PageSize).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID 按主键查询录像。
func (r *GbCloudRecordRepo) GetByID(ctx context.Context, id int64) (*model.GbCloudRecord, error) {
	var m model.GbCloudRecord
	err := bootstrap.DB().WithContext(ctx).
		Preload("Device").Preload("Channel").First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 写入一条录像索引。
func (r *GbCloudRecordRepo) Create(ctx context.Context, m *model.GbCloudRecord) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// UpdateFields 局部更新（收藏标记 / 补充时长等）。
func (r *GbCloudRecordRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	if _, ok := fields["updated_by"]; !ok {
		fields["updated_by"] = model.UsernameFromCtx(ctx)
	}
	if _, ok := fields["updated_dt"]; !ok {
		fields["updated_dt"] = time.Now()
	}
	return bootstrap.DB().WithContext(ctx).
		Model(&model.GbCloudRecord{}).Where("id = ?", id).Updates(fields).Error
}

// Delete 软删除录像索引（不删除磁盘文件）。
func (r *GbCloudRecordRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbCloudRecord{}, id).Error
}

// ── 录像计划 ──────────────────────────────────────────────────

// GbRecordPlanRepo 录像计划数据访问。表 ac_gb_record_plan（含 dept_id）。
type GbRecordPlanRepo struct{}

// NewGbRecordPlanRepo 构造录像计划仓储。
func NewGbRecordPlanRepo() *GbRecordPlanRepo { return &GbRecordPlanRepo{} }

// Page 分页查询录像计划。
func (r *GbRecordPlanRepo) Page(ctx context.Context, req *request.PageGbRecordPlanReq, ds *utils.DataScope) ([]model.GbRecordPlan, int64, error) {
	build := func() *gorm.DB {
		q := bootstrap.DB().WithContext(ctx).Model(&model.GbRecordPlan{})
		if kw := strings.TrimSpace(req.Keyword); kw != "" {
			q = q.Where("name ILIKE ?", "%"+kw+"%")
		}
		return ds.ApplyToQuery(q, "ac_gb_record_plan.dept_id")
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.GbRecordPlan
	err := build().Order("id DESC").Offset(req.Offset()).Limit(req.PageSize).Find(&items).Error
	return items, total, err
}

// ListAll 查询全部启用计划（通道绑定计划的下拉）。
func (r *GbRecordPlanRepo) ListAll(ctx context.Context, ds *utils.DataScope) ([]model.GbRecordPlan, error) {
	q := bootstrap.DB().WithContext(ctx).Model(&model.GbRecordPlan{}).
		Where("(active IS NULL OR active = true)")
	q = ds.ApplyToQuery(q, "ac_gb_record_plan.dept_id")

	var items []model.GbRecordPlan
	if err := q.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 按主键查询计划。
func (r *GbRecordPlanRepo) GetByID(ctx context.Context, id int64) (*model.GbRecordPlan, error) {
	var m model.GbRecordPlan
	if err := bootstrap.DB().WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 新增计划。
func (r *GbRecordPlanRepo) Create(ctx context.Context, m *model.GbRecordPlan) error {
	return bootstrap.DB().WithContext(ctx).Create(m).Error
}

// Update 全量保存计划。
func (r *GbRecordPlanRepo) Update(ctx context.Context, m *model.GbRecordPlan) error {
	return bootstrap.DB().WithContext(ctx).Save(m).Error
}

// Delete 软删除计划。
func (r *GbRecordPlanRepo) Delete(ctx context.Context, id int64) error {
	return bootstrap.DB().WithContext(ctx).Delete(&model.GbRecordPlan{}, id).Error
}

// CountChannels 统计绑定该计划的通道数（删除前引用校验）。
func (r *GbRecordPlanRepo) CountChannels(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbChannel{}).
		Where("record_plan_id = ?", id).Count(&n).Error
	return n, err
}

// ── 录像计划明细 ──────────────────────────────────────────────

// GbRecordPlanItemRepo 录像计划明细数据访问。表 ac_gb_record_plan_item（含 dept_id）。
type GbRecordPlanItemRepo struct{}

// NewGbRecordPlanItemRepo 构造录像计划明细仓储。
func NewGbRecordPlanItemRepo() *GbRecordPlanItemRepo { return &GbRecordPlanItemRepo{} }

// ListByPlanID 查询某计划的全部时间段（按星期与开始时间排序）。
func (r *GbRecordPlanItemRepo) ListByPlanID(ctx context.Context, planID int64) ([]model.GbRecordPlanItem, error) {
	var items []model.GbRecordPlanItem
	err := bootstrap.DB().WithContext(ctx).Model(&model.GbRecordPlanItem{}).
		Where("plan_id = ?", planID).
		Order("week_day ASC, start_time ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ReplaceForPlan 以「先删后插」方式整体替换某计划的时间段。
//
// 计划明细是整体提交的集合（无独立身份），逐条 diff 无意义；
// 在同一事务内软删除旧明细后批量插入新明细，保证一致性。
func (r *GbRecordPlanItemRepo) ReplaceForPlan(ctx context.Context, planID int64, items []model.GbRecordPlanItem) error {
	return bootstrap.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", planID).Delete(&model.GbRecordPlanItem{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

// DeleteByPlanID 软删除某计划的全部明细。
func (r *GbRecordPlanItemRepo) DeleteByPlanID(ctx context.Context, planID int64) error {
	return bootstrap.DB().WithContext(ctx).
		Where("plan_id = ?", planID).
		Delete(&model.GbRecordPlanItem{}).Error
}
