package voiceagent

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocket 转发参数。
const (
	wsReadLimit     = 4 << 20 // 4 MiB：容纳 20ms 音频帧的批量发送
	wsWriteWait     = 10 * time.Second
	wsPongWait      = 60 * time.Second
	wsPingPeriod    = 25 * time.Second
	wsHandshakeWait = 10 * time.Second
)

// Gateway WS 反向代理：客户端 ↔ Go ↔ Pipecat /ws/voice。
//
// 只做鉴权、上下文签名与双向字节转发；不解析音频，不做 VAD/STT。
type Gateway struct {
	provider ConfigProvider
	tickets  *TicketService
	sessions *repository.VoiceSessionAgentRepo
}

// NewGateway 构造网关。
func NewGateway(provider ConfigProvider, tickets *TicketService) *Gateway {
	return &Gateway{
		provider: provider,
		tickets:  tickets,
		sessions: repository.NewVoiceSessionAgentRepo(),
	}
}

// HandleWeb 处理 /web/voice/ws 与 /app/voice/ws（ticket 鉴权）。
func (g *Gateway) HandleWeb(c *gin.Context, kind string) {
	cfg, err := g.provider.Runtime(c.Request.Context())
	if err != nil {
		utils.ServerError(c, "读取语音配置失败: "+err.Error())
		return
	}
	if !cfg.WSEnabled {
		utils.Forbidden(c, "语音 WebSocket 未启用")
		return
	}
	ticket := c.Query("ticket")
	if strings.TrimSpace(ticket) == "" {
		utils.Unauthorized(c, "缺少 ticket")
		return
	}
	claims, err := g.tickets.Verify(kind, ticket)
	if err != nil {
		utils.Unauthorized(c, err.Error())
		return
	}

	sessionID, err := randomHex(16)
	if err != nil {
		utils.ServerError(c, "生成会话ID失败")
		return
	}
	ctx := request.ChatContext{
		SessionID:    sessionID,
		UserID:       claims.UserID,
		UserName:     claims.UserName,
		RoleCode:     claims.RoleCode,
		DeptID:       claims.DeptID,
		DataPerm:     claims.DataPerm,
		CrossDeptIDs: claims.CrossDeptIDs,
		DeviceID:     claims.DeviceID,
		ClientID:     claims.ClientID,
		Kind:         kind,
		AgentCode:    claims.AgentCode,
		Scopes:       []string{"voice"},
	}
	g.serve(c, cfg, &ctx)
}

// HandleDevice 处理 /device/voice/ws（HTTP Basic 产品凭据）。
func (g *Gateway) HandleDevice(c *gin.Context) {
	cfg, err := g.provider.Runtime(c.Request.Context())
	if err != nil {
		utils.ServerError(c, "读取语音配置失败: "+err.Error())
		return
	}
	if !cfg.WSEnabled {
		utils.Forbidden(c, "语音 WebSocket 未启用")
		return
	}
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		utils.Unauthorized(c, "缺少设备 Basic 凭据")
		return
	}
	deviceCode := strings.TrimSpace(c.Query("device_code"))
	if deviceCode == "" {
		deviceCode = strings.TrimSpace(c.GetHeader("X-Device-Code"))
	}
	ident, err := AuthenticateDevice(c.Request.Context(), username, password, deviceCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrDeviceNoAIChatWS):
			utils.Forbidden(c, err.Error())
		case errors.Is(err, ErrDeviceMismatch):
			utils.Forbidden(c, err.Error())
		default:
			utils.Unauthorized(c, err.Error())
		}
		return
	}
	sessionID, err := randomHex(16)
	if err != nil {
		utils.ServerError(c, "生成会话ID失败")
		return
	}
	ctx := request.ChatContext{
		SessionID: sessionID,
		DeviceID:  ident.DeviceID,
		ProductID: ident.ProductID,
		Kind:      "device",
		Scopes:    []string{"voice", "device"},
	}
	g.serve(c, cfg, &ctx)
}

// serve 完成升级、上游连接与双向转发。
func (g *Gateway) serve(c *gin.Context, cfg *RuntimeConfig, chatCtx *request.ChatContext) {
	wsURL, err := cfg.PipecatWSURL()
	if err != nil {
		utils.ServerError(c, err.Error())
		return
	}
	if strings.TrimSpace(cfg.PipecatToken) == "" {
		utils.ServerError(c, "未配置 PIPECAT_TOKEN")
		return
	}
	signer := NewContextSigner(ContextSecret())
	signed, err := signer.Sign(*chatCtx)
	if err != nil {
		utils.ServerError(c, "上下文签名失败: "+err.Error())
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		// 鉴权已完成（ticket/设备凭据），不再依赖 Origin：
		// App/Electron/直连设备没有浏览器 Origin，跨域限制在网关层无意义。
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	client, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 已写入 HTTP 错误响应，这里只记录。
		return
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.PipecatToken))
	headers.Set("X-Voice-Context", signed)
	headers.Set("User-Agent", "ai-camera-voice-gateway/"+chatCtx.Kind)

	dialer := &websocket.Dialer{
		HandshakeTimeout: wsHandshakeWait,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	dialCtx, cancel := context.WithTimeout(c.Request.Context(), wsHandshakeWait)
	defer cancel()
	upstream, resp, err := dialer.DialContext(dialCtx, wsURL, headers)
	if err != nil {
		msg := "无法连接语音服务"
		if resp != nil {
			msg += " (HTTP " + http.StatusText(resp.StatusCode) + ")"
		}
		_ = client.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, msg),
			time.Now().Add(wsWriteWait),
		)
		_ = client.Close()
		return
	}

	g.trackSession(c.Request.Context(), chatCtx)
	closeCode := g.proxy(client, upstream)
	g.finishSession(c.Request.Context(), chatCtx.SessionID, closeCode)
}

// trackSession 记录一条 WebSocket 语音会话（失败不影响转发）。
func (g *Gateway) trackSession(ctx context.Context, chatCtx *request.ChatContext) {
	if g.sessions == nil {
		return
	}
	now := time.Now()
	m := &model.VoiceSession{
		SessionID:  chatCtx.SessionID,
		Type:       model.VoiceSessionAgent,
		Transport:  model.VoiceTransportWebSocket,
		ClientID:   chatCtx.ClientID,
		AgentCode:  chatCtx.AgentCode,
		StartDT:    &now,
		Status:     "active",
		DeptID:     nilIfZero(chatCtx.DeptID),
		LastSeenDT: &now,
	}
	if chatCtx.UserID > 0 {
		uid := chatCtx.UserID
		m.UserID = &uid
	}
	if chatCtx.DeviceID > 0 {
		did := chatCtx.DeviceID
		m.DeviceID = &did
	}
	_ = g.sessions.Create(ctx, m)
}

// finishSession 更新会话结束状态（失败不影响流程）。
func (g *Gateway) finishSession(ctx context.Context, sessionID string, closeCode int) {
	if g.sessions == nil || sessionID == "" {
		return
	}
	now := time.Now()
	_ = g.sessions.UpdateFields(ctx, sessionID, map[string]interface{}{
		"status":       "stopped",
		"end_dt":       now,
		"last_seen_at": now,
		"close_code":   closeCode,
	})
}

// proxy 双向转发，返回关闭码。
func (g *Gateway) proxy(client, upstream *websocket.Conn) int {
	stop := make(chan struct{})
	go pingLoop(client, stop)

	done := make(chan int, 2)
	go func() { done <- pump(upstream, client, true) }()  // 客户端 → Pipecat
	go func() { done <- pump(client, upstream, false) }() // Pipecat → 客户端

	code := <-done
	close(stop)
	_ = client.Close()
	_ = upstream.Close()
	// 等待另一方向退出，避免 goroutine 泄漏。
	select {
	case <-done:
	case <-time.After(time.Second):
	}
	return code
}

// pingLoop 定期向客户端发 ping，维持 NAT/代理连接。
func pingLoop(conn *websocket.Conn, stop <-chan struct{}) {
	ticker := time.NewTicker(wsPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
				return
			}
		}
	}
}

// pump 把 src 的消息原样写到 dst（text/binary 不解析）。
//
// keepAlive=true 时启用读超时 + pong 续期（仅客户端方向）：
// 服务端 Pipecat 不保证回 pong，给上游设读超时会误杀长连接。
func pump(dst, src *websocket.Conn, keepAlive bool) int {
	src.SetReadLimit(wsReadLimit)
	if keepAlive {
		_ = src.SetReadDeadline(time.Now().Add(wsPongWait))
		src.SetPongHandler(func(string) error {
			return src.SetReadDeadline(time.Now().Add(wsPongWait))
		})
	}

	for {
		mt, data, err := src.ReadMessage()
		if err != nil {
			code := closeCode(err)
			_ = dst.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(code, ""),
				time.Now().Add(wsWriteWait),
			)
			return code
		}
		if mt == websocket.CloseMessage {
			_ = dst.WriteControl(websocket.CloseMessage, data, time.Now().Add(wsWriteWait))
			return closeCodeFromPayload(data)
		}
		_ = dst.SetWriteDeadline(time.Now().Add(wsWriteWait))
		if err := dst.WriteMessage(mt, data); err != nil {
			return websocket.CloseAbnormalClosure
		}
	}
}

func closeCode(err error) int {
	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		return ce.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return websocket.CloseGoingAway
	}
	return websocket.CloseAbnormalClosure
}

func closeCodeFromPayload(data []byte) int {
	if len(data) >= 2 {
		return int(data[0])<<8 | int(data[1])
	}
	return websocket.CloseNormalClosure
}

func nilIfZero(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	return &v
}
