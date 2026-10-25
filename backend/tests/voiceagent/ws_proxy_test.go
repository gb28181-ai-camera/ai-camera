package voiceagent_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/service/voiceagent"
	"github.com/boticz/ai-camera-backend/tests/testutil"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func TestWSProxyWebTicket(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	pipecat := newEchoPipecat(t)

	cfg := testConfig()
	cfg.PipecatEndpoint = pipecat.URL()
	cfg.PipecatToken = "pipecat-token"

	tickets := voiceagent.NewTicketService("secret", 60*time.Second)
	gw := voiceagent.NewGateway(&staticProvider{cfg: cfg}, tickets)
	server := newWSRouter(gw, "/web/voice/ws")
	defer server.Close()

	ticket, claims, err := tickets.Issue(voiceagent.TicketClaims{
		UserID: 42, UserName: "alice", RoleCode: "admin", DeptID: 3,
		DataPerm: "dept", Kind: "web", ClientID: "web-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = claims

	conn, resp, err := dialWS(t, wsURL(server.URL, "/web/voice/ws", "ticket="+url.QueryEscape(ticket)), nil)
	if err != nil {
		t.Fatalf("连接网关失败: %v (resp=%v)", err, resp)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"hello"}`)); err != nil {
		t.Fatal(err)
	}
	mt, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("读取回显失败: %v", err)
	}
	if mt != websocket.TextMessage || string(data) != `{"type":"hello"}` {
		t.Fatalf("text 转发不一致: mt=%d data=%s", mt, data)
	}

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	mt, data, err = conn.ReadMessage()
	if err != nil {
		t.Fatalf("读取二进制回显失败: %v", err)
	}
	if mt != websocket.BinaryMessage || len(data) != 4 {
		t.Fatalf("binary 转发不一致: mt=%d data=%v", mt, data)
	}

	// 上游收到的头
	if got, _ := pipecat.authHeader.Load().(string); got != "Bearer pipecat-token" {
		t.Errorf("Authorization = %q", got)
	}
	rawCtx, _ := pipecat.ctxHeader.Load().(string)
	if rawCtx == "" {
		t.Fatal("缺少 X-Voice-Context")
	}
	signer := voiceagent.NewContextSigner("ctx-secret")
	chat, err := signer.Verify(rawCtx)
	if err != nil {
		t.Fatalf("X-Voice-Context 校验失败: %v", err)
	}
	if chat.UserID != 42 || chat.DeptID != 3 || chat.Kind != "web" || chat.SessionID == "" {
		t.Errorf("上下文异常: %+v", chat)
	}
	if chat.UserName != "alice" || chat.DataPerm != "dept" {
		t.Errorf("上下文身份字段丢失: %+v", chat)
	}

	// 重放：同一 ticket 第二次连接应 401
	_, resp2, err2 := dialWS(t, wsURL(server.URL, "/web/voice/ws", "ticket="+url.QueryEscape(ticket)), nil)
	if err2 == nil {
		t.Fatal("重放 ticket 应失败")
	}
	if resp2 == nil || resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("重放状态码 = %v, 期望 401", resp2)
	}

	// 关闭后会话状态应更新
	_ = conn.Close()
	waitSessionStopped(t, chat.SessionID)
}

func TestWSProxyDeviceBasic(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	pipecat := newEchoPipecat(t)

	testutil.WithTx(t, func(ctx context.Context, tx *gorm.DB) {
		productID, _, username, password := seedAIChatProduct(t, ctx)
		dev := seedDevice(t, ctx, productID, uniq("DEVV"))

		cfg := testConfig()
		cfg.PipecatEndpoint = pipecat.URL()
		cfg.PipecatToken = "pipecat-token"
		gw := voiceagent.NewGateway(&staticProvider{cfg: cfg}, voiceagent.NewTicketService("secret", 60*time.Second))
		server := newWSRouter(gw, "/web/voice/ws")
		defer server.Close()

		header := http.Header{}
		header.Set("Authorization", "Basic "+basic(username, password))
		u := wsURL(server.URL, "/device/voice/ws", "device_code="+url.QueryEscape(dev.DeviceCode))
		conn, resp, err := dialWS(t, u, header)
		if err != nil {
			t.Fatalf("设备连接失败: %v (resp=%v)", err, resp)
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte("ping-device")); err != nil {
			t.Fatal(err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil || string(data) != "ping-device" {
			t.Fatalf("设备回显失败: err=%v data=%s", err, data)
		}

		rawCtx, _ := pipecat.ctxHeader.Load().(string)
		chat, err := voiceagent.NewContextSigner("ctx-secret").Verify(rawCtx)
		if err != nil {
			t.Fatalf("设备上下文校验失败: %v", err)
		}
		if chat.Kind != "device" || chat.DeviceID != dev.ID || chat.ProductID != productID {
			t.Errorf("设备上下文异常: %+v", chat)
		}
		if len(chat.Scopes) != 2 {
			t.Errorf("设备 scopes = %v", chat.Scopes)
		}

		// 错误密码 → 401
		bad := http.Header{}
		bad.Set("Authorization", "Basic "+basic(username, "wrong"))
		_, resp2, err2 := dialWS(t, u, bad)
		if err2 == nil {
			t.Fatal("错误密码应连接失败")
		}
		if resp2 == nil || resp2.StatusCode != http.StatusUnauthorized {
			t.Errorf("错误密码状态码 = %v", resp2)
		}

		// 无凭据 → 401
		_, resp3, err3 := dialWS(t, u, nil)
		if err3 == nil {
			t.Fatal("无凭据应连接失败")
		}
		if resp3 == nil || resp3.StatusCode != http.StatusUnauthorized {
			t.Errorf("无凭据状态码 = %v", resp3)
		}
	})
}

func TestWSProxyRequiresConfiguredPipecat(t *testing.T) {
	t.Setenv("PIPECAT_CONTEXT_SECRET", "ctx-secret")
	cfg := testConfig()
	// 未配置 endpoint
	tickets := voiceagent.NewTicketService("secret", 60*time.Second)
	gw := voiceagent.NewGateway(&staticProvider{cfg: cfg}, tickets)
	server := newWSRouter(gw, "/web/voice/ws")
	defer server.Close()

	ticket, _, _ := tickets.Issue(voiceagent.TicketClaims{UserID: 1, Kind: "web"})
	_, resp, err := dialWS(t, wsURL(server.URL, "/web/voice/ws", "ticket="+url.QueryEscape(ticket)), nil)
	if err == nil {
		t.Fatal("未配置 Pipecat 时应失败")
	}
	if resp == nil || resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("状态码 = %v, 期望 500", resp)
	}
}

func waitSessionStopped(t *testing.T, sessionID string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var s model.VoiceSession
		err := testutil.InitTestDB().Where("session_id = ?", sessionID).First(&s).Error
		if err == nil && s.Status == "stopped" {
			if s.Transport != model.VoiceTransportWebSocket {
				t.Errorf("transport = %q", s.Transport)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("会话 %s 未在超时内结束", sessionID)
}

func basic(username, password string) string {
	return base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
}
