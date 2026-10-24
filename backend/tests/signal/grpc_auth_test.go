package signal_test

import (
	"context"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/mqttrpc"
	proto "github.com/boticz/ai-camera-backend/internal/mqttrpc/proto"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// TestGRPCMqttAuth 内部 fork 的 gRPC 契约：code=200 且 data=true 才放行。
func TestGRPCMqttAuth(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		active := true
		g := &model.IotServiceGroup{
			Name: "grpc组", Code: "grpc-group", Type: model.IotServiceGroupWebRTC,
			Username: "grpc_user", Password: "grpc_pass", Active: &active,
		}
		if err := tx.Create(g).Error; err != nil {
			t.Fatalf("创建服务组失败: %v", err)
		}
		item := &model.IotServiceItem{GroupID: g.ID, Protocol: model.IotProtocolMQTT, Host: "mqtt", Port: 1883, Active: &active}
		if err := tx.Create(item).Error; err != nil {
			t.Fatalf("创建服务项失败: %v", err)
		}

		srv := mqttrpc.New(nil)

		ok, err := srv.MqttAuth(ctx, &proto.MqttAuthRequest{Username: "grpc_user", Password: "grpc_pass"})
		if err != nil {
			t.Fatalf("gRPC 鉴权返回错误: %v", err)
		}
		if ok.GetCode() != 200 || !ok.GetData() {
			t.Fatalf("正确凭据应放行，实际 %+v", ok)
		}

		bad, _ := srv.MqttAuth(ctx, &proto.MqttAuthRequest{Username: "grpc_user", Password: "wrong"})
		if bad.GetCode() == 200 || bad.GetData() {
			t.Fatalf("错误密码必须拒绝，实际 %+v", bad)
		}
	})
}
