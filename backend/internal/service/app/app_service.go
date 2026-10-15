// Package app 提供移动端专用业务逻辑。
//
// 设计原则：本服务**不重复实现**已有领域业务，仅做移动端场景的编排与聚合：
//   - 风险 / 告警 / 巡检 / 设备 / 通知 / 个人中心 → 委托对应领域 Service
//   - 移动端菜单、首页仪表盘、待办汇总 → 本服务自有逻辑
package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/alarm"
	"github.com/boticz/ai-camera-backend/internal/service/device"
	"github.com/boticz/ai-camera-backend/internal/service/inspect"
	"github.com/boticz/ai-camera-backend/internal/service/risk"
	"github.com/boticz/ai-camera-backend/internal/service/system"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// AppService 移动端业务服务（无状态，可安全复用）。
type AppService struct{}

// NewAppService 构造移动端服务。
func NewAppService() *AppService { return &AppService{} }

// ────────────────────────────────────────────────────────────────
// 委托的领域服务（每次调用新建，与其它模块保持一致的构造方式）
// ────────────────────────────────────────────────────────────────

func (s *AppService) riskSvc() *risk.RiskService             { return risk.NewRiskService() }
func (s *AppService) alarmSvc() *alarm.AlarmService          { return alarm.NewAlarmService() }
func (s *AppService) inspectSvc() *inspect.InspectService    { return inspect.NewInspectService() }
func (s *AppService) deviceSvc() *device.DeviceService       { return device.NewDeviceService() }
func (s *AppService) notifySvc() *system.NotificationService { return system.NewNotificationService() }
func (s *AppService) profileSvc() *system.ProfileService     { return system.NewProfileService() }

// ════════════════════════════════════════════════════════════════
// 菜单
// ════════════════════════════════════════════════════════════════

// Menus 返回当前用户的移动端菜单树。
//
// 与 PC 端菜单相互独立：仅取 ac_menu 中 platform='mobile' 且启用、未隐藏的菜单；
// 非超级管理员再按「用户 → 角色 / 部门 → 角色 / 岗位 → 角色」三路角色
// 求 ac_role_menu_rel 的交集后收敛，最后按 parent_id 组树。
func (s *AppService) Menus(ctx context.Context, username string) ([]model.Menu, error) {
	db := bootstrap.DB()

	var user model.User
	if err := db.WithContext(ctx).Where("user_name = ?", username).First(&user).Error; err != nil {
		return nil, fmt.Errorf("用户不存在: %w", err)
	}

	isAdmin := user.IsAdmin != nil && *user.IsAdmin

	q := db.WithContext(ctx).Model(&model.Menu{}).
		Where("platform = ?", "mobile").
		Where("(active IS NULL OR active = ?)", true).
		Where("(hidden IS NULL OR hidden = ?)", false)

	if !isAdmin {
		roleIDs := s.userRoleIDs(ctx, &user)
		if len(roleIDs) == 0 {
			return []model.Menu{}, nil
		}

		var menuIDs []int64
		if err := db.WithContext(ctx).Model(&model.RoleMenuRel{}).
			Where("role_id IN ?", roleIDs).
			Distinct().Pluck("menu_id", &menuIDs).Error; err != nil {
			return nil, err
		}
		if len(menuIDs) == 0 {
			return []model.Menu{}, nil
		}
		q = q.Where("id IN ?", menuIDs)
	}

	var menus []model.Menu
	if err := q.Order("sequence ASC, id ASC").Find(&menus).Error; err != nil {
		return nil, err
	}
	return buildMenuTree(menus, nil), nil
}

// userRoleIDs 汇总用户的全部角色 ID（用户直授 + 部门 + 岗位三路），并过滤停用角色。
func (s *AppService) userRoleIDs(ctx context.Context, user *model.User) []int64 {
	db := bootstrap.DB()
	set := map[int64]struct{}{}

	collect := func(q *gorm.DB) {
		var ids []int64
		if err := q.WithContext(ctx).Pluck("role_id", &ids).Error; err != nil {
			return
		}
		for _, id := range ids {
			set[id] = struct{}{}
		}
	}

	// 1) 用户 → 角色
	collect(db.Model(&model.UserRoleRel{}).Where("user_id = ?", user.ID))

	// 2) 部门 → 角色
	if user.DeptID != nil && *user.DeptID > 0 {
		collect(db.Model(&model.DeptRoleRel{}).Where("dept_id = ?", *user.DeptID))
	}

	// 3) 岗位 → 角色（主岗位 + ac_user_post_rel 多岗位）
	postSet := map[int64]struct{}{}
	if user.PostID != nil && *user.PostID > 0 {
		postSet[*user.PostID] = struct{}{}
	}
	var postIDs []int64
	if err := db.WithContext(ctx).Model(&model.UserPostRel{}).
		Where("user_id = ?", user.ID).Pluck("post_id", &postIDs).Error; err == nil {
		for _, id := range postIDs {
			postSet[id] = struct{}{}
		}
	}
	if len(postSet) > 0 {
		all := make([]int64, 0, len(postSet))
		for id := range postSet {
			all = append(all, id)
		}
		collect(db.Model(&model.PostRoleRel{}).Where("post_id IN ?", all))
	}

	if len(set) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}

	// 停用角色不参与授权
	var activeIDs []int64
	if err := db.WithContext(ctx).Model(&model.Role{}).
		Where("id IN ?", ids).
		Where("(active IS NULL OR active = ?)", true).
		Pluck("id", &activeIDs).Error; err != nil {
		return nil
	}
	sort.Slice(activeIDs, func(i, j int) bool { return activeIDs[i] < activeIDs[j] })
	return activeIDs
}

// buildMenuTree 按 parent_id 递归组装菜单树（parentID 为 nil 表示顶层）。
func buildMenuTree(menus []model.Menu, parentID *int64) []model.Menu {
	out := make([]model.Menu, 0)
	for _, m := range menus {
		if !sameParent(m.ParentID, parentID) {
			continue
		}
		id := m.ID
		node := m
		node.Children = buildMenuTree(menus, &id)
		out = append(out, node)
	}
	return out
}

// sameParent 判断两个父 ID 是否相同（nil 视为顶层）。
func sameParent(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ════════════════════════════════════════════════════════════════
// 首页与待办
// ════════════════════════════════════════════════════════════════

// Home 返回移动端首页仪表盘数据。
func (s *AppService) Home(ctx context.Context, userID int64, ds *utils.DataScope) (*response.AppHomeResp, error) {
	db := bootstrap.DB()
	ds = ensureScope(ds)

	resp := &response.AppHomeResp{RecentRisks: []model.RiskRecord{}}

	// 待处理风险
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.RiskRecord{}).Where("status = ?", model.HandleStatusPending), "dept_id",
	).Count(&resp.PendingRiskCount).Error; err != nil {
		return nil, err
	}

	// 待处理告警
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.AlarmRecord{}).Where("status = ?", model.HandleStatusPending), "dept_id",
	).Count(&resp.PendingAlarmCount).Error; err != nil {
		return nil, err
	}

	// 我的待巡检任务（已发布 + 巡检人为当前用户）
	if err := db.WithContext(ctx).Model(&model.InspectTask{}).
		Where("status = ?", model.InspectStatusPublished).
		Where("inspector_id = ?", userID).
		Count(&resp.PendingInspectCount).Error; err != nil {
		return nil, err
	}

	// 今日新增风险 / 告警
	today := startOfToday()
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.RiskRecord{}).Where("occurred_dt >= ?", today), "dept_id",
	).Count(&resp.TodayRiskCount).Error; err != nil {
		return nil, err
	}
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.AlarmRecord{}).Where("occurred_dt >= ?", today), "dept_id",
	).Count(&resp.TodayAlarmCount).Error; err != nil {
		return nil, err
	}

	// 设备在线状态统计
	var total, online int64
	if err := ds.ApplyToQuery(db.WithContext(ctx).Model(&model.Device{}), "dept_id").Count(&total).Error; err != nil {
		return nil, err
	}
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.Device{}).Where("status = ?", model.DeviceStatusOnline), "dept_id",
	).Count(&online).Error; err != nil {
		return nil, err
	}
	resp.DeviceStat = response.AppDeviceStat{Total: total, Online: online, Offline: total - online}

	// 最新 5 条风险（含风险类型 / 设备 / 区域名称）
	var recent []model.RiskRecord
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.RiskRecord{}).
			Preload("RiskType").Preload("Device").Preload("Region").
			Order("occurred_dt DESC").Order("id DESC").Limit(5), "dept_id",
	).Find(&recent).Error; err != nil {
		return nil, err
	}
	if recent != nil {
		resp.RecentRisks = recent
	}

	// 未读通知数
	unread, err := s.notifySvc().UnreadCount(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp.UnreadCount = unread

	return resp, nil
}

// Todos 返回当前用户的移动端待办汇总。
//
// 固定四类：待处理风险、待处理告警、我的待巡检任务、未读通知。
func (s *AppService) Todos(ctx context.Context, userID int64, ds *utils.DataScope) ([]response.AppTodoItem, error) {
	db := bootstrap.DB()
	ds = ensureScope(ds)

	var riskCount int64
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.RiskRecord{}).Where("status = ?", model.HandleStatusPending), "dept_id",
	).Count(&riskCount).Error; err != nil {
		return nil, err
	}

	var alarmCount int64
	if err := ds.ApplyToQuery(
		db.WithContext(ctx).Model(&model.AlarmRecord{}).Where("status = ?", model.HandleStatusPending), "dept_id",
	).Count(&alarmCount).Error; err != nil {
		return nil, err
	}

	var inspectCount int64
	if err := db.WithContext(ctx).Model(&model.InspectTask{}).
		Where("status = ?", model.InspectStatusPublished).
		Where("inspector_id = ?", userID).
		Count(&inspectCount).Error; err != nil {
		return nil, err
	}

	unread, err := s.notifySvc().UnreadCount(ctx, userID)
	if err != nil {
		return nil, err
	}

	return []response.AppTodoItem{
		{Category: "risk", Title: "待处理风险", Count: riskCount},
		{Category: "alarm", Title: "待处理告警", Count: alarmCount},
		{Category: "inspect", Title: "我的待巡检任务", Count: inspectCount},
		{Category: "notification", Title: "未读通知", Count: unread},
	}, nil
}

// ════════════════════════════════════════════════════════════════
// 风险（委托 service/risk）
// ════════════════════════════════════════════════════════════════

// PageRisks 分页查询风险监控（移动端）。
func (s *AppService) PageRisks(ctx context.Context, req *request.PageRiskReq, ds *utils.DataScope) (*response.PageRiskRecordResp, error) {
	return s.riskSvc().Page(ctx, req, ds)
}

// GetRisk 查询风险详情。
func (s *AppService) GetRisk(ctx context.Context, id int64) (*model.RiskRecord, error) {
	return s.riskSvc().Get(ctx, id)
}

// HandleRisk 处理风险。
func (s *AppService) HandleRisk(ctx context.Context, id int64, req *request.HandleRiskReq, operator string) error {
	return s.riskSvc().Handle(ctx, id, req, operator)
}

// UpdateRiskAccurate 确认检测准确性（yes/no）。
func (s *AppService) UpdateRiskAccurate(ctx context.Context, id int64, accurate, operator string) error {
	return s.riskSvc().UpdateAccurate(ctx, id, accurate, operator)
}

// ════════════════════════════════════════════════════════════════
// 告警（委托 service/alarm）
// ════════════════════════════════════════════════════════════════

// PageAlarms 分页查询设备告警（移动端）。
func (s *AppService) PageAlarms(ctx context.Context, req *request.PageAlarmReq, ds *utils.DataScope) (*response.PageAlarmRecordResp, error) {
	return s.alarmSvc().Page(ctx, req, ds)
}

// HandleAlarm 处理设备告警。
func (s *AppService) HandleAlarm(ctx context.Context, id int64, req *request.HandleAlarmReq, operator string) error {
	return s.alarmSvc().Handle(ctx, id, req, operator)
}

// ════════════════════════════════════════════════════════════════
// 巡检（委托 service/inspect）
// ════════════════════════════════════════════════════════════════

// PageInspectTasks 分页查询「我的巡检任务」。
//
// 巡检人固定为当前登录用户，因此先把 userID 写入查询条件再委托巡检服务，
// 以保证分页计数与筛选条件一致。
func (s *AppService) PageInspectTasks(ctx context.Context, userID int64, req *request.PageInspectTaskReq) (*response.PageInspectTaskResp, error) {
	if req == nil {
		req = &request.PageInspectTaskReq{}
	}
	inspectorID := userID
	req.InspectorID = &inspectorID
	return s.inspectSvc().PageTasks(ctx, req, &utils.DataScope{UserID: userID, DataPerm: "all"})
}

// GetInspectTask 查询巡检任务详情。
func (s *AppService) GetInspectTask(ctx context.Context, id int64) (*model.InspectTask, error) {
	return s.inspectSvc().GetTask(ctx, id)
}

// FinishInspectTask 完成巡检任务并生成巡检记录。
func (s *AppService) FinishInspectTask(ctx context.Context, taskID, userID int64, req *request.FinishInspectTaskReq) (*model.InspectRecord, error) {
	return s.inspectSvc().FinishTask(ctx, taskID, userID, req)
}

// ════════════════════════════════════════════════════════════════
// 设备（委托 service/device）
// ════════════════════════════════════════════════════════════════

// PageDevices 分页查询设备（移动端）。
func (s *AppService) PageDevices(ctx context.Context, req *request.PageDeviceReq, ds *utils.DataScope) (*response.PageDeviceResp, error) {
	return s.deviceSvc().Page(ctx, req, ds)
}

// GetDevice 查询设备详情。
func (s *AppService) GetDevice(ctx context.Context, id int64) (*model.Device, error) {
	return s.deviceSvc().Get(ctx, id)
}

// ════════════════════════════════════════════════════════════════
// 通知（委托 service/system）
// ════════════════════════════════════════════════════════════════

// PageNotifications 分页查询当前用户的通知。
func (s *AppService) PageNotifications(ctx context.Context, userID int64, req *request.PageNotificationReq) (*response.PageNotificationResp, error) {
	return s.notifySvc().Page(ctx, userID, req)
}

// MarkNotificationRead 标记通知已读。
func (s *AppService) MarkNotificationRead(ctx context.Context, userID, id int64) error {
	return s.notifySvc().MarkRead(ctx, userID, id)
}

// ════════════════════════════════════════════════════════════════
// 个人中心（委托 service/system）
// ════════════════════════════════════════════════════════════════

// GetProfile 查询当前用户资料。
func (s *AppService) GetProfile(ctx context.Context, userID int64) (*model.User, error) {
	return s.profileSvc().Get(ctx, userID)
}

// UpdateProfile 更新当前用户资料。
func (s *AppService) UpdateProfile(ctx context.Context, userID int64, req *request.UpdateProfileReq) error {
	return s.profileSvc().Update(ctx, userID, req)
}

// ChangePassword 修改当前用户密码。
func (s *AppService) ChangePassword(ctx context.Context, userID int64, req *request.ChangePasswordReq) error {
	return s.profileSvc().ChangePassword(ctx, userID, req)
}

// ────────────────────────────────────────────────────────────────
// 内部工具
// ────────────────────────────────────────────────────────────────

// ensureScope 兜底数据权限，避免 nil 解引用。
func ensureScope(ds *utils.DataScope) *utils.DataScope {
	if ds == nil {
		return &utils.DataScope{DataPerm: "all"}
	}
	return ds
}

// startOfToday 返回本地时区当天 00:00:00。
func startOfToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
