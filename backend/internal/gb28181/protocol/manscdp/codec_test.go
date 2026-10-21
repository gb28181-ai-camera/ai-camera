package manscdp

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
)

// sampleCatalog 返回一份含中文通道名的目录响应。
func sampleCatalog() *CatalogResponse {
	return &CatalogResponse{
		CmdType:  CmdCatalog,
		SN:       42,
		DeviceID: "34020000001320000001",
		SumNum:   2,
		DeviceList: DeviceList{
			Num: 2,
			Items: []CatalogItem{
				{
					DeviceID: "34020000001320000001", Name: "东门球机",
					Manufacturer: "AI Camera", Model: "IPC-1", Owner: "园区",
					CivilCode: "340200", Address: "东门", Parental: 0,
					ParentID: "34020000001320000001", RegisterWay: 1,
					Status: "ON", PTZType: 1,
				},
				{
					DeviceID: "34020000001320000002", Name: "仓库枪机",
					Manufacturer: "AI Camera", Model: "IPC-2",
					RegisterWay: 1, Status: "ON",
				},
			},
		},
	}
}

// 正常 UTF-8：声明与内容一致，往返后中文不变。
func TestMarshalDecodeUTF8(t *testing.T) {
	body, err := Marshal(sampleCatalog(), charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	if !bytes.HasPrefix(body, []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
		t.Fatalf("XML 声明不正确：%q", firstLine(body))
	}

	env, err := Decode(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if env.Root() != "Response" || env.CmdType != CmdCatalog || env.SN != 42 {
		t.Fatalf("信封解析错误：root=%s cmd=%s sn=%d", env.Root(), env.CmdType, env.SN)
	}
	if env.DeviceID != "34020000001320000001" {
		t.Fatalf("DeviceID = %q", env.DeviceID)
	}
	if env.CharsetMismatch {
		t.Fatalf("声明与配置一致时不应标记 mismatch")
	}

	_, out, err := DecodeCatalog(body, charset.UTF8)
	if err != nil {
		t.Fatalf("DecodeCatalog 失败：%v", err)
	}
	if got := out.DeviceList.Items[0].Name; got != "东门球机" {
		t.Fatalf("通道名 = %q，期望 东门球机", got)
	}
	if out.DeviceList.Items[1].Name != "仓库枪机" {
		t.Fatalf("第二条通道名 = %q", out.DeviceList.Items[1].Name)
	}
	if out.DeviceList.Items[0].PTZType != 1 {
		t.Fatalf("嵌套 Info>PTZType 未解析：%d", out.DeviceList.Items[0].PTZType)
	}
}

// 声明 GB2312 + 内容 GB2312：必须能正确解回中文（结论六的正向路径）。
func TestMarshalDecodeGB2312(t *testing.T) {
	body, err := Marshal(sampleCatalog(), charset.GB2312)
	if err != nil {
		t.Fatalf("Marshal(GB2312) 失败：%v", err)
	}
	if !bytes.HasPrefix(body, []byte(`<?xml version="1.0" encoding="GB2312"?>`)) {
		t.Fatalf("XML 声明不正确：%q", firstLine(body))
	}
	// 中文确实被转成了 GB 字节（不是 UTF-8）
	if bytes.Contains(body, []byte("东门球机")) {
		t.Fatalf("报文中仍含 UTF-8 中文，说明没有转码")
	}

	_, out, err := DecodeCatalog(body, charset.GB2312)
	if err != nil {
		t.Fatalf("DecodeCatalog(GB2312) 失败：%v", err)
	}
	if got := out.DeviceList.Items[0].Name; got != "东门球机" {
		t.Fatalf("通道名 = %q，期望 东门球机（GB2312 往返失败）", got)
	}
}

// 声明 GB2312 但设备配置为 UTF-8：按声明解（字节为准），并标记 mismatch 供调用方告警。
func TestDecodeCharsetMismatch(t *testing.T) {
	body, err := Marshal(sampleCatalog(), charset.GB2312)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	env, err := Decode(body, charset.UTF8)
	if err != nil {
		t.Fatalf("Decode 失败：%v", err)
	}
	if !env.CharsetMismatch {
		t.Fatalf("声明 GB2312、配置 UTF-8 时应标记 CharsetMismatch")
	}
}

// 无 XML 声明 + GB2312 字节：必须报错（设计 §2.6 第 3 行）。
// 这是「设备注册上来了但目录一条都没有」的根因，绝不能被静默吞掉。
func TestDecodeNoDeclarationGBBytesMustFail(t *testing.T) {
	body, err := Marshal(sampleCatalog(), charset.GB2312)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	stripped := stripDeclaration(body)
	if bytes.Contains(stripped, []byte("<?xml")) {
		t.Fatalf("测试样本仍带声明")
	}

	if _, err := Decode(stripped, charset.GB2312); err == nil {
		t.Fatalf("无声明 + GB2312 字节必须报错，实际解码成功")
	} else if !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("错误信息应指出 UTF-8 问题，实际：%v", err)
	}
	if _, err := Decode(stripped, charset.UTF8); err == nil {
		t.Fatalf("无声明 + GB2312 字节按 UTF-8 解码也必须报错")
	}
}

// 无声明但内容是 UTF-8：允许（部分设备不发 XML 声明）。
func TestDecodeNoDeclarationUTF8(t *testing.T) {
	body, err := Marshal(sampleCatalog(), charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal 失败：%v", err)
	}
	stripped := stripDeclaration(body)
	_, out, err := DecodeCatalog(stripped, charset.UTF8)
	if err != nil {
		t.Fatalf("无声明 UTF-8 应可解码：%v", err)
	}
	if out.DeviceList.Items[0].Name != "东门球机" {
		t.Fatalf("通道名乱码：%q", out.DeviceList.Items[0].Name)
	}
}

// SumNum 分片：两片复用同一 SN，Num 分别是本片条数，SumNum 恒为总数。
func TestCatalogFragments(t *testing.T) {
	chunk1 := &CatalogResponse{
		CmdType: CmdCatalog, SN: 7, DeviceID: "34020000001320000001", SumNum: 3,
		DeviceList: DeviceList{Num: 2, Items: []CatalogItem{
			{DeviceID: "34020000001320000001", Name: "通道一"},
			{DeviceID: "34020000001320000002", Name: "通道二"},
		}},
	}
	chunk2 := &CatalogResponse{
		CmdType: CmdCatalog, SN: 7, DeviceID: "34020000001320000001", SumNum: 3,
		DeviceList: DeviceList{Num: 1, Items: []CatalogItem{
			{DeviceID: "34020000001320000003", Name: "通道三"},
		}},
	}

	total := 0
	for i, chunk := range []*CatalogResponse{chunk1, chunk2} {
		body, err := Marshal(chunk, charset.GB2312)
		if err != nil {
			t.Fatalf("第 %d 片编码失败：%v", i+1, err)
		}
		_, out, err := DecodeCatalog(body, charset.GB2312)
		if err != nil {
			t.Fatalf("第 %d 片解码失败：%v", i+1, err)
		}
		if out.SN != 7 {
			t.Fatalf("第 %d 片 SN = %d，期望 7", i+1, out.SN)
		}
		if out.SumNum != 3 {
			t.Fatalf("第 %d 片 SumNum = %d，期望 3", i+1, out.SumNum)
		}
		if out.DeviceList.Num != len(out.DeviceList.Items) {
			t.Fatalf("第 %d 片 Num=%d 与条目数 %d 不一致", i+1, out.DeviceList.Num, len(out.DeviceList.Items))
		}
		total += len(out.DeviceList.Items)
	}
	if total != 3 {
		t.Fatalf("两片合计 %d 条，期望 3", total)
	}
}

// PTZCmd 是 8 字节二进制，必须原样出现在报文里，不能被 XML 转义/替换。
func TestMarshalPTZControlRawBytes(t *testing.T) {
	cmd, err := BuildDirectionCmd(DirectionUpLeft, 5)
	if err != nil {
		t.Fatalf("构造 PTZ 指令失败：%v", err)
	}
	if len(cmd) != 8 {
		t.Fatalf("PTZCmd 长度 = %d，期望 8", len(cmd))
	}
	ctrl := NewPTZControl("34020000001320000001", 9, cmd)
	body, err := Marshal(ctrl, charset.UTF8)
	if err != nil {
		t.Fatalf("Marshal(Control) 失败：%v", err)
	}
	if bytes.Contains(body, []byte(ptzPlaceholder)) {
		t.Fatalf("占位符未被替换：%q", body)
	}
	if !bytes.Contains(body, cmd) {
		t.Fatalf("报文未包含原始 PTZ 字节：%x\n%s", cmd, body)
	}
	if !bytes.Contains(body, []byte("<CmdType>DeviceControl</CmdType>")) {
		t.Fatalf("CmdType 不正确：%s", body)
	}

	// 注意：不在这里 Decode —— PTZCmd 的 0xA5 等字节不是合法 UTF-8，
	// 而平台的 Decode 对「无声明/非法 UTF-8」是严格拒绝的（防静默乱码）。
	// Control 是平台 → 设备的单向下行报文，入向不会收到它。
	if utf8.Valid(body) {
		t.Fatalf("含二进制 PTZCmd 的报文不应是合法 UTF-8")
	}
}

// PTZ 校验和与速度夹取。
func TestBuildPTZCmdChecksum(t *testing.T) {
	cmd, err := BuildDirectionCmd(DirectionUp, 8)
	if err != nil {
		t.Fatalf("构造失败：%v", err)
	}
	want := []byte{0xA5, 0x0F, 0x01, 0x08, 8, 8, 8}
	want = append(want, checksum(want))
	if !bytes.Equal(cmd, want) {
		t.Fatalf("指令 = %x，期望 %x", cmd, want)
	}

	// 速度越界：0 按最慢（1），99 按最快（8）
	slow, _ := BuildDirectionCmd(DirectionStop, 0)
	if slow[4] != 1 || slow[5] != 1 || slow[6] != 1 {
		t.Fatalf("速度 0 未被夹到 1：%x", slow)
	}
	fast, _ := BuildDirectionCmd(DirectionRight, 99)
	if fast[4] != 8 || fast[5] != 8 || fast[6] != 8 {
		t.Fatalf("速度 99 未被夹到 8：%x", fast)
	}

	if _, err := BuildDirectionCmd("diagonal-warp", 5); err == nil {
		t.Fatalf("未知方向应报错")
	}
}

// 未知 XML 编码必须显式失败，不能猜。
func TestDecodeUnknownEncoding(t *testing.T) {
	body := []byte("<?xml version=\"1.0\" encoding=\"Shift_JIS\"?><Notify><CmdType>Keepalive</CmdType><SN>1</SN><DeviceID>1</DeviceID></Notify>")
	if _, err := Decode(body, charset.GB2312); err == nil {
		t.Fatalf("未知编码应报错")
	}
}

// 缺少 CmdType 的报文必须报错（否则上层无法路由）。
func TestDecodeMissingCmdType(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?><Notify><SN>1</SN></Notify>`)
	if _, err := Decode(body, charset.UTF8); err == nil {
		t.Fatalf("缺少 CmdType 应报错")
	}
}

// 报警与移动位置的结构解析。
func TestDecodeAlarmAndMobilePosition(t *testing.T) {
	alarmXML := `<?xml version="1.0" encoding="UTF-8"?><Notify><CmdType>Alarm</CmdType><SN>3</SN><DeviceID>34020000001320000001</DeviceID><AlarmPriority>1</AlarmPriority><AlarmMethod>5</AlarmMethod><AlarmTime>2026-10-21T10:00:00</AlarmTime><AlarmDescription>移动侦测</AlarmDescription><Longitude>120.1</Longitude><Latitude>30.2</Latitude></Notify>`
	_, alarm, err := DecodeAlarm([]byte(alarmXML), charset.UTF8)
	if err != nil {
		t.Fatalf("DecodeAlarm 失败：%v", err)
	}
	if alarm.AlarmPriority != "1" || alarm.AlarmMethod != "5" || alarm.AlarmDescription != "移动侦测" {
		t.Fatalf("报警字段解析错误：%+v", alarm)
	}
	if alarm.Longitude != 120.1 || alarm.Latitude != 30.2 {
		t.Fatalf("报警经纬度错误：%+v", alarm)
	}

	posXML := `<?xml version="1.0" encoding="UTF-8"?><Notify><CmdType>MobilePosition</CmdType><SN>4</SN><DeviceID>34020000001320000001</DeviceID><Time>2026-10-21T10:00:01</Time><Longitude>120.3</Longitude><Latitude>30.4</Latitude><Speed>12.5</Speed><Direction>90</Direction><Altitude>10</Altitude></Notify>`
	_, pos, err := DecodeMobilePosition([]byte(posXML), charset.UTF8)
	if err != nil {
		t.Fatalf("DecodeMobilePosition 失败：%v", err)
	}
	if pos.Longitude != 120.3 || pos.Speed != 12.5 || pos.Direction != 90 {
		t.Fatalf("位置字段解析错误：%+v", pos)
	}
}

// ────────────────────────────────────────────────────────────────
// 测试辅助
// ────────────────────────────────────────────────────────────────

func stripDeclaration(body []byte) []byte {
	if i := bytes.Index(body, []byte("?>")); i >= 0 {
		rest := body[i+2:]
		return bytes.TrimLeft(rest, "\r\n")
	}
	return body
}

func firstLine(body []byte) string {
	if i := bytes.IndexByte(body, '\n'); i >= 0 {
		return string(body[:i])
	}
	return string(body)
}
