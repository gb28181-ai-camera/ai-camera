package response

import "time"

// ════════════════════════════════════════════════════════════════
// 移动端国标视频（GB/T 28181）响应类型
//
// 为什么 App 要单独定义一套响应类型，而不是直接复用 /web/gb 的：
//  1. **字段裁剪**：Web 的 GbDeviceItem/GbChannelItem 是管理面结构，带着
//     sip_password、sip_realm、sdp_ip、password 等排障/配置字段。移动端只做
//     看视频、控云台、查报警，把这些原样透出既增加带宽，也扩大敏感字段暴露面。
//  2. **契约隔离**：移动端接口契约（§10.12）只承诺移动端需要的字段。若直接复用
//     Web 结构，后端以后给 Web 加字段会「顺带」改变 App 的响应，属于隐性破坏。
//  3. **双保险脱敏**：即使某个上游 service 将来忘记对敏感字段脱敏，只要 App 的
//     DTO 里根本没有该字段，序列化结果就不可能带上它。
//
// 命名沿用项目约定：AppXxxResp 为 data 载荷，RAppXxx 为带 BaseResponse 的信封。
// ════════════════════════════════════════════════════════════════

// ── 设备 / 通道 ────────────────────────────────────────────────

// AppGbDeviceItem 移动端国标设备列表项。
//
// ★ 不含 sip_password / sip_realm / sip_transport / sdp_ip 等接入配置字段。
type AppGbDeviceItem struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	DeviceCode     string `json:"device_code"`
	GbDeviceID     string `json:"gb_device_id" example:"34020000001320000001"`
	RegionID       *int64 `json:"region_id"`
	RegionName     string `json:"region_name" example:"东门区域"`
	Location       string `json:"location"`
	Status         string `json:"status" example:"online"`
	RegisterStatus string `json:"register_status" example:"activated"`
	ChannelCount   int64  `json:"channel_count" example:"4"`
	Active         *bool  `json:"active"`
}

// AppPageGbDeviceResp 移动端国标设备分页载荷。
type AppPageGbDeviceResp struct {
	Items    []AppGbDeviceItem `json:"items"`
	Total    int64             `json:"total" example:"12"`
	Page     int               `json:"page" example:"1"`
	PageSize int               `json:"page_size" example:"20"`
}

// RAppPageGbDevice 移动端国标设备分页响应。
type RAppPageGbDevice struct {
	BaseResponse
	Data AppPageGbDeviceResp `json:"data"`
}

// AppGbChannelItem 移动端国标通道列表项。
//
// 设备名称/区域名称由 service 层回填，移动端列表不需要二次请求。
// ★ 不含设备侧上报的通道 password。
type AppGbChannelItem struct {
	ID              int64  `json:"id"`
	DeviceID        int64  `json:"device_id"`
	DeviceName      string `json:"device_name"`
	GbDeviceID      string `json:"gb_device_id"`
	GbChannelID     string `json:"gb_channel_id" example:"34020000001320000001"`
	Name            string `json:"name"`
	Status          string `json:"status" example:"ON"`
	ChannelType     int64  `json:"channel_type"`
	PtzType         int64  `json:"ptz_type"`
	HasAudio        *bool  `json:"has_audio"`
	EnableBroadcast *bool  `json:"enable_broadcast"`
	RegionName      string `json:"region_name"`
	GroupName       string `json:"group_name"`
}

// AppPageGbChannelResp 移动端国标通道分页载荷。
type AppPageGbChannelResp struct {
	Items    []AppGbChannelItem `json:"items"`
	Total    int64              `json:"total" example:"32"`
	Page     int                `json:"page" example:"1"`
	PageSize int                `json:"page_size" example:"20"`
}

// RAppPageGbChannel 移动端国标通道分页响应。
type RAppPageGbChannel struct {
	BaseResponse
	Data AppPageGbChannelResp `json:"data"`
}

// ── 实时播放 / 抓图 ────────────────────────────────────────────

// AppGbPlayResp 移动端实时点播结果。
//
// 保留三种协议的完整地址，由 App 按网络状况选择（弱网优先 HLS，局域网优先 WebRTC）。
// ★ 不返回 ssrc 与 media_server_id：前者只用于排障，后者属于部署拓扑信息。
type AppGbPlayResp struct {
	App      string `json:"app" example:"rtp"`
	Stream   string `json:"stream" example:"3B9ACA01"`
	HTTPBase string `json:"http_base" example:"http://10.0.0.5:8082"`
	WebRTC   string `json:"webrtc"`
	FLV      string `json:"flv"`
	HLS      string `json:"hls"`
	// StreamID 会话标识（SIP Call-ID），停止播放时回传。
	StreamID string `json:"stream_id"`
}

// RAppGbPlay 移动端实时点播响应。
type RAppGbPlay struct {
	BaseResponse
	Data AppGbPlayResp `json:"data"`
}

// AppGbSnapshotResp 移动端抓图结果。
type AppGbSnapshotResp struct {
	URL       string `json:"url" example:"http://localhost:8080/uploads/202610/2f1c.jpg"`
	FileName  string `json:"file_name" example:"2f1c.jpg"`
	FileSize  int64  `json:"file_size" example:"48213"`
	CaptureDT string `json:"capture_dt" example:"2026-10-21 10:00:00"`
}

// RAppGbSnapshot 移动端抓图响应。
type RAppGbSnapshot struct {
	BaseResponse
	Data AppGbSnapshotResp `json:"data"`
}

// ── 录像查询 / 回放 ────────────────────────────────────────────

// AppGbRecordItem 设备侧录像条目（移动端裁剪版）。
//
// 只保留时间轴与展示名称；file_path/address/secrecy 属于设备侧细节，
// 移动端不直接使用。
type AppGbRecordItem struct {
	ChannelID int64  `json:"channel_id"`
	Name      string `json:"name"`
	StartTime string `json:"start_time" example:"2026-01-01T08:00:00"`
	EndTime   string `json:"end_time" example:"2026-01-01T09:00:00"`
	// TimeLen 录像时长（秒）。
	TimeLen int64 `json:"time_len" example:"3600"`
}

// AppGbRecordQueryResp 移动端录像检索结果。
type AppGbRecordQueryResp struct {
	ChannelID int64 `json:"channel_id"`
	Total     int   `json:"total"`
	// Timeout=true 表示设备在等待窗口内没有回齐响应（items 可能为空）。
	Timeout bool              `json:"timeout"`
	Items   []AppGbRecordItem `json:"items"`
}

// RAppGbRecordQuery 移动端录像检索响应。
type RAppGbRecordQuery struct {
	BaseResponse
	Data AppGbRecordQueryResp `json:"data"`
}

// AppGbPlaybackResp 移动端回放会话。
//
// ★ 不含 ssrc / ws_flv / media_server_id：App 只需要播放地址与停止用的 stream_id。
type AppGbPlaybackResp struct {
	StreamID  string `json:"stream_id"`
	App       string `json:"app" example:"rtp"`
	Stream    string `json:"stream" example:"0200000001"`
	HTTPBase  string `json:"http_base" example:"http://10.0.0.5:8082"`
	WebRTC    string `json:"webrtc"`
	FLV       string `json:"flv"`
	HLS       string `json:"hls"`
	ChannelID int64  `json:"channel_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// RAppGbPlayback 移动端回放会话响应。
type RAppGbPlayback struct {
	BaseResponse
	Data AppGbPlaybackResp `json:"data"`
}

// ── 语音对讲 ──────────────────────────────────────────────────

// AppGbVoiceSessionResp 移动端语音对讲会话。
//
// 移动端只需要「会话标识 + 回推地址参数 + 是否接入 Agent」；
// type 恒为 talk，故不返回。ssrc 供 App 侧把音频推到 ZLM 的 RTP 端口。
type AppGbVoiceSessionResp struct {
	SessionID    string `json:"session_id"`
	Mode         string `json:"mode" example:"half_duplex"`
	AgentEnabled bool   `json:"agent_enabled" example:"false"`
	SSRC         string `json:"ssrc" example:"0342000002"`
	AudioCodec   string `json:"audio_codec" example:"G.711A"`
	StartDT      string `json:"start_dt" example:"2026-10-21 10:00:00"`
}

// RAppGbVoiceSession 移动端语音对讲响应。
type RAppGbVoiceSession struct {
	BaseResponse
	Data AppGbVoiceSessionResp `json:"data"`
}

// ── 国标报警 ──────────────────────────────────────────────────

// AppGbAlarmItem 移动端国标报警列表项。
//
// 保留现场处置所需字段（描述、时间、抓图）；省略设备/通道国标编码、
// 处理备注、经纬度等移动端列表不展示的字段。
type AppGbAlarmItem struct {
	ID               int64      `json:"id"`
	DeviceID         *int64     `json:"device_id"`
	ChannelID        *int64     `json:"channel_id"`
	DeviceName       string     `json:"device_name"`
	ChannelName      string     `json:"channel_name"`
	AlarmPriority    string     `json:"alarm_priority" example:"1"`
	AlarmType        string     `json:"alarm_type"`
	AlarmTime        *time.Time `json:"alarm_time"`
	AlarmDescription string     `json:"alarm_description"`
	SnapPath         string     `json:"snap_path"`
	Handled          *bool      `json:"handled"`
}

// AppPageGbAlarmResp 移动端国标报警分页载荷。
type AppPageGbAlarmResp struct {
	Items    []AppGbAlarmItem `json:"items"`
	Total    int64            `json:"total" example:"42"`
	Page     int              `json:"page" example:"1"`
	PageSize int              `json:"page_size" example:"20"`
}

// RAppPageGbAlarm 移动端国标报警分页响应。
type RAppPageGbAlarm struct {
	BaseResponse
	Data AppPageGbAlarmResp `json:"data"`
}
