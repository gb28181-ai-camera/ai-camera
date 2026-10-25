package manscdp

import "fmt"

// ════════════════════════════════════════════════════════════════
// 云台巡航 / 扫描扩展指令（GB/T 28181-2016/2022 附录 A.3.5 / A.3.6）
//
// 8 字节 PTZCmd 布局（与 ptz.go 的 buildPTZBytes 一致）：
//
//	[0] 0xA5  [1] 0x0F  [2] 0x01  [3] 指令码
//	[4] 数据1  [5] 数据2  [6] 数据3(高 4 bit)  [7] 校验和
//
// 巡航（表 A.9）：
//
//	0x84 加入巡航点      数据1=巡航组号, 数据2=预置位号
//	0x85 删除巡航点      数据1=巡航组号, 数据2=预置位号（0 表示删除整条巡航）
//	0x86 设置巡航速度    数据1=巡航组号, 数据2/3=12 bit 速度
//	0x87 设置巡航停留时间 数据1=巡航组号, 数据2/3=12 bit 秒数
//	0x88 开始巡航        数据1=巡航组号, 数据2=0
//
// 扫描（表 A.10）：
//
//	0x89 数据2=0 开始自动扫描；数据2=1 左边界；数据2=2 右边界
//	0x8A 设置扫描速度    数据1=扫描组号, 数据2/3=12 bit 速度
//
// 停止巡航/扫描使用「指令码为 0」的全零 PTZ 停止指令。
// ════════════════════════════════════════════════════════════════

// 云台巡航/扫描指令码（导出给业务层与单测使用）。
const (
	PtzCmdCruisePointAdd    byte = 0x84
	PtzCmdCruisePointDelete byte = 0x85
	PtzCmdCruiseSpeed       byte = 0x86
	PtzCmdCruiseStayTime    byte = 0x87
	PtzCmdCruiseStart       byte = 0x88
	PtzCmdScan              byte = 0x89
	PtzCmdScanSpeed         byte = 0x8A
)

// 扫描 0x89 的数据2 取值。
const (
	ScanActionStart      = 0
	ScanActionLeftBound  = 1
	ScanActionRightBound = 2
)

// BuildCruisePointCmd 构造「加入巡航点」指令（0x84）。
func BuildCruisePointCmd(groupID, presetID int) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("巡航组号必须在 1-255 之间")
	}
	if presetID < 1 || presetID > 255 {
		return nil, fmt.Errorf("预置位编号必须在 1-255 之间")
	}
	return buildExtPTZCmd(PtzCmdCruisePointAdd, groupID, presetID, 0), nil
}

// BuildDeleteCruisePointCmd 构造「删除巡航点」指令（0x85）。
// presetID 传 0 表示删除整条巡航（标准规定数据2=0）。
func BuildDeleteCruisePointCmd(groupID, presetID int) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("巡航组号必须在 1-255 之间")
	}
	if presetID < 0 || presetID > 255 {
		return nil, fmt.Errorf("预置位编号必须在 0-255 之间")
	}
	return buildExtPTZCmd(PtzCmdCruisePointDelete, groupID, presetID, 0), nil
}

// BuildCruiseSpeedCmd 构造「设置巡航速度」指令（0x86）。
// speed 为标准 12 bit 量纲（1–4095）。
func BuildCruiseSpeedCmd(groupID, speed int) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("巡航组号必须在 1-255 之间")
	}
	if speed < 1 || speed > 4095 {
		return nil, fmt.Errorf("巡航速度必须在 1-4095 之间")
	}
	return buildExtPTZCmd(PtzCmdCruiseSpeed, groupID, speed&0xFF, (speed>>8)&0x0F), nil
}

// BuildCruiseStayTimeCmd 构造「设置巡航停留时间」指令（0x87）。
// seconds 为标准 12 bit 量纲（1–4095），单位秒。
func BuildCruiseStayTimeCmd(groupID, seconds int) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("巡航组号必须在 1-255 之间")
	}
	if seconds < 1 || seconds > 4095 {
		return nil, fmt.Errorf("巡航停留时间必须在 1-4095 秒之间")
	}
	return buildExtPTZCmd(PtzCmdCruiseStayTime, groupID, seconds&0xFF, (seconds>>8)&0x0F), nil
}

// BuildCruiseStartCmd 构造「开始巡航」指令（0x88）。
func BuildCruiseStartCmd(groupID int) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("巡航组号必须在 1-255 之间")
	}
	return buildExtPTZCmd(PtzCmdCruiseStart, groupID, 0, 0), nil
}

// BuildScanStartCmd 构造「开始自动扫描」指令（0x89，数据2=0）。
func BuildScanStartCmd(groupID int) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("扫描组号必须在 1-255 之间")
	}
	return buildExtPTZCmd(PtzCmdScan, groupID, ScanActionStart, 0), nil
}

// BuildScanBoundCmd 构造「设置扫描左/右边界」指令（0x89，数据2=1/2）。
//
// ★ 标准语义是把云台**当前位置**记为边界，指令不携带角度值。
// 调用方必须先通过方向控制把云台转到目标位置，再下发本指令。
func BuildScanBoundCmd(groupID int, left bool) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("扫描组号必须在 1-255 之间")
	}
	action := ScanActionRightBound
	if left {
		action = ScanActionLeftBound
	}
	return buildExtPTZCmd(PtzCmdScan, groupID, action, 0), nil
}

// BuildScanSpeedCmd 构造「设置扫描速度」指令（0x8A）。
func BuildScanSpeedCmd(groupID, speed int) ([]byte, error) {
	if groupID < 1 || groupID > 255 {
		return nil, fmt.Errorf("扫描组号必须在 1-255 之间")
	}
	if speed < 1 || speed > 4095 {
		return nil, fmt.Errorf("扫描速度必须在 1-4095 之间")
	}
	return buildExtPTZCmd(PtzCmdScanSpeed, groupID, speed&0xFF, (speed>>8)&0x0F), nil
}

// BuildPTZStopCmd 构造云台停止指令（指令码全零）。
//
// 标准规定停止巡航/扫描都复用这条指令，而不是单独的 0x86/0x8A。
func BuildPTZStopCmd() []byte {
	return buildExtPTZCmd(0x00, 0, 0, 0)
}

// buildExtPTZCmd 组装 8 字节扩展指令并计算校验和。
func buildExtPTZCmd(code byte, data1, data2, data3 int) []byte {
	out := []byte{
		0xA5, 0x0F, 0x01, code,
		byte(data1 & 0xFF),
		byte(data2 & 0xFF),
		byte((data3 & 0x0F) << 4),
		0,
	}
	out[7] = checksum(out[:7])
	return out
}
