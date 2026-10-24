// Package diagnostics 提供国标设备的可达性 / NAT 诊断。
//
// 背景：GB28181 现场最常见的两类「灵异故障」是
//
//  1. 设备能注册、目录也能同步，但点播黑屏 —— SIP 通了，RTP 打到了
//     设备不可达的地址（SDP 通告了 127.0.0.1 或容器 NAT 地址）；
//  2. 平台显示在线，但运维不知道是「设备没发心跳」还是「平台没收到」。
//
// 这个包把排障时人工要看的十几项信息（建档、在线、注册有效期、内存中的
// SIP 端点、最近心跳、SDP 通告地址、ZLM 可达性）收敛成一次 POST 请求，
// 每项给出中文结论。**只读 + 只发一条标准 DeviceInfo 查询**，不修改设备
// 状态、不做端口扫描。
//
// 设计约束：诊断结果里的「跳过（skipped）」与「失败（failed）」必须区分。
// 跳过表示当前部署形态下无法检查（例如 API 进程没有内嵌 SIP 服务），
// 把它当成失败会让运维误判；反过来把失败当跳过则会掩盖真问题。
package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/gb28181/control"
	"github.com/boticz/ai-camera-backend/internal/gb28181/device"
	"github.com/boticz/ai-camera-backend/internal/gb28181/metrics"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm"
)

// ErrDeviceNotFound 设备不存在（或已软删除）。
//
// 单独定义而不复用 service 层的错误，是为了让 diagnostics 保持低耦合：
// 这个包只依赖 model / gorm / 信令运行时接口，不依赖具体业务服务。
var ErrDeviceNotFound = errors.New("国标设备不存在")

// 检查项状态。语义见包注释。
const (
	StatusOK      = "ok"
	StatusWarning = "warn"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// Item 是单项检查结果。
//
// OK 的取值：
//   - ok / warn → true（warn 表示能工作但存在风险，不算失败）
//   - failed / skipped → false
//
// Status 与 OK 同时返回，是为了让前端/脚本既能做二元判断，也能区分
// 「失败」与「无法检查」。只返回 ok=false 会把两类问题混在一起。
type Item struct {
	Name   string `json:"name" example:"SIP 信令地址"`
	OK     bool   `json:"ok" example:"true"`
	Status string `json:"status" example:"ok"` // ok / warn / failed / skipped
	Detail string `json:"detail" example:"设备最近一次报文来自 192.168.1.50:5060（UDP）"`
}

// Report 一次诊断的完整结果。
type Report struct {
	DeviceID   int64     `json:"device_id" example:"12"`
	GbDeviceID string    `json:"gb_device_id" example:"34020000001320000001"`
	CheckedAt  time.Time `json:"checked_at"`
	Items      []Item    `json:"items"`
	Summary    string    `json:"summary" example:"共 10 项：正常 7，警告 2，失败 1，跳过 0。存在 1 项失败，请按详情排查。"`
}

// RReport 是 Swagger 用的响应包装类型。
//
// 实际响应由 utils.Success 生成，结构与本类型一致：{code, message, data}。
// 单独定义而不复用 response.BaseResponse 是历史原因（BaseResponse 的 Data
// 是 interface{}），这里用具体类型才能让生成的 TypeScript 有强类型。
type RReport struct {
	Code    int    `json:"code" example:"200"`
	Message string `json:"message,omitempty"`
	Data    Report `json:"data"`
}

// Options 是诊断服务的可替换依赖。
//
// 全部做成可注入字段，单测可以在不启动 SIP / ZLM 的前提下覆盖
// ok / warn / failed / skipped 四条分支。
type Options struct {
	DB              *gorm.DB
	Now             func() time.Time
	Endpoint        func(gbDeviceID string) (device.Endpoint, bool)
	QueryDeviceInfo func(ctx context.Context, dev *model.Device) error
	ProbeZLM        func(ctx context.Context, server *model.GbMediaServer) (bool, string)
	MediaPublicIP   string
	ProbeTimeout    time.Duration
}

// Service 设备诊断服务。
type Service struct {
	opts Options
}

// NewService 构造诊断服务，使用全局 DB 与进程内 SIP 运行时。
func NewService() *Service { return NewServiceWithOptions(Options{}) }

// NewServiceWithOptions 构造诊断服务并补齐缺省依赖。
func NewServiceWithOptions(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.ProbeTimeout <= 0 {
		o.ProbeTimeout = 3 * time.Second
	}

	if o.MediaPublicIP == "" {
		if cfg := bootstrap.Config(); cfg != nil {
			o.MediaPublicIP = strings.TrimSpace(cfg.GB28181.Media.PublicIP)
		}
	}
	if o.ProbeZLM == nil {
		o.ProbeZLM = defaultProbeZLM
	}
	return &Service{opts: o}
}

// runtimeHooks 从进程内信令控制器上取两个可选能力。
//
// 用匿名接口而非具体类型断言：既能工作于 *sipserver.Service，
// 也不会因为信令实现新增方法而被动改变依赖。
func runtimeHooks() (
	func(string) (device.Endpoint, bool),
	func(context.Context, *model.Device) error,
) {
	ctl := control.Default()
	if ctl == nil {
		return nil, nil
	}

	var endpoint func(string) (device.Endpoint, bool)
	if p, ok := ctl.(interface{ Devices() *device.Manager }); ok {
		if mgr := p.Devices(); mgr != nil {
			endpoint = mgr.Endpoint
		}
	}

	var query func(context.Context, *model.Device) error
	if q, ok := ctl.(interface {
		QueryDeviceInfo(context.Context, *model.Device) error
	}); ok {
		query = q.QueryDeviceInfo
	}
	return endpoint, query
}

func (s *Service) db() *gorm.DB {
	if s.opts.DB != nil {
		return s.opts.DB
	}
	return bootstrap.DB()
}

// sipRuntime 在**请求时**解析进程内 SIP 能力。
//
// ★ 为什么不能放在构造时解析：
//
//	main() 里 `router.SetupRouter()` 先执行，`sipserver.New/Start` 后执行，
//	`control.Default()` 在构造 Handler 时还是 nil。若那时把 nil 缓存进 Service，
//	内嵌 SIP 启动后诊断仍会永远显示 skipped。
//
// 这里每次诊断重新取一次（代价是一次接口断言），既兼容「HTTP 进程先起、
// 信令后起」，也兼容测试里显式注入的 Option。
func (s *Service) sipRuntime() (
	func(string) (device.Endpoint, bool),
	func(context.Context, *model.Device) error,
) {
	endpoint := s.opts.Endpoint
	query := s.opts.QueryDeviceInfo
	if endpoint != nil && query != nil {
		return endpoint, query
	}
	ep, q := runtimeHooks()
	if endpoint == nil {
		endpoint = ep
	}
	if query == nil {
		query = q
	}
	return endpoint, query
}

// Diagnose 对一台国标设备执行全量检查。
func (s *Service) Diagnose(ctx context.Context, deviceID int64) (*Report, error) {
	db := s.db()
	if db == nil {
		return nil, errors.New("数据库未初始化，无法执行诊断")
	}

	var dev model.Device
	if err := db.WithContext(ctx).First(&dev, deviceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeviceNotFound
		}
		return nil, err
	}

	rep := &Report{
		DeviceID:   dev.ID,
		GbDeviceID: dev.GbDeviceID,
		CheckedAt:  s.opts.Now(),
		Items:      make([]Item, 0, 10),
	}

	endpoint, queryDeviceInfo := s.sipRuntime()

	rep.Items = append(rep.Items, s.checkProfile(&dev))
	rep.Items = append(rep.Items, checkGbID(&dev))
	rep.Items = append(rep.Items, checkOnline(&dev))
	rep.Items = append(rep.Items, s.checkExpires(&dev))
	rep.Items = append(rep.Items, s.checkEndpoint(&dev, endpoint))
	rep.Items = append(rep.Items, s.checkLastSeen(&dev))
	rep.Items = append(rep.Items, s.checkSDP(&dev))
	rep.Items = append(rep.Items, s.probeDevice(ctx, &dev, endpoint, queryDeviceInfo))

	// ZLM 相关两项：可达性（主动 HTTP 探测）与心跳（数据库里的在线标记）。
	// 分成两项是有意的：网络可达但 hook 心跳停了，说明 ZLM 活着但回调链路断了，
	// 与「ZLM 根本没起来」是两种完全不同的故障。
	zlmItem, hbItem := s.checkMediaServer(ctx, &dev)
	rep.Items = append(rep.Items, zlmItem, hbItem)

	rep.Summary = summarize(rep.Items)
	metrics.IncDiagnose(classify(rep.Items))
	return rep, nil
}

func (s *Service) checkProfile(dev *model.Device) Item {
	detail := fmt.Sprintf("已建档：%s（设备编号 %s，ID=%d）", dev.Name, dev.DeviceCode, dev.ID)
	return Item{Name: "设备建档", OK: true, Status: StatusOK, Detail: detail}
}

func checkGbID(dev *model.Device) Item {
	if strings.TrimSpace(dev.GbDeviceID) == "" {
		return Item{
			Name:   "国标编码",
			OK:     false,
			Status: StatusFailed,
			Detail: "设备未配置 20 位国标编码，SIP 注册会被拒绝（403）。请先生成激活凭证并写入设备。",
		}
	}
	return Item{
		Name:   "国标编码",
		OK:     true,
		Status: StatusOK,
		Detail: "国标编码 " + dev.GbDeviceID,
	}
}

func checkOnline(dev *model.Device) Item {
	if dev.Status == model.DeviceStatusOnline {
		return Item{Name: "运行状态", OK: true, Status: StatusOK, Detail: "平台记录为在线（online）"}
	}
	return Item{
		Name:   "运行状态",
		OK:     false,
		Status: StatusWarning,
		Detail: fmt.Sprintf("平台记录为离线（%s）。离线不等于不可达，先看下一项注册有效期与最近心跳。", dev.Status),
	}
}

func (s *Service) checkExpires(dev *model.Device) Item {
	now := s.opts.Now()
	if dev.SipExpires == nil {
		if dev.RegisterStatus == model.GbRegisterStatusPending {
			return Item{
				Name:   "SIP 注册有效期",
				OK:     false,
				Status: StatusWarning,
				Detail: "尚无注册记录（register_status=pending），设备从未成功注册过。",
			}
		}
		return Item{
			Name:   "SIP 注册有效期",
			OK:     false,
			Status: StatusFailed,
			Detail: "没有注册有效期（sip_expires 为空），设备已注销或从未注册。",
		}
	}

	left := dev.SipExpires.Sub(now)
	if left <= 0 {
		return Item{
			Name:   "SIP 注册有效期",
			OK:     false,
			Status: StatusFailed,
			Detail: fmt.Sprintf("注册已于 %s 过期（已过期 %s）。设备需要重新 REGISTER。",
				dev.SipExpires.Format("2006-01-02 15:04:05"), humanDuration(-left)),
		}
	}
	return Item{
		Name:   "SIP 注册有效期",
		OK:     true,
		Status: StatusOK,
		Detail: fmt.Sprintf("有效期至 %s，剩余 %s。", dev.SipExpires.Format("2006-01-02 15:04:05"), humanDuration(left)),
	}
}

func (s *Service) checkEndpoint(dev *model.Device, endpoint func(string) (device.Endpoint, bool)) Item {
	if endpoint == nil {
		return Item{
			Name:   "SIP 信令地址",
			OK:     false,
			Status: StatusSkipped,
			Detail: "当前进程未启动 SIP 信令服务（GB28181_EMBEDDED=false 的纯 API 进程），无法读取内存中的设备端点。",
		}
	}
	if strings.TrimSpace(dev.GbDeviceID) == "" {
		return Item{
			Name:   "SIP 信令地址",
			OK:     false,
			Status: StatusSkipped,
			Detail: "设备没有国标编码，无法按编码查询 SIP 端点。",
		}
	}
	ep, ok := endpoint(dev.GbDeviceID)
	if !ok {
		return Item{
			Name:   "SIP 信令地址",
			OK:     false,
			Status: StatusFailed,
			Detail: "内存中没有该设备的源地址。设备可能尚未注册，或平台重启后还没收到它的下一条报文。",
		}
	}
	age := s.opts.Now().Sub(ep.UpdatedAt)
	return Item{
		Name:   "SIP 信令地址",
		OK:     true,
		Status: StatusOK,
		Detail: fmt.Sprintf("最近一次报文来自 %s（%s），%s前更新。", ep.Addr, ep.Transport, humanDuration(age)),
	}
}

func (s *Service) checkLastSeen(dev *model.Device) Item {
	now := s.opts.Now()
	last := dev.LastBeatDT
	source := "心跳"
	if last == nil {
		last = dev.RegisterDT
		source = "注册"
	}
	if last == nil {
		return Item{
			Name:   "最近注册/心跳",
			OK:     false,
			Status: StatusSkipped,
			Detail: "从未记录到注册或心跳时间，无法判断链路是否存活。",
		}
	}

	interval := dev.InfoInterval
	if interval <= 0 {
		interval = 60
	}
	threshold := time.Duration(interval) * time.Second * 3
	if threshold < 3*time.Minute {
		threshold = 3 * time.Minute
	}
	age := now.Sub(*last)
	if age < 0 {
		age = 0
	}

	if age > threshold {
		return Item{
			Name:   "最近注册/心跳",
			OK:     false,
			Status: StatusWarning,
			Detail: fmt.Sprintf("最近一次%s在 %s（%s前），已超过阈值 %s。设备可能离线或网络中断。",
				source, last.Format("2006-01-02 15:04:05"), humanDuration(age), humanDuration(threshold)),
		}
	}
	return Item{
		Name:   "最近注册/心跳",
		OK:     true,
		Status: StatusOK,
		Detail: fmt.Sprintf("最近一次%s在 %s（%s前），阈值 %s。",
			source, last.Format("2006-01-02 15:04:05"), humanDuration(age), humanDuration(threshold)),
	}
}

// checkSDP 检查 SDP 里通告给设备的收流地址。
//
// 地址优先级与信令层一致：设备级 sdp_ip 覆盖 > 全局 media.public_ip。
// 这里不做 DNS 解析：设备拿到的是 IP 字面量，主机名/域名不在本期支持范围内。
func (s *Service) checkSDP(dev *model.Device) Item {
	ip := strings.TrimSpace(dev.SDPIP)
	source := "设备 sdp_ip"
	if ip == "" {
		ip = strings.TrimSpace(s.opts.MediaPublicIP)
		source = "gb28181.media.public_ip"
	}
	if ip == "" {
		return Item{
			Name:   "SDP 通告地址",
			OK:     false,
			Status: StatusFailed,
			Detail: "未配置 SDP 通告地址（设备 sdp_ip 与 gb28181.media.public_ip 均为空）。设备不知道往哪里推 RTP，点播会黑屏。",
		}
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return Item{
			Name:   "SDP 通告地址",
			OK:     false,
			Status: StatusFailed,
			Detail: fmt.Sprintf("%s=%q 不是合法 IP。设备侧无法解析主机名，必须填 IP 字面量。", source, ip),
		}
	}

	switch {
	case parsed.IsLoopback():
		return Item{
			Name:   "SDP 通告地址",
			OK:     false,
			Status: StatusFailed,
			Detail: fmt.Sprintf("%s=%s 是回环地址，真机绝对不可达（设备会把 RTP 发给自己）。请改成设备可达的局域网 IP 或公网映射地址。", source, ip),
		}
	case parsed.IsUnspecified():
		return Item{
			Name:   "SDP 通告地址",
			OK:     false,
			Status: StatusFailed,
			Detail: fmt.Sprintf("%s=%s 是未指定地址（0.0.0.0），设备无法据此推流。", source, ip),
		}
	case parsed.IsPrivate():
		return Item{
			Name:   "SDP 通告地址",
			OK:     true,
			Status: StatusWarning,
			Detail: fmt.Sprintf("%s=%s 是私网地址。仅当设备与平台同网段、或已做 NAT 端口映射（10000/udp+tcp）时可达；跨公网部署请改为公网可达地址。", source, ip),
		}
	case parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast():
		return Item{
			Name:   "SDP 通告地址",
			OK:     true,
			Status: StatusWarning,
			Detail: fmt.Sprintf("%s=%s 是链路本地地址，通常只在同一二层网段内可达。", source, ip),
		}
	default:
		return Item{
			Name:   "SDP 通告地址",
			OK:     true,
			Status: StatusOK,
			Detail: fmt.Sprintf("%s=%s 是公网地址，请确认 10000/udp+tcp 已放通到 ZLM。", source, ip),
		}
	}
}

// probeDevice 用一条标准 DeviceInfo 查询验证「设备此刻是否响应」。
//
// 为什么用 DeviceInfo 而不是 ICMP / TCP 探测：
//   - 设备在内网/NAT 后面，平台侧主动拨号往往不可达；
//   - 能收到 SIP 响应才真正说明「注册地址与鉴权链路都活着」；
//   - 这是现有信令能力，不需要新增网络权限，也不会产生额外副作用。
func (s *Service) probeDevice(
	ctx context.Context,
	dev *model.Device,
	endpoint func(string) (device.Endpoint, bool),
	queryDeviceInfo func(context.Context, *model.Device) error,
) Item {
	name := "设备可达性探测"
	if queryDeviceInfo == nil || endpoint == nil {
		return Item{
			Name:   name,
			OK:     false,
			Status: StatusSkipped,
			Detail: "未启用 SIP 信令服务，跳过主动探测。",
		}
	}
	if strings.TrimSpace(dev.GbDeviceID) == "" {
		return Item{Name: name, OK: false, Status: StatusSkipped, Detail: "设备没有国标编码，跳过探测。"}
	}
	if _, ok := endpoint(dev.GbDeviceID); !ok {
		return Item{Name: name, OK: false, Status: StatusSkipped, Detail: "SIP 端点未知，跳过探测（先解决注册问题）。"}
	}

	probeCtx, cancel := context.WithTimeout(ctx, s.opts.ProbeTimeout)
	defer cancel()

	start := s.opts.Now()
	err := queryDeviceInfo(probeCtx, dev)
	elapsed := s.opts.Now().Sub(start)
	if err != nil {
		return Item{
			Name:   name,
			OK:     false,
			Status: StatusFailed,
			Detail: fmt.Sprintf("已向设备发送 DeviceInfo 查询，但在 %s 内未收到有效响应：%v。设备可能离线、SIP 地址已失效或网络被阻断。", humanDuration(elapsed), err),
		}
	}
	return Item{
		Name:   name,
		OK:     true,
		Status: StatusOK,
		Detail: fmt.Sprintf("已下发 DeviceInfo 查询并收到设备响应（耗时 %s）。", humanDuration(elapsed)),
	}
}

// checkMediaServer 返回「ZLM 可达性」与「ZLM 心跳」两项。
func (s *Service) checkMediaServer(ctx context.Context, dev *model.Device) (Item, Item) {
	reachName := "ZLM 节点可达性"
	beatName := "ZLM 节点心跳"

	ms, err := s.findMediaServer(ctx, dev)
	if err != nil {
		return Item{Name: reachName, OK: false, Status: StatusFailed, Detail: "查询媒体节点失败：" + err.Error()},
			Item{Name: beatName, OK: false, Status: StatusSkipped, Detail: "媒体节点查询失败，跳过心跳检查。"}
	}
	if ms == nil {
		return Item{
				Name:   reachName,
				OK:     false,
				Status: StatusSkipped,
				Detail: "未配置媒体节点（ac_gb_media_server 为空）。国标视频不可用；请先添加 ZLM 节点并 seed 默认节点。",
			}, Item{
				Name:   beatName,
				OK:     false,
				Status: StatusSkipped,
				Detail: "没有媒体节点，跳过心跳检查。",
			}
	}

	// 主动探测：短超时，避免诊断接口被一个失联的 ZLM 拖住。
	probeCtx, cancel := context.WithTimeout(ctx, s.opts.ProbeTimeout)
	defer cancel()
	ok, detail := s.opts.ProbeZLM(probeCtx, ms)

	reachStatus := StatusOK
	reachDetail := fmt.Sprintf("节点 %s（%s）：%s", ms.Name, ms.ServerID, detail)
	if !ok {
		reachStatus = StatusFailed
		reachDetail = fmt.Sprintf("节点 %s（%s）不可达：%s", ms.Name, ms.ServerID, detail)
	}
	reachItem := Item{Name: reachName, OK: ok, Status: reachStatus, Detail: reachDetail}

	// 心跳项只读数据库，不做网络探测；它与上面的可达性互补。
	beatItem := Item{Name: beatName}
	if ms.Online != nil && *ms.Online {
		beatItem.OK = true
		beatItem.Status = StatusOK
		beatItem.Detail = "节点标记为在线"
		if ms.LastKeepaliveDT != nil {
			age := s.opts.Now().Sub(*ms.LastKeepaliveDT)
			beatItem.Detail = fmt.Sprintf("节点标记为在线，最近一次 hook 心跳在 %s（%s前）。",
				ms.LastKeepaliveDT.Format("2006-01-02 15:04:05"), humanDuration(age))
		}
	} else {
		beatItem.OK = false
		beatItem.Status = StatusWarning
		beatItem.Detail = "节点标记为离线（online=false）。若上文可达性探测成功，说明 ZLM 活着但 hook 回调没到平台，检查 ZLM_HOOK_BASE 与网络策略。"
	}
	return reachItem, beatItem
}

// findMediaServer 解析设备实际使用的 ZLM 节点。
//
// 规则：设备指定了具体 server_id → 用它；否则取默认节点；再退化为最早创建的一个。
// 这比「只看 default_server」更贴近实际：多节点部署时设备会绑定到具体节点。
func (s *Service) findMediaServer(ctx context.Context, dev *model.Device) (*model.GbMediaServer, error) {
	db := s.db()
	if db == nil {
		return nil, errors.New("数据库未初始化")
	}

	activeCond := "(active IS NULL OR active = true)"

	if id := strings.TrimSpace(dev.MediaServerID); id != "" && id != "auto" {
		var m model.GbMediaServer
		err := db.WithContext(ctx).
			Where("server_id = ? AND "+activeCond, id).
			First(&m).Error
		if err == nil {
			return &m, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		// 指定的节点不存在时继续走默认节点，并在下面通过 server_id 体现差异。
	}

	var m model.GbMediaServer
	err := db.WithContext(ctx).
		Where("default_server = true AND " + activeCond).
		Order("id ASC").First(&m).Error
	if err == nil {
		return &m, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = db.WithContext(ctx).
		Where(activeCond).
		Order("id ASC").First(&m).Error
	if err == nil {
		return &m, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

// defaultProbeZLM 调用 ZLM 的 getServerConfig 验证节点可达且 secret 正确。
func defaultProbeZLM(ctx context.Context, m *model.GbMediaServer) (bool, string) {
	if m == nil {
		return false, "节点为空"
	}
	host := strings.TrimSpace(m.IP)
	if host == "" {
		return false, "节点未配置 IP"
	}
	port := m.HTTPPort
	if port <= 0 {
		port = 80
	}

	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, strconv.FormatInt(port, 10)),
		Path:   "/index/api/getServerConfig",
	}
	if m.Secret != "" {
		q := u.Query()
		q.Set("secret", m.Secret)
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, "构造请求失败：" + err.Error()
	}
	client := &http.Client{Timeout: 3 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return false, fmt.Sprintf("HTTP 请求失败（%s）：%v", elapsed.Round(time.Millisecond), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, "读取响应失败：" + err.Error()
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("HTTP %d（%s）", resp.StatusCode, elapsed.Round(time.Millisecond))
	}

	var out struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, "响应不是合法 JSON：" + err.Error()
	}
	if out.Code != 0 {
		return false, fmt.Sprintf("ZLM 返回 code=%d msg=%s（多为 secret 不一致）", out.Code, out.Msg)
	}

	version := ""
	mediaID := ""
	var data map[string]interface{}
	if len(out.Data) > 0 && json.Unmarshal(out.Data, &data) == nil {
		version = firstString(data, "general.version", "version", "api.version")
		mediaID = firstString(data, "general.mediaServerId", "mediaServerId")
	}
	parts := []string{fmt.Sprintf("HTTP API 可达（%s）", elapsed.Round(time.Millisecond))}
	if mediaID != "" {
		parts = append(parts, "mediaServerId="+mediaID)
	}
	if version != "" {
		parts = append(parts, "version="+version)
	}
	return true, strings.Join(parts, "，")
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func classify(items []Item) string {
	hasWarn := false
	for _, it := range items {
		switch it.Status {
		case StatusFailed:
			return StatusFailed
		case StatusWarning:
			hasWarn = true
		}
	}
	if hasWarn {
		return StatusWarning
	}
	for _, it := range items {
		if it.Status == StatusOK {
			return StatusOK
		}
	}
	return StatusSkipped
}

func summarize(items []Item) string {
	var ok, warn, failed, skipped int
	for _, it := range items {
		switch it.Status {
		case StatusOK:
			ok++
		case StatusWarning:
			warn++
		case StatusFailed:
			failed++
		default:
			skipped++
		}
	}
	head := fmt.Sprintf("共 %d 项：正常 %d，警告 %d，失败 %d，跳过 %d。", len(items), ok, warn, failed, skipped)
	switch {
	case failed > 0:
		return head + fmt.Sprintf("存在 %d 项失败，请按详情逐项排查。", failed)
	case warn > 0:
		return head + "检查项基本可用，但存在风险项，建议在真机验收前处理。"
	default:
		return head + "全部检查通过。"
	}
}

func humanDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%.1f 分钟", d.Minutes())
	case d < 48*time.Hour:
		return fmt.Sprintf("%.1f 小时", d.Hours())
	default:
		return fmt.Sprintf("%.1f 天", d.Hours()/24)
	}
}
