package response

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）—— 实时播放 / 云台 / 语音响应类型
//
// 字段严格按 docs/开发文档/后端开发/00-接口契约.md §15.1 / §15.2 / §15.4。
// 命名遵循项目规范：
//   - RGbPlay / RGbPlayStatus / RGbSnapshot / RGbVoiceSession → 单条/复合详情
//   - RGbPresetList → 简单列表
// 禁止使用 response.PageResp（本组接口没有分页）。
// ════════════════════════════════════════════════════════════════

// GbPlayResp 实时点播结果（契约 §15.1）。
//
// ★ 前端二选一即可：
//   - 用 WebRTC / FLV / HLS 整串地址最省事；
//   - 要给 mpegts.js 自己拼，则用 HTTPBase + App + Stream。
//
// 两个都返回，避免前端为了拿 HTTPBase 再查一次媒体节点列表。
type GbPlayResp struct {
	// App 恒为 "rtp"。
	App string `json:"app" example:"rtp"`
	// Stream = %08X(ssrc) 大写（设计 §2.2 实测：两种端口模式都是）。
	Stream string `json:"stream" example:"3B9ACA01"`
	// SSRC 10 位十进制，供排障。
	SSRC string `json:"ssrc" example:"0342000001"`
	// HTTPBase ZLM 的 HTTP 基址，如 http://10.0.0.5:8082。
	HTTPBase string `json:"http_base" example:"http://10.0.0.5:8082"`
	// WebRTC 完整 WHEP URL。
	WebRTC string `json:"webrtc" example:"http://10.0.0.5:8082/index/api/webrtc?app=rtp&stream=3B9ACA01&type=play"`
	// FLV 完整 HTTP-FLV URL。
	FLV string `json:"flv" example:"http://10.0.0.5:8082/rtp/3B9ACA01.live.flv"`
	// HLS 完整 HLS URL。
	HLS string `json:"hls" example:"http://10.0.0.5:8082/rtp/3B9ACA01/hls.m3u8"`
	// MediaServerID ZLM 的 mediaServerId。
	MediaServerID string `json:"media_server_id" example:"zlm-1"`
	// StreamID 会话标识（SIP Call-ID），停止播放时回传。
	StreamID string `json:"stream_id" example:"34020000002000000001-1750000000000-abcd"`
	// Codec 实际协商到的视频编码，如 H264 / H265。
	//
	// 来自 ZLM getMediaInfo 的 tracks.codec_id_name，用于前端展示与排障；
	// 查询失败时留空（不影响播放本身），后端只记 WARN。
	Codec string `json:"codec" example:"H264"`
}

// RGbPlay 实时点播响应。
type RGbPlay struct {
	BaseResponse
	Data GbPlayResp `json:"data"`
}

// GbPlayStatusResp 点播状态。
//
// Playing 以平台注册表为准，Ready 是 ZLM 的实时判断：
// 二者不一致时（Playing=true、Ready=false）通常意味着设备停了推流但 BYE 没发出来。
type GbPlayStatusResp struct {
	ChannelID int64  `json:"channel_id"`
	Playing   bool   `json:"playing"`
	Ready     bool   `json:"ready"`
	App       string `json:"app" example:"rtp"`
	Stream    string `json:"stream" example:"3B9ACA01"`
	SSRC      string `json:"ssrc" example:"0342000001"`
	HTTPBase  string `json:"http_base" example:"http://10.0.0.5:8082"`
	WebRTC    string `json:"webrtc"`
	FLV       string `json:"flv"`
	HLS       string `json:"hls"`

	MediaServerID string `json:"media_server_id" example:"zlm-1"`
	StreamID      string `json:"stream_id"`
	StartDT       string `json:"start_dt" example:"2026-10-21 10:00:00"`
}

// RGbPlayStatus 点播状态响应。
type RGbPlayStatus struct {
	BaseResponse
	Data GbPlayStatusResp `json:"data"`
}

// GbSnapshotResp 抓图结果。
//
// URL 指向本服务的静态上传目录（与 /web/upload 使用同一套存储约定），
// 前端可直接用于 <img src>。FileName 是落盘后的随机文件名，不是设备侧名称。
type GbSnapshotResp struct {
	URL           string `json:"url" example:"http://localhost:8080/uploads/202610/2f1c.jpg"`
	FileName      string `json:"file_name" example:"2f1c.jpg"`
	FileSize      int64  `json:"file_size" example:"48213"`
	Stream        string `json:"stream" example:"3B9ACA01"`
	MediaServerID string `json:"media_server_id" example:"zlm-1"`
	CaptureDT     string `json:"capture_dt" example:"2026-10-21 10:00:00"`
}

// RGbSnapshot 抓图响应。
type RGbSnapshot struct {
	BaseResponse
	Data GbSnapshotResp `json:"data"`
}

// GbPresetItem 云台预置位。
//
// 本期平台侧不落库：列表来自本进程缓存（AddPreset/DeletePreset 维护）。
// 设备侧的真实预置位以设备为准，重启后端后缓存为空。
type GbPresetItem struct {
	PresetID int    `json:"preset_id" example:"1"`
	Name     string `json:"name" example:"大门"`
}

// RGbPresetList 预置位列表响应。
type RGbPresetList struct {
	BaseResponse
	Data []GbPresetItem `json:"data"`
}

// ════════════════════════════════════════════════════════════════
// GB/T 28181-2022 云台扩展查询应答
//
// 字段严格对应 internal/gb28181/protocol/manscdp/types_2022.go 的 2022 类型：
//   - Pan/Tilt/Zoom 用 *float64：0 是合法值，必须区分「设备未上报」与「上报了 0」；
//   - 标准中 minOccurs="0" 的字段（视场角/可视距离/看守位块）缺失时省略；
//   - 容量单位沿用标准：MB。
// 未在 manscdp 中确认的字段一律不在这里新增，避免前端展示「看起来有值、
// 实际设备从未上报」的假数据。
// ════════════════════════════════════════════════════════════════

// GbPtzPositionResp PTZ 精准位置（A.2.6.15）。
type GbPtzPositionResp struct {
	// Pan 水平角度（度）。设备未上报时省略（与「上报了 0」区分）。
	Pan *float64 `json:"pan,omitempty" example:"10.5"`
	// Tilt 垂直角度（度）。设备未上报时省略。
	Tilt *float64 `json:"tilt,omitempty" example:"20"`
	// Zoom 变焦倍数。设备未上报时省略。
	Zoom *float64 `json:"zoom,omitempty" example:"2.5"`
	// HorizontalFieldAngle 水平视场角（度）。★ manscdp 中标注「待真机确认」，
	// 设备不上报时省略。
	HorizontalFieldAngle *float64 `json:"horizontal_field_angle,omitempty" example:"60"`
	// VerticalFieldAngle 垂直视场角（度）。★ 待真机确认。
	VerticalFieldAngle *float64 `json:"vertical_field_angle,omitempty" example:"35"`
	// MaxViewDistance 可视距离（米）。★ 待真机确认。
	MaxViewDistance *float64 `json:"max_view_distance,omitempty" example:"100"`
}

// RGbPtzPosition PTZ 精准位置响应。
type RGbPtzPosition struct {
	BaseResponse
	Data GbPtzPositionResp `json:"data"`
}

// GbCruiseTrackItem 一条巡航轨迹的摘要（A.2.6.13）。
type GbCruiseTrackItem struct {
	// Number 轨迹编号，从 0 开始。
	Number int `json:"number" example:"0"`
	// Name 轨迹名称。
	Name string `json:"name" example:"厂区巡航"`
}

// RGbCruiseTrackList 巡航轨迹列表响应。
type RGbCruiseTrackList struct {
	BaseResponse
	Data []GbCruiseTrackItem `json:"data"`
}

// GbCruisePointItem 一个巡航点（A.2.6.14）。
type GbCruisePointItem struct {
	// PresetIndex 预置位编号。
	PresetIndex int `json:"preset_index" example:"1"`
	// StayTime 停留时间（秒）。
	StayTime int `json:"stay_time" example:"10"`
	// Speed 云台速度 1–15。
	Speed int `json:"speed" example:"5"`
}

// GbCruiseTrackDetailResp 单条巡航轨迹明细（A.2.6.14）。
type GbCruiseTrackDetailResp struct {
	Number int                 `json:"number" example:"0"`
	Name   string              `json:"name" example:"厂区巡航"`
	Points []GbCruisePointItem `json:"points"`
}

// RGbCruiseTrackDetail 单条巡航轨迹响应。
type RGbCruiseTrackDetail struct {
	BaseResponse
	Data GbCruiseTrackDetailResp `json:"data"`
}

// GbHomePositionResp 看守位（A.2.6.12）。
//
// Supported 是平台派生字段：标准中 HomePosition 块 minOccurs="0"，
// 设备不支持时整块省略。若不单独暴露该标志，前端会把「块缺失」
// 误显示成「已关闭（Enabled=0）」，这两种语义完全不同。
type GbHomePositionResp struct {
	Supported bool `json:"supported" example:"true"`
	// Enabled 0 关闭 / 1 开启。
	Enabled int `json:"enabled" example:"1"`
	// ResetTime 自动归位时间间隔（秒）。
	ResetTime int `json:"reset_time" example:"300"`
	// PresetIndex 归位调用的预置位编号 0–255。
	PresetIndex int `json:"preset_index" example:"1"`
}

// RGbHomePosition 看守位响应。
type RGbHomePosition struct {
	BaseResponse
	Data GbHomePositionResp `json:"data"`
}

// GbSDCardItem 单张存储卡（A.2.6.16）。
type GbSDCardItem struct {
	// Capacity 总空间（MB）。
	Capacity int64 `json:"capacity" example:"32768"`
	// FreeSpace 剩余空间（MB）。
	FreeSpace int64 `json:"free_space" example:"16384"`
	// Status 卡状态。★ 标准未明确取值含义，原样透传。
	Status int `json:"status" example:"0"`
}

// GbSDCardStatusResp 存储卡状态（A.2.6.16）。
type GbSDCardStatusResp struct {
	// Num 卡片数量。
	Num int `json:"num" example:"1"`
	// Status 整体状态。★ 取值含义待真机确认，原样透传。
	Status int `json:"status" example:"0"`
	// FormatProgress 格式化进度 0–100；0 表示未在格式化。
	FormatProgress int            `json:"format_progress" example:"0"`
	Cards          []GbSDCardItem `json:"cards"`
}

// RGbSDCardStatus 存储卡状态响应。
type RGbSDCardStatus struct {
	BaseResponse
	Data GbSDCardStatusResp `json:"data"`
}

// GbVoiceSessionResp 语音会话（契约 §15.4）。
type GbVoiceSessionResp struct {
	SessionID string `json:"session_id"`
	// Type broadcast | talk。
	Type string `json:"type" example:"talk"`
	// Mode 半双工/全双工。一期默认 half_duplex（设计 §8.1.1：全双工必然自激）。
	Mode string `json:"mode" example:"half_duplex"`
	// AgentEnabled 是否接入 Pipecat 语音 Agent；控制面不可达时降级为 false。
	AgentEnabled bool `json:"agent_enabled" example:"false"`
	// SSRC 回推音频的 SSRC（10 位十进制）。
	SSRC string `json:"ssrc" example:"0342000002"`
	// AudioCodec 固定 G.711A。
	AudioCodec string `json:"audio_codec" example:"G.711A"`
	StartDT    string `json:"start_dt" example:"2026-10-21 10:00:00"`
}

// RGbVoiceSession 语音会话响应。
type RGbVoiceSession struct {
	BaseResponse
	Data GbVoiceSessionResp `json:"data"`
}
