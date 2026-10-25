package response

// ════════════════════════════════════════════════════════════════
// v0.6.0 实时音视频：MQTT 房间信令
// ════════════════════════════════════════════════════════════════

// AvViewerTicketResp 查看方 WS ticket 与房间信息。
//
// ice_servers 与 App 的 /app/rtc/ice-servers 同源（TURN REST 短期凭据），
// 但 subject 使用 web-<user_id>，便于在 coturn 日志里区分来源。
type AvViewerTicketResp struct {
	Ticket       string      `json:"ticket" example:"eyJ1aWQiOjF9.xxxx"`
	ExpiresIn    int         `json:"expires_in" example:"60"`
	WsURL        string      `json:"ws_url" example:"/web/av/ws"`
	RoomID       string      `json:"room_id" example:"CAM_001"`
	DeviceID     int64       `json:"device_id" example:"1"`
	DeviceCode   string      `json:"device_code" example:"CAM_001"`
	DeviceName   string      `json:"device_name" example:"东门摄像头"`
	ClientID     string      `json:"client_id" example:"viewer-1a2b3c4d"`
	Role         string      `json:"role" example:"viewer"`
	GroupCode    string      `json:"group_code" example:"webrtc"`
	DeviceOnline bool        `json:"device_online" example:"false"`
	IceServers   []IceServer `json:"ice_servers"`
}

// RAvViewerTicket 查看方 ticket 响应。
type RAvViewerTicket struct {
	BaseResponse
	Data AvViewerTicketResp `json:"data"`
}
