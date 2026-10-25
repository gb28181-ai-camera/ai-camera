package gb28181_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// ════════════════════════════════════════════════════════════════
// 心跳/注册间隔统计（WVP 对齐 GET /web/gb/devices/:id/time-statistics）
// ════════════════════════════════════════════════════════════════

func TestCoreTimeStatisticsKeepalive(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		gbID := "34020000001320003001"
		createDevice(t, ctx, "CAM_CORE_STAT_001", "统计设备", gbID)

		base := time.Now().Add(-10 * time.Minute)
		logs := []model.GbSipLog{
			{GbDeviceID: gbID, Direction: "in", Method: "MESSAGE", CmdType: "Keepalive", CreatedDT: base},
			{GbDeviceID: gbID, Direction: "in", Method: "MESSAGE", CmdType: "Keepalive", CreatedDT: base.Add(10 * time.Second)},
			{GbDeviceID: gbID, Direction: "in", Method: "MESSAGE", CmdType: "Keepalive", CreatedDT: base.Add(30 * time.Second)},
			{GbDeviceID: gbID, Direction: "in", Method: "MESSAGE", CmdType: "Keepalive", CreatedDT: base.Add(60 * time.Second)},
			// 出向报文不得参与统计。
			{GbDeviceID: gbID, Direction: "out", Method: "MESSAGE", CmdType: "Keepalive", CreatedDT: base.Add(15 * time.Second)},
		}
		if err := tx.Create(&logs).Error; err != nil {
			t.Fatalf("写入信令日志失败: %v", err)
		}

		svc := gbsvc.NewGbTimeStatisticsService()
		dev := &model.Device{}
		if err := tx.Where("gb_device_id = ?", gbID).First(dev).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}

		resp, err := svc.TimeStatistics(ctx, dev.ID, "keepalive", 50)
		if err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		if resp.Count != 3 || len(resp.Points) != 3 {
			t.Fatalf("应有 3 个间隔，实际 count=%d points=%d", resp.Count, len(resp.Points))
		}
		if resp.Points[0].IntervalSeconds != 10 || resp.Points[1].IntervalSeconds != 20 || resp.Points[2].IntervalSeconds != 30 {
			t.Fatalf("间隔序列不正确: %+v", resp.Points)
		}
		if resp.AverageSeconds != 20 || resp.MaxSeconds != 30 || resp.MinSeconds != 10 {
			t.Fatalf("统计值不正确: avg=%v max=%d min=%d", resp.AverageSeconds, resp.MaxSeconds, resp.MinSeconds)
		}
		if resp.Points[0].Time == "" {
			t.Fatalf("时间点未格式化")
		}

		// count 表示采样条数：取最近 2 条间隔需要 3 条报文。
		resp, err = svc.TimeStatistics(ctx, dev.ID, "keepalive", 2)
		if err != nil {
			t.Fatalf("限制条数统计失败: %v", err)
		}
		if resp.Count != 2 || resp.Points[0].IntervalSeconds != 20 || resp.Points[1].IntervalSeconds != 30 {
			t.Fatalf("限制条数结果不正确: %+v", resp.Points)
		}

		// 非法类型
		if _, err := svc.TimeStatistics(ctx, dev.ID, "heartbeat", 10); !errors.Is(err, gbsvc.ErrGbTimeStatTypeInvalid) {
			t.Fatalf("非法类型应返回 ErrGbTimeStatTypeInvalid，实际 %v", err)
		}
	})
}

func TestCoreTimeStatisticsRegister(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		gbID := "34020000001320003002"
		createDevice(t, ctx, "CAM_CORE_STAT_002", "注册统计设备", gbID)

		base := time.Now().Add(-time.Hour)
		logs := []model.GbSipLog{
			{GbDeviceID: gbID, Direction: "in", Method: "REGISTER", StatusCode: 401, CreatedDT: base},
			{GbDeviceID: gbID, Direction: "in", Method: "REGISTER", StatusCode: 200, CreatedDT: base.Add(2 * time.Second)},
			{GbDeviceID: gbID, Direction: "in", Method: "REGISTER", StatusCode: 200, CreatedDT: base.Add(3602 * time.Second)},
		}
		if err := tx.Create(&logs).Error; err != nil {
			t.Fatalf("写入注册日志失败: %v", err)
		}
		dev := &model.Device{}
		if err := tx.Where("gb_device_id = ?", gbID).First(dev).Error; err != nil {
			t.Fatalf("读取设备失败: %v", err)
		}

		resp, err := gbsvc.NewGbTimeStatisticsService().TimeStatistics(ctx, dev.ID, "register", 50)
		if err != nil {
			t.Fatalf("注册统计失败: %v", err)
		}
		if resp.Count != 2 || resp.Points[0].IntervalSeconds != 2 || resp.Points[1].IntervalSeconds != 3600 {
			t.Fatalf("注册间隔不正确: %+v", resp.Points)
		}
		if resp.MaxSeconds != 3600 || resp.MinSeconds != 2 {
			t.Fatalf("注册统计极值不正确: max=%d min=%d", resp.MaxSeconds, resp.MinSeconds)
		}
	})
}

func TestCoreTimeStatisticsNoLogs(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		d := createDevice(t, ctx, "CAM_CORE_STAT_003", "无日志设备", "34020000001320003003")
		resp, err := gbsvc.NewGbTimeStatisticsService().TimeStatistics(ctx, d.ID, "", 50)
		if err != nil {
			t.Fatalf("空数据统计失败: %v", err)
		}
		if resp.Type != "keepalive" || resp.Count != 0 || len(resp.Points) != 0 {
			t.Fatalf("空数据应返回空序列: %+v", resp)
		}
		_ = tx
	})
}
