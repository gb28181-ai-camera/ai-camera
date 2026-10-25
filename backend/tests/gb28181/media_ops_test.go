package gb28181_test

// ════════════════════════════════════════════════════════════════
// Wave 2 运维类服务测试：录像计划 / 云端录像 / 国标报警 /
// 移动位置 / 语音 Agent 配置。
//
// 运行前先清空测试库：
//   cd backend && go run ./tests/testutil/resetdb/main.go
// 环境变量指向 ai_camera_test（禁止连开发库）。
// ════════════════════════════════════════════════════════════════

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	svcsetting "github.com/boticz/ai-camera-backend/internal/service/setting"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// pbool 返回布尔指针。
func pbool(v bool) *bool { return &v }

// pstring 返回字符串指针。
func pstring(v string) *string { return &v }

// ptime 返回时间指针。
func ptime(v time.Time) *time.Time { return &v }

// ── 录像计划 CRUD 与通道绑定 ─────────────────────────────────

func TestGbRecordPlanCRUDAndLinkChannels(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := gbsvc.NewGbRecordPlanService()

		plan, err := svc.Create(ctx, &request.CreateGbRecordPlanReq{
			Name:   "园区全天录像",
			Snap:   pbool(true),
			Remark: "测试计划",
			Items: []request.GbRecordPlanItemReq{
				{WeekDay: 1, StartTime: "08:00", EndTime: "18:00"},
				{WeekDay: 2, StartTime: "00:00:00", EndTime: "23:59:59"},
			},
		})
		if err != nil {
			t.Fatalf("创建录像计划失败: %v", err)
		}
		if plan.Name != "园区全天录像" || plan.Snap == nil || !*plan.Snap {
			t.Fatalf("计划字段不正确: %+v", plan)
		}

		detail, err := svc.Get(ctx, plan.ID)
		if err != nil {
			t.Fatalf("查询计划详情失败: %v", err)
		}
		if len(detail.Items) != 2 {
			t.Fatalf("时间段应有 2 条，实际 %d", len(detail.Items))
		}
		// HH:mm 应被归一化为 HH:mm:ss
		if detail.Items[0].StartTime != "08:00:00" || detail.Items[0].EndTime != "18:00:00" {
			t.Fatalf("时间未归一化为 HH:mm:ss: %+v", detail.Items[0])
		}

		// 分页应带出时间段数与通道数
		page, err := svc.Page(ctx, &request.PageGbRecordPlanReq{Keyword: "园区"}, allScope())
		if err != nil {
			t.Fatalf("计划分页失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].ItemCount != 2 || page.Items[0].ChannelCount != 0 {
			t.Fatalf("计划分页统计不正确: %+v", page.Items)
		}

		// 编辑：改名 + 清空时间段（显式传空切片）
		if err := svc.Update(ctx, plan.ID, &request.UpdateGbRecordPlanReq{
			Name:  "园区白天录像",
			Items: []request.GbRecordPlanItemReq{},
		}); err != nil {
			t.Fatalf("更新计划失败: %v", err)
		}
		detail, err = svc.Get(ctx, plan.ID)
		if err != nil {
			t.Fatalf("更新后查询计划失败: %v", err)
		}
		if detail.Name != "园区白天录像" || len(detail.Items) != 0 {
			t.Fatalf("计划更新不正确: name=%q items=%d", detail.Name, len(detail.Items))
		}

		// 非法时间段应被拒绝
		if err := svc.Update(ctx, plan.ID, &request.UpdateGbRecordPlanReq{
			Items: []request.GbRecordPlanItemReq{{WeekDay: 9, StartTime: "08:00", EndTime: "09:00"}},
		}); err == nil {
			t.Fatalf("week_day 越界时应拒绝更新")
		}
		// 结束早于开始 = 跨天时段（22:00–06:00），现在必须允许；
		// 只有起止相同（零长度窗口）才拒绝。
		if err := svc.Update(ctx, plan.ID, &request.UpdateGbRecordPlanReq{
			Items: []request.GbRecordPlanItemReq{{WeekDay: 1, StartTime: "22:00", EndTime: "06:00"}},
		}); err != nil {
			t.Fatalf("跨天时段应允许更新: %v", err)
		}
		if err := svc.Update(ctx, plan.ID, &request.UpdateGbRecordPlanReq{
			Items: []request.GbRecordPlanItemReq{{WeekDay: 1, StartTime: "08:00", EndTime: "08:00"}},
		}); err == nil {
			t.Fatalf("起止相同时应拒绝更新")
		}

		// 通道绑定：计划关系存在 ac_gb_channel.record_plan_id
		dev := createDevice(t, ctx, "CAM_PLAN_001", "计划设备", "34020000001320001001")
		ch1 := createChannel(t, ctx, dev, "34020000001320001001", "通道一", "")
		ch2 := createChannel(t, ctx, dev, "34020000001320001002", "通道二", "")

		if err := svc.LinkChannels(ctx, plan.ID, []int64{ch1.ID, ch2.ID}); err != nil {
			t.Fatalf("绑定通道失败: %v", err)
		}
		var saved1, saved2 model.GbChannel
		if err := tx.First(&saved1, ch1.ID).Error; err != nil {
			t.Fatalf("读取通道一失败: %v", err)
		}
		if err := tx.First(&saved2, ch2.ID).Error; err != nil {
			t.Fatalf("读取通道二失败: %v", err)
		}
		if saved1.RecordPlanID == nil || *saved1.RecordPlanID != plan.ID ||
			saved2.RecordPlanID == nil || *saved2.RecordPlanID != plan.ID {
			t.Fatalf("通道绑定未落库: ch1=%v ch2=%v", saved1.RecordPlanID, saved2.RecordPlanID)
		}

		// 整体替换：只保留通道一，通道二应解绑
		if err := svc.LinkChannels(ctx, plan.ID, []int64{ch1.ID}); err != nil {
			t.Fatalf("替换绑定失败: %v", err)
		}
		if err := tx.First(&saved2, ch2.ID).Error; err != nil {
			t.Fatalf("读取通道二失败: %v", err)
		}
		if saved2.RecordPlanID != nil {
			t.Fatalf("未提交的通道应解绑，实际 %v", saved2.RecordPlanID)
		}

		// 有通道引用时禁止删除
		if err := svc.Delete(ctx, plan.ID); !errors.Is(err, gbsvc.ErrGbRecordPlanInUse) {
			t.Fatalf("通道引用时应拒绝删除，实际 %v", err)
		}
		if err := svc.LinkChannels(ctx, plan.ID, nil); err != nil {
			t.Fatalf("解绑全部通道失败: %v", err)
		}
		if err := svc.Delete(ctx, plan.ID); err != nil {
			t.Fatalf("删除计划失败: %v", err)
		}
		if _, err := svc.Get(ctx, plan.ID); !errors.Is(err, gbsvc.ErrGbRecordPlanNotFound) {
			t.Fatalf("删除后应返回计划不存在，实际 %v", err)
		}
	})
}

// ── 云端录像分页 / 收藏 / 删除 ───────────────────────────────

func TestGbCloudRecordPageCollectDelete(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_CLOUD_001", "云端录像设备", "34020000001320001101")
		ch := createChannel(t, ctx, dev, "34020000001320001101", "云端录像通道", "")

		base := time.Now().Add(-3 * time.Hour)
		records := []model.GbCloudRecord{
			{App: "record", Stream: "stream-a", DeviceID: &dev.ID, ChannelID: &ch.ID,
				StartTime: ptime(base), EndTime: ptime(base.Add(time.Hour)),
				FileName: "a.mp4", FilePath: "/www/record/record/stream-a/a.mp4", FileSize: 1024, TimeLen: 3600},
			{App: "record", Stream: "stream-b", DeviceID: &dev.ID, ChannelID: &ch.ID,
				StartTime: ptime(base.Add(2 * time.Hour)), EndTime: ptime(base.Add(3 * time.Hour)),
				FileName: "b.mp4", FilePath: "/www/record/record/stream-b/b.mp4", FileSize: 2048, TimeLen: 3600},
			{App: "record", Stream: "stream-c", DeviceID: &dev.ID, ChannelID: &ch.ID,
				StartTime: ptime(base.Add(4 * time.Hour)), EndTime: ptime(base.Add(5 * time.Hour)),
				FileName: "c.mp4", FilePath: "/www/record/record/stream-c/c.mp4", FileSize: 4096, TimeLen: 3600},
		}
		if err := tx.Create(&records).Error; err != nil {
			t.Fatalf("写入录像失败: %v", err)
		}

		svc := gbsvc.NewGbCloudRecordService()

		page, err := svc.Page(ctx, &request.PageGbCloudRecordReq{ChannelID: &ch.ID}, allScope())
		if err != nil {
			t.Fatalf("录像分页失败: %v", err)
		}
		if page.Total != 3 || len(page.Items) != 3 {
			t.Fatalf("应有 3 条录像，实际 total=%d items=%d", page.Total, len(page.Items))
		}
		if page.Items[0].DeviceName != "云端录像设备" || page.Items[0].ChannelName != "云端录像通道" {
			t.Fatalf("关联名称缺失: %+v", page.Items[0])
		}

		// 日期列表：三条录像跨越的日期数至少 1
		dates, err := svc.ListDates(ctx, &request.GbCloudRecordDateReq{ChannelID: &ch.ID},
			&utils.DataScope{DataPerm: "all", UserName: "tester"})
		if err != nil {
			t.Fatalf("查询日期列表失败: %v", err)
		}
		if len(dates) == 0 {
			t.Fatalf("日期列表不应为空")
		}

		// 收藏
		if err := svc.Collect(ctx, []int64{records[0].ID}, true); err != nil {
			t.Fatalf("收藏失败: %v", err)
		}
		collect := true
		page, err = svc.Page(ctx, &request.PageGbCloudRecordReq{ChannelID: &ch.ID, Collect: &collect}, allScope())
		if err != nil {
			t.Fatalf("按收藏筛选失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].ID != records[0].ID {
			t.Fatalf("收藏筛选不正确: %+v", page.Items)
		}

		// 删除（只删索引）
		if err := svc.Delete(ctx, []int64{records[1].ID}); err != nil {
			t.Fatalf("删除录像失败: %v", err)
		}
		page, err = svc.Page(ctx, &request.PageGbCloudRecordReq{ChannelID: &ch.ID}, allScope())
		if err != nil {
			t.Fatalf("删除后分页失败: %v", err)
		}
		if page.Total != 2 {
			t.Fatalf("删除后应剩 2 条，实际 %d", page.Total)
		}
		var deleted model.GbCloudRecord
		if err := tx.Unscoped().First(&deleted, records[1].ID).Error; err != nil {
			t.Fatalf("读取已删除录像失败: %v", err)
		}
		if !deleted.DeletedAt.Valid {
			t.Fatalf("录像应为软删除")
		}
	})
}

// ── 国标报警分页 / 处理 / 清除 ───────────────────────────────

func TestGbAlarmPageHandleClear(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_ALARM_001", "报警设备", "34020000001320001201")
		ch := createChannel(t, ctx, dev, "34020000001320001201", "报警通道", "")

		now := time.Now()
		alarms := []model.GbAlarm{
			{DeviceID: &dev.ID, ChannelID: &ch.ID, GbDeviceID: dev.GbDeviceID, GbChannelID: ch.GbChannelID,
				AlarmPriority: "1", AlarmType: "100", AlarmTime: &now, AlarmDescription: "移动侦测",
				Handled: pbool(false)},
			{DeviceID: &dev.ID, ChannelID: &ch.ID, GbDeviceID: dev.GbDeviceID, GbChannelID: ch.GbChannelID,
				AlarmPriority: "2", AlarmType: "200", AlarmTime: &now, AlarmDescription: "视频遮挡",
				Handled: pbool(false)},
		}
		if err := tx.Create(&alarms).Error; err != nil {
			t.Fatalf("写入报警失败: %v", err)
		}

		svc := gbsvc.NewGbAlarmService()

		page, err := svc.Page(ctx, &request.PageGbAlarmReq{ChannelID: &ch.ID}, allScope())
		if err != nil {
			t.Fatalf("报警分页失败: %v", err)
		}
		if page.Total != 2 || page.Items[0].DeviceName != "报警设备" || page.Items[0].ChannelName != "报警通道" {
			t.Fatalf("报警分页关联名称缺失: %+v", page.Items)
		}

		detail, err := svc.Get(ctx, alarms[0].ID)
		if err != nil {
			t.Fatalf("报警详情失败: %v", err)
		}
		if detail.AlarmType != "100" || detail.AlarmDescription != "移动侦测" {
			t.Fatalf("报警详情字段不正确: %+v", detail.GbAlarmItem)
		}

		// Handled 缺省按 true
		if err := svc.Handle(ctx, alarms[0].ID, &request.GbAlarmHandleReq{Remark: "已现场确认"}, "tester"); err != nil {
			t.Fatalf("处理报警失败: %v", err)
		}
		var saved model.GbAlarm
		if err := tx.First(&saved, alarms[0].ID).Error; err != nil {
			t.Fatalf("读取报警失败: %v", err)
		}
		if saved.Handled == nil || !*saved.Handled || saved.Remark != "已现场确认" || saved.UpdatedBy != "tester" {
			t.Fatalf("处理结果未落库: %+v", saved)
		}

		// 已处理筛选
		handled := true
		page, err = svc.Page(ctx, &request.PageGbAlarmReq{Handled: &handled}, allScope())
		if err != nil {
			t.Fatalf("按处理状态筛选失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].ID != alarms[0].ID {
			t.Fatalf("已处理筛选不正确: %+v", page.Items)
		}

		// 清除
		if err := svc.Clear(ctx, []int64{alarms[0].ID, alarms[1].ID}); err != nil {
			t.Fatalf("清除报警失败: %v", err)
		}
		page, err = svc.Page(ctx, &request.PageGbAlarmReq{ChannelID: &ch.ID}, allScope())
		if err != nil {
			t.Fatalf("清除后分页失败: %v", err)
		}
		if page.Total != 0 {
			t.Fatalf("清除后不应有报警，实际 %d", page.Total)
		}
	})
}

// ── 移动位置：最新 / 实时 / 历史 ─────────────────────────────

func TestGbPositionLatestRealtimeHistory(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		dev := createDevice(t, ctx, "CAM_POS_001", "位置设备", "34020000001320001301")
		ch1 := createChannel(t, ctx, dev, "34020000001320001301", "位置通道一", "")
		ch2 := createChannel(t, ctx, dev, "34020000001320001302", "位置通道二", "")

		base := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
		positions := []model.GbMobilePosition{
			{DeviceID: &dev.ID, ChannelID: &ch1.ID, GbChannelID: ch1.GbChannelID,
				ReportTime: ptime(base), Longitude: 120.1, Latitude: 30.1, Speed: 10, Direction: 90},
			{DeviceID: &dev.ID, ChannelID: &ch1.ID, GbChannelID: ch1.GbChannelID,
				ReportTime: ptime(base.Add(time.Hour)), Longitude: 120.2, Latitude: 30.2, Speed: 20, Direction: 180},
			{DeviceID: &dev.ID, ChannelID: &ch2.ID, GbChannelID: ch2.GbChannelID,
				ReportTime: ptime(base.Add(30 * time.Minute)), Longitude: 121.1, Latitude: 31.1, Speed: 5, Direction: 45},
		}
		if err := tx.Create(&positions).Error; err != nil {
			t.Fatalf("写入位置失败: %v", err)
		}

		svc := gbsvc.NewGbPositionService()

		latest, err := svc.Latest(ctx, ch1.ID)
		if err != nil {
			t.Fatalf("查询最新位置失败: %v", err)
		}
		if latest.Longitude != 120.2 || latest.ChannelName != "位置通道一" {
			t.Fatalf("最新位置不正确: %+v", latest)
		}

		realtime, err := svc.Realtime(ctx, &utils.DataScope{DataPerm: "all", UserName: "tester"})
		if err != nil {
			t.Fatalf("查询实时位置失败: %v", err)
		}
		if len(realtime) != 2 {
			t.Fatalf("实时位置应按通道去重为 2 条，实际 %d", len(realtime))
		}

		history, err := svc.History(ctx, &request.GbPositionHistoryReq{
			ChannelID: &ch1.ID,
			DateRangeReq: request.DateRangeReq{
				StartDT: base.Add(-time.Minute).Format("2006-01-02 15:04:05"),
				EndDT:   base.Add(2 * time.Hour).Format("2006-01-02 15:04:05"),
			},
		}, &utils.DataScope{DataPerm: "all", UserName: "tester"})
		if err != nil {
			t.Fatalf("查询位置历史失败: %v", err)
		}
		if history.Total != 2 || len(history.Items) != 2 {
			t.Fatalf("位置历史应有 2 条，实际 %d", history.Total)
		}
	})
}

// ── 语音 Agent 配置：脱敏 + 写回空值保持原值 ─────────────────

func TestVoiceAgentConfigMaskingAndKeepSecret(t *testing.T) {
	// Apply 支持环境变量覆盖（容器部署密钥不落盘）；单测必须清空外部环境，
	// 否则本机 .env 里的 PIPECAT_* 会让「未配置」场景被误判为已配置。
	t.Setenv("PIPECAT_ENDPOINT", "")
	t.Setenv("PIPECAT_TOKEN", "")
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := svcsetting.NewVoiceAgentService()
		secret := "sk-voice-secret-1234"
		provider := "openai"

		if err := svc.SaveVoiceAgent(ctx, &request.UpdateVoiceAgentReq{
			STTProvider: pstring(provider),
			STTAPIKey:   pstring(secret),
			STTModel:    pstring("whisper-1"),
			LLMProvider: pstring("anthropic"),
			LLMTemperature: func() *float64 {
				v := 0.5
				return &v
			}(),
			BargeIn: pbool(false),
		}); err != nil {
			t.Fatalf("保存语音 Agent 配置失败: %v", err)
		}

		// 读取：密钥必须脱敏，非敏感项原样返回
		cfg, err := svc.GetVoiceAgent(ctx)
		if err != nil {
			t.Fatalf("读取语音 Agent 配置失败: %v", err)
		}
		if cfg.STTProvider != provider || cfg.STTModel != "whisper-1" || cfg.LLMTemperature != 0.5 {
			t.Fatalf("配置项未正确保存: %+v", cfg)
		}
		if !strings.Contains(cfg.STTAPIKey, "****") || cfg.STTAPIKey == secret {
			t.Fatalf("STT API Key 应脱敏，实际 %q", cfg.STTAPIKey)
		}

		// 敏感键必须写入 secret=true（否则脱敏逻辑只靠内存判断，重启后失效）
		var row model.ThirdPartyConfig
		if err := tx.Where("category = ? AND code = ?", svcsetting.CategoryVoiceAgent, "stt_api_key").
			First(&row).Error; err != nil {
			t.Fatalf("读取配置行失败: %v", err)
		}
		if row.Secret == nil || !*row.Secret {
			t.Fatalf("stt_api_key 的 secret 标记应为 true")
		}

		// 写回空值：不得覆盖原密钥
		if err := svc.SaveVoiceAgent(ctx, &request.UpdateVoiceAgentReq{
			STTAPIKey: pstring(""),
			STTModel:  pstring("whisper-2"),
		}); err != nil {
			t.Fatalf("空值写回失败: %v", err)
		}
		var afterEmpty model.ThirdPartyConfig
		if err := tx.Where("category = ? AND code = ?", svcsetting.CategoryVoiceAgent, "stt_api_key").
			First(&afterEmpty).Error; err != nil {
			t.Fatalf("读取配置行失败: %v", err)
		}
		if afterEmpty.Value != secret {
			t.Fatalf("空值不应覆盖密钥，实际 %q", afterEmpty.Value)
		}

		// 写回脱敏串 ****：同样不得覆盖
		if err := svc.SaveVoiceAgent(ctx, &request.UpdateVoiceAgentReq{
			STTAPIKey: pstring(utils.MaskedValue),
		}); err != nil {
			t.Fatalf("脱敏串写回失败: %v", err)
		}
		var afterMasked model.ThirdPartyConfig
		if err := tx.Where("category = ? AND code = ?", svcsetting.CategoryVoiceAgent, "stt_api_key").
			First(&afterMasked).Error; err != nil {
			t.Fatalf("读取配置行失败: %v", err)
		}
		if afterMasked.Value != secret {
			t.Fatalf("脱敏串不应覆盖密钥，实际 %q", afterMasked.Value)
		}

		// 本地 provider 的连通性测试不需要网络
		testResp, err := svc.Test(ctx, &request.TestVoiceAgentReq{Category: "stt", Provider: "local_whisper"})
		if err != nil {
			t.Fatalf("本地 provider 测试失败: %v", err)
		}
		if !testResp.Success {
			t.Fatalf("本地 provider 测试应成功: %+v", testResp)
		}

		// 云端 provider 未配置 endpoint → 明确错误（不是假成功）
		if _, err := svc.Test(ctx, &request.TestVoiceAgentReq{Category: "llm", Provider: "openai"}); err == nil {
			t.Fatalf("缺少 endpoint 时应返回错误")
		}

		// 未配置 Pipecat 控制面 → Apply 明确报错
		if err := svc.Apply(ctx); err == nil {
			t.Fatalf("未配置 pipecat_endpoint/token 时应返回错误")
		}

		// 配置 Pipecat 后仍需真实网络；这里只验证「缺 token 时错误信息明确」
		if err := svc.SaveVoiceAgent(ctx, &request.UpdateVoiceAgentReq{
			PipecatEndpoint: pstring("http://127.0.0.1:1"),
		}); err != nil {
			t.Fatalf("保存 Pipecat endpoint 失败: %v", err)
		}
		if err := svc.Apply(ctx); err == nil || !strings.Contains(err.Error(), "pipecat_token") {
			t.Fatalf("缺少 token 时应提示 pipecat_token，实际 %v", err)
		}
	})
}

// ── 级联平台：密码脱敏 + 映射表整体替换 ─────────────────────

func TestGbPlatformPasswordMaskAndMappings(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		svc := gbsvc.NewGbPlatformService()

		platform, err := svc.Create(ctx, &request.CreateGbPlatformReq{
			Name:       "上级平台A",
			ServerGBID: "34020000002000009001",
			ServerIP:   "10.0.0.8",
			ServerPort: 5060,
			Username:   "admin",
			Password:   "platform-secret",
		})
		if err != nil {
			t.Fatalf("创建上级平台失败: %v", err)
		}
		if platform.Password != utils.MaskedValue {
			t.Fatalf("Create 返回值中的密码应已脱敏，实际 %q", platform.Password)
		}
		// 数据库中仍是明文（供 SIP Digest 使用）
		var saved model.GbPlatform
		if err := tx.First(&saved, platform.ID).Error; err != nil {
			t.Fatalf("读取平台失败: %v", err)
		}
		if saved.Password != "platform-secret" {
			t.Fatalf("数据库中的密码被误改: %q", saved.Password)
		}

		detail, err := svc.Get(ctx, platform.ID)
		if err != nil {
			t.Fatalf("查询平台详情失败: %v", err)
		}
		if detail.Password != utils.MaskedValue {
			t.Fatalf("详情中的密码应脱敏，实际 %q", detail.Password)
		}

		// 重复国标编码
		if _, err := svc.Create(ctx, &request.CreateGbPlatformReq{
			Name: "重复平台", ServerGBID: "34020000002000009001",
		}); !errors.Is(err, gbsvc.ErrGbPlatformGBIDExists) {
			t.Fatalf("重复国标编码应被拒绝，实际 %v", err)
		}

		// 映射表整体替换：连续两次设置同一集合不得撞唯一索引
		groupIDs := []int64{910001, 910002}
		for _, gid := range groupIDs {
			if err := tx.Create(&model.GbGroup{BaseModel: model.BaseModel{ID: gid}, Name: "级联分组"}).Error; err != nil {
				t.Fatalf("创建分组失败: %v", err)
			}
		}
		region := &model.Region{Name: "级联区域"}
		if err := tx.Create(region).Error; err != nil {
			t.Fatalf("创建区域失败: %v", err)
		}

		for i := 0; i < 2; i++ {
			if err := svc.SetGroups(ctx, platform.ID, groupIDs); err != nil {
				t.Fatalf("第 %d 次设置分组失败: %v", i+1, err)
			}
			if err := svc.SetRegions(ctx, platform.ID, []int64{region.ID}); err != nil {
				t.Fatalf("第 %d 次设置区域失败: %v", i+1, err)
			}
		}
		detail, err = svc.Get(ctx, platform.ID)
		if err != nil {
			t.Fatalf("查询平台详情失败: %v", err)
		}
		if len(detail.GroupIDs) != 2 || len(detail.RegionIDs) != 1 {
			t.Fatalf("映射未正确保存: groups=%v regions=%v", detail.GroupIDs, detail.RegionIDs)
		}

		// 删除平台：映射行应被物理清理
		if err := svc.Delete(ctx, platform.ID); err != nil {
			t.Fatalf("删除平台失败: %v", err)
		}
		var groupRows, regionRows int64
		if err := tx.Unscoped().Model(&model.GbPlatformGroup{}).
			Where("platform_id = ?", platform.ID).Count(&groupRows).Error; err != nil {
			t.Fatalf("统计分组映射失败: %v", err)
		}
		if err := tx.Unscoped().Model(&model.GbPlatformRegion{}).
			Where("platform_id = ?", platform.ID).Count(&regionRows).Error; err != nil {
			t.Fatalf("统计区域映射失败: %v", err)
		}
		if groupRows != 0 || regionRows != 0 {
			t.Fatalf("删除平台后映射应被清理: groups=%d regions=%d", groupRows, regionRows)
		}
		if _, err := svc.Get(ctx, platform.ID); !errors.Is(err, gbsvc.ErrGbPlatformNotFound) {
			t.Fatalf("删除后应返回平台不存在，实际 %v", err)
		}
	})
}

// ── 拉流代理 / 推流：唯一约束与基础 CRUD ─────────────────────

func TestGbStreamProxyAndPushCRUD(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		proxySvc := gbsvc.NewGbStreamProxyService()
		proxy, err := proxySvc.Create(ctx, &request.CreateGbStreamProxyReq{
			Name: "代理A", App: "proxy", Stream: "cam-1", SrcURL: "rtsp://10.0.0.9/live",
		})
		if err != nil {
			t.Fatalf("创建拉流代理失败: %v", err)
		}
		if proxy.TimeoutMS != 10000 || !boolValue(proxy.EnableAudio) || !boolValue(proxy.Enable) {
			t.Fatalf("拉流代理默认值不正确: %+v", proxy)
		}
		if _, err := proxySvc.Create(ctx, &request.CreateGbStreamProxyReq{
			Name: "代理B", App: "proxy", Stream: "cam-1", SrcURL: "rtsp://10.0.0.9/live2",
		}); !errors.Is(err, gbsvc.ErrGbStreamProxyExists) {
			t.Fatalf("重复 app+stream 应被拒绝，实际 %v", err)
		}
		name := "代理A-改名"
		if err := proxySvc.Update(ctx, proxy.ID, &request.UpdateGbStreamProxyReq{Name: &name}); err != nil {
			t.Fatalf("更新拉流代理失败: %v", err)
		}
		page, err := proxySvc.Page(ctx, &request.PageGbStreamProxyReq{Keyword: "改名"}, allScope())
		if err != nil {
			t.Fatalf("拉流代理分页失败: %v", err)
		}
		if page.Total != 1 || page.Items[0].Name != name {
			t.Fatalf("拉流代理分页不正确: %+v", page.Items)
		}
		if err := proxySvc.Delete(ctx, proxy.ID); err != nil {
			t.Fatalf("删除拉流代理失败: %v", err)
		}

		pushSvc := gbsvc.NewGbStreamPushService()
		push, err := pushSvc.Create(ctx, &request.CreateGbStreamPushReq{
			Name: "推流A", App: "push", Stream: "cam-1",
		})
		if err != nil {
			t.Fatalf("创建推流失败: %v", err)
		}
		if push.Status != model.GbStatusOff || boolValue(push.Pushing) {
			t.Fatalf("推流默认状态应为 OFF: %+v", push)
		}
		if _, err := pushSvc.Create(ctx, &request.CreateGbStreamPushReq{
			Name: "推流B", App: "push", Stream: "cam-1",
		}); !errors.Is(err, gbsvc.ErrGbStreamPushExists) {
			t.Fatalf("重复 app+stream 应被拒绝，实际 %v", err)
		}
		if err := pushSvc.Delete(ctx, push.ID); err != nil {
			t.Fatalf("删除推流失败: %v", err)
		}
	})
}
