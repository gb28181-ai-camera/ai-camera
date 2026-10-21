// Package ssrc 实现 GB28181 的 SSRC 分配（设计 §6.4）。
//
// 格式：`0` + domainPart(5 位) + %04d(index)，共 10 位十进制数字。
//
//	首位  0 = 实时点播，1 = 回放
//	domainPart 取 SIP 域的第 4–8 位（sipDomain[3:8]）
//	index 是 10000 个槽位中的一个（线性探测）
//
// 为什么是**进程内单写者**：SSRC 全局唯一性是单端口收流模式的硬前提
// （ZLM 只按 SSRC 区分流）。把分配放进 Redis 会让多个写者竞争，
// 反而需要分布式锁；单节点 sipsvc 内用 mutex + 位图最可靠。
//
// 与 ZLM 的关系：ZLM 只认识 `%08X(ssrc)` 的十六进制流名。
// 本包同时提供十进制 SSRC ↔ 十六进制流名的转换，避免各处重复实现。
package ssrc

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const (
	// SlotCount 是单前缀下的槽位数（4 位十进制）。
	SlotCount = 10000
	// SSRCWidth 是 SSRC 字符串长度。
	SSRCWidth = 10
)

// Allocator 是 SSRC 分配器，并发安全。
type Allocator struct {
	mu         sync.Mutex
	domainPart string
	cursor     int
	used       [SlotCount]bool
	// active 记录本进程当前持有的 SSRC。Rebuild 时要把它们并进去，
	// 否则「ZLM 里刚建的流还没出现在 getMediaList」时会被重复分配。
	active map[string]bool
}

// New 用 SIP 域构造分配器。domain 形如 3402000000。
func New(domain string) *Allocator {
	return &Allocator{
		domainPart: domainPartOf(domain),
		active:     map[string]bool{},
	}
}

// domainPartOf 取域的第 4–8 位；域过短时用 0 补齐。
func domainPartOf(domain string) string {
	domain = strings.TrimSpace(domain)
	if len(domain) >= 8 {
		return domain[3:8]
	}
	out := domain
	for len(out) < 5 {
		out += "0"
	}
	return out[:5]
}

// Format 把槽位拼成 10 位 SSRC。
func (a *Allocator) Format(realtime bool, index int) string {
	prefix := "0"
	if !realtime {
		prefix = "1"
	}
	return fmt.Sprintf("%s%s%04d", prefix, a.domainPart, index)
}

// Allocate 分配一个未使用的 SSRC。realtime=false 表示回放。
//
// 从上次分配的位置继续找（轮转），避免总是复用最小的槽位 ——
// 否则被释放的 0001 会被立刻重新分配，而 ZLM 侧的旧流可能还没清理干净。
func (a *Allocator) Allocate(realtime bool) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := 0; i < SlotCount; i++ {
		idx := (a.cursor + i) % SlotCount
		ssrc := a.Format(realtime, idx)
		if a.used[idx] {
			continue
		}
		a.used[idx] = true
		a.active[ssrc] = true
		a.cursor = (idx + 1) % SlotCount
		return ssrc, nil
	}
	return "", fmt.Errorf("SSRC 池已耗尽（%d 个槽位全部占用）", SlotCount)
}

// Release 归还一个 SSRC。未知或格式非法时静默忽略。
func (a *Allocator) Release(ssrc string) {
	idx, ok := a.index(ssrc)
	if !ok {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.used[idx] = false
	delete(a.active, ssrc)
}

// MarkUsed 标记一个外部已知的 SSRC 已占用（如从 ZLM getMediaList 重建时）。
func (a *Allocator) MarkUsed(ssrc string) {
	idx, ok := a.index(ssrc)
	if !ok {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.used[idx] = true
}

// Rebuild 用 ZLM 当前的流列表重建占用位图。
//
// streams 是 ZLM getMediaList 返回的流名（`%08X(ssrc)` 大写十六进制）。
// 实现会保留本进程仍然持有的分配（active），再并入外部已知的流，
// 因此可以在运行期周期性调用（设计 §6.4：WVP 每 5 秒重建一次）。
func (a *Allocator) Rebuild(streams []string) {
	// 全程持锁：active 会被 Allocate/Release 并发修改，无锁遍历是数据竞争。
	a.mu.Lock()
	defer a.mu.Unlock()

	var used [SlotCount]bool
	for ssrc := range a.active {
		if idx, ok := a.index(ssrc); ok {
			used[idx] = true
		}
	}
	for _, s := range streams {
		ssrc, err := FromHexStream(s)
		if err != nil {
			continue
		}
		if idx, ok := a.index(ssrc); ok {
			used[idx] = true
		}
	}
	a.used = used
}

// InUse 判断 SSRC 是否已被占用。
func (a *Allocator) InUse(ssrc string) bool {
	idx, ok := a.index(ssrc)
	if !ok {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.used[idx]
}

// index 从 10 位 SSRC 反推槽位；格式不合法时返回 false。
func (a *Allocator) index(ssrc string) (int, bool) {
	if len(ssrc) != SSRCWidth {
		return 0, false
	}
	if ssrc[0] != '0' && ssrc[0] != '1' {
		return 0, false
	}
	if ssrc[1:6] != a.domainPart {
		return 0, false
	}
	idx, err := strconv.Atoi(ssrc[6:])
	if err != nil || idx < 0 || idx >= SlotCount {
		return 0, false
	}
	return idx, true
}

// ToHexStream 把 10 位十进制 SSRC 转成 ZLM 的流名（8 位大写十六进制）。
func ToHexStream(ssrc string) (string, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(ssrc), 10, 32)
	if err != nil {
		return "", fmt.Errorf("SSRC %q 不是合法的十进制 32 位整数：%w", ssrc, err)
	}
	return fmt.Sprintf("%08X", uint32(v)), nil
}

// FromHexStream 把 ZLM 流名转回 10 位十进制 SSRC。
func FromHexStream(stream string) (string, error) {
	stream = strings.TrimSpace(stream)
	if stream == "" {
		return "", fmt.Errorf("流名为空")
	}
	v, err := strconv.ParseUint(stream, 16, 32)
	if err != nil {
		return "", fmt.Errorf("流名 %q 不是合法的十六进制 SSRC：%w", stream, err)
	}
	return fmt.Sprintf("%010d", v), nil
}

// Validate 校验 SSRC 是否为 10 位数字（不校验域前缀）。
func Validate(ssrc string) error {
	if len(ssrc) != SSRCWidth {
		return fmt.Errorf("SSRC %q 长度应为 %d", ssrc, SSRCWidth)
	}
	if _, err := strconv.ParseUint(ssrc, 10, 64); err != nil {
		return fmt.Errorf("SSRC %q 应为十进制数字：%w", ssrc, err)
	}
	return nil
}
