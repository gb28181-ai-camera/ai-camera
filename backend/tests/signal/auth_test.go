// Package signal_test 覆盖 MQTT 信令所需的内部账号鉴权与 ACL。
//
// 这些用例**不需要数据库**：__platform__ 分支在查库之前返回，
// 正好用来验证「token 未配置时 fail-closed」这条最关键的约束。
package signal_test

import (
	"context"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	iot "github.com/boticz/ai-camera-backend/internal/service/iot"
	mqttsignal "github.com/boticz/ai-camera-backend/internal/signal/mqtt"
)

// TestPlatformConstantsMatch 鉴权侧与信令 worker 侧必须使用同一个内部账号。
// 两处独立定义是为了避免 service → signal 的反向依赖，这条用例守住一致性。
func TestPlatformConstantsMatch(t *testing.T) {
	if iot.PlatformUsername != mqttsignal.PlatformUsername {
		t.Fatalf("用户名不一致: iot=%q signaling=%q", iot.PlatformUsername, mqttsignal.PlatformUsername)
	}
	if iot.PlatformGroupCode != mqttsignal.PlatformGroupCode {
		t.Fatalf("组编码不一致: iot=%q signaling=%q", iot.PlatformGroupCode, mqttsignal.PlatformGroupCode)
	}
}

// TestPlatformAuthenticate 内部账号只接受 MQTT_RPC_TOKEN。
func TestPlatformAuthenticate(t *testing.T) {
	t.Setenv("MQTT_RPC_TOKEN", "rpc-token-1")
	svc := iot.NewMqttAuthService()
	ctx := context.Background()

	resp, err := svc.Authenticate(ctx, &request.MqttAuthReq{
		Username: iot.PlatformUsername,
		Password: "rpc-token-1",
		ClientID: "ai-camera-signaling",
	})
	if err != nil {
		t.Fatalf("鉴权不应返回异常: %v", err)
	}
	if !resp.Allow {
		t.Fatalf("正确 token 应放行，实际 %+v", resp)
	}
	if resp.GroupCode != iot.PlatformGroupCode {
		t.Fatalf("内部账号应返回 %q，实际 %q", iot.PlatformGroupCode, resp.GroupCode)
	}

	wrong, _ := svc.Authenticate(ctx, &request.MqttAuthReq{
		Username: iot.PlatformUsername,
		Password: "wrong",
	})
	if wrong.Allow {
		t.Fatalf("错误 token 必须拒绝，实际 %+v", wrong)
	}
}

// TestPlatformAuthFailClosedWithoutToken MQTT_RPC_TOKEN 为空时一律拒绝。
func TestPlatformAuthFailClosedWithoutToken(t *testing.T) {
	t.Setenv("MQTT_RPC_TOKEN", "")
	svc := iot.NewMqttAuthService()
	ctx := context.Background()

	resp, _ := svc.Authenticate(ctx, &request.MqttAuthReq{
		Username: iot.PlatformUsername,
		Password: "", // 空 token 下空密码绝不能通过
	})
	if resp.Allow {
		t.Fatal("token 未配置时必须 fail-closed")
	}

	acl, _ := svc.CheckACL(ctx, &request.MqttACLReq{
		Username: iot.PlatformUsername,
		Topic:    "iot/g1/signal/backend",
		Action:   "subscribe",
	})
	if acl.Allow {
		t.Fatal("token 未配置时 ACL 也必须 fail-closed")
	}
}

// TestPlatformACL 内部账号只能访问两条信令话题。
func TestPlatformACL(t *testing.T) {
	t.Setenv("MQTT_RPC_TOKEN", "rpc-token-1")
	svc := iot.NewMqttAuthService()
	ctx := context.Background()

	cases := []struct {
		name   string
		action string
		topic  string
		allow  bool
	}{
		{"订阅通配符", "subscribe", "iot/+/signal/backend", true},
		{"订阅具体组", "subscribe", "iot/iot-webrtc-01/signal/backend", true},
		{"订阅其他话题", "subscribe", "iot/g1/app/c1/signal", false},
		{"订阅 #", "subscribe", "iot/#", false},
		{"发布 answer", "publish", "iot/g1/app/flutter-1/signal", true},
		{"发布设备房间信令", "publish", "iot/g1/device/CAM_001/signal", true},
		{"发布设备话题多一层", "publish", "iot/g1/device/CAM_001/signal/extra", false},
		{"发布多一层", "publish", "iot/g1/app/flutter-1/signal/extra", false},
		{"发布到后端话题", "publish", "iot/g1/signal/backend", false},
		{"发布通配符", "publish", "iot/g1/app/+/signal", false},
		{"发布设备通配符", "publish", "iot/g1/device/+/signal", false},
		// 内部账号服务所有服务组，因此任意组的 app 话题都应允许。
		{"其他组发布", "publish", "iot/g2/app/flutter-1/signal", true},
		{"未知动作", "delete", "iot/g1/app/flutter-1/signal", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := svc.CheckACL(ctx, &request.MqttACLReq{
				Username: iot.PlatformUsername,
				ClientID: "ai-camera-signaling",
				Topic:    c.topic,
				Action:   c.action,
			})
			if err != nil {
				t.Fatalf("ACL 不应返回异常: %v", err)
			}
			if resp.Allow != c.allow {
				t.Fatalf("topic=%q action=%q 期望 allow=%v，实际 %+v",
					c.topic, c.action, c.allow, resp)
			}
		})
	}
}
