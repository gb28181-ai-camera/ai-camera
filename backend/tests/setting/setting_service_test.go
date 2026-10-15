package setting

import (
	"context"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	svcsetting "github.com/boticz/ai-camera-backend/internal/service/setting"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 告警设置（SettingService）：一律以当前登录用户为界 + 敏感项脱敏
// ════════════════════════════════════════════════════════════════

func settingSvc() *svcsetting.SettingService { return svcsetting.NewSettingService() }

// ── 设备告警设置 ──────────────────────────────────────────────

// TestSettingService_CreateAlarmSetting 创建告警设置：归属强制取当前用户。
func TestSettingService_CreateAlarmSetting(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userID := int64(940001)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		got, err := svc.CreateAlarmSetting(ctx, userID, &request.CreateAlarmSettingReq{
			AlarmTypeID: alarmTypeID, NotifyType: model.NotifyTypeApp, NotifyFreq: model.NotifyFreqHourly,
			NotifyUserIDs: "2,3", Threshold: "20", Remark: "电量低于20%通知",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		var m model.AlarmSetting
		if err := tx.Where("id = ?", got.ID).First(&m).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.UserID != userID {
			t.Errorf("user_id = %d, 期望入参 %d（来源为上下文而非请求体）", m.UserID, userID)
		}
		if m.AlarmTypeID != alarmTypeID || m.NotifyType != model.NotifyTypeApp ||
			m.NotifyFreq != model.NotifyFreqHourly || m.NotifyUserIDs != "2,3" || m.Threshold != "20" ||
			m.Remark != "电量低于20%通知" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.Active == nil || !*m.Active {
			t.Errorf("active = %v, 期望 true", m.Active)
		}
		if m.CreatedBy != testUserName || m.CreatedDT.IsZero() {
			t.Errorf("审计字段缺失: created_by=%q created_dt=%v", m.CreatedBy, m.CreatedDT)
		}

		// 缺省 notify_type 为 app
		def, err := svc.CreateAlarmSetting(ctx, userID, &request.CreateAlarmSettingReq{AlarmTypeID: alarmTypeID})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		var d model.AlarmSetting
		_ = tx.Where("id = ?", def.ID).First(&d).Error
		if d.NotifyType != model.NotifyTypeApp {
			t.Errorf("默认 notify_type = %q, 期望 app", d.NotifyType)
		}
		if d.Active == nil || !*d.Active {
			t.Errorf("默认 active = %v, 期望 true", d.Active)
		}
	})
}

// TestSettingService_CreateAlarmSetting_Enums 通知类型与频率枚举组合落库。
func TestSettingService_CreateAlarmSetting_Enums(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userID := int64(940002)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		cases := []struct {
			notifyType string
			freq       int
		}{
			{model.NotifyTypeApp, model.NotifyFreqRealtime},
			{model.NotifyTypeSms, model.NotifyFreqHourly},
			{model.NotifyTypeInner, model.NotifyFreqDaily},
		}
		for _, c := range cases {
			got, err := svc.CreateAlarmSetting(ctx, userID, &request.CreateAlarmSettingReq{
				AlarmTypeID: alarmTypeID, NotifyType: c.notifyType, NotifyFreq: c.freq,
			})
			if err != nil {
				t.Fatalf("创建 notify_type=%s 失败: %v", c.notifyType, err)
			}
			var m model.AlarmSetting
			_ = tx.Where("id = ?", got.ID).First(&m).Error
			if m.NotifyType != c.notifyType || m.NotifyFreq != c.freq {
				t.Errorf("枚举未原样保存: %q/%d", m.NotifyType, m.NotifyFreq)
			}
		}
	})
}

// TestSettingService_CreateAlarmSetting_NotifyUsers 通知人员列表原样保存，允许为空。
func TestSettingService_CreateAlarmSetting_NotifyUsers(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userID := int64(940003)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		for _, ids := range []string{"2,3,5", "", "2, 3,abc"} {
			got, err := svc.CreateAlarmSetting(ctx, userID, &request.CreateAlarmSettingReq{
				AlarmTypeID: alarmTypeID, NotifyUserIDs: ids,
			})
			if err != nil {
				t.Fatalf("创建 notify_user_ids=%q 失败: %v", ids, err)
			}
			var m model.AlarmSetting
			_ = tx.Where("id = ?", got.ID).First(&m).Error
			if m.NotifyUserIDs != ids {
				t.Errorf("notify_user_ids 未原样保存: %q -> %q", ids, m.NotifyUserIDs)
			}
		}
	})
}

// TestSettingService_PageAlarmSettings_Isolation 只能查看本人的告警设置。
func TestSettingService_PageAlarmSettings_Isolation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA, userB := int64(940010), int64(940011)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		for i := 0; i < 2; i++ {
			if _, err := svc.CreateAlarmSetting(ctx, userA, &request.CreateAlarmSettingReq{AlarmTypeID: alarmTypeID}); err != nil {
				t.Fatalf("创建失败: %v", err)
			}
		}
		for i := 0; i < 3; i++ {
			if _, err := svc.CreateAlarmSetting(ctx, userB, &request.CreateAlarmSettingReq{AlarmTypeID: alarmTypeID}); err != nil {
				t.Fatalf("创建失败: %v", err)
			}
		}

		pageA, err := svc.PageAlarmSettings(ctx, userA, &request.PageAlarmSettingReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		pageB, _ := svc.PageAlarmSettings(ctx, userB, &request.PageAlarmSettingReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if pageA.Total != 2 {
			t.Errorf("A total = %d, 期望 2", pageA.Total)
		}
		if pageB.Total != 3 {
			t.Errorf("B total = %d, 期望 3", pageB.Total)
		}
		for _, it := range pageA.Items {
			if it.UserID != userA {
				t.Errorf("A 的列表出现他人配置: %+v", it)
			}
			if it.AlarmType == nil || it.AlarmType.Name == "" {
				t.Errorf("AlarmType.Name 未回显: %+v", it.AlarmType)
			}
		}

		// 按告警类型过滤
		filtered, _ := svc.PageAlarmSettings(ctx, userA, &request.PageAlarmSettingReq{
			PageReq: request.PageReq{Page: 1, PageSize: 20}, AlarmTypeID: &alarmTypeID,
		})
		if filtered.Total != 2 {
			t.Errorf("按告警类型过滤 total = %d, 期望 2", filtered.Total)
		}
	})
}

// TestSettingService_UpdateAlarmSetting 更新本人配置：字段全部生效。
func TestSettingService_UpdateAlarmSetting(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA := int64(940020)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		created, err := svc.CreateAlarmSetting(ctx, userA, &request.CreateAlarmSettingReq{
			AlarmTypeID: alarmTypeID, NotifyType: model.NotifyTypeApp, NotifyFreq: model.NotifyFreqRealtime,
			NotifyUserIDs: "1", Threshold: "10", Remark: "旧备注",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		if err := svc.UpdateAlarmSetting(ctx, userA, created.ID, &request.UpdateAlarmSettingReq{
			AlarmTypeID: alarmTypeID, NotifyType: model.NotifyTypeSms, NotifyFreq: model.NotifyFreqDaily,
			NotifyUserIDs: "4", Threshold: "15", Remark: "改为短信",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		var m model.AlarmSetting
		_ = tx.Where("id = ?", created.ID).First(&m).Error
		if m.NotifyType != model.NotifyTypeSms || m.NotifyFreq != model.NotifyFreqDaily ||
			m.NotifyUserIDs != "4" || m.Threshold != "15" || m.Remark != "改为短信" {
			t.Errorf("字段未全部更新: %+v", m)
		}
		if m.UserID != userA {
			t.Errorf("user_id 不应变化: %d", m.UserID)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}

		// 未传 active/notify_type 时沿用原值
		if err := svc.UpdateAlarmSetting(ctx, userA, created.ID, &request.UpdateAlarmSettingReq{
			AlarmTypeID: alarmTypeID, NotifyFreq: model.NotifyFreqHourly,
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}
		var m2 model.AlarmSetting
		_ = tx.Where("id = ?", created.ID).First(&m2).Error
		if m2.NotifyType != model.NotifyTypeSms {
			t.Errorf("未传 notify_type 时应沿用原值, got %q", m2.NotifyType)
		}
		if m2.Active == nil || !*m2.Active {
			t.Errorf("未传 active 时应沿用原值, got %v", m2.Active)
		}
		if m2.NotifyFreq != model.NotifyFreqHourly {
			t.Errorf("notify_freq = %d, 期望 60", m2.NotifyFreq)
		}
	})
}

// TestSettingService_UpdateAlarmSetting_OtherUser 不能修改他人配置。
func TestSettingService_UpdateAlarmSetting_OtherUser(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA, userB := int64(940030), int64(940031)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		created, err := svc.CreateAlarmSetting(ctx, userA, &request.CreateAlarmSettingReq{
			AlarmTypeID: alarmTypeID, NotifyType: model.NotifyTypeApp, NotifyFreq: 30,
			NotifyUserIDs: "1", Threshold: "10", Remark: "A的配置",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		before := *created

		err = svc.UpdateAlarmSetting(ctx, userB, created.ID, &request.UpdateAlarmSettingReq{
			AlarmTypeID: alarmTypeID, NotifyType: model.NotifyTypeSms, NotifyFreq: 1440,
			NotifyUserIDs: "9", Threshold: "99", Remark: "篡改",
		})
		if err == nil {
			t.Fatal("越权修改他人配置应返回错误")
		}
		if !strings.Contains(err.Error(), "无权") {
			t.Errorf("错误信息 = %q, 期望提示无权操作", err.Error())
		}

		var m model.AlarmSetting
		_ = tx.Where("id = ?", created.ID).First(&m).Error
		if m.NotifyType != before.NotifyType || m.NotifyFreq != before.NotifyFreq ||
			m.NotifyUserIDs != before.NotifyUserIDs || m.Threshold != before.Threshold || m.Remark != before.Remark {
			t.Errorf("越权修改时字段不应被改动: %+v", m)
		}
		if !m.UpdatedDT.Equal(before.UpdatedDT) {
			t.Errorf("越权修改不应刷新 updated_dt")
		}
	})
}

// TestSettingService_DeleteAlarmSetting 删除本人配置（软删除）。
func TestSettingService_DeleteAlarmSetting(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA := int64(940040)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		created, err := svc.CreateAlarmSetting(ctx, userA, &request.CreateAlarmSettingReq{AlarmTypeID: alarmTypeID})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		if err := svc.DeleteAlarmSetting(ctx, userA, created.ID); err != nil {
			t.Fatalf("删除失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_alarm_setting", created.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		page, _ := svc.PageAlarmSettings(ctx, userA, &request.PageAlarmSettingReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if page.Total != 0 {
			t.Errorf("删除后 total = %d, 期望 0", page.Total)
		}
	})
}

// TestSettingService_DeleteAlarmSetting_OtherUser 不能删除他人配置。
func TestSettingService_DeleteAlarmSetting_OtherUser(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA, userB := int64(940050), int64(940051)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		created, err := svc.CreateAlarmSetting(ctx, userA, &request.CreateAlarmSettingReq{AlarmTypeID: alarmTypeID})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		if err := svc.DeleteAlarmSetting(ctx, userB, created.ID); err == nil {
			t.Fatal("越权删除他人配置应返回错误")
		}
		if dt := softDeletedAt(t, tx, "ac_alarm_setting", created.ID); dt != nil {
			t.Error("越权删除时 deleted_at 应仍为空")
		}
		page, _ := svc.PageAlarmSettings(ctx, userA, &request.PageAlarmSettingReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if page.Total != 1 {
			t.Errorf("A 的配置数不应变化: %d", page.Total)
		}
	})
}

// TestSettingService_DisableAllAlarm 关闭所有告警只影响本人。
func TestSettingService_DisableAllAlarm(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA, userB := int64(940060), int64(940061)
		alarmTypeID := pickID(t, tx, "ac_alarm_type", "")
		var aIDs []int64
		for i := 0; i < 3; i++ {
			rec, err := svc.CreateAlarmSetting(ctx, userA, &request.CreateAlarmSettingReq{AlarmTypeID: alarmTypeID})
			if err != nil {
				t.Fatalf("创建失败: %v", err)
			}
			aIDs = append(aIDs, rec.ID)
		}
		var bIDs []int64
		for i := 0; i < 2; i++ {
			rec, err := svc.CreateAlarmSetting(ctx, userB, &request.CreateAlarmSettingReq{AlarmTypeID: alarmTypeID})
			if err != nil {
				t.Fatalf("创建失败: %v", err)
			}
			bIDs = append(bIDs, rec.ID)
		}

		if err := svc.DisableAllAlarm(ctx, userA); err != nil {
			t.Fatalf("关闭失败: %v", err)
		}
		for _, id := range aIDs {
			if activeFlag(t, tx, "ac_alarm_setting", id) {
				t.Errorf("A 的配置 %d 应已停用", id)
			}
		}
		for _, id := range bIDs {
			if !activeFlag(t, tx, "ac_alarm_setting", id) {
				t.Errorf("B 的配置 %d 不应受影响", id)
			}
		}
	})
}

// ── 风险通知设置 ──────────────────────────────────────────────

// TestSettingService_CreateRiskNotifySetting 创建风险通知设置：归属强制取当前用户。
func TestSettingService_CreateRiskNotifySetting(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userID := int64(940071)
		riskTypeID := pickID(t, tx, "ac_risk_type", "")
		got, err := svc.CreateRiskNotifySetting(ctx, userID, &request.CreateRiskNotifySettingReq{
			RiskTypeID: riskTypeID, NotifyType: model.NotifyTypeInner, NotifyFreq: model.NotifyFreqHourly,
			NotifyUserIDs: "2", Remark: "高风险必通知",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		var m model.RiskNotifySetting
		if err := tx.Where("id = ?", got.ID).First(&m).Error; err != nil {
			t.Fatalf("回读失败: %v", err)
		}
		if m.UserID != userID || m.RiskTypeID != riskTypeID || m.NotifyType != model.NotifyTypeInner ||
			m.NotifyFreq != model.NotifyFreqHourly || m.NotifyUserIDs != "2" || m.Remark != "高风险必通知" {
			t.Errorf("字段不一致: %+v", m)
		}
		if m.Active == nil || !*m.Active || m.CreatedBy != testUserName || m.CreatedDT.IsZero() {
			t.Errorf("默认值/审计字段异常: %+v", m)
		}
	})
}

// TestSettingService_PageRiskNotifySettings_Isolation 风险通知设置的本人隔离与越权拦截。
func TestSettingService_PageRiskNotifySettings_Isolation(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA, userB := int64(940080), int64(940081)
		riskTypeID := pickID(t, tx, "ac_risk_type", "")
		for i := 0; i < 2; i++ {
			if _, err := svc.CreateRiskNotifySetting(ctx, userA, &request.CreateRiskNotifySettingReq{RiskTypeID: riskTypeID}); err != nil {
				t.Fatalf("创建失败: %v", err)
			}
		}
		for i := 0; i < 2; i++ {
			if _, err := svc.CreateRiskNotifySetting(ctx, userB, &request.CreateRiskNotifySettingReq{RiskTypeID: riskTypeID}); err != nil {
				t.Fatalf("创建失败: %v", err)
			}
		}

		pageA, err := svc.PageRiskNotifySettings(ctx, userA, &request.PageRiskNotifySettingReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if pageA.Total != 2 {
			t.Errorf("A total = %d, 期望 2", pageA.Total)
		}
		for _, it := range pageA.Items {
			if it.UserID != userA {
				t.Errorf("A 的列表出现他人配置: %+v", it)
			}
			if it.RiskType == nil || it.RiskType.Name == "" {
				t.Errorf("RiskType.Name 未回显: %+v", it.RiskType)
			}
		}

		target := pageA.Items[0].ID
		if err := svc.UpdateRiskNotifySetting(ctx, userB, target, &request.UpdateRiskNotifySettingReq{
			RiskTypeID: riskTypeID, NotifyType: model.NotifyTypeSms, Remark: "篡改",
		}); err == nil {
			t.Error("越权修改他人风险通知设置应返回错误")
		}
		if err := svc.DeleteRiskNotifySetting(ctx, userB, target); err == nil {
			t.Error("越权删除他人风险通知设置应返回错误")
		}
		if dt := softDeletedAt(t, tx, "ac_risk_notify_setting", target); dt != nil {
			t.Error("越权删除时 deleted_at 应仍为空")
		}
	})
}

// TestSettingService_UpdateRiskNotifySetting 更新本人风险通知设置。
func TestSettingService_UpdateRiskNotifySetting(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA := int64(940090)
		riskTypeID := pickID(t, tx, "ac_risk_type", "")
		created, err := svc.CreateRiskNotifySetting(ctx, userA, &request.CreateRiskNotifySettingReq{
			RiskTypeID: riskTypeID, NotifyType: model.NotifyTypeApp, NotifyFreq: 0, NotifyUserIDs: "1", Remark: "旧",
		})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		if err := svc.UpdateRiskNotifySetting(ctx, userA, created.ID, &request.UpdateRiskNotifySettingReq{
			RiskTypeID: riskTypeID, NotifyType: model.NotifyTypeSms, NotifyFreq: model.NotifyFreqDaily,
			NotifyUserIDs: "5", Remark: "新",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}
		var m model.RiskNotifySetting
		_ = tx.Where("id = ?", created.ID).First(&m).Error
		if m.NotifyType != model.NotifyTypeSms || m.NotifyFreq != model.NotifyFreqDaily ||
			m.NotifyUserIDs != "5" || m.Remark != "新" {
			t.Errorf("字段未全部更新: %+v", m)
		}
		if m.UserID != userA || m.RiskTypeID != riskTypeID {
			t.Errorf("归属字段不应变化: %d/%d", m.UserID, m.RiskTypeID)
		}
		if m.UpdatedBy != testUserName {
			t.Errorf("updated_by = %q", m.UpdatedBy)
		}
	})
}

// TestSettingService_DeleteRiskNotifySetting 删除本人记录，越权被拒。
func TestSettingService_DeleteRiskNotifySetting(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA, userB := int64(940100), int64(940101)
		riskTypeID := pickID(t, tx, "ac_risk_type", "")
		own, err := svc.CreateRiskNotifySetting(ctx, userA, &request.CreateRiskNotifySettingReq{RiskTypeID: riskTypeID})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}
		other, err := svc.CreateRiskNotifySetting(ctx, userB, &request.CreateRiskNotifySettingReq{RiskTypeID: riskTypeID})
		if err != nil {
			t.Fatalf("创建失败: %v", err)
		}

		if err := svc.DeleteRiskNotifySetting(ctx, userA, own.ID); err != nil {
			t.Fatalf("删除本人配置失败: %v", err)
		}
		if dt := softDeletedAt(t, tx, "ac_risk_notify_setting", own.ID); dt == nil {
			t.Error("deleted_at 应非空（软删除）")
		}
		page, _ := svc.PageRiskNotifySettings(ctx, userA, &request.PageRiskNotifySettingReq{PageReq: request.PageReq{Page: 1, PageSize: 20}})
		if page.Total != 0 {
			t.Errorf("删除后 A 的 total = %d, 期望 0", page.Total)
		}

		if err := svc.DeleteRiskNotifySetting(ctx, userA, other.ID); err == nil {
			t.Error("删除他人记录应返回错误")
		}
		if dt := softDeletedAt(t, tx, "ac_risk_notify_setting", other.ID); dt != nil {
			t.Error("越权删除时 deleted_at 应仍为空")
		}
	})
}

// TestSettingService_DisableAllRiskNotify 关闭所有风险通知只影响本人。
func TestSettingService_DisableAllRiskNotify(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		userA, userB := int64(940110), int64(940111)
		riskTypeID := pickID(t, tx, "ac_risk_type", "")
		a, _ := svc.CreateRiskNotifySetting(ctx, userA, &request.CreateRiskNotifySettingReq{RiskTypeID: riskTypeID})
		b, _ := svc.CreateRiskNotifySetting(ctx, userB, &request.CreateRiskNotifySettingReq{RiskTypeID: riskTypeID})

		if err := svc.DisableAllRiskNotify(ctx, userA); err != nil {
			t.Fatalf("关闭失败: %v", err)
		}
		if activeFlag(t, tx, "ac_risk_notify_setting", a.ID) {
			t.Error("A 的配置应已停用")
		}
		if !activeFlag(t, tx, "ac_risk_notify_setting", b.ID) {
			t.Error("B 的配置不应受影响")
		}
	})
}

// ── 第三方配置（个推 / 短信） ─────────────────────────────────

// TestSettingService_GetGetui_MaskSecret 个推配置读取时敏感项脱敏。
func TestSettingService_GetGetui_MaskSecret(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		if err := svc.UpdateGetui(ctx, &request.UpdateGetuiReq{
			AppID: "appid-1234", AppKey: "appkey-5678", AppSecret: "secret-9999", MasterSecret: "master-8888",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}

		got, err := svc.GetGetui(ctx)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if got.AppID != "appid-1234" || got.AppKey != "appkey-5678" {
			t.Errorf("非敏感项应返回明文: %+v", got)
		}
		if got.AppSecret == "secret-9999" || got.MasterSecret == "master-8888" {
			t.Errorf("敏感项不得返回明文: %q/%q", got.AppSecret, got.MasterSecret)
		}
		if got.AppSecret != "secr****" || got.MasterSecret != "mast****" {
			t.Errorf("脱敏格式错误: %q/%q, 期望前 4 位 + ****", got.AppSecret, got.MasterSecret)
		}

		// 库中仍保存明文
		var secret string
		_ = tx.Table("ac_thirdparty_config").Where("category = ? AND code = ?", "getui", model.GetuiCfgAppSecret).
			Pluck("value", &secret).Error
		if secret != "secret-9999" {
			t.Errorf("库中应保存明文, got %q", secret)
		}
	})
}

// TestSettingService_UpdateGetui 更新个推配置：非敏感项与敏感项均落库。
func TestSettingService_UpdateGetui(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		if err := svc.UpdateGetui(ctx, &request.UpdateGetuiReq{
			AppID: "id1", AppKey: "key1", AppSecret: "secret1", MasterSecret: "master1",
		}); err != nil {
			t.Fatalf("首次更新失败: %v", err)
		}
		if err := svc.UpdateGetui(ctx, &request.UpdateGetuiReq{
			AppID: "id2", AppKey: "key2", AppSecret: "secret2", MasterSecret: "master2",
		}); err != nil {
			t.Fatalf("二次更新失败: %v", err)
		}

		for code, want := range map[string]string{
			model.GetuiCfgAppID: "id2", model.GetuiCfgAppKey: "key2",
			model.GetuiCfgAppSecret: "secret2", model.GetuiCfgMasterSecret: "master2",
		} {
			var value string
			if err := tx.Table("ac_thirdparty_config").
				Where("category = ? AND code = ?", "getui", code).Pluck("value", &value).Error; err != nil {
				t.Fatalf("读取 %s 失败: %v", code, err)
			}
			if value != want {
				t.Errorf("%s = %q, 期望 %q", code, value, want)
			}
		}
		if n := countRows(t, tx, "ac_thirdparty_config", "category = ?", "getui"); n != 4 {
			t.Errorf("getui 配置条数 = %d, 期望 4（不重复插入）", n)
		}

		got, _ := svc.GetGetui(ctx)
		if got.AppSecret == "secret2" || got.MasterSecret == "master2" {
			t.Error("再次读取时敏感项仍应脱敏")
		}
	})
}

// TestSettingService_GetSms_MaskSecret 短信配置读取时敏感项脱敏。
func TestSettingService_GetSms_MaskSecret(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		if err := svc.UpdateSms(ctx, &request.UpdateSmsReq{
			SecretID: "AKIDxxxx", SecretKey: "key-123456", SdkAppID: "1400000000", SignName: "AI摄像头",
		}); err != nil {
			t.Fatalf("更新失败: %v", err)
		}
		got, err := svc.GetSms(ctx)
		if err != nil {
			t.Fatalf("读取失败: %v", err)
		}
		if got.SecretID != "AKIDxxxx" || got.SdkAppID != "1400000000" || got.SignName != "AI摄像头" {
			t.Errorf("非敏感项应返回明文: %+v", got)
		}
		if got.SecretKey == "key-123456" {
			t.Error("secret_key 不得返回明文")
		}
		if got.SecretKey != "key-****" {
			t.Errorf("脱敏格式错误: %q", got.SecretKey)
		}
	})
}

// TestSettingService_UpdateSms_Partial 部分更新不清空未提交项。
func TestSettingService_UpdateSms_Partial(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		svc := settingSvc()

		if err := svc.UpdateSms(ctx, &request.UpdateSmsReq{
			SecretID: "AKID-orig", SecretKey: "key-orig", SdkAppID: "140-orig", SignName: "旧签名",
		}); err != nil {
			t.Fatalf("初始化失败: %v", err)
		}
		// 仅提交 SignName
		if err := svc.UpdateSms(ctx, &request.UpdateSmsReq{SignName: "新签名"}); err != nil {
			t.Fatalf("部分更新失败: %v", err)
		}

		var sign, sid, skey, appID string
		_ = tx.Table("ac_thirdparty_config").Where("category = ? AND code = ?", "sms", model.SmsCfgSignName).Pluck("value", &sign).Error
		_ = tx.Table("ac_thirdparty_config").Where("category = ? AND code = ?", "sms", model.SmsCfgSecretID).Pluck("value", &sid).Error
		_ = tx.Table("ac_thirdparty_config").Where("category = ? AND code = ?", "sms", model.SmsCfgSecretKey).Pluck("value", &skey).Error
		_ = tx.Table("ac_thirdparty_config").Where("category = ? AND code = ?", "sms", model.SmsCfgSdkAppID).Pluck("value", &appID).Error

		if sign != "新签名" {
			t.Errorf("sign_name = %q, 期望 新签名", sign)
		}
		if sid != "AKID-orig" || skey != "key-orig" || appID != "140-orig" {
			t.Errorf("未提交项不应被清空: %q/%q/%q", sid, skey, appID)
		}

		// 提交脱敏串同样跳过
		if err := svc.UpdateSms(ctx, &request.UpdateSmsReq{SecretKey: "key-****"}); err != nil {
			t.Fatalf("提交脱敏串失败: %v", err)
		}
		var afterKey string
		_ = tx.Table("ac_thirdparty_config").Where("category = ? AND code = ?", "sms", model.SmsCfgSecretKey).Pluck("value", &afterKey).Error
		if afterKey != "key-orig" {
			t.Errorf("脱敏串不应覆盖真实密钥: %q", afterKey)
		}
	})
}
