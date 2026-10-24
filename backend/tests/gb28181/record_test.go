package gb28181_test

// 录像计划调度、下载进度与 zip 打包的单元测试。
//
// 这些用例刻意不碰数据库：
//   - 窗口匹配 / 进度计算是纯函数；
//   - 调度器通过 WithLoader/WithRunner 注入假实现；
//   - zip 打包用 httptest 提供文件。
//
// 因此可以单独运行：
//   go test ./tests/gb28181/ -run Record -count=1 -p 1

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boticz/ai-camera-backend/internal/gb28181/record"
	gbsvc "github.com/boticz/ai-camera-backend/internal/service/gb28181"
)

// ── 时间窗匹配 ────────────────────────────────────────────────

func TestRecordWindowMatchSameDay(t *testing.T) {
	w := record.Window{WeekDay: 1, StartTime: "08:00:00", EndTime: "18:00:00"}

	mon := func(h, m int) time.Time {
		return time.Date(2026, 10, 26, h, m, 0, 0, time.Local) // 2026-10-26 周一
	}

	cases := []struct {
		at   time.Time
		want bool
	}{
		{mon(8, 0), true},
		{mon(10, 30), true},
		{mon(17, 59), true},
		{mon(18, 0), false}, // 结束时刻不包含
		{mon(7, 59), false},
		{mon(20, 0), false},
		// 周二同一时刻不应命中「周一」的计划
		{time.Date(2026, 10, 27, 10, 0, 0, 0, time.Local), false},
	}
	for _, c := range cases {
		if got := w.ActiveAt(c.at); got != c.want {
			t.Fatalf("ActiveAt(%s) = %v, want %v", c.at.Format(time.RFC3339), got, c.want)
		}
	}
}

func TestRecordWindowMatchCrossDay(t *testing.T) {
	// 周一 22:00 – 周二 06:00
	w := record.Window{WeekDay: 1, StartTime: "22:00:00", EndTime: "06:00:00"}

	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"周一 21:59", time.Date(2026, 10, 26, 21, 59, 0, 0, time.Local), false},
		{"周一 22:00", time.Date(2026, 10, 26, 22, 0, 0, 0, time.Local), true},
		{"周一 23:59", time.Date(2026, 10, 26, 23, 59, 0, 0, time.Local), true},
		{"周二 00:00（跨天尾段）", time.Date(2026, 10, 27, 0, 0, 0, 0, time.Local), true},
		{"周二 05:59（跨天尾段）", time.Date(2026, 10, 27, 5, 59, 0, 0, time.Local), true},
		{"周二 06:00（结束）", time.Date(2026, 10, 27, 6, 0, 0, 0, time.Local), false},
		{"周二 22:00（属于周二自己的计划）", time.Date(2026, 10, 27, 22, 0, 0, 0, time.Local), false},
		{"周三 02:00（不属于周一的尾段）", time.Date(2026, 10, 28, 2, 0, 0, 0, time.Local), false},
	}
	for _, c := range cases {
		if got := w.ActiveAt(c.at); got != c.want {
			t.Fatalf("%s: ActiveAt() = %v, want %v", c.name, got, c.want)
		}
	}

	// EndAt：周一晚属于同一窗口时，结束在周二 06:00；周二凌晨也在周二 06:00 结束。
	monNight := time.Date(2026, 10, 26, 23, 0, 0, 0, time.Local)
	if got := w.EndAt(monNight); !got.Equal(time.Date(2026, 10, 27, 6, 0, 0, 0, time.Local)) {
		t.Fatalf("EndAt(周一 23:00) = %s", got)
	}
	tueDawn := time.Date(2026, 10, 27, 2, 0, 0, 0, time.Local)
	if got := w.EndAt(tueDawn); !got.Equal(time.Date(2026, 10, 27, 6, 0, 0, 0, time.Local)) {
		t.Fatalf("EndAt(周二 02:00) = %s", got)
	}
}

func TestRecordPickActiveOverlapping(t *testing.T) {
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.Local)
	list := []record.Target{
		{ChannelID: 1, Window: record.Window{PlanID: 1, ItemID: 10, WeekDay: 1, StartTime: "08:00:00", EndTime: "12:00:00"}},
		{ChannelID: 1, Window: record.Window{PlanID: 1, ItemID: 11, WeekDay: 1, StartTime: "10:00:00", EndTime: "14:00:00"}},
	}
	got, ok := record.PickActive(list, now)
	if !ok {
		t.Fatal("期望命中一个时间段")
	}
	// 结束最晚的优先，保证最长覆盖。
	if got.Window.ItemID != 11 {
		t.Fatalf("PickActive 选中 ItemID=%d, want 11", got.Window.ItemID)
	}
}

// ── 调度幂等 ──────────────────────────────────────────────────

type fakeRunner struct {
	mu      sync.Mutex
	starts  int
	stops   int
	started chan struct{}
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{started: make(chan struct{}, 16)}
}

func (f *fakeRunner) Start(_ context.Context, t record.Target, now time.Time) (record.JobRef, error) {
	f.mu.Lock()
	f.starts++
	f.mu.Unlock()
	f.started <- struct{}{}
	return record.JobRef{
		ChannelID: t.ChannelID,
		Key:       t.Window.Key(),
		Stream:    "STREAM-1",
		App:       "rtp",
		EndsAt:    t.Window.EndAt(now),
	}, nil
}

func (f *fakeRunner) Stop(_ context.Context, _ record.JobRef) {
	f.mu.Lock()
	f.stops++
	f.mu.Unlock()
}

func (f *fakeRunner) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.stops
}

func TestRecordSchedulerIdempotent(t *testing.T) {
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.Local)
	var clock atomic.Int64
	clock.Store(now.Unix())

	targets := []record.Target{{
		ChannelID:   7,
		ChannelName: "测试通道",
		Window: record.Window{
			PlanID: 1, ItemID: 100, WeekDay: 1,
			StartTime: "08:00:00", EndTime: "18:00:00",
		},
	}}
	loader := func(context.Context) ([]record.Target, error) { return targets, nil }
	runner := newFakeRunner()

	s := record.NewScheduler(
		record.WithInterval(time.Hour),
		record.WithNow(func() time.Time { return time.Unix(clock.Load(), 0) }),
		record.WithLoader(loader),
		record.WithRunner(runner),
	)

	ctx := context.Background()
	s.ReconcileOnce(ctx)
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("第一轮没有启动任务")
	}

	// 再扫两次：同一通道同一时间段不能重复启动。
	s.ReconcileOnce(ctx)
	s.ReconcileOnce(ctx)
	time.Sleep(100 * time.Millisecond)
	if starts, _ := runner.counts(); starts != 1 {
		t.Fatalf("重复扫描导致启动了 %d 次，want 1", starts)
	}

	// 时间推进到窗口外：应当停止，且只停一次。
	clock.Store(time.Date(2026, 10, 26, 19, 0, 0, 0, time.Local).Unix())
	s.ReconcileOnce(ctx)
	s.ReconcileOnce(ctx)
	if _, stops := runner.counts(); stops != 1 {
		t.Fatalf("停止次数 = %d, want 1", stops)
	}
}

func TestRecordSchedulerCrossDayStart(t *testing.T) {
	// 周二凌晨 02:00，命中的是「周一 22:00–06:00」。
	at := time.Date(2026, 10, 27, 2, 0, 0, 0, time.Local)
	target := record.Target{
		ChannelID: 9,
		Window: record.Window{
			PlanID: 2, ItemID: 200, WeekDay: 1,
			StartTime: "22:00:00", EndTime: "06:00:00",
		},
	}
	runner := newFakeRunner()
	s := record.NewScheduler(
		record.WithNow(func() time.Time { return at }),
		record.WithLoader(func(context.Context) ([]record.Target, error) {
			return []record.Target{target}, nil
		}),
		record.WithRunner(runner),
	)
	s.ReconcileOnce(context.Background())
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("跨天时段没有启动任务")
	}
}

// ── 下载进度 ──────────────────────────────────────────────────

func TestRecordComputeProgress(t *testing.T) {
	cases := []struct {
		name  string
		in    record.ProgressInput
		want  float64
		known bool
	}{
		{
			name:  "已完成片段 30s / 总 60s",
			in:    record.ProgressInput{CompletedSeconds: 30, TotalSeconds: 60},
			want:  50,
			known: true,
		},
		{
			name:  "录制中按存活时长估算",
			in:    record.ProgressInput{LiveSeconds: 20, LiveKnown: true, TotalSeconds: 60},
			want:  33.3,
			known: true,
		},
		{
			name:  "没有任何数据",
			in:    record.ProgressInput{TotalSeconds: 60},
			want:  0,
			known: false,
		},
		{
			name:  "时间窗非法",
			in:    record.ProgressInput{CompletedSeconds: 10, TotalSeconds: 0},
			want:  0,
			known: false,
		},
		{
			name:  "录制中不显示 100%",
			in:    record.ProgressInput{CompletedSeconds: 70, TotalSeconds: 60},
			want:  99,
			known: true,
		},
		{
			name:  "最终完成显示 100%",
			in:    record.ProgressInput{CompletedSeconds: 60, TotalSeconds: 60, Final: true},
			want:  100,
			known: true,
		},
	}
	for _, c := range cases {
		got, known := record.ComputeProgress(c.in)
		if known != c.known {
			t.Fatalf("%s: known = %v, want %v", c.name, known, c.known)
		}
		if diff := got - c.want; diff > 0.05 || diff < -0.05 {
			t.Fatalf("%s: ComputeProgress = %v, want %v", c.name, got, c.want)
		}
	}
}

// ── zip 打包 ──────────────────────────────────────────────────

func TestRecordZipPackaging(t *testing.T) {
	mux := http.NewServeMux()
	files := map[string]string{
		"/record/record/stream/a.mp4": "AAAA-content",
		"/record/record/stream/b.mp4": "BBBB-content",
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", "12")
			return
		}
		_, _ = io.WriteString(w, body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	entries := []gbsvc.ZipEntry{
		{ID: 1, FileName: "a.mp4", URL: srv.URL + "/record/record/stream/a.mp4"},
		{ID: 2, FileName: "b.mp4", URL: srv.URL + "/record/record/stream/b.mp4"},
		{ID: 3, FileName: "missing.mp4", URL: srv.URL + "/record/record/stream/missing.mp4"},
	}

	var buf bytes.Buffer
	n, err := gbsvc.WriteZipStream(context.Background(), &buf, entries)
	if err != nil {
		t.Fatalf("WriteZipStream 失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("写入文件数 = %d, want 2（不存在的应跳过）", n)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("解析 zip 失败: %v", err)
	}
	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("打开 zip 条目 %s 失败: %v", f.Name, err)
		}
		body, _ := io.ReadAll(rc)
		rc.Close()
		got[f.Name] = string(body)
	}
	if got["a.mp4"] != "AAAA-content" || got["b.mp4"] != "BBBB-content" {
		t.Fatalf("zip 内容不符: %#v", got)
	}
	if _, ok := got["missing.mp4"]; ok {
		t.Fatal("不存在的文件不应出现在 zip 中")
	}
}

func TestRecordSkippedHeaderValue(t *testing.T) {
	v := gbsvc.SkippedHeaderValue([]string{"录像 1.mp4"})
	if v == "" {
		t.Fatal("跳过列表不能为空")
	}
	// 必须是纯 ASCII（HTTP 头不允许直接放中文）。
	for _, r := range v {
		if r > 127 {
			t.Fatalf("X-Skipped 含非 ASCII 字符: %q", v)
		}
	}
}
