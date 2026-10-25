package response

// ════════════════════════════════════════════════════════════════
// 国标视频（GB/T 28181）—— Wave 3 运维增强响应类型
//
// 覆盖：媒体节点主机指标 / 流会话 / 平台信息 / 日志检索 /
// 流播放地址 / 推流导入 / 云端录像合并任务 / 免登录分享。
//
// 命名与全局规范保持一致：
//   - RPageXxx → Data PageXxxResp{Items, Total, Page, PageSize}
//   - RXxxList  → Data []XxxItem
//   - RXxx      → Data XxxResp
//
// ★ 安全：分享令牌的 TokenHash 永不返回；唯一一次明文只出现在
// GbShareTokenCreatedResp.Token 中。
// ════════════════════════════════════════════════════════════════

import "time"

// ── 媒体节点主机指标 ──────────────────────────────────────────

// GbHostMetricsPayload 后端所在主机的资源快照。
//
// 采集的是「本服务所在主机」而不是 ZLM 容器内部：WVP 同样用 oshi
// 采本机，ZLM 没有暴露 CPU/内存的 REST API。多副本部署时各副本只
// 能看到自己所在节点的指标，这是已知且可接受的限制。
type GbHostMetricsPayload struct {
	CPUPercent float64 `json:"cpu_percent" example:"12.5"`
	CPUCores   int     `json:"cpu_cores" example:"8"`
	Load1      float64 `json:"load1" example:"0.42"`
	Load5      float64 `json:"load5" example:"0.51"`
	Load15     float64 `json:"load15" example:"0.48"`

	MemUsed    uint64  `json:"mem_used" example:"4294967296"`
	MemTotal   uint64  `json:"mem_total" example:"17179869184"`
	MemPercent float64 `json:"mem_percent" example:"25"`

	DiskUsed    uint64  `json:"disk_used" example:"107374182400"`
	DiskTotal   uint64  `json:"disk_total" example:"536870912000"`
	DiskPercent float64 `json:"disk_percent" example:"20"`

	NetRxBytes uint64  `json:"net_rx_bytes" example:"123456789"`
	NetTxBytes uint64  `json:"net_tx_bytes" example:"987654321"`
	NetRxRate  float64 `json:"net_rx_rate" example:"1024"`
	NetTxRate  float64 `json:"net_tx_rate" example:"2048"`

	// OSInfo 形如 "linux/amd64" 或 "windows/amd64"。
	OSInfo string `json:"os_info" example:"linux/amd64"`
}

// GbMediaMetricsResp 媒体节点指标（主机资源 + 节点状态 + 流数）。
type GbMediaMetricsResp struct {
	MediaServerID   string               `json:"media_server_id" example:"zlm-1"`
	MediaServerName string               `json:"media_server_name" example:"主节点"`
	Online          bool                 `json:"online"`
	StreamCount     int                  `json:"stream_count" example:"12"`
	Host            GbHostMetricsPayload `json:"host"`
	SampledDT       time.Time            `json:"sampled_dt"`
	Message         string               `json:"message"`
}

// RGbMediaMetrics 媒体节点当前指标响应。
type RGbMediaMetrics struct {
	BaseResponse
	Data GbMediaMetricsResp `json:"data"`
}

// GbMediaMetricPoint 一个采样点。
type GbMediaMetricPoint struct {
	TS          time.Time `json:"ts"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	DiskPercent float64   `json:"disk_percent"`
	NetRxRate   float64   `json:"net_rx_rate"`
	NetTxRate   float64   `json:"net_tx_rate"`
	Load1       float64   `json:"load1"`
	StreamCount int       `json:"stream_count"`
	Online      bool      `json:"online"`
}

// GbMediaMetricsHistoryResp 指标历史时间序列。
type GbMediaMetricsHistoryResp struct {
	MediaServerID string               `json:"media_server_id"`
	Range         string               `json:"range" example:"1h"`
	IntervalSec   int                  `json:"interval_sec" example:"5"`
	Items         []GbMediaMetricPoint `json:"items"`
}

// RGbMediaMetricsHistory 指标历史响应。
type RGbMediaMetricsHistory struct {
	BaseResponse
	Data GbMediaMetricsHistoryResp `json:"data"`
}

// ── 流会话 ────────────────────────────────────────────────────

// GbMediaStreamItem 一条 ZLM 流会话。
type GbMediaStreamItem struct {
	App            string  `json:"app" example:"rtp"`
	Stream         string  `json:"stream" example:"0200000001"`
	Schema         string  `json:"schema" example:"rtsp"`
	Vhost          string  `json:"vhost"`
	OriginType     string  `json:"origin_type"`
	OriginURL      string  `json:"origin_url"`
	ReaderCount    int     `json:"reader_count" example:"2"`
	AliveSecond    int64   `json:"alive_second" example:"320"`
	BytesSpeed     int64   `json:"bytes_speed" example:"524288"`
	Codec          string  `json:"codec" example:"H264"`
	Width          int     `json:"width" example:"1920"`
	Height         int     `json:"height" example:"1080"`
	FPS            float64 `json:"fps" example:"25"`
	HasAudio       bool    `json:"has_audio"`
	AudioCodec     string  `json:"audio_codec"`
	CreateStamp    int64   `json:"create_stamp" example:"1767225600"`
	OriginPeerIP   string  `json:"origin_peer_ip"`
	OriginPeerPort int     `json:"origin_peer_port"`
}

// GbMediaStreamListResp 媒体节点当前流会话。
type GbMediaStreamListResp struct {
	MediaServerID string              `json:"media_server_id"`
	Online        bool                `json:"online"`
	Total         int                 `json:"total"`
	Items         []GbMediaStreamItem `json:"items"`
	Message       string              `json:"message"`
}

// RGbMediaStreamList 流会话列表响应。
type RGbMediaStreamList struct {
	BaseResponse
	Data GbMediaStreamListResp `json:"data"`
}

// ── 平台信息 ──────────────────────────────────────────────────

// GbSipStatus SIP 信令服务状态。
type GbSipStatus struct {
	Enabled       bool   `json:"enabled"`
	Embedded      bool   `json:"embedded"`
	Running       bool   `json:"running" example:"true"`
	ListenAddr    string `json:"listen_addr" example:"0.0.0.0:5060"`
	AdvertiseAddr string `json:"advertise_addr" example:"10.0.0.5:5060"`
	Domain        string `json:"domain" example:"3402000000"`
	Transport     string `json:"transport" example:"UDP"`
}

// GbDefaultMediaServerSummary 默认媒体节点摘要。
type GbDefaultMediaServerSummary struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	ServerID        string     `json:"server_id"`
	IP              string     `json:"ip"`
	HTTPPort        int64      `json:"http_port"`
	StreamIP        string     `json:"stream_ip"`
	Online          bool       `json:"online"`
	LastKeepaliveDT *time.Time `json:"last_keepalive_dt"`
}

// GbDependencyStatus 依赖组件状态（DB / Redis）。
type GbDependencyStatus struct {
	Configured bool    `json:"configured"`
	Online     bool    `json:"online"`
	Address    string  `json:"address"`
	LatencyMS  float64 `json:"latency_ms" example:"1.2"`
	Message    string  `json:"message"`
}

// GbPlatformInfoResp 平台运行信息。
type GbPlatformInfoResp struct {
	Version            string                       `json:"version" example:"v0.4.0"`
	BuildVersion       string                       `json:"build_version"`
	BuildTime          string                       `json:"build_time"`
	GoVersion          string                       `json:"go_version" example:"go1.26.0"`
	OS                 string                       `json:"os" example:"linux"`
	Arch               string                       `json:"arch" example:"amd64"`
	Hostname           string                       `json:"hostname"`
	StartedDT          time.Time                    `json:"started_dt"`
	UptimeSeconds      int64                        `json:"uptime_seconds" example:"86400"`
	SIP                GbSipStatus                  `json:"sip"`
	DefaultMediaServer *GbDefaultMediaServerSummary `json:"default_media_server"`
	Database           GbDependencyStatus           `json:"database"`
	Redis              GbDependencyStatus           `json:"redis"`
}

// RGbPlatformInfo 平台信息响应。
type RGbPlatformInfo struct {
	BaseResponse
	Data GbPlatformInfoResp `json:"data"`
}

// ── 日志 ──────────────────────────────────────────────────────

// GbLogItem 一条内存日志。
type GbLogItem struct {
	ID      int64                  `json:"id"`
	Time    time.Time              `json:"time"`
	Level   string                 `json:"level" example:"info"`
	Logger  string                 `json:"logger,omitempty"`
	Message string                 `json:"message"`
	Caller  string                 `json:"caller,omitempty"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// PageGbLogResp 日志分页载荷。
type PageGbLogResp struct {
	Items    []GbLogItem `json:"items"`
	Total    int64       `json:"total" example:"128"`
	Page     int         `json:"page" example:"1"`
	PageSize int         `json:"page_size" example:"50"`
}

// RPageGbLog 日志分页响应。
type RPageGbLog struct {
	BaseResponse
	Data PageGbLogResp `json:"data"`
}

// ── 流播放地址 ────────────────────────────────────────────────

// GbStreamPlayResp 拉流代理 / 推流的播放地址。
type GbStreamPlayResp struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	App           string   `json:"app" example:"proxy"`
	Stream        string   `json:"stream" example:"cam-1"`
	MediaServerID string   `json:"media_server_id"`
	WebRTC        string   `json:"webrtc"`
	FLV           string   `json:"flv"`
	HLS           string   `json:"hls"`
	WSFLV         string   `json:"ws_flv"`
	RTSP          string   `json:"rtsp"`
	RTMP          string   `json:"rtmp"`
	Protocols     []string `json:"protocols" example:"webrtc,flv,hls"`
}

// RGbStreamPlay 流播放地址响应。
type RGbStreamPlay struct {
	BaseResponse
	Data GbStreamPlayResp `json:"data"`
}

// ── 推流导入 ──────────────────────────────────────────────────

// GbStreamImportRowError 一行导入失败原因。
type GbStreamImportRowError struct {
	Row    int    `json:"row" example:"3"`
	Line   int    `json:"line" example:"4"`
	Name   string `json:"name"`
	App    string `json:"app"`
	Stream string `json:"stream"`
	Reason string `json:"reason"`
}

// GbStreamImportResultResp 推流导入结果（同步返回，不建任务表）。
type GbStreamImportResultResp struct {
	Total        int                      `json:"total" example:"10"`
	SuccessCount int                      `json:"success_count" example:"8"`
	FailedCount  int                      `json:"failed_count" example:"2"`
	Items        []GbStreamImportRowError `json:"errors"`
}

// RGbStreamImportResult 推流导入响应。
type RGbStreamImportResult struct {
	BaseResponse
	Data GbStreamImportResultResp `json:"data"`
}

// ── 云端录像合并任务 ──────────────────────────────────────────

// GbMergeTaskItem 合并任务列表项。
type GbMergeTaskItem struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status" example:"success"`
	RecordIDs   string     `json:"record_ids"`
	RecordCount int        `json:"record_count"`
	FileName    string     `json:"file_name"`
	FilePath    string     `json:"file_path"`
	FileSize    int64      `json:"file_size"`
	DurationSec int64      `json:"duration_sec"`
	ErrorMsg    string     `json:"error_msg"`
	StartedDT   *time.Time `json:"started_dt"`
	FinishedDT  *time.Time `json:"finished_dt"`
	CreatedBy   string     `json:"created_by"`
	CreatedDT   time.Time  `json:"created_dt"`
	UpdatedDT   time.Time  `json:"updated_dt"`
}

// PageGbMergeTaskResp 合并任务分页载荷。
type PageGbMergeTaskResp struct {
	Items    []GbMergeTaskItem `json:"items"`
	Total    int64             `json:"total" example:"3"`
	Page     int               `json:"page" example:"1"`
	PageSize int               `json:"page_size" example:"20"`
}

// RPageGbMergeTask 合并任务分页响应。
type RPageGbMergeTask struct {
	BaseResponse
	Data PageGbMergeTaskResp `json:"data"`
}

// RGbMergeTask 合并任务详情响应。
type RGbMergeTask struct {
	BaseResponse
	Data GbMergeTaskItem `json:"data"`
}

// ── 免登录分享 ────────────────────────────────────────────────

// GbShareTokenItem 分享令牌列表项（不含哈希与明文）。
type GbShareTokenItem struct {
	ID          int64      `json:"id"`
	ChannelID   int64      `json:"channel_id"`
	TokenPrefix string     `json:"token_prefix"`
	ExpiresDT   time.Time  `json:"expires_dt"`
	Revoked     bool       `json:"revoked"`
	UseCount    int64      `json:"use_count"`
	LastUsedDT  *time.Time `json:"last_used_dt"`
	Remark      string     `json:"remark"`
	CreatedBy   string     `json:"created_by"`
	CreatedDT   time.Time  `json:"created_dt"`
}

// RGbShareTokenList 分享令牌简单列表响应（只含未过期未吊销）。
type RGbShareTokenList struct {
	BaseResponse
	Data []GbShareTokenItem `json:"data"`
}

// GbShareTokenCreatedResp 新建令牌响应。
//
// Token 是唯一一次出现的明文；客户端必须立即展示/复制，服务端无法再次还原。
type GbShareTokenCreatedResp struct {
	GbShareTokenItem
	Token string `json:"token"`
}

// RGbShareTokenCreated 新建令牌响应。
type RGbShareTokenCreated struct {
	BaseResponse
	Data GbShareTokenCreatedResp `json:"data"`
}

// GbShareInfoResp 匿名访问的分享信息。
type GbShareInfoResp struct {
	ChannelID    int64     `json:"channel_id"`
	ChannelName  string    `json:"channel_name"`
	GbChannelID  string    `json:"gb_channel_id"`
	DeviceName   string    `json:"device_name"`
	ExpiresDT    time.Time `json:"expires_dt"`
	Protocols    []string  `json:"protocols" example:"webrtc,flv,hls"`
	PlayEndpoint string    `json:"play_endpoint" example:"/public/gb/shares/<token>/play"`
}

// RGbShareInfo 匿名分享信息响应。
type RGbShareInfo struct {
	BaseResponse
	Data GbShareInfoResp `json:"data"`
}

// GbSharePlayResp 匿名播放结果（短时有效，不落库）。
type GbSharePlayResp struct {
	ChannelID    int64  `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	App          string `json:"app" example:"rtp"`
	Stream       string `json:"stream"`
	Protocol     string `json:"protocol" example:"webrtc"`
	WebRTC       string `json:"webrtc"`
	FLV          string `json:"flv"`
	HLS          string `json:"hls"`
	WSFLV        string `json:"ws_flv"`
	ExpiresInSec int64  `json:"expires_in_sec" example:"3600"`
}

// RGbSharePlay 匿名播放响应。
type RGbSharePlay struct {
	BaseResponse
	Data GbSharePlayResp `json:"data"`
}
