package hook

import (
	"sort"
	"sync"
	"time"
)

// ActiveStream 是内存中的活跃流快照。
//
// 为什么用内存而不是直接查 ZLM：on_stream_changed 是幂等的事件源，
// 编排层需要低延迟地回答「这个流现在在不在」，而 ZLM 的 getMediaList
// 是 HTTP 调用。内存表只作为**加速视图**，权威数据仍在 ZLM/Redis；
// 进程重启丢失可接受（ZLM 会在流变化时再次回调）。
type ActiveStream struct {
	App           string
	Stream        string
	Schema        string
	MediaServerID string
	OriginURL     string
	Tracks        []Track
	Since         time.Time
}

// ActiveStreams 并发安全的活跃流表。
type ActiveStreams struct {
	mu    sync.RWMutex
	items map[string]ActiveStream
}

// NewActiveStreams 创建空的活跃流表。
func NewActiveStreams() *ActiveStreams {
	return &ActiveStreams{items: make(map[string]ActiveStream)}
}

func streamKey(app, stream string) string {
	return app + "/" + stream
}

// Set 新增或覆盖一条活跃流。
func (a *ActiveStreams) Set(s ActiveStream) {
	if a == nil {
		return
	}
	if s.Since.IsZero() {
		s.Since = time.Now()
	}
	a.mu.Lock()
	a.items[streamKey(s.App, s.Stream)] = s
	a.mu.Unlock()
}

// Remove 移除一条活跃流。
func (a *ActiveStreams) Remove(app, stream string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	delete(a.items, streamKey(app, stream))
	a.mu.Unlock()
}

// Get 查询一条活跃流。
func (a *ActiveStreams) Get(app, stream string) (ActiveStream, bool) {
	if a == nil {
		return ActiveStream{}, false
	}
	a.mu.RLock()
	s, ok := a.items[streamKey(app, stream)]
	a.mu.RUnlock()
	return s, ok
}

// Len 返回活跃流数量。
func (a *ActiveStreams) Len() int {
	if a == nil {
		return 0
	}
	a.mu.RLock()
	n := len(a.items)
	a.mu.RUnlock()
	return n
}

// List 返回按 app/stream 排序的活跃流快照。
//
// 排序是为了让测试与日志输出稳定（map 遍历顺序随机）。
func (a *ActiveStreams) List() []ActiveStream {
	if a == nil {
		return nil
	}
	a.mu.RLock()
	out := make([]ActiveStream, 0, len(a.items))
	for _, s := range a.items {
		out = append(out, s)
	}
	a.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].App != out[j].App {
			return out[i].App < out[j].App
		}
		return out[i].Stream < out[j].Stream
	})
	return out
}
