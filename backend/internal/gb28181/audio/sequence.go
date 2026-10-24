package audio

import "sort"

// SequenceBuffer 按 RTP 序号做乱序重排与丢包容忍。
//
// 为什么不直接按到达顺序处理：ZLM 把设备 RTP 转推到 Pipecat 的 UDP 端口，
// 经过容器网络后可能乱序。G.711 20ms 帧一旦乱序直接播放，听感是「咔哒」
// 与吞字；而等所有缺口补齐又会把延迟无限拉大。
//
// 策略：
//   - 期望序号之前的包视为迟到/重复，直接丢弃（实时语音不能为了补一帧
//     把整条链路回退）；
//   - 期望序号之后的包进入 pending，按序号连续吐出；
//   - pending 超过 capacity 时，强制吐出序号最小的包，并把跳过的序号
//     计入 Lost —— 这就是「容忍丢包」：宁可少一帧，也不无限等。
//
// 序号是 16 位、会回绕，因此所有比较都用 int16 差值，而不是直接比大小。
type SequenceBuffer struct {
	capacity int

	pending map[uint16]*RTPPacket
	next    uint16
	started bool

	packets    uint64
	duplicates uint64
	reordered  uint64
	lost       uint64
}

// NewSequenceBuffer 构造序号缓冲。capacity 建议取 16~64（20ms 帧下
// 等价于 320ms~1.28s 的最大重排窗口；默认 32）。
func NewSequenceBuffer(capacity int) *SequenceBuffer {
	if capacity <= 0 {
		capacity = 32
	}
	return &SequenceBuffer{
		capacity: capacity,
		pending:  make(map[uint16]*RTPPacket, capacity),
	}
}

// Push 接收一个包，返回当前可以按序输出的包列表（可能为空）。
func (b *SequenceBuffer) Push(packet *RTPPacket) []*RTPPacket {
	if b == nil || packet == nil {
		return nil
	}
	b.packets++

	if !b.started {
		b.started = true
		b.next = packet.SequenceNumber
		out := []*RTPPacket{packet}
		b.next++
		return append(out, b.drain()...)
	}

	delta := int16(packet.SequenceNumber - b.next)
	switch {
	case delta < 0:
		// 已经输出过（或太旧）：实时语音里没有补发的意义。
		b.duplicates++
		return nil

	case delta == 0:
		out := []*RTPPacket{packet}
		b.next++
		return append(out, b.drain()...)

	default:
		if _, exists := b.pending[packet.SequenceNumber]; exists {
			b.duplicates++
			return nil
		}
		b.pending[packet.SequenceNumber] = packet
		b.reordered++

		var out []*RTPPacket
		for len(b.pending) > b.capacity {
			seq := b.smallestPending()
			// next..seq 之间缺失的包记为丢包。
			if gap := uint16(seq - b.next); gap > 0 {
				b.lost += uint64(gap)
			}
			out = append(out, b.pending[seq])
			delete(b.pending, seq)
			b.next = seq + 1
			out = append(out, b.drain()...)
		}
		return out
	}
}

// Flush 把所有 pending 按序号顺序吐出（会话结束或长时间无新包时调用）。
//
// 不做「等缺口」处理：flush 的语义就是「不再等了」。
func (b *SequenceBuffer) Flush() []*RTPPacket {
	if b == nil || len(b.pending) == 0 {
		return nil
	}
	seqs := make([]uint16, 0, len(b.pending))
	for seq := range b.pending {
		seqs = append(seqs, seq)
	}
	// 按相对 next 的距离排序，保证回绕时仍然是从近到远。
	sort.Slice(seqs, func(i, j int) bool {
		return int16(seqs[i]-b.next) < int16(seqs[j]-b.next)
	})

	out := make([]*RTPPacket, 0, len(seqs))
	for _, seq := range seqs {
		if gap := uint16(seq - b.next); gap > 0 {
			b.lost += uint64(gap)
		}
		out = append(out, b.pending[seq])
		delete(b.pending, seq)
		b.next = seq + 1
	}
	return out
}

// Pending 返回当前未输出的包数量（调试/健康检查）。
func (b *SequenceBuffer) Pending() int {
	if b == nil {
		return 0
	}
	return len(b.pending)
}

// Stats 返回累计统计，用于 Pipecat /health 与日志。
func (b *SequenceBuffer) Stats() SequenceStats {
	if b == nil {
		return SequenceStats{}
	}
	return SequenceStats{
		Packets:    b.packets,
		Duplicates: b.duplicates,
		Reordered:  b.reordered,
		Lost:       b.lost,
		Pending:    len(b.pending),
	}
}

// SequenceStats 是序号缓冲的累计统计快照。
type SequenceStats struct {
	Packets    uint64 `json:"packets"`
	Duplicates uint64 `json:"duplicates"`
	Reordered  uint64 `json:"reordered"`
	Lost       uint64 `json:"lost"`
	Pending    int    `json:"pending"`
}

// drain 连续吐出 pending 中紧跟 next 的包。
func (b *SequenceBuffer) drain() []*RTPPacket {
	var out []*RTPPacket
	for {
		packet, ok := b.pending[b.next]
		if !ok {
			return out
		}
		delete(b.pending, b.next)
		out = append(out, packet)
		b.next++
	}
}

// smallestPending 找出相对 next 距离最小的 pending 序号（回绕安全）。
func (b *SequenceBuffer) smallestPending() uint16 {
	first := true
	var best uint16
	for seq := range b.pending {
		if first || int16(seq-b.next) < int16(best-b.next) {
			best = seq
			first = false
		}
	}
	return best
}
