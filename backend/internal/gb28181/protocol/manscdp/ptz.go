package manscdp

import "fmt"

// 云台方向取值，必须与前端 PtzControl 组件、契约 §15.2 的枚举一致。
const (
	DirectionUp        = "up"
	DirectionDown      = "down"
	DirectionLeft      = "left"
	DirectionRight     = "right"
	DirectionUpLeft    = "up-left"
	DirectionUpRight   = "up-right"
	DirectionDownLeft  = "down-left"
	DirectionDownRight = "down-right"
	DirectionStop      = "stop"
)

// 变焦 / 光圈 / 聚焦动作取值（契约 §15.2）。
const (
	ActionZoomIn    = "zoom-in"
	ActionZoomOut   = "zoom-out"
	ActionZoomStop  = "zoom-stop"
	ActionIrisIn    = "iris-in"
	ActionIrisOut   = "iris-out"
	ActionIrisStop  = "iris-stop"
	ActionFocusNear = "focus-near"
	ActionFocusFar  = "focus-far"
	ActionFocusStop = "focus-stop"
)

// PTZ 指令码的位定义。
//
// 这组取值沿用 WVP `PtzCmdUtils` 的约定（真机兼容性最好）：
//
//	0x08 上    0x04 下    0x02 左    0x01 右
//	0x10 变倍+  0x20 变倍-  0x40 变焦+  0x80 变焦-
//
// 多方向按位或组合（如左上 = 0x08|0x02 = 0x0A）。
const (
	ptzCmdUp    byte = 0x08
	ptzCmdDown  byte = 0x04
	ptzCmdLeft  byte = 0x02
	ptzCmdRight byte = 0x01

	ptzCmdZoomIn  byte = 0x10
	ptzCmdZoomOut byte = 0x20
	// 光圈/聚焦共用 0x40/0x80，具体含义由设备的 CommandType 区分。
	ptzCmdFocusNear byte = 0x40
	ptzCmdFocusFar  byte = 0x80
)

// BuildDirectionCmd 按方向与速度构造 8 字节 PTZ 指令。
//
// 速度量纲是契约钉死的 **1–8**（1 最慢、8 最快），直接填入指令的
// 水平/垂直/变倍速度字节。越界值会被夹到 1–8，避免前端传 0 导致「按了没反应」。
func BuildDirectionCmd(direction string, speed int) ([]byte, error) {
	var cmd byte
	switch direction {
	case DirectionUp:
		cmd = ptzCmdUp
	case DirectionDown:
		cmd = ptzCmdDown
	case DirectionLeft:
		cmd = ptzCmdLeft
	case DirectionRight:
		cmd = ptzCmdRight
	case DirectionUpLeft:
		cmd = ptzCmdUp | ptzCmdLeft
	case DirectionUpRight:
		cmd = ptzCmdUp | ptzCmdRight
	case DirectionDownLeft:
		cmd = ptzCmdDown | ptzCmdLeft
	case DirectionDownRight:
		cmd = ptzCmdDown | ptzCmdRight
	case DirectionStop, "":
		cmd = 0x00
	default:
		return nil, fmt.Errorf("未知的云台方向 %q", direction)
	}
	return buildPTZBytes(cmd, speed), nil
}

// BuildActionCmd 构造变焦 / 光圈 / 聚焦指令。
func BuildActionCmd(action string, speed int) ([]byte, error) {
	var cmd byte
	switch action {
	case ActionZoomIn:
		cmd = ptzCmdZoomIn
	case ActionZoomOut:
		cmd = ptzCmdZoomOut
	case ActionZoomStop:
		cmd = 0x00
	case ActionIrisIn:
		cmd = ptzCmdFocusNear
	case ActionIrisOut:
		cmd = ptzCmdFocusFar
	case ActionIrisStop:
		cmd = 0x00
	case ActionFocusNear:
		cmd = ptzCmdFocusNear
	case ActionFocusFar:
		cmd = ptzCmdFocusFar
	case ActionFocusStop:
		cmd = 0x00
	default:
		return nil, fmt.Errorf("未知的云台动作 %q", action)
	}
	return buildPTZBytes(cmd, speed), nil
}

// buildPTZBytes 组装 `A5 0F 01 <cmd> <horiz> <vert> <zoom> <sum>`。
//
// 第 8 字节是前 7 字节的算术和（模 256），设备用它做完整性校验。
func buildPTZBytes(cmd byte, speed int) []byte {
	s := speedByte(speed)
	out := []byte{0xA5, 0x0F, 0x01, cmd, s, s, s, 0}
	out[7] = checksum(out[:7])
	return out
}

// speedByte 把契约的 1–8 夹到合法区间。0 或负数按最慢处理，>8 按最快处理。
func speedByte(speed int) byte {
	if speed < 1 {
		return 1
	}
	if speed > 8 {
		return 8
	}
	return byte(speed)
}

// checksum 8 字节指令的校验和。
func checksum(head []byte) byte {
	var sum byte
	for _, b := range head {
		sum += b
	}
	return sum
}

// NewControl 构造一条控制报文。CmdType 固定为 DeviceControl。
func NewControl(deviceID string, sn int) *Control {
	return &Control{CmdType: CmdControl, SN: sn, DeviceID: deviceID}
}

// NewPTZControl 构造一条 PTZ 控制报文。
func NewPTZControl(deviceID string, sn int, cmd []byte) *Control {
	c := NewControl(deviceID, sn)
	c.PTZCmd = cmd
	return c
}

// NewRecordControl 构造录像开始/停止控制。
func NewRecordControl(deviceID string, sn int, start bool) *Control {
	c := NewControl(deviceID, sn)
	if start {
		c.RecordCmd = RecordCmdStart
	} else {
		c.RecordCmd = RecordCmdStop
	}
	return c
}

// NewDeviceControl 构造设备控制（重启 / 报警复位 / 布撤防 / 校时）。
func NewDeviceControl(deviceID string, sn int, teleBoot, alarmCmd, guardCmd string) *Control {
	return &Control{
		CmdType:  CmdControl,
		SN:       sn,
		DeviceID: deviceID,
		TeleBoot: teleBoot,
		AlarmCmd: alarmCmd,
		GuardCmd: guardCmd,
	}
}
