package audio

import (
	"fmt"
	"strconv"
	"strings"
)

// 语音链路的流名约定。
//
// 为什么需要「Agent 流名」：downlink 的音频源不是设备自己的流，而是
// Pipecat 通过 ZLM WebRTC push 建立的另一条流。若复用设备流名，
// startSendRtp 会把设备麦克风原样回推给设备喇叭（自激），且 ZLM 同一
// app/stream 不能同时存在两种来源。
//
// 设备流名在单端口模式下是 SSRC 的大写十六进制（如 0BEBC201）；
// Agent 流名加 "AI" 前缀，既避免撞名（十六进制不含 I），又能在日志里
// 一眼区分上行源与下行源。
const agentStreamPrefix = "AI"

// ParseSSRC 把十进制 SSRC 字符串转成 uint32。
//
// 只做格式校验，不强制 10 位：测试与部分厂商设备会给其它长度，
// 真正的位数约束在 ssrc.Allocator 里。
func ParseSSRC(value string) (uint32, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return 0, fmt.Errorf("SSRC 为空")
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("SSRC %q 不是合法的 uint32 十进制数：%w", s, err)
	}
	return uint32(v), nil
}

// StreamNameFromSSRC 返回 ZLM 单端口模式下使用的流名（%08X 大写）。
func StreamNameFromSSRC(ssrc uint32) string {
	return fmt.Sprintf("%08X", ssrc)
}

// AgentStreamName 由设备流名推导 Pipecat 下行流的 ZLM stream 名。
//
// 确定性推导而不是新增数据库字段：ac_voice_session 只有 Stream/SSRC
// 两个媒体字段，而停止会话时必须能找回 Agent 流。设备流名与 SSRC
// 一一对应，因此「AI + 设备流名」在重启后（只要会话行还在）仍然可算。
//
// 非法字符（路径分隔符、空格等）会被剔除，防止拼出无法作为 ZLM stream
// 的标识；设备流名本来就是十六进制，正常情况下不会触发。
func AgentStreamName(deviceStream string) string {
	s := strings.TrimSpace(deviceStream)
	if s == "" {
		return agentStreamPrefix + "0"
	}
	var b strings.Builder
	b.Grow(len(agentStreamPrefix) + len(s))
	b.WriteString(agentStreamPrefix)
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9',
			r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z':
			b.WriteRune(r)
		}
	}
	if b.Len() == len(agentStreamPrefix) {
		return agentStreamPrefix + "0"
	}
	return b.String()
}

// sessionStreamSeparator 分隔「设备 ZLM 流名」与「Pipecat UDP 端口」。
//
// ac_voice_session 没有单独的端口列；为了把 Pipecat 会话的
// session_id / 端口 / SSRC 都落到现有字段，端口以 `|` 追加在 Stream 后：
//
//	Stream = "0BEBC201|41234"
//
// 这样设备流名仍然可读，解析也向后兼容（旧行没有 `|`，端口按 0 处理）。
// 其它模块不读取语音会话的 Stream，因此不会误把后缀当成流名。
const sessionStreamSeparator = "|"

// EncodeSessionStream 把设备流名与 Pipecat UDP 端口编码进一个字段。
//
// uplinkPort<=0（广播/纯对讲）时只返回设备流名，保持旧语义。
func EncodeSessionStream(deviceStream string, uplinkPort int) string {
	deviceStream = strings.TrimSpace(deviceStream)
	if uplinkPort <= 0 {
		return deviceStream
	}
	return fmt.Sprintf("%s%s%d", deviceStream, sessionStreamSeparator, uplinkPort)
}

// DecodeSessionStream 还原设备流名与 Pipecat UDP 端口。
//
// 解析失败（例如旧数据或手工写入）时把整串当作设备流名，端口为 0，
// 不返回错误 —— 停止会话不应该因为一个展示字段格式不对而失败。
func DecodeSessionStream(value string) (deviceStream string, uplinkPort int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0
	}
	parts := strings.SplitN(value, sessionStreamSeparator, 2)
	deviceStream = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		if port, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && port > 0 {
			uplinkPort = port
		}
	}
	return deviceStream, uplinkPort
}

// AudioBridgeConnected 判断 Pipecat 返回的音频桥状态是否可用。
//
// 只有 connected 表示「上行接收 + 下行推流」都已就绪；degraded 表示
// 只接通了一半（例如 aiortc 缺失或 ZLM WebRTC push 失败），此时
// 对讲仍然听不见 agent 的声音，按「未接通」处理更诚实。
func AudioBridgeConnected(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "connected")
}

// DecideAgent 汇总「是否最终启用语音 Agent」。
//
// 注意判断的是 Pipecat 自报的 **agent_ready**（UDP 已监听 + WebRTC 已连接
// + 管线可运行），而不是 audio_bridge 的瞬时值：会话刚建立时设备还没
// 开始推流，audio_bridge 必然是 degraded；若据此拒绝启用，后端就永远不会
// 调 startSendRtp，形成死锁。
//
// 抽成纯函数是为了让降级路径可单测：用户没请求、控制面不可达、
// 或 Pipecat 自报未就绪，任一条件成立都返回 false。上层据此把
// agent_enabled 落成 false，而不是让整个对讲失败。
func DecideAgent(wantAgent, pipecatReachable, agentReady bool) bool {
	return wantAgent && pipecatReachable && agentReady
}
