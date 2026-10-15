package manscdp

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// 中文通道名：故意选 GB2312 与 UTF-8 字节完全不同的常用字，
// 保证往返测试真的能区分编码，而不是「恰好都一样」。
const (
	cnName    = "东门球机"
	cnChinese = "东门球机通道1"
)

func sampleCatalog() *Catalog {
	return &Catalog{
		CmdType:  CmdCatalog,
		SN:       1,
		DeviceID: "34020000001320000001",
		SumNum:   1,
		DeviceList: DeviceList{
			Num: 1,
			Items: []CatalogItem{{
				DeviceID:     "34020000001310000001",
				Name:         cnChinese,
				Manufacturer: "Boticz",
				Model:        "AICam-SIM",
				Owner:        "Owner",
				CivilCode:    "340200",
				Address:      "合肥市",
				Parental:     0,
				RegisterWay:  1,
				Secrecy:      0,
				Status:       "ON",
				PTZType:      1,
			}},
		},
	}
}

// TestCatalogRoundTripGB2312 验证「中文通道名 → GB2312 字节 → 解回中文」的完整往返。
// 这是 §2.6 结论六最容易静默出错的地方。
func TestCatalogRoundTripGB2312(t *testing.T) {
	orig := sampleCatalog()
	data, err := Encode(orig, CharsetGB2312)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}

	// 1) 声明必须是 GB2312
	if got := DeclaredCharset(data); got != CharsetGB2312 {
		t.Fatalf("XML 声明的 encoding 应为 GB2312，实际 %q\n报文：%s", got, data)
	}
	// 2) 中文必须是 GB2312 字节，而不是 UTF-8 字节
	if bytes.Contains(data, []byte(cnChinese)) {
		t.Fatalf("报文里出现了 UTF-8 的明文中文，说明没有转码\n报文：%s", data)
	}
	gb, _ := gb2312Bytes(cnChinese)
	if !bytes.Contains(data, gb) {
		t.Fatalf("报文里找不到 GB2312 编码的中文（期望字节 %x）\n报文：%s", gb, data)
	}

	// 3) 解回来必须一字不差
	var back Catalog
	if err := Decode(data, CharsetGB2312, &back); err != nil {
		t.Fatalf("解码失败：%v", err)
	}
	got := back.DeviceList.Items[0]
	if got.Name != cnChinese {
		t.Fatalf("通道名往返不一致：want %q got %q", cnChinese, got.Name)
	}
	if got.Address != "合肥市" {
		t.Fatalf("地址往返不一致：%q", got.Address)
	}
	if got.DeviceID != "34020000001310000001" || back.DeviceID != "34020000001320000001" {
		t.Fatalf("DeviceID 往返不一致：%+v", back)
	}
	if back.SumNum != 1 || back.DeviceList.Num != 1 || back.SN != 1 {
		t.Fatalf("SumNum/Num/SN 往返不一致：%+v", back)
	}
	if got.PTZType != 1 {
		t.Fatalf("Info>PTZType 往返不一致：%d", got.PTZType)
	}
	if !strings.HasPrefix(string(data), `<?xml version="1.0" encoding="GB2312"?>`) {
		t.Fatalf("XML 声明格式不对：%s", data[:60])
	}
}

// TestDecodeNoDeclarationFallback 覆盖「报文没有 XML 声明」这条路径。
//
// 声明缺失时 CharsetReader 根本不会被调用，必须靠 fallback charset 预转码。
// 实测（见 TestRawDecoderRejectsGBBytesWithoutDeclaration）：不给 fallback 时不是静默乱码，
// 而是 encoding/xml 的严格 UTF-8 校验直接报错 —— 两种失败模式都会让目录为空，都不能漏。
func TestDecodeNoDeclarationFallback(t *testing.T) {
	withDecl, err := Encode(sampleCatalog(), CharsetGB2312)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	// 剥掉声明行，模拟老设备不带声明的报文
	body := withDecl[bytes.Index(withDecl, []byte("<Response>")):]

	// A) 不给 fallback（按 UTF-8 解）→ 必须失败，绝不能「悄悄解出一个错的中文名」
	var broken Catalog
	if err := Decode(body, CharsetUTF8, &broken); err == nil {
		t.Fatalf("按 UTF-8 解 GB2312 字节应当报错，实际解出了 %q", broken.DeviceList.Items[0].Name)
	}

	// B) 给 GB2312 fallback → 必须拿到正确中文
	var fixed Catalog
	if err := Decode(body, CharsetGB2312, &fixed); err != nil {
		t.Fatalf("按 GB2312 回退解码失败：%v", err)
	}
	if fixed.DeviceList.Items[0].Name != cnChinese {
		t.Fatalf("回退解码后中文仍不正确：%q", fixed.DeviceList.Items[0].Name)
	}
}

// TestRawDecoderRejectsGBBytesWithoutDeclaration 锁定一个与设计文档 §2.6 表格不同的实测结论：
//
//	无声明 + GB2312 字节 → Go 报 `invalid UTF-8`，**不是**静默乱码。
//
// 若哪天 Go 放宽了严格模式，本测试会失败并提醒我们复核结论六的兜底策略。
func TestRawDecoderRejectsGBBytesWithoutDeclaration(t *testing.T) {
	gb, err := gb2312Bytes(`<Response><CmdType>Catalog</CmdType><Name>` + cnChinese + `</Name></Response>`)
	if err != nil {
		t.Fatalf("构造 GB2312 样本失败：%v", err)
	}
	var v Catalog
	raw := xml.NewDecoder(bytes.NewReader(gb))
	err = raw.Decode(&v)
	if err == nil {
		t.Fatal("期望 encoding/xml 对非 UTF-8 字节报错，实际成功了 —— 结论六的失败模式已变化")
	}
	if !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("期望 invalid UTF-8 错误，实际：%v", err)
	}
}

// TestMismatchedDeclarationIsSilentMojibake 锁定**真正会静默乱码**的那条路径：
// 声明 GB2312 但实际发的是 UTF-8 字节。转码器不报错，解出来是乱码 ——
// 这条路径在代码层面无法自动判别，只能靠真实设备用例兜住。
func TestMismatchedDeclarationIsSilentMojibake(t *testing.T) {
	// 声明 GB2312，内容却是 UTF-8
	raw := []byte(`<?xml version="1.0" encoding="GB2312"?><Response><CmdType>Catalog</CmdType>` +
		`<DeviceID>34020000001320000001</DeviceID><SumNum>0</SumNum><DeviceList Num="0"></DeviceList>` +
		`<DeviceName>` + cnChinese + `</DeviceName></Response>`)
	var env struct {
		XMLName    xml.Name `xml:"Response"`
		CmdType    string   `xml:"CmdType"`
		DeviceName string   `xml:"DeviceName"`
	}
	if err := Decode(raw, CharsetGB2312, &env); err != nil {
		t.Fatalf("声明与内容不匹配时不应报错（这正是危险之处）：%v", err)
	}
	if env.DeviceName == cnChinese {
		t.Fatal("编码不匹配竟解出正确中文，测试样本无效")
	}
	if env.DeviceName == "" {
		t.Fatal("期望得到乱码而非空串")
	}
	// 记录实际乱码形态，便于将来在日志里认出它
	t.Logf("编码不匹配时的静默乱码形态：%q", env.DeviceName)
}

// TestUTF8RoundTrip 覆盖 UTF-8 声明路径：不转码，中文以 UTF-8 字节出门。
func TestUTF8RoundTrip(t *testing.T) {
	data, err := Encode(sampleCatalog(), CharsetUTF8)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	if got := DeclaredCharset(data); got != CharsetUTF8 {
		t.Fatalf("声明应为 UTF-8，实际 %q", got)
	}
	if !bytes.Contains(data, []byte(cnChinese)) {
		t.Fatalf("UTF-8 报文应包含 UTF-8 明文中文")
	}
	var back Catalog
	if err := Decode(data, CharsetUTF8, &back); err != nil {
		t.Fatalf("解码失败：%v", err)
	}
	if back.DeviceList.Items[0].Name != cnChinese {
		t.Fatalf("UTF-8 往返不一致：%q", back.DeviceList.Items[0].Name)
	}
}

// TestCharsetReaderRequiredForGBDeclaration 锁定「声明非 UTF-8 且 CharsetReader 为 nil → 直接报错」
// 这条容易踩坑的路径。若哪天 Go 改了行为，这个测试会失败并提醒我们复核结论六。
func TestCharsetReaderRequiredForGBDeclaration(t *testing.T) {
	data, err := Encode(sampleCatalog(), CharsetGB2312)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	// 故意用裸 Decoder（不给 CharsetReader）—— 这是很多 Go 项目的默认写法
	var v Catalog
	err = xml.NewDecoder(bytes.NewReader(data)).Decode(&v)
	if err == nil {
		t.Fatal("期望裸 Decoder 解析 GB2312 声明时报错，实际成功了 —— 结论六的行为已变化，需复核")
	}
	if !strings.Contains(err.Error(), "CharsetReader") {
		t.Fatalf("期望报错提到 CharsetReader，实际：%v", err)
	}
}

// TestDecodeTrustsDeclaration 验证「声明了 GB2312 就一定按 GB2312 解」，不被 fallback 覆盖。
func TestDecodeTrustsDeclaration(t *testing.T) {
	data, err := Encode(sampleCatalog(), CharsetGB2312)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	// fallback 给一个「错误」的值（UTF-8），声明仍应优先生效
	var back Catalog
	if err := Decode(data, CharsetUTF8, &back); err != nil {
		t.Fatalf("解码失败：%v", err)
	}
	if back.DeviceList.Items[0].Name != cnChinese {
		t.Fatalf("声明 GB2312 时应按 GB2312 解，实际得到 %q", back.DeviceList.Items[0].Name)
	}
}

// TestDecodeUnknownCharset 未知编码必须显式报错，而不是静默吞掉。
func TestDecodeUnknownCharset(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="SHIFT_JIS"?><Response><CmdType>Keepalive</CmdType></Response>`)
	var v Catalog
	err := Decode(raw, CharsetGB2312, &v)
	if err == nil {
		t.Fatal("未知编码应当报错")
	}
	if !strings.Contains(err.Error(), "SHIFT_JIS") {
		t.Fatalf("错误信息应包含未知编码名，实际：%v", err)
	}
}

// TestKeepaliveRoundTrip 覆盖 Notify 根元素与心跳报文。
func TestKeepaliveRoundTrip(t *testing.T) {
	alive := NewKeepalive("34020000001320000001", 42)
	data, err := Encode(alive, CharsetGB2312)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	if !bytes.Contains(data, []byte("<Notify>")) {
		t.Fatalf("心跳的根元素应为 Notify：%s", data)
	}
	env, err := CmdTypeOf(data, CharsetGB2312)
	if err != nil {
		t.Fatalf("CmdTypeOf 失败：%v", err)
	}
	if env.CmdType != CmdKeepalive || env.SN != 42 || env.DeviceID != "34020000001320000001" {
		t.Fatalf("路由字段解析错误：%+v", env)
	}
}

// TestAlarmRoundTrip 覆盖报警报文的中文描述（GB2312 与之相关的又一条链路）。
func TestAlarmRoundTrip(t *testing.T) {
	alarm := &Alarm{
		CmdType:          CmdAlarm,
		SN:               7,
		DeviceID:         "34020000001320000001",
		AlarmPriority:    "1",
		AlarmMethod:      "5",
		AlarmTime:        "2026-10-15T10:00:00",
		AlarmDescription: cnName,
	}
	data, err := Encode(alarm, CharsetGB2312)
	if err != nil {
		t.Fatalf("编码失败：%v", err)
	}
	var back Alarm
	if err := Decode(data, CharsetGB2312, &back); err != nil {
		t.Fatalf("解码失败：%v", err)
	}
	if back.AlarmDescription != cnName {
		t.Fatalf("报警描述往返不一致：%q", back.AlarmDescription)
	}
}

// TestDeclaredCharsetVariants 覆盖声明的各种书写形态。
func TestDeclaredCharsetVariants(t *testing.T) {
	cases := map[string]string{
		`<?xml version="1.0" encoding="GB2312"?><R/>`:                  "GB2312",
		`<?xml version="1.0" encoding='gbk'?><R/>`:                     "gbk",
		`<?xml version="1.0" encoding = "UTF-8" ?><R/>`:                "UTF-8",
		`<?xml version="1.0"?><R/>`:                                    "",
		`<R/>`:                                                         "",
		"\xEF\xBB\xBF" + `<?xml version="1.0" encoding="GB2312"?><R/>`: "GB2312",
	}
	for raw, want := range cases {
		if got := DeclaredCharset([]byte(raw)); got != want {
			t.Errorf("DeclaredCharset(%q) = %q, want %q", raw, got, want)
		}
	}
}

// gb2312Bytes 把 UTF-8 字符串转成 GB2312/GB18030 字节，仅用于断言「报文确实转码了」。
func gb2312Bytes(s string) ([]byte, error) {
	return simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(s))
}
