package mediaserver

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// 本文件按 WVP ZLMRESTfulUtils.java 的方法集合映射 ZLM REST API。
// 所有方法都做三件事：组装参数 → do() → 解码 data。
// 参数命名保持 ZLM 的 snake_case 语义，Go 侧用驼峰结构体承载。

// GetServerConfig 获取节点完整配置（getServerConfig）。
//
// ZLM 返回的是一个**数组**，每个元素是一张扁平的「配置键 → 值」表，
// 键形如 general.mediaServerId、api.secret、http.port。
// 返回 []map 而不是强类型，是因为不同版本 ZLM 的配置项会增删。
func (c *Client) GetServerConfig(ctx context.Context) ([]map[string]any, error) {
	data, err := c.do(ctx, "getServerConfig", nil)
	if err != nil {
		return nil, err
	}
	var items []map[string]any
	if err := decode("getServerConfig", data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// GetMediaList 查询流列表（getMediaList）。
//
// 任一参数为空则不下发该过滤条件；vhost 为空时使用 __defaultVhost__。
// 单端口模式下 returned stream 是 SSRC 的大写十六进制（设计 §2.2）。
func (c *Client) GetMediaList(ctx context.Context, app, stream, schema, vhost string) ([]MediaInfo, error) {
	params := map[string]any{
		"vhost": vhostOr(vhost),
	}
	if app != "" {
		params["app"] = app
	}
	if stream != "" {
		params["stream"] = stream
	}
	if schema != "" {
		params["schema"] = schema
	}
	data, err := c.do(ctx, "getMediaList", params)
	if err != nil {
		return nil, err
	}
	var items []MediaInfo
	if err := decode("getMediaList", data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// GetMediaInfo 查询单条流的详情（getMediaInfo）。
//
// schema 必须是 ZLM 认识的协议名（rtsp/rtmp/hls/http-flv/...）。
// 流不存在时 ZLM 可能返回 data=null，此时返回 (nil, nil)，由调用方决定语义。
func (c *Client) GetMediaInfo(ctx context.Context, app, stream, schema, vhost string) (*MediaInfo, error) {
	params := map[string]any{
		"app":    app,
		"stream": stream,
		"schema": schema,
		"vhost":  vhostOr(vhost),
	}
	data, err := c.do(ctx, "getMediaInfo", params)
	if err != nil {
		return nil, err
	}
	var info MediaInfo
	if err := decode("getMediaInfo", data, &info); err != nil {
		return nil, err
	}
	if info.Stream == "" {
		return nil, nil
	}
	return &info, nil
}

// CloseRtpServer 关闭指定的 RTP server（closeRtpServer）。
//
// 返回值 hit 表示是否真的命中了端口（ZLM 的 data.hit）。
// 单端口模式下没有流级 RTP server，调用通常返回 hit=0；
// 停止流的正确入口是 close_streams（见 CloseStreams）。
func (c *Client) CloseRtpServer(ctx context.Context, streamID string) (bool, error) {
	resp, err := c.doResponse(ctx, "closeRtpServer", map[string]any{"stream_id": streamID})
	if err != nil {
		return false, err
	}
	// ZLM 把 hit 放在根层级；少数版本放在 data.hit。
	if resp.hasData() {
		var r struct {
			Hit int `json:"hit"`
		}
		if err := decode("closeRtpServer", resp.Data, &r); err != nil {
			return false, err
		}
		if r.Hit != 0 {
			return r.Hit == 1, nil
		}
	}
	if resp.Hit != nil {
		return *resp.Hit == 1, nil
	}
	return false, nil
}

// ConnectRtpServer 让 ZLM 主动向设备拨号收流（connectRtpServer，TCP-ACTIVE）。
//
// ⚠️ 与单端口模式互斥（设计 §7.2）：单端口模式不创建流级 RTP server，
// 该方法仅在将来为个别只支持 TCP-ACTIVE 的设备切换多端口模式时才会用到。
func (c *Client) ConnectRtpServer(ctx context.Context, streamID, targetAddr string, port int) error {
	_, err := c.do(ctx, "connectRtpServer", map[string]any{
		"stream_id": streamID,
		"dst_url":   targetAddr,
		"dst_port":  port,
	})
	return err
}

// ListRtpServer 列出全部 RTP server（listRtpServer）。
func (c *Client) ListRtpServer(ctx context.Context) ([]RtpServer, error) {
	data, err := c.do(ctx, "listRtpServer", nil)
	if err != nil {
		return nil, err
	}
	var items []RtpServer
	if err := decode("listRtpServer", data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// GetRtpInfo 查询某个 RTP server 的收流信息（getRtpInfo）。
func (c *Client) GetRtpInfo(ctx context.Context, streamID string) (*RtpInfo, error) {
	resp, err := c.doResponse(ctx, "getRtpInfo", map[string]any{"stream_id": streamID})
	if err != nil {
		return nil, err
	}
	var info RtpInfo
	if resp.hasData() {
		if err := decode("getRtpInfo", resp.Data, &info); err != nil {
			return nil, err
		}
	}
	// ZLM 实际把 exist/peer_*/local_* 放在根层级。
	if resp.Exist != nil {
		info.Exist = *resp.Exist
	}
	if resp.PeerIP != "" {
		info.PeerIP = resp.PeerIP
	}
	if resp.PeerPort != nil {
		info.PeerPort = *resp.PeerPort
	}
	if resp.LocalIP != "" {
		info.LocalIP = resp.LocalIP
	}
	if resp.LocalPort != nil {
		info.LocalPort = *resp.LocalPort
	}
	return &info, nil
}

// UpdateRtpServerSSRC 修正 RTP server 期望的 SSRC（updateRtpServerSSRC）。
//
// 场景：INVITE 200 OK 里设备回的 y= 行 SSRC 与平台分配的不同（设计 §7.1 步骤⑥）。
// 仅在多端口模式（流级 RTP server）下有意义。
func (c *Client) UpdateRtpServerSSRC(ctx context.Context, app, streamID string, ssrc uint32) error {
	_, err := c.do(ctx, "updateRtpServerSSRC", map[string]any{
		"app":       app,
		"stream_id": streamID,
		"ssrc":      ssrc,
	})
	return err
}

// OpenRtpServer 创建流级 RTP server（openRtpServer）。
//
// ⚠️ **本项目单端口模式下禁止调用**（deploy/zlm/config.ini 有详细说明）：
// port=0 时 ZLM 会从默认 port_range 分配一个宿主机未发布的端口，
// 结果是「接口返回成功、端口也分配了，但永远收不到流」。
// 保留该方法只为将来切换多端口模式时使用；当前编排不要调用它。
func (c *Client) OpenRtpServer(ctx context.Context, p OpenRtpServerParam) (*OpenRtpServerResult, error) {
	params := map[string]any{
		"stream_id": p.StreamID,
		"port":      p.Port,
		"tcp_mode":  p.TCPMode,
	}
	if p.ReusePort {
		params["re_use_port"] = 1
	}
	if p.SSRC != 0 {
		params["ssrc"] = p.SSRC
	}
	if p.OnlyAudio {
		params["only_audio"] = 1
	}
	resp, err := c.doResponse(ctx, "openRtpServer", params)
	if err != nil {
		return nil, err
	}
	var r OpenRtpServerResult
	if resp.hasData() {
		if err := decode("openRtpServer", resp.Data, &r); err != nil {
			return nil, err
		}
	}
	// ZLM 的 openRtpServer 把分配到的端口放在根层级：{"code":0,"port":10000}。
	if r.Port == 0 && resp.Port != nil {
		r.Port = *resp.Port
	}
	return &r, nil
}

// StartSendRTP 向外部目标发送 RTP（startSendRtp）—— 语音回推、国标级联推流。
//
// 语义提醒：SSRC 是**接收端**的 SSRC；dst_url 是接收端地址（设备或上级平台）。
func (c *Client) StartSendRTP(ctx context.Context, p SendRTPParam) (*SendRTPResult, error) {
	params := map[string]any{
		"vhost":    vhostOr(p.Vhost),
		"app":      p.App,
		"stream":   p.Stream,
		"ssrc":     p.SSRC,
		"dst_url":  p.DstURL,
		"dst_port": p.DstPort,
	}
	if p.IsUDP {
		params["is_udp"] = 1
	}
	if p.SrcPort > 0 {
		params["src_port"] = p.SrcPort
	}
	if p.PT > 0 {
		params["pt"] = p.PT
	}
	if p.UsePS {
		params["use_ps"] = 1
	}
	if p.RecvStreamID != "" {
		params["recv_stream_id"] = p.RecvStreamID
	}
	if p.OnlyAudio {
		params["only_audio"] = 1
	}
	resp, err := c.doResponse(ctx, "startSendRtp", params)
	if err != nil {
		return nil, err
	}
	var r SendRTPResult
	if resp.hasData() {
		if err := decode("startSendRtp", resp.Data, &r); err != nil {
			return nil, err
		}
	}
	// 部分版本把 local_port 放在根层级。
	if r.LocalPort == 0 && resp.LocalPort != nil {
		r.LocalPort = *resp.LocalPort
	}
	return &r, nil
}

// StopSendRTP 停止 RTP 发送（stopSendRtp）。
func (c *Client) StopSendRTP(ctx context.Context, app, stream string, ssrc uint32) error {
	params := map[string]any{
		"vhost":  DefaultVhost,
		"app":    app,
		"stream": stream,
	}
	if ssrc != 0 {
		params["ssrc"] = ssrc
	}
	_, err := c.do(ctx, "stopSendRtp", params)
	return err
}

// AddStreamProxy 让 ZLM 主动拉流（addStreamProxy，rtsp/rtmp/http 源）。
func (c *Client) AddStreamProxy(ctx context.Context, p AddStreamProxyParam) (*StreamProxyResult, error) {
	retry := p.RetryCount
	if retry <= 0 {
		retry = 3 // 与 WVP 默认一致
	}
	params := map[string]any{
		"vhost":        vhostOr(p.Vhost),
		"app":          p.App,
		"stream":       p.Stream,
		"url":          p.URL,
		"enable_audio": boolInt(p.EnableAudio),
		"enable_mp4":   boolInt(p.EnableMP4),
		"retry_count":  retry,
	}
	if p.RTPType != "" {
		params["rtp_type"] = p.RTPType
	}
	if p.TimeoutSec > 0 {
		params["timeout_sec"] = p.TimeoutSec
	}
	data, err := c.do(ctx, "addStreamProxy", params)
	if err != nil {
		return nil, err
	}
	var r StreamProxyResult
	if err := decode("addStreamProxy", data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// DelStreamProxy 删除拉流代理（delStreamProxy）。
func (c *Client) DelStreamProxy(ctx context.Context, key string) error {
	_, err := c.do(ctx, "delStreamProxy", map[string]any{"key": key})
	return err
}

// AddFFmpegSource 通过 ffmpeg 拉流（addFFmpegSource）。
func (c *Client) AddFFmpegSource(ctx context.Context, p AddFFmpegSourceParam) (*StreamProxyResult, error) {
	params := map[string]any{
		"src_url":    p.SrcURL,
		"dst_url":    p.DstURL,
		"timeout_ms": p.TimeoutMS,
		"enable_mp4": boolInt(p.EnableMP4),
	}
	if p.FFmpegCmdKey != "" {
		params["ffmpeg_cmd_key"] = p.FFmpegCmdKey
	}
	data, err := c.do(ctx, "addFFmpegSource", params)
	if err != nil {
		return nil, err
	}
	var r StreamProxyResult
	if err := decode("addFFmpegSource", data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// StartRecord 开始录制 MP4（startRecord，type=1）。
//
// maxSecond<=0 时使用 ZLM 配置的 mp4_max_second。
func (c *Client) StartRecord(ctx context.Context, app, stream, vhost string, maxSecond int) error {
	params := map[string]any{
		"type":   1,
		"vhost":  vhostOr(vhost),
		"app":    app,
		"stream": stream,
	}
	if maxSecond > 0 {
		params["max_second"] = maxSecond
	}
	_, err := c.do(ctx, "startRecord", params)
	return err
}

// StopRecord 停止录制（stopRecord，type=0）。
func (c *Client) StopRecord(ctx context.Context, app, stream, vhost string) error {
	_, err := c.do(ctx, "stopRecord", map[string]any{
		"type":   0,
		"vhost":  vhostOr(vhost),
		"app":    app,
		"stream": stream,
	})
	return err
}

// GetSnap 抓图（getSnap），返回 JPEG 原始字节。
//
// url 是 ZLM 可拉取的流地址（通常用 GetMediaList 得到的流拼 rtsp/http-flv），
// timeoutSec 控制 ZLM 内部等待关键帧的时间，expireSec 是缓存过期时间。
func (c *Client) GetSnap(ctx context.Context, streamURL string, timeoutSec, expireSec int, async bool) ([]byte, error) {
	params := map[string]any{
		"url":         streamURL,
		"timeout_sec": timeoutSec,
		"expire_sec":  expireSec,
		"async":       boolInt(async),
	}
	return c.doGetRaw(ctx, "getSnap", params)
}

// CloseStreams 强制关闭一个流及其全部播放会话（close_streams）。
//
// 为什么需要它：单端口模式没有流级 RTP server，closeRtpServer 无从谈起；
// 「停止点播/清理残留流」必须走 close_streams。
func (c *Client) CloseStreams(ctx context.Context, app, stream string) error {
	_, err := c.do(ctx, "close_streams", map[string]any{
		"vhost":  DefaultVhost,
		"app":    app,
		"stream": stream,
		"force":  1,
	})
	return err
}

// IsStreamReady 判断流是否在线（isMediaOnline）。
//
// 前端「流已就绪」提示与编排侧 10s 超时判定都用它。
func (c *Client) IsStreamReady(ctx context.Context, app, stream, schema, vhost string) (bool, error) {
	params := map[string]any{
		"vhost": vhostOr(vhost),
	}
	if app != "" {
		params["app"] = app
	}
	if stream != "" {
		params["stream"] = stream
	}
	if schema != "" {
		params["schema"] = schema
	}
	resp, err := c.doResponse(ctx, "isMediaOnline", params)
	if err != nil {
		return false, err
	}
	// ZLM 实际返回 {"code":0,"online":true}（根层级），少数版本给 data:bool。
	if resp.hasData() {
		var online bool
		if err := decode("isMediaOnline", resp.Data, &online); err != nil {
			return false, err
		}
		return online, nil
	}
	if resp.Online != nil {
		return *resp.Online, nil
	}
	return false, nil
}

// RestartServer 重启 ZLM 节点（restartServer）。
//
// 进程会立即断开连接，调用方不要等待响应体中的业务数据。
func (c *Client) RestartServer(ctx context.Context) error {
	_, err := c.do(ctx, "restartServer", nil)
	return err
}

// GetThreadsLoad 查询节点线程负载（getThreadsLoad），用于节点健康检查。
func (c *Client) GetThreadsLoad(ctx context.Context) ([]ThreadLoad, error) {
	data, err := c.do(ctx, "getThreadsLoad", nil)
	if err != nil {
		return nil, err
	}
	var items []ThreadLoad
	if err := decode("getThreadsLoad", data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// ── 播放地址构造 ──────────────────────────────────────────────

// BuildPlayURLs 按媒体节点配置构造一个流的全部播放地址。
//
// 端口选择规则（与 WVP 的 getStreamInfoByAppAndStream 对齐）：
//   - WebRTC / HTTP-FLV / HLS：走 HTTP 端口；FLV/WS-FLV 若单独配置了端口则优先用它，
//     否则回退 http_port（新版 ZLM 默认把 flv/hls 合并到 http 端口）。
//   - WS-FLV：ws:// 协议，端口语义同上。
//   - RTSP / RTMP：各自独立端口；端口为 0（未配置）时该地址留空 ——
//     不同协议不能回退到 http_port，否则会生成一个「看起来有地址但连不上」的坑。
//
// app/stream 会做 URL 转义；单端口模式下 stream 应是 SSRC 的大写十六进制。
func BuildPlayURLs(ms *model.GbMediaServer, app, stream string) PlayURLs {
	if ms == nil {
		return PlayURLs{}
	}
	host := strings.TrimSpace(ms.StreamIP)
	if host == "" {
		host = strings.TrimSpace(ms.IP)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	// 兼容配置里带协议的地址。
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://"), "/")

	httpPort := portOr(ms.HTTPPort, 80)
	flvPort := portOr(ms.FLVPort, httpPort)
	wsFlvPort := portOr(ms.WSFLVPort, httpPort)

	ea := url.PathEscape(app)
	es := url.PathEscape(stream)

	urls := PlayURLs{
		WebRTC: fmt.Sprintf("http://%s:%d/index/api/webrtc?app=%s&stream=%s&type=play",
			host, httpPort, url.QueryEscape(app), url.QueryEscape(stream)),
		FLV:   fmt.Sprintf("http://%s:%d/%s/%s.live.flv", host, flvPort, ea, es),
		HLS:   fmt.Sprintf("http://%s:%d/%s/%s/hls.m3u8", host, httpPort, ea, es),
		WSFLV: fmt.Sprintf("ws://%s:%d/%s/%s.live.flv", host, wsFlvPort, ea, es),
	}
	if ms.RTSPPort > 0 {
		urls.RTSP = fmt.Sprintf("rtsp://%s:%d/%s/%s", host, ms.RTSPPort, ea, es)
	}
	if ms.RTMPPort > 0 {
		urls.RTMP = fmt.Sprintf("rtmp://%s:%d/%s/%s", host, ms.RTMPPort, ea, es)
	}
	return urls
}

// boolInt 把 bool 转成 ZLM 期望的 0/1。
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
