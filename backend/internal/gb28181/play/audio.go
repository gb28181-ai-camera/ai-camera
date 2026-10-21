package play

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/model"
)

// 语音会话的 SDP s= 取值（GB28181 附录 A.2）。
//
// 点播/回放有 sdp.SessionNamePlay / Playback；语音广播与对讲在 SDP 里
// 用 Broadcast / Talk 区分，设备据此选择音频通道行为。
const (
	SessionNameBroadcast = "Broadcast"
	SessionNameTalk      = "Talk"
)

// AudioPayloadTypePCMA 是 G.711A 的 RTP 负载类型（静态 PT 8）。
const AudioPayloadTypePCMA = 8

// BuildAudioSDP 构造语音广播/对讲的平台侧 SDP。
//
// 与视频点播的差异（为什么不能复用 sipserver.BuildPlaySDP）：
//   - 媒体行必须是 m=audio（视频点播是 m=video）；
//   - 编码是 PCMA/8000（G.711A），不是 PS；
//   - 广播/对讲需要 a=sendrecv：平台要往设备喇叭推音频。
//
// 收流地址仍用 ZLM 的单端口代理（rtp_proxy_port）：设备先把会话里的
// 音频送到 ZLM，startSendRtp 再按设备 200 OK 里的地址回推。
//
// o= 用户名按契约 §15.5b 填设备国标编码（与点播一致，避免 ACK 打不到设备）。
func BuildAudioSDP(dev *model.Device, ip string, port int, ssrc, sessionName string) string {
	origin := ""
	if dev != nil {
		origin = strings.TrimSpace(dev.GbDeviceID)
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "127.0.0.1"
	}
	if port <= 0 {
		port = 10000
	}
	if sessionName == "" {
		sessionName = SessionNameTalk
	}

	var b strings.Builder
	b.WriteString("v=0\r\n")
	b.WriteString("o=" + origin + " 0 0 IN IP4 " + ip + "\r\n")
	b.WriteString("s=" + sessionName + "\r\n")
	b.WriteString("c=IN IP4 " + ip + "\r\n")
	b.WriteString("t=0 0\r\n")
	b.WriteString("m=audio " + strconv.Itoa(port) + " RTP/AVP " + strconv.Itoa(AudioPayloadTypePCMA) + "\r\n")
	b.WriteString("a=sendrecv\r\n")
	b.WriteString("a=rtpmap:" + strconv.Itoa(AudioPayloadTypePCMA) + " PCMA/8000\r\n")
	b.WriteString("a=ptime:20\r\n")
	b.WriteString("y=" + ssrc + "\r\n")
	return b.String()
}

// ParseAudioEndpoint 从设备 200 OK 的 SDP 中解析音频接收地址与端口。
//
// 语音回推（ZLM startSendRtp）的 dst_url/dst_port 只能来自设备侧 SDP：
// 设备在 m=audio 里告诉平台「把 RTP 发到哪个 IP:端口」。
// 解析失败时必须显式返回 ok=false —— 编一个平台自己的地址去推流只会
// 让音频打进 ZLM 自己，表现为「接口成功、设备没声音」。
//
// 注意：c= 是会话级默认值，m=audio 也可能带自己的 c= 行；这里优先用
// m=audio 之后、下一个 m= 之前出现的 c=，否则回落到会话级 c=。
func ParseAudioEndpoint(body string) (ip string, port int, ok bool) {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")

	sessionIP := ""
	audioIP := ""
	audioPort := 0
	inAudio := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "c="):
			v := strings.TrimSpace(strings.TrimPrefix(line, "c="))
			fields := strings.Fields(v) // IN IP4 10.0.0.1
			if len(fields) >= 3 {
				if inAudio {
					audioIP = fields[2]
				} else {
					sessionIP = fields[2]
				}
			}
		case strings.HasPrefix(line, "m="):
			fields := strings.Fields(line)
			if len(fields) >= 2 && strings.EqualFold(fields[0], "m=audio") {
				if p, err := strconv.Atoi(fields[1]); err == nil && p > 0 {
					audioPort = p
				}
				inAudio = true
			} else {
				// 进入下一条媒体行，结束音频段。
				inAudio = false
			}
		}
	}

	ip = audioIP
	if ip == "" {
		ip = sessionIP
	}
	if ip == "" || audioPort <= 0 {
		return "", 0, false
	}
	return ip, audioPort, true
}

// FormatSubject 构造 INVITE 的 Subject 头：<通道编码>:<ssrc>,<域>。
//
// 这是 WVP 的约定，很多设备用它关联会话与 SSRC；缺失时部分设备虽然
// 回 200 OK，但不会推流，且没有任何报错。
func FormatSubject(channelID, ssrc, domain string) string {
	return fmt.Sprintf("%s:%s,%s", channelID, ssrc, domain)
}
