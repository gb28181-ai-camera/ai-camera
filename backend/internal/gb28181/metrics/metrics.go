// Package metrics 提供进程内的 GB28181 指标采集与 Prometheus 文本输出。
//
// 为什么手写而不引 prometheus/client_golang：
//  1. 本项目对新增第三方依赖敏感（安全评审 + 供应链），而这里需要的
//     只是 counter / gauge 两种最朴素的类型；
//  2. Prometheus 的文本格式（text exposition format 0.0.4）本身只有
//     「# HELP / # TYPE / name{label="v"} value」几行，自己实现可完全掌控
//     输出，不会因 SDK 版本升级导致 /metrics 结构漂移；
//  3. 不需要 histogram / summary —— 信令场景关心的是「有没有失败」，
//     而不是延迟分位；延迟已经在 ac_gb_sip_log.elapsed_ms 里逐条记录。
//
// 线程安全：计数用 sync/atomic，标签表用 RWMutex 保护。
//
// 标签基数纪律：只允许**固定枚举值**（result / direction / event）。
// 严禁把设备国标编码、Call-ID、远端 IP 等无界值作为标签 —— 那会让
// Prometheus 的时序数量随设备数线性膨胀，最终打爆内存。需要按设备排障时
// 查 ac_gb_sip_log 表，不要查指标。
package metrics

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ContentType 是 /metrics 响应应使用的 Content-Type（Prometheus 文本格式）。
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

// 指标名前缀。统一前缀既避免与 Go runtime / 进程指标混淆，也方便告警规则按前缀筛选。
const prefix = "aicamera_gb_"

type metricKind uint8

const (
	kindCounter metricKind = iota
	kindGauge
)

func (k metricKind) String() string {
	if k == kindGauge {
		return "gauge"
	}
	return "counter"
}

// metric 是一个带固定标签名的计数器/仪表。
//
// values 的 key 由标签值用 \x00 拼接而成（\x00 不可能出现在 Prometheus 标签值中，
// 因此不存在拼接歧义）。order 保存首次出现的顺序，保证 /metrics 输出稳定可 diff。
type metric struct {
	name       string
	help       string
	kind       metricKind
	labelNames []string

	mu     sync.RWMutex
	values map[string]*series
	order  []string
}

type series struct {
	labels []string
	value  atomic.Int64
}

func newMetric(name, help string, kind metricKind, labelNames []string, presets ...[]string) *metric {
	m := &metric{
		name:       name,
		help:       help,
		kind:       kind,
		labelNames: labelNames,
		values:     make(map[string]*series, len(presets)+4),
	}
	// 预置常见标签组合：即使一次都没发生，/metrics 里也能看到 0 值序列，
	// 图表面板不会因为「指标暂时不存在」而出现断线。
	for _, preset := range presets {
		m.series(preset...)
	}
	// 无标签指标同样预置一个空序列，否则从未打点的 gauge（如活跃流数为 0）
	// 只会输出 HELP/TYPE 而没有样本，PromQL 里看起来像「指标不存在」。
	if len(labelNames) == 0 && len(presets) == 0 {
		m.series()
	}
	return m
}

// series 取（或创建）一个标签组合对应的计数器。标签个数不匹配时返回 nil，
// 调用方静默忽略 —— 指标绝不能因为一次错误调用把业务打挂。
func (m *metric) series(labelValues ...string) *series {
	if len(labelValues) != len(m.labelNames) {
		return nil
	}
	key := strings.Join(labelValues, "\x00")

	m.mu.RLock()
	s := m.values[key]
	m.mu.RUnlock()
	if s != nil {
		return s
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if s = m.values[key]; s != nil {
		return s
	}
	s = &series{labels: append([]string(nil), labelValues...)}
	m.values[key] = s
	m.order = append(m.order, key)
	return s
}

func (m *metric) inc(labelValues ...string) {
	if s := m.series(labelValues...); s != nil {
		s.value.Add(1)
	}
}

func (m *metric) add(delta int64, labelValues ...string) {
	if s := m.series(labelValues...); s != nil {
		s.value.Add(delta)
	}
}

func (m *metric) set(v int64, labelValues ...string) {
	if s := m.series(labelValues...); s != nil {
		s.value.Store(v)
	}
}

// addNonNegative 用于「活跃数」类仪表：并发 Add(-1) 与 Add(+1) 交错时，
// 极端情况下可能短暂出现 -1（例如回调先于建流到达），对运维没有意义，
// 因此钳到 0。注意这是「读到再写」的非原子组合，只用于展示型 gauge，
// 不承担精确对账职责。
func (m *metric) addNonNegative(delta int64, labelValues ...string) {
	s := m.series(labelValues...)
	if s == nil {
		return
	}
	for {
		old := s.value.Load()
		next := old + delta
		if next < 0 {
			next = 0
		}
		if s.value.CompareAndSwap(old, next) {
			return
		}
	}
}

// ────────────────────────────────────────────────────────────────
// 全局指标
// ────────────────────────────────────────────────────────────────

var (
	processStartTime = time.Now()

	// 注册结果。rejected 与 failed 语义不同：
	//   failed   = 网络/解析错误，设备可能根本没收到响应；
	//   rejected = 设备已建档但 Digest 鉴权失败（密码/realm 配错）。
	// 把两者分开，值班人员才能一眼判断该查网络还是查密码。
	sipRegisters = newMetric(
		prefix+"sip_registers_total",
		"GB28181 SIP REGISTER 处理结果总数",
		kindCounter, []string{"result"},
		[]string{"success"}, []string{"failed"}, []string{"rejected"},
	)

	// SIP 报文方向。入向/出向分开，便于发现「设备在发、平台没回」。
	sipMessages = newMetric(
		prefix+"sip_messages_total",
		"GB28181 SIP 报文总数（按方向）",
		kindCounter, []string{"direction"},
		[]string{"in"}, []string{"out"},
	)

	// 活跃流（ZLM 中正在收/发的国标流）。由 hook 的 on_stream_changed 维护。
	activeStreams = newMetric(
		prefix+"active_streams",
		"当前活跃的 GB28181 媒体流数量",
		kindGauge, nil,
	)

	// ZLM hook 回调。按事件类型分，用于确认「ZLM 是否还在回调平台」。
	hookCallbacks = newMetric(
		prefix+"hook_callbacks_total",
		"ZLM 回调平台的事件总数（按事件类型）",
		kindCounter, []string{"event"},
		[]string{"server_keepalive"}, []string{"server_started"},
		[]string{"publish"}, []string{"play"},
		[]string{"stream_changed"}, []string{"stream_none_reader"},
		[]string{"stream_not_found"}, []string{"send_rtp_stopped"},
		[]string{"rtp_server_timeout"}, []string{"record_mp4"},
	)

	// 语音会话（广播/对讲/Agent）。建会话 +1、结束 -1。
	voiceSessions = newMetric(
		prefix+"voice_sessions",
		"当前活跃的语音广播/对讲/Agent 会话数量",
		kindGauge, nil,
	)

	// 进程启动时间（Unix 秒）。用于计算 uptime 与识别意外重启。
	processStart = newMetric(
		prefix+"process_start_time_seconds",
		"进程启动时间（Unix 秒）",
		kindGauge, nil,
	)

	// 诊断接口调用结果。失败率升高通常意味着现场网络/NAT 配置问题。
	diagnoses = newMetric(
		prefix+"diagnoses_total",
		"设备诊断接口调用次数（按总体结论）",
		kindCounter, []string{"result"},
		[]string{"ok"}, []string{"warning"}, []string{"failed"}, []string{"skipped"},
	)

	// 信令日志清理删除行数。用于确认 janitor 在工作、以及日志表增长速率。
	sipLogDeleted = newMetric(
		prefix+"sip_log_deleted_total",
		"信令日志 janitor 累计物理删除的行数",
		kindCounter, nil,
	)

	// 信令日志队列丢弃数（Manager.LogSip 队列满时累加）。
	// 由主会话在信令层接线：metrics.SetSipLogDropped(mgr.DroppedLogs())。
	sipLogDropped = newMetric(
		prefix+"sip_log_dropped",
		"因异步写队列满而被丢弃的信令日志条数（应长期为 0）",
		kindGauge, nil,
	)
)

func init() {
	// 启动时间在包初始化时固定，后续 scrape 不会变化。
	processStart.set(processStartTime.Unix())
}

// ────────────────────────────────────────────────────────────────
// 打点 API
// ────────────────────────────────────────────────────────────────

// IncSipRegister 记录一次 REGISTER 处理结果。
// result 取值：success / failed / rejected（其它值也接受，便于扩展）。
func IncSipRegister(result string) { sipRegisters.inc(strings.ToLower(strings.TrimSpace(result))) }

// IncSipMessage 记录一条 SIP 报文。direction 取值：in / out。
func IncSipMessage(direction string) {
	sipMessages.inc(strings.ToLower(strings.TrimSpace(direction)))
}

// SetActiveStreams 直接设置活跃流数量（hook 全量快照场景）。
func SetActiveStreams(n int64) { activeStreams.set(n) }

// AddActiveStreams 增减活跃流数量（on_stream_changed 增量场景）。
func AddActiveStreams(delta int64) { activeStreams.addNonNegative(delta) }

// IncHookCallback 记录一次 ZLM 回调。event 使用无前缀的事件名（如 server_keepalive）。
func IncHookCallback(event string) { hookCallbacks.inc(strings.TrimSpace(event)) }

// SetVoiceSessions 设置当前语音会话数（定期对账场景）。
func SetVoiceSessions(n int64) { voiceSessions.set(n) }

// AddVoiceSessions 增减语音会话数（会话建立/结束场景）。
func AddVoiceSessions(delta int64) { voiceSessions.addNonNegative(delta) }

// IncDiagnose 记录一次诊断调用。result 取值：ok / warning / failed / skipped。
func IncDiagnose(result string) { diagnoses.inc(strings.ToLower(strings.TrimSpace(result))) }

// AddSipLogDeleted 累加 janitor 删除的行数。
func AddSipLogDeleted(n int64) {
	if n > 0 {
		sipLogDeleted.add(n)
	}
}

// SetSipLogDropped 设置信令日志队列的累计丢弃数。
func SetSipLogDropped(n int64) { sipLogDropped.set(n) }

// ────────────────────────────────────────────────────────────────
// 文本渲染
// ────────────────────────────────────────────────────────────────

// Render 生成 Prometheus 文本格式的完整指标快照。
//
// 每次抓取都重新生成字符串：指标数量固定在十几个，分配开销可忽略；
// 换来的是无缓存失效问题，且便于单测直接断言文本。
func Render() string {
	var b strings.Builder
	for _, m := range []*metric{
		sipRegisters,
		sipMessages,
		activeStreams,
		hookCallbacks,
		voiceSessions,
		processStart,
		diagnoses,
		sipLogDeleted,
		sipLogDropped,
	} {
		writeMetric(&b, m)
	}
	return b.String()
}

func writeMetric(b *strings.Builder, m *metric) {
	// 先做一次快照，避免持锁写 IO。
	m.mu.RLock()
	type row struct {
		labels []string
		value  int64
	}
	rows := make([]row, 0, len(m.order))
	for _, key := range m.order {
		s := m.values[key]
		rows = append(rows, row{labels: s.labels, value: s.value.Load()})
	}
	m.mu.RUnlock()

	b.WriteString("# HELP ")
	b.WriteString(m.name)
	b.WriteByte(' ')
	b.WriteString(escapeHelp(m.help))
	b.WriteByte('\n')
	b.WriteString("# TYPE ")
	b.WriteString(m.name)
	b.WriteByte(' ')
	b.WriteString(m.kind.String())
	b.WriteByte('\n')

	for _, r := range rows {
		b.WriteString(m.name)
		if len(m.labelNames) > 0 {
			b.WriteByte('{')
			for i, name := range m.labelNames {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(name)
				b.WriteString("=\"")
				b.WriteString(escapeLabel(r.labels[i]))
				b.WriteByte('"')
			}
			b.WriteByte('}')
		}
		b.WriteByte(' ')
		b.WriteString(strconv.FormatInt(r.value, 10))
		b.WriteByte('\n')
	}
}

// escapeLabel 按 Prometheus 文本格式转义标签值。
func escapeLabel(s string) string {
	if !strings.ContainsAny(s, "\\\"\n") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// escapeHelp 按 Prometheus 文本格式转义 HELP 文本（反斜杠与换行）。
func escapeHelp(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "\n", `\n`)
}
