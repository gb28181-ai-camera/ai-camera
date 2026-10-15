// Package sdp 解析与构造 GB28181 实时点播所用的会话描述（SDP，RFC 4566）。
//
// 它只覆盖点播场景需要的那一小撮字段：`c=`（平台收流地址）、`m=video <端口>`、
// `a=setup`、`y=`（平台分配的 SSRC）与 `s=`（会话名 Play / Playback）。
// 其中 `y=` 与 `f=` 是 GB28181 对 SDP 的私有扩展（不是 `a=` 属性行），
// 标准 SDP 解析器（如 Java 的 jain-sip）会因不认识它们而解析失败 ——
// WVP 的 `SipUtils.getSsrcFromSdp` 正是靠**先手工摘掉 y= 行**再交给 jain-sip 的。
// 因此本包**自己按行扫**，不依赖任何 SDP 库，顺带把 `y=` 原样收下。
package sdp

import (
	"fmt"
	"strconv"
	"strings"
)

// Origin 是 `o=` 行的五个字段（username 尤其重要：WVP 用它构造 ACK 的 Request-URI）。
type Origin struct {
	Username string // 用户名，点播时 WVP 填的是**设备编码**
	SessID   string // 会话标识
	Version  string // 会话版本
	NetType  string // IN
	AddrType string // IP4 / IP6
	Address  string // 会话源地址
}

// Media 是一段 `m=` 媒体描述及其附带的属性。
type Media struct {
	Type         string              // video / audio
	Port         int                 // 端口
	Protocol     string              // RTP/AVP 或 TCP/RTP/AVP（区分 UDP / TCP 模式）
	Formats      []string            // 负载类型列表，如 ["96","98","97","99"]
	ConnectionIP string              // 媒体级 c=，为空表示沿用会话级
	Attrs        map[string][]string // a= 属性：名字 → 值列表（无值属性为空串）
	Extras       map[string]string   // 非标准的单字母行：y= / f= 等
}

// Session 是一份解析后的 SDP 会话描述。
type Session struct {
	Version      string
	Origin       Origin
	Name         string              // s=，点播为 Play，回放为 Playback
	ConnectionIP string              // 会话级 c=
	Time         string              // t=，通常 "0 0"
	Attrs        map[string][]string // 会话级 a= 属性
	Extras       map[string]string   // 会话级非标准行
	Medias       []Media
}

// Parse 解析一份 SDP 文本。
//
// 行分隔符同时接受 CRLF 与 LF（真实设备两种都有）。
// 按 RFC 4566：第一个 `m=` 之前的行是会话级，之后的行归属当前媒体段；
// 但 `y=` / `f=` 这类私有行**两种层级都可能出现**（WVP 放在 m= 之后，即媒体级），
// 因此查找 SSRC 时要两级都看（见 Session.VideoSSRC）。
func Parse(text string) (*Session, error) {
	s := &Session{
		Attrs:  map[string][]string{},
		Extras: map[string]string{},
	}
	lines := splitLines(text)
	if len(lines) == 0 {
		return nil, fmt.Errorf("SDP 为空")
	}

	// 当前所处的媒体段下标；-1 表示仍在会话级。
	mediaIdx := -1
	current := func() *Media {
		if mediaIdx < 0 || mediaIdx >= len(s.Medias) {
			return nil
		}
		return &s.Medias[mediaIdx]
	}

	for _, line := range lines {
		if len(line) < 2 || line[1] != '=' {
			return nil, fmt.Errorf("SDP 行格式非法（缺少 '='）：%q", line)
		}
		kind, value := line[0], line[2:]

		switch kind {
		case 'm':
			m, err := parseMedia(value)
			if err != nil {
				return nil, err
			}
			s.Medias = append(s.Medias, m)
			mediaIdx = len(s.Medias) - 1
		case 'v', 's', 'c', 't', 'o', 'y', 'f':
			// 这些字段在会话级与媒体级都有意义，按当前层级落位。
			if cur := current(); cur != nil {
				applyField(kind, value, cur, nil)
			} else {
				applyField(kind, value, nil, s)
			}
		case 'a':
			if cur := current(); cur != nil {
				putAttr(cur.Attrs, value)
			} else {
				putAttr(s.Attrs, value)
			}
		default:
			// 其它单字母行（b= / k= / z= …）本场景用不到，但按私有行收下，
			// 避免因为厂商塞了额外字段就整份解析失败。
			if cur := current(); cur != nil {
				cur.Extras[string(kind)] = value
			} else {
				s.Extras[string(kind)] = value
			}
		}
	}

	if len(s.Medias) == 0 {
		return nil, fmt.Errorf("SDP 缺少 m= 行")
	}
	return s, nil
}

// parseMedia 解析 `m=` 的取值：`<类型> <端口> <协议> [负载类型…]`。
func parseMedia(value string) (Media, error) {
	fields := strings.Fields(value)
	if len(fields) < 3 {
		return Media{}, fmt.Errorf("m= 行字段不足：%q", value)
	}
	port, err := strconv.Atoi(fields[1])
	if err != nil {
		return Media{}, fmt.Errorf("m= 端口非法：%q", fields[1])
	}
	return Media{
		Type:     strings.ToLower(fields[0]),
		Port:     port,
		Protocol: fields[2],
		Formats:  fields[3:],
		Attrs:    map[string][]string{},
		Extras:   map[string]string{},
	}, nil
}

// applyField 把会话级/媒体级的具名字段落到对应结构上。
// 当 media 非 nil 时写媒体级，否则写会话级。
func applyField(kind byte, value string, media *Media, sess *Session) {
	switch kind {
	case 'v':
		if sess != nil {
			sess.Version = value
		}
	case 's':
		if sess != nil {
			sess.Name = value
		}
	case 't':
		if sess != nil {
			sess.Time = value
		}
	case 'c':
		ip := parseConnection(value)
		if media != nil {
			media.ConnectionIP = ip
		} else {
			sess.ConnectionIP = ip
		}
	case 'o':
		if sess != nil {
			sess.Origin = parseOrigin(value)
		}
	case 'y', 'f':
		// y= 是平台分配的 SSRC，f= 是媒体能力描述；两者都是 GB28181 私有扩展，
		// 原样收进 Extras，不做解释。
		if media != nil {
			media.Extras[string(kind)] = value
		} else {
			sess.Extras[string(kind)] = value
		}
	}
}

// putAttr 把 `a=` 的一行拆成「名字 → 值」追加进 map。
// `a=recvonly`（无值）→ {"recvonly": [""]}；`a=setup:passive` → {"setup": ["passive"]}。
//
// 用切片而不是单值：`a=rtpmap` 会为每个负载类型各出现一次，
// 单值 map 会把前面的覆盖掉，导致「码流是不是 PS」这类判断拿到错误答案。
func putAttr(dst map[string][]string, value string) {
	name, val, _ := strings.Cut(value, ":")
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	dst[name] = append(dst[name], val)
}

// firstAttr 取某个属性名的第一个值，不存在时返回空串并置 ok=false。
func firstAttr(attrs map[string][]string, name string) (string, bool) {
	vals, ok := attrs[name]
	if !ok || len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}

// parseConnection 从 `IN IP4 10.0.0.5` 里取出地址部分。
func parseConnection(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

// parseOrigin 解析 `o=` 的六个字段。
func parseOrigin(value string) Origin {
	fields := strings.Fields(value)
	o := Origin{}
	if len(fields) > 0 {
		o.Username = fields[0]
	}
	if len(fields) > 1 {
		o.SessID = fields[1]
	}
	if len(fields) > 2 {
		o.Version = fields[2]
	}
	if len(fields) > 3 {
		o.NetType = fields[3]
	}
	if len(fields) > 4 {
		o.AddrType = fields[4]
	}
	if len(fields) > 5 {
		o.Address = fields[5]
	}
	return o
}

// splitLines 按 CRLF / LF / CR 切行，并丢弃空行。
func splitLines(text string) []string {
	raw := strings.FieldsFunc(text, func(r rune) bool { return r == '\r' || r == '\n' })
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// VideoMedia 返回第一个视频媒体段，没有则返回 nil。
func (s *Session) VideoMedia() *Media {
	for i := range s.Medias {
		if s.Medias[i].Type == "video" {
			return &s.Medias[i]
		}
	}
	return nil
}

// VideoSSRC 返回视频媒体段声明的 SSRC（`y=`）。
//
// 先查媒体级、再查会话级 —— 因为 `y=` 出现在哪一级取决于对端实现。
// 都没有时返回空串，调用方需自行兜底（见 InviteInfo 的注释）。
func (s *Session) VideoSSRC() string {
	if v := s.VideoMedia(); v != nil {
		if ssrc := v.Extras["y"]; ssrc != "" {
			return ssrc
		}
	}
	return s.Extras["y"]
}

// Setup 返回 `a=setup:` 的取值（active / passive / actpass），没有则为空。
// 空值意味着 UDP 模式 —— 只有 TCP 模式才会带 a=setup。
func (s *Session) Setup() string {
	if v := s.VideoMedia(); v != nil {
		if setup, ok := firstAttr(v.Attrs, "setup"); ok {
			return setup
		}
	}
	setup, _ := firstAttr(s.Attrs, "setup")
	return setup
}

// VideoConnectionIP 返回视频段实际生效的收流地址：媒体级 c= 优先，否则会话级。
func (s *Session) VideoConnectionIP() string {
	if v := s.VideoMedia(); v != nil && v.ConnectionIP != "" {
		return v.ConnectionIP
	}
	return s.ConnectionIP
}

// HasFormat 判断 m= 的负载类型列表里是否包含某个值（如 "96"）。
func (m *Media) HasFormat(pt string) bool {
	for _, f := range m.Formats {
		if f == pt {
			return true
		}
	}
	return false
}

// Rtpmap 返回某个负载类型对应的 `a=rtpmap` 取值（如 "96" → "PS/90000"），没有则空串。
func (m *Media) Rtpmap(pt string) string {
	for _, v := range m.Attrs["rtpmap"] {
		// 形如 "96 PS/90000"
		codec, enc, found := strings.Cut(v, " ")
		if found && codec == pt {
			return enc
		}
	}
	return ""
}

// IsUDP 判断该媒体段是否为 UDP（非 TCP）收流。
func (m *Media) IsUDP() bool {
	return !strings.HasPrefix(strings.ToUpper(m.Protocol), "TCP")
}
