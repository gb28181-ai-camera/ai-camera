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
