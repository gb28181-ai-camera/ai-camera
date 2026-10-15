package setting

import (
	"context"
	"errors"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
)

// 第三方配置分类（ac_thirdparty_config.category）。
const (
	CategoryGetui = "getui"
	CategorySms   = "sms"
)

// sensitiveCodes 读取时一律脱敏的配置键：
// 无论配置行是否标记 secret，密钥类字段都不允许回显明文。
var sensitiveCodes = map[string]bool{
	model.GetuiCfgAppSecret:    true,
	model.GetuiCfgMasterSecret: true,
	model.SmsCfgSecretKey:      true,
}

// SettingService 告警设置业务逻辑。
//
// 设备告警设置与风险通知设置一律以当前登录用户为界：
// 列表只返回本人配置，新增强制写入本人，更新/删除先校验归属。
type SettingService struct {
	repo *repository.SettingRepo
}

// NewSettingService 构造告警设置服务。
func NewSettingService() *SettingService {
	return &SettingService{repo: repository.NewSettingRepo()}
}

// ────────────────────────────────────────────────────────────────
// 设备告警设置
// ────────────────────────────────────────────────────────────────

// PageAlarmSettings 当前用户的设备告警设置分页查询。
func (s *SettingService) PageAlarmSettings(ctx context.Context, userID int64, req *request.PageAlarmSettingReq) (*response.PageAlarmSettingResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageAlarmSettings(ctx, userID, req)
	if err != nil {
		return nil, err
	}
	return &response.PageAlarmSettingResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// CreateAlarmSetting 新增设备告警设置，强制归属当前用户。
func (s *SettingService) CreateAlarmSetting(ctx context.Context, userID int64, req *request.CreateAlarmSettingReq) (*model.AlarmSetting, error) {
	m := &model.AlarmSetting{
		UserID:        userID,
		AlarmTypeID:   req.AlarmTypeID,
		Active:        req.Active,
		NotifyType:    req.NotifyType,
		NotifyFreq:    req.NotifyFreq,
		NotifyUserIDs: req.NotifyUserIDs,
		Threshold:     req.Threshold,
		Remark:        req.Remark,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	if m.NotifyType == "" {
		m.NotifyType = model.NotifyTypeApp
	}
	if err := s.repo.CreateAlarmSetting(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateAlarmSetting 更新设备告警设置。仅本人配置可修改。
func (s *SettingService) UpdateAlarmSetting(ctx context.Context, userID, id int64, req *request.UpdateAlarmSettingReq) error {
	existing, err := s.repo.GetAlarmSetting(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return errors.New("无权操作他人配置")
	}

	m := &model.AlarmSetting{
		BaseModel:     model.BaseModel{ID: id},
		UserID:        userID,
		AlarmTypeID:   req.AlarmTypeID,
		Active:        req.Active,
		NotifyType:    req.NotifyType,
		NotifyFreq:    req.NotifyFreq,
		NotifyUserIDs: req.NotifyUserIDs,
		Threshold:     req.Threshold,
		Remark:        req.Remark,
	}
	// 未传值表示不修改，沿用原值，避免全量更新把开关或通知方式置空。
	if m.Active == nil {
		m.Active = existing.Active
	}
	if m.NotifyType == "" {
		m.NotifyType = existing.NotifyType
	}
	return s.repo.UpdateAlarmSetting(ctx, m)
}

// DeleteAlarmSetting 删除设备告警设置。仅本人配置可删除。
func (s *SettingService) DeleteAlarmSetting(ctx context.Context, userID, id int64) error {
	existing, err := s.repo.GetAlarmSetting(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return errors.New("无权操作他人配置")
	}
	return s.repo.DeleteAlarmSetting(ctx, id)
}

// DisableAllAlarm 关闭当前用户的所有设备告警通知。
func (s *SettingService) DisableAllAlarm(ctx context.Context, userID int64) error {
	return s.repo.DisableAllAlarm(ctx, userID)
}

// ────────────────────────────────────────────────────────────────
// 风险通知设置
// ────────────────────────────────────────────────────────────────

// PageRiskNotifySettings 当前用户的风险通知设置分页查询。
func (s *SettingService) PageRiskNotifySettings(ctx context.Context, userID int64, req *request.PageRiskNotifySettingReq) (*response.PageRiskNotifySettingResp, error) {
	req.Normalize()
	items, total, err := s.repo.PageRiskNotifySettings(ctx, userID, req)
	if err != nil {
		return nil, err
	}
	return &response.PageRiskNotifySettingResp{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    items,
	}, nil
}

// CreateRiskNotifySetting 新增风险通知设置，强制归属当前用户。
func (s *SettingService) CreateRiskNotifySetting(ctx context.Context, userID int64, req *request.CreateRiskNotifySettingReq) (*model.RiskNotifySetting, error) {
	m := &model.RiskNotifySetting{
		UserID:        userID,
		RiskTypeID:    req.RiskTypeID,
		Active:        req.Active,
		NotifyType:    req.NotifyType,
		NotifyFreq:    req.NotifyFreq,
		NotifyUserIDs: req.NotifyUserIDs,
		Remark:        req.Remark,
	}
	if m.Active == nil {
		m.Active = boolPtr(true)
	}
	if m.NotifyType == "" {
		m.NotifyType = model.NotifyTypeApp
	}
	if err := s.repo.CreateRiskNotifySetting(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateRiskNotifySetting 更新风险通知设置。仅本人配置可修改。
func (s *SettingService) UpdateRiskNotifySetting(ctx context.Context, userID, id int64, req *request.UpdateRiskNotifySettingReq) error {
	existing, err := s.repo.GetRiskNotifySetting(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return errors.New("无权操作他人配置")
	}

	m := &model.RiskNotifySetting{
		BaseModel:     model.BaseModel{ID: id},
		UserID:        userID,
		RiskTypeID:    req.RiskTypeID,
		Active:        req.Active,
		NotifyType:    req.NotifyType,
		NotifyFreq:    req.NotifyFreq,
		NotifyUserIDs: req.NotifyUserIDs,
		Remark:        req.Remark,
	}
	if m.Active == nil {
		m.Active = existing.Active
	}
	if m.NotifyType == "" {
		m.NotifyType = existing.NotifyType
	}
	return s.repo.UpdateRiskNotifySetting(ctx, m)
}

// DeleteRiskNotifySetting 删除风险通知设置。仅本人配置可删除。
func (s *SettingService) DeleteRiskNotifySetting(ctx context.Context, userID, id int64) error {
	existing, err := s.repo.GetRiskNotifySetting(ctx, id)
	if err != nil {
		return err
	}
	if existing.UserID != userID {
		return errors.New("无权操作他人配置")
	}
	return s.repo.DeleteRiskNotifySetting(ctx, id)
}

// DisableAllRiskNotify 关闭当前用户的所有风险通知。
func (s *SettingService) DisableAllRiskNotify(ctx context.Context, userID int64) error {
	return s.repo.DisableAllRiskNotify(ctx, userID)
}

// ────────────────────────────────────────────────────────────────
// 第三方配置（个推 / 短信）
// ────────────────────────────────────────────────────────────────

// GetGetui 读取个推配置，敏感项脱敏返回。
func (s *SettingService) GetGetui(ctx context.Context) (*response.GetuiConfigResp, error) {
	values, secrets, err := s.loadThirdParty(ctx, CategoryGetui)
	if err != nil {
		return nil, err
	}
	return &response.GetuiConfigResp{
		AppID:        values[model.GetuiCfgAppID],
		AppKey:       values[model.GetuiCfgAppKey],
		AppSecret:    maskIfSecret(model.GetuiCfgAppSecret, values[model.GetuiCfgAppSecret], secrets[model.GetuiCfgAppSecret]),
		MasterSecret: maskIfSecret(model.GetuiCfgMasterSecret, values[model.GetuiCfgMasterSecret], secrets[model.GetuiCfgMasterSecret]),
	}, nil
}

// UpdateGetui 更新个推配置。空值与脱敏串跳过，不覆盖原密钥。
func (s *SettingService) UpdateGetui(ctx context.Context, req *request.UpdateGetuiReq) error {
	items := []thirdPartyItem{
		{code: model.GetuiCfgAppID, name: "个推 AppID", value: req.AppID},
		{code: model.GetuiCfgAppKey, name: "个推 AppKey", value: req.AppKey},
		{code: model.GetuiCfgAppSecret, name: "个推 AppSecret", value: req.AppSecret},
		{code: model.GetuiCfgMasterSecret, name: "个推 MasterSecret", value: req.MasterSecret},
	}
	return s.saveThirdParty(ctx, CategoryGetui, items)
}

// GetSms 读取短信配置，敏感项脱敏返回。
func (s *SettingService) GetSms(ctx context.Context) (*response.SmsConfigResp, error) {
	values, secrets, err := s.loadThirdParty(ctx, CategorySms)
	if err != nil {
		return nil, err
	}
	return &response.SmsConfigResp{
		SecretID:  values[model.SmsCfgSecretID],
		SecretKey: maskIfSecret(model.SmsCfgSecretKey, values[model.SmsCfgSecretKey], secrets[model.SmsCfgSecretKey]),
		SdkAppID:  values[model.SmsCfgSdkAppID],
		SignName:  values[model.SmsCfgSignName],
	}, nil
}

// UpdateSms 更新短信配置。空值与脱敏串跳过，不覆盖原密钥。
func (s *SettingService) UpdateSms(ctx context.Context, req *request.UpdateSmsReq) error {
	items := []thirdPartyItem{
		{code: model.SmsCfgSecretID, name: "短信 SecretID", value: req.SecretID},
		{code: model.SmsCfgSecretKey, name: "短信 SecretKey", value: req.SecretKey},
		{code: model.SmsCfgSdkAppID, name: "短信 SdkAppID", value: req.SdkAppID},
		{code: model.SmsCfgSignName, name: "短信签名", value: req.SignName},
	}
	return s.saveThirdParty(ctx, CategorySms, items)
}

// thirdPartyItem 一条待写入的第三方配置。
type thirdPartyItem struct {
	code  string
	name  string
	value string
}

// loadThirdParty 读取分类下的配置，返回 键->值 与 键->是否敏感 两个映射。
func (s *SettingService) loadThirdParty(ctx context.Context, category string) (map[string]string, map[string]bool, error) {
	rows, err := s.repo.ListThirdPartyConfigs(ctx, category)
	if err != nil {
		return nil, nil, err
	}
	values := make(map[string]string, len(rows))
	secrets := make(map[string]bool, len(rows))
	for _, row := range rows {
		values[row.Code] = row.Value
		secrets[row.Code] = row.Secret != nil && *row.Secret
	}
	return values, secrets, nil
}

// saveThirdParty 逐键写入配置：空值与脱敏串（含 ****）跳过，避免用展示值覆盖真实密钥。
func (s *SettingService) saveThirdParty(ctx context.Context, category string, items []thirdPartyItem) error {
	for _, it := range items {
		value := strings.TrimSpace(it.value)
		if value == "" || strings.Contains(value, "****") {
			continue
		}
		if err := s.repo.UpsertThirdPartyConfig(ctx, category, it.code, it.name, value, sensitiveCodes[it.code]); err != nil {
			return err
		}
	}
	return nil
}

// maskIfSecret 敏感值脱敏：仅保留前 4 位。
func maskIfSecret(code, value string, rowSecret bool) string {
	if value == "" {
		return ""
	}
	if rowSecret || sensitiveCodes[code] {
		return maskSecret(value)
	}
	return value
}

// maskSecret 脱敏算法：前 4 位 + ****。
func maskSecret(v string) string {
	if v == "" {
		return ""
	}
	if len(v) >= 4 {
		return v[:4] + "****"
	}
	return "****"
}

// boolPtr 返回布尔值的指针，用于 *bool 字段显式赋值。
func boolPtr(b bool) *bool { return &b }
