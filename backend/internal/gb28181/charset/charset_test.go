package charset

import (
	"bytes"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":          GB2312,
		"gb2312":    GB2312,
		"GBK":       GB2312,
		"gb18030":   GB2312,
		"utf-8":     UTF8,
		"  UTF-8  ": UTF8,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Fatalf("Normalize(%q)=%q，期望 %q", in, got, want)
		}
	}
}

func TestDeclared(t *testing.T) {
	if got := Declared([]byte(`<?xml version="1.0" encoding="GB2312"?><a/>`)); got != "GB2312" {
		t.Fatalf("Declared = %q", got)
	}
	if got := Declared([]byte(`<?xml version="1.0"?><a/>`)); got != "" {
		t.Fatalf("无 encoding 属性应返回空：%q", got)
	}
	if got := Declared([]byte(`<a/>`)); got != "" {
		t.Fatalf("无声明应返回空：%q", got)
	}
	// BOM 前缀
	if got := Declared(append([]byte{0xEF, 0xBB, 0xBF}, []byte(`<?xml version="1.0" encoding="UTF-8"?><a/>`)...)); got != "UTF-8" {
		t.Fatalf("BOM 场景 Declared = %q", got)
	}
}

func TestWithDeclaration(t *testing.T) {
	out := WithDeclaration([]byte(`<?xml version="1.0" encoding="GB2312"?><a>中文</a>`), UTF8)
	if !bytes.Contains(out, []byte(`encoding="UTF-8"`)) {
		t.Fatalf("声明未改写：%q", out)
	}
	if bytes.Contains(out, []byte("GB2312")) {
		t.Fatalf("旧声明残留：%q", out)
	}
	if !bytes.Contains(out, []byte("中文")) {
		t.Fatalf("正文被破坏：%q", out)
	}

	// 没有声明时补一行
	out = WithDeclaration([]byte("<a>中文</a>"), GB2312)
	if !bytes.HasPrefix(out, []byte(`<?xml version="1.0" encoding="GB2312"?>`)) {
		t.Fatalf("未补声明：%q", out)
	}

	// 有声明但没有 encoding 属性
	out = WithDeclaration([]byte(`<?xml version="1.0"?><a/>`), UTF8)
	if !bytes.Contains(out, []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
		t.Fatalf("未补 encoding 属性：%q", out)
	}
}

func TestEncodeForDevice(t *testing.T) {
	const xml = `<?xml version="1.0" encoding="UTF-8"?><Name>东门球机</Name>`

	utf8Body, err := EncodeForDevice(xml, UTF8)
	if err != nil {
		t.Fatalf("UTF-8 编码失败：%v", err)
	}
	if !bytes.Contains(utf8Body, []byte("东门球机")) || !bytes.Contains(utf8Body, []byte(`encoding="UTF-8"`)) {
		t.Fatalf("UTF-8 输出错误：%q", utf8Body)
	}

	gbBody, err := EncodeForDevice(xml, GB2312)
	if err != nil {
		t.Fatalf("GB2312 编码失败：%v", err)
	}
	if !bytes.Contains(gbBody, []byte(`encoding="GB2312"`)) {
		t.Fatalf("GB2312 声明错误：%q", gbBody)
	}
	if bytes.Contains(gbBody, []byte("东门球机")) {
		t.Fatalf("GB2312 输出仍含 UTF-8 中文：%q", gbBody)
	}
	// 往返：按 GB2312 解回 UTF-8
	back, err := ToUTF8(gbBody, GB2312)
	if err != nil {
		t.Fatalf("ToUTF8 失败：%v", err)
	}
	if !strings.Contains(string(back), "东门球机") {
		t.Fatalf("往返后中文丢失：%q", back)
	}
}

func TestIsGB(t *testing.T) {
	for _, cs := range []string{"GB2312", "gbk", "GB18030", "EUC-CN", "CP936"} {
		if !IsGB(cs) {
			t.Fatalf("%q 应识别为 GB 系", cs)
		}
	}
	for _, cs := range []string{"", "UTF-8", "Shift_JIS"} {
		if IsGB(cs) {
			t.Fatalf("%q 不应识别为 GB 系", cs)
		}
	}
}
