package gb28181_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 设备控制服务 / 同步进度 / 通道媒体信息
// ════════════════════════════════════════════════════════════════

func TestCoreDeviceControlCommands(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_CTL_001", "控制设备", "34020000001320005001")
		ch := createChannel(t, ctx, d, "34020000001320005001", "控制通道", "")

		fake := &coreCaptureController{}
		control.Set(fake)
		defer control.Set(nil)

		svc := gbsvc.NewGbDeviceControlService()

		// 重启
		if err := svc.Reboot(ctx, d.ID); err != nil {
			t.Fatalf("重启指令下发失败: %v", err)
		}
		if len(fake.bodies) != 1 || !strings.Contains(fake.bodies[0], "<TeleBoot>Boot</TeleBoot>") {
			t.Fatalf("重启报文不正确: %+v", fake.bodies)
		}

		// 布防 / 撤防
		if err := svc.Guard(ctx, d.ID, "SetGuard"); err != nil {
			t.Fatalf("布防失败: %v", err)
		}
		if !strings.Contains(fake.bodies[len(fake.bodies)-1], "<GuardCmd>SetGuard</GuardCmd>") {
			t.Fatalf("布防报文不正确: %s", fake.bodies[len(fake.bodies)-1])
		}
		if err := svc.Guard(ctx, d.ID, "setguard"); err != nil {
			t.Fatalf("布防大小写不应影响解析: %v", err)
		}
		if err := svc.Guard(ctx, d.ID, "Disarm"); !errors.Is(err, gbsvc.ErrGbGuardCmdInvalid) {
			t.Fatalf("非法布防指令应被拒绝，实际 %v", err)
		}

		// 报警复位：指定通道时 DeviceID 用通道国标编码。
		chID := ch.ID
		if err := svc.AlarmReset(ctx, d.ID, &chID); err != nil {
			t.Fatalf("报警复位失败: %v", err)
		}
		last := fake.bodies[len(fake.bodies)-1]
		if !strings.Contains(last, "<AlarmCmd>ResetAlarm</AlarmCmd>") ||
			!strings.Contains(last, "<DeviceID>34020000001320005001</DeviceID>") {
			t.Fatalf("报警复位报文不正确: %s", last)
		}

		// 通道不属于该设备时必须拒绝。
		other := createDevice(t, ctx, "CAM_CORE_CTL_002", "其他设备", "34020000001320005002")
		otherCh := createChannel(t, ctx, other, "34020000001320005002", "其他通道", "")
		otherChID := otherCh.ID
		if err := svc.AlarmReset(ctx, d.ID, &otherChID); !errors.Is(err, gbsvc.ErrGbAlarmChannelMismatch) {
			t.Fatalf("跨设备通道应被拒绝，实际 %v", err)
		}

		// 通道录像控制
		chSvc := gbsvc.NewGbChannelService()
		if err := chSvc.RecordControl(ctx, ch.ID, "start"); err != nil {
			t.Fatalf("开始录像失败: %v", err)
		}
		if !strings.Contains(fake.bodies[len(fake.bodies)-1], "<RecordCmd>Record</RecordCmd>") {
			t.Fatalf("录像控制报文不正确: %s", fake.bodies[len(fake.bodies)-1])
		}
		if err := chSvc.RecordControl(ctx, ch.ID, "pause"); !errors.Is(err, gbsvc.ErrGbRecordActionInvalid) {
			t.Fatalf("非法录像动作应被拒绝，实际 %v", err)
		}

		// 没有在线播放会话时 media-info 返回明确的 404 语义错误。
		if _, err := chSvc.MediaInfo(ctx, ch.ID); !errors.Is(err, gbsvc.ErrGbNoActiveStream) {
			t.Fatalf("无在线流应返回 ErrGbNoActiveStream，实际 %v", err)
		}

		// SIP 未启动 → 503 语义错误（而不是静默成功）。
		control.Set(nil)
		if err := svc.Reboot(ctx, d.ID); !errors.Is(err, gbsvc.ErrSipServiceUnavailable) {
			t.Fatalf("SIP 未启动应返回 ErrSipServiceUnavailable，实际 %v", err)
		}
	})
}

func TestCoreDeviceSyncStatusIdle(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_SYNC_001", "同步设备", "34020000001320006001")
		resp, err := gbsvc.NewGbDeviceSyncStatusService().SyncStatus(ctx, d.ID)
		if err != nil {
			t.Fatalf("查询同步进度失败: %v", err)
		}
		if resp.Status != "idle" || resp.DeviceID != d.ID || resp.GbDeviceID != d.GbDeviceID {
			t.Fatalf("无同步记录时应返回 idle: %+v", resp)
		}
	})
}
