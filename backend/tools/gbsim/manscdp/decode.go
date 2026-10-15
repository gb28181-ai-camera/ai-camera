package manscdp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Decode 把 MANSCDP 报文解码到 v。
//
// fallbackCharset 是「报文没有 XML 声明时」采用的编码，通常取设备建档时的
// `ac_device.charset`（§4.1）。两条路径都要处理，缺一不可：
//
//  1. 有声明且为 GB 系 → 交给 Decoder.CharsetReader 转码。
//     Go 的 encoding/xml 在声明非 UTF-8 而 CharsetReader 为 nil 时**直接返回错误**
//     （`xml: encoding "GB2312" declared but Decoder.CharsetReader is nil`），
//     表现是「设备注册上来了但目录一条都没有」，而不是乱码 —— 这是最容易漏的一条路径。
//  2. 无声明 → 声明缺失时 CharsetReader 根本不会被调用，必须自己按 fallback 预转码。
//     **实测更正**：这条路径在 Go 下不是静默乱码，而是硬报错
//     `XML syntax error on line 1: invalid UTF-8`（encoding/xml 默认 Strict 模式会校验 UTF-8），
//     因此「按乱码预期写兜底」同样会漏掉它。
//  3. 真正会**静默乱码**的是「声明与实际字节编码不一致」：例如声明 GB2312 却发了 UTF-8 字节，
//     转码器照单全收、不报错，解出来是 `涓滈棬鐞冩満` 这样的乱码。
//     这条路径代码层面无法自动判别，只能靠端到端用例（真实设备）覆盖。
//
// 声明了一个我们不认识的编码时，CharsetReader 返回错误让整条报文失败 ——
// 宁可显式失败并记日志，也不要静默吞掉（对齐 §2.6 的结论）。
func Decode(data []byte, fallbackCharset string, v interface{}) error {
	src := data
	if declared := DeclaredCharset(data); declared == "" && IsGBCharset(fallbackCharset) {
		converted, err := simplifiedchinese.GB18030.NewDecoder().Bytes(data)
		if err != nil {
			return fmt.Errorf("MANSCDP 按 %s 预转码失败（报文无 XML 声明）：%w", fallbackCharset, err)
		}
		src = converted
	}

	dec := xml.NewDecoder(bytes.NewReader(src))
	dec.CharsetReader = charsetReader
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("MANSCDP XML 解码失败：%w", err)
	}
	return nil
}

// CmdTypeOf 只解出 CmdType / SN / DeviceID，用于「先路由再细解」。
func CmdTypeOf(data []byte, fallbackCharset string) (*Envelope, error) {
	var env Envelope
	if err := Decode(data, fallbackCharset, &env); err != nil {
		return nil, err
	}
	if env.CmdType == "" {
		return nil, fmt.Errorf("MANSCDP 报文缺少 CmdType")
	}
	return &env, nil
}

// charsetReader 是 xml.Decoder.CharsetReader 的实现：把声明中的非 UTF-8 编码转成 UTF-8。
//
// 注意 encoding/xml 对 "UTF-8"/"utf-8" 的声明**不会**调用本函数（内建处理），
// 所以这里只需覆盖 GB 系与「未知编码」两种情况。
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	if IsGBCharset(charset) {
		return simplifiedchinese.GB18030.NewDecoder().Reader(input), nil
	}
	return nil, fmt.Errorf("不支持的 XML 编码 %q（仅支持 UTF-8 / GB2312 / GBK / GB18030）", charset)
}

// DeclaredCharset 从 XML 声明中提取 encoding 值；没有声明或没有 encoding 属性时返回 ""。
//
// 只在报文最前面找，不做全文扫描 —— 声明若没出现在开头，本来就已违反 XML 规范。
func DeclaredCharset(data []byte) string {
	// 允许 UTF-8 BOM 前缀
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
	rest := decl[idx+len("encoding"):]
	rest = strings.TrimLeft(rest, " \t\r\n")
	if !strings.HasPrefix(rest, "=") {
		return ""
	}
	rest = strings.TrimLeft(rest[1:], " \t\r\n")
	quote := byte(0)
	if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
		quote = rest[0]
		rest = rest[1:]
	}
	if quote != 0 {
		if i := strings.IndexByte(rest, quote); i >= 0 {
			return strings.TrimSpace(rest[:i])
		}
		return ""
	}
	// 无引号：取到空白为止
	if i := strings.IndexAny(rest, " \t\r\n"); i >= 0 {
		return strings.TrimSpace(rest[:i])
	}
	return strings.TrimSpace(rest)
}
