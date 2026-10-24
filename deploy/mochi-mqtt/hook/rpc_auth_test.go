package hook

import (
	"context"
	"errors"
	"testing"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

// fakeRPC 用可控响应替身后端，覆盖「允许 / 拒绝 / 故障」三条分支。
type fakeRPC struct {
	authResp AuthResponse
	authErr  error
	aclResp  ACLResponse
	aclErr   error
	lastAuth AuthRequest
	lastACL  ACLRequest
}

func (f *fakeRPC) Authenticate(_ context.Context, req AuthRequest) (AuthResponse, error) {
	f.lastAuth = req
	return f.authResp, f.authErr
}

func (f *fakeRPC) CheckACL(_ context.Context, req ACLRequest) (ACLResponse, error) {
	f.lastACL = req
	return f.aclResp, f.aclErr
}

func testClient() *mqtt.Client {
	return &mqtt.Client{
		ID: "client-1",
		Properties: mqtt.ClientProperties{
			Username: []byte("iot_user"),
		},
		Net: mqtt.ClientConnection{Remote: "10.0.0.9:50123"},
	}
}

func testPacket(user, pass string) packets.Packet {
	return packets.Packet{
		Connect: packets.ConnectParams{
			Username: []byte(user),
			Password: []byte(pass),
		},
	}
}

// TestAuthenticateAllow 鉴权通过时应把请求参数完整传给后端。
func TestAuthenticateAllow(t *testing.T) {
	rpc := &fakeRPC{authResp: AuthResponse{Allow: true, GroupCode: "iot-webrtc"}}
	h := New(Config{BackendURL: "http://backend", Token: "t"}, rpc)

	if !h.OnConnectAuthenticate(testClient(), testPacket("iot_user", "secret")) {
		t.Fatal("后端允许时应返回 true")
	}
	if rpc.lastAuth.Username != "iot_user" || rpc.lastAuth.Password != "secret" {
		t.Fatalf("请求参数未透传: %+v", rpc.lastAuth)
	}
	if rpc.lastAuth.ClientID != "client-1" || rpc.lastAuth.RemoteAddr != "10.0.0.9:50123" {
		t.Fatalf("客户端信息未透传: %+v", rpc.lastAuth)
	}
	if h.groupCodes["client-1"] != "iot-webrtc" {
		t.Fatalf("应缓存 group_code，实际 %q", h.groupCodes["client-1"])
	}
}

// TestAuthenticateDeny 后端明确拒绝时返回 false。
func TestAuthenticateDeny(t *testing.T) {
	rpc := &fakeRPC{authResp: AuthResponse{Allow: false, Reason: "密码错误"}}
	h := New(Config{BackendURL: "http://backend", Token: "t"}, rpc)
	if h.OnConnectAuthenticate(testClient(), testPacket("iot_user", "bad")) {
		t.Fatal("后端拒绝时应返回 false")
	}
}

// TestAuthenticateFailClosed 后端故障/超时必须拒绝，绝不能放行。
func TestAuthenticateFailClosed(t *testing.T) {
	rpc := &fakeRPC{authErr: errors.New("connection refused")}
	h := New(Config{BackendURL: "http://backend", Token: "t"}, rpc)
	if h.OnConnectAuthenticate(testClient(), testPacket("iot_user", "secret")) {
		t.Fatal("RPC 失败时必须 fail-closed")
	}
}

// TestACLActionMapping write=true 映射为 publish，false 映射为 subscribe。
func TestACLActionMapping(t *testing.T) {
	rpc := &fakeRPC{aclResp: ACLResponse{Allow: true}}
	h := New(Config{BackendURL: "http://backend", Token: "t"}, rpc)
	cl := testClient()

	if !h.OnACLCheck(cl, "iot/g/write", true) {
		t.Fatal("允许的 publish 应返回 true")
	}
	if rpc.lastACL.Action != "publish" {
		t.Fatalf("write=true 应映射为 publish，实际 %q", rpc.lastACL.Action)
	}
	if rpc.lastACL.Username != "iot_user" {
		t.Fatalf("用户名应从 Client.Properties 取，实际 %q", rpc.lastACL.Username)
	}

	if !h.OnACLCheck(cl, "iot/g/read", false) {
		t.Fatal("允许的 subscribe 应返回 true")
	}
	if rpc.lastACL.Action != "subscribe" {
		t.Fatalf("write=false 应映射为 subscribe，实际 %q", rpc.lastACL.Action)
	}
}

// TestACLFailClosed ACL 故障同样必须拒绝。
func TestACLFailClosed(t *testing.T) {
	rpc := &fakeRPC{aclErr: errors.New("timeout")}
	h := New(Config{BackendURL: "http://backend", Token: "t"}, rpc)
	if h.OnACLCheck(testClient(), "iot/g/x", true) {
		t.Fatal("ACL RPC 失败时必须 fail-closed")
	}
}

// TestProvides 只声明本 hook 负责的两个扩展点。
func TestProvides(t *testing.T) {
	h := New(Config{}, &fakeRPC{})
	if !h.Provides(mqtt.OnConnectAuthenticate) || !h.Provides(mqtt.OnACLCheck) {
		t.Fatal("应提供鉴权与 ACL 两个 hook")
	}
	if h.Provides(mqtt.OnConnect) {
		t.Fatal("不应声明不处理的 hook")
	}
}
