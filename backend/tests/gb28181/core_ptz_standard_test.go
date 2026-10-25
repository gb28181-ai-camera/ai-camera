package gb28181_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 服务层必须把标准指令码真正发出去
//
// 纯构造函数的断言在 core_control_xml_test.go；这里验证业务方法没有
// 又退回旧的 0x85/0x86 映射（「按了没反应」最常见的原因）。
// ════════════════════════════════════════════════════════════════

func TestCorePtzStandardCommandWiring(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, _ *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_PTZSTD_001", "标准云台设备", "34020000001320007001")
		ch := createChannel(t, ctx, d, "34020000001320007001", "标准云台通道", "")

		fake := &coreCaptureController{}
		control.Set(fake)
		defer control.Set(nil)

		svc := gbsvc.NewGbPtzService()
		want := func(prefix string) {
			t.Helper()
			if len(fake.bodies) == 0 {
				t.Fatalf("未下发任何报文")
			}
			last := fake.bodies[len(fake.bodies)-1]
			if !strings.Contains(last, prefix) {
				t.Fatalf("报文未包含标准指令 % X: % X", prefix, last)
			}
		}

		// 加入/删除巡航点
		if err := svc.AddCruisePoint(ctx, ch.ID, 1, 5); err != nil {
			t.Fatalf("加入巡航点失败: %v", err)
		}
		want("\xA5\x0F\x01\x84\x01\x05")

		if err := svc.DeleteCruisePoint(ctx, ch.ID, 1, 5); err != nil {
			t.Fatalf("删除巡航点失败: %v", err)
		}
		want("\xA5\x0F\x01\x85\x01\x05")

		// 巡航速度 + 停留时间 → 两条报文
		before := len(fake.bodies)
		if err := svc.SetCruisePoint(ctx, ch.ID, 1, 300, 10); err != nil {
			t.Fatalf("设置巡航参数失败: %v", err)
		}
		if len(fake.bodies) != before+2 {
			t.Fatalf("速度与停留时间应各下发一条，实际新增 %d 条", len(fake.bodies)-before)
		}
		if !strings.Contains(fake.bodies[before], "\xA5\x0F\x01\x86\x01\x2C\x10") {
			t.Fatalf("巡航速度报文不正确: % X", fake.bodies[before])
		}
		if !strings.Contains(fake.bodies[before+1], "\xA5\x0F\x01\x87\x01\x0A") {
			t.Fatalf("停留时间报文不正确: % X", fake.bodies[before+1])
		}

		// 扫描边界 + 速度
		left := true
		speed := 5
		before = len(fake.bodies)
		if err := svc.SetScanBounds(ctx, ch.ID, &request.GbScanBoundsReq{
			GroupID: 1, SetLeft: &left, Speed: &speed,
		}); err != nil {
			t.Fatalf("设置扫描边界失败: %v", err)
		}
		if len(fake.bodies) != before+2 {
			t.Fatalf("边界与速度应各下发一条，实际新增 %d 条", len(fake.bodies)-before)
		}
		if !strings.Contains(fake.bodies[before], "\xA5\x0F\x01\x89\x01\x01") {
			t.Fatalf("左边界报文不正确: % X", fake.bodies[before])
		}
		if !strings.Contains(fake.bodies[before+1], "\xA5\x0F\x01\x8A\x01\x05") {
			t.Fatalf("扫描速度报文不正确: % X", fake.bodies[before+1])
		}

		// 全是 false 的「设置边界」不能静默成功。
		if err := svc.SetScanBounds(ctx, ch.ID, &request.GbScanBoundsReq{
			GroupID: 1, SetLeft: boolPtr(false), SetRight: boolPtr(false),
		}); !errors.Is(err, gbsvc.ErrGbPtzScanBoundInvalid) {
			t.Fatalf("空扫描设置应返回 ErrGbPtzScanBoundInvalid，实际 %v", err)
		}

		// 巡航启动 0x88 / 停止 = 全零指令码
		if err := svc.Cruise(ctx, ch.ID, &request.GbPtzCruiseReq{Action: "start", GroupID: 1}); err != nil {
			t.Fatalf("启动巡航失败: %v", err)
		}
		want("\xA5\x0F\x01\x88\x01\x00")
		if err := svc.Cruise(ctx, ch.ID, &request.GbPtzCruiseReq{Action: "stop", GroupID: 1}); err != nil {
			t.Fatalf("停止巡航失败: %v", err)
		}
		want("\xA5\x0F\x01\x00\x00\x00")

		// 扫描启动 0x89 数据2=0 / 停止 = 全零指令码
		if err := svc.Scan(ctx, ch.ID, &request.GbPtzScanReq{Action: "start", GroupID: 1}); err != nil {
			t.Fatalf("启动扫描失败: %v", err)
		}
		want("\xA5\x0F\x01\x89\x01\x00")
		if err := svc.Scan(ctx, ch.ID, &request.GbPtzScanReq{Action: "stop", GroupID: 1}); err != nil {
			t.Fatalf("停止扫描失败: %v", err)
		}
		want("\xA5\x0F\x01\x00\x00\x00")
	})
}
