package manscdp

import (
	"encoding/xml"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// CharsetUTF8 与 CharsetGB2312 是平台侧 `ac_device.charset` 的两个取值。
const (
	CharsetUTF8   = "UTF-8"
	CharsetGB2312 = "GB2312"
)

// Encode 把结构体编码成 MANSCDP 报文。
//
// charset 决定 XML 声明的 encoding 值以及正文的字节编码：
// GB 系（GB2312/GBK/GB18030）会整体转码为对应字节；UTF-8 则原样输出。
//
// 转码前先拼声明、转码时整体处理 —— 因为声明行本身是纯 ASCII，
// GB 系编码对 ASCII 是恒等映射，所以整段转换不会破坏标签。
func Encode(v interface{}, charset string) ([]byte, error) {
	body, err := xml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("MANSCDP XML 编码失败：%w", err)
	}
	decl := xmlDeclaration(charset)
	out := make([]byte, 0, len(decl)+len(body))
	out = append(out, decl...)
	out = append(out, body...)

	if IsGBCharset(charset) {
		converted, err := simplifiedchinese.GB18030.NewEncoder().Bytes(out)
		if err != nil {
			return nil, fmt.Errorf("MANSCDP 转码为 %s 失败：%w", charset, err)
		}
		return converted, nil
	}
	return out, nil
}

// EncodeString 是 Encode 的字符串便捷版本，便于日志打印。
func EncodeString(v interface{}, charset string) (string, error) {
	b, err := Encode(v, charset)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// xmlDeclaration 生成 `<?xml version="1.0" encoding="X"?>\r\n`。
// 行尾用 CRLF 对齐 WVP 的报文风格（WVP 的 getCatalogXml 用 "\r\n"），便于逐字节比对。
func xmlDeclaration(charset string) []byte {
	return []byte("<?xml version=\"1.0\" encoding=\"" + NormalizeCharset(charset) + "\"?>\r\n")
}

// NormalizeCharset 把 charset 规范成写进 XML 声明的形式。
func NormalizeCharset(charset string) string {
	cs := strings.TrimSpace(charset)
	if cs == "" {
		return CharsetGB2312
	}
	if IsGBCharset(cs) {
		// 统一声明为 GB2312：GBK / GB18030 是它的超集，
		// 对 GB2312 范围内的字符（常用汉字）字节完全一致。
		return CharsetGB2312
	}
	return strings.ToUpper(cs)
}

// IsGBCharset 判断 charset 是否属于 GB 系。
func IsGBCharset(charset string) bool {
	switch strings.ToUpper(strings.TrimSpace(charset)) {
	case "GB2312", "GB-2312", "GBK", "GB18030", "EUC-CN", "CP936":
		return true
	default:
		return false
	}
}
