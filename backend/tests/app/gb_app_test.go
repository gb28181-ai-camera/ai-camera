package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/model"
	appsvc "github.com/boticz/ai-camera-backend/internal/service/app"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 移动端国标视频（/app/gb）薄封装服务测试
//
// 覆盖点：
//  1. 设备/通道列表只返回国标数据，且 DataScope 生效；
//  2. 移动端 DTO 字段裁剪（禁止 sip_password / password 等敏感字段）；
//  3. SIP 未启动时点播/回放/对讲必须显式报错（不能静默成功）；
//  4. 云台方向/速度在访问数据库前就被拒绝；
//  5. 国标报警分页与字段裁剪。
//
// 运行前先清空测试库：
//   cd backend && go run ./tests/testutil/resetdb/main.go
// ════════════════════════════════════════════════════════════════

// mkAppGbDevice 直接写入一台设备（gbID 为空表示普通 AI 相机）。
func mkAppGbDevice(t *testing.T, ctx context.Context, prefix, gbID, sipPassword string) *model.Device {
	t.Helper()
	active := true
	d := &model.Device{
		Name:           prefix,
		DeviceCode:     uniq(prefix + "_CODE"),
		GbDeviceID:     gbID,
		SipPassword:    sipPassword,
		Active:         &active,
		Status:         model.DeviceStatusOnline,
		RegisterStatus: model.GbRegisterStatusPending,
	}
	if err := bootstrap.DB().WithContext(ctx).Create(d).Error; err != nil {
		t.Fatalf("创建设备失败: %v", err)
	}
	return d
}

// gbChannelSeq 生成 20 位国标通道编码（<10 位域><132><7 位序号>）。
//
// ac_gb_channel.gb_channel_id 是 varchar(20)，不能直接用 uniq()（会超长）；
// 进程内自增序号保证同一测试进程内唯一，跨进程由事务回滚隔离。
var gbChannelSeq int64

func nextGbChannelID() string {
	return fmt.Sprintf("3402000000132%07d", atomic.AddInt64(&gbChannelSeq, 1))
}

// mkAppGbChannel 直接写入一条国标通道。
func mkAppGbChannel(t *testing.T, ctx context.Context, dev *model.Device, name, password string) *model.GbChannel {
	t.Helper()
	c := &model.GbChannel{
		DeviceID:    dev.ID,
		DeviceCode:  dev.DeviceCode,
		GbDeviceID:  dev.GbDeviceID,
		GbChannelID: nextGbChannelID(),
		Name:        name,
		Status:      model.GbStatusOn,
		Password:    password,
		PtzType:     1,
	}
	if err := bootstrap.DB().WithContext(ctx).Create(c).Error; err != nil {
		t.Fatalf("创建通道失败: %v", err)
	}
	return c
}

// appGbSvc 构造被测服务。
func appGbSvc() *appsvc.AppGbService { return appsvc.NewAppGbService() }

// disableSIP 确保当前进程内没有 SIP 控制器（模拟「只读部署 / 信令服务未启动」）。
//
// 测试结束后恢复原值，避免影响同包其它用例。
func disableSIP(t *testing.T) {
	t.Helper()
	prev := control.Default()
	control.Set(nil)
	t.Cleanup(func() { control.Set(prev) })
}

// mustJSON 序列化响应，用于断言「字段没有被透出」。
func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	return string(b)
}

// ── 设备列表 ──────────────────────────────────────────────────

func TestAppGbDevicePageOnlyGbAndTrimmed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		prefix := uniq("APPGB_DEV")
		gb1 := mkAppGbDevice(t, ctx, prefix+"_A", "34020000001320000001", "SuperSecret123")
		gb2 := mkAppGbDevice(t, ctx, prefix+"_B", "34020000001320000002", "AnotherSecret456")
		normal := mkAppGbDevice(t, ctx, prefix+"_N", "", "NoSipSecret")
		if err := tx.Model(gb2).Update("register_status", model.GbRegisterStatusActivated).Error; err != nil {
			t.Fatalf("更新注册状态失败: %v", err)
		}

		svc := appGbSvc()

		// 关键字只命中本用例的两台国标设备（普通相机不带该前缀，且本来就会被过滤）
		resp, err := svc.PageDevices(ctx, &request.PageGbDeviceReq{Keyword: prefix}, dsAll())
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 2 || len(resp.Items) != 2 {
			t.Fatalf("应只返回 2 台国标设备，实际 total=%d items=%d", resp.Total, len(resp.Items))
		}
		for _, item := range resp.Items {
			if item.GbDeviceID == "" {
				t.Fatalf("返回了非国标设备: %+v", item)
			}
			if item.ID == normal.ID {
				t.Fatalf("普通 AI 相机不应出现在国标设备列表")
			}
		}

		// 字段裁剪双保险：DTO 中不存在敏感字段，序列化结果也不得出现明文。
		raw := mustJSON(t, resp)
		for _, forbidden := range []string{"sip_password", "sip_realm", "sdp_ip", "SuperSecret123", "AnotherSecret456"} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("移动端设备列表不应包含 %q：%s", forbidden, raw)
			}
		}

		// 注册状态筛选
		pending, err := svc.PageDevices(ctx, &request.PageGbDeviceReq{Keyword: prefix, RegisterStatus: model.GbRegisterStatusPending}, dsAll())
		if err != nil {
			t.Fatalf("按注册状态筛选失败: %v", err)
		}
		if pending.Total != 1 || pending.Items[0].ID != gb1.ID {
			t.Fatalf("pending 筛选应命中设备 %d，实际 total=%d", gb1.ID, pending.Total)
		}

		// 分页
		page1, err := svc.PageDevices(ctx, &request.PageGbDeviceReq{Keyword: prefix, PageReq: request.PageReq{Page: 1, PageSize: 1}}, dsAll())
		if err != nil {
			t.Fatalf("分页查询失败: %v", err)
		}
		page2, err := svc.PageDevices(ctx, &request.PageGbDeviceReq{Keyword: prefix, PageReq: request.PageReq{Page: 2, PageSize: 1}}, dsAll())
		if err != nil {
			t.Fatalf("分页查询失败: %v", err)
		}
		if len(page1.Items) != 1 || len(page2.Items) != 1 || page1.Items[0].ID == page2.Items[0].ID {
			t.Fatalf("分页未生效：page1=%+v page2=%+v", page1.Items, page2.Items)
		}
	})
}

func TestAppGbDevicePageRespectsDataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		prefix := uniq("APPGB_SCOPE")
		// 本部门数据：用 testCtx() 触发 GORM 钩子回填 dept_id=1
		mine := mkAppGbDevice(t, testCtx(), prefix+"_MINE", "34020000001320000011", "")
		otherCtx := ctxFor(testUserID, "other-user", 2)
		other := mkAppGbDevice(t, otherCtx, prefix+"_OTHER", "34020000001320000012", "")

		svc := appGbSvc()
		// dept 级权限：仅本部门（dept 1）
		resp, err := svc.PageDevices(ctx, &request.PageGbDeviceReq{Keyword: prefix},
			dsPerm("dept", testUserName, int64Ptr(testDeptID), ""))
		if err != nil {
			t.Fatalf("分页失败: %v", err)
		}
		if resp.Total != 1 || resp.Items[0].ID != mine.ID {
			t.Fatalf("dept 数据权限应只返回设备 %d，实际 total=%d items=%+v", mine.ID, resp.Total, resp.Items)
		}
		for _, item := range resp.Items {
			if item.ID == other.ID {
				t.Fatalf("不应返回其它部门的设备")
			}
		}
	})
}

// ── 通道列表 ──────────────────────────────────────────────────

func TestAppGbChannelPageFiltersAndTrimmed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		prefix := uniq("APPGB_CH")
		devA := mkAppGbDevice(t, ctx, prefix+"_DEVA", "34020000001320000021", "")
		devB := mkAppGbDevice(t, ctx, prefix+"_DEVB", "34020000001320000022", "")
		chA1 := mkAppGbChannel(t, ctx, devA, prefix+"_通道1", "ChannelSecret1")
		chA2 := mkAppGbChannel(t, ctx, devA, prefix+"_通道2", "ChannelSecret2")
		chB1 := mkAppGbChannel(t, ctx, devB, prefix+"_通道3", "ChannelSecret3")

		svc := appGbSvc()

		resp, err := svc.PageChannels(ctx, &request.PageGbChannelReq{DeviceID: &devA.ID}, dsAll())
		if err != nil {
			t.Fatalf("通道分页失败: %v", err)
		}
		if resp.Total != 2 || len(resp.Items) != 2 {
			t.Fatalf("按设备筛选应返回 2 条通道，实际 total=%d items=%d", resp.Total, len(resp.Items))
		}
		seen := map[int64]bool{}
		for _, item := range resp.Items {
			if item.DeviceID != devA.ID {
				t.Fatalf("返回了其它设备的通道: %+v", item)
			}
			if item.DeviceName != devA.Name {
				t.Fatalf("应回填设备名称 %q，实际 %q", devA.Name, item.DeviceName)
			}
			seen[item.ID] = true
		}
		if !seen[chA1.ID] || !seen[chA2.ID] || seen[chB1.ID] {
			t.Fatalf("通道筛选结果不正确: %+v", resp.Items)
		}

		// 字段裁剪：通道密码不得出现在移动端响应中
		raw := mustJSON(t, resp)
		for _, forbidden := range []string{"password", "ChannelSecret1", "ChannelSecret2"} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("移动端通道列表不应包含 %q：%s", forbidden, raw)
			}
		}

		// 关键字筛选
		kw, err := svc.PageChannels(ctx, &request.PageGbChannelReq{Keyword: prefix + "_通道2"}, dsAll())
		if err != nil {
			t.Fatalf("关键字筛选失败: %v", err)
		}
		if kw.Total != 1 || kw.Items[0].ID != chA2.ID {
			t.Fatalf("关键字应命中通道 %d，实际 total=%d", chA2.ID, kw.Total)
		}
	})
}

func TestAppGbChannelPageRespectsDataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		prefix := uniq("APPGB_CHSCOPE")
		devA := mkAppGbDevice(t, testCtx(), prefix+"_A", "34020000001320000031", "")
		chA := mkAppGbChannel(t, testCtx(), devA, prefix+"_A通道", "")
		otherCtx := ctxFor(testUserID, "other-user", 2)
		devB := mkAppGbDevice(t, otherCtx, prefix+"_B", "34020000001320000032", "")
		mkAppGbChannel(t, otherCtx, devB, prefix+"_B通道", "")

		svc := appGbSvc()
		resp, err := svc.PageChannels(ctx, &request.PageGbChannelReq{Keyword: prefix},
			dsPerm("dept", testUserName, int64Ptr(testDeptID), ""))
		if err != nil {
			t.Fatalf("通道分页失败: %v", err)
		}
		if resp.Total != 1 || resp.Items[0].ID != chA.ID {
			t.Fatalf("dept 数据权限应只返回通道 %d，实际 total=%d items=%+v", chA.ID, resp.Total, resp.Items)
		}
	})
}

// ── SIP 未启动时的显式错误 ────────────────────────────────────

func TestAppGbPlayFailsLoudWhenSIPUnavailable(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		disableSIP(t)
		dev := mkAppGbDevice(t, ctx, uniq("APPGB_PLAY"), "34020000001320000041", "")
		ch := mkAppGbChannel(t, ctx, dev, uniq("APPGB_PLAY_CH"), "")

		_, err := appGbSvc().Play(ctx, ch.ID, &request.GbPlayReq{Protocol: "webrtc"})
		if !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("SIP 未启动时应返回 ErrSipServiceUnavailable，实际 %v", err)
		}

		// 抓图同样依赖 SIP/媒体链路，必须显式失败而不是返回空图。
		if _, err := appGbSvc().Snapshot(ctx, ch.ID); !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("SIP 未启动时抓图应显式失败，实际 %v", err)
		}
	})
}

func TestAppGbPlaybackFailsLoudWhenSIPUnavailable(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		disableSIP(t)
		dev := mkAppGbDevice(t, ctx, uniq("APPGB_PB"), "34020000001320000042", "")
		ch := mkAppGbChannel(t, ctx, dev, uniq("APPGB_PB_CH"), "")

		svc := appGbSvc()

		// 时间窗非法应在触达 SIP 之前就被拒绝。
		_, err := svc.QueryRecord(ctx, ch.ID, &request.GbRecordQueryReq{
			ChannelID: ch.ID, StartTime: "not-a-time", EndTime: "2026-01-01T23:59:59",
		})
		if !errors.Is(err, gbsvc.ErrPlaybackTimeInvalid) {
			t.Fatalf("非法时间窗应返回 ErrPlaybackTimeInvalid，实际 %v", err)
		}

		// 合法时间窗 + SIP 未启动 → 503 语义错误。
		_, err = svc.QueryRecord(ctx, ch.ID, &request.GbRecordQueryReq{
			ChannelID: ch.ID, StartTime: "2026-01-01T00:00:00", EndTime: "2026-01-01T23:59:59",
		})
		if !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("SIP 未启动时录像查询应返回 ErrSipServiceUnavailable，实际 %v", err)
		}

		_, err = svc.StartPlayback(ctx, &request.GbPlaybackReq{
			ChannelID: ch.ID, StartTime: "2026-01-01T00:00:00", EndTime: "2026-01-01T23:59:59",
		})
		if !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("SIP 未启动时回放应返回 ErrSipServiceUnavailable，实际 %v", err)
		}
	})
}

func TestAppGbTalkFailsLoudWhenSIPUnavailable(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		disableSIP(t)
		dev := mkAppGbDevice(t, ctx, uniq("APPGB_TALK"), "34020000001320000043", "")
		ch := mkAppGbChannel(t, ctx, dev, uniq("APPGB_TALK_CH"), "")

		_, err := appGbSvc().StartTalk(ctx, ch.ID, testUserID, true)
		if !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("SIP 未启动时对讲应返回 ErrSipServiceUnavailable，实际 %v", err)
		}
	})
}

// ── 云台参数校验 ──────────────────────────────────────────────

func TestAppGbPtzRejectsInvalidDirectionAndSpeed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		disableSIP(t)
		dev := mkAppGbDevice(t, ctx, uniq("APPGB_PTZ"), "34020000001320000051", "")
		ch := mkAppGbChannel(t, ctx, dev, uniq("APPGB_PTZ_CH"), "")

		svc := appGbSvc()

		// 非法方向：在访问数据库之前拒绝（通道 ID 可以是任意值）。
		err := svc.Ptz(ctx, ch.ID, &request.GbPtzReq{Direction: "diagonal", Speed: 5})
		if !errors.Is(err, appsvc.ErrAppGbPtzDirectionInvalid) {
			t.Fatalf("非法方向应被拒绝，实际 %v", err)
		}

		// 非法速度：契约钉死 1–8。
		err = svc.Ptz(ctx, ch.ID, &request.GbPtzReq{Direction: "up", Speed: 9})
		if !errors.Is(err, appsvc.ErrAppGbPtzSpeedInvalid) {
			t.Fatalf("速度 9 应被拒绝，实际 %v", err)
		}
		err = svc.Ptz(ctx, ch.ID, &request.GbPtzReq{Direction: "up", Speed: -1})
		if !errors.Is(err, appsvc.ErrAppGbPtzSpeedInvalid) {
			t.Fatalf("负速度应被拒绝，实际 %v", err)
		}

		// direction 与 action 互斥 / 两者都为空。
		err = svc.Ptz(ctx, ch.ID, &request.GbPtzReq{Direction: "up", Action: "zoom-in", Speed: 5})
		if !errors.Is(err, gbsvc.ErrGbPtzDirectionConflict) {
			t.Fatalf("direction+action 应被拒绝，实际 %v", err)
		}
		err = svc.Ptz(ctx, ch.ID, &request.GbPtzReq{})
		if !errors.Is(err, gbsvc.ErrGbPtzCommandMissing) {
			t.Fatalf("空指令应被拒绝，实际 %v", err)
		}

		// 合法参数且 SIP 未启动 → 明确的服务不可用错误（而不是静默成功）。
		err = svc.Ptz(ctx, ch.ID, &request.GbPtzReq{Direction: "up-left", Speed: 5})
		if !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("合法方向在 SIP 未启动时应返回 ErrSipServiceUnavailable，实际 %v", err)
		}
	})
}

// ── 国标报警 ──────────────────────────────────────────────────

func TestAppGbAlarmPageFiltersAndTrimmed(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		prefix := uniq("APPGB_ALARM")
		dev := mkAppGbDevice(t, ctx, prefix+"_DEV", "34020000001320000061", "")
		ch := mkAppGbChannel(t, ctx, dev, prefix+"_CH", "")

		now := time.Now().Add(-time.Minute)
		older := now.Add(-2 * time.Hour)
		handled := true
		alarms := []model.GbAlarm{
			{
				DeviceID: &dev.ID, ChannelID: &ch.ID, GbDeviceID: dev.GbDeviceID, GbChannelID: ch.GbChannelID,
				AlarmPriority: "1", AlarmType: "VideoLoss", AlarmTime: &now,
				AlarmDescription: prefix + "_报警A", Handled: boolPtr(false), SnapPath: "/uploads/a.jpg",
			},
			{
				DeviceID: &dev.ID, ChannelID: &ch.ID, GbDeviceID: dev.GbDeviceID, GbChannelID: ch.GbChannelID,
				AlarmPriority: "3", AlarmType: "DiskFull", AlarmTime: &older,
				AlarmDescription: prefix + "_报警B", Handled: &handled,
			},
		}
		for i := range alarms {
			if err := tx.WithContext(ctx).Create(&alarms[i]).Error; err != nil {
				t.Fatalf("创建国标报警失败: %v", err)
			}
		}

		svc := appGbSvc()

		resp, err := svc.PageAlarms(ctx, &request.PageGbAlarmReq{Keyword: prefix}, dsAll())
		if err != nil {
			t.Fatalf("报警分页失败: %v", err)
		}
		if resp.Total != 2 || len(resp.Items) != 2 {
			t.Fatalf("应返回 2 条报警，实际 total=%d items=%d", resp.Total, len(resp.Items))
		}
		for _, item := range resp.Items {
			if item.DeviceName != dev.Name || item.ChannelName != ch.Name {
				t.Fatalf("应回填设备/通道名称，实际 %+v", item)
			}
			if item.AlarmDescription == "" || item.AlarmTime == nil {
				t.Fatalf("报警描述/时间不能为空: %+v", item)
			}
		}

		// 字段裁剪：国标编码、备注等管理面字段不透出
		raw := mustJSON(t, resp)
		for _, forbidden := range []string{"gb_device_id", "gb_channel_id", "alarm_method", "remark", "record_path"} {
			if strings.Contains(raw, forbidden) {
				t.Fatalf("移动端报警列表不应包含 %q：%s", forbidden, raw)
			}
		}

		// handled=false 筛选
		unhandled, err := svc.PageAlarms(ctx, &request.PageGbAlarmReq{Keyword: prefix, Handled: boolPtr(false)}, dsAll())
		if err != nil {
			t.Fatalf("按处理状态筛选失败: %v", err)
		}
		if unhandled.Total != 1 || unhandled.Items[0].ID != alarms[0].ID {
			t.Fatalf("handled=false 应命中报警 %d，实际 total=%d", alarms[0].ID, unhandled.Total)
		}

		// 分页
		page2, err := svc.PageAlarms(ctx, &request.PageGbAlarmReq{
			Keyword: prefix, PageReq: request.PageReq{Page: 2, PageSize: 1},
		}, dsAll())
		if err != nil {
			t.Fatalf("报警分页失败: %v", err)
		}
		if len(page2.Items) != 1 || page2.Total != 2 {
			t.Fatalf("第二页应返回 1 条且 total=2，实际 total=%d items=%d", page2.Total, len(page2.Items))
		}
	})
}

func TestAppGbAlarmPageRespectsDataScope(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		prefix := uniq("APPGB_ALARMSCOPE")
		devA := mkAppGbDevice(t, testCtx(), prefix+"_A", "34020000001320000071", "")
		chA := mkAppGbChannel(t, testCtx(), devA, prefix+"_CHA", "")
		otherCtx := ctxFor(testUserID, "other-user", 2)
		devB := mkAppGbDevice(t, otherCtx, prefix+"_B", "34020000001320000072", "")
		chB := mkAppGbChannel(t, otherCtx, devB, prefix+"_CHB", "")

		now := time.Now()
		mine := model.GbAlarm{
			DeviceID: &devA.ID, ChannelID: &chA.ID, GbDeviceID: devA.GbDeviceID, GbChannelID: chA.GbChannelID,
			AlarmPriority: "1", AlarmType: "VideoLoss", AlarmTime: &now, AlarmDescription: prefix + "_MINE",
		}
		other := model.GbAlarm{
			DeviceID: &devB.ID, ChannelID: &chB.ID, GbDeviceID: devB.GbDeviceID, GbChannelID: chB.GbChannelID,
			AlarmPriority: "1", AlarmType: "VideoLoss", AlarmTime: &now, AlarmDescription: prefix + "_OTHER",
		}
		if err := tx.WithContext(testCtx()).Create(&mine).Error; err != nil {
			t.Fatalf("创建报警失败: %v", err)
		}
		if err := tx.WithContext(otherCtx).Create(&other).Error; err != nil {
			t.Fatalf("创建报警失败: %v", err)
		}

		resp, err := appGbSvc().PageAlarms(ctx, &request.PageGbAlarmReq{Keyword: prefix},
			dsPerm("dept", testUserName, int64Ptr(testDeptID), ""))
		if err != nil {
			t.Fatalf("报警分页失败: %v", err)
		}
		if resp.Total != 1 || resp.Items[0].ID != mine.ID {
			t.Fatalf("dept 数据权限应只返回报警 %d，实际 total=%d items=%+v", mine.ID, resp.Total, resp.Items)
		}
	})
}
