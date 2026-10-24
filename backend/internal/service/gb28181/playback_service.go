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

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/dto/request"
	"github.com/boticz/ai-camera-backend/internal/dto/response"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/mediaserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/protocol/manscdp"
	"github.com/boticz/ai-camera-backend/internal/gb28181/record"
	"github.com/boticz/ai-camera-backend/internal/gb28181/sipserver"
	"github.com/boticz/ai-camera-backend/internal/gb28181/ssrc"
	"github.com/boticz/ai-camera-backend/internal/model"
	"github.com/boticz/ai-camera-backend/internal/repository"
	"github.com/boticz/ai-camera-backend/pkg/utils"
	"go.uber.org/zap"
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
// 下载任务
//
// 持久化策略（不改表结构）：
//   - 任务元数据（设备/通道/时间窗/状态）写成 ac_gb_cloud_record 的一行
//     **软删除快照**（deleted_at 非空）。默认查询会自动排除它，所以云端
//     录像列表看不到这条内部记录，不需要改 repository 的查询条件；
//   - 每 10 秒把「已落盘片段的大小/时长合计」快照回这一行，进程重启后
//     可由 Remark 前缀恢复任务列表；
//   - 真实文件信息仍由 ZLM 的 on_record_mp4 回调写成普通的
//     ac_gb_cloud_record 行（deleted_at 为空），那些行才是用户可见的录像。
//
// 为什么不用新表：本批次不允许改表结构；用软删除行承载内部状态既不污染
// 业务查询，也复用了现有索引列（start_time / stream）。
// ════════════════════════════════════════════════════════════════

const (
	downloadStatusDownloading = "downloading"
	downloadStatusStopped     = "stopped"
	downloadStatusCompleted   = "completed"
	downloadStatusInterrupted = "interrupted"

	// downloadSnapshotPrefix 是内部快照行的 Remark 前缀。
	// 格式：gb-download-task:<id>|<status>|<设备录像开始>|<设备录像结束>
	downloadSnapshotPrefix   = "gb-download-task:"
	downloadSnapshotInterval = 10 * time.Second
)

// downloadTask 是进程内的可变任务状态。
type downloadTask struct {
	ID            int64
	ChannelID     int64
	ChannelName   string
	DeviceID      int64
	DeviceName    string
	StreamID      string
	App           string
	Stream        string
	SSRC          string
	StartTime     string
	EndTime       string
	Status        string
	Message       string
	CreatedDT     time.Time
	StoppedDT     *time.Time
	MediaServerID string
	// TotalSeconds 是请求的设备录像时间窗长度，用于计算百分比。
	TotalSeconds float64
	// SnapshotID 是 ac_gb_cloud_record 中承载任务快照的软删除行。
	SnapshotID int64
}

var (
	downloadMu    sync.Mutex
	downloadTasks = map[int64]*downloadTask{}
	downloadSeq   atomic.Int64

	downloadBootOnce sync.Once
)

// NewGbPlaybackService 构造回放服务，并初始化下载任务的恢复/快照循环。
func NewGbPlaybackService() *GbPlaybackService {
	initDownloadTasks()
	return &GbPlaybackService{}
}

// initDownloadTasks 只执行一次：从快照恢复任务 + 启动周期快照。
func initDownloadTasks() {
	downloadBootOnce.Do(func() {
		restoreDownloadTasks(context.Background())
		go downloadSnapshotLoop()
	})
}

// StartDownload 发起下载 INVITE、登记任务并让 ZLM 开始录制。
func (s *GbPlaybackService) StartDownload(ctx context.Context, req *request.GbDownloadReq) (*response.GbDownloadResp, error) {
	if req == nil {
		return nil, ErrPlaybackTimeInvalid
	}
	startTime, endTime, err := validateTimeWindow(req.StartTime, req.EndTime)
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

	ms, err := resolveMediaServerForDevice(ctx, dev)
	if err != nil {
		_ = dlg.Bye(ctx)
		sipSvc.ReleaseSSRC(ssrcStr)
		return nil, err
	}

	now := time.Now()
	totalSeconds := endTime.Sub(startTime).Seconds()
	taskID := downloadSeq.Add(1)

	task := &downloadTask{
		ID:            taskID,
		ChannelID:     ch.ID,
		ChannelName:   ch.Name,
		DeviceID:      dev.ID,
		DeviceName:    dev.Name,
		StreamID:      dlg.CallID,
		App:           dlg.App,
		Stream:        stream,
		SSRC:          ssrcStr,
		StartTime:     req.StartTime,
		EndTime:       req.EndTime,
		Status:        downloadStatusDownloading,
		Message:       "正在建立录制流",
		CreatedDT:     now,
		MediaServerID: ms.ServerID,
		TotalSeconds:  totalSeconds,
	}

	// 先落内部快照。失败只影响「重启后能否恢复任务列表」，不应阻断下载。
	if row, rerr := createDownloadSnapshot(ctx, task, ch, dev, now); rerr != nil {
		utils.Warn("[GB-DOWNLOAD] 写入下载任务快照失败（不影响本次下载）",
			zap.Int64("task_id", taskID), zap.Error(rerr))
	} else {
		task.SnapshotID = row
	}

	// hook 落库时需要把 MP4 归属到设备/通道。
	record.RegisterStreamOwner(record.StreamOwner{
		App:           dlg.App,
		Stream:        stream,
		MediaServerID: ms.ServerID,
		DeviceID:      dev.ID,
		ChannelID:     ch.ID,
	})

	downloadMu.Lock()
	downloadTasks[taskID] = task
	// 在启动后台 goroutine 之前生成响应快照，避免与状态更新并发读写。
	item := task.toItem()
	downloadMu.Unlock()

	maxSecond := int(totalSeconds)
	if maxSecond < 1 {
		maxSecond = 1
	}
	// 设备开始推流有时间差，StartRecord 对尚未创建的流会失败，异步重试。
	go startDownloadRecording(taskID, ms, dlg.App, stream, maxSecond)
	// 到点自动收尾：进程存活时任务一定能结束，不会永远停在 downloading。
	go watchDownload(taskID, now.Add(time.Duration(totalSeconds)*time.Second))

	return &response.GbDownloadResp{GbDownloadItem: item}, nil
}

// PageDownload 分页查询下载任务，并实时补全进度与已落盘大小。
func (s *GbPlaybackService) PageDownload(ctx context.Context, req *request.PageGbDownloadReq, _ *utils.DataScope) (*response.PageGbDownloadResp, error) {
	if req == nil {
		req = &request.PageGbDownloadReq{}
	}
	req.Normalize()

	downloadMu.Lock()
	items := make([]response.GbDownloadItem, 0, len(downloadTasks))
	serverIDs := make([]string, 0, len(downloadTasks))
	for _, t := range downloadTasks {
		items = append(items, t.toItem())
		serverIDs = append(serverIDs, t.MediaServerID)
	}
	downloadMu.Unlock()

	// 进度是查询时计算的：数据库里是已落盘片段的真实值，ZLM 提供当前流的
	// 存活时长估算。任务量很小（单进程补录场景），逐个查询可以接受。
	for i := range items {
		s.fillDownloadProgress(ctx, &items[i], serverIDs[i])
	}

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
	s.stopDownloadTask(ctx, task, downloadStatusStopped)
	return nil
}

// stopDownloadTask 是 StopDownload 与自动收尾共用的停止逻辑。
//
// 顺序：先 stopRecord，再 BYE/close_streams。反过来流会立即消失，
// ZLM 找不到流，最后一个 MP4 片段可能没有正常收尾、on_record_mp4 不回调。
func (s *GbPlaybackService) stopDownloadTask(ctx context.Context, task *downloadTask, status string) {
	downloadMu.Lock()
	if task.Status != downloadStatusDownloading {
		downloadMu.Unlock()
		return
	}
	downloadMu.Unlock()

	if ms, err := resolveMediaServerByKey(ctx, task.MediaServerID); err == nil && ms != nil {
		rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = mediaserver.FromModel(ms).StopRecord(rctx, task.App, task.Stream, "")
		cancel()
	}
	if sipSvc, err := activeSipService(); err == nil {
		if dlg, ok := sipSvc.Sessions().Get(task.StreamID); ok && dlg != nil {
			_ = dlg.Bye(ctx)
			releasePlayStream(ctx, dlg.App, dlg.Stream)
			sipSvc.ReleaseSSRC(task.SSRC)
		}
	}
	// 延迟清理归属，给最后一个 on_record_mp4 回调留出落库时间。
	record.UnregisterStreamOwnerAfter(task.Stream, 15*time.Second)

	now := time.Now()
	downloadMu.Lock()
	task.Status = status
	task.StoppedDT = &now
	if status == downloadStatusCompleted {
		task.Message = "下载完成"
	} else if task.Message == "" || task.Message == "正在建立录制流" || task.Message == "ZLM 录制中" {
		// 保留「无法开始 ZLM 录制: ...」这类诊断信息，别用笼统的「已停止」盖掉。
		task.Message = "已停止"
	}
	downloadMu.Unlock()

	snapshotDownloadTask(context.Background(), task)
}

// watchDownload 在时间窗结束时自动停止任务。
//
// 多留 2 秒：StopRecord 之后 ZLM 才关闭 MP4 并回调 on_record_mp4，
// 立刻做最终快照可能漏掉最后一个片段。
func watchDownload(taskID int64, expectedEnd time.Time) {
	d := time.Until(expectedEnd)
	if d > 0 {
		timer := time.NewTimer(d)
		<-timer.C
		timer.Stop()
	}
	downloadMu.Lock()
	task := downloadTasks[taskID]
	downloadMu.Unlock()
	if task == nil {
		return
	}
	// stopDownloadTask 内部会检查状态，重复调用（用户手动停止）是幂等的。
	(&GbPlaybackService{}).stopDownloadTask(context.Background(), task, downloadStatusCompleted)
	time.Sleep(2 * time.Second)
	snapshotDownloadTask(context.Background(), task)
}

// startDownloadRecording 异步让 ZLM 开始录制下载流。
//
// maxSecond 设置为整个时间窗：ZLM 到点会自动切分片段并回调 on_record_mp4，
// 进度与文件大小因此能分段得到真实值。
func startDownloadRecording(taskID int64, ms *model.GbMediaServer, app, stream string, maxSecond int) {
	client := mediaserver.FromModel(ms)
	var lastErr error
	for i := 0; i < 5; i++ {
		time.Sleep(time.Second)
		rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		lastErr = client.StartRecord(rctx, app, stream, "", maxSecond)
		cancel()
		if lastErr == nil {
			setDownloadMessage(taskID, "ZLM 录制中")
			return
		}
	}
	setDownloadMessage(taskID, "无法开始 ZLM 录制: "+lastErr.Error())
	// 录制没起来：把会话也收掉，避免设备一直推流到无人消费的端口。
	if t := taskByID(taskID); t != nil {
		(&GbPlaybackService{}).stopDownloadTask(context.Background(), t, downloadStatusStopped)
	}
}

// setDownloadMessage 更新任务的提示文案。
func setDownloadMessage(taskID int64, msg string) {
	downloadMu.Lock()
	if t := downloadTasks[taskID]; t != nil && t.Status == downloadStatusDownloading {
		t.Message = msg
	}
	downloadMu.Unlock()
}

// ── 任务 → 响应 ───────────────────────────────────────────────

func (t *downloadTask) toItem() response.GbDownloadItem {
	if t == nil {
		return response.GbDownloadItem{}
	}
	return response.GbDownloadItem{
		ID:          t.ID,
		ChannelID:   t.ChannelID,
		ChannelName: t.ChannelName,
		DeviceID:    t.DeviceID,
		DeviceName:  t.DeviceName,
		StreamID:    t.StreamID,
		App:         t.App,
		Stream:      t.Stream,
		SSRC:        t.SSRC,
		StartTime:   t.StartTime,
		EndTime:     t.EndTime,
		Status:      t.Status,
		FilePath:    "",
		Message:     t.Message,
		CreatedDT:   t.CreatedDT,
		StoppedDT:   t.StoppedDT,
	}
}

// fillDownloadProgress 补齐单条任务的进度与文件大小。
//
// 数据来源：
//   - file_size / time_len：ac_gb_cloud_record 中该流的普通行（on_record_mp4
//     写入），是真实值；
//   - 当前片段：ZLM getMediaList 的 aliveSecond，只能作为估算；
//   - 两者都拿不到时 Progress 保持 nil（JSON null），前端显示「未知」，
//     绝不能填 0 —— 那会把「设备没推流」误报成「刚开始下载」。
func (s *GbPlaybackService) fillDownloadProgress(ctx context.Context, item *response.GbDownloadItem, mediaServerID string) {
	db := bootstrap.DB()
	if db == nil || item == nil || strings.TrimSpace(item.Stream) == "" {
		return
	}

	type aggRow struct {
		Size    int64 `gorm:"column:size"`
		Seconds int64 `gorm:"column:seconds"`
	}
	var agg aggRow
	err := db.WithContext(ctx).
		Table("ac_gb_cloud_record").
		Select("COALESCE(SUM(file_size),0) AS size, COALESCE(SUM(time_len),0) AS seconds").
		Where("stream = ? AND deleted_at IS NULL", item.Stream).
		Where("remark IS NULL OR remark NOT LIKE ?", downloadSnapshotPrefix+"%").
		Scan(&agg).Error
	if err != nil {
		utils.Warn("[GB-DOWNLOAD] 汇总已落盘录像失败",
			zap.String("stream", item.Stream), zap.Error(err))
	}

	if agg.Size > 0 {
		item.FileSize = agg.Size
	}

	var latest struct {
		FilePath string `gorm:"column:file_path"`
	}
	_ = db.WithContext(ctx).
		Table("ac_gb_cloud_record").
		Select("file_path").
		Where("stream = ? AND deleted_at IS NULL", item.Stream).
		Where("remark IS NULL OR remark NOT LIKE ?", downloadSnapshotPrefix+"%").
		Order("start_time DESC NULLS LAST, id DESC").
		Limit(1).Scan(&latest).Error
	if latest.FilePath != "" {
		item.FilePath = latest.FilePath
	}

	total := 0.0
	if st, ok := parseGbTime(item.StartTime); ok {
		if et, ok2 := parseGbTime(item.EndTime); ok2 {
			total = et.Sub(st).Seconds()
		}
	}

	liveSeconds := 0.0
	liveKnown := false
	if item.Status == downloadStatusDownloading {
		if ms, err := resolveMediaServerByKey(ctx, mediaServerID); err == nil && ms != nil {
			qctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			list, lerr := mediaserver.FromModel(ms).GetMediaList(qctx, item.App, item.Stream, "", "")
			cancel()
			if lerr == nil {
				for _, m := range list {
					if m.Stream == "" || m.Stream == item.Stream {
						liveSeconds = float64(m.AliveSecond)
						liveKnown = true
						break
					}
				}
			}
		}
	}

	final := item.Status != downloadStatusDownloading
	p, known := record.ComputeProgress(record.ProgressInput{
		CompletedSeconds: float64(agg.Seconds),
		LiveSeconds:      liveSeconds,
		LiveKnown:        liveKnown,
		TotalSeconds:     total,
		Final:            final,
	})
	if known {
		item.Progress = &p
	} else {
		item.Progress = nil
	}
}

// ── 持久化快照 ────────────────────────────────────────────────

// formatDownloadRemark 生成内部快照行的 Remark。
func formatDownloadRemark(id int64, status, startTime, endTime string) string {
	return fmt.Sprintf("%s%d|%s|%s|%s", downloadSnapshotPrefix, id, status, startTime, endTime)
}

// createDownloadSnapshot 写一条**软删除**的 ac_gb_cloud_record 作为任务快照。
//
// 软删除（deleted_at 非空）是这里的关键：GORM 的默认查询会自动排除它，
// 因此云端录像列表/日期列表不会被内部任务行污染，无需改 repository。
func createDownloadSnapshot(ctx context.Context, t *downloadTask, ch *model.GbChannel, dev *model.Device, now time.Time) (int64, error) {
	db := bootstrap.DB()
	if db == nil {
		return 0, errors.New("数据库未初始化")
	}
	end := now.Add(time.Duration(t.TotalSeconds) * time.Second)
	deviceID, channelID := dev.ID, ch.ID
	row := &model.GbCloudRecord{
		App:           t.App,
		Stream:        t.Stream,
		CallID:        t.StreamID,
		DeviceID:      &deviceID,
		ChannelID:     &channelID,
		StartTime:     &now,
		EndTime:       &end,
		MediaServerID: t.MediaServerID,
		Remark:        formatDownloadRemark(t.ID, t.Status, t.StartTime, t.EndTime),
		DeptID:        ch.DeptID,
	}
	// 先创建再软删除：直接带 DeletedAt Create 也可以，但显式两步更清晰，
	// 且能确保 created_dt 由数据库默认值/钩子正确填充。
	if err := db.WithContext(ctx).Create(row).Error; err != nil {
		return 0, err
	}
	if err := db.WithContext(ctx).Delete(&model.GbCloudRecord{}, row.ID).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// updateDownloadSnapshot 用 Unscoped 更新软删除行（默认作用域会过滤掉它）。
func updateDownloadSnapshot(ctx context.Context, id int64, updates map[string]any) error {
	db := bootstrap.DB()
	if db == nil || id <= 0 {
		return nil
	}
	return db.WithContext(ctx).Unscoped().Model(&model.GbCloudRecord{}).
		Where("id = ?", id).Updates(updates).Error
}

// snapshotDownloadTask 把任务状态与已落盘片段的合计写回快照行。
//
// 先在锁内拷贝可变字段，再做数据库/网络操作：避免长时间持锁，
// 也避免与 StopDownload/定时快照并发读写 task 字段。
func snapshotDownloadTask(ctx context.Context, t *downloadTask) {
	if t == nil {
		return
	}
	downloadMu.Lock()
	id, snapID, stream := t.ID, t.SnapshotID, t.Stream
	status, startTime, endTime := t.Status, t.StartTime, t.EndTime
	downloadMu.Unlock()

	if snapID <= 0 {
		return
	}
	size, seconds, latest := aggregateStreamRecords(ctx, stream)
	updates := map[string]any{
		"remark":     formatDownloadRemark(id, status, startTime, endTime),
		"file_size":  size,
		"time_len":   seconds,
		"updated_dt": time.Now(),
	}
	if latest != "" {
		updates["file_path"] = latest
	}
	if err := updateDownloadSnapshot(ctx, snapID, updates); err != nil {
		utils.Warn("[GB-DOWNLOAD] 更新下载任务快照失败",
			zap.Int64("task_id", id), zap.Error(err))
	}
}

// aggregateStreamRecords 汇总某个流已落盘录像的大小、时长与最近文件路径。
func aggregateStreamRecords(ctx context.Context, stream string) (size, seconds int64, latest string) {
	db := bootstrap.DB()
	if db == nil || strings.TrimSpace(stream) == "" {
		return 0, 0, ""
	}
	type aggRow struct {
		Size    int64 `gorm:"column:size"`
		Seconds int64 `gorm:"column:seconds"`
	}
	var agg aggRow
	_ = db.WithContext(ctx).
		Table("ac_gb_cloud_record").
		Select("COALESCE(SUM(file_size),0) AS size, COALESCE(SUM(time_len),0) AS seconds").
		Where("stream = ? AND deleted_at IS NULL", stream).
		Where("remark IS NULL OR remark NOT LIKE ?", downloadSnapshotPrefix+"%").
		Scan(&agg).Error
	var row struct {
		FilePath string `gorm:"column:file_path"`
	}
	_ = db.WithContext(ctx).
		Table("ac_gb_cloud_record").Select("file_path").
		Where("stream = ? AND deleted_at IS NULL", stream).
		Where("remark IS NULL OR remark NOT LIKE ?", downloadSnapshotPrefix+"%").
		Order("start_time DESC NULLS LAST, id DESC").Limit(1).Scan(&row).Error
	return agg.Size, agg.Seconds, row.FilePath
}

// downloadSnapshotLoop 周期性把内存状态刷到快照行。
func downloadSnapshotLoop() {
	ticker := time.NewTicker(downloadSnapshotInterval)
	defer ticker.Stop()
	for range ticker.C {
		downloadMu.Lock()
		tasks := make([]*downloadTask, 0, len(downloadTasks))
		for _, t := range downloadTasks {
			// 同一个锁内筛选，避免快照已结束任务的过期状态。
			if t.Status == downloadStatusDownloading {
				tasks = append(tasks, t)
			}
		}
		downloadMu.Unlock()
		for _, t := range tasks {
			snapshotDownloadTask(context.Background(), t)
		}
	}
}

// restoreDownloadTasks 从软删除快照恢复任务列表。
//
// 只在进程启动时执行一次。恢复出来的 downloading 任务被标记为 interrupted：
// 原 SIP 会话与 ZLM 录制已经随上个进程结束，不能假装还在录。
func restoreDownloadTasks(ctx context.Context) {
	db := bootstrap.DB()
	if db == nil {
		return
	}
	// 清理 30 天前的内部快照（物理删除）：它们不属于用户数据，
	// 留着只会让 ac_gb_cloud_record 随下载次数无限增长。
	_ = db.WithContext(ctx).Unscoped().
		Where("remark LIKE ?", downloadSnapshotPrefix+"%").
		Where("start_time IS NOT NULL AND start_time < ?", time.Now().AddDate(0, 0, -30)).
		Delete(&model.GbCloudRecord{}).Error

	var rows []model.GbCloudRecord
	err := db.WithContext(ctx).Unscoped().
		Preload("Device").Preload("Channel").
		Where("remark LIKE ?", downloadSnapshotPrefix+"%").
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		utils.Warn("[GB-DOWNLOAD] 恢复下载任务失败", zap.Error(err))
		return
	}

	for i := range rows {
		r := rows[i]
		id, status, startTime, endTime, ok := parseDownloadRemark(r.Remark)
		if !ok || id <= 0 {
			continue
		}

		task := &downloadTask{
			ID:           id,
			StreamID:     r.CallID,
			App:          r.App,
			Stream:       r.Stream,
			StartTime:    startTime,
			EndTime:      endTime,
			Status:       status,
			Message:      "从快照恢复",
			CreatedDT:    time.Now(),
			SnapshotID:   r.ID,
			TotalSeconds: 0,
		}
		if r.DeviceID != nil {
			task.DeviceID = *r.DeviceID
		}
		if r.ChannelID != nil {
			task.ChannelID = *r.ChannelID
		}
		if r.Device != nil {
			task.DeviceName = r.Device.Name
		}
		if r.Channel != nil {
			task.ChannelName = r.Channel.Name
		}
		if r.StartTime != nil {
			task.CreatedDT = *r.StartTime
		}
		if r.MediaServerID != "" {
			task.MediaServerID = r.MediaServerID
		}
		if st, ok := parseGbTime(startTime); ok {
			if et, ok2 := parseGbTime(endTime); ok2 {
				task.TotalSeconds = et.Sub(st).Seconds()
			}
		}
		if ssrcStr, serr := ssrc.FromHexStream(r.Stream); serr == nil {
			task.SSRC = ssrcStr
		}

		if status == downloadStatusDownloading {
			task.Status = downloadStatusInterrupted
			task.Message = "进程重启，下载会话已中断"
			_ = updateDownloadSnapshot(context.Background(), r.ID, map[string]any{
				"remark": formatDownloadRemark(task.ID, task.Status, task.StartTime, task.EndTime),
			})
		}

		downloadMu.Lock()
		if _, exists := downloadTasks[id]; !exists {
			downloadTasks[id] = task
		}
		downloadMu.Unlock()

		for {
			cur := downloadSeq.Load()
			if cur >= id || downloadSeq.CompareAndSwap(cur, id) {
				break
			}
		}
	}
}

// parseDownloadRemark 解析快照 Remark。
func parseDownloadRemark(v string) (id int64, status, startTime, endTime string, ok bool) {
	if !strings.HasPrefix(v, downloadSnapshotPrefix) {
		return 0, "", "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(v, downloadSnapshotPrefix), "|", 4)
	if len(parts) != 4 {
		return 0, "", "", "", false
	}
	if _, err := fmt.Sscanf(parts[0], "%d", &id); err != nil {
		return 0, "", "", "", false
	}
	return id, parts[1], parts[2], parts[3], true
}

// taskByID 返回任务快照（不存在时 nil）。
func taskByID(id int64) *downloadTask {
	downloadMu.Lock()
	defer downloadMu.Unlock()
	return downloadTasks[id]
}
