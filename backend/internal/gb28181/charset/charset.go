// Package charset 处理 GB28181 XML body 的字符集问题（GB2312 ↔ UTF-8）。
//
// 为什么单独成包：GB28181 的中文设备名/通道名以 GB2312 字节进入 XML body，
// 而 Go 的 encoding/xml 只认 UTF-8。转码既发生在「解码设备报文」这一侧，
// 也发生在「编码平台下行报文」这一侧，两侧的规则必须一致，否则会出现
// 「声明是 GB2312、内容却是 UTF-8」这类静默乱码（设计 §2.6 实测表第 5 行）。
//
// 关键约束：**声明值与实际字节必须一致**。本包的所有编码入口都会先写声明、
// 再按同一字符集转码；解码入口则按声明选择解码器，声明缺失时一律按 UTF-8
// 严格解析（非法 UTF-8 直接报错，而不是猜一个编码把它解成乱码）。
package charset

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// 两个对外的字符集取值，与 ac_device.charset 的枚举一致。
const (
	UTF8   = "UTF-8"
	GB2312 = "GB2312"
)

// Normalize 把任意写法规范成本包使用的两个取值之一。
//
// 空值按 GB2312 处理：GB28181 设备（尤其 2016 版）默认就是 GB2312，
// 与 WVP 注册时的兜底一致（设计 §2.6）。
func Normalize(cs string) string {
	if IsGB(cs) {
		return GB2312
	}
	if strings.TrimSpace(cs) == "" {
		return GB2312
	}
	return UTF8
}

// IsGB 判断是否属于 GB 系编码。
//
// GB2312 是 GBK/GB18030 的子集，反过来说 GB18030 能无损解 GB2312 的全部字节，
// 因此一律用 GB18030 做编解码，避免遇到设备写了 GBK 别名就失败。
func IsGB(cs string) bool {
	switch strings.ToUpper(strings.TrimSpace(cs)) {
	case "GB2312", "GB-2312", "GBK", "GB18030", "EUC-CN", "CP936":
		return true
	default:
		return false
	}
}

// ToUTF8 把设备字节转成 UTF-8。cs 为 UTF-8 时原样返回。
func ToUTF8(data []byte, cs string) ([]byte, error) {
	if !IsGB(cs) {
		return data, nil
	}
	out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("按 %s 解码失败：%w", cs, err)
	}
	return out, nil
}

// FromUTF8 把 UTF-8 文本转成设备字符集。cs 为 UTF-8 时原样返回。
func FromUTF8(data []byte, cs string) ([]byte, error) {
	if !IsGB(cs) {
		return data, nil
	}
	out, err := simplifiedchinese.GB18030.NewEncoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("按 %s 编码失败：%w", cs, err)
	}
	return out, nil
}

// Declared 从 XML 声明里取出 encoding 值；没有声明时返回空串。
//
// 只在报文开头查找：声明若不在开头，本来就已违反 XML 规范，按无声明处理。
func Declared(data []byte) string {
	head := data
	if bytes.HasPrefix(head, []byte{0xEF, 0xBB, 0xBF}) {
		head = head[3:]
	}
	if !bytes.HasPrefix(head, []byte("<?xml")) {
		return ""
	}
	end := bytes.Index(head, []byte("?>"))
	if end < 0 {
		return ""
	}
	decl := string(head[:end])
	idx := strings.Index(strings.ToLower(decl), "encoding")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimLeft(decl[idx+len("encoding"):], " \t\r\n")
	if !strings.HasPrefix(rest, "=") {
		return ""
	}
	rest = strings.TrimLeft(rest[1:], " \t\r\n")
	if rest == "" {
		return ""
	}
	quote := byte(0)
	if rest[0] == '"' || rest[0] == '\'' {
		quote = rest[0]
		rest = rest[1:]
		if i := strings.IndexByte(rest, quote); i >= 0 {
			return strings.TrimSpace(rest[:i])
		}
		return ""
	}
	if i := strings.IndexAny(rest, " \t\r\n"); i >= 0 {
		return strings.TrimSpace(rest[:i])
	}
	return strings.TrimSpace(rest)
}

// declaration 生成 XML 声明行。行尾用 CRLF，与 WVP 的报文风格一致。
func declaration(cs string) []byte {
	return []byte("<?xml version=\"1.0\" encoding=\"" + Normalize(cs) + "\"?>\r\n")
}

// WithDeclaration 确保报文带有与 cs 一致的 XML 声明。
//
// 已有声明时**改写 encoding 属性**（保留 version/standalone 等其余内容），
// 没有则在最前面补一行。调用方应保证 data 是 UTF-8 文本。
func WithDeclaration(data []byte, cs string) []byte {
	want := Normalize(cs)
	head := data
	bom := []byte(nil)
	if bytes.HasPrefix(head, []byte{0xEF, 0xBB, 0xBF}) {
		bom = head[:3]
		head = head[3:]
	}
	if bytes.HasPrefix(head, []byte("<?xml")) {
		if end := bytes.Index(head, []byte("?>")); end >= 0 {
			decl := head[:end]
			lower := strings.ToLower(string(decl))
			if idx := strings.Index(lower, "encoding"); idx >= 0 {
				// 找到 encoding 属性值并替换
				rest := decl[idx+len("encoding"):]
				eq := bytes.IndexByte(rest, '=')
				if eq >= 0 {
					valueStart := idx + len("encoding") + eq + 1
					for valueStart < len(decl) && (decl[valueStart] == ' ' || decl[valueStart] == '\t') {
						valueStart++
					}
					quote := byte(0)
					if valueStart < len(decl) && (decl[valueStart] == '"' || decl[valueStart] == '\'') {
						quote = decl[valueStart]
						valueStart++
					}
					valueEnd := valueStart
					if quote != 0 {
						for valueEnd < len(decl) && decl[valueEnd] != quote {
							valueEnd++
						}
					} else {
						for valueEnd < len(decl) && decl[valueEnd] != ' ' && decl[valueEnd] != '\t' {
							valueEnd++
						}
					}
					out := make([]byte, 0, len(data)+8)
					out = append(out, bom...)
					out = append(out, decl[:valueStart]...)
					out = append(out, want...)
					out = append(out, decl[valueEnd:]...)
					out = append(out, head[end:]...)
					return out
				}
			}
			// 有声明但没有 encoding：在 ?> 前补一个属性
			out := make([]byte, 0, len(data)+24)
			out = append(out, bom...)
			out = append(out, head[:end]...)
			out = append(out, []byte(" encoding=\""+want+"\"")...)
			out = append(out, head[end:]...)
			return out
		}
	}
	out := make([]byte, 0, len(data)+len(declaration(cs)))
	out = append(out, bom...)
	out = append(out, declaration(cs)...)
	out = append(out, head...)
	return out
}

// EncodeForDevice 把平台侧的 UTF-8 XML 文本转成「声明与字节都匹配设备 charset」的报文。
//
// 这是 cmd/sipsvc 发送 MESSAGE 前必须走的一步：业务层只负责拼 UTF-8 XML，
// 字符集转换集中在这里，避免每个调用点各写一遍、各错一处。
func EncodeForDevice(xmlUTF8 string, cs string) ([]byte, error) {
	withDecl := WithDeclaration([]byte(xmlUTF8), cs)
	return FromUTF8(withDecl, cs)
}
