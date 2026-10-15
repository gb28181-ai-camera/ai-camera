package risk

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

// 风险服务哨兵错误：Handler 据此映射 HTTP 状态码，避免把业务错误当成 500。
var (
	ErrNotFound         = errors.New("风险记录不存在")
	ErrDeviceNotFound   = errors.New("设备不存在")
	ErrRiskTypeNotFound = errors.New("风险类型不存在")
	ErrAccurateRequired = errors.New("请先确认检测准确后方可处理")
	ErrAlreadyHandled   = errors.New("该风险已处理")
	ErrInvalidAccurate  = errors.New("检测准确性取值无效")
)

// falseAlarmResult 误报办结时写入的处理结果文案（与操作手册一致）。
const falseAlarmResult = "误报，无需处理"

// RiskService AI 风险业务逻辑。
//
// 生命周期（对应操作手册「AI风险」）：
//  1. 设备上报 → status=pending、read=false、detect_accurate 待人工确认
//  2. 人工确认「否」（误报）→ 直接 status=handled 且 read=true，处理结果写「误报，无需处理」
//  3. 人工确认「是」（确认风险）→ read=true，status 保持 pending 等待处理
//  4. 人工处理 → status=handled，记录处理人/时间/结果/图片
type RiskService struct {
	repo *repository.RiskRepo
}

// NewRiskService 构造风险服务。
func NewRiskService() *RiskService {
	return &RiskService{repo: repository.NewRiskRepo()}
}

// Page 分页查询风险记录。
func (s *RiskService) Page(ctx context.Context, req *request.PageRiskReq, ds *utils.DataScope) (*response.PageRiskRecordResp, error) {
	req.Normalize()

	items, total, err := s.repo.Page(ctx, req, ds)
	if err != nil {
		return nil, err
	}

	return &response.PageRiskRecordResp{
		Items:    items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// Get 查询风险详情（含风险类型、设备、区域）。
func (s *RiskService) Get(ctx context.Context, id int64) (*model.RiskRecord, error) {
	rec, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rec, nil
}

// MarkRead 标记风险通知已读。已读时直接返回，保证幂等。
func (s *RiskService) MarkRead(ctx context.Context, id int64) error {
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

// UpdateAccurate 人工确认检测准确性。accurate 取值 yes / no。
//
//   - no（误报）：直接办结 —— status=handled、read=true、处理结果「误报，无需处理」；
//   - yes（确认风险）：read=true，status 保持 pending，等待「处理」操作。
//
// 两种情况都会记录 detect_accurate / accurate_by / accurate_dt。
func (s *RiskService) UpdateAccurate(ctx context.Context, id int64, accurate, operator string) error {
	accurate = strings.TrimSpace(accurate)
	if accurate != model.DetectAccurateYes && accurate != model.DetectAccurateNo {
		return ErrInvalidAccurate
	}

	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if operator == "" {
		operator = model.UsernameFromCtx(ctx)
	}

	now := time.Now()
	fields := map[string]interface{}{
		"detect_accurate": accurate,
		"accurate_by":     operator,
		"accurate_dt":     now,
		"read":            true,
		"read_dt":         now,
	}
	if accurate == model.DetectAccurateNo {
		fields["status"] = model.HandleStatusHandled
		fields["handle_result"] = falseAlarmResult
	}

	return s.repo.UpdateFields(ctx, id, fields)
}

// Handle 处理风险：写入处理结果与处理图片并办结。
//
// 前置条件：必须先确认「检测准确=是」（误报已在 UpdateAccurate 中直接办结），
// 且当前状态为未处理。
func (s *RiskService) Handle(ctx context.Context, id int64, req *request.HandleRiskReq, operator string) error {
	rec, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	if rec.DetectAccurate != model.DetectAccurateYes {
		return ErrAccurateRequired
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
	// 处理动作隐含已读，仅在未读时补写，避免覆盖原始已读时间。
	if rec.Read == nil || !*rec.Read {
		fields["read"] = true
		fields["read_dt"] = now
	}

	return s.repo.UpdateFields(ctx, id, fields)
}

// Delete 删除风险记录（软删除）。
func (s *RiskService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// PendingCount 已确认待处理数量：status=pending 的记录总数（含未读）。
func (s *RiskService) PendingCount(ctx context.Context, ds *utils.DataScope) (int64, error) {
	return s.repo.CountPending(ctx, ds)
}

// Report 设备上报风险（ApiKey 认证，无用户上下文）。
//
// 依据 device_code 定位设备、risk_type_code 定位风险类型，落库时冗余
// device_code / region_id / location / risk_level（取风险类型的 level）。
// 数据权限归属取自设备所属部门，否则部门级用户将看不到设备上报的记录。
func (s *RiskService) Report(ctx context.Context, req *request.ReportRiskReq) (*model.RiskRecord, error) {
	dev, err := s.repo.FindDeviceByCode(ctx, req.DeviceCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	riskType, err := s.repo.FindRiskTypeByCode(ctx, req.RiskTypeCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRiskTypeNotFound
		}
		return nil, err
	}

	occurred, ok := parseOccurredDT(req.OccurredDT)
	if !ok {
		occurred = time.Now()
	}

	unread := false
	rec := &model.RiskRecord{
		RiskTypeID:     riskType.ID,
		DeviceID:       dev.ID,
		DeviceCode:     dev.DeviceCode,
		RegionID:       dev.RegionID,
		Location:       dev.Location,
		ImageURL:       req.ImageURL,
		VideoURL:       req.VideoURL,
		RiskLevel:      riskType.Level,
		DetectScore:    req.DetectScore,
		OccurredDT:     occurred,
		DetectAccurate: "",
		Read:           &unread,
		Status:         model.HandleStatusPending,
	}
	if dev.DeptID != nil {
		rec.DeptID = dev.DeptID
	}

	if err := s.repo.Create(ctx, rec); err != nil {
		return nil, err
	}

	// 消息推送：向订阅了 risk 事件的 WebHook 通道（飞书/钉钉/企业微信/自定义）分发。
	// 推送失败只记录到通道状态，不影响上报结果，故忽略返回值。
	settingsvc.NewWebhookService().Dispatch(ctx, &webhook.Message{
		Event:      model.WebhookEventRisk,
		Title:      riskType.Name,
		Content:    fmt.Sprintf("设备 %s 于 %s 识别到风险", dev.Name, occurred.Format("2006-01-02 15:04:05")),
		OccurredDT: occurred,
		Fields: map[string]string{
			"风险类型": riskType.Name,
			"风险等级": riskType.Level,
			"设备名称": dev.Name,
			"设备编号": dev.DeviceCode,
			"设备位置": dev.Location,
			"置信度":  fmt.Sprintf("%.2f", req.DetectScore),
			"发生时间": occurred.Format("2006-01-02 15:04:05"),
			"处理状态": "未处理",
		},
	})

	// 系统通知（ac_notification）由统计/定时任务扫描未读风险后统一生成，
	// 以避免风险切片与通知切片相互依赖。
	return rec, nil
}

// occurredDTLayouts 设备上报的时间格式。
var occurredDTLayouts = []string{
	"2006-01-02 15:04:05",
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// parseOccurredDT 解析上报的发生时间；空串或格式不识别返回 false。
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
