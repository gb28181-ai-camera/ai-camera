package gb28181

// 本文件实现回放、下载与「设备侧录像检索」。
//
// 三条硬约束（设计 §2.5 / §6.5）：
//  1. 回放检索走带外 MESSAGE（由 sipserver.QueryRecordInfo 封装）；
//  2. 回放控制（暂停/恢复/拖动/倍速）走**带内 SIP INFO + MANSRTSP**；
//  3. 云台控制与本文件无关 —— 它走带外 MESSAGE，两者不可复用发送函数。

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"gorm.io/gorm"
)

// ── 回放/下载业务错误 ─────────────────────────────────────────

var (
	// ErrPlaybackTimeInvalid 回放时间窗非法（缺字段、格式不符或结束早于开始）。
	ErrPlaybackTimeInvalid = errors.New("回放时间窗非法：请使用 2006-01-02T15:04:05 格式，且结束时间晚于开始时间")
	// ErrPlaybackSessionNotFound 回放/下载会话不存在或已结束。
	ErrPlaybackSessionNotFound = errors.New("回放会话不存在或已结束")
	// ErrPlaybackActionInvalid 回放控制动作非法。
	ErrPlaybackActionInvalid = errors.New("回放控制动作非法，仅支持 pause/resume/seek/speed")
	// ErrGbDownloadNotFound 下载任务不存在。
	ErrGbDownloadNotFound = errors.New("下载任务不存在")
)

// recordQueryTimeout 录像检索等待设备响应的上限。
//
// 设备侧 Catalog/RecordInfo 响应可能分多包返回，但正常局域网内应在 1 秒内回齐。
// 取 5 秒既能覆盖慢设备，又不会让前端按钮长时间无反馈；超时按「空列表 + timeout」
// 返回，而不是把 HTTP 请求挂死。
const recordQueryTimeout = 5 * time.Second

// ════════════════════════════════════════════════════════════════
// 共享辅助：SIP 服务实例 / 通道与设备 / 媒体节点
// ════════════════════════════════════════════════════════════════

// activeSipService 取当前进程内的 SIP 服务实例。
//
// control.Default() 只有 CRUD 所需的最小接口（QueryCatalog/Subscribe/SendMessage），
// 回放需要 QueryRecordInfo / SendInvite / Sessions / BuildPlaySDP，因此这里
// 断言回具体实现。cmd/server 的内嵌模式与 cmd/sipsvc 都会 control.Set(*Service)。
//
// 注意：生产环境若 API 与 sipsvc 分进程部署，API 进程里 control.Default() 为 nil，
// 所有依赖 SIP 的接口会返回 ErrSipServiceUnavailable（HTTP 503），而不是静默成功。
// 跨进程命令通道不在本批次范围内。
func activeSipService() (*sipserver.Service, error) {
	ctl := control.Default()
	if ctl == nil {
		return nil, ErrSipServiceUnavailable
	}
	svc, ok := ctl.(*sipserver.Service)
	if !ok || svc == nil {
		return nil, ErrSipServiceUnavailable
	}
	return svc, nil
}

// loadChannelAndDevice 读取通道及其归属设备。
//
// GbChannelRepo.GetByID 已 Preload Device；但历史数据可能缺失关联，
// 此时按 device_id 再查一次，避免把「设备未配置」误报成「通道不存在」。
func loadChannelAndDevice(ctx context.Context, channelID int64) (*model.GbChannel, *model.Device, error) {
	ch, err := repository.NewGbChannelRepo().GetByID(ctx, channelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrGbChannelNotFound
		}
		return nil, nil, err
	}
	dev := ch.Device
	if dev == nil {
		dev, err = repository.NewDeviceRepo().GetByID(ctx, ch.DeviceID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil, ErrGbDeviceNotFound
			}
			return nil, nil, err
		}
	}
	if strings.TrimSpace(dev.GbDeviceID) == "" {
		return nil, nil, ErrGbDeviceNotConfigured
	}
	return ch, dev, nil
}

// resolveMediaServerForDevice 按设备绑定的节点解析 ZLM，缺省回落默认节点。
func resolveMediaServerForDevice(ctx context.Context, dev *model.Device) (*model.GbMediaServer, error) {
	if dev != nil {
		if ms, err := resolveMediaServerByKey(ctx, dev.MediaServerID); err == nil && ms != nil {
			return ms, nil
		}
	}
	return resolveMediaServerByKey(ctx, "")
}

// resolveMediaServerByKey 按 server_id 解析节点；空值/"auto" 时取默认节点。
func resolveMediaServerByKey(ctx context.Context, serverID string) (*model.GbMediaServer, error) {
	repo := repository.NewGbMediaServerRepo()
	key := strings.TrimSpace(serverID)
	if key != "" && !strings.EqualFold(key, "auto") {
		ms, err := repo.GetByServerID(ctx, key)
		if err == nil {
			return ms, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	ms, err := repo.GetDefault(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("未配置默认流媒体节点，请先在媒体节点页面添加并设为默认")
		}
		return nil, err
	}
	return ms, nil
}

// mediaHTTPBase 拼出 ZLM 的 HTTP 基址（播放地址三件套之一）。
func mediaHTTPBase(ms *model.GbMediaServer) string {
	if ms == nil {
		return ""
	}
	host := strings.TrimSpace(ms.StreamIP)
	if host == "" {
		host = strings.TrimSpace(ms.IP)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://"), "/")
	port := ms.HTTPPort
	if port <= 0 {
		port = 80
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}

// applyPlayURLs 把 BuildPlayURLs 的结果填入响应结构。
func applyPlayURLs(dst *response.GbPlaybackResp, urls mediaserver.PlayURLs) {
	dst.WebRTC = urls.WebRTC
	dst.FLV = urls.FLV
	dst.HLS = urls.HLS
	dst.WSFLV = urls.WSFLV
}

// ── 时间解析 ──────────────────────────────────────────────────

// gbTimeLayouts 设备侧常见时间格式。
//
// GB28181 规范是 "2006-01-02T15:04:05"，但现场设备经常多带时区或毫秒，
// 严格只认一种格式会表现为「明明有录像却检索不到」。
var gbTimeLayouts = []string{
	"2006-01-02T15:04:05",
	time.RFC3339,
	"2006-01-02T15:04:05.000",
	"2006-01-02 15:04:05",
}

// parseGbTime 解析设备时间；无法解析时返回零值。
func parseGbTime(s string) (time.Time, bool) {
	v := strings.TrimSpace(s)
	if v == "" {
		return time.Time{}, false
	}
	for _, layout := range gbTimeLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// validateTimeWindow 校验回放时间窗。
func validateTimeWindow(start, end string) (time.Time, time.Time, error) {
	st, ok := parseGbTime(start)
	if !ok {
		return time.Time{}, time.Time{}, ErrPlaybackTimeInvalid
	}
	et, ok := parseGbTime(end)
	if !ok {
		return time.Time{}, time.Time{}, ErrPlaybackTimeInvalid
	}
	if !et.After(st) {
		return time.Time{}, time.Time{}, ErrPlaybackTimeInvalid
	}
	return st, et, nil
}

// buildTimeShiftSDP 构造回放/下载 SDP。
//
// 复用 sipserver.BuildPlaySDP 保证 o=/c=/m=/y= 与实时点播完全一致
// （尤其是 o= 必须是设备国标编码，契约 §15.5b），再补三类回放专有行：
//   - u=<通道>:<开始>:<结束>：设备据此定位录像片段；
//   - t=<unix 起> <unix 止>：回放时间窗；
//   - a=playback：回放标识（下载会话不加）。
//
// 这些行 sipserver 的实时点播构造器不提供，也不应为了回放污染点播路径。
func buildTimeShiftSDP(sipSvc *sipserver.Service, dev *model.Device, ssrc, sessionName, channelID, start, end string) string {
	raw := sipSvc.BuildPlaySDP(dev, ssrc, sessionName)
	st, _ := parseGbTime(start)
	et, _ := parseGbTime(end)

	var b strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "t=0 0" {
			fmt.Fprintf(&b, "t=%d %d\r\n", st.Unix(), et.Unix())
			continue
		}
		b.WriteString(line + "\r\n")
		if strings.HasPrefix(line, "s=") {
			fmt.Fprintf(&b, "u=%s:%s:%s\r\n", channelID, start, end)
			continue
		}
		if sessionName == "Playback" && strings.HasPrefix(line, "a=rtpmap:96") {
			b.WriteString("a=playback\r\n")
		}
	}
	return b.String()
}

// ════════════════════════════════════════════════════════════════
// 录像检索：进程级回调 + 轻量等待表
//
// SetRecordInfoHandler 是进程级单例，只能注册一个回调；回调只带
// (device, RecordInfoResponse)，不带发起查询时的时间窗。因此路由键取
// 「设备国标编码 + 通道国标编码」，时间窗只用于过滤条目。
//
// ★ 已知限制：sipserver.QueryRecordInfo 内部自行生成 SN 且不返回，
// 业务层无法用 SN 精确匹配；若同一通道有并发检索，后发起的查询会顶掉
// 前一个等待者（前者按超时返回）。单用户排障场景足够，多用户并发检索
// 同一通道时可能出现「其中一次为空」—— 这是 SIP 层未暴露 SN 的边界，
// 已在交付报告中记录，修复需要 sipserver 返回 SN（跨 Agent 文件）。
// ════════════════════════════════════════════════════════════════

type recordWaiter struct {
	key       string
	start     time.Time
	end       time.Time
	channelID int64
	gbChID    string
	deviceID  int64

	mu       sync.Mutex
	items    []manscdp.RecordItem
	done     bool
	aborted  bool // 被同键的新检索顶掉：返回已收到的部分并标记 timeout
	doneOnce sync.Once
	doneCh   chan struct{}
}

func newRecordWaiter(key string, channelID int64, gbChID string, deviceID int64, start, end time.Time) *recordWaiter {
	return &recordWaiter{
		key:       key,
		start:     start,
		end:       end,
		channelID: channelID,
		gbChID:    gbChID,
		deviceID:  deviceID,
		doneCh:    make(chan struct{}),
	}
}

// add 合并一包响应。
//
// 设备可能分多包返回，SumNum 是总数；累计条目数达到 SumNum 即结束。
// SumNum==0 表示设备明确回复「无录像」，立即结束（不能等满超时）。
func (w *recordWaiter) add(resp *manscdp.RecordInfoResponse) {
	if w == nil || resp == nil {
		return
	}
	w.mu.Lock()
	if w.done {
		w.mu.Unlock()
		return
	}
	for _, item := range resp.RecordList.Items {
		if !w.inWindow(item) {
			continue
		}
		w.items = append(w.items, item)
	}
	expected := resp.SumNum
	if expected <= 0 || len(w.items) >= expected {
		w.done = true
		w.mu.Unlock()
		w.finish()
		return
	}
	w.mu.Unlock()
}

// inWindow 判断条目是否与请求时间窗重叠。
//
// 设备有时会把相邻时段的录像一并返回；不过滤会让前端看到时间窗外
// 的片段并在 seek 时失败。时间解析失败时放行（宁可多给，不可漏给）。
func (w *recordWaiter) inWindow(item manscdp.RecordItem) bool {
	st, ok1 := parseGbTime(item.StartTime)
	et, ok2 := parseGbTime(item.EndTime)
	if !ok1 || !ok2 {
		return true
	}
	return !et.Before(w.start) && !st.After(w.end)
}

func (w *recordWaiter) finish() {
	w.doneOnce.Do(func() { close(w.doneCh) })
}

// abort 标记被并发检索顶掉后结束，避免旧请求把部分结果当成完整结果。
func (w *recordWaiter) abort() {
	w.mu.Lock()
	w.aborted = true
	w.mu.Unlock()
	w.finish()
}

// wasAborted 返回是否被顶掉。
func (w *recordWaiter) wasAborted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.aborted
}

func (w *recordWaiter) snapshot() []manscdp.RecordItem {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]manscdp.RecordItem, len(w.items))
	copy(out, w.items)
	return out
}

var (
	recordWaitersMu sync.Mutex
	recordWaiters   = map[string]*recordWaiter{}
)

// recordWaitKey 路由键。
func recordWaitKey(gbDeviceID, gbChannelID string) string {
	return strings.TrimSpace(gbDeviceID) + "|" + strings.TrimSpace(gbChannelID)
}

// handleRecordInfoResponse 进程级回调：把响应分发给当前等待者。
func handleRecordInfoResponse(dev *model.Device, resp *manscdp.RecordInfoResponse) {
	if dev == nil || resp == nil {
		return
	}
	key := recordWaitKey(dev.GbDeviceID, resp.DeviceID)
	recordWaitersMu.Lock()
	w := recordWaiters[key]
	recordWaitersMu.Unlock()
	if w == nil {
		// 没有进行中的检索：静默丢弃。这是常态（设备可能主动补发），
		// 不写日志以免刷屏。
		return
	}
	w.add(resp)
}

// registerRecordWaiter 登记等待者；同键旧等待者立即结束（返回已收到的部分）。
func registerRecordWaiter(key string, w *recordWaiter) *recordWaiter {
	recordWaitersMu.Lock()
	old := recordWaiters[key]
	recordWaiters[key] = w
	recordWaitersMu.Unlock()
	if old != nil {
		old.abort()
	}
	return old
}

func unregisterRecordWaiter(key string, w *recordWaiter) {
	recordWaitersMu.Lock()
	if recordWaiters[key] == w {
		delete(recordWaiters, key)
	}
	recordWaitersMu.Unlock()
}

// ════════════════════════════════════════════════════════════════
// GbPlaybackService
// ════════════════════════════════════════════════════════════════

// GbPlaybackService 回放与下载业务逻辑。
type GbPlaybackService struct{}

// NewGbPlaybackService 构造回放服务。
func NewGbPlaybackService() *GbPlaybackService { return &GbPlaybackService{} }

// QueryRecord 检索设备侧录像。
//
// 超时返回已收到的条目（可能为空）且 Timeout=true，不返回 error ——
// 「设备没响应」是业务结果而不是服务端故障，前端需要区分展示。
func (s *GbPlaybackService) QueryRecord(ctx context.Context, channelID int64, req *request.GbRecordQueryReq) (*response.GbRecordQueryResp, error) {
	if req == nil {
		return nil, ErrPlaybackTimeInvalid
	}
	start, end, err := validateTimeWindow(req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	ch, dev, err := loadChannelAndDevice(ctx, channelID)
	if err != nil {
		return nil, err
	}
	sipSvc, err := activeSipService()
	if err != nil {
		return nil, err
	}

	// 回调是进程级单例。每次检索都重新注册而不是 sync.Once：
	// SIP 服务重启会创建新实例，旧实例上的回调不会跟过来；重复设置开销可忽略。
	sipSvc.SetRecordInfoHandler(handleRecordInfoResponse)

	key := recordWaitKey(dev.GbDeviceID, ch.GbChannelID)
	waiter := newRecordWaiter(key, ch.ID, ch.GbChannelID, dev.ID, start, end)
	registerRecordWaiter(key, waiter)
	defer unregisterRecordWaiter(key, waiter)

	if err := sipSvc.QueryRecordInfo(ctx, dev, ch.GbChannelID, req.StartTime, req.EndTime); err != nil {
		return nil, err
	}

	timer := time.NewTimer(recordQueryTimeout)
	defer timer.Stop()

	timedOut := false
	select {
	case <-waiter.doneCh:
		timedOut = waiter.wasAborted()
	case <-timer.C:
		timedOut = true
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	items := waiter.snapshot()
	out := make([]response.GbRecordItem, 0, len(items))
	for _, it := range items {
		out = append(out, toGbRecordItem(ch, dev, it))
	}
	return &response.GbRecordQueryResp{
		ChannelID:   ch.ID,
		GbChannelID: ch.GbChannelID,
		Total:       len(out),
		Timeout:     timedOut,
		Items:       out,
	}, nil
}

// toGbRecordItem 设备条目 → 响应。
func toGbRecordItem(ch *model.GbChannel, dev *model.Device, item manscdp.RecordItem) response.GbRecordItem {
	var timeLen int64
	if st, ok := parseGbTime(item.StartTime); ok {
		if et, ok2 := parseGbTime(item.EndTime); ok2 && et.After(st) {
			timeLen = int64(et.Sub(st).Seconds())
		}
	}
	return response.GbRecordItem{
		ChannelID:   ch.ID,
		GbChannelID: ch.GbChannelID,
		DeviceID:    dev.ID,
		Name:        item.Name,
		FilePath:    item.FilePath,
		Address:     item.Address,
		StartTime:   item.StartTime,
		EndTime:     item.EndTime,
		Secrecy:     item.Secrecy,
		Type:        item.Type,
		TimeLen:     timeLen,
	}
}

// StartPlayback 发起回放 INVITE，返回 ZLM 播放地址。
func (s *GbPlaybackService) StartPlayback(ctx context.Context, req *request.GbPlaybackReq) (*response.GbPlaybackResp, error) {
	if req == nil {
		return nil, ErrPlaybackTimeInvalid
	}
	_, _, err := validateTimeWindow(req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	ch, dev, err := loadChannelAndDevice(ctx, req.ChannelID)
	if err != nil {
		return nil, err
	}
	sipSvc, err := activeSipService()
	if err != nil {
		return nil, err
	}

	ssrcStr, stream, err := sipSvc.AllocateSSRC()
	if err != nil {
		return nil, fmt.Errorf("分配 SSRC 失败: %w", err)
	}

	sdpBody := buildTimeShiftSDP(sipSvc, dev, ssrcStr, "Playback", ch.GbChannelID, req.StartTime, req.EndTime)
	target := sipserver.CallTarget{
		Device:      dev,
		ChannelID:   ch.GbChannelID,
		SessionName: "Playback",
		Subject:     fmt.Sprintf("%s:%s,%s,%s", ch.GbChannelID, ssrcStr, req.StartTime, req.EndTime),
		SSRC:        ssrcStr,
	}
	dlg, err := sipSvc.SendInvite(ctx, target, []byte(sdpBody), "")
	if err != nil {
		sipSvc.ReleaseSSRC(ssrcStr)
		return nil, err
	}

	ms, err := resolveMediaServerForDevice(ctx, dev)
	if err != nil {
		// 会话已建立但拿不到节点配置：结束会话，避免设备持续推流到无人消费的端口。
		_ = dlg.Bye(ctx)
		sipSvc.ReleaseSSRC(ssrcStr)
		return nil, err
	}
	urls := mediaserver.BuildPlayURLs(ms, dlg.App, stream)

	resp := &response.GbPlaybackResp{
		StreamID:      dlg.CallID,
		App:           dlg.App,
		Stream:        stream,
		SSRC:          ssrcStr,
		HTTPBase:      mediaHTTPBase(ms),
		MediaServerID: ms.ServerID,
		ChannelID:     ch.ID,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
	}
	applyPlayURLs(resp, urls)
	return resp, nil
}

// Control 回放控制：pause / resume / seek / speed。
//
// ★ 必须走会话内 INFO（MANSRTSP）。云台的 SendMessage 是带外 MESSAGE，
// 两者混用会「按了没反应但也不报错」—— 这是设计 §2.5 点名的一周级 bug。
func (s *GbPlaybackService) Control(ctx context.Context, req *request.GbPlaybackControlReq) error {
	if req == nil {
		return ErrPlaybackActionInvalid
	}
	sipSvc, err := activeSipService()
	if err != nil {
		return err
	}
	dlg, ok := sipSvc.Sessions().Get(strings.TrimSpace(req.StreamID))
	if !ok || dlg == nil {
		return ErrPlaybackSessionNotFound
	}

	var body string
	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "pause":
		body = "PAUSE RTSP/1.0\r\nCSeq: 1\r\n\r\n"
	case "resume":
		body = "PLAY RTSP/1.0\r\nCSeq: 1\r\n\r\n"
	case "seek":
		if strings.TrimSpace(req.SeekTime) == "" {
			return errors.New("seek 动作必须提供 seek_time")
		}
		if _, ok := parseGbTime(req.SeekTime); !ok {
			return errors.New("seek_time 格式应为 2006-01-02T15:04:05")
		}
		body = fmt.Sprintf("PLAY RTSP/1.0\r\nCSeq: 1\r\nRange: npt=%s\r\n\r\n", req.SeekTime)
	case "speed":
		if req.Scale <= 0 {
			return errors.New("speed 动作的 scale 必须大于 0（1.0 为原速）")
		}
		body = fmt.Sprintf("PLAY RTSP/1.0\r\nCSeq: 1\r\nScale: %.1f\r\n\r\n", req.Scale)
	default:
		return ErrPlaybackActionInvalid
	}

	// Dialog.Info 内部已经检查了响应状态（Do 返回非 2xx 会带 error），
	// 因此这里只处理发送失败。
	if err := dlg.Info(ctx, body, sipserver.ContentTypeMANSRTSP); err != nil {
		return fmt.Errorf("发送带内 INFO 失败: %w", err)
	}
	return nil
}

// StopPlayback 结束回放会话。
//
// 幂等：会话已不存在时返回 nil —— 前端关闭播放器、设备主动 BYE、
// 定时清理都可能重复触发停止，重复报错没有价值。
func (s *GbPlaybackService) StopPlayback(ctx context.Context, streamID string) error {
	sipSvc, err := activeSipService()
	if err != nil {
		return err
	}
	dlg, ok := sipSvc.Sessions().Get(strings.TrimSpace(streamID))
	if !ok || dlg == nil {
		return nil
	}
	_ = dlg.Bye(ctx)
	releasePlayStream(ctx, dlg.App, dlg.Stream)
	sipSvc.ReleaseSSRC(dlg.SSRC)
	return nil
}

// releasePlayStream 结束 SIP 会话后清理 ZLM 侧残留流。
//
// 单端口模式没有流级 RTP server，closeRtpServer 无从谈起；停止流必须走
// close_streams（mediaserver.CloseStreams 的注释有同样说明）。
func releasePlayStream(ctx context.Context, app, stream string) {
	if strings.TrimSpace(stream) == "" {
		return
	}
	ms, err := resolveMediaServerByKey(ctx, "")
	if err != nil || ms == nil {
		return
	}
	if client := mediaserver.FromModel(ms); client != nil {
		// 尽力而为：流可能已经自行断开，清理失败不影响停止语义。
		_ = client.CloseStreams(ctx, app, stream)
	}
}

// ════════════════════════════════════════════════════════════════
// 下载任务（进程内表）
//
// 为什么不用数据库：现有 20 张国标表里没有「下载任务」表
// （ac_gb_cloud_record 是 ZLM 落盘 MP4 的索引，语义不同）。本批次不做迁移，
// 任务只存在于当前 API 进程内存：
//   - 单进程部署（开发/一体机）下行为完整；
//   - 多副本或重启后任务列表会丢失，但设备侧下载会话本身由 SIP 生命周期
//     管理，重启会随进程一起终止 —— 不会留下「僵尸任务」。
// 若后续需要持久化/断点续传，应新增 ac_gb_download_task 表并走正式迁移。
// ════════════════════════════════════════════════════════════════

var (
	downloadMu    sync.Mutex
	downloadTasks = map[int64]*response.GbDownloadItem{}
	downloadSeq   atomic.Int64
)

// StartDownload 发起下载 INVITE 并登记内存任务。
func (s *GbPlaybackService) StartDownload(ctx context.Context, req *request.GbDownloadReq) (*response.GbDownloadResp, error) {
	if req == nil {
		return nil, ErrPlaybackTimeInvalid
	}
	_, _, err := validateTimeWindow(req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	ch, dev, err := loadChannelAndDevice(ctx, req.ChannelID)
	if err != nil {
		return nil, err
	}
	sipSvc, err := activeSipService()
	if err != nil {
		return nil, err
	}

	ssrcStr, stream, err := sipSvc.AllocateSSRC()
	if err != nil {
		return nil, fmt.Errorf("分配 SSRC 失败: %w", err)
	}
	sdpBody := buildTimeShiftSDP(sipSvc, dev, ssrcStr, "Download", ch.GbChannelID, req.StartTime, req.EndTime)
	target := sipserver.CallTarget{
		Device:      dev,
		ChannelID:   ch.GbChannelID,
		SessionName: "Download",
		Subject:     fmt.Sprintf("%s:%s,%s,%s", ch.GbChannelID, ssrcStr, req.StartTime, req.EndTime),
		SSRC:        ssrcStr,
	}
	dlg, err := sipSvc.SendInvite(ctx, target, []byte(sdpBody), "")
	if err != nil {
		sipSvc.ReleaseSSRC(ssrcStr)
		return nil, err
	}

	// 下载会话建立后让 ZLM 把收到的 PS 流落成 MP4。
	// 设备开始推流有时间差，StartRecord 对尚未创建的流会失败，
	// 因此异步重试几次；失败只更新任务提示，不阻塞 HTTP 响应。
	ms, err := resolveMediaServerForDevice(ctx, dev)
	if err != nil {
		_ = dlg.Bye(ctx)
		sipSvc.ReleaseSSRC(ssrcStr)
		return nil, err
	}

	taskID := downloadSeq.Add(1)
	go startDownloadRecording(taskID, ms, dlg.App, stream)

	item := &response.GbDownloadItem{
		ID:          taskID,
		ChannelID:   ch.ID,
		ChannelName: ch.Name,
		DeviceID:    dev.ID,
		DeviceName:  dev.Name,
		StreamID:    dlg.CallID,
		App:         dlg.App,
		Stream:      stream,
		SSRC:        ssrcStr,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Status:      "downloading",
		Progress:    0,
		CreatedDT:   time.Now(),
	}
	// 说明：真实下载进度需要 ZLM 侧的文件写入回调（on_record_mp4），
	// 本批次只维护会话状态，Progress 由后续批次接入回调后更新。
	downloadMu.Lock()
	downloadTasks[item.ID] = item
	downloadMu.Unlock()

	return &response.GbDownloadResp{GbDownloadItem: *item}, nil
}

// PageDownload 分页查询内存下载任务。
func (s *GbPlaybackService) PageDownload(_ context.Context, req *request.PageGbDownloadReq, _ *utils.DataScope) (*response.PageGbDownloadResp, error) {
	if req == nil {
		req = &request.PageGbDownloadReq{}
	}
	req.Normalize()

	downloadMu.Lock()
	items := make([]response.GbDownloadItem, 0, len(downloadTasks))
	for _, t := range downloadTasks {
		items = append(items, *t)
	}
	downloadMu.Unlock()

	// 无数据库，排序在内存完成：任务量天然很小（单进程排障/补录场景）。
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })

	kw := strings.ToLower(strings.TrimSpace(req.Keyword))
	status := strings.TrimSpace(req.Status)
	filtered := items[:0]
	for _, it := range items {
		if req.ChannelID != nil && it.ChannelID != *req.ChannelID {
			continue
		}
		if status != "" && it.Status != status {
			continue
		}
		if kw != "" {
			hay := strings.ToLower(it.ChannelName + "|" + it.DeviceName + "|" + it.Stream + "|" + it.StreamID)
			if !strings.Contains(hay, kw) {
				continue
			}
		}
		filtered = append(filtered, it)
	}

	total := int64(len(filtered))
	start := req.Offset()
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + req.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return &response.PageGbDownloadResp{
		Items:    append([]response.GbDownloadItem(nil), filtered[start:end]...),
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// StopDownload 停止下载任务（保留历史记录，状态置 stopped）。
func (s *GbPlaybackService) StopDownload(ctx context.Context, id int64) error {
	downloadMu.Lock()
	task := downloadTasks[id]
	downloadMu.Unlock()
	if task == nil {
		return ErrGbDownloadNotFound
	}
	if task.Status != "downloading" {
		return nil // 幂等
	}

	// 先停录制再 BYE：BYE 后流可能立即消失，StopRecord 就找不到流了。
	if ms, err := resolveMediaServerByKey(ctx, ""); err == nil && ms != nil {
		_ = mediaserver.FromModel(ms).StopRecord(ctx, task.App, task.Stream, "")
	}
	if sipSvc, err := activeSipService(); err == nil {
		if dlg, ok := sipSvc.Sessions().Get(task.StreamID); ok && dlg != nil {
			_ = dlg.Bye(ctx)
			releasePlayStream(ctx, dlg.App, dlg.Stream)
			sipSvc.ReleaseSSRC(task.SSRC)
		}
	}

	now := time.Now()
	downloadMu.Lock()
	task.Status = "stopped"
	task.StoppedDT = &now
	downloadMu.Unlock()
	return nil
}

// startDownloadRecording 异步让 ZLM 开始录制下载流。
//
// 设备从 200 OK 到真正发 RTP 通常有几百毫秒到数秒的时差；立即调用
// startRecord 大概率返回「流不存在」。这里按 1 秒间隔重试 5 次，
// 全部失败才把原因写进任务提示。
func startDownloadRecording(taskID int64, ms *model.GbMediaServer, app, stream string) {
	client := mediaserver.FromModel(ms)
	var lastErr error
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lastErr = client.StartRecord(rctx, app, stream, "", 0)
		cancel()
		if lastErr == nil {
			setDownloadMessage(taskID, "ZLM 录制中")
			return
		}
	}
	setDownloadMessage(taskID, "无法开始 ZLM 录制: "+lastErr.Error())
}

// setDownloadMessage 更新任务的提示文案。
func setDownloadMessage(taskID int64, msg string) {
	downloadMu.Lock()
	if t := downloadTasks[taskID]; t != nil && t.Status == "downloading" {
		t.Message = msg
	}
	downloadMu.Unlock()
}
