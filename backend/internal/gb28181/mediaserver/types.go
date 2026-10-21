package mediaserver

// 本文件定义 ZLM API 的请求/响应结构，与 client.go 的通用传输逻辑分离，
// 便于 Wave 2 的播放编排、云端录像、语音回推直接复用类型。

// MediaTrack 一条音视频轨道的描述（getMediaList / on_stream_changed 中出现）。
type MediaTrack struct {
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

// OriginSock 流的来源 socket 信息。
//
// 单端口收流模式下 originSock.local_port 恒为 ZLM 的 rtp_proxy.port(10000)，
// identifier 即 SSRC 的大写十六进制 —— 排查「流有没有进来」时非常有用的证据。
type OriginSock struct {
	Identifier string `json:"identifier"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	PeerIP     string `json:"peer_ip"`
	PeerPort   int    `json:"peer_port"`
}

// MediaInfo getMediaList / getMediaInfo 返回的流信息。
type MediaInfo struct {
	App           string       `json:"app"`
	Stream        string       `json:"stream"`
	Schema        string       `json:"schema"`
	Vhost         string       `json:"vhost"`
	OriginType    int          `json:"originType"`
	OriginTypeStr string       `json:"originTypeStr"`
	OriginURL     string       `json:"originUrl"`
	ReaderCount   int          `json:"readerCount"`
	TotalReader   int          `json:"totalReaderCount"`
	AliveSecond   int64        `json:"aliveSecond"`
	BytesSpeed    int64        `json:"bytesSpeed"`
	CreateStamp   int64        `json:"createStamp"`
	OriginSock    *OriginSock  `json:"originSock"`
	Tracks        []MediaTrack `json:"tracks"`
}

// RtpServer listRtpServer 返回的 RTP server 条目。
type RtpServer struct {
	Port        int    `json:"port"`
	StreamID    string `json:"stream_id"`
	SSRC        string `json:"ssrc"`
	TCPMode     int    `json:"tcp_mode"`
	ReUsePort   bool   `json:"re_use_port"`
	TotalReader int    `json:"total_reader_count"`
}

// RtpInfo getRtpInfo 返回的收流统计。
type RtpInfo struct {
	Exist     bool   `json:"exist"`
	PeerIP    string `json:"peer_ip"`
	PeerPort  int    `json:"peer_port"`
	LocalIP   string `json:"local_ip"`
	LocalPort int    `json:"local_port"`
}

// OpenRtpServerParam openRtpServer 参数。
//
// ⚠️ 本项目**禁止使用**该 API（仅保留能力对齐）：deploy/zlm/config.ini 刻意
// 未配置 rtp_proxy.port_range，openRtpServer(port=0) 会从 ZLM 默认端口段里
// 挑一个宿主机没发布的端口，表现为「接口成功但永远收不到流」。
// 单端口模式应直接把 rtp_proxy.port 写进 SDP，让 ZLM 按 SSRC 自动建流。
type OpenRtpServerParam struct {
	StreamID  string
	Port      int
	TCPMode   int
	ReusePort bool
	SSRC      uint32
	OnlyAudio bool
}

// OpenRtpServerResult openRtpServer 的返回。
type OpenRtpServerResult struct {
	Port     int    `json:"port"`
	StreamID string `json:"stream_id"`
}

// SendRTPParam startSendRtp 参数（语音回推 / 级联推流）。
type SendRTPParam struct {
	Vhost        string
	App          string
	Stream       string
	SSRC         uint32 // 目标（接收端）SSRC，不是源流 SSRC
	DstURL       string
	DstPort      int
	IsUDP        bool
	SrcPort      int
	PT           int
	UsePS        bool
	RecvStreamID string
	OnlyAudio    bool
}

// SendRTPResult startSendRtp 的返回。
type SendRTPResult struct {
	LocalPort int `json:"local_port"`
}

// AddStreamProxyParam addStreamProxy 参数（拉流代理）。
type AddStreamProxyParam struct {
	Vhost       string
	App         string
	Stream      string
	URL         string
	EnableAudio bool
	EnableMP4   bool
	RTPType     string // 0=tcp, 1=udp（ZLM 约定，字符串透传）
	TimeoutSec  int
	RetryCount  int
}

// AddFFmpegSourceParam addFFmpegSource 参数（ffmpeg 拉流）。
type AddFFmpegSourceParam struct {
	SrcURL       string
	DstURL       string
	TimeoutMS    int
	EnableMP4    bool
	FFmpegCmdKey string
}

// StreamProxyResult addStreamProxy / addFFmpegSource 的返回。
type StreamProxyResult struct {
	Key string `json:"key"`
}

// ThreadLoad getThreadsLoad 返回的线程负载条目。
type ThreadLoad struct {
	Name string `json:"name"`
	Load int    `json:"load"`
}

// PlayURLs 一个流的全部对外播放地址。
//
// 字段命名与前端契约保持一致：flv 是 **http-flv**（mpegts.js 可播），
// ws_flv 是 WebSocket-FLV，hls 是 m3u8 地址。
type PlayURLs struct {
	WebRTC string `json:"webrtc"` // WHEP 风格：/index/api/webrtc?...&type=play
	FLV    string `json:"flv"`    // http-flv
	HLS    string `json:"hls"`    // HLS m3u8
	WSFLV  string `json:"ws_flv"` // WebSocket-FLV
	RTSP   string `json:"rtsp"`
	RTMP   string `json:"rtmp"`
}
