package iot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	svciot "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// TestDeviceReportMQTT 覆盖 MQTT 上报的电量/位置/告警落库。
func TestDeviceReportMQTT(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		ctx = testCtx()
		group := mustGroup(t, ctx, uniq("RPT_G"), model.IotServiceGroupWebRTC)
		mustService(t, ctx, group.ID, "mqtt", model.IotProtocolMQTT)
		product := mustProduct(t, ctx, uniq("RPT_P"))
		if _, err := svciot.NewProductService().BindServiceGroups(ctx, product.ID, []int64{group.ID}); err != nil {
			t.Fatalf("绑定服务组失败: %v", err)
		}
		dev := mustDevice(t, ctx, uniq("RPT_D"))
		if err := svciot.NewDeviceService().SetProduct(ctx, dev.ID, &product.ID); err != nil {
			t.Fatalf("设置设备产品失败: %v", err)
		}

		// 告警类型（测试库不一定有种子数据）。
		alarmType := &model.AlarmType{
			Active: boolPtr(true), Name: "跌倒", Code: "fall", Level: "high",
		}
		if err := tx.Create(alarmType).Error; err != nil {
			t.Fatalf("创建告警类型失败: %v", err)
		}

		svc := svciot.NewDeviceReportService()
		report := func(v map[string]interface{}) {
			t.Helper()
			raw, _ := json.Marshal(v)
			if err := svc.Handle(ctx, group.Code, raw); err != nil {
				t.Fatalf("上报失败 type=%v: %v", v["type"], err)
			}
		}

		report(map[string]interface{}{
			"type": "battery", "device_code": dev.DeviceCode,
			"battery": 66, "voltage": "12.1", "storage_free": 2048,
		})
		got, err := repository.NewDeviceRepo().GetByID(ctx, dev.ID)
		if err != nil {
			t.Fatalf("回读设备失败: %v", err)
		}
		if got.Battery != 66 || got.Voltage != "12.1" || got.StorageFree != 2048 || got.Status != "online" {
			t.Fatalf("电量上报未落库: %+v", got)
		}
		if got.LastBeatDT == nil || time.Since(*got.LastBeatDT) > time.Minute {
			t.Errorf("last_beat_dt 未刷新: %v", got.LastBeatDT)
		}

		report(map[string]interface{}{
			"type": "position", "device_code": dev.DeviceCode,
			"longitude": 120.1693, "latitude": 30.2741,
			"speed": 1.5, "direction": 90, "altitude": 12,
		})
		got, _ = repository.NewDeviceRepo().GetByID(ctx, dev.ID)
		if got.Longitude == "" || got.Latitude == "" {
			t.Fatalf("设备最新位置未更新: %+v", got)
		}
		pos, err := repository.NewIotDevicePositionRepo().Latest(ctx, dev.ID)
		if err != nil || pos == nil {
			t.Fatalf("位置历史未写入: %v", err)
		}
		if pos.Longitude != 120.1693 || pos.Latitude != 30.2741 || pos.Speed != 1.5 {
			t.Fatalf("位置历史字段错误: %+v", pos)
		}

		report(map[string]interface{}{
			"type": "alarm", "device_code": dev.DeviceCode,
			"alarm_type_code": "fall", "alarm_level": "high", "content": "测试跌倒",
		})
		var alarm model.AlarmRecord
		if err := tx.Where("device_id = ? AND alarm_type_id = ?", dev.ID, alarmType.ID).
			First(&alarm).Error; err != nil {
			t.Fatalf("告警未落库: %v", err)
		}
		if alarm.Content != "测试跌倒" || alarm.Status != model.HandleStatusPending || alarm.DeviceCode != dev.DeviceCode {
			t.Fatalf("告警字段错误: %+v", alarm)
		}

		// 未绑定到该服务组的设备必须拒绝。
		other := mustDevice(t, ctx, uniq("RPT_X"))
		raw, _ := json.Marshal(map[string]interface{}{"type": "battery", "device_code": other.DeviceCode, "battery": 50})
		if err := svc.Handle(ctx, group.Code, raw); err == nil {
			t.Fatal("未绑定服务组的设备上报应被拒绝")
		}
	})
}
