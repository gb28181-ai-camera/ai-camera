// Package hook 接收 ZLMediaKit 的 10 个 HTTP 回调（hook）。
//
// 设计要点：
//  1. 只做「校验 secret → 解析 JSON → 交给 Sink」，业务逻辑全部下沉到 Sink，
//     避免 hook 包反向依赖 service 层，也便于 Wave 2 替换/组合实现。
//  2. ZLM 是外部进程，没有 JWT。这些路由**绝对不能**挂 auth/casbin 中间件，
//     鉴权只依赖 ZLM 回调 URL 上的 secret 查询参数。
//  3. ZLM 只关心 HTTP 200 + JSON code：code=0 表示成功，code!=0 表示拒绝。
//     因此即使内部处理失败，非鉴权类回调也应返回 0，避免 ZLM 反复重试。
package hook

import (
	"context"
	"time"
)

// DefaultVhost ZLM 默认虚拟主机。
const DefaultVhost = "__defaultVhost__"

// Sink 是 hook 事件的业务出口。
//
// 返回值语义：
//   - OnPublish / OnPlay 返回 false 表示拒绝（ZLM 会断开推流/播放），
//     error 非 nil 时按**拒绝**处理（fail-closed）。
//   - 其余方法返回 error 只用于日志记录，不影响给 ZLM 的响应。
type Sink interface {
	OnServerKeepalive(ctx context.Context, event *MediaServerEvent) error
	OnServerStarted(ctx context.Context, event *MediaServerEvent) error
	OnPublish(ctx context.Context, event *PublishEvent) (bool, error)
	OnPlay(ctx context.Context, event *PlayEvent) (bool, error)
	OnStreamChanged(ctx context.Context, event *StreamChangedEvent) error
	OnStreamNoneReader(ctx context.Context, event *StreamEvent) error
	OnStreamNotFound(ctx context.Context, event *StreamEvent) error
	OnSendRTPStopped(ctx context.Context, event *SendRTPStoppedEvent) error
	OnRTPServerTimeout(ctx context.Context, event *StreamEvent) error
	OnRecordMP4(ctx context.Context, event *RecordMP4Event) error
}

// MediaServerEvent on_server_keepalive / on_server_started 的事件。
//
// Data 是 keepalive 的 data 子对象；Config 是 started 的完整配置表
// （ZLM 的配置项是扁平 key，如 general.mediaServerId）。
type MediaServerEvent struct {
	MediaServerID string
	IP            string
	Data          map[string]any
	Config        map[string]any
	ReceivedAt    time.Time
}

// PublishEvent on_publish 事件（rtsp/rtmp/rtp 推流鉴权）。
type PublishEvent struct {
	MediaServerID string `json:"mediaServerId"`
	ID            string `json:"id"`
	App           string `json:"app"`
	Stream        string `json:"stream"`
	IP            string `json:"ip"`
	Params        string `json:"params"`
	Port          int    `json:"port"`
	Schema        string `json:"schema"`
	Vhost         string `json:"vhost"`
}

// PlayEvent on_play 事件（播放鉴权）。
type PlayEvent struct {
	MediaServerID string `json:"mediaServerId"`
	ID            string `json:"id"`
	App           string `json:"app"`
	Stream        string `json:"stream"`
	IP            string `json:"ip"`
	Params        string `json:"params"`
	Port          int    `json:"port"`
	Schema        string `json:"schema"`
	Vhost         string `json:"vhost"`
}

// Track on_stream_changed 中的音视频轨道。
type Track struct {
	CodecID     int     `json:"codec_id"`
	CodecIDName string  `json:"codec_id_name"`
	CodecType   int     `json:"codec_type"`
	Ready       bool    `json:"ready"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FPS         float64 `json:"fps"`
	SampleRate  int     `json:"sample_rate"`
	Channels    int     `json:"channels"`
}

// OriginSock on_stream_changed 中的来源 socket 信息。
type OriginSock struct {
	Identifier string `json:"identifier"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	PeerIP     string `json:"peer_ip"`
	PeerPort   int    `json:"peer_port"`
}

// StreamChangedEvent on_stream_changed 事件（流注册/注销）。
type StreamChangedEvent struct {
	MediaServerID    string      `json:"mediaServerId"`
	Regist           bool        `json:"regist"`
	App              string      `json:"app"`
	Stream           string      `json:"stream"`
	CallID           string      `json:"callId"`
	TotalReaderCount int         `json:"totalReaderCount"`
	Schema           string      `json:"schema"`
	OriginType       int         `json:"originType"`
	OriginTypeStr    string      `json:"originTypeStr"`
	OriginURL        string      `json:"originUrl"`
	OriginSock       *OriginSock `json:"originSock"`
	CreateStamp      int64       `json:"createStamp"`
	AliveSecond      int64       `json:"aliveSecond"`
	BytesSpeed       int64       `json:"bytesSpeed"`
	Tracks           []Track     `json:"tracks"`
	Vhost            string      `json:"vhost"`
	Params           string      `json:"params"`
}

// StreamEvent 通用流事件：on_stream_none_reader / on_stream_not_found /
// on_rtp_server_timeout 共用。
//
// on_rtp_server_timeout 的负载字段名不同（stream_id/local_port/...），
// Handler 会做一次转换，业务层只需处理统一结构。
type StreamEvent struct {
	MediaServerID string `json:"mediaServerId"`
	ID            string `json:"id"`
	App           string `json:"app"`
	Stream        string `json:"stream"`
	Schema        string `json:"schema"`
	Vhost         string `json:"vhost"`
	IP            string `json:"ip"`
	Params        string `json:"params"`
	Port          int    `json:"port"`

	// 以下字段仅 on_rtp_server_timeout 使用。
	StreamID  string `json:"stream_id"`
	SSRC      string `json:"ssrc"`
	LocalPort int    `json:"local_port"`
	TCPMode   int    `json:"tcpMode"`
	ReUsePort bool   `json:"re_use_port"`
}

// SendRTPStoppedEvent on_send_rtp_stopped 事件（语音回推被动结束）。
type SendRTPStoppedEvent struct {
	MediaServerID string `json:"mediaServerId"`
	App           string `json:"app"`
	Stream        string `json:"stream"`
}

// RecordMP4Event on_record_mp4 事件（录像落盘完成）。
type RecordMP4Event struct {
	MediaServerID string  `json:"mediaServerId"`
	App           string  `json:"app"`
	Stream        string  `json:"stream"`
	FileName      string  `json:"file_name"`
	FilePath      string  `json:"file_path"`
	FileSize      int64   `json:"file_size"`
	Folder        string  `json:"folder"`
	URL           string  `json:"url"`
	Vhost         string  `json:"vhost"`
	StartTime     int64   `json:"start_time"`
	TimeLen       float64 `json:"time_len"`
	Params        string  `json:"params"`
}

// Result 通用 hook 响应：{code, msg}。
//
// ZLM 约定的成功值是 code=0；拒绝（on_publish/on_play）用 code=-1。
type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// NoneReaderResult on_stream_none_reader 的响应，额外携带 close 指令。
//
// 本期固定 close=false：流是否关闭由后续的点播编排（Redis 会话表）决定，
// hook 层不擅自关流。
type NoneReaderResult struct {
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
	Close bool   `json:"close"`
}

// SuccessResult 返回 code=0 的标准成功响应。
func SuccessResult() Result { return Result{Code: 0, Msg: "success"} }

// FailResult 返回 code=-1 的拒绝响应。
func FailResult(msg string) Result { return Result{Code: -1, Msg: msg} }
