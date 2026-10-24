package manscdp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/boticz/ai-camera-backend/internal/gb28181/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// ptzPlaceholder 是 PTZCmd 原始字节在 XML 文本里的占位符。
//
// 为什么需要占位符：PTZCmd 以 0xA5 开头，是二进制字节，不是合法的 UTF-8/XML 文本。
// 直接放进字符串会被 encoding/xml 当成非法字符替换掉，设备就收不到指令。
// 因此先用纯 ASCII 占位符完成「序列化 + 字符集转码」，最后再做一次字节级替换。
const ptzPlaceholder = "__AI_CAMERA_PTZ_CMD__"

// Marshal 把结构体编码为带 XML 声明、且字节编码与 charset 一致的报文。
func Marshal(v any, cs string) ([]byte, error) {
	var (
		body []byte
		raw  []byte
		err  error
	)
	switch c := v.(type) {
	case *Control:
		body, raw, err = marshalControl(c)
	case Control:
		body, raw, err = marshalControl(&c)
	default:
		body, err = xml.Marshal(v)
	}
	if err != nil {
		return nil, fmt.Errorf("MANSCDP 编码失败：%w", err)
	}

	// 声明与内容必须一致：先补/改声明，再整体转码（声明行是纯 ASCII，转码恒等）。
	out := charset.WithDeclaration(body, cs)
	out, err = charset.FromUTF8(out, cs)
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		out = bytes.ReplaceAll(out, []byte(ptzPlaceholder), raw)
	}
	return out, nil
}

// MarshalString 是 Marshal 的字符串便捷版本（日志、单测、手工构造时用）。
func MarshalString(v any, cs string) (string, error) {
	b, err := Marshal(v, cs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// marshalControl 序列化控制报文，返回占位符文本与待替换的原始 PTZ 字节。
func marshalControl(c *Control) ([]byte, []byte, error) {
	if c == nil {
		return nil, nil, fmt.Errorf("Control 为 nil")
	}
	// 用独立的 wire 结构避免给对外类型加字符串字段。
	type controlWire struct {
		XMLName   xml.Name `xml:"Control"`
		CmdType   string   `xml:"CmdType"`
		SN        int      `xml:"SN"`
		DeviceID  string   `xml:"DeviceID"`
		PTZCmd    string   `xml:"PTZCmd,omitempty"`
		RecordCmd string   `xml:"RecordCmd,omitempty"`
		TeleBoot  string   `xml:"TeleBoot,omitempty"`
		GuardCmd  string   `xml:"GuardCmd,omitempty"`
		AlarmCmd  string   `xml:"AlarmCmd,omitempty"`
		IFameCmd  string   `xml:"IFameCmd,omitempty"`
		DragZoom  string   `xml:"DragZoom,omitempty"`
		// 2022 精准控制子元素。放在最后与标准 XSD 的 sequence 顺序一致。
		PTZPrecise *PTZPreciseCtrl `xml:"PTZPreciseCtrl,omitempty"`
	}
	wire := controlWire{
		CmdType:    c.CmdType,
		SN:         c.SN,
		DeviceID:   c.DeviceID,
		RecordCmd:  c.RecordCmd,
		TeleBoot:   c.TeleBoot,
		GuardCmd:   c.GuardCmd,
		AlarmCmd:   c.AlarmCmd,
		IFameCmd:   c.IFameCmd,
		DragZoom:   c.DragZoom,
		PTZPrecise: c.PTZPrecise,
	}
	if wire.CmdType == "" {
		wire.CmdType = CmdControl
	}
	if len(c.PTZCmd) > 0 {
		wire.PTZCmd = ptzPlaceholder
	}
	body, err := xml.Marshal(wire)
	if err != nil {
		return nil, nil, err
	}
	return body, c.PTZCmd, nil
}

// Decode 解析报文信封（CmdType / SN / DeviceID），用于「先路由再细解」。
//
// cs 是设备建档时的 ac_device.charset，仅用于**检测声明与实际配置是否一致**
// （结果放在 Envelope.CharsetMismatch），不参与字节解码 —— 字节必须按报文自身的
// 声明解，否则「声明 GB2312、配置写 UTF-8」时会把刚解码对的中文再转坏一次。
//
// 没有 XML 声明时按 UTF-8 严格解析：非法 UTF-8 直接报错（设计 §2.6 第 3 行）。
// 这是「设备注册上来了但目录一条都没有」的根因，必须让调用方看见错误并记日志，
// 不允许退化成静默乱码。
func Decode(body []byte, cs string) (*Envelope, error) {
	declared := charset.Declared(body)
	if declared == "" && !utf8.Valid(body) {
		return nil, fmt.Errorf("MANSCDP 报文没有 XML 声明且不是合法 UTF-8（%d 字节）：设备 charset 为 %q，拒绝解码",
			len(body), cs)
	}

	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.CharsetReader = charsetReader

	var env Envelope
	if err := dec.Decode(&env); err != nil {
		return nil, fmt.Errorf("MANSCDP XML 解码失败：%w", err)
	}
	if env.CmdType == "" {
		return nil, fmt.Errorf("MANSCDP 报文缺少 CmdType（根元素 %s）", env.XMLName.Local)
	}
	env.DeclaredCharset = declared
	if declared != "" && strings.TrimSpace(cs) != "" {
		if charset.Normalize(declared) != charset.Normalize(cs) {
			env.CharsetMismatch = true
		}
	}
	return &env, nil
}

// DecodeInto 在 Decode 的基础上把正文解到具体结构体。
func DecodeInto(body []byte, cs string, v any) (*Envelope, error) {
	env, err := Decode(body, cs)
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.CharsetReader = charsetReader
	if err := dec.Decode(v); err != nil {
		return nil, fmt.Errorf("MANSCDP %s 解码失败：%w", env.CmdType, err)
	}
	return env, nil
}

// Unmarshal 是 DecodeInto 的别名，语义对齐 encoding/xml 的习惯命名。
//
// 与 encoding/xml.Unmarshal 的差别：必须传设备 charset，且无声明时按 UTF-8
// 严格解析（非法字节报错，不静默乱码，见 Decode 的说明）。
func Unmarshal(data []byte, cs string, v any) error {
	_, err := DecodeInto(data, cs, v)
	return err
}

// DecodeCatalog 是 DecodeInto 的目录专用便捷函数。
func DecodeCatalog(body []byte, cs string) (*Envelope, *CatalogResponse, error) {
	var resp CatalogResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodeDeviceInfo 设备信息响应。
func DecodeDeviceInfo(body []byte, cs string) (*Envelope, *DeviceInfoResponse, error) {
	var resp DeviceInfoResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodeDeviceStatus 设备状态响应。
func DecodeDeviceStatus(body []byte, cs string) (*Envelope, *DeviceStatusResponse, error) {
	var resp DeviceStatusResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// DecodeKeepalive 心跳。
func DecodeKeepalive(body []byte, cs string) (*Envelope, *Keepalive, error) {
	var msg Keepalive
	env, err := DecodeInto(body, cs, &msg)
	if err != nil {
		return nil, nil, err
	}
	return env, &msg, nil
}

// DecodeAlarm 报警通知。
func DecodeAlarm(body []byte, cs string) (*Envelope, *Alarm, error) {
	var msg Alarm
	env, err := DecodeInto(body, cs, &msg)
	if err != nil {
		return nil, nil, err
	}
	return env, &msg, nil
}

// DecodeMobilePosition 移动位置通知。
func DecodeMobilePosition(body []byte, cs string) (*Envelope, *MobilePosition, error) {
	var msg MobilePosition
	env, err := DecodeInto(body, cs, &msg)
	if err != nil {
		return nil, nil, err
	}
	return env, &msg, nil
}

// DecodeRecordInfo 录像查询响应。
func DecodeRecordInfo(body []byte, cs string) (*Envelope, *RecordInfoResponse, error) {
	var resp RecordInfoResponse
	env, err := DecodeInto(body, cs, &resp)
	if err != nil {
		return nil, nil, err
	}
	return env, &resp, nil
}

// charsetReader 把 XML 声明中的非 UTF-8 编码转成 UTF-8。
//
// encoding/xml 对 "UTF-8"/"utf-8" 不会调用本函数，其余编码都走这里。
// 未知编码返回错误而不是猜一个：宁可显式失败并记日志，也不要静默乱码。
func charsetReader(label string, input io.Reader) (io.Reader, error) {
	if charset.IsGB(label) {
		return simplifiedchinese.GB18030.NewDecoder().Reader(input), nil
	}
	if strings.EqualFold(strings.TrimSpace(label), "utf-8") {
		return input, nil
	}
	return nil, fmt.Errorf("不支持的 XML 编码 %q（仅支持 UTF-8 / GB2312 / GBK / GB18030）", label)
}
