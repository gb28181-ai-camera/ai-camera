package voiceagent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

func TestAuthenticateDeviceBasic(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		productID, _, username, password := seedAIChatProduct(t, ctx)
		dev := seedDevice(t, ctx, productID, uniq("DEV"))

		ident, err := voiceagent.AuthenticateDevice(ctx, username, password, dev.DeviceCode)
		if err != nil {
			t.Fatalf("设备鉴权失败: %v", err)
		}
		if ident.ProductID != productID || ident.DeviceID != dev.ID {
			t.Errorf("身份不一致: %+v", ident)
		}

		if _, err := voiceagent.AuthenticateDevice(ctx, username, "wrong", ""); !errors.Is(err, voiceagent.ErrDeviceUnauthorized) {
			t.Fatalf("错误密码应失败，实际 %v", err)
		}
		if _, err := voiceagent.AuthenticateDevice(ctx, "no-such-user", password, ""); !errors.Is(err, voiceagent.ErrDeviceUnauthorized) {
			t.Fatalf("未知用户应失败，实际 %v", err)
		}

		// 设备编号属于其他产品 → 拒绝
		otherProduct, _, _, _ := seedAIChatProduct(t, ctx)
		otherDev := seedDevice(t, ctx, otherProduct, uniq("OTHER"))
		if _, err := voiceagent.AuthenticateDevice(ctx, username, password, otherDev.DeviceCode); !errors.Is(err, voiceagent.ErrDeviceMismatch) {
			t.Fatalf("跨产品设备应失败，实际 %v", err)
		}

		// 设备编号不存在 → 拒绝
		if _, err := voiceagent.AuthenticateDevice(ctx, username, password, "no-such-device"); !errors.Is(err, voiceagent.ErrDeviceMismatch) {
			t.Fatalf("未知设备应失败，实际 %v", err)
		}
	})
}

func TestAuthenticateDeviceRequiresAIChatWebSocket(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		// 产品绑定 webrtc 服务组（无 WebSocket 服务）→ 拒绝。
		p := &model.IotProduct{Name: uniq("p"), Code: uniq("code")}
		if err := tx.WithContext(ctx).Create(p).Error; err != nil {
			t.Fatal(err)
		}
		g := &model.IotServiceGroup{Name: uniq("g"), Code: uniq("gc"), Type: model.IotServiceGroupWebRTC}
		if err := tx.WithContext(ctx).Create(g).Error; err != nil {
			t.Fatal(err)
		}
		s := &model.IotService{GroupID: g.ID, Code: uniq("mqtt"), Name: "MQTT", Protocol: model.IotProtocolMQTT}
		if err := tx.WithContext(ctx).Create(s).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.WithContext(ctx).Create(&model.IotProductServiceGroup{ProductID: p.ID, ServiceGroupID: g.ID}).Error; err != nil {
			t.Fatal(err)
		}
		username, password := uniq("u"), uniq("pwd")
		if err := tx.WithContext(ctx).Create(&model.IotProductService{
			ProductID: p.ID, ServiceID: s.ID, Username: username, Password: password,
		}).Error; err != nil {
			t.Fatal(err)
		}

		if _, err := voiceagent.AuthenticateDevice(ctx, username, password, ""); !errors.Is(err, voiceagent.ErrDeviceNoAIChatWS) {
			t.Fatalf("应返回 ErrDeviceNoAIChatWS，实际 %v", err)
		}
	})
}
