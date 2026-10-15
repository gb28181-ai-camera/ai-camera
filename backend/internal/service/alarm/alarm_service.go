package alarm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	settingsvc "github.com/boticz/ai-camera-backend/internal/service/setting"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/pkg/webhook"
	"gorm.io/gorm"
)

// 告警服务哨兵错误：Handler 据此映射 HTTP 状态码。
var (
	ErrNotFound          = errors.New("告警记录不存在")
	ErrDeviceNotFound    = errors.New("设备不存在")
	ErrAlarmTypeNotFound = errors.New("告警类型不存在")
	ErrAlreadyHandled    = errors.New("该告警已处理")
)

// AlarmService 设备告警业务逻辑。
//
// 与风险的区别：告警来自设备自身状态（离线/电量/流量），
// 没有「检测准确」确认环节，可直接处理。
type AlarmService struct {
	repo *repository.AlarmRepo
}

// NewAlarmService 构造告警服务。
func NewAlarmService() *AlarmService {
	return &AlarmService{repo: repository.NewAlarmRepo()}
}

// Page 分页查询告警记录。
func (s *AlarmService) Page(ctx context.Context, req *request.PageAlarmReq, ds *utils.DataScope) (*response.PageAlarmRecordResp, error) {
	req.Normalize()

	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}

	return &response.PageAlarmRecordResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Get 查询告警详情（含告警类型、设备、区域）。
func (s *AlarmService) Get(ctx context.Context, id int64) (*model.AlarmRecord, error) {
	rec, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rec, nil
}

// MarkRead 标记告警通知已读。已读时直接返回，保证幂等。
func (s *AlarmService) MarkRead(ctx context.Context, id int64) error {
	rec, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if rec.Read != nil && *rec.Read {
		return nil
	}

	return s.repo.UpdateFields(ctx, id, map[string]interface{}{
		"read":    true,
		"read_dt": time.Now(),
	})
}

// Handle 处理告警：写入处理结果与处理图片并办结。
// 无「检测准确」前置条件，但已处理的告警不可重复处理。
func (s *AlarmService) Handle(ctx context.Context, id int64, req *request.HandleAlarmReq, operator string) error {
	rec, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if rec.Status == model.HandleStatusHandled {
		return ErrAlreadyHandled
	}

	if operator == "" {
		operator = model.UsernameFromCtx(ctx)
	}

	now := time.Now()
	fields := map[string]interface{}{
		"handle_result": req.HandleResult,
		"handle_image":  req.HandleImage,
		"handler":       operator,
		"handled_dt":    now,
		"status":        model.HandleStatusHandled,
	}
	// 处理动作隐含已读，仅在未读时补写。
	if rec.Read == nil || !*rec.Read {
		fields["read"] = true
		fields["read_dt"] = now
	}

	return s.repo.UpdateFields(ctx, id, fields)
}

// Delete 删除告警记录（软删除）。
func (s *AlarmService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// PendingCount 未处理告警数量：status=pending 的记录总数。
func (s *AlarmService) PendingCount(ctx context.Context, ds *utils.DataScope) (int64, error) {
	return s.repo.CountPending(ctx, ds)
}

// Report 设备上报告警（ApiKey 认证，无用户上下文）。
//
// 依据 device_code 定位设备、alarm_type_code 定位告警类型，落库时冗余
// device_code / region_id / location；告警等级优先取上报值，缺省回落到告警类型配置。
// 数据权限归属取自设备所属部门。
func (s *AlarmService) Report(ctx context.Context, req *request.ReportAlarmReq) (*model.AlarmRecord, error) {
	dev, err := s.repo.FindDeviceByCode(ctx, req.DeviceCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	alarmType, err := s.repo.FindAlarmTypeByCode(ctx, req.AlarmTypeCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAlarmTypeNotFound
		}
		return nil, err
	}

	occurred, ok := parseOccurredDT(req.OccurredDT)
	if !ok {
		occurred = time.Now()
	}

	level := strings.TrimSpace(req.AlarmLevel)
	if level == "" {
		level = alarmType.Level
	}

	unread := false
	rec := &model.AlarmRecord{
		AlarmTypeID: alarmType.ID,
		DeviceID:    dev.ID,
		DeviceCode:  dev.DeviceCode,
		RegionID:    dev.RegionID,
		Location:    dev.Location,
		AlarmLevel:  level,
		Content:     req.Content,
		OccurredDT:  occurred,
		Read:        &unread,
		Status:      model.HandleStatusPending,
	}
	if dev.DeptID != nil {
		rec.DeptID = dev.DeptID
	}

	if err := s.repo.Create(ctx, rec); err != nil {
		return nil, err
	}

	// 消息推送：向订阅了 alarm 事件的 WebHook 通道分发。
	// 推送失败只记录到通道状态，不影响上报结果，故忽略返回值。
	settingsvc.NewWebhookService().Dispatch(ctx, &webhook.Message{
		Event:      model.WebhookEventAlarm,
		Title:      alarmType.Name,
		Content:    fmt.Sprintf("设备 %s 于 %s 触发告警", dev.Name, occurred.Format("2006-01-02 15:04:05")),
		OccurredDT: occurred,
		Fields: map[string]string{
			"告警类型": alarmType.Name,
			"告警等级": alarmType.Level,
			"设备名称": dev.Name,
			"设备编号": dev.DeviceCode,
			"设备位置": dev.Location,
			"发生时间": occurred.Format("2006-01-02 15:04:05"),
			"处理状态": "未处理",
		},
	})

	// 系统通知（ac_notification）由统计/定时任务扫描未读告警后统一生成。
	return rec, nil
}

// occurredDTLayouts 设备上报的时间格式。
var occurredDTLayouts = []string{
	"2006-01-02 15:04:05",
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// parseOccurredDT 解析上报的告警时间；空串或格式不识别返回 false。
func parseOccurredDT(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range occurredDTLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
