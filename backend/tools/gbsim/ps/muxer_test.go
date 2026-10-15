package ps

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"os"
	"testing"
)

// demuxPES 是测试用的**独立** PS 反向解析器。
//
// 它刻意不复用封装器里的任何代码，只看字节流本身：
// 起始码 00 00 01 BA（PS 包头）/ BC（PSM）/ E0-EF（视频 PES）。
// 这样往返测试才能真正验证「封装结果符合标准」，而不是「自己解自己」。
type demuxPES struct {
	streamID byte
	pts      uint64
	payload  []byte
}

func demuxPS(t *testing.T, data []byte) (pes []demuxPES, psmCount int) {
	t.Helper()
	for i := 0; i+4 <= len(data); {
		if data[i] != 0x00 || data[i+1] != 0x00 || data[i+2] != 0x01 {
			i++
			continue
		}
		switch data[i+3] {
		case 0xBA: // PS 包头：起始码 4 字节 + pack_header 10 字节
			i += 14
		case 0xBC: // PSM
			if i+6 > len(data) {
				t.Fatalf("PSM 截断于偏移 %d", i)
			}
			l := int(data[i+4])<<8 | int(data[i+5])
			i += 6 + l
			psmCount++
		default:
			// PES：0xE0-0xEF 视频、0xC0-0xDF 音频、0xBD 私有流
			if data[i+3] < 0xBC || data[i+3] > 0xEF || i+9 > len(data) {
				i += 3
				continue
			}
			l := int(data[i+4])<<8 | int(data[i+5])
			if l == 0 {
				t.Fatalf("测试解析器不支持 PES_packet_length=0 的不定长写法（偏移 %d）", i)
			}
			// PES 头从 i+6 起：flags(1) PTS_DTS_flags(1) header_data_length(1) 可选头(hdrLen) 负载
			flags := data[i+6]
			if flags&0xC0 != 0x80 {
				t.Fatalf("PES 标志字节高两位应为 '10'，实际 %02x（偏移 %d）", flags, i)
			}
			ptsDTS := data[i+7]
			hdrLen := int(data[i+8])
			p := demuxPES{streamID: data[i+3]}
			if ptsDTS&0x80 != 0 {
				decoded, err := DecodePTS(data[i+9 : i+9+5])
				if err != nil {
					t.Fatalf("解析 PTS 失败：%v（偏移 %d）", err, i)
				}
				p.pts = decoded
			}
			payloadStart := i + 9 + hdrLen
			payloadEnd := i + 6 + l
			if payloadEnd > len(data) || payloadStart > payloadEnd {
				t.Fatalf("PES 负载越界：start=%d end=%d len=%d", payloadStart, payloadEnd, len(data))
			}
			p.payload = data[payloadStart:payloadEnd]
			pes = append(pes, p)
			i = payloadEnd
		}
	}
	return pes, psmCount
}

// syntheticAnnexB 造一段形状真实的 Annex-B 码流：两个访问单元，
// 每个访问单元 = AUD + SPS + PPS + SEI + IDR（或非 IDR slice）。
func syntheticAnnexB() ([]byte, [][]byte) {
	rnd := rand.New(rand.NewSource(20261015))
	mk := func(nalType byte, n int) []byte {
		nal := make([]byte, n+1)
		nal[0] = 0x60 | nalType // nal_ref_idc=3, 后 5 位为类型
		rnd.Read(nal[1:])
		return nal
	}
	var nals [][]byte
	for frame := 0; frame < 2; frame++ {
		nals = append(nals,
			mk(NALTypeAUD, 2),
			mk(NALTypeSPS, 20),
			mk(NALTypePPS, 6),
			mk(NALTypeSEI, 12),
		)
		if frame == 0 {
			nals = append(nals, mk(NALTypeIDR, 900))
		} else {
			nals = append(nals, mk(NALTypeNonIDR, 700))
		}
	}
	// 混合 3 字节与 4 字节起始码，验证解析器两种都能吃
	var buf bytes.Buffer
	for i, nal := range nals {
		if i%2 == 0 {
			buf.Write([]byte{0x00, 0x00, 0x00, 0x01})
		} else {
			buf.Write([]byte{0x00, 0x00, 0x01})
		}
		buf.Write(nal)
	}
	return buf.Bytes(), nals
}

// TestSplitAnnexBAndRoundTripViaPES 是核心往返测试：
// Annex-B → 切 NAL → 每 NAL 一个 PES → 反向解析 → NAL 必须逐字节一致。
func TestSplitAnnexBAndRoundTripViaPES(t *testing.T) {
	stream, want := syntheticAnnexB()

	got := SplitAnnexB(stream)
	if len(got) != len(want) {
		t.Fatalf("切出的 NAL 数量不符：want %d got %d", len(want), len(got))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Fatalf("第 %d 个 NAL 切分不一致：want %x got %x", i, want[i], got[i])
		}
	}

	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WritePSHeader(); err != nil {
		t.Fatalf("写 PS 头失败：%v", err)
	}
	for i, nal := range got {
		if err := m.WriteNAL(uint64(1000+i*3600), nal); err != nil {
			t.Fatalf("写 NAL 失败：%v", err)
		}
	}

	pes, psmCount := demuxPS(t, buf.Bytes())
	if psmCount != 1 {
		t.Fatalf("期望恰好 1 个 PSM，实际 %d", psmCount)
	}
	if len(pes) != len(got) {
		t.Fatalf("反向解析出的 PES 数量不符：want %d got %d", len(got), len(pes))
	}
	for i, p := range pes {
		if p.streamID != StreamIDVideo {
			t.Fatalf("第 %d 个 PES 的 stream_id 应为 0xE0，实际 %02x", i, p.streamID)
		}
		if !bytes.Equal(p.payload, got[i]) {
			t.Fatalf("第 %d 个 NAL 往返不一致\nwant %x\ngot  %x", i, got[i], p.payload)
		}
		if wantPTS := uint64(1000 + i*3600); p.pts != wantPTS {
			t.Fatalf("第 %d 个 PES 的 PTS 不符：want %d got %d", i, wantPTS, p.pts)
		}
	}
	if m.BytesWritten() != int64(buf.Len()) {
		t.Fatalf("BytesWritten 统计错误：%d vs %d", m.BytesWritten(), buf.Len())
	}
}

// TestRoundTripViaAccessUnit 验证「一个访问单元一个 PES」的封装：
// 每个 PES 的负载应等于该访问单元所有 NAL 以 4 字节起始码拼接的结果。
func TestRoundTripViaAccessUnit(t *testing.T) {
	stream, _ := syntheticAnnexB()
	units := GroupAccessUnits(SplitAnnexB(stream))
	if len(units) != 2 {
		t.Fatalf("期望切成 2 个访问单元，实际 %d", len(units))
	}
	if len(units[0]) != 5 || len(units[1]) != 5 {
		t.Fatalf("每个访问单元应含 5 个 NAL，实际 %d / %d", len(units[0]), len(units[1]))
	}

	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if n, err := m.WriteAnnexBStream(stream, 90000, 3600); err != nil {
		t.Fatalf("WriteAnnexBStream 失败：%v", err)
	} else if n != 2 {
		t.Fatalf("WriteAnnexBStream 返回的访问单元数应为 2，实际 %d", n)
	}

	pes, psmCount := demuxPS(t, buf.Bytes())
	if psmCount != 1 {
		t.Fatalf("期望 1 个 PSM，实际 %d", psmCount)
	}
	if len(pes) != 2 {
		t.Fatalf("期望 2 个 PES，实际 %d", len(pes))
	}
	for i, u := range units {
		want := JoinAnnexB(u)
		if !bytes.Equal(pes[i].payload, want) {
			t.Fatalf("第 %d 个访问单元往返不一致\nwant %x\ngot  %x", i, want, pes[i].payload)
		}
		if wantPTS := uint64(90000 + i*3600); pes[i].pts != wantPTS {
			t.Fatalf("第 %d 个访问单元 PTS 不符：want %d got %d", i, wantPTS, pes[i].pts)
		}
	}
}

// TestPSHeaderLayout 逐字节校验 PS 包头与 PSM 的布局。
func TestPSHeaderLayout(t *testing.T) {
	pack := BuildPackHeader()
	wantPack := []byte{0x00, 0x00, 0x01, 0xBA, 0x44, 0x00, 0x04, 0x00, 0x04, 0x01, 0x00, 0x00, 0x03, 0xF8}
	if !bytes.Equal(pack, wantPack) {
		t.Fatalf("PS 包头不符\nwant %x\ngot  %x", wantPack, pack)
	}
	if len(pack) != 14 {
		t.Fatalf("PS 包头应为 14 字节，实际 %d", len(pack))
	}

	psm := BuildPSM(StreamTypeH264)
	if !bytes.HasPrefix(psm, []byte{0x00, 0x00, 0x01, 0xBC}) {
		t.Fatalf("PSM 起始码不符：%x", psm[:4])
	}
	length := int(psm[4])<<8 | int(psm[5])
	if length != len(psm)-6 {
		t.Fatalf("PSM 长度字段（%d）与实际剩余字节（%d）不符", length, len(psm)-6)
	}
	// body = version(1) + marker(1) + ps_info_len(2) + es_map_len(2) + es(4) + crc(4)
	if length != 14 {
		t.Fatalf("单条基本流时 program_stream_map_length 应为 14，实际 %d", length)
	}
	buf := psm[6:]
	if buf[0] != 0xE0 {
		t.Fatalf("current_next_indicator/version 字节应为 0xE0，实际 %02x", buf[0])
	}
	if buf[1] != 0xFF {
		t.Fatalf("保留位+marker 字节应为 0xFF，实际 %02x", buf[1])
	}
	if esMapLen := int(buf[4])<<8 | int(buf[5]); esMapLen != 4 {
		t.Fatalf("elementary_stream_map_length 应为 4，实际 %d", esMapLen)
	}
	if buf[6] != StreamTypeH264 {
		t.Fatalf("基本流类型应为 H.264(0x1B)，实际 %02x", buf[6])
	}
	if buf[7] != StreamIDVideo {
		t.Fatalf("基本流 ID 应为 0xE0，实际 %02x", buf[7])
	}
	// CRC 必须覆盖 body 的前 10 字节
	wantCRC := CRC32MPEG2(buf[:len(buf)-4])
	gotCRC := binary.BigEndian.Uint32(buf[len(buf)-4:])
	if gotCRC != wantCRC {
		t.Fatalf("PSM CRC32 不符：want %08x got %08x", wantCRC, gotCRC)
	}
}

// TestCRC32MPEG2CheckValue 用 CRC-32/MPEG-2 的标准检查值锁定实现。
func TestCRC32MPEG2CheckValue(t *testing.T) {
	if got := CRC32MPEG2([]byte("123456789")); got != 0x0376E6E7 {
		t.Fatalf("CRC-32/MPEG-2 检查值不符：want 0376e6e7 got %08x", got)
	}
}

// TestPTSRoundTrip 覆盖 PTS 编解码的边界值（33 位空间与标记位）。
func TestPTSRoundTrip(t *testing.T) {
	for _, pts := range []uint64{0, 1, 0x7F, 0x80, 3600, 90000, 1 << 30, 1<<33 - 1} {
		enc := encodePTS(pts)
		if len(enc) != 5 {
			t.Fatalf("PTS %d 编码后应为 5 字节，实际 %d", pts, len(enc))
		}
		// 标记位：首字节 '0010' 前缀，三个 marker bit 必须为 1
		if enc[0]&0xF0 != 0x20 || enc[0]&0x01 != 1 || enc[2]&0x01 != 1 || enc[4]&0x01 != 1 {
			t.Fatalf("PTS %d 的标记位不合法：%x", pts, enc)
		}
		got, err := DecodePTS(enc)
		if err != nil {
			t.Fatalf("PTS %d 解码失败：%v", pts, err)
		}
		if got != pts {
			t.Fatalf("PTS 往返不一致：want %d got %d", pts, got)
		}
	}
	// 超过 33 位必须被截断（MPEG 只允许 33 位）
	if got, _ := DecodePTS(encodePTS(1<<33 | 5)); got != 5 {
		t.Fatalf("超 33 位应被截断为低 33 位，实际 %d", got)
	}
}

// TestDecodePTSRejectsMalformed 验证逆运算对畸形输入的拒绝。
func TestDecodePTSRejectsMalformed(t *testing.T) {
	if _, err := DecodePTS([]byte{0x01, 0x02}); err == nil {
		t.Fatal("长度不足应当报错")
	}
	if _, err := DecodePTS([]byte{0x11, 0x00, 0x00, 0x00, 0x00}); err == nil {
		t.Fatal("前缀非 '0010' 应当报错")
	}
}

// TestSplitAnnexBEdgeCases 覆盖起始码切分的边界：无起始码、纯填充、尾部零字节。
func TestSplitAnnexBEdgeCases(t *testing.T) {
	if got := SplitAnnexB(nil); len(got) != 0 {
		t.Fatalf("空输入应返回空列表，实际 %v", got)
	}
	if got := SplitAnnexB([]byte{1, 2, 3, 4}); len(got) != 0 {
		t.Fatalf("无起始码应返回空列表，实际 %x", got)
	}
	// 尾部 padding：NAL 与下一个起始码之间的 0x00 属于填充，不计入 NAL
	stream := []byte{
		0x00, 0x00, 0x00, 0x01, 0x67, 0xAA,
		0x00, 0x00, // trailing_zero_8bits
		0x00, 0x00, 0x01, 0x68, 0xBB,
		0x00, // 尾部补零
	}
	got := SplitAnnexB(stream)
	if len(got) != 2 {
		t.Fatalf("期望 2 个 NAL，实际 %d：%x", len(got), got)
	}
	if !bytes.Equal(got[0], []byte{0x67, 0xAA}) || !bytes.Equal(got[1], []byte{0x68, 0xBB}) {
		t.Fatalf("NAL 内容不符：%x", got)
	}
}

// TestSplitAnnexBRealSample 用真实样本文件做一次切分，确认能切出 SPS/PPS。
func TestSplitAnnexBRealSample(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample.h264")
	if err != nil {
		t.Skipf("样本文件不可用，跳过：%v", err)
	}
	nals := SplitAnnexB(data)
	if len(nals) == 0 {
		t.Fatal("样本切不出任何 NAL")
	}
	var hasSPS, hasPPS, hasVCL bool
	for _, nal := range nals {
		switch NALType(nal) {
		case NALTypeSPS:
			hasSPS = true
		case NALTypePPS:
			hasPPS = true
		case NALTypeIDR, NALTypeNonIDR:
			hasVCL = true
		}
	}
	if !hasSPS || !hasPPS || !hasVCL {
		t.Fatalf("样本应含 SPS/PPS/VCL，实际 SPS=%v PPS=%v VCL=%v（共 %d 个 NAL）",
			hasSPS, hasPPS, hasVCL, len(nals))
	}

	// 整段样本走一遍封装 + 反向解析，确认逐字节一致
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WritePSHeader(); err != nil {
		t.Fatalf("写 PS 头失败：%v", err)
	}
	for i, nal := range nals {
		if err := m.WriteNAL(uint64(i)*3600, nal); err != nil {
			t.Fatalf("写 NAL 失败：%v", err)
		}
	}
	pes, _ := demuxPS(t, buf.Bytes())
	if len(pes) != len(nals) {
		t.Fatalf("样本往返的 PES 数量不符：want %d got %d", len(nals), len(pes))
	}
	for i := range nals {
		if !bytes.Equal(pes[i].payload, nals[i]) {
			t.Fatalf("样本第 %d 个 NAL 往返不一致", i)
		}
	}
	t.Logf("样本校验通过：%d 字节 / %d 个 NAL / PS 输出 %d 字节",
		len(data), len(nals), buf.Len())
}

// TestGroupAccessUnits 验证访问单元分组启发式。
func TestGroupAccessUnits(t *testing.T) {
	mk := func(tp byte) []byte { return []byte{0x60 | tp, 0x00, 0x01} }
	units := GroupAccessUnits([][]byte{
		mk(NALTypeAUD), mk(NALTypeSPS), mk(NALTypePPS), mk(NALTypeIDR),
		mk(NALTypeAUD), mk(NALTypeSPS), mk(NALTypePPS), mk(NALTypeNonIDR),
		mk(NALTypeNonIDR),
	})
	if len(units) != 3 {
		t.Fatalf("期望 3 个访问单元，实际 %d", len(units))
	}
	if len(units[0]) != 4 || len(units[1]) != 4 || len(units[2]) != 1 {
		t.Fatalf("分组大小不符：%d/%d/%d", len(units[0]), len(units[1]), len(units[2]))
	}
}

// TestLargeNALChunking 验证超过单个 PES 容量的 NAL 会被切成多个 PES，
// 且反向拼接后与原始 NAL 完全一致。
func TestLargeNALChunking(t *testing.T) {
	nal := make([]byte, maxPESPayload*2+123)
	nal[0] = 0x65 // IDR
	for i := 1; i < len(nal); i++ {
		nal[i] = byte(i % 251)
	}
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	if err := m.WriteNAL(4242, nal); err != nil {
		t.Fatalf("写大 NAL 失败：%v", err)
	}
	pes, _ := demuxPS(t, buf.Bytes())
	if len(pes) != 3 {
		t.Fatalf("期望切成 3 个 PES，实际 %d", len(pes))
	}
	var joined []byte
	for _, p := range pes {
		joined = append(joined, p.payload...)
		if p.pts != 4242 {
			t.Fatalf("切片应复用同一 PTS，实际 %d", p.pts)
		}
	}
	if !bytes.Equal(joined, nal) {
		t.Fatalf("大 NAL 分片往返不一致：len want %d got %d", len(nal), len(joined))
	}
}

// TestMuxerConcurrentWriteSafety 确认封装器在多 goroutine 写同一 io.Writer 时不会交错破坏字节数统计。
func TestMuxerConcurrentWriteSafety(t *testing.T) {
	var buf bytes.Buffer
	m := NewMuxer(&buf)
	const workers = 8
	done := make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 20; j++ {
				nal := []byte{0x41, byte(i), byte(j)}
				if err := m.WriteNAL(uint64(j), nal); err != nil {
					t.Errorf("并发写入失败：%v", err)
					return
				}
			}
		}(i)
	}
	for i := 0; i < workers; i++ {
		<-done
	}
	if m.BytesWritten() != int64(buf.Len()) {
		t.Fatalf("并发下字节数统计不一致：%d vs %d", m.BytesWritten(), buf.Len())
	}
	// 每个 PS 包 = 14 字节包头 + 4 字节起始码 + 2 字节长度 + 3 字节标志 + 5 字节 PTS + 3 字节负载
	const perPacket = 14 + 4 + 2 + 3 + 5 + 3
	if want := int64(workers * 20 * perPacket); m.BytesWritten() != want {
		t.Fatalf("总字节数不符：want %d got %d", want, m.BytesWritten())
	}
}
