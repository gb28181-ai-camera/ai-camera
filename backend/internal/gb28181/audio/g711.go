// Package audio 提供国标语音链路的纯函数：G.711A（PCMA）编解码、
// RTP 包解析/封装、乱序序号缓冲与 SSRC/流名换算。
//
// 为什么把这些函数单独成包：语音链路的正确性（尤其是 G.711 与 RTP 头）
// 不依赖数据库、SIP 会话或 ZLM，可以完全用单元测试钉住；而 voice_service
// 里的编排需要真实环境，不适合做细粒度回归。
package audio

import "encoding/binary"

// G.711 的采样率与 20ms 帧长。
const (
	// SampleRate 是 G.711 窄带语音的采样率。
	SampleRate = 8000
	// SamplesPer20ms 是 20ms 对应的采样数（160）。
	SamplesPer20ms = SampleRate / 50
	// PCM16BytesPer20ms 是 160 个 16-bit 采样占用的字节数（320）。
	PCM16BytesPer20ms = SamplesPer20ms * 2
	// PCMABytesPer20ms 是 160 个 A-law 字节（160）。
	PCMABytesPer20ms = SamplesPer20ms
)

// alawSegEnd 是 A-law 各段的上界。与 ITU-T G.711 的 Sun 实现一致。
var alawSegEnd = [8]int{0xFF, 0x1FF, 0x3FF, 0x7FF, 0xFFF, 0x1FFF, 0x3FFF, 0x7FFF}

// PCMAEncodeSample 把单个 16-bit PCM 采样编码为 A-law 字节。
//
// 实现直接对应 G.711 的「符号位 + 3 位段号 + 4 位尾数」结构。
// 负数先做 -x-1 再取符号掩码，避免 -32768 取反溢出。
func PCMAEncodeSample(sample int16) byte {
	s := int(sample)
	mask := 0xD5
	if s < 0 {
		mask = 0x55
		s = -s - 1
	}
	if s > 32767 {
		s = 32767
	}

	segment := 8
	for i, end := range alawSegEnd {
		if s <= end {
			segment = i
			break
		}
	}

	value := 0
	if segment >= 8 {
		value = 0x7F
	} else {
		value = segment << 4
		if segment < 2 {
			value |= (s >> 4) & 0x0F
		} else {
			value |= (s >> (segment + 3)) & 0x0F
		}
	}
	return byte(value ^ mask)
}

// PCMADecodeSample 把单个 A-law 字节解码为 16-bit PCM 采样。
//
// 解码结果带 G.711 固有的量化偏置；encode(decode(x)) 对 256 个字节值
// 是稳定的（见测试），因此往返不会漂移。
func PCMADecodeSample(value byte) int16 {
	v := int(value) ^ 0x55
	sample := (v & 0x0F) << 4
	segment := (v & 0x70) >> 4
	switch segment {
	case 0:
		sample += 8
	case 1:
		sample += 0x108
	default:
		sample += 0x108
		sample <<= segment - 1
	}
	if v&0x80 != 0 {
		return int16(sample)
	}
	return int16(-sample)
}

// PCMAToPCM16 批量把 A-law 转为 16-bit PCM（小端、单声道）。
//
// 输出长度恒为输入长度的 2 倍，调用方不需要处理错误。
func PCMAToPCM16(data []byte) []byte {
	out := make([]byte, len(data)*2)
	for i, b := range data {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(PCMADecodeSample(b)))
	}
	return out
}

// PCM16ToPCMA 批量把 16-bit PCM（小端、单声道）转为 A-law。
//
// 长度为奇数时返回错误而不是静默丢半个采样：语音链路里出现奇数字节
// 通常意味着上游帧边界错了，静默截断会把问题藏起来。
func PCM16ToPCMA(pcm []byte) ([]byte, error) {
	if len(pcm)%2 != 0 {
		return nil, ErrOddPCM16Length
	}
	out := make([]byte, len(pcm)/2)
	for i := range out {
		sample := int16(binary.LittleEndian.Uint16(pcm[i*2:]))
		out[i] = PCMAEncodeSample(sample)
	}
	return out, nil
}
