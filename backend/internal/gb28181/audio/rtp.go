package audio

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// RTP 解析错误。单独定义便于调用方区分「这包不是 RTP」与「程序 bug」。
var (
	// ErrOddPCM16Length 表示 16-bit PCM 字节数为奇数。
	ErrOddPCM16Length = errors.New("PCM16 数据长度必须是偶数")
	// ErrRTPTooShort 表示报文不足 12 字节固定头。
	ErrRTPTooShort = errors.New("RTP 报文长度不足 12 字节")
	// ErrRTPVersion 表示版本号不是 2。
	ErrRTPVersion = errors.New("RTP 版本号不是 2")
	// ErrRTPTruncated 表示 CSRC / 扩展头 / 填充声明的长度超出报文。
	ErrRTPTruncated = errors.New("RTP 报文头长度与实际数据不符")
	// ErrRTPPadding 表示 padding 计数非法。
	ErrRTPPadding = errors.New("RTP padding 计数非法")
)

// RTPPacket 是解析后的 RTP 固定头 + 载荷。
//
// 只保留语音链路需要的字段：CSRC 与扩展头在解析时被跳过（但保留原文，
// marshal 时可原样写回），避免为不使用的字段引入额外复杂度。
type RTPPacket struct {
	Version        uint8
	Padding        bool
	Extension      bool
	Marker         bool
	PayloadType    uint8
	SequenceNumber uint16
	Timestamp      uint32
	SSRC           uint32

	CSRC          []uint32
	ExtensionData []byte // 扩展头原文（含 4 字节 profile/length）
	PaddingSize   int

	Payload []byte
}

// RTPHeader 是封装一个语音包所需的最小头信息。
type RTPHeader struct {
	Marker         bool
	PayloadType    uint8
	SequenceNumber uint16
	Timestamp      uint32
	SSRC           uint32
}

const rtpFixedHeaderSize = 12

// ParseRTP 解析一个 RTP 报文。
//
// 安全约束：所有长度字段（CSRC 计数、扩展长度、padding）在切片前都要
// 与报文实际长度比较，避免恶意/损坏报文触发越界 panic。
func ParseRTP(data []byte) (*RTPPacket, error) {
	if len(data) < rtpFixedHeaderSize {
		return nil, ErrRTPTooShort
	}
	version := data[0] >> 6
	if version != 2 {
		return nil, ErrRTPVersion
	}

	p := &RTPPacket{
		Version:        version,
		Padding:        data[0]&0x20 != 0,
		Extension:      data[0]&0x10 != 0,
		Marker:         data[1]&0x80 != 0,
		PayloadType:    data[1] & 0x7F,
		SequenceNumber: binary.BigEndian.Uint16(data[2:4]),
		Timestamp:      binary.BigEndian.Uint32(data[4:8]),
		SSRC:           binary.BigEndian.Uint32(data[8:12]),
	}

	offset := rtpFixedHeaderSize
	csrcCount := int(data[0] & 0x0F)
	if csrcCount > 0 {
		need := offset + csrcCount*4
		if len(data) < need {
			return nil, ErrRTPTruncated
		}
		p.CSRC = make([]uint32, csrcCount)
		for i := 0; i < csrcCount; i++ {
			p.CSRC[i] = binary.BigEndian.Uint32(data[offset+i*4:])
		}
		offset = need
	}

	if p.Extension {
		if len(data) < offset+4 {
			return nil, ErrRTPTruncated
		}
		extWords := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		extLen := 4 + extWords*4
		if len(data) < offset+extLen {
			return nil, ErrRTPTruncated
		}
		p.ExtensionData = append([]byte(nil), data[offset:offset+extLen]...)
		offset += extLen
	}

	end := len(data)
	if p.Padding {
		pad := int(data[end-1])
		// padding 计数包含自身，且不能把固定头/扩展头吃掉。
		if pad == 0 || pad > end-offset {
			return nil, ErrRTPPadding
		}
		p.PaddingSize = pad
		end -= pad
	}

	// 复制载荷：UDP 读缓冲会被复用，不能把切片指向原 buffer。
	p.Payload = append([]byte(nil), data[offset:end]...)
	return p, nil
}

// Marshal 把 RTPPacket 重新序列化（测试与回环工具使用）。
func (p *RTPPacket) Marshal() ([]byte, error) {
	if p == nil {
		return nil, errors.New("RTPPacket 为 nil")
	}
	if len(p.CSRC) > 15 {
		return nil, fmt.Errorf("CSRC 数量 %d 超过 RTP 头 4 bit 上限", len(p.CSRC))
	}
	if p.Extension && len(p.ExtensionData) < 4 {
		return nil, ErrRTPTruncated
	}

	size := rtpFixedHeaderSize + len(p.CSRC)*4 + len(p.ExtensionData) + len(p.Payload) + p.PaddingSize
	out := make([]byte, size)

	out[0] = 2 << 6
	if p.Padding {
		out[0] |= 0x20
	}
	if p.Extension {
		out[0] |= 0x10
	}
	out[0] |= byte(len(p.CSRC))
	out[1] = p.PayloadType & 0x7F
	if p.Marker {
		out[1] |= 0x80
	}
	binary.BigEndian.PutUint16(out[2:4], p.SequenceNumber)
	binary.BigEndian.PutUint32(out[4:8], p.Timestamp)
	binary.BigEndian.PutUint32(out[8:12], p.SSRC)

	offset := rtpFixedHeaderSize
	for _, csrc := range p.CSRC {
		binary.BigEndian.PutUint32(out[offset:], csrc)
		offset += 4
	}
	if p.Extension {
		copy(out[offset:], p.ExtensionData)
		offset += len(p.ExtensionData)
	}
	copy(out[offset:], p.Payload)
	offset += len(p.Payload)
	if p.PaddingSize > 0 {
		// 最后一个字节是 padding 计数；ZLM/设备通常不校验填充内容。
		out[size-1] = byte(p.PaddingSize)
	}
	return out, nil
}

// BuildRTP 用最小头信息封装一个 RTP 包（无 CSRC / 扩展 / padding）。
//
// 这是 Pipecat 下行与测试用的高频路径，避免构造中间结构体。
func BuildRTP(header RTPHeader, payload []byte) []byte {
	out := make([]byte, rtpFixedHeaderSize+len(payload))
	out[0] = 2 << 6
	out[1] = header.PayloadType & 0x7F
	if header.Marker {
		out[1] |= 0x80
	}
	binary.BigEndian.PutUint16(out[2:4], header.SequenceNumber)
	binary.BigEndian.PutUint32(out[4:8], header.Timestamp)
	binary.BigEndian.PutUint32(out[8:12], header.SSRC)
	copy(out[rtpFixedHeaderSize:], payload)
	return out
}

// IsPCMA 判断报文是否是窄带 PCMA（静态负载类型 8）。
//
// 国标设备偶尔会用动态 PT；调用方若配置了其它 PT，应以配置为准，
// 这个辅助函数只用于「未显式配置」时的默认过滤。
func (p *RTPPacket) IsPCMA() bool {
	return p != nil && p.PayloadType == 8
}
