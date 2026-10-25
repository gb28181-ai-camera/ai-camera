package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logger *zap.Logger

// ════════════════════════════════════════════════════════════════
// 内存日志环：历史检索与 SSE 实时推送共用同一份数据
//
// 设计取舍：
//   - 不新增日志表：日志是运维辅助数据，写库会放大写压力且需要清理策略；
//   - 环缓冲固定容量，满了覆盖最旧的一条（O(1)，不做整体搬移）；
//   - 订阅通道有缓冲，**慢客户端直接丢事件**，绝不阻塞业务日志写入；
//   - 日志内容不做脱敏以外的加工：结构化字段以 JSON 解码后的 map 保存。
// ════════════════════════════════════════════════════════════════

// LogEntry 一条内存日志。
type LogEntry struct {
	ID      int64                  `json:"id"`
	Time    time.Time              `json:"time"`
	Level   string                 `json:"level"`
	Logger  string                 `json:"logger,omitempty"`
	Message string                 `json:"message"`
	Caller  string                 `json:"caller,omitempty"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
}

// defaultLogRingCapacity 默认保留的日志条数。
//
// 5000 条在典型运维场景下覆盖最近数小时；可按需调整，内存上限约 5~10MB。
const defaultLogRingCapacity = 5000

// logRing 定长环缓冲。
type logRing struct {
	mu      sync.RWMutex
	buf     []LogEntry
	head    int // 最旧一条的下标
	count   int
	nextID  int64
	subs    map[int]chan LogEntry
	nextSub int
}

func newLogRing(capacity int) *logRing {
	if capacity <= 0 {
		capacity = defaultLogRingCapacity
	}
	return &logRing{
		buf:  make([]LogEntry, capacity),
		subs: make(map[int]chan LogEntry),
	}
}

// append 写入一条日志并广播给订阅者。
func (r *logRing) append(e LogEntry) {
	r.mu.Lock()
	e.ID = r.nextID
	r.nextID++
	if r.count < len(r.buf) {
		r.buf[(r.head+r.count)%len(r.buf)] = e
		r.count++
	} else {
		r.buf[r.head] = e
		r.head = (r.head + 1) % len(r.buf)
	}
	// 复制订阅者列表后在锁外发送：写日志的路径不能被慢客户端拖住。
	chans := make([]chan LogEntry, 0, len(r.subs))
	for _, ch := range r.subs {
		chans = append(chans, ch)
	}
	r.mu.Unlock()

	for _, ch := range chans {
		select {
		case ch <- e:
		default:
			// 缓冲已满：丢弃该事件（实时流允许丢帧，历史检索仍完整）。
		}
	}
}

// subscribe 注册订阅者，返回通道与取消函数。
func (r *logRing) subscribe(buffer int) (<-chan LogEntry, func()) {
	if buffer <= 0 {
		buffer = 256
	}
	ch := make(chan LogEntry, buffer)

	r.mu.Lock()
	id := r.nextSub
	r.nextSub++
	r.subs[id] = ch
	r.mu.Unlock()

	unsubscribe := func() {
		r.mu.Lock()
		delete(r.subs, id)
		r.mu.Unlock()
	}
	return ch, unsubscribe
}

// query 按级别与关键字检索，返回按时间倒序的条目与命中总数。
func (r *logRing) query(minLevel, keyword string, offset, limit int) ([]LogEntry, int64) {
	min := parseLogLevel(minLevel)
	kw := strings.ToLower(strings.TrimSpace(keyword))

	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]LogEntry, 0, minInt(limit, r.count))
	var total int64
	// 从最新到最旧遍历。
	for i := r.count - 1; i >= 0; i-- {
		e := r.buf[(r.head+i)%len(r.buf)]
		if !logLevelAtLeast(e.Level, min) {
			continue
		}
		if kw != "" && !logEntryContains(e, kw) {
			continue
		}
		if total < int64(offset) {
			total++
			continue
		}
		if len(matched) < limit {
			matched = append(matched, e)
		}
		total++
	}
	return matched, total
}

// snapshot 返回最近 n 条（最新在前）。
func (r *logRing) snapshot(n int) []LogEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n <= 0 || n > r.count {
		n = r.count
	}
	out := make([]LogEntry, 0, n)
	for i := r.count - 1; i >= r.count-n; i-- {
		out = append(out, r.buf[(r.head+i)%len(r.buf)])
	}
	return out
}

// reset 清空日志（测试用）。
func (r *logRing) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.head, r.count = 0, 0
}

var globalLogRing = newLogRing(defaultLogRingCapacity)

// logLevelRank 把级别字符串映射为可比较的序数。
func logLevelRank(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return 0
	case "info":
		return 1
	case "warn", "warning":
		return 2
	case "error":
		return 3
	case "fatal", "panic", "dpanic":
		return 4
	default:
		return 1
	}
}

func parseLogLevel(minLevel string) int {
	if strings.TrimSpace(minLevel) == "" {
		return -1 // 不过滤
	}
	return logLevelRank(minLevel)
}

func logLevelAtLeast(level string, min int) bool {
	if min < 0 {
		return true
	}
	return logLevelRank(level) >= min
}

// logEntryContains 在 message / logger / caller / 字段中做子串匹配。
func logEntryContains(e LogEntry, lowerKeyword string) bool {
	if strings.Contains(strings.ToLower(e.Message), lowerKeyword) ||
		strings.Contains(strings.ToLower(e.Logger), lowerKeyword) ||
		strings.Contains(strings.ToLower(e.Caller), lowerKeyword) {
		return true
	}
	if len(e.Fields) == 0 {
		return false
	}
	raw, err := json.Marshal(e.Fields)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(raw)), lowerKeyword)
}

// LogLevelMatches 判断日志级别是否达到过滤级别（空 filter 表示不过滤）。
func LogLevelMatches(level, filter string) bool {
	return logLevelAtLeast(level, parseLogLevel(filter))
}

// LogEntryMatches 判断一条日志是否满足级别 + 关键字过滤（SSE 侧使用）。
func LogEntryMatches(e LogEntry, level, keyword string) bool {
	if !LogLevelMatches(e.Level, level) {
		return false
	}
	kw := strings.ToLower(strings.TrimSpace(keyword))
	return kw == "" || logEntryContains(e, kw)
}

// LogHistory 检索内存日志，最新在前。page 从 1 开始。
func LogHistory(level, keyword string, page, pageSize int) ([]LogEntry, int64) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 500 {
		pageSize = 500
	}
	return globalLogRing.query(level, keyword, (page-1)*pageSize, pageSize)
}

// RecentLogs 返回最近 n 条日志（最新在前），供 SSE 建连时回放。
func RecentLogs(n int) []LogEntry { return globalLogRing.snapshot(n) }

// SubscribeLogs 订阅实时日志。取消函数必须调用（通常 defer）。
func SubscribeLogs(buffer int) (<-chan LogEntry, func()) {
	return globalLogRing.subscribe(buffer)
}

// ResetLogRing 清空内存日志（测试用）。
func ResetLogRing() { globalLogRing.reset() }

// ────────────────────────────────────────────────────────────────
// zap ring core
// ────────────────────────────────────────────────────────────────

// ringCore 把日志同时写入原有 core 与内存环。
type ringCore struct {
	core   zapcore.Core
	enc    zapcore.Encoder
	fields []zapcore.Field
}

// newRingCore 包装已有 core；encoder 只用于把结构化字段摊平成 map。
func newRingCore(core zapcore.Core) zapcore.Core {
	cfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	return &ringCore{core: core, enc: zapcore.NewJSONEncoder(cfg)}
}

// Enabled 沿用底层 core 的级别判断。
func (c *ringCore) Enabled(level zapcore.Level) bool { return c.core.Enabled(level) }

// With 派生子 core；字段在 Write 时合并进内存条目。
func (c *ringCore) With(fields []zapcore.Field) zapcore.Core {
	merged := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	merged = append(merged, c.fields...)
	merged = append(merged, fields...)
	return &ringCore{core: c.core.With(fields), enc: c.enc.Clone(), fields: merged}
}

// Check 把自身加入 CheckedEntry。
func (c *ringCore) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(ent.Level) {
		return ce.AddCore(ent, c)
	}
	return ce
}

// Write 先写底层 core，再写内存环。内存环异常绝不影响日志主路径。
func (c *ringCore) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	err := c.core.Write(ent, fields)

	entry := LogEntry{
		Time:    ent.Time,
		Level:   ent.Level.String(),
		Logger:  ent.LoggerName,
		Message: ent.Message,
	}
	if ent.Caller.Defined {
		entry.Caller = ent.Caller.TrimmedPath()
	}

	all := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	all = append(all, c.fields...)
	all = append(all, fields...)
	if len(all) > 0 {
		entry.Fields = encodeFieldsToMap(c.enc, ent, all)
	}

	globalLogRing.append(entry)
	return err
}

// Sync 透传底层 core。
func (c *ringCore) Sync() error { return c.core.Sync() }

// encodeFieldsToMap 用 JSON 编码器把 zap 字段摊平成 map。
//
// 之所以不用类型断言逐项处理 zapcore.Field：字段种类多（ObjectMarshaler、
// ArrayMarshaler、Error、Reflect…），手工覆盖必然漏项；JSON 编码器是标准路径，
// 与日志文件里的字段语义天然一致。
func encodeFieldsToMap(enc zapcore.Encoder, ent zapcore.Entry, fields []zapcore.Field) map[string]interface{} {
	buf, err := enc.EncodeEntry(ent, fields)
	if err != nil {
		return nil
	}
	defer buf.Free()

	var raw map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		return nil
	}
	// 去掉已单独建模的键，其余作为结构化字段。
	delete(raw, "ts")
	delete(raw, "level")
	delete(raw, "logger")
	delete(raw, "caller")
	delete(raw, "msg")
	delete(raw, "stacktrace")
	if len(raw) == 0 {
		return nil
	}
	return raw
}

// ────────────────────────────────────────────────────────────────
// 初始化
// ────────────────────────────────────────────────────────────────

// InitLogger 依据配置初始化全局 zap 日志。
// format=console 时输出到标准输出（测试环境使用），否则写入滚动文件。
func InitLogger(cfg *config.LogConfig) {
	level := zapcore.InfoLevel
	if cfg != nil {
		switch cfg.Level {
		case "debug":
			level = zapcore.DebugLevel
		case "warn":
			level = zapcore.WarnLevel
		case "error":
			level = zapcore.ErrorLevel
		case "fatal":
			level = zapcore.FatalLevel
		}
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	var writeSyncer zapcore.WriteSyncer

	consoleMode := cfg == nil || cfg.Format == "console" || cfg.Output == "console"
	if consoleMode {
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
		writeSyncer = zapcore.AddSync(os.Stdout)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)

		// 确保日志目录存在
		if dir := filepath.Dir(cfg.Output); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}

		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.Output,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
		})
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)
	// 内存环 tee：历史检索与 SSE 实时日志共用同一份数据。
	logger = zap.New(newRingCore(core), zap.AddCaller(), zap.AddCallerSkip(0))
	zap.ReplaceGlobals(logger)
}

// Logger 返回全局 logger；未初始化时返回一个可用的 no-op 实例，避免调用方 panic。
func Logger() *zap.Logger {
	if logger == nil {
		return zap.NewNop()
	}
	return logger
}

// Info / Warn / Error / Debug 便捷函数。
func Info(msg string, fields ...zap.Field)  { Logger().Info(msg, fields...) }
func Warn(msg string, fields ...zap.Field)  { Logger().Warn(msg, fields...) }
func Error(msg string, fields ...zap.Field) { Logger().Error(msg, fields...) }
func Debug(msg string, fields ...zap.Field) { Logger().Debug(msg, fields...) }

// minInt 取较小值。
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
