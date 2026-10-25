package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/service/avroom"
	svciot "github.com/boticz/ai-camera-backend/internal/service/iot"
	signalroom "github.com/boticz/ai-camera-backend/internal/signal/room"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ════════════════════════════════════════════════════════════════
// 实时音视频 → 查看设备画面（MQTT 房间信令）
//
//   - GET  /web/av/devices                     WebRTC 服务组设备下拉
//   - POST /web/av/devices/:id/viewer-ticket   签发 60s 一次性 WS ticket
//   - GET  /web/av/ws?ticket=...                信令 WS（ticket 鉴权）
//
// WS 不走 JWT/Casbin：浏览器 WebSocket 无法设置 Authorization 头，
// 与语音网关一样使用一次性 ticket；ticket 本身由已认证接口签发。
// ════════════════════════════════════════════════════════════════

// AvRoomHandler 实时音视频房间处理器。
type AvRoomHandler struct {
	svc     *avroom.Service
	devices *svciot.DeviceService
}

// NewAvRoomHandler 构造处理器。
func NewAvRoomHandler() *AvRoomHandler {
	return &AvRoomHandler{svc: avroom.NewService(), devices: svciot.NewDeviceService()}
}

// PageDevices 可查看的 WebRTC 设备
//
// @Summary      实时音视频设备下拉
// @Description  只返回产品绑定且启用 WebRTC 服务组的设备；分页参数与 IOT 设备列表一致
// @Tags         AvRoom
// @Accept       json
// @Produce      json
// @Param        page      query     int     false  "页码"
// @Param        page_size query     int     false  "每页条数"
// @Param        keyword   query     string  false  "设备名称/编号"
// @Success      200  {object}  response.RPageIotDevice
// @Failure      400  {object}  response.BaseResponse
// @Router       /web/av/devices [get]
// @ID           pageAvDevices
func (h *AvRoomHandler) PageDevices(c *gin.Context) {
	var req request.PageIotDeviceReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	req.ServiceGroupType = "webrtc"
	resp, err := h.devices.Page(c.Request.Context(), &req)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, resp)
}

// ViewerTicket 签发查看方 WS ticket
//
// @Summary      查看设备画面 ticket
// @Description  校验设备可见性与 WebRTC/MQTT 服务组配置，返回 60 秒一次性 WS ticket 与 TURN 短期凭据
// @Tags         AvRoom
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RAvViewerTicket
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Router       /web/av/devices/{id}/viewer-ticket [post]
// @ID           createAvViewerTicket
func (h *AvRoomHandler) ViewerTicket(c *gin.Context) {
	deviceID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || deviceID <= 0 {
		utils.BadRequest(c, "设备ID无效")
		return
	}
	resp, err := h.svc.IssueTicket(
		c.Request.Context(),
		c.GetInt64("user_id"),
		c.GetString("user_name"),
		c.GetString("role_code"),
		c.GetInt64("dept_id"),
		c.GetString("data_perm"),
		c.GetString("cross_dept_ids"),
		deviceID,
		signalroom.RoleViewer,
	)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	resp.WsURL = wsURL(c, resp.WsURL)
	utils.Success(c, resp)
}

// AppCallTicket App 通话 WS ticket
//
// @Summary      App 通话 ticket
// @Description  App 已绑定设备时签发 role=app 的 60 秒一次性 ticket；用于设备 ↔ App 通话与查看画面
// @Tags         AvRoom
// @Accept       json
// @Produce      json
// @Param        id  path      int  true  "设备ID"
// @Success      200  {object}  response.RAvViewerTicket
// @Failure      400  {object}  response.BaseResponse
// @Failure      401  {object}  response.BaseResponse
// @Router       /app/av/devices/{id}/call-ticket [post]
// @ID           createAppCallTicket
func (h *AvRoomHandler) AppCallTicket(c *gin.Context) {
	deviceID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || deviceID <= 0 {
		utils.BadRequest(c, "设备ID无效")
		return
	}
	resp, err := h.svc.IssueTicket(
		c.Request.Context(),
		c.GetInt64("user_id"),
		c.GetString("user_name"),
		c.GetString("role_code"),
		c.GetInt64("dept_id"),
		c.GetString("data_perm"),
		c.GetString("cross_dept_ids"),
		deviceID,
		signalroom.RoleApp,
	)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	resp.WsURL = wsURL(c, "/app/av/ws")
	utils.Success(c, resp)
}

// wsUpgrader WebSocket 升级器。
//
// CheckOrigin 返回 true：鉴权完全依赖一次性 ticket，Origin 不是安全边界；
// 生产部署仍应通过反向代理限制 /web/av/ws 的来源。
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 8192,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ViewerWS 查看方信令 WebSocket
//
// @Summary      实时音视频信令 WebSocket
// @Description  使用 /viewer-ticket 签发的一次性 ticket；浏览器与设备经 MQTT 交换 joinRoom/offer/answer/iceCandidate/leaveRoom
// @Tags         AvRoom
// @Param        ticket  query  string  true  "一次性 ticket"
// @Router       /web/av/ws [get]
func (h *AvRoomHandler) ViewerWS(c *gin.Context) {
	hub := signalroom.DefaultHub()
	if hub == nil {
		utils.BadRequest(c, "MQTT 房间信令未启用")
		return
	}
	ticket := strings.TrimSpace(c.Query("ticket"))
	if ticket == "" {
		utils.Unauthorized(c, "缺少 ticket")
		return
	}
	claims, info, err := h.svc.VerifyTicket(c.Request.Context(), ticket)
	if err != nil {
		utils.Unauthorized(c, "ticket 无效或已过期: "+err.Error())
		return
	}

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 已写入 HTTP 响应，不能再调用 c.JSON。
		return
	}

	role := claims.Role
	if role == "" {
		role = signalroom.RoleViewer
	}
	viewer := &signalroom.Viewer{
		ID:         claims.ClientID,
		DeviceID:   info.Device.ID,
		RoomID:     info.RoomID,
		DeviceCode: info.Device.DeviceCode,
		GroupCode:  info.GroupCode,
		UserID:     claims.UserID,
		Role:       role,
	}
	session := &avViewerSession{
		conn:   conn,
		viewer: viewer,
		hub:    hub,
		send:   make(chan *signalroom.Message, 128),
		done:   make(chan struct{}),
	}
	viewer.Send = session.enqueue

	if _, err := hub.Join(viewer); err != nil {
		_ = conn.WriteJSON(signalroom.NewError(signalroom.CodeRoomFull, err.Error()))
		_ = conn.Close()
		return
	}
	utils.Info("WebRTC 查看方已连接",
		zap.String("viewer", viewer.ID),
		zap.String("room", viewer.RoomID),
		zap.String("device", viewer.DeviceCode),
		zap.Int64("user_id", viewer.UserID))

	// ready 让浏览器知道自己的 client_id / 房间与设备在线状态；
	// ICE 服务器已在 ticket 响应里下发，避免 WS 握手前额外往返。
	session.enqueue(&signalroom.Message{
		Type:         signalroom.TypeReady,
		RoomID:       viewer.RoomID,
		DeviceCode:   viewer.DeviceCode,
		ClientID:     viewer.ID,
		Role:         viewer.Role,
		DeviceOnline: hub.DeviceOnline(viewer.RoomID),
	})

	go session.writeLoop()
	session.readLoop(c.Request.Context())
}

// avViewerSession 一路查看方 WS 会话。
type avViewerSession struct {
	conn   *websocket.Conn
	viewer *signalroom.Viewer
	hub    *signalroom.Hub
	send   chan *signalroom.Message
	done   chan struct{}
	once   sync.Once
}

// enqueue 非阻塞投递；队列满说明客户端明显跟不上，直接关闭避免内存膨胀。
func (s *avViewerSession) enqueue(msg *signalroom.Message) bool {
	if s == nil || msg == nil {
		return false
	}
	select {
	case <-s.done:
		return false
	case s.send <- msg:
		return true
	default:
		utils.Warn("WebRTC 查看方发送队列已满，断开连接",
			zap.String("viewer", s.viewer.ID))
		s.close()
		return false
	}
}

func (s *avViewerSession) writeLoop() {
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-s.done:
			return
		case msg := <-s.send:
			_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.conn.WriteJSON(msg); err != nil {
				utils.Debug("WebRTC 查看方写失败", zap.String("viewer", s.viewer.ID), zap.Error(err))
				s.close()
				return
			}
		case <-ping.C:
			_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.close()
				return
			}
		}
	}
}

func (s *avViewerSession) readLoop(ctx context.Context) {
	defer s.close()
	s.conn.SetReadLimit(2 << 20)
	_ = s.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	s.conn.SetPongHandler(func(string) error {
		return s.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		default:
		}
		_, payload, err := s.conn.ReadMessage()
		if err != nil {
			if !errors.Is(err, websocket.ErrCloseSent) {
				utils.Debug("WebRTC 查看方读结束", zap.String("viewer", s.viewer.ID), zap.Error(err))
			}
			return
		}
		msg, err := signalroom.ParseMessage(payload)
		if err != nil {
			s.enqueue(signalroom.NewError(signalroom.CodeInvalidMessage, err.Error()))
			continue
		}
		// 浏览器不允许指定 room/device/client：Hub 会用 ticket 里的身份覆盖。
		if err := s.hub.ForwardFromViewer(s.viewer.ID, msg); err != nil {
			s.enqueue(signalroom.NewError(signalroom.CodeInvalidType, err.Error()))
		}
	}
}

func (s *avViewerSession) close() {
	s.once.Do(func() {
		close(s.done)
		s.hub.LeaveViewer(s.viewer.ID, "connection_closed")
		_ = s.conn.Close()
	})
}
