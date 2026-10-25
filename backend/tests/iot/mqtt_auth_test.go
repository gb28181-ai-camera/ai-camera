package iot_test

import (
	"context"
	"testing"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	iot "github.com/boticz/ai-camera-backend/internal/service/iot"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"gorm.io/gorm"
)

// createMqttFixture 建「服务组 → MQTT 服务 → 产品 → 产品服务凭据」。
//
// MQTT 鉴权以产品×服务凭据为准，因此必须把整条链建出来。
func createMqttFixture(
	t *testing.T, tx *gorm.DB,
	username, password, groupType string, withMQTT bool,
) *model.IotServiceGroup {
	t.Helper()
	active := true
	groupCode := "code-" + username
	g := &model.IotServiceGroup{
		Name:   "测试组-" + username,
		Code:   groupCode,
		Type:   groupType,
		Active: &active,
	}
	if err := tx.Create(g).Error; err != nil {
		t.Fatalf("创建服务组失败: %v", err)
	}
	if !withMQTT {
		return g
	}

	svc := &model.IotService{
		GroupID:  g.ID,
		Code:     "mqtt",
		Name:     "MQTT",
		Protocol: model.IotProtocolMQTT,
		Host:     "mqtt",
		Port:     1883,
		Active:   &active,
	}
	if err := tx.Create(svc).Error; err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	product := &model.IotProduct{
		Name: "产品-" + username, Code: "product-" + username, Active: &active,
	}
	if err := tx.Create(product).Error; err != nil {
		t.Fatalf("创建产品失败: %v", err)
	}
	cred := &model.IotProductService{
		Active: &active, ProductID: product.ID, ServiceID: svc.ID,
		Username: username, Password: password,
	}
	if err := tx.Create(cred).Error; err != nil {
		t.Fatalf("创建产品服务凭据失败: %v", err)
	}
	return g
}

// TestMqttAuthenticate MQTT 鉴权只接受启用的 webrtc 组且密码必须匹配。
func TestMqttAuthenticate(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		createMqttFixture(t, tx, "mqtt_user_ok", "secret-1", model.IotServiceGroupWebRTC, true)
		svc := iot.NewMqttAuthService()

		resp, err := svc.Authenticate(ctx, &request.MqttAuthReq{
			Username: "mqtt_user_ok", Password: "secret-1",
		})
		if err != nil {
			t.Fatalf("鉴权失败: %v", err)
		}
		if !resp.Allow || resp.GroupCode != "code-mqtt_user_ok" {
			t.Fatalf("正确凭据应放行并返回组编码，实际 %+v", resp)
		}

		resp, _ = svc.Authenticate(ctx, &request.MqttAuthReq{Username: "mqtt_user_ok", Password: "bad"})
		if resp.Allow {
			t.Fatal("错误密码必须拒绝")
		}

		resp, _ = svc.Authenticate(ctx, &request.MqttAuthReq{Username: "no_such_user", Password: "x"})
		if resp.Allow || resp.Reason != "用户名或密码错误" {
			t.Fatalf("未知用户应拒绝且文案统一，实际 %+v", resp)
		}
	})
}

// TestMqttAuthenticateRejectsWrongGroupType 非 webrtc 组不能用于 MQTT。
func TestMqttAuthenticateRejectsWrongGroupType(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		createMqttFixture(t, tx, "gb_user", "pwd", model.IotServiceGroupGb28181, true)
		svc := iot.NewMqttAuthService()
		resp, _ := svc.Authenticate(ctx, &request.MqttAuthReq{Username: "gb_user", Password: "pwd"})
		if resp.Allow {
			t.Fatal("gb28181 组不应通过 MQTT 鉴权")
		}
	})
}

// TestMqttAuthenticateRequiresItem 没有 MQTT 服务/凭据时不能通过。
func TestMqttAuthenticateRequiresItem(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		createMqttFixture(t, tx, "no_item_user", "pwd", model.IotServiceGroupWebRTC, false)
		svc := iot.NewMqttAuthService()
		resp, _ := svc.Authenticate(ctx, &request.MqttAuthReq{Username: "no_item_user", Password: "pwd"})
		if resp.Allow {
			t.Fatal("未配置 MQTT 服务时必须拒绝")
		}
	})
}

// TestMqttACL 话题必须落在 iot/<group_code>/ 命名空间内。
func TestMqttACL(t *testing.T) {
	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		createMqttFixture(t, tx, "acl_user", "pwd", model.IotServiceGroupWebRTC, true)
		svc := iot.NewMqttAuthService()

		allow, _ := svc.CheckACL(ctx, &request.MqttACLReq{
			Username: "acl_user", Topic: "iot/code-acl_user/device-1/data", Action: "publish",
		})
		if !allow.Allow {
			t.Fatalf("本组命名空间内应允许: %+v", allow)
		}

		deny, _ := svc.CheckACL(ctx, &request.MqttACLReq{
			Username: "acl_user", Topic: "iot/other-group/data", Action: "publish",
		})
		if deny.Allow {
			t.Fatal("跨组话题必须拒绝")
		}

		badAction, _ := svc.CheckACL(ctx, &request.MqttACLReq{
			Username: "acl_user", Topic: "iot/code-acl_user/x", Action: "delete",
		})
		if badAction.Allow {
			t.Fatal("未知动作必须拒绝")
		}
	})
}
