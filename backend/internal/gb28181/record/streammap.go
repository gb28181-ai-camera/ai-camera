package record

import (
	"strings"
	"sync"
	"time"
)

// StreamOwner 记录「ZLM 流 → 国标设备/通道」的映射。
//
// 为什么需要它：ZLM 的 on_record_mp4 回调只带 app/stream，没有设备信息。
// 单端口模式下 stream 是 %08X(ssrc)，平台自己分配的 SSRC 与通道的对应关系
// 只存在于发起 INVITE 的那段代码里。调度器和下载任务在启动录制时登记，
// hook 落库时反查，才能把 MP4 正确归属到设备/通道。
//
// 进程重启后映射为空，但此时旧的 ZLM 流也已在进程退出时断开，
// 不会出现「新回调查到旧归属」的错误。
type StreamOwner struct {
	App           string
	Stream        string
	MediaServerID string
	DeviceID      int64
	ChannelID     int64
	Since         time.Time
}

var (
	streamOwnerMu sync.RWMutex
	streamOwners  = map[string]StreamOwner{}
)

// RegisterStreamOwner 登记/覆盖一条流的归属。
func RegisterStreamOwner(o StreamOwner) {
	stream := strings.TrimSpace(o.Stream)
	if stream == "" {
		return
	}
	o.Stream = stream
	if o.Since.IsZero() {
		o.Since = time.Now()
	}
	streamOwnerMu.Lock()
	streamOwners[stream] = o
	streamOwnerMu.Unlock()
}

// UnregisterStreamOwner 移除归属（停止录制/关闭下载任务时调用）。
func UnregisterStreamOwner(stream string) {
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return
	}
	streamOwnerMu.Lock()
	delete(streamOwners, stream)
	streamOwnerMu.Unlock()
}

// UnregisterStreamOwnerAfter 延迟移除归属。
//
// 为什么需要延迟：StopRecord 之后 ZLM 还要关闭 MP4 并回调 on_record_mp4，
// 立刻删除映射会让最后一个片段丢失 device_id/channel_id。留一个短窗口
// 让回调先落库；SSRC 池是轮转分配的，短时间内不会复用同一个流名。
func UnregisterStreamOwnerAfter(stream string, d time.Duration) {
	if d <= 0 {
		UnregisterStreamOwner(stream)
		return
	}
	go func() {
		time.Sleep(d)
		UnregisterStreamOwner(stream)
	}()
}

// ResolveStreamOwner 按流名反查归属。
func ResolveStreamOwner(stream string) (StreamOwner, bool) {
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return StreamOwner{}, false
	}
	streamOwnerMu.RLock()
	o, ok := streamOwners[stream]
	streamOwnerMu.RUnlock()
	return o, ok
}
