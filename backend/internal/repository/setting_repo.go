package repository

import (
	"context"
	"errors"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// SettingRepo 告警设置（设备告警设置 / 风险通知设置 / 第三方配置）数据访问层。
//
// 设备告警与风险通知设置均以「当前登录用户」为界：
// 列表只查本人数据，更新/删除前由 Service 校验归属。
type SettingRepo struct{}

// NewSettingRepo 构造告警设置仓储。
func NewSettingRepo() *SettingRepo { return &SettingRepo{} }

func (r *SettingRepo) db(ctx context.Context) *gorm.DB {
	return bootstrap.DB().WithContext(ctx)
}

// ────────────────────────────────────────────────────────────────
// 设备告警设置
// ────────────────────────────────────────────────────────────────

// PageAlarmSettings 当前用户的设备告警设置分页查询（含告警类型名称）。
func (r *SettingRepo) PageAlarmSettings(ctx context.Context, userID int64, req *request.PageAlarmSettingReq) ([]model.AlarmSetting, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.AlarmSetting{}).Where("ac_alarm_setting.user_id = ?", userID)
		if req.AlarmTypeID != nil && *req.AlarmTypeID > 0 {
			q = q.Where("ac_alarm_setting.alarm_type_id = ?", *req.AlarmTypeID)
		}
		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			sub := r.db(ctx).Model(&model.AlarmType{}).Select("id").Where("name ILIKE ?", like)
			q = q.Where("ac_alarm_setting.alarm_type_id IN (?)", sub)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.AlarmSetting, 0)
	if err := build().
		Preload("AlarmType").
		Order("ac_alarm_setting.id ASC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetAlarmSetting 按主键读取设备告警设置（含告警类型）。
func (r *SettingRepo) GetAlarmSetting(ctx context.Context, id int64) (*model.AlarmSetting, error) {
	var m model.AlarmSetting
	if err := r.db(ctx).Preload("AlarmType").Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateAlarmSetting 新增设备告警设置。
func (r *SettingRepo) CreateAlarmSetting(ctx context.Context, m *model.AlarmSetting) error {
	return r.db(ctx).Create(m).Error
}

// UpdateAlarmSetting 全量更新设备告警设置可编辑字段。
func (r *SettingRepo) UpdateAlarmSetting(ctx context.Context, m *model.AlarmSetting) error {
	return r.db(ctx).
		Model(m).
		Select("alarm_type_id", "active", "notify_type", "notify_freq", "notify_user_ids", "threshold", "remark").
		Updates(m).Error
}

// DeleteAlarmSetting 软删除设备告警设置。
func (r *SettingRepo) DeleteAlarmSetting(ctx context.Context, id int64) error {
	return r.db(ctx).Delete(&model.AlarmSetting{}, id).Error
}

// DisableAllAlarm 关闭当前用户所有设备告警通知（active = false）。
func (r *SettingRepo) DisableAllAlarm(ctx context.Context, userID int64) error {
	active := false
	return r.db(ctx).
		Model(&model.AlarmSetting{}).
		Where("user_id = ?", userID).
		Select("active").
		Updates(&model.AlarmSetting{Active: &active}).Error
}

// ────────────────────────────────────────────────────────────────
// 风险通知设置
// ────────────────────────────────────────────────────────────────

// PageRiskNotifySettings 当前用户的风险通知设置分页查询（含风险类型名称）。
func (r *SettingRepo) PageRiskNotifySettings(ctx context.Context, userID int64, req *request.PageRiskNotifySettingReq) ([]model.RiskNotifySetting, int64, error) {
	build := func() *gorm.DB {
		q := r.db(ctx).Model(&model.RiskNotifySetting{}).Where("ac_risk_notify_setting.user_id = ?", userID)
		if req.Keyword != "" {
			like := "%" + req.Keyword + "%"
			sub := r.db(ctx).Model(&model.RiskType{}).Select("id").Where("name ILIKE ?", like)
			q = q.Where("ac_risk_notify_setting.risk_type_id IN (?)", sub)
		}
		return q
	}

	var total int64
	if err := build().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]model.RiskNotifySetting, 0)
	if err := build().
		Preload("RiskType").
		Order("ac_risk_notify_setting.id ASC").
		Offset(req.Offset()).Limit(req.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetRiskNotifySetting 按主键读取风险通知设置（含风险类型）。
func (r *SettingRepo) GetRiskNotifySetting(ctx context.Context, id int64) (*model.RiskNotifySetting, error) {
	var m model.RiskNotifySetting
	if err := r.db(ctx).Preload("RiskType").Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateRiskNotifySetting 新增风险通知设置。
func (r *SettingRepo) CreateRiskNotifySetting(ctx context.Context, m *model.RiskNotifySetting) error {
	return r.db(ctx).Create(m).Error
}

// UpdateRiskNotifySetting 全量更新风险通知设置可编辑字段。
func (r *SettingRepo) UpdateRiskNotifySetting(ctx context.Context, m *model.RiskNotifySetting) error {
	return r.db(ctx).
		Model(m).
		Select("risk_type_id", "active", "notify_type", "notify_freq", "notify_user_ids", "remark").
		Updates(m).Error
}

// DeleteRiskNotifySetting 软删除风险通知设置。
func (r *SettingRepo) DeleteRiskNotifySetting(ctx context.Context, id int64) error {
	return r.db(ctx).Delete(&model.RiskNotifySetting{}, id).Error
}

// DisableAllRiskNotify 关闭当前用户所有风险通知（active = false）。
func (r *SettingRepo) DisableAllRiskNotify(ctx context.Context, userID int64) error {
	active := false
	return r.db(ctx).
		Model(&model.RiskNotifySetting{}).
		Where("user_id = ?", userID).
		Select("active").
		Updates(&model.RiskNotifySetting{Active: &active}).Error
}

// ────────────────────────────────────────────────────────────────
// 第三方配置（个推 / 短信）
// ────────────────────────────────────────────────────────────────

// ListThirdPartyConfigs 按分类（getui / sms）读取键值配置。
func (r *SettingRepo) ListThirdPartyConfigs(ctx context.Context, category string) ([]model.ThirdPartyConfig, error) {
	items := make([]model.ThirdPartyConfig, 0)
	if err := r.db(ctx).
		Where("category = ?", category).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// UpsertThirdPartyConfig 按「分类 + 键」写入配置，不存在则新增。
func (r *SettingRepo) UpsertThirdPartyConfig(ctx context.Context, category, code, name, value string, secret bool) error {
	var existing model.ThirdPartyConfig
	err := r.db(ctx).Where("category = ? AND code = ?", category, code).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db(ctx).Create(&model.ThirdPartyConfig{
				Category: category,
				Code:     code,
				Name:     name,
				Value:    value,
				Secret:   &secret,
			}).Error
		}
		return err
	}

	return r.db(ctx).
		Model(&model.ThirdPartyConfig{}).
		Where("id = ?", existing.ID).
		Select("name", "value", "secret").
		Updates(&model.ThirdPartyConfig{Name: name, Value: value, Secret: &secret}).Error
}
