// Package ps 实现 GB28181 设备侧推流所需的 MPEG-2 PS（Program Stream）封装器。
//
// 链路：Annex-B H.264 码流 → 切 NAL → 装 PES → 外套 PS 包（PS 包头 + PSM）。
// 平台侧由 ZLMediaKit 解复用（§2.1 结论一：Go 服务不参与 RTP/PS 的解复用），
// 因此本包只做**封装**，不做解封装；解封装只在本包的测试里实现，用作独立的反向校验。
//
// 码流结构（GB/T 28181-2016 附录 C，等价于 ISO/IEC 13818-1 Program Stream）：
//
//	PS 包头  00 00 01 BA + 10 字节 pack_header
//	PSM      00 00 01 BC + program_stream_map（声明基本流类型，H.264 = 0x1B）
//	PES      00 00 01 E0 + PES 头（含 PTS）+ 负载
package ps

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// 起始码（32 位，含前缀 00 00 01）。
const (
	PackStartCode = 0x000001BA
	PSMStartCode  = 0x000001BC
	PESStartCode  = 0x000001
)

// PES 的 stream_id。
const (
	StreamIDVideo = 0xE0
	StreamIDAudio = 0xC0
)

// 基本流类型（ISO/IEC 13818-1 Table 2-34）。
const (
	StreamTypeH264  = 0x1B
	StreamTypeH265  = 0x24
	StreamTypeG711A = 0x90
	StreamTypeAAC   = 0x0F
)

// ClockRate 是 PS/PES 里 PTS 的时基：MPEG 系统层固定 90kHz。
const ClockRate = 90000

// maxPESPayload 是单个 PES 能承载的最大字节数。
//
// PES_packet_length 是 16 位，且还要扣掉 3 字节标志 + 5 字节 PTS。
const maxPESPayload = 0xFFFF - 8

// H.264 NAL 单元类型。
const (
	NALTypeNonIDR = 1
	NALTypeIDR    = 5
	NALTypeSEI    = 6
	NALTypeSPS    = 7
	NALTypePPS    = 8
	NALTypeAUD    = 9
)

// Muxer 把 Annex-B H.264 NAL 封装成 PS 字节流。零值不可用，必须经 NewMuxer 构造。
type Muxer struct {
	w          io.Writer
	streamType byte
	mu         sync.Mutex
	written    int64
	headerDone bool
}

// Option 定制 Muxer 行为。
type Option func(*Muxer)

// WithStreamType 指定基本流类型（默认 H.264，即 0x1B）。
func WithStreamType(t byte) Option {
	return func(m *Muxer) { m.streamType = t }
}

// NewMuxer 构造一个写入 w 的 PS 封装器。
func NewMuxer(w io.Writer, opts ...Option) *Muxer {
	m := &Muxer{w: w, streamType: StreamTypeH264}
	for _, o := range opts {
		o(m)
	}
	return m
}

// BytesWritten 返回已写出的 PS 字节数，便于模拟器打印推流进度。
func (m *Muxer) BytesWritten() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.written
}

// WritePSHeader 写出 PS 包头 + PSM。
//
// PSM 声明本 PS 流里的基本流类型与 ID，平台侧靠它决定用哪个解码器。
// 真实设备会在每个关键帧前重复写 PSM（便于中途接入的播放器同步），本批次只在流首写一次。
func (m *Muxer) WritePSHeader() error {
	var buf bytes.Buffer
	buf.Write(BuildPackHeader())
	buf.Write(BuildPSM(m.streamType))
	m.mu.Lock()
	m.headerDone = true
	m.mu.Unlock()
	return m.write(buf.Bytes())
}

// WriteNAL 把一个 NAL 单元封装成 PS 包：PS 包头 + 一个或多个 PES。
//
// 注意：**每个 NAL 一个 PES 不是真实设备的做法** —— 真实设备按「一个访问单元（一帧）一个 PES」
// 封装。这里保留该接口是因为它是模拟器最细粒度的控制点；推真实码流请用 WriteAccessUnit。
func (m *Muxer) WriteNAL(pts uint64, nal []byte) error {
	if len(nal) == 0 {
		return nil
	}
	var buf bytes.Buffer
	buf.Write(BuildPackHeader())
	for off := 0; off < len(nal); off += maxPESPayload {
		end := off + maxPESPayload
		if end > len(nal) {
			end = len(nal)
		}
		buf.Write(BuildPES(StreamIDVideo, pts, nal[off:end]))
	}
	return m.write(buf.Bytes())
}

// WriteAccessUnit 把一个访问单元（一帧的全部 NAL）封装进**同一个 PES**。
//
// NAL 之间用 4 字节起始码 00 00 00 01 连接，与真实设备一致。
func (m *Muxer) WriteAccessUnit(pts uint64, nals [][]byte) error {
	if len(nals) == 0 {
		return nil
	}
	payload := JoinAnnexB(nals)
	var buf bytes.Buffer
	buf.Write(BuildPackHeader())
	for off := 0; off < len(payload); off += maxPESPayload {
		end := off + maxPESPayload
		if end > len(payload) {
			end = len(payload)
		}
		buf.Write(BuildPES(StreamIDVideo, pts, payload[off:end]))
	}
	return m.write(buf.Bytes())
}

// WriteAnnexBStream 把整段 Annex-B 码流切成访问单元后依次封装。
//
// basePTS 是首个访问单元的 PTS（90kHz 单位），frameDuration 是每帧的 PTS 步进
// （25fps → 90000/25 = 3600）。返回封装出的访问单元数。
func (m *Muxer) WriteAnnexBStream(data []byte, basePTS, frameDuration uint64) (int, error) {
	if err := m.WritePSHeader(); err != nil {
		return 0, err
	}
	units := GroupAccessUnits(SplitAnnexB(data))
	pts := basePTS
	for _, unit := range units {
		if err := m.WriteAccessUnit(pts, unit); err != nil {
			return 0, err
		}
		pts += frameDuration
	}
	return len(units), nil
}

// WritePSChunk 直接写一段已经封装好的 PS 字节（例如从文件读取的 PS 样本），
// 让 RTP 层可以复用同一套分包逻辑。
func (m *Muxer) WritePSChunk(chunk []byte) error {
	return m.write(chunk)
}

// write 是唯一的写出口，负责统计字节数。
func (m *Muxer) write(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n, err := m.w.Write(b)
	m.written += int64(n)
	if err != nil {
		return fmt.Errorf("写入 PS 字节流失败：%w", err)
	}
	return nil
}

// SplitAnnexB 按起始码（00 00 01 或 00 00 00 01）把 Annex-B 字节流切成 NAL 列表。
//
// 返回的每个元素**不含起始码**，且已去掉尾部的 zero_8bits / cabac_zero_word 填充
// （H.264 的 Annex-B 允许在 NAL 之间插入任意多个 0x00）。
func SplitAnnexB(data []byte) [][]byte {
	var out [][]byte
	start := -1
	for i := 0; i+2 < len(data); {
		if data[i] == 0x00 && data[i+1] == 0x00 && data[i+2] == 0x01 {
			if start >= 0 {
				if nal := trimTrailingZeros(data[start:i]); len(nal) > 0 {
					out = append(out, nal)
				}
			}
			i += 3
			start = i
			continue
		}
		i++
	}
	if start >= 0 {
		if nal := trimTrailingZeros(data[start:]); len(nal) > 0 {
			out = append(out, nal)
		}
	}
	return out
}

// trimTrailingZeros 去掉 NAL 尾部的 0x00 填充帧。
func trimTrailingZeros(b []byte) []byte {
	end := len(b)
	for end > 0 && b[end-1] == 0x00 {
		end--
	}
	return b[:end]
}

// NALType 返回 NAL 单元类型。
//
// H.264 用首字节的低 5 位；H.265 用 (首字节 >> 1) & 0x3F。这里按 H.264 处理，
// H.265 支持留给后续批次（本批次只做 H.264）。
func NALType(nal []byte) byte {
	if len(nal) == 0 {
		return 0
	}
	return nal[0] & 0x1F
}

// IsVCL 判断是否为承载图像数据的 VCL NAL（类型 1 与 5）。
func IsVCL(t byte) bool {
	return t == NALTypeNonIDR || t == NALTypeIDR
}

// JoinAnnexB 用 4 字节起始码把多个 NAL 拼成一段 Annex-B 码流。
func JoinAnnexB(nals [][]byte) []byte {
	var buf bytes.Buffer
	for _, nal := range nals {
		buf.Write([]byte{0x00, 0x00, 0x00, 0x01})
		buf.Write(nal)
	}
	return buf.Bytes()
}

// GroupAccessUnits 把 NAL 列表按「访问单元（一帧）」分组。
//
// 判定规则（H.264 单层码流的通行启发式）：
//   - SPS/PPS/SEI/AUD 等非 VCL NAL 若出现在某个 VCL NAL 之后，则开启新的一帧；
//   - 连续两个 VCL NAL 中，前一个是 IDR 之外的 slice 且当前是新的 slice 起始，
//     在「一帧一 slice」的常见码流里同样意味着新帧。
//
// 对模拟器手里的样本码流（每个访问单元以 AUD/SPS 开头）这条规则足够。
func GroupAccessUnits(nals [][]byte) [][][]byte {
	var units [][][]byte
	var cur [][]byte
	seenVCL := false
	for _, nal := range nals {
		t := NALType(nal)
		if seenVCL && !IsVCL(t) {
			// 非 VCL NAL 落在 VCL 之后 → 这是下一帧的前导（AUD/SPS/PPS/SEI）
			units = append(units, cur)
			cur = nil
			seenVCL = false
		}
		if IsVCL(t) && seenVCL {
			// 一帧一 slice 的码流：第二个 VCL 说明上一帧已结束
			if t == NALTypeNonIDR {
				units = append(units, cur)
				cur = nil
			}
		}
		cur = append(cur, nal)
		if IsVCL(t) {
			seenVCL = true
		}
	}
	if len(cur) > 0 {
		units = append(units, cur)
	}
	return units
}

// BuildPackHeader 返回 14 字节的 MPEG-2 PS 包头。
//
// 字段取值：SCR 全 0、program_mux_rate = 0、无 stuffing。
// 这是 GB28181 生态里的通行固定值（ZLM 侧只校验起始码与长度，不校验这些字段）。
func BuildPackHeader() []byte {
	return []byte{
		0x00, 0x00, 0x01, 0xBA,
		0x44, 0x00, 0x04, 0x00, 0x04, 0x01, 0x00, 0x00, 0x03, 0xF8,
	}
}

// BuildPSM 返回一个只声明单条视频基本流的 program_stream_map。
//
// 结构（ISO/IEC 13818-1 2.5.4）：
//
//	start_code(4) | length(2) | version(1) | marker(1) | ps_info_len(2)
//	| es_map_len(2) | [stream_type(1) es_id(1) es_info_len(2)] | CRC32(4)
func BuildPSM(streamType byte) []byte {
	es := []byte{streamType, StreamIDVideo, 0x00, 0x00}
	body := []byte{
		0xE0,       // current_next_indicator=1, reserved='11', map_version=0
		0xFF,       // reserved='1111111' + marker_bit=1
		0x00, 0x00, // program_stream_info_length = 0
	}
	body = append(body, byte(len(es)>>8), byte(len(es)))
	body = append(body, es...)
	crc := CRC32MPEG2(body)
	body = append(body, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))

	out := make([]byte, 0, 6+len(body))
	out = append(out, 0x00, 0x00, 0x01, 0xBC, byte(len(body)>>8), byte(len(body)))
	return append(out, body...)
}

// BuildPES 构造一个只带 PTS 的 PES 包。
//
// 负载超过 65527 字节时 PES_packet_length 写 0（视频基本流的合法写法，表示长度不定，
// 由后续起始码定界）；调用方通常已经按 maxPESPayload 切好片。
func BuildPES(streamID byte, pts uint64, payload []byte) []byte {
	ptsBytes := encodePTS(pts)
	pesLen := 3 + len(ptsBytes) + len(payload)

	buf := make([]byte, 0, 9+len(payload))
	buf = append(buf, 0x00, 0x00, 0x01, streamID)
	if pesLen > 0xFFFF {
		buf = append(buf, 0x00, 0x00)
	} else {
		buf = append(buf, byte(pesLen>>8), byte(pesLen))
	}
	buf = append(buf, 0x80)                // '10' + PES_scrambling/priority/alignment… 全 0
	buf = append(buf, 0x80)                // PTS_DTS_flags='10'（只带 PTS）
	buf = append(buf, byte(len(ptsBytes))) // PES_header_data_length
	buf = append(buf, ptsBytes...)
	return append(buf, payload...)
}

// encodePTS 把 33 位 PTS 编成 ISO/IEC 13818-1 规定的 5 字节标记形式：
// '0010' PTS[32:30] '1' PTS[29:15] '1' PTS[14:0] '1'。
func encodePTS(pts uint64) []byte {
	p := pts & 0x1FFFFFFFF
	return []byte{
		byte(((p>>30)&0x07)<<1) | 0x21,
		byte((p >> 22) & 0xFF),
		byte(((p>>15)&0x7F)<<1) | 0x01,
		byte((p >> 7) & 0xFF),
		byte((p&0x7F)<<1) | 0x01,
	}
}

// DecodePTS 是 encodePTS 的逆运算，供测试与排障使用。
func DecodePTS(b []byte) (uint64, error) {
	if len(b) < 5 {
		return 0, fmt.Errorf("PTS 需要 5 字节，实际 %d", len(b))
	}
	if b[0]&0x01 != 0x01 || b[2]&0x01 != 0x01 || b[4]&0x01 != 0x01 {
		return 0, fmt.Errorf("PTS 标记位不合法：%x", b[:5])
	}
	if b[0]&0xF0 != 0x20 {
		return 0, fmt.Errorf("PTS 前缀应为 '0010'，实际 %x", b[0])
	}
	p := uint64(b[0]>>1&0x07) << 30
	p |= uint64(b[1]) << 22
	p |= uint64(b[2]>>1) << 15
	p |= uint64(b[3]) << 7
	p |= uint64(b[4] >> 1)
	return p, nil
}

// CRC32MPEG2 计算 ISO/IEC 13818-1 附录 A 规定的 CRC-32/MPEG-2：
// 多项式 0x04C11DB7，初值全 1，输入输出均不反转，不做末尾异或。
func CRC32MPEG2(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc ^= uint32(b) << 24
		for i := 0; i < 8; i++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
